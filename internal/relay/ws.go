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

// maxWSFrame bounds an inbound /ws frame (acks are tiny; sized ahead for the
// outbound-envelope submit in subtask 4).
const maxWSFrame = 128 << 10

// wsFrame is the /ws wire form (JSON text). Server→daemon: type "message" with
// the delivery fields. Daemon→server: type "ack" with the message id.
type wsFrame struct {
	Type     string          `json:"type"`
	ID       string          `json:"id,omitempty"`
	ThreadID string          `json:"thread_id,omitempty"`
	SenderID string          `json:"sender_id,omitempty"`
	Envelope json.RawMessage `json:"envelope,omitempty"`
}

// wsConn serializes writes to one socket — coder/websocket allows only one write
// in progress at a time, and the connect-drain and live pushes race otherwise.
type wsConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func (c *wsConn) writeFrame(ctx context.Context, f wsFrame) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
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

// snapshot copies a person's device→conn map so callers can push without holding
// the lock during I/O.
func (h *hub) snapshot(person string) map[string]*wsConn {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]*wsConn, len(h.conns[person]))
	maps.Copy(out, h.conns[person])
	return out
}

// handleWS upgrades a daemon connection after authenticating its device
// credential (a distinct long-lived token, T-06) and re-checking the device is
// live (ActiveDeviceByID — the revocation kill switch, arch §4.3). It registers
// the socket, resends any unacked inbox, then loops reading acks. Outbound
// submit (subtask 4) extends the read loop.
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
	if _, err := s.store.ActiveDeviceByID(device); err != nil {
		s.log.Warn("ws: device not active", "device_id", device) // revoked/unknown — no oracle to caller
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.log.Warn("ws: accept failed", "err", err) // Accept already wrote the response
		return
	}
	c.SetReadLimit(maxWSFrame)
	wc := &wsConn{conn: c}
	s.hub.add(person, device, wc)
	s.log.Info("ws: connected", "person_id", person, "device_id", device)
	defer func() {
		s.hub.remove(person, device, wc)
		_ = c.CloseNow() // best-effort on teardown
		s.log.Info("ws: disconnected", "person_id", person, "device_id", device)
	}()

	ctx := r.Context()
	s.pushInbox(ctx, device, wc) // resend anything unacked on (re)connect

	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var f wsFrame
		if err := json.Unmarshal(data, &f); err != nil {
			s.log.Warn("ws: bad frame", "err", err)
			continue
		}
		if f.Type == "ack" && f.ID != "" {
			if err := s.store.Ack(f.ID, device, time.Now().UTC()); err != nil {
				s.log.Warn("ws: ack", "err", err, "message_id", f.ID)
			}
		}
	}
}

// pushInbox drains a device's unacked inbox to its socket, marking each
// delivered. Best-effort: a write error ends the drain (the socket is dying; the
// durable inbox re-sends on the next connect).
//
// ponytail: re-sends all unacked (Inbox filters on acked_at, not delivered), so
// a live notify can resend an already-delivered-but-unacked message. Duplicates
// are safe (idempotent ack, daemon dedups on id) and the backlog is ~0 under
// prompt acks. Add an undelivered-only read if dup churn ever matters.
func (s *Server) pushInbox(ctx context.Context, device string, c *wsConn) {
	items, err := s.store.Inbox(device)
	if err != nil {
		s.log.Error("ws: read inbox", "err", err, "device_id", device)
		return
	}
	for _, it := range items {
		if err := c.writeFrame(ctx, wsFrame{
			Type: "message", ID: it.ID, ThreadID: it.ThreadID,
			SenderID: it.SenderID, Envelope: it.Envelope,
		}); err != nil {
			return
		}
		if err := s.store.MarkDelivered(it.ID, device, time.Now().UTC()); err != nil {
			s.log.Error("ws: mark delivered", "err", err, "message_id", it.ID)
		}
	}
}

// Notify pushes pending mail to all of a person's connected devices — the
// delivery push after a message is ingested for them (satisfies mcp.Notifier).
// Fire-and-forget: a missing or slow socket never blocks the caller; the durable
// inbox is the backstop.
func (s *Server) Notify(personID string) {
	for device, c := range s.hub.snapshot(personID) {
		go s.pushInbox(context.Background(), device, c)
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
