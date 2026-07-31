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

func TestEffectiveTimeout(t *testing.T) {
	cases := []struct {
		secs       int
		clientType string
		want       time.Duration
	}{
		{9999, "chatgpt", 45 * time.Second},  // over-cap clamps down
		{10, "claude.ai", 10 * time.Second},  // under-cap honored
		{0, "chatgpt", 45 * time.Second},     // non-positive → cap
		{-5, "claude.ai", 240 * time.Second}, // negative → cap
	}
	for _, c := range cases {
		if got := effectiveTimeout(c.secs, c.clientType); got != c.want {
			t.Errorf("effectiveTimeout(%d, %q) = %v, want %v", c.secs, c.clientType, got, c.want)
		}
	}
}
