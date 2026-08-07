package relay

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ADVERSARIAL (askrelay-test, WP-06 quality pass): RFC 6749 §4.1.2.1 error
// discipline on paths oauth_authorize_test.go doesn't reach, state edge
// cases (absence, CRLF/header-injection attempts), reflected-param escaping
// beyond `state` (client_id, redirect_uri -- including a DCR-registered
// redirect_uri that itself carries a script tag), CIMD same-origin bypass
// variants (port/subdomain/scheme/userinfo/case), DCR exact-match robustness,
// and body-size enforcement on the POST endpoint.
// ---------------------------------------------------------------------------

func TestAuthorizeMissingParamsAlwaysInline(t *testing.T) {
	s := testServer(t)
	cid := dcrClient(t, s, "https://client.example.com/cb")
	cases := []struct{ name, clientID, redirect string }{
		{"missing client_id", "", "https://client.example.com/cb"},
		{"missing redirect_uri", cid, ""},
		{"both missing", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := authzQuery(c.clientID, c.redirect)
			req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
			rec := httptest.NewRecorder()
			s.handleAuthorize(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if rec.Header().Get("Location") != "" {
				t.Error("must not redirect when client_id/redirect_uri themselves are untrusted")
			}
		})
	}
}

// TestAuthorizeUnregisteredClientIsInline: a well-formed but never-registered
// client_id must be an inline 400, not a redirect (redirect_uri is not
// trusted until a KNOWN client vouches for it).
func TestAuthorizeUnregisteredClientIsInline(t *testing.T) {
	s := testServer(t)
	q := authzQuery("00000000-0000-0000-0000-000000000000", "https://client.example.com/cb")
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	s.handleAuthorize(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if rec.Header().Get("Location") != "" {
		t.Error("must not redirect for an unregistered client_id")
	}
}

// TestAuthorizeResourceMismatchRedirectsWithError: unlike a bad client/
// redirect pairing, a bad `resource` happens AFTER redirect_uri is trusted,
// so it must redirect with error+state (not go inline). Not covered by the
// implementer's PKCE-downgrade/response-type redirect tests.
func TestAuthorizeResourceMismatchRedirectsWithError(t *testing.T) {
	s := testServer(t)
	const redirect = "https://client.example.com/cb"
	cid := dcrClient(t, s, redirect)

	q := authzQuery(cid, redirect)
	q.Set("resource", "https://not-this-relay.example.com")
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	s.handleAuthorize(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("bad Location: %v", err)
	}
	if loc.Query().Get("error") != "invalid_target" {
		t.Errorf("error = %q, want invalid_target", loc.Query().Get("error"))
	}
	if loc.Query().Get("state") != "xyz" {
		t.Error("state not echoed on the resource-mismatch error redirect")
	}
}

// TestAuthorizeStateNotEchoedWhenAbsent: the success redirect must not add a
// `state` key at all when the client never sent one.
func TestAuthorizeStateNotEchoedWhenAbsent(t *testing.T) {
	s := testServer(t)
	const redirect = "https://client.example.com/cb"
	cid := dcrClient(t, s, redirect)
	_, _, cred := enrolledCredential(t, s)

	form := authzQuery(cid, redirect)
	form.Del("state")
	form.Set("credential", cred)
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAuthorizeSubmit(rec, req)

	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("bad Location: %v", err)
	}
	if loc.Query().Has("state") {
		t.Errorf("state param present (%q) though the client never sent one", loc.Query().Get("state"))
	}
}

// TestAuthorizeStateCRLFNotInjected: a state value carrying CRLF must never
// reach the raw Location header unescaped (HTTP response splitting).
// url.Values.Encode() percent-encodes it; this pins that against a future
// change (e.g. building Location by string concatenation).
func TestAuthorizeStateCRLFNotInjected(t *testing.T) {
	s := testServer(t)
	const redirect = "https://client.example.com/cb"
	cid := dcrClient(t, s, redirect)

	q := authzQuery(cid, redirect)
	q.Set("state", "xyz\r\nSet-Cookie: evil=1\r\nX-Injected: true")
	q.Set("code_challenge_method", "plain") // force the cheapest redirect-error path
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	s.handleAuthorize(rec, req)

	loc := rec.Header().Get("Location")
	if strings.ContainsAny(loc, "\r\n") {
		t.Errorf("Location header contains a raw CR/LF: %q", loc)
	}
	if rec.Header().Get("Set-Cookie") != "" || rec.Header().Get("X-Injected") != "" {
		t.Error("attacker-controlled state injected extra response headers")
	}
}

// TestAuthorizeReflectedRedirectURIEscaped: DCR only validates redirect_uri's
// scheme/host/fragment (oauth_dcr.go validRedirectURI) -- the PATH is
// untrusted display text once the login page reflects it as a hidden field.
// A registered redirect_uri carrying a script tag in its path must still come
// out escaped.
func TestAuthorizeReflectedRedirectURIEscaped(t *testing.T) {
	s := testServer(t)
	evilRedirect := `https://client.example.com/cb"><script>alert(1)</script>`
	cid := dcrClient(t, s, evilRedirect)

	q := authzQuery(cid, evilRedirect)
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	s.handleAuthorize(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "<script>alert(1)</script>") {
		t.Error("redirect_uri reflected unescaped into the login page (XSS)")
	}
}

// TestAuthorizeReflectedCIMDClientIDEscaped: for CIMD, client_id is a fully
// attacker-controlled https URL (no registration, no server-chosen id). Same
// XSS check as above, on the field an attacker actually controls end to end.
func TestAuthorizeReflectedCIMDClientIDEscaped(t *testing.T) {
	s := testServer(t)
	evil := `https://evil.example.com/"><script>alert(1)</script>`

	q := authzQuery(evil, evil) // same-origin (identical string) satisfies the CIMD check
	req := httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	s.handleAuthorize(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "<script>alert(1)</script>") {
		t.Error("CIMD client_id reflected unescaped into the login page (XSS)")
	}
}

// TestResolveClientCIMDCrossOriginVariants: the same-origin CIMD check
// (oauth_authorize.go resolveClient) exercised against the specific bypass
// shapes called out in the WP-06 review -- port, subdomain, scheme downgrade,
// userinfo trick, and host case mismatch. All must fail closed.
func TestResolveClientCIMDCrossOriginVariants(t *testing.T) {
	s := testServer(t)
	const clientID = "https://chatgpt.com/.well-known/oauth-client"
	cases := []struct {
		name, redirect string
		wantOK         bool
	}{
		{"exact same origin", "https://chatgpt.com/callback", true},
		{"different path, same origin (by design)", "https://chatgpt.com/other/callback", true},
		{"port added", "https://chatgpt.com:8443/callback", false},
		{"subdomain", "https://evil.chatgpt.com/callback", false},
		{"scheme downgrade to http", "http://chatgpt.com/callback", false},
		{"userinfo prefix does not fool Host parsing", "https://chatgpt.com@evil.com/callback", false},
		{"case-different host", "https://ChatGPT.com/callback", false},
		{"suffix typosquat", "https://chatgpt.com.evil.com/callback", false},
		{"trailing-dot host", "https://chatgpt.com./callback", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ok, err := s.resolveClient(clientID, c.redirect)
			if err != nil {
				t.Fatalf("resolveClient error: %v", err)
			}
			if ok != c.wantOK {
				t.Errorf("resolveClient(%q, %q) ok = %v, want %v", clientID, c.redirect, ok, c.wantOK)
			}
		})
	}
}

// TestAuthorizeDCRRedirectExactMatchOnly: a DCR client registered for exactly
// one redirect_uri must not resolve for near-miss variants (trailing slash,
// query string, sibling path, subdomain, suffix typosquat, scheme case).
func TestAuthorizeDCRRedirectExactMatchOnly(t *testing.T) {
	s := testServer(t)
	const registered = "https://client.example.com/cb"
	cid := dcrClient(t, s, registered)

	attempts := []string{
		registered + "/",
		registered + "?extra=1",
		"https://client.example.com/cb2",
		"https://sub.client.example.com/cb",
		"https://client.example.com.evil.com/cb",
		"HTTPS://client.example.com/cb",
	}
	for _, redirect := range attempts {
		t.Run(redirect, func(t *testing.T) {
			_, ok, err := s.resolveClient(cid, redirect)
			if err != nil {
				t.Fatalf("resolveClient error: %v", err)
			}
			if ok {
				t.Errorf("redirect_uri %q accepted for a client only registered for %q", redirect, registered)
			}
		})
	}
}

// TestAuthorizeSubmitBodyOverLimitRejected: the maxAuthzBody MaxBytesReader
// guard on the POST endpoint.
func TestAuthorizeSubmitBodyOverLimitRejected(t *testing.T) {
	s := testServer(t)
	huge := strings.Repeat("a", maxAuthzBody+1024)
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader("credential="+huge))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAuthorizeSubmit(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("oversized body: status = %d, want 400", rec.Code)
	}
}
