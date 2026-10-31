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
	ID          string                     `json:"id"`
	Save        bool                       `json:"save"`
	Version     int                        `json:"version"`
	Turns       []account.ConversationTurn `json:"turns"`
	Reflections []account.Reflection       `json:"reflections,omitempty"`
	Paused      bool                       `json:"paused"`
	Busy        bool                       `json:"busy"`
	AIState     string                     `json:"aiState"`
	RecordID    string                     `json:"recordId,omitempty"`
	// HoldExpiresIn is the number of seconds before the server discards a
	// held room: 30 idle minutes, capped at 2 hours from creation. It is
	// relative to this response so a wrong client clock cannot hide expiry.
	HoldExpiresIn int `json:"holdExpiresIn,omitempty"`
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

// holdExpiresIn is the server's hold deadline as a relative duration. The
// room is discarded at the earlier of 30 minutes without a request and 2
// hours from creation, matching sweep and the idle timer.
func holdExpiresIn(c *room, now time.Time) int {
	if c.created.IsZero() || c.last.IsZero() {
		return 0
	}
	expires := c.last.Add(30 * time.Minute)
	if absolute := c.created.Add(2 * time.Hour); absolute.Before(expires) {
		expires = absolute
	}
	left := expires.Sub(now)
	if left <= 0 {
		return 0
	}
	return int((left + time.Second - 1) / time.Second)
}

func roomCopy(c *room) room {
	r := *c
	r.Turns = append([]account.ConversationTurn{}, c.Turns...)
	r.Reflections = append([]account.Reflection(nil), c.Reflections...)
	r.HoldExpiresIn = holdExpiresIn(c, time.Now())
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
	in := account.RecordInput{Paused: c.Paused, Kind: "conversation", Title: title, Turns: c.Turns,
		Reflections: c.Reflections, StorageGranted: true, ExpectedVersion: c.recordVersion}
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
		c.Reflections = rec.Reflections
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

// memoryContextSpans selects confirmed/corrected memory items for AT-1.
// The store returns items newest first. At most five items and 600 runes
// are sent, and an item is included whole only when it fits the remaining
// allowance, so memory text is never cut mid-sentence. Items that would
// exceed the allowance are skipped deterministically; if none fit, no
// memory is sent.
func memoryContextSpans(items []account.PersonalRecord) []ai.Span {
	const maxItems, maxRunes = 5, 600
	out := []ai.Span{}
	used := 0
	for _, r := range items {
		if len(out) >= maxItems {
			break
		}
		if r.ReviewStatus != "confirmed" && r.ReviewStatus != "corrected" {
			continue
		}
		text := strings.TrimSpace(r.Text)
		if title := strings.TrimSpace(r.Title); title != "" {
			text = title + "\n" + text
		}
		n := utf8.RuneCountInString(text)
		if n == 0 || used+n > maxRunes {
			continue
		}
		out = append(out, ai.Span{Key: "memory-" + strconv.Itoa(len(out)), Role: ai.RoleMemory, Text: text})
		used += n
	}
	return out
}

// listeningSpans builds the AT-1 payload: memory first, then the newest
// conversation turns. When memory is present the turn count and total
// runes are capped so the router's DropOldest cannot discard the memory
// context first; memory spans are never dropped here.
func listeningSpans(turns []account.ConversationTurn, memory []ai.Span) []ai.Span {
	pol, ok := ai.PolicyFor(ai.TaskListeningReply)
	if !ok {
		return append([]ai.Span{}, memory...)
	}
	spans := append([]ai.Span{}, memory...)
	total := 0
	for _, s := range memory {
		total += utf8.RuneCountInString(s.Text)
	}
	maxTurns := pol.MaxSpans - len(memory)
	if maxTurns > 12 {
		maxTurns = 12
	}
	selected := []ai.Span{}
	for i := len(turns) - 1; i >= 0 && len(selected) < maxTurns; i-- {
		t := turns[i]
		if t.Role == "user" && safety.Assess(t.Text).Level == safety.LevelCrisis {
			continue
		}
		role := ai.RoleUser
		if t.Role == "assistant" {
			role = ai.RoleEarlierReply
		}
		n := utf8.RuneCountInString(t.Text)
		if pol.MaxTotalRunes > 0 && total+n > pol.MaxTotalRunes {
			break
		}
		selected = append(selected, ai.Span{Key: "turn-" + strconv.Itoa(i), Role: role, Text: t.Text})
		total += n
	}
	for i := len(selected) - 1; i >= 0; i-- {
		spans = append(spans, selected[i])
	}
	return spans
}

// isRoleLimit reports whether a run failed because the redacted payload
// exceeded a per-role limit (the router makes no provider call in that case).
func isRoleLimit(err error) bool {
	var re *ai.RunError
	if !errors.As(err, &re) {
		return false
	}
	for _, reason := range re.Reasons {
		if reason == ai.ReasonRoleLimit {
			return true
		}
	}
	return false
}

// listeningTurnInput is the body of both turn routes. The streaming route
// accepts exactly the same fields and pipeline as the JSON route.
type listeningTurnInput struct {
	Text            string `json:"text"`
	ExpectedVersion int    `json:"expectedVersion"`
	Intent          string `json:"intent"`
}

// listeningTurn is the result of a completed turn. Released is non-empty
// only when the router released validated text and it was stored in the
// room; it is the only text a streaming route may emit.
type listeningTurn struct {
	Room     room
	Released []string
}

// turnRejection is a pre-stream refusal with a stable API error code.
type turnRejection struct {
	status int
	code   string
	field  string
}

func (e *turnRejection) write(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, e.status, e.code, e.field)
}

// runListeningTurn is the one turn pipeline: validation, the user turn,
// persistence, crisis pause, consent, usage and budget limits, the AI
// router (AT-1 plus prompt/output guards and the verifier), settlement and
// storing the released reply. beforeAI, when set, runs once after every
// pre-AI check passed and immediately before the router call; the SSE
// handler uses it to open the stream and send status without any content.
func (s *Server) runListeningTurn(r *http.Request, p account.Principal, id string, in listeningTurnInput, beforeAI func()) (listeningTurn, *turnRejection, error) {
	var out listeningTurn
	if strings.TrimSpace(in.Text) == "" || utf8.RuneCountInString(in.Text) > 3000 {
		return out, &turnRejection{status: 400, code: "invalid_input", field: "text"}, nil
	}
	l := s.v3.listening
	l.mu.Lock()
	c := l.owned(id, p)
	if c == nil {
		l.mu.Unlock()
		return out, &turnRejection{status: 404, code: "not_found"}, nil
	}
	if c.Busy || c.Version != in.ExpectedVersion {
		l.mu.Unlock()
		return out, &turnRejection{status: 409, code: "stale_version"}, nil
	}
	if len(c.Turns) >= 62 {
		l.mu.Unlock()
		return out, &turnRejection{status: 409, code: "limit_reached"}, nil
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
		return out, nil, err
	}
	if c.Paused || s.opt.AIRouter == nil {
		c.AIState = "unavailable"
		if c.Paused {
			c.AIState = "paused"
		}
		out.Room = roomCopy(c)
		l.mu.Unlock()
		return out, nil, nil
	}
	granted := false
	memoryGranted := false
	cs, consentErr := s.opt.Accounts.Consents(r.Context(), p.UserID)
	if consentErr != nil {
		l.mu.Unlock()
		return out, nil, consentErr
	}
	for _, consent := range cs {
		if consent.Purpose == "ai_processing" && consent.Granted && !consent.Outdated {
			granted = true
		}
		if consent.Purpose == "personal_memory" && consent.Granted && !consent.Outdated {
			memoryGranted = true
		}
	}
	if !granted {
		c.AIState = "not_consented"
		out.Room = roomCopy(c)
		l.mu.Unlock()
		return out, nil, nil
	}
	u := l.usage[p.UserID]
	if time.Since(u.at) > 10*time.Minute {
		u = roomUsage{at: time.Now()}
	}
	if u.n >= 8 {
		c.AIState = "rate_limited"
		out.Room = roomCopy(c)
		l.mu.Unlock()
		return out, nil, nil
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
			out.Room = roomCopy(c)
			l.mu.Unlock()
			return out, nil, nil
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
	// Memory is context only: a failed read never blocks the reply, and
	// nothing is loaded unless the current personal_memory consent is on.
	var memories []account.PersonalRecord
	if memoryGranted {
		if view, memErr := s.opt.Accounts.Memory(ctx, p.UserID); memErr != nil {
			s.log.Warn("ai_memory_context_failed", "task", "listening_reply", "error", memErr.Error())
		} else if view.Enabled {
			memories = view.Items
		}
	}
	if beforeAI != nil {
		beforeAI()
	}
	profile, err := s.opt.Accounts.Profile(ctx, p.UserID)
	var output ai.Output
	if err == nil {
		memory := memoryContextSpans(memories)
		spans := listeningSpans(turns, memory)
		input := ai.Input{Subject: ai.Subject(p.UserID.String()), Language: profile.Locale, Spans: spans, Intent: in.Intent, PrivateNames: s.privateNamesFor(ctx, p.UserID, "listening_reply"), Commit: func(ctx context.Context) error {
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
		}}
		output, err = s.opt.AIRouter.Run(ctx, ai.TaskListeningReply, input)
		if err != nil && len(memory) > 0 && isRoleLimit(err) {
			// Per-role limits apply after redaction, which can lengthen
			// text. Retry once without memory instead of losing the reply;
			// the role-limit failure made no provider call.
			input.Spans = input.Spans[len(memory):]
			output, err = s.opt.AIRouter.Run(ctx, ai.TaskListeningReply, input)
		}
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
		return out, &turnRejection{status: 404, code: "not_found"}, nil
	}
	live.Busy = false
	live.cancel = nil
	if err != nil || live.Version != version || len(output.Items) == 0 {
		live.AIState = "unavailable"
		if live.Version != version {
			live.AIState = "canceled"
		}
		out.Room = roomCopy(live)
		return out, nil, nil
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
	stored := true
	if err := s.persistRoom(r.Context(), live); err != nil {
		live.Turns = live.Turns[:len(live.Turns)-len(output.Items)]
		live.Version--
		live.AIState = "storage_unavailable"
		stored = false
	}
	if stored {
		out.Released = make([]string, 0, len(output.Items))
		for _, item := range output.Items {
			out.Released = append(out.Released, item.Text)
		}
	}
	out.Room = roomCopy(live)
	return out, nil, nil
}

// v3SendTurn is the JSON route. It keeps the existing response shape while
// sharing the pipeline with the streaming route.
func (s *Server) v3SendTurn(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in listeningTurnInput
	if !decode(w, r, &in) {
		return
	}
	out, reject, err := s.runListeningTurn(r, p, r.PathValue("id"), in, nil)
	if reject != nil {
		reject.write(w, r)
		return
	}
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, 200, out.Room)
}
