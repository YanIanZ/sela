package journey_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/fixtures"
	"github.com/YanIanZ/sela/internal/journey"
	"github.com/YanIanZ/sela/internal/ledger"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/store"
)

// --- test doubles (TEST ONLY: never Meta inference) ---

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) Add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type fakeProvider struct {
	mu      sync.Mutex
	calls   int
	delay   time.Duration
	started chan struct{}
	respond func(req llm.Request) (string, error)
	// mode and model simulate another provider mode (default test double).
	mode  llm.Mode
	model string
}

func (f *fakeProvider) Info() llm.Info {
	if f.mode != "" {
		return llm.Info{Mode: f.mode, ServedModel: f.model}
	}
	return llm.Info{Mode: llm.ModeTestDouble, ServedModel: "test-double-not-meta"}
}

func (f *fakeProvider) Health(context.Context) llm.Health { return llm.Health{State: "ok"} }

func (f *fakeProvider) Calls() int { f.mu.Lock(); defer f.mu.Unlock(); return f.calls }

func (f *fakeProvider) Complete(ctx context.Context, req llm.Request, onDelta func(string) error) (llm.Result, error) {
	f.mu.Lock()
	f.calls++
	started := f.started
	f.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return llm.Result{}, &llm.Error{Kind: llm.KindCanceled, Err: ctx.Err()}
		}
	}
	text, err := f.respond(req)
	if err != nil {
		return llm.Result{}, err
	}
	if onDelta != nil {
		for i := 0; i < len(text); i += 7 {
			end := i + 7
			if end > len(text) {
				end = len(text)
			}
			if err := onDelta(text[i:end]); err != nil {
				return llm.Result{}, err
			}
		}
	}
	if f.model != "" {
		return llm.Result{Text: text, Model: f.model}, nil
	}
	return llm.Result{Text: text, Model: "test-double-not-meta"}, nil
}

// inputOf extracts the JSON payload between the prompt markers.
func inputOf(t testing.TB, req llm.Request) map[string]any {
	msg := req.Messages[len(req.Messages)-1].Content
	i, j := strings.Index(msg, journey.InputOpen), strings.Index(msg, journey.InputClose)
	if i < 0 || j < 0 {
		t.Fatalf("prompt markers missing")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(msg[i+len(journey.InputOpen):j]), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// quotingDraft returns a draft that quotes every segment verbatim.
func quotingDraft(t testing.TB) func(llm.Request) (string, error) {
	return func(req llm.Request) (string, error) {
		in := inputOf(t, req)
		if in["task"] == "opening_draft" {
			items := in["items"].([]any)
			first := items[0].(map[string]any)["text"].(string)
			return fmt.Sprintf(`{"lines":[%q]}`, first), nil
		}
		var claims []map[string]any
		for _, raw := range in["segments"].([]any) {
			seg := raw.(map[string]any)
			claims = append(claims, map[string]any{"segment_id": seg["id"], "quote": seg["text"], "kind": "experience", "text": seg["text"],
				"subject": "self", "time": "unspecified", "certainty": "stated", "negated": false})
		}
		raw, _ := json.Marshal(map[string]any{"claims": claims, "unknowns": []string{}, "clarification": map[string]any{"field": "none"}})
		return string(raw), nil
	}
}

type harness struct {
	t     *testing.T
	svc   *journey.Service
	clock *clock
	llm   *fakeProvider
	store journey.Store
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	cat, err := fixtures.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)}
	st := store.NewMemory(store.Options{MaxSessions: 100, Now: c.Now})
	p := &fakeProvider{}
	p.respond = quotingDraft(t)
	svc := journey.New(journey.Config{Now: c.Now, OpTimeout: 5 * time.Second}, st, cat, p, llm.NewAdmission(2))
	return &harness{t: t, svc: svc, clock: c, llm: p, store: st}
}

func (h *harness) session(lang string) (string, *journey.SessionView) {
	h.t.Helper()
	id, v, err := h.svc.CreateSession(journey.CreateInput{Language: lang, AdultAttested: true, PrivacyPolicy: h.svc.Policy("privacy")})
	if err != nil {
		h.t.Fatal(err)
	}
	return id, v
}

func (h *harness) state(id string) *journey.SessionView {
	h.t.Helper()
	v, err := h.svc.State(id)
	if err != nil {
		h.t.Fatal(err)
	}
	return v
}

func (h *harness) consentAI(id string, granted bool) *journey.SessionView {
	h.t.Helper()
	v, err := h.svc.SetConsent(id, journey.ConsentAI, journey.ConsentInput{ExpectedVersion: h.state(id).Version, Granted: granted, PolicyVersion: h.svc.Policy("ai_processing")})
	if err != nil {
		h.t.Fatal(err)
	}
	return v
}

func (h *harness) story(id, scenario string) *journey.StoryResult {
	h.t.Helper()
	res, err := h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Mode: journey.StoryScenario, ScenarioID: scenario})
	if err != nil {
		h.t.Fatal(err)
	}
	return res
}

// manualApprove adds every segment as a manual item and approves.
func (h *harness) manualApprove(id string) *journey.SessionView {
	h.t.Helper()
	v := h.state(id)
	for _, seg := range v.Story.Segments {
		var err error
		v, err = h.svc.AddManualItem(id, journey.AddItemInput{ExpectedVersion: v.Version, SegmentID: seg.ID, Kind: ledger.KindExperience})
		if err != nil {
			h.t.Fatal(err)
		}
	}
	v, err := h.svc.Approve(id, journey.ApproveInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version})
	if err != nil {
		h.t.Fatal(err)
	}
	return v
}

func (h *harness) prefs(id string) *journey.SessionView {
	h.t.Helper()
	v, err := h.svc.SetPreferences(id, journey.PreferencesInput{ExpectedVersion: h.state(id).Version, Language: "id", Format: "online", Budget: "low_or_free", Availability: "weekday_evening", Barrier: "cost"})
	if err != nil {
		h.t.Fatal(err)
	}
	return v
}

func (h *harness) share(id, key string) *journey.HandoffCreated {
	h.t.Helper()
	v := h.state(id)
	out, err := h.svc.CreateHandoff(id, key, journey.HandoffInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version, ProfessionalID: "pro-rani", SlotID: "slot-rani-1",
		Fields: []journey.ShareField{journey.FieldItems, journey.FieldDisplayName}, DurationHours: 24, ShareConsent: true, SharingPolicy: h.svc.Policy("sharing")})
	if err != nil {
		h.t.Fatal(err)
	}
	return out
}

func code(err error) string {
	var e *journey.Error
	if errors.As(err, &e) {
		return e.Code
	}
	if err == nil {
		return ""
	}
	return "other:" + err.Error()
}

func wantCode(t *testing.T, err error, want string) {
	t.Helper()
	if got := code(err); got != want {
		t.Fatalf("error code = %q, want %q (err=%v)", got, want, err)
	}
}

func waitOp(t *testing.T, svc *journey.Service, sessionID, opID string) []journey.OpEvent {
	t.Helper()
	sub, err := svc.Subscribe(sessionID, opID)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		events, done, wake := sub.Events(0)
		if done {
			return events
		}
		select {
		case <-wake:
		case <-deadline:
			t.Fatal("operation did not finish")
		}
	}
}

// --- tests ---

func TestCreateSessionValidation(t *testing.T) {
	h := newHarness(t)
	_, _, err := h.svc.CreateSession(journey.CreateInput{Language: "fr", AdultAttested: true, PrivacyPolicy: h.svc.Policy("privacy")})
	wantCode(t, err, journey.CodeInvalid)
	_, _, err = h.svc.CreateSession(journey.CreateInput{Language: "id", AdultAttested: false, PrivacyPolicy: h.svc.Policy("privacy")})
	wantCode(t, err, journey.CodeAdultRequired)
	_, _, err = h.svc.CreateSession(journey.CreateInput{Language: "id", AdultAttested: true, PrivacyPolicy: "privacy-old"})
	wantCode(t, err, journey.CodePolicyOutdated)
	_, _, err = h.svc.CreateSession(journey.CreateInput{Language: "id", AdultAttested: true, PrivacyPolicy: h.svc.Policy("privacy"), Identity: journey.Identity{Mode: journey.IdentityPseudonym, Pseudonym: "a@b.c"}})
	wantCode(t, err, journey.CodeInvalid)
	_, v, err := h.svc.CreateSession(journey.CreateInput{Language: "en", AdultAttested: true, PrivacyPolicy: h.svc.Policy("privacy"), Identity: journey.Identity{Mode: journey.IdentityPseudonym, Pseudonym: "  Sky   Walker "}})
	if err != nil || v.Identity.Pseudonym != "Sky Walker" || !v.Synthetic {
		t.Fatalf("pseudonym session: %v %+v", err, v)
	}
}

func TestConsentsAreIndependentAndRetentionWorks(t *testing.T) {
	h := newHarness(t)
	id, v := h.session("id")
	if v.Retention != journey.RetentionSession || !v.ExpiresAt.Equal(h.clock.Now().Add(journey.SessionMaxLife)) {
		t.Fatalf("default retention: %+v", v)
	}
	v = h.consentAI(id, true)
	if v.Consents[journey.ConsentStorage].Granted || v.Retention != journey.RetentionSession {
		t.Fatal("AI consent must not imply storage")
	}
	v, err := h.svc.SetConsent(id, journey.ConsentStorage, journey.ConsentInput{ExpectedVersion: v.Version, Granted: true, PolicyVersion: h.svc.Policy("storage")})
	if err != nil || v.Retention != journey.RetentionDay || !v.ExpiresAt.Equal(h.clock.Now().Add(journey.DayMaxLife)) {
		t.Fatalf("storage consent: %v %+v", err, v)
	}
	// Idle 3 hours: alive only because of storage consent.
	h.clock.Add(3 * time.Hour)
	v, err = h.svc.SetConsent(id, journey.ConsentStorage, journey.ConsentInput{ExpectedVersion: h.state(id).Version, Granted: false})
	if err != nil || v.Retention != journey.RetentionSession {
		t.Fatalf("withdraw storage: %v", err)
	}
	h.clock.Add(journey.SessionIdle + time.Second)
	_, err = h.svc.State(id)
	wantCode(t, err, journey.CodeSessionMissing)
}

func TestSessionIdleExpiryPurges(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.clock.Add(journey.SessionIdle + time.Second)
	_, err := h.svc.State(id)
	wantCode(t, err, journey.CodeSessionMissing)
}

func TestStaleVersionRejectedWithoutSideEffects(t *testing.T) {
	h := newHarness(t)
	id, v := h.session("id")
	_, err := h.svc.SetConsent(id, journey.ConsentAI, journey.ConsentInput{ExpectedVersion: v.Version - 1, Granted: true, PolicyVersion: h.svc.Policy("ai_processing")})
	wantCode(t, err, journey.CodeStale)
	if h.state(id).Consents[journey.ConsentAI].Granted {
		t.Fatal("stale write had a side effect")
	}
}

func TestCrisisStoryInterruptsBeforeAnyModelCallOrHandoff(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.consentAI(id, true)
	res := h.story(id, "id-crisis-interrupt")
	if res.Safety != "crisis" || !res.View.Safety.Interrupted || res.View.Story != nil || res.View.Stage != "crisis" {
		t.Fatalf("crisis route: %+v", res)
	}
	_, err := h.svc.StartLedgerDraft(id, h.state(id).Version)
	wantCode(t, err, journey.CodeCrisis)
	_, err = h.svc.SetPreferences(id, journey.PreferencesInput{ExpectedVersion: h.state(id).Version, Language: "id", Format: "online", Budget: "any", Availability: "any", Barrier: "none"})
	wantCode(t, err, journey.CodeCrisis)
	if h.llm.Calls() != 0 {
		t.Fatalf("model called %d times on crisis path", h.llm.Calls())
	}
	// A later ordinary story cannot reopen the journey in this session.
	_, err = h.svc.StartLedgerDraft(id, h.state(id).Version)
	wantCode(t, err, journey.CodeCrisis)
}

func TestCrisisAfterHandoffBlocksRecipientTransitions(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.story(id, "id-nadia-overwhelm")
	h.manualApprove(id)
	created := h.share(id, "key-crisis-0000001")
	rid, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: created.Token, Code: created.Code})
	if err != nil {
		t.Fatal(err)
	}
	// The user later writes crisis text in an opening-practice attempt.
	_, err = h.svc.Practice(id, journey.PracticeInput{Attempt: "Aku ingin mengakhiri hidupku malam ini"})
	if err != nil {
		t.Fatal(err)
	}
	if !h.state(id).Safety.Interrupted {
		t.Fatal("practice text did not pass the safety gate")
	}
	rh, _ := h.svc.RecipientGet(rid)
	_, err = h.svc.RecipientAct(rid, journey.RecipientActionInput{ExpectedVersion: rh.Version, Action: "acknowledge"})
	wantCode(t, err, journey.CodeInvalidTransition)
}

func TestNoAIConsentManualPathReachesFollowUp(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.story(id, "id-nadia-overwhelm")
	_, err := h.svc.StartLedgerDraft(id, h.state(id).Version)
	wantCode(t, err, journey.CodeConsentRequired)
	v := h.manualApprove(id)
	if v.Ledger.Provenance != journey.ProvenanceManual || v.Ledger.Approved == nil {
		t.Fatalf("manual approval: %+v", v.Ledger)
	}
	h.prefs(id)
	ns, err := h.svc.ComputeNextSteps(id)
	if err != nil || len(ns.Exact) == 0 {
		t.Fatalf("next steps: %v %+v", err, ns)
	}
	created := h.share(id, "key-manual-000000001")
	if created.Handoff.Status != journey.HandoffRequested || created.Handoff.CareReceived {
		t.Fatalf("receipt: %+v", created.Handoff)
	}
	rid, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: created.Token, Code: strings.ToLower(created.Code)})
	if err != nil {
		t.Fatalf("recipient auth: %v", err)
	}
	rh, err := h.svc.RecipientGet(rid)
	if err != nil || !rh.Synthetic || rh.Projection.DisplayName != "anonymous" || len(rh.Projection.Items) == 0 {
		t.Fatalf("recipient view: %v %+v", err, rh)
	}
	if rh.Projection.Preferences != nil || rh.Projection.Opening != nil || rh.Projection.Items[0].Quote != "" {
		t.Fatal("projection leaked unselected fields")
	}
	_, err = h.svc.RecipientAct(rid, journey.RecipientActionInput{ExpectedVersion: rh.Version, Action: "complete", PlanID: "plan-psychologist-followup"})
	wantCode(t, err, journey.CodeInvalidTransition) // must acknowledge first
	rh, err = h.svc.RecipientAct(rid, journey.RecipientActionInput{ExpectedVersion: rh.Version, Action: "acknowledge"})
	if err != nil || rh.Status != journey.HandoffAcknowledged {
		t.Fatal(err)
	}
	_, err = h.svc.RecipientAct(rid, journey.RecipientActionInput{ExpectedVersion: rh.Version, Action: "complete", PlanID: "plan-psychiatrist-followup"})
	wantCode(t, err, journey.CodeInvalid) // plan not owned by this professional
	rh, err = h.svc.RecipientAct(rid, journey.RecipientActionInput{ExpectedVersion: rh.Version, Action: "complete", PlanID: "plan-psychologist-followup"})
	if err != nil || rh.Status != journey.HandoffCompleted {
		t.Fatal(err)
	}
	v, err = h.svc.CheckIn(id, journey.CheckInInput{ExpectedVersion: h.state(id).Version, HandoffID: created.Handoff.ID, OptionID: "checkin-scheduled"})
	if err != nil || len(v.CheckIns) != 1 || v.Handoffs[0].CareReceived {
		t.Fatalf("check-in: %v", err)
	}
	_, err = h.svc.CheckIn(id, journey.CheckInInput{ExpectedVersion: v.Version, HandoffID: created.Handoff.ID, OptionID: "checkin-scheduled"})
	wantCode(t, err, journey.CodeInvalidTransition)
	if h.llm.Calls() != 0 {
		t.Fatal("manual path called the model")
	}
}

func TestAIDraftNeedsReviewEditInvalidatesApproval(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("en")
	h.consentAI(id, true)
	h.story(id, "en-sam-shift-work")
	op, err := h.svc.StartLedgerDraft(id, h.state(id).Version)
	if err != nil {
		t.Fatal(err)
	}
	events := waitOp(t, h.svc, id, op.ID)
	last := events[len(events)-1]
	if last.Type != "done" {
		t.Fatalf("op ended with %s %+v", last.Type, last.Data)
	}
	v := h.state(id)
	if len(v.Ledger.Items) == 0 || v.Ledger.Provenance != journey.ProvenanceTestDouble {
		t.Fatalf("ledger after draft: %+v", v.Ledger)
	}
	_, err = h.svc.Approve(id, journey.ApproveInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version})
	wantCode(t, err, journey.CodeReviewIncomplete)
	for _, it := range v.Ledger.Items {
		v, err = h.svc.ReviewItem(id, it.ID, journey.ItemInput{ExpectedVersion: v.Version, Action: journey.ActionConfirm})
		if err != nil {
			t.Fatal(err)
		}
	}
	v, err = h.svc.Approve(id, journey.ApproveInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version})
	if err != nil || v.Ledger.Approved == nil {
		t.Fatal(err)
	}
	approvedVersion := v.Ledger.Version
	v, err = h.svc.ReviewItem(id, v.Ledger.Items[0].ID, journey.ItemInput{ExpectedVersion: v.Version, Action: journey.ActionEdit, Wording: "I feel tense most days."})
	if err != nil || v.Ledger.Approved != nil || v.Ledger.Version == approvedVersion {
		t.Fatalf("edit must invalidate approval: %v %+v", err, v.Ledger.Approved)
	}
	_, err = h.svc.CreateHandoff(id, "key-stale-0000000001", journey.HandoffInput{ExpectedVersion: v.Version, LedgerVersion: approvedVersion, ProfessionalID: "pro-maya", SlotID: "slot-maya-1",
		Fields: []journey.ShareField{journey.FieldItems}, DurationHours: 24, ShareConsent: true, SharingPolicy: h.svc.Policy("sharing")})
	wantCode(t, err, journey.CodeApprovalRequired)
}

func TestShareRequiresExplicitConsentAndCurrentPolicy(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.story(id, "id-nadia-overwhelm")
	v := h.manualApprove(id)
	base := journey.HandoffInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version, ProfessionalID: "pro-rani", SlotID: "slot-rani-1", Fields: []journey.ShareField{journey.FieldItems}, DurationHours: 24, SharingPolicy: h.svc.Policy("sharing")}
	_, err := h.svc.CreateHandoff(id, "key-consent-00000001", base)
	wantCode(t, err, journey.CodeConsentRequired)
	base.ShareConsent, base.SharingPolicy = true, "sharing-old"
	_, err = h.svc.CreateHandoff(id, "key-consent-00000002", base)
	wantCode(t, err, journey.CodePolicyOutdated)
	base.SharingPolicy, base.Fields = h.svc.Policy("sharing"), []journey.ShareField{journey.FieldDisplayName}
	_, err = h.svc.CreateHandoff(id, "key-consent-00000003", base)
	wantCode(t, err, journey.CodeInvalid)
	base.Fields, base.SlotID = []journey.ShareField{journey.FieldItems}, "slot-maya-1"
	_, err = h.svc.CreateHandoff(id, "key-consent-00000004", base)
	wantCode(t, err, journey.CodeInvalid) // slot of another professional
	_, err = h.svc.CreateHandoff(id, "short", base)
	wantCode(t, err, journey.CodeIdempotencyKey)
	base.ProfessionalID, base.SlotID = "pro-community", "slot-community-1"
	_, err = h.svc.CreateHandoff(id, "key-consent-00000005", base)
	wantCode(t, err, journey.CodeSlotUnavailable)
}

func TestIdempotentHandoffUnderConcurrency(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.story(id, "id-nadia-overwhelm")
	v := h.manualApprove(id)
	in := journey.HandoffInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version, ProfessionalID: "pro-rani", SlotID: "slot-rani-1", Fields: []journey.ShareField{journey.FieldItems}, DurationHours: 24, ShareConsent: true, SharingPolicy: h.svc.Policy("sharing")}
	var wg sync.WaitGroup
	results := make([]*journey.HandoffCreated, 8)
	errs := make([]error, 8)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = h.svc.CreateHandoff(id, "key-dup-000000000001", in)
		}(i)
	}
	wg.Wait()
	tokens := map[string]bool{}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		tokens[results[i].Token] = true
	}
	if len(tokens) != 1 || len(h.state(id).Handoffs) != 1 {
		t.Fatalf("expected one receipt, got %d tokens and %d handoffs", len(tokens), len(h.state(id).Handoffs))
	}
	in.DurationHours = 72
	_, err := h.svc.CreateHandoff(id, "key-dup-000000000001", in)
	wantCode(t, err, journey.CodeIdempotency)
	// Another key for the same active recipient is a conflict, not a duplicate.
	in.DurationHours, in.ExpectedVersion = 24, h.state(id).Version
	_, err = h.svc.CreateHandoff(id, "key-dup-000000000002", in)
	wantCode(t, err, journey.CodeHandoffExists)
}

func TestTokenAloneIsNotEnoughAndLockout(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.story(id, "id-nadia-overwhelm")
	h.manualApprove(id)
	c := h.share(id, "key-lock-00000000001")
	_, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c.Token, Code: ""})
	wantCode(t, err, journey.CodeAccessEnded)
	_, err = h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c.Token + "x", Code: c.Code})
	wantCode(t, err, journey.CodeAccessEnded)
	for i := 0; i < 4; i++ {
		_, err = h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c.Token, Code: "AAAA-AAAA"})
		wantCode(t, err, journey.CodeAccessEnded)
	}
	// 5 wrong attempts in total: locked even for the right code.
	_, err = h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c.Token, Code: "AAAA-AAAA"})
	wantCode(t, err, journey.CodeAccessEnded)
	_, err = h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c.Token, Code: c.Code})
	wantCode(t, err, journey.CodeAccessEnded)
	if got := h.state(id).Handoffs[0].Share; got != journey.ShareLocked {
		t.Fatalf("share state = %s, want locked", got)
	}
}

func TestRevokeExpiryAndDeleteEndRecipientAccess(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	// Keep the owner session alive across the simulated hour (storage consent).
	if _, err := h.svc.SetConsent(id, journey.ConsentStorage, journey.ConsentInput{ExpectedVersion: h.state(id).Version, Granted: true, PolicyVersion: h.svc.Policy("storage")}); err != nil {
		t.Fatal(err)
	}
	h.story(id, "id-nadia-overwhelm")
	h.manualApprove(id)
	c := h.share(id, "key-revoke-000000001")
	rid, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c.Token, Code: c.Code})
	if err != nil {
		t.Fatal(err)
	}
	v, err := h.svc.RevokeShare(id, c.Handoff.ID, journey.RevokeInput{ExpectedVersion: h.state(id).Version})
	if err != nil || v.Handoffs[0].Share != journey.ShareRevoked {
		t.Fatal(err)
	}
	if _, err := h.svc.RevokeShare(id, c.Handoff.ID, journey.RevokeInput{}); err != nil {
		t.Fatalf("repeat revoke must be safe: %v", err)
	}
	_, err = h.svc.RecipientGet(rid)
	wantCode(t, err, journey.CodeAccessEnded)
	_, err = h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c.Token, Code: c.Code})
	wantCode(t, err, journey.CodeAccessEnded)

	// Expiry: a 1-hour share.
	v = h.state(id)
	c2, err := h.svc.CreateHandoff(id, "key-expire-000000001", journey.HandoffInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version, ProfessionalID: "pro-campus", SlotID: "slot-campus-2",
		Fields: []journey.ShareField{journey.FieldItems}, DurationHours: 1, ShareConsent: true, SharingPolicy: h.svc.Policy("sharing")})
	if err != nil {
		t.Fatal(err)
	}
	rid2, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c2.Token, Code: c2.Code})
	if err != nil {
		t.Fatal(err)
	}
	h.clock.Add(61 * time.Minute)
	h.state(id) // keep the owner session alive
	_, err = h.svc.RecipientGet(rid2)
	wantCode(t, err, journey.CodeAccessEnded)

	// Delete: a third share, then delete the whole session.
	v = h.state(id)
	c3, err := h.svc.CreateHandoff(id, "key-delete-000000001", journey.HandoffInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version, ProfessionalID: "pro-maya", SlotID: "slot-maya-2",
		Fields: []journey.ShareField{journey.FieldItems}, DurationHours: 24, ShareConsent: true, SharingPolicy: h.svc.Policy("sharing")})
	if err != nil {
		t.Fatal(err)
	}
	rid3, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c3.Token, Code: c3.Code})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.svc.DeleteSession(id); err != nil {
		t.Fatal(err)
	}
	_, err = h.svc.RecipientGet(rid3)
	wantCode(t, err, journey.CodeAccessEnded)
	_, err = h.svc.State(id)
	wantCode(t, err, journey.CodeSessionMissing)
}

func TestSessionsAreIsolated(t *testing.T) {
	h := newHarness(t)
	a, _ := h.session("id")
	b, _ := h.session("en")
	h.consentAI(a, true)
	h.story(a, "id-nadia-overwhelm")
	op, err := h.svc.StartLedgerDraft(a, h.state(a).Version)
	if err != nil {
		t.Fatal(err)
	}
	waitOp(t, h.svc, a, op.ID)
	_, err = h.svc.Subscribe(b, op.ID)
	wantCode(t, err, journey.CodeNotFound)
	_, err = h.svc.CancelOp(b, op.ID)
	wantCode(t, err, journey.CodeNotFound)
	// Approve and share in A; B sees nothing and cannot revoke it.
	v := h.state(a)
	for _, it := range v.Ledger.Items {
		v, _ = h.svc.ReviewItem(a, it.ID, journey.ItemInput{ExpectedVersion: v.Version, Action: journey.ActionConfirm})
	}
	if _, err := h.svc.Approve(a, journey.ApproveInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version}); err != nil {
		t.Fatal(err)
	}
	c := h.share(a, "key-isolate-00000001")
	_, err = h.svc.RevokeShare(b, c.Handoff.ID, journey.RevokeInput{ExpectedVersion: h.state(b).Version})
	wantCode(t, err, journey.CodeNotFound)
	if vb := h.state(b); vb.Story != nil || len(vb.Handoffs) != 0 {
		t.Fatal("session B sees A's data")
	}
	// Deleting B leaves A's share working.
	if err := h.svc.DeleteSession(b); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c.Token, Code: c.Code}); err != nil {
		t.Fatalf("A's share broken by deleting B: %v", err)
	}
}

func TestStaleAIResultIsDiscarded(t *testing.T) {
	for _, tc := range []struct {
		name   string
		action func(h *harness, id string)
		want   string
	}{
		{"consent withdrawn", func(h *harness, id string) { h.consentAI(id, false) }, "canceled"},
		{"story replaced", func(h *harness, id string) { h.story(id, "en-sam-shift-work") }, "canceled"},
		{"manual edit", func(h *harness, id string) {
			v := h.state(id)
			if _, err := h.svc.AddManualItem(id, journey.AddItemInput{ExpectedVersion: v.Version, SegmentID: v.Story.Segments[0].ID, Kind: ledger.KindFeeling}); err != nil {
				h.t.Fatal(err)
			}
		}, "stale_result"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.llm.delay = 300 * time.Millisecond
			h.llm.started = make(chan struct{}, 1)
			id, _ := h.session("id")
			h.consentAI(id, true)
			h.story(id, "id-nadia-overwhelm")
			op, err := h.svc.StartLedgerDraft(id, h.state(id).Version)
			if err != nil {
				t.Fatal(err)
			}
			<-h.llm.started
			tc.action(h, id)
			events := waitOp(t, h.svc, id, op.ID)
			last := events[len(events)-1]
			data, _ := json.Marshal(last.Data)
			if last.Type != "error" || !strings.Contains(string(data), tc.want) {
				t.Fatalf("last event %s %s, want error %s", last.Type, data, tc.want)
			}
			h.svc.Wait()
			v := h.state(id)
			for _, it := range v.Ledger.Items {
				if it.Provenance == journey.ProvenanceTestDouble {
					t.Fatal("stale model result was committed")
				}
			}
		})
	}
}

func TestCancelOperation(t *testing.T) {
	h := newHarness(t)
	h.llm.delay = 2 * time.Second
	h.llm.started = make(chan struct{}, 1)
	id, _ := h.session("id")
	h.consentAI(id, true)
	h.story(id, "id-nadia-overwhelm")
	op, err := h.svc.StartLedgerDraft(id, h.state(id).Version)
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.svc.StartLedgerDraft(id, h.state(id).Version)
	wantCode(t, err, journey.CodeOpInProgress)
	<-h.llm.started
	start := time.Now()
	if _, err := h.svc.CancelOp(id, op.ID); err != nil {
		t.Fatal(err)
	}
	waitOp(t, h.svc, id, op.ID)
	h.svc.Wait()
	if time.Since(start) > time.Second {
		t.Fatal("cancellation did not stop the model call promptly")
	}
	if st := h.state(id).Ops[0].State; st != journey.OpCanceled {
		t.Fatalf("op state %s", st)
	}
}

func TestMeaningLockRejectsFabricatedAndUnsafeModelOutput(t *testing.T) {
	h := newHarness(t)
	h.llm.respond = func(req llm.Request) (string, error) {
		return `{"claims":[
		 {"segment_id":"s999","quote":"tidak ada","kind":"feeling","text":"x","subject":"self","time":"current","certainty":"stated","negated":false},
		 {"segment_id":"s2","quote":"susah tidur","kind":"experience","text":"Saya susah tidur","subject":"self","time":"current","certainty":"stated","negated":false},
		 {"segment_id":"s1","quote":"aku merasa kewalahan dengan skripsi dan kerja paruh waktu","kind":"feeling","text":"Kamu mengalami depresi berat","subject":"self","time":"current","certainty":"stated","negated":false}
		],"unknowns":[],"clarification":{"field":"none"}}`, nil
	}
	id, _ := h.session("id")
	h.consentAI(id, true)
	h.story(id, "id-nadia-overwhelm")
	op, err := h.svc.StartLedgerDraft(id, h.state(id).Version)
	if err != nil {
		t.Fatal(err)
	}
	events := waitOp(t, h.svc, id, op.ID)
	if events[len(events)-1].Type != "done" {
		t.Fatalf("expected done, got %+v", events[len(events)-1])
	}
	v := h.state(id)
	for _, it := range v.Ledger.Items {
		if strings.Contains(strings.ToLower(it.Wording), "depresi") || it.Quote == "susah tidur" {
			t.Fatalf("unsafe or negation-dropping claim committed: %+v", it)
		}
	}
	if v.Ledger.Rejected["unknown_segment"] != 1 || v.Ledger.Rejected["negation_dropped"] != 1 {
		t.Fatalf("rejections not recorded: %+v", v.Ledger.Rejected)
	}
}

func TestMalformedModelOutputFailsHonestly(t *testing.T) {
	h := newHarness(t)
	h.llm.respond = func(llm.Request) (string, error) { return `{"claims":[{"segment_id":`, nil }
	id, _ := h.session("id")
	h.consentAI(id, true)
	h.story(id, "id-nadia-overwhelm")
	op, _ := h.svc.StartLedgerDraft(id, h.state(id).Version)
	events := waitOp(t, h.svc, id, op.ID)
	data, _ := json.Marshal(events[len(events)-1].Data)
	if !strings.Contains(string(data), "ai_invalid_output") {
		t.Fatalf("want ai_invalid_output, got %s", data)
	}
	if len(h.state(id).Ledger.Items) != 0 {
		t.Fatal("malformed output produced items")
	}
}

func TestProviderUnavailableAndDisabled(t *testing.T) {
	h := newHarness(t)
	h.llm.respond = func(llm.Request) (string, error) { return "", &llm.Error{Kind: llm.KindUnavailable, Status: 503} }
	id, _ := h.session("id")
	h.consentAI(id, true)
	h.story(id, "id-nadia-overwhelm")
	op, _ := h.svc.StartLedgerDraft(id, h.state(id).Version)
	events := waitOp(t, h.svc, id, op.ID)
	data, _ := json.Marshal(events[len(events)-1].Data)
	if !strings.Contains(string(data), journey.CodeAIUnavailable) {
		t.Fatalf("want ai_unavailable, got %s", data)
	}
	cat := fixtures.MustLoad()
	svc := journey.New(journey.Config{Now: h.clock.Now}, store.NewMemory(store.Options{Now: h.clock.Now}), cat, nil, nil)
	sid, _, _ := svc.CreateSession(journey.CreateInput{Language: "id", AdultAttested: true, PrivacyPolicy: svc.Policy("privacy")})
	v, _ := svc.State(sid)
	v, _ = svc.SetConsent(sid, journey.ConsentAI, journey.ConsentInput{ExpectedVersion: v.Version, Granted: true, PolicyVersion: svc.Policy("ai_processing")})
	if _, err := svc.SubmitStory(sid, journey.StoryInput{ExpectedVersion: v.Version, Mode: journey.StoryScenario, ScenarioID: "id-nadia-overwhelm"}); err != nil {
		t.Fatal(err)
	}
	v, _ = svc.State(sid)
	_, err := svc.StartLedgerDraft(sid, v.Version)
	wantCode(t, err, journey.CodeAIDisabled)
}

// TestModalColdStartRetries covers Modal's immediate 503 while no GPU
// container is ready: the operation reports "warming", retries within the
// cold-start budget and then succeeds; a test double's 503 is never retried;
// and a cold start that outlasts the budget fails honestly.
func TestModalColdStartRetries(t *testing.T) {
	cat := fixtures.MustLoad()
	run := func(t *testing.T, mode llm.Mode, failures int, estimate time.Duration) (*fakeProvider, []journey.OpEvent) {
		c := &clock{t: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)}
		p := &fakeProvider{mode: mode, model: "served-under-test"}
		draft := quotingDraft(t)
		p.respond = func(req llm.Request) (string, error) {
			if p.Calls() <= failures {
				return "", &llm.Error{Kind: llm.KindUnavailable, Status: 503}
			}
			return draft(req)
		}
		svc := journey.New(journey.Config{Now: c.Now, OpTimeout: 5 * time.Second, ColdStartEstimate: estimate, WarmPollInterval: 10 * time.Millisecond},
			store.NewMemory(store.Options{Now: c.Now}), cat, p, llm.NewAdmission(1))
		sid, v, _ := svc.CreateSession(journey.CreateInput{Language: "id", AdultAttested: true, PrivacyPolicy: svc.Policy("privacy")})
		v, _ = svc.SetConsent(sid, journey.ConsentAI, journey.ConsentInput{ExpectedVersion: v.Version, Granted: true, PolicyVersion: svc.Policy("ai_processing")})
		if _, err := svc.SubmitStory(sid, journey.StoryInput{ExpectedVersion: v.Version, Mode: journey.StoryScenario, ScenarioID: "id-nadia-overwhelm"}); err != nil {
			t.Fatal(err)
		}
		v, _ = svc.State(sid)
		op, err := svc.StartLedgerDraft(sid, v.Version)
		if err != nil {
			t.Fatal(err)
		}
		return p, waitOp(t, svc, sid, op.ID)
	}
	t.Run("modal warms then succeeds", func(t *testing.T) {
		p, events := run(t, llm.ModeModal, 2, time.Second)
		if p.Calls() != 3 {
			t.Fatalf("calls = %d, want 3", p.Calls())
		}
		if last := events[len(events)-1]; last.Type != "done" {
			t.Fatalf("last event %s %v", last.Type, last.Data)
		}
		retries := 0
		for _, e := range events {
			if e.Type == "status" {
				if m, ok := e.Data.(map[string]any); ok && m["state"] == "warming" && m["attempt"] != nil {
					retries++
				}
			}
		}
		if retries != 2 {
			t.Fatalf("warming retry events = %d, want 2", retries)
		}
	})
	t.Run("test double 503 is not retried", func(t *testing.T) {
		p, events := run(t, llm.ModeTestDouble, 5, time.Second)
		if p.Calls() != 1 || events[len(events)-1].Type != "error" {
			t.Fatalf("calls = %d, last = %s", p.Calls(), events[len(events)-1].Type)
		}
	})
	t.Run("cold start beyond budget fails honestly", func(t *testing.T) {
		p, events := run(t, llm.ModeModal, 1000, 25*time.Millisecond)
		data, _ := json.Marshal(events[len(events)-1].Data)
		if p.Calls() < 2 || p.Calls() > 6 || !strings.Contains(string(data), journey.CodeAIUnavailable) {
			t.Fatalf("calls = %d, last = %s", p.Calls(), data)
		}
	})
}

func TestPerSessionAIRateLimit(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.consentAI(id, true)
	h.story(id, "id-nadia-overwhelm")
	for i := 0; i < h.svc.MaxOpsPerWindow(); i++ {
		op, err := h.svc.StartLedgerDraft(id, h.state(id).Version)
		if err != nil {
			t.Fatalf("op %d: %v", i, err)
		}
		waitOp(t, h.svc, id, op.ID)
	}
	_, err := h.svc.StartLedgerDraft(id, h.state(id).Version)
	wantCode(t, err, journey.CodeRateLimited)
}

func TestPracticeTextBoundsAndContactDetails(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	_, err := h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Mode: journey.StoryPractice, Text: "pendek"})
	wantCode(t, err, journey.CodeInvalid)
	_, err = h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Mode: journey.StoryPractice, Text: "Aku capek sekali minggu ini. Hubungi aku di 0812-3456-7890 ya."})
	wantCode(t, err, journey.CodeContactDetails)
	_, err = h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Mode: journey.StoryPractice, Text: "Email aku: tokoh.fiktif@example.com, aku lelah."})
	wantCode(t, err, journey.CodeContactDetails)
	res, err := h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Mode: journey.StoryPractice, Text: "Tokoh fiktifku merasa lelah sejak 2026-10-01. Dia ingin bicara dengan konselor."})
	if err != nil || res.View.Story == nil || res.View.Story.Mode != journey.StoryPractice {
		t.Fatalf("practice story: %v", err)
	}
	_, err = h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Mode: journey.StoryScenario, ScenarioID: "../etc/passwd"})
	wantCode(t, err, journey.CodeInvalid)
}

func TestClarificationAnswerAndSkip(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.story(id, "id-nadia-overwhelm")
	v := h.state(id)
	if v.Ledger.Clarification == "" {
		t.Fatal("expected a clarification suggestion for an empty ledger")
	}
	v, err := h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: v.Version, Field: ledger.UnknownDuration, Answer: "Sejak awal semester ini."})
	if err != nil || len(v.Ledger.Items) != 1 || v.Ledger.Items[0].Kind != ledger.KindDuration || v.Story.Segments[len(v.Story.Segments)-1].Origin != "clarification" {
		t.Fatalf("clarify answer: %v %+v", err, v.Ledger)
	}
	v, err = h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: v.Version, Field: ledger.UnknownGoal, Skip: true})
	if err != nil || v.Ledger.Clarification == ledger.UnknownGoal {
		t.Fatalf("skip: %v", err)
	}
	_, err = h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: v.Version, Field: ledger.UnknownImpact, Answer: "Aku mau bunuh diri"})
	if err != nil || !h.state(id).Safety.Interrupted {
		t.Fatalf("crisis clarification must interrupt: %v", err)
	}
}

func TestOpeningPracticeTemplateAndCoverage(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.story(id, "id-nadia-overwhelm")
	_, err := h.svc.TemplateOpening(id, journey.OpeningInput{ExpectedVersion: h.state(id).Version, Recipient: "professional"})
	wantCode(t, err, journey.CodeApprovalRequired)
	h.manualApprove(id)
	v, err := h.svc.TemplateOpening(id, journey.OpeningInput{ExpectedVersion: h.state(id).Version, Recipient: "professional"})
	if err != nil || v.Opening == nil || len(v.Opening.Lines) == 0 || v.Opening.Provenance != journey.ProvenanceManual {
		t.Fatalf("template: %v %+v", err, v.Opening)
	}
	res, err := h.svc.Practice(id, journey.PracticeInput{Attempt: "Saya merasa kewalahan dengan skripsi."})
	if err != nil || len(res.Coverage.Mentioned) == 0 {
		t.Fatalf("practice coverage: %v %+v", err, res)
	}
}

func TestNextStepsAreDeterministicAndHonestWhenEmpty(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.story(id, "id-nadia-overwhelm")
	_, err := h.svc.SetPreferences(id, journey.PreferencesInput{ExpectedVersion: h.state(id).Version, Language: "en", Format: "in_person", Budget: "free_only", Availability: "weekend", Barrier: "not_ready"})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := h.svc.ComputeNextSteps(id)
	b, _ := h.svc.ComputeNextSteps(id)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("next steps not deterministic")
	}
	if len(a.Exact) != 0 || a.HandoffFirst || len(a.Steps) == 0 {
		t.Fatalf("expected honest empty state with non-handoff steps first: %s", ja)
	}
	for _, o := range a.Near {
		misses := 0
		for _, r := range o.Reasons {
			if !r.Match {
				misses++
			}
		}
		if misses != 1 {
			t.Fatalf("near option with %d misses", misses)
		}
	}
}

func TestClarificationRequestRequiresNewShare(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.story(id, "id-nadia-overwhelm")
	h.manualApprove(id)
	c := h.share(id, "key-clarify-00000001")
	rid, _ := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c.Token, Code: c.Code})
	rh, _ := h.svc.RecipientGet(rid)
	if _, err := h.svc.RecipientAct(rid, journey.RecipientActionInput{ExpectedVersion: rh.Version, Action: "clarify", ReasonID: "clarify-schedule"}); err != nil {
		t.Fatal(err)
	}
	// The user answers by editing the brief: new version, new approval, new consent.
	v := h.state(id)
	v, err := h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: v.Version, Field: ledger.UnknownPreference, Answer: "Malam hari kerja setelah jam tujuh."})
	if err != nil {
		t.Fatal(err)
	}
	v, err = h.svc.Approve(id, journey.ApproveInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version})
	if err != nil {
		t.Fatal(err)
	}
	c2, err := h.svc.CreateHandoff(id, "key-clarify-00000002", journey.HandoffInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version, ProfessionalID: "pro-rani", SlotID: "slot-rani-2",
		Fields: []journey.ShareField{journey.FieldItems}, DurationHours: 24, ShareConsent: true, SharingPolicy: h.svc.Policy("sharing")})
	if err != nil {
		t.Fatal(err)
	}
	if c2.Handoff.ReplacesID != c.Handoff.ID {
		t.Fatal("replacement not linked")
	}
	if _, err := h.svc.RecipientGet(rid); code(err) != journey.CodeAccessEnded {
		t.Fatal("superseded share still readable")
	}
}

func TestStoreCapacityNeverEvictsActiveSessions(t *testing.T) {
	cat := fixtures.MustLoad()
	c := &clock{t: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)}
	st := store.NewMemory(store.Options{MaxSessions: 2, Now: c.Now})
	svc := journey.New(journey.Config{Now: c.Now}, st, cat, nil, nil)
	mk := func() error {
		_, _, err := svc.CreateSession(journey.CreateInput{Language: "id", AdultAttested: true, PrivacyPolicy: svc.Policy("privacy")})
		return err
	}
	if mk() != nil || mk() != nil {
		t.Fatal("first two sessions must fit")
	}
	wantCode(t, mk(), journey.CodeCapacity)
	c.Add(11 * time.Minute) // both idle > 10 min: one may be evicted
	if err := mk(); err != nil {
		t.Fatalf("idle eviction failed: %v", err)
	}
}
