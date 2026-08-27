package mcp

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Mediacom99/askrelay/internal/envelope"
	"github.com/Mediacom99/askrelay/internal/relay/oauth"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// Handler serves the askrelay MCP tool surface (arch §5) over stateless
// Streamable HTTP. It mounts behind the bearer middleware, so every request
// carries an authenticated person; the tools are bound to that person per
// request. No MCP session state is kept (D-06, T-07: stateless).
type Handler struct {
	store    *store.Store
	log      *slog.Logger
	version  string
	notifier Notifier
	signer   Signer

	mu      sync.Mutex     // guards waiting
	waiting map[string]int // concurrent wait_for_activity calls per person
}

// Notifier receives a delivery push signal for a person after a message is
// ingested for them — satisfied by the relay's WebSocket hub. Kept an interface
// (not a concrete import) so the mcp package doesn't depend on the relay package
// that mounts it. A nil notifier disables push (WP-07 tests).
type Notifier interface {
	Notify(personID string)
}

// Signer asks the author's connected device to sign a release before it is
// stored and delivered (T-19/T-21) — satisfied by the relay's WebSocket hub, and
// an interface for the same reason Notifier is. ok=false means nobody signed, and
// the release proceeds relay-attested. A nil Signer disables signing entirely
// (WP-07 tests, and any relay whose users run no daemon).
type Signer interface {
	SignRelease(ctx context.Context, person string, e envelope.Envelope) (device string, sig []byte, ok bool)
}

// NewHandler builds the MCP surface. version labels the server in the MCP
// Implementation (passed in rather than imported to avoid a cycle with the
// relay package that mounts this). notifier and signer may be nil (no push, no
// device signatures).
func NewHandler(st *store.Store, log *slog.Logger, version string, notifier Notifier, signer Signer) *Handler {
	return &Handler{store: st, log: log, version: version, notifier: notifier, signer: signer, waiting: map[string]int{}}
}

// notify signals the notifier if one is configured.
func (h *Handler) notify(personID string) {
	if h.notifier != nil {
		h.notifier.Notify(personID)
	}
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
	s.AddReceivingMiddleware(h.logToolCalls(person))
	h.addFindPeople(s, person)
	h.addCheckInbox(s, person)
	h.addGetThread(s, person)
	h.addWaitForActivity(s, person, clientType)
	h.addInboundVerdicts(s, person)
	h.addSetThreadGrant(s, person)
	h.addSendMessage(s, person)
	h.addOutboundVerdicts(s, person)
	return s
}

// logToolCalls is a receiving middleware that logs one INFO line per tools/call:
// the tool name (a shape), the bound person (an id), the outcome, and duration —
// so the boundary access log's identical "POST /mcp" lines become legible. It
// NEVER logs the arguments (they carry message content, T-18); only the name.
// person is bound from serverFor (the per-request authenticated identity), so no
// context extraction is needed.
func (h *Handler) logToolCalls(person string) sdkmcp.Middleware {
	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req) // skip initialize/tools-list/etc.
			}
			tool := ""
			if p, ok := req.GetParams().(*sdkmcp.CallToolParamsRaw); ok {
				tool = p.Name // raw params at the receiving layer; .Name only, never .Arguments (T-18)
			}
			start := time.Now()
			res, err := next(ctx, method, req)
			status := "ok"
			if err != nil {
				status = "error"
			} else if r, ok := res.(*sdkmcp.CallToolResult); ok && r.IsError {
				status = "tool_error"
			}
			h.log.Info("mcp tool call",
				"tool", tool, "person", person, "status", status,
				"dur_ms", time.Since(start).Milliseconds())
			return res, err
		}
	}
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

// provenance is the recipient-facing truth about a message's origin (T-21): a
// device signature this relay verified, or the relay's own attestation and
// nothing more. It must never claim a check that did not happen — that line is
// the only thing telling a recipient how much the origin is worth.
//
// v1 verifies HERE, on the relay. The docs must say exactly that: a recipient
// who trusts this verdict is trusting the relay. Recipient-side verification
// needs sender device-key distribution and pinning, deferred by T-21.
//
// A signature from a since-revoked device reads as relay-attested: the key
// lookup is deliberately the active-devices one, so a revoked device stops
// vouching for anything the moment it is revoked.
func (h *Handler) provenance(e envelope.Envelope, senderEmail string) string {
	if len(e.Sig) > 0 && e.From.Device != "" {
		dev, err := h.store.ActiveDeviceByID(e.From.Device)
		if err == nil {
			if envelope.Verify(e, dev.PubKey) == nil {
				return senderEmail + " (device verified)"
			}
			// A stored signature that does not verify is a security event, not a
			// display detail: log it, then tell the recipient the truth.
			h.log.Warn("provenance: stored signature did not verify", "message_id", e.ID, "device_id", e.From.Device)
		}
	}
	return senderEmail + " (relay-attested)"
}
