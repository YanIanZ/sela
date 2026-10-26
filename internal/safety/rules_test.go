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
			if p.anchored && p.reject != nil {
				t.Errorf("%s: anchored patterns cannot have post-checks", r.id)
			}
			if p.maximal && (p.anchored || p.reject != nil) {
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
	"en-explicit-suicide":   true, // \bkms branch → precededByNumber
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
	})
	run("selfOrThing", selfOrThing, []tc{
		{"part of me wants to die", "wants to die", true},
		{"everything in me still wants to die", "wants to die", true},
		{"my soul wants to die", "wants to die", true},
		{"my whole body wants to die", "wants to die", true},
		{"my laptop wants to die", "wants to die", true},
		{"my phone battery literally wants to die", "wants to die", true},
		{"my friend wants to die", "wants to die", false},
		{"she wants to die", "wants to die", false},
		{"his body wants to die", "wants to die", false},
		{"wants to die", "wants to die", false},
	})
	run("precededByNumber", precededByNumber, []tc{
		{"i am 5 kms from home", "kms", true},
		{"kms", "kms", false},
		{"honestly kms", "kms", false},
	})
	run("circumstantialLife", circumstantialLife, []tc{
		{"capek hidup kayak gini terus", "capek hidup", true},
		{"capek hidup di jakarta", "capek hidup", true},
		{"capek idup", "capek idup", false},
		{"bosan hidup rasanya", "bosan hidup", false},
	})
}
