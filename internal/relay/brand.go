package relay

import (
	"embed"
	"net/http"
)

//go:embed assets/mark.png assets/mark-dark.png
var brandFS embed.FS

// handleBrand serves an embedded brand mark by name. The login page references
// these same-origin, so its CSP stays img-src 'self' — no external fetch and no
// inline base64 blob. {name} is a single path segment (Go's ServeMux never
// matches across '/'), so there is no path traversal.
func (s *Server) handleBrand(w http.ResponseWriter, r *http.Request) {
	data, err := brandFS.ReadFile("assets/" + r.PathValue("name"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	_, _ = w.Write(data)
}
