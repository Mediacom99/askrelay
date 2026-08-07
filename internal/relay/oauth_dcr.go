package relay

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/Mediacom99/askrelay/internal/relay/oauth"
)

// maxRegisterBody bounds the DCR JSON body.
const maxRegisterBody = 8 << 10

// handleASMetadata serves RFC 8414 authorization-server metadata.
func (s *Server) handleASMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, oauth.AuthServerMetadata(s.cfg.BaseURL))
}

// handleJWKS serves the relay's Ed25519 verification key as a JWK Set.
func (s *Server) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, oauth.JWKSet{Keys: []oauth.JWK{s.issuer.PublicJWK()}})
}

// handleRegister is the RFC 7591 Dynamic Client Registration endpoint (claude.ai's
// path). Registration is unauthenticated by design (the client has no token yet);
// DCR churn/abuse is the tracked R-10 risk, mitigated operationally (reverse-proxy
// rate limiting), not here. Only redirect_uris are security-relevant and are
// validated; the rest of the client metadata is untrusted display text.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var meta oauthex.ClientRegistrationMetadata
	if err := json.NewDecoder(io.LimitReader(r.Body, maxRegisterBody)).Decode(&meta); err != nil {
		s.registerError(w, "invalid_client_metadata", "malformed registration body")
		return
	}
	if len(meta.RedirectURIs) == 0 {
		s.registerError(w, "invalid_redirect_uri", "at least one redirect_uri is required")
		return
	}
	for _, u := range meta.RedirectURIs {
		if !validRedirectURI(u) {
			s.registerError(w, "invalid_redirect_uri", "redirect_uri must be an absolute https URL (or http loopback), no fragment")
			return
		}
	}
	id, err := s.store.RegisterClient(meta.RedirectURIs, meta.ClientName, time.Now().UTC())
	if err != nil {
		s.log.Error("dcr: register client", "err", err)
		s.registerError(w, "invalid_client_metadata", "registration failed")
		return
	}
	s.log.Info("client registered", "client", id, "redirect_uris", len(meta.RedirectURIs))
	meta.TokenEndpointAuthMethod = "none" // public client; PKCE is the protection
	resp := oauthex.ClientRegistrationResponse{
		ClientRegistrationMetadata: meta,
		ClientID:                   id,
		ClientIDIssuedAt:           time.Now().UTC(),
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, &resp) // pointer: a value won't trigger MarshalJSON
}

// registerError writes an RFC 7591 §3.2.2 registration error.
func (s *Server) registerError(w http.ResponseWriter, code, desc string) {
	writeJSON(w, http.StatusBadRequest, oauthex.ClientRegistrationError{ErrorCode: code, ErrorDescription: desc})
}

// validRedirectURI enforces an absolute redirect: https anywhere, or http only
// for loopback (native apps). Fragments are forbidden (RFC 6749 §3.1.2).
func validRedirectURI(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Fragment != "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return u.Host != ""
	case "http":
		h := u.Hostname()
		return h == "localhost" || h == "127.0.0.1" || h == "::1"
	default:
		return false
	}
}
