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
func (s *Server) v3DeleteRecord(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion int `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	s.v3.listening.mu.Lock()
	defer s.v3.listening.mu.Unlock()
	if err := s.opt.Accounts.DeleteRecord(r.Context(), p.UserID, r.PathValue("id"), in.ExpectedVersion); err != nil {
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
