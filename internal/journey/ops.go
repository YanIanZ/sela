package journey

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/YanIanZ/sela/internal/ledger"
	"github.com/YanIanZ/sela/internal/llm"
)

// OpEvent is one server-sent event of an AI operation. Data never contains
// unvalidated model text: "item" events carry claims that already passed
// Meaning Lock, and "done" carries counts.
type OpEvent struct {
	Seq  int    `json:"seq"`
	Type string `json:"type"` // status | item | progress | done | error
	Data any    `json:"data"`
}

// opRuntime is the in-process side of an operation: cancellation and an
// append-only, bounded event log with broadcast.
type opRuntime struct {
	id        string
	sessionID string
	kind      OpKind
	cancel    context.CancelCauseFunc

	mu     sync.Mutex
	events []OpEvent
	done   bool
	wake   chan struct{}
	ended  time.Time
}

const maxOpEvents = 256

var (
	errOpCanceledByUser  = errors.New("canceled_by_user")
	errOpConsentWithdraw = errors.New("consent_withdrawn")
)

func (r *opRuntime) emit(typ string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done || len(r.events) >= maxOpEvents && typ != "done" && typ != "error" {
		return
	}
	r.events = append(r.events, OpEvent{Seq: len(r.events) + 1, Type: typ, Data: data})
	if typ == "done" || typ == "error" {
		r.done = true
	}
	close(r.wake)
	r.wake = make(chan struct{})
}

// Events returns events after seq, whether the operation finished, and a
// channel closed on the next event.
func (r *opRuntime) Events(after int) ([]OpEvent, bool, <-chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if after < 0 {
		after = 0
	}
	var out []OpEvent
	if after < len(r.events) {
		out = append(out, r.events[after:]...)
	}
	return out, r.done, r.wake
}

// Subscription streams an operation's events for one session.
type Subscription interface {
	Events(after int) ([]OpEvent, bool, <-chan struct{})
}

// Subscribe returns the operation's event stream if it belongs to the session.
func (s *Service) Subscribe(sessionID, opID string) (Subscription, error) {
	if err := s.view(sessionID, func(sess *Session) error {
		if sess.op(opID) == nil {
			return errf(CodeNotFound, 404, "opId")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	s.opsMu.Lock()
	rt := s.ops[opID]
	s.opsMu.Unlock()
	if rt == nil || rt.sessionID != sessionID {
		return nil, errf(CodeNotFound, 404, "opId")
	}
	return rt, nil
}

// CancelOp cancels a running operation owned by the session. Idempotent.
func (s *Service) CancelOp(sessionID, opID string) (*SessionView, error) {
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		rec := sess.op(opID)
		if rec == nil {
			return errf(CodeNotFound, 404, "opId")
		}
		s.cancelRecord(sess, rec, errOpCanceledByUser)
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// cancelRecord marks a non-final record canceled and cancels its runtime.
// Caller holds the session.
func (s *Service) cancelRecord(sess *Session, rec *OpRecord, cause error) {
	if rec.State != OpQueued && rec.State != OpRunning {
		return
	}
	rec.State = OpCanceled
	rec.ErrorCode = cause.Error()
	rec.EndedAt = s.now()
	s.opsMu.Lock()
	rt := s.ops[rec.ID]
	s.opsMu.Unlock()
	if rt != nil {
		rt.cancel(cause)
		rt.emit("error", map[string]any{"code": "canceled", "reason": cause.Error()})
	}
}

func (s *Service) cancelSessionOps(sess *Session, reason string) {
	for _, rec := range sess.Ops {
		s.cancelRecord(sess, rec, errors.New(reason))
	}
}

func (s *Service) cancelKind(sess *Session, kind OpKind, reason string) {
	for _, rec := range sess.Ops {
		if rec.Kind == kind {
			s.cancelRecord(sess, rec, errors.New(reason))
		}
	}
}

// reapOps forgets finished runtimes after a grace period for late subscribers.
func (s *Service) reapOps(now time.Time) {
	s.opsMu.Lock()
	defer s.opsMu.Unlock()
	for id, rt := range s.ops {
		rt.mu.Lock()
		old := rt.done && !rt.ended.IsZero() && now.Sub(rt.ended) > 10*time.Minute
		rt.mu.Unlock()
		if old {
			delete(s.ops, id)
		}
	}
}

// AIStatus is the public provider status used by the UI.
type AIStatus struct {
	Info        llm.Info `json:"provider"`
	Available   bool     `json:"available"`
	LikelyWarm  bool     `json:"likelyWarm"`
	ColdStartMS int64    `json:"coldStartEstimateMs"`
	InUse       int      `json:"inUse"`
}

// AIStatus reports the provider mode without contacting it (a health call
// could start a paid GPU container).
func (s *Service) AIStatus() AIStatus {
	info := s.provider.Info()
	s.warmMu.Lock()
	warm := !s.lastLiveOK.IsZero() && s.now().Sub(s.lastLiveOK) < s.cfg.WarmWindow
	s.warmMu.Unlock()
	return AIStatus{Info: info, Available: info.Mode != llm.ModeDisabled, LikelyWarm: warm, ColdStartMS: s.cfg.ColdStartEstimate.Milliseconds(), InUse: s.admission.InUse()}
}

// startOp validates preconditions and registers an operation. Caller holds
// the session; run is started after the session lock is released.
func (s *Service) startOp(sess *Session, kind OpKind) (*OpRecord, error) {
	if sess.Safety.Interrupted {
		return nil, errCrisis
	}
	if !sess.consent(ConsentAI).Granted {
		return nil, errf(CodeConsentRequired, 403, "ai_processing")
	}
	if s.provider.Info().Mode == llm.ModeDisabled {
		return nil, errf(CodeAIDisabled, 503, "")
	}
	recent := 0
	for _, o := range sess.Ops {
		if o.Kind == kind && (o.State == OpQueued || o.State == OpRunning) {
			return nil, errf(CodeOpInProgress, 409, o.ID)
		}
		if s.now().Sub(o.StartedAt) < s.cfg.OpWindow {
			recent++
		}
	}
	if recent >= s.cfg.MaxOpsPerWindow {
		return nil, errf(CodeRateLimited, 429, "")
	}
	rec := &OpRecord{ID: "op_" + randomToken(12), Kind: kind, State: OpQueued, StartedAt: s.now(), ConsentRevision: sess.consent(ConsentAI).Revision}
	if sess.Story != nil {
		rec.StoryVersion = sess.Story.Version
	}
	if sess.Ledger != nil {
		rec.BriefVersion = sess.Ledger.Version
	}
	sess.Ops = append(sess.Ops, rec)
	if len(sess.Ops) > 32 {
		sess.Ops = sess.Ops[len(sess.Ops)-32:]
	}
	return rec, nil
}

func (s *Service) launch(sessionID string, rec *OpRecord, job func(ctx context.Context, rt *opRuntime) error) {
	ctx, cancel := context.WithCancelCause(context.Background())
	ctx, stop := context.WithTimeout(ctx, s.cfg.OpTimeout)
	rt := &opRuntime{id: rec.ID, sessionID: sessionID, kind: rec.Kind, cancel: cancel, wake: make(chan struct{})}
	s.opsMu.Lock()
	s.ops[rec.ID] = rt
	s.opsMu.Unlock()
	s.opsInFlight.Add(1)
	go func() {
		defer s.opsInFlight.Done()
		defer stop()
		defer cancel(nil)
		rt.emit("status", map[string]any{"state": "queued"})
		err := job(ctx, rt)
		s.finishOp(sessionID, rec.ID, rt, ctx, err)
	}()
}

// finishOp records the terminal state. The success path committed already.
func (s *Service) finishOp(sessionID, opID string, rt *opRuntime, ctx context.Context, err error) {
	code := ""
	if err != nil {
		code = opErrorCode(ctx, err)
	}
	_ = s.store.Update(sessionID, func(sess *Session) error {
		rec := sess.op(opID)
		if rec == nil || rec.State == OpCanceled || rec.State == OpSucceeded {
			return nil
		}
		rec.EndedAt = s.now()
		if err != nil {
			rec.State, rec.ErrorCode = OpFailed, code
			if code == "canceled" {
				rec.State = OpCanceled
			}
		}
		return nil
	})
	if err != nil {
		s.metrics.add("ai_op_" + code)
		rt.emit("error", map[string]any{"code": code})
	}
	rt.mu.Lock()
	rt.ended = s.now()
	rt.mu.Unlock()
}

// errStaleResult means the session changed while the model ran.
var errStaleResult = errors.New("stale_result")

func opErrorCode(ctx context.Context, err error) string {
	var appErr *Error
	switch {
	case errors.Is(err, errStaleResult):
		return "stale_result"
	case errors.As(err, &appErr):
		return appErr.Code
	case context.Cause(ctx) != nil && !errors.Is(context.Cause(ctx), context.DeadlineExceeded) && !errors.Is(context.Cause(ctx), context.Canceled):
		return "canceled"
	}
	switch llm.KindOf(err) {
	case llm.KindTimeout:
		return "ai_timeout"
	case llm.KindCanceled:
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "ai_timeout"
		}
		return "canceled"
	case llm.KindDisabled:
		return CodeAIDisabled
	case llm.KindAuth:
		return "ai_auth"
	case llm.KindRateLimited:
		return "ai_busy"
	case llm.KindModelMismatch:
		return "ai_model_mismatch"
	case llm.KindOutputTooLong, llm.KindProtocol, llm.KindRejected:
		return "ai_invalid_output"
	default:
		return CodeAIUnavailable
	}
}

func (s *Service) provenance(res llm.Result) (Provenance, string) {
	info := s.provider.Info()
	switch info.Mode {
	case llm.ModeModal:
		if res.Model == info.ServedModel {
			return ProvenanceLiveMeta, info.ModelID + "@" + info.Revision
		}
	case llm.ModeTestDouble:
		return ProvenanceTestDouble, res.Model
	}
	return "", ""
}

func (s *Service) markLive() {
	if s.provider.Info().Mode != llm.ModeModal {
		return
	}
	s.warmMu.Lock()
	s.lastLiveOK = s.now()
	s.warmMu.Unlock()
}

// generate runs admission, the warm/cold status event and the model call.
func (s *Service) generate(ctx context.Context, rt *opRuntime, req llm.Request, onDelta func(string) error) (llm.Result, error) {
	release, err := s.admission.Acquire(ctx)
	if err != nil {
		return llm.Result{}, err
	}
	defer release()
	status := s.AIStatus()
	if status.Info.Mode == llm.ModeModal && !status.LikelyWarm {
		rt.emit("status", map[string]any{"state": "warming", "coldStartEstimateMs": status.ColdStartMS})
	} else {
		rt.emit("status", map[string]any{"state": "running"})
	}
	started := time.Now()
	warmBudget := 2 * s.cfg.ColdStartEstimate
	streamed := false
	track := func(d string) error {
		streamed = true
		if onDelta == nil {
			return nil
		}
		return onDelta(d)
	}
	for attempt := 1; ; attempt++ {
		res, err := s.provider.Complete(ctx, req, track)
		if err == nil {
			s.markLive()
			return res, nil
		}
		if streamed || !coldStartRetryable(status.Info.Mode, err) || time.Since(started)+s.cfg.WarmPollInterval > warmBudget {
			return res, err
		}
		s.metrics.add("ai_cold_start_retry")
		rt.emit("status", map[string]any{"state": "warming", "coldStartEstimateMs": status.ColdStartMS, "elapsedMs": time.Since(started).Milliseconds(), "attempt": attempt})
		t := time.NewTimer(s.cfg.WarmPollInterval)
		select {
		case <-ctx.Done():
			t.Stop()
			return llm.Result{}, ctx.Err()
		case <-t.C:
		}
	}
}

// coldStartRetryable reports whether err is Modal's "no container ready yet"
// answer (HTTP 503 before any output), which is retried while the GPU
// container boots. Every other failure is returned to the user at once.
func coldStartRetryable(mode llm.Mode, err error) bool {
	if mode != llm.ModeModal {
		return false
	}
	var e *llm.Error
	return errors.As(err, &e) && e.Kind == llm.KindUnavailable && e.Status == http.StatusServiceUnavailable
}

// StartLedgerDraft starts an AI draft of the Story Ledger. The deterministic
// pipeline (SPEC V3, AI policy): session/version → crisis gate (already
// applied when the story was submitted) → explicit AI consent → per-session
// rate limit → global admission → bounded model call → Meaning Lock →
// recheck story/ledger/consent versions → commit with provenance.
func (s *Service) StartLedgerDraft(sessionID string, expectedVersion int) (*OpRecord, error) {
	var rec *OpRecord
	var segments []ledger.Segment
	var lang string
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, expectedVersion); err != nil {
			return err
		}
		if err := s.requireLedger(sess); err != nil {
			return err
		}
		r, err := s.startOp(sess, OpLedgerDraft)
		if err != nil {
			return err
		}
		rec = r
		segments = append([]ledger.Segment{}, sess.Story.Segments...)
		lang = sess.Language
		bump(sess)
		return nil
	})
	if err != nil {
		return nil, err
	}
	snapshot := *rec
	s.metrics.add("ai_op_started")
	s.launch(sessionID, rec, func(ctx context.Context, rt *opRuntime) error {
		return s.runLedgerDraft(ctx, rt, sessionID, snapshot, segments, lang)
	})
	return &snapshot, nil
}

func (s *Service) runLedgerDraft(ctx context.Context, rt *opRuntime, sessionID string, rec OpRecord, segments []ledger.Segment, lang string) error {
	req := ledgerRequest(segments, lang, s.cfg.MaxOutputTokens)
	streamer := newArrayStreamer("claims")
	previewed := 0
	deltas := 0
	res, err := s.generate(ctx, rt, req, func(delta string) error {
		deltas++
		for _, raw := range streamer.Feed(delta) {
			var c ledger.Candidate
			if json.Unmarshal(raw, &c) != nil {
				continue
			}
			r := ledger.ValidateDraft(ledger.Draft{Claims: []ledger.Candidate{c}}, segments)
			for _, claim := range r.Accepted {
				previewed++
				rt.emit("item", map[string]any{"preview": true, "index": previewed, "kind": claim.Kind, "quote": claim.Quote, "wording": claim.Wording, "fromQuote": claim.FromQuote, "segmentId": claim.SegmentID})
			}
		}
		if deltas%16 == 0 {
			rt.emit("progress", map[string]any{"chunks": deltas, "previewed": previewed})
		}
		return nil
	})
	if err != nil {
		return err
	}
	rt.emit("status", map[string]any{"state": "validating"})
	draft, perr := ledger.ParseDraft([]byte(res.Text))
	if perr != nil {
		return errf("ai_invalid_output", 502, "")
	}
	result := ledger.ValidateDraft(draft, segments)
	prov, model := s.provenance(res)
	if prov == "" {
		return errf("ai_model_mismatch", 502, "")
	}
	rejected := map[string]int{}
	for _, r := range result.Rejected {
		rejected[string(r.Reason)]++
	}
	var summary map[string]any
	err = s.store.Update(sessionID, func(sess *Session) error {
		cur := sess.op(rec.ID)
		if cur == nil || cur.State == OpCanceled {
			return errStaleResult
		}
		c := sess.consent(ConsentAI)
		if sess.Safety.Interrupted || !c.Granted || c.Revision != rec.ConsentRevision ||
			sess.Story == nil || sess.Story.Version != rec.StoryVersion || sess.Ledger == nil || sess.Ledger.Version != rec.BriefVersion {
			return errStaleResult
		}
		l := sess.Ledger
		items := make([]Item, 0, len(result.Accepted))
		for _, c := range result.Accepted {
			origin := WordingModel
			if c.FromQuote {
				origin = WordingQuote
			}
			items = append(items, Item{
				ID: l.newItemID(), Kind: c.Kind, SegmentID: c.SegmentID, Quote: c.Quote, Start: c.Start, End: c.End,
				Wording: c.Wording, WordingOrigin: origin, Qualifiers: c.Qualifiers, Status: ItemUnreviewed, Provenance: prov, Notes: c.Notes,
			})
		}
		l.Items = items
		l.Provenance, l.Model, l.Rejected = prov, model, rejected
		l.Skipped = nil
		l.Clarification = ""
		if result.Clarification != "" {
			l.Clarification = result.Clarification
		}
		changeLedger(l)
		recompute(l)
		if result.Clarification != "" && containsField(l.Unknowns, result.Clarification) {
			l.Clarification = result.Clarification
		}
		cur.State, cur.Provenance, cur.EndedAt = OpSucceeded, prov, s.now()
		bump(sess)
		summary = map[string]any{"accepted": len(items), "rejected": rejected, "provenance": prov, "model": model,
			"latencyMs": res.Latency.Milliseconds(), "firstTokenMs": res.FirstToken.Milliseconds(), "ledgerVersion": l.Version, "sessionVersion": sess.Version}
		return nil
	})
	if errors.Is(err, ErrNotFound) {
		return errStaleResult
	}
	if err != nil {
		return err
	}
	s.metrics.add("ai_op_succeeded")
	rt.emit("done", summary)
	return nil
}
