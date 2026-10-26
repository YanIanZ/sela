package journey_test

// Regression tests for the core review of the journey service: crisis gate
// order, view isolation, stale AI results, operation purging, per-session
// and per-share bounds, revoke, retention clock edges and share expiry.

import (
	"context"
	"encoding/json"
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

// newHarnessWith builds a harness around a caller-provided store.
func newHarnessWith(t *testing.T, mk func(now func() time.Time) journey.Store) *harness {
	t.Helper()
	c := &clock{t: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)}
	st := mk(c.Now)
	p := &fakeProvider{}
	p.respond = quotingDraft(t)
	svc := journey.New(journey.Config{Now: c.Now, OpTimeout: 10 * time.Second}, st, fixtures.MustLoad(), p, llm.NewAdmission(2))
	return &harness{t: t, svc: svc, clock: c, llm: p, store: st}
}

// practiceStory submits practice text and fails the test on error.
func (h *harness) practiceStory(id, text string) *journey.SessionView {
	h.t.Helper()
	res, err := h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Mode: journey.StoryPractice, Text: text})
	if err != nil {
		h.t.Fatal(err)
	}
	return res.View
}

func eventsJSON(t *testing.T, events []journey.OpEvent) string {
	t.Helper()
	raw, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

const crisisEN = "I want to kill myself tonight"

// [1] Crisis language is routed before any other validation of the request,
// so contact details, length, line count, a missing approval or the story
// mode never turn it away as an ordinary input error, and story text sent
// beside a scenario is never ignored.
func TestCrisisGateRunsBeforeOtherValidation(t *testing.T) {
	cases := []struct {
		name string
		// approved: start from an approved manual ledger.
		approved bool
		act      func(h *harness, id string) error
	}{
		{"wording edit with a phone number", true, func(h *harness, id string) error {
			v := h.state(id)
			_, err := h.svc.ReviewItem(id, v.Ledger.Items[0].ID, journey.ItemInput{ExpectedVersion: v.Version, Action: journey.ActionEdit, Wording: crisisEN + ", call 0812 3456 7890"})
			return err
		}},
		{"opening lines with an email", true, func(h *harness, id string) error {
			_, err := h.svc.EditOpening(id, journey.EditOpeningInput{ExpectedVersion: h.state(id).Version, Lines: []string{crisisEN + ", email me at a@b.co"}})
			return err
		}},
		{"too many opening lines", true, func(h *harness, id string) error {
			_, err := h.svc.EditOpening(id, journey.EditOpeningInput{ExpectedVersion: h.state(id).Version, Lines: []string{"one", "two", "three", crisisEN}})
			return err
		}},
		{"opening before approval", false, func(h *harness, id string) error {
			_, err := h.svc.EditOpening(id, journey.EditOpeningInput{ExpectedVersion: h.state(id).Version, Lines: []string{crisisEN}})
			return err
		}},
		{"practice attempt before approval", false, func(h *harness, id string) error {
			_, err := h.svc.Practice(id, journey.PracticeInput{Attempt: crisisEN})
			return err
		}},
		{"over-long clarification answer", false, func(h *harness, id string) error {
			_, err := h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: h.state(id).Version, Field: ledger.UnknownImpact, Answer: crisisEN + strings.Repeat(" really", 80)})
			return err
		}},
		{"short practice story", false, func(h *harness, id string) error {
			_, err := h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Mode: journey.StoryPractice, Text: "Aku mau bunuh diri."})
			return err
		}},
		{"practice story with contact details", false, func(h *harness, id string) error {
			_, err := h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Mode: journey.StoryPractice, Text: "Aku mau bunuh diri malam ini, hubungi 081234567890"})
			return err
		}},
		// The text is screened whatever the mode, before the mode is checked.
		{"story text with a missing mode", false, func(h *harness, id string) error {
			_, err := h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Text: crisisEN})
			return err
		}},
		{"story text with an unknown mode", false, func(h *harness, id string) error {
			_, err := h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Mode: "bogus", Text: crisisEN})
			return err
		}},
		{"story text beside a scenario", false, func(h *harness, id string) error {
			res, err := h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Mode: journey.StoryScenario, ScenarioID: "en-sam-shift-work", Text: crisisEN})
			if err == nil && res.Safety != "crisis" {
				t.Errorf("story result safety = %q, want crisis", res.Safety)
			}
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			id, _ := h.session("en")
			h.story(id, "en-sam-shift-work")
			if tc.approved {
				h.manualApprove(id)
			}
			if err := tc.act(h, id); err != nil {
				t.Fatalf("crisis language turned away as %s", code(err))
			}
			v := h.state(id)
			if !v.Safety.Interrupted || v.Safety.Level != "crisis" || v.Story != nil || v.Ledger != nil {
				t.Fatalf("crisis route not taken: %+v", v.Safety)
			}
			_, err := h.svc.CreateHandoff(id, "key-crisis-order-0001", journey.HandoffInput{ExpectedVersion: v.Version, ProfessionalID: "pro-rani", SlotID: "slot-rani-1",
				Fields: []journey.ShareField{journey.FieldItems}, DurationHours: 24, ShareConsent: true, SharingPolicy: h.svc.Policy("sharing")})
			wantCode(t, err, journey.CodeCrisis)
		})
	}
}

// J2: an interrupted session accepts no new story.
func TestInterruptedSessionRefusesNewStory(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.story(id, "id-crisis-interrupt")
	_, err := h.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: h.state(id).Version, Mode: journey.StoryScenario, ScenarioID: "id-nadia-overwhelm"})
	wantCode(t, err, journey.CodeCrisis)
	if v := h.state(id); v.Story != nil || !v.Safety.Interrupted {
		t.Fatalf("story stored after crisis: %+v", v.Story)
	}
}

// J8 / [10]: a crisis pseudonym takes the crisis route and is not stored,
// even when it would not pass the pseudonym pattern.
func TestCrisisPseudonymInterruptsSession(t *testing.T) {
	h := newHarness(t)
	for _, p := range []string{"Kill Myself", "Ingin Mati", "I want to die!!"} {
		_, v, err := h.svc.CreateSession(journey.CreateInput{Language: "id", AdultAttested: true, PrivacyPolicy: h.svc.Policy("privacy"), Identity: journey.Identity{Mode: journey.IdentityPseudonym, Pseudonym: p}})
		if err != nil {
			t.Fatalf("%q: %v", p, err)
		}
		if !v.Safety.Interrupted || v.Identity.Mode != journey.IdentityAnonymous || v.Identity.Pseudonym != "" {
			t.Fatalf("%q: safety=%+v identity=%+v", p, v.Safety, v.Identity)
		}
	}
	id, _, err := h.svc.CreateSession(journey.CreateInput{Language: "en", AdultAttested: true, PrivacyPolicy: h.svc.Policy("privacy"), Identity: journey.Identity{Mode: journey.IdentityPseudonym, Pseudonym: "Sky Walker"}})
	if err != nil {
		t.Fatal(err)
	}
	lang := "id"
	v, err := h.svc.UpdateSettings(id, journey.SettingsInput{ExpectedVersion: h.state(id).Version, Language: &lang, Identity: &journey.Identity{Mode: journey.IdentityPseudonym, Pseudonym: "Want To Die"}})
	if err != nil {
		t.Fatal(err)
	}
	if !v.Safety.Interrupted || v.Identity.Pseudonym != "Sky Walker" || v.Language != "en" {
		t.Fatalf("settings crisis route: safety=%+v identity=%+v lang=%s", v.Safety, v.Identity, v.Language)
	}
}

// [2] The view is a snapshot: the operation goroutine never writes memory a
// caller encodes after the session lock is released (go test -race).
func TestSessionViewSharesNoMemoryWithOperations(t *testing.T) {
	h := newHarness(t)
	h.llm.delay = 30 * time.Millisecond
	id, _ := h.session("id")
	h.consentAI(id, true)
	h.story(id, "id-nadia-overwhelm")
	op, err := h.svc.StartLedgerDraft(id, h.state(id).Version)
	if err != nil {
		t.Fatal(err)
	}
	v := h.state(id)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_, _ = json.Marshal(v)
			}
		}
	}()
	waitOp(t, h.svc, id, op.ID)
	h.svc.Wait()
	close(stop)
	wg.Wait()
	if v.Ops[0].State == journey.OpSucceeded {
		t.Fatal("an earlier view changed when the operation committed")
	}
}

// [3] An approval or a skip made while the model runs makes the result
// stale; it never wipes them.
func TestLedgerDraftNeverOverwritesUserActions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		act   func(h *harness, id string)
		check func(t *testing.T, v *journey.SessionView)
	}{
		{"approval", func(h *harness, id string) {
			v := h.state(id)
			if _, err := h.svc.Approve(id, journey.ApproveInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version}); err != nil {
				h.t.Fatal(err)
			}
		}, func(t *testing.T, v *journey.SessionView) {
			if v.Ledger.Approved == nil || v.Ledger.Provenance != journey.ProvenanceManual {
				t.Fatalf("approval wiped: approved=%v provenance=%s", v.Ledger.Approved != nil, v.Ledger.Provenance)
			}
		}},
		{"skip", func(h *harness, id string) {
			if _, err := h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: h.state(id).Version, Field: ledger.UnknownGoal, Skip: true}); err != nil {
				h.t.Fatal(err)
			}
		}, func(t *testing.T, v *journey.SessionView) {
			if len(v.Ledger.Skipped) != 1 || v.Ledger.Provenance != journey.ProvenanceManual {
				t.Fatalf("skip lost: skipped=%v provenance=%s", v.Ledger.Skipped, v.Ledger.Provenance)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.llm.delay = 300 * time.Millisecond
			h.llm.started = make(chan struct{}, 1)
			id, _ := h.session("id")
			h.consentAI(id, true)
			h.story(id, "id-nadia-overwhelm")
			v := h.state(id)
			for _, seg := range v.Story.Segments {
				var err error
				if v, err = h.svc.AddManualItem(id, journey.AddItemInput{ExpectedVersion: v.Version, SegmentID: seg.ID, Kind: ledger.KindExperience}); err != nil {
					t.Fatal(err)
				}
			}
			op, err := h.svc.StartLedgerDraft(id, h.state(id).Version)
			if err != nil {
				t.Fatal(err)
			}
			<-h.llm.started
			tc.act(h, id)
			events := waitOp(t, h.svc, id, op.ID)
			h.svc.Wait()
			if last := events[len(events)-1]; last.Type != "error" || !strings.Contains(eventsJSON(t, events[len(events)-1:]), "stale_result") {
				t.Fatalf("want stale_result, got %s", eventsJSON(t, events[len(events)-1:]))
			}
			tc.check(t, h.state(id))
		})
	}
}

// [3] A skipped field stays skipped after an AI draft and is not suggested
// again, even when the model suggests it.
func TestRedraftKeepsSkippedFields(t *testing.T) {
	h := newHarness(t)
	base := h.llm.respond
	h.llm.respond = func(req llm.Request) (string, error) {
		out, err := base(req)
		return strings.Replace(out, `"clarification":{"field":"none"}`, `"clarification":{"field":"goal"}`, 1), err
	}
	id, _ := h.session("en")
	h.consentAI(id, true)
	h.story(id, "en-sam-shift-work")
	if _, err := h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: h.state(id).Version, Field: ledger.UnknownGoal, Skip: true}); err != nil {
		t.Fatal(err)
	}
	op, err := h.svc.StartLedgerDraft(id, h.state(id).Version)
	if err != nil {
		t.Fatal(err)
	}
	if events := waitOp(t, h.svc, id, op.ID); events[len(events)-1].Type != "done" {
		t.Fatalf("draft: %s", eventsJSON(t, events))
	}
	v := h.state(id)
	if len(v.Ledger.Skipped) != 1 || v.Ledger.Skipped[0] != ledger.UnknownGoal || v.Ledger.Clarification == ledger.UnknownGoal {
		t.Fatalf("skip forgotten: skipped=%v clarification=%q", v.Ledger.Skipped, v.Ledger.Clarification)
	}
}

// [4] The user's own opening lines written while an opening draft runs are
// never replaced by the model's lines.
func TestOpeningDraftNeverOverwritesUserLines(t *testing.T) {
	h := newHarness(t)
	h.llm.delay = 300 * time.Millisecond
	h.llm.started = make(chan struct{}, 1)
	id, _ := h.session("id")
	h.consentAI(id, true)
	h.story(id, "id-nadia-overwhelm")
	h.manualApprove(id)
	op, err := h.svc.StartOpeningDraft(id, journey.OpeningInput{ExpectedVersion: h.state(id).Version, Recipient: "professional"})
	if err != nil {
		t.Fatal(err)
	}
	<-h.llm.started
	if _, err := h.svc.EditOpening(id, journey.EditOpeningInput{ExpectedVersion: h.state(id).Version, Lines: []string{"My own first line."}}); err != nil {
		t.Fatal(err)
	}
	waitOp(t, h.svc, id, op.ID)
	h.svc.Wait()
	o := h.state(id).Opening
	if !o.UserEdited || o.Provenance != journey.ProvenanceManual || o.Lines[0].Text != "My own first line." {
		t.Fatalf("user lines overwritten: %+v", o)
	}
}

// J1: the projection says who wrote the shared opening lines.
func TestProjectionLabelsOpeningProvenance(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.consentAI(id, true)
	h.story(id, "id-nadia-overwhelm")
	h.manualApprove(id)
	op, err := h.svc.StartOpeningDraft(id, journey.OpeningInput{ExpectedVersion: h.state(id).Version, Recipient: "professional"})
	if err != nil {
		t.Fatal(err)
	}
	waitOp(t, h.svc, id, op.ID)
	h.svc.Wait()
	shareOpening := func(key string) journey.Projection {
		v := h.state(id)
		out, err := h.svc.CreateHandoff(id, key, journey.HandoffInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version, ProfessionalID: "pro-rani", SlotID: "slot-rani-1",
			Fields: []journey.ShareField{journey.FieldItems, journey.FieldOpening}, DurationHours: 24, ShareConsent: true, SharingPolicy: h.svc.Policy("sharing")})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.svc.RevokeShare(id, out.Handoff.ID, journey.RevokeInput{}); err != nil {
			t.Fatal(err)
		}
		return out.Handoff.Projection
	}
	p := shareOpening("key-opening-prov-0001")
	if len(p.Opening) == 0 || p.OpeningProvenance != journey.ProvenanceTestDouble || p.OpeningUserEdited {
		t.Fatalf("AI opening projection: %+v", p)
	}
	if _, err := h.svc.EditOpening(id, journey.EditOpeningInput{ExpectedVersion: h.state(id).Version, Lines: []string{"My own first line."}}); err != nil {
		t.Fatal(err)
	}
	p = shareOpening("key-opening-prov-0002")
	if p.OpeningProvenance != journey.ProvenanceManual || !p.OpeningUserEdited {
		t.Fatalf("user opening projection: %+v", p)
	}
}

// [5] / J9: deletion, crisis and story replacement drop the story content
// from operation event logs at once.
func TestOperationLogsDropContent(t *testing.T) {
	draft := func(t *testing.T, h *harness, id string) (string, string) {
		t.Helper()
		op, err := h.svc.StartLedgerDraft(id, h.state(id).Version)
		if err != nil {
			t.Fatal(err)
		}
		events := waitOp(t, h.svc, id, op.ID)
		h.svc.Wait()
		seg := h.state(id).Story.Segments[0].Text
		if !strings.Contains(eventsJSON(t, events), seg) {
			t.Fatal("precondition: item previews carry the story text")
		}
		return op.ID, seg
	}
	t.Run("crisis", func(t *testing.T) {
		h := newHarness(t)
		id, _ := h.session("en")
		h.consentAI(id, true)
		h.story(id, "en-sam-shift-work")
		opID, seg := draft(t, h, id)
		v := h.state(id)
		if _, err := h.svc.ReviewItem(id, v.Ledger.Items[0].ID, journey.ItemInput{ExpectedVersion: v.Version, Action: journey.ActionEdit, Wording: crisisEN}); err != nil {
			t.Fatal(err)
		}
		sub, err := h.svc.Subscribe(id, opID)
		if err != nil {
			t.Fatal(err)
		}
		events, done, _ := sub.Events(0)
		if !done || len(events) != 1 || strings.Contains(eventsJSON(t, events), seg) {
			t.Fatalf("discarded draft replayable after crisis: %s", eventsJSON(t, events))
		}
	})
	t.Run("story replaced", func(t *testing.T) {
		h := newHarness(t)
		id, _ := h.session("en")
		h.consentAI(id, true)
		h.story(id, "en-sam-shift-work")
		opID, seg := draft(t, h, id)
		h.story(id, "id-nadia-overwhelm")
		sub, err := h.svc.Subscribe(id, opID)
		if err != nil {
			t.Fatal(err)
		}
		events, _, _ := sub.Events(0)
		if len(events) != 1 || strings.Contains(eventsJSON(t, events), seg) {
			t.Fatalf("replaced story still in the event log: %s", eventsJSON(t, events))
		}
	})
	t.Run("deleted", func(t *testing.T) {
		h := newHarness(t)
		id, _ := h.session("en")
		h.consentAI(id, true)
		h.practiceStory(id, "CANARYSTORY my fictional character feels tired every evening. She wants to talk to someone.")
		opID, _ := draft(t, h, id)
		if err := h.svc.DeleteSession(id); err != nil {
			t.Fatal(err)
		}
		if events, ok := h.svc.RuntimeEvents(opID); ok {
			t.Fatalf("runtime kept after deletion: %s", eventsJSON(t, events))
		}
	})
}

// slowProvider blocks until its context ends (or a long delay passes).
type slowProvider struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
}

func (p *slowProvider) Info() llm.Info {
	return llm.Info{Mode: llm.ModeTestDouble, ServedModel: "test-double-not-meta"}
}
func (p *slowProvider) Health(context.Context) llm.Health { return llm.Health{State: "ok"} }
func (p *slowProvider) Calls() int                        { p.mu.Lock(); defer p.mu.Unlock(); return p.calls }
func (p *slowProvider) Complete(ctx context.Context, _ llm.Request, _ func(string) error) (llm.Result, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	select {
	case p.started <- struct{}{}:
	default:
	}
	select {
	case <-time.After(5 * time.Second):
		return llm.Result{}, &llm.Error{Kind: llm.KindUnavailable}
	case <-ctx.Done():
		return llm.Result{}, &llm.Error{Kind: llm.KindCanceled, Err: ctx.Err()}
	}
}

// [12] / J9: a session purged by Sweep (expiry) or by capacity eviction has
// its running model call canceled at once and its event log dropped.
func TestStorePurgeCancelsOperations(t *testing.T) {
	for _, tc := range []struct {
		name  string
		purge func(t *testing.T, svc *journey.Service, c *clock, victim string)
	}{
		{"sweep after expiry", func(t *testing.T, svc *journey.Service, c *clock, _ string) {
			c.Add(3 * time.Hour)
			if n := svc.Sweep(); n == 0 {
				t.Fatal("nothing swept")
			}
		}},
		{"capacity eviction", func(t *testing.T, svc *journey.Service, c *clock, victim string) {
			c.Add(11 * time.Minute)
			if _, _, err := svc.CreateSession(journey.CreateInput{Language: "id", AdultAttested: true, PrivacyPolicy: svc.Policy("privacy")}); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.State(victim); code(err) != journey.CodeSessionMissing {
				t.Fatalf("victim not evicted: %v", err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &clock{t: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)}
			p := &slowProvider{started: make(chan struct{}, 1)}
			svc := journey.New(journey.Config{Now: c.Now, OpTimeout: 30 * time.Second}, store.NewMemory(store.Options{MaxSessions: 1, Now: c.Now}), fixtures.MustLoad(), p, llm.NewAdmission(2))
			id, v, _ := svc.CreateSession(journey.CreateInput{Language: "id", AdultAttested: true, PrivacyPolicy: svc.Policy("privacy")})
			v, _ = svc.SetConsent(id, journey.ConsentAI, journey.ConsentInput{ExpectedVersion: v.Version, Granted: true, PolicyVersion: svc.Policy("ai_processing")})
			res, err := svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: v.Version, Mode: journey.StoryScenario, ScenarioID: "id-nadia-overwhelm"})
			if err != nil {
				t.Fatal(err)
			}
			op, err := svc.StartLedgerDraft(id, res.View.Version)
			if err != nil {
				t.Fatal(err)
			}
			<-p.started
			start := time.Now()
			tc.purge(t, svc, c, id)
			svc.Wait()
			if d := time.Since(start); d > 2*time.Second {
				t.Fatalf("model call ran %s after its session was purged", d)
			}
			if _, ok := svc.RuntimeEvents(op.ID); ok {
				t.Fatal("runtime kept after the session was purged")
			}
		})
	}
}

// deleteHookStore runs a hook right before Delete reaches the memory store.
type deleteHookStore struct {
	*store.Memory
	beforeDelete func(id string)
}

func (d *deleteHookStore) Delete(id string) {
	if f := d.beforeDelete; f != nil {
		d.beforeDelete = nil
		f(id)
	}
	d.Memory.Delete(id)
}

// [12] An operation started concurrently with DeleteSession can never run
// on the deleted session: cancel and purge happen under one session lock.
func TestDeleteSessionLeavesNoGapForNewOperations(t *testing.T) {
	var ds *deleteHookStore
	h := newHarnessWith(t, func(now func() time.Time) journey.Store {
		ds = &deleteHookStore{Memory: store.NewMemory(store.Options{Now: now})}
		return ds
	})
	id, _ := h.session("en")
	h.consentAI(id, true)
	h.story(id, "en-sam-shift-work")
	version := h.state(id).Version
	started := make(chan error, 1)
	ds.beforeDelete = func(sid string) {
		// A concurrent request arrives while the deletion is in progress.
		go func() {
			_, err := h.svc.StartLedgerDraft(sid, version)
			started <- err
		}()
		select {
		case err := <-started:
			started <- err
		case <-time.After(300 * time.Millisecond):
		}
	}
	if err := h.svc.DeleteSession(id); err != nil {
		t.Fatal(err)
	}
	var startErr error
	select {
	case startErr = <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent start never returned")
	}
	h.svc.Wait()
	if h.llm.Calls() != 0 {
		t.Fatalf("model called for a deleted session (start err %v)", startErr)
	}
	wantCode(t, startErr, journey.CodeSessionMissing)
}

// [6] / J4: one link cannot exhaust recipient capacity for other visitors;
// each handoff keeps at most three principals and a new sign-in replaces
// the oldest.
func TestRecipientPrincipalsAreBoundedPerHandoff(t *testing.T) {
	h := newHarnessWith(t, func(now func() time.Time) journey.Store {
		return store.NewMemory(store.Options{MaxSessions: 3, Now: now})
	})
	a, _ := h.session("id")
	h.story(a, "id-nadia-overwhelm")
	h.manualApprove(a)
	ca := h.share(a, "key-cap-a-000000001")
	var ids []string
	for i := 0; i < 50; i++ {
		rid, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: ca.Token, Code: ca.Code})
		if err != nil {
			t.Fatalf("sign-in %d: %v", i, err)
		}
		ids = append(ids, rid)
	}
	if got := h.store.Stats().Recipients; got != journey.MaxRecipientsPerHandoff {
		t.Fatalf("recipients for one handoff = %d", got)
	}
	_, err := h.svc.RecipientGet(ids[0])
	wantCode(t, err, journey.CodeAccessEnded)
	for _, rid := range ids[len(ids)-journey.MaxRecipientsPerHandoff:] {
		if _, err := h.svc.RecipientGet(rid); err != nil {
			t.Fatalf("newest principal refused: %v", err)
		}
	}
	b, _ := h.session("en")
	h.story(b, "en-sam-shift-work")
	h.manualApprove(b)
	cb := h.share(b, "key-cap-b-000000001")
	if _, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: cb.Token, Code: cb.Code}); err != nil {
		t.Fatalf("another visitor's sign-in failed: %v", err)
	}
}

// J4: a sign-in that fails for capacity reasons is the uniform access_ended.
func TestRecipientSignInFailureIsUniform(t *testing.T) {
	h := newHarnessWith(t, func(now func() time.Time) journey.Store {
		return store.NewMemory(store.Options{MaxRecipients: 1, Now: now})
	})
	a, _ := h.session("id")
	h.story(a, "id-nadia-overwhelm")
	h.manualApprove(a)
	ca := h.share(a, "key-uniform-a-00001")
	if _, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: ca.Token, Code: ca.Code}); err != nil {
		t.Fatal(err)
	}
	b, _ := h.session("id")
	h.story(b, "id-nadia-overwhelm")
	h.manualApprove(b)
	cb := h.share(b, "key-uniform-b-00001")
	_, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: cb.Token, Code: cb.Code})
	wantCode(t, err, journey.CodeAccessEnded)
}

// distinctSentences returns n short, distinct, non-crisis sentences.
func distinctSentences(n int) string {
	words := []string{"red", "blue", "green", "amber", "violet", "silver", "golden", "olive", "coral", "ivory", "teal", "maroon", "navy", "lime", "plum", "rose", "sand", "slate", "pearl", "mint", "honey", "cocoa", "lilac", "jade", "ruby", "onyx", "azure", "beige", "cream", "denim", "ebony", "fawn", "ginger", "hazel", "indigo", "khaki", "lemon", "mauve", "ochre", "peach"}
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "My fictional character painted the %s fence today. ", words[i%len(words)])
	}
	return strings.TrimSpace(b.String())
}

// [7] / J6: Clarify accepts one answer or skip per unknown field, and the
// item and story caps answer limit_reached.
func TestClarifyIsBounded(t *testing.T) {
	t.Run("one answer or skip per unknown field", func(t *testing.T) {
		h := newHarness(t)
		id, _ := h.session("en")
		h.story(id, "en-sam-shift-work")
		v, err := h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: h.state(id).Version, Field: ledger.UnknownDuration, Answer: "Since the start of the year."})
		if err != nil {
			t.Fatal(err)
		}
		_, err = h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: v.Version, Field: ledger.UnknownDuration, Answer: "Since the start of the year, really."})
		wantCode(t, err, journey.CodeInvalidTransition)
		v, err = h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: v.Version, Field: ledger.UnknownGoal, Skip: true})
		if err != nil {
			t.Fatal(err)
		}
		_, err = h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: v.Version, Field: ledger.UnknownGoal, Answer: "I want to sleep better."})
		wantCode(t, err, journey.CodeInvalidTransition)
		// Removing the answer item makes the field unknown again, but it was
		// answered: it is neither suggested nor accepted again.
		for _, it := range v.Ledger.Items {
			if it.Kind == ledger.KindDuration {
				if v, err = h.svc.ReviewItem(id, it.ID, journey.ItemInput{ExpectedVersion: v.Version, Action: journey.ActionRemove}); err != nil {
					t.Fatal(err)
				}
			}
		}
		if v.Ledger.Clarification == ledger.UnknownDuration {
			t.Fatal("answered field suggested again")
		}
		_, err = h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: v.Version, Field: ledger.UnknownDuration, Answer: "Since the start of the year."})
		wantCode(t, err, journey.CodeInvalidTransition)
		if n := len(h.state(id).Story.Segments); n > 20 {
			t.Fatalf("story grew to %d segments", n)
		}
	})
	t.Run("item cap", func(t *testing.T) {
		h := newHarness(t)
		id, _ := h.session("en")
		v := h.practiceStory(id, distinctSentences(journey.MaxLedgerItems+1))
		for i, seg := range v.Story.Segments {
			var err error
			v, err = h.svc.AddManualItem(id, journey.AddItemInput{ExpectedVersion: v.Version, SegmentID: seg.ID, Kind: ledger.KindExperience})
			if i < journey.MaxLedgerItems && err != nil {
				t.Fatalf("item %d: %v", i, err)
			}
			if i == journey.MaxLedgerItems {
				wantCode(t, err, journey.CodeLimitReached)
			}
		}
		v = h.state(id)
		_, err := h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: v.Version, Field: ledger.UnknownDuration, Answer: "Since the start of the year."})
		wantCode(t, err, journey.CodeLimitReached)
	})
	t.Run("story segment cap", func(t *testing.T) {
		h := newHarness(t)
		id, _ := h.session("en")
		v := h.practiceStory(id, distinctSentences(40))
		_, err := h.svc.Clarify(id, journey.ClarifyInput{ExpectedVersion: v.Version, Field: ledger.UnknownDuration, Answer: strings.Repeat("Yes. ", 30)})
		wantCode(t, err, journey.CodeLimitReached)
	})
}

// [7] / J6: a session holds at most MaxHandoffs handoffs.
func TestHandoffsAreBoundedPerSession(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.story(id, "id-nadia-overwhelm")
	h.manualApprove(id)
	for i := 0; i < journey.MaxHandoffs; i++ {
		c := h.share(id, fmt.Sprintf("key-handoff-cap-%04d", i))
		if _, err := h.svc.RevokeShare(id, c.Handoff.ID, journey.RevokeInput{}); err != nil {
			t.Fatal(err)
		}
	}
	v := h.state(id)
	_, err := h.svc.CreateHandoff(id, "key-handoff-cap-over", journey.HandoffInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version, ProfessionalID: "pro-rani", SlotID: "slot-rani-1",
		Fields: []journey.ShareField{journey.FieldItems}, DurationHours: 24, ShareConsent: true, SharingPolicy: h.svc.Policy("sharing")})
	wantCode(t, err, journey.CodeLimitReached)
	// The receipt of an existing key still replays at the cap.
	if _, err := h.svc.CreateHandoff(id, "key-handoff-cap-0000", journey.HandoffInput{LedgerVersion: v.Ledger.Version, ProfessionalID: "pro-rani", SlotID: "slot-rani-1",
		Fields: []journey.ShareField{journey.FieldItems, journey.FieldDisplayName}, DurationHours: 24, ShareConsent: true, SharingPolicy: h.svc.Policy("sharing")}); err != nil {
		t.Fatalf("replay at the cap: %v", err)
	}
}

// [8] Capacity eviction never purges a storage-consented session with an
// active share.
func TestEvictionSparesRetainedSessions(t *testing.T) {
	h := newHarnessWith(t, func(now func() time.Time) journey.Store {
		return store.NewMemory(store.Options{MaxSessions: 2, Now: now})
	})
	a, _ := h.session("id")
	if _, err := h.svc.SetConsent(a, journey.ConsentStorage, journey.ConsentInput{ExpectedVersion: h.state(a).Version, Granted: true, PolicyVersion: h.svc.Policy("storage")}); err != nil {
		t.Fatal(err)
	}
	h.story(a, "id-nadia-overwhelm")
	h.manualApprove(a)
	ca := h.share(a, "key-evict-a-0000001")
	rid, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: ca.Token, Code: ca.Code})
	if err != nil {
		t.Fatal(err)
	}
	h.clock.Add(time.Minute)
	b, _ := h.session("en")
	h.clock.Add(11 * time.Minute)
	h.state(b)
	_, _, err = h.svc.CreateSession(journey.CreateInput{Language: "id", AdultAttested: true, PrivacyPolicy: h.svc.Policy("privacy")})
	wantCode(t, err, journey.CodeCapacity)
	if _, err := h.svc.State(a); err != nil {
		t.Fatalf("retained session evicted: %v", err)
	}
	if _, err := h.svc.RecipientGet(rid); err != nil {
		t.Fatalf("active share ended by eviction: %v", err)
	}
}

// [8] follow-up: one client filling the store with storage-consented sessions
// and leaving locks new visitors out for at most the ordinary idle timeout,
// not for the 24-hour retention; each new visitor then evicts only the least
// recently seen of those sessions.
func TestIdleProtectedSessionsNeverLockOutVisitors(t *testing.T) {
	h := newHarnessWith(t, func(now func() time.Time) journey.Store {
		return store.NewMemory(store.Options{MaxSessions: 5, Now: now})
	})
	var ids []string
	firstSeen := h.clock.Now()
	for i := 0; i < 5; i++ {
		id, v := h.session("en")
		if _, err := h.svc.SetConsent(id, journey.ConsentStorage, journey.ConsentInput{ExpectedVersion: v.Version, Granted: true, PolicyVersion: h.svc.Policy("storage")}); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		h.clock.Add(time.Minute)
	}
	newVisitor := func() error {
		_, _, err := h.svc.CreateSession(journey.CreateInput{Language: "id", AdultAttested: true, PrivacyPolicy: h.svc.Policy("privacy")})
		return err
	}
	// Within the ordinary idle timeout the retained sessions are spared.
	h.clock.Add(firstSeen.Add(journey.SessionIdle - time.Minute).Sub(h.clock.Now()))
	wantCode(t, newVisitor(), journey.CodeCapacity)
	h.clock.Add(time.Minute)
	if err := newVisitor(); err != nil {
		t.Fatalf("idle storage-consented sessions lock out a new visitor: %v", err)
	}
	if _, err := h.svc.State(ids[0]); code(err) != journey.CodeSessionMissing {
		t.Fatalf("least recently seen session not the one evicted: %v", err)
	}
	for _, id := range ids[1:] {
		if _, err := h.svc.State(id); err != nil {
			t.Fatalf("more than one retained session evicted: %v", err)
		}
	}
}

// [9] / J3: revoke is version-independent and idempotent.
func TestRevokeIsVersionIndependent(t *testing.T) {
	h := newHarness(t)
	id, _ := h.session("id")
	h.story(id, "id-nadia-overwhelm")
	h.manualApprove(id)
	c := h.share(id, "key-revoke-nobody-01")
	v, err := h.svc.RevokeShare(id, c.Handoff.ID, journey.RevokeInput{})
	if err != nil || v.Handoffs[0].Share != journey.ShareRevoked {
		t.Fatalf("revoke without expectedVersion: %v", err)
	}
	if _, err := h.svc.RevokeShare(id, c.Handoff.ID, journey.RevokeInput{ExpectedVersion: 1}); err != nil {
		t.Fatalf("repeat revoke: %v", err)
	}
	// The recipient opening the link bumps the owner's version; a revoke
	// sent with the earlier version still ends access.
	v = h.state(id)
	c2, err := h.svc.CreateHandoff(id, "key-revoke-stale-001", journey.HandoffInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version, ProfessionalID: "pro-maya", SlotID: "slot-maya-1",
		Fields: []journey.ShareField{journey.FieldItems}, DurationHours: 24, ShareConsent: true, SharingPolicy: h.svc.Policy("sharing")})
	if err != nil {
		t.Fatal(err)
	}
	before := h.state(id).Version
	rid, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c2.Token, Code: c2.Code})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.RevokeShare(id, c2.Handoff.ID, journey.RevokeInput{ExpectedVersion: before}); err != nil {
		t.Fatalf("revoke after the recipient opened: %v", err)
	}
	_, err = h.svc.RecipientGet(rid)
	wantCode(t, err, journey.CodeAccessEnded)
}

// [11] Storage decisions never extend a session beyond its bound.
func TestStorageDecisionsNeverExtendTheSession(t *testing.T) {
	t.Run("repeated decline", func(t *testing.T) {
		h := newHarness(t)
		id, v := h.session("id")
		created := v.CreatedAt
		version := v.Version
		for i := 0; i < 12; i++ {
			h.clock.Add(25 * time.Minute)
			v, err := h.svc.SetConsent(id, journey.ConsentStorage, journey.ConsentInput{ExpectedVersion: version, Granted: false})
			if v != nil {
				version = v.Version
			}
			if h.clock.Now().Before(created.Add(journey.SessionMaxLife)) {
				if err != nil || v.ExpiresAt.After(created.Add(journey.SessionMaxLife)) {
					t.Fatalf("decline %d: %v expiresAt=%s", i, err, v.ExpiresAt)
				}
				continue
			}
			wantCode(t, err, journey.CodeSessionMissing)
			return
		}
		t.Fatal("session outlived its absolute maximum")
	})
	t.Run("repeated grant", func(t *testing.T) {
		h := newHarness(t)
		id, _ := h.session("id")
		first := h.clock.Now()
		grant := func() (*journey.SessionView, error) {
			return h.svc.SetConsent(id, journey.ConsentStorage, journey.ConsentInput{ExpectedVersion: h.state(id).Version, Granted: true, PolicyVersion: h.svc.Policy("storage")})
		}
		if _, err := grant(); err != nil {
			t.Fatal(err)
		}
		h.clock.Add(23 * time.Hour)
		v, err := grant()
		if err != nil || !v.ExpiresAt.Equal(first.Add(journey.DayMaxLife)) {
			t.Fatalf("re-grant moved the day limit: %v %s", err, v.ExpiresAt)
		}
		h.clock.Add(time.Hour)
		_, err = h.svc.State(id)
		wantCode(t, err, journey.CodeSessionMissing)
	})
}

// [12] / J5: an active share keeps the owner session alive through idle
// time (never beyond its absolute expiry), and owner and recipient see that
// effective expiry.
func TestActiveShareSuspendsOwnerIdleTimeout(t *testing.T) {
	h := newHarness(t)
	id, created := h.session("id")
	h.story(id, "id-nadia-overwhelm")
	h.manualApprove(id)
	c := h.share(id, "key-idle-expiry-0001")
	absolute := created.CreatedAt.Add(journey.SessionMaxLife)
	if !c.Handoff.ExpiresAt.Equal(absolute) {
		t.Fatalf("receipt expiresAt = %s, want %s", c.Handoff.ExpiresAt, absolute)
	}
	if v := h.state(id); !v.IdleExpiresAt.Equal(absolute) {
		t.Fatalf("idleExpiresAt = %s, want the share's end %s", v.IdleExpiresAt, absolute)
	}
	rid, err := h.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: c.Token, Code: c.Code})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ { // the owner stays idle for an hour
		h.clock.Add(10 * time.Minute)
		rh, err := h.svc.RecipientGet(rid)
		if err != nil {
			t.Fatalf("recipient at +%dm: %v", (i+1)*10, err)
		}
		if !rh.ExpiresAt.Equal(absolute) {
			t.Fatalf("recipient expiresAt = %s", rh.ExpiresAt)
		}
	}
	v := h.state(id)
	if _, err := h.svc.RevokeShare(id, v.Handoffs[0].ID, journey.RevokeInput{}); err != nil {
		t.Fatal(err)
	}
	if v := h.state(id); !v.IdleExpiresAt.Equal(h.clock.Now().Add(journey.SessionIdle)) {
		t.Fatalf("idle timeout not restored after revoke: %s", v.IdleExpiresAt)
	}
	h.clock.Add(journey.SessionIdle + time.Second)
	_, err = h.svc.State(id)
	wantCode(t, err, journey.CodeSessionMissing)
}
