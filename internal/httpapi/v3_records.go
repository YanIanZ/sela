package httpapi

import (
	"github.com/YanIanZ/sela/internal/account"
	"net/http"
)

func (s *Server) v3Records(w http.ResponseWriter, r *http.Request, p account.Principal) {
	records, err := s.opt.Accounts.Records(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, 200, map[string]any{"records": records})
}
func (s *Server) v3Record(w http.ResponseWriter, r *http.Request, p account.Principal) {
	record, err := s.opt.Accounts.Record(r.Context(), p.UserID, r.PathValue("id"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, 200, record)
}
func (s *Server) v3CreateRecord(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in account.RecordInput
	if !decode(w, r, &in) {
		return
	}
	if in.Kind != "note" && in.Kind != "checkin" {
		writeError(w, r, 400, "invalid_input", "kind")
		return
	}
	record, err := s.opt.Accounts.CreateRecord(r.Context(), p.UserID, in)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, 201, record)
}
func (s *Server) v3UpdateRecord(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in account.RecordInput
	if !decode(w, r, &in) {
		return
	}
	if in.Kind != "note" && in.Kind != "checkin" {
		writeError(w, r, 400, "invalid_input", "kind")
		return
	}
	record, err := s.opt.Accounts.UpdateRecord(r.Context(), p.UserID, r.PathValue("id"), in)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, 200, record)
}

// v3DerivedImpact answers the owner-only, content-free impact of deleting
// one source record: which owned summaries pin it and how many shares each
// carries. The source id arrives as a query parameter; unknown, malformed
// and foreign ids are the same 404.
func (s *Server) v3DerivedImpact(w http.ResponseWriter, r *http.Request, p account.Principal) {
	impact, err := s.opt.Accounts.DerivedImpact(r.Context(), p.UserID, r.URL.Query().Get("source"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, impact)
}

func (s *Server) v3DeleteRecord(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion int `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	derived := r.URL.Query().Get("derived")
	if derived == "" {
		derived = account.DerivedKeep
	}
	if derived != account.DerivedKeep && derived != account.DerivedDelete {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "derived")
		return
	}
	s.v3.listening.mu.Lock()
	defer s.v3.listening.mu.Unlock()
	if err := s.opt.Accounts.DeleteRecord(r.Context(), p.UserID, r.PathValue("id"), in.ExpectedVersion, derived); err != nil {
		s.v3Error(w, r, err)
		return
	}
	for id, room := range s.v3.listening.rooms {
		if room.owner == p.UserID && room.RecordID == r.PathValue("id") {
			if room.cancel != nil {
				room.cancel()
			}
			if room.timer != nil {
				room.timer.Stop()
			}
			delete(s.v3.listening.rooms, id)
		}
	}
	writeData(w, 200, map[string]bool{"deleted": true})
}
func (s *Server) v3RecordHistory(w http.ResponseWriter, r *http.Request, p account.Principal) {
	records, err := s.opt.Accounts.RecordHistory(r.Context(), p.UserID, r.PathValue("id"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, 200, map[string]any{"versions": records})
}
