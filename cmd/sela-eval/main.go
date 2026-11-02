// Command sela-eval runs Sela's evaluation suite against the pre-registered
// thresholds in eval/thresholds.json and writes a JSON + Markdown report.
//
// Modes:
//
//	-mode offline      deterministic checks only (safety lexicon, gold
//	                   qualifiers, retrieval, citations, authorization,
//	                   MIE Meaning Diff on eval/mie/cases.jsonl)
//	-mode test_double  offline + ledger/fault runs against the in-process
//	                   cmd/fakellm handler (TEST ONLY; never Meta inference)
//	-mode live         offline + ledger runs against the configured Modal
//	                   endpoint (SELA_LLM_* env, as for cmd/sela)
//	-mode moderation   S-036 only: the deterministic part of the community
//	                   publishing pipeline (safety router + community rules)
//	                   on eval/moderation/cases.jsonl (also part of offline)
//	-mode moderation_live
//	                   S-036 only: the full pipeline with Meta moderation
//	                   (SELA_LLM_* env, as for cmd/sela) on -splits, capped at
//	                   -max-calls provider HTTP requests including retries
//
// Metrics marked "live" in the thresholds are reported as not_measured in
// the other modes. All inputs are synthetic fixtures; outputs contain counts,
// rates and ids, never model text.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type threshold struct {
	Op    string  `json:"op"`
	Value float64 `json:"value"`
	Mode  string  `json:"mode"`
	Why   string  `json:"why"`
}

type thresholds struct {
	Version  string               `json:"version"`
	Metrics  map[string]threshold `json:"metrics"`
	Versions []struct {
		Version string               `json:"version"`
		Harness string               `json:"harness"`
		Metrics map[string]threshold `json:"metrics"`
	} `json:"versions"`
}

// merge adds the metrics of the extending versions this harness judges
// ("harness": "sela-eval"); other versions belong to other harnesses.
func (th *thresholds) merge() {
	for _, v := range th.Versions {
		if v.Harness != "sela-eval" {
			continue
		}
		for k, m := range v.Metrics {
			if _, dup := th.Metrics[k]; !dup {
				th.Metrics[k] = m
			}
		}
		th.Version += "+" + v.Version
	}
}

// Metric is one measured value with its verdict.
type Metric struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Threshold string  `json:"threshold,omitempty"`
	Status    string  `json:"status"` // pass | fail | not_measured | info
	Detail    string  `json:"detail,omitempty"`
	N         int     `json:"n,omitempty"`
}

// Report is the evaluation output.
type Report struct {
	Started    time.Time         `json:"started"`
	Finished   time.Time         `json:"finished"`
	Mode       string            `json:"mode"`
	Thresholds string            `json:"thresholds"`
	Provider   map[string]string `json:"provider"`
	Versions   map[string]string `json:"versions"`
	Metrics    []Metric          `json:"metrics"`
	Notes      []string          `json:"notes"`
	Failures   []string          `json:"failures"`
	measured   map[string]Metric
	only       string // judge only metrics with this prefix ("" = all)
}

func (r *Report) put(name string, value float64, n int, detail string) {
	if r.measured == nil {
		r.measured = map[string]Metric{}
	}
	r.measured[name] = Metric{Name: name, Value: value, N: n, Detail: detail}
}

func (r *Report) note(format string, args ...any) {
	r.Notes = append(r.Notes, fmt.Sprintf(format, args...))
}

func (r *Report) judge(th thresholds) {
	names := map[string]bool{}
	for k := range th.Metrics {
		names[k] = true
	}
	for k := range r.measured {
		names[k] = true
	}
	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		m, measured := r.measured[k]
		t, hasT := th.Metrics[k]
		if r.only != "" && !strings.HasPrefix(k, r.only) {
			continue
		}
		if !measured {
			m = Metric{Name: k, Status: "not_measured"}
			if hasT {
				m.Threshold = fmt.Sprintf("%s %g", t.Op, t.Value)
				switch {
				case strings.HasPrefix(k, "moderation.live.") && r.Mode != "moderation_live":
					m.Detail = "requires -mode moderation_live (live Meta moderation)"
				case strings.HasPrefix(k, "moderation.live.") && r.Mode == "moderation_live":
					m.Detail = "split not run in this invocation (see -splits)"
				case strings.HasPrefix(k, "moderation.rules.") && r.Mode == "moderation_live":
					m.Detail = "measured by -mode moderation or offline"
				case t.Mode == "live" && r.Mode != "live" && r.Mode != "mie-live":
					m.Detail = "requires live Meta inference (blocked: see docs/status/BLOCKERS.md B-1)"
				}
			}
			r.Metrics = append(r.Metrics, m)
			continue
		}
		if !hasT {
			m.Status = "info"
			r.Metrics = append(r.Metrics, m)
			continue
		}
		if t.Op == "report" {
			// Registered without a pass bar.
			m.Threshold, m.Status = "report", "info"
			r.Metrics = append(r.Metrics, m)
			continue
		}
		m.Threshold = fmt.Sprintf("%s %g", t.Op, t.Value)
		ok := (t.Op == ">=" && m.Value >= t.Value) || (t.Op == "<=" && m.Value <= t.Value)
		if ok {
			m.Status = "pass"
		} else {
			m.Status = "fail"
			r.Failures = append(r.Failures, fmt.Sprintf("%s = %g (want %s %g)", k, m.Value, t.Op, t.Value))
		}
		r.Metrics = append(r.Metrics, m)
	}
}

func main() {
	mode := flag.String("mode", "offline", "offline | test_double | live | moderation | moderation_live")
	root := flag.String("root", ".", "repository root")
	out := flag.String("out", "eval/results", "output directory")
	splits := flag.String("splits", "", "moderation modes: comma-separated splits (default dev,tune,heldout for moderation, dev,tune for moderation_live)")
	maxCalls := flag.Int("max-calls", 800, "moderation_live: hard cap on provider HTTP requests including retries (at most 800)")
	var lo mieLiveOpts
	flag.StringVar(&lo.action, "mie-live", "", "live MIE benchmark: run | sheet | labels | score (replaces -mode)")
	flag.StringVar(&lo.raw, "mie-raw", "", "mie-live: raw directory outside the repository (holds model text)")
	flag.StringVar(&lo.envFile, "mie-env", "", "mie-live run: env file with META_LLAMA_API_KEY or SELA_LLM_API_KEY")
	flag.StringVar(&lo.baseURL, "mie-base-url", mieLiveBaseURL, "mie-live run: Meta Model API base URL")
	flag.StringVar(&lo.callsFile, "mie-calls", "", "mie-live: shared provider-call ledger (default: next to -mie-raw)")
	flag.StringVar(&lo.splits, "mie-splits", "dev,tune,heldout", "mie-live run: splits to run")
	flag.IntVar(&lo.maxCalls, "mie-max-calls", 1200, "mie-live run: hard cap on provider HTTP attempts across all runs")
	flag.IntVar(&lo.concurrency, "mie-concurrency", 3, "mie-live run: concurrent provider calls")
	flag.IntVar(&lo.limit, "mie-limit", 0, "mie-live run: at most this many cases (smoke runs)")
	flag.Parse()
	if lo.action != "" {
		lo.root, lo.out = *root, *out
		if err := runMIELive(lo); err != nil {
			fmt.Fprintln(os.Stderr, "sela-eval:", err)
			os.Exit(1)
		}
		return
	}
	switch *mode {
	case "offline", "test_double", "live", "moderation", "moderation_live":
	default:
		fmt.Fprintln(os.Stderr, "sela-eval: -mode must be offline, test_double, live, moderation or moderation_live")
		os.Exit(2)
	}
	if *maxCalls < 1 || *maxCalls > 800 {
		fmt.Fprintln(os.Stderr, "sela-eval: -max-calls must be between 1 and 800")
		os.Exit(2)
	}
	raw, err := os.ReadFile(filepath.Join(*root, "eval", "thresholds.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "sela-eval:", err)
		os.Exit(2)
	}
	var th thresholds
	if err := json.Unmarshal(raw, &th); err != nil {
		fmt.Fprintln(os.Stderr, "sela-eval: thresholds:", err)
		os.Exit(2)
	}
	th.merge()
	rep := &Report{Started: time.Now().UTC(), Mode: *mode, Thresholds: th.Version, Provider: map[string]string{}, Versions: versions()}
	dir := *out
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(*root, dir)
	}
	if strings.HasPrefix(*mode, "moderation") {
		opt := moderationOptions{live: *mode == "moderation_live", maxCalls: *maxCalls, out: dir, splits: []string{"dev", "tune", "heldout"}}
		if opt.live {
			opt.splits = []string{"dev", "tune"}
		}
		if *splits != "" {
			opt.splits = strings.Split(*splits, ",")
		}
		rep.only = "moderation."
		if err := runModeration(rep, *root, opt); err != nil {
			rep.Failures = append(rep.Failures, "moderation: "+err.Error())
		}
		finish(rep, th, dir)
		return
	}
	steps := []struct {
		name string
		fn   func(*Report, string) error
	}{
		{"safety", runSafety},
		{"qualifiers", runQualifiers},
		{"retrieval", runRetrieval},
		{"authorization", runAuthorization},
		{"crisis_path", runCrisisPath},
		{"mie", runMIE},
		{"moderation", func(r *Report, root string) error {
			return runModeration(r, root, moderationOptions{splits: []string{"dev", "tune", "heldout"}, out: dir})
		}},
	}
	for _, s := range steps {
		if err := s.fn(rep, *root); err != nil {
			rep.Failures = append(rep.Failures, s.name+": "+err.Error())
		}
	}
	if *mode != "offline" {
		if err := runModel(rep, *root, *mode); err != nil {
			rep.Failures = append(rep.Failures, "model: "+err.Error())
		}
	}
	finish(rep, th, dir)
}

// finish judges, writes the report and exits non-zero on any failure.
func finish(rep *Report, th thresholds, dir string) {
	rep.judge(th)
	rep.Finished = time.Now().UTC()
	if err := write(rep, dir); err != nil {
		fmt.Fprintln(os.Stderr, "sela-eval:", err)
		os.Exit(2)
	}
	for _, m := range rep.Metrics {
		fmt.Printf("%-13s %-48s %10.4g  %s %s\n", m.Status, m.Name, m.Value, m.Threshold, m.Detail)
	}
	if len(rep.Failures) > 0 {
		fmt.Println("\nFAILURES:")
		for _, f := range rep.Failures {
			fmt.Println(" -", f)
		}
		os.Exit(1)
	}
}

func write(rep *Report, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	stamp := rep.Started.Format("20060102T150405Z")
	raw, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%s.json", stamp, rep.Mode)), append(raw, '\n'), 0o644); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Sela evaluation — %s mode\n\nStarted %s · thresholds %s\n\n| Metric | Value | Threshold | Status | Detail |\n| --- | --- | --- | --- | --- |\n",
		rep.Mode, rep.Started.Format(time.RFC3339), rep.Thresholds)
	for _, m := range rep.Metrics {
		fmt.Fprintf(&b, "| %s | %.4g | %s | %s | %s |\n", m.Name, m.Value, m.Threshold, m.Status, strings.ReplaceAll(m.Detail, "|", "/"))
	}
	if len(rep.Notes) > 0 {
		b.WriteString("\n## Notes\n\n")
		for _, n := range rep.Notes {
			fmt.Fprintf(&b, "- %s\n", n)
		}
	}
	if len(rep.Failures) > 0 {
		b.WriteString("\n## Failures\n\n")
		for _, f := range rep.Failures {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	return os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%s.md", stamp, rep.Mode)), []byte(b.String()), 0o644)
}
