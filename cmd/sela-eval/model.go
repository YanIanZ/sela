package main

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/config"
	"github.com/YanIanZ/sela/internal/fixtures"
	"github.com/YanIanZ/sela/internal/journey"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/store"
)

// H100 list price on Modal, observed 2026-10-26 (ADR-0001), and the Modal
// scale-down window configured in modal/sela_llm.py.
const (
	gpuUSDPerSecond  = 0.001097
	scaledownSeconds = 300
)

type opOutcome struct {
	done    map[string]any
	errCode string
	elapsed time.Duration
}

// runDraft runs one ledger draft for a scenario in a fresh session.
func runDraft(svc *journey.Service, cat *fixtures.Catalog, scenario, lang string) (*journey.SessionView, opOutcome, error) {
	id, v, err := svc.CreateSession(journey.CreateInput{Language: lang, AdultAttested: true, PrivacyPolicy: cat.Policies["privacy"].Version})
	if err != nil {
		return nil, opOutcome{}, err
	}
	if v, err = svc.SetConsent(id, journey.ConsentAI, journey.ConsentInput{ExpectedVersion: v.Version, Granted: true, PolicyVersion: cat.Policies["ai_processing"].Version}); err != nil {
		return nil, opOutcome{}, err
	}
	if _, err = svc.SubmitStory(id, journey.StoryInput{ExpectedVersion: v.Version, Mode: journey.StoryScenario, ScenarioID: scenario}); err != nil {
		return nil, opOutcome{}, err
	}
	v, _ = svc.State(id)
	start := time.Now()
	op, err := svc.StartLedgerDraft(id, v.Version)
	if err != nil {
		return nil, opOutcome{}, err
	}
	sub, err := svc.Subscribe(id, op.ID)
	if err != nil {
		return nil, opOutcome{}, err
	}
	var out opOutcome
	deadline := time.After(15 * time.Minute)
	for {
		events, done, wake := sub.Events(0)
		if done {
			last := events[len(events)-1]
			if m, ok := last.Data.(map[string]any); ok {
				if last.Type == "done" {
					out.done = m
				} else {
					out.errCode = fmt.Sprint(m["code"])
					if r, ok := m["reason"]; ok {
						out.errCode += ":" + fmt.Sprint(r)
					}
				}
			}
			break
		}
		select {
		case <-wake:
		case <-deadline:
			return nil, out, errors.New("operation did not finish in 15 minutes")
		}
	}
	out.elapsed = time.Since(start)
	svc.Wait()
	final, err := svc.State(id)
	return final, out, err
}

var (
	idStop = map[string]bool{"aku": true, "saya": true, "dan": true, "yang": true, "tidak": true, "dengan": true, "ini": true, "itu": true, "di": true, "ke": true, "untuk": true, "tapi": true, "sering": true, "merasa": true, "kalau": true, "juga": true, "karena": true, "sudah": true, "belum": true, "nggak": true, "ingin": true, "bisa": true}
	enStop = map[string]bool{"i": true, "the": true, "and": true, "to": true, "of": true, "my": true, "a": true, "is": true, "it": true, "that": true, "in": true, "for": true, "with": true, "but": true, "not": true, "feel": true, "have": true, "am": true, "would": true, "like": true, "about": true}
	wordRE = regexp.MustCompile(`[\p{L}']+`)
)

// detectLang is a coarse stop-word vote used only for the bilingual metric.
func detectLang(s string) string {
	var id, en int
	for _, w := range wordRE.FindAllString(strings.ToLower(s), -1) {
		if idStop[w] {
			id++
		}
		if enStop[w] {
			en++
		}
	}
	switch {
	case id > en:
		return "id"
	case en > id:
		return "en"
	}
	return "unknown"
}

func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range wordRE.FindAllString(strings.ToLower(s), -1) {
		if len(w) > 2 {
			out[w] = true
		}
	}
	return out
}

// overlap is the share of the gold quote's tokens present in the claim quote.
func overlap(gold, claim string) float64 {
	g, c := tokens(gold), tokens(claim)
	if len(g) == 0 {
		return 0
	}
	n := 0
	for t := range g {
		if c[t] {
			n++
		}
	}
	return float64(n) / float64(len(g))
}

type ledgerTotals struct {
	candidates, rejected, accepted, residual, forbidden int
	important, covered                                  int
	langItems, langMatch                                int
	warm                                                []float64
	cold                                                []float64
	failures                                            []string
}

func (t *ledgerTotals) add(sc *fixtures.Scenario, lang string, v *journey.SessionView, out opOutcome, first bool) {
	if out.errCode != "" || v == nil || v.Ledger == nil {
		t.failures = append(t.failures, fmt.Sprintf("%s/%s: %s", sc.ID, lang, out.errCode))
		return
	}
	segs := map[string]string{}
	for _, s := range v.Story.Segments {
		segs[s.ID] = s.Text
	}
	t.accepted += len(v.Ledger.Items)
	rej := 0
	for _, n := range v.Ledger.Rejected {
		rej += n
	}
	t.rejected += rej
	t.candidates += len(v.Ledger.Items) + rej
	for _, it := range v.Ledger.Items {
		if txt, ok := segs[it.SegmentID]; !ok || it.End > len(txt) || txt[it.Start:it.End] != it.Quote {
			t.residual++
		}
		for _, f := range sc.Forbidden {
			if regexp.MustCompile("(?i)"+f.Pattern).MatchString(it.Wording) && !strings.EqualFold(it.Wording, it.Quote) {
				t.forbidden++
			}
		}
		if it.WordingOrigin == journey.WordingModel {
			t.langItems++
			if detectLang(it.Wording) == lang {
				t.langMatch++
			}
		}
	}
	for _, f := range sc.Facts {
		if !f.Important {
			continue
		}
		t.important++
		for _, it := range v.Ledger.Items {
			if overlap(f.Quote, it.Quote) >= 0.6 {
				t.covered++
				break
			}
		}
	}
	ms := float64(out.elapsed.Milliseconds())
	if first {
		t.cold = append(t.cold, ms)
	} else {
		t.warm = append(t.warm, ms)
	}
}

func percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64{}, xs...)
	sort.Float64s(s)
	i := int(float64(len(s)-1) * p)
	return s[i]
}

// runModel runs ledger drafts for every seeded scenario in both languages
// (and, in test_double mode, the fault-injection battery).
func runModel(rep *Report, root, mode string) error {
	cat, err := fixtures.Load()
	if err != nil {
		return err
	}
	var provider llm.Provider
	var cleanup func()
	switch mode {
	case "live":
		// Only the model settings matter here, not the server's database
		// and HTTP guards.
		cfg, err := config.LoadMode(config.ModeModel)
		if err != nil {
			return err
		}
		if cfg.LLMMode != "modal" {
			return errors.New("live mode requires SELA_LLM_MODE=modal and its settings")
		}
		provider, err = llm.New(llm.Config{Mode: llm.ModeModal, BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, ModalKey: cfg.ModalKey, ModalSecret: cfg.ModalSecret,
			ServedModel: cfg.ServedModel, ModelID: cfg.ModelID, Revision: cfg.ModelRevision})
		if err != nil {
			return err
		}
		cleanup = func() {}
	case "test_double":
		provider, cleanup, err = testDouble("")
		if err != nil {
			return err
		}
	}
	defer cleanup()
	info := provider.Info()
	rep.Provider = map[string]string{"mode": string(info.Mode), "served_model": info.ServedModel, "model_id": info.ModelID, "revision": info.Revision, "endpoint": info.Endpoint}
	svc := journey.New(journey.Config{OpTimeout: 12 * time.Minute, MaxOpsPerWindow: 1000}, store.NewMemory(store.Options{}), cat, provider, llm.NewAdmission(1))
	var tot ledgerTotals
	first := true
	runStart := time.Now()
	for _, sc := range cat.Scenarios {
		if sc.Kind != "seeded" {
			continue
		}
		for _, lang := range []string{"id", "en"} {
			v, out, err := runDraft(svc, cat, sc.ID, lang)
			if err != nil {
				return err
			}
			s := sc
			tot.add(&s, lang, v, out, first)
			first = false
		}
	}
	runSeconds := time.Since(runStart).Seconds()
	rep.put("ledger.raw_rejected_rate", ratio(tot.rejected, tot.candidates), tot.candidates, "candidates Meaning Lock rejected before commit (informational)")
	rep.put("ledger.residual_unsupported_rate", ratio(tot.residual, tot.accepted), tot.accepted, "")
	rep.put("meaning_lock.residual_forbidden_hits", float64(tot.forbidden), tot.accepted, "")
	if len(tot.failures) > 0 {
		rep.note("ledger runs that failed: %s", strings.Join(tot.failures, "; "))
	}
	if mode == "live" {
		rep.put("ledger.important_fact_coverage", ratio(tot.covered, tot.important), tot.important, "")
		rep.put("ledger.bilingual_wording_language_match", ratio(tot.langMatch, tot.langItems), tot.langItems, "stop-word language vote on model wording")
		rep.put("latency.ledger_draft_p50_warm_ms", percentile(tot.warm, 0.5), len(tot.warm), "")
		rep.put("latency.ledger_draft_p95_warm_ms", percentile(tot.warm, 0.95), len(tot.warm), "")
		if len(tot.cold) > 0 {
			rep.put("latency.first_request_ms", tot.cold[0], 1, "includes any cold start (informational)")
		}
		avg := runSeconds / float64(len(tot.warm)+len(tot.cold))
		est := (percentile(tot.cold, 0)/1000 + 3*avg + scaledownSeconds) * gpuUSDPerSecond
		rep.put("cost.per_demo_session_usd", est, 1, "estimate: first request + 3 operations + scale-down window at H100 list price")
	} else {
		rep.put("ledger.test_double_coverage", ratio(tot.covered, tot.important), tot.important, "TEST DOUBLE: measures the pipeline, not Meta model quality")
	}
	if mode == "test_double" {
		return runFaults(rep, cat)
	}
	return nil
}
