package httpapi

import (
	"net/http"

	"github.com/YanIanZ/sela/internal/account"
)

// Meaning Integrity Engine read routes (D-078). They return ids, versions,
// counts and reasons only: never a quote, a claim text or a title.

// v3MeaningImpact previews what editing, correcting or deleting a record
// would invalidate, so the owner sees it before confirming.
func (s *Server) v3MeaningImpact(w http.ResponseWriter, r *http.Request, p account.Principal) {
	impact, err := s.opt.Accounts.MeaningImpact(r.Context(), p.UserID, r.URL.Query().Get("id"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"impact": impact})
}

// v3MeaningLineage answers "why did Sela say this?": the parents of the
// record's current version (with whether each is still current) and what
// currently rests on it.
func (s *Server) v3MeaningLineage(w http.ResponseWriter, r *http.Request, p account.Principal) {
	lineage, err := s.opt.Accounts.MeaningLineage(r.Context(), p.UserID, r.URL.Query().Get("id"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"lineage": lineage})
}

// v3MeaningEvents lists the owner's latest invalidations.
func (s *Server) v3MeaningEvents(w http.ResponseWriter, r *http.Request, p account.Principal) {
	events, err := s.opt.Accounts.MeaningEvents(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"events": events})
}
