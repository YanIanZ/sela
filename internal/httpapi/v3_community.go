package httpapi

import (
	"errors"
	"net/http"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/community"
	"github.com/YanIanZ/sela/internal/safety"
)

// communityAllowed checks the store, the adult age band and the community
// consent. It writes the refusal itself.
func (s *Server) communityAllowed(w http.ResponseWriter, r *http.Request, p account.Principal) bool {
	if s.opt.Community == nil {
		writeError(w, r, http.StatusServiceUnavailable, "store_unavailable", "")
		return false
	}
	prof, err := s.opt.Accounts.Profile(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return false
	}
	if prof.AgeBand != account.AgeBand18Plus {
		writeError(w, r, http.StatusForbidden, "age_not_supported", "community")
		return false
	}
	consents, err := s.opt.Accounts.Consents(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return false
	}
	for _, c := range consents {
		if c.Purpose == account.PolicyCommunity && c.Granted && !c.Outdated {
			return true
		}
	}
	writeError(w, r, http.StatusForbidden, "consent_required", account.PolicyCommunity)
	return false
}

func (s *Server) communityError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, community.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "")
	case errors.Is(err, community.ErrInvalid):
		writeError(w, r, http.StatusBadRequest, "invalid_input", "")
	case errors.Is(err, community.ErrRateLimit):
		writeError(w, r, http.StatusTooManyRequests, "rate_limited", "")
	case errors.Is(err, community.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "")
	default:
		s.log.Warn("community.error")
		writeError(w, r, http.StatusInternalServerError, "internal", "")
	}
}

func (s *Server) moderator() community.Moderator {
	if s.opt.Moderator != nil {
		return s.opt.Moderator
	}
	return community.MetaModerator{}
}

// moderate runs the safety router and moderation. On crisis it answers 422
// crisis itself (the app opens quick-help mode) and returns ok=false.
func (s *Server) moderate(w http.ResponseWriter, r *http.Request, body string, max int, contextText string) (community.Draft, safety.Level, bool) {
	text, level, err := community.Prepare(body, max)
	if errors.Is(err, community.ErrCrisis) {
		writeError(w, r, http.StatusUnprocessableEntity, "crisis", "body")
		return community.Draft{}, level, false
	}
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "body")
		return community.Draft{}, level, false
	}
	v := s.moderator().Moderate(r.Context(), text, contextText)
	s.log.Info("community.moderated", "status", string(v.Status), "source", v.Source, "labels", len(v.Labels))
	return community.Draft{Body: text, Status: v.Status, Labels: v.Labels}, level, true
}

func (s *Server) v3CommunityFeed(w http.ResponseWriter, r *http.Request, p account.Principal) {
	if !s.communityAllowed(w, r, p) {
		return
	}
	posts, err := s.opt.Community.Feed(r.Context(), p.UserID, r.URL.Query().Get("topic"), 30)
	if err != nil {
		s.communityError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"posts": posts})
}

func (s *Server) v3CommunityCreatePost(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Body  string `json:"body"`
		Topic string `json:"topic"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !s.communityAllowed(w, r, p) {
		return
	}
	d, level, ok := s.moderate(w, r, in.Body, community.MaxPostRunes, "")
	if !ok {
		return
	}
	d.Topic = in.Topic
	post, err := s.opt.Community.CreatePost(r.Context(), p.UserID, d)
	if err != nil {
		s.communityError(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"post": post, "support": level == safety.LevelElevated})
}

func (s *Server) v3CommunityThread(w http.ResponseWriter, r *http.Request, p account.Principal) {
	if !s.communityAllowed(w, r, p) {
		return
	}
	post, replies, err := s.opt.Community.Thread(r.Context(), p.UserID, r.PathValue("id"))
	if err != nil {
		s.communityError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"post": post, "replies": replies})
}

func (s *Server) v3CommunityCreateReply(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Body string `json:"body"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !s.communityAllowed(w, r, p) {
		return
	}
	post, _, err := s.opt.Community.Thread(r.Context(), p.UserID, r.PathValue("id"))
	if err != nil || post.Status != community.StatusPublished {
		writeError(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	d, level, ok := s.moderate(w, r, in.Body, community.MaxReplyRunes, post.Body)
	if !ok {
		return
	}
	reply, err := s.opt.Community.CreateReply(r.Context(), p.UserID, post.ID, d)
	if err != nil {
		s.communityError(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"reply": reply, "support": level == safety.LevelElevated})
}

func (s *Server) v3CommunityReport(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Kind   string `json:"kind"`
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !s.communityAllowed(w, r, p) {
		return
	}
	if err := s.opt.Community.Report(r.Context(), p.UserID, in.Kind, in.ID, in.Reason); err != nil {
		s.communityError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"reported": true})
}

func (s *Server) v3CommunityDeletePost(w http.ResponseWriter, r *http.Request, p account.Principal) {
	s.communityDelete(w, r, p, "post")
}

func (s *Server) v3CommunityDeleteReply(w http.ResponseWriter, r *http.Request, p account.Principal) {
	s.communityDelete(w, r, p, "reply")
}

func (s *Server) communityDelete(w http.ResponseWriter, r *http.Request, p account.Principal, kind string) {
	if s.opt.Community == nil {
		writeError(w, r, http.StatusServiceUnavailable, "store_unavailable", "")
		return
	}
	if err := s.opt.Community.DeleteOwn(r.Context(), p.UserID, kind, r.PathValue("id")); err != nil {
		s.communityError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
