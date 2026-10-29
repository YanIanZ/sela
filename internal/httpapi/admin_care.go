package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/care"
)

func (s *Server) adminCareRoutes() {
	m := s.mux
	m.HandleFunc("GET /api/admin/v1/directory", s.read(s.withAdmin(s.adminDirectoryList)))
	m.HandleFunc("POST /api/admin/v1/directory", s.mutation("mutation", smallBody, s.withAdmin(s.adminDirectoryCreate)))
	m.HandleFunc("PATCH /api/admin/v1/directory/{id}", s.mutation("mutation", smallBody, s.withAdmin(s.adminDirectoryUpdate)))
	m.HandleFunc("POST /api/admin/v1/directory/{id}/submit", s.mutation("mutation", smallBody, s.withAdmin(s.adminDirectorySubmit)))
	m.HandleFunc("POST /api/admin/v1/directory/{id}/verify", s.mutation("mutation", smallBody, s.withAdmin(s.adminDirectoryVerify)))
	m.HandleFunc("POST /api/admin/v1/directory/{id}/reject", s.mutation("mutation", smallBody, s.withAdmin(s.adminDirectoryReject)))
	m.HandleFunc("POST /api/admin/v1/directory/{id}/unpublish", s.mutation("mutation", smallBody, s.withAdmin(s.adminDirectoryUnpublish)))
	m.HandleFunc("GET /api/admin/v1/directory/audit", s.read(s.withAdmin(s.adminDirectoryAudit)))
	m.HandleFunc("GET /api/admin/v1/sandbox/requests", s.read(s.withAdmin(s.adminSandboxList)))
	m.HandleFunc("POST /api/admin/v1/sandbox/requests", s.mutation("mutation", smallBody, s.withAdmin(s.adminSandboxCreate)))
	m.HandleFunc("POST /api/admin/v1/sandbox/requests/{id}/transition", s.mutation("mutation", smallBody, s.withAdmin(s.adminSandboxTransition)))
}

func (s *Server) careReady(w http.ResponseWriter, r *http.Request) bool {
	if s.opt.Care == nil {
		writeError(w, r, http.StatusServiceUnavailable, "store_unavailable", "")
		return false
	}
	return true
}

func (s *Server) canDirectoryRead(p admin.Principal) bool {
	return p.Can(admin.PermDirectoryWrite) || p.Can(admin.PermDirectoryVerify)
}

func (s *Server) adminDirectoryList(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !s.canDirectoryRead(p) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if !s.careReady(w, r) {
		return
	}
	entries, err := s.opt.Care.List(r.Context(), care.Status(r.URL.Query().Get("status")))
	if err != nil {
		if errors.Is(err, care.ErrInvalid) {
			writeError(w, r, http.StatusBadRequest, "invalid_input", "")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"entries": entries})
}

type directoryWriteRequest struct {
	care.EntryInput
	ExpectedVersion int `json:"expectedVersion"`
}

func (s *Server) adminDirectoryCreate(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermDirectoryWrite) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if !s.careReady(w, r) {
		return
	}
	var req directoryWriteRequest
	if !decode(w, r, &req) {
		return
	}
	req.Synthetic = false
	entry, err := s.opt.Care.Create(r.Context(), p.AdminID, req.EntryInput)
	if err != nil {
		s.careError(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, entry)
}

func (s *Server) adminDirectoryUpdate(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermDirectoryWrite) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if !s.careReady(w, r) {
		return
	}
	var req directoryWriteRequest
	if !decode(w, r, &req) {
		return
	}
	req.Synthetic = false
	entry, err := s.opt.Care.Update(r.Context(), p.AdminID, r.PathValue("id"), req.ExpectedVersion, req.EntryInput)
	if err != nil {
		s.careError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, entry)
}

type directoryActionRequest struct {
	ExpectedVersion int    `json:"expectedVersion"`
	Reason          string `json:"reason"`
	ReverifyAt      string `json:"reverifyAt"`
}

func (s *Server) adminDirectorySubmit(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermDirectoryWrite) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	s.directoryAction(w, r, p, "submit")
}

func (s *Server) adminDirectoryVerify(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermDirectoryVerify) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	s.directoryAction(w, r, p, "verify")
}

func (s *Server) adminDirectoryReject(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermDirectoryVerify) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	s.directoryAction(w, r, p, "reject")
}

func (s *Server) adminDirectoryUnpublish(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermDirectoryVerify) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	s.directoryAction(w, r, p, "unpublish")
}

func (s *Server) directoryAction(w http.ResponseWriter, r *http.Request, p admin.Principal, action string) {
	if !s.careReady(w, r) {
		return
	}
	var req directoryActionRequest
	if !decodeOptional(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	var (
		entry care.Entry
		err   error
	)
	switch action {
	case "submit":
		entry, err = s.opt.Care.Submit(r.Context(), p.AdminID, id, req.ExpectedVersion)
	case "verify":
		var at time.Time
		if req.ReverifyAt != "" {
			at, err = time.Parse("2006-01-02", req.ReverifyAt)
			if err != nil {
				writeError(w, r, http.StatusBadRequest, "invalid_input", "reverifyAt")
				return
			}
		}
		entry, err = s.opt.Care.Verify(r.Context(), p.AdminID, id, req.ExpectedVersion, at)
	case "reject":
		entry, err = s.opt.Care.Reject(r.Context(), p.AdminID, id, req.ExpectedVersion, req.Reason)
	case "unpublish":
		entry, err = s.opt.Care.Unpublish(r.Context(), p.AdminID, id, req.ExpectedVersion, req.Reason)
	}
	if err != nil {
		s.careError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, entry)
}

func (s *Server) adminDirectoryAudit(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !s.canDirectoryRead(p) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if !s.careReady(w, r) {
		return
	}
	events, err := s.opt.Care.Audit(r.Context(), 50)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) adminSandboxList(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermSandboxWrite) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if !s.careReady(w, r) {
		return
	}
	requests, err := s.opt.Care.ListSandbox(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"requests": requests})
}

func (s *Server) adminSandboxCreate(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermSandboxWrite) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if !s.careReady(w, r) {
		return
	}
	var req struct {
		Organization string `json:"organization"`
		Professional string `json:"professional"`
		Service      string `json:"service"`
		Note         string `json:"note"`
	}
	if !decode(w, r, &req) {
		return
	}
	request, err := s.opt.Care.CreateSandbox(r.Context(), p.AdminID, req.Organization, req.Professional, req.Service, req.Note)
	if err != nil {
		s.careError(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, request)
}

func (s *Server) adminSandboxTransition(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermSandboxWrite) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if !s.careReady(w, r) {
		return
	}
	var req struct {
		ExpectedVersion int                `json:"expectedVersion"`
		Status          care.SandboxStatus `json:"status"`
		Note            string             `json:"note"`
	}
	if !decode(w, r, &req) {
		return
	}
	request, err := s.opt.Care.TransitionSandbox(r.Context(), p.AdminID, r.PathValue("id"), req.ExpectedVersion, req.Status, req.Note)
	if err != nil {
		s.careError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, request)
}

func (s *Server) careError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, care.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "directory_forbidden", "")
	case errors.Is(err, care.ErrConflict):
		writeError(w, r, http.StatusConflict, "directory_conflict", "")
	case errors.Is(err, care.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "")
	case errors.Is(err, care.ErrInvalid):
		writeError(w, r, http.StatusBadRequest, "invalid_input", "")
	default:
		writeError(w, r, http.StatusInternalServerError, "internal", "")
	}
}
