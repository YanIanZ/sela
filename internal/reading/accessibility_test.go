package reading

import (
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"reflect"
	"strings"
	"testing"

	readingdata "github.com/YanIanZ/sela/data/content/reading"
)

// a11yFixture returns a draft item whose content the test overrides.
func a11yFixture(id string, content map[string]any, kind string) map[string]any {
	it := unreviewedItem(id)
	if kind != "" {
		it["kind"] = kind
	}
	if content != nil {
		c := it["content"].(map[string]any)
		for k, v := range content {
			if v == nil {
				delete(c, k)
				continue
			}
			c[k] = v
		}
	}
	return it
}

func a11yReportFor(t *testing.T, it map[string]any) A11yReport {
	t.Helper()
	lib, err := LoadFS(fixtureFS(it))
	if err != nil {
		t.Fatal(err)
	}
	r, _, ok := lib.Accessibility(it["id"].(string))
	if !ok {
		t.Fatal("no report")
	}
	return r
}

func checkByID(r A11yReport, id string) A11yCheck {
	for _, c := range r.Checks {
		if c.ID == id {
			return c
		}
	}
	return A11yCheck{}
}

func loc(id, en string) map[string]string { return map[string]string{"id": id, "en": en} }

func TestA11yFixturePasses(t *testing.T) {
	t.Parallel()
	r := a11yReportFor(t, a11yFixture("clean", nil, ""))
	if r.Version != A11yChecksVersion || r.Result != A11yResultPass {
		t.Fatalf("clean fixture = %+v", r)
	}
	want := []string{"language_parity", "heading_structure", "list_structure", "link_text", "image_alt",
		"sensory_instructions", "sentence_length", "readability", "block_length"}
	var got []string
	for _, c := range r.Checks {
		got = append(got, c.ID)
		if c.Findings == nil {
			t.Fatalf("%s: findings must be an empty list, not null", c.ID)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("check order = %v, want %v", got, want)
	}
	if checkByID(r, "image_alt").Status != A11yNotApplicable {
		t.Fatalf("image_alt without images = %+v", checkByID(r, "image_alt"))
	}
}

func TestA11yBlockingRules(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("kata ", 16)
	cases := []struct {
		name, kind, check, code string
		content                 map[string]any
	}{
		{"sensory colour and position", "", "sensory_instructions", "sensory_only_instruction",
			map[string]any{"steps": map[string]any{"id": []string{"Tekan tombol hijau di kanan."}, "en": []string{"Press the green button on the right."}}}},
		{"colour named as the property", "", "sensory_instructions", "sensory_only_instruction",
			map[string]any{"summary": loc("Pilih kotak yang berwarna merah.", "Choose the option shown in red.")}},
		{"bare url", "", "link_text", "bare_url",
			map[string]any{"summary": loc("Buka https://example.org untuk info.", "Open www.example.org for info.")}},
		{"generic markdown link", "", "link_text", "link_text_not_descriptive",
			map[string]any{"summary": loc("Baca panduan [di sini](https://example.org).", "Read the guide [here](https://example.org).")}},
		{"generic phrase", "", "link_text", "generic_link_phrase",
			map[string]any{"summary": loc("Klik di sini untuk lanjut.", "Click here to continue.")}},
		{"image without alt", "", "image_alt", "image_without_alt",
			map[string]any{"summary": loc("Lihat ![](a.png) ini.", "See <img src=\"a.png\"> this.")}},
		{"typed step numbers", "", "list_structure", "typed_list_marker",
			map[string]any{"steps": map[string]any{"id": []string{"1. Tarik napas."}, "en": []string{"1. Breathe in."}}}},
		{"exercise without steps", "", "list_structure", "steps_missing", map[string]any{"steps": nil}},
		{"empty english", "", "language_parity", "empty_text", map[string]any{"summary": loc("Ringkasan.", " ")}},
		{"step counts differ", "", "language_parity", "step_count_differs",
			map[string]any{"steps": map[string]any{"id": []string{"Satu.", "Dua."}, "en": []string{"One."}}}},
		{"guide without sections", "guide", "heading_structure", "guide_without_sections", map[string]any{"steps": nil}},
		{"duplicate heading", "guide", "heading_structure", "heading_duplicate", map[string]any{"steps": nil, "sections": []any{
			map[string]any{"heading": loc("Bagian", "Part"), "paragraphs": []any{loc("Isi.", "Text.")}},
			map[string]any{"heading": loc("Bagian", "Part"), "paragraphs": []any{loc("Isi lagi.", "More text.")}},
		}}},
		{"heading repeats title", "guide", "heading_structure", "heading_repeats_title", map[string]any{"steps": nil, "sections": []any{
			map[string]any{"heading": loc("Judul repeat", "Title repeat"), "paragraphs": []any{loc("Isi.", "Text.")}},
		}}},
		{"empty section", "guide", "heading_structure", "section_without_text", map[string]any{"steps": nil, "sections": []any{
			map[string]any{"heading": loc("Bagian", "Part"), "paragraphs": []any{}},
		}}},
		{"heading too long", "guide", "heading_structure", "heading_too_long", map[string]any{"steps": nil, "sections": []any{
			map[string]any{"heading": loc(long, "Part"), "paragraphs": []any{loc("Isi.", "Text.")}},
		}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			content := c.content
			if c.check == "heading_structure" && c.code == "heading_repeats_title" {
				content["title"] = loc("Judul repeat", "Title repeat")
			}
			r := a11yReportFor(t, a11yFixture("x", content, c.kind))
			ch := checkByID(r, c.check)
			if ch.Status != A11yFail || !ch.Blocking || r.Result != A11yResultBlocked {
				t.Fatalf("%s = %+v (result %s)", c.check, ch, r.Result)
			}
			found := false
			for _, f := range ch.Findings {
				found = found || f.Code == c.code
			}
			if !found {
				t.Fatalf("%s findings = %+v, want %s", c.check, ch.Findings, c.code)
			}
		})
	}
}

func TestA11yRulesDoNotFlagNamedControlsOrNeutralWords(t *testing.T) {
	t.Parallel()
	r := a11yReportFor(t, a11yFixture("ok", map[string]any{
		"summary": loc(
			`Tombol "Butuh bantuan sekarang" selalu ada di bagian atas layar. Kamu bisa mengatur tema warna.`,
			`Open "Need help now" at the top. You can change the colour theme.`),
		"steps": map[string]any{
			"id": []string{"Sebutkan dalam hati: 'Ini rasa cemas'."},
			"en": []string{"Name it: 'Here is worry'."},
		},
	}, ""))
	if c := checkByID(r, "sensory_instructions"); c.Status != A11yPass {
		t.Fatalf("sensory = %+v", c)
	}
	if c := checkByID(r, "link_text"); c.Status != A11yPass {
		t.Fatalf("link_text = %+v (a quoted 'Here is' is not a link)", c)
	}
	withAlt := a11yReportFor(t, a11yFixture("alt", map[string]any{
		"summary": loc("Lihat ![Diagram napas empat hitungan](a.png).", `See <img alt="Four-count breathing diagram" src="a.png">.`),
	}, ""))
	if c := checkByID(withAlt, "image_alt"); c.Status != A11yPass {
		t.Fatalf("image with alt = %+v", c)
	}
}

func TestA11yReadabilityIsOverridable(t *testing.T) {
	t.Parallel()
	long := "Kalimat ini sengaja dibuat sangat panjang supaya jumlah katanya melewati batas dua puluh kata yang ditetapkan oleh panduan gaya konten Sela."
	r := a11yReportFor(t, a11yFixture("long", map[string]any{"summary": loc(long, "Short English text.")}, ""))
	c := checkByID(r, "sentence_length")
	if c.Status != A11yFail || c.Blocking || r.Result != A11yResultNeedsOverride {
		t.Fatalf("sentence_length = %+v, result %s", c, r.Result)
	}
	if c.Findings[0].Path != "content.summary.id" || c.Findings[0].Lang != "id" || c.Findings[0].Value <= 20 {
		t.Fatalf("finding = %+v", c.Findings[0])
	}
}

func TestA11yReportIsDeterministic(t *testing.T) {
	t.Parallel()
	lib := Embedded()
	for _, it := range lib.items {
		again := it.checkAccessibility(lib.sources)
		a, _ := json.Marshal(it.a11y)
		b, _ := json.Marshal(again)
		if string(a) != string(b) {
			t.Fatalf("%s: report changed between runs", it.ID)
		}
	}
}

// TestA11yEmbeddedSetAgreesWithAuthoringChecks pins the server metrics to
// sela-plain-language-v1 as stored by tests/content/reading_validator.py,
// and records that no embedded item currently fails a blocking check.
func TestA11yEmbeddedSetAgreesWithAuthoringChecks(t *testing.T) {
	t.Parallel()
	type plain struct {
		Metrics map[string]struct {
			Sentences            int      `json:"sentences"`
			Words                int      `json:"words"`
			Avg                  float64  `json:"avg_words_per_sentence"`
			Max                  int      `json:"max_words_per_sentence"`
			SentencesOver20Words int      `json:"sentences_over_20_words"`
			FK                   *float64 `json:"fk_grade"`
		} `json:"metrics"`
		Result string `json:"result"`
	}
	stored := map[string]plain{}
	var set setDoc
	if err := readJSON(readingdata.FS, "set.json", &set); err != nil {
		t.Fatal(err)
	}
	for _, f := range set.Files {
		raw, err := fs.ReadFile(readingdata.FS, f.Path)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Items []struct {
				ID     string `json:"id"`
				Checks struct {
					Plain plain `json:"plain_language"`
				} `json:"checks"`
			} `json:"items"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		for _, it := range doc.Items {
			stored[it.ID] = it.Checks.Plain
		}
	}
	lib := Embedded()
	if len(lib.items) != 38 {
		t.Fatalf("items = %d", len(lib.items))
	}
	for _, it := range lib.items {
		r := it.a11y
		if blocking, _ := r.Failing(); len(blocking) > 0 {
			t.Errorf("%s: blocking accessibility failures %v", it.ID, blocking)
		}
		want, ok := stored[it.ID]
		if !ok {
			t.Fatalf("%s: no stored plain-language check", it.ID)
		}
		for _, lang := range []string{"id", "en"} {
			got, w := r.Metrics[lang], want.Metrics[lang]
			if got.Sentences != w.Sentences || got.Words != w.Words || got.MaxWordsPerSentence != w.Max ||
				got.SentencesOver20Words != w.SentencesOver20Words || math.Abs(got.AvgWordsPerSentence-w.Avg) > 0.051 {
				t.Errorf("%s %s: metrics %+v, stored %+v", it.ID, lang, got, w)
			}
			if (got.FKGrade == nil) != (w.FK == nil) || got.FKGrade != nil && math.Abs(*got.FKGrade-*w.FK) > 0.051 {
				t.Errorf("%s %s: fk %v, stored %v", it.ID, lang, got.FKGrade, w.FK)
			}
		}
		_, overridable := r.Failing()
		readabilityFails := false
		for _, id := range overridable {
			readabilityFails = readabilityFails || id == "sentence_length" || id == "readability"
		}
		if readabilityFails != (want.Result == "needs_editor") {
			t.Errorf("%s: server readability failing=%t, authoring result %s", it.ID, readabilityFails, want.Result)
		}
	}
}

func fullChecklist() map[string]bool {
	m := map[string]bool{}
	for _, id := range ManualCheckIDs() {
		m[id] = true
	}
	return m
}

func gateRequest(reason string) *A11yRequest {
	return &A11yRequest{ChecksVersion: A11yChecksVersion, ManualVersion: A11yManualVersion, ManualChecks: fullChecklist(), OverrideReason: reason}
}

func TestGateAccessibility(t *testing.T) {
	t.Parallel()
	pass := a11yReportFor(t, a11yFixture("clean", nil, ""))
	needs := a11yReportFor(t, a11yFixture("long", map[string]any{"summary": loc(
		"Kalimat ini sengaja dibuat sangat panjang supaya jumlah katanya melewati batas dua puluh kata yang ditetapkan oleh panduan gaya konten Sela.",
		"Short.")}, ""))
	blocked := a11yReportFor(t, a11yFixture("blocked", map[string]any{"summary": loc("Tekan tombol merah.", "Press the red button.")}, ""))
	reason := "Kalimat bantuan darurat dibiarkan utuh agar nomor 119 tidak terpisah."

	cases := []struct {
		name   string
		report A11yReport
		req    *A11yRequest
		want   error
		field  string
	}{
		{"missing", pass, nil, ErrA11yChecklist, "accessibility"},
		{"old checks version", pass, func() *A11yRequest { r := gateRequest(""); r.ChecksVersion = "sela-reading-a11y-v0"; return r }(), ErrA11yVersion, "accessibility.checksVersion"},
		{"old manual version", pass, func() *A11yRequest { r := gateRequest(""); r.ManualVersion = "x"; return r }(), ErrA11yVersion, "accessibility.manualVersion"},
		{"blocked even with a reason", blocked, gateRequest(reason), ErrA11yBlocked, "accessibility.checks.sensory_instructions"},
		{"checklist item false", pass, func() *A11yRequest { r := gateRequest(""); r.ManualChecks["names_and_order"] = false; return r }(), ErrA11yChecklist, "accessibility.manualChecks.names_and_order"},
		{"checklist item missing", pass, func() *A11yRequest {
			r := gateRequest("")
			delete(r.ManualChecks, "read_aloud_plain_language")
			return r
		}(), ErrA11yChecklist, "accessibility.manualChecks.read_aloud_plain_language"},
		{"unknown checklist item", pass, func() *A11yRequest { r := gateRequest(""); r.ManualChecks["extra"] = true; return r }(), ErrA11yChecklist, "accessibility.manualChecks.extra"},
		{"override missing", needs, gateRequest(""), ErrA11yOverrideRequired, "accessibility.overrideReason"},
		{"override too short", needs, gateRequest("terlalu pendek"), ErrA11yOverrideRequired, "accessibility.overrideReason"},
		{"override too long", needs, gateRequest(strings.Repeat("a", 301)), ErrA11yOverrideRequired, "accessibility.overrideReason"},
		{"override without a failure", pass, gateRequest(reason), ErrA11yOverrideUnneeded, "accessibility.overrideReason"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := GateAccessibility(c.report, c.req)
			var ge *A11yGateError
			if !errors.Is(err, c.want) || !errors.As(err, &ge) || ge.Field != c.field {
				t.Fatalf("err = %v, want %v on %s", err, c.want, c.field)
			}
		})
	}

	rec, err := GateAccessibility(pass, gateRequest(""))
	if err != nil || rec.Result != A11yApprovedPass || rec.OverrideReason != "" || !rec.validRecord() {
		t.Fatalf("pass record = %+v, %v", rec, err)
	}
	rec, err = GateAccessibility(needs, gateRequest("  "+reason+"  "))
	if err != nil || rec.Result != A11yApprovedOverridden || rec.OverrideReason != reason ||
		!reflect.DeepEqual(rec.Overridden, []string{"sentence_length"}) || !rec.validRecord() {
		t.Fatalf("override record = %+v, %v", rec, err)
	}
}

func TestA11yRecordValidation(t *testing.T) {
	t.Parallel()
	pass := a11yReportFor(t, a11yFixture("clean", nil, ""))
	needs := a11yReportFor(t, a11yFixture("long", map[string]any{"summary": loc(
		"Kalimat ini sengaja dibuat sangat panjang supaya jumlah katanya melewati batas dua puluh kata yang ditetapkan oleh panduan gaya konten Sela.",
		"Short.")}, ""))
	blocked := a11yReportFor(t, a11yFixture("blocked", map[string]any{"summary": loc("Tekan tombol merah.", "Press the red button.")}, ""))
	good := ApprovalAccessibility{ChecksVersion: A11yChecksVersion, Result: A11yApprovedPass, Checks: pass.Checks,
		ManualVersion: A11yManualVersion, ManualConfirmed: ManualCheckIDs()}
	if !good.validRecord() {
		t.Fatal("good record refused")
	}
	bad := map[string]func(a *ApprovalAccessibility){
		"nil checks":              func(a *ApprovalAccessibility) { a.Checks = nil },
		"wrong version":           func(a *ApprovalAccessibility) { a.ChecksVersion = "v0" },
		"partial checklist":       func(a *ApprovalAccessibility) { a.ManualConfirmed = a.ManualConfirmed[:1] },
		"blocked checks":          func(a *ApprovalAccessibility) { a.Checks = blocked.Checks },
		"pass hides a failure":    func(a *ApprovalAccessibility) { a.Checks = needs.Checks },
		"override without reason": func(a *ApprovalAccessibility) { a.Checks, a.Result = needs.Checks, A11yApprovedOverridden },
		"unknown result":          func(a *ApprovalAccessibility) { a.Result = "skipped" },
	}
	for name, mutate := range bad {
		a := good
		a.ManualConfirmed = append([]string(nil), good.ManualConfirmed...)
		mutate(&a)
		if a.validRecord() {
			t.Errorf("%s: accepted", name)
		}
	}
	var nilRec *ApprovalAccessibility
	if nilRec.validRecord() {
		t.Error("nil record accepted")
	}
}

func TestA11ySentenceSplitMatchesValidator(t *testing.T) {
	t.Parallel()
	got := a11ySentences(`Satu dua. "Tiga empat." Lima? enam! Tujuh 2.5 delapan`)
	want := []string{`Satu dua.`, `"Tiga empat."`, `Lima?`, `enam!`, `Tujuh 2.5 delapan`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sentences = %q", got)
	}
	for word, n := range map[string]int{"the": 1, "breathing": 2, "consultation": 4} {
		if a11ySyllables(word) != n {
			t.Errorf("syllables(%s) = %d, want %d", word, a11ySyllables(word), n)
		}
	}
}
