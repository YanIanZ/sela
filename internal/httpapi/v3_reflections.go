package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/safety"
)

// v3CreateReflection runs AT-2 over the room's recent turns and adds
// "Sela menangkap…" cards. The quote is always the user's own words; the
// interpretation stays unreviewed until the user confirms, corrects or
// rejects it.
func (s *Server) v3CreateReflection(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion int `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	l := s.v3.listening
	l.mu.Lock()
	c := l.owned(r.PathValue("id"), p)
	if c == nil {
		l.mu.Unlock()
		writeError(w, r, 404, "not_found", "")
		return
	}
	if c.Busy {
		l.mu.Unlock()
		writeError(w, r, 409, "invalid_transition", "")
		return
	}
	if c.Version != in.ExpectedVersion {
		l.mu.Unlock()
		writeError(w, r, 409, "stale_version", "")
		return
	}
	if len(c.Reflections) >= 20 {
		l.mu.Unlock()
		writeError(w, r, 409, "limit_reached", "")
		return
	}
	if s.opt.AIRouter == nil {
		l.mu.Unlock()
		writeError(w, r, 503, "store_unavailable", "")
		return
	}
	consents, err := s.opt.Accounts.Consents(r.Context(), p.UserID)
	if err != nil {
		l.mu.Unlock()
		s.v3Error(w, r, err)
		return
	}
	granted := false
	for _, consent := range consents {
		if consent.Purpose == "ai_processing" && consent.Granted && !consent.Outdated {
			granted = true
		}
	}
	if !granted {
		l.mu.Unlock()
		writeError(w, r, 409, "consent_required", "")
		return
	}
	profile, err := s.opt.Accounts.Profile(r.Context(), p.UserID)
	if err != nil {
		l.mu.Unlock()
		s.v3Error(w, r, err)
		return
	}
	// Take a stable copy of the turns the model may see. Crisis turns are
	// never sent (AT-0/AT-1 rule) and the newest 12 turns bound the input.
	turns := append([]account.ConversationTurn{}, c.Turns...)
	version := c.Version
	l.mu.Unlock()

	spans := []ai.Span{}
	quotes := map[string]string{}
	start := max(0, len(turns)-12)
	for i, t := range turns[start:] {
		role := ai.RoleUser
		if t.Role == "assistant" {
			role = ai.RoleEarlierReply
		}
		if t.Role == "user" && safety.Assess(t.Text).Level == safety.LevelCrisis {
			continue
		}
		key := strconv.Itoa(start + i)
		spans = append(spans, ai.Span{Key: key, Role: role, Text: t.Text})
		if t.Role == "user" {
			quotes[key] = t.Text
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	output, err := s.opt.AIRouter.Run(ctx, ai.TaskReflection, ai.Input{
		Subject: ai.Subject(p.UserID.String()), Language: profile.Locale, Spans: spans, Synthetic: s.v3.cfg.Synthetic(),
		PrivateNames: s.privateNamesFor(ctx, p.UserID, "reflection"),
		Commit: func(ctx context.Context) error {
			principal, e := s.opt.Accounts.Authenticate(ctx, s.authToken(r))
			if e != nil || principal.UserID != p.UserID {
				return errors.New("session ended")
			}
			l.mu.Lock()
			defer l.mu.Unlock()
			live := l.owned(c.ID, p)
			if live == nil || live.Version != version || live.Paused {
				return errors.New("stale conversation")
			}
			return nil
		},
	})
	if err != nil {
		s.log.Warn("ai_run_failed", "task", "reflection", "error", err.Error())
		l.mu.Lock()
		if live := l.owned(c.ID, p); live != nil {
			live.AIState = "unavailable"
			out := roomCopy(live)
			l.mu.Unlock()
			writeData(w, http.StatusOK, out)
			return
		}
		l.mu.Unlock()
		writeError(w, r, 409, "invalid_transition", "")
		return
	}

	now := time.Now()
	added := make([]account.Reflection, 0, len(output.Items))
	for _, item := range output.Items {
		quote := ""
		turnID := ""
		for _, key := range item.SpanKeys {
			if q, ok := quotes[key]; ok {
				quote, turnID = q, key
				break
			}
		}
		if strings.TrimSpace(quote) == "" || strings.TrimSpace(item.Text) == "" {
			continue
		}
		added = append(added, account.Reflection{
			ID: newReflectionID(), TurnID: turnID, Quote: quote, Text: item.Text,
			ReviewStatus: account.ReflectionUnreviewed, Provenance: "unconfirmed_interpretation",
			CreatedAt: now, UpdatedAt: now,
		})
	}
	l.mu.Lock()
	live := l.owned(c.ID, p)
	if live == nil || live.Version != version {
		l.mu.Unlock()
		writeError(w, r, 409, "stale_version", "")
		return
	}
	if len(added) > 0 {
		live.Reflections = append(live.Reflections, added...)
		live.Version++
		if err := s.persistRoom(r.Context(), live); err != nil {
			live.Reflections = live.Reflections[:len(live.Reflections)-len(added)]
			live.Version--
			l.mu.Unlock()
			s.v3Error(w, r, err)
			return
		}
	}
	out := roomCopy(live)
	l.mu.Unlock()
	writeData(w, http.StatusOK, out)
}

// v3ReviewReflection applies the user's review to one card. A correction is
// stored as the user's own wording; confirm/reject/undo keep it.
func (s *Server) v3ReviewReflection(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion int    `json:"expectedVersion"`
		Action          string `json:"action"`
		Text            string `json:"text"`
	}
	if !decode(w, r, &in) {
		return
	}
	l := s.v3.listening
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.owned(r.PathValue("id"), p)
	if c == nil {
		writeError(w, r, 404, "not_found", "")
		return
	}
	if c.Version != in.ExpectedVersion {
		writeError(w, r, 409, "stale_version", "")
		return
	}
	idx := -1
	for i := range c.Reflections {
		if c.Reflections[i].ID == r.PathValue("reflectionId") {
			idx = i
			break
		}
	}
	if idx < 0 {
		writeError(w, r, 404, "not_found", "")
		return
	}
	status, provenance := "", ""
	switch in.Action {
	case "confirm":
		status = account.ReflectionConfirmed
	case "reject":
		status = account.ReflectionRejected
	case "undo":
		status = account.ReflectionUnreviewed
	case "correct":
		text := strings.TrimSpace(in.Text)
		if text == "" || len([]rune(text)) > 1200 {
			writeError(w, r, 400, "invalid_input", "text")
			return
		}
		status, provenance = account.ReflectionCorrected, "user_correction"
		c.Reflections[idx].Text = text
	default:
		writeError(w, r, 400, "invalid_input", "action")
		return
	}
	if status != "" {
		c.Reflections[idx].ReviewStatus = status
	}
	if provenance != "" {
		c.Reflections[idx].Provenance = provenance
	}
	c.Reflections[idx].UpdatedAt = time.Now()
	c.Version++
	if err := s.persistRoom(r.Context(), c); err != nil {
		c.Version--
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, roomCopy(c))
}

// newReflectionID returns an opaque per-room id (16 random hex bytes).
func newReflectionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
