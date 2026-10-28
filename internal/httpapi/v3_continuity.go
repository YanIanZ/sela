package httpapi

import (
	"net/http"

	"github.com/YanIanZ/sela/internal/account"
)

// Memory is explicit and owner-created only. Without the personal_memory
// consent the list is disabled and empty; no AI drafting or extraction
// exists in this increment.
func (s *Server) v3Memory(w http.ResponseWriter, r *http.Request, p account.Principal) {
	view, err := s.opt.Accounts.Memory(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, view)
}

func (s *Server) v3CreateMemory(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in account.RecordInput
	if !decode(w, r, &in) {
		return
	}
	record, err := s.opt.Accounts.CreateMemory(r.Context(), p.UserID, in)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, record)
}

func (s *Server) v3UpdateMemory(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in account.RecordInput
	if !decode(w, r, &in) {
		return
	}
	record, err := s.opt.Accounts.UpdateMemory(r.Context(), p.UserID, r.PathValue("id"), in)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, record)
}

func (s *Server) v3DeleteMemory(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion int `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.opt.Accounts.DeleteMemory(r.Context(), p.UserID, r.PathValue("id"), in.ExpectedVersion); err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) v3Summaries(w http.ResponseWriter, r *http.Request, p account.Principal) {
	list, err := s.opt.Accounts.Summaries(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"summaries": list})
}

func (s *Server) v3Summary(w http.ResponseWriter, r *http.Request, p account.Principal) {
	record, err := s.opt.Accounts.Summary(r.Context(), p.UserID, r.PathValue("id"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, record)
}

func (s *Server) v3CreateSummary(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in account.RecordInput
	if !decode(w, r, &in) {
		return
	}
	record, err := s.opt.Accounts.CreateSummary(r.Context(), p.UserID, in)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, record)
}

func (s *Server) v3UpdateSummary(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in account.RecordInput
	if !decode(w, r, &in) {
		return
	}
	record, err := s.opt.Accounts.UpdateSummary(r.Context(), p.UserID, r.PathValue("id"), in)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, record)
}

func (s *Server) v3ApproveSummary(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion int `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	record, err := s.opt.Accounts.ApproveSummary(r.Context(), p.UserID, r.PathValue("id"), in.ExpectedVersion)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, record)
}

func (s *Server) v3DeleteSummary(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion int `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.opt.Accounts.DeleteSummary(r.Context(), p.UserID, r.PathValue("id"), in.ExpectedVersion); err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) v3CreateFollowup(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in account.RecordInput
	if !decode(w, r, &in) {
		return
	}
	record, err := s.opt.Accounts.CreateFollowup(r.Context(), p.UserID, r.PathValue("id"), in)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, record)
}

func (s *Server) v3Followups(w http.ResponseWriter, r *http.Request, p account.Principal) {
	list, err := s.opt.Accounts.Followups(r.Context(), p.UserID, r.PathValue("id"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"followups": list})
}
