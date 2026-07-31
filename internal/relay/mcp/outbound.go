package mcp

import (
	"context"
	"errors"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Mediacom99/askrelay/internal/a2a"
	"github.com/Mediacom99/askrelay/internal/envelope"
	"github.com/Mediacom99/askrelay/internal/gate"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// errNotReviewable is the sanitized error for a release/discard of a draft that
// is not pending_review. errTooLong caps message/edit text at MaxBodyBytes.
var (
	errNotReviewable = errors.New("draft is not awaiting review")
	errTooLong       = errors.New("message text is too long")
)

type sendMessageInput struct {
	To     string `json:"to" jsonschema:"recipient email (must be on the roster)"`
	Text   string `json:"text"`
	Thread string `json:"thread,omitempty" jsonschema:"existing thread id; omit to start a new ask"`
}

type sendMessageOutput struct {
	DraftID  string `json:"draft_id"`
	ThreadID string `json:"thread_id"`
	State    string `json:"state"` // "pending_review" | "sent" (auto-released by a grant)
}

// addSendMessage registers send_message, bound to person. It creates an
// outbound draft (a new thread for a fresh ask), then auto-releases it if the
// person holds an outbound grant on the thread; otherwise it stays
// pending_review for approve_reply. Delivery is WP-08.
func (h *Handler) addSendMessage(s *sdkmcp.Server, person string) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "send_message",
		Description: "Ask or reply. The outbound gate holds it for your review unless a grant covers the thread.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in sendMessageInput) (*sdkmcp.CallToolResult, sendMessageOutput, error) {
		now := time.Now().UTC()
		if len(in.Text) > envelope.MaxBodyBytes {
			return nil, sendMessageOutput{}, errTooLong
		}

		recipient, err := h.store.PersonByEmail(in.To)
		if errors.Is(err, store.ErrNotFound) {
			// Accepted membership oracle (maintainer decision): the tool needs
			// to tell the user an unknown recipient on a same-team relay.
			return nil, sendMessageOutput{}, errors.New("recipient is not on the roster")
		}
		if err != nil {
			h.log.Error("send_message: resolve recipient", "err", err)
			return nil, sendMessageOutput{}, errInternal
		}
		if recipient.ID == person {
			return nil, sendMessageOutput{}, errors.New("cannot send a message to yourself")
		}

		threadID := in.Thread
		if threadID == "" {
			if threadID, err = h.store.StartThread(person, recipient.ID, now); err != nil {
				h.log.Error("send_message: start thread", "err", err)
				return nil, sendMessageOutput{}, errInternal
			}
		}

		// Unsigned draft envelope; envelope.New mints the id (= the draft id).
		e := envelope.New(
			envelope.Party{Person: person},
			recipient.ID, threadID, a2a.StateSubmitted, true,
			envelope.Message{Role: "agent", Parts: []envelope.Part{{Type: "text", Text: in.Text}}},
		)
		draftID, err := h.store.CreateDraft(threadID, person, e, now)
		if errors.Is(err, store.ErrNotParticipant) || errors.Is(err, store.ErrNotFound) {
			return nil, sendMessageOutput{}, errNotFound // unknown/not-your thread — no oracle
		}
		if err != nil {
			h.log.Error("send_message: create draft", "err", err)
			return nil, sendMessageOutput{}, errInternal
		}

		// Auto-release iff an outbound grant covers the thread (a fresh ask
		// never has one, so it stays pending_review).
		state := "pending_review"
		rel, err := h.store.ReleaseReplyViaGrant(draftID, person, now)
		if err != nil {
			h.log.Error("send_message: grant release", "err", err)
			return nil, sendMessageOutput{}, errInternal
		}
		if rel != nil {
			state = "sent"
		}
		return emptyResult(), sendMessageOutput{DraftID: draftID, ThreadID: threadID, State: state}, nil
	})
}

type approveReplyInput struct {
	ID         string  `json:"id"`
	EditedText *string `json:"edited_text,omitempty" jsonschema:"optional human edit before release"`
}

type discardReplyInput struct {
	ID string `json:"id"`
}

type replyOutput struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// addOutboundVerdicts registers approve_reply and discard_reply, bound to
// person — the outbound gate (D-11). Author-gated in the store.
func (h *Handler) addOutboundVerdicts(s *sdkmcp.Server, person string) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "approve_reply",
		Description: "Release one of your drafts (optionally editing it first).",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in approveReplyInput) (*sdkmcp.CallToolResult, replyOutput, error) {
		if in.EditedText != nil && len(*in.EditedText) > envelope.MaxBodyBytes {
			return nil, replyOutput{}, errTooLong
		}
		_, err := h.store.ReleaseDraft(person, in.ID, in.EditedText, time.Now().UTC())
		if e := mapReplyErr(h, "approve_reply", err); e != nil {
			return nil, replyOutput{}, e
		}
		return emptyResult(), replyOutput{ID: in.ID, State: "sent"}, nil
	})

	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "discard_reply",
		Description: "Discard one of your drafts without sending it.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in discardReplyInput) (*sdkmcp.CallToolResult, replyOutput, error) {
		err := h.store.DiscardDraft(person, in.ID, time.Now().UTC())
		if e := mapReplyErr(h, "discard_reply", err); e != nil {
			return nil, replyOutput{}, e
		}
		return emptyResult(), replyOutput{ID: in.ID, State: "discarded"}, nil
	})
}

// mapReplyErr sanitizes a draft-verdict store error for the client.
func mapReplyErr(h *Handler, tool string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound):
		return errNotFound
	case errors.Is(err, gate.ErrIllegalTransition):
		return errNotReviewable
	default:
		h.log.Error(tool, "err", err)
		return errInternal
	}
}
