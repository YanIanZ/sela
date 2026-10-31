package httpapi

import (
	"errors"
	"net/http"

	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/community"
)

// adminCommunityRoutes is the community moderation queue (D-072). Operators
// see only held, hidden and reported items — never an author, a profile or
// anyone's private space — and every decision is audited.
func (s *Server) adminCommunityRoutes() {
	m := s.mux
	m.HandleFunc("GET /api/admin/v1/community/queue", s.read(s.withAdmin(s.adminCommunityQueue)))
	m.HandleFunc("POST /api/admin/v1/community/{kind}/{id}/decision", s.mutation("mutation", smallBody, s.withAdmin(s.adminCommunityDecide)))
}

func (s *Server) adminCommunityQueue(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermCommunityModerate) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if s.opt.Community == nil {
		writeError(w, r, http.StatusServiceUnavailable, "store_unavailable", "")
		return
	}
	items, err := s.opt.Community.Queue(r.Context(), 50)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminCommunityDecide(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermCommunityModerate) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if s.opt.Community == nil {
		writeError(w, r, http.StatusServiceUnavailable, "store_unavailable", "")
		return
	}
	var in struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	kind, id := r.PathValue("kind"), r.PathValue("id")
	old, next, err := s.opt.Community.Decide(r.Context(), kind, id, in.Action)
	if err != nil {
		switch {
		case errors.Is(err, community.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "not_found", "")
		case errors.Is(err, community.ErrInvalid):
			writeError(w, r, http.StatusBadRequest, "invalid_input", "action")
		case errors.Is(err, community.ErrForbidden):
			writeError(w, r, http.StatusConflict, "already_removed", "")
		default:
			writeError(w, r, http.StatusInternalServerError, "internal", "")
		}
		return
	}
	if s.opt.Admin != nil {
		if err := s.opt.Admin.Audit(r.Context(), &p.AdminID, "community."+in.Action, "community_"+kind, id, string(old), string(next), in.Reason); err != nil {
			s.log.Warn("community.audit_failed")
		}
	}
	writeData(w, http.StatusOK, map[string]any{"kind": kind, "id": id, "status": next})
}
