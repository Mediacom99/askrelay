package relay

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// maxEnrollBody bounds the enrollment request — a pubkey plus a short label,
// nothing large. Pre-bounding the read is the WP-01/WP-03 ingress discipline
// (never hand an unbounded reader to a decoder).
const maxEnrollBody = 4 << 10

type enrollRequest struct {
	PubKey string `json:"pubkey"` // base64 (std) of the 32-byte Ed25519 public key
	Label  string `json:"label"`
}

type enrollResponse struct {
	PersonID         string `json:"person_id"`
	DeviceID         string `json:"device_id"`
	BaseURL          string `json:"base_url"`
	DeviceCredential string `json:"device_credential"` // long-lived WS credential (T-06)
}

// handleEnroll consumes a single-use invite and registers a device (arch
// §4.3). The endpoint is unauthenticated and public, so every failure is a
// sanitized, non-oracling response (T-17): a bad token, an expired invite, and
// an in-use key are all opaque 4xx — the caller learns only "no", never which
// tokens exist. Security refusals log at Warn with a reason code, never the
// offending token or key (T-18).
func (s *Server) handleEnroll(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	var req enrollRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxEnrollBody)).Decode(&req); err != nil {
		s.httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	pub, err := base64.StdEncoding.DecodeString(req.PubKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		s.httpError(w, http.StatusBadRequest, "invalid pubkey")
		return
	}

	person, device, err := s.store.Enroll(token, ed25519.PublicKey(pub), req.Label, time.Now().UTC())
	switch {
	case errors.Is(err, store.ErrInviteInvalid):
		s.log.Warn("enroll refused", "reason", "invite_invalid")
		s.httpError(w, http.StatusForbidden, "invalid or expired invite")
		return
	case errors.Is(err, store.ErrKeyInUse):
		s.log.Warn("enroll refused", "reason", "key_in_use")
		s.httpError(w, http.StatusConflict, "device key already registered")
		return
	case err != nil:
		s.log.Error("enroll failed", "err", err) // full error stays server-side only
		s.httpError(w, http.StatusInternalServerError, "enrollment failed")
		return
	}

	cred, err := s.issuer.MintDeviceCredential(person.ID, device.ID, time.Now().UTC())
	if err != nil {
		s.log.Error("mint device credential", "err", err) // device is enrolled but unusable
		s.httpError(w, http.StatusInternalServerError, "enrollment failed")
		return
	}

	s.log.Info("device enrolled", "person", person.ID, "device", device.ID)
	writeJSON(w, http.StatusOK, enrollResponse{
		PersonID:         person.ID,
		DeviceID:         device.ID,
		BaseURL:          s.cfg.BaseURL,
		DeviceCredential: cred,
	})
}

// httpError writes a sanitized JSON error — a stable, content-free message
// (T-17: remote-facing errors are sanitized).
func (s *Server) httpError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
