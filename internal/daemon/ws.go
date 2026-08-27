package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/Mediacom99/askrelay/internal/envelope"
)

const (
	// maxFrame bounds one server→daemon frame, mirroring the relay's own cap.
	maxFrame = 128 << 10
	// Reconnect bounds. Jittered so a relay restart doesn't bring every daemon
	// back in lockstep.
	backoffMin = time.Second
	backoffMax = 30 * time.Second
	// writeTimeout bounds one reply write, so a stalled socket fails instead of
	// blocking the read loop forever.
	writeTimeout = 10 * time.Second
)

// ErrUnauthorized means the relay refused our device credential: revoked
// device, or a credential that no longer matches. Retrying cannot fix either.
var ErrUnauthorized = errors.New("daemon: relay rejected the device credential — run `askrelay enroll` again")

// frame mirrors the relay's /ws wire form (internal/relay/ws.go), declaring only
// the fields the daemon reads. Deliberately duplicated rather than shared: this
// is a wire contract between two processes, and sharing the struct would drag
// the relay's unexported internals into the daemon's surface.
type frame struct {
	Type     string          `json:"type"`
	ID       string          `json:"id,omitempty"`
	ThreadID string          `json:"thread_id,omitempty"`
	SenderID string          `json:"sender_id,omitempty"`
	Envelope json.RawMessage `json:"envelope,omitempty"`
	Sig      []byte          `json:"sig,omitempty"`
}

// listen holds a session to the relay open until ctx is cancelled, reconnecting
// with jittered exponential backoff. A dropped socket loses only a nudge — the
// relay's inbox is durable and re-pushes on reconnect (WP-08).
func (d *Daemon) listen(ctx context.Context) {
	backoff := backoffMin
	for ctx.Err() == nil {
		err := d.session(ctx)
		switch {
		case ctx.Err() != nil:
			return
		case errors.Is(err, ErrUnauthorized):
			d.log.Error("ws: unauthorized, stopping", "err", err)
			return
		}
		d.log.Warn("ws: session ended", "err", err, "retry_in", backoff.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff + rand.N(backoff/2)):
		}
		if backoff *= 2; backoff > backoffMax {
			backoff = backoffMax
		}
	}
}

// session runs one connection to exhaustion and returns why it ended.
//
// It NEVER sends an ack. An ack starts the relay's ack-grace deletion timer
// (T-09), and this daemon only knows a message arrived — not that its human
// read it. Acking belongs to a consumer that actually showed it to someone (the
// ST-7 stdio server's check_inbox). The cost of not acking is that the relay
// re-pushes every unacked message on each reconnect, which d.seen filters.
func (d *Daemon) session(ctx context.Context) error {
	c, resp, err := websocket.Dial(ctx, d.cfg.RelayURL+"/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + d.cfg.DeviceCredential}},
	})
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return ErrUnauthorized
		}
		return fmt.Errorf("daemon: dial relay: %w", err)
	}
	defer func() { _ = c.CloseNow() }() // best-effort teardown
	c.SetReadLimit(maxFrame)
	d.log.Info("ws: connected", "relay", d.cfg.RelayURL, "device", d.cfg.DeviceID)

	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return fmt.Errorf("daemon: ws read: %w", err)
		}
		var f frame
		if err := json.Unmarshal(data, &f); err != nil {
			d.log.Warn("ws: unparseable frame") // never log the frame itself (T-18)
			continue
		}
		switch {
		case f.Type == "sign_request" && f.ID != "":
			// Same goroutine writes the reply: the read loop is this session's
			// only writer, so no write mutex is needed.
			if err := d.answerSignRequest(ctx, c, f); err != nil {
				return fmt.Errorf("daemon: answer sign request: %w", err)
			}
		case f.Type == "message" && f.ID != "":
			if d.seen[f.ID] {
				continue // re-pushed on reconnect precisely because we never ack
			}
			d.seen[f.ID] = true
			d.log.Info("mail", "message_id", f.ID, "thread", f.ThreadID)
			d.notify()
		default:
			d.log.Warn("ws: unexpected frame", "type", f.Type)
		}
	}
}

// osNotify raises a desktop notification. The text is a FIXED string: it never
// carries message content, both because content must not escape the approval
// path (T-18) and because interpolating remote text into an osascript program
// would be a command-injection surface.
func (d *Daemon) osNotify() {
	const title, body = "askrelay", "A message is waiting for your approval"
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("osascript", "-e",
			`display notification "`+body+`" with title "`+title+`"`)
	case "linux":
		cmd = exec.Command("notify-send", title, body)
	default:
		return // the log line above is the notification on this platform
	}
	if err := cmd.Run(); err != nil {
		d.log.Warn("notify failed", "err", err)
	}
}

// answerSignRequest signs a release the relay is holding — but ONLY if this
// machine forwarded that exact body (T-21). A refusal is the correct answer for
// anything else, including a draft created straight against the relay, and it is
// logged at Warn because a relay asking us to sign text we never wrote is a
// security event rather than a nuisance.
//
// Three things must hold: the envelope is from our person, it names OUR device
// (so the signature is bound to the key making it), and its body matches what we
// forwarded. The relay verifies the result independently, so a bug here degrades
// the message to relay-attested rather than forging anything.
func (d *Daemon) answerSignRequest(ctx context.Context, c *websocket.Conn, f frame) error {
	refuse := func(reason string) error {
		d.log.Warn("ws: refusing to sign", "reason", reason, "message_id", f.ID)
		return d.writeFrame(ctx, c, frame{Type: "sign_refused", ID: f.ID})
	}
	var e envelope.Envelope
	if err := json.Unmarshal(f.Envelope, &e); err != nil {
		return refuse("unparseable envelope")
	}
	switch {
	case e.ID != f.ID:
		return refuse("envelope id does not match the request")
	case e.From.Person != d.cfg.PersonID:
		return refuse("not our person")
	case e.From.Device != d.cfg.DeviceID:
		return refuse("names another device")
	case !d.forwarded(e.ID, bodyText(e)):
		return refuse("this machine never forwarded that body")
	}
	if err := envelope.Sign(&e, d.key); err != nil {
		return refuse("signing failed: " + err.Error())
	}
	d.log.Info("signed a release", "message_id", e.ID, "thread", e.Thread)
	if err := d.writeFrame(ctx, c, frame{Type: "signature", ID: e.ID, Sig: e.Sig}); err != nil {
		return err
	}
	d.forgetSignable(e.ID)
	return nil
}

// bodyText concatenates the envelope's text parts — what the ledger hashes, so
// the record survives changes to the surrounding envelope structure.
func bodyText(e envelope.Envelope) string {
	var b strings.Builder
	for _, p := range e.Body.Parts {
		if p.Type == "text" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func (d *Daemon) writeFrame(ctx context.Context, c *websocket.Conn, f frame) error {
	blob, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("daemon: marshal frame: %w", err)
	}
	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return c.Write(wctx, websocket.MessageText, blob)
}
