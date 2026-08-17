package relay

import (
	"context"
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
	p, rej := s.parseAuthz(r.Context(), r.URL.Query())
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
		s.renderError(w, http.StatusBadRequest, "invalid form")
		return
	}
	p, rej := s.parseAuthz(r.Context(), r.PostForm)
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
		s.renderError(w, http.StatusInternalServerError, "authorization failed")
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
		s.renderError(w, http.StatusInternalServerError, "authorization failed")
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
func (s *Server) parseAuthz(ctx context.Context, v url.Values) (authzParams, *authzReject) {
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
	clientType, ok, err := s.resolveClient(ctx, p.ClientID, p.RedirectURI)
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
	if p.Resource != "" && !sameResource(p.Resource, s.cfg.BaseURL) {
		return authzParams{}, &authzReject{redirectURI: p.RedirectURI, oauthErr: "invalid_target", desc: "resource does not match this relay"}
	}
	return p, nil
}

// sameResource reports whether an RFC 8707 resource indicator refers to this
// relay, treating a root trailing slash as equivalent (RFC 3986 §6.2.3): clients
// send the origin as http://host:port/, but our BaseURL is slash-less.
func sameResource(resource, baseURL string) bool {
	return strings.TrimRight(resource, "/") == strings.TrimRight(baseURL, "/")
}

// resolveClient validates the client_id ↔ redirect_uri pairing and returns the
// derived client_type. A CIMD client_id (an https URL) is validated against its
// fetched metadata document (resolveCIMD). A DCR client_id must exist and list
// redirect_uri exactly. ok=false is any invalid pairing; err is only an
// infrastructure fault.
func (s *Server) resolveClient(ctx context.Context, clientID, redirectURI string) (clientType string, ok bool, err error) {
	ru, perr := url.Parse(redirectURI)
	if perr != nil || !ru.IsAbs() {
		return "", false, nil
	}
	if cu, cerr := url.Parse(clientID); cerr == nil && cu.Scheme == "https" && cu.Host != "" {
		return s.resolveCIMD(ctx, clientID, cu, redirectURI)
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

// resolveCIMD validates a Client ID Metadata Document client_id: fetch the doc,
// require it to self-declare the same client_id (confused-deputy guard), and
// accept redirectURI only if listed there (exact, or loopback-port-agnostic per
// RFC 8252 §7.3). Any fetch/validation failure fails closed (ok=false) with a
// reason-coded Warn — never a 500, never the URL/body (T-17/T-18). The §5.4
// profile comes from the client_id origin, not the (often loopback) redirect host.
func (s *Server) resolveCIMD(ctx context.Context, clientID string, cu *url.URL, redirectURI string) (string, bool, error) {
	meta, err := s.fetchCIMD(ctx, clientID)
	if err != nil {
		s.log.Warn("authorize refused", "reason", "cimd_fetch")
		return "", false, nil
	}
	if meta.ClientID != clientID {
		s.log.Warn("authorize refused", "reason", "cimd_client_id_mismatch")
		return "", false, nil
	}
	for _, reg := range meta.RedirectURIs {
		if redirectMatches(reg, redirectURI) {
			return clientTypeFor(cu.Host), true, nil
		}
	}
	s.log.Warn("authorize refused", "reason", "cimd_redirect_unlisted")
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
		s.renderError(w, rej.status, rej.msg)
		return
	}
	u, err := url.Parse(rej.redirectURI)
	if err != nil {
		s.renderError(w, http.StatusBadRequest, "invalid redirect_uri")
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
	// form-action must permit BOTH the POST back to us ('self') AND the OAuth 302
	// to the client's validated callback — browsers enforce form-action across the
	// redirect, so 'self' alone silently blocks the hand-off to the client (the
	// login page appears to "do nothing" on submit). p.RedirectURI is already
	// validated (resolveClient) before we render, so its origin is trusted.
	formAction := "'self'"
	if u, err := url.Parse(p.RedirectURI); err == nil && u.Host != "" {
		host = u.Host
		if u.Scheme != "" {
			formAction += " " + u.Scheme + "://" + u.Host
		}
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; form-action "+formAction+"; base-uri 'none'")
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

// renderError serves the branded HTML error page for BROWSER-facing authorize
// failures — the hard rejects that render inline instead of redirecting. API
// endpoints (enroll/token/mcp) keep using httpError (JSON); this is only for the
// human-facing /authorize paths. detail must be sanitized/content-free (T-17).
func (s *Server) renderError(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; base-uri 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = errorTmpl.Execute(w, errorData{Heading: "Couldn't authorize", Detail: detail})
}

// brandCSS is the shared style for the server-rendered pages (login + error),
// factored into one const so the two can't drift apart.
const brandCSS = `
:root{--accent:#3b82f6;--accent-press:#2563eb;--bg:#f6f7f9;--card:#fff;--text:#0a121d;--muted:#5b6472;--border:#e3e7ec;--err:#b00020}
@media (prefers-color-scheme:dark){:root{--bg:#0a121d;--card:#111c2b;--text:#e7ecf3;--muted:#8b97a8;--border:#1e2c3d;--err:#ff8a8a}}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;padding:1.5rem;font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;background:var(--bg);color:var(--text);line-height:1.5}
.card{width:100%;max-width:26rem;background:var(--card);border:1px solid var(--border);border-radius:14px;padding:2rem;box-shadow:0 1px 3px rgba(0,0,0,.06),0 8px 24px rgba(0,0,0,.06)}
.brand{display:flex;align-items:center;gap:.55rem;margin-bottom:1.25rem}
.mark{width:34px;height:34px;display:block}
.brand b{font-size:1.15rem;letter-spacing:-.01em}
h1{font-size:1.2rem;margin:0 0 .4rem}
p{margin:0 0 1rem;color:var(--muted)}
p strong{color:var(--text)}`

// brandMark is the pigeon mark + wordmark header shared by both pages (served
// same-origin from /brand, so the CSP stays img-src 'self').
const brandMark = `<div class="brand"><picture><source media="(prefers-color-scheme:dark)" srcset="/brand/mark-dark.png"><img class="mark" src="/brand/mark.png" width="34" height="34" alt=""></picture><b>askrelay</b></div>`

// loginTmpl is server-rendered; html/template contextually escapes every
// attacker-controlled param reflected into the page.
var loginTmpl = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>askrelay — authorize</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>` + brandCSS + `
label{display:block;font-size:.85rem;font-weight:600;margin-bottom:.4rem}
textarea{width:100%;min-height:5.5rem;padding:.7rem;border:1px solid var(--border);border-radius:8px;background:var(--bg);color:var(--text);font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.8rem;resize:vertical}
textarea:focus{outline:2px solid var(--accent);outline-offset:1px;border-color:var(--accent)}
button{margin-top:1rem;width:100%;padding:.7rem;border:0;border-radius:8px;background:var(--accent);color:#fff;font-size:1rem;font-weight:600;cursor:pointer}
button:hover{background:var(--accent-press)}
.err{color:var(--err);font-weight:600;font-size:.9rem;margin:0 0 .8rem}
.hint{font-size:.78rem;color:var(--muted);margin:.9rem 0 0}
code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.9em}
</style>
</head><body>
<main class="card">` + brandMark + `
<h1>Authorize access</h1>
<p><strong>{{.AppHost}}</strong> wants to act as you in askrelay.</p>
{{if .ErrMsg}}<p class="err">{{.ErrMsg}}</p>{{end}}
<form method="post" action="{{.AuthorizePath}}">
<label for="cred">Device credential</label>
<textarea id="cred" name="credential" rows="4" required autofocus placeholder="Paste the credential printed by askrelay enroll"></textarea>
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
<p class="hint">askrelay never sees your AI provider's credentials. Only paste a credential you generated with <code>askrelay enroll</code>.</p>
</main>
</body></html>`))

// errorTmpl is the branded page for browser-facing authorize failures (the hard
// rejects that render inline instead of redirecting). Detail is a sanitized,
// content-free string (T-17).
var errorTmpl = template.Must(template.New("error").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>askrelay</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>` + brandCSS + `</style>
</head><body>
<main class="card">` + brandMark + `
<h1>{{.Heading}}</h1>
<p>{{.Detail}}</p>
</main>
</body></html>`))

type errorData struct{ Heading, Detail string }

// newOpaqueToken returns a 256-bit URL-safe random token (auth codes, refresh
// tokens). Stored only as its SHA-256 (the store hashes on write).
func newOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("relay: random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
