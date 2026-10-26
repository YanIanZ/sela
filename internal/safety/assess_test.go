package safety

import (
	"encoding/json"
	"fmt"
	"math/rand"
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

// perfBudget is the absolute bound for an 8000-character input in a normal
// build. Typical times are 2-20ms (see the package documentation); the bound
// is generous so that a slow or loaded CI runner does not fail, while
// TestLinearTime catches super-linear behaviour by growth, not by time.
const perfBudget = 200 * time.Millisecond

// adversarialInputs are 8000-character inputs that stress one part of the
// engine each: dense matches, many frames, quotes, denials and breakers, the
// inputs that were quadratic before (one frame per segment, an unbounded
// "hopeless ... want to end it" gap, many quotations in a single clause),
// and inputs that change every context unit in the derived variants.
func adversarialInputs() map[string]string {
	repeat := func(unit string) string {
		return strings.Repeat(unit, 8000/len(unit)+1)[:8000]
	}
	ending := func(unit, end string) string {
		return strings.Repeat(unit, (8000-len(end))/len(unit)) + end
	}
	return map[string]string{
		"dense-crisis":  repeat("aku mau mati i want to die "),
		"quotes":        repeat(`temanku bilang "aku mau mati". `),
		"frames":        repeat("my friend said she wants to die, she wants to kill herself, "),
		"denials":       repeat("aku tidak mau bunuh diri tapi "),
		"no-punct":      repeat("aku capek banget kerja terus dan semua orang bikin pusing "),
		"letter-spaced": repeat("b u n u h d i r i "),
		"she-said":      ending("she said, ", "i want to die"),
		"dia-bilang":    ending("dia bilang, ", "aku mau mati"),
		"hopeless":      ending("hopeless ", "i want to end it"),
		"quote-clause":  repeat(`temanku bilang "aku mau mati" `),
		"own-quotes":    repeat(`aku bilang "aku mau mati", dia bilang `),
		"hedges":        repeat("aku tidak mau bunuh diri, tapi "),
		// Every unit differs in several variants (elongation, emoji, "1").
		"variant-units": repeat("aku mau matiii 😭 1. "),
		"variant-said":  repeat("she said, i want to dieee 😭, "),
		"variant-frame": repeat("temanku bilang 😭 dia mau mati, "),
		"leet":          repeat("k1ll m@t! $uicide kiII "),
		"spaced-phrase": repeat("a k u  m a u  m a t i  "),
		"colon-speech":  repeat("temanku bilang: aku capek aku mau mati, "),
		"shared-too":    repeat("she wants to die too, she wants to kill herself too "),
		// Many single quotes among double quotes (findQuotes was
		// O(singles x doubles)), and one post-check window per match.
		"mixed-quotes": repeat(`'a' "b" `),
		"legal-will":   repeat("surat wasiat "),
		"accident":     repeat("I cut myself "),
		"gym":          repeat("I hurt myself at the gym "),
		"kms":          repeat("kms "),
		"media-report": repeat("aku baca berita ada mahasiswa bunuh diri pengen nyusul "),
		"elongated":    repeat("pengeen maati meee tooo "),
		// Runs of pause marks and sentence punctuation, and many shared
		// third-person wishes in one segment, were quadratic (review
		// finding).
		"dash-pauses": repeat("- - "),
		"dot-runs":    repeat("... "),
		"bangs":       repeat("!!!! "),
		"shared-wish": repeat("she wants to kill herself "),
		"said-wish":   repeat("my friend said she wants to die "),
		"adoption":    repeat("temanku bilang dia mau bunuh diri. aku juga wkwk "),
		"news-intent": repeat("aku mau bom bunuh diri "),
		"wish-tail":   repeat("my friend said she wants to die. pengen "),
		// Round 3: the wake-routine check, "aku juga" tails, trailing-only
		// forms and the new self-harm and vehicle patterns, all many times
		// in one sentence.
		"wake-routine": repeat("i hope i don't wake up late "),
		"juga-tails":   repeat("aku juga udah aku pun lagi "),
		"trail-forms":  repeat("gw banget metoo big mood i'm next "),
		"hit-myself":   repeat("I hit myself "),
		"vehicle-wish": repeat("i hope a car hits me "),
	}
}

// TestPerformance checks that 8000-character inputs, realistic and
// adversarial, are assessed within perfBudget in a normal build. Under the
// race detector, which slows regexp matching by more than 20x, the absolute
// bound is skipped and times are only reported; TestLinearTime still checks
// the growth.
func TestPerformance(t *testing.T) {
	in := perfInput()
	if len(in) != 8000 {
		t.Fatalf("input is %d bytes", len(in))
	}
	if a := Assess(in); a.Level != LevelCrisis {
		t.Fatalf("realistic input level %s, want crisis (rules %v)", a.Level, a.RuleIDs)
	}
	inputs := adversarialInputs()
	inputs["realistic"] = in
	for name, s := range inputs {
		if raceEnabled {
			start := time.Now()
			Assess(s)
			t.Logf("%s: %v (race detector on, not asserted)", name, time.Since(start))
			continue
		}
		d := medianDuration(func() { Assess(s) })
		switch {
		case d > perfBudget:
			t.Errorf("%s 8000-char input took %v, budget %v", name, d, perfBudget)
		default:
			t.Logf("%s: %v", name, d)
		}
	}
}

// TestLinearTime checks that Assess stays linear in the input length:
// making the input eight times longer must not make it take much more than
// eight times as long (a quadratic step would take 64 times as long). A
// failing measurement is repeated once, so a burst of load on the runner
// does not fail it, while a super-linear step fails both times. The ratio is
// independent of the machine's speed, so it is also checked under the race
// detector, on shorter inputs.
func TestLinearTime(t *testing.T) {
	if testing.Short() {
		t.Skip("timing check skipped under -short")
	}
	base, reps := 4000, 5
	if raceEnabled {
		base, reps = 1000, 3
	}
	units := map[string]string{
		"she-said": "she said, ", "dia-bilang": "dia bilang, ", "hopeless": "hopeless ",
		"quote-clause": `temanku bilang "aku mau mati" `, "colons": "temanku bilang: kamu, ",
		"breakers": "dia bilang dia capek terus dia ", "hedges": "aku tidak mau bunuh diri, tapi ",
		"variant-units": "aku mau matiii 😭 1. ", "variant-said": "she said, i want to dieee 😭, ",
		"shared-too": "she wants to die too, ", "colon-speech": "temanku bilang: aku capek aku mau mati, ",
		"spaced-phrase": "a k u  m a u  m a t i  ", "leet": "k1ll m@t! $uicide kiII ",
		"mixed-quotes": `'a' "b" `, "legal-will": "surat wasiat ", "gym": "I hurt myself at the gym ",
		"media-report": "aku baca berita ada mahasiswa bunuh diri pengen nyusul ", "elongated": "pengeen maati meee tooo ",
		"dash-pauses": "- - ", "dot-runs": "... ", "bangs": "!!!! ", "shared-wish": "she wants to kill herself ",
		"said-wish": "my friend said she wants to die ", "adoption": "temanku bilang dia mau bunuh diri. aku juga wkwk ",
		"news-intent": "aku mau bom bunuh diri ", "wish-tail": "my friend said she wants to die. pengen ",
		"wake-routine": "i hope i don't wake up late ", "juga-tails": "aku juga udah aku pun lagi ",
		"trail-forms": "gw banget metoo big mood i'm next ", "hit-myself": "I hit myself ",
		"vehicle-wish": "i hope a car hits me ",
	}
	fastest := func(in string) time.Duration {
		best := time.Duration(1<<63 - 1)
		for i := 0; i < reps; i++ {
			start := time.Now()
			Assess(in)
			best = min(best, time.Since(start))
		}
		return best
	}
	const growth, limit = 8, 24 // linear is 8x, quadratic 64x
	for name, u := range units {
		small := strings.Repeat(u, base/len(u)) + "i want to die"
		large := strings.Repeat(u, growth*base/len(u)) + "i want to die"
		superLinear := func() (bool, time.Duration, time.Duration) {
			ds, dl := fastest(small), fastest(large)
			// Ignore timings too small to measure reliably.
			return dl > limit*ds && dl > 10*time.Millisecond, ds, dl
		}
		bad, ds, dl := superLinear()
		if bad {
			bad, ds, dl = superLinear()
		}
		if bad {
			t.Errorf("%s: %d chars %v, %d chars %v: grows faster than linear", name, base, ds, growth*base, dl)
		} else {
			t.Logf("%s: %d chars %v, %d chars %v", name, base, ds, growth*base, dl)
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

// TestSkipEquivalence checks the skip optimisation of assess: matching only
// the context units of a derived variant that differ from the canonical
// form's gives exactly the result of matching every variant in full. The
// inputs join corpus texts and short pieces that change in the variants
// (elongation, "1", hyphens, emoji, quotes ending in sentence punctuation,
// adoption phrases) with every kind of separator.
func TestSkipEquivalence(t *testing.T) {
	var pool []string
	for _, v := range loadVectors(t) {
		pool = append(pool, v.Text)
	}
	pool = append(pool, "capekkk", "hari-hari berat", "me tooooo", "1 relate", "so do 1", "she said 1 was dramatic",
		"temanku bilang dia mau bunuh diri", "my friend said she wants to die", "aku juga", "same", "relatable",
		"mati lampu", "\"I want to die.\" she said 1 was dramatic", "I want to killll myself", "mau-mati",
		"jemuran digantung", "tooo", "me-too", "\"aku mau mati.\" kata dia", "\"aku mau mati.\" kata 1",
		"I don't want to die, but", "tapi-", "aku tidak mau bunuh diri", "aku pengen 😭 mati", "\"die\"",
		"i wanna die", "she wants to die tooo", "aku pun", "same lol", "a k u  m a u  m a t i", "k|ll myself",
		"meee tooo", "pengeen maati", "$u1c1d3", "aku pengen... 😭 mati", "I want to . . . die", "aku pengen - mati",
		"pengen nyusul", "aku sama", "omg same", "Mama bilang: makan dulu", "Nyokap ngomong gt", "lirik lagunya:",
		"kms", "5 kms", "I cut myself shaving", "at the gym", "dari sosmed", "mati rasa", "my uncle killed himself too",
		// Round 2: the context-marker gate, adoption tails and compounds.
		"because I wanted to", "I liked it", "biar lega", "like the doctor said", "surat wasiat yang sah",
		"aku juga wkwk", "so do i haha", "aku pengen", "I want to", "pengen ngelakuin juga", "aku mau bom bunuh diri",
		"I want to be a suicide bomber", "misi bunuh diri", "nggak sengaja ngiris tangan pas potong bawang",
		"aku juga panik", "wanna", "aku baca berita ada mahasiswa bunuh diri", "jadi pengen ngelakuin juga")
	seps := []string{". ", ".\n", "! ", "? ", ", ", " ", "... ", "\n", " tapi ", "; "}
	rng := rand.New(rand.NewSource(5))
	n := 8000
	if raceEnabled {
		n = 800
	}
	for i := 0; i < n; i++ {
		var b strings.Builder
		for j, k := 0, 1+rng.Intn(8); j < k; j++ {
			if j > 0 {
				b.WriteString(seps[rng.Intn(len(seps))])
			}
			b.WriteString(pool[rng.Intn(len(pool))])
		}
		s := b.String()
		if got, want := assess(s, true), assess(s, false); !reflect.DeepEqual(got, want) {
			t.Fatalf("assess(%q): with skip %+v, in full %+v", s, got, want)
		}
	}
}

// TestCrossVariantAdoption covers adoption that only a derived variant
// spells while the attributed match is in an unchanged unit (review
// finding: the skipped unit's match was never promoted).
func TestCrossVariantAdoption(t *testing.T) {
	for _, in := range []string{
		"my friend said she wants to die. me tooooo",
		"temanku bilang dia mau bunuh diri. me tooooo",
		"In the movie, he wanted to die. 1 relate.",
		"\"I want to die.\" she said 1 was dramatic",
		"\"aku pengen bunuh diri\" kata temanku.\nme tooooo",
		// Both words elongated: the canonical form reads "me to".
		"my friend said she wants to die. meee tooo",
		"temanku bilang dia mau bunuh diri. meeee toooo",
	} {
		if a := Assess(in); a.Level != LevelCrisis {
			t.Errorf("Assess(%q) = %s %v, want crisis", in, a.Level, a.RuleIDs)
		}
	}
}

func TestContextUnits(t *testing.T) {
	keys := func(s string) []string {
		var out []string
		for _, u := range contextUnits(analyse(s)) {
			out = append(out, s[u.start:u.end])
		}
		return out
	}
	cases := map[string][]string{
		"a. b! c? d":                        {"a", " b", " c", " d"},
		`"i want to die." she said`:         {`"i want to die." she said`},
		"temanku bilang \"capek?\" ok. dia": {"temanku bilang \"capek?\" ok", " dia"},
		"":                                  {""},
	}
	for in, want := range cases {
		if got := keys(in); !slices.Equal(got, want) {
			t.Errorf("contextUnits(%q) = %q, want %q", in, got, want)
		}
	}
	// The key records the quotations and a following conjunction, which
	// change how matches count.
	a := contextUnits(analyse("aku tidak mau bunuh diri. tapi"))[0].key
	b := contextUnits(analyse("aku tidak mau bunuh diri. ok"))[0].key
	if a == b {
		t.Error("a following conjunction must change the unit key")
	}
	c := contextUnits(analyse(`x "a b" y`))[0].key
	d := contextUnits(analyse(`x "a b y`))[0].key
	if c == d {
		t.Error("the quotations must change the unit key")
	}
}
