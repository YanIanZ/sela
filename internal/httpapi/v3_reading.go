package httpapi

import "net/http"

// v3ReadingList is public: only items that passed the runtime editorial
// gate are present. It carries no user data and needs no store.
func (s *Server) v3ReadingList(w http.ResponseWriter, r *http.Request) {
	if s.v3.readingLib == nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": s.v3.readingLib.List()})
}

// v3ReadingItem answers one approved item. An unpublished, forged or
// unknown slug is indistinguishable: 404 not_found.
func (s *Server) v3ReadingItem(w http.ResponseWriter, r *http.Request) {
	if s.v3.readingLib == nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	item, ok := s.v3.readingLib.Get(r.PathValue("slug"))
	if !ok {
		writeError(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	writeData(w, http.StatusOK, item)
}
