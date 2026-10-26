// Package safety is Sela's deterministic safety router. It runs before every
// model call and before every ordinary handoff transition (SPEC V3).
//
// It is a rule-based lexicon for Indonesian, English and code-switched text,
// ported from the inherited Dengar.in crisis engine
// (services/crisis-engine/src/crisisDetector.ts). It is not a clinical risk
// assessment and makes no safety promise: it decides which product path is
// shown. Ambiguous input takes the safer path.
//
// The package has no network, model or logging dependency. Assessment values
// carry rule identifiers only, never the input text, so callers may log them.
//
// # Matching
//
// Assess normalises the text (see Normalize) and matches the rule catalogue
// against the canonical form and two derived variants (dash-joined and
// double-letter, see variants), keeping the strongest outcome per rule. Every
// v1 rule is ported with its v1 id; v1 severity 'crisis' maps to LevelCrisis
// and 'high' to LevelElevated. Go's regexp is RE2 and has no lookarounds; the
// two v1 negative lookaheads (passive-ideation and en-end-everything) and the
// new idiom checks are written as post-match functions documented on each
// rule. Rules run per sentence ('.', '!', '?'), which finds exactly what a
// whole-text search finds, behind a literal prefilter derived from each
// expression.
//
// # Routing policy
//
// Matches are then judged in context. Ambiguity takes the safer path.
//
//  1. Clauses. The text is split into clauses at . ! ? ; and newlines and at
//     the conjunctions tapi, tetapi, tp, tpi, namun, padahal, melainkan,
//     sedangkan, but, however, although and whereas (never inside quotation
//     marks), and clauses into segments at commas. Context (denial, third
//     party) is decided per segment or clause, while matching also sees the
//     whole text, so a phrase that spans clauses ("aku mau\nmati") counts.
//     The anchored v1 pattern "^kill me$" is tried on the whole text and on
//     each clause.
//  2. Denial. A segment that is an explicit whole-clause denial in v1's
//     contextualDiscussion shape ("aku tidak mau bunuh diri", "I don't want
//     to kill myself"; widened to colloquial pronouns and negators, trailing
//     particles such as "kok", and "I'm not suicidal") does not count, and
//     Denial is set. Only that segment is exempt: "Aku tidak mau bunuh diri
//     tapi aku ingin mengakhiri hidupku malam ini" is crisis. A denial that
//     is immediately followed by a contrastive conjunction is hedged ("aku
//     tidak mau bunuh diri, tapi kadang kepikiran") and counts as elevated.
//     As in v1 the whole text is also checked as one denial, so the words of
//     a denial may be separated by newlines.
//  3. Third party. Risk language attributed to someone else, with no
//     first-person risk, routes to LevelElevated with ThirdParty set. v1
//     returned none for its media frames; this is an intentional change, so
//     concern about someone else still surfaces human-support information.
//     Attribution comes from v1's media frames ("dalam film itu tokoh ...",
//     "in the movie, the character ...", widened to more media and
//     subjects), reporting frames at the start of a segment ("temanku bilang
//     ...", "kata dia, ...", "my friend said ..."), quotations with a
//     third-party reporting verb before or after them, direct speech after a
//     colon ("temanku bilang: ..."), and third-person wording ("killed
//     himself", "wants to die"). A frame is void when its reported words
//     contain a first-person word outside quotation marks ("temanku bilang
//     aku mau mati" is crisis). When the speaker adopts the language anywhere
//     ("aku juga mau mati", "aku juga", "me too", "same here", "I relate", or
//     "juga"/"too" inside the reported words) attributed matches count in
//     full. A bare third-person subject without a frame ("temanku mau bunuh
//     diri") is not attributed and stays crisis.
//  4. Study or prevention context is not recognised: "aku sedang belajar
//     tentang pencegahan bunuh diri" stays crisis exactly as in v1. This is a
//     known, deliberate conservative false positive.
//  5. Everyday idioms stay none ("mati lampu", "mati gaya", "hp-ku mati",
//     "this joke is killing me", "I'm dying to see that movie", "I could kill
//     for a coffee"). Where a v1 rule matched an unambiguous idiom, a
//     documented post-check rejects it: a device, power or plant subject
//     before "mati" ("baterai hp-ku mau mati"), the adverb "mati-matian", and
//     a number before "kms" ("5 kms"). Ambiguous hyperbole keeps its v1 level
//     ("rasanya mau mati aja kalau disuruh presentasi" is crisis).
//
// # Known limitations
//
// This is a lexicon, not a classifier. It has no acute-violence rules (the
// category exists but, as in v1, nothing uses it), does not recognise
// personal names as third-party subjects, treats advice to others ("jangan
// bunuh diri ya") and some reassurances outside the denial shape ("bukan
// berarti aku mau bunuh diri") as the speaker's own words, and keeps v1's
// conservative matches such as "goodbye everyone" and "i have a rope".
// testdata/vectors.json records each of these decisions.
package safety

// RulesetVersion identifies the lexicon and routing policy. Change it whenever
// a rule, normalisation step or routing decision changes.
const RulesetVersion = "sela-safety-2026.10.26-1"

// Level is the routing decision.
type Level string

const (
	// LevelNone: continue the ordinary journey.
	LevelNone Level = "none"
	// LevelElevated: continue, but show human-support information alongside
	// (distress without explicit intent, concern about someone else, quoted
	// risk language, ambiguous hyperbole).
	LevelElevated Level = "elevated"
	// LevelCrisis: interrupt. No model call, no handoff, no follow-up plan.
	LevelCrisis Level = "crisis"
)

// Category groups rules for display and evaluation.
type Category string

const (
	CategorySuicide      Category = "suicide"
	CategorySelfHarm     Category = "self_harm"
	CategoryViolence     Category = "acute_violence"
	CategoryHopelessness Category = "extreme_hopelessness"
)

// Assessment is the result of Assess. It never contains the input text.
type Assessment struct {
	Level Level `json:"level"`
	// RuleIDs lists every matched rule id in catalogue order.
	RuleIDs []string `json:"ruleIds,omitempty"`
	// Categories lists the distinct categories of matched rules.
	Categories []Category `json:"categories,omitempty"`
	// ThirdParty is true when the only risk language was attributed to
	// someone else (a friend, a film character, a quotation).
	ThirdParty bool `json:"thirdParty,omitempty"`
	// Denial is true when a clause was recognised as an explicit whole-clause
	// denial ("aku tidak mau bunuh diri") and therefore did not count.
	Denial bool `json:"denial,omitempty"`
	// Ruleset is RulesetVersion at evaluation time.
	Ruleset string `json:"ruleset"`
}

// Interrupts reports whether the ordinary journey must stop.
func (a Assessment) Interrupts() bool { return a.Level == LevelCrisis }

// Max returns the more severe of two assessments, merging rule ids.
func Max(a, b Assessment) Assessment {
	rank := map[Level]int{LevelNone: 0, LevelElevated: 1, LevelCrisis: 2}
	out := a
	if rank[b.Level] > rank[a.Level] {
		out.Level = b.Level
	}
	out.RuleIDs = mergeStrings(a.RuleIDs, b.RuleIDs)
	out.Categories = mergeCategories(a.Categories, b.Categories)
	out.ThirdParty = (a.ThirdParty || a.Level == LevelNone) && (b.ThirdParty || b.Level == LevelNone) && (a.ThirdParty || b.ThirdParty)
	out.Denial = a.Denial || b.Denial
	if out.Ruleset == "" {
		out.Ruleset = RulesetVersion
	}
	return out
}

func mergeStrings(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range append(append([]string{}, a...), b...) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func mergeCategories(a, b []Category) []Category {
	if len(b) == 0 {
		return a
	}
	seen := make(map[Category]bool, len(a)+len(b))
	out := make([]Category, 0, len(a)+len(b))
	for _, c := range append(append([]Category{}, a...), b...) {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}
