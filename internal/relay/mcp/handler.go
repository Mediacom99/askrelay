package mcp

import (
	"log/slog"
	"net/http"

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
}

// NewHandler builds the MCP surface. version labels the server in the MCP
// Implementation (passed in rather than imported to avoid a cycle with the
// relay package that mounts this).
func NewHandler(st *store.Store, log *slog.Logger, version string) *Handler {
	return &Handler{store: st, log: log, version: version}
}

// HTTPHandler returns the stateless Streamable-HTTP handler for POST /mcp
// (T-07: stateless, protocol 2025-11-25). go-sdk calls getServer per request;
// we read the person the bearer middleware authenticated and build a server
// whose tools are scoped to them.
func (h *Handler) HTTPHandler() http.Handler {
	getServer := func(r *http.Request) *sdkmcp.Server {
		person, clientType, ok := oauth.PersonFromContext(r.Context())
		if !ok {
			// Unreachable behind the bearer middleware; a tool-less server
			// fails any call cleanly rather than acting unauthenticated.
			return h.serverFor("", "")
		}
		return h.serverFor(person, clientType)
	}
	return sdkmcp.NewStreamableHTTPHandler(getServer, &sdkmcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
}

// serverFor builds a per-request server with every tool bound to person.
// Later subtasks register the remaining tools.
func (h *Handler) serverFor(person, clientType string) *sdkmcp.Server {
	_ = clientType // used by wait_for_activity (profile caps)
	s := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "askrelay", Version: h.version}, nil)
	h.addCheckInbox(s, person)
	return s
}
