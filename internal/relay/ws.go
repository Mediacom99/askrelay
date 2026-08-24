package relay

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	// maxWSFrame bounds an inbound /ws frame (acks are tiny).
	maxWSFrame = 128 << 10
	// writeTimeout bounds a single push write, so a stalled/dead socket fails
	// the write instead of blocking a goroutine forever (a full send buffer on a
	// sleeping laptop / dropped NAT session has nothing to cancel otherwise).
	writeTimeout = 10 * time.Second
	// revokeCheckInterval bounds how long a revoked device's idle socket can
	// linger — the reconcile loop severs it (also catches out-of-process revokes,
	// which the running relay never sees directly: today a direct UPDATE of
	// devices.revoked_at, and the `device revoke` CLI once WP-12 lands).
	revokeCheckInterval = 15 * time.Second
)

// wsFrame is the /ws wire form (JSON text). Server→daemon: type "message" with
// the delivery fields. Daemon→server: type "ack" with the message id.
// (Daemon-signed outbound submit is deferred to WP-09, where it is gated through
// a released draft rather than ingested raw.)
type wsFrame struct {
	Type     string          `json:"type"`
	ID       string          `json:"id,omitempty"`
	ThreadID string          `json:"thread_id,omitempty"`
	SenderID string          `json:"sender_id,omitempty"`
	Envelope json.RawMessage `json:"envelope,omitempty"`
}

// wsConn is one live socket plus its coalescing wake channel. All writes flow
// through the single per-connection writerLoop, so no write mutex is needed.
type wsConn struct {
	conn *websocket.Conn
	wake chan struct{} // cap 1; a non-blocking send coalesces overlapping pushes
}

func (c *wsConn) writeFrame(ctx context.Context, f wsFrame) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return c.conn.Write(ctx, websocket.MessageText, b)
}

// hub is the in-memory registry of live daemon WebSocket connections, keyed by
// person then device. It is the delivery/approval push fan-out target (WP-08);
// nothing authoritative lives here — the durable inbox (WP-03) is the source of
// truth, so a dropped connection loses only a nudge, re-sent on reconnect.
type hub struct {
	mu    sync.Mutex
	conns map[string]map[string]*wsConn // personID -> deviceID -> conn
}

func newHub() *hub {
	return &hub{conns: make(map[string]map[string]*wsConn)}
}

// add registers c under (person, device); a reconnect supersedes and closes any
// previous socket for the same device.
func (h *hub) add(person, device string, c *wsConn) {
	h.mu.Lock()
	old := h.conns[person][device]
	byDevice := h.conns[person]
	if byDevice == nil {
		byDevice = make(map[string]*wsConn)
		h.conns[person] = byDevice
	}
	byDevice[device] = c
	h.mu.Unlock()
	if old != nil {
		_ = old.conn.CloseNow() // best-effort: superseded by a reconnect
	}
}

// remove drops (person, device) only if c is still the registered conn — a
// socket already superseded by a reconnect must not evict its replacement.
func (h *hub) remove(person, device string, c *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if byDevice := h.conns[person]; byDevice != nil && byDevice[device] == c {
		delete(byDevice, device)
		if len(byDevice) == 0 {
			delete(h.conns, person)
		}
	}
}

// closeAll closes every registered connection so the read loops exit and Run can
// return. Best-effort and abrupt (CloseNow, no close handshake) — hijacked WS
// conns aren't tracked by http.Server's graceful shutdown, so this is the drain.
func (h *hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, byDevice := range h.conns {
		for _, c := range byDevice {
			_ = c.conn.CloseNow()
		}
	}
	h.conns = make(map[string]map[string]*wsConn)
}

// snapshot copies a person's device→conn map so callers can push without holding
// the lock during I/O.
func (h *hub) snapshot(person string) map[string]*wsConn {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]*wsConn, len(h.conns[person]))
	maps.Copy(out, h.conns[person])
	return out
}

// devices copies every live (deviceID→conn) across all persons — device ids are
// globally unique. Used by the revocation reconcile.
func (h *hub) devices() map[string]*wsConn {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := map[string]*wsConn{}
	for _, byDevice := range h.conns {
		maps.Copy(out, byDevice)
	}
	return out
}

// handleWS upgrades a daemon connection after authenticating its device
// credential (a distinct long-lived token, T-06), re-checking the device is live
// (ActiveDeviceByID — the revocation kill switch, arch §4.3), and confirming the
// device belongs to the credential's person. It registers the socket, starts a
// single writer goroutine that pushes inbox mail, then loops reading acks —
// re-checking revocation on every frame.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	tok, ok := bearerToken(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	person, device, err := s.issuer.VerifyDeviceCredential(tok, time.Now().UTC())
	if err != nil {
		s.log.Warn("ws: device credential rejected", "err", err) // reason logged, never returned (T-17/T-18)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	dev, err := s.store.ActiveDeviceByID(device)
	if err != nil {
		s.log.Warn("ws: device not active", "device_id", device) // revoked/unknown — no oracle to caller
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if dev.PersonID != person {
		// Defense in depth: the credential's person must own the device. Today
		// MintDeviceCredential only ever pairs a device with its own person, so
		// this can't trigger — but the identity boundary asserts it locally
		// rather than trusting a cross-file invariant.
		s.log.Warn("ws: credential/device person mismatch", "device_id", device)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.log.Warn("ws: accept failed", "err", err) // Accept already wrote the response
		return
	}
	c.SetReadLimit(maxWSFrame)
	wc := &wsConn{conn: c, wake: make(chan struct{}, 1)}
	s.hub.add(person, device, wc)
	s.log.Info("ws: connected", "person_id", person, "device_id", device)

	connCtx, cancel := context.WithCancel(r.Context())
	defer func() {
		cancel() // stop the writer goroutine
		s.hub.remove(person, device, wc)
		_ = c.CloseNow() // best-effort on teardown
		s.log.Info("ws: disconnected", "person_id", person, "device_id", device)
	}()

	go s.writerLoop(connCtx, device, wc)
	s.wake(wc) // resend anything unacked on (re)connect

	for {
		_, data, err := c.Read(connCtx)
		if err != nil {
			return
		}
		// Re-check revocation on every inbound frame: a device revoked
		// mid-session (out of band) is severed here too, not only by push/reconcile.
		if _, err := s.store.ActiveDeviceByID(device); err != nil {
			s.log.Warn("ws: device revoked mid-session", "device_id", device)
			return
		}
		var f wsFrame
		if err := json.Unmarshal(data, &f); err != nil {
			s.log.Warn("ws: bad frame", "err", err)
			continue
		}
		switch f.Type {
		case "ack":
			if f.ID != "" {
				if err := s.store.Ack(f.ID, device, time.Now().UTC()); err != nil {
					s.log.Warn("ws: ack", "err", err, "message_id", f.ID)
				}
			}
		default:
			s.log.Warn("ws: unknown frame type", "type", f.Type)
		}
	}
}

// writerLoop is the single writer for one connection: it drains the inbox each
// time it is woken, until the connection context is cancelled. One goroutine per
// connection (never per push), so a stalled socket can't stack goroutines.
func (s *Server) writerLoop(ctx context.Context, device string, c *wsConn) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.wake:
			s.drainInbox(ctx, device, c)
		}
	}
}

// wake signals c's writer to drain, coalescing: if a drain is already pending the
// signal is dropped (that pending drain will read the newly-arrived mail too).
func (s *Server) wake(c *wsConn) {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// drainInbox sends a device's unacked inbox to its socket, marking each
// delivered. It first re-checks the device is live (severing a revoked device —
// the push-side kill switch) and bounds each write so a stalled socket fails
// fast and is closed rather than leaking a blocked goroutine.
func (s *Server) drainInbox(ctx context.Context, device string, c *wsConn) {
	if _, err := s.store.ActiveDeviceByID(device); err != nil {
		s.log.Warn("ws: device revoked, closing socket", "device_id", device)
		_ = c.conn.CloseNow() // read loop exits and deregisters
		return
	}
	items, err := s.store.Inbox(device)
	if err != nil {
		s.log.Error("ws: read inbox", "err", err, "device_id", device)
		return
	}
	for _, it := range items {
		wctx, wcancel := context.WithTimeout(ctx, writeTimeout)
		werr := c.writeFrame(wctx, wsFrame{
			Type: "message", ID: it.ID, ThreadID: it.ThreadID,
			SenderID: it.SenderID, Envelope: it.Envelope,
		})
		wcancel()
		if werr != nil {
			s.log.Warn("ws: push write failed, closing socket", "err", werr, "device_id", device)
			_ = c.conn.CloseNow() // dead/stalled socket → sever
			return
		}
		if err := s.store.MarkDelivered(it.ID, device, time.Now().UTC()); err != nil {
			s.log.Error("ws: mark delivered", "err", err, "message_id", it.ID)
		}
	}
}

// Notify wakes every connected device of a person to push pending mail — the
// delivery push after a message is ingested for them (satisfies mcp.Notifier).
// Non-blocking: it only signals; the per-connection writer does the work, so a
// missing or slow socket never blocks the caller and never spawns work.
func (s *Server) Notify(personID string) {
	for _, c := range s.hub.snapshot(personID) {
		s.wake(c)
	}
}

// reconcileRevocations periodically severs sockets whose device is no longer
// active — bounding how long a revoked (or out-of-process-revoked) device's idle
// socket survives, until ctx is cancelled.
func (s *Server) reconcileRevocations(ctx context.Context) {
	t := time.NewTicker(revokeCheckInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.severRevoked()
		}
	}
}

// severRevoked closes every live socket whose device is no longer active. It
// catches idle sockets and out-of-process revokes (the `device revoke` CLI
// writes the DB directly, so the relay never sees the call).
func (s *Server) severRevoked() {
	for device, c := range s.hub.devices() {
		if _, err := s.store.ActiveDeviceByID(device); err != nil {
			s.log.Warn("ws: severing revoked device", "device_id", device)
			_ = c.conn.CloseNow() // read loop exits and deregisters
		}
	}
}

// bearerToken extracts a bearer token from the Authorization header.
func bearerToken(r *http.Request) (string, bool) {
	const p = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(p) || !strings.EqualFold(h[:len(p)], p) {
		return "", false
	}
	return h[len(p):], true
}
