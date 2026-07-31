package mcp

import (
	"context"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const pollInterval = time.Second

// profileCap is the T-10 wait_for_activity ceiling per client profile. The
// client_type strings must match what WP-06's AS stamps on the token. The
// 25-min claude-code-remote figure is provisional until spike S-02.
func profileCap(clientType string) time.Duration {
	switch clientType {
	case "claude.ai", "claude-desktop":
		return 240 * time.Second
	case "claude-code-remote":
		return 25 * time.Minute
	default: // chatgpt and anything unrecognized: the safe 45s
		return 45 * time.Second
	}
}

type waitInput struct {
	TimeoutSeconds int `json:"timeout_seconds" jsonschema:"how long to wait; capped by your client profile"`
}

type waitOutput struct {
	Activity bool `json:"activity"` // true → call check_inbox
}

// effectiveTimeout clamps the requested wait to the client's profile cap (T-10);
// a non-positive or over-cap request becomes the cap. Extracted so the clamp is
// unit-testable without a wall-clock wait.
func effectiveTimeout(requestedSecs int, clientType string) time.Duration {
	d := time.Duration(requestedSecs) * time.Second
	if capped := profileCap(clientType); d <= 0 || d > capped {
		return capped
	}
	return d
}

// addWaitForActivity registers wait_for_activity, bound to person and their
// client profile. It naively polls the store; the daemon uses the WS hub
// (WP-08) for true push instead.
func (h *Handler) addWaitForActivity(s *sdkmcp.Server, person, clientType string) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "wait_for_activity",
		Description: "Long-poll: returns when you have activity to check, or after a timeout capped by your client.",
		Annotations: &sdkmcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in waitInput) (*sdkmcp.CallToolResult, waitOutput, error) {
		if !h.waitAcquire(person) {
			return emptyResult(), waitOutput{Activity: false}, nil // too many concurrent waits
		}
		defer h.waitRelease(person)

		deadline := time.Now().Add(effectiveTimeout(in.TimeoutSeconds, clientType))
		// ponytail: naive per-second store poll — fine for pull clients on a
		// single-team relay; the WS hub (WP-08) is the real-time / scale path.
		for {
			has, err := h.store.HasActivity(person)
			if err != nil {
				h.log.Error("wait_for_activity", "err", err)
				return nil, waitOutput{}, errInternal
			}
			if has {
				return emptyResult(), waitOutput{Activity: true}, nil
			}
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return emptyResult(), waitOutput{Activity: false}, nil
			}
			select {
			case <-ctx.Done(): // client disconnected — idiomatic cancellation
				return nil, waitOutput{}, ctx.Err()
			case <-time.After(min(pollInterval, remaining)):
			}
		}
	})
}
