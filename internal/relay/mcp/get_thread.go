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
			out.Messages = append(out.Messages, threadMsg{ID: m.MessageID, From: m.SenderEmail})
			// view.State is the authoritative thread state (never e.State).
			text.WriteString(Spotlight(e, m.SenderEmail+" (device verified)", view.State))
			text.WriteString("\n\n")
		}
		for _, d := range view.Drafts {
			out.Drafts = append(out.Drafts, inboxEntry{ID: d.DraftID, ThreadID: d.ThreadID, Kind: "draft"})
		}
		return textResult(strings.TrimRight(text.String(), "\n")), out, nil
	})
}
