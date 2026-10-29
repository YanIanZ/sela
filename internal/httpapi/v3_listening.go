package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/budget"
	"github.com/YanIanZ/sela/internal/safety"
)

type room struct {
	ID            string                     `json:"id"`
	Save          bool                       `json:"save"`
	Version       int                        `json:"version"`
	Turns         []account.ConversationTurn `json:"turns"`
	Paused        bool                       `json:"paused"`
	Busy          bool                       `json:"busy"`
	AIState       string                     `json:"aiState"`
	RecordID      string                     `json:"recordId,omitempty"`
	owner         account.UserID
	session       account.SessionID
	created, last time.Time
	recordVersion int
	timer         *time.Timer
	cancel        context.CancelFunc
}
type listeningStore struct {
	mu    sync.Mutex
	rooms map[string]*room
	usage map[account.UserID]roomUsage
}
type roomUsage struct {
	at time.Time
	n  int
}

func newListeningStore() *listeningStore {
	return &listeningStore{rooms: map[string]*room{}, usage: map[account.UserID]roomUsage{}}
}
func (l *listeningStore) sweep() {
	now := time.Now()
	for id, c := range l.rooms {
		if now.Sub(c.last) > 30*time.Minute || now.Sub(c.created) > 2*time.Hour {
			if c.cancel != nil {
				c.cancel()
			}
			if c.timer != nil {
				c.timer.Stop()
			}
			delete(l.rooms, id)
		}
	}
	for id, u := range l.usage {
		if now.Sub(u.at) > 10*time.Minute {
			delete(l.usage, id)
		}
	}
}
func (l *listeningStore) owned(id string, p account.Principal) *room {
	l.sweep()
	c := l.rooms[id]
	if c == nil || c.owner != p.UserID || c.session != p.SessionID {
		return nil
	}
	c.last = time.Now()
	if c.timer != nil {
		c.timer.Reset(min(30*time.Minute, 2*time.Hour-time.Since(c.created)))
	}
	return c
}
func (l *listeningStore) drop(c *room) {
	if c.cancel != nil {
		c.cancel()
	}
	if c.timer != nil {
		c.timer.Stop()
	}
	delete(l.rooms, c.ID)
}

func (l *listeningStore) purge(p account.Principal, all bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, c := range l.rooms {
		if c.owner == p.UserID && (all || c.session == p.SessionID) {
			l.drop(c)
		}
	}
}

// purgeOthers removes live rooms from the owner's other sessions. Signing out
// other devices must not discard an unsaved conversation in this tab.
func (l *listeningStore) purgeOthers(p account.Principal) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, c := range l.rooms {
		if c.owner == p.UserID && c.session != p.SessionID {
			l.drop(c)
		}
	}
}

// purgeSession removes the rooms bound to one exact session.
func (l *listeningStore) purgeSession(owner account.UserID, session account.SessionID) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, c := range l.rooms {
		if c.owner == owner && c.session == session {
			l.drop(c)
		}
	}
}
func roomCopy(c *room) room {
	r := *c
	r.Turns = append([]account.ConversationTurn{}, c.Turns...)
	return r
}

// roomLastActivity is the ordering key for the Home “Lanjutkan” card: the
// newest turn wins, and an empty room sorts as oldest.
func roomLastActivity(c room) time.Time {
	if len(c.Turns) == 0 {
		return time.Time{}
	}
	return c.Turns[len(c.Turns)-1].CreatedAt
}
func (s *Server) persistRoom(ctx context.Context, c *room) error {
	if !c.Save {
		return nil
	}
	title := ""
	for _, t := range c.Turns {
		if t.Role == "user" {
			title = t.Text
			break
		}
	}
	runes := []rune(title)
	if len(runes) > 80 {
		title = string(runes[:80]) + "…"
	}
	in := account.RecordInput{Paused: c.Paused, Kind: "conversation", Title: title, Turns: c.Turns, StorageGranted: true, ExpectedVersion: c.recordVersion}
	var record account.PersonalRecord
	var err error
	if c.RecordID == "" {
		record, err = s.opt.Accounts.CreateRecord(ctx, c.owner, in)
	} else {
		record, err = s.opt.Accounts.UpdateRecord(ctx, c.owner, c.RecordID, in)
	}
	if err == nil {
		c.RecordID = record.ID
		c.recordVersion = record.Version
	}
	return err
}
func (s *Server) v3CreateRoom(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Save     *bool  `json:"save"`
		RecordID string `json:"recordId"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Save == nil {
		writeError(w, r, 400, "invalid_input", "save")
		return
	}
	l := s.v3.listening
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep()
	if len(l.rooms) >= 500 {
		writeError(w, r, 503, "capacity", "")
		return
	}
	count := 0
	for _, c := range l.rooms {
		if c.session == p.SessionID {
			count++
		}
	}
	if count >= 5 {
		writeError(w, r, 429, "rate_limited", "")
		return
	}
	var id [16]byte
	_, _ = rand.Read(id[:])
	now := time.Now()
	c := &room{ID: hex.EncodeToString(id[:]), Save: *in.Save, Version: 1, Turns: []account.ConversationTurn{}, AIState: "not_requested", owner: p.UserID, session: p.SessionID, created: now, last: now}
	if in.RecordID != "" {
		rec, err := s.opt.Accounts.Record(r.Context(), p.UserID, in.RecordID)
		if err != nil {
			s.v3Error(w, r, err)
			return
		}
		if rec.Kind != "conversation" || !*in.Save {
			writeError(w, r, 400, "invalid_input", "recordId")
			return
		}
		c.RecordID = rec.ID
		c.recordVersion = rec.Version
		c.Turns = rec.Turns
		c.Paused = rec.Paused
	}
	if err := s.persistRoom(r.Context(), c); err != nil {
		s.v3Error(w, r, err)
		return
	}
	l.rooms[c.ID] = c
	c.timer = time.AfterFunc(30*time.Minute, func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if current := l.rooms[c.ID]; current == c {
			if c.cancel != nil {
				c.cancel()
			}
			delete(l.rooms, c.ID)
		}
	})
	writeData(w, 201, roomCopy(c))
}
func (s *Server) v3Rooms(w http.ResponseWriter, r *http.Request, p account.Principal) {
	l := s.v3.listening
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep()
	out := []room{}
	for _, c := range l.rooms {
		if c.owner == p.UserID && c.session == p.SessionID {
			out = append(out, roomCopy(c))
		}
	}
	// Map iteration is random; Home picks the first room for “Lanjutkan”.
	// Order by the newest turn, then by id, so the choice is stable.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := roomLastActivity(out[i]), roomLastActivity(out[j])
		if !a.Equal(b) {
			return a.After(b)
		}
		return out[i].ID < out[j].ID
	})
	writeData(w, 200, map[string]any{"conversations": out})
}
func (s *Server) v3Room(w http.ResponseWriter, r *http.Request, p account.Principal) {
	l := s.v3.listening
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.owned(r.PathValue("id"), p)
	if c == nil {
		writeError(w, r, 404, "not_found", "")
		return
	}
	writeData(w, 200, roomCopy(c))
}
func (s *Server) v3EndRoom(w http.ResponseWriter, r *http.Request, p account.Principal) {
	l := s.v3.listening
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.owned(r.PathValue("id"), p)
	if c == nil {
		writeError(w, r, 404, "not_found", "")
		return
	}
	if c.cancel != nil {
		c.cancel()
	}
	if c.timer != nil {
		c.timer.Stop()
	}
	delete(l.rooms, c.ID)
	writeData(w, 200, map[string]bool{"ended": true, "saved": c.Save})
}
func (s *Server) v3StopRoom(w http.ResponseWriter, r *http.Request, p account.Principal) {
	l := s.v3.listening
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.owned(r.PathValue("id"), p)
	if c == nil {
		writeError(w, r, 404, "not_found", "")
		return
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.Version++
	c.AIState = "canceled"
	writeData(w, 200, roomCopy(c))
}
func (s *Server) v3ResumeRoom(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion int `json:"expectedVersion"`
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
	if c.Version != in.ExpectedVersion || c.Busy {
		writeError(w, r, 409, "stale_version", "")
		return
	}
	c.Paused = false
	if err := s.persistRoom(r.Context(), c); err != nil {
		c.Paused = true
		s.v3Error(w, r, err)
		return
	}
	c.AIState = "not_requested"
	c.Version++
	writeData(w, 200, roomCopy(c))
}
func (s *Server) v3RoomStorage(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Save            *bool `json:"save"`
		ExpectedVersion int   `json:"expectedVersion"`
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
	if in.Save == nil {
		writeError(w, r, 400, "invalid_input", "save")
		return
	}
	if c.Version != in.ExpectedVersion || c.Busy {
		writeError(w, r, 409, "stale_version", "")
		return
	}
	if !*in.Save && c.RecordID != "" {
		if err := s.opt.Accounts.DeleteRecord(r.Context(), p.UserID, c.RecordID, c.recordVersion); err != nil {
			s.v3Error(w, r, err)
			return
		}
		c.RecordID = ""
		c.recordVersion = 0
	}
	before := c.Save
	c.Save = *in.Save
	if err := s.persistRoom(r.Context(), c); err != nil {
		c.Save = before
		s.v3Error(w, r, err)
		return
	}
	c.Version++
	writeData(w, 200, roomCopy(c))
}
func (s *Server) v3SendTurn(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Text            string `json:"text"`
		ExpectedVersion int    `json:"expectedVersion"`
		Intent          string `json:"intent"`
	}
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Text) == "" || utf8.RuneCountInString(in.Text) > 3000 {
		writeError(w, r, 400, "invalid_input", "text")
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
	if c.Busy || c.Version != in.ExpectedVersion {
		l.mu.Unlock()
		writeError(w, r, 409, "stale_version", "")
		return
	}
	if len(c.Turns) >= 62 {
		l.mu.Unlock()
		writeError(w, r, 409, "limit_reached", "")
		return
	}
	c.Turns = append(c.Turns, account.ConversationTurn{Role: "user", ReviewStatus: "confirmed", Text: in.Text, Provenance: "user_original", CreatedAt: time.Now()})
	c.Version++
	if safety.Assess(in.Text).Level == safety.LevelCrisis {
		c.Paused = true
	}
	if err := s.persistRoom(r.Context(), c); err != nil {
		c.Turns = c.Turns[:len(c.Turns)-1]
		c.Version--
		l.mu.Unlock()
		s.v3Error(w, r, err)
		return
	}
	if c.Paused || s.opt.AIRouter == nil {
		c.AIState = "unavailable"
		if c.Paused {
			c.AIState = "paused"
		}
		out := roomCopy(c)
		l.mu.Unlock()
		writeData(w, 200, out)
		return
	}
	granted := false
	cs, consentErr := s.opt.Accounts.Consents(r.Context(), p.UserID)
	if consentErr != nil {
		l.mu.Unlock()
		s.v3Error(w, r, consentErr)
		return
	}
	for _, consent := range cs {
		if consent.Purpose == "ai_processing" && consent.Granted && !consent.Outdated {
			granted = true
		}
	}
	if !granted {
		c.AIState = "not_consented"
		out := roomCopy(c)
		l.mu.Unlock()
		writeData(w, 200, out)
		return
	}
	u := l.usage[p.UserID]
	if time.Since(u.at) > 10*time.Minute {
		u = roomUsage{at: time.Now()}
	}
	if u.n >= 8 {
		c.AIState = "rate_limited"
		out := roomCopy(c)
		l.mu.Unlock()
		writeData(w, 200, out)
		return
	}
	var reservation budget.Reservation
	if s.opt.Budget != nil && s.opt.Budget.Enabled() {
		var admErr error
		reservation, admErr = s.opt.Budget.Admit(r.Context(), "listening_reply", s.v3.cfg.Synthetic())
		if admErr != nil {
			c.AIState = "unavailable"
			if errors.Is(admErr, budget.ErrExceeded) {
				c.AIState = "budget_limited"
			}
			out := roomCopy(c)
			l.mu.Unlock()
			writeData(w, 200, out)
			return
		}
	}
	u.n++
	l.usage[p.UserID] = u
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	c.cancel = cancel
	c.Busy = true
	c.AIState = "running"
	version := c.Version
	turns := append([]account.ConversationTurn{}, c.Turns...)
	l.mu.Unlock()
	defer cancel()
	profile, err := s.opt.Accounts.Profile(ctx, p.UserID)
	var output ai.Output
	if err == nil {
		spans := []ai.Span{}
		start := max(0, len(turns)-12)
		for i, t := range turns[start:] {
			role := ai.RoleUser
			if t.Role == "assistant" {
				role = ai.RoleEarlierReply
			}
			if t.Role == "user" && safety.Assess(t.Text).Level == safety.LevelCrisis {
				continue
			}
			spans = append(spans, ai.Span{Key: strconv.Itoa(i), Role: role, Text: t.Text})
		}
		output, err = s.opt.AIRouter.Run(ctx, ai.TaskListeningReply, ai.Input{Subject: ai.Subject(p.UserID.String()), Language: profile.Locale, Spans: spans, Intent: in.Intent, Commit: func(ctx context.Context) error {
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
		}})
	}
	if err != nil {
		// The router's error text is a content-free outcome/reason code.
		s.log.Warn("ai_run_failed", "task", "listening_reply", "error", err.Error())
	}
	if reservation.Admitted() {
		settleCtx, settleCancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
		settlement := budget.Settlement{ActualUSD: reservation.ReservedUSD, Outcome: "failed"}
		if err == nil {
			settlement = budget.Settlement{
				ActualUSD:    output.Provenance.EstimatedCostUSD,
				InputTokens:  output.Provenance.InputTokens,
				OutputTokens: output.Provenance.OutputTokens,
				Outcome:      string(output.Provenance.Outcome),
			}
			if output.Provenance.Generator != nil {
				settlement.Provider = string(output.Provenance.Generator.Provider)
			}
		}
		if settleErr := s.opt.Budget.Settle(settleCtx, reservation, settlement); settleErr != nil {
			s.log.Warn("ai_budget_settle_failed", "error", settleErr)
		}
		settleCancel()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	live := l.owned(c.ID, p)
	if live == nil {
		writeError(w, r, 404, "not_found", "")
		return
	}
	live.Busy = false
	live.cancel = nil
	if err != nil || live.Version != version || len(output.Items) == 0 {
		live.AIState = "unavailable"
		if live.Version != version {
			live.AIState = "canceled"
		}
		writeData(w, 200, roomCopy(live))
		return
	}
	model := ""
	if output.Provenance.Generator != nil {
		model = output.Provenance.Generator.ModelID
	}
	provenanceJSON, _ := json.Marshal(output.Provenance)
	for _, item := range output.Items {
		live.Turns = append(live.Turns, account.ConversationTurn{Role: "assistant", AIProvenance: provenanceJSON, ReviewStatus: "unreviewed", Text: item.Text, Provenance: output.Provenance.DraftProvenance, Model: model, CreatedAt: time.Now()})
	}
	live.Version++
	live.AIState = "ready"
	if err := s.persistRoom(r.Context(), live); err != nil {
		live.Turns = live.Turns[:len(live.Turns)-len(output.Items)]
		live.Version--
		live.AIState = "storage_unavailable"
	}
	writeData(w, 200, roomCopy(live))
}
