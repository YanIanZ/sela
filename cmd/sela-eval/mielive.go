package main

// Live Meta ablation of the Meaning Integrity Engine (H-010..H-013,
// docs/hackathon/EVALUATION.md configs B0/B1/B2/T) on eval/mie/cases.jsonl.
//
//	sela-eval -mie-live run   -mie-raw DIR -mie-env FILE   live calls (resumable)
//	sela-eval -mie-live sheet -mie-raw DIR                 blind label sheet
//	sela-eval -mie-live labels -mie-raw DIR                blind labels -> eval/mie/live_labels.jsonl
//	sela-eval -mie-live score -mie-raw DIR                 metrics -> eval/results
//
// Stages of "run" (one generation per case, shared by every arm, so the
// arms differ only in post-processing):
//
//	gen1  AT-2 reflection of the case source alone (production prompt)
//	ver1  Meta verifier on each valid gen1 interpretation
//	vgold Meta verifier on the labelled case candidate (gold gating)
//	gen2  critical cases: source + the candidate as an earlier Sela reply
//	      that the scripted user rejected + the user's fixed correction
//	      message (no graph: B0/B1/B2 of phase p2)
//	ver2  Meta verifier on each valid gen2 interpretation
//
// T in phase p2 applies graph exclusion: the rejected interpretation is
// never context, so its request is byte-identical to gen1's (checked by
// request hash) and gen1/ver1 are reused.
//
// The raw directory holds model text of synthetic inputs and must stay
// outside the repository. Everything written under eval/ is metadata only:
// ids, counts, rates, hashes, latencies and tokens, never case or model
// text. The API key is read from the env file and only ever sent in the
// Authorization header.

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/ai/guard"
	"github.com/YanIanZ/sela/internal/ai/metaverifier"
	"github.com/YanIanZ/sela/internal/ai/minimize"
	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
	"github.com/YanIanZ/sela/internal/ledger"
	"github.com/YanIanZ/sela/internal/llm"
)

const (
	mieLiveHarness    = "sela-eval-mie-live"
	mieLiveGenerator  = "muse-spark-1.3"
	mieLiveVerifier   = metaverifier.DefaultModel
	mieLiveBaseURL    = "https://api.meta.ai/v1"
	mieLiveAudience   = verifier.AudienceAdult
	usdPerMInput      = 1.25 // ai-prices-2026-10-30, Meta Model API Standard mode
	usdPerMOutput     = 4.25
	mieLiveLabelsPath = "eval/mie/live_labels.jsonl"
)

// Scripted rejection message of phase p2 (the user's own words in the
// no-graph baseline). Fixed per language; never model-written.
var correctionText = map[string]string{
	"id": "Bukan itu maksudku. Yang kamu tulis sebelumnya tidak sesuai dengan yang aku katakan.",
	"en": "That's not what I meant. What you wrote before doesn't match what I said.",
}

type mieLiveOpts struct {
	action, raw, envFile, baseURL, root, out, callsFile, splits string
	maxCalls, concurrency, limit                                int
}

// liveRecord is one stage outcome of one case. Text and SourceRefs stay in
// the raw directory; metaRecord is what leaves it.
type liveRecord struct {
	Stage      string    `json:"stage"`
	CaseID     string    `json:"case_id"`
	RequestSHA string    `json:"request_sha256,omitempty"`
	Model      string    `json:"served_model,omitempty"`
	Status     string    `json:"status"`            // ok | failed | skipped
	Reason     string    `json:"reason,omitempty"`  // transport reason or parse outcome
	Outcome    string    `json:"outcome,omitempty"` // gen: valid|invalid|declined|crisis|failed; ver: release|withhold|failed|request_invalid
	Risk       string    `json:"risk,omitempty"`
	Text       string    `json:"text,omitempty"`
	TextSHA    string    `json:"text_sha256,omitempty"`
	CitesUser  bool      `json:"cites_user,omitempty"`
	B1Pass     *bool     `json:"b1_pass,omitempty"`
	B1Codes    []string  `json:"b1_codes,omitempty"`
	Verdict    string    `json:"verdict,omitempty"`
	Issues     []string  `json:"issues,omitempty"`
	Withhold   bool      `json:"withhold_all,omitempty"`
	TMatchesP1 *bool     `json:"t_request_matches_p1,omitempty"`
	LatencyMS  int64     `json:"latency_ms"`
	FirstMS    int64     `json:"first_content_ms,omitempty"`
	InTok      int       `json:"input_tokens"`
	OutTok     int       `json:"output_tokens"`
	Attempts   int       `json:"attempts"`
	HTTPStatus int       `json:"http_status,omitempty"`
	At         time.Time `json:"at"`
}

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func promptLang(c mieCase) string {
	if c.Lang == "en" {
		return "en"
	}
	return "id" // "mixed" is Indonesian with English code-switching
}

// readKey reads the Meta API key from an env file without a shell.
func readKey(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", errors.New("mie-live: cannot open the env file")
	}
	defer f.Close()
	vals := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		vals[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	for _, k := range []string{"META_LLAMA_API_KEY", "SELA_LLM_API_KEY", "SELA_META_VERIFIER_API_KEY"} {
		if vals[k] != "" {
			return vals[k], nil
		}
	}
	return "", errors.New("mie-live: no Meta API key in the env file")
}

// ---------------------------------------------------------------- store

type liveStore struct {
	mu   sync.Mutex
	path string
	recs map[string]liveRecord
}

func recKey(stage, caseID string) string { return stage + "/" + caseID }

func openStore(dir string) (*liveStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &liveStore{path: filepath.Join(dir, "records.jsonl"), recs: map[string]liveRecord{}}
	raw, err := os.ReadFile(s.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r liveRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("records.jsonl: %w", err)
		}
		s.recs[recKey(r.Stage, r.CaseID)] = r
	}
	return s, nil
}

func (s *liveStore) get(stage, caseID string) (liveRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.recs[recKey(stage, caseID)]
	return r, ok
}

func (s *liveStore) put(r liveRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := json.Marshal(r)
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(raw, '\n')); err != nil {
		return err
	}
	s.recs[recKey(r.Stage, r.CaseID)] = r
	return nil
}

// ---------------------------------------------------------------- budget

// callBudget enforces the hard provider-call cap across every run that
// shares the calls file (retries and smoke runs included).
type callBudget struct {
	mu       sync.Mutex
	path     string
	used     int
	reserved int
	max      int
}

func openBudget(path string, max int) (*callBudget, error) {
	b := &callBudget{path: path, max: max}
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		var e struct {
			Attempts int `json:"attempts"`
		}
		if json.Unmarshal([]byte(line), &e) == nil {
			b.used += e.Attempts
		}
	}
	return b, nil
}

var errBudget = errors.New("mie-live: provider call budget would be exceeded; stopping")

func (b *callBudget) reserve(n int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.used+b.reserved+n > b.max {
		return errBudget
	}
	b.reserved += n
	return nil
}

func (b *callBudget) settle(reserved, attempts int, stage, caseID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reserved -= reserved
	b.used += attempts
	raw, _ := json.Marshal(map[string]any{"stage": stage, "case_id": caseID, "attempts": attempts, "at": time.Now().UTC()})
	if f, err := os.OpenFile(b.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		_, _ = f.Write(append(raw, '\n'))
		f.Close()
	}
}

func (b *callBudget) total() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}

// ---------------------------------------------------------------- run

type liveRunner struct {
	gen    ai.Generator
	ver    *metaverifier.Client
	store  *liveStore
	budget *callBudget
	pol    ai.Policy
}

// genSpans builds the AT-2 payload of a stage.
func (lr *liveRunner) genSpans(c mieCase, stage string) (minimize.Payload, error) {
	sel := []minimize.Selection{{Key: "source", Role: minimize.RoleUser, Text: c.Source}}
	if stage == "gen2" {
		sel = append(sel,
			minimize.Selection{Key: "rejected", Role: minimize.RoleEarlierReply, Text: c.Candidate},
			minimize.Selection{Key: "correction", Role: minimize.RoleUser, Text: correctionText[promptLang(c)]})
	}
	return minimize.Build(sel, minimize.Limits{MaxSpans: lr.pol.MaxSpans, MaxSpanRunes: lr.pol.MaxSpanRunes,
		MaxTotalRunes: lr.pol.MaxTotalRunes, DropOldest: lr.pol.DropOldest, PerRole: lr.pol.RoleLimits}, minimize.NewSession(nil))
}

func requestHash(c mieCase, p minimize.Payload) (string, string, string, error) {
	system, user, err := ai.EvalPrompt(ai.TaskReflection, promptLang(c), mieLiveAudience, p.Spans)
	if err != nil {
		return "", "", "", err
	}
	return system, user, sha(system + "\x00" + user), nil
}

// b1 is the deterministic Go stage (config B1): the production output
// guards and the Meaning Diff against the user's source words.
func b1(source, text string) (bool, []string) {
	var codes []string
	g := guard.Check(text, guard.Options{})
	for _, b := range g.BlockedBy() {
		codes = append(codes, "guard:"+string(b))
	}
	m := ledger.CompareMeaning(source, text)
	for _, d := range m.Differences {
		if d.Severity == ledger.SeverityCritical {
			codes = append(codes, "meaning:"+d.Code)
		}
	}
	return !g.Blocked() && !m.Critical, codes
}

func (lr *liveRunner) runGen(ctx context.Context, c mieCase, stage string) error {
	if _, done := lr.store.get(stage, c.CaseID); done {
		return nil
	}
	payload, err := lr.genSpans(c, stage)
	if err != nil {
		return lr.store.put(liveRecord{Stage: stage, CaseID: c.CaseID, Status: "skipped", Outcome: "invalid", Reason: "payload: " + err.Error(), At: time.Now().UTC()})
	}
	system, user, h, err := requestHash(c, payload)
	if err != nil {
		return err
	}
	rec := liveRecord{Stage: stage, CaseID: c.CaseID, RequestSHA: h}
	if stage == "gen2" {
		tp, err := lr.genSpans(c, "gen1")
		if err != nil {
			return err
		}
		_, _, th, err := requestHash(c, tp)
		if err != nil {
			return err
		}
		g1, ok := lr.store.get("gen1", c.CaseID)
		match := ok && g1.RequestSHA == th
		rec.TMatchesP1 = &match
	}
	reserve := 1 + lr.pol.MetaMaxRetries
	if err := lr.budget.reserve(reserve); err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, lr.pol.Budget)
	res, gerr := lr.gen.Generate(cctx, ai.GenerateRequest{Task: ai.TaskReflection, System: system, User: user,
		Schema: ai.GenerationSchemaJSON, MaxTokens: lr.pol.MaxOutputTokens, MaxRetries: lr.pol.MetaMaxRetries,
		MaxRetryAfter: lr.pol.MaxRetryAfter})
	cancel()
	attempts := max(res.Attempts, 1)
	lr.budget.settle(reserve, attempts, stage, c.CaseID)
	rec.Model, rec.LatencyMS, rec.FirstMS = res.ServedModel, res.Latency.Milliseconds(), res.FirstContent.Milliseconds()
	rec.InTok, rec.OutTok, rec.Attempts, rec.At = res.InputTokens, res.OutputTokens, attempts, time.Now().UTC()
	if gerr != nil {
		rec.Status, rec.Outcome = "failed", "failed"
		var ge *ai.GenerateError
		if errors.As(gerr, &ge) {
			rec.Reason, rec.HTTPStatus = string(ge.Class)+"/"+string(ge.Reason), ge.Status
		} else {
			rec.Reason = "error"
		}
		return lr.store.put(rec)
	}
	rec.Status = "ok"
	d, perr := ai.EvalParse(ai.TaskReflection, res.Text, payload)
	switch {
	case perr != nil:
		rec.Outcome, rec.Reason = "invalid", "parse"
		if strings.Contains(strings.ToLower(res.Text), "\"declined\"") {
			rec.Reason = "parse_schema"
		}
	case d.Declined:
		rec.Outcome, rec.Risk = "declined", string(d.Risk)
	case d.Risk == ai.RiskCrisis:
		rec.Outcome, rec.Risk = "crisis", string(d.Risk)
	default:
		rec.Outcome, rec.Risk = "valid", string(d.Risk)
		it := d.Items[0]
		rec.Text, rec.TextSHA = it.Text, sha(it.Text)
		for _, id := range it.SpanIDs {
			for _, s := range payload.Spans {
				if s.ID == id && s.Role == minimize.RoleUser {
					rec.CitesUser = true
				}
			}
		}
		pass, codes := b1(c.Source, it.Text)
		rec.B1Pass, rec.B1Codes = &pass, codes
	}
	return lr.store.put(rec)
}

// runVer verifies one claim against the user-authored spans of the stage.
func (lr *liveRunner) runVer(ctx context.Context, c mieCase, stage string) error {
	if _, done := lr.store.get(stage, c.CaseID); done {
		return nil
	}
	var claim string
	var payload minimize.Payload
	var err error
	switch stage {
	case "vgold":
		claim = c.Candidate
		payload, err = lr.genSpans(c, "gen1")
	case "ver1", "ver2":
		src := map[string]string{"ver1": "gen1", "ver2": "gen2"}[stage]
		g, ok := lr.store.get(src, c.CaseID)
		if !ok || g.Outcome != "valid" {
			return nil // nothing to verify; the generation outcome is the result
		}
		claim = g.Text
		payload, err = lr.genSpans(c, src)
	}
	if err != nil {
		return err
	}
	req := verifier.Request{Task: string(ai.TaskReflection), Language: promptLang(c), Audience: mieLiveAudience}
	var cited []string
	for _, s := range payload.Spans {
		if s.Role == minimize.RoleUser {
			req.Spans = append(req.Spans, verifier.Span{ID: s.ID, Text: s.Text})
			cited = append(cited, s.ID)
		}
	}
	req.Claims = []verifier.Claim{{ID: "c1", Text: claim, SpanIDs: cited}}
	user, _ := verifier.UserMessage(req)
	rec := liveRecord{Stage: stage, CaseID: c.CaseID, RequestSHA: sha(verifier.Instructions + "\x00" + user), TextSHA: sha(claim)}
	if req.Validate() != nil {
		rec.Status, rec.Outcome, rec.Reason, rec.At = "skipped", "request_invalid", "VC-1", time.Now().UTC()
		return lr.store.put(rec)
	}
	reserve := 1 + transport.DefaultMaxRetries
	if err := lr.budget.reserve(reserve); err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, lr.pol.Budget)
	res, verr := lr.ver.Verify(cctx, req, verifier.CallOptions{AttemptTimeout: lr.pol.VerifierAttempt, MaxRetryAfter: lr.pol.MaxRetryAfter})
	cancel()
	attempts := max(res.Attempts, 1)
	lr.budget.settle(reserve, attempts, stage, c.CaseID)
	rec.Model, rec.LatencyMS, rec.InTok, rec.OutTok = res.ServedModel, res.Latency.Milliseconds(), res.InputTokens, res.OutputTokens
	rec.Attempts, rec.HTTPStatus, rec.At = attempts, res.Status, time.Now().UTC()
	if verr != nil {
		rec.Status, rec.Outcome, rec.Reason = "failed", "failed", string(transport.ReasonOf(verr))
		return lr.store.put(rec)
	}
	rec.Status = "ok"
	dec := verifier.Decide(req, res.Report, verifier.ReleaseOptions{WithholdOnAnyNotReleased: true, HasQuote: func(string) bool { return true }})
	if len(res.Report.Claims) > 0 {
		rec.Verdict = string(res.Report.Claims[0].Verdict)
	}
	for _, is := range res.Report.Issues {
		rec.Issues = append(rec.Issues, string(is.Category))
	}
	rec.Withhold = dec.WithholdAll
	rec.Outcome = "withhold"
	if dec.Released() == 1 && !dec.WithholdAll {
		rec.Outcome = "release"
	}
	return lr.store.put(rec)
}

func selectCases(cases []mieCase, splits string, limit int) []mieCase {
	want := map[string]bool{}
	for _, s := range strings.Split(splits, ",") {
		want[strings.TrimSpace(s)] = true
	}
	var out []mieCase
	for _, c := range cases {
		if want[c.Split] {
			out = append(out, c)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func runMIELiveCalls(o mieLiveOpts) error {
	cases, sum, err := loadMIECases(o.root)
	if err != nil {
		return err
	}
	if sum != mieDatasetSHA {
		return fmt.Errorf("mie-live: dataset sha256 %s differs from the registered %s", sum, mieDatasetSHA)
	}
	if o.raw == "" || strings.HasPrefix(mustAbs(o.raw), mustAbs(o.root)+string(os.PathSeparator)) {
		return errors.New("mie-live: -mie-raw must be a directory outside the repository (it holds model text)")
	}
	key, err := readKey(o.envFile)
	if err != nil {
		return err
	}
	pol, _ := ai.PolicyFor(ai.TaskReflection)
	gen, err := ai.NewMetaGenerator(llm.Config{Mode: llm.ModeMeta, BaseURL: o.baseURL, APIKey: key, ServedModel: mieLiveGenerator,
		ModelID: mieLiveGenerator, Revision: mieLiveGenerator, ReasoningEffort: "minimal"}, llm.NewAdmission(o.concurrency))
	if err != nil {
		return err
	}
	ver, err := metaverifier.New(metaverifier.Config{APIKey: transport.NewSecret(key), BaseURL: o.baseURL, Model: mieLiveVerifier})
	if err != nil {
		return err
	}
	store, err := openStore(o.raw)
	if err != nil {
		return err
	}
	callsFile := o.callsFile
	if callsFile == "" {
		callsFile = filepath.Join(filepath.Dir(mustAbs(o.raw)), "provider_calls.jsonl")
	}
	budget, err := openBudget(callsFile, o.maxCalls)
	if err != nil {
		return err
	}
	lr := &liveRunner{gen: gen, ver: ver, store: store, budget: budget, pol: pol}
	sel := selectCases(cases, o.splits, o.limit)
	fmt.Printf("mie-live: %d cases, provider calls used so far %d of %d\n", len(sel), budget.total(), o.maxCalls)
	ctx := context.Background()
	stages := []struct {
		name string
		gen  bool
		only func(mieCase) bool
	}{
		{"gen1", true, nil},
		{"ver1", false, nil},
		{"vgold", false, nil},
		{"gen2", true, func(c mieCase) bool { return c.ExpectedCritical }},
		{"ver2", false, func(c mieCase) bool { return c.ExpectedCritical }},
	}
	for _, st := range stages {
		var wg sync.WaitGroup
		jobs := make(chan mieCase)
		var mu sync.Mutex
		var firstErr error
		for w := 0; w < o.concurrency; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for c := range jobs {
					var err error
					if st.gen {
						err = lr.runGen(ctx, c, st.name)
					} else {
						err = lr.runVer(ctx, c, st.name)
					}
					if err != nil {
						mu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						mu.Unlock()
					}
				}
			}()
		}
		for _, c := range sel {
			mu.Lock()
			stop := firstErr != nil
			mu.Unlock()
			if stop {
				break
			}
			if st.only == nil || st.only(c) {
				jobs <- c
			}
		}
		close(jobs)
		wg.Wait()
		fmt.Printf("mie-live: stage %s done, provider calls used %d\n", st.name, budget.total())
		if firstErr != nil {
			return firstErr
		}
	}
	return nil
}

func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if r, err := filepath.EvalSymlinks(a); err == nil {
		return r
	}
	return a
}

// ---------------------------------------------------------------- labels

// The blind label sheet shows an opaque item id, the source, the output
// and, for critical cases, the rejected earlier interpretation. It never
// shows the case id, split, stage (arm) or any check result, and its order
// is shuffled with a fixed seed.
type sheetItem struct {
	Item     string `json:"item"`
	Lang     string `json:"lang"`
	Source   string `json:"source"`
	Output   string `json:"output"`
	Rejected string `json:"rejected_interpretation,omitempty"`
}

type sheetMap struct {
	Stage   string `json:"stage"`
	CaseID  string `json:"case_id"`
	TextSHA string `json:"text_sha256"`
}

// liveLabel is one label as committed (no text).
type liveLabel struct {
	Stage    string `json:"stage"`
	CaseID   string `json:"case_id"`
	TextSHA  string `json:"text_sha256"`
	Label    string `json:"label"`          // faithful | unsupported
	Reason   string `json:"reason"`         // rubric reject reason, or "none"
	Leak     *bool  `json:"leak,omitempty"` // critical cases: reasserts the rejected meaning
	Labeller string `json:"labeller"`
}

func writeSheet(o mieLiveOpts) error {
	cases, _, err := loadMIECases(o.root)
	if err != nil {
		return err
	}
	store, err := openStore(o.raw)
	if err != nil {
		return err
	}
	byID := map[string]mieCase{}
	for _, c := range cases {
		byID[c.CaseID] = c
	}
	var items []sheetItem
	m := map[string]sheetMap{}
	for _, r := range store.recs {
		if (r.Stage != "gen1" && r.Stage != "gen2") || r.Outcome != "valid" {
			continue
		}
		c := byID[r.CaseID]
		id := sha(r.Stage + "/" + r.CaseID + "/" + r.TextSHA)[:10]
		it := sheetItem{Item: id, Lang: c.Lang, Source: c.Source, Output: r.Text}
		if c.ExpectedCritical {
			it.Rejected = c.Candidate
		}
		items = append(items, it)
		m[id] = sheetMap{Stage: r.Stage, CaseID: r.CaseID, TextSHA: r.TextSHA}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Item < items[j].Item })
	rand.New(rand.NewSource(20261102)).Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	f, err := os.OpenFile(filepath.Join(o.raw, "label_sheet.jsonl"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, it := range items {
		raw, _ := json.Marshal(it)
		_, _ = f.Write(append(raw, '\n'))
	}
	raw, _ := json.MarshalIndent(m, "", " ")
	if err := os.WriteFile(filepath.Join(o.raw, "label_map.json"), raw, 0o600); err != nil {
		return err
	}
	fmt.Printf("mie-live: %d items in the blind label sheet\n", len(items))
	return nil
}

// runMIELive dispatches -mie-live.
func runMIELive(o mieLiveOpts) error {
	if o.raw == "" {
		return errors.New("mie-live: -mie-raw is required")
	}
	if o.concurrency < 1 || o.concurrency > 8 {
		return errors.New("mie-live: -mie-concurrency must be within [1, 8]")
	}
	switch o.action {
	case "run":
		return runMIELiveCalls(o)
	case "sheet":
		return writeSheet(o)
	case "labels":
		return convertLabels(o)
	case "score":
		if o.callsFile == "" {
			o.callsFile = filepath.Join(filepath.Dir(mustAbs(o.raw)), "provider_calls.jsonl")
		}
		return scoreMIELive(o)
	}
	return errors.New("mie-live: action must be run, sheet, labels or score")
}

// convertLabels turns the blind labels (raw/labels_blind.jsonl: item,
// label, reason, leak) into the committed metadata-only labels file.
func convertLabels(o mieLiveOpts) error {
	rawMap, err := os.ReadFile(filepath.Join(o.raw, "label_map.json"))
	if err != nil {
		return err
	}
	m := map[string]sheetMap{}
	if err := json.Unmarshal(rawMap, &m); err != nil {
		return err
	}
	blind, err := os.ReadFile(filepath.Join(o.raw, "labels_blind.jsonl"))
	if err != nil {
		return err
	}
	var out []liveLabel
	seen := map[string]bool{}
	for i, line := range strings.Split(string(blind), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var b struct {
			Item   string `json:"item"`
			Label  string `json:"label"`
			Reason string `json:"reason"`
			Leak   *bool  `json:"leak"`
		}
		if err := json.Unmarshal([]byte(line), &b); err != nil {
			return fmt.Errorf("labels_blind.jsonl line %d: %w", i+1, err)
		}
		sm, ok := m[b.Item]
		if !ok || seen[b.Item] {
			return fmt.Errorf("labels_blind.jsonl line %d: unknown or repeated item", i+1)
		}
		if b.Label != "faithful" && b.Label != "unsupported" {
			return fmt.Errorf("labels_blind.jsonl line %d: label must be faithful or unsupported", i+1)
		}
		seen[b.Item] = true
		out = append(out, liveLabel{Stage: sm.Stage, CaseID: sm.CaseID, TextSHA: sm.TextSHA, Label: b.Label, Reason: b.Reason, Leak: b.Leak,
			Labeller: "ai-agent-claude-opus-5.5 (single labeller, blind to arm and checks; not human)"})
	}
	if len(seen) != len(m) {
		return fmt.Errorf("labels cover %d of %d sheet items", len(seen), len(m))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Stage != out[j].Stage {
			return out[i].Stage < out[j].Stage
		}
		return out[i].CaseID < out[j].CaseID
	})
	var b strings.Builder
	for _, l := range out {
		raw, _ := json.Marshal(l)
		b.Write(raw)
		b.WriteByte('\n')
	}
	return os.WriteFile(filepath.Join(o.root, mieLiveLabelsPath), []byte(b.String()), 0o644)
}
