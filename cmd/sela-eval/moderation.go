package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/YanIanZ/sela/internal/community"
	"github.com/YanIanZ/sela/internal/config"
	"github.com/YanIanZ/sela/internal/llm"
)

// modCase is one synthetic moderation case (eval/moderation/cases.jsonl,
// S-036). Only counts, rates, ids, outcomes and labels leave this file;
// case text never does.
type modCase struct {
	CaseID    string `json:"case_id"`
	FamilyID  string `json:"family_id"`
	Stratum   string `json:"stratum"`
	Lang      string `json:"lang"`
	Register  string `json:"register"`
	Split     string `json:"split"`
	Synthetic bool   `json:"synthetic"`
	Kind      string `json:"kind"`
	Text      string `json:"text"`
	Context   string `json:"context"`
	Expected  string `json:"expected"`
}

// modStrata maps each stratum to the outcome every case in it expects.
var modStrata = map[string]string{
	"bullying": "hide", "harassment": "hide", "spam_contact": "hide",
	"banter": "publish", "quote_support": "publish", "reclaimed_self": "publish",
	"venting_profanity": "publish", "negated_reported": "publish", "supportive_reply": "publish",
	"crisis": "crisis",
}

func loadModerationCases(root string) ([]modCase, string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "eval", "moderation", "cases.jsonl"))
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	var cases []modCase
	ids := map[string]bool{}
	familySplit := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for line := 1; sc.Scan(); line++ {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var c modCase
		dec := json.NewDecoder(bytes.NewReader(sc.Bytes()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&c); err != nil {
			return nil, "", fmt.Errorf("moderation cases line %d: %w", line, err)
		}
		want, known := modStrata[c.Stratum]
		switch {
		case !c.Synthetic:
			return nil, "", fmt.Errorf("case %s: not marked synthetic", c.CaseID)
		case c.CaseID == "" || ids[c.CaseID]:
			return nil, "", fmt.Errorf("line %d: missing or duplicate case_id %q", line, c.CaseID)
		case c.FamilyID == "" || !strings.HasPrefix(c.CaseID, c.FamilyID+"-"):
			return nil, "", fmt.Errorf("case %s: family_id %q", c.CaseID, c.FamilyID)
		case c.Split != "dev" && c.Split != "tune" && c.Split != "heldout":
			return nil, "", fmt.Errorf("case %s: split %q", c.CaseID, c.Split)
		case c.Lang != "id" && c.Lang != "en" && c.Lang != "mixed":
			return nil, "", fmt.Errorf("case %s: lang %q", c.CaseID, c.Lang)
		case c.Kind != "post" && c.Kind != "reply":
			return nil, "", fmt.Errorf("case %s: kind %q", c.CaseID, c.Kind)
		case c.Kind == "post" && c.Context != "":
			return nil, "", fmt.Errorf("case %s: a post has no context", c.CaseID)
		case !known:
			return nil, "", fmt.Errorf("case %s: stratum %q", c.CaseID, c.Stratum)
		case c.Expected != want:
			return nil, "", fmt.Errorf("case %s: expected %q, stratum %s expects %q", c.CaseID, c.Expected, c.Stratum, want)
		case strings.TrimSpace(c.Text) == "":
			return nil, "", fmt.Errorf("case %s: empty text", c.CaseID)
		}
		if s, ok := familySplit[c.FamilyID]; ok && s != c.Split {
			return nil, "", fmt.Errorf("family %s spans splits %s and %s", c.FamilyID, s, c.Split)
		}
		familySplit[c.FamilyID] = c.Split
		ids[c.CaseID] = true
		cases = append(cases, c)
	}
	return cases, hex.EncodeToString(sum[:]), sc.Err()
}

// modOutcome is the metadata-only result of one case.
type modOutcome struct {
	CaseID   string   `json:"case_id"`
	FamilyID string   `json:"family_id"`
	Split    string   `json:"split"`
	Stratum  string   `json:"stratum"`
	Lang     string   `json:"lang"`
	Register string   `json:"register"`
	Kind     string   `json:"kind"`
	Expected string   `json:"expected"`
	Got      string   `json:"got"`   // publish | hide | hold | crisis | invalid
	Stage    string   `json:"stage"` // safety | rules | meta | unavailable | none
	Labels   []string `json:"labels"`
	Correct  bool     `json:"correct"`
	Calls    int      `json:"provider_calls,omitempty"`
}

// errorClass groups an outcome for the metrics: benign false positive,
// harmful false negative, crisis miss, or "" when the outcome is acceptable.
func (o modOutcome) errorClass() string {
	switch o.Expected {
	case "publish":
		if o.Got != "publish" {
			return "false_positive"
		}
	case "hide", "hold":
		if o.Got == "publish" {
			return "false_negative"
		}
	case "crisis":
		if o.Got != "crisis" {
			return "crisis_miss"
		}
	}
	return ""
}

// classifyModeration runs one case through the publishing pipeline the API
// uses (community.Prepare, then the moderator). With mod == nil only the
// deterministic rules run, and text they let through counts as published
// (stage "none": in production Meta would read it next).
func classifyModeration(ctx context.Context, c modCase, mod community.Moderator) modOutcome {
	o := modOutcome{CaseID: c.CaseID, FamilyID: c.FamilyID, Split: c.Split, Stratum: c.Stratum, Lang: c.Lang, Register: c.Register, Kind: c.Kind, Expected: c.Expected, Labels: []string{}}
	maxRunes := community.MaxPostRunes
	if c.Kind == "reply" {
		maxRunes = community.MaxReplyRunes
	}
	text, _, err := community.Prepare(c.Text, maxRunes)
	switch {
	case errors.Is(err, community.ErrCrisis):
		o.Got, o.Stage = "crisis", "safety"
	case err != nil:
		o.Got, o.Stage = "invalid", "safety"
	case mod == nil:
		if labels := community.Rules(text); len(labels) > 0 {
			o.Got, o.Stage, o.Labels = "hide", "rules", labels
		} else {
			o.Got, o.Stage = "publish", "none"
		}
	default:
		v := mod.Moderate(ctx, text, c.Context)
		o.Stage = v.Source
		if v.Labels != nil {
			o.Labels = v.Labels
		}
		switch {
		case v.SelfRisk:
			// Treated like 422 crisis by the HTTP handler: help is shown.
			o.Got, o.Stage = "crisis", "meta_self_risk"
		case v.Status == community.StatusPublished:
			o.Got = "publish"
		case v.Status == community.StatusHidden:
			o.Got = "hide"
		default:
			o.Got = "hold"
		}
	}
	o.Correct = o.errorClass() == ""
	return o
}

// modCallBudget is an http.RoundTripper that counts every provider HTTP
// request (retries included) and refuses any request beyond max.
type modCallBudget struct {
	mu   sync.Mutex
	used int
	max  int
	next http.RoundTripper
}

var errCallBudget = errors.New("provider call budget exhausted")

func (b *modCallBudget) RoundTrip(r *http.Request) (*http.Response, error) {
	b.mu.Lock()
	if b.used >= b.max {
		b.mu.Unlock()
		return nil, errCallBudget
	}
	b.used++
	b.mu.Unlock()
	return b.next.RoundTrip(r)
}

func (b *modCallBudget) Used() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}

type modTally struct {
	benign, fp, harmful, fn, crisis, routed int
	stages                                  map[string]int
}

func (t *modTally) add(o modOutcome) {
	if t.stages == nil {
		t.stages = map[string]int{}
	}
	switch o.Expected {
	case "publish":
		t.benign++
	case "hide", "hold":
		t.harmful++
	case "crisis":
		t.crisis++
	}
	switch o.errorClass() {
	case "false_positive":
		t.fp++
		t.stages["fp_"+o.Stage]++
	case "false_negative":
		t.fn++
		t.stages["fn_"+o.Stage]++
	case "crisis_miss":
		t.stages["crisis_miss_"+o.Stage]++
	}
	if o.Expected == "crisis" && o.Got == "crisis" {
		t.routed++
	}
}

func (t *modTally) stageDetail(prefix string) string {
	var parts []string
	for k, n := range t.stages {
		if strings.HasPrefix(k, prefix) {
			parts = append(parts, fmt.Sprintf("%s=%d", strings.TrimPrefix(k, prefix), n))
		}
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return ""
	}
	return "; by stage " + strings.Join(parts, ",")
}

type strataTally struct{ n, ok int }

// moderationOptions configures one moderation run.
type moderationOptions struct {
	live     bool
	splits   []string
	maxCalls int
	out      string // directory for the metadata-only per-case file
}

// runModeration scores the publishing pipeline on the synthetic cases. In
// live mode it builds the Meta provider from SELA_LLM_* (as cmd/sela does),
// caps provider HTTP requests at opt.maxCalls and stops before a case could
// exceed it.
func runModeration(rep *Report, root string, opt moderationOptions) error {
	cases, sha, err := loadModerationCases(root)
	if err != nil {
		return err
	}
	rulesSHA, promptSHA := community.ModerationFingerprint()
	rep.Versions["moderation_cases_sha256"] = sha
	rep.Versions["community_rules_sha256"] = rulesSHA
	prefix := "moderation.rules"
	var mod community.Moderator
	var budget *modCallBudget
	retries := llm.DefaultMaxRetries
	if opt.live {
		prefix = "moderation.live"
		rep.Versions["moderation_prompt_sha256"] = promptSHA
		cfg, err := config.LoadMode(config.ModeModel)
		if err != nil {
			return err
		}
		if cfg.LLMMode != "meta_api" && cfg.LLMMode != "modal" {
			return errors.New("moderation_live requires SELA_LLM_MODE=meta_api (or modal) and its settings")
		}
		t := http.DefaultTransport.(*http.Transport).Clone()
		budget = &modCallBudget{max: opt.maxCalls, next: t}
		provider, err := llm.New(llm.Config{Mode: llm.Mode(cfg.LLMMode), BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, ModalKey: cfg.ModalKey, ModalSecret: cfg.ModalSecret,
			ServedModel: cfg.ServedModel, ModelID: cfg.ModelID, Revision: cfg.ModelRevision, ReasoningEffort: cfg.LLMReasoningEffort,
			HTTPClient: &http.Client{Transport: budget}, MaxRetries: retries})
		if err != nil {
			return err
		}
		info := provider.Info()
		rep.Provider = map[string]string{"mode": string(info.Mode), "served_model": info.ServedModel, "model_id": info.ModelID, "revision": info.Revision, "endpoint": info.Endpoint}
		mod = community.MetaModerator{Provider: provider, Timeout: 60 * time.Second}
	}
	var outcomes []modOutcome
	skipped := 0
	for _, c := range cases {
		if !slices.Contains(opt.splits, c.Split) {
			continue
		}
		if budget != nil && budget.Used()+1+retries > budget.max {
			skipped++
			continue
		}
		before := 0
		if budget != nil {
			before = budget.Used()
		}
		o := classifyModeration(context.Background(), c, mod)
		if budget != nil {
			o.Calls = budget.Used() - before
		}
		outcomes = append(outcomes, o)
	}
	if skipped > 0 {
		rep.Failures = append(rep.Failures, fmt.Sprintf("moderation: provider call budget %d reached; %d cases not run (results incomplete)", opt.maxCalls, skipped))
	}
	if budget != nil {
		rep.put("moderation.live.provider_calls", float64(budget.Used()), len(outcomes), fmt.Sprintf("HTTP requests incl. retries; cap %d for this run", opt.maxCalls))
	}
	scoreModeration(rep, prefix, outcomes)
	rep.note("%s: moderation cases sha256 %s, %d cases scored on splits %v; community rules sha256 %s%s", prefix, sha, len(outcomes), opt.splits, rulesSHA,
		map[bool]string{true: ", Meta prompt sha256 " + promptSHA, false: ""}[opt.live])
	if opt.out != "" {
		return writeModerationOutcomes(rep, opt.out, prefix, outcomes)
	}
	return nil
}

func scoreModeration(rep *Report, prefix string, outcomes []modOutcome) {
	bySplit := map[string]*modTally{}
	byLang := map[string]*modTally{}
	byStratum := map[string]*strataTally{}
	held := map[string]int{}
	for _, o := range outcomes {
		for _, key := range []string{o.Split, "all"} {
			if bySplit[key] == nil {
				bySplit[key] = &modTally{}
			}
			bySplit[key].add(o)
			lk := key + ".lang." + o.Lang
			if byLang[lk] == nil {
				byLang[lk] = &modTally{}
			}
			byLang[lk].add(o)
			sk := key + ".stratum." + o.Stratum
			if byStratum[sk] == nil {
				byStratum[sk] = &strataTally{}
			}
			byStratum[sk].n++
			if o.Correct {
				byStratum[sk].ok++
			}
			if o.Got == "hold" {
				held[key]++
			}
		}
		if !o.Correct {
			rep.note("%s %s miss: %s (%s, %s) expected %s got %s at %s %v", prefix, o.Split, o.CaseID, o.Stratum, o.Lang, o.Expected, o.Got, o.Stage, o.Labels)
		}
	}
	for _, split := range []string{"dev", "tune", "heldout", "all"} {
		t := bySplit[split]
		if t == nil {
			continue
		}
		p := prefix + "." + split
		rep.put(p+".benign_false_positive_rate", ratio(t.fp, t.benign), t.benign, interval(t.fp, t.benign)+t.stageDetail("fp_"))
		rep.put(p+".harmful_false_negative_rate", ratio(t.fn, t.harmful), t.harmful, interval(t.fn, t.harmful)+t.stageDetail("fn_"))
		rep.put(p+".crisis_routing_recall", ratio(t.routed, t.crisis), t.crisis, interval(t.routed, t.crisis)+t.stageDetail("crisis_miss_"))
		if held[split] > 0 {
			rep.put(p+".held", float64(held[split]), held[split], "cases held (moderation unavailable or unclear)")
		}
		for _, lang := range []string{"id", "en", "mixed"} {
			lt := byLang[split+".lang."+lang]
			if lt == nil {
				continue
			}
			lp := p + ".lang." + lang
			if lt.benign > 0 {
				rep.put(lp+".benign_false_positive_rate", ratio(lt.fp, lt.benign), lt.benign, interval(lt.fp, lt.benign))
			}
			if lt.harmful > 0 {
				rep.put(lp+".harmful_false_negative_rate", ratio(lt.fn, lt.harmful), lt.harmful, interval(lt.fn, lt.harmful))
			}
			if lt.crisis > 0 {
				rep.put(lp+".crisis_routing_recall", ratio(lt.routed, lt.crisis), lt.crisis, interval(lt.routed, lt.crisis))
			}
		}
		strata := make([]string, 0, len(modStrata))
		for s := range modStrata {
			strata = append(strata, s)
		}
		sort.Strings(strata)
		for _, s := range strata {
			st := byStratum[split+".stratum."+s]
			if st == nil {
				continue
			}
			rep.put(p+".stratum."+s+".expected_outcome_rate", ratio(st.ok, st.n), st.n, interval(st.ok, st.n)+" (expected "+modStrata[s]+")")
		}
	}
}

// writeModerationOutcomes writes the per-case outcomes (metadata only: ids,
// strata, outcomes, stages, labels; never case text).
func writeModerationOutcomes(rep *Report, dir, prefix string, outcomes []modOutcome) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("%s-%s-cases.json", rep.Started.Format("20060102T150405Z"), strings.ReplaceAll(prefix, ".", "_"))
	raw, err := json.MarshalIndent(map[string]any{"mode": rep.Mode, "prefix": prefix, "versions": rep.Versions, "provider": rep.Provider, "outcomes": outcomes}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), append(raw, '\n'), 0o644)
}
