package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mediacom99/askrelay/internal/relay/oauth"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "relay.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	iss, err := oauth.NewIssuer(filepath.Join(dir, "signing.key"), "https://relay.example.com", time.Hour)
	if err != nil {
		t.Fatalf("oauth.NewIssuer: %v", err)
	}
	cfg := Config{ListenAddr: "127.0.0.1:0", DBPath: "x", BaseURL: "https://relay.example.com"}
	return NewServer(cfg, st, iss, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// serve routes a request through the full handler chain (mux + logging) so
// PathValue is populated exactly as in production.
func (s *Server) serve(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.logRequests(s.mux).ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	s := testServer(t)
	rec := s.serve(httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" || body["version"] != Version {
		t.Errorf("body = %v, want status ok + version %q", body, Version)
	}
}

func TestRunGracefulShutdown(t *testing.T) {
	s := testServer(t)
	s.cfg.ListenAddr = "127.0.0.1:0" // ephemeral port
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	time.Sleep(50 * time.Millisecond) // let ListenAndServe start
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v, want clean shutdown", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

// enrollBody builds a valid enroll request body for a fresh key.
func enrollBody(t *testing.T) (io.Reader, ed25519.PublicKey) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	b, _ := json.Marshal(enrollRequest{PubKey: base64.StdEncoding.EncodeToString(pub), Label: "laptop"})
	return strings.NewReader(string(b)), pub
}

func TestEnrollHappyPath(t *testing.T) {
	s := testServer(t)
	token, err := s.store.CreateInvite("marco@example.com", time.Hour, time.Now().UTC())
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	body, _ := enrollBody(t)
	rec := s.serve(httptest.NewRequest("POST", "/enroll/"+token, body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	var resp enrollResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.PersonID == "" || resp.DeviceID == "" || resp.BaseURL != "https://relay.example.com" {
		t.Errorf("response missing fields: %+v", resp)
	}
	// The returned device credential must verify as this device's.
	person, device, err := s.issuer.VerifyDeviceCredential(resp.DeviceCredential, time.Now())
	if err != nil {
		t.Fatalf("returned device_credential does not verify: %v", err)
	}
	if person != resp.PersonID || device != resp.DeviceID {
		t.Errorf("credential = (person %q, device %q), want (%q, %q)", person, device, resp.PersonID, resp.DeviceID)
	}
}

func TestEnrollErrors(t *testing.T) {
	s := testServer(t)

	// Unknown/invalid token → 403, opaque.
	body, _ := enrollBody(t)
	rec := s.serve(httptest.NewRequest("POST", "/enroll/no-such-token", body))
	if rec.Code != http.StatusForbidden {
		t.Errorf("bad token: status = %d, want 403", rec.Code)
	}

	// Malformed pubkey → 400.
	bad, _ := json.Marshal(enrollRequest{PubKey: "not-base64!!", Label: "x"})
	tok, _ := s.store.CreateInvite("a@example.com", time.Hour, time.Now().UTC())
	rec = s.serve(httptest.NewRequest("POST", "/enroll/"+tok, strings.NewReader(string(bad))))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad pubkey: status = %d, want 400", rec.Code)
	}

	// Duplicate key → 409.
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	mk := func(email string) *http.Request {
		tk, _ := s.store.CreateInvite(email, time.Hour, time.Now().UTC())
		b, _ := json.Marshal(enrollRequest{PubKey: base64.StdEncoding.EncodeToString(pub)})
		return httptest.NewRequest("POST", "/enroll/"+tk, strings.NewReader(string(b)))
	}
	if rec := s.serve(mk("b@example.com")); rec.Code != http.StatusOK {
		t.Fatalf("first dup-key enroll: status = %d, want 200", rec.Code)
	}
	if rec := s.serve(mk("c@example.com")); rec.Code != http.StatusConflict {
		t.Errorf("second dup-key enroll: status = %d, want 409", rec.Code)
	}

	// Oversized body → 400 (the LimitReader truncates, decode fails).
	tok2, _ := s.store.CreateInvite("d@example.com", time.Hour, time.Now().UTC())
	huge := `{"label":"` + strings.Repeat("A", maxEnrollBody+100) + `"}`
	rec = s.serve(httptest.NewRequest("POST", "/enroll/"+tok2, strings.NewReader(huge)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("oversized body: status = %d, want 400", rec.Code)
	}
}
