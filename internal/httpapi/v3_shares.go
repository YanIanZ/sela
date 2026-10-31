package httpapi

import (
	"net/http"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

// Recipient cookie names mirror the owner cookie: __Host- requires Secure,
// Path=/ and no Domain; plain-HTTP development uses the unprefixed name.
const (
	recipientCookie         = "__Host-sela_recipient_v3"
	insecureRecipientCookie = "sela_recipient_v3"
)

func (s *Server) recipientCookieName() string {
	if s.opt.SecureCookies {
		return recipientCookie
	}
	return insecureRecipientCookie
}

func (s *Server) v3CreateShare(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		SummaryID     string `json:"summaryId"`
		Label         string `json:"label"`
		Purpose       string `json:"purpose"`
		DurationHours int    `json:"durationHours"`
	}
	if !decode(w, r, &in) {
		return
	}
	consents, err := s.opt.Accounts.Consents(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	granted := false
	for _, c := range consents {
		if c.Purpose == "sharing" && c.Granted && !c.Outdated {
			granted = true
		}
	}
	if !granted {
		writeError(w, r, http.StatusForbidden, "consent_required", "sharing")
		return
	}
	share, token, code, err := s.opt.Accounts.CreateShare(r.Context(), p.UserID, in.SummaryID, in.Label, in.Purpose, in.DurationHours)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"share": share, "token": token, "code": code})
}

func (s *Server) v3Shares(w http.ResponseWriter, r *http.Request, p account.Principal) {
	list, err := s.opt.Accounts.Shares(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"shares": list})
}

func (s *Server) v3RevokeShare(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion int `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	share, err := s.opt.Accounts.RevokeShare(r.Context(), p.UserID, r.PathValue("id"), in.ExpectedVersion)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, share)
}

func (s *Server) v3RecipientSession(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
		Code  string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	projection, session, err := s.opt.Accounts.OpenShare(r.Context(), in.Token, in.Code)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	maxAge := time.Until(projection.ExpiresAt)
	if maxAge > 24*time.Hour {
		maxAge = 24 * time.Hour
	}
	if maxAge <= 0 {
		maxAge = time.Minute
	}
	s.setCookie(w, s.recipientCookieName(), session, "/api/v3/recipient", time.Now().Add(maxAge))
	writeData(w, http.StatusOK, map[string]any{"share": projection})
}

func (s *Server) v3RecipientShare(w http.ResponseWriter, r *http.Request) {
	projection, err := s.opt.Accounts.RecipientShare(r.Context(), s.recipientToken(r))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"share": projection})
}

func (s *Server) v3RecipientAcknowledge(w http.ResponseWriter, r *http.Request) {
	if err := s.opt.Accounts.AcknowledgeShare(r.Context(), s.recipientToken(r)); err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]bool{"acknowledged": true})
}

func (s *Server) recipientToken(r *http.Request) string {
	c, err := r.Cookie(s.recipientCookieName())
	if err != nil {
		return ""
	}
	return c.Value
}
