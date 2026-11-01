package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/care"
)

// adminInviteRoutes are partner invitations and activation (D-079), the
// operator task counts for the console home and the partner team view.
func (s *Server) adminInviteRoutes() {
	m := s.mux
	m.HandleFunc("POST /api/admin/v1/network/invites", s.mutation("mutation", smallBody, s.withAdmin(s.adminCreateInvite)))
	m.HandleFunc("GET /api/admin/v1/network/orgs/{id}/invites", s.read(s.withAdmin(s.adminListInvites)))
	m.HandleFunc("POST /api/admin/v1/network/invites/{id}/revoke", s.mutation("mutation", smallBody, s.withAdmin(s.adminRevokeInvite)))
	// Activation is public: the one-time token in the request body is the
	// credential. It shares the login throttle.
	m.HandleFunc("POST /api/admin/v1/activation", s.mutation("admin-login", smallBody, s.adminActivationStart))
	m.HandleFunc("POST /api/admin/v1/activation/complete", s.mutation("admin-login", smallBody, s.adminActivationComplete))
	m.HandleFunc("GET /api/admin/v1/tasks", s.read(s.withAdmin(s.adminTasks)))
	m.HandleFunc("GET /api/admin/v1/partner/members", s.read(s.withAdmin(s.adminPartnerMembers)))
}

func (s *Server) inviteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, admin.ErrInvalid):
		writeError(w, r, http.StatusBadRequest, "invalid_input", "")
	case errors.Is(err, admin.ErrUsernameTaken):
		writeError(w, r, http.StatusConflict, "username_taken", "username")
	case errors.Is(err, admin.ErrInviteInvalid):
		writeError(w, r, http.StatusGone, "invite_invalid", "")
	case errors.Is(err, admin.ErrBadCode):
		writeError(w, r, http.StatusUnauthorized, "admin_invalid_credentials", "totp")
	default:
		writeError(w, r, http.StatusInternalServerError, "internal", "")
	}
}

func (s *Server) adminCreateInvite(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermNetworkManage) {
		forbidden(w, r)
		return
	}
	var in struct {
		Username         string `json:"username"`
		Role             string `json:"role"`
		CareOrgID        string `json:"careOrgId"`
		DisplayName      string `json:"displayName"`
		ProfessionalType string `json:"professionalType"`
		LicenseNo        string `json:"licenseNo"`
	}
	if !decode(w, r, &in) {
		return
	}
	inv, token, err := s.opt.Admin.CreateInvite(r.Context(), p.AdminID, admin.PartnerInput{
		Username: strings.TrimSpace(in.Username), Role: admin.Role(in.Role), CareOrgID: in.CareOrgID,
		DisplayName: in.DisplayName, ProfessionalType: in.ProfessionalType, LicenseNo: in.LicenseNo,
	})
	if err != nil {
		s.inviteError(w, r, err)
		return
	}
	origin := strings.TrimRight(s.opt.PublicOrigin, "/")
	if origin == "" {
		origin = "http://" + r.Host
	}
	// The token travels in the URL fragment, so it never reaches a server
	// log, a proxy or a Referer header.
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusCreated, map[string]any{"invite": inv, "activationUrl": origin + "/operator/activate#" + token})
}

func (s *Server) adminListInvites(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermNetworkManage) && !p.Can(admin.PermNetworkVerify) {
		forbidden(w, r)
		return
	}
	list, err := s.opt.Admin.ListInvites(r.Context(), r.PathValue("id"))
	if err != nil {
		s.inviteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, map[string]any{"invites": list})
}

func (s *Server) adminRevokeInvite(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermNetworkManage) {
		forbidden(w, r)
		return
	}
	if err := s.opt.Admin.RevokeInvite(r.Context(), p.AdminID, r.PathValue("id")); err != nil {
		s.inviteError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"revoked": true})
}

func (s *Server) adminActivationStart(w http.ResponseWriter, r *http.Request) {
	if s.opt.Admin == nil {
		writeError(w, r, http.StatusServiceUnavailable, "admin_unavailable", "")
		return
	}
	var in struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &in) {
		return
	}
	act, err := s.opt.Admin.StartActivation(r.Context(), in.Token)
	if err != nil {
		s.inviteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, map[string]any{"activation": act})
}

func (s *Server) adminActivationComplete(w http.ResponseWriter, r *http.Request) {
	if s.opt.Admin == nil {
		writeError(w, r, http.StatusServiceUnavailable, "admin_unavailable", "")
		return
	}
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
		TOTP     string `json:"totp"`
	}
	if !decode(w, r, &in) {
		return
	}
	sess, err := s.opt.Admin.CompleteActivation(r.Context(), in.Token, in.Password, in.TOTP)
	if err != nil {
		if errors.Is(err, admin.ErrInvalid) {
			writeError(w, r, http.StatusBadRequest, "password_weak", "password")
			return
		}
		s.inviteError(w, r, err)
		return
	}
	s.setAdminCookie(w, sess.Token, time.Until(sess.Principal.AbsoluteExpiresAt))
	writeData(w, http.StatusOK, map[string]any{"admin": adminView(sess.Principal)})
}

// adminTasks counts what waits for this operator, per permission held.
func (s *Server) adminTasks(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if admin.IsPartnerRole(p.Role) {
		forbidden(w, r)
		return
	}
	out := map[string]any{}
	if (p.Can(admin.PermNetworkManage) || p.Can(admin.PermNetworkVerify)) && s.opt.Network != nil {
		t, err := s.opt.Network.Tasks(r.Context())
		if err != nil {
			s.networkError(w, r, err)
			return
		}
		out["network"] = t
	}
	if p.Can(admin.PermCommunityModerate) && s.opt.Community != nil {
		items, err := s.opt.Community.Queue(r.Context(), 50)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal", "")
			return
		}
		out["moderation"] = len(items)
	}
	if p.Can(admin.PermDirectoryVerify) && s.opt.Care != nil {
		pending, err := s.opt.Care.List(r.Context(), care.StatusPending)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal", "")
			return
		}
		out["directoryPending"] = len(pending)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, map[string]any{"tasks": out})
}

// adminPartnerMembers is the partner's own team. Licence numbers are shown
// only to the clinic administrator.
func (s *Server) adminPartnerMembers(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	org, ok := partnerScope(w, r, p, admin.PermCareRequestsRead)
	if !ok || !s.networkReady(w, r) {
		return
	}
	members, err := s.opt.Network.Members(r.Context(), org)
	if err != nil {
		s.networkError(w, r, err)
		return
	}
	if p.Role != admin.RolePartnerAdmin {
		for i := range members {
			members[i].LicenseNo = ""
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, map[string]any{"members": members})
}
