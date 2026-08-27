package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type sendArgs struct {
	To   string `json:"to"`
	Text string `json:"text"`
}

type sendOut struct {
	DraftID string `json:"draft_id"`
	State   string `json:"state"`
	Text    string `json:"text"`
}

type findArgs struct {
	Query string `json:"query"`
}

// capture records what the fake relay actually received.
type capture struct {
	mu    sync.Mutex
	auth  string
	send  sendArgs
	find  findArgs
	calls int
}

func (c *capture) snap() (string, sendArgs, findArgs, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.auth, c.send, c.find, c.calls
}

// fakeRelay stands up a real go-sdk MCP server over HTTP with two tools: one
// that carries a message body and one that does not.
func fakeRelay(t *testing.T) (*httptest.Server, *capture) {
	t.Helper()
	rec := &capture{}
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "askrelay", Version: "test"}, nil)
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "send_message", Description: "Draft an ask or a reply."},
		func(_ context.Context, _ *sdkmcp.CallToolRequest, in sendArgs) (*sdkmcp.CallToolResult, sendOut, error) {
			rec.mu.Lock()
			rec.send, rec.calls = in, rec.calls+1
			rec.mu.Unlock()
			return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{}},
				sendOut{DraftID: "draft-1", State: "pending_review", Text: in.Text}, nil
		})
	sdkmcp.AddTool(srv, &sdkmcp.Tool{Name: "find_people", Description: "Look up people on the roster."},
		func(_ context.Context, _ *sdkmcp.CallToolRequest, in findArgs) (*sdkmcp.CallToolResult, findArgs, error) {
			rec.mu.Lock()
			rec.find, rec.calls = in, rec.calls+1
			rec.mu.Unlock()
			return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{}}, in, nil
		})
	h := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return srv }, nil)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.auth = r.Header.Get("Authorization")
		rec.mu.Unlock()
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts, rec
}

func testStdioDaemon(t *testing.T, relayURL, hook string) (*Daemon, *sdkmcp.ClientSession) {
	t.Helper()
	d := &Daemon{
		cfg: Config{RelayURL: relayURL, PersonID: "p1", DeviceID: "d1",
			DeviceCredential: "cred", RedactHook: hook},
		log:     slog.New(slog.DiscardHandler),
		version: "test",
	}
	cs, err := d.relaySession(context.Background())
	if err != nil {
		t.Fatalf("relaySession: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return d, cs
}

func call(t *testing.T, d *Daemon, cs *sdkmcp.ClientSession, tool, args string) (*sdkmcp.CallToolResult, error) {
	t.Helper()
	return d.proxy(cs, tool)(context.Background(), &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: tool, Arguments: json.RawMessage(args)},
	})
}

// The proxy's contract: the relay is the single definition of the tool table,
// calls arrive there authenticated as this device (T-20), and the relay's
// structuredContent comes back untouched — Claude Code reads only that.
func TestProxyForwardsVerbatimAsTheDevice(t *testing.T) {
	ts, rec := fakeRelay(t)
	d, cs := testStdioDaemon(t, ts.URL, "")

	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) != 2 {
		t.Fatalf("discovered %d tools, want 2", len(tools.Tools))
	}
	for _, tool := range tools.Tools {
		if tool.Name == "send_message" && tool.Description != "Draft an ask or a reply." {
			t.Errorf("description = %q, want the relay's own text", tool.Description)
		}
	}

	res, err := call(t, d, cs, "send_message", `{"to":"bob@example.com","text":"is staging green?"}`)
	if err != nil {
		t.Fatalf("proxy call: %v", err)
	}
	auth, send, _, _ := rec.snap()
	if auth != "Bearer cred" {
		t.Errorf("relay saw Authorization %q, want the device credential", auth)
	}
	if send != (sendArgs{To: "bob@example.com", Text: "is staging green?"}) {
		t.Errorf("relay saw args %+v, want them forwarded verbatim", send)
	}
	structured, _ := json.Marshal(res.StructuredContent)
	for _, want := range []string{"draft-1", "pending_review", "is staging green?"} {
		if !strings.Contains(string(structured), want) {
			t.Errorf("structuredContent %s lost %q", structured, want)
		}
	}
	if len(res.Content) != 0 {
		t.Errorf("unredacted send got a warning block: %+v", res.Content)
	}
}

// ST-5: a secret must be replaced by a marker BEFORE the relay sees it — that is
// the entire difference between this path and connecting straight to the relay.
func TestProxyRedactsBeforeTheRelaySeesIt(t *testing.T) {
	ts, rec := fakeRelay(t)
	d, cs := testStdioDaemon(t, ts.URL, "")

	const secret = "AKIAIOSFODNN7EXAMPLE"
	res, err := call(t, d, cs, "send_message",
		`{"to":"bob@example.com","text":"deploy with `+secret+` please"}`)
	if err != nil {
		t.Fatalf("proxy call: %v", err)
	}

	_, send, _, _ := rec.snap()
	if strings.Contains(send.Text, secret) {
		t.Fatalf("the relay received the raw secret: %q", send.Text)
	}
	if !strings.Contains(send.Text, "⟦redacted:aws-access-key⟧") {
		t.Errorf("relay text = %q, want a visible redaction marker", send.Text)
	}
	if send.To != "bob@example.com" {
		t.Errorf("recipient was altered: %q", send.To)
	}
	// The human is told, and the relay's echo shows exactly what will be sent.
	if len(res.Content) == 0 {
		t.Fatal("no redaction warning surfaced to the human")
	}
	text, ok := res.Content[0].(*sdkmcp.TextContent)
	if !ok || !strings.Contains(text.Text, "aws-access-key") {
		t.Errorf("warning block = %+v, want the redacted kinds", res.Content[0])
	}
}

// Only message bodies are rewritten. A roster query is not a message, and the
// proxy must not edit arguments it has no business touching.
func TestProxyLeavesNonMessageToolsAlone(t *testing.T) {
	ts, rec := fakeRelay(t)
	d, cs := testStdioDaemon(t, ts.URL, "")

	const query = "AKIAIOSFODNN7EXAMPLE"
	if _, err := call(t, d, cs, "find_people", `{"query":"`+query+`"}`); err != nil {
		t.Fatalf("proxy call: %v", err)
	}
	if _, _, find, _ := rec.snap(); find.Query != query {
		t.Errorf("find_people query = %q, want it untouched", find.Query)
	}
}

// T-12 fail-closed: a broken hook blocks the send. The relay must never be
// reached — the point is that nothing leaks past a scanner that didn't run.
func TestProxyFailsClosedOnBrokenHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell hook fixture is POSIX")
	}
	hook := filepath.Join(t.TempDir(), "hook.sh")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatalf("write hook: %v", err)
	}
	ts, rec := fakeRelay(t)
	d, cs := testStdioDaemon(t, ts.URL, hook)

	_, err := call(t, d, cs, "send_message", `{"to":"bob@example.com","text":"anything"}`)
	if err == nil {
		t.Fatal("broken hook did not block the send")
	}
	if !strings.Contains(err.Error(), "blocked this send") {
		t.Errorf("error = %v, want it to name the block", err)
	}
	if _, _, _, calls := rec.snap(); calls != 0 {
		t.Errorf("relay was called %d times despite a failed hook", calls)
	}
}
