package redact

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeHook writes an executable /bin/sh script and returns its path.
func writeHook(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "hook.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write hook: %v", err)
	}
	return p
}

func TestHookHappyPath(t *testing.T) {
	hook := writeHook(t, `sed 's/hunter2/CUSTOM-REDACTED/'`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := Hook(ctx, hook, "PASSWORD=hunter2")
	if err != nil {
		t.Fatalf("Hook: %v", err)
	}
	if !strings.Contains(out, "PASSWORD=CUSTOM-REDACTED") {
		t.Errorf("hook output = %q, want it to contain the replacement", out)
	}
}

func TestHookTimeoutFailsClosed(t *testing.T) {
	hook := writeHook(t, `sleep 5`)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	out, err := Hook(ctx, hook, "secret text")
	if err == nil {
		t.Fatal("timed-out hook returned nil error — MUST fail closed")
	}
	if out != "" {
		t.Errorf("timed-out hook returned text %q, want empty (no fall-through)", out)
	}
}

func TestHookNonZeroExitFailsClosed(t *testing.T) {
	hook := writeHook(t, `exit 1`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := Hook(ctx, hook, "secret text")
	if err == nil {
		t.Fatal("failing hook returned nil error — MUST fail closed")
	}
	if out != "" {
		t.Errorf("failing hook returned text %q, want empty", out)
	}
}

func TestHookMissingPathFailsClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := Hook(ctx, filepath.Join(t.TempDir(), "does-not-exist"), "x"); err == nil {
		t.Fatal("missing hook returned nil error — MUST fail closed")
	}
}

func TestHookOutputCapFailsClosed(t *testing.T) {
	// Emit ~2 MiB, over maxHookOutput (1 MiB).
	hook := writeHook(t, `yes | head -c 2097152`)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := Hook(ctx, hook, "x")
	if err == nil {
		t.Fatal("runaway-output hook returned nil error — MUST fail closed")
	}
	if len(out) != 0 {
		t.Errorf("runaway hook returned %d bytes, want none", len(out))
	}
}
