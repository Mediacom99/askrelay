package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

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
// only that, per the WP-07 learnings). ST-5 hooks redaction here, on the
// outbound text arguments only.
func (d *Daemon) proxy(cs *sdkmcp.ClientSession, name string) sdkmcp.ToolHandler {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		var args any
		if len(req.Params.Arguments) > 0 {
			args = json.RawMessage(req.Params.Arguments)
		}
		res, err := cs.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			// The relay's tool errors are already sanitized (T-17); a transport
			// failure is ours. Neither carries message content.
			d.log.Warn("proxy: relay call failed", "tool", name, "err", err)
			return nil, fmt.Errorf("daemon: relay tool %q: %w", name, err)
		}
		return res, nil
	}
}
