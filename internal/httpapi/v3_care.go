package httpapi

import "net/http"

// v3CareDirectory is public: verified, sourced planned-care entries. It
// never requires a location and never creates a care request.
func (s *Server) v3CareDirectory(w http.ResponseWriter, r *http.Request) {
	if s.opt.Care == nil {
		writeError(w, r, http.StatusServiceUnavailable, "store_unavailable", "")
		return
	}
	q := r.URL.Query()
	entries, err := s.opt.Care.Public(r.Context(), q.Get("q"), q.Get("kind"), q.Get("city"), s.v3.cfg.Synthetic())
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"entries": entries})
}
