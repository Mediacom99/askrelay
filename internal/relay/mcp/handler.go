package mcp

import (
	"log/slog"
	"net/http"
	"sync"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Mediacom99/askrelay/internal/relay/oauth"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// Handler serves the askrelay MCP tool surface (arch §5) over stateless
// Streamable HTTP. It mounts behind the bearer middleware, so every request
// carries an authenticated person; the tools are bound to that person per
// request. No MCP session state is kept (D-06, T-07: stateless).
type Handler struct {
	store   *store.Store
	log     *slog.Logger
	version string

	mu      sync.Mutex     // guards waiting
	waiting map[string]int // concurrent wait_for_activity calls per person
}

// NewHandler builds the MCP surface. version labels the server in the MCP
// Implementation (passed in rather than imported to avoid a cycle with the
// relay package that mounts this).
func NewHandler(st *store.Store, log *slog.Logger, version string) *Handler {
	return &Handler{store: st, log: log, version: version, waiting: map[string]int{}}
}

// HTTPHandler returns the stateless Streamable-HTTP handler for POST /mcp
// (T-07: stateless, protocol 2025-11-25). go-sdk calls getServer per request;
// we read the person the bearer middleware authenticated and build a server
// whose tools are scoped to them.
func (h *Handler) HTTPHandler() http.Handler {
	getServer := func(r *http.Request) *sdkmcp.Server {
		person, clientType, ok := oauth.PersonFromContext(r.Context())
		if !ok {
			// Unreachable behind the bearer middleware. Build a genuinely
			// tool-less server so, even if reached, no tool can run.
			return sdkmcp.NewServer(&sdkmcp.Implementation{Name: "askrelay", Version: h.version}, nil)
		}
		return h.serverFor(person, clientType)
	}
	return sdkmcp.NewStreamableHTTPHandler(getServer, &sdkmcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
}

// serverFor builds a per-request server with every tool bound to person.
func (h *Handler) serverFor(person, clientType string) *sdkmcp.Server {
	s := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "askrelay", Version: h.version}, nil)
	h.addCheckInbox(s, person)
	h.addGetThread(s, person)
	h.addWaitForActivity(s, person, clientType)
	h.addInboundVerdicts(s, person)
	h.addSetThreadGrant(s, person)
	h.addSendMessage(s, person)
	h.addOutboundVerdicts(s, person)
	return s
}

// emptyResult / textResult return a NON-nil Content so go-sdk does not auto-fill
// Content with a duplicate JSON copy of the structured output (chatgpt-field.md:
// send structuredContent without a duplicate serialized text block).
func emptyResult() *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{}}
}

func textResult(text string) *sdkmcp.CallToolResult {
	if text == "" {
		return emptyResult()
	}
	return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}}}
}

// waitAcquire bounds concurrent wait_for_activity calls per person; false means
// the cap is reached. release decrements.
func (h *Handler) waitAcquire(person string) bool {
	const maxPerPerson = 2
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.waiting[person] >= maxPerPerson {
		return false
	}
	h.waiting[person]++
	return true
}

func (h *Handler) waitRelease(person string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.waiting[person] > 0 {
		h.waiting[person]--
	}
	if h.waiting[person] == 0 {
		delete(h.waiting, person)
	}
}
