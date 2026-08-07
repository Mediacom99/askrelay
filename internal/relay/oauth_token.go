package relay

import (
	"errors"
	"net/http"
	"time"

	"github.com/Mediacom99/askrelay/internal/relay/oauth"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// refreshTTL bounds a refresh token's life. The real revocation control is the
// live ActiveDeviceByID re-check on every refresh (revoke the device → refresh
// dies immediately); this TTL is only a backstop for an abandoned grant.
const refreshTTL = 30 * 24 * time.Hour

// maxTokenBody bounds the token-endpoint form (a code/refresh token + verifier).
const maxTokenBody = 8 << 10

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// handleToken is the OAuth 2.1 token endpoint: authorization_code (with PKCE) and
// refresh_token grants. Every failure is an opaque RFC 6749 §5.2 error; the
// device is re-checked live before any token is minted (WP-06 obligation).
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxTokenBody)
	if err := r.ParseForm(); err != nil {
		s.tokenError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.tokenFromCode(w, r)
	case "refresh_token":
		s.tokenFromRefresh(w, r)
	default:
		s.tokenError(w, http.StatusBadRequest, "unsupported_grant_type")
	}
}

func (s *Server) tokenFromCode(w http.ResponseWriter, r *http.Request) {
	code := r.PostForm.Get("code")
	verifier := r.PostForm.Get("code_verifier")
	clientID := r.PostForm.Get("client_id")
	redirectURI := r.PostForm.Get("redirect_uri")
	if code == "" || verifier == "" || clientID == "" || redirectURI == "" {
		s.tokenError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	now := time.Now().UTC()
	// Read the binding WITHOUT consuming: a bad client_id/redirect/PKCE attempt
	// must not burn a code the legitimate client can still redeem — the code
	// travels in a redirect and can be observed by a third party.
	b, err := s.store.AuthCodeByCode(code, now)
	if err != nil {
		s.log.Warn("token refused", "grant", "authorization_code", "reason", "bad_code")
		s.tokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	// The code is bound to the client + redirect it was issued for, and to the
	// PKCE challenge whose verifier only the real client holds.
	if b.ClientID != clientID || b.RedirectURI != redirectURI || !oauth.VerifyPKCE(b.CodeChallenge, verifier) {
		s.log.Warn("token refused", "grant", "authorization_code", "reason", "binding_mismatch")
		s.tokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	// Binding matched — claim the code single-use. A lost concurrent race (or an
	// expiry between read and claim) collapses to the same opaque refusal.
	if err := s.store.ConsumeAuthCode(code, now); err != nil {
		s.log.Warn("token refused", "grant", "authorization_code", "reason", "code_claim_lost")
		s.tokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	s.issueTokens(w, store.RefreshGrant{
		ClientID: b.ClientID, PersonID: b.PersonID, DeviceID: b.DeviceID,
		ClientType: b.ClientType, Resource: b.Resource,
	}, now)
}

func (s *Server) tokenFromRefresh(w http.ResponseWriter, r *http.Request) {
	rt := r.PostForm.Get("refresh_token")
	if rt == "" {
		s.tokenError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	now := time.Now().UTC()
	g, err := s.store.ConsumeRefreshToken(rt, now)
	if err != nil {
		reason := "bad_refresh"
		if errors.Is(err, store.ErrRefreshReused) {
			reason = "refresh_reuse" // family already revoked by the store
		}
		s.log.Warn("token refused", "grant", "refresh_token", "reason", reason)
		s.tokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	// A public client MAY send client_id; if it does, it must match the grant.
	if cid := r.PostForm.Get("client_id"); cid != "" && cid != g.ClientID {
		s.log.Warn("token refused", "grant", "refresh_token", "reason", "client_mismatch")
		s.tokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	s.issueTokens(w, g, now)
}

// issueTokens is the shared tail of both grants: the live device re-check (the
// WP-06 obligation — a revoked device cannot mint fresh tokens), then a
// single-audience access token + a rotated refresh token.
func (s *Server) issueTokens(w http.ResponseWriter, g store.RefreshGrant, now time.Time) {
	if _, err := s.store.ActiveDeviceByID(g.DeviceID); err != nil {
		s.log.Warn("token refused", "reason", "device_inactive")
		s.tokenError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	access, err := s.issuer.Mint(g.PersonID, g.ClientType, now)
	if err != nil {
		s.log.Error("token: mint access", "err", err)
		s.tokenError(w, http.StatusInternalServerError, "server_error")
		return
	}
	refresh, err := newOpaqueToken()
	if err != nil {
		s.log.Error("token: mint refresh", "err", err)
		s.tokenError(w, http.StatusInternalServerError, "server_error")
		return
	}
	if err := s.store.CreateRefreshToken(refresh, g, now.Add(refreshTTL), now); err != nil {
		s.log.Error("token: store refresh", "err", err)
		s.tokenError(w, http.StatusInternalServerError, "server_error")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.issuer.AccessTTL().Seconds()),
		RefreshToken: refresh,
	})
}

// tokenError writes an RFC 6749 §5.2 error (opaque code only, T-17).
func (s *Server) tokenError(w http.ResponseWriter, code int, oauthErr string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, code, map[string]string{"error": oauthErr})
}
