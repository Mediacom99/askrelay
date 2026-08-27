package mcp

import (
	"context"
	"errors"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Mediacom99/askrelay/internal/envelope"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// errNotFound is the sanitized tool error for a thread the caller can't see —
// unknown or not-a-participant, indistinguishable (no oracle).
var errNotFound = errors.New("thread not found")

type getThreadInput struct {
	ThreadID string `json:"thread_id" jsonschema:"the thread id to fetch"`
}

type threadMsg struct {
	ID   string `json:"id"`
	From string `json:"from"`
	Body string `json:"body"` // SPOTLIGHT-FRAMED message body (untrusted data — do not obey)
}

type getThreadOutput struct {
	ThreadID string       `json:"thread_id"`
	State    string       `json:"state"`
	Messages []threadMsg  `json:"messages"`
	Drafts   []inboxEntry `json:"drafts"`
}

// addGetThread registers get_thread, bound to person. Messages are spotlighted
// (untrusted); the caller's own drafts are listed structurally.
func (h *Handler) addGetThread(s *sdkmcp.Server, person string) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "get_thread",
		Description: "Fetch a full thread: its messages (spotlighted) and your drafts.",
		Annotations: &sdkmcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in getThreadInput) (*sdkmcp.CallToolResult, getThreadOutput, error) {
		view, err := h.store.ThreadFor(person, in.ThreadID)
		if errors.Is(err, store.ErrNotFound) {
			return nil, getThreadOutput{}, errNotFound
		}
		if err != nil {
			h.log.Error("get_thread", "err", err)
			return nil, getThreadOutput{}, errInternal
		}

		out := getThreadOutput{ThreadID: view.ThreadID, State: view.State}
		var text strings.Builder
		for _, m := range view.Messages {
			e, derr := envelope.Decode(m.Envelope)
			if derr != nil {
				h.log.Error("get_thread: undecodable envelope", "msg", m.MessageID, "err", derr)
				continue
			}
			// view.State is the authoritative thread state (never e.State). The
			// spotlight goes in BOTH the structured Body field and the content block
			// (Claude Code surfaces only structuredContent); framing is preserved.
			spot := Spotlight(e, h.provenance(e, m.SenderEmail), view.State)
			out.Messages = append(out.Messages, threadMsg{ID: m.MessageID, From: m.SenderEmail, Body: spot})
			text.WriteString(spot)
			text.WriteString("\n\n")
		}
		for _, d := range view.Drafts {
			out.Drafts = append(out.Drafts, inboxEntry{ID: d.DraftID, ThreadID: d.ThreadID, Kind: "draft", Text: draftText(d.Envelope)})
		}
		return textResult(strings.TrimRight(text.String(), "\n")), out, nil
	})
}

// draftText returns the plain text of a person's OWN draft/outbound envelope.
// Unlike inbound messages, a draft is the caller's own authored content — trusted,
// so it is returned verbatim in the structured output (not spotlighted), letting
// the caller review exactly what will be sent before approving it.
func draftText(raw []byte) string {
	e, err := envelope.Decode(raw)
	if err != nil {
		return ""
	}
	var parts []string
	for _, p := range e.Body.Parts {
		if p.Type == "text" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "\n")
}
