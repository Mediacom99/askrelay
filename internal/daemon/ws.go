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
	"time"

	"github.com/coder/websocket"
)

const (
	// maxFrame bounds one server→daemon frame, mirroring the relay's own cap.
	maxFrame = 128 << 10
	// Reconnect bounds. Jittered so a relay restart doesn't bring every daemon
	// back in lockstep.
	backoffMin = time.Second
	backoffMax = 30 * time.Second
)

// ErrUnauthorized means the relay refused our device credential: revoked
// device, or a credential that no longer matches. Retrying cannot fix either.
var ErrUnauthorized = errors.New("daemon: relay rejected the device credential — run `askrelay enroll` again")

// frame mirrors the relay's /ws wire form (internal/relay/ws.go), declaring only
// the fields the daemon reads. Deliberately duplicated rather than shared: this
// is a wire contract between two processes, and sharing the struct would drag
// the relay's unexported internals into the daemon's surface.
type frame struct {
	Type     string `json:"type"`
	ID       string `json:"id,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
	SenderID string `json:"sender_id,omitempty"`
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
		if f.Type != "message" || f.ID == "" {
			d.log.Warn("ws: unexpected frame", "type", f.Type)
			continue
		}
		if d.seen[f.ID] {
			continue // re-pushed on reconnect precisely because we never ack
		}
		d.seen[f.ID] = true
		d.log.Info("mail", "message_id", f.ID, "thread", f.ThreadID)
		d.notify()
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
