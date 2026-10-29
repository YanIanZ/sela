package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"strings"

	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/budget"
	"github.com/YanIanZ/sela/internal/fixtures"
)

// Operator cookies are deliberately separate from the anonymous-user
// cookie. A consumer session can never authorize an /api/admin/v1 route.
const (
	AdminCookie         = "__Host-sela_admin"
	InsecureAdminCookie = "sela_admin"
)

func (s *Server) adminRoutes() {
	m := s.mux
	m.HandleFunc("POST /api/admin/v1/sessions", s.mutation("admin-login", smallBody, s.adminLogin))
	m.HandleFunc("DELETE /api/admin/v1/sessions/current", s.mutation("mutation", smallBody, s.withAdmin(s.adminLogout)))
	m.HandleFunc("GET /api/admin/v1/me", s.read(s.withAdmin(s.adminMe)))
	m.HandleFunc("GET /api/admin/v1/overview", s.read(s.withAdmin(s.adminOverview)))
	m.HandleFunc("GET /api/admin/v1/audit", s.read(s.withAdmin(s.adminAudit)))
	m.HandleFunc("GET /api/admin/v1/staff", s.read(s.withAdmin(s.adminStaff)))
	m.HandleFunc("POST /api/admin/v1/staff/{username}/revoke", s.mutation("mutation", smallBody, s.withAdmin(s.adminRevokeStaff)))
	m.HandleFunc("GET /api/admin/v1/cache/health", s.read(s.withAdmin(s.adminCacheHealth)))
	m.HandleFunc("POST /api/admin/v1/cache/invalidate", s.mutation("mutation", smallBody, s.withAdmin(s.adminCacheInvalidate)))
}

func (s *Server) adminCookieName() string {
	if s.opt.SecureCookies {
		return AdminCookie
	}
	return InsecureAdminCookie
}

func (s *Server) adminToken(r *http.Request) string {
	c, err := r.Cookie(s.adminCookieName())
	if err != nil {
		return ""
	}
	return c.Value
}

func (s *Server) setAdminCookie(w http.ResponseWriter, token string, maxAge time.Duration) {
	c := &http.Cookie{Name: s.adminCookieName(), Value: token, Path: "/", HttpOnly: true, Secure: s.opt.SecureCookies, SameSite: http.SameSiteStrictMode}
	if maxAge > 0 {
		c.MaxAge = int(maxAge / time.Second)
	}
	http.SetCookie(w, c)
}

func (s *Server) clearAdminCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: s.adminCookieName(), Value: "", Path: "/", HttpOnly: true, Secure: s.opt.SecureCookies,
		SameSite: http.SameSiteStrictMode, MaxAge: -1})
}

type adminHandler func(w http.ResponseWriter, r *http.Request, p admin.Principal)

func (s *Server) withAdmin(h adminHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.opt.Admin == nil {
			writeError(w, r, http.StatusServiceUnavailable, "admin_unavailable", "")
			return
		}
		p, err := s.opt.Admin.AuthenticateSession(r.Context(), s.adminToken(r))
		if err != nil {
			if errors.Is(err, admin.ErrNotSignedIn) {
				writeError(w, r, http.StatusUnauthorized, "admin_not_signed_in", "")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal", "")
			return
		}
		h(w, r, p)
	}
}

// adminOverviewView flattens the privacy-safe overview and adds the
// durable AI spend aggregates when a budget is configured.
type adminOverviewView struct {
	admin.Overview
	AISpend       *budget.Summary    `json:"aiSpend,omitempty"`
	AISpendByTask []budget.TaskSpend `json:"aiSpendByTask,omitempty"`
}

func adminView(p admin.Principal) map[string]any {
	return map[string]any{
		"username":    p.Username,
		"role":        string(p.Role),
		"orgId":       p.OrgID,
		"permissions": admin.Permissions(p.Role),
	}
}

func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	if s.opt.Admin == nil {
		writeError(w, r, http.StatusServiceUnavailable, "admin_unavailable", "")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		TOTP     string `json:"totp"`
	}
	if !decode(w, r, &req) {
		return
	}
	sess, err := s.opt.Admin.Authenticate(r.Context(), req.Username, req.Password, req.TOTP)
	if err != nil {
		switch {
		case errors.Is(err, admin.ErrThrottled):
			writeError(w, r, http.StatusTooManyRequests, "admin_throttled", "")
		case errors.Is(err, admin.ErrInvalidCredentials):
			writeError(w, r, http.StatusUnauthorized, "admin_invalid_credentials", "")
		default:
			writeError(w, r, http.StatusInternalServerError, "internal", "")
		}
		return
	}
	s.setAdminCookie(w, sess.Token, time.Until(sess.Principal.AbsoluteExpiresAt))
	writeData(w, http.StatusOK, map[string]any{"admin": adminView(sess.Principal)})
}

func (s *Server) adminLogout(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if err := s.opt.Admin.Logout(r.Context(), p); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	s.clearAdminCookie(w)
	writeData(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) adminMe(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	writeData(w, http.StatusOK, map[string]any{"admin": adminView(p)})
}

func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermOverviewRead) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	o, err := s.opt.Admin.Overview(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	o.CacheEnabled = s.opt.Cache.Enabled()
	o.DatabaseReady = s.opt.Accounts != nil
	o.Version = s.opt.Version
	view := adminOverviewView{Overview: o}
	if s.opt.Budget != nil && s.opt.Budget.Enabled() {
		summary, err := s.opt.Budget.Summary(r.Context())
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal", "")
			return
		}
		view.AISpend = &summary
		byTask, err := s.opt.Budget.ByTask(r.Context(), 10)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal", "")
			return
		}
		view.AISpendByTask = byTask
	}
	writeData(w, http.StatusOK, view)
}

func (s *Server) adminAudit(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermAuditRead) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	events, err := s.opt.Admin.ListAudit(r.Context(), limit)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) adminStaff(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermStaffManage) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	staff, err := s.opt.Admin.ListStaff(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"staff": staff})
}

func (s *Server) adminRevokeStaff(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermStaffManage) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decodeOptional(w, r, &req) {
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "operator console"
	}
	username := r.PathValue("username")
	if err := s.opt.Admin.Revoke(r.Context(), &p.AdminID, username, reason); err != nil {
		switch {
		case errors.Is(err, admin.ErrLastOwner):
			writeError(w, r, http.StatusConflict, "admin_last_owner", "")
		case errors.Is(err, admin.ErrInvalid):
			writeError(w, r, http.StatusNotFound, "not_found", "")
		default:
			writeError(w, r, http.StatusInternalServerError, "internal", "")
		}
		return
	}
	writeData(w, http.StatusOK, map[string]any{"username": username, "status": "revoked"})
}

func (s *Server) adminCacheHealth(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermOverviewRead) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if s.opt.Cache == nil {
		writeData(w, http.StatusOK, map[string]any{"cache": map[string]any{"enabled": false, "ok": false}})
		return
	}
	writeData(w, http.StatusOK, map[string]any{"cache": s.opt.Cache.Health(r.Context())})
}

func (s *Server) adminCacheInvalidate(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !p.Can(admin.PermCacheWrite) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if s.opt.Cache == nil {
		writeError(w, r, http.StatusServiceUnavailable, "cache_disabled", "")
		return
	}
	key := s.opt.Cache.Key("help-content", fixtures.Version, "v1")
	if err := s.opt.Cache.Delete(r.Context(), key); err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "cache_disabled", "")
		return
	}
	if err := s.opt.Admin.Audit(r.Context(), &p.AdminID, "admin.cache_invalidate", "public_cache", "help-content", "", "invalidated", "operator request"); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"invalidated": "help-content"})
}
