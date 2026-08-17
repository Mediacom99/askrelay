package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// enrollStub serves a canned enroll response, standing in for a live relay's
// POST /enroll/{token} so cmdEnroll can be tested without a running server.
func enrollStub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(enrollResp{
			PersonID: "p1", DeviceID: "d1",
			BaseURL: "http://relay.example", DeviceCredential: "CRED123",
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCmdEnroll(t *testing.T) {
	srv := enrollStub(t)
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.json")

	if err := cmdEnroll([]string{srv.URL + "/enroll/tok", "-config", cfg, "-label", "lap"}); err != nil {
		t.Fatalf("cmdEnroll: %v", err)
	}

	// config.json holds the arch §7 fields, 0600.
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var dc deviceConfig
	if err := json.Unmarshal(raw, &dc); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if dc.RelayURL != "http://relay.example" || dc.PersonID != "p1" || dc.DeviceID != "d1" {
		t.Errorf("config = %+v", dc)
	}
	if fi, _ := os.Stat(cfg); fi.Mode().Perm() != 0o600 {
		t.Errorf("config perm = %v, want 0600", fi.Mode().Perm())
	}
	// the private key is written beside it, 0600.
	if fi, err := os.Stat(filepath.Join(dir, "device.key")); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("device.key: err=%v perm=%v", err, fi.Mode().Perm())
	}

	// re-enroll without -force is refused; with -force it succeeds.
	if err := cmdEnroll([]string{srv.URL + "/enroll/tok", "-config", cfg}); err == nil {
		t.Error("second enroll without -force should error")
	}
	if err := cmdEnroll([]string{srv.URL + "/enroll/tok", "-config", cfg, "-force"}); err != nil {
		t.Errorf("enroll -force: %v", err)
	}
}

func TestCmdEnrollHTTPErrorWritesNothing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"invalid or expired invite"}`, http.StatusForbidden)
	}))
	defer srv.Close()

	cfg := filepath.Join(t.TempDir(), "config.json")
	if err := cmdEnroll([]string{srv.URL + "/enroll/tok", "-config", cfg}); err == nil {
		t.Fatal("expected error on HTTP 403")
	}
	if _, err := os.Stat(cfg); !os.IsNotExist(err) {
		t.Error("config must not be written when enrollment fails")
	}
}

func TestCmdDevice(t *testing.T) {
	if err := cmdDevice(nil); err == nil {
		t.Error("no subcommand should error")
	}
	if err := cmdDevice([]string{"frobnicate"}); err == nil {
		t.Error("unknown subcommand should error")
	}

	db := filepath.Join(t.TempDir(), "t.db")
	st, err := store.Open(db)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	now := time.Now().UTC()
	tok, err := st.CreateInvite("z@x.com", time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, d, err := st.Enroll(tok, pub, "lap", now)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	st.Close() // release before the CLI reopens the same db

	if err := cmdDeviceList([]string{"-db", db}); err != nil {
		t.Errorf("device list: %v", err)
	}
	if err := cmdDeviceRevoke([]string{d.ID, "-db", db}); err != nil {
		t.Errorf("device revoke: %v", err)
	}
	if err := cmdDeviceRevoke([]string{"no-such-id", "-db", db}); err == nil {
		t.Error("revoking an unknown device should error")
	}
}
