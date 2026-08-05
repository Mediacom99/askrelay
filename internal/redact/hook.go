package redact

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// maxHookOutput caps a hook's stdout. A redaction replacement of a small message
// is small; unbounded output is treated as a failure (fail-closed).
const maxHookOutput = 1 << 20 // 1 MiB

// Hook runs the external redaction executable at path — the message text on the
// program's stdin, the replacement read from its stdout — bounded by ctx (the
// caller sets the timeout via a deadline). It is the pluggable "bring your own
// scanner" step (T-12), normally composed AFTER the built-in Redact.
//
// FAIL-CLOSED: on ANY failure (non-zero exit, ctx timeout/cancel, spawn error,
// or output past maxHookOutput) it returns "" and an error. The caller MUST
// treat that as "block the send" and never fall through to the un-hooked text —
// a broken or slow hook must never silently leak a secret.
func Hook(ctx context.Context, path, text string) (string, error) {
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = strings.NewReader(text)
	out := &cappedWriter{max: maxHookOutput}
	var errb bytes.Buffer
	cmd.Stdout = out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		// ctx deadline/cancel surfaces here too (CommandContext kills the process).
		return "", fmt.Errorf("redact: hook %q failed (fail-closed): %w", path, err)
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
