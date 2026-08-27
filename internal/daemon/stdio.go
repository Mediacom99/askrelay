package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Mediacom99/askrelay/internal/redact"
)

// hookTimeout bounds the external redaction hook. Generous for a scanner over
// one message; a hook slower than this blocks the send (fail-closed, T-12).
const hookTimeout = 5 * time.Second

// redactFields names the outbound message-body argument of each tool that has
// one. Nothing else is rewritten: a find_people query is not a message, and
// silently editing it would be surprising rather than protective.
var redactFields = map[string]string{
	"send_message":  "text",
	"approve_reply": "edited_text",
}

// bearerTransport attaches the device credential to every relay request (T-20),
// which is how the daemon authenticates to /mcp without running the AS dance
// against itself.
type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context()) // never mutate the caller's request
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

// relaySession opens an MCP client session to the relay's /mcp as this device.
func (d *Daemon) relaySession(ctx context.Context) (*sdkmcp.ClientSession, error) {
	c := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "askrelay-daemon", Version: d.version}, nil)
	cs, err := c.Connect(ctx, &sdkmcp.StreamableClientTransport{
		Endpoint:   d.cfg.RelayURL + "/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport{token: d.cfg.DeviceCredential}},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("daemon: connect relay mcp: %w", err)
	}
	return cs, nil
}

// ServeStdio runs the local stdio MCP server that an AI client (Claude Code,
// Codex) spawns, proxying every tool call to the relay as this device.
//
// The tool table is FETCHED from the relay rather than redeclared here: the
// WP-07 surface stays the single definition, so a new relay tool needs no daemon
// release, and the descriptions the model reads — which carry the approval
// nudges — cannot drift between the two paths.
//
// This is the path on which redaction (ST-5) and signatures (T-19) mean
// anything: every call flows through here, so the daemon sees the outbound text
// before it leaves the machine, and sees the human's approval and any edit.
func (d *Daemon) ServeStdio(ctx context.Context) error {
	cs, err := d.relaySession(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = cs.Close() }()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("daemon: list relay tools: %w", err)
	}
	s := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "askrelay", Version: d.version}, nil)
	for _, t := range tools.Tools {
		s.AddTool(t, d.proxy(cs, t.Name))
	}
	d.log.Info("stdio mcp server ready", "relay", d.cfg.RelayURL, "tools", len(tools.Tools))
	return s.Run(ctx, &sdkmcp.StdioTransport{})
}

// proxy forwards one tool call to the relay verbatim — raw arguments in, the
// relay's result out untouched, so structuredContent survives (Claude Code reads
// only that, per the WP-07 learnings) — except that an outbound message body is
// redacted first (ST-5), which is the whole reason this path exists.
func (d *Daemon) proxy(cs *sdkmcp.ClientSession, name string) sdkmcp.ToolHandler {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		raw := req.Params.Arguments
		var warning string
		if field, ok := redactFields[name]; ok {
			var err error
			// Fail-closed: on any redaction error nothing is forwarded, so a
			// broken scanner blocks the send instead of leaking past it (T-12).
			if raw, warning, err = d.redactField(ctx, raw, field); err != nil {
				return nil, err
			}
		}
		var args any
		if len(raw) > 0 {
			args = raw
		}
		res, err := cs.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			// The relay's tool errors are already sanitized (T-17); a transport
			// failure is ours. Neither carries message content.
			d.log.Warn("proxy: relay call failed", "tool", name, "err", err)
			return nil, fmt.Errorf("daemon: relay tool %q: %w", name, err)
		}
		if warning != "" {
			// Kinds and counts only — never the matched text (T-18).
			d.log.Warn("redacted outbound text before it left this machine", "tool", name, "kinds", warning)
			res.Content = append([]sdkmcp.Content{&sdkmcp.TextContent{
				Text: "askrelay redacted secrets before sending (" + warning + "). The queued text shown here is exactly what will be sent.",
			}}, res.Content...)
		}
		// Record what we forwarded so a later sign request for it is recognisable
		// (T-21). The POST-redaction text is what the relay will ask us to sign.
		d.noteForwarded(name, raw, res)
		return res, nil
	}
}

// redactField rewrites one string argument through the built-in patterns and, if
// configured, the external hook — returning the patched arguments and a warning
// summarising what was replaced ("" when nothing was).
//
// Arguments are re-encoded ONLY when the text actually changed; every other call
// is forwarded byte-identical, so no unrelated argument can be altered on the
// way through.
func (d *Daemon) redactField(ctx context.Context, raw json.RawMessage, field string) (json.RawMessage, string, error) {
	if len(raw) == 0 {
		return raw, "", nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, "", fmt.Errorf("daemon: decode tool arguments: %w", err)
	}
	text, ok := m[field].(string)
	if !ok || text == "" {
		return raw, "", nil // field absent (approve_reply without an edit) or empty
	}

	res := redact.Redact(text)
	out := res.Text
	if d.cfg.RedactHook != "" {
		hctx, cancel := context.WithTimeout(ctx, hookTimeout)
		defer cancel()
		hooked, err := redact.Hook(hctx, d.cfg.RedactHook, out)
		if err != nil {
			return nil, "", fmt.Errorf("daemon: redaction hook blocked this send: %w", err)
		}
		out = hooked
	}
	if out == text {
		return raw, "", nil
	}
	m[field] = out
	patched, err := json.Marshal(m)
	if err != nil {
		return nil, "", fmt.Errorf("daemon: re-encode tool arguments: %w", err)
	}
	warning := res.Summary()
	if warning == "" {
		warning = "external hook rewrote the text"
	}
	return patched, warning, nil
}

// noteForwarded records the body this machine just forwarded, keyed by the draft
// it became, so the sign handler can recognise it later (T-21).
//
// send_message mints the draft, so its id has to be read back out of the relay's
// structured result; approve_reply's edit names the draft in its own arguments.
func (d *Daemon) noteForwarded(tool string, raw json.RawMessage, res *sdkmcp.CallToolResult) {
	var args struct {
		ID         string  `json:"id"`
		Text       string  `json:"text"`
		EditedText *string `json:"edited_text"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return
	}
	switch tool {
	case "send_message":
		blob, err := json.Marshal(res.StructuredContent)
		if err != nil {
			return
		}
		var out struct {
			DraftID string `json:"draft_id"`
		}
		if err := json.Unmarshal(blob, &out); err != nil || out.DraftID == "" {
			return
		}
		d.recordSignable(out.DraftID, args.Text)
	case "approve_reply":
		if args.EditedText != nil && args.ID != "" {
			// The human edited at review time; the edited body is what will be
			// signed, so it replaces the record made at send time.
			d.recordSignable(args.ID, *args.EditedText)
		}
	}
}
