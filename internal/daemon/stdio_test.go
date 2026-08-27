package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

// The proxy's contract: the relay is the single definition of the tool table,
// calls arrive there verbatim and authenticated as this device (T-20), and the
// relay's structuredContent comes back untouched — Claude Code reads only that.
func TestProxyForwardsVerbatimAsTheDevice(t *testing.T) {
	var mu sync.Mutex
	var gotAuth string
	var gotArgs sendArgs

	relay := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "askrelay", Version: "test"}, nil)
	sdkmcp.AddTool(relay, &sdkmcp.Tool{Name: "send_message", Description: "Draft an ask or a reply."},
		func(_ context.Context, _ *sdkmcp.CallToolRequest, in sendArgs) (*sdkmcp.CallToolResult, sendOut, error) {
			mu.Lock()
			gotArgs = in
			mu.Unlock()
			return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{}},
				sendOut{DraftID: "draft-1", State: "pending_review", Text: in.Text}, nil
		})

	mcpHandler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return relay }, nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotAuth = r.Header.Get("Authorization")
		mu.Unlock()
		mcpHandler.ServeHTTP(w, r)
	}))
	defer srv.Close()

	d := &Daemon{
		cfg:     Config{RelayURL: srv.URL, PersonID: "p1", DeviceID: "d1", DeviceCredential: "cred"},
		log:     slog.New(slog.DiscardHandler),
		version: "test",
	}
	ctx := context.Background()
	cs, err := d.relaySession(ctx)
	if err != nil {
		t.Fatalf("relaySession: %v", err)
	}
	defer func() { _ = cs.Close() }()

	// The tool table is discovered, not redeclared: descriptions (which carry
	// the approval nudges) must arrive as the relay wrote them.
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "send_message" {
		t.Fatalf("discovered tools = %+v, want exactly send_message", tools.Tools)
	}
	if got := tools.Tools[0].Description; got != "Draft an ask or a reply." {
		t.Errorf("description = %q, want the relay's own text", got)
	}

	res, err := d.proxy(cs, "send_message")(ctx, &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{
			Name:      "send_message",
			Arguments: json.RawMessage(`{"to":"bob@example.com","text":"is staging green?"}`),
		},
	})
	if err != nil {
		t.Fatalf("proxy call: %v", err)
	}

	mu.Lock()
	auth, args := gotAuth, gotArgs
	mu.Unlock()
	if auth != "Bearer cred" {
		t.Errorf("relay saw Authorization %q, want the device credential", auth)
	}
	if args != (sendArgs{To: "bob@example.com", Text: "is staging green?"}) {
		t.Errorf("relay saw args %+v, want them forwarded verbatim", args)
	}
	structured, _ := json.Marshal(res.StructuredContent)
	for _, want := range []string{"draft-1", "pending_review", "is staging green?"} {
		if !strings.Contains(string(structured), want) {
			t.Errorf("structuredContent %s lost %q", structured, want)
		}
	}
}
