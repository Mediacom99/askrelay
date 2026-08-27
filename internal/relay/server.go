package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Mediacom99/askrelay/internal/relay/mcp"
	"github.com/Mediacom99/askrelay/internal/relay/oauth"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// Version is the relay build version, stamped into /healthz and the `version`
// verb. A plain var (overridable via -ldflags in WP-14) keeps it simple.
var Version = "dev"

// Server is the relay HTTP server: the store, its config, a logger, and the
// routed mux. Construct with NewServer; run with Run. The MCP surface (WP-07),
// OAuth (WP-05/06), and the WebSocket hub (WP-08) mount onto this skeleton
// later.
type Server struct {
	cfg    Config
	store  *store.Store
	issuer *oauth.Issuer
	log    *slog.Logger
	mux    *http.ServeMux
	hub    *hub
	// signWait correlates outstanding sign_requests with their replies (T-21).
	signWait *signWaiters
	// bearer guards protected routes with access-token validation (WP-05); the
	// /mcp route is wrapped with it in NewServer.
	bearer func(http.Handler) http.Handler
	// fetchCIMD fetches a Client ID Metadata Document (WP-06/S-04). A field, not a
	// direct call, so tests inject a fake: the SSRF guard blocks the loopback
	// address an httptest server listens on, so the CIMD path can't be exercised
	// over real HTTP in a unit test.
	fetchCIMD func(context.Context, string) (*clientMetadata, error)
}

// NewServer wires the routes; it does not listen. store, issuer, and log must
// be non-nil.
func NewServer(cfg Config, st *store.Store, iss *oauth.Issuer, log *slog.Logger) *Server {
	s := &Server{cfg: cfg, store: st, issuer: iss, log: log, mux: http.NewServeMux(),
		hub: newHub(), signWait: newSignWaiters()}
	// T-20: /mcp also accepts a daemon's device credential. The store checks
	// mirror /ws exactly — active device, and the credential's person owns it.
	s.bearer = oauth.NewBearerMiddleware(iss, cfg.BaseURL, log, func(person, device string) error {
		dev, err := st.ActiveDeviceByID(device)
		if err != nil {
			return err
		}
		if dev.PersonID != person {
			return fmt.Errorf("relay: device %s does not belong to the credential's person", device)
		}
		return nil
	})
	s.fetchCIMD = newCIMDFetcher().get
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /brand/{name}", s.handleBrand)
	s.mux.HandleFunc("POST /enroll/{token}", s.handleEnroll)
	s.mux.Handle("GET "+oauth.PRMPath, oauth.ProtectedResourceMetadataHandler(cfg.BaseURL))
	s.mux.HandleFunc("GET "+oauth.AuthServerMetaPath, s.handleASMetadata)
	s.mux.HandleFunc("GET "+oauth.JWKSPath, s.handleJWKS)
	s.mux.HandleFunc("GET "+oauth.AuthorizePath, s.handleAuthorize)
	s.mux.HandleFunc("POST "+oauth.AuthorizePath, s.handleAuthorizeSubmit)
	s.mux.HandleFunc("POST "+oauth.TokenPath, s.handleToken)
	s.mux.HandleFunc("POST "+oauth.RegisterPath, s.handleRegister)
	s.mux.Handle("POST /mcp", s.bearer(maxBytes(mcp.NewHandler(st, log, Version, s, s).HTTPHandler(), maxMCPBody)))
	s.mux.HandleFunc("GET /ws", s.handleWS)
	return s
}

// Run serves until ctx is cancelled (SIGINT/SIGTERM from the caller), then
// drains in-flight requests within a short grace before returning.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.ListenAddr,
		Handler:           s.logRequests(s.recoverPanic(s.mux)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No global WriteTimeout on purpose: wait_for_activity long-polls up to
		// the client-profile cap (240s / 25m, mcp.profileCap), and a global write
		// deadline would sever those legitimate long-polls. Use per-handler
		// http.ResponseController deadlines if finer control is ever needed.
		ErrorLog: slog.NewLogLogger(s.log.Handler(), slog.LevelError),
	}
	errc := make(chan error, 1)
	go func() {
		s.log.Info("relay listening", "addr", s.cfg.ListenAddr, "version", Version)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
			return
		}
		errc <- nil
	}()
	go s.runSweeper(ctx)
	go s.reconcileRevocations(ctx)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		s.log.Info("relay shutting down")
		s.hub.closeAll() // drop WS conns so their read loops exit before Shutdown waits
		shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}

// sweepInterval is how often the retention sweeper runs. Fixed (not a config
// knob): the T-09 grace/TTL horizons are hours-to-days, so hourly is ample.
const sweepInterval = time.Hour

// runSweeper deletes expired messages/drafts and prunes replay tombstones (T-09)
// on a ticker until ctx is cancelled.
func (s *Server) runSweeper(ctx context.Context) {
	t := time.NewTicker(sweepInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.sweepOnce(time.Now().UTC())
		}
	}
}

// sweepOnce runs one retention pass, logging counts only (T-18). A failed sweep
// is logged and retried next tick — never fatal.
func (s *Server) sweepOnce(now time.Time) {
	pol := store.RetentionPolicy{AckGrace: s.cfg.AckGrace, HardTTL: s.cfg.HardTTL}
	fresh := store.Freshness{MaxAge: s.cfg.MaxAge, MaxSkew: s.cfg.MaxSkew}
	msgs, err := s.store.SweepMessages(pol, now)
	if err != nil {
		s.log.Error("sweep messages", "err", err)
	}
	drafts, err := s.store.SweepDrafts(pol, now)
	if err != nil {
		s.log.Error("sweep drafts", "err", err)
	}
	tombs, err := s.store.PruneTombstones(fresh, now)
	if err != nil {
		s.log.Error("prune tombstones", "err", err)
	}
	oauthRows, err := s.store.SweepOAuth(now)
	if err != nil {
		s.log.Error("sweep oauth", "err", err)
	}
	s.log.Info("retention sweep", "messages", msgs, "drafts", drafts, "tombstones", tombs, "oauth", oauthRows)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": Version})
}

// logRequests logs one line per request at the boundary — method, path,
// status, duration. Ids and shapes only, never bodies or content (T-18).
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Info("request",
			"method", r.Method, "path", r.URL.Path,
			"status", rec.status, "dur_ms", time.Since(start).Milliseconds())
	})
}

// recoverPanic turns a handler panic into a sanitized 500 plus a boundary log
// line, instead of a bare connection reset. net/http already keeps the process
// alive on a per-request panic, but the panic would bypass slog and the client
// would see an abrupt drop. The panic value is deliberately NOT logged — it can
// carry message content (T-18); the method+path are enough to locate the fault.
// Sits inside logRequests so the recovered request still gets its 500 access-log
// line.
func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("request panic", "method", r.Method, "path", r.URL.Path)
				s.httpError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder captures the response status for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap exposes the underlying ResponseWriter so http.ResponseController can
// reach its Hijacker — the WebSocket upgrade (/ws) hijacks the connection, and
// without this the logging wrapper hides that capability (a 501 at Accept).
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// maxMCPBody bounds a /mcp request body — the WP-07 ingress note (a 64 KiB
// envelope + JSON-RPC framing fits comfortably).
const maxMCPBody = 128 << 10

// maxBytes caps the request body of next (WP-01/03 ingress discipline).
func maxBytes(next http.Handler, n int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, n)
		next.ServeHTTP(w, r)
	})
}

// writeJSON writes v as a JSON body with the given status.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
