package mcp

import (
	"context"
	"errors"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Mediacom99/askrelay/internal/envelope"
)

// errInternal is the sanitized error surfaced to MCP clients; the real cause is
// logged server-side (T-17: remote-facing errors are sanitized).
var errInternal = errors.New("internal error")

type checkInboxInput struct{}

type inboxEntry struct {
	ID       string `json:"id"`
	ThreadID string `json:"thread_id"`
	Kind     string `json:"kind"` // "message" (awaiting my verdict) | "draft" (awaiting my review)
}

type checkInboxOutput struct {
	ToApprove []inboxEntry `json:"to_approve"`
	ToReview  []inboxEntry `json:"to_review"`
}

// addCheckInbox registers check_inbox, bound to person. Inbound messages are
// spotlighted (untrusted data); the person's own drafts are locally-authored
// outbound text and are listed structurally, not spotlighted.
func (h *Handler) addCheckInbox(s *sdkmcp.Server, person string) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "check_inbox",
		Description: "List everything awaiting you: inbound messages to approve and your own drafts to review.",
		Annotations: &sdkmcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, _ checkInboxInput) (*sdkmcp.CallToolResult, checkInboxOutput, error) {
		inbound, err := h.store.InboundAwaiting(person)
		if err != nil {
			h.log.Error("check_inbox: inbound query", "err", err)
			return nil, checkInboxOutput{}, errInternal
		}
		drafts, err := h.store.DraftsAwaitingReview(person)
		if err != nil {
			h.log.Error("check_inbox: drafts query", "err", err)
			return nil, checkInboxOutput{}, errInternal
		}

		var out checkInboxOutput
		var text strings.Builder
		for _, it := range inbound {
			e, derr := envelope.Decode(it.Envelope)
			if derr != nil {
				// Never surface a corrupt stored blob as content; log and skip.
				h.log.Error("check_inbox: undecodable stored envelope", "msg", it.MessageID, "err", derr)
				continue
			}
			out.ToApprove = append(out.ToApprove, inboxEntry{ID: it.MessageID, ThreadID: it.ThreadID, Kind: "message"})
			text.WriteString(Spotlight(e, it.SenderEmail+" (device verified)"))
			text.WriteString("\n\n")
		}
		for _, d := range drafts {
			out.ToReview = append(out.ToReview, inboxEntry{ID: d.DraftID, ThreadID: d.ThreadID, Kind: "draft"})
		}

		result := &sdkmcp.CallToolResult{}
		if text.Len() > 0 {
			result.Content = []sdkmcp.Content{&sdkmcp.TextContent{Text: strings.TrimRight(text.String(), "\n")}}
		}
		return result, out, nil
	})
}
