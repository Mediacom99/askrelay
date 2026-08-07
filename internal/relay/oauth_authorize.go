package relay

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/Mediacom99/askrelay/internal/relay/oauth"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// codeTTL bounds an authorization code's life: it is exchanged immediately, so
// a minute is generous (OAuth 2.1 recommends ≤ ~1 min).
const codeTTL = 60 * time.Second

// maxAuthzBody bounds the POST form (hidden OAuth params + one pasted credential).
const maxAuthzBody = 8 << 10

// authzParams is a validated /authorize request. ClientType is derived from the
// (already-validated) client host and drives the §5.4 profile on the token.
type authzParams struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	State               string
	Resource            string
	Scope               string
	ClientType          string
}

// authzReject is a parse/validation failure. A non-empty redirectURI means the
// redirect target is trusted and the error goes back there (RFC 6749 §4.1.2.1);
// otherwise it is rendered inline (client_id / redirect_uri itself was bad).
type authzReject struct {
	redirectURI string
	oauthErr    string
	desc        string
	status      int
	msg         string
}

// handleAuthorize (GET) validates the request and renders the login page.
func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	p, rej := s.parseAuthz(r.URL.Query())
	if rej != nil {
		s.rejectAuthz(w, r, rej, r.URL.Query().Get("state"))
		return
	}
	s.renderLogin(w, p, "")
}

// handleAuthorizeSubmit (POST) re-validates, verifies the pasted device
// credential, mints a single-use PKCE-bound code, and redirects back.
func (s *Server) handleAuthorizeSubmit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthzBody)
	if err := r.ParseForm(); err != nil {
		s.httpError(w, http.StatusBadRequest, "invalid form")
		return
	}
	p, rej := s.parseAuthz(r.PostForm)
	if rej != nil {
		s.rejectAuthz(w, r, rej, r.PostForm.Get("state"))
		return
	}

	now := time.Now().UTC()
	person, device, err := s.issuer.VerifyDeviceCredential(r.PostForm.Get("credential"), now)
	if err != nil {
		s.log.Warn("authorize refused", "reason", "bad_credential")
		s.renderLogin(w, p, "That device credential is not valid. Paste a current one and try again.")
		return
	}
	if _, err := s.store.ActiveDeviceByID(device); err != nil {
		// Revoked or unknown device: same opaque refusal, no oracle (T-17).
		s.log.Warn("authorize refused", "reason", "device_inactive")
		s.renderLogin(w, p, "That device credential is not valid. Paste a current one and try again.")
		return
	}

	code, err := newOpaqueToken()
	if err != nil {
		s.log.Error("authorize: mint code", "err", err)
		s.httpError(w, http.StatusInternalServerError, "authorization failed")
		return
	}
	err = s.store.CreateAuthCode(code, store.AuthCode{
		ClientID:      p.ClientID,
		RedirectURI:   p.RedirectURI,
		CodeChallenge: p.CodeChallenge,
		Resource:      p.Resource,
		PersonID:      person,
		DeviceID:      device,
		ClientType:    p.ClientType,
	}, now.Add(codeTTL), now)
	if err != nil {
		s.log.Error("authorize: store code", "err", err)
		s.httpError(w, http.StatusInternalServerError, "authorization failed")
		return
	}

	s.log.Info("authorization granted", "person", person, "device", device)
	u, _ := url.Parse(p.RedirectURI) // parseable: validated in resolveClient
	q := u.Query()
	q.Set("code", code)
	if p.State != "" {
		q.Set("state", p.State)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// parseAuthz validates an /authorize request from v (query or POST form).
// client_id + redirect_uri are validated FIRST; only after redirect_uri is
// trusted are the remaining failures made redirectable.
func (s *Server) parseAuthz(v url.Values) (authzParams, *authzReject) {
	p := authzParams{
		ResponseType:        v.Get("response_type"),
		ClientID:            v.Get("client_id"),
		RedirectURI:         v.Get("redirect_uri"),
		CodeChallenge:       v.Get("code_challenge"),
		CodeChallengeMethod: v.Get("code_challenge_method"),
		State:               v.Get("state"),
		Resource:            v.Get("resource"),
		Scope:               v.Get("scope"),
	}
	if p.ClientID == "" || p.RedirectURI == "" {
		return authzParams{}, &authzReject{status: http.StatusBadRequest, msg: "missing client_id or redirect_uri"}
	}
	clientType, ok, err := s.resolveClient(p.ClientID, p.RedirectURI)
	if err != nil {
		s.log.Error("authorize: resolve client", "err", err)
		return authzParams{}, &authzReject{status: http.StatusInternalServerError, msg: "authorization failed"}
	}
	if !ok {
		// redirect_uri is NOT trusted → inline, never redirect.
		s.log.Warn("authorize refused", "reason", "bad_client_or_redirect")
		return authzParams{}, &authzReject{status: http.StatusBadRequest, msg: "invalid client_id or redirect_uri"}
	}
	p.ClientType = clientType

	// From here redirect_uri is trusted, so protocol errors go back to it.
	if p.ResponseType != "code" {
		return authzParams{}, &authzReject{redirectURI: p.RedirectURI, oauthErr: "unsupported_response_type", desc: "only response_type=code is supported"}
	}
	if p.CodeChallenge == "" || p.CodeChallengeMethod != "S256" {
		return authzParams{}, &authzReject{redirectURI: p.RedirectURI, oauthErr: "invalid_request", desc: "PKCE with code_challenge_method=S256 is required"}
	}
	if p.Resource != "" && p.Resource != s.cfg.BaseURL {
		return authzParams{}, &authzReject{redirectURI: p.RedirectURI, oauthErr: "invalid_target", desc: "resource does not match this relay"}
	}
	return p, nil
}

// resolveClient validates the client_id ↔ redirect_uri pairing and returns the
// derived client_type. A CIMD client_id (an https URL) requires a same-origin
// redirect (no remote fetch — SSRF-safe v1; the metadata-document fetch is a
// later, S-04-gated upgrade). A DCR client_id must exist and list redirect_uri
// exactly. ok=false is any invalid pairing; err is only an infrastructure fault.
func (s *Server) resolveClient(clientID, redirectURI string) (clientType string, ok bool, err error) {
	ru, perr := url.Parse(redirectURI)
	if perr != nil || !ru.IsAbs() {
		return "", false, nil
	}
	if cu, cerr := url.Parse(clientID); cerr == nil && cu.Scheme == "https" && cu.Host != "" {
		// CIMD: same scheme+host binds the redirect to the client's own origin.
		if ru.Scheme == cu.Scheme && ru.Host == cu.Host {
			return clientTypeFor(ru.Host), true, nil
		}
		return "", false, nil
	}
	c, cerr := s.store.ClientByID(clientID)
	if errors.Is(cerr, store.ErrClientUnknown) {
		return "", false, nil
	}
	if cerr != nil {
		return "", false, fmt.Errorf("relay: resolve client: %w", cerr)
	}
	if slices.Contains(c.RedirectURIs, redirectURI) {
		return clientTypeFor(ru.Host), true, nil
	}
	return "", false, nil
}

// clientTypeFor maps a validated client host to a §5.4 profile string. Only
// claude.ai gets a longer wait cap; everything else (chatgpt, unknown) falls to
// the safe 45s default in mcp.profileCap — a wrong guess can only shorten a wait.
func clientTypeFor(host string) string {
	h := strings.ToLower(host)
	switch {
	case h == "claude.ai" || strings.HasSuffix(h, ".claude.ai"):
		return "claude.ai"
	case strings.Contains(h, "chatgpt.com") || strings.Contains(h, "openai.com"):
		return "chatgpt"
	default:
		return ""
	}
}

// rejectAuthz sends a parse/validation failure either inline or (once the
// redirect target is trusted) back to the client with an OAuth error.
func (s *Server) rejectAuthz(w http.ResponseWriter, r *http.Request, rej *authzReject, state string) {
	if rej.redirectURI == "" {
		s.httpError(w, rej.status, rej.msg)
		return
	}
	u, err := url.Parse(rej.redirectURI)
	if err != nil {
		s.httpError(w, http.StatusBadRequest, "invalid redirect_uri")
		return
	}
	q := u.Query()
	q.Set("error", rej.oauthErr)
	if rej.desc != "" {
		q.Set("error_description", rej.desc)
	}
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// renderLogin serves the device-credential login page, re-emitting the OAuth
// params as hidden fields so the POST carries them. errMsg is shown on a retry.
func (s *Server) renderLogin(w http.ResponseWriter, p authzParams, errMsg string) {
	host := p.RedirectURI
	if u, err := url.Parse(p.RedirectURI); err == nil {
		host = u.Host
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = loginTmpl.Execute(w, loginData{AppHost: host, ErrMsg: errMsg, P: p, AuthorizePath: oauth.AuthorizePath})
}

type loginData struct {
	AppHost       string
	ErrMsg        string
	P             authzParams
	AuthorizePath string
}

// loginTmpl is server-rendered; html/template contextually escapes every
// attacker-controlled param reflected into the page.
var loginTmpl = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>askrelay — authorize</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>body{font-family:system-ui,sans-serif;max-width:30rem;margin:4rem auto;padding:0 1rem;line-height:1.5}
textarea{width:100%;box-sizing:border-box;font-family:monospace;font-size:.85rem}
button{margin-top:1rem;padding:.6rem 1.2rem;font-size:1rem}
.err{color:#b00020;font-weight:600}</style>
</head><body>
<h1>Authorize access</h1>
<p>The app at <strong>{{.AppHost}}</strong> wants to act as you in askrelay.</p>
<p>Paste your <strong>device credential</strong> to continue.</p>
{{if .ErrMsg}}<p class="err">{{.ErrMsg}}</p>{{end}}
<form method="post" action="{{.AuthorizePath}}">
<textarea name="credential" rows="4" required autofocus placeholder="device credential"></textarea>
<input type="hidden" name="response_type" value="{{.P.ResponseType}}">
<input type="hidden" name="client_id" value="{{.P.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.P.RedirectURI}}">
<input type="hidden" name="code_challenge" value="{{.P.CodeChallenge}}">
<input type="hidden" name="code_challenge_method" value="{{.P.CodeChallengeMethod}}">
<input type="hidden" name="state" value="{{.P.State}}">
<input type="hidden" name="resource" value="{{.P.Resource}}">
<input type="hidden" name="scope" value="{{.P.Scope}}">
<button type="submit">Authorize</button>
</form>
</body></html>`))

// newOpaqueToken returns a 256-bit URL-safe random token (auth codes, refresh
// tokens). Stored only as its SHA-256 (the store hashes on write).
func newOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("relay: random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
