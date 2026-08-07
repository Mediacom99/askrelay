package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// RFC 7636 Appendix B PKCE verifier matching testChallenge (authorize_test.go).
const testVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

func postToken(s *Server, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleToken(rec, req)
	return rec
}

// seedCode enrolls a device, registers a client, and stores a fresh auth code
// bound to testChallenge, returning the pieces a token exchange needs.
func seedCode(t *testing.T, s *Server) (code, clientID, redirect, person, device string) {
	t.Helper()
	now := time.Now().UTC()
	redirect = "https://client.example.com/cb"
	clientID = dcrClient(t, s, redirect)
	person, device, _ = enrolledCredential(t, s)
	code, err := newOpaqueToken()
	if err != nil {
		t.Fatalf("newOpaqueToken: %v", err)
	}
	err = s.store.CreateAuthCode(code, store.AuthCode{
		ClientID: clientID, RedirectURI: redirect, CodeChallenge: testChallenge,
		Resource: "https://relay.example.com", PersonID: person, DeviceID: device,
	}, now.Add(time.Minute), now)
	if err != nil {
		t.Fatalf("CreateAuthCode: %v", err)
	}
	return code, clientID, redirect, person, device
}

func codeForm(code, clientID, redirect, verifier string) url.Values {
	return url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"client_id":     {clientID},
		"redirect_uri":  {redirect},
	}
}

func decodeToken(t *testing.T, rec *httptest.ResponseRecorder) tokenResponse {
	t.Helper()
	var tr tokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &tr); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	return tr
}

func assertTokenError(t *testing.T, rec *httptest.ResponseRecorder, wantErr string) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != wantErr {
		t.Errorf("error = %q, want %q", body["error"], wantErr)
	}
}

func TestTokenCodeHappyPath(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, person, _ := seedCode(t, s)

	rec := postToken(s, codeForm(code, clientID, redirect, testVerifier))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	tr := decodeToken(t, rec)
	if tr.TokenType != "Bearer" || tr.ExpiresIn != 3600 || tr.RefreshToken == "" {
		t.Errorf("token response = %+v, want Bearer/3600/refresh", tr)
	}
	gotPerson, _, err := s.issuer.Verify(tr.AccessToken, time.Now().UTC())
	if err != nil || gotPerson != person {
		t.Errorf("access verify = (%q, %v), want %q", gotPerson, err, person)
	}
}

func TestTokenPKCEMismatch(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, _, _ := seedCode(t, s)
	assertTokenError(t, postToken(s, codeForm(code, clientID, redirect, "wrong-verifier")), "invalid_grant")
}

func TestTokenCodeReplay(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, _, _ := seedCode(t, s)
	if rec := postToken(s, codeForm(code, clientID, redirect, testVerifier)); rec.Code != http.StatusOK {
		t.Fatalf("first exchange status = %d, want 200", rec.Code)
	}
	assertTokenError(t, postToken(s, codeForm(code, clientID, redirect, testVerifier)), "invalid_grant")
}

func TestTokenClientMismatch(t *testing.T) {
	s := testServer(t)
	code, _, redirect, _, _ := seedCode(t, s)
	assertTokenError(t, postToken(s, codeForm(code, "some-other-client", redirect, testVerifier)), "invalid_grant")
}

func TestTokenDeviceRevoked(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, _, device := seedCode(t, s)
	if err := s.store.RevokeDevice(device, time.Now().UTC()); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}
	assertTokenError(t, postToken(s, codeForm(code, clientID, redirect, testVerifier)), "invalid_grant")
}

func TestTokenRefreshRotationAndReuse(t *testing.T) {
	s := testServer(t)
	code, clientID, redirect, person, _ := seedCode(t, s)
	first := decodeToken(t, postToken(s, codeForm(code, clientID, redirect, testVerifier)))

	refreshForm := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {first.RefreshToken}}
	rec := postToken(s, refreshForm)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	second := decodeToken(t, rec)
	if second.AccessToken == "" || second.RefreshToken == first.RefreshToken {
		t.Error("refresh must rotate the refresh token and mint a new access token")
	}
	gotPerson, _, err := s.issuer.Verify(second.AccessToken, time.Now().UTC())
	if err != nil || gotPerson != person {
		t.Errorf("refreshed access verify = (%q, %v), want %q", gotPerson, err, person)
	}
	// Reusing the now-consumed first refresh token → invalid_grant.
	assertTokenError(t, postToken(s, refreshForm), "invalid_grant")
}

func TestTokenUnsupportedGrant(t *testing.T) {
	s := testServer(t)
	assertTokenError(t, postToken(s, url.Values{"grant_type": {"client_credentials"}}), "unsupported_grant_type")
}
