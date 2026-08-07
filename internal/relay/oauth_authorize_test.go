package relay

import (
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// RFC 7636 Appendix B PKCE challenge, reused across the authorize/token tests.
const testChallenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

// enrolledCredential enrolls a fresh device and returns a valid device credential.
func enrolledCredential(t *testing.T, s *Server) (personID, deviceID, cred string) {
	t.Helper()
	now := time.Now().UTC()
	tok, err := s.store.CreateInvite("marco@example.com", time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	p, d, err := s.store.Enroll(tok, pub, "laptop", now)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	cred, err = s.issuer.MintDeviceCredential(p.ID, d.ID, now)
	if err != nil {
		t.Fatalf("MintDeviceCredential: %v", err)
	}
	return p.ID, d.ID, cred
}

// dcrClient registers a DCR client with one redirect URI and returns its id.
func dcrClient(t *testing.T, s *Server, redirectURI string) string {
	t.Helper()
	id, err := s.store.RegisterClient([]string{redirectURI}, "Test", time.Now().UTC())
	if err != nil {
		t.Fatalf("RegisterClient: %v", err)
	}
	return id
}

// authzQuery builds a well-formed /authorize query for a DCR client.
func authzQuery(clientID, redirectURI string) url.Values {
	return url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"code_challenge":        {testChallenge},
		"code_challenge_method": {"S256"},
		"state":                 {"xyz"},
		"resource":              {"https://relay.example.com"},
	}
}

func TestAuthorizeGETRendersForm(t *testing.T) {
	s := testServer(t)
	const redirect = "https://client.example.com/cb"
	cid := dcrClient(t, s, redirect)

	req := httptest.NewRequest("GET", "/oauth/authorize?"+authzQuery(cid, redirect).Encode(), nil)
	rec := httptest.NewRecorder()
	s.handleAuthorize(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `name="credential"`) {
		t.Error("login form missing credential field")
	}
	if !strings.Contains(body, cid) {
		t.Error("hidden client_id not carried into the form")
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("CSP = %q, want a restrictive policy", csp)
	}
}

func TestAuthorizePOSTHappyPath(t *testing.T) {
	s := testServer(t)
	const redirect = "https://client.example.com/cb"
	cid := dcrClient(t, s, redirect)
	person, device, cred := enrolledCredential(t, s)

	form := authzQuery(cid, redirect)
	form.Set("credential", cred)
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAuthorizeSubmit(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body=%s", rec.Code, rec.Body.String())
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("bad Location: %v", err)
	}
	if loc.Scheme+"://"+loc.Host+loc.Path != redirect {
		t.Errorf("redirect target = %q, want %q", loc, redirect)
	}
	if loc.Query().Get("state") != "xyz" {
		t.Errorf("state = %q, want xyz", loc.Query().Get("state"))
	}
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatal("no code in redirect")
	}
	// The code carries the person/device/challenge binding.
	b, err := s.store.AuthCodeByCode(code, time.Now().UTC())
	if err != nil {
		t.Fatalf("AuthCodeByCode: %v", err)
	}
	if b.PersonID != person || b.DeviceID != device || b.CodeChallenge != testChallenge {
		t.Errorf("binding = %+v, want person %q device %q challenge %q", b, person, device, testChallenge)
	}
}

func TestAuthorizeBadRedirectIsInline(t *testing.T) {
	s := testServer(t)
	cid := dcrClient(t, s, "https://client.example.com/cb")

	// A redirect_uri not registered for this client → inline error, NO redirect.
	q := authzQuery(cid, "https://evil.example.com/cb")
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	s.handleAuthorize(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if rec.Header().Get("Location") != "" {
		t.Error("must NOT redirect to an unvalidated redirect_uri")
	}
}

func TestAuthorizePKCEDowngradeRejected(t *testing.T) {
	s := testServer(t)
	const redirect = "https://client.example.com/cb"
	cid := dcrClient(t, s, redirect)

	cases := []struct{ name, method, challenge string }{
		{"plain method", "plain", testChallenge},
		{"missing method", "", testChallenge},
		{"missing challenge", "S256", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := authzQuery(cid, redirect)
			q.Set("code_challenge_method", c.method)
			q.Set("code_challenge", c.challenge)
			req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
			rec := httptest.NewRecorder()
			s.handleAuthorize(rec, req)

			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302 (redirect error)", rec.Code)
			}
			loc, _ := url.Parse(rec.Header().Get("Location"))
			if loc.Query().Get("error") != "invalid_request" {
				t.Errorf("error = %q, want invalid_request", loc.Query().Get("error"))
			}
			if loc.Query().Get("state") != "xyz" {
				t.Error("state not echoed on error redirect")
			}
		})
	}
}

func TestAuthorizeWrongResponseType(t *testing.T) {
	s := testServer(t)
	const redirect = "https://client.example.com/cb"
	cid := dcrClient(t, s, redirect)

	q := authzQuery(cid, redirect)
	q.Set("response_type", "token")
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	s.handleAuthorize(rec, req)

	loc, _ := url.Parse(rec.Header().Get("Location"))
	if rec.Code != http.StatusFound || loc.Query().Get("error") != "unsupported_response_type" {
		t.Errorf("status=%d error=%q, want 302 unsupported_response_type", rec.Code, loc.Query().Get("error"))
	}
}

func TestAuthorizeBadCredentialReRenders(t *testing.T) {
	s := testServer(t)
	const redirect = "https://client.example.com/cb"
	cid := dcrClient(t, s, redirect)

	form := authzQuery(cid, redirect)
	form.Set("credential", "not-a-real-credential")
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAuthorizeSubmit(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (form re-render)", rec.Code)
	}
	if rec.Header().Get("Location") != "" {
		t.Error("bad credential must not redirect")
	}
	if !strings.Contains(rec.Body.String(), "not valid") {
		t.Error("re-rendered form missing the error banner")
	}
}

func TestAuthorizeRevokedDeviceRejected(t *testing.T) {
	s := testServer(t)
	const redirect = "https://client.example.com/cb"
	cid := dcrClient(t, s, redirect)
	_, device, cred := enrolledCredential(t, s)
	if err := s.store.RevokeDevice(device, time.Now().UTC()); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}

	form := authzQuery(cid, redirect)
	form.Set("credential", cred)
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAuthorizeSubmit(rec, req)

	if rec.Code != http.StatusOK || rec.Header().Get("Location") != "" {
		t.Errorf("revoked device: status=%d location=%q, want 200 re-render", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAuthorizeReflectedParamEscaped(t *testing.T) {
	s := testServer(t)
	const redirect = "https://client.example.com/cb"
	cid := dcrClient(t, s, redirect)

	q := authzQuery(cid, redirect)
	q.Set("state", `"><script>alert(1)</script>`)
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	s.handleAuthorize(rec, req)

	if strings.Contains(rec.Body.String(), "<script>alert(1)</script>") {
		t.Error("reflected state was not HTML-escaped (XSS)")
	}
}

func TestResolveClientCIMDSameOrigin(t *testing.T) {
	s := testServer(t)
	// CIMD: https-URL client_id, same-origin redirect → resolves.
	ct, ok, err := s.resolveClient("https://chatgpt.com/.well-known/oauth-client", "https://chatgpt.com/callback")
	if err != nil || !ok {
		t.Fatalf("same-origin CIMD: ok=%v err=%v, want ok", ok, err)
	}
	if ct != "chatgpt" {
		t.Errorf("client_type = %q, want chatgpt", ct)
	}
	// Cross-origin redirect → rejected.
	if _, ok, _ := s.resolveClient("https://chatgpt.com/.well-known/oauth-client", "https://evil.example.com/cb"); ok {
		t.Error("cross-origin CIMD redirect must be rejected")
	}
}

func TestClientTypeFor(t *testing.T) {
	cases := map[string]string{
		"claude.ai":          "claude.ai",
		"foo.claude.ai":      "claude.ai",
		"chatgpt.com":        "chatgpt",
		"api.openai.com":     "chatgpt",
		"client.example.com": "",
		"localhost:1234":     "",
	}
	for host, want := range cases {
		if got := clientTypeFor(host); got != want {
			t.Errorf("clientTypeFor(%q) = %q, want %q", host, got, want)
		}
	}
}
