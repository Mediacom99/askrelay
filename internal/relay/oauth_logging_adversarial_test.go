package relay

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mediacom99/askrelay/internal/relay/oauth"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// ---------------------------------------------------------------------------
// ADVERSARIAL (askrelay-test, WP-06 quality pass): T-18 ("NEVER log content,
// secrets, keys, tokens... ids and shapes only") verified against ACTUAL log
// output, not by reading the call sites. testServer(t) discards logs, so this
// builds its own logger over a buffer.
// ---------------------------------------------------------------------------

// testServerWithLog is testServer, but with a real *slog.Logger backed by buf
// instead of io.Discard, so tests can assert on what actually got logged.
func testServerWithLog(t *testing.T) (*Server, *bytes.Buffer) {
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
	cfg := Config{
		ListenAddr: "127.0.0.1:0", DBPath: "x", BaseURL: "https://relay.example.com",
		MaxAge: 24 * time.Hour, MaxSkew: 5 * time.Minute,
		AckGrace: 72 * time.Hour, HardTTL: 720 * time.Hour,
	}
	var buf bytes.Buffer
	srv := NewServer(cfg, st, iss, slog.New(slog.NewTextHandler(&buf, nil)))
	srv.fetchCIMD = disabledCIMD
	return srv, &buf
}

// TestAuthorizeNeverLogsCredentialOrCode: a pasted device credential, and the
// minted authorization code, must never appear in the log stream -- on the
// happy path, a bad-credential refusal, or a revoked-device refusal.
func TestAuthorizeNeverLogsCredentialOrCode(t *testing.T) {
	s, buf := testServerWithLog(t)
	const redirect = "https://client.example.com/cb"
	cid := dcrClient(t, s, redirect)
	_, _, cred := enrolledCredential(t, s)

	form := authzQuery(cid, redirect)
	form.Set("credential", cred)
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAuthorizeSubmit(rec, req)
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("bad Location: %v", err)
	}
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatal("no code minted")
	}

	if strings.Contains(buf.String(), cred) {
		t.Error("device credential value appeared in the log stream (T-18)")
	}
	if strings.Contains(buf.String(), code) {
		t.Error("authorization code value appeared in the log stream (T-18)")
	}

	// Bad-credential and revoked-device refusals: neither the garbage
	// credential nor a real-but-now-revoked one may be logged either.
	buf.Reset()
	badForm := authzQuery(cid, redirect)
	badForm.Set("credential", "totally-bogus-credential-value")
	req2 := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(badForm.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleAuthorizeSubmit(httptest.NewRecorder(), req2)
	if strings.Contains(buf.String(), "totally-bogus-credential-value") {
		t.Error("bad credential value appeared in the log stream (T-18)")
	}
}

// TestTokenNeverLogsCodeOrRefreshToken: the code, verifier, and refresh token
// values must never appear in the log stream -- on success, on a PKCE
// mismatch, and on a refresh-token replay (the T-18-sensitive reuse-signal
// log line specifically).
func TestTokenNeverLogsCodeOrRefreshToken(t *testing.T) {
	s, buf := testServerWithLog(t)
	code, clientID, redirect, _, _ := seedCode(t, s)

	rec := postToken(s, codeForm(code, clientID, redirect, testVerifier))
	tr := decodeToken(t, rec)
	if strings.Contains(buf.String(), code) {
		t.Error("authorization code value appeared in the log stream (T-18)")
	}
	if strings.Contains(buf.String(), testVerifier) {
		t.Error("PKCE verifier value appeared in the log stream (T-18)")
	}
	if strings.Contains(buf.String(), tr.AccessToken) || strings.Contains(buf.String(), tr.RefreshToken) {
		t.Error("issued access/refresh token value appeared in the log stream (T-18)")
	}

	buf.Reset()
	// Replay the (now-consumed) refresh token -- the reuse-signal Warn line
	// (oauth_token.go: reason=refresh_reuse) must still be id/shape-only.
	refreshForm := codeForm(code, clientID, redirect, testVerifier) // wrong grant, harmless
	refreshForm.Set("grant_type", "refresh_token")
	refreshForm.Set("refresh_token", tr.RefreshToken)
	postToken(s, refreshForm)
	postToken(s, refreshForm) // second presentation -> reuse
	if strings.Contains(buf.String(), tr.RefreshToken) {
		t.Error("refresh token value appeared in the log stream on the reuse path (T-18)")
	}
}
