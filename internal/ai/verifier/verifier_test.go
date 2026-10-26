package verifier

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func request() Request {
	return Request{
		Task: "summary_draft", Language: "id", Audience: AudienceAdult,
		Spans: []Span{{ID: "s1", Text: "Aku belum pernah cerita ke siapa pun soal ini."}, {ID: "s2", Text: "Aku susah tidur sejak dua bulan."}},
		Claims: []Claim{
			{ID: "c1", Text: "Kamu belum pernah bercerita kepada siapa pun.", SpanIDs: []string{"s1"}},
			{ID: "c2", Text: "Kamu susah tidur sejak dua bulan.", SpanIDs: []string{"s2"}},
		},
	}
}

const validReport = `{"claims":[
 {"claim_id":"c1","verdict":"supported","span_ids":["s1"],"reason":"Sama dengan kutipan."},
 {"claim_id":"c2","verdict":"unsupported","span_ids":["s2"],"reason":"Menambah durasi."}],
 "issues":[{"category":"lost_context","claim_id":"c2","span_ids":["s2"],"reason":"Kualifikasi hilang."}]}`

// TestVC1RequestLimits covers the VC-1 input limits.
func TestVC1RequestLimits(t *testing.T) {
	if err := request().Validate(); err != nil {
		t.Fatal(err)
	}
	mut := []func(*Request){
		func(r *Request) { r.Task = "" },
		func(r *Request) { r.Language = "fr" },
		func(r *Request) { r.Audience = "17" },
		func(r *Request) { r.Claims = nil },
		func(r *Request) { r.Spans = nil },
		func(r *Request) {
			for i := 0; i < MaxClaims; i++ {
				r.Claims = append(r.Claims, Claim{ID: "x" + string(rune('a'+i)), Text: "t", SpanIDs: []string{"s1"}})
			}
		},
		func(r *Request) {
			for i := 0; i < MaxSpans; i++ {
				r.Spans = append(r.Spans, Span{ID: "y" + string(rune('a'+i)), Text: "t"})
			}
		},
		func(r *Request) { r.Spans[1].ID = "s1" },
		func(r *Request) { r.Spans[0].Text = strings.Repeat("a", MaxSpanBytes+1) },
		func(r *Request) { r.Claims[0].Text = strings.Repeat("a", MaxClaimBytes+1) },
		func(r *Request) { r.Claims[0].SpanIDs = []string{"s9"} },
		func(r *Request) { r.Claims[0].SpanIDs = []string{"s1", "s1"} },
		func(r *Request) { r.Claims[0].SpanIDs = []string{"s1", "s2", "s1", "s2", "s1", "s2"} },
		func(r *Request) { r.Claims[0].SpanIDs = nil },
		func(r *Request) { r.Claims[1].ID = "c1" },
		func(r *Request) { r.Claims[0].ID = "user-123/db" },
		func(r *Request) { r.Claims[0].ID = "s1" },
	}
	for i, m := range mut {
		r := request()
		m(&r)
		if err := r.Validate(); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("mutation %d accepted: %v", i, err)
		}
	}
}

// TestVC2Schema checks the output schema's shape: strict objects, the four
// verdicts, the seven categories, "reason" and never "reasoning".
func TestVC2Schema(t *testing.T) {
	var s map[string]any
	if err := json.Unmarshal(Schema(), &s); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(SchemaJSON, "reasoning") || strings.Contains(SchemaJSON, "minLength") || strings.Contains(SchemaJSON, "anyOf") {
		t.Fatal("schema uses a forbidden construct")
	}
	if strings.Count(SchemaJSON, `"additionalProperties":false`) != 3 {
		t.Fatal("every object must forbid additional properties")
	}
	for _, v := range Verdicts() {
		if !strings.Contains(SchemaJSON, `"`+string(v)+`"`) {
			t.Fatalf("schema lacks verdict %s", v)
		}
	}
	for _, c := range Categories() {
		if !strings.Contains(SchemaJSON, `"`+string(c)+`"`) {
			t.Fatalf("schema lacks category %s", c)
		}
	}
}

// TestVC7Instructions: no chain-of-thought is requested, data is marked
// as data, and the DeepSeek variant names json and carries the schema.
func TestVC7Instructions(t *testing.T) {
	low := strings.ToLower(Instructions)
	for _, bad := range []string{"step by step", "think step", "chain of thought", "show your reasoning", "explain your reasoning"} {
		if strings.Contains(low, bad) {
			t.Fatalf("instructions ask for reasoning: %q", bad)
		}
	}
	if !strings.Contains(low, "data") || !strings.Contains(Instructions, InputOpenMarker) {
		t.Fatal("instructions do not mark the input as data")
	}
	ds := DeepSeekInstructions()
	if !strings.Contains(ds, "json") || !strings.Contains(ds, SchemaJSON) {
		t.Fatal("DeepSeek instructions lack json or the schema")
	}
	msg, err := UserMessage(request())
	if err != nil || !strings.Contains(msg, InputOpenMarker) || !strings.HasSuffix(msg, InputCloseMarker) {
		t.Fatalf("user message %q %v", msg, err)
	}
}

func TestVC6ParseValid(t *testing.T) {
	rep, err := Parse(validReport, request())
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Claims) != 2 || rep.Claims[0].Verdict != Supported || rep.Claims[1].Verdict != Unsupported {
		t.Fatalf("claims %+v", rep.Claims)
	}
	if len(rep.Issues) != 1 || rep.Issues[0].Category != LostContext {
		t.Fatalf("issues %+v", rep.Issues)
	}
	if rep.Counts()[Unsupported] != 1 {
		t.Fatal("counts")
	}
}

// TestVC6CaseInsensitiveEnums: enum casing is not guaranteed by the
// provider, so it is compared case-insensitively and normalized.
func TestVC6CaseInsensitiveEnums(t *testing.T) {
	text := strings.NewReplacer(`"supported"`, `"SUPPORTED"`, `"unsupported"`, `"Unsupported"`, `"lost_context"`, `"Lost_Context"`).Replace(validReport)
	rep, err := Parse(text, request())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Claims[0].Verdict != Supported || rep.Claims[1].Verdict != Unsupported || rep.Issues[0].Category != LostContext {
		t.Fatalf("not normalized: %+v", rep)
	}
}

// TestVC6Rejections: any violation rejects the whole report.
func TestVC6Rejections(t *testing.T) {
	cases := map[string]string{
		"not json":           `nope`,
		"trailing":           validReport + ` {}`,
		"unknown field":      strings.Replace(validReport, `"issues":[`, `"approved":true,"issues":[`, 1),
		"claim extra field":  strings.Replace(validReport, `"reason":"Sama dengan kutipan."`, `"reason":"x","confidence":0.9`, 1),
		"missing issues":     `{"claims":[{"claim_id":"c1","verdict":"supported","span_ids":["s1"],"reason":"a"},{"claim_id":"c2","verdict":"supported","span_ids":["s2"],"reason":"a"}]}`,
		"missing claim":      `{"claims":[{"claim_id":"c1","verdict":"supported","span_ids":["s1"],"reason":"a"}],"issues":[]}`,
		"extra claim":        `{"claims":[{"claim_id":"c1","verdict":"supported","span_ids":["s1"],"reason":"a"},{"claim_id":"c2","verdict":"supported","span_ids":["s2"],"reason":"a"},{"claim_id":"c3","verdict":"supported","span_ids":["s2"],"reason":"a"}],"issues":[]}`,
		"duplicate claim":    `{"claims":[{"claim_id":"c1","verdict":"supported","span_ids":["s1"],"reason":"a"},{"claim_id":"c1","verdict":"supported","span_ids":["s1"],"reason":"a"}],"issues":[]}`,
		"unknown claim":      `{"claims":[{"claim_id":"c1","verdict":"supported","span_ids":["s1"],"reason":"a"},{"claim_id":"c9","verdict":"supported","span_ids":["s2"],"reason":"a"}],"issues":[]}`,
		"verdict enum":       strings.Replace(validReport, `"unsupported"`, `"partially_supported"`, 1),
		"verdict spaces":     strings.Replace(validReport, `"unsupported"`, `" unsupported"`, 1),
		"category enum":      strings.Replace(validReport, `"lost_context"`, `"tone"`, 1),
		"unknown span":       strings.Replace(validReport, `"span_ids":["s2"],"reason":"Menambah`, `"span_ids":["s7"],"reason":"Menambah`, 1),
		"repeated span":      strings.Replace(validReport, `"span_ids":["s1"]`, `"span_ids":["s1","s1"]`, 1),
		"supported no own":   strings.Replace(validReport, `"span_ids":["s1"],"reason":"Sama`, `"span_ids":["s2"],"reason":"Sama`, 1),
		"supported no spans": strings.Replace(validReport, `"span_ids":["s1"],"reason":"Sama`, `"span_ids":[],"reason":"Sama`, 1),
		"empty reason":       strings.Replace(validReport, `"Sama dengan kutipan."`, `"  "`, 1),
		"control reason":     strings.Replace(validReport, `"Sama dengan kutipan."`, `"\u0007​"`, 1),
		"long reason":        strings.Replace(validReport, `"Sama dengan kutipan."`, `"`+strings.Repeat("x", MaxReasonRunes+1)+`"`, 1),
		"null verdict":       strings.Replace(validReport, `"verdict":"supported"`, `"verdict":null`, 1),
		"null span_ids":      strings.Replace(validReport, `"span_ids":["s1"]`, `"span_ids":null`, 1),
		"issue bad claim":    strings.Replace(validReport, `"claim_id":"c2","span_ids":["s2"],"reason":"Kualifikasi`, `"claim_id":"c7","span_ids":["s2"],"reason":"Kualifikasi`, 1),
		"too large":          `{"claims":[],"issues":[],"x":"` + strings.Repeat("a", MaxResponseBytes) + `"}`,
		"array":              `[]`,
	}
	for name, text := range cases {
		if _, err := Parse(text, request()); !errors.Is(err, ErrInvalidOutput) {
			t.Errorf("%s: accepted (%v)", name, err)
		}
	}
}

func TestVC6ReasonCleaning(t *testing.T) {
	text := strings.Replace(validReport, `"Sama dengan kutipan."`, `"Sama\ndengan\tkutipan.\u0000 <b>x</b>"`, 1)
	rep, err := Parse(text, request())
	if err != nil {
		t.Fatal(err)
	}
	if got := rep.Claims[0].Reason; got != "Sama dengan kutipan. <b>x</b>" {
		t.Fatalf("reason %q", got)
	}
	// Issue with claim_id "" concerns the whole output.
	text = strings.Replace(validReport, `"claim_id":"c2","span_ids":["s2"],"reason":"Kualifikasi`, `"claim_id":"","span_ids":[],"reason":"Kualifikasi`, 1)
	if rep, err := Parse(text, request()); err != nil || rep.Issues[0].ClaimID != "" {
		t.Fatalf("output-level issue: %v", err)
	}
}

func report(verdicts ...Verdict) Report {
	var rep Report
	for i, v := range verdicts {
		id := "c" + string(rune('1'+i))
		rep.Claims = append(rep.Claims, ClaimVerdict{ClaimID: id, Verdict: v, SpanIDs: []string{"s" + string(rune('1'+i))}, Reason: "r"})
	}
	return rep
}

// TestVC8Release covers every release rule.
func TestVC8Release(t *testing.T) {
	req := request()
	quote := func(string) bool { return true }
	t.Run("supported released, unsupported falls back to quote", func(t *testing.T) {
		d := Decide(req, report(Supported, Unsupported), ReleaseOptions{HasQuote: quote})
		if d.WithholdAll || d.Claims[0].Disposition != Release || d.Claims[1].Disposition != QuoteFallback ||
			d.Claims[1].Explanation != "verdict.unsupported" || d.Released() != 1 {
			t.Fatalf("%+v", d)
		}
	})
	t.Run("no quote drops", func(t *testing.T) {
		d := Decide(req, report(Contradicted, InsufficientEvidence), ReleaseOptions{})
		if d.Claims[0].Disposition != Drop || d.Claims[1].Disposition != Drop || d.Claims[1].Explanation != "verdict.insufficient_evidence" {
			t.Fatalf("%+v", d)
		}
	})
	// A claim-scoped issue of a per-claim category (lost_context,
	// citation_invalid) withholds that claim only.
	for _, c := range []Category{LostContext, CitationInvalid} {
		t.Run("supported with claim issue "+string(c)+" is not released", func(t *testing.T) {
			rep := report(Supported, Supported)
			rep.Issues = []Issue{{Category: c, ClaimID: "c1"}}
			d := Decide(req, rep, ReleaseOptions{HasQuote: quote})
			if d.Claims[0].Disposition != QuoteFallback || d.Claims[0].Explanation != "issue."+string(c) || d.Claims[1].Disposition != Release ||
				d.WithholdAll || len(d.Withheld) != 0 || d.Released() != 1 {
				t.Fatalf("%+v", d)
			}
		})
	}
	// VC-8: medical_language, manipulation, age_inappropriate and
	// information_leak withhold the whole output whatever claim they are
	// attached to; the claim keeps the issue in its record.
	for _, c := range []Category{MedicalLanguage, Manipulation, AgeInappropriate, InformationLeak} {
		t.Run("claim-scoped "+string(c)+" withholds the whole output", func(t *testing.T) {
			rep := report(Supported, Supported)
			rep.Issues = []Issue{{Category: c, ClaimID: "c2", SpanIDs: []string{"s2"}}}
			d := Decide(req, rep, ReleaseOptions{HasQuote: quote})
			if !d.WithholdAll || d.SafetySignal || d.Released() != 0 || len(d.Withheld) != 1 || d.Withheld[0] != c ||
				len(d.Claims[1].Issues) != 1 || d.Claims[1].Issues[0] != c || !OutputLevel(c) {
				t.Fatalf("%+v", d)
			}
		})
	}
	for _, c := range []Category{LostContext, CitationInvalid, SafetySignal} {
		if OutputLevel(c) {
			t.Errorf("%s is not an output-level category", c)
		}
	}
	for _, c := range []Category{MedicalLanguage, Manipulation, AgeInappropriate, InformationLeak, LostContext, CitationInvalid} {
		t.Run("output-level "+string(c)+" withholds", func(t *testing.T) {
			rep := report(Supported, Supported)
			rep.Issues = []Issue{{Category: c, ClaimID: ""}}
			d := Decide(req, rep, ReleaseOptions{})
			if !d.WithholdAll || d.SafetySignal || d.Released() != 0 || d.Withheld[0] != c {
				t.Fatalf("%+v", d)
			}
		})
	}
	t.Run("safety signal raises and withholds", func(t *testing.T) {
		rep := report(Supported, Supported)
		rep.Issues = []Issue{{Category: SafetySignal, ClaimID: "c2"}}
		d := Decide(req, rep, ReleaseOptions{})
		if !d.SafetySignal || !d.WithholdAll {
			t.Fatalf("%+v", d)
		}
	})
	t.Run("AT-1 any non-supported withholds the reply", func(t *testing.T) {
		d := Decide(req, report(Supported, InsufficientEvidence), ReleaseOptions{WithholdOnAnyNotReleased: true})
		if !d.WithholdAll {
			t.Fatalf("%+v", d)
		}
		if d := Decide(req, report(Supported, Supported), ReleaseOptions{WithholdOnAnyNotReleased: true}); d.WithholdAll || d.Released() != 2 {
			t.Fatalf("%+v", d)
		}
	})
}

// TestChecksCoverCK1toCK9 maps every verdict and category to the nine
// checks.
func TestChecksCoverCK1toCK9(t *testing.T) {
	rep := report(Supported, Unsupported)
	rep.Claims = append(rep.Claims, ClaimVerdict{ClaimID: "c3", Verdict: Contradicted})
	for _, c := range Categories() {
		rep.Issues = append(rep.Issues, Issue{Category: c})
	}
	got := rep.Checks()
	if len(got) != 9 {
		t.Fatalf("checks %v", got)
	}
	for i, ck := range Checks() {
		if got[i] != ck {
			t.Fatalf("checks %v", got)
		}
	}
	if CheckOfCategory("x") != "" || ChecksOfVerdict("x") != nil {
		t.Fatal("unknown values map to checks")
	}
}

// TestVC3VerdictDefinitions: the instructions carry the four VC-3
// definitions, and "unsupported" is a positive finding of an addition
// while "insufficient_evidence" means the verifier cannot tell.
func TestVC3VerdictDefinitions(t *testing.T) {
	for _, want := range []string{
		`"supported": every important detail follows from the cited spans`,
		`"unsupported": the claim adds a detail, cause, interpretation, diagnosis, number or time that no supplied span states`,
		`"contradicted": a supplied span conflicts with the claim`,
		`"insufficient_evidence": the spans are too short, ambiguous or off-topic to decide`,
	} {
		if !strings.Contains(Instructions, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
}

// TestVC9ExplanationCodes: the user sees Sela's own fixed explanation per
// verdict or issue (a code the UI maps to reviewed text), never the
// verifier's reason as advice.
func TestVC9ExplanationCodes(t *testing.T) {
	if ExplainSupported != "verified.supported" || ExplainVerdict(Contradicted) != "verdict.contradicted" ||
		ExplainIssue(MedicalLanguage) != "issue.medical_language" {
		t.Fatal("explanation codes changed")
	}
	d := Decide(request(), report(Unsupported, Supported), ReleaseOptions{})
	if d.Claims[0].Explanation != "verdict.unsupported" || d.Claims[0].Reason != "r" {
		t.Fatalf("%+v", d.Claims[0])
	}
}

// TestVC6DuplicateAndCaseVariantKeys: encoding/json matches keys
// case-insensitively and lets the last duplicate win; Parse rejects both
// before decoding, so a report cannot say "contradicted" and "supported"
// for one claim.
func TestVC6DuplicateAndCaseVariantKeys(t *testing.T) {
	req := Request{Task: "summary_draft", Language: "id", Audience: AudienceAdult,
		Spans:  []Span{{ID: "s1", Text: "Aku belum pernah cerita ke siapa pun."}},
		Claims: []Claim{{ID: "c1", Text: "Kamu sudah cerita ke temanmu.", SpanIDs: []string{"s1"}}}}
	for name, text := range map[string]string{
		"case-variant verdict": `{"claims":[{"claim_id":"c1","verdict":"contradicted","span_ids":["s1"],"reason":"negation flip","Verdict":"supported"}],"issues":[]}`,
		"case-variant issues":  `{"claims":[{"claim_id":"c1","verdict":"contradicted","span_ids":["s1"],"reason":"negation flip"}],"issues":[],"ISSUES":[]}`,
		"duplicate verdict":    `{"claims":[{"claim_id":"c1","verdict":"contradicted","span_ids":["s1"],"reason":"x","verdict":"supported"}],"issues":[]}`,
		"duplicate issues":     `{"claims":[{"claim_id":"c1","verdict":"supported","span_ids":["s1"],"reason":"x"}],"issues":[{"category":"safety_signal","claim_id":"","span_ids":[],"reason":"r"}],"issues":[]}`,
		"capitalised field":    `{"Claims":[{"claim_id":"c1","verdict":"supported","span_ids":["s1"],"reason":"x"}],"issues":[]}`,
		"duplicate in issue":   `{"claims":[{"claim_id":"c1","verdict":"supported","span_ids":["s1"],"reason":"x"}],"issues":[{"category":"safety_signal","category":"lost_context","claim_id":"","span_ids":[],"reason":"r"}]}`,
	} {
		if _, err := Parse(text, req); !errors.Is(err, ErrInvalidOutput) {
			t.Errorf("%s: accepted (%v)", name, err)
		}
	}
	if err := CheckKeys(`{"a":[{"b":1},{"b":2}],"c":{"a":{"b":[]}}}`, map[string]bool{"a": true, "b": true, "c": true}); err != nil {
		t.Fatalf("repeated keys in sibling objects rejected: %v", err)
	}
	if err := CheckKeys(`{"a":1,"a":2}`, map[string]bool{"a": true}); !errors.Is(err, ErrKeys) {
		t.Fatalf("duplicate accepted: %v", err)
	}
	if err := CheckKeys(`{"a":`, map[string]bool{"a": true}); err == nil {
		t.Fatal("incomplete value accepted")
	}
}
