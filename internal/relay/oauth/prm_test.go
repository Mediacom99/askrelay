package oauth

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProtectedResourceMetadata(t *testing.T) {
	const base = "https://relay.example.com"
	rec := httptest.NewRecorder()
	ProtectedResourceMetadataHandler(base).ServeHTTP(rec, httptest.NewRequest("GET", PRMPath, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var doc struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode PRM: %v", err)
	}
	if doc.Resource != base {
		t.Errorf("resource = %q, want %q", doc.Resource, base)
	}
	if len(doc.AuthorizationServers) != 1 || doc.AuthorizationServers[0] != base {
		t.Errorf("authorization_servers = %v, want [%q]", doc.AuthorizationServers, base)
	}
}

// probe reports the authenticated person; the bearer middleware wraps it.
func probe(w http.ResponseWriter, r *http.Request) {
	person, clientType, ok := PersonFromContext(r.Context())
	if !ok {
		http.Error(w, "no identity", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(person + "|" + clientType))
}

func TestBearerMiddleware(t *testing.T) {
	iss := testIssuer(t)
	// nil device check: device credentials are refused, the pre-T-20 behaviour.
	mw := NewBearerMiddleware(iss, testAud, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	handler := mw(http.HandlerFunc(probe))

	serve := func(authHeader string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/mcp", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	// Valid access token → 200 + identity.
	access, err := iss.Mint("person-1", "claude.ai", time.Now().UTC())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if rec := serve("Bearer " + access); rec.Code != http.StatusOK || rec.Body.String() != "person-1|claude.ai" {
		t.Errorf("valid token: status=%d body=%q, want 200 person-1|claude.ai", rec.Code, rec.Body.String())
	}

	// No token → 401 with a WWW-Authenticate challenge pointing at the PRM.
	rec := serve("")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: status = %d, want 401", rec.Code)
	}
	if ch := rec.Header().Get("WWW-Authenticate"); !strings.Contains(ch, "resource_metadata=") {
		t.Errorf("challenge = %q, want a resource_metadata param", ch)
	}

	// Garbage token → 401 (not 500 — our error maps to auth.ErrInvalidToken),
	// and the body must NOT leak the internal validation reason (T-17): only
	// the generic sentinel message reaches the caller.
	rec = serve("Bearer not.a.jwt")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("garbage token: status = %d, want 401", rec.Code)
	}
	for _, leak := range []string{"malformed", "JSON", "decode", "oauth:", "signature", "expired"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("401 body leaks internal detail %q: %s", leak, rec.Body.String())
		}
	}

	// A DEVICE credential with no device check wired → 401. Accepting one is
	// opt-in (T-20): a relay that does not pass a check keeps use separation.
	device, _ := iss.MintDeviceCredential("person-1", "device-1", time.Now().UTC())
	if rec := serve("Bearer " + device); rec.Code != http.StatusUnauthorized {
		t.Errorf("device credential, no check wired: status = %d, want 401", rec.Code)
	}
}

// T-20: the daemon authenticates /mcp with its device credential. The device is
// re-checked on EVERY request, so revocation severs it immediately — the
// property the OAuth path cannot offer (access tokens live to expiry).
func TestBearerMiddlewareDeviceCredential(t *testing.T) {
	iss := testIssuer(t)
	var checked [][2]string
	var checkErr error
	mw := NewBearerMiddleware(iss, testAud, slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(person, device string) error {
			checked = append(checked, [2]string{person, device})
			return checkErr
		})
	handler := mw(http.HandlerFunc(probe))
	serve := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	device, err := iss.MintDeviceCredential("person-1", "device-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("MintDeviceCredential: %v", err)
	}

	// Active device → 200, stamped as the daemon profile (device credentials
	// carry no client_type claim of their own).
	if rec := serve(device); rec.Code != http.StatusOK || rec.Body.String() != "person-1|"+ClientTypeDaemon {
		t.Errorf("active device: status=%d body=%q, want 200 person-1|%s", rec.Code, rec.Body.String(), ClientTypeDaemon)
	}
	if want := [][2]string{{"person-1", "device-1"}}; len(checked) != 1 || checked[0] != want[0] {
		t.Errorf("store check got %v, want exactly one call with %v", checked, want)
	}

	// Revoked device (or a person/device mismatch — both surface as an error
	// from the relay's check) → 401, on the very next request.
	checkErr = errors.New("revoked")
	if rec := serve(device); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked device: status = %d, want 401", rec.Code)
	}
	if len(checked) != 2 {
		t.Errorf("store consulted %d times, want 2 — the check must run per request", len(checked))
	}

	// An ACCESS token still works through the same middleware, unchanged.
	checkErr = nil
	access, err := iss.Mint("person-2", "claude.ai", time.Now().UTC())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if rec := serve(access); rec.Code != http.StatusOK || rec.Body.String() != "person-2|claude.ai" {
		t.Errorf("access token: status=%d body=%q, want 200 person-2|claude.ai", rec.Code, rec.Body.String())
	}
	if len(checked) != 2 {
		t.Errorf("store consulted %d times; an access token must not hit the device check", len(checked))
	}
}
