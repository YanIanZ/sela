package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

func (s *Server) v3Policies(w http.ResponseWriter, r *http.Request) {
	writeData(w, http.StatusOK, map[string]any{"policies": s.v3.policies})
}

func (s *Server) v3AuthOptions(w http.ResponseWriter, r *http.Request) {
	c := s.v3.cfg
	bands := c.AllowedAgeBands
	if bands == nil {
		bands = []string{}
	}
	writeData(w, http.StatusOK, map[string]any{
		"username":      map[string]int{"minLength": account.UsernameMinLength, "maxLength": account.UsernameMaxLength},
		"password":      map[string]int{"minLength": account.PasswordMinLength, "maxLength": account.PasswordMaxLength},
		"recoveryCodes": map[string]int{"count": account.RecoveryCodeCount, "groups": 4, "groupLength": 4},
		"ageBands":      map[string]any{"allowed": bands},
		"guest": map[string]any{"available": s.opt.Accounts != nil, "inactiveTtlHours": int(c.GuestInactiveTTL / time.Hour),
			"maxAgeDays": int(c.GuestMaxAge / (24 * time.Hour))},
		"sessions": map[string]int{"idleHours": int(c.SessionIdle / time.Hour), "absoluteHours": int(c.SessionAbsolute / time.Hour),
			"trustedIdleDays": int(c.TrustedIdle / (24 * time.Hour)), "trustedAbsoluteDays": int(c.TrustedAbsolute / (24 * time.Hour))},
		"guardianConsentRequired": c.GuardianConsentRequired,
		"environment":             c.Environment,
		"synthetic":               c.Synthetic(),
	})
}

// signedIn answers a sign-in path: the cookie is set (rotation) and the
// one-time secrets appear only in this response.
func (s *Server) signedIn(w http.ResponseWriter, status int, res account.AuthResult, body map[string]any) {
	s.setAuthCookie(w, res.Token, res.CookieMaxAge)
	if body == nil {
		body = map[string]any{}
	}
	body["user"], body["session"] = res.Profile, res.Session
	writeData(w, status, body)
}

type registerBody struct {
	Username           string `json:"username"`
	Password           string `json:"password"`
	Locale             string `json:"locale"`
	AgeBand            string `json:"ageBand"`
	TermsPolicyVersion string `json:"termsPolicyVersion"`
	RememberDevice     bool   `json:"rememberDevice"`
}

func (s *Server) v3Register(w http.ResponseWriter, r *http.Request) {
	var in registerBody
	if !decode(w, r, &in) {
		return
	}
	if !s.limitBy(w, r, rlRegister, s.clientKey(r)) {
		s.logAuth(w, r, "register", account.ErrRateLimited)
		return
	}
	res, err := s.opt.Accounts.Register(r.Context(), account.RegisterInput{Username: in.Username, Password: in.Password, Locale: in.Locale,
		AgeBand: in.AgeBand, TermsPolicyVersion: in.TermsPolicyVersion, RememberDevice: in.RememberDevice,
		PriorToken: s.authToken(r), UserAgent: r.UserAgent()})
	s.logAuth(w, r, "register", err)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	s.signedIn(w, http.StatusCreated, res, map[string]any{"recoveryCodes": res.RecoveryCodes})
}

type loginBody struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	RememberDevice bool   `json:"rememberDevice"`
}

func (s *Server) v3Login(w http.ResponseWriter, r *http.Request) {
	var in loginBody
	if !decode(w, r, &in) {
		return
	}
	if !s.limitBy(w, r, rlLoginClient, s.clientKey(r)) || !s.limitBy(w, r, rlLoginUser, s.usernameKey(in.Username)) {
		s.logAuth(w, r, "login", account.ErrRateLimited)
		return
	}
	res, err := s.opt.Accounts.Login(r.Context(), account.LoginInput{Username: in.Username, Password: in.Password,
		RememberDevice: in.RememberDevice, PriorToken: s.authToken(r), UserAgent: r.UserAgent()})
	s.logAuth(w, r, "login", err)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	s.signedIn(w, http.StatusOK, res, nil)
}

// v3Logout is idempotent: without a (valid) session it still answers 200
// and clears the cookie.
func (s *Server) v3Logout(w http.ResponseWriter, r *http.Request) {
	var in struct{}
	if !decodeOptional(w, r, &in) {
		return
	}
	if tok := s.authToken(r); tok != "" {
		p, err := s.opt.Accounts.Authenticate(r.Context(), tok)
		switch {
		case err == nil:
			s.v3.listening.purge(p, false)
			if err := s.opt.Accounts.Logout(r.Context(), p.UserID, p.SessionID); err != nil {
				s.v3Error(w, r, err)
				return
			}
		case account.CodeOf(err) != "not_signed_in":
			// Storage failed: the session may still be live, so do not
			// claim a sign-out.
			s.v3Error(w, r, err)
			return
		}
	}
	s.clearAuthCookie(w)
	writeData(w, http.StatusOK, map[string]any{"signedOut": true})
}

func (s *Server) v3LogoutAll(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		KeepCurrent bool `json:"keepCurrent"`
	}
	if !decodeOptional(w, r, &in) {
		return
	}
	if in.KeepCurrent {
		s.v3.listening.purgeOthers(p)
	} else {
		s.v3.listening.purge(p, true)
	}
	n, err := s.opt.Accounts.LogoutAll(r.Context(), p.UserID, p.SessionID, in.KeepCurrent)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	if !in.KeepCurrent {
		s.clearAuthCookie(w)
	}
	writeData(w, http.StatusOK, map[string]any{"revoked": n, "signedOut": !in.KeepCurrent})
}

type guestBody struct {
	Locale             string `json:"locale"`
	AgeBand            string `json:"ageBand"`
	TermsPolicyVersion string `json:"termsPolicyVersion"`
	GuestNoticeVersion string `json:"guestNoticeVersion"`
}

func (s *Server) v3Guest(w http.ResponseWriter, r *http.Request) {
	var in guestBody
	if !decode(w, r, &in) {
		return
	}
	if !s.limitBy(w, r, rlGuest, s.clientKey(r)) {
		s.logAuth(w, r, "guest_start", account.ErrRateLimited)
		return
	}
	res, err := s.opt.Accounts.StartGuest(r.Context(), account.GuestInput{Locale: in.Locale, AgeBand: in.AgeBand,
		TermsPolicyVersion: in.TermsPolicyVersion, GuestNoticeVersion: in.GuestNoticeVersion, PriorToken: s.authToken(r), UserAgent: r.UserAgent()})
	s.logAuth(w, r, "guest_start", err)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	s.signedIn(w, http.StatusCreated, res, nil)
}

func (s *Server) v3Upgrade(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in loginBody
	if !decode(w, r, &in) {
		return
	}
	if !s.limitBy(w, r, rlRegister, s.clientKey(r)) {
		s.logAuth(w, r, "guest_upgrade", account.ErrRateLimited)
		return
	}
	res, err := s.opt.Accounts.UpgradeGuest(r.Context(), p.UserID, p.SessionID, account.UpgradeInput{Username: in.Username,
		Password: in.Password, RememberDevice: in.RememberDevice, UserAgent: r.UserAgent()})
	s.logAuth(w, r, "guest_upgrade", err)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	s.v3.listening.purge(p, false)
	s.signedIn(w, http.StatusOK, res, map[string]any{"recoveryCodes": res.RecoveryCodes})
}

type recoveryBody struct {
	Username       string `json:"username"`
	RecoveryCode   string `json:"recoveryCode"`
	NewPassword    string `json:"newPassword"`
	RememberDevice bool   `json:"rememberDevice"`
}

func (s *Server) v3Recovery(w http.ResponseWriter, r *http.Request) {
	var in recoveryBody
	if !decode(w, r, &in) {
		return
	}
	if !s.limitBy(w, r, rlRecoveryClient, s.clientKey(r)) || !s.limitBy(w, r, rlRecoveryUser, s.usernameKey(in.Username)) {
		s.logAuth(w, r, "recovery", account.ErrRateLimited)
		return
	}
	res, err := s.opt.Accounts.Recover(r.Context(), account.RecoveryInput{Username: in.Username, RecoveryCode: in.RecoveryCode,
		NewPassword: in.NewPassword, RememberDevice: in.RememberDevice, PriorToken: s.authToken(r), UserAgent: r.UserAgent()})
	s.logAuth(w, r, "recovery", err)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	if id, e := account.ParseUUID(res.Profile.ID); e == nil {
		s.v3.listening.purge(account.Principal{UserID: account.UserID(id)}, true)
	}
	s.signedIn(w, http.StatusOK, res, map[string]any{"remainingRecoveryCodes": res.RemainingRecoveryCodes})
}

func (s *Server) sessionSecretLimit(w http.ResponseWriter, r *http.Request, p account.Principal, action string) bool {
	if p.Kind != account.KindAccount {
		writeError(w, r, http.StatusForbidden, "account_required", "")
		return false
	}
	if !s.limitBy(w, r, rlSessionSecret, idKey(p.SessionID)) {
		s.logAuth(w, r, action, account.ErrRateLimited)
		return false
	}
	return true
}

func (s *Server) v3Reauth(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &in) || !s.sessionSecretLimit(w, r, p, "reauth") {
		return
	}
	until, err := s.opt.Accounts.Reauthenticate(r.Context(), p.UserID, p.SessionID, in.Password)
	s.logAuth(w, r, "reauth", err)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"reauthenticatedUntil": account.Timestamp(until)})
}

func (s *Server) v3ChangePassword(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decode(w, r, &in) || !s.sessionSecretLimit(w, r, p, "password_change") {
		return
	}
	res, err := s.opt.Accounts.ChangePassword(r.Context(), p.UserID, p.SessionID, in.CurrentPassword, in.NewPassword, r.UserAgent())
	s.logAuth(w, r, "password_change", err)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	s.v3.listening.purgeOthers(p)
	s.setAuthCookie(w, res.Token, res.CookieMaxAge)
	writeData(w, http.StatusOK, map[string]any{"session": res.Session, "revokedSessions": res.RevokedSessions})
}

func (s *Server) v3RegenerateCodes(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &in) || !s.sessionSecretLimit(w, r, p, "recovery_regenerate") {
		return
	}
	codes, at, err := s.opt.Accounts.RegenerateRecoveryCodes(r.Context(), p.UserID, p.SessionID, in.Password)
	s.logAuth(w, r, "recovery_regenerate", err)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"recoveryCodes": codes, "generatedAt": account.Timestamp(at)})
}

func (s *Server) v3Me(w http.ResponseWriter, r *http.Request, p account.Principal) {
	prof, err := s.opt.Accounts.Profile(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, prof)
}

func (s *Server) v3PatchMe(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion *int    `json:"expectedVersion"`
		Locale          *string `json:"locale"`
		AgeBand         *string `json:"ageBand"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.ExpectedVersion == nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "expectedVersion")
		return
	}
	prof, err := s.opt.Accounts.UpdateProfile(r.Context(), p.UserID, p.SessionID, account.ProfileUpdate{ExpectedVersion: *in.ExpectedVersion,
		Locale: in.Locale, AgeBand: in.AgeBand})
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, prof)
}

func (s *Server) v3AcknowledgeCodes(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct{}
	if !decodeOptional(w, r, &in) {
		return
	}
	prof, err := s.opt.Accounts.AcknowledgeRecoveryCodes(r.Context(), p.UserID, p.SessionID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, prof)
}

func (s *Server) v3Sessions(w http.ResponseWriter, r *http.Request, p account.Principal) {
	list, err := s.opt.Accounts.Sessions(r.Context(), p.UserID, p.SessionID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"sessions": list})
}

func (s *Server) v3RevokeSession(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct{}
	if !decodeOptional(w, r, &in) {
		return
	}
	id, err := account.ParseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	current, err := s.opt.Accounts.RevokeSession(r.Context(), p.UserID, p.SessionID, account.SessionID(id))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	s.v3.listening.purgeSession(p.UserID, account.SessionID(id))
	if current {
		s.clearAuthCookie(w)
	}
	writeData(w, http.StatusOK, map[string]any{"revoked": true, "current": current})
}

func (s *Server) v3Consents(w http.ResponseWriter, r *http.Request, p account.Principal) {
	list, err := s.opt.Accounts.Consents(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"consents": list})
}

func (s *Server) v3PutConsent(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Granted       *bool  `json:"granted"`
		PolicyVersion string `json:"policyVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Granted == nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "granted")
		return
	}
	s.v3.listening.mu.Lock()
	defer s.v3.listening.mu.Unlock()
	c, changed, err := s.opt.Accounts.SetConsent(r.Context(), p.UserID, p.SessionID, r.PathValue("purpose"), *in.Granted, in.PolicyVersion)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	if changed {
		for _, room := range s.v3.listening.rooms {
			if room.owner == p.UserID {
				if room.cancel != nil {
					room.cancel()
				}
				room.Version++
				room.AIState = "consent_changed"
			}
		}
	}
	writeData(w, http.StatusOK, map[string]any{"consent": c, "changed": changed})
}

// page reads ?limit= and ?cursor=; a malformed limit is 400 (field limit).
func page(w http.ResponseWriter, r *http.Request) (account.Page, bool) {
	q := r.URL.Query()
	var pg account.Page
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > account.MaxPageLimit {
			writeError(w, r, http.StatusBadRequest, "invalid_input", "limit")
			return pg, false
		}
		pg.Limit = n
	}
	pg.Cursor = q.Get("cursor")
	return pg, true
}

func nextCursor(c string) any {
	if c == "" {
		return nil
	}
	return c
}

func (s *Server) v3ConsentHistory(w http.ResponseWriter, r *http.Request, p account.Principal) {
	pg, ok := page(w, r)
	if !ok {
		return
	}
	events, next, err := s.opt.Accounts.ConsentHistory(r.Context(), p.UserID, r.URL.Query().Get("purpose"), pg)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"events": events, "nextCursor": nextCursor(next)})
}

func (s *Server) v3GuardianConsent(w http.ResponseWriter, r *http.Request, p account.Principal) {
	v, err := s.opt.Accounts.GuardianConsent(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, v)
}

func (s *Server) v3PutGuardianConsent(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Relationship  string  `json:"relationship"`
		Method        string  `json:"method"`
		GuardianName  *string `json:"guardianName"`
		PolicyVersion string  `json:"policyVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	rec, err := s.opt.Accounts.SetGuardianConsent(r.Context(), p.UserID, p.SessionID, account.GuardianInput{Relationship: in.Relationship,
		Method: in.Method, GuardianName: in.GuardianName, PolicyVersion: in.PolicyVersion})
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"active": rec})
}

func (s *Server) v3DeleteGuardianConsent(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct{}
	if !decodeOptional(w, r, &in) {
		return
	}
	if err := s.opt.Accounts.WithdrawGuardianConsent(r.Context(), p.UserID, p.SessionID); err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"active": nil})
}

func (s *Server) v3Audit(w http.ResponseWriter, r *http.Request, p account.Principal) {
	pg, ok := page(w, r)
	if !ok {
		return
	}
	events, next, err := s.opt.Accounts.AuditEvents(r.Context(), p.UserID, p.SessionID, pg)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"events": events, "nextCursor": nextCursor(next)})
}

// exportCSP lets the HTML export render its inline style and nothing else.
const exportCSP = "default-src 'none'; style-src 'unsafe-inline'; img-src data:; sandbox"

func (s *Server) recentProof(p account.Principal) bool {
	now := time.Now()
	if s.v3.cfg.Now != nil {
		now = s.v3.cfg.Now()
	}
	return p.Kind == account.KindGuest || now.Sub(p.AuthenticatedAt) <= s.v3.cfg.ReauthWindow
}

func (s *Server) v3Export(w http.ResponseWriter, r *http.Request, p account.Principal) {
	q := r.URL.Query()
	format := q.Get("format")
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "html" && format != "md" {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "format")
		return
	}
	if l := q.Get("lang"); l != "" && l != "id" && l != "en" {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "lang")
		return
	}
	// The store checks the reauth window again; checking first means a
	// refused attempt does not use up the export quota.
	if !s.recentProof(p) {
		writeError(w, r, http.StatusForbidden, "reauth_required", "")
		return
	}
	if !s.limitBy(w, r, rlExport, idKey(p.UserID)) {
		return
	}
	f, err := s.opt.Accounts.Export(r.Context(), p.UserID, p.SessionID, format, q.Get("lang"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", f.ContentType)
	h.Set("Content-Disposition", `attachment; filename="`+f.Filename+`"`)
	h.Set("Cache-Control", "no-store")
	if format == "html" {
		h.Set("Content-Security-Policy", exportCSP)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(f.Body)
}

func (s *Server) v3DeleteMe(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Confirm bool `json:"confirm"`
	}
	if !decodeOptional(w, r, &in) {
		return
	}
	if !in.Confirm {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "confirm")
		return
	}
	s.v3.listening.purge(p, true)
	rec, err := s.opt.Accounts.DeleteAccount(r.Context(), p.UserID, p.SessionID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	s.clearAuthCookie(w)
	writeData(w, http.StatusOK, rec)
}
