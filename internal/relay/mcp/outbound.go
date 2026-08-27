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
	Text     string `json:"text"`  // echo of the queued body, so the human reviews exactly what will send before approving
}

// addSendMessage registers send_message, bound to person. It creates an
// outbound draft (a new thread for a fresh ask), then auto-releases it if the
// person holds an outbound grant on the thread; otherwise it stays
// pending_review for approve_reply. Delivery is WP-08.
func (h *Handler) addSendMessage(s *sdkmcp.Server, person string) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "send_message",
		Description: "Draft an ask or a reply. It is HELD for your human's review and is NOT sent until they approve it (approve_reply) — do not approve your own draft. If a thread grant already covers this direction, the relay auto-releases it.",
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
		recipientID, rel, err := h.store.ReleaseReplyViaGrant(draftID, person, now)
		if err != nil {
			h.log.Error("send_message: grant release", "err", err)
			return nil, sendMessageOutput{}, errInternal
		}
		if rel != nil { // grant fired → released AND delivered atomically
			state = "sent"
			h.notify(recipientID)
		}
		return emptyResult(), sendMessageOutput{DraftID: draftID, ThreadID: threadID, State: state, Text: in.Text}, nil
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
	// Attestation is what the recipient will be shown about this message's
	// origin: "device-signed" when the author's daemon signed the exact released
	// bytes, "relay-attested" otherwise (T-21). The sender is told which they got.
	Attestation string `json:"attestation,omitempty"`
}

// addOutboundVerdicts registers approve_reply and discard_reply, bound to
// person — the outbound gate (D-11). Author-gated in the store.
func (h *Handler) addOutboundVerdicts(s *sdkmcp.Server, person string) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "approve_reply",
		Description: "Release (SEND) one of your drafts to its recipient — irreversible. Only call AFTER your human has explicitly approved this specific draft; otherwise show them the draft and wait. Optionally edit the text first.",
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in approveReplyInput) (*sdkmcp.CallToolResult, replyOutput, error) {
		if in.EditedText != nil && len(*in.EditedText) > envelope.MaxBodyBytes {
			return nil, replyOutput{}, errTooLong
		}
		now := time.Now().UTC()
		// T-19/T-21: offer the release to the author's daemon for a device
		// signature first. The same `now` is used for both calls, so the bytes
		// signed are the bytes delivered.
		device, sig := h.signatureFor(ctx, person, in.ID, in.EditedText, now)
		var (
			recipientID string
			signed      bool
			err         error
		)
		if sig != nil {
			recipientID, _, signed, err = h.store.ReleaseDraftSigned(person, in.ID, in.EditedText, now, device, sig)
			if err == nil && !signed {
				// SignRelease already verified this signature, so a mismatch here
				// means the release re-derived different bytes: a bug, not a
				// hostile daemon. Delivery went ahead relay-attested.
				h.log.Error("approve_reply: signature dropped at release", "message_id", in.ID)
			}
		} else {
			recipientID, _, err = h.store.ReleaseDraft(person, in.ID, in.EditedText, now)
		}
		if e := mapReplyErr(h, "approve_reply", err); e != nil {
			return nil, replyOutput{}, e
		}
		h.notify(recipientID) // release delivered it atomically
		return emptyResult(), replyOutput{ID: in.ID, State: "sent", Attestation: attestationOf(signed)}, nil
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

// signatureFor asks the author's daemon to sign the release-to-be. Every failure
// path returns no signature: no signer wired, an unreadable draft (the release
// below reports the real error), no daemon connected, a refusal, or a signature
// that did not verify. T-21: signing degrades the attestation, it never blocks
// the send.
func (h *Handler) signatureFor(ctx context.Context, person, draftID string, edited *string, now time.Time) (string, []byte) {
	if h.signer == nil {
		return "", nil
	}
	e, err := h.store.DraftForSigning(person, draftID, edited, now)
	if err != nil {
		return "", nil
	}
	device, sig, ok := h.signer.SignRelease(ctx, person, e)
	if !ok {
		return "", nil
	}
	return device, sig
}

// attestationOf names what the recipient will be told about a message's origin.
func attestationOf(signed bool) string {
	if signed {
		return "device-signed"
	}
	return "relay-attested"
}
