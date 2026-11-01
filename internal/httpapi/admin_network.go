package httpapi

import (
	"errors"
	"net/http"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/carenet"
)

// adminNetworkRoutes are the professional care network (D-076):
//   - /network/* lets internal operators create, verify and suspend partner
//     organisations and verify practitioner licences;
//   - /partner/* is the professional workspace. Every query is filtered by
//     the principal's own organisation; staff see status and times, and
//     only practitioners and partner administrators open a summary.
func (s *Server) adminNetworkRoutes() {
	m := s.mux
	m.HandleFunc("GET /api/admin/v1/network/orgs", s.read(s.withAdmin(s.adminNetworkOrgs)))
	m.HandleFunc("POST /api/admin/v1/network/orgs", s.mutation("mutation", smallBody, s.withAdmin(s.adminNetworkCreateOrg)))
	m.HandleFunc("POST /api/admin/v1/network/orgs/{id}/status", s.mutation("mutation", smallBody, s.withAdmin(s.adminNetworkOrgStatus)))
	m.HandleFunc("GET /api/admin/v1/network/orgs/{id}/members", s.read(s.withAdmin(s.adminNetworkMembers)))
	m.HandleFunc("POST /api/admin/v1/network/members/{username}/verify-license", s.mutation("mutation", smallBody, s.withAdmin(s.adminNetworkVerifyLicense)))

	m.HandleFunc("GET /api/admin/v1/partner/org", s.read(s.withAdmin(s.adminPartnerOrg)))
	m.HandleFunc("GET /api/admin/v1/partner/requests", s.read(s.withAdmin(s.adminPartnerRequests)))
	m.HandleFunc("GET /api/admin/v1/partner/requests/{id}", s.read(s.withAdmin(s.adminPartnerRequest)))
	m.HandleFunc("POST /api/admin/v1/partner/requests/{id}/respond", s.mutation("mutation", smallBody, s.withAdmin(s.adminPartnerRespond)))
	m.HandleFunc("POST /api/admin/v1/partner/requests/{id}/complete", s.mutation("mutation", smallBody, s.withAdmin(s.adminPartnerComplete)))
}

func (s *Server) networkReady(w http.ResponseWriter, r *http.Request) bool {
	if s.opt.Network == nil || s.opt.Accounts == nil {
		writeError(w, r, http.StatusServiceUnavailable, "store_unavailable", "")
		return false
	}
	return true
}

func (s *Server) networkError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, carenet.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "")
	case errors.Is(err, carenet.ErrInvalid):
		writeError(w, r, http.StatusBadRequest, "invalid_input", "")
	case errors.Is(err, carenet.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
	case errors.Is(err, carenet.ErrConflict):
		writeError(w, r, http.StatusConflict, "invalid_transition", "")
	default:
		var ae *account.Error
		if errors.As(err, &ae) {
			s.v3Error(w, r, err)
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal", "")
	}
}

func forbidden(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
}

func (s *Server) adminNetworkOrgs(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermNetworkManage) && !p.Can(admin.PermNetworkVerify) {
		forbidden(w, r)
		return
	}
	if !s.networkReady(w, r) {
		return
	}
	orgs, err := s.opt.Network.Orgs(r.Context())
	if err != nil {
		s.networkError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, map[string]any{"orgs": orgs, "realPartners": s.opt.Network.AllowsReal()})
}

func (s *Server) adminNetworkCreateOrg(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermNetworkManage) {
		forbidden(w, r)
		return
	}
	if !s.networkReady(w, r) {
		return
	}
	var in carenet.OrgInput
	if !decode(w, r, &in) {
		return
	}
	org, err := s.opt.Network.CreateOrg(r.Context(), p.AdminID.String(), in)
	if err != nil {
		s.networkError(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"org": org})
}

func (s *Server) adminNetworkOrgStatus(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermNetworkVerify) {
		forbidden(w, r)
		return
	}
	if !s.networkReady(w, r) {
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &in) {
		return
	}
	org, err := s.opt.Network.SetOrgStatus(r.Context(), p.AdminID.String(), r.PathValue("id"), in.Status)
	if err != nil {
		s.networkError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"org": org})
}

func (s *Server) adminNetworkMembers(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermNetworkManage) && !p.Can(admin.PermNetworkVerify) {
		forbidden(w, r)
		return
	}
	if !s.networkReady(w, r) {
		return
	}
	members, err := s.opt.Network.Members(r.Context(), r.PathValue("id"))
	if err != nil {
		s.networkError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, map[string]any{"members": members})
}

func (s *Server) adminNetworkVerifyLicense(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermNetworkVerify) {
		forbidden(w, r)
		return
	}
	if !s.networkReady(w, r) {
		return
	}
	if err := s.opt.Network.VerifyLicense(r.Context(), p.AdminID.String(), r.PathValue("username")); err != nil {
		s.networkError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"username": r.PathValue("username"), "licenseVerified": true})
}

// partnerScope returns the caller's organisation, refusing anyone who is not
// a partner account: internal operators never act inside an organisation.
func partnerScope(w http.ResponseWriter, r *http.Request, p admin.Principal, perm string) (string, bool) {
	if !admin.IsPartnerRole(p.Role) || p.CareOrgID == "" || !p.Can(perm) {
		forbidden(w, r)
		return "", false
	}
	return p.CareOrgID, true
}

func (s *Server) adminPartnerOrg(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	org, ok := partnerScope(w, r, p, admin.PermCareRequestsRead)
	if !ok || !s.networkReady(w, r) {
		return
	}
	o, err := s.opt.Network.Org(r.Context(), org)
	if err != nil {
		s.networkError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"org": o, "displayName": p.DisplayName})
}

func (s *Server) adminPartnerRequests(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	org, ok := partnerScope(w, r, p, admin.PermCareRequestsRead)
	if !ok || !s.networkReady(w, r) {
		return
	}
	list, err := s.opt.Accounts.PartnerCareRequests(r.Context(), org)
	if err != nil {
		s.networkError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, map[string]any{"requests": list})
}

func (s *Server) adminPartnerRequest(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	org, ok := partnerScope(w, r, p, admin.PermCareRequestsRead)
	if !ok || !s.networkReady(w, r) {
		return
	}
	content := p.Can(admin.PermCareRequestsContent)
	req, err := s.opt.Accounts.PartnerCareRequest(r.Context(), org, r.PathValue("id"), content)
	if err != nil {
		s.networkError(w, r, err)
		return
	}
	if req.Summary != nil && s.opt.Admin != nil {
		if err := s.opt.Admin.Audit(r.Context(), &p.AdminID, "care_request.open", "care_request", req.ID, "", req.State, ""); err != nil {
			s.log.Warn("care_request.audit_failed")
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, map[string]any{"request": req, "canReadContent": content})
}

func (s *Server) adminPartnerRespond(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	org, ok := partnerScope(w, r, p, admin.PermCareRequestsRespond)
	if !ok || !s.networkReady(w, r) {
		return
	}
	var in account.PartnerRespondInput
	if !decode(w, r, &in) {
		return
	}
	req, err := s.opt.Accounts.PartnerRespond(r.Context(), org, r.PathValue("id"), in)
	if err != nil {
		s.networkError(w, r, err)
		return
	}
	if s.opt.Admin != nil {
		action := "care_request.propose"
		if in.Decision == "decline" {
			action = "care_request.decline"
		}
		if err := s.opt.Admin.Audit(r.Context(), &p.AdminID, action, "care_request", req.ID, "", req.State, in.DeclineReason); err != nil {
			s.log.Warn("care_request.audit_failed")
		}
	}
	writeData(w, http.StatusOK, map[string]any{"request": req})
}

func (s *Server) adminPartnerComplete(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	org, ok := partnerScope(w, r, p, admin.PermCareRequestsRespond)
	if !ok || !s.networkReady(w, r) {
		return
	}
	var in struct {
		ExpectedVersion int `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	req, err := s.opt.Accounts.PartnerComplete(r.Context(), org, r.PathValue("id"), in.ExpectedVersion)
	if err != nil {
		s.networkError(w, r, err)
		return
	}
	if s.opt.Admin != nil {
		if err := s.opt.Admin.Audit(r.Context(), &p.AdminID, "care_request.complete", "care_request", req.ID, "confirmed", req.State, ""); err != nil {
			s.log.Warn("care_request.audit_failed")
		}
	}
	writeData(w, http.StatusOK, map[string]any{"request": req})
}
