package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// signableTTL bounds how long a forwarded-body record lingers. A draft nobody
// ever approves would otherwise leave a file behind forever.
const signableTTL = 7 * 24 * time.Hour

// The daemon signs only bodies it forwarded itself (T-21) — otherwise a
// compromised relay could obtain a signature over text its user never wrote,
// which is exactly the property device signatures exist to provide.
//
// The record cannot live in memory: `askrelay mcp` forwards the drafts, while
// `askrelay daemon` holds the /ws connection a sign request arrives on — two
// processes. So it is one small 0600 file per pending draft under the config
// directory, holding a hash of the body and nothing else. Cross-process, and it
// survives a daemon restart, which a map would not.

// bodyHash is what the ledger stores: a hash over the message text only, so the
// envelope's surrounding structure can change without invalidating the record.
func bodyHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func (d *Daemon) signablePath(draftID string) (string, error) {
	// Defence in depth: a draft id reaches us from the relay, and it must never
	// be able to walk out of the pending directory.
	if draftID == "" || strings.ContainsAny(draftID, `/\.`) {
		return "", fmt.Errorf("daemon: refusing suspicious draft id")
	}
	// An unset dir would put the ledger in the process's working directory —
	// which in a test is the package source tree. Refuse rather than scatter.
	if d.dir == "" {
		return "", fmt.Errorf("daemon: no config directory for the pending ledger")
	}
	return filepath.Join(d.dir, "pending", draftID), nil
}

// recordSignable notes that this machine forwarded text as draft draftID, so a
// later sign request for it can be recognised. Never stores the text itself.
func (d *Daemon) recordSignable(draftID, text string) {
	path, err := d.signablePath(draftID)
	if err != nil {
		d.log.Warn("ledger: bad draft id", "err", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		d.log.Warn("ledger: mkdir", "err", err)
		return
	}
	if err := os.WriteFile(path, []byte(bodyHash(text)), 0o600); err != nil {
		d.log.Warn("ledger: write", "err", err, "draft_id", draftID)
	}
}

// forwarded reports whether this machine forwarded exactly this body for this
// draft. A miss is the normal answer for a draft created straight against the
// relay with no daemon in the path.
func (d *Daemon) forwarded(draftID, text string) bool {
	path, err := d.signablePath(draftID)
	if err != nil {
		return false
	}
	want, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return string(want) == bodyHash(text)
}

// forgetSignable drops a record once its draft has been signed and released.
func (d *Daemon) forgetSignable(draftID string) {
	if path, err := d.signablePath(draftID); err == nil {
		_ = os.Remove(path)
	}
}

// sweepSignable removes records past signableTTL — drafts that were never
// approved, or were released without this daemon.
func (d *Daemon) sweepSignable(now time.Time) {
	dir := filepath.Join(d.dir, "pending")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // no pending directory yet: nothing to sweep
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < signableTTL {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}
