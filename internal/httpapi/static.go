package httpapi

import (
	"io"
	"io/fs"
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
// Anything that looks like the API (/API/..., /api%2f..., /apifoo) gets a
// JSON 404 instead of the app, so a client never mistakes HTML for data.
func (s *Server) static() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(strings.ToLower(r.URL.Path), "/api") {
			writeError(w, r, http.StatusNotFound, "not_found", "")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "")
			return
		}
		if !s.frontendBuilt() {
			writeError(w, r, http.StatusServiceUnavailable, "frontend_not_built", "")
			return
		}
		// Files are opened through os.Root, which refuses any name that
		// leaves the web directory, symlinks included; the cleaned name
		// is also checked with fs.ValidPath before use.
		root, err := os.OpenRoot(s.opt.WebDir)
		if err != nil {
			writeError(w, r, http.StatusServiceUnavailable, "frontend_not_built", "")
			return
		}
		defer root.Close()
		webFS := root.FS()
		clean := path.Clean("/" + r.URL.Path)
		name := strings.TrimPrefix(clean, "/")
		if name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) {
			writeError(w, r, http.StatusNotFound, "not_found", "")
			return
		}
		if info, err := fs.Stat(webFS, name); err == nil && !info.IsDir() {
			if strings.HasPrefix(clean, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			serveFile(w, r, webFS, name)
			return
		}
		if strings.HasPrefix(clean, "/assets/") || path.Ext(clean) != "" {
			writeError(w, r, http.StatusNotFound, "not_found", "")
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		serveFile(w, r, webFS, "index.html")
	})
}

// serveFile writes one regular file from fsys. It uses ServeContent rather
// than ServeFileFS so that "index.html" is served in place instead of
// redirected to the directory.
func serveFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	f, err := fsys.Open(name)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	rs, seekable := f.(io.ReadSeeker)
	if err != nil || info.IsDir() || !seekable {
		writeError(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), rs)
}
