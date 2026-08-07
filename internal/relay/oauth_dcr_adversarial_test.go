package relay

import (
	"net/http"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ADVERSARIAL (askrelay-test, WP-06 quality pass): malformed/type-confused
// JSON on the unauthenticated DCR endpoint, dangerous redirect_uri schemes
// beyond oauth_dcr_test.go's table, the forced token_endpoint_auth_method
// override, and body-size discipline (handleRegister uses io.LimitReader,
// not the http.MaxBytesReader every other endpoint here uses).
// ---------------------------------------------------------------------------

func TestRegisterTypeConfusionAndMalformedJSON(t *testing.T) {
	s := testServer(t)
	bodies := []string{
		`{"redirect_uris":"https://a.example.com/cb"}`, // string, not array
		`{"redirect_uris":[123]}`,                      // number in array
		`{"redirect_uris":[null]}`,                     // null element -> ""
		`{"redirect_uris":[["nested"]]}`,               // nested array element
		`null`,                                         // JSON null body
		`[]`,                                           // top-level array
		`"just a string"`,                              // top-level string
		`42`,                                           // top-level number
		`{"redirect_uris":["https://a.example.com/cb"],`, // truncated JSON
		``, // empty body
	}
	for _, body := range bodies {
		t.Run(body, func(t *testing.T) {
			rec := postJSON(s, "/oauth/register", body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("body %q: status = %d, want 400; resp=%s", body, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestRegisterAlwaysOverridesAuthMethodToNone: an attacker requesting
// client_secret_basic (hoping to later authenticate with a secret this AS
// never issues) is forced back to "none" regardless.
func TestRegisterAlwaysOverridesAuthMethodToNone(t *testing.T) {
	s := testServer(t)
	rec := postJSON(s, "/oauth/register",
		`{"redirect_uris":["https://app.example.com/cb"],"token_endpoint_auth_method":"client_secret_basic"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"token_endpoint_auth_method":"none"`) {
		t.Errorf("body = %s, want token_endpoint_auth_method forced to none regardless of what was requested", rec.Body.String())
	}
}

func TestRegisterDangerousSchemesRejected(t *testing.T) {
	s := testServer(t)
	for _, body := range []string{
		`{"redirect_uris":["javascript:alert(1)"]}`,
		`{"redirect_uris":["data:text/html,<script>alert(1)</script>"]}`,
		`{"redirect_uris":["https:evil.com"]}`,                            // opaque form, empty Host
		`{"redirect_uris":["http://192.168.1.1/cb"]}`,                     // http, non-loopback
		`{"redirect_uris":["https://good.com/cb","javascript:alert(1)"]}`, // one good, one bad
	} {
		t.Run(body, func(t *testing.T) {
			if rec := postJSON(s, "/oauth/register", body); rec.Code != http.StatusBadRequest {
				t.Errorf("body %q: status = %d, want 400", body, rec.Code)
			}
		})
	}
}

// TestRegisterBodyOverLimitStillRejected: a body far larger than
// maxRegisterBody is still refused (via a truncated/malformed JSON parse) --
// the observable outcome is still correct even with the io.LimitReader gap
// noted below.
func TestRegisterBodyOverLimitStillRejected(t *testing.T) {
	s := testServer(t)
	huge := `{"redirect_uris":["https://a.example.com/` + strings.Repeat("a", maxRegisterBody+1024) + `"]}`
	rec := postJSON(s, "/oauth/register", huge)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("oversized DCR body: status = %d, want 400 (body=%d bytes)", rec.Code, len(huge))
	}
}

// TestRegisterBodySizeCap characterizes the fixed body cap: handleRegister now
// wraps r.Body in http.MaxBytesReader(maxRegisterBody), consistent with every
// other body-limited endpoint (/authorize, /token, /enroll, /mcp). Two shapes:
//   - A small valid JSON prefix followed by huge trailing garbage still
//     succeeds — json.Decode stops after the first complete value and never
//     reads the trailing bytes, so the cap is not tripped (the decoded document
//     is bounded; this is the same behavior all json.Decode endpoints share).
//   - A single JSON value larger than the cap is rejected — MaxBytesReader trips
//     mid-value and Decode fails, unlike the old io.LimitReader which silently
//     truncated.
func TestRegisterBodySizeCap(t *testing.T) {
	s := testServer(t)

	trailing := `{"redirect_uris":["https://app.example.com/cb"]}` + strings.Repeat(" ", 10*maxRegisterBody)
	if rec := postJSON(s, "/oauth/register", trailing); rec.Code != http.StatusCreated {
		t.Errorf("small-prefix + trailing garbage: status = %d, want 201 (Decode stops at first value)", rec.Code)
	}

	oversized := `{"client_name":"` + strings.Repeat("A", 10*maxRegisterBody) + `","redirect_uris":["https://app.example.com/cb"]}`
	if rec := postJSON(s, "/oauth/register", oversized); rec.Code != http.StatusBadRequest {
		t.Errorf("oversized leading value: status = %d, want 400 (MaxBytesReader caps the read)", rec.Code)
	}
}
