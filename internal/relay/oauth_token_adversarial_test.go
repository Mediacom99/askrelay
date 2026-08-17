package relay

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ADVERSARIAL (askrelay-test, WP-06 quality pass): the concurrency race and
// cross-client-theft angles the WP-06 review calls out by name, PKCE beyond
// oauth_token_test.go's single mismatch case, the device-revoked-mid-flow
// obligation on the REFRESH path (only the code-exchange path is covered by
// oauth_token_test.go's TestTokenDeviceRevoked), multi-generation rotation,
// sibling-in-family death exercised through the real HTTP endpoints, and
// response-header discipline (Cache-Control: no-store).
// ---------------------------------------------------------------------------

// TestTokenCodeConcurrentDoubleExchangeExactlyOneWins fires N concurrent
// /token POSTs at the SAME code+verifier. Exactly one must succeed.
func TestTokenCodeConcurrentDoubleExchangeExactlyOneWins(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, _, _ := seedCode(t, s)
	form := codeForm(code, clientID, redirect, testVerifier)

	const n = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	oks, invalidGrants := 0, 0
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := postToken(s, form)
			mu.Lock()
			defer mu.Unlock()
			switch rec.Code {
			case http.StatusOK:
				oks++
			case http.StatusBadRequest:
				invalidGrants++
			default:
				t.Errorf("unexpected status %d", rec.Code)
			}
		}()
	}
	wg.Wait()
	if oks != 1 || invalidGrants != n-1 {
		t.Errorf("concurrent code exchange: oks=%d invalid=%d, want exactly 1 ok and %d invalid_grant", oks, invalidGrants, n-1)
	}
}

// TestTokenMismatchedAttemptBurnsCodeForLegitimateClient: oauth_token.go's
// tokenFromCode calls store.ConsumeAuthCode (which marks the code used)
// BEFORE checking that client_id/redirect_uri/PKCE match the code's binding.
// A single /token POST bearing the right CODE but a wrong client_id -- no
// proof of possession of the verifier or the real client's identity, exactly
// the situation PKCE exists to make harmless -- should fail WITHOUT
// consuming the code, so the legitimate client can still redeem it
// afterward. This test demonstrates that it does NOT: the wrong-client probe
// burns the code, and the legitimate client's fully-correct follow-up
// exchange then also fails.
func TestTokenMismatchedAttemptBurnsCodeForLegitimateClient(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, _, _ := seedCode(t, s)

	// An attacker (or a MITM that only saw the code fly by, never the
	// verifier) probes /token with a wrong client_id.
	assertTokenError(t, postToken(s, codeForm(code, "attacker-client", redirect, testVerifier)), "invalid_grant")

	// The legitimate client, with the CORRECT client_id/redirect_uri/verifier,
	// must still be able to redeem the code it was actually issued.
	rec := postToken(s, codeForm(code, clientID, redirect, testVerifier))
	if rec.Code != http.StatusOK {
		t.Errorf("legitimate exchange after an attacker's wrong-client probe: status=%d body=%s, want 200 OK -- "+
			"the code was burned by the FAILED probe because ConsumeAuthCode runs before the client_id/redirect_uri/PKCE "+
			"binding check in tokenFromCode (oauth_token.go), so anyone who merely sees a code (no verifier needed) can "+
			"deny the legitimate client its token",
			rec.Code, rec.Body.String())
	}
}

// TestTokenCrossClientAndRedirectMismatchRejected: independent fresh codes
// per case (a code is single-attempt, see the finding above) -- each
// dimension of the binding (client_id, redirect_uri) checked in isolation.
func TestTokenCrossClientAndRedirectMismatchRejected(t *testing.T) {
	s := testServer(t)
	otherClientID := dcrClient(t, s, "https://attacker.example.com/cb")

	cases := []struct {
		name             string
		useOtherClient   bool
		useOtherRedirect bool
	}{
		{"right client, wrong redirect", false, true},
		{"wrong client, right redirect", true, false},
		{"wrong client, wrong redirect", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, clientID, redirect, _, _ := seedCode(t, s) // fresh code per case
			cid, red := clientID, redirect
			if c.useOtherClient {
				cid = otherClientID
			}
			if c.useOtherRedirect {
				red = "https://attacker.example.com/cb"
			}
			assertTokenError(t, postToken(s, codeForm(code, cid, red, testVerifier)), "invalid_grant")
		})
	}
}

// TestTokenCIMDRedirectPathSwapRejected: both callbacks are listed in the CIMD
// document, so either is a valid redirect at /authorize — but the code is bound
// to the EXACT redirect_uri string used there, so the other listed callback must
// not redeem it.
func TestTokenCIMDRedirectPathSwapRejected(t *testing.T) {
	s := testServer(t)
	const clientID = "https://claude.ai/oauth/claude-code-client-metadata"
	const redirectA = "http://localhost/callback-a"
	const redirectB = "http://localhost/callback-b"
	s.fetchCIMD = func(context.Context, string) (*clientMetadata, error) {
		return &clientMetadata{ClientID: clientID, RedirectURIs: []string{redirectA, redirectB}}, nil
	}
	_, _, cred := enrolledCredential(t, s)

	form := authzQuery(clientID, redirectA)
	form.Set("credential", cred)
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAuthorizeSubmit(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("authorize status = %d, want 302; body=%s", rec.Code, rec.Body.String())
	}
	loc, _ := url.Parse(rec.Header().Get("Location"))
	code := loc.Query().Get("code")

	assertTokenError(t, postToken(s, codeForm(code, clientID, redirectB, testVerifier)), "invalid_grant")
}

// TestTokenPKCEPlainConfusionRejected: presenting the CHALLENGE itself as the
// verifier (as if S256 had never been applied) must never match.
func TestTokenPKCEPlainConfusionRejected(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, _, _ := seedCode(t, s)
	assertTokenError(t, postToken(s, codeForm(code, clientID, redirect, testChallenge)), "invalid_grant")
}

// TestTokenPKCEVerifierExtremesNoPanic: length/encoding extremes on
// code_verifier, a value fully attacker-controlled in the /token POST body.
// None should match (a fresh code + testChallenge each time); the point is
// no panic and a clean invalid_grant regardless of shape. Kept comfortably
// under maxTokenBody (8KiB) so the failure mode under test is the PKCE
// comparison, not the body-size guard (see the dedicated oversized-verifier
// test below for that boundary).
func TestTokenPKCEVerifierExtremesNoPanic(t *testing.T) {
	verifiers := []string{
		"a",
		strings.Repeat("v", 2_000), // far beyond RFC 7636's 128-char max, still << maxTokenBody
		"日本語🔥",
		"\x00\x01\x02",
		strings.Repeat("=", 50),
	}
	for i, v := range verifiers {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			s := testServer(t)
			code, clientID, redirect, _, _ := seedCode(t, s)
			assertTokenError(t, postToken(s, codeForm(code, clientID, redirect, v)), "invalid_grant")
		})
	}
}

// TestTokenVerifierExceedingBodyLimitIsInvalidRequestNotPanic: a verifier
// large enough to push the whole form past maxTokenBody trips the
// MaxBytesReader guard during ParseForm, landing on invalid_request (not
// invalid_grant) -- still a clean 400, never a panic or a 500. Documents the
// boundary rather than asserting a specific error family.
func TestTokenVerifierExceedingBodyLimitIsInvalidRequestNotPanic(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, _, _ := seedCode(t, s)
	rec := postToken(s, codeForm(code, clientID, redirect, strings.Repeat("v", 10_000)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// TestTokenCodeGrantMissingParamsInvalidRequest: each required param missing
// in isolation -> invalid_request (distinct from invalid_grant), and
// crucially the code is NOT consumed by these (checked before ConsumeAuthCode
// in tokenFromCode), so reusing the same code across subtests is safe.
func TestTokenCodeGrantMissingParamsInvalidRequest(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, _, _ := seedCode(t, s)

	for _, field := range []string{"code", "code_verifier", "client_id", "redirect_uri"} {
		t.Run("missing "+field, func(t *testing.T) {
			f := codeForm(code, clientID, redirect, testVerifier)
			f.Del(field)
			assertTokenError(t, postToken(s, f), "invalid_request")
		})
	}
}

// TestTokenResponseCacheControlNoStore: both the success and error response
// bodies must carry Cache-Control: no-store (tokens/errors are never cached).
func TestTokenResponseCacheControlNoStore(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, _, _ := seedCode(t, s)

	rec := postToken(s, codeForm(code, clientID, redirect, testVerifier))
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("success Cache-Control = %q, want no-store", cc)
	}
	// Replay of the now-consumed code is the error path.
	rec2 := postToken(s, codeForm(code, clientID, redirect, testVerifier))
	if cc := rec2.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("error Cache-Control = %q, want no-store", cc)
	}
}

// TestTokenRefreshFailsAfterDeviceRevokedPostExchange: the WP-06 security
// obligation on the REFRESH path specifically -- device revoked AFTER a
// successful code exchange (not before, which oauth_token_test.go's
// TestTokenDeviceRevoked already covers) must still block a subsequent
// refresh.
func TestTokenRefreshFailsAfterDeviceRevokedPostExchange(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, _, device := seedCode(t, s)
	first := decodeToken(t, postToken(s, codeForm(code, clientID, redirect, testVerifier)))
	if first.RefreshToken == "" {
		t.Fatal("no refresh token issued")
	}

	if err := s.store.RevokeDevice(device, time.Now().UTC()); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}

	refreshForm := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {first.RefreshToken}}
	assertTokenError(t, postToken(s, refreshForm), "invalid_grant")
}

// TestTokenRefreshSiblingDiesOnReuseAtHTTPLevel extends the store-level
// TestRefreshRotateAndReuseRevokesFamily through the REAL HTTP endpoints:
// two independent authorize+code-exchange rounds for the same person+client
// produce two refresh tokens in one family; rotating and then replaying one
// must kill the other, never-used sibling too.
func TestTokenRefreshSiblingDiesOnReuseAtHTTPLevel(t *testing.T) {
	s := testServer(t)
	const redirect = "https://client.example.com/cb"
	clientID := dcrClient(t, s, redirect)
	_, _, cred := enrolledCredential(t, s)

	authorizeOnce := func() string {
		form := authzQuery(clientID, redirect)
		form.Set("credential", cred)
		req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		s.handleAuthorizeSubmit(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("authorize status = %d, want 302; body=%s", rec.Code, rec.Body.String())
		}
		loc, _ := url.Parse(rec.Header().Get("Location"))
		return loc.Query().Get("code")
	}

	codeA := authorizeOnce()
	tokA := decodeToken(t, postToken(s, codeForm(codeA, clientID, redirect, testVerifier)))
	codeB := authorizeOnce()
	tokB := decodeToken(t, postToken(s, codeForm(codeB, clientID, redirect, testVerifier)))
	if tokA.RefreshToken == "" || tokB.RefreshToken == "" || tokA.RefreshToken == tokB.RefreshToken {
		t.Fatalf("expected two distinct refresh tokens, got %q and %q", tokA.RefreshToken, tokB.RefreshToken)
	}

	refreshFormA := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tokA.RefreshToken}}
	if rec := postToken(s, refreshFormA); rec.Code != http.StatusOK {
		t.Fatalf("rotate A: status = %d; body=%s", rec.Code, rec.Body.String())
	}
	// Replay the now-superseded A -> reuse signal, family (person x client) revoked.
	assertTokenError(t, postToken(s, refreshFormA), "invalid_grant")

	// B was never used and is a sibling in the same family -- it must be dead too.
	refreshFormB := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tokB.RefreshToken}}
	assertTokenError(t, postToken(s, refreshFormB), "invalid_grant")
}

// TestTokenRefreshRotationAcrossManyGenerations: N=6 rotations. Each new
// access token verifies for the same person; every superseded generation is
// dead afterward.
func TestTokenRefreshRotationAcrossManyGenerations(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, person, _ := seedCode(t, s)
	tok := decodeToken(t, postToken(s, codeForm(code, clientID, redirect, testVerifier)))

	var used []string
	const rounds = 6
	for i := range rounds {
		used = append(used, tok.RefreshToken)
		form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}}
		rec := postToken(s, form)
		if rec.Code != http.StatusOK {
			t.Fatalf("round %d: status = %d, body=%s", i, rec.Code, rec.Body.String())
		}
		next := decodeToken(t, rec)
		if next.RefreshToken == tok.RefreshToken {
			t.Fatalf("round %d: refresh token did not rotate", i)
		}
		gotPerson, _, err := s.issuer.Verify(next.AccessToken, time.Now().UTC())
		if err != nil || gotPerson != person {
			t.Fatalf("round %d: access token invalid: person=%q err=%v", i, gotPerson, err)
		}
		tok = next
	}
	for i, rt := range used {
		form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {rt}}
		if rec := postToken(s, form); rec.Code == http.StatusOK {
			t.Errorf("generation %d refresh token still usable after %d rotations", i, rounds)
		}
	}
}

// TestTokenMalformedContentType: a non-form body still resolves to a clean
// 400 (empty PostForm -> empty grant_type -> unsupported_grant_type), not a
// crash or a 500.
func TestTokenMalformedContentType(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(`{"grant_type":"authorization_code"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleToken(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// TestTokenGrantTypeMissingOrEmpty: no grant_type at all, and an
// explicitly-empty one, both land on unsupported_grant_type.
func TestTokenGrantTypeMissingOrEmpty(t *testing.T) {
	s := testServer(t)
	assertTokenError(t, postToken(s, url.Values{}), "unsupported_grant_type")
	assertTokenError(t, postToken(s, url.Values{"grant_type": {""}}), "unsupported_grant_type")
}

// TestTokenBodyOverLimitRejected: the maxTokenBody MaxBytesReader guard.
func TestTokenBodyOverLimitRejected(t *testing.T) {
	s := testServer(t)
	huge := strings.Repeat("a", maxTokenBody+1024)
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader("grant_type=authorization_code&code="+huge))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleToken(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("oversized body: status = %d, want 400", rec.Code)
	}
}
