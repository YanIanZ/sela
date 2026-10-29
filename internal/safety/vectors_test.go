package safety

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// vector is one entry of testdata/vectors.json. Text, Want, Langs, Tags and
// Source are required (Tags may be empty for a plain coverage case);
// Category (the expected first category, from v1's matchedDomain assertions)
// and Note (why the level was chosen) are optional.
type vector struct {
	Text     string   `json:"text"`
	Want     Level    `json:"want"`
	Langs    []string `json:"langs"`
	Tags     []string `json:"tags"`
	Source   string   `json:"source"`
	Category Category `json:"category,omitempty"`
	Note     string   `json:"note,omitempty"`
}

var (
	vectorTags = []string{"negation", "quote", "slang", "codeswitch", "ambiguous", "idiom",
		"third_party", "denial", "normalisation", "v1"}
	vectorLangs  = []string{"id", "en", "mixed"}
	vectorSource = regexp.MustCompile(`^(?:v1:(?:crisisEngine|crisisRegression|crisisEnglish|crisisCoverageGaps)\.test\.mjs|new)$`)
)

func loadVectors(t testing.TB) []vector {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var vs []vector
	if err := dec.Decode(&vs); err != nil {
		t.Fatalf("vectors.json: %v", err)
	}
	if len(vs) == 0 {
		t.Fatal("vectors.json is empty")
	}
	return vs
}

func TestVectorsSchema(t *testing.T) {
	seen := map[string]int{}
	for i, v := range loadVectors(t) {
		if j, dup := seen[v.Text]; dup {
			t.Errorf("vector %d duplicates vector %d", i, j)
		}
		seen[v.Text] = i
		if _, ok := levelRank[v.Want]; !ok {
			t.Errorf("vector %d: invalid want %q", i, v.Want)
		}
		if len(v.Langs) == 0 {
			t.Errorf("vector %d: langs must not be empty", i)
		}
		for _, l := range v.Langs {
			if !slices.Contains(vectorLangs, l) {
				t.Errorf("vector %d: invalid lang %q", i, l)
			}
		}
		for _, tag := range v.Tags {
			if !slices.Contains(vectorTags, tag) {
				t.Errorf("vector %d: invalid tag %q", i, tag)
			}
		}
		if !vectorSource.MatchString(v.Source) {
			t.Errorf("vector %d: invalid source %q", i, v.Source)
		}
		if isV1 := strings.HasPrefix(v.Source, "v1:"); isV1 != slices.Contains(v.Tags, "v1") {
			t.Errorf("vector %d: the v1 tag must be set exactly for v1 sources", i)
		}
		if v.Category != "" && !slices.Contains([]Category{CategorySuicide, CategorySelfHarm, CategoryViolence, CategoryHopelessness}, v.Category) {
			t.Errorf("vector %d: invalid category %q", i, v.Category)
		}
	}
}

// TestVectors runs every vector through Assess.
func TestVectors(t *testing.T) {
	for i, v := range loadVectors(t) {
		t.Run(fmt.Sprintf("%03d", i), func(t *testing.T) {
			a := Assess(v.Text)
			if a.Level != v.Want {
				t.Fatalf("Assess(%q).Level = %s, want %s (rules %v; %s)", v.Text, a.Level, v.Want, a.RuleIDs, v.Note)
			}
			if v.Category != "" && (len(a.Categories) == 0 || a.Categories[0] != v.Category) {
				t.Errorf("Assess(%q).Categories = %v, want first %s", v.Text, a.Categories, v.Category)
			}
			if a.Ruleset != RulesetVersion {
				t.Errorf("Ruleset = %q", a.Ruleset)
			}
			if (a.Level == LevelNone) != (len(a.RuleIDs) == 0) {
				t.Errorf("Assess(%q): level %s with rules %v", v.Text, a.Level, a.RuleIDs)
			}
			if a.ThirdParty && a.Level != LevelElevated {
				t.Errorf("Assess(%q): ThirdParty with level %s", v.Text, a.Level)
			}
			tagged := func(tag string) bool { return slices.Contains(v.Tags, tag) }
			if tagged("third_party") && v.Want == LevelElevated && !a.ThirdParty {
				t.Errorf("Assess(%q).ThirdParty = false, want true", v.Text)
			}
			if tagged("denial") && v.Want == LevelNone && !a.Denial {
				t.Errorf("Assess(%q).Denial = false, want true", v.Text)
			}
		})
	}
}

// TestVectorCounts reports the corpus composition (run with -v to see it)
// and keeps every tag represented.
func TestVectorCounts(t *testing.T) {
	vs := loadVectors(t)
	count := func(key func(v vector) []string) map[string]int {
		m := map[string]int{}
		for _, v := range vs {
			for _, k := range key(v) {
				m[k]++
			}
		}
		return m
	}
	byTag := count(func(v vector) []string { return v.Tags })
	byWant := count(func(v vector) []string { return []string{string(v.Want)} })
	byLang := count(func(v vector) []string { return v.Langs })
	bySource := count(func(v vector) []string { return []string{v.Source} })
	t.Logf("vectors: %d", len(vs))
	for _, group := range []struct {
		name string
		m    map[string]int
	}{{"tag", byTag}, {"want", byWant}, {"lang", byLang}, {"source", bySource}} {
		keys := make([]string, 0, len(group.m))
		for k := range group.m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%d", k, group.m[k]))
		}
		t.Logf("by %s: %s", group.name, strings.Join(parts, " "))
	}
	for _, tag := range vectorTags {
		if byTag[tag] < 3 {
			t.Errorf("tag %s has %d vectors, want at least 3", tag, byTag[tag])
		}
	}
	for _, lvl := range []Level{LevelNone, LevelElevated, LevelCrisis} {
		if byWant[string(lvl)] == 0 {
			t.Errorf("no vectors want %s", lvl)
		}
	}
}

// v1NonInputs are string literals in the v1 test files that are not inputs to
// the engine: titles, messages, module paths, ids, expected outputs and
// arguments other than the text.
var v1NonInputs = []string{
	"../../apps/web/src/lib/api/chatHistory.ts", "../../apps/web/src/lib/api/crisisGate.ts",
	"../../apps/web/src/lib/crisisGuard.ts", "../../apps/web/src/lib/voiceRoom.ts",
	"../../services/crisis-engine/src/crisisDetector.ts", "node:assert/strict", "node:test",
	"/crisis", "/resources", "15-17", "18-24", "25-34", "en", "en-",
	"user", "assistant", "system", "developer", "text_evaluated",
	"crisis", "high", "none", "suicide", "self_harm", "extreme_hopelessness",
	"healing119-kemenkes", "lisa-helpline", "tepsa-kemensos",
	"baygon", "bundir", "mati", "belajar", "perlindungan remaja",
	"Mau ngobrol juga dengan orang terlatih?", "Want to talk to a trained person too?",
	"Minors aged 15-17 must receive TePSA/KPAI contact", "Must provide emergency contacts",
	"Must provide intervention copy",
	"Age-Adapted Resource Escalation", "Crisis coverage gaps (EN + ID)",
	"Deterministic Crisis Engine Tests", "Empty and Malformed Input Handling",
	"English crisis coverage (same engine, same gates)",
	"English crisis text reaches every gate the Indonesian text reaches",
	"Explicit Crisis Phrases (High-Risk Language)",
	"False-Positive-Sensitive Phrases (Must NOT Trigger Crisis)",
	"Indonesian Distress Language (Non-Crisis)", "Text Normalization & Anti-Evasion",
	"Zero-LLM Hard Boundary Verification",
	"catches leetspeak, spacing and apostrophe variants",
	"catches mixed Indonesian and English in one message",
	"chat history screen halts on English crisis in any position",
	"collapses repeated elongated characters", "deterministic crisis regression corpus",
	"executes synchronously in under 5 milliseconds with zero network requests",
	"exempts only whole-sentence English denial or a clearly third-person story",
	"finds crisis content in any user history position with harmless current input",
	"finds crisis history before rejecting an empty current message",
	"gives English intervention copy only when lang is en; contacts stay identical",
	"keeps assistant context as untrusted user-role text and rejects fake system roles",
	"keeps the Indonesian corpus unchanged (no weakening)",
	"normalizes leetspeak numbers to alphabet",
	"provides adult crisis hotlines (Healing119 / LISA) for age bracket 18-24",
	"provides minor/teen hotlines (TePSA/KPAI) for age bracket 15-17",
	"returns non-crisis for empty string",
	"returns non-crisis for null/undefined/malformed input without throwing",
	"returns non-crisis for whitespace string",
	"safely handles empty, null, undefined, and non-string inputs",
	"server crisis gate blocks English crisis and localises the high-distress pathway",
	"the explicit forms of passive ideation are not lowered by the new high rules",
	"untrusted chat history boundary",
	"voice room final-transcript screen and inline guard treat English like Indonesian",
}

var v1TestFiles = []string{"crisisEngine", "crisisRegression", "crisisEnglish", "crisisCoverageGaps"}

// TestVectorsCoverV1 checks, against the v1 test files themselves, that every
// string literal they contain is either ported as a vector or listed in
// v1NonInputs, and that every v1-sourced vector really comes from the file it
// names. (v1 also passed null, undefined, 12345 and {}; Go's string API makes
// those inputs impossible.)
func TestVectorsCoverV1(t *testing.T) {
	texts := map[string]vector{}
	for _, v := range loadVectors(t) {
		texts[v.Text] = v
	}
	nonInput := map[string]bool{}
	for _, s := range v1NonInputs {
		nonInput[s] = true
	}
	literals := map[string]map[string]bool{}
	for _, name := range v1TestFiles {
		path := filepath.Join("testdata", "v1", name+".test.mjs")
		src, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			t.Skipf("v1 test file %s is not present", path)
		}
		if err != nil {
			t.Fatal(err)
		}
		literals[name] = map[string]bool{}
		for _, lit := range jsStringLiterals(string(src)) {
			literals[name][lit] = true
			if _, ok := texts[lit]; !ok && !nonInput[lit] {
				t.Errorf("%s: string %q is neither a vector nor a known non-input", name, lit)
			}
		}
	}
	for text, v := range texts {
		name, ok := strings.CutPrefix(v.Source, "v1:")
		if !ok {
			continue
		}
		if !literals[strings.TrimSuffix(name, ".test.mjs")][text] {
			t.Errorf("vector %q claims %s but is not a string literal there", text, v.Source)
		}
	}
}

// jsStringLiterals returns the single- and double-quoted string literals of
// a JavaScript source, decoding \uXXXX and simple escapes. Comments and
// template literals are skipped.
func jsStringLiterals(src string) []string {
	var out []string
	for i := 0; i < len(src); {
		switch {
		case strings.HasPrefix(src[i:], "//"):
			j := strings.IndexByte(src[i:], '\n')
			if j < 0 {
				return out
			}
			i += j
		case strings.HasPrefix(src[i:], "/*"):
			j := strings.Index(src[i+2:], "*/")
			if j < 0 {
				return out
			}
			i += j + 4
		case src[i] == '`':
			j := i + 1
			for j < len(src) && src[j] != '`' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			i = j + 1
		case src[i] == '\'' || src[i] == '"':
			q := src[i]
			var b strings.Builder
			j := i + 1
			for j < len(src) && src[j] != q {
				if src[j] == '\\' && j+1 < len(src) {
					switch e := src[j+1]; e {
					case 'u':
						var r rune
						fmt.Sscanf(src[j+2:j+6], "%04x", &r)
						b.WriteRune(r)
						j += 6
					case 'n':
						b.WriteByte('\n')
						j += 2
					default:
						b.WriteByte(e)
						j += 2
					}
					continue
				}
				b.WriteByte(src[j])
				j++
			}
			out = append(out, b.String())
			i = j + 1
		default:
			i++
		}
	}
	return out
}
