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
// against the canonical form and three derived variants (dash-joined,
// double-letter, and a free-standing "1" read as "i"; see variants), keeping
// the strongest outcome per rule. Every v1 rule is ported with its v1 id; v1
// severity 'crisis' maps to LevelCrisis and 'high' to LevelElevated. Go's
// regexp is RE2 and has no lookarounds; the two v1 negative lookaheads
// (passive-ideation and en-end-everything) and the new idiom checks are
// written as post-match functions documented on each rule. Rules run per
// sentence ('.', '!', '?'), which finds exactly what a whole-text search
// finds, behind a literal prefilter derived from each expression.
//
// Assess runs in time linear in the length of the input: an adversarial
// 8000-character text takes about 10ms and a realistic one about 3ms in a
// normal build. It has no input cap of its own, because truncating would drop
// risk language; callers bound request sizes.
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
//     is followed by a contrastive conjunction is hedged and counts as
//     elevated, whether or not anything follows the conjunction ("aku tidak
//     mau bunuh diri, tapi kadang kepikiran", "I don't want to die, but...").
//     As in v1 the whole text is also checked as one denial, so the words of
//     a denial may be separated by newlines.
//  3. Third party. Risk language clearly attributed to someone else, with no
//     first-person risk, routes to LevelElevated with ThirdParty set (see
//     Third-party attribution). v1 returned none for its media frames; this
//     is an intentional change, so concern about someone else still surfaces
//     human-support information. When the speaker adopts the language
//     anywhere ("aku juga mau mati", "aku juga", "me too", "same here", "I
//     relate", or "juga"/"too" inside the reported words) each attributed
//     match counts in full. A bare third-person subject without a frame
//     ("temanku mau bunuh diri") is not attributed and stays crisis.
//  4. Study or prevention context is not recognised: "aku sedang belajar
//     tentang pencegahan bunuh diri" stays crisis exactly as in v1. This is a
//     known, deliberate conservative false positive.
//  5. Everyday idioms stay none ("mati lampu", "mati gaya", "hp-ku mati",
//     "this joke is killing me", "I'm dying to see that movie", "I could kill
//     for a coffee"). Where a v1 rule matched an unambiguous idiom, a
//     documented post-check rejects it: a device, power or plant subject
//     before "mau mati" or "mati aja" ("baterai hp-ku mau mati"; a wish such
//     as "pengen mati" is never a device's), the adverb "mati-matian", and a
//     number before "kms" ("5 kms"). Ambiguous hyperbole keeps its v1 level
//     ("rasanya mau mati aja kalau disuruh presentasi" is crisis).
//
// # Third-party attribution
//
// Text is attributed to someone else by:
//
//   - Media frames for recorded or published content: v1's "dalam film itu
//     tokoh ..." and "in the movie, the character ...", widened to more media
//     and subjects. Classes and lectures are events the speaker attends and
//     do not frame.
//   - Reporting frames at the start of a segment: "temanku bilang ...", "kata
//     dia, ...", "katanya ...", "my friend said ...". "kata" and "menurut"
//     need a third-party subject, since "kata hatiku" and "kata terakhirku"
//     are the speaker's own. The reported content runs to the end of the
//     segment, and on through following segments that open with a
//     third-person pronoun up to the next frame. It stops at a breaker word
//     that starts the speaker's own reaction in subjectless Indonesian
//     ("bikin", "rasanya", "sekarang", "terus", "jadi": "Pacarku bilang putus
//     rasanya pengen mati" is crisis) unless a third-person pronoun is
//     attached ("terus dia mau mati").
//   - Quotations with a third-party reporting frame right before them, or an
//     attribution right after them that no first-person word follows before
//     the next comma ("\"...\", she said"). A first-person reporting frame
//     before a quotation ("aku cerita \"...\"", "I told her \"...\"") makes
//     it the speaker's own words whatever follows. A quotation never spans a
//     newline or a sentence boundary, so a stray quote mark cannot turn the
//     speaker's sentences into a quotation.
//   - Direct speech after a colon ("temanku bilang: ..."), up to the end of
//     its segment or a first-person reply ("aku jawab ...", "I replied ...").
//   - Third-person wording ("killed himself", "she wants to die"). The
//     speaker's inner self is not a third person ("part of me wants to die"
//     is crisis) and a device is an idiom ("my laptop wants to die").
//
// Reported content is void when it contains a first-person word outside
// quotation marks, including any -ku form other than a relative or role
// ("temanku bilang aku mau mati" and "kata dia, hidupku ..." are crisis).
//
// # Known limitations
//
// This is a lexicon, not a classifier. It has no acute-violence rules (the
// category exists but, as in v1, nothing uses it), does not recognise
// personal names as third-party subjects, treats advice to others ("jangan
// bunuh diri ya") and some reassurances outside the denial shape ("bukan
// berarti aku mau bunuh diri") as the speaker's own words, and keeps v1's
// conservative matches such as "goodbye everyone" and "i have a rope". Some
// third-party reports take the safer path because their extent is ambiguous:
// unquoted direct speech past its first segment ("temanku bilang: aku capek,
// aku mau mati"), quotations of more than one sentence, subjectless content
// after a breaker word, and "<device> aku mau mati" in the middle of a clause
// ("di kosan hp aku mau mati"). testdata/vectors.json records each of these
// decisions.
package safety

// RulesetVersion identifies the lexicon and routing policy. Change it whenever
// a rule, normalisation step or routing decision changes.
const RulesetVersion = "sela-safety-2026.10.26-2"

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
