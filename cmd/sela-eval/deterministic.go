package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/YanIanZ/sela/internal/fixtures"
	"github.com/YanIanZ/sela/internal/journey"
	"github.com/YanIanZ/sela/internal/ledger"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/research"
	"github.com/YanIanZ/sela/internal/safety"
	"github.com/YanIanZ/sela/internal/store"
)

func versions() map[string]string {
	return map[string]string{
		"safety_ruleset":   safety.RulesetVersion,
		"ledger_schema":    ledger.SchemaVersion,
		"ledger_validator": ledger.ValidatorVersion,
		"prompts":          journey.PromptVersion,
		"fixtures":         fixtures.Version,
	}
}

type vector struct {
	Text  string   `json:"text"`
	Want  string   `json:"want"`
	Langs []string `json:"langs"`
	Tags  []string `json:"tags"`
}

func hasTag(v vector, tag string) bool {
	for _, t := range v.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// runSafety measures the deterministic lexicon on the labelled vectors.
func runSafety(rep *Report, root string) error {
	raw, err := os.ReadFile(filepath.Join(root, "internal", "safety", "testdata", "vectors.json"))
	if err != nil {
		return err
	}
	var vs []vector
	if err := json.Unmarshal(raw, &vs); err != nil {
		return err
	}
	rank := map[safety.Level]int{safety.LevelNone: 0, safety.LevelElevated: 1, safety.LevelCrisis: 2}
	var crisisN, crisisHit, elevN, elevHit, idiomN, idiomFP, exact int
	byTag := map[string][2]int{}
	for _, v := range vs {
		got := safety.Assess(v.Text).Level
		if string(got) == v.Want {
			exact++
		}
		switch v.Want {
		case "crisis":
			crisisN++
			if got == safety.LevelCrisis {
				crisisHit++
			}
		case "elevated":
			elevN++
			if rank[got] >= rank[safety.LevelElevated] {
				elevHit++
			}
		}
		if hasTag(v, "idiom") && v.Want == "none" {
			idiomN++
			if got != safety.LevelNone {
				idiomFP++
			}
		}
		for _, t := range v.Tags {
			c := byTag[t]
			c[1]++
			if string(got) == v.Want {
				c[0]++
			}
			byTag[t] = c
		}
	}
	rep.put("safety.crisis_vector_recall", ratio(crisisHit, crisisN), crisisN, "")
	rep.put("safety.elevated_vector_recall", ratio(elevHit, elevN), elevN, "elevated or crisis counts as shown support")
	rep.put("safety.idiom_false_positive_rate", ratio(idiomFP, idiomN), idiomN, "")
	rep.put("safety.exact_level_agreement", ratio(exact, len(vs)), len(vs), "informational: exact label match over all vectors")
	tags := make([]string, 0, len(byTag))
	for t := range byTag {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	parts := make([]string, 0, len(tags))
	for _, t := range tags {
		parts = append(parts, fmt.Sprintf("%s %d/%d", t, byTag[t][0], byTag[t][1]))
	}
	rep.note("safety exact agreement by tag: %s", strings.Join(parts, ", "))
	return nil
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// locate finds a gold quote in the scenario's segments (case-insensitive).
func locate(segs []ledger.Segment, quote string) (ledger.Segment, int, int, bool) {
	q := strings.ToLower(quote)
	for _, s := range segs {
		if i := strings.Index(strings.ToLower(s.Text), q); i >= 0 {
			return s, i, i + len(quote), true
		}
	}
	return ledger.Segment{}, 0, 0, false
}

// runQualifiers compares deterministic Meaning Lock analysis to the gold
// annotations of every seeded scenario.
func runQualifiers(rep *Report, root string) error {
	cat, err := fixtures.Load()
	if err != nil {
		return err
	}
	var n, neg, subj, tim, cert int
	var misses []string
	for _, sc := range cat.Scenarios {
		segs := ledger.SplitSegments(sc.Story, "story", 1)
		for _, f := range sc.Facts {
			seg, s, e, ok := locate(segs, f.Quote)
			if !ok {
				misses = append(misses, sc.ID+": quote spans segments: "+f.Quote)
				continue
			}
			q := ledger.Analyze(seg, s, e)
			n++
			if q.Negated == f.Negated {
				neg++
			} else {
				misses = append(misses, fmt.Sprintf("%s negation %q", sc.ID, f.Quote))
			}
			if string(q.Subject) == f.Subject {
				subj++
			} else {
				misses = append(misses, fmt.Sprintf("%s subject %q got %s want %s", sc.ID, f.Quote, q.Subject, f.Subject))
			}
			if string(q.Time) == f.Time {
				tim++
			} else {
				misses = append(misses, fmt.Sprintf("%s time %q got %s want %s", sc.ID, f.Quote, q.Time, f.Time))
			}
			if string(q.Certainty) == f.Certainty {
				cert++
			} else {
				misses = append(misses, fmt.Sprintf("%s certainty %q got %s want %s", sc.ID, f.Quote, q.Certainty, f.Certainty))
			}
		}
	}
	rep.put("meaning_lock.gold_qualifier_negation_accuracy", ratio(neg, n), n, "")
	rep.put("meaning_lock.gold_qualifier_subject_accuracy", ratio(subj, n), n, "")
	rep.put("meaning_lock.gold_qualifier_time_accuracy", ratio(tim, n), n, "")
	rep.put("meaning_lock.gold_qualifier_certainty_accuracy", ratio(cert, n), n, "")
	if len(misses) > 0 {
		rep.note("qualifier disagreements (synthetic gold, quotes are fictional): %s", strings.Join(misses, "; "))
	}
	return nil
}

var ridPattern = regexp.MustCompile(`\bR\d{3}\b`)

// runRetrieval checks curated feature queries and citation validity.
func runRetrieval(rep *Report, root string) error {
	idx, err := research.Load(filepath.Join(root, "data", "research", "corpus", "knowledge_base.jsonl"))
	if err != nil {
		rep.note("retrieval not measured: %v", err)
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(root, "eval", "retrieval_queries.json"))
	if err != nil {
		return err
	}
	var qs struct {
		Queries []struct {
			Query  string   `json:"query"`
			Expect []string `json:"expect"`
		} `json:"queries"`
	}
	if err := json.Unmarshal(raw, &qs); err != nil {
		return err
	}
	hit := 0
	var missed []string
	for _, q := range qs.Queries {
		res := idx.Search(q.Query, "", "", 5).(research.SearchResult)
		found := false
		for _, h := range res.Results {
			for _, e := range q.Expect {
				found = found || h.Record.ID == e
			}
		}
		if found {
			hit++
		} else {
			missed = append(missed, q.Query)
		}
	}
	rep.put("retrieval.expected_in_top5", ratio(hit, len(qs.Queries)), len(qs.Queries), "")
	if len(missed) > 0 {
		rep.note("retrieval misses: %s", strings.Join(missed, "; "))
	}
	known := map[string]bool{}
	for _, id := range idx.IDs() {
		known[id] = true
	}
	cited := map[string]bool{}
	sources, _ := filepath.Glob(filepath.Join(root, "docs", "research", "sela-relay", "*.md"))
	sources = append(sources, filepath.Join(root, "data", "research", "corpus", "evidence_map.json"))
	for _, p := range sources {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, id := range ridPattern.FindAllString(string(b), -1) {
			cited[id] = true
		}
	}
	valid := 0
	var bad []string
	for id := range cited {
		if known[id] {
			valid++
		} else {
			bad = append(bad, id)
		}
	}
	sort.Strings(bad)
	rep.put("retrieval.citation_validity", ratio(valid, len(cited)), len(cited), strings.Join(bad, ","))
	return nil
}

// countingProvider is a TEST-ONLY scripted provider used to check that the
// crisis path never calls a model.
type countingProvider struct {
	mu    sync.Mutex
	calls int
}

func (c *countingProvider) Info() llm.Info {
	return llm.Info{Mode: llm.ModeTestDouble, ServedModel: "test-double-not-meta"}
}
func (c *countingProvider) Health(context.Context) llm.Health { return llm.Health{State: "ok"} }
func (c *countingProvider) Complete(ctx context.Context, req llm.Request, onDelta func(string) error) (llm.Result, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return llm.Result{Text: `{"claims":[],"unknowns":[],"clarification":{"field":"none"}}`, Model: "test-double-not-meta"}, nil
}

type evalClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *evalClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *evalClock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type rig struct {
	svc   *journey.Service
	clock *evalClock
	cat   *fixtures.Catalog
	llm   *countingProvider
}

func newRig() (*rig, error) {
	cat, err := fixtures.Load()
	if err != nil {
		return nil, err
	}
	c := &evalClock{t: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)}
	p := &countingProvider{}
	svc := journey.New(journey.Config{Now: c.now, OpTimeout: 10 * time.Second}, store.NewMemory(store.Options{Now: c.now}), cat, p, llm.NewAdmission(2))
	return &rig{svc: svc, clock: c, cat: cat, llm: p}, nil
}

func (r *rig) session(lang string) (string, error) {
	id, _, err := r.svc.CreateSession(journey.CreateInput{Language: lang, AdultAttested: true, PrivacyPolicy: r.cat.Policies["privacy"].Version})
	return id, err
}

func (r *rig) version(id string) int {
	v, err := r.svc.State(id)
	if err != nil {
		return -1
	}
	return v.Version
}

// approvedShare builds a manual, approved brief and shares it.
func (r *rig) approvedShare(id, key, pro, slot string, hours int) (*journey.HandoffCreated, error) {
	if _, err := r.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: r.version(id), Mode: journey.StoryScenario, ScenarioID: "id-nadia-overwhelm"}); err != nil {
		return nil, err
	}
	v, _ := r.svc.State(id)
	for _, seg := range v.Story.Segments[:3] {
		var err error
		if v, err = r.svc.AddManualItem(id, journey.AddItemInput{ExpectedVersion: v.Version, SegmentID: seg.ID, Kind: ledger.KindExperience}); err != nil {
			return nil, err
		}
	}
	v, err := r.svc.Approve(id, journey.ApproveInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version})
	if err != nil {
		return nil, err
	}
	return r.svc.CreateHandoff(id, key, journey.HandoffInput{ExpectedVersion: v.Version, LedgerVersion: v.Ledger.Version, ProfessionalID: pro, SlotID: slot,
		Fields: []journey.ShareField{journey.FieldItems}, DurationHours: hours, ShareConsent: true, SharingPolicy: r.cat.Policies["sharing"].Version})
}

// runAuthorization attempts every out-of-scope access the API must refuse
// and counts the ones that succeed (violations).
func runAuthorization(rep *Report, root string) error {
	r, err := newRig()
	if err != nil {
		return err
	}
	a, _ := r.session("id")
	b, _ := r.session("en")
	// Both visitors keep 24-hour retention so the expiry checks below (which
	// move the clock past the idle limit) end shares, not sessions.
	for _, id := range []string{a, b} {
		if _, err := r.svc.SetConsent(id, journey.ConsentStorage, journey.ConsentInput{ExpectedVersion: r.version(id), Granted: true, PolicyVersion: r.cat.Policies["storage"].Version}); err != nil {
			return err
		}
	}
	share, err := r.approvedShare(a, "eval-auth-key-000001", "pro-rani", "slot-rani-1", 1)
	if err != nil {
		return err
	}
	attempts := []struct {
		name string
		fn   func() error
	}{
		{"token without code", func() error {
			_, err := r.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: share.Token})
			return err
		}},
		{"code with wrong token", func() error {
			_, err := r.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: share.Token[:len(share.Token)-2] + "xx", Code: share.Code})
			return err
		}},
		{"other session revokes", func() error {
			_, err := r.svc.RevokeShare(b, share.Handoff.ID, journey.RevokeInput{ExpectedVersion: r.version(b)})
			return err
		}},
		{"other session reads ops", func() error {
			v, _ := r.svc.State(a)
			for _, o := range v.Ops {
				if _, err := r.svc.Subscribe(b, o.ID); err == nil {
					return nil
				}
			}
			_, err := r.svc.Subscribe(b, "op_guess")
			return err
		}},
		{"other session check-in on foreign handoff", func() error {
			_, err := r.svc.CheckIn(b, journey.CheckInInput{ExpectedVersion: r.version(b), HandoffID: share.Handoff.ID, OptionID: "checkin-scheduled"})
			return err
		}},
	}
	rid, err := r.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: share.Token, Code: share.Code})
	if err != nil {
		return fmt.Errorf("legitimate recipient refused: %w", err)
	}
	h, _ := r.svc.RecipientGet(rid)
	attempts = append(attempts,
		struct {
			name string
			fn   func() error
		}{"recipient completes before acknowledging", func() error {
			_, err := r.svc.RecipientAct(rid, journey.RecipientActionInput{ExpectedVersion: h.Version, Action: "complete", PlanID: "plan-psychologist-followup"})
			return err
		}},
		struct {
			name string
			fn   func() error
		}{"recipient uses a plan of another professional", func() error {
			if _, err := r.svc.RecipientAct(rid, journey.RecipientActionInput{ExpectedVersion: h.Version, Action: "acknowledge"}); err != nil {
				return nil // counted by the next attempts
			}
			h2, _ := r.svc.RecipientGet(rid)
			_, err := r.svc.RecipientAct(rid, journey.RecipientActionInput{ExpectedVersion: h2.Version, Action: "complete", PlanID: "plan-psychiatrist-followup"})
			return err
		}},
	)
	violations := 0
	var which []string
	for _, at := range attempts {
		if at.fn() == nil {
			violations++
			which = append(which, at.name)
		}
	}
	// After expiry (1-hour share), the principal and new sign-ins must fail.
	r.clock.add(61 * time.Minute)
	r.version(a)
	if _, err := r.svc.RecipientGet(rid); err == nil {
		violations++
		which = append(which, "recipient after expiry")
	}
	if _, err := r.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: share.Token, Code: share.Code}); err == nil {
		violations++
		which = append(which, "sign-in after expiry")
	}
	// Revocation and deletion.
	share2, err := r.svc.CreateHandoff(a, "eval-auth-key-000002", journey.HandoffInput{ExpectedVersion: r.version(a), LedgerVersion: mustLedgerVersion(r, a), ProfessionalID: "pro-maya", SlotID: "slot-maya-1",
		Fields: []journey.ShareField{journey.FieldItems}, DurationHours: 24, ShareConsent: true, SharingPolicy: r.cat.Policies["sharing"].Version})
	if err != nil {
		return err
	}
	rid2, err := r.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: share2.Token, Code: share2.Code})
	if err != nil {
		return err
	}
	if _, err := r.svc.RevokeShare(a, share2.Handoff.ID, journey.RevokeInput{ExpectedVersion: r.version(a)}); err != nil {
		return err
	}
	if _, err := r.svc.RecipientGet(rid2); err == nil {
		violations++
		which = append(which, "recipient after revoke")
	}
	share3, err := r.svc.CreateHandoff(a, "eval-auth-key-000003", journey.HandoffInput{ExpectedVersion: r.version(a), LedgerVersion: mustLedgerVersion(r, a), ProfessionalID: "pro-campus", SlotID: "slot-campus-1",
		Fields: []journey.ShareField{journey.FieldItems}, DurationHours: 24, ShareConsent: true, SharingPolicy: r.cat.Policies["sharing"].Version})
	if err != nil {
		return err
	}
	rid3, _ := r.svc.AuthenticateRecipient(journey.RecipientAuthInput{Token: share3.Token, Code: share3.Code})
	_ = r.svc.DeleteSession(a)
	if _, err := r.svc.RecipientGet(rid3); err == nil {
		violations++
		which = append(which, "recipient after owner deletion")
	}
	if _, err := r.svc.State(b); err != nil {
		violations++
		which = append(which, "deleting A affected B")
	}
	rep.put("authorization.violations", float64(violations), len(attempts)+5, strings.Join(which, "; "))
	return nil
}

func mustLedgerVersion(r *rig, id string) int {
	v, err := r.svc.State(id)
	if err != nil || v.Ledger == nil {
		return -1
	}
	return v.Ledger.Version
}

// runCrisisPath checks that the crisis fixture causes zero model calls.
func runCrisisPath(rep *Report, root string) error {
	r, err := newRig()
	if err != nil {
		return err
	}
	id, _ := r.session("id")
	if _, err := r.svc.SetConsent(id, journey.ConsentAI, journey.ConsentInput{ExpectedVersion: r.version(id), Granted: true, PolicyVersion: r.cat.Policies["ai_processing"].Version}); err != nil {
		return err
	}
	res, err := r.svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: r.version(id), Mode: journey.StoryScenario, ScenarioID: "id-crisis-interrupt"})
	if err != nil {
		return err
	}
	_, _ = r.svc.StartLedgerDraft(id, r.version(id))
	r.svc.Wait()
	calls := r.llm.calls
	detail := ""
	if res.Safety != safety.LevelCrisis {
		calls++
		detail = "crisis fixture did not route to crisis"
	}
	rep.put("safety.model_calls_on_crisis_path", float64(calls), 1, detail)
	return nil
}
