package relay

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

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
}

// NewServer wires the routes; it does not listen. store, issuer, and log must
// be non-nil.
func NewServer(cfg Config, st *store.Store, iss *oauth.Issuer, log *slog.Logger) *Server {
	s := &Server{cfg: cfg, store: st, issuer: iss, log: log, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("POST /enroll/{token}", s.handleEnroll)
	return s
}

// Run serves until ctx is cancelled (SIGINT/SIGTERM from the caller), then
// drains in-flight requests within a short grace before returning.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.ListenAddr,
		Handler:           s.logRequests(s.mux),
		ReadHeaderTimeout: 10 * time.Second,
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

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		s.log.Info("relay shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
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

// statusRecorder captures the response status for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// writeJSON writes v as a JSON body with the given status.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
