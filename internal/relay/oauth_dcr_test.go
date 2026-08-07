package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func postJSON(s *Server, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return s.serve(req)
}

func TestRegisterHappyPath(t *testing.T) {
	s := testServer(t)
	rec := postJSON(s, "/oauth/register", `{"redirect_uris":["https://app.example.com/cb"],"client_name":"App"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if id, _ := resp["client_id"].(string); id == "" {
		t.Error("no client_id in registration response")
	}
	if resp["token_endpoint_auth_method"] != "none" {
		t.Errorf("auth method = %v, want none (public client)", resp["token_endpoint_auth_method"])
	}
}

func TestRegisterBadRedirect(t *testing.T) {
	s := testServer(t)
	for _, body := range []string{
		`{"redirect_uris":[]}`,
		`{"redirect_uris":["ftp://x/cb"]}`,
		`{"redirect_uris":["http://evil.example.com/cb"]}`, // http, non-loopback
		`{"redirect_uris":["https://app.example.com/cb#frag"]}`,
	} {
		if rec := postJSON(s, "/oauth/register", body); rec.Code != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, want 400", body, rec.Code)
		}
	}
}

func TestASMetadataAndJWKS(t *testing.T) {
	s := testServer(t)

	rec := s.serve(httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("metadata status = %d, want 200", rec.Code)
	}
	var meta map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if meta["issuer"] != "https://relay.example.com" {
		t.Errorf("issuer = %v, want the base URL", meta["issuer"])
	}

	rec = s.serve(httptest.NewRequest("GET", "/oauth/jwks", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("jwks status = %d, want 200", rec.Code)
	}
	var jwks struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &jwks); err != nil {
		t.Fatalf("decode jwks: %v", err)
	}
	if len(jwks.Keys) != 1 || jwks.Keys[0]["kty"] != "OKP" {
		t.Errorf("jwks = %+v, want one OKP key", jwks)
	}
}

// TestOAuthEndToEndThroughMux drives the whole flow through the routed mux:
// register → enroll → authorize → token, and verifies the issued access token.
func TestOAuthEndToEndThroughMux(t *testing.T) {
	s := testServer(t)
	const redirect = "https://app.example.com/cb"

	rec := postJSON(s, "/oauth/register", `{"redirect_uris":["`+redirect+`"],"client_name":"App"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d", rec.Code)
	}
	var reg map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &reg); err != nil {
		t.Fatalf("decode register: %v", err)
	}
	clientID, _ := reg["client_id"].(string)

	_, _, cred := enrolledCredential(t, s)

	form := authzQuery(clientID, redirect)
	form.Set("credential", cred)
	req := httptest.NewRequest("POST", "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = s.serve(req)
	if rec.Code != http.StatusFound {
		t.Fatalf("authorize status = %d; body=%s", rec.Code, rec.Body.String())
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("bad Location: %v", err)
	}
	code := loc.Query().Get("code")

	tf := codeForm(code, clientID, redirect, testVerifier)
	req = httptest.NewRequest("POST", "/oauth/token", strings.NewReader(tf.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = s.serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("token status = %d; body=%s", rec.Code, rec.Body.String())
	}
	tr := decodeToken(t, rec)
	if _, _, err := s.issuer.Verify(tr.AccessToken, time.Now().UTC()); err != nil {
		t.Errorf("issued access token does not verify: %v", err)
	}
}
