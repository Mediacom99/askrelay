package oauth

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// PRMPath is the RFC 9728 protected-resource-metadata well-known path.
const PRMPath = "/.well-known/oauth-protected-resource"

// ProtectedResourceMetadataHandler serves the RFC 9728 PRM document: this relay
// is the resource (baseURL), and its embedded authorization server (WP-06) is
// at the same base. Bearer tokens arrive in the Authorization header.
func ProtectedResourceMetadataHandler(baseURL string) http.Handler {
	return auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               baseURL,
		AuthorizationServers:   []string{baseURL},
		BearerMethodsSupported: []string{"header"},
	})
}

// NewBearerMiddleware guards a handler with access-token validation. On failure
// it emits 401 + a WWW-Authenticate challenge pointing at the PRM (RFC 9728
// discovery). The verifier returns a BARE auth.ErrInvalidToken — go-sdk echoes
// the error string into the 401 body, so the failure reason must not travel to
// the caller (T-17); it is logged server-side instead, as a reason code only,
// never the token (T-18). Expiration is set because go-sdk re-checks it.
func NewBearerMiddleware(iss *Issuer, baseURL string, log *slog.Logger) func(http.Handler) http.Handler {
	verifier := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		claims, err := iss.parse(token, time.Now().UTC())
		if err != nil {
			log.Warn("bearer rejected", "reason", "invalid_token")
			return nil, auth.ErrInvalidToken
		}
		if claims.Use != useAccess {
			log.Warn("bearer rejected", "reason", "wrong_use")
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{
			UserID:     claims.Subject,
			Expiration: claims.ExpiresAt.Time,
			Extra:      map[string]any{"client_type": claims.ClientType},
		}, nil
	}
	return auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: baseURL + PRMPath,
	})
}

// PersonFromContext returns the authenticated person and client type attached
// by the bearer middleware, so downstream handlers (WP-07 tools) never touch
// go-sdk's auth package directly.
func PersonFromContext(ctx context.Context) (person, clientType string, ok bool) {
	info := auth.TokenInfoFromContext(ctx)
	if info == nil || info.UserID == "" {
		return "", "", false
	}
	ct, _ := info.Extra["client_type"].(string)
	return info.UserID, ct, true
}
