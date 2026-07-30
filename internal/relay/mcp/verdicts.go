package mcp

import (
	"context"
	"errors"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Mediacom99/askrelay/internal/gate"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// errNotAwaiting is the sanitized error for a verdict on a message whose thread
// is not in input-required (already approved/declined).
var errNotAwaiting = errors.New("message is not awaiting approval")

type verdictInput struct {
	ID string `json:"id" jsonschema:"the message id"`
}

type verdictOutput struct {
	ID          string `json:"id"`
	ThreadState string `json:"thread_state"`
}

// addInboundVerdicts registers approve_message and decline_message, bound to
// person — the inbound gate from within a session (D-03). The store confirms
// the message is the caller's to act on and applies the gate transition
// atomically.
func (h *Handler) addInboundVerdicts(s *sdkmcp.Server, person string) {
	verdict := func(name, desc string, apply func(string) (string, error)) {
		sdkmcp.AddTool(s, &sdkmcp.Tool{Name: name, Description: desc},
			func(_ context.Context, _ *sdkmcp.CallToolRequest, in verdictInput) (*sdkmcp.CallToolResult, verdictOutput, error) {
				state, err := apply(in.ID)
				switch {
				case errors.Is(err, store.ErrNotFound):
					return nil, verdictOutput{}, errNotFound
				case errors.Is(err, gate.ErrIllegalTransition):
					return nil, verdictOutput{}, errNotAwaiting
				case err != nil:
					h.log.Error(name, "err", err)
					return nil, verdictOutput{}, errInternal
				}
				return nil, verdictOutput{ID: in.ID, ThreadState: state}, nil
			})
	}
	verdict("approve_message", "Approve an inbound message awaiting your verdict.", func(id string) (string, error) {
		st, err := h.store.ApproveInboundMessage(person, id, time.Now().UTC())
		return string(st), err
	})
	verdict("decline_message", "Decline an inbound message awaiting your verdict.", func(id string) (string, error) {
		st, err := h.store.DeclineInboundMessage(person, id, time.Now().UTC())
		return string(st), err
	})
}

type setGrantInput struct {
	ThreadID  string `json:"thread_id" jsonschema:"the thread to grant on"`
	Direction string `json:"direction" jsonschema:"inbound or outbound"`
	Enabled   bool   `json:"enabled" jsonschema:"true to grant auto-approve, false to revoke"`
}

type setGrantOutput struct {
	ThreadID  string `json:"thread_id"`
	Direction string `json:"direction"`
	Enabled   bool   `json:"enabled"`
}

// addSetThreadGrant registers set_thread_grant, bound to person — a revocable
// per-thread, per-direction auto-approve (D-03/D-11). Only a thread
// participant may grant on it.
func (h *Handler) addSetThreadGrant(s *sdkmcp.Server, person string) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "set_thread_grant",
		Description: "Enable or disable per-thread auto-approve for a direction (inbound or outbound).",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in setGrantInput) (*sdkmcp.CallToolResult, setGrantOutput, error) {
		dir := gate.Direction(in.Direction)
		now := time.Now().UTC()
		var err error
		if in.Enabled {
			err = h.store.SetThreadGrant(in.ThreadID, person, dir, now)
		} else {
			err = h.store.RevokeThreadGrant(in.ThreadID, person, dir, now)
		}
		switch {
		case errors.Is(err, gate.ErrWrongDirection):
			return nil, setGrantOutput{}, errors.New("direction must be inbound or outbound")
		case errors.Is(err, store.ErrNotParticipant):
			return nil, setGrantOutput{}, errNotFound // not your thread — no oracle
		case !in.Enabled && errors.Is(err, store.ErrNotFound):
			// disabling an inactive grant is idempotent success — fall through
		case err != nil:
			h.log.Error("set_thread_grant", "err", err)
			return nil, setGrantOutput{}, errInternal
		}
		return nil, setGrantOutput(in), nil
	})
}
