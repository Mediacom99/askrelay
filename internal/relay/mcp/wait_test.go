package mcp

import (
	"testing"
	"time"
)

func TestProfileCap(t *testing.T) {
	cases := map[string]time.Duration{
		"claude.ai":          240 * time.Second,
		"claude-desktop":     240 * time.Second,
		"claude-code-remote": 25 * time.Minute,
		"chatgpt":            45 * time.Second,
		"":                   45 * time.Second,
		"something-new":      45 * time.Second,
	}
	for ct, want := range cases {
		if got := profileCap(ct); got != want {
			t.Errorf("profileCap(%q) = %v, want %v", ct, got, want)
		}
	}
}
