package safety

import (
	"errors"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// goldenRuleIDs is the catalogue in order. Ids are part of the Assessment
// contract (callers log and compare them): never rename, drop or reorder
// one without bumping RulesetVersion and updating this list.
var goldenRuleIDs = []string{
	// v1, in crisisDetector.ts order
	"explicit-suicide-phrase", "death-wish-phrase", "no-will-to-live", "farewell-will-phrase",
	"self-cutting", "self-harm-intent", "first-person-euphemism", "hanging-phrase",
	"ingestion-poison", "jumping-phrases", "end-everything-phrase", "overdose-intent",
	"self-cutting-affixed", "life-not-worth-living", "cannot-bear-anymore",
	"severe-exhaustion-pain", "passive-ideation", "en-passive-ideation", "en-explicit-suicide",
	"en-death-wish", "en-self-harm", "en-lethal-means", "en-farewell-note", "en-end-everything",
	"en-overdose-means", "en-self-harm-ongoing", "en-ambiguous-distress", "en-severe-distress",
	// added by Sela
	"id-death-wish-slang", "id-no-will-to-live-slang", "id-vanish-forever",
	"id-vanish-from-world", "id-tired-of-living", "id-cannot-bear-slang", "id-better-dead",
	"en-death-wish-modal", "en-slang-suicide", "en-third-person-suicide",
	"en-third-person-self-harm",
	// added in sela-safety-2026.10.26-2
	"en-inner-death-wish",
	// added in sela-safety-2026.10.26-3
	"id-suicide-colloquial", "id-hanging-colloquial", "id-jumping-colloquial", "id-sleep-forever",
	"id-death-wish-formal", "mixed-death-wish", "en-tired-of-living", "en-sleep-forever",
	"en-self-harm-verbs", "en-lethal-means-forms", "id-self-harm-colloquial", "id-vehicle-death-wish",
	"en-vehicle-death-wish",
	// added in sela-safety-2026.10.26-5
	"en-hope-to-die", "en-death-wish-forms", "id-hope-to-die", "id-death-wish-adverb", "en-overdose-plan",
	"id-overdose-plan", "self-harm-slang", "en-passive-death-wish", "id-passive-death-wish",
}

func TestRuleIDsStableAndUnique(t *testing.T) {
	var ids []string
	seen := map[string]bool{}
	idRe := regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	for _, r := range catalogue {
		if seen[r.id] {
			t.Errorf("duplicate rule id %s", r.id)
		}
		seen[r.id] = true
		if !idRe.MatchString(r.id) {
			t.Errorf("rule id %q is not kebab-case", r.id)
		}
		ids = append(ids, r.id)
	}
	if !slices.Equal(ids, goldenRuleIDs) {
		t.Errorf("catalogue ids changed:\n got %v\nwant %v", ids, goldenRuleIDs)
	}
	for id, i := range ruleIndex {
		if catalogue[i].id != id {
			t.Errorf("ruleIndex[%s] = %d", id, i)
		}
	}
}

func TestCatalogueShape(t *testing.T) {
	validCat := []Category{CategorySuicide, CategorySelfHarm, CategoryViolence, CategoryHopelessness}
	v1Done := false
	for _, r := range catalogue {
		if !slices.Contains(validCat, r.category) {
			t.Errorf("%s: invalid category %q", r.id, r.category)
		}
		if r.level != LevelCrisis && r.level != LevelElevated {
			t.Errorf("%s: invalid level %q", r.id, r.level)
		}
		switch r.origin {
		case "v1":
			if v1Done {
				t.Errorf("%s: v1 rules must precede Sela rules", r.id)
			}
		case "sela":
			v1Done = true
		default:
			t.Errorf("%s: invalid origin %q", r.id, r.origin)
		}
		if r.thirdPerson && r.origin != "sela" {
			t.Errorf("%s: only Sela rules are third-person rules", r.id)
		}
		if len(r.patterns) == 0 {
			t.Errorf("%s: no patterns", r.id)
		}
		for _, p := range r.patterns {
			src := p.re.String()
			// pattern.each restarts searches at word starts, which is only
			// exact for expressions that begin with \b.
			if p.anchored != strings.HasPrefix(src, "^") || (!p.anchored && !strings.HasPrefix(src, `\b`)) {
				t.Errorf("%s: pattern must start with \\b (or ^ when anchored): %s", r.id, src)
			}
			if p.anchored && (p.reject != nil || p.marker != nil || p.third != nil) {
				t.Errorf("%s: anchored patterns cannot have post-checks", r.id)
			}
			if p.maximal && (p.anchored || p.reject != nil || p.marker != nil || p.third != nil) {
				t.Errorf("%s: maximal patterns cannot be anchored or have post-checks", r.id)
			}
			// Normalised text is lowercase, so a literal capital never matches.
			for i := 0; i < len(src); i++ {
				if c := src[i]; c >= 'A' && c <= 'Z' && (i == 0 || src[i-1] != '\\') {
					t.Errorf("%s: uppercase literal in %s", r.id, src)
					break
				}
			}
		}
	}
}

// v1RuleSource is one rule parsed from crisisDetector.ts.
type v1RuleSource struct{ id, pattern, category, severity string }

func loadV1Rules(t *testing.T) []v1RuleSource {
	t.Helper()
	path := filepath.Join("..", "..", "services", "crisis-engine", "src", "crisisDetector.ts")
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("v1 engine %s is not present", path)
	}
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?s)id: '([^']+)',.*?pattern: /(.*?)/i,\s*category: '([^']+)',\s*severity: '([^']+)'`)
	var out []v1RuleSource
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		out = append(out, v1RuleSource{m[1], m[2], m[3], m[4]})
	}
	if len(out) == 0 {
		t.Fatal("no rules parsed from the v1 engine")
	}
	return out
}

// v1SplitRules lists the v1 rules whose expression was split into several
// RE2 patterns (lookaheads or a post-check); the others must be verbatim.
var v1SplitRules = map[string]bool{
	"passive-ideation":      true, // two (?!...) lookaheads → placeNotWorld
	"en-end-everything":     true, // (?!\s*admin) → followedByAdmin
	"en-explicit-suicide":   true, // \bkms branch → kmsNotSlang
	"en-ambiguous-distress": true, // ^kill me$ branch is anchored
}

// TestV1Parity checks every v1 rule against the v1 source: same id, order,
// category and mapped level, and the same expression unless the rule is one
// of the documented splits.
func TestV1Parity(t *testing.T) {
	v1 := loadV1Rules(t)
	var ported []rule
	for _, r := range catalogue {
		if r.origin == "v1" {
			ported = append(ported, r)
		}
	}
	if len(ported) != len(v1) {
		t.Fatalf("ported %d v1 rules, v1 has %d", len(ported), len(v1))
	}
	for i, want := range v1 {
		got := ported[i]
		if got.id != want.id {
			t.Errorf("rule %d: id %s, v1 has %s", i, got.id, want.id)
			continue
		}
		if string(got.category) != want.category {
			t.Errorf("%s: category %s, v1 %s", got.id, got.category, want.category)
		}
		wantLevel := map[string]Level{"crisis": LevelCrisis, "high": LevelElevated}[want.severity]
		if got.level != wantLevel {
			t.Errorf("%s: level %s, v1 severity %s", got.id, got.level, want.severity)
		}
		if v1SplitRules[got.id] {
			continue
		}
		if len(got.patterns) != 1 || got.patterns[0].re.String() != want.pattern {
			t.Errorf("%s: expression differs from v1\n got %s\nwant %s", got.id, got.patterns[0].re.String(), want.pattern)
		}
	}
}

// TestPrefilterSound checks the derived prefilters never reject a sentence
// their expression matches, on the vector corpus and on random token soup
// built from the expressions' own literals.
func TestPrefilterSound(t *testing.T) {
	var inputs []string
	for _, v := range loadVectors(t) {
		for _, s := range variants(v.Text) {
			for _, sp := range sentenceSpans(s) {
				inputs = append(inputs, s[sp.start:sp.end])
			}
			inputs = append(inputs, s)
		}
	}
	vocab := map[string]bool{}
	word := regexp.MustCompile(`[a-z']+`)
	for _, r := range catalogue {
		for _, p := range r.patterns {
			for _, w := range word.FindAllString(strings.ReplaceAll(p.re.String(), `\s`, " "), -1) {
				vocab[w] = true
			}
		}
	}
	for _, in := range inputs {
		for _, w := range strings.Fields(in) {
			vocab[w] = true
		}
	}
	words := make([]string, 0, len(vocab))
	for w := range vocab {
		words = append(words, w)
	}
	slices.Sort(words)
	seps := []string{" ", " ", " ", "", "-", "'", ", "}
	rng := rand.New(rand.NewSource(1))
	samples := 6000
	if raceEnabled {
		samples = 1500 // the race detector makes regexp matching ~20x slower
	}
	for n := 0; n < samples; n++ {
		var b strings.Builder
		for k := 1 + rng.Intn(10); k > 0; k-- {
			b.WriteString(words[rng.Intn(len(words))])
			b.WriteString(seps[rng.Intn(len(seps))])
		}
		inputs = append(inputs, b.String())
	}
	matched := 0
	for _, r := range catalogue {
		for _, p := range r.patterns {
			for _, in := range inputs {
				if !p.re.MatchString(in) {
					continue
				}
				matched++
				if !p.mayMatch(in) {
					t.Errorf("%s: prefilter rejects a match: %q", r.id, in)
				}
				if p.atoms != nil && !slices.ContainsFunc(p.atoms, func(a string) bool { return strings.Contains(in, a) }) {
					t.Errorf("%s: atoms %v miss a match: %q", r.id, p.atoms, in)
				}
			}
		}
	}
	if matched < samples/12 {
		t.Errorf("only %d matches exercised the prefilters", matched)
	}
	// The context expressions are guarded the same way.
	context := map[string]guarded{"denial": denialRe, "adoption": adoptionRe}
	for name, a := range map[string]altGuarded{"frame": frameRe, "reportingHead": reportingHeadRe} {
		for i, g := range a.alts {
			context[name+strings.Repeat("'", i)] = g
		}
	}
	for name, g := range context {
		for _, in := range inputs {
			if g.re.MatchString(in) && g.f != nil && !g.f.match(in) {
				t.Errorf("%s: prefilter rejects a match: %q", name, in)
			}
		}
	}
}

// TestMaximalPatterns checks the property that lets a maximal pattern skip
// its other matches: on every sentence, the first match contains every match
// that starts at a later word.
func TestMaximalPatterns(t *testing.T) {
	inputs := []string{"hopeless hopeless i want to end it", "hopeless, want to end it and hopelessly wanna to end it",
		"i feel hopeless. want to end it", "hopelesswant to end it", "so hopeless i want to end it, hopeless want to end it all"}
	for _, v := range loadVectors(t) {
		inputs = append(inputs, Normalize(v.Text))
	}
	words := []string{"hopeless", "hopelessly", "want to end it", "wanna to end it", "want to end it all", "i", "so",
		"tired", ",", "and", "want", "to end", "it", "unhopeless"}
	rng := rand.New(rand.NewSource(3))
	for n := 0; n < 2000; n++ {
		var b strings.Builder
		for k := 1 + rng.Intn(12); k > 0; k-- {
			b.WriteString(words[rng.Intn(len(words))])
			b.WriteByte(' ')
		}
		inputs = append(inputs, b.String())
	}
	found := 0
	for _, r := range catalogue {
		for _, p := range r.patterns {
			if !p.maximal {
				continue
			}
			for _, in := range inputs {
				for _, sp := range sentenceSpans(in) {
					s := in[sp.start:sp.end]
					first := p.re.FindStringIndex(s)
					if first == nil {
						continue
					}
					found++
					for pos := nextWordStart(s, first[0]+1); pos <= len(s); pos = nextWordStart(s, pos+1) {
						loc := p.re.FindStringIndex(s[pos:])
						if loc == nil {
							break
						}
						if pos+loc[0] < first[0] || pos+loc[1] > first[1] {
							t.Errorf("%s: match %q is not inside the first match %q", r.id, s[pos+loc[0]:pos+loc[1]], s[first[0]:first[1]])
						}
					}
				}
			}
		}
	}
	if found < 50 {
		t.Errorf("only %d maximal matches exercised", found)
	}
}

func TestPrefilterDerivation(t *testing.T) {
	cases := []struct {
		expr    string
		matches []string
		rejects []string
	}{
		{`\bmau\s*mati\b`, []string{"aku mau mati"}, []string{"aku mau pulang", "mati lampu"}},
		{`\b(pengen\s*mati|mau\s*mati|mati\s*aja)\b`, []string{"pengen mati", "mati aja"}, []string{"mau makan", "hati aja"}},
		{`\b(?:want|wish)\s*(?:to\s*)?(?:die|dead)\b`, []string{"i want to die"}, []string{"i want to sleep", "they die"}},
		{`^kill\s*me[\s.!]*$`, []string{"kill me!"}, []string{"call me"}},
		{`\b(?:a|b)?\s*c+d*`, []string{"c", "ac"}, []string{"ab"}},
	}
	for _, c := range cases {
		f := compileFilter(c.expr)
		re := regexp.MustCompile(c.expr)
		for _, s := range c.matches {
			if !re.MatchString(s) {
				t.Fatalf("bad case: %s does not match %q", c.expr, s)
			}
			if f != nil && !f.match(s) {
				t.Errorf("%s: filter rejects match %q", c.expr, s)
			}
		}
		for _, s := range c.rejects {
			if re.MatchString(s) {
				t.Fatalf("bad case: %s matches %q", c.expr, s)
			}
			if f == nil || f.match(s) {
				t.Errorf("%s: filter should reject %q", c.expr, s)
			}
		}
	}
	if f := compileFilter(`\b(?:x)?\s*`); f != nil {
		t.Errorf("an expression that can match empty must have no filter, got %+v", f)
	}
}

// TestPostChecks covers the code that re-expresses v1 lookaheads and the new
// idiom checks. Each case is the text after normalisation and the phrase the
// pattern matched.
func TestPostChecks(t *testing.T) {
	type tc struct {
		text, match string
		reject      bool
	}
	run := func(name string, fn func(s string, start, end int) bool, cases []tc) {
		for _, c := range cases {
			i := strings.Index(c.text, c.match)
			if i < 0 {
				t.Fatalf("%s: %q not in %q", name, c.match, c.text)
			}
			if got := fn(c.text, i, i+len(c.match)); got != c.reject {
				t.Errorf("%s(%q at %q) = %v, want %v", name, c.text, c.match, got, c.reject)
			}
		}
	}
	// v1: (?!\s+(?:di|dalam)\s+(?!dunia))
	run("placeNotWorld", placeNotWorld, []tc{
		{"lebih baik aku nggak ada di rapat itu", "lebih baik aku nggak ada", true},
		{"mending aku gak ada di grup itu lagi", "mending aku gak ada", true},
		{"lebih baik aku nggak ada dalam hidup mereka", "lebih baik aku nggak ada", true},
		{"rasanya lebih baik aku nggak ada di dunia ini", "lebih baik aku nggak ada", false},
		{"lebih baik aku nggak ada dalam dunia ini", "lebih baik aku nggak ada", false},
		{"lebih baik aku nggak ada dia senang", "lebih baik aku nggak ada", false},
		{"lebih baik aku nggak ada aja", "lebih baik aku nggak ada", false},
		{"andai aku nggak pernah lahir", "andai aku nggak pernah lahir", false},
	})
	// v1: (?!\s*admin)
	run("followedByAdmin", followedByAdmin, []tc{
		{"i am done with life admin today", "i am done with life", true},
		{"i'm done with life", "i'm done with life", false},
		{"i'm done with life. admin is fine", "i'm done with life", false},
	})
	run("idiomaticDeath", idiomaticDeath, []tc{
		{"baterai hp-ku mau mati", "mau mati", true},
		{"hp gue mau mati nih", "mau mati", true},
		{"lampunya mati aja dari tadi", "mati aja", true},
		{"aku mau mati-matian belajar", "mau mati", true},
		{"aku mau mati matian belajar", "mau mati", true},
		{"aku mau mati", "mau mati", false},
		{"hp aku rusak, aku mau mati", "mau mati", false},
		{"rasanya mau mati aja", "mau mati", false},
		// Aspect words between the device and the match are skipped.
		{"hp-ku mau mati aja", "mati aja", true},
		{"laptopku udah mau mati", "mau mati", true},
		// A wish is never a device's: only "mau mati"/"mati aja" are checked.
		{"mati lampu aku pengen mati", "pengen mati", false},
		{"dari pagi di depan komputer pengen mati", "pengen mati", false},
		{"nunggu motor lebih baik mati", "lebih baik mati", false},
		{"lampu pengen mati", "pengen mati", false},
		{"aku pengen mati-matian", "pengen mati", true},
		// A governed device is an object or a place, not the subject.
		{"di depan laptop mau mati", "mau mati", false},
		{"nunggu wifi mau mati", "mau mati", false},
		{"di kosan hp-ku mau mati", "mau mati", true},
		// "<device> aku" is "my device" only at the start of a clause.
		{"dari tadi nunggu wifi aku mau mati", "mau mati", false},
		{"di kosan hp aku mau mati", "mau mati", false},
		{"duh hp aku mau mati", "mau mati", true},
		{"baterai hp aku mau mati", "mau mati", true},
		{"mati lampu, hp gue mau mati", "mau mati", true},
		// An idiom noun after "mati", wishes included (review finding).
		{"besok mau mati listrik seharian", "mau mati", true},
		{"pengen mati lampu aja biar libur", "pengen mati", true},
		{"lucu banget aku mau mati ketawa", "mau mati", true},
		{"harusnya aku mati rasa", "harusnya aku mati", true},
		{"seharusnya aku mati rasa aja sama dia", "seharusnya aku mati", true},
		{"aku mau mati rasanya", "mau mati", false},
		// "rasa" that opens a new noun phrase is not numbness, and "kata" is
		// not an idiom noun (review finding).
		{"aku mau mati rasa ini terlalu berat", "mau mati", false},
		{"pengen mati rasa sakitnya ga hilang", "pengen mati", false},
		{"pengen mati kata orang itu dosa", "pengen mati", false},
		{"aku mau mati, lampu mati", "mau mati", false},
		// A device after "mati aja", past a particle.
		{"tiba-tiba mati aja tv-nya", "mati aja", true},
		{"yaudah mati aja lah hp-ku", "mati aja", true},
		{"mati aja lah aku", "mati aja", false},
		// Animals and plants are things; a species after its kind too.
		{"kucingku mau mati, sakit parah", "mau mati", true},
		{"ikan cupangku mau mati", "mau mati", true},
		{"kucing tetangga mau mati", "mau mati", true},
		{"bunga mawarnya mau mati", "mau mati", true},
		{"gara-gara kucing aku mau mati", "mau mati", false},
		// A swear word is a thing only with a possessive suffix.
		{"anjing aku mau mati aja", "mau mati", false},
		{"anjingku udah mau mati", "mau mati", true},
		// "<thing>-ku aku" is the speaker as a new subject, and "tiap jam"
		// is a time (round-3 review finding).
		{"kucingku aku mau mati", "mau mati", false},
		{"hp-ku aku mau mati aja", "mau mati", false},
		{"tiap jam mau mati rasanya", "mau mati", false},
		{"setiap jam mau mati aja", "mau mati", false},
		{"2 jam mau mati", "mau mati", false},
		{"jam mau mati", "mau mati", true},
	})
	selfCases := []tc{
		{"part of me wants to die", "wants to die", true},
		{"every inch of me wants to die", "wants to die", true},
		{"the darkness inside me wants to die", "wants to die", true},
		{"you and me wants to die", "wants to die", true},
		{"me wants to die", "wants to die", true},
		{"god, me wants to die", "wants to die", true},
		{"the depressed me wants to die", "wants to die", true},
		{"the guy who hates me wants to die", "wants to die", false},
		// An object "me" is not the speaker's inner self (review finding).
		{"someone close to me wants to die", "wants to die", false},
		{"the girl next to me wants to die", "wants to die", false},
		{"my friend who lives with me wants to die", "wants to die", false},
		{"the guy who bullied me wants to die", "wants to die", false},
		{"everyone around me wants to die", "wants to die", false},
		{"the girl in front of me wants to die", "wants to die", false},
		{"everything in me still wants to die", "wants to die", true},
		{"my soul wants to die", "wants to die", true},
		{"my whole body wants to die", "wants to die", true},
		{"something inside me wants to die", "wants to die", true},
		{"a piece of me wants to die", "wants to die", true},
		{"the real me wants to die", "wants to die", true},
		{"my inner child wants to die", "wants to die", true},
		{"my inner self wants to die", "wants to die", true},
		{"everything about me wants to die", "wants to die", true},
		{"this body wants to die", "wants to die", true},
		{"the guy who talks about me wants to die", "wants to die", false},
		{"my friend wants to die", "wants to die", false},
		{"she wants to die", "wants to die", false},
		{"his body wants to die", "wants to die", false},
		{"his whole body wants to die", "wants to die", false},
		{"his inner child wants to die", "wants to die", false},
		{"my child wants to die", "wants to die", false},
		{"wants to die", "wants to die", false},
	}
	run("selfOrThing", selfOrThing, append(selfCases,
		tc{"my laptop wants to die", "wants to die", true},
		tc{"my phone battery literally wants to die", "wants to die", true},
		tc{"the plant next to me wants to die", "wants to die", true},
		tc{"she wants to die her hair", "wants to die", true}))
	// en-inner-death-wish counts exactly the inner-self subjects selfOrThing
	// takes away from en-third-person-suicide.
	for i := range selfCases {
		selfCases[i].reject = !selfCases[i].reject
	}
	run("notInnerSubject", notInnerSubject, selfCases)
	run("kmsNotSlang", kmsNotSlang, []tc{
		{"i am 5 kms from home", "kms", true},
		{"kms", "kms", false},
		{"honestly kms", "kms", false},
		{"i want to kms", "kms", false},
		{"it's a few kms away", "kms", true},
		{"we drove hundreds of kms", "kms", true},
		{"berapa kms dari sini", "kms", true},
		{"aku lupa bawa kms ke posyandu", "kms", true},
		{"kms anakku hilang", "kms", true},
		{"bawa buku kms anakku", "kms", true},
		{"jadi anak sulung, kms", "kms", false},
		// A distance needs a number or a quantity word, the card a card or
		// health-post word, and an intent word keeps the slang (review
		// finding).
		{"i want to kms to be honest", "kms", false},
		{"im gonna kms from stress", "kms", false},
		{"kms to escape this", "kms", false},
		{"pengen kms dari kemarin", "kms", false},
		{"capek jadi anak sulung kms", "kms", false},
		{"waiting at the clinic kms", "kms", false},
		{"honestly kms anak-anak", "kms", false},
		{"kms anak saya hilang", "kms", true},
		{"how many kms is it", "kms", true},
		// A number after a time word is a time (round-3 review finding).
		{"exam at 8 kms", "kms", false},
		{"meeting jam 9 kms", "kms", false},
		{"i ran 5 kms today", "kms", true},
		// "until" measures a distance, and a speed is not a time (round-4
		// review finding).
		{"i'll run until 10 kms", "kms", true},
		{"at 5 kms per hour it takes forever", "kms", true},
		// "by" a number is a deadline, and "until" a time unless a motion
		// verb comes before it (round-5 review finding).
		{"done by 9 kms", "kms", false},
		{"gonna wait until 10 kms", "kms", false},
		{"keep cycling until 40 kms", "kms", true},
	})
	run("suicideCompound", suicideCompound, []tc{
		{"gol bunuh diri bikin kalah", "bunuh diri", true},
		{"itu misi bunuh diri", "bunuh diri", true},
		{"ini bunuh diri karier", "bunuh diri", true},
		{"nonton suicide squad", "suicide", true},
		{"that would be career suicide", "suicide", true},
		{"run suicide sprints", "suicide", true},
		{"aku mau bunuh diri", "bunuh diri", false},
		{"bom bunuh diri", "bunuh diri", false},
		{"i am suicidal", "suicidal", false},
		{"thinking about suicide", "suicide", false},
		// A pronoun after the compound noun, or an intent word that governs
		// the match, keeps it (review finding).
		{"aku mau bunuh diri karier aku hancur", "bunuh diri", false},
		{"aku mau bunuh diri karier", "bunuh diri", false},
		{"i want to watch suicide squad", "suicide", true},
		{"aku mau nonton suicide squad", "suicide", true},
		// An intent word or a first-person subject that governs the
		// compound keeps it, past articles and light verbs (review finding).
		{"aku mau jalanin misi bunuh diri biar mati", "bunuh diri", false},
		{"i want to go on a suicide mission and never come back", "suicide", false},
		{"i'm planning a suicide run tonight", "suicide", false},
		{"bek itu cetak gol bunuh diri", "bunuh diri", true},
		{"i'd commit career suicide", "suicide", true},
		{"i want to commit career suicide", "suicide", false},
		{"i'm on a suicide mission at work lol", "suicide", true},
		{"aku jalanin misi bunuh diri di game", "bunuh diri", true},
		{"aku bunuh diri karier", "bunuh diri", false},
	})
	run("suicideNews", suicideNews, []tc{
		{"bom bunuh diri di gereja", "bunuh diri", true},
		{"pelaku bom bunuh diri", "bunuh diri", true},
		{"the suicide bomber", "suicide", true},
		{"aku mau bunuh diri", "bunuh diri", false},
		{"suicide is not the answer", "suicide", false},
		{"aku baca berita bom bunuh diri di gereja", "bunuh diri", true},
		{"my brother is a suicide bomber", "suicide", true},
		// The speaker's own plan is not someone else's act (review finding).
		{"aku mau bom bunuh diri", "bunuh diri", false},
		{"aku pengen jadi pelaku bom bunuh diri", "bunuh diri", false},
		{"aku mau ngelakuin serangan bunuh diri", "bunuh diri", false},
		{"i want to be a suicide bomber", "suicide", false},
		{"i'm going to wear a suicide vest tomorrow", "suicide", false},
		{"i am a suicide bomber", "suicide", false},
		// A third party's readiness is their act; a third party joined to
		// the speaker or after a preposition is not the subject (round-5
		// review finding).
		{"teroris itu siap jadi pelaku bom bunuh diri", "bunuh diri", true},
		{"teroris itu siap jalanin misi bunuh diri", "bunuh diri", true},
		{"aku dan temanku siap jadi pelaku bom bunuh diri", "bunuh diri", false},
		{"demi mereka siap jadi pelaku bom bunuh diri", "bunuh diri", false},
		{"aku ikut mereka siap jadi pelaku bom bunuh diri", "bunuh diri", false},
	})
	run("suicideCompoundReady", suicideCompound, []tc{
		{"untuk dia rela jalanin misi bunuh diri", "bunuh diri", false},
		{"teroris itu siap jalanin misi bunuh diri", "bunuh diri", false},
		{"aku lagi ngerjain misi bunuh diri di kantor", "bunuh diri", true},
	})
	// A context marker discards a match only in a short text with no risk
	// signal (see markersApply); the test text stands for the whole text.
	marker := func(fn func(s string, start, end int) bool) func(s string, start, end int) bool {
		return func(s string, start, end int) bool { return fn(s, start, end) && markersApply(s) }
	}
	run("relativeWill", relativeWill, []tc{
		{"kakek bikin surat wasiat untuk pembagian warisan", "surat wasiat", true},
		{"nenek ninggalin surat wasiat", "surat wasiat", true},
		{"kakek mau bikin surat wasiat untuk pembagian warisan", "surat wasiat", true},
		{"kakek bikin surat wasiat terakhirku", "surat wasiat", false},
		{"aku sama kakek bikin surat wasiat", "surat wasiat", false},
		{"gimana cara bikin surat wasiat yang sah", "surat wasiat", false},
	})
	run("legalWill", marker(legalWill), []tc{
		{"gimana cara bikin surat wasiat yang sah", "surat wasiat", true},
		{"notaris bantu bikin surat wasiat", "surat wasiat", true},
		{"cara bikin surat wasiat", "surat wasiat", true},
		// A risk signal anywhere keeps the match (review finding).
		{"lagi bikin surat wasiat yang sah, abis ini aku pamit", "surat wasiat", false},
		{"udah bikin surat wasiat yang sah buat keluarga, selamat tinggal", "surat wasiat", false},
		{"surat wasiat udah sah, besok aku pergi selamanya", "surat wasiat", false},
		{"cara bikin surat wasiat yang sah? soalnya aku mau pergi", "surat wasiat", false},
		{"ini surat wasiat terakhirku", "surat wasiat", false},
		{"surat wasiat terakhirku tolong bagi harta ke adik", "surat wasiat", false},
		{"aku nulis surat wasiat buat keluargaku", "surat wasiat", false},
		{"aku udah nulis surat wasiat yang sah buat keluargaku", "surat wasiat", false},
		{"selamat tinggal semuanya", "selamat tinggal semuanya", false},
	})
	run("selfHarmIdiom", selfHarmIdiom, []tc{
		{"i'm burning myself out", "burning myself", true},
		{"cut myself off from them", "cut myself", true},
		{"cutting myself some slack", "cutting myself", true},
		{"i cut myself a break", "cut myself", true},
		{"i burned myself out", "burned myself", true},
		// Accidents are a context marker, not an idiom (see accidental).
		{"i cut myself shaving", "cut myself", false},
		{"i cut myself", "cut myself", false},
		{"i hurt myself out of anger", "hurt myself", false},
		{"i cut myself on purpose", "cut myself", false},
		{"i cut myself by accident, and i cut myself again", "cut myself again", false},
		// Only "cut myself off" is the idiom (review finding).
		{"i want to hurt myself off and on", "hurt myself", false},
		{"i've been cutting myself off and on", "cutting myself", false},
	})
	run("accidental", marker(accidental), []tc{
		{"nggak sengaja ngiris tangan pas potong bawang", "ngiris tangan", true},
		{"i accidentally cut myself while cooking", "cut myself", true},
		{"i hurt myself playing football", "hurt myself", true},
		{"i cut myself shaving", "cut myself", true},
		{"i accidentally cut myself", "cut myself", true},
		{"i hurt myself at the gym", "hurt myself", true},
		{"i cut myself when i was cooking", "cut myself", true},
		{"i burned myself on the stove", "burned myself", true},
		{"i injured myself at practice", "injured myself", true},
		// Hitting oneself: furniture or a door, or an instrument with an
		// accident word (round-5 review finding).
		{"i hit myself on the cabinet door", "hit myself", true},
		{"i hit myself with a hammer by accident", "hit myself", true},
		{"i hit myself with a hammer", "hit myself", false},
		{"i hit myself on the edge", "hit myself", false},
		{"i hit myself on my bed", "hit myself", false},
		// Round-2 review regressions: a purpose, motive, feeling or habit
		// word anywhere in the text, after a comma, a clause word or a full
		// stop, keeps the match; so does a habitual verb form, a bare
		// "lagi" (again) after the injury, or a method word.
		{"aku ngiris tangan lagi bikin aku tenang", "ngiris tangan", false},
		{"aku iris tangan pas lagi masak, biar sakitnya pindah", "iris tangan", false},
		{"aku ngiris tangan pas potong bawang terus rasanya lega", "ngiris tangan", false},
		{"aku iris tanganku waktu cukur karena aku mau", "iris tangan", false},
		{"i cut myself while cooking because i wanted to", "cut myself", false},
		{"i cut myself while cooking to feel better", "cut myself", false},
		{"i cut myself shaving and i liked it", "cut myself", false},
		{"i cut myself shaving. i liked it.", "cut myself", false},
		{"i hurt myself at the gym so i would feel something", "hurt myself", false},
		{"i burn myself on the stove", "burn myself", false},
		{"i burned myself on the stove when i'm sad", "burned myself", false},
		{"i hurt myself running into traffic", "hurt myself", false},
		{"i cut myself by accident and i want to do it again", "cut myself", false},
		{"i hurt myself working out because i deserve it", "hurt myself", false},
		{"i'm cutting myself while cooking", "cutting myself", false},
		{"aku nyayat tangan pas masak", "nyayat tangan", false},
		{"aku sengaja nyayat tanganku", "nyayat tangan", false},
		// The review regressions: an intent, modal or habit word, a
		// self-directed method, a deliberate or negating word, or a marker
		// that is not next to the injury keeps the match.
		{"i want to hurt myself at the gym", "hurt myself", false},
		{"after working out i want to hurt myself", "hurt myself", false},
		{"i keep hurting myself when things aren't working out", "hurting myself", false},
		{"i want to cut myself with my shaving razor", "cut myself", false},
		{"i cut myself with my shaving razor", "cut myself", false},
		{"i didn't cut myself accidentally", "cut myself", false},
		{"i cut myself not accidentally", "cut myself", false},
		{"i'm going to slit my wrists with a shaving blade tonight", "slit my wrists", false},
		{"accidentally cut my wrists", "cut my wrists", false},
		{"pengen sayat nadi pas lagi masak", "sayat nadi", false},
		{"nggak sengaja nyayat pergelangan tangan", "nyayat pergelangan tangan", false},
		{"aku sering iris tangan pas lagi main hp sendirian", "iris tangan", false},
		{"aku iris tangan pas lagi main hp", "iris tangan", false},
		{"at the gym today i want to hurt myself", "hurt myself", false},
	})
	run("lethalIdiom", lethalIdiom, []tc{
		{"shooting myself in the foot", "shooting myself", true},
		{"i drown myself in work", "drown myself", true},
		{"i want to drown myself in the river", "drown myself", false},
		{"i want to shoot myself in the head", "shoot myself", false},
		{"i want to drown myself", "drown myself", false},
		{"i'd be shooting myself in the foot if i quit", "shooting myself", true},
		{"i drown myself in my sorrows", "drown myself", true},
		// Water of any kind, or an intent word, keeps the match (review
		// finding).
		{"i want to drown myself in the nearest river", "drown myself", false},
		{"i'm going to drown myself in cold water", "drown myself", false},
		{"i want to drown myself in work", "drown myself", false},
	})
	run("cuttingObject", cuttingObject, []tc{
		{"i've been cutting carbs", "been cutting", true},
		{"started cutting back on coffee", "started cutting", true},
		{"been cutting down on sugar", "been cutting", true},
		{"keep cutting corners", "keep cutting", true},
		{"been cutting class", "been cutting", true},
		{"been cutting my hair", "been cutting", true},
		{"keep cutting it close", "keep cutting", true},
		{"i've been cutting", "been cutting", false},
		{"been cutting my arms", "been cutting", false},
		{"been cutting again", "been cutting again", false},
		// Particles are idioms only in fixed phrases (review finding).
		{"started cutting back", "started cutting", false},
		{"i started cutting in high school", "started cutting", false},
		{"i've been cutting in secret", "been cutting", false},
		{"i started cutting up again", "started cutting", false},
		{"i've been cutting off and on for years", "been cutting", false},
	})
	run("asPrescribed", marker(asPrescribed), []tc{
		{"i took all my pills like the doctor said", "took all my pills", true},
		{"i took all my meds as prescribed", "took all my meds", true},
		{"i took all my pills", "took all my pills", false},
		{"i took all my meds this morning", "took all my meds", false},
		// A time is not a marker, and a risk signal anywhere keeps the
		// match (review finding).
		{"i took all my pills on time", "took all my pills", false},
		{"i took all my pills like the doctor said, all of them at once so i won't wake up", "took all my pills", false},
		{"i took all my pills as prescribed, then i swallowed the rest", "took all my pills", false},
		{"i took all my meds as prescribed. i'm done", "took all my meds", false},
		// Only "all my pills" is an adherence report (review sweep).
		{"i took a lot of pills like the doctor said", "took a lot of pills", false},
		{"i took a whole bottle of pills as prescribed", "took a whole bottle of pills", false},
		{"just took all of my meds as prescribed", "just took all of my meds", true},
	})
	run("dyeObject", dyeObject, []tc{
		{"i need to die my hair black", "i need to die", true},
		{"want to die it red", "want to die", true},
		{"i want to die my life is over", "want to die", false},
		{"i want to die", "want to die", false},
		{"she wants to die her hair", "wants to die", true},
		// A verb after the hair noun starts a new clause (review finding).
		{"i want to die my hair is falling out", "want to die", false},
		{"i want to die my hair looks so bad", "want to die", false},
	})
	run("vanishPlace", vanishPlace, []tc{
		{"pengen ngilang selamanya dari sosmed", "pengen ngilang selamanya", true},
		{"mau hilang selamanya dari hidupnya", "mau hilang selamanya", true},
		{"pengen ngilang selamanya dari dunia ini", "pengen ngilang selamanya", false},
		{"pengen ngilang selamanya dari hidup ini", "pengen ngilang selamanya", false},
		{"pengen ngilang selamanya", "pengen ngilang selamanya", false},
		// A time or people is not a place (review finding).
		{"pengen ngilang selamanya dari kemarin", "pengen ngilang selamanya", false},
		{"pengen ngilang selamanya dari kalian", "pengen ngilang selamanya", false},
	})
	run("ownEuphemism", ownEuphemism, []tc{
		{"aku pengen tidur selamanya", "pengen tidur selamanya", true},
		{"gua pengen tidur selamanya", "pengen tidur selamanya", false},
		{"pengen tidur selamanya", "pengen tidur selamanya", false},
		{"aku pgn tidur selamanya", "pgn tidur selamanya", false},
	})
	run("thingSubject", thingSubject, []tc{
		{"kucingku loncat dari atap", "loncat dari atap", true},
		{"kucingku mau loncat dari atap", "loncat dari atap", true},
		{"kucingku lompat dari jendela", "lompat dari jendela", true},
		// Only an animal can jump (review sweep).
		{"mati lampu mau loncat dari gedung", "loncat dari gedung", false},
		{"hp-ku mau loncat dari gedung", "loncat dari gedung", false},
		{"aku pengen loncat dari gedung", "loncat dari gedung", false},
		{"pengen lompat dari gedung", "lompat dari gedung", false},
	})
	run("circumstantialLifeEn", circumstantialLifeEn, []tc{
		{"tired of living like this", "tired of living", true},
		{"sick of life in the city", "sick of life", true},
		{"tired of life's drama", "tired of life", true},
		{"tired of living", "tired of living", false},
		{"so tired of life, i want to die", "tired of life", false},
		// Pain is not a circumstance (review finding).
		{"tired of living in pain", "tired of living", false},
		{"tired of living with this depression", "tired of living", false},
	})
	run("asPrescribed", marker(asPrescribed), []tc{
		{"i took all my pills on purpose like the doctor said", "took all my pills", false},
	})
	run("leisureJump", leisureJump, []tc{
		{"kami terjun dari tebing di bali seru banget", "terjun dari tebing", true},
		{"anak-anak lompat dari atap rumah ke kolam", "lompat dari atap", true},
		{"anak2 lompat dari atap ke kolam", "lompat dari atap", true},
		{"kami mau terjun dari tebing seru", "terjun dari tebing", false},
		{"aku lompat dari atap ke kolam", "lompat dari atap", false},
		{"terjun dari tebing seru", "terjun dari tebing", false},
	})
	run("mixedIdiom", mixedIdiom, []tc{
		{"mau die cast mobil mainan", "mau die", true},
		{"pengen die hard fans", "pengen die", true},
		{"pengen die aja", "pengen die", false},
		{"hp-ku mau die", "mau die", true},
		// A device cannot "unalive" (review sweep).
		{"mati lampu mau unalive aja", "mau unalive", false},
	})
	run("wakeRoutine", wakeRoutine, []tc{
		{"i hope i don't wake up late tomorrow", "hope i don't wake up", true},
		{"i hope i never wake up early again", "hope i never wake up", true},
		{"i hope i never wake up", "hope i never wake up", false},
		{"i hope i don't wake up tomorrow", "hope i don't wake up", false},
		// Only a whole routine phrase, with no risk or death word in the
		// sentence (round-3 review finding).
		{"i hope i don't wake up too early", "hope i don't wake up", true},
		{"i hope i don't wake up with a headache", "hope i don't wake up", true},
		{"i hope i don't wake up before my alarm", "hope i don't wake up", true},
		{"i hope i don't wake up at 3 am", "hope i don't wake up", true},
		{"i hope i don't wake up during the night", "hope i don't wake up", true},
		{"i hope i dont wake up at all", "hope i dont wake up", false},
		{"honestly i hope i don't wake up too", "hope i don't wake up", false},
		{"i hope i don't wake up early, or at all", "hope i don't wake up", false},
		{"i hope i never wake up with this pain again", "hope i never wake up", false},
		{"i hope i never wake up sick again", "hope i never wake up", false},
		{"i hope i don't wake up during the night, i want it to be over", "hope i don't wake up", false},
		{"i hope i never wake up before my alarm so i can die in my sleep", "hope i never wake up", false},
		{"i hope i don't wake up late lol bye forever", "hope i don't wake up", false},
		// The pattern is wide: the text after the sentence is read for a
		// risk or death word; "by" a time is a deadline and a year is not a
		// time (round-4 review finding).
		{"i hope i don't wake up late. or ever", "hope i don't wake up", false},
		{"i hope i don't wake up early. or at all", "hope i don't wake up", false},
		{"i hope i don't wake up during the night. i want it to be over", "hope i don't wake up", false},
		{"i hope i don't wake up early. anyway work was fine", "hope i don't wake up", true},
		{"i hope i don't wake up by morning", "hope i don't wake up", false},
		{"i hope i don't wake up until 2030", "hope i don't wake up", false},
		{"i hope i don't wake up until 7", "hope i don't wake up", true},
		{"i hope i never wake up before noon on weekends", "hope i never wake up", true},
	})
	// Ruleset -5 checks.
	run("hopeDeathIdiom", hopeDeathIdiom, []tc{
		{"i hope i die of old age", "hope i die", true},
		{"i wish i could die my hair blue", "wish i could die", true},
		{"i hope i die soon", "hope i die", false},
		{"i hope i'll die of old age soon", "hope i'll die", false},
		// Round-5 review findings: "the"/"this" or a time after "see" is
		// not the idiom, a risk word anywhere in the sentence keeps the
		// match, and the game, sleep-in, old-age and happiness idioms.
		{"i hope i'm not alive to see it", "hope i'm not alive", true},
		{"i hope i'm not alive to see the morning", "hope i'm not alive", false},
		{"i hope i'm not alive to see this", "hope i'm not alive", false},
		{"i hope i'm not alive to see it, i can't keep going", "hope i'm not alive", false},
		{"i hope i die first in the game so i can spectate", "hope i die", true},
		{"i hope i die in the next round of chemo", "hope i die", false},
		{"i hope i don't see the sunrise, i'm sleeping in all weekend", "hope i don't see the sunrise", true},
		{"i hope i'm dead asleep by 10", "hope i'm dead", true},
		{"i hope i get old and die peacefully", "hope i get old and die", true},
		{"i hope i get the job and die of happiness", "hope i get the job and die", true},
		{"i hope i don't make it to the meeting", "hope i don't make it", true},
		{"i hope i don't make it", "hope i don't make it", false},
		{"i hope i'm not around when you wake up", "hope i'm not around", false},
		{"i hope i don't make it home", "hope i don't make it", false},
	})
	run("lifeEndsHappy", lifeEndsHappy, []tc{
		{"semoga hidupku berakhir bahagia", "semoga hidupku berakhir", true},
		{"semoga hidupku cepet berakhir", "semoga hidupku cepet berakhir", false},
		{"i hope my life ends happily, i want to be old", "hope my life ends", false},
	})
	run("deathFormIdiom", deathFormIdiom, []tc{
		{"i'd love to be dead honest with you", "i'd love to be dead", true},
		{"i'd love to be dead", "i'd love to be dead", false},
	})
	run("hopeOfOthers", hopeOfOthers, []tc{
		{"they wish i would die", "wish i would die", true},
		{"mereka berharap aku mati", "berharap aku mati", true},
		{"ibuku berharap aku mati", "berharap aku mati", true},
		{"ibu aku berharap aku mati", "berharap aku mati", true},
		{"aku berharap mati", "berharap mati", false},
		{"i really hope i die", "hope i die", false},
		// A third party after a verb or a preposition is an object
		// (round-5 review finding).
		{"terus mereka berharap aku mati", "berharap aku mati", true},
		{"aku kangen ibu berharap aku meninggal", "berharap aku meninggal", false},
		{"aku capek sama mereka berharap aku mati", "berharap aku mati", false},
		{"i hate you wish i would die", "wish i would die", false},
		{"i can't stand my mom hoping i die", "hoping i die", false},
	})
	run("hopeTarget", hopeTarget, []tc{
		{"semoga cepet mati tuh orang", "semoga cepet mati", true},
		{"semoga besok mati lampu", "semoga besok mati", true},
		{"semoga aku cepet mati", "semoga aku cepet mati", false},
		{"semoga cepet mati aja", "semoga cepet mati", false},
		// The victim must end the clause (round-5 review finding).
		{"moga mampus si koruptor", "moga mampus", true},
		{"semoga cepet mati semuanya beres", "semoga cepet mati", false},
		{"semoga cepet meninggal dia pasti seneng", "semoga cepet meninggal", false},
		{"semoga cepet meninggal si aku yang nggak berguna ini", "semoga cepet meninggal", false},
		{"doain cepet meninggal ya kalian", "doain cepet meninggal", false},
		// A bare "itu" needs a person after it (round-6 review finding).
		{"semoga cepet mati itu aja", "semoga cepet mati", false},
		{"semoga cepet mati itu orang", "semoga cepet mati", true},
	})
	run("notAliveObject", notAliveObject, []tc{
		{"semoga aku nggak ada kelas besok", "semoga aku nggak ada", true},
		{"semoga aku nggak ada di kelas", "semoga aku nggak ada", true},
		// Only a listed everyday place, and not before "lagi" (round-6
		// review finding).
		{"semoga aku nggak ada di sini", "semoga aku nggak ada", false},
		{"semoga aku nggak ada di muka bumi ini lagi", "semoga aku nggak ada", false},
		{"semoga aku nggak ada di kelas lagi", "semoga aku nggak ada", false},
		{"semoga aku nggak hidup di tahun depan", "semoga aku nggak hidup", false},
		{"semoga besok aku udah nggak ada", "semoga besok aku udah nggak ada", false},
		{"semoga aku nggak ada lagi di dunia ini", "semoga aku nggak ada", false},
		{"semoga aku nggak ada di dunia ini", "semoga aku nggak ada", false},
		// Only a listed object or place rejects (round-5 review finding).
		{"semoga aku nggak hidup susah", "semoga aku nggak hidup", true},
		{"semoga besok aku udah nggak ada tapi ya gitu deh", "semoga besok aku udah nggak ada", false},
		{"semoga aku udah nggak ada soalnya capek", "semoga aku udah nggak ada", false},
		{"semoga aku udah gak ada idk", "semoga aku udah gak ada", false},
	})
	run("adverbDeathIdiom", adverbDeathIdiom, []tc{
		{"nenekku mau cepet meninggal", "mau cepet meninggal", true},
		{"baterai hp ku mau cepet mati", "mau cepet mati", true},
		{"aku mau cepet meninggal", "mau cepet meninggal", false},
		{"aku pengen banget mati", "pengen banget mati", false},
	})
	run("negatedIntent", negatedIntent, []tc{
		{"i don't want to take all my pills", "want to take all my pills", true},
		{"i'm not going to take all my meds", "going to take all my meds", true},
		{"aku nggak mau minum semua obatku", "mau minum semua obatku", true},
		{"i want to take all my pills", "want to take all my pills", false},
		// A contrastive conjunction that opens the next sentence hedges
		// the refusal (round-5 review finding; the pattern is wide).
		{"i don't want to take all my pills. but sometimes", "want to take all my pills", false},
		{"i don't want to take all my pills. my doctor says so. but", "want to take all my pills", true},
	})
	run("odQuestion", odQuestion, []tc{
		{"boleh minum obat sekaligus dengan vitamin", "minum obat sekaligus", true},
		{"boleh nggak minum obatnya sekaligus pagi aja", "minum obatnya sekaligus", true},
		{"boleh minum obat tidur sekaligus 10", "minum obat tidur sekaligus", false},
		{"aku minum obat sekaligus", "minum obat sekaligus", false},
	})
	run("bottleObject", bottleObject, []tc{
		{"i took the whole bottle of water", "took the whole bottle", true},
		{"i took the whole bottle", "took the whole bottle", false},
		{"i took the whole bottle of tylenol", "took the whole bottle", false},
		{"i'm going to take the whole bottle with me", "take the whole bottle", false},
	})
	run("shNotSlang", shNotSlang, []tc{
		{"pengen sh*t", "pengen sh", true},
		{"pengen sh t", "pengen sh", true},
		{"pengen sh lagi", "pengen sh", false},
	})
	run("dieOnHill", dieOnHill, []tc{
		{"please let me die on this hill", "please let me die", true},
		{"please let me die", "please let me die", false},
	})
	run("heartGerund", heartGerund, []tc{
		{"i hope my heart stops racing", "hope my heart stops", true},
		{"i hope my heart stops beating", "hope my heart stops", false},
		{"i hope my heart stops", "hope my heart stops", false},
	})
	run("takeMePlace", takeMePlace, []tc{
		{"god take me back to school", "god take me", true},
		{"god take me to heaven", "god take me", false},
		{"lord take me home", "lord take me", false},
		// Round-5 review finding: only an everyday destination rejects.
		{"god take me out to dinner", "god take me", true},
		{"lord take me higher", "lord take me", true},
		{"god take me out of this world", "god take me", false},
		{"lord take me there now", "lord take me", false},
		{"god take me to my family", "god take me", false},
		{"god take me somewhere i can rest", "god take me", false},
	})
	run("drownIdiomID", drownIdiomID, []tc{
		{"menenggelamkan diri dalam pekerjaan", "menenggelamkan diri", true},
		{"aku mau menenggelamkan diri dalam pekerjaan", "menenggelamkan diri", false},
		{"aku mau nenggelamin diri", "nenggelamin diri", false},
	})
	run("burnNews", burnNews, []tc{
		{"aksi bakar diri di depan gedung", "bakar diri", true},
		{"aku pengen bakar diri", "bakar diri", false},
	})
	// The context-marker gate is closed by default: every word outside the
	// marker patterns' matches must be benign (round-3 review finding).
	gate := map[string]bool{
		"i cut myself shaving":                                                true,
		"i cut myself shaving this morning":                                   true,
		"i accidentally cut myself while cooking, not a big deal":             true,
		"nggak sengaja ngiris tangan pas lagi potong bawang":                  true,
		"gimana cara bikin surat wasiat yang sah?":                            true,
		"i took all my pills like the doctor said":                            true,
		"i cut myself shaving, it helped":                                     false,
		"i cut myself shaving. it was worth it":                               false,
		"i cut myself shaving 😅":                                              false,
		"i 'accidentally' cut myself while cooking":                           false,
		"nggak sengaja ngiris tangan pas potong bawang, bikin nagih":          false,
		"nggak sengaja ngiris tangan lagi":                                    false,
		"i took all my pills like the doctor said, all 30 of them":            false,
		"i took all my pills like the doctor said lol bye":                    false,
		"lagi bikin surat wasiat yang sah, udah waktunya":                     false,
		"nggak sengaja ngiris tangan pas potong bawang, sebenernya disengaja": false,
		// A bracket or an emoticon reads as sarcasm (round-4 review finding).
		"i accidentally cut myself ;)":                 false,
		"i accidentally cut myself :-)":                false,
		"i (accidentally) cut myself while cooking":    false,
		"i hit myself on the cabinet door by accident": true,
	}
	for in, want := range gate {
		if got := markersApply(in); got != want {
			t.Errorf("markersApply(%q) = %v, want %v", in, got, want)
		}
	}
	run("circumstantialLife", circumstantialLife, []tc{
		{"capek hidup kayak gini terus", "capek hidup", true},
		{"capek hidup di jakarta", "capek hidup", true},
		{"capek idup", "capek idup", false},
		{"bosan hidup rasanya", "bosan hidup", false},
	})
}
