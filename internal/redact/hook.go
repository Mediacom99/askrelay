package redact

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

const (
	// maxHookOutput caps a hook's stdout. A redaction replacement of a small
	// message is small; unbounded output is treated as a failure (fail-closed).
	maxHookOutput = 1 << 20 // 1 MiB
	// hookWaitDelay bounds how long Wait lingers after the process exits (or ctx
	// fires) for I/O pipes still held open by an orphaned descendant. Without it,
	// a hook that backgrounds anything inheriting stdout makes Run() block past
	// the caller's deadline — potentially forever — and then return a false
	// success; with it, Wait force-closes the pipes and returns ErrWaitDelay.
	hookWaitDelay = 500 * time.Millisecond
)

// Hook runs the external redaction executable at path — the message text on the
// program's stdin, the replacement read from its stdout — bounded by ctx (the
// caller sets the timeout via a deadline). It is the pluggable "bring your own
// scanner" step (T-12), normally composed AFTER the built-in Redact.
//
// FAIL-CLOSED: on ANY failure it returns "" and an error, and the caller MUST
// treat that as "block the send" — never fall through to the un-hooked text. A
// broken or slow hook must never silently leak. Failure covers: non-zero exit;
// ctx timeout/cancel; a descendant holding stdout open past hookWaitDelay
// (ErrWaitDelay); spawn error; output past maxHookOutput; and — since an empty
// replacement for non-empty input is almost always a broken/misconfigured hook
// (crashed, wrote to stderr, no-op) — a silent wipe, which is blocked rather
// than shipped as a blanked message.
func Hook(ctx context.Context, path, text string) (string, error) {
	cmd := exec.CommandContext(ctx, path)
	cmd.WaitDelay = hookWaitDelay
	cmd.Stdin = strings.NewReader(text)
	out := &cappedWriter{max: maxHookOutput}
	cmd.Stdout = out
	// stderr is discarded, never captured or surfaced: a hook that echoes the
	// secret to stderr for debugging must not have it leak into an error or log,
	// and discarding also bounds memory with no separate cap.
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("redact: hook %q failed (fail-closed): %w", path, err)
	}
	if len(text) > 0 && out.buf.Len() == 0 {
		return "", errors.New("redact: hook produced empty output for non-empty input (fail-closed)")
	}
	return out.buf.String(), nil
}

// cappedWriter fails once more than max bytes are written, so a runaway hook
// can't exhaust memory (and the failure fails the send, closed).
type cappedWriter struct {
	buf bytes.Buffer
	n   int
	max int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if w.n+len(p) > w.max {
		return 0, errors.New("redact: hook output exceeded cap")
	}
	w.n += len(p)
	return w.buf.Write(p)
}
