package daemon

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Config is the daemon/user config written by `enroll` (arch §7): JSON at
// ~/.config/askrelay/config.json (0600). The daemon owns this type so the writer
// (enroll) and the reader (daemon) share one contract.
type Config struct {
	RelayURL string `json:"relay_url"`
	PersonID string `json:"person_id"`
	DeviceID string `json:"device_id"`
	KeyPath  string `json:"device_key_path"`
	// DeviceCredential is the long-lived /ws bearer (T-06) minted at enrollment.
	// It is a secret at rest: this file is written 0600 and never logged.
	DeviceCredential string `json:"device_credential"`
	// RedactHook is an optional executable run after the built-in patterns
	// (T-12, "bring your own scanner"). Empty = built-ins only. It is
	// fail-closed: a hook that errors blocks the send.
	RedactHook string `json:"redact_hook,omitempty"`
}

// LoadConfig reads and validates the daemon config JSON.
func LoadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("daemon: read config %q: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("daemon: parse config %q: %w", path, err)
	}
	if c.RelayURL == "" || c.PersonID == "" || c.DeviceID == "" || c.KeyPath == "" || c.DeviceCredential == "" {
		return Config{}, fmt.Errorf("daemon: config %q is incomplete — run `askrelay enroll` first", path)
	}
	return c, nil
}

// loadKey reads the device's raw 64-byte Ed25519 private key (as written by
// enroll). The public half is re-derived from the seed and compared, so an
// in-place-corrupted key is caught at startup rather than producing signatures
// that never verify (mirrors the relay's readKey).
func loadKey(path string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("daemon: read device key %q: %w", path, err)
	}
	if len(b) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("daemon: device key %q is %d bytes, want %d — re-enroll", path, len(b), ed25519.PrivateKeySize)
	}
	key := ed25519.PrivateKey(b)
	if !key.Public().(ed25519.PublicKey).Equal(ed25519.NewKeyFromSeed(b[:ed25519.SeedSize]).Public()) {
		return nil, fmt.Errorf("daemon: device key %q is corrupt", path)
	}
	return key, nil
}

// Daemon is the optional local component (arch §6): a stdio MCP server + an
// outbound-only WS client to the relay + an outbound queue. It redacts and signs
// outbound before it leaves the machine (the "local-signed" mode) and reacts to
// relay push. Nothing core requires it (D-06). ST-1 is the skeleton; the WS
// client (ST-2), outbound signing (ST-3), and stdio server (ST-4) mount here.
type Daemon struct {
	cfg Config
	key ed25519.PrivateKey
	log *slog.Logger
	// version is stamped on the MCP Implementation this daemon advertises.
	version string
	// dir is the config directory: home of the device key and the pending-draft
	// ledger the two local roles share (T-21).
	dir  string
	seen map[string]bool // message ids notified this run; touched only by session's goroutine
	// notify raises the OS notification. A field, not a direct call, so tests
	// can observe notifications without firing real desktop popups.
	notify func()
}

// Load reads the config + device key at configPath and builds a Daemon, failing
// fast if enrollment is incomplete. It serves both local roles: Run is the
// background notifier, ServeStdio is the MCP server an AI client spawns.
func Load(configPath, version string, log *slog.Logger) (*Daemon, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	key, err := loadKey(cfg.KeyPath)
	if err != nil {
		return nil, err
	}
	d := &Daemon{cfg: cfg, key: key, log: log, version: version,
		dir: filepath.Dir(configPath), seen: map[string]bool{}}
	d.notify = d.osNotify
	return d, nil
}

// Run drives the daemon until ctx is cancelled (SIGINT/SIGTERM from the caller).
func (d *Daemon) Run(ctx context.Context) error {
	d.log.Info("daemon started", "relay", d.cfg.RelayURL, "person", d.cfg.PersonID, "device", d.cfg.DeviceID)
	d.sweepSignable(time.Now())
	d.listen(ctx) // returns on ctx cancellation, or on an unrecoverable auth failure
	d.log.Info("daemon shutting down")
	return nil
}
