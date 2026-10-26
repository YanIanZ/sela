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
// against the canonical form and up to six derived variants (see variants):
// dash-joined, double-letter, a free-standing "1" read as "i", a lexical
// reading (every elongated word resolved against the catalogue vocabulary,
// two-letter elongation inside a word included; fused spaced-out words
// split; symbols, emoji, emphasis marks, hesitation dots and spaced dashes
// between words read as spaces; contractions and common misspellings spelt
// out), its dash-joined form, and an alternative leetspeak reading (@ $ ! |
// 7 +, and each 1 or | read as i or l, whichever spells a vocabulary word:
// "k1ll", "$u1c1d3", "m@t1"). Variants only ever add matches: each is
// matched in its own context and the strongest outcome per rule is kept.
// Adoption (see Routing policy) is folded over all variants, so "my friend
// said she wants to die. me tooooo" counts although only one variant spells
// "me too". Every v1 rule is ported with its v1 id; v1 severity 'crisis' maps
// to LevelCrisis and 'high' to LevelElevated. Go's regexp is RE2 and has no
// lookarounds; the two v1 negative lookaheads (passive-ideation and
// en-end-everything) and the idiom checks are written as post-match functions
// documented on each rule (most of them in checks.go). Rules run per sentence
// ('.', '!', '?'), which finds exactly what a whole-text search finds, behind
// a literal prefilter derived from each expression.
//
// Assess runs in time linear in the length of the input: an adversarial
// 8000-character text takes up to about 20ms and a realistic one about 3ms
// in a normal build (TestPerformance bounds both at 200ms, TestLinearTime
// checks the growth). It has no input cap of its own, because truncating
// would drop risk language; callers bound request sizes.
//
// # Routing policy
//
// Matches are then judged in context. Ambiguity takes the safer path, and a
// false-positive fix never costs a first-person false negative.
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
//     tapi aku ingin mengakhiri hidupku malam ini" is crisis, and passive
//     ideation after a denial ("I'm not suicidal, just tired of living") is
//     elevated. A denial that is followed by a contrastive conjunction is
//     hedged and counts as elevated, whether or not anything follows the
//     conjunction ("aku tidak mau bunuh diri, tapi kadang kepikiran", "I
//     don't want to die, but..."). As in v1 the whole text is also checked as
//     one denial, so the words of a denial may be separated by newlines.
//  3. Third party. Risk language clearly attributed to someone else, with no
//     first-person risk, routes to LevelElevated with ThirdParty set (see
//     Third-party attribution). v1 returned none for its media frames; this
//     is an intentional change, so concern about someone else still surfaces
//     human-support information. When the speaker adopts the language
//     anywhere, each attributed match counts in full: "aku juga", "aku pun",
//     "aku sama", "sama kayak aku", "me too", "so am I", "same here", "omg
//     same", "literally me", "I relate", an imitation of the reported act
//     ("pengen nyusul", "aku mau ikut", "aku pengen kayak dia", "kok aku iri",
//     "I want to try it"), "juga"/"too" inside the reported words, or "too"
//     after third-person wish or intent wording ("she wants to die too"; not
//     after a past act: "my uncle killed himself too" stays elevated). A bare
//     third-person subject without a frame ("temanku mau bunuh diri") is not
//     attributed and stays crisis.
//  4. Study or prevention context is not recognised: "aku sedang belajar
//     tentang pencegahan bunuh diri" stays crisis exactly as in v1. This is a
//     known, deliberate conservative false positive.
//  5. Everyday idioms and benign uses stay none, but only when no intent,
//     plan or self-directed method is expressed. Where a rule matches one, a
//     documented post-check rejects it. A fixed idiom is a set phrase that
//     completes the match and has no self-harm reading: a device, power,
//     plant or animal subject ("baterai hp-ku mau mati", "kucingku mau
//     mati"), an idiom noun after "mati" ("mau mati lampu", "mati gaya",
//     "mati rasa" at the end of its phrase), "mati-matian", "bunuh
//     diri"/"suicide" inside a compound ("gol bunuh diri", "career suicide",
//     "Suicide Squad"; a news compound such as "bom bunuh diri" is
//     attributed), "burning myself out", "cut myself off from", "shooting
//     myself in the foot", "drown myself in work", "cutting carbs", "cutting
//     back on", "die" spelt for "dye" ("I need to die my hair black"),
//     vanishing forever from social media or someone's life, and English
//     compounds such as "die cast". A context marker is a word near the match
//     that suggests a benign reading: an accident right next to an injury
//     that can happen by accident ("I cut myself shaving", "nggak sengaja
//     ngiris tangan pas potong bawang"), medication taken as prescribed ("I
//     took all my pills like the doctor said"), "kms" after a number or a
//     quantity word ("5 kms", "a few kms") or as the child health card ("KMS
//     anakku", near "posyandu"), a legal will ("cara bikin surat wasiat yang
//     sah"), and a leisure jump by a group ("kami terjun dari tebing di bali
//     seru banget"). A context marker never rejects a match whose phrase
//     carries an intent, modal or habit word ("I want to hurt myself at the
//     gym", "aku sering iris tangan pas lagi main hp"; for "kms", one that
//     governs it: "i want to kms", while "aku mau lari 5 kms" is a distance)
//     or a deliberate or negating word ("I cut myself not accidentally"), and
//     the accident check never applies to wrists, veins, nadi, an arm or a
//     neck. Ambiguous hyperbole keeps its v1 level ("rasanya mau mati aja
//     kalau disuruh presentasi" is crisis).
//
// # Third-party attribution
//
// Text is attributed to someone else by:
//
//   - Media frames for recorded or published content: v1's "dalam film itu
//     tokoh ..." and "in the movie, the character ...", widened to more media
//     and subjects, and the speaker reading or watching a report about
//     someone else ("aku baca berita ada mahasiswa ...", "I read a news
//     article about a student ..."). Classes and lectures are events the
//     speaker attends and do not frame. In a media frame the report ends at a
//     subjectless wish after the risk phrase ("aku baca berita ada mahasiswa
//     bunuh diri pengen nyusul": the wish is the speaker's).
//   - Reporting frames at the start of a segment: "temanku bilang ...", "kata
//     dia, ...", "katanya ...", "my friend said ...". "kata" and "menurut"
//     need a third-party subject, since "kata hatiku" and "kata terakhirku"
//     are the speaker's own. The reported content runs to the end of the
//     segment, and on through following segments that open with a
//     third-person pronoun up to the next frame. It stops at a breaker word
//     that starts the speaker's own reaction in subjectless Indonesian
//     ("bikin", "rasanya", "sekarang", "terus", "jadi": "Pacarku bilang putus
//     rasanya pengen mati" is crisis) unless a third-person pronoun is
//     attached ("terus dia mau mati"). Content that opens with a closed
//     report, a quotation (after at most three words such as "sambil
//     marah") or an anaphoric "gitu"/"gt"/"begitu"/"itu" ("Mama bilang
//     \"sabar ya\" pengen bunuh diri", "Nyokap ngomong gitu pengen mati
//     aja"), ends there unless a third-person pronoun follows.
//   - Quotations with a third-party reporting frame right before them (a
//     song's lyrics included: "the lyrics say \"...\""), or an attribution
//     right after them that no first-person word follows before the next
//     comma ("\"...\", she said"). A first-person reporting frame before a
//     quotation ("aku cerita \"...\"", "I told her \"...\"") makes it the
//     speaker's own words whatever follows. A quotation never spans a newline
//     or a sentence boundary, and never ends with a reporting frame after a
//     comma, so a stray quote mark cannot turn the speaker's sentences or
//     clauses into a quotation.
//   - Direct speech after a colon ("temanku bilang: ...", "lirik lagunya:
//     ..."), up to the end of its segment, a first-person reply ("aku jawab
//     ...", "I replied ...") or the start of the speaker's own words after
//     its opening words: a later first-person subject ("Mama bilang: makan
//     dulu aku mau mati"), or, unless the speech opens with the quoted
//     person's own "aku", a breaker or a subjectless wish ("makan dulu
//     rasanya pengen mati", "stop crying wanna die").
//   - Third-person wording ("killed himself", "she wants to die"). The
//     speaker's inner self, or a subject that includes the speaker, is not a
//     third person ("part of me", "something inside me", "the real me", "my
//     inner child", "you and me": crisis), while an object "me" is ("someone
//     close to me wants to die" is elevated). A device or a plant is an idiom
//     ("my laptop wants to die", "the plant next to me wants to die").
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
// ("di kosan hp aku mau mati"). Adoption words that are also ordinary words
// ("aku juga", "aku pun", "aku sama", "mau ikut", "me to", "I'm the same")
// count only at the end of their segment or before a listed follower (a
// wish, a feeling word or a particle), so "aku juga nonton beritanya" is not
// adoption. The post-checks see the normalised, lowercased phrase around a
// match only (at most 120 bytes each way, never past clause punctuation or a
// clause word): capital "KMS" cannot be told from slang without a card
// or health-post word, a bare time does not mark medication adherence ("I
// took all my meds this morning" is crisis), and a leisure jump is only
// rejected for a group or third-party subject ("aku lompat dari atap ke
// kolam" is crisis). The lexical variant resolves elongation and splits
// fused words against the catalogue vocabulary only, and bare "mati" in any
// spelling is not a risk phrase ("m@t1", as v1 "m4t1"). testdata/vectors.json
// records each of these decisions.
package safety

// RulesetVersion identifies the lexicon and routing policy. Change it whenever
// a rule, normalisation step or routing decision changes.
const RulesetVersion = "sela-safety-2026.10.26-3"

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
