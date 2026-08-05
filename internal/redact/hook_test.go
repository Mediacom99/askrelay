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

// CRITICAL: a hook that backgrounds a grandchild inheriting its stdout fd (a
// stray `&`, nohup, double-fork daemonization — all common in real shell
// scripts) makes Hook() ignore the ctx deadline entirely AND return success.
// cmd.Run()'s stdout copy blocks on Read() until every writer of the pipe
// closes it; CommandContext only kills the ONE tracked process, not
// descendants, so the immediate script can exit 0 in milliseconds while a
// grandchild silently holds the pipe open. Hook() then hangs until that
// grandchild closes stdout on its own — for as long as it likes — and once it
// does, reports err=nil (since the tracked process's own exit was clean),
// handing the caller a "successful" (here, empty) replacement long after the
// deadline it was told to respect. This falsifies both the doc comment above
// ("ctx deadline/cancel surfaces here too") and the fail-closed contract.
func TestHookOrphanGrandchildIgnoresDeadlineAndFailsOpen(t *testing.T) {
	hook := writeHook(t, "(sleep 3) &\nexit 0")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	out, err := Hook(ctx, hook, "secret text")
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("Hook took %v despite a 200ms ctx deadline — an orphaned grandchild holding stdout open defeats the deadline (hook.go doc comment claims ctx cancel always surfaces; it does not)", elapsed)
	}
	if err == nil {
		t.Errorf("Hook returned success (err=nil, out=%q) after blowing past its deadline — MUST fail closed on a deadline violation regardless of the tracked process's own exit code", out)
	}
}

// A hook that exits 0 but writes nothing to stdout for non-empty input is
// almost always broken/misconfigured; Hook must fail closed (block the send)
// rather than silently ship a blanked message (WP-11 quality-pass decision).
func TestHookEmptyOutputForNonEmptyInputBlocks(t *testing.T) {
	hook := writeHook(t, `exit 0`) // writes nothing
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := Hook(ctx, hook, "PASSWORD=hunter2 do not lose this message")
	if err == nil {
		t.Fatal("hook that emptied non-empty input returned nil error — MUST fail closed")
	}
	if out != "" {
		t.Errorf("out = %q, want empty on the error path", out)
	}
}

// A hook that writes its (would-be) replacement to stderr instead of stdout is a
// common misconfiguration: stdout is empty, so it lands in the same fail-closed
// "empty output" bucket — and the stderr content (which could contain the
// secret) is discarded, never surfaced in the error.
func TestHookStderrOnlyOutputBlocks(t *testing.T) {
	hook := writeHook(t, `echo "meant to replace: hunter2" 1>&2`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := Hook(ctx, hook, "PASSWORD=hunter2")
	if err == nil {
		t.Fatal("hook that wrote only to stderr returned nil error — MUST fail closed")
	}
	if out != "" {
		t.Errorf("out = %q, want empty (script wrote only to stderr)", out)
	}
}

// ctx already cancelled before Hook is even called must still fail closed
// (never spawn the process / never return the raw input).
func TestHookCtxAlreadyCancelled(t *testing.T) {
	hook := writeHook(t, `cat`) // would otherwise just echo stdin back
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out, err := Hook(ctx, hook, "secret text")
	if err == nil {
		t.Fatal("Hook with an already-cancelled ctx returned nil error — MUST fail closed")
	}
	if out != "" {
		t.Errorf("Hook with an already-cancelled ctx returned text %q, want empty", out)
	}
}

// A hook exiting non-zero but ALSO writing valid-looking replacement text to
// stdout first must still fail closed — the exit code, not the presence of
// output, is what the caller must trust.
func TestHookNonZeroExitWithStdoutStillFailsClosed(t *testing.T) {
	hook := writeHook(t, `printf 'looks fine, PASSWORD=CUSTOM-REDACTED'; exit 1`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := Hook(ctx, hook, "PASSWORD=hunter2")
	if err == nil {
		t.Fatal("non-zero exit with stdout output returned nil error — MUST fail closed")
	}
	if out != "" {
		t.Errorf("non-zero exit hook returned text %q alongside an error — caller MUST see empty text on any error path", out)
	}
}

// The cap trip races the child still writing: a hook that keeps writing after
// the cap fires and ignores SIGPIPE (ignoring the broken pipe rather than
// dying from it — plausible for a runtime/script that traps or doesn't check
// write() return values) must not be allowed to defeat the deadline either.
// This exercises the same "descendant keeps producing after the reader gave
// up" hazard as the orphan test above, from the output-cap side.
func TestHookRunawayOutputIgnoringSIGPIPERespectsDeadline(t *testing.T) {
	hook := writeHook(t, "trap '' PIPE\nwhile :; do printf '%s' 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'; done")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	out, err := Hook(ctx, hook, "x")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("runaway SIGPIPE-ignoring hook returned nil error — MUST fail closed")
	}
	if out != "" {
		t.Errorf("runaway hook returned %d bytes, want none", len(out))
	}
	if elapsed > 3*time.Second {
		t.Errorf("Hook took %v against a 1s ctx deadline fighting a SIGPIPE-ignoring writer — deadline not respected", elapsed)
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
