package httpapi

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func (s *Server) frontendBuilt() bool {
	if s.opt.WebDir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(s.opt.WebDir, "index.html"))
	return err == nil
}

// static serves the built single-page app. Unknown paths fall back to
// index.html for client-side routing; hashed assets are cached immutably.
func (s *Server) static() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, r, http.StatusMethodNotAllowed, "invalid_input", "method")
			return
		}
		if !s.frontendBuilt() {
			writeError(w, r, http.StatusServiceUnavailable, "frontend_not_built", "")
			return
		}
		clean := path.Clean("/" + r.URL.Path)
		file := filepath.Join(s.opt.WebDir, filepath.FromSlash(clean))
		if !strings.HasPrefix(file, filepath.Clean(s.opt.WebDir)) {
			writeError(w, r, http.StatusNotFound, "not_found", "")
			return
		}
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			if strings.HasPrefix(clean, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			http.ServeFile(w, r, file)
			return
		}
		if strings.HasPrefix(clean, "/assets/") || path.Ext(clean) != "" {
			writeError(w, r, http.StatusNotFound, "not_found", "")
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(s.opt.WebDir, "index.html"))
	})
}
