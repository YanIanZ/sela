package httpapi

import (
	"net/http"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

// The planned-care path (D-076): a person chooses a verified organisation
// from the care network and sends one care request that carries an
// approved consultation summary. Sending needs the same `sharing` consent
// as a share link, because it is a share addressed to one organisation.

func (s *Server) v3CareNetwork(w http.ResponseWriter, r *http.Request, p account.Principal) {
	if s.opt.Network == nil {
		writeError(w, r, http.StatusServiceUnavailable, "store_unavailable", "")
		return
	}
	orgs, err := s.opt.Network.PublicOrgs(r.Context())
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, map[string]any{"orgs": orgs, "realPartners": s.opt.Network.AllowsReal()})
}

func (s *Server) sharingGranted(r *http.Request, p account.Principal) (bool, error) {
	consents, err := s.opt.Accounts.Consents(r.Context(), p.UserID)
	if err != nil {
		return false, err
	}
	for _, c := range consents {
		if c.Purpose == "sharing" && c.Granted && !c.Outdated {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) v3CreateCareRequest(w http.ResponseWriter, r *http.Request, p account.Principal) {
	if s.opt.Network == nil {
		writeError(w, r, http.StatusServiceUnavailable, "store_unavailable", "")
		return
	}
	var in account.CareRequestInput
	if !decode(w, r, &in) {
		return
	}
	granted, err := s.sharingGranted(r, p)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	if !granted {
		writeError(w, r, http.StatusForbidden, "consent_required", "sharing")
		return
	}
	req, err := s.opt.Accounts.CreateCareRequest(r.Context(), p.UserID, in, s.opt.Network.AllowsReal())
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"request": req})
}

func (s *Server) v3CareRequests(w http.ResponseWriter, r *http.Request, p account.Principal) {
	list, err := s.opt.Accounts.CareRequests(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"requests": list})
}

func (s *Server) v3CareRequest(w http.ResponseWriter, r *http.Request, p account.Principal) {
	req, err := s.opt.Accounts.CareRequest(r.Context(), p.UserID, r.PathValue("id"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"request": req})
}

func (s *Server) v3ConfirmCareRequest(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Slot            time.Time `json:"slot"`
		ExpectedVersion int       `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	req, err := s.opt.Accounts.ConfirmCareRequest(r.Context(), p.UserID, r.PathValue("id"), in.Slot, in.ExpectedVersion)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"request": req})
}

func (s *Server) v3CancelCareRequest(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion int `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	req, err := s.opt.Accounts.CancelCareRequest(r.Context(), p.UserID, r.PathValue("id"), in.ExpectedVersion)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"request": req})
}
