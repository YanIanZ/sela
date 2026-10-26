package safety

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

func TestEmptyAndWhitespace(t *testing.T) {
	for _, s := range []string{"", " ", "    ", "\n\t\r\n", "\u200b\ufeff", "\u00a0\u3000", " . , ! ? ; "} {
		a := Assess(s)
		want := Assessment{Level: LevelNone, Ruleset: RulesetVersion}
		if !reflect.DeepEqual(a, want) {
			t.Errorf("Assess(%q) = %+v, want %+v", s, a, want)
		}
		if a.Interrupts() {
			t.Errorf("Assess(%q).Interrupts() = true", s)
		}
	}
}

func TestDeterminism(t *testing.T) {
	vs := loadVectors(t)
	first := make([]Assessment, len(vs))
	for i, v := range vs {
		first[i] = Assess(v.Text)
	}
	for round := 0; round < 3; round++ {
		for i, v := range vs {
			if got := Assess(v.Text); !reflect.DeepEqual(got, first[i]) {
				t.Fatalf("Assess(%q) changed between runs: %+v vs %+v", v.Text, got, first[i])
			}
		}
	}
	// Concurrent callers see the same results (run with -race).
	var wg sync.WaitGroup
	errs := make(chan string, 8)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for k := range vs {
				i := (k + offset*37) % len(vs)
				if got := Assess(vs[i].Text); !reflect.DeepEqual(got, first[i]) {
					errs <- vs[i].Text
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for text := range errs {
		t.Errorf("concurrent Assess(%q) differs", text)
	}
}

// TestAssessmentHasNoInputText checks that an Assessment is built only from
// a closed vocabulary (levels, categories, rule ids, the ruleset) and never
// echoes the input or its normalised form.
func TestAssessmentHasNoInputText(t *testing.T) {
	ids := map[string]bool{}
	var vocab strings.Builder
	for _, r := range catalogue {
		ids[r.id] = true
		fmt.Fprintf(&vocab, "%s %s ", r.id, r.category)
	}
	vocab.WriteString(RulesetVersion + " none elevated crisis level ruleIds categories thirdParty denial ruleset true false")
	cats := map[Category]bool{CategorySuicide: true, CategorySelfHarm: true, CategoryViolence: true, CategoryHopelessness: true}
	allowedKeys := map[string]bool{"level": true, "ruleIds": true, "categories": true, "thirdParty": true, "denial": true, "ruleset": true}

	inputs := []string{"my secret is: Aku mau bunuh diri malam ini di rumah nenek", "\"I want to die\" she said, call 555-0100"}
	for _, v := range loadVectors(t) {
		inputs = append(inputs, v.Text)
	}
	for _, in := range inputs {
		a := Assess(in)
		b, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		for k := range m {
			if !allowedKeys[k] {
				t.Errorf("unexpected key %q in %s", k, b)
			}
		}
		if _, ok := levelRank[a.Level]; !ok || a.Ruleset != RulesetVersion {
			t.Errorf("bad level or ruleset in %s", b)
		}
		for _, id := range a.RuleIDs {
			if !ids[id] {
				t.Errorf("rule id %q is not in the catalogue", id)
			}
		}
		for _, c := range a.Categories {
			if !cats[c] {
				t.Errorf("category %q is not a known category", c)
			}
		}
		for _, s := range []string{strings.TrimSpace(in), Normalize(in)} {
			if len(s) >= 6 && !strings.Contains(vocab.String(), s) && strings.Contains(string(b), s) {
				t.Errorf("assessment JSON contains the input: %s", b)
			}
		}
	}
}

// TestAssessmentInvariants checks structural properties on the corpus.
func TestAssessmentInvariants(t *testing.T) {
	for _, v := range loadVectors(t) {
		checkInvariants(t, v.Text, Assess(v.Text))
	}
}

func checkInvariants(t *testing.T, in string, a Assessment) {
	t.Helper()
	prev := -1
	cats := map[Category]bool{}
	for _, id := range a.RuleIDs {
		i, ok := ruleIndex[id]
		if !ok || i <= prev {
			t.Fatalf("Assess(%q): rule ids not unique and in catalogue order: %v", in, a.RuleIDs)
		}
		prev = i
		cats[catalogue[i].category] = true
	}
	if len(cats) != len(a.Categories) {
		t.Fatalf("Assess(%q): categories %v do not match rules %v", in, a.Categories, a.RuleIDs)
	}
	for _, c := range a.Categories {
		if !cats[c] {
			t.Fatalf("Assess(%q): category %s without a rule", in, c)
		}
	}
	if (a.Level == LevelNone) != (len(a.RuleIDs) == 0) {
		t.Fatalf("Assess(%q): level %s with rules %v", in, a.Level, a.RuleIDs)
	}
	if a.ThirdParty && a.Level != LevelElevated {
		t.Fatalf("Assess(%q): ThirdParty at level %s", in, a.Level)
	}
	if a.Interrupts() != (a.Level == LevelCrisis) {
		t.Fatalf("Assess(%q): Interrupts disagrees with level", in)
	}
	if a.Ruleset != RulesetVersion {
		t.Fatalf("Assess(%q): ruleset %q", in, a.Ruleset)
	}
}

func FuzzAssess(f *testing.F) {
	for _, s := range []string{"", "aku mau bunuh diri", "temanku bilang \"aku mau mati\"", "I don't want to die, but",
		"kill me!!!", "b u n u h d i r i", "\"'\"'", "tapi tapi, , ;", "hp-ku mau mati", "\xff\xfe", "a b c d e f"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		a := Assess(in)
		checkInvariants(t, in, a)
		if b := Assess(in); !reflect.DeepEqual(a, b) {
			t.Fatalf("Assess(%q) is not deterministic", in)
		}
		if n := Normalize(in); !utf8.ValidString(n) {
			t.Fatalf("Normalize(%q) is not valid UTF-8", in)
		}
	})
}

func TestMax(t *testing.T) {
	none := Assess("hari ini cerah")
	crisis := Assess("aku mau bunuh diri")
	elevated := Assess("aku gak kuat lagi")
	thirdParty := Assess("temanku bilang dia mau bunuh diri")
	denied := Assess("aku tidak mau bunuh diri")
	for name, a := range map[string]Assessment{"none": none, "crisis": crisis, "elevated": elevated, "thirdParty": thirdParty, "denied": denied} {
		if !reflect.DeepEqual(Max(a, a), a) {
			t.Errorf("Max(%s, %s) is not %s", name, name, name)
		}
	}
	cases := []struct {
		name       string
		a, b       Assessment
		level      Level
		thirdParty bool
		denial     bool
	}{
		{"none+crisis", none, crisis, LevelCrisis, false, false},
		{"crisis+none", crisis, none, LevelCrisis, false, false},
		{"elevated+crisis", elevated, crisis, LevelCrisis, false, false},
		{"thirdParty+none", thirdParty, none, LevelElevated, true, false},
		{"none+thirdParty", none, thirdParty, LevelElevated, true, false},
		{"thirdParty+thirdParty", thirdParty, thirdParty, LevelElevated, true, false},
		{"thirdParty+elevated", thirdParty, elevated, LevelElevated, false, false},
		{"thirdParty+crisis", thirdParty, crisis, LevelCrisis, false, false},
		{"denied+none", denied, none, LevelNone, false, true},
		{"denied+crisis", denied, crisis, LevelCrisis, false, true},
	}
	for _, c := range cases {
		got := Max(c.a, c.b)
		if got.Level != c.level || got.ThirdParty != c.thirdParty || got.Denial != c.denial || got.Ruleset != RulesetVersion {
			t.Errorf("%s: Max = %+v, want level %s thirdParty %v denial %v", c.name, got, c.level, c.thirdParty, c.denial)
		}
		want := append(append([]string{}, c.a.RuleIDs...), c.b.RuleIDs...)
		want = slices.Compact(sortedUnique(want))
		if got := sortedUnique(got.RuleIDs); !slices.Equal(got, want) {
			t.Errorf("%s: merged rule ids %v, want %v", c.name, got, want)
		}
		if got.Interrupts() != (c.level == LevelCrisis) {
			t.Errorf("%s: Interrupts = %v", c.name, got.Interrupts())
		}
	}
	// Merging a chat history: the most severe message wins.
	history := []string{"Hari biasa", "Aku mau bunuh diri", "Halo"}
	merged := Assess("")
	for _, h := range history {
		merged = Max(merged, Assess(h))
	}
	if merged.Level != LevelCrisis || !slices.Contains(merged.RuleIDs, "explicit-suicide-phrase") {
		t.Errorf("history merge = %+v", merged)
	}
}

func sortedUnique(s []string) []string {
	out := append([]string{}, s...)
	sort.Strings(out)
	return slices.Compact(out)
}

// perfInput builds a realistic 8000-byte diary-style text: Indonesian,
// English and code-switched sentences, a quotation, a reporting frame, a
// denial and one crisis phrase near the end.
func perfInput() string {
	sentences := []string{
		"Hari ini aku capek banget, tugas kantor numpuk dan bos marah-marah terus.",
		"Temanku bilang aku harus istirahat, tapi deadline besok jam 9 pagi.",
		"I feel so tired lately, my boss is toxic and I just want to sleep for a week.",
		"Kemarin mati lampu dua jam, hp-ku mati juga, jadi mati gaya di kosan.",
		"Aku tidak mau bunuh diri, aku cuma butuh teman cerita.",
		"My friend texted me \"this joke is killing me\" and we laughed for an hour.",
		"Skripsi saya revisi terus, dosen pembimbing susah ditemui, honestly so done.",
		"Weekend nanti mau nonton film baru, I'm dying to see it!",
	}
	var b strings.Builder
	for i := 0; b.Len() < 7950; i++ {
		b.WriteString(sentences[i%len(sentences)])
		b.WriteByte(' ')
	}
	b.WriteString("Aku mau bunuh diri.")
	s := b.String()
	for len(s) < 8000 {
		s += " ok"
	}
	return s[:8000]
}

func medianDuration(f func()) time.Duration {
	var ds []time.Duration
	for i := 0; i < 7; i++ {
		start := time.Now()
		f()
		ds = append(ds, time.Since(start))
	}
	slices.Sort(ds)
	return ds[len(ds)/2]
}

// TestPerformance checks an 8000-character input is assessed in under 20ms.
// The realistic input must meet the budget in every build, including -race.
// Adversarial inputs (dense matches, many frames and quotes) must meet it in
// normal builds; the race detector slows them by more than 20x, so under
// -race their time is only reported.
func TestPerformance(t *testing.T) {
	const budget = 20 * time.Millisecond
	in := perfInput()
	if len(in) != 8000 {
		t.Fatalf("input is %d bytes", len(in))
	}
	if a := Assess(in); a.Level != LevelCrisis {
		t.Fatalf("realistic input level %s, want crisis (rules %v)", a.Level, a.RuleIDs)
	}
	if d := medianDuration(func() { Assess(in) }); d > budget {
		t.Errorf("realistic 8000-char input took %v, budget %v", d, budget)
	} else {
		t.Logf("realistic 8000-char input: %v (race=%v)", d, raceEnabled)
	}
	repeat := func(unit string) string {
		return strings.Repeat(unit, 8000/len(unit)+1)[:8000]
	}
	ending := func(unit, end string) string {
		return strings.Repeat(unit, (8000-len(end))/len(unit)) + end
	}
	for name, s := range map[string]string{
		"dense-crisis":  repeat("aku mau mati i want to die "),
		"quotes":        repeat(`temanku bilang "aku mau mati". `),
		"frames":        repeat("my friend said she wants to die, she wants to kill herself, "),
		"denials":       repeat("aku tidak mau bunuh diri tapi "),
		"no-punct":      repeat("aku capek banget kerja terus dan semua orang bikin pusing "),
		"letter-spaced": repeat("b u n u h d i r i "),
		// Inputs that were quadratic before (review finding): one frame per
		// segment, an unbounded "hopeless ... want to end it" gap, and many
		// quotations in a single clause.
		"she-said":     ending("she said, ", "i want to die"),
		"dia-bilang":   ending("dia bilang, ", "aku mau mati"),
		"hopeless":     ending("hopeless ", "i want to end it"),
		"quote-clause": repeat(`temanku bilang "aku mau mati" `),
		"own-quotes":   repeat(`aku bilang "aku mau mati", dia bilang `),
		"hedges":       repeat("aku tidak mau bunuh diri, tapi "),
	} {
		d := medianDuration(func() { Assess(s) })
		switch {
		case raceEnabled:
			t.Logf("adversarial %s: %v (race detector on, not asserted)", name, d)
		case d > budget:
			t.Errorf("adversarial %s took %v, budget %v", name, d, budget)
		default:
			t.Logf("adversarial %s: %v", name, d)
		}
	}
}

// TestLinearTime checks that Assess stays linear in the input length on the
// inputs that used to be quadratic: quadrupling the input must not take much
// more than four times as long. Skipped under the race detector, whose
// overhead makes timings unreliable.
func TestLinearTime(t *testing.T) {
	if raceEnabled || testing.Short() {
		t.Skip("timing check skipped under -race and -short")
	}
	units := map[string]string{
		"she-said": "she said, ", "dia-bilang": "dia bilang, ", "hopeless": "hopeless ",
		"quote-clause": `temanku bilang "aku mau mati" `, "colons": "temanku bilang: kamu, ",
		"breakers": "dia bilang dia capek terus dia ", "hedges": "aku tidak mau bunuh diri, tapi ",
	}
	fastest := func(in string) time.Duration {
		best := time.Duration(1<<63 - 1)
		for i := 0; i < 5; i++ {
			start := time.Now()
			Assess(in)
			best = min(best, time.Since(start))
		}
		return best
	}
	for name, u := range units {
		small := strings.Repeat(u, 8000/len(u)) + "i want to die"
		large := strings.Repeat(u, 32000/len(u)) + "i want to die"
		ds, dl := fastest(small), fastest(large)
		// Linear is 4x; allow noise, and ignore sub-millisecond timings.
		if dl > 10*ds && dl > 5*time.Millisecond {
			t.Errorf("%s: 8000 chars %v, 32000 chars %v: not linear", name, ds, dl)
		} else {
			t.Logf("%s: 8000 chars %v, 32000 chars %v", name, ds, dl)
		}
	}
}

// TestAtomIndexSkipped covers sentencesWith when a derived variant skips
// unchanged sentences: an atom that first occurs in a skipped sentence must
// not hide the next kept sentence (review finding).
func TestAtomIndexSkipped(t *testing.T) {
	text := "digantung. a. b. c. d. e. f. gantung diri. digantung. gantung."
	all := sentenceSpans(text)
	var kept []span
	for _, sp := range all {
		if strings.Contains(text[sp.start:sp.end], "gantung diri") || text[sp.start:sp.end] == " gantung" {
			kept = append(kept, sp)
		}
	}
	if len(kept) != 2 {
		t.Fatalf("setup: kept %d sentences", len(kept))
	}
	x := &atomIndex{text: text, sentences: kept}
	if got := x.sentencesWith("gantung"); !slices.Equal(got, []int{0, 1}) {
		t.Errorf("sentencesWith(gantung) = %v, want [0 1]", got)
	}
	if got := x.sentencesWith("diri"); !slices.Equal(got, []int{0}) {
		t.Errorf("sentencesWith(diri) = %v, want [0]", got)
	}
	if got := x.sentencesWith("zzz"); len(got) != 0 {
		t.Errorf("sentencesWith(zzz) = %v, want none", got)
	}
	// End to end: more sentences than directCheck, the atom in an earlier
	// unchanged sentence, the match only in the dash-joined variant.
	in := "Hari-hari berat. Kerja-kerja terus. Teman-teman sibuk. Rapat-rapat panjang. Tugas-tugas numpuk. " +
		"Malam-malam begadang. Jemuran belum digantung. Pengen gan-tung di-ri."
	if n := len(sentenceSpans(Normalize(in))); n <= directCheck {
		t.Fatalf("setup: %d sentences, need more than %d", n, directCheck)
	}
	if a := Assess(in); a.Level != LevelCrisis {
		t.Errorf("Assess(%q) = %s, want crisis", in, a.Level)
	}
}

func BenchmarkAssess8000(b *testing.B) {
	in := perfInput()
	b.SetBytes(int64(len(in)))
	for i := 0; i < b.N; i++ {
		Assess(in)
	}
}

func BenchmarkAssessShort(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Assess("aku capek banget hari ini, tapi masih kuat kok")
	}
}
