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

// addWaitForActivity registers wait_for_activity, bound to person and their
// client profile. It naively polls the store; the daemon uses the WS hub
// (WP-08) for true push instead.
func (h *Handler) addWaitForActivity(s *sdkmcp.Server, person, clientType string) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "wait_for_activity",
		Description: "Long-poll: returns when you have activity to check, or after a timeout capped by your client.",
		Annotations: &sdkmcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in waitInput) (*sdkmcp.CallToolResult, waitOutput, error) {
		d := time.Duration(in.TimeoutSeconds) * time.Second
		if capped := profileCap(clientType); d <= 0 || d > capped {
			d = capped
		}
		deadline := time.Now().Add(d)
		// ponytail: naive per-second store poll — fine for pull clients on a
		// single-team relay; the WS hub (WP-08) is the real-time / scale path.
		for {
			has, err := h.store.HasActivity(person)
			if err != nil {
				h.log.Error("wait_for_activity", "err", err)
				return nil, waitOutput{}, errInternal
			}
			if has {
				return nil, waitOutput{Activity: true}, nil
			}
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return nil, waitOutput{Activity: false}, nil
			}
			wait := min(pollInterval, remaining)
			select {
			case <-ctx.Done(): // client disconnected
				return nil, waitOutput{}, ctx.Err()
			case <-time.After(wait):
			}
		}
	})
}
