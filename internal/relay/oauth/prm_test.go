package oauth

import (
	"encoding/json"
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
	mw := NewBearerMiddleware(iss, testAud)
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

	// Garbage token → 401 (not 500 — our error maps to auth.ErrInvalidToken).
	if rec := serve("Bearer not.a.jwt"); rec.Code != http.StatusUnauthorized {
		t.Errorf("garbage token: status = %d, want 401", rec.Code)
	}

	// A DEVICE credential presented as a bearer token → 401 (use separation).
	device, _ := iss.MintDeviceCredential("person-1", "device-1", time.Now().UTC())
	if rec := serve("Bearer " + device); rec.Code != http.StatusUnauthorized {
		t.Errorf("device credential as bearer: status = %d, want 401", rec.Code)
	}
}
