package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/ai/guard"
	"github.com/YanIanZ/sela/internal/ai/verifier"
	"github.com/YanIanZ/sela/internal/ledger"
)

// mieDatasetSHA is the registered eval/mie/cases.jsonl hash.
const mieDatasetSHA = "90bf7057e46027c18461077d0bff314306fb10c4877582c0bb637f7c7fac6564"

// Arms of the ablation. T differs from B2 only in phase p2 (graph
// exclusion of the rejected interpretation); in phase p1 T's machine stage
// is B2 by definition and its human review is not scored.
var mieArms = []string{"B0", "B1", "B2"}

type liveCase struct {
	c                  mieCase
	g1, v1, vg, g2, v2 *liveRecord
	l1, l2             *liveLabel
	goldB1             bool // candidate passes the Go stage
}

func valid(g *liveRecord) bool { return g != nil && g.Outcome == "valid" }

// released reports whether an arm releases a generation before user review.
func released(arm string, g, v *liveRecord) bool {
	if !valid(g) {
		return false
	}
	b1 := g.B1Pass != nil && *g.B1Pass
	switch arm {
	case "B0":
		return true
	case "B1":
		return b1
	default: // B2, T
		return b1 && v != nil && v.Outcome == "release"
	}
}

func unsupported(l *liveLabel) bool { return l != nil && l.Label == "unsupported" }
func leaked(l *liveLabel) bool      { return l != nil && l.Leak != nil && *l.Leak }

// p1Arm returns the generation, verification and label an arm uses in a
// phase; T in p2 uses the p1 generation (graph exclusion).
func (lc *liveCase) phase(phase, arm string) (*liveRecord, *liveRecord, *liveLabel) {
	if phase == "p2" && arm != "T" {
		return lc.g2, lc.v2, lc.l2
	}
	return lc.g1, lc.v1, lc.l1
}

// familyBoot is a percentile bootstrap (resampling paraphrase families) of
// the mean of per-case values d.
func familyBoot(cs []*liveCase, d func(*liveCase) float64) (float64, float64) {
	fam := map[string][]float64{}
	var keys []string
	for _, lc := range cs {
		if _, ok := fam[lc.c.FamilyID]; !ok {
			keys = append(keys, lc.c.FamilyID)
		}
		fam[lc.c.FamilyID] = append(fam[lc.c.FamilyID], d(lc))
	}
	if len(keys) == 0 {
		return 0, 0
	}
	sort.Strings(keys)
	r := rand.New(rand.NewSource(20261102))
	const iters = 4000
	means := make([]float64, iters)
	for i := range means {
		sum, n := 0.0, 0
		for range keys {
			for _, x := range fam[keys[r.Intn(len(keys))]] {
				sum += x
				n++
			}
		}
		means[i] = sum / float64(n)
	}
	sort.Float64s(means)
	return means[int(0.025*iters)], means[int(0.975*iters)]
}

func b2i(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// scoreGroup puts the phase metrics of one group of cases.
func scoreGroup(rep *Report, prefix string, cs []*liveCase, full bool) {
	n := len(cs)
	if n == 0 {
		return
	}
	// Phase p1: first-pass interpretation.
	nValid, faithfulValid, unsupValid := 0, 0, 0
	outcomes := map[string]int{}
	for _, lc := range cs {
		o := "missing"
		if lc.g1 != nil {
			o = lc.g1.Outcome
		}
		outcomes[o]++
		if valid(lc.g1) {
			nValid++
			if unsupported(lc.l1) {
				unsupValid++
			} else {
				faithfulValid++
			}
		}
	}
	if full {
		rep.put(prefix+".p1.generation_invalid_or_abstained", ratio(n-nValid, n), n, interval(n-nValid, n)+"; outcomes "+fmtCounts(outcomes))
		rep.put(prefix+".p1.generated_unsupported_rate", ratio(unsupValid, nValid), nValid, interval(unsupValid, nValid)+" (label of every valid generation, before any gate)")
	}
	perCase := map[string]float64{}
	for _, arm := range mieArms {
		rel, uns, withheldFaithful, blockedUnsup := 0, 0, 0, 0
		for _, lc := range cs {
			g, v, l := lc.phase("p1", arm)
			r := released(arm, g, v)
			if r {
				rel++
				if unsupported(l) {
					uns++
				}
			} else if valid(g) {
				if unsupported(l) {
					blockedUnsup++
				} else {
					withheldFaithful++
				}
			}
		}
		perCase[arm] = ratio(uns, n)
		rep.put(fmt.Sprintf("%s.p1.%s.uir", prefix, arm), ratio(uns, rel), rel, interval(uns, rel)+" unsupported / released")
		if full {
			rep.put(fmt.Sprintf("%s.p1.%s.unsupported_released_per_case", prefix, arm), ratio(uns, n), n, interval(uns, n))
			rep.put(fmt.Sprintf("%s.p1.%s.released_rate", prefix, arm), ratio(rel, n), n, interval(rel, n))
		}
		if arm != "B0" {
			rep.put(fmt.Sprintf("%s.p1.%s.unnecessary_abstention", prefix, arm), ratio(withheldFaithful, faithfulValid), faithfulValid, interval(withheldFaithful, faithfulValid)+" faithful valid outputs withheld")
			if full {
				rep.put(fmt.Sprintf("%s.p1.%s.unsupported_blocked_recall", prefix, arm), ratio(blockedUnsup, unsupValid), unsupValid, interval(blockedUnsup, unsupValid))
			}
		}
	}
	lo, hi := familyBoot(cs, func(lc *liveCase) float64 {
		g, v, l := lc.phase("p1", "B0")
		b0 := released("B0", g, v) && unsupported(l)
		g, v, l = lc.phase("p1", "B2")
		return b2i(b0) - b2i(released("B2", g, v) && unsupported(l))
	})
	rep.put(prefix+".p1.uir_benefit_b2_vs_b0", perCase["B0"]-perCase["B2"], n, fmt.Sprintf("unsupported released per case B0 %.3f - B2 %.3f; family bootstrap 95%% CI %.3f..%.3f", perCase["B0"], perCase["B2"], lo, hi))

	// Phase p2: after the scripted rejection (critical cases only).
	var crit []*liveCase
	for _, lc := range cs {
		if lc.c.ExpectedCritical {
			crit = append(crit, lc)
		}
	}
	if len(crit) > 0 {
		leakRate := map[string]float64{}
		for _, arm := range []string{"B0", "B1", "B2", "T"} {
			rel, uns, leak := 0, 0, 0
			for _, lc := range crit {
				g, v, l := lc.phase("p2", arm)
				if released(arm, g, v) {
					rel++
					if unsupported(l) {
						uns++
					}
					if leaked(l) {
						leak++
					}
				}
			}
			leakRate[arm] = ratio(leak, len(crit))
			rep.put(fmt.Sprintf("%s.p2.%s.correction_leakage", prefix, arm), ratio(leak, len(crit)), len(crit), interval(leak, len(crit))+" released outputs reasserting the rejected meaning / rejected cases")
			if full {
				rep.put(fmt.Sprintf("%s.p2.%s.uir", prefix, arm), ratio(uns, rel), rel, interval(uns, rel))
				rep.put(fmt.Sprintf("%s.p2.%s.released_rate", prefix, arm), ratio(rel, len(crit)), len(crit), interval(rel, len(crit)))
			}
		}
		if full {
			genLeak, nv := 0, 0
			for _, lc := range crit {
				if valid(lc.g2) {
					nv++
					if leaked(lc.l2) {
						genLeak++
					}
				}
			}
			rep.put(prefix+".p2.no_graph_generated_leak_rate", ratio(genLeak, nv), nv, interval(genLeak, nv)+" valid no-graph generations reasserting the rejected meaning, before any gate")
		}
		lo, hi := familyBoot(crit, func(lc *liveCase) float64 {
			g, v, l := lc.phase("p2", "B0")
			b0 := released("B0", g, v) && leaked(l)
			g, v, l = lc.phase("p2", "T")
			return b2i(b0) - b2i(released("T", g, v) && leaked(l))
		})
		rep.put(prefix+".p2.leakage_benefit_t_vs_b0", leakRate["B0"]-leakRate["T"], len(crit), fmt.Sprintf("family bootstrap 95%% CI %.3f..%.3f", lo, hi))
	}

	// Gold gating: the labelled candidate as if generated.
	nc, nb := 0, 0
	far := map[string]int{}
	abst := map[string]int{}
	for _, lc := range cs {
		verOK := lc.vg != nil && lc.vg.Outcome == "release"
		rel := map[string]bool{"B0": true, "B1": lc.goldB1, "B2": lc.goldB1 && verOK, "verifier_only": verOK}
		for arm, r := range rel {
			if lc.c.ExpectedCritical && r {
				far[arm]++
			}
			if !lc.c.ExpectedCritical && !r {
				abst[arm]++
			}
		}
		if lc.c.ExpectedCritical {
			nc++
		} else {
			nb++
		}
	}
	for _, arm := range []string{"B0", "B1", "B2", "verifier_only"} {
		if nc > 0 {
			rep.put(fmt.Sprintf("%s.gold.%s.false_acceptance_critical", prefix, arm), ratio(far[arm], nc), nc, interval(far[arm], nc))
		}
		if nb > 0 {
			rep.put(fmt.Sprintf("%s.gold.%s.benign_unnecessary_abstention", prefix, arm), ratio(abst[arm], nb), nb, interval(abst[arm], nb))
		}
	}
}

func fmtCounts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, " ")
}

func usd(in, out int) float64 { return (float64(in)*usdPerMInput + float64(out)*usdPerMOutput) / 1e6 }

func loadLabels(root string) (map[string]*liveLabel, error) {
	raw, err := os.ReadFile(filepath.Join(root, mieLiveLabelsPath))
	if err != nil {
		return nil, err
	}
	out := map[string]*liveLabel{}
	for i, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var l liveLabel
		if err := json.Unmarshal([]byte(line), &l); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", mieLiveLabelsPath, i+1, err)
		}
		out[recKey(l.Stage, l.CaseID)] = &l
	}
	return out, nil
}

func liveThresholds(root string) (thresholds, string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "eval", "thresholds.json"))
	if err != nil {
		return thresholds{}, "", err
	}
	var all thresholds
	if err := json.Unmarshal(raw, &all); err != nil {
		return thresholds{}, "", err
	}
	th := thresholds{Metrics: map[string]threshold{}}
	for _, v := range all.Versions {
		if v.Harness == mieLiveHarness {
			th.Version = v.Version
			for k, m := range v.Metrics {
				th.Metrics[k] = m
			}
		}
	}
	if th.Version == "" {
		return th, "", errors.New("mie-live: no registered thresholds for harness " + mieLiveHarness)
	}
	return th, sha(string(raw)), nil
}

func scoreMIELive(o mieLiveOpts) error {
	cases, sum, err := loadMIECases(o.root)
	if err != nil {
		return err
	}
	if sum != mieDatasetSHA {
		return fmt.Errorf("mie-live: dataset sha256 %s differs from the registered %s", sum, mieDatasetSHA)
	}
	th, thSHA, err := liveThresholds(o.root)
	if err != nil {
		return err
	}
	store, err := openStore(o.raw)
	if err != nil {
		return err
	}
	labels, err := loadLabels(o.root)
	if err != nil {
		return err
	}
	rep := &Report{Started: time.Now().UTC(), Mode: "mie-live", Thresholds: th.Version, Provider: map[string]string{}, Versions: map[string]string{
		"meaning_diff": ledger.MeaningVersion, "guards": guard.Version, "prompt": ai.PromptVersion, "generation_schema": ai.GenerationSchemaVersion,
		"verifier_schema": verifier.SchemaVersion, "verifier_instructions": verifier.InstructionsVersion, "mie_cases_sha256": sum,
		"thresholds_sha256": thSHA, "price_table": ai.PriceTableVersion,
	}}
	get := func(stage, id string) *liveRecord {
		if r, ok := store.get(stage, id); ok {
			return &r
		}
		return nil
	}
	var all []*liveCase
	missingLabels := 0
	tMismatch := 0
	for _, c := range cases {
		lc := &liveCase{c: c, g1: get("gen1", c.CaseID), v1: get("ver1", c.CaseID), vg: get("vgold", c.CaseID), g2: get("gen2", c.CaseID), v2: get("ver2", c.CaseID)}
		if lc.g1 == nil || lc.vg == nil || (c.ExpectedCritical && lc.g2 == nil) {
			continue // not run (smoke or partial store)
		}
		lc.goldB1, _ = b1(c.Source, c.Candidate)
		for _, x := range []struct {
			g *liveRecord
			l **liveLabel
		}{{lc.g1, &lc.l1}, {lc.g2, &lc.l2}} {
			if !valid(x.g) {
				continue
			}
			l, ok := labels[recKey(x.g.Stage, c.CaseID)]
			if !ok || l.TextSHA != x.g.TextSHA {
				missingLabels++
				continue
			}
			*x.l = l
		}
		if lc.g2 != nil && (lc.g2.TMatchesP1 == nil || !*lc.g2.TMatchesP1) {
			tMismatch++
		}
		all = append(all, lc)
	}
	if missingLabels > 0 {
		return fmt.Errorf("mie-live: %d valid generations have no matching label (text hash)", missingLabels)
	}
	if tMismatch > 0 {
		return fmt.Errorf("mie-live: %d T requests differ from the p1 request", tMismatch)
	}
	if len(all) == 0 {
		return errors.New("mie-live: no complete cases in the raw store")
	}
	groups := map[string][]*liveCase{}
	for _, lc := range all {
		groups[lc.c.Split] = append(groups[lc.c.Split], lc)
		groups["all"] = append(groups["all"], lc)
		groups[lc.c.Split+".lang."+lc.c.Lang] = append(groups[lc.c.Split+".lang."+lc.c.Lang], lc)
		groups["all.lang."+lc.c.Lang] = append(groups["all.lang."+lc.c.Lang], lc)
	}
	for _, g := range []string{"dev", "tune", "heldout", "all"} {
		scoreGroup(rep, "mie_live."+g, groups[g], true)
		for _, lang := range []string{"id", "en", "mixed"} {
			if g == "heldout" || g == "all" {
				scoreGroup(rep, "mie_live."+g+".lang."+lang, groups[g+".lang."+lang], false)
			}
		}
	}

	// Operational metrics over every record of the run.
	models := map[string]map[string]int{}
	var genLat, genFirst, verLat, b0Lat, b2Lat []float64
	genReq, genValid, verAll, ver200, verOK, verRefused, verFailed := 0, 0, 0, 0, 0, 0, 0
	genIn, genOut, verIn, verOut, attempts := 0, 0, 0, 0, 0
	genOutcomes, verFailures := map[string]int{}, map[string]int{}
	var b0Cost, b2Cost float64
	b0Rel, b2Rel := 0, 0
	for _, lc := range all {
		for _, r := range []*liveRecord{lc.g1, lc.v1, lc.vg, lc.g2, lc.v2} {
			if r == nil {
				continue
			}
			attempts += r.Attempts
			if r.Model != "" {
				if models[r.Stage] == nil {
					models[r.Stage] = map[string]int{}
				}
				models[r.Stage][r.Model]++
			}
			switch r.Stage {
			case "gen1", "gen2":
				genReq++
				genIn += r.InTok
				genOut += r.OutTok
				genOutcomes[r.Outcome+":"+r.Reason]++
				if r.Outcome == "valid" {
					genValid++
				}
				if r.Status == "ok" {
					genLat = append(genLat, float64(r.LatencyMS))
					genFirst = append(genFirst, float64(r.FirstMS))
				}
			default:
				if r.Status == "skipped" {
					verFailures["request_invalid"]++
					continue
				}
				verAll++
				verIn += r.InTok
				verOut += r.OutTok
				if r.HTTPStatus == 200 {
					ver200++
				}
				switch r.Status {
				case "ok":
					verOK++
					verLat = append(verLat, float64(r.LatencyMS))
				case "failed":
					verFailed++
					verFailures[r.Reason]++
					if r.Reason == "refused" {
						verRefused++
					}
				}
			}
		}
		// Phase p1 end-to-end latency and cost per arm.
		if lc.g1 != nil {
			gl := float64(lc.g1.LatencyMS)
			gc := usd(lc.g1.InTok, lc.g1.OutTok)
			b0Lat = append(b0Lat, gl)
			b0Cost += gc
			if released("B0", lc.g1, lc.v1) {
				b0Rel++
			}
			e2e, c2 := gl, gc
			if valid(lc.g1) && lc.g1.B1Pass != nil && *lc.g1.B1Pass && lc.v1 != nil {
				e2e += float64(lc.v1.LatencyMS)
				c2 += usd(lc.v1.InTok, lc.v1.OutTok)
			}
			b2Lat = append(b2Lat, e2e)
			b2Cost += c2
			if released("B2", lc.g1, lc.v1) {
				b2Rel++
			}
		}
	}
	for stage, m := range models {
		var parts []string
		for k, v := range m {
			parts = append(parts, fmt.Sprintf("%s x%d", k, v))
		}
		sort.Strings(parts)
		rep.Provider["served_model."+stage] = strings.Join(parts, ", ")
	}
	rep.Provider["generator_configured"] = mieLiveGenerator
	rep.Provider["verifier_configured"] = mieLiveVerifier
	rep.Provider["endpoint"] = "api.meta.ai"
	rep.put("mie_live.all.generator.valid_output_rate", ratio(genValid, genReq), genReq, interval(genValid, genReq)+"; outcomes "+fmtCounts(genOutcomes))
	rep.put("mie_live.all.generator.p50_ms", percentile(genLat, 0.5), len(genLat), "completed generation calls, retries included; run concurrency "+fmt.Sprint(o.concurrency))
	rep.put("mie_live.all.generator.p95_ms", percentile(genLat, 0.95), len(genLat), "")
	rep.put("mie_live.all.generator.first_content_p50_ms", percentile(genFirst, 0.5), len(genFirst), "")
	rep.put("mie_live.all.verifier.contract_valid_rate", ratio(verOK, ver200), ver200, interval(verOK, ver200)+" HTTP 200 responses passing VC-6")
	rep.put("mie_live.all.verifier.failure_rate", ratio(verFailed, verAll), verAll, interval(verFailed, verAll)+" (fail closed: withheld); reasons "+fmtCounts(verFailures))
	rep.put("mie_live.all.verifier.refusal_rate", ratio(verRefused, verAll), verAll, interval(verRefused, verAll))
	rep.put("mie_live.all.verifier.p50_ms", percentile(verLat, 0.5), len(verLat), "completed verifier calls")
	rep.put("mie_live.all.verifier.p95_ms", percentile(verLat, 0.95), len(verLat), "")
	rep.put("mie_live.all.p1.B0.e2e_p50_ms", percentile(b0Lat, 0.5), len(b0Lat), "generation only (B1 adds < 1 ms of Go checks)")
	rep.put("mie_live.all.p1.B0.e2e_p95_ms", percentile(b0Lat, 0.95), len(b0Lat), "")
	rep.put("mie_live.all.p1.B2.e2e_p50_ms", percentile(b2Lat, 0.5), len(b2Lat), "generation + verifier when the Go stage passes (sequential)")
	rep.put("mie_live.all.p1.B2.e2e_p95_ms", percentile(b2Lat, 0.95), len(b2Lat), "")
	rep.put("mie_live.all.tokens.generator_input", float64(genIn), genReq, "")
	rep.put("mie_live.all.tokens.generator_output", float64(genOut), genReq, "includes hidden reasoning tokens if the provider counts them")
	rep.put("mie_live.all.tokens.verifier_input", float64(verIn), verAll, "")
	rep.put("mie_live.all.tokens.verifier_output", float64(verOut), verAll, "")
	rep.put("mie_live.all.verifier.cost_per_call_usd_mean", usd(verIn, verOut)/float64(max(verAll, 1)), verAll, "estimate: measured tokens x "+ai.PriceTableVersion)
	n1 := len(b0Lat)
	rep.put("mie_live.all.p1.B0.cost_per_case_usd", b0Cost/float64(max(n1, 1)), n1, fmt.Sprintf("per released interpretation %.5f", b0Cost/float64(max(b0Rel, 1))))
	rep.put("mie_live.all.p1.B2.cost_per_case_usd", b2Cost/float64(max(n1, 1)), n1, fmt.Sprintf("generator + verifier when the Go stage passes; per released interpretation %.5f", b2Cost/float64(max(b2Rel, 1))))
	rep.put("mie_live.all.cost.run_total_usd", usd(genIn+verIn, genOut+verOut), attempts, "every scored generation and verifier call of this store")
	rep.put("mie_live.all.provider_attempts_scored", float64(attempts), len(all), "HTTP attempts behind the scored records (smoke runs and the global ledger are reported in notes)")
	if o.callsFile != "" {
		if b, err := openBudget(o.callsFile, 0); err == nil {
			rep.note("provider-call ledger %s: %d HTTP attempts in total across every run of this benchmark (cap %d)", filepath.Base(o.callsFile), b.used, 1200)
		}
	}
	rep.note("%d cases scored (%d critical with phase p2); arms share one generation per case; T(p1) = B2 machine stage, T(p2) reuses the p1 request (hash-checked for every case)", len(all), len(groups["all"])-countBenign(all))
	rep.note("labels: %s, single AI-agent labeller blind to arm, split and gate results; not human, no kappa; owner adjudication pending", mieLiveLabelsPath)
	rep.note("not measured: readability, verifiable quote precision (AT-2 cards are paraphrases, not quotes), stale disclosure and consent cases (deterministic suites, not live), human task completion")
	rep.judge(th)
	rep.Finished = time.Now().UTC()
	dir := o.out
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(o.root, dir)
	}
	if err := write(rep, dir); err != nil {
		return err
	}
	// Metadata-only per-record file (no case or model text).
	var b strings.Builder
	for _, lc := range all {
		for _, r := range []*liveRecord{lc.g1, lc.v1, lc.vg, lc.g2, lc.v2} {
			if r == nil {
				continue
			}
			m := *r
			m.Text = ""
			raw, _ := json.Marshal(struct {
				liveRecord
				Split string `json:"split"`
				Lang  string `json:"lang"`
			}{m, lc.c.Split, lc.c.Lang})
			b.Write(raw)
			b.WriteByte('\n')
		}
	}
	stamp := rep.Started.Format("20060102T150405Z")
	if err := os.WriteFile(filepath.Join(dir, stamp+"-mie-live-records.jsonl"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	for _, m := range rep.Metrics {
		if m.Status != "info" || strings.HasPrefix(m.Name, "mie_live.heldout.p") || strings.HasPrefix(m.Name, "mie_live.all.p") {
			fmt.Printf("%-13s %-58s %10.4g  %s %s\n", m.Status, m.Name, m.Value, m.Threshold, m.Detail)
		}
	}
	if len(rep.Failures) > 0 {
		fmt.Println("\nFAILED preregistered targets:")
		for _, f := range rep.Failures {
			fmt.Println(" -", f)
		}
	}
	return nil
}

func countBenign(cs []*liveCase) int {
	n := 0
	for _, lc := range cs {
		if !lc.c.ExpectedCritical {
			n++
		}
	}
	return n
}
