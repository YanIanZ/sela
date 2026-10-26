package safety

import (
	"regexp"
	"strings"
)

// rule is one catalogue entry. A rule matches when any of its patterns has an
// accepted match.
type rule struct {
	id       string
	category Category
	// level is the routing level when the match counts in full. v1 severity
	// 'crisis' maps to LevelCrisis and v1 'high' to LevelElevated.
	level Level
	// origin is "v1" for rules ported from crisisDetector.ts and "sela" for
	// rules added in this package.
	origin string
	// thirdPerson marks rules whose wording is itself about someone else
	// ("killed himself"). Their matches are always attributed to a third party
	// (LevelElevated) unless the speaker adopts them.
	thirdPerson bool
	patterns    []*pattern
}

// pattern is one RE2 expression plus the code that replaces what RE2 cannot
// express.
type pattern struct {
	re *regexp.Regexp
	// anchored patterns are matched against the whole text and against each
	// clause (they use ^ and $); all other patterns start with \b and are
	// matched per sentence.
	anchored bool
	// reject, when set, inspects an RE2 match s[start:end] of the sentence s
	// and reports that it must be discarded. It re-expresses v1 lookarounds
	// and the documented idiom checks; it looks at s only, never beyond the
	// sentence, unless the pattern is wide.
	reject func(s string, start, end int) bool
	// wide patterns are matched per sentence like the others, but their
	// checks receive the sentence followed by up to checkWindow bytes of the
	// text after it, so that a check can read an elliptical continuation in
	// the next sentence ("I hope I don't wake up late. or ever"; see
	// wakeRoutine). contextUnits keys a unit that may hold a wide match with
	// the text that follows it.
	wide bool
	// third, when set, reports that an accepted match names someone else's
	// act ("bom bunuh diri", "suicide bomber"): it is attributed to a third
	// party like third-person wording. Same contract as reject.
	third func(s string, start, end int) bool
	// marker, when set, reports a context marker next to an accepted match
	// that suggests a benign reading ("I cut myself shaving", "like the
	// doctor said", "surat wasiat yang sah"). Unlike reject, it discards the
	// match only when the whole text is short, carries no risk signal and
	// has no word outside a closed benign vocabulary (see markersApply).
	// Same contract as reject.
	marker func(s string, start, end int) bool
	// maximal patterns are evaluated on their first match per sentence only.
	// The expression must be such that this match contains every other match
	// in the sentence (TestMaximalPatterns) and that no match can lie inside
	// a denial (the denial shape has none of its words). Every scope that
	// contains the first match then contains the others, so dispose judges
	// no other match more severely. It keeps a pattern with an unbounded gap
	// linear in the sentence length.
	maximal bool
	// filter is the derived prefilter (nil: none) and atoms its strongest
	// flat atom set, used to find candidate sentences (see prefilter.go).
	filter *filter
	atoms  []string
	// win bounds the length of a match, so that a long sentence is matched
	// only in windows around the atoms (see window.go).
	win windowInfo
	// word is set when the expression is one literal word between word
	// boundaries (`\bsh\b`, `\bkms\b`): it is then found by a string search
	// instead of the regexp (see pattern.find), so a text made of many such
	// words does not pay a regexp call per match (review finding).
	word string
}

// literalWordRe recognises an expression that is one literal word between
// word boundaries.
var literalWordRe = regexp.MustCompile(`^\\b([a-z]+)\\b$`)

func pat(expr string) *pattern {
	f := compileFilter(expr)
	p := &pattern{re: regexp.MustCompile(expr), filter: f, atoms: f.flatAtoms(), win: newWindowInfo(expr)}
	if m := literalWordRe.FindStringSubmatch(expr); m != nil {
		p.word = m[1]
	}
	return p
}

func patReject(expr string, reject func(s string, start, end int) bool) *pattern {
	p := pat(expr)
	p.reject = reject
	return p
}

// patCheck is a pattern with a post-check and an attribution check.
func patCheck(expr string, reject, third func(s string, start, end int) bool) *pattern {
	p := patReject(expr, reject)
	p.third = third
	return p
}

// patMarker is a pattern with an optional post-check and a context marker.
func patMarker(expr string, reject, marker func(s string, start, end int) bool) *pattern {
	p := patReject(expr, reject)
	p.marker = marker
	return p
}

// patWideReject is a wide pattern with a post-check (see pattern.wide).
func patWideReject(expr string, reject func(s string, start, end int) bool) *pattern {
	p := patReject(expr, reject)
	p.wide = true
	return p
}

func patAnchored(expr string) *pattern {
	p := pat(expr)
	p.anchored = true
	return p
}

func patMaximal(expr string) *pattern {
	p := pat(expr)
	p.maximal = true
	return p
}

func v1Rule(id string, c Category, l Level, ps ...*pattern) rule {
	return rule{id: id, category: c, level: l, origin: "v1", patterns: ps}
}

func newRule(id string, c Category, l Level, ps ...*pattern) rule {
	return rule{id: id, category: c, level: l, origin: "sela", patterns: ps}
}

func thirdPersonRule(id string, c Category, ps ...*pattern) rule {
	r := newRule(id, c, LevelCrisis, ps...)
	r.thirdPerson = true
	return r
}

// catalogue is the ordered rule list. Every v1 rule comes first, in v1 order
// and with its v1 id, so the first category matches v1's matchedDomain. Rules
// added by Sela follow. Ids are stable: never rename or reorder an entry, and
// bump RulesetVersion whenever the catalogue changes.
var catalogue = []rule{
	// ---- v1: Indonesian, explicit suicidal intent and slang ----
	// Post-checks (new, documented): "bunuh diri" inside a fixed compound
	// is not the speaker's suicide ("gol bunuh diri", "misi bunuh diri",
	// "bunuh diri karier"), and a news compound is someone else's act ("bom
	// bunuh diri", attributed; see suicideCompound).
	v1Rule("explicit-suicide-phrase", CategorySuicide, LevelCrisis,
		patCheck(`\b(bunuh\s*diri|bundir|akhiri\s*hidup(?:ku|nya)?|mengakhiri\s*hidup(?:ku|nya)?)\b`, suicideCompound, suicideNews)),
	// Post-check (new, documented idiom decision): a device, plant or animal
	// subject ("baterai hp-ku mau mati", "kucingku mau mati"), an idiom noun
	// after "mati" ("mau mati lampu", "mati gaya", "mati rasa"), a device after
	// "mati aja" ("mati aja tv-nya") and the adverb "mati-matian" are not
	// death wishes (see idiomaticDeath).
	v1Rule("death-wish-phrase", CategorySuicide, LevelCrisis,
		patReject(`\b(pengen\s*mati|mau\s*mati|ingin\s*mati|mati\s*aja|mending\s*mati|lebih\s*baik\s*mati)\b`, idiomaticDeath)),
	v1Rule("no-will-to-live", CategorySuicide, LevelCrisis,
		pat(`\b(g(a|ak|k)\s*mau\s*hidup\s*lagi|tidak\s*mau\s*hidup\s*lagi|capek\s*hidup\s*pengen\s*ngilang\s*selamanya)\b`)),
	// Post-check (new): a relative's will ("kakek bikin surat wasiat untuk
	// pembagian warisan") is not the speaker's farewell note (see
	// relativeWill). Context marker (new): a legal will ("cara bikin surat
	// wasiat yang sah") is not one either when the rest of the text is
	// benign (see legalWill and markersApply).
	v1Rule("farewell-will-phrase", CategoryHopelessness, LevelCrisis,
		patMarker(`\b(surat\s*wasiat|pamit\s*selamanya|selamat\s*tinggal\s*semuanya|titip\s*pesan\s*terakhir|dunia\s*lebih\s*baik\s*tanpa(ku|aku))\b`, relativeWill, legalWill)),

	// ---- v1: Indonesian, acute self-harm and lethal means ----
	// Context marker (new): the cutting rules discard a sliced hand that an
	// accident marker right next to it explains ("nggak sengaja ngiris
	// tangan pas potong bawang") when the rest of the text is benign (see
	// markersApply); never
	// nadi, urat, lengan, leher or a wrist, nor "sayat"/"silet" (see
	// accidental).
	v1Rule("self-cutting", CategorySelfHarm, LevelCrisis,
		patMarker(`\b(sayat|nyayat|iris)\s*(tangan|nadi|lengan|urat|leher)\b`, nil, accidental)),
	v1Rule("self-harm-intent", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:menyakiti|melukai|mencelakai)\s*diri(?:ku| sendiri)?\b`)),
	v1Rule("first-person-euphemism", CategorySuicide, LevelCrisis,
		pat(`\b(?:aku|saya|gue|gw)\s+(?:ingin|mau|pengen|akan)\s+(?:tidur\s*selamanya|menghilang\s*selamanya|ga\s*bangun\s*lagi)\b`)),
	v1Rule("hanging-phrase", CategorySelfHarm, LevelCrisis,
		pat(`\b(gantung\s*diri|menggantung\s*diri)\b`)),
	v1Rule("ingestion-poison", CategorySelfHarm, LevelCrisis,
		pat(`\b(minum\s*(baygon|racun|pestisida|deterjen|cairan\s*pemutih))\b`)),
	// Post-check (new): an animal or device subject ("kucingku loncat dari
	// atap") is not the speaker (see thingSubject).
	v1Rule("jumping-phrases", CategorySuicide, LevelCrisis,
		patReject(`\b(loncat\s*dari\s*(gedung|jembatan|lantai|atap)|nabrakin\s*diri\s*ke\s*(kereta|truk|mobil))\b`, thingSubject)),

	// ---- v1: coverage gaps found in manual testing ----
	v1Rule("end-everything-phrase", CategorySuicide, LevelCrisis,
		pat(`\b(?:ingin|mau|pengen|akan)?\s*(?:mengakhiri|akhiri|menyudahi|sudahi|bunuh\s*diri)\s*(?:semuanya|semua\s*ini|segalanya)\b`)),
	v1Rule("overdose-intent", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:(?:mau|ingin|pengen|akan|udah|sudah|berencana)\s*overdosis|overdosis\s*obat|minum\s*obat\s*(?:sebanyak\s*(?:mungkin|mungkin\s*biar)|banyak\s*banget\s*biar|segenggam|sekaligus\s*(?:semua|sebotol))|nelen\s*obat\s*(?:semua|sebotol|segenggam))\b`)),
	v1Rule("self-cutting-affixed", CategorySelfHarm, LevelCrisis,
		patMarker(`\b(?:menyayat|nyayat|sayat|mengiris|ngiris|iris|nyilet|menyilet|silet)\s*(?:pergelangan\s*)?(?:tangan|nadi|lengan|urat|leher|paha)(?:ku|nya)?\b`, nil, accidental)),
	v1Rule("life-not-worth-living", CategorySuicide, LevelCrisis,
		pat(`\b(?:hidup(?:ku)?\s*(?:ini\s*)?(?:tidak|gak|ga|nggak|enggak)\s*layak\s*(?:dijalani|untuk\s*dijalani)|(?:gak|ga|nggak|tidak)\s*ada\s*gunanya\s*(?:aku\s*)?hidup|buat\s*apa\s*(?:aku\s*)?hidup\s*lagi)\b`)),

	// ---- v1: high distress (v1 'high' → LevelElevated) ----
	v1Rule("cannot-bear-anymore", CategoryHopelessness, LevelElevated,
		pat(`\b(?:(?:g(?:a|ak|k)|nggak|enggak|tidak|udah\s*(?:gak|ga|nggak|tidak))\s*(?:kuat|sanggup|tahan)\s*lagi|bunuh\s*(?:aku|saya|gue|gw)\s*(?:aja|saja)?)\b`)),
	v1Rule("severe-exhaustion-pain", CategoryHopelessness, LevelElevated,
		pat(`\b(sakit\s*banget\s*jiwaku|hancur\s*lebur|g(a|ak|k)\s*kuat\s*lagi\s*nanggung\s*hidup|teror\s*pinjol\s*bikin\s*gila)\b`)),
	// v1 passive-ideation (SCRUM-44). The v1 expression is split in three: the
	// "tanpa aku" branch needs no check; the two "nggak ada"/"nggak pernah
	// lahir" branches carried the lookahead (?!\s+(?:di|dalam)\s+(?!dunia)),
	// re-expressed by placeNotWorld: a trailing place ("nggak ada di rapat
	// itu") rejects the match, "di dunia" does not.
	v1Rule("passive-ideation", CategoryHopelessness, LevelElevated,
		pat(`\b(?:semua\s*orang|orang[\s-]*orang|mereka|keluarga(?:ku)?|teman[\s-]*teman(?:ku)?|semuanya)\s*(?:akan\s*|bakal\s*|pasti\s*|mungkin\s*)?lebih\s*(?:baik|tenang|bahagia|senang)\s*tanpa\s*(?:ada\s*)?(?:aku|saya|gue|gw|diriku)\b`),
		patReject(`\b(?:semua\s*orang|orang[\s-]*orang|mereka|keluarga(?:ku)?|teman[\s-]*teman(?:ku)?|semuanya)\s*(?:akan\s*|bakal\s*|pasti\s*|mungkin\s*)?lebih\s*(?:baik|tenang|bahagia|senang)\s*(?:kalau|jika)\s*(?:aku|saya|gue|gw)\s*(?:nggak|ngga|gak|ga|tidak|tak|enggak)\s*ada\b`, placeNotWorld),
		patReject(`\b(?:lebih\s*baik|mending|andai(?:kan)?|coba)\s*(?:kalau\s*|jika\s*)?(?:aku|saya|gue|gw)\s*(?:nggak|ngga|gak|ga|tidak|tak|enggak)\s*(?:pernah\s*)?(?:ada|lahir|dilahirkan)\b`, placeNotWorld)),
	v1Rule("en-passive-ideation", CategoryHopelessness, LevelElevated,
		pat(`\b(?:wish\s*(?:that\s*)?i\s*(?:could\s*)?(?:just\s*)?(?:disappear|vanish)\s*(?:forever|for\s*good|completely)|wish\s*(?:that\s*)?i\s*(?:didn'?t|did\s*not|never)\s*exist(?:ed)?|(?:everyone|everybody|they|my\s*family|people)\s*(?:would|will)\s*be\s*(?:happier|better\s*off|relieved)\s*(?:without\s*me|if\s*i\s*(?:was|were)\s*gone|if\s*i\s*(?:wasn'?t|weren'?t)\s*(?:here|around)))\b`)),

	// ---- v1: English coverage ----
	// v1 en-explicit-suicide. The "kms" branch is a separate pattern so that a
	// distance ("5 kms from home", "a few kms away") or the child health card
	// KMS ("bawa KMS ke posyandu") rejects the match (new, documented
	// false-positive fix; see kmsNotSlang). "suicide" inside a fixed compound
	// is rejected ("Suicide Squad", "career suicide", "suicide sprints") and a
	// news compound is attributed ("suicide bomber"; see suicideCompound).
	v1Rule("en-explicit-suicide", CategorySuicide, LevelCrisis,
		patCheck(`\b(?:kill(?:ing)?\s*myself|suicid(?:e|al)|take\s*my\s*(?:own\s*)?life|end(?:ing)?\s*my\s*(?:own\s*)?life|end\s*it\s*all|unalive\s*myself|off\s*myself)\b`, suicideCompound, suicideNews),
		patReject(`\bkms\b`, kmsNotSlang)),
	// Post-check (new): "die" spelt for "dye" ("I want to die my hair red") is
	// rejected (see dyeObject).
	v1Rule("en-death-wish", CategorySuicide, LevelCrisis,
		patReject(`\b(?:(?:want|wanna|wish|wished|wanted|hope|hoping)\s*(?:to\s*)?(?:be\s*)?(?:die|dead|to\s*die)|wish\s*i\s*(?:was|were|had)\s*(?:dead|never\s*born)|better\s*off\s*dead|rather\s*be\s*dead|(?:don'?t|do\s*not)\s*(?:want|wanna)\s*to\s*(?:live|be\s*alive|exist|wake\s*up)(?:\s*anymore)?|no\s*reason\s*to\s*(?:live|go\s*on|keep\s*going)|can'?t\s*go\s*on\s*(?:living|anymore)|ready\s*to\s*die)\b`, dyeObject)),
	// Post-checks (new): fixed idioms ("burning myself out", "cut myself off
	// from", "cutting myself some slack") are rejected (see selfHarmIdiom).
	// Context marker (new): an explicit past accident ("I cut myself
	// shaving", "I accidentally cut myself", "I hurt myself at the gym") is
	// discarded when the rest of the text is benign (see accidental and
	// markersApply); wrists, veins and arms never are.
	v1Rule("en-self-harm", CategorySelfHarm, LevelCrisis,
		patMarker(`\b(?:(?:hurt(?:ing)?|harm(?:ing)?|cut(?:ting)?|burn(?:ing)?|injur(?:e|ing)|punish(?:ing)?)\s*myself|self[\s-]*harm(?:ing)?|self[\s-]*injur(?:y|ing)|slit(?:ting)?\s*my\s*wrists?|cut(?:ting)?\s*my\s*(?:wrists?|arms?|veins?|thighs?))\b`, selfHarmIdiom, accidental)),
	// Post-checks (new): "shooting myself in the foot" and "drown myself in
	// work" are idioms when no intent, modal or habit word comes with them;
	// drowning in water of any kind is not (see lethalIdiom).
	v1Rule("en-lethal-means", CategorySelfHarm, LevelCrisis,
		patReject(`\b(?:hang(?:ing)?\s*myself|(?:take|took|taking)\s*(?:an\s*)?overdose|overdos(?:e|ing)\s*(?:on\s*)?(?:pills|meds|medication|myself)|od\s*on\s*(?:pills|meds)|jump(?:ing)?\s*(?:off|from)\s*(?:a|the|this)?\s*(?:bridge|building|roof|window|balcony|cliff)|drink(?:ing)?\s*(?:bleach|poison)|swallow(?:ing)?\s*(?:all\s*)?(?:the|my)?\s*pills|shoot(?:ing)?\s*myself|drown(?:ing)?\s*myself|step(?:ping)?\s*in\s*front\s*of\s*(?:a|the)\s*(?:train|truck|car|bus))\b`, lethalIdiom)),
	v1Rule("en-farewell-note", CategoryHopelessness, LevelCrisis,
		pat(`\b(?:goodbye\s*(?:everyone|forever|world|all)|this\s*is\s*(?:my\s*)?(?:goodbye|last\s*message|final\s*message|suicide\s*note)|(?:writing|wrote|written)\s*(?:my\s*)?(?:suicide|goodbye)\s*(?:note|letter)|(?:world|everyone|everybody|they)\s*(?:is|are|would\s*be|will\s*be)\s*better\s*(?:off\s*)?without\s*me|(?:want|wanna|wish|hope|like)\s*to\s*(?:sleep\s*forever|disappear\s*forever|never\s*wake\s*up|not\s*wake\s*up))\b`)),
	// v1 en-end-everything. The "done with life" branch carried the lookahead
	// (?!\s*admin); it is a separate pattern whose match is rejected by
	// followedByAdmin ("I am done with life admin today"). The
	// "hopeless ... want to end it" branch has an unbounded gap; it is a
	// separate maximal pattern (its first match runs from the first "hopeless"
	// to the last "want to end it" of the sentence), so repeated "hopeless"
	// does not make matching quadratic.
	v1Rule("en-end-everything", CategorySuicide, LevelCrisis,
		pat(`\b(?:end(?:ing)?\s*(?:it|everything|things|my\s*pain\s*for\s*good)\s*(?:all|tonight|today|now|for\s*good|soon)|(?:want|wanna|going\s*to|gonna|plan(?:ning)?\s*to|ready\s*to|about\s*to|decided\s*to)\s*end\s*everything|(?:want|wanna)\s*to\s*stop\s*(?:existing|living|being\s*alive)|(?:don'?t|do\s*not|can'?t)\s*see\s*(?:the|any)\s*point\s*(?:in|of)\s*(?:living|life|being\s*alive|going\s*on|staying\s*alive)|life\s*(?:is\s*)?not\s*worth\s*living|(?:nobody|no\s*one|no-one)\s*would\s*(?:miss|care\s*about|notice)\s*(?:me\s*)?if\s*i\s*(?:was|were|died|disappeared|(?:was|were)\s*gone|killed\s*myself))\b`),
		patMaximal(`\bhopeless[^.!?]*\b(?:want|wanna)\s*to\s*end\s*it\b`),
		patReject(`\b(?:i'?m|im|i\s*am)\s*(?:(?:so|just|really|honestly|literally|completely|totally|kinda|truly)\s*)*done\s*with\s*(?:life|living|being\s*alive)\b`, followedByAdmin)),
	// Context marker (new): taking medication as prescribed ("I took all my
	// pills like the doctor said") is discarded when the rest of the text is
	// benign (see asPrescribed and markersApply). A bare time ("this morning", "on
	// time") is not enough: overdose disclosures carry one too, so those
	// stay crisis.
	v1Rule("en-overdose-means", CategorySelfHarm, LevelCrisis,
		patMarker(`\b(?:(?:plan(?:ning)?\s*to|going\s*to|gonna|want\s*to|wanna|will|about\s*to|ready\s*to|trying\s*to)\s*(?:overdose|od\b)|(?:took|taken|swallowed|just\s*took)\s*(?:a\s*lot\s*of|all\s*(?:of\s*)?(?:my|the)|too\s*many|a\s*bunch\s*of|so\s*many|a\s*whole\s*(?:bottle|pack)\s*of|handfuls?\s*of)\s*(?:pills|meds|tablets|medication|medicine|sleeping\s*pills|painkillers)|(?:bought|got|have|found|made|tied)\s*(?:a\s*)?(?:rope|noose)\b|\bnoose\b|(?:stockpil\w*|saving\s*up|hoarding)\s*(?:my\s*)?(?:pills|meds|tablets|medication))\b`, nil, asPrescribed)),
	// Post-check (new): "cutting" completed by an everyday object or a fixed
	// idiom ("been cutting carbs", "started cutting back on coffee", "keep
	// cutting corners", "cutting in line") is not self-harm; "cutting in high
	// school", "cutting up again" and "cutting off and on" are (see
	// cuttingObject).
	v1Rule("en-self-harm-ongoing", CategorySelfHarm, LevelCrisis,
		patReject(`\b(?:(?:been|started|keep|kept|still)\s*(?:self[\s-]*)?(?:cutting|burning\s*my\s*skin)(?:\s*again)?|cutting\s*again|relapsed\s*(?:on|into|with)\s*(?:cutting|self[\s-]*harm))\b`, cuttingObject)),
	// v1 en-ambiguous-distress. The anchored "^kill me$" branch is matched
	// against the whole text (v1) and against each clause.
	v1Rule("en-ambiguous-distress", CategoryHopelessness, LevelElevated,
		patAnchored(`^kill\s*me[\s.!]*$`),
		pat(`\b(?:(?:please|someone|somebody|just)\s*kill\s*me|i\s*(?:just\s*)?want\s*(?:it|this|everything)\s*to\s*(?:stop|end)|(?:i\s*)?can'?t\s*go\s*on|(?:want|wanna)\s*to\s*end\s*it)\b`)),
	v1Rule("en-severe-distress", CategoryHopelessness, LevelElevated,
		pat(`\b(?:can'?t\s*(?:take|handle|do|bear)\s*(?:this|it|life)\s*anymore|(?:i|i'?ve)\s*give(?:n)?\s*up\s*on\s*(?:life|everything|living)|nothing\s*matters\s*anymore|(?:i|i'?m|i\s*am|i\s*feel)\s*(?:completely\s*|totally\s*)?hopeless|no\s*way\s*out|i\s*hate\s*my\s*life|life\s*is\s*(?:pointless|not\s*worth\s*(?:it|living)))\b`)),

	// ---- Sela additions: Indonesian slang and colloquial spellings ----
	// Slang spellings of "want to die" (pgn, pingin, kepengen, mw) and the
	// synonyms "modar", "mokat" and "mampus" ("pengen mampus aja", "mending
	// mampus"). Same idiom post-check as death-wish-phrase.
	newRule("id-death-wish-slang", CategorySuicide, LevelCrisis,
		patReject(`\b(?:pgn|pngn|pingin|pengin|kepengen|kepingin|mw)\s*mati\b|\b(?:pgn|pngn|pingin|pengin|kepengen|kepingin|mw|pengen|mau|ingin|mending|mendingan)\s*(?:modar|mokat|mampus)\b`, idiomaticDeath)),
	// "ga mau idup lagi", "nggak pengen hidup lagi". v1 no-will-to-live misses
	// "nggak"/"enggak" (its \bg cannot start inside "nggak") and "idup".
	newRule("id-no-will-to-live-slang", CategorySuicide, LevelCrisis,
		pat(`\b(?:ga|gak|gk|nggak|ngga|ngak|enggak|engga|tidak|tdk|tak|ndak)\s*(?:mau|mw|pengen|pgn|pingin|ingin|kepengen)\s*h?idup\s*(?:lagi|lg)\b`)),
	// "pengen ngilang aja selamanya": vanishing *forever* is treated like v1's
	// "aku mau menghilang selamanya" (crisis) without requiring a pronoun.
	// Vanishing forever from a place other than the world ("dari sosmed",
	// "dari grup itu", "dari hidupnya") is rejected (see vanishPlace).
	newRule("id-vanish-forever", CategorySuicide, LevelCrisis,
		patReject(`\b(?:ingin|mau|mw|pengen|pgn|pingin|pengin|kepengen|akan)\s*(?:menghilang|ngilang|hilang|lenyap)\s*(?:(?:aja|saja)\s*|dari\s*(?:dunia|bumi)\s*(?:ini\s*)?)*(?:untuk\s*)?selamanya\b`, vanishPlace)),
	// "rasanya mau hilang dari dunia ini": vanishing from the world without
	// "forever" is ambiguous (escape vs. not existing) and routes to elevated,
	// like v1 passive-ideation "lebih baik aku nggak ada di dunia ini".
	newRule("id-vanish-from-world", CategoryHopelessness, LevelElevated,
		pat(`\b(?:ingin|mau|mw|pengen|pgn|pingin|pengin|kepengen)\s*(?:menghilang|ngilang|hilang|lenyap)\s*(?:(?:aja|saja)\s*)?dari\s*(?:dunia|bumi)\b`)),
	// "capek idup", "bosan hidup": ambiguous, so elevated. A circumstantial
	// continuation ("capek hidup kayak gini terus", v1 benign) is rejected.
	newRule("id-tired-of-living", CategoryHopelessness, LevelElevated,
		patReject(`\b(?:capek|cape|capai|lelah|letih|bosen|bosan|muak)\s*h?idup\b`, circumstantialLife)),
	// "ngga kuat lagi", "gk sanggup lg": spellings v1 cannot-bear-anymore misses.
	newRule("id-cannot-bear-slang", CategoryHopelessness, LevelElevated,
		pat(`\b(?:ga|gak|gk|nggak|ngga|ngak|enggak|engga|tidak|tdk|tak|udh\s*(?:ga|gak|gk|nggak|tdk))\s*(?:kuat|sanggup|tahan)\s*(?:lagi|lg)\b`)),

	// "lebih baik aku mati", "aku pantas mati": v1 death-wish-phrase only
	// covers "lebih baik mati"/"mending mati" with nothing in between.
	newRule("id-better-dead", CategorySuicide, LevelCrisis,
		patReject(`\b(?:lebih\s*baik|mending(?:an)?|sebaiknya|harusnya|seharusnya)\s*(?:aku|saya|gue|gw|gua|ku)\s*(?:mati|meninggal|mampus|modar)\b`, idiomaticDeath),
		patReject(`\b(?:aku|saya|gue|gw|gua)\s*(?:pantas|layak|harus(?:nya)?|seharusnya)\s*(?:mati|dibunuh)\b`, idiomaticDeath)),

	// ---- Sela additions: English slang and modal death wishes ----
	// "I should just die", "I deserve to die": not covered by v1.
	// "die" spelt for "dye" is rejected ("I need to die my hair black").
	newRule("en-death-wish-modal", CategorySuicide, LevelCrisis,
		patReject(`\bi\s*(?:should|deserve\s*to|ought\s*to|need\s*to|might\s*as\s*well|may\s*as\s*well)\s*(?:just\s*)?(?:die|be\s*dead)\b`, dyeObject)),
	newRule("en-slang-suicide", CategorySuicide, LevelCrisis,
		pat(`\b(?:sewer\s*slide|(?:want|wanna|gonna|going\s*to|plan(?:ning)?\s*to|thinking\s*(?:about|of)|think\s*about|ready\s*to|about\s*to)\s*(?:to\s*)?unaliv(?:e|ing)|unaliv(?:e|ed|ing)\s*my\s*self|(?:end|ended|ending)\s*my\s*self|(?:wanting|wishing|hoping)\s*to\s*(?:die|be\s*dead))\b`)),

	// ---- Sela additions: third-person wording (always attributed) ----
	// "my friend wants to kill herself", "the character killed himself". The
	// level applies only if the speaker adopts it ("me too"); otherwise the
	// match is third-party and routes to elevated.
	// The "wants to die" branch has no subject in the match; selfOrThing
	// rejects it when the subject is the speaker's inner self ("part of me",
	// "something inside me", "my soul": counted by en-inner-death-wish
	// instead, through the same innerSubject check), a device ("my laptop
	// wants to die": an idiom) or when "die" is spelt for "dye".
	thirdPersonRule("en-third-person-suicide", CategorySuicide,
		pat(`\b(?:kill(?:s|ed|ing)?\s*(?:him|her|them)\s*sel(?:f|ves)|(?:end|ends|ended|ending)\s*(?:his|her|their)\s*(?:own\s*)?li(?:fe|ves)|(?:take|takes|took|taken|taking)\s*(?:his|her|their)\s*own\s*li(?:fe|ves))\b`),
		patReject(`\bwants\s*to\s*(?:die|be\s*dead|end\s*it\s*all)\b`, selfOrThing)),
	thirdPersonRule("en-third-person-self-harm", CategorySelfHarm,
		pat(`\b(?:hurt|hurts|hurting|harm|harms|harmed|harming|cut|cuts|cutting|burn|burns|burned|burnt|burning|hang|hangs|hanged|hanging|drown|drowns|drowned|drowning|shoot|shoots|shooting|starve|starves|starving)\s*(?:him|her|them)\s*sel(?:f|ves)\b`)),

	// ---- Sela additions, ruleset 2026.10.26-2 ----
	// "part of me wants to die", "my soul wants to be dead": the speaker's
	// own death wish phrased through an inner self (v1 none). The second
	// pattern counts every "wants to die" whose subject innerSubject reads as
	// the speaker's inner self ("something inside me", "the real me", "my
	// inner child"), which is exactly what selfOrThing takes away from
	// en-third-person-suicide.
	newRule("en-inner-death-wish", CategorySuicide, LevelCrisis,
		patReject(`\bwants?\s*to\s*(?:die|be\s*dead|end\s*it\s*all|stop\s*existing|stop\s*living)\b`, notInnerSubject),
		pat(`\b(?:part\s*of\s*me|everything\s*in\s*me|something\s*in\s*me|all\s*of\s*me|(?:the\s*)?rest\s*of\s*me|my\s*(?:whole\s*)?(?:heart|soul|body|mind|brain|spirit|self|being))\s*(?:(?:just|really|still|kinda|sometimes|literally|honestly|also|actually|lowkey|always|truly|secretly|now|only)\s*)*(?:wants?|wanted)\s*to\s*(?:die|be\s*dead|end\s*it\s*all|stop\s*existing|stop\s*living)\b`)),

	// ---- Sela additions, ruleset 2026.10.26-3 ----
	// Colloquial Jakarta forms of "end my life" (-in suffix, ng- prefix).
	newRule("id-suicide-colloquial", CategorySuicide, LevelCrisis,
		pat(`\b(?:ngakhiri|ngakhirin|akhirin|ngakirin|akirin)\s*(?:(?:aja|saja|ajah)\s*)?(?:hidup(?:ku|nya)?|semuanya|segalanya|semua\s*ini)\b`)),
	// "gantungin diri", "ngegantung diri" ("menggantungkan diri", to depend
	// on someone, is not listed).
	newRule("id-hanging-colloquial", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:gantungin|ngegantungin|ngegantung)\s*diri\b`)),
	// "lompat/terjun dari gedung": synonyms of v1 jumping-phrases' "loncat"
	// (and v1's phrase with "atas"). Same subject check (animals and devices
	// are rejected); a leisure jump by others is rejected too ("kami terjun
	// dari tebing di bali seru banget", "anak-anak lompat dari atap rumah ke
	// kolam"; see leisureJump). A jump into water or a ravine ("pengen terjun
	// ke jurang", "mau lompat ke sungai") is as often a game, so it needs a
	// wish or intent word right before the verb.
	newRule("id-jumping-colloquial", CategorySuicide, LevelCrisis,
		// "dr" is the chat spelling of "dari"; a window or balcony is a
		// height too (review finding).
		patReject(`\b(?:(?:lompat|melompat|terjun|nerjun|meloncat|nyebur)\s*(?:dari|dr)\s*(?:atas\s*)?|loncat\s*(?:dari\s*atas|dr(?:\s*atas)?)\s*)(?:gedung|jembatan|lantai|atap|tebing|apartemen|rooftop|menara|flyover|jendela|balkon)\b|\bloncat\s*dari\s*(?:tebing|apartemen|rooftop|menara|flyover|jendela|balkon)\b`, jumpNotSpeaker),
		patReject(`\b(?:pengen|pgn|pingin|pengin|kepengen|kepingin|mau|mw|ingin|akan|mending|mendingan|hampir|nyaris|coba|nyoba|mencoba|berniat|niat|rasanya)\s+(?:(?:aja|saja|mau|pengen|ingin|udah|sudah|hampir|nyaris|coba|akan|bakal|tinggal)\s+){0,2}(?:lompat|melompat|terjun|nerjun|meloncat|nyebur|loncat)\s*ke\s*(?:dalam\s*)?(?:jurang|sungai|laut|rel|kali|danau|waduk|bendungan|sumur)\b`, thingSubject)),
	// "pengen tidur selamanya" without a pronoun, or with gua/ku: v1
	// first-person-euphemism needs aku/saya/gue/gw and counts those itself
	// (see ownEuphemism).
	newRule("id-sleep-forever", CategorySuicide, LevelCrisis,
		patReject(`\b(?:ingin|mau|mw|pengen|pgn|pingin|pengin|kepengen|akan)\s*(?:tidur\s*(?:untuk\s*)?selamanya|(?:ga|gak|nggak|ngga|tidak|tak|enggak)\s*bangun\s*lagi)\b`, ownEuphemism),
		// "pengen tidur dan ga bangun lagi", "pengen tidur terus ga usah bangun
		// lagi" (review finding).
		pat(`\b(?:ingin|mau|mw|pengen|pgn|pingin|pengin|kepengen|semoga|moga|moga-moga|mudah-mudahan|berharap)\s*(?:(?:aja|saja|ajah|deh|ya)\s*)?(?:`+idHopeSelf+`\s*)?tidur\s*(?:(?:aja|saja|dan|terus|trus|lalu|abis\s*itu)\s*)*(?:ga|gak|gk|nggak|ngga|tidak|tak|enggak)\s*(?:usah\s*|perlu\s*)?bangun\s*(?:lagi|lg)\b`),
		// A hope: "semoga aku ga bangun lagi besok", "berharap tidur
		// selamanya" (review finding). "bangun kesiangan lagi" (oversleeping
		// again) does not match.
		pat(`\b`+idSleepHope+idHopeTime+`(?:`+idHopeSelf+`\s*)?`+idHopeTime+
			`(?:(?:ga|gak|gk|nggak|ngga|tidak|tak|enggak|engga)\s*(?:usah\s*|perlu\s*)?bangun(?:[\s-]*bangun)?\s*(?:lagi|lg|besok|esok|selamanya)|tidur\s*(?:untuk\s*)?selamanya)\b`),
		// "semoga tidurku malam ini yang terakhir", "semoga malam ini
		// tidurku yang terakhir" (review findings).
		pat(`\b`+idSleepHope+idHopeTime+`tidur(?:ku|\s*`+idHopeSelf+`)?\s*`+idHopeTime+`(?:ini\s*)?(?:yang|yg|jadi|menjadi|adalah)\s*(?:yang\s*|yg\s*)?(?:terakhir|penghabisan)\b`)),
	// "aku ingin meninggal saja": the formal verb. A wish (pengen/ingin) is
	// counted with any subject; "mau meninggal" ("about to die") only with a
	// first-person subject or "aja" ("rasanya mau meninggal aja"), since
	// "nenekku mau meninggal" is a grandmother who is dying.
	newRule("id-death-wish-formal", CategorySuicide, LevelCrisis,
		pat(`\b(?:pengen|ingin|pingin|pengin|kepengen|pgn)\s*meninggal\b|\b(?:aku|saya|gue|gw|gua|ku)\s*(?:mau|mw)\s*meninggal\b|\b(?:mau|mw)\s*meninggal\s*(?:aja|saja|ajah)\b`)),
	// Code-switched death wishes: an Indonesian verb with an English object
	// ("pengen die aja", "mau unalive aja") and the reverse ("aku wanna
	// mati"). Same idiom post-check as death-wish-phrase ("hp-ku mau die").
	newRule("mixed-death-wish", CategorySuicide, LevelCrisis,
		patReject(`\b(?:pengen|mau|mw|ingin|pingin|pengin|pgn|kepengen)\s*(?:die|unalive|kill\s*myself|end\s*it\s*all)\b|\b(?:wanna|want\s*to|gonna|going\s*to)\s*(?:mati|bunuh\s*diri|bundir|modar|mokat)\b`, mixedIdiom)),
	// "tired of living", "sick of life": the English counterpart of
	// id-tired-of-living, ambiguous and so elevated. A circumstance after it
	// ("tired of living like this", "sick of life in the city") is rejected,
	// as "capek hidup kayak gini" is.
	newRule("en-tired-of-living", CategoryHopelessness, LevelElevated,
		patReject(`\b(?:tired|sick|exhausted|weary)\s*of\s*(?:living|life|being\s*alive|existing)\b`, circumstantialLifeEn)),
	// "I want to sleep and never wake up": v1 en-farewell-note only has
	// "want to never wake up" right after the verb.
	newRule("en-sleep-forever", CategorySuicide, LevelCrisis,
		// "I hope I fall asleep and never wake up", "I hope I sleep and
		// never wake up" (review finding).
		// A time word may come first: "I hope one day I sleep and never wake
		// up", "I hope tonight I fall asleep and never wake up" (review
		// finding).
		pat(`\b(?:want|wanna|wish|hope|hoping|like|ready|pray|praying)\s*,?\s*(?:that\s*)?`+enHopeTime+`(?:i(?:'ll)?\s*(?:(?:can|could|will|would|just|finally)\s*)*)?(?:to\s*)?(?:(?:just|go\s*to|fall|falls|fell)\s*)*(?:sleep|asleep)\s*(?:tonight\s*)?(?:and|&|then|n)\s*(?:never|not|don'?t|won'?t|wouldn'?t)\s*(?:ever\s*)?wake\s*(?:up)?\b`),
		// "I hope I never wake up", "I wish I wouldn't wake up" (review
		// finding). A routine ("I hope I don't wake up late") is rejected
		// (see wakeRoutine).
		// "I hope I sleep forever", "I wish I could sleep forever", "I hope
		// tonight I sleep forever", "I hope this time I sleep forever".
		pat(`\b(?:hope|hoping|hopefully|wish|wishing|pray|praying)\s*,?\s*(?:that\s*)?`+enHopeTime+`i(?:'ll)?\s*(?:(?:can|could|will|would|just)\s*)*(?:sleep|go\s*to\s*sleep|fall\s*asleep)\s*(?:and\s*never\s*wake\s*up\s*)?forever\b`),
		// A time word may come first: "I hope tonight I don't wake up", "I
		// hope tonight is the night I don't wake up". wakeRoutine reads the
		// following sentence too ("... late. or ever"), so the pattern is
		// wide.
		patWideReject(`\b(?:hope|hoping|hopefully|wish|wishing|pray|praying)\s*,?\s*(?:that\s*)?`+enHopeTime+`(?:i|i'?ll)\s*(?:just\s*)?(?:never|don'?t|do\s*not|won'?t|will\s*not|wouldn'?t|would\s*not|didn'?t)\s*(?:ever\s*)?wake\s*(?:up)?\b`, wakeRoutine)),

	// Past and other forms of en-self-harm's "<verb> myself" ("I burned
	// myself on purpose", "I harmed myself last night", "I want to stab
	// myself"), a relapse ("I relapsed last night and cut again", "I cut
	// again"). Same idiom and accident checks as en-self-harm (review
	// finding: v1 none).
	newRule("en-self-harm-verbs", CategorySelfHarm, LevelCrisis,
		patMarker(`\b(?:harm(?:ed|s)|burn(?:ed|t|s)|injur(?:ed|es)|punish(?:ed|es)|stab(?:bed|bing|s)?)\s*myself\b`, selfHarmIdiom, accidental),
		pat(`\bi\s*(?:(?:just|literally|finally|then|have|had|did)\s*)?cut\s*again\b|\brelaps(?:e|ed|ing)\b(?:\s+(?:last|night|today|yesterday|this|morning|evening|week|again|and|then|i|just|finally))*\s+(?:cut|cutting|burned|burnt|burning|self[\s-]*harmed|self[\s-]*harming|hurt\s*myself)\b`),
		// Hitting oneself: "I hit myself", "I keep punching myself", "I bang
		// my head against the wall until it bleeds", "I scratch myself until
		// I bleed" (review finding: v1 none). Banging one's head and
		// scratching are everyday idioms or itches on their own, so they
		// need the injury.
		// An accident told as one past event ("I hit myself on the cabinet
		// door by accident") is a context marker (see accidental).
		patMarker(`\b(?:hit(?:s|ting)?|punch(?:es|ed|ing)?|slap(?:s|ped|ping)?|smack(?:s|ed|ing)?)\s*myself\b|\b(?:bang(?:s|ed|ing)?|hit(?:s|ting)?|slam(?:s|med|ming)?|smash(?:es|ed|ing)?)\s*my\s*head\s*(?:(?:against|on|into)\s*(?:the\s*|a\s*)?(?:wall|walls|floor|desk|table|door|mirror)\s*)?(?:until|till|til|so\s*hard\s*(?:that\s*)?)\s*(?:it|i)\s*(?:bleed|bleeds|bled|bleeding|bruise|bruised|passed?\s*out)\b|\bscratch(?:es|ed|ing)?\s*myself\s*(?:raw|bloody|(?:until|till|til)\s*(?:i|it)\s*(?:bleed|bleeds|bled|bleeding))\b`, nil, accidental)),
	// Past hanging and vehicles: "I hanged myself but the rope broke", "I
	// want to jump in front of a train", "throw myself under a bus" (review
	// finding: v1 none).
	newRule("en-lethal-means-forms", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:hanged|hung)\s*myself\b|\b(?:step(?:ped|ping|s)?|jump(?:ed|ing|s)?|(?:throw(?:ing|n|s)?|threw)\s*myself)\s*in\s*front\s*of\s*(?:a|an|the|some)?\s*(?:(?:moving|speeding|oncoming)\s*)?(?:train|truck|car|bus|lorry|subway|tram|vehicle)s?\b|\b(?:throw(?:ing|n|s)?|threw)\s*myself\s*(?:under|onto|on\s*to)\s*(?:a|an|the)?\s*(?:train|truck|car|bus|lorry|subway|tram|tracks)\b|\bjump(?:ed|ing|s)?\s*(?:onto|on\s*to|into)\s*(?:the\s*)?(?:train\s*|subway\s*|railway\s*)?tracks\b`),
		// "I want to throw myself off a bridge", "throw myself into the river"
		// (review finding: v1 none). "throw myself into my work" is not listed.
		pat(`\b(?:throw(?:ing|n|s)?|threw|hurl(?:ing|ed|s)?|fling(?:ing|s)?|flung)\s*myself\s*(?:off|from|out\s*of|over)\s*(?:of\s*)?(?:a|an|the|this|that|my|our|some)?\s*(?:(?:tall|high|nearest)\s*)?(?:bridge|building|roof|rooftop|window|balcony|cliff|tower|ledge|overpass|parking\s*garage|car\s*park|train|stairwell|railing|edge|skyscraper|apartment|dam)s?\b|\b(?:throw(?:ing|n|s)?|threw)\s*myself\s*into\s*(?:the\s*)?(?:river|sea|ocean|lake|water|traffic|road|canal|waves)\b`)),
	// Colloquial "menyakiti/melukai diri": "nyakitin diri sendiri",
	// "ngelukain diri" (review finding: v1 none).
	newRule("id-self-harm-colloquial", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:nyakitin|nyakiti|ngelukain|ngelukai|lukain|ngelukaiin|nyelakain|celakain)\s*diri(?:ku|\s*sendiri)?\b`),
		// Hitting oneself: "aku mukul diriku sendiri", "aku sering mukulin
		// diri sendiri", "mukulin kepalaku sendiri sampai berdarah" (review
		// finding: v1 none). A head needs "-ku" or "sendiri" ("mukul kepala
		// adikku" is someone else's).
		pat(`\b(?:mukul|mukulin|memukul|memukuli|mukulan|nonjok|nonjokin|ninju|ninjuin|nampar|namparin|menampar)\s*(?:diri(?:ku)?(?:\s*sendiri)?|kepala(?:ku|\s*(?:aku|gue|gw|saya))(?:\s*sendiri)?|kepala\s*sendiri)\b`),
		// Stabbing, drowning and burning oneself: "aku mau nusuk diri
		// sendiri", "aku mau nenggelamin diri", "aku pengen bakar diri"
		// (review finding: v1 none). "menenggelamkan diri dalam pekerjaan"
		// (burying oneself in work) is an idiom without an intent word (see
		// drownIdiomID), and a reported self-immolation ("aksi bakar diri")
		// is attributed (see burnNews).
		pat(`\b(?:nusuk|nusukin|menusuk|menusukkan|tusuk|nikam|menikam|tikam)\s*(?:diri(?:ku)?(?:\s*sendiri)?|(?:perut|dada|tangan|leher|nadi|paha)(?:ku)?\s*sendiri)\b`),
		patReject(`\b(?:nenggelamin|nenggelamkan|neggelamin|menenggelamkan|tenggelamin|tenggelamkan|ngelelepin)\s*diri(?:ku)?(?:\s*sendiri)?\b`, drownIdiomID),
		patCheck(`\b(?:bakar|membakar|ngebakar|bakarin|ngebakarin)\s*(?:diri(?:ku)?(?:\s*sendiri)?|(?:tangan|badan|kulit)(?:ku)?\s*sendiri)\b`, nil, burnNews)),
	// A wish to be hit by a vehicle: "pengen ditabrak kereta aja",
	// "pengen ketabrak truk aja biar selesai". "mau" ("about to": "tadi
	// mau ketabrak mobil") counts only with "aja" after the vehicle (review
	// finding: v1 none).
	newRule("id-vehicle-death-wish", CategorySuicide, LevelCrisis,
		pat(`\b(?:(?:pengen|pgn|pingin|pengin|kepengen|kepingin|ingin|mending|mendingan)|(?:semoga|moga|moga-moga|mudah-mudahan|berharap)\s*,?\s*`+idHopeTime+`(?:(?:aku|saya|gue|gw|gua|ku)\s*)?`+idHopeTime+`)\s*(?:(?:aja|saja)\s*)?(?:ditabrak|ketabrak|tertabrak|dilindes|kelindes|dilindas|kelindas|terlindas)\s*(?:(?:sama|oleh)\s*)?(?:kereta(?:\s*api)?|krl|mrt|truk|truck|mobil|bus|bis|motor)\b|\b(?:mau|mw)\s*(?:ditabrak|ketabrak|tertabrak|dilindes|kelindes|dilindas|kelindas|terlindas)\s*(?:(?:sama|oleh)\s*)?(?:kereta(?:\s*api)?|krl|mrt|truk|truck|mobil|bus|bis|motor)\s*(?:aja|saja|ajah)\b`)),
	// The English counterpart of id-vehicle-death-wish: "I want to get hit by
	// a car", "I want to get run over by a truck", "i wish a car would hit
	// me", "i hope a bus hits me tomorrow" (review finding: v1 none). A
	// negated hope ("I hope a car doesn't hit me") does not match.
	newRule("en-vehicle-death-wish", CategorySuicide, LevelCrisis,
		pat(`\b(?:want|wanna|wish|wished|hope|hoping|hopefully|pray|praying|would\s*love|love|ready)\s*(?:to\s*)?(?:that\s*)?(?:i(?:'d|'ll|\s*would|\s*could|\s*will|\s*can|\s*just)*\s*)?(?:just\s*)?(?:get|be|getting|being)\s*(?:hit|run\s*over|ran\s*over|struck|killed|flattened|crushed|mowed\s*down)\s*by\s*(?:a|an|the|some)?\s*(?:(?:moving|speeding|passing|oncoming)\s*)?(?:train|truck|car|bus|lorry|vehicle|semi|tram|subway|motorbike|motorcycle)s?\b`),
		pat(`\b(?:wish|wished|hope|hoping|pray|praying)\s*(?:that\s*)?(?:a|an|the|some)\s*(?:(?:moving|speeding|passing|oncoming)\s*)?(?:train|truck|car|bus|lorry|vehicle|semi|tram|subway|motorbike|motorcycle)\s*(?:would\s*|will\s*|could\s*|just\s*)*(?:(?:hit|hits|run\s*over|runs\s*over|ran\s*over|kill|kills|crush|crushes)\s*me|(?:run|runs|ran)\s*me\s*over|mows?\s*me\s*down)\b`),
		// "i want a car to hit me".
		pat(`\b(?:want|wanted|need)\s*(?:a|an|the|some)\s*(?:train|truck|car|bus|lorry|vehicle|semi|tram|subway)\s*to\s*(?:(?:hit|run\s*over|kill|crush)\s*me|run\s*me\s*over)\b`)),

	// ---- Sela additions, ruleset 2026.10.26-5 ----
	// A hope, wish or prayer to die, with a bounded run of time words,
	// pronouns, modals and adverbs between the verb and the death word ("I
	// hope I die", "I really hope I die soon", "I wish I could just die", "I
	// pray that I die in my sleep", "I hope tonight I die", "I hope I'm dead
	// by tomorrow"), and the negated forms of surviving ("I hope I don't
	// survive the night", "I hope I never see tomorrow", "I hope I'm not alive
	// next year", "I hope tomorrow never comes"; review finding: v1 none). A
	// hope someone else voices ("they wish I would die") is attributed (see
	// hopeOfOthers); "die" spelt for "dye" and dying of old age are rejected
	// (see hopeDeathIdiom).
	newRule("en-hope-to-die", CategorySuicide, LevelCrisis,
		patCheck(`\b`+enHopeLead+enHopeSubject+enHopeFill+`(?:die|be\s*dead|(?:am|was|were|'m)\s*dead|dead|(?:(?:be|am|was|were|'m)\s*)?gone|(?:get|got)\s*(?:killed|murdered)|drop\s*dead|pass\s*away|stop\s*breathing|(?:stop|stopped|cease)\s*(?:existing|to\s*exist|living)|(?:get|catch|got|develop)\s+(?:[a-z']+\s+){1,3}and\s+(?:just\s+)?die)\b`, hopeDeathIdiom, hopeOfOthers),
		patCheck(`\b`+enHopeLead+enHopeSubject+enHopeFill+`(?:don'?t|dont|do\s*not|won'?t|wont|will\s*not|never|wouldn'?t|would\s*not|can'?t|cannot|didn'?t|am\s*not|'m\s*not|wasn'?t|weren'?t|was\s*not|were\s*not|not)\s*(?:ever\s*)?`+
			`(?:survive|make\s*it\s*(?:through|to|past|until|till)\s*(?:the\s*)?(?:night|tonight|morning|tomorrow|next|another|end|my|weekend|christmas|new\s*year|\d+)|make\s*it|exist\s*(?:anymore|any\s*more|tomorrow|tonight|by\s*(?:morning|tomorrow|tonight)|next\s*(?:week|month|year)|in\s*the\s*morning)|`+
			`live\s*(?:to\s*see|past|until|till|through|beyond)\s*(?:the\s*)?(?:\d+|tomorrow|next|another|my|morning|twenty|thirty|forty|fifty|old\s*age)|live\s*(?:much\s*)?(?:longer|another\s*(?:day|week|year))|live\s*(?:very\s*|too\s*|that\s*|for\s*)?long|live\s*anymore|`+
			`(?:have|need)\s*to\s*(?:live|be\s*alive|exist)\s*(?:much\s*)?(?:longer|anymore|another|past|until|till|through|beyond)|`+
			`see\s*(?:tomorrow|the\s*(?:morning|sunrise|next\s*day|light\s*of\s*day)|morning|another\s*(?:day|morning|year)|next\s*(?:week|month|year)|my\s*(?:next\s*)?birthday|daylight|sunrise)|`+
			`last\s*(?:the\s*night|the\s*week|much\s*longer|another\s*(?:day|week|year))|(?:be\s*)?(?:alive|here|around)(?:\s*anymore)?)\b`, hopeDeathIdiom, hopeOfOthers),
		// "I hope it kills me", "I hope the pills kill me", "I hope this
		// finally kills me", "I wish my life would end", "I hope my life ends
		// soon" (review finding).
		patCheck(`\b`+enHopeLead+`(?:it|this|that|(?:the|these|those|my)\s*[a-z]+(?:\s*[a-z]+)?)\s*`+enHopeFill+`(?:kills?|ends?|takes?)\s*me\b`, nil, hopeOfOthers),
		patCheck(`\b`+enHopeLead+`my\s*life\s*`+enHopeFill+`(?:ends?|be\s*over|is\s*over|was\s*over|were\s*over|stops?)\b`, lifeEndsHappy, hopeOfOthers),
		pat(`\b(?:hope|hoping|hopefully|wish|wishing|pray|praying)\s*(?:that\s*)?(?:tomorrow|the\s*morning|morning|the\s*next\s*day)\s*(?:never|doesn'?t|does\s*not|won'?t|will\s*not|would\s*never|wouldn'?t)\s*(?:ever\s*)?(?:comes?|arrives?)\b`),
		// "I hope my heart stops in my sleep", "I wish my heart would just
		// stop"; "stops racing" is not one (see heartGerund).
		patReject(`\b`+enHopeLead+`my\s*heart\s*(?:(?:would|will|could|just|finally)\s*){0,3}(?:stops|stop|gives\s*out|give\s*out)(?:\s*beating)?\b`, heartGerund)),
	// Other death wishes that v1 en-death-wish misses: an adverb or an
	// intensifier between the wish and "die" ("I want to just die", "I want to
	// fucking die", "I want to quickly die"), "I'd like to die", "please let
	// me die", "I want someone to kill me", "I can't wait to die" (review
	// finding: v1 none). "die" spelt for "dye" is rejected, and so are the
	// idiom "let me die on this hill" and a refused permission ("please don't
	// let me die"; see letMeDieNot).
	newRule("en-death-wish-forms", CategorySuicide, LevelCrisis,
		patReject(`\b(?:(?:want|wanted|wanting|wish|wished|hope|hoping|need|ready|like|love)\s*to|wanna)\s*(?:`+enDeathAdverb+`\s*){1,3}(?:to\s*)?(?:die|be\s*dead)\b`, dyeObject),
		patReject(`\b(?:want|wanted|wish|need)\s*(?:(?:so|really|very)\s*)?(?:bad(?:ly)?|much)\s*to\s*(?:just\s*)?(?:die|be\s*dead)\b`, dyeObject),
		// A plea to God: "God, please take me", "lord just take me now"; not
		// "take me home" (see takeMePlace).
		patReject(`\b(?:god|lord|jesus)\s*,?\s*(?:(?:please|pls|plz|just|now|already|finally)\s*,?\s*){0,3}take\s*me\b`, takeMePlace),
		patReject(`\b(?:i'?d|id|i\s*would)\s*(?:(?:really|just|honestly|so|actually|truly)\s*)*(?:like|love|prefer)\s*to\s*(?:just\s*)?(?:die|be\s*dead)\b`, deathFormIdiom),
		patReject(`\blet\s*me\s*(?:just\s*)?die\b`, letMeDieNot),
		pat(`\b(?:want|wanted|wanna|need|wish|wished|hope|hoping|pray|praying)\s*(?:that\s*)?(?:someone|somebody|anyone|anybody|god)\s*(?:(?:would|could|will|to|just|finally|'d|'ll)\s*){0,3}(?:kill|murder|shoot|end)\s*me\b`),
		patReject(`\b(?:can'?t|cant|cannot|can\s*not)\s*wait\s*to\s*(?:just\s*)?(?:die|be\s*dead)\b`, dyeObject)),
	// The Indonesian counterpart of en-hope-to-die: "semoga aku mati",
	// "semoga aku cepet meninggal", "moga aku mati besok", "aku berharap
	// mati", "aku doain diriku sendiri mati", "semoga besok aku udah nggak
	// ada" (review finding: v1 none). The idiom checks of death-wish-phrase
	// apply ("semoga besok mati lampu"), a hope for someone else's death
	// ("semoga cepet mati tuh orang") is rejected (see hopeTarget), a hope
	// someone else voices ("mereka berharap aku mati") is attributed (see
	// hopeOfOthers), and "nggak ada" is rejected only before a listed object
	// or place ("semoga aku nggak ada kelas besok"; see notAliveObject).
	newRule("id-hope-to-die", CategorySuicide, LevelCrisis,
		patCheck(`\b`+idHopeLead+idHopeFill+`(?:`+idHopeSelf+`\s*)?`+idHopeFill+`(?:mati|meninggal|mampus|modar|mokat|tewas|wafat)\b`, hopeTarget, hopeOfOthers),
		patCheck(`\b`+idHopeLead+idHopeFill+idHopeSelf+`\s*`+idHopeFill+`(?:(?:udah|sudah|udh|sdh|dah)\s*)?(?:nggak|ngga|gak|ga|gk|tidak|tdk|tak|enggak|engga)\s*(?:ada|bernyawa|bernapas|bernafas|hidup|idup)\b`, notAliveObject, hopeOfOthers),
		// "semoga ajalku cepet datang", "pengen ajal cepet jemput".
		pat(`\b(?:`+idHopeLead+`|(?:pengen|pgn|pingin|pengin|kepengen|ingin)\s*)`+idHopeFill+`ajal(?:ku|\s*(?:aku|gue|saya))?\s*`+idHopeFill+`(?:datang|dateng|tiba|jemput|menjemput|jemput\s*aku)\b`),
		// A plea to God: "ya allah cabut nyawaku", "tuhan ambil aku aja",
		// "tuhan matiin aku".
		// "tuhan kenapa gak ambil aku aja" asks why God has not (review
		// finding).
		pat(`\b(?:ya\s*allah|ya\s*tuhan|tuhan|allah|gusti|ya\s*rabb)\s*,?\s*(?:(?:kenapa|knp|napa|kok)\s*(?:gak|nggak|ngga|ga|g|tidak|enggak|gk|belum|blm)\s*)?(?:(?:tolong|please|plis|pls|segera|cepet|cepat|aja|langsung|mending)\s*)*(?:cabut|ambil|matiin|matikan)\s*(?:aja\s*)?(?:nyawa(?:ku|\s*`+idPleaSelf+`)|`+idPleaSelf+`|diriku)\b|\bcabut\s*(?:aja\s*)?nyawa(?:ku|\s*`+idPleaSelf+`)\b`),
		// "semoga hidupku cepet berakhir", "semoga umurku gak panjang",
		// "semoga aku cepet dipanggil tuhan", "semoga aku segera dijemput
		// ajal" (review finding). A happy ending is rejected (see
		// lifeEndsHappy).
		patCheck(`\b`+idHopeLead+idHopeFill+`(?:hidup|umur|usia)(?:ku|\s*`+idHopeSelf+`)?\s*`+idHopeFill+`(?:berakhir|selesai|tamat|usai|habis|pendek|(?:nggak|ngga|gak|ga|gk|tidak|tdk|tak|enggak|engga)\s*(?:panjang|lama))\b`, lifeEndsHappy, hopeOfOthers),
		patCheck(`\b`+idHopeLead+idHopeFill+`(?:`+idHopeSelf+`\s*)?`+idHopeFill+`(?:dipanggil|diambil|dijemput|dicabut\s*nyawa(?:ku|nya)?)\s*(?:(?:sama|oleh|ama|sm)\s*)?(?:tuhan|allah|yang\s*maha\s*kuasa|sang\s*pencipta|ajal|maut|malaikat\s*(?:maut|izrail)|izrail)\b`, nil, hopeOfOthers)),
	// An intensifier or a speed adverb between an Indonesian wish and the
	// death word: "aku pengen banget mati", "pengen cepet mati", "pgn cpt
	// mati", "aku ingin sekali mati", "aku mau cepet-cepet mati" (review
	// finding: v1 none). Same idiom checks as death-wish-phrase; "mau ...
	// meninggal" ("about to die") needs a first-person subject or "aja", as
	// in id-death-wish-formal (see adverbDeathIdiom).
	newRule("id-death-wish-adverb", CategorySuicide, LevelCrisis,
		patReject(`\b(?:pengen|pgn|pngn|pingin|pengin|kepengen|kepingin|ingin|mau|mw|mending|mendingan)\s*(?:`+idDeathAdverb+`\s*){1,3}(?:mati|mampus|modar|mokat|meninggal)\b`, adverbDeathIdiom)),
	// A present or future overdose plan: "I'm going to take all my pills
	// tonight", "I want to take the whole bottle", "I'm thinking of taking all
	// my meds at once", "I'll take every pill I have", "I want to swallow a
	// bottle of pills", and past forms v1 en-overdose-means does not list ("I
	// took the whole bottle", "I swallowed every pill I had"). The plan forms
	// have no context marker: medication adherence never discards them, and
	// their object never narrows them ("I'm going to take the whole bottle of
	// wine" counts). A negated intent ("I don't want to take all my pills")
	// is rejected unless a contrastive conjunction hedges it, and a past
	// bottle of something else ("I took the whole bottle of water") is not an
	// overdose (see negatedIntent, bottleObject).
	newRule("en-overdose-plan", CategorySelfHarm, LevelCrisis,
		patWideReject(`\b`+enOdIntent+`\s*`+enOdFill+enOdVerb+`\s*`+enOdQty+`\s*`+enMeds+`\b`, negatedIntent),
		patWideReject(`\b`+enOdIntent+`\s*`+enOdFill+enOdVerb+`\s*`+enOdBottle, negatedIntent),
		patReject(`\b(?:took|taken|swallowed|downed|popped|chugged|ate)\s*(?:`+enOdBottle+`|`+enOdQtyPast+`\s*`+enMeds+`\b)`, bottleObject),
		// Pills named earlier: "I'm going to swallow them all", "I want to
		// take all of them at once".
		patWideReject(`\b`+enOdIntent+`\s*`+enOdFill+`(?:(?:swallow|swallowing|down|downing|pop|popping|chug|chugging)\s*(?:them\s*all|all\s*of\s*them)|(?:take|taking)\s*(?:them\s*all|all\s*of\s*them)\s*(?:at\s*once|in\s*one\s*go|together))\b`, negatedIntent),
		// A present plan with a time: "I'm taking all my pills tonight",
		// "tonight I take all my pills", "I'm taking the whole bottle now"
		// (review finding).
		pat(`\b(?:i'?m|im|i\s*am)\s*(?:(?:just|finally|literally|really|actually)\s*)?(?:taking|swallowing|downing|popping|chugging)\s*(?:`+enOdQty+`\s*`+enMeds+`|`+enOdBottle+`)\s*`+enOdNow),
		pat(`\b`+enOdNow+`\s*,?\s*(?:i'?m|im|i\s*am|i)\s*(?:(?:just|finally|literally|really|actually)\s*)?(?:take|taking|swallow|swallowing|down|downing|pop|popping)\s*(?:`+enOdQty+`\s*`+enMeds+`\b|`+enOdBottle+`)`)),
	// The Indonesian counterpart: "aku mau minum semua obat tidurku", "aku
	// pengen minum semua obatku sekaligus", "aku mau nelen semua obat",
	// "aku pengen minum sebotol obat tidur" (review finding: v1 none). "semua"
	// needs an intent word or "sekaligus" (an adherence report says "udah
	// minum semua obat"); "sebotol" and "segenggam" do not. A negated intent
	// is rejected.
	newRule("id-overdose-plan", CategorySelfHarm, LevelCrisis,
		patWideReject(`\b`+idOdIntent+`\s*`+idOdFill+idOdVerb+`\s*(?:semua|seluruh|semuanya|sebotol|segenggam|sekotak|sebungkus|satu\s*botol|satu\s*kotak|sestrip|satu\s*strip|banyak)\s*(?:(?:isi|dari)\s*)?`+idMeds+`\b`, negatedIntent),
		pat(`\b(?:`+idOdVerb+`|ngabisin|menghabiskan|habisin|abisin)\s*(?:sebotol|segenggam|sekotak|sebungkus|satu\s*botol|satu\s*kotak|sestrip|satu\s*strip)\s*`+idMeds+`\b`),
		// A question asking whether medicine may be taken together ("boleh
		// minum obat sekaligus dengan vitamin?") is rejected (see
		// odQuestion; review finding).
		patReject(`\b`+idOdVerb+`\s*`+idMeds+`\s*(?:semua(?:nya)?\s*)?(?:sekaligus|sebotol|segenggam|sekotak)\b`, odQuestion),
		pat(`\b`+idOdVerb+`\s*(?:semua|seluruh)\s*`+idMeds+`\s*(?:sekaligus|barengan|bareng)\b`),
		// "aku mau OD"; a present plan with a time: "malam ini aku minum
		// semua obat tidurku" (review finding).
		patWideReject(`\b`+idOdIntent+`\s*`+idOdFill+`(?:od|o\.d|overdo(?:sis|se))\b`, negatedIntent),
		pat(`\b(?:malam\s*ini|nanti\s*malam|sekarang|skrg|abis\s*ini|habis\s*ini|besok)\s*,?\s*(?:(?:aku|gue|gw|gua|saya|ku|aq)\s*)?(?:(?:langsung|mau|akan|bakal)\s*)?`+idOdVerb+`\s*(?:semua|seluruh|semuanya|sebotol|segenggam|sekotak|sebungkus)\s*`+idMeds+`\b`),
		// "minum obat sebanyak-banyaknya", "udah siapin semua obat buat
		// diminum sekaligus" (review finding).
		pat(`\b`+idOdVerb+`\s*`+idMeds+`\s*(?:sebanyak[\s-]*banyaknya|sebanyak\s*mungkin|sebanyaknya|sebanyak\s*yang\s*aku\s*bisa)\b`),
		pat(`\b(?:siapin|nyiapin|menyiapkan|siapkan|ngumpulin|kumpulin|ngumpulkan|mengumpulkan|nimbun|numpuk|nyimpen)\s*(?:semua|seluruh|banyak)\s*`+idMeds+`\s*`+
			`(?:(?:buat|untuk|utk|biar|mau|terus|trus|lalu|dan|nanti|aku|gue|gw)\s*){0,3}(?:di)?(?:minum|minumin|telan|nelen|tenggak)\s*(?:semua(?:nya)?\s*)?(?:sekaligus|barengan|bareng)\b`)),
	// Self-harm slang and relapse: "pengen sh lagi", "urge to sh", "relapsed
	// on sh", "I want to sh", "did sh", "sh relapse", "aku relapse, nyilet
	// lagi", "nyayat lagi", "pengen nyilet" (review findings: v1 none). "sh"
	// spelling "sh*t" is rejected (see shSlang, shNotSlang).
	newRule("self-harm-slang", CategorySelfHarm, LevelCrisis,
		// English intent words and a past "did sh" lead too ("I want to sh",
		// "gonna sh tonight", "i did sh last night", "sh relapse"; review
		// finding). The lead and the relapse word are checked around a bare
		// "sh" (see shSlang): one alternation of every lead was slow on long
		// texts with "she".
		patReject(`\bsh\b`, shSlang),
		pat(`\b(?:nyilet|nyiletin|menyilet|nyayat|nyayatin|menyayat)\s*(?:lagi|lg)\b|\brelaps(?:e|ed|ing)?\s*,?\s*(?:(?:lagi|lg)\s*,?\s*)?(?:nyilet|nyiletin|menyilet|nyayat|nyayatin|menyayat)\b`),
		// "pengen nyilet": an intent word before the verb, without "lagi"
		// (review finding). The match carries the intent word, so no
		// false-positive check reads its object ("pengen nyilet kertas"
		// counts).
		pat(`\b(?:pengen|pgn|pngn|pingin|pengin|kepengen|kepingin|mau|mw|ingin|akan|bakal|niat|berniat|kepikiran|pengennya)\s*(?:(?:aja|saja|banget|bgt|buat|untuk|coba|nyoba)\s*){0,2}(?:nyilet|nyiletin|menyilet|nyayat|nyayatin|menyayat)\b`)),
	// First-person passive death wishes that no other rule lists: wanting
	// not to exist or not to be alive, wanting it all to end, wanting to be
	// gone ("I just want to not exist", "I want to not be alive anymore", "I
	// just want it all to end", "I want to be gone"; review finding: v1
	// none). A negated wish ("I don't want to be gone") is rejected (see
	// negatedIntent), and so are an online absence ("I want to not exist on
	// social media"), a stay elsewhere ("I want to not be here today") and
	// being away for an everyday occasion ("I want to be gone by the time the
	// party starts"; see absenceOccasion).
	newRule("en-passive-death-wish", CategorySuicide, LevelCrisis,
		patReject(`\b(?:want|wanna|wanted|wish|wishing|need|needed)\s*(?:to\s*)?(?:(?:just|really|honestly|simply|literally|actually)\s*){0,2}(?:not|never)\s*(?:to\s*)?(?:(?:have\s*)?exist(?:ed)?|be\s*alive|(?:have\s*)?been\s*born|be\s*born|be\s*here|be\s*around)\b`, absenceOccasion),
		patReject(`\b(?:want|wanna|wanted|wish|wishing|need|needed)\s*(?:for\s*)?(?:it\s*all|my\s*(?:whole\s*)?life|this\s*life|my\s*existence)\s*to\s*(?:(?:just|finally|all|already)\s*)?(?:end|be\s*over|stop)\b`, absenceOccasion),
		patReject(`\b(?:want|wanna|wanted|wish|wishing|need|needed)\s*(?:to\s*)?(?:(?:just|really|honestly|simply|literally|actually)\s*){0,2}be\s*gone\b`, absenceOccasion),
		// "I want to leave this world" (review sweep: v1 none); "... a
		// better place" is rejected (see absenceOccasion).
		patReject(`\b(?:want|wanna|wanted|wish|wishing|need|needed)\s*(?:to\s*)?(?:(?:just|really|honestly|simply|literally|actually)\s*){0,2}leave\s*this\s*(?:world|earth|planet|life)\b`, absenceOccasion)),
	// The Indonesian counterparts: not being there ("aku pengen nggak ada",
	// "pengen nggak ada lagi di dunia ini"), leaving or resting forever ("aku
	// pengen pergi selamanya", "pengen istirahat selamanya"), being done
	// ("aku pengen udahan aja"), wanting everything to end ("pengen semuanya
	// berakhir") and being ready to die ("aku siap mati", "gue udah siap buat
	// mati"; review finding: v1 none, while English "ready to die" is v1
	// crisis). A negated wish is rejected (see negatedIntent), and so are a
	// listed object or everyday place after "nggak ada" ("pengen nggak ada
	// pr", "pengen nggak ada di kelas"), leaving for a place ("pengen pergi
	// selamanya dari kota ini"), ending something with someone ("pengen
	// udahan aja sama dia"), a happy ending ("pengen semuanya berakhir
	// bahagia") and devotion ("aku siap mati demi kamu"); see idAbsence.
	newRule("id-passive-death-wish", CategorySuicide, LevelCrisis,
		patReject(`\b(?:pengen|pgn|pngn|pingin|pengin|kepengen|kepingin|ingin|mau|mw)\s*(?:(?:aja|saja|banget|bgt|rasanya|sih|deh|udah|sudah|udh)\s*){0,2}(?:nggak|ngga|gak|ga|gk|tidak|tdk|tak|enggak|engga)\s*(?:ada|hidup|idup|bernyawa|bernapas|bernafas)\b`, idAbsence),
		patReject(`\b(?:pengen|pgn|pngn|pingin|pengin|kepengen|kepingin|ingin|mau|mw|akan)\s*(?:(?:aja|saja|banget|bgt)\s*)?(?:pergi|istirahat|beristirahat|rehat)\s*(?:(?:aja|saja|jauh|dengan\s*tenang|yang\s*tenang|untuk|utk|buat)\s*){0,2}(?:selamanya|selama-lamanya|abadi)\b`, idAbsence),
		patReject(`\b(?:pengen|pgn|pngn|pingin|pengin|kepengen|kepingin|ingin|mau|mw)\s*(?:(?:aja|saja|banget|bgt)\s*)?udahan\b`, idAbsence),
		// Stopping living and leaving this world ("aku pengen berhenti hidup",
		// "mau pergi aja dari dunia ini", "pengen ninggalin dunia ini"; review
		// sweep: v1 none). A manner after "hidup" ("berhenti hidup boros") and
		// a short break ("pergi dari dunia ini sebentar") are rejected (see
		// idAbsence).
		patReject(`\b(?:pengen|pgn|pngn|pingin|pengin|kepengen|kepingin|ingin|mau|mw)\s*(?:(?:aja|saja|banget|bgt|rasanya)\s*){0,2}(?:berhenti|stop)\s*(?:hidup|idup|bernapas|bernafas)\b`, idAbsence),
		patReject(`\b(?:pengen|pgn|pngn|pingin|pengin|kepengen|kepingin|ingin|mau|mw)\s*(?:(?:aja|saja|banget|bgt|rasanya)\s*){0,2}(?:(?:pergi|keluar|minggat)\s*(?:(?:aja|saja)\s*)?(?:dari|dr)|ninggalin|ninggalkan|meninggalkan|tinggalin)\s*(?:dunia|bumi)\s*(?:ini|fana)\b`, idAbsence),
		patReject(`\b(?:pengen|pgn|pngn|pingin|pengin|kepengen|kepingin|ingin|mau|mw)\s*(?:(?:aja|saja|banget|bgt)\s*)?(?:semua(?:nya)?(?:\s*ini)?|segalanya|hidup(?:ku|\s*(?:aku|gue|gw|saya))?)\s*(?:(?:cepet|cepat|segera|aja|saja)\s*)?(?:berakhir|tamat|usai)\b`, idAbsence),
		patReject(`\b(?:aku|saya|gue|gw|gua|ku|aq|ak|sy)\s*(?:(?:udah|sudah|udh|sdh|dah|emang|memang|benar-benar|bener-bener|beneran|sangat|pun)\s*){0,2}(?:siap|rela|ikhlas)\s*(?:(?:buat|untuk|utk)\s*)?(?:mati|meninggal|mampus)\b`, idAbsence)),
}

// The vocabulary of the hope and wish rules of ruleset -5.
const (
	// enHopeLead is a hope, wish or prayer verb and a bounded run of time
	// words before the subject ("I hope tonight I ...", "I pray to god that
	// ...", "I hope tonight is the night I ...").
	enHopeLead = `(?:hope|hoping|hoped|hopefully|wish|wishing|wished|pray|praying|prayed)\s*,?\s*(?:to\s*god\s*,?\s*)?(?:that\s*)?` + enHopeTime + `(?:that\s*)?`
	enHopeTime = `(?:(?:tonight|tomorrow|today|someday|one\s*day|this\s*time|this\s*year|soon|maybe|finally|eventually|god|please|lord|` +
		`(?:every|each)\s*(?:night|day|morning)|sometimes|always|secretly|still|really|just|honestly|lowkey|so\s*(?:much|badly|bad|hard)|` +
		`more\s*than\s*anything|with\s*all\s*my\s*heart|desperately|truly|genuinely|` +
		`(?:tonight|today|tomorrow)\s*(?:is|will\s*be)\s*the\s*(?:night|day)(?:\s*(?:that|when))?)\s*,?\s*){0,3}`
	enHopeSubject = `(?:i|i'?ll|i'?d|i'?m|im)\s*`
	// enHopeFill is a bounded run of modals and adverbs between the subject
	// and the death word ("I wish I could just die", "I hope I finally die").
	enHopeFill = `(?:(?:just|really|finally|simply|actually|literally|honestly|seriously|quickly|soon|will|would|could|can|might|may|` +
		`gonna|going\s*to|eventually|someday|peacefully|quietly|already|then|also|somehow|suddenly)\s*){0,4}`
	// enDeathAdverb are the adverbs and intensifiers that may stand between
	// "want to" and "die".
	enDeathAdverb = `(?:just|really|literally|fucking|fuckin|freaking|frickin|fking|fkn|effing|so|actually|honestly|seriously|simply|` +
		`finally|quickly|quietly|peacefully|already|kinda|totally|truly|genuinely|desperately|badly|now|soon|go|lowkey|fr|rly|rlly)`

	idHopeLead = `(?:semoga|smoga|moga|moga-moga|moga2|mudah-mudahan|mudah2an|berharap|ngarep|ngarepin|harap(?:nya)?|doain|doakan|mendoakan|berdoa)\s*,?\s*` +
		`(?:(?:biar|supaya|agar|spy|kalau|kalo|klo|aja|saja|deh|ya)\s*)?`
	idHopeSelf = `(?:aku|saya|gue|gw|gua|ku|aq|ak|sy|gwe|diriku(?:\s*sendiri)?|diri\s*(?:aku|gue|sendiri))`
	idHopeFill = `(?:(?:besok(?:\s*pagi)?|esok|nanti(?:\s*malam)?|malam\s*ini|pagi\s*ini|hari\s*ini|segera|cepet|cepat|cpt|cpet|cepetan|` +
		`cepat-cepat|cepet-cepet|secepatnya|buru-buru|langsung|aja|saja|ajah|udah|sudah|udh|sdh|bisa|akan|bakal|beneran|bener-bener|` +
		`benar-benar|akhirnya|juga|jg|ya\s*allah|ya\s*tuhan|tuhan|deh|ya|banget|bgt|bngt|bgtt|sekali|sangat|amat|bener|bnr|serius|sih|lah)\s*){0,3}`
	// idDeathAdverb are the intensifiers and speed adverbs that may stand
	// between an Indonesian wish and the death word.
	idDeathAdverb = `(?:banget|bgt|bngt|bgtt|bener|beneran|bener-bener|benar-benar|bnr|sekali|sangat|amat|segera|cepat|cepet|cpt|cpet|` +
		`cepat-cepat|cepet-cepet|cepetan|secepatnya|buru-buru|langsung|aja|saja|ajah|udah|sudah|udh|sdh|akhirnya|bisa|sih|deh|parah|serius)`
)

// The vocabulary of the overdose-plan rules of ruleset -5.
const (
	enOdIntent = `(?:going\s*to|gonna|want\s*to|wanna|wanted\s*to|will|'ll|plan(?:ning)?\s*(?:to|on)|planned\s*(?:to|on)|about\s*to|ready\s*to|` +
		`decided\s*to|thinking\s*(?:of|about)|thought\s*(?:of|about)|considering|tempted\s*to|might|could|gotta|urge\s*to)`
	enOdFill = `(?:(?:just|finally|literally|really|actually|honestly|seriously|go|and|simply)\s*){0,2}`
	enOdVerb = `(?:take|taking|swallow|swallowing|down|downing|pop|popping|chug|chugging|eat|eating|od\s*on|overdose\s*on|overdosing\s*on)`
	enOdQty  = `(?:all\s*(?:of\s*)?(?:my|the|these|those|his|her)?|every\s*(?:single\s*)?(?:one\s*of\s*(?:my|the)\s*)?|the\s*rest\s*of\s*(?:my|the)|` +
		`(?:a|an|the|my|half\s*(?:a|the|my))\s*(?:whole\s*|entire\s*|full\s*)?(?:bottle|pack|packet|box|strip|jar|handful|bunch)\s*of\s*(?:my\s*|the\s*)?|` +
		`(?:\d+|two|three|four|five)\s*(?:bottles?|packs?|box(?:es)?|strips?)\s*of\s*(?:my\s*|the\s*)?|handfuls\s*of\s*(?:my\s*|the\s*)?|a\s*(?:lot|ton|bunch)\s*of\s*(?:my\s*|the\s*)?|lots\s*of|too\s*many|so\s*many|as\s*many|` +
		// A number of ten or more ("take 30 pills", "take 50 of my sleeping
		// pills"; review finding).
		`(?:[1-9]\d+|ten|twenty|thirty|forty|fifty|sixty|a\s*hundred|hundreds\s*of|a\s*dozen|dozens\s*of)\s*(?:of\s*(?:my|the|these|those)\s*)?)`
	// enOdQtyPast leaves out "all (of) my/the", which v1 en-overdose-means
	// counts with its adherence marker.
	enOdQtyPast = `(?:every\s*(?:single\s*)?(?:one\s*of\s*(?:my|the)\s*)?|the\s*rest\s*of\s*(?:my|the)|` +
		`(?:a|an|the|my|half\s*(?:a|the|my))\s*(?:entire\s*|full\s*)?(?:bottle|pack|packet|box|strip|jar|handful)\s*of\s*(?:my\s*|the\s*)?)`
	enOdBottle = `(?:the|a|an|my|that|this)\s*(?:whole|entire|full)\s*(?:bottle|pack|packet|box|strip|jar)s?\b`
	// enOdNow is the time of a present plan ("tonight", "right now").
	enOdNow = `(?:tonight|today|now|right\s*now|rn|tomorrow|this\s*(?:evening|morning|afternoon|weekend)|(?:all\s*)?at\s*once|in\s*one\s*go)\b`
	enMeds  = `(?:(?:sleeping|sleep|pain|anxiety|heart|blood\s*pressure|allergy)\s*)?(?:pills?|meds|medications?|medicines?|tablets?|capsules?|` +
		`painkillers?|antidepressants?|benzos?|opioids?|opiates?|paracetamol|acetaminophen|tylenol|panadol|advil|ibuprofen|aspirin|xanax|` +
		`valium|ambien|klonopin|ativan|zoloft|prozac|lexapro|sertraline|fluoxetine|seroquel|quetiapine|lithium|insulin|oxycodone|oxy|` +
		`percocet|vicodin|morphine|fentanyl|melatonin|diazepam|alprazolam|zolpidem|clonazepam|tramadol|codeine|prescriptions?|drugs)`

	idOdIntent = `(?:mau|mw|pengen|pgn|pngn|pingin|pengin|kepengen|kepingin|ingin|akan|bakal|berencana|rencana|niat|berniat|siap|kepikiran|mikir|mikirin)`
	idOdFill   = `(?:(?:aja|saja|langsung|udah|sudah|banget|bgt|coba|nyoba|buat|untuk|sekalian)\s*){0,2}`
	idOdVerb   = `(?:minum|minumin|meminum|nelen|nelan|menelan|telan|nenggak|menenggak|tenggak)`
	idMeds     = `(?:obat|pil|tablet|kapsul|antidepresan|parasetamol|paracetamol|panadol|bodrex|ctm|antimo)(?:-obatan)?(?:ku|nya|mu)?` +
		`(?:\s*(?:tidur|penenang|antidepresan|keras)(?:ku|nya|mu)?)?`
)

// idSleepHope is the hope lead of id-sleep-forever, with an optional "aja"
// ("semoga aja aku nggak bangun lagi"; review finding).
const idSleepHope = `(?:semoga|smoga|moga|moga-moga|moga2|mudah-mudahan|mudah2an|berharap|harap(?:nya)?)\s*,?\s*(?:(?:aja|saja|ajah|deh|ya)\s*,?\s*)?`

// idPleaSelf is the speaker in a plea to God ("tuhan ambil gw aja").
const idPleaSelf = `(?:aku|gue|gw|gua|saya|aq|ak|gwe|sy)`

// idHopeTime is an optional time word in the Indonesian hope patterns of
// id-sleep-forever; it may stand before or after the pronoun ("semoga besok
// aku ga bangun lagi", "semoga aku besok ga bangun lagi").
const idHopeTime = `(?:(?:besok(?:\s*pagi)?|esok|nanti(?:\s*malam)?|malam\s*ini|pagi\s*ini|hari\s*ini)\s*)?`

// ruleIndex maps rule ids to catalogue positions.
var ruleIndex = func() map[string]int {
	m := make(map[string]int, len(catalogue))
	for i, r := range catalogue {
		if _, dup := m[r.id]; dup {
			panic("safety: duplicate rule id " + r.id)
		}
		m[r.id] = i
	}
	return m
}()

// wakeRoutine rejects a hope not to wake up that names a routine as a whole
// phrase: "I hope I don't wake up late", "... too early", "... before my
// alarm", "... at 3am", "... with a headache", "... hungover", "... during
// the night". Only listed trailing words may follow the routine in its
// clause ("late tomorrow", "early again", "early for work", "before noon on
// weekends"). A bare or final "too", "at all", "again", "ever", "sick" or
// "with this pain" is not a routine ("I hope I don't wake up at all", "I
// hope I never wake up sick again"), "by" a time is a deadline, not a
// routine ("I hope I don't wake up by morning"), a number of three or more
// digits is a year rather than a time ("until 2030"), and an intent, modal
// or habit word or a death word anywhere else in the sentence or in the text
// after it keeps the match ("I hope I don't wake up during the night, I want
// it to be over", "... before my alarm so I can die in my sleep", "... late.
// or ever", "... early. or at all"). The pattern is wide (see pattern.wide):
// s runs on past the sentence, and like every post-check it reads at most
// checkWindow bytes on either side of the match.
func wakeRoutine(s string, start, end int) bool {
	lo, hi := max(0, start-checkWindow), min(len(s), end+checkWindow)
	if hasRiskWord(s[lo:start]) || wakeDeathRe.MatchString(s[lo:hi]) {
		return false
	}
	var w []string // the words of the clause after the match
	clauseEnd := hi
	for i := end; i < hi && len(w) < 12; {
		c := s[i]
		switch {
		case strings.IndexByte(",;:\n.!?", c) >= 0:
			clauseEnd = i
			i = hi
		case isAlnum(c):
			j := i
			for j < hi && (isAlnum(s[j]) || s[j] == '\'') {
				j++
			}
			w = append(w, s[i:j])
			i = j
		default:
			i++
		}
	}
	at := func(k int) string {
		if k < len(w) {
			return w[k]
		}
		return ""
	}
	k := 0 // words of the routine phrase
	switch t := at(0); {
	case t == "late" || t == "early" || wakeStates[t]:
		k = 1
	case t == "too" && (at(1) == "early" || at(1) == "late" || at(1) == "tired"):
		k = 2
	case t == "before" || t == "until" || t == "till" || t == "at":
		if n := wakeTime(w[1:]); n > 0 {
			k = 1 + n
		}
	case t == "with" && (at(1) == "a" || at(1) == "an" || at(1) == "my") && wakeAilments[at(2)]:
		k = 3
	case t == "during" && at(1) == "the" && at(2) == "night":
		k = 3
	case t == "in" && at(1) == "the" && at(2) == "middle" && at(3) == "of" && at(4) == "the" && at(5) == "night":
		k = 6
	}
	if k == 0 || len(w) == 12 || hasRiskWord(s[clauseEnd:hi]) {
		return false
	}
	for _, x := range w[k:] {
		if !wakeTrail[x] && x != "again" {
			return false
		}
	}
	return true
}

// wakeTime returns the number of words of a time at the start of w ("3am",
// "6", "6 am", "noon", "my alarm", "the morning"), or 0.
func wakeTime(w []string) int {
	if len(w) == 0 {
		return 0
	}
	switch {
	case isDigit(w[0][0]):
		if len(w) > 1 && (w[1] == "am" || w[1] == "pm" || w[1] == "o'clock") {
			return 2
		}
		n := 0
		for n < len(w[0]) && isDigit(w[0][n]) {
			n++
		}
		if n > 2 && w[0][n:] == "" {
			return 0 // a year ("until 2030"), not a clock time
		}
		return 1
	case wakeTimes[w[0]]:
		return 1
	case (w[0] == "my" || w[0] == "the") && len(w) > 1 && (w[1] == "alarm" || wakeTimes[w[1]]):
		return 2
	}
	return 0
}

var (
	wakeStates   = setOf("hungover groggy cranky grumpy sore tired exhausted")
	wakeAilments = setOf("headache hangover migraine cold fever sore stiff")
	wakeTimes    = setOf("noon midnight dawn sunrise sunset morning alarm six seven eight nine ten five four")
	// wakeTrail may follow the routine phrase in its clause.
	wakeTrail = setOf("tomorrow today for work school class the exam test interview lol haha lmao pls please ugh tbh " +
		"on my weekends weekend weekdays saturday saturdays sunday sundays holidays holiday vacation days day off")
	// wakeDeathRe: a death or finality word in the sentence keeps the match.
	wakeDeathRe = mustGuarded(`\b(?:die|dies|died|dying|dead|death|kill|killed|suicide|suicidal|forever|over|end|ending|gone|at\s+all|ever|anymore|mati|meninggal)\b`)
)

// placeNotWorld re-expresses the v1 lookahead (?!\s+(?:di|dalam)\s+(?!dunia)):
// the match is rejected when it is followed by "di"/"dalam" and a place that
// is not "dunia" ("lebih baik aku nggak ada di rapat itu").
func placeNotWorld(s string, _, end int) bool {
	loc := placeAfter.FindStringIndex(s[end:])
	if loc == nil {
		return false
	}
	return !strings.HasPrefix(s[end+loc[1]:], "dunia")
}

var placeAfter = regexp.MustCompile(`^\s+(?:di|dalam)\s+`)

// followedByAdmin re-expresses the v1 lookahead (?!\s*admin) of
// en-end-everything ("done with life admin").
func followedByAdmin(s string, _, end int) bool {
	return strings.HasPrefix(strings.TrimLeft(s[end:], " \t\n\r\f"), "admin")
}

// selfOrThing rejects a third-person "wants to die" whose subject is the
// speaker's inner self (see innerSubject), a device ("my laptop", "my phone
// battery"), or whose "die" is spelt for "dye". Adverbs between the subject
// and "wants" are skipped ("part of me still wants to die").
func selfOrThing(s string, start, end int) bool {
	if innerSubject(s, start) || dyeObject(s, start, end) {
		return true
	}
	return enDevices[subjectHead(s, start)]
}

// subjectHead returns the head of the subject before the verb at start: the
// word before it past adverbs or, when that is an object pronoun after a
// preposition ("the plant next to me", "the guy behind her"), the word
// before the preposition.
func subjectHead(s string, start int) string {
	prev, ps := tokenBefore(s, start)
	for n := 0; n < 3 && subjectAdverbs[prev]; n++ {
		prev, ps = tokenBefore(s, ps)
	}
	if !objectPronouns[prev] {
		return prev
	}
	w, ws := tokenBefore(s, ps)
	if !prepositions[w] {
		return prev
	}
	for n := 0; n < 2; n++ {
		m, ms := tokenBefore(s, ws)
		if !prepModifiers[m] {
			break
		}
		ws = ms
	}
	head, _ := tokenBefore(s, ws)
	return head
}

var (
	objectPronouns = setOf("me him her us them you")
	prepositions   = setOf("to with near beside behind around by from next")
	prepModifiers  = setOf("next close right")
)

// notInnerSubject is the post-check of en-inner-death-wish's subjectless
// pattern: it keeps exactly the matches selfOrThing takes away as the
// speaker's inner self.
func notInnerSubject(s string, start, end int) bool {
	return !innerSubject(s, start) || dyeObject(s, start, end)
}

// innerSubject reports whether the subject of the "wants to die" that starts
// at start is the speaker's inner self, or includes the speaker: "me"
// heading an inner-self noun phrase (see innerMe), an inner-self noun after
// "my" or "my whole"/"my inner" ("my soul", "my whole body", "my inner
// self"), or "my inner child/voice". Adverbs between the subject and the verb
// are skipped. An object "me" ("someone close to me", "the guy who bullied
// me") is not the speaker's inner self.
func innerSubject(s string, start int) bool {
	prev, ps := tokenBefore(s, start)
	for n := 0; n < 3 && subjectAdverbs[prev]; n++ {
		prev, ps = tokenBefore(s, ps)
	}
	if prev == "me" {
		return innerMe(s, ps)
	}
	det, ds := tokenBefore(s, ps)
	switch {
	case innerSelf[prev] && (det == "my" || det == "this"):
		return true // "my soul", "this body" (review finding)
	case innerSelf[prev] && (det == "whole" || det == "inner" || det == "own"):
		mine, _ := tokenBefore(s, ds)
		return mine == "my"
	case innerOnly[prev] && det == "inner":
		mine, _ := tokenBefore(s, ds)
		return mine == "my"
	}
	return false
}

// innerMe reports whether the "me" that starts at at is the speaker, as the
// subject or the head of an inner-self noun phrase: a part of the speaker
// ("part of me", "a piece of me", "every inch of me"), whatever is inside the
// speaker ("something inside me", "the darkness in me"), a self ("the real
// me"), a subject that includes the speaker ("you and me"), or a bare "me"
// ("me wants to die"). Only an object "me" is not: after a preposition or a
// verb ("someone close to me", "everyone around me", "the girl in front of
// me", "the guy who bullied me"). Prepositions and verbs are listed; any
// other word before "me" keeps the safer reading.
func innerMe(s string, at int) bool {
	w1, p1 := tokenBefore(s, at)
	switch {
	case w1 == "about":
		// "everything about me wants to die" (review finding); "the guy
		// who talks about me" is an object.
		w0, _ := tokenBefore(s, p1)
		return innerAbout[w0]
	case w1 == "of":
		w0, _ := tokenBefore(s, p1)
		return innerParts[w0]
	case w1 == "in" || w1 == "inside" || w1 == "within", innerAdjectives[w1]:
		return true
	case objectPrepositions[w1], objectVerbs[w1]:
		return false
	}
	return true
}

// innerAbout are the subjects of "<x> about me" that are the speaker's
// whole self ("everything about me").
var innerAbout = setOf("everything something all part parts most rest every")

var (
	innerParts = setOf("part parts piece pieces half rest all most bit bits side sides version corner fraction some much " +
		"each inch cell cells fiber fibre bone bones ounce")
	innerAdjectives = setOf("real old new true inner other younger older past present current depressed sad broken " +
		"dark darker darkest suicidal tired exhausted weak worst ugly whole damaged scared stressed worried troubled " +
		"confused frustrated wounded traumatized traumatised scarred tortured isolated")
	objectPrepositions = setOf("to with around behind near beside besides by from for at like than about against " +
		"among amongst between under over after before above below toward towards without beyond past next upon")
	objectVerbs = setOf("told left hurt hit saw met made gave sent taught beat let got get know love hate like want " +
		"need help call text tell see meet make give send teach follow block ghost dump bully treat raise drove " +
		"brought bought took found kept knew hates loves likes tells texts calls knows wants needs helps sees " +
		"follows treats makes gives sends bullies leaves hits hurts beats blames ignores texted bullied called asked " +
		"helped raised abused hated loved liked followed blocked ghosted dumped dated married saved hugged kissed " +
		"touched cheated trusted believed warned stalked harassed attacked punched slapped kicked assaulted adopted " +
		"betrayed rejected abandoned ignored judged mocked teased insulted messaged dmed visited invited tagged " +
		"emailed contacted replied answered")
)

var subjectAdverbs = setOf("just really still kinda sometimes literally honestly also actually lowkey always truly " +
	"secretly now only even")

var innerSelf = setOf("heart soul body mind brain spirit self being")

// innerOnly nouns are the speaker's inner self only after "my inner" ("my
// child" is the speaker's son or daughter).
var innerOnly = setOf("child kid voice critic demon demons teen")

var enDevices = setOf("laptop phone battery computer pc mac macbook iphone android tablet ipad charger car engine " +
	"wifi router modem internet tv fridge fan printer console ps5 xbox switch controller headphones earbuds " +
	"airpods keyboard mouse camera server app game plant plants battery's")

// circumstantialLife rejects "capek hidup" when a circumstance follows
// ("capek hidup kayak gini terus", "capek hidup di jakarta").
func circumstantialLife(s string, _, end int) bool {
	return circumstanceAfter.MatchString(s[end:])
}

var circumstanceAfter = regexp.MustCompile(`^\s*(?:kayak|kaya|kek|kyk|seperti|spt|gini|begini|begitu|gitu|di\s|dengan\s|dgn\s|bareng\s|sama\s|ngekos|merantau|sendiri\s+di)`)

// idiomaticDeath rejects a "mati" match that is an everyday idiom:
//   - the adverb "mati-matian" (all-out effort), after any branch;
//   - an idiom noun right after "mati" ("mau mati lampu", "besok mau mati
//     listrik", "mati gaya", "mati kutu", "mati ketawa", and "mati rasa" at
//     the end of its phrase; see deathIdioms), after any branch that ends in
//     "mati", wishes included ("pengen mati lampu aja biar libur", "harusnya
//     aku mati rasa");
//   - a device noun right after "mati aja" ("tiba-tiba mati aja tv-nya");
//   - a device, power, plant or animal subject before "mau mati", "mw mati",
//     "mau modar" or "mati aja", the only forms a thing can take ("baterai
//     hp-ku mau mati", "kucingku mau mati", "lampunya mati aja"). A wish
//     ("pengen", "ingin", "mending", "lebih baik") is never a thing's, so
//     those branches are not checked for a subject.
//
// The subject is the word before the match, past aspect words such as
// "udah" ("hp-ku udah mau mati"). It is a thing when it is a thing noun
// ("hp-ku", "lampunya", "kucingku") that is not governed by a preposition or
// verb ("di depan laptop mau mati", "nunggu wifi mau mati" stay counted), a
// species after its kind ("bunga mawarnya", "kucing tetangga"), or a thing
// noun plus a separate possessive pronoun ("hp gue mau mati") at the start of
// a clause or after another thing noun ("baterai hp aku mau mati"). In
// run-on text "<thing> aku" is read as a new subject ("dari tadi nunggu wifi
// aku mau mati" stays counted). A pronoun with no thing ("aku mau mati") is
// never rejected.
func idiomaticDeath(s string, start, end int) bool {
	if matianAfter.MatchString(s[end:]) {
		return true
	}
	m := s[start:end]
	if strings.HasSuffix(m, "mati") {
		w := nextWords(s, end, 2)
		// The -nya form of an idiom noun is the same idiom ("semoga mati
		// lampunya"; review finding).
		if deathIdioms[w[0]] || deathIdioms[strings.TrimSuffix(w[0], "nya")] || (w[0] == "rasa" && (w[1] == "" || rasaFollow[w[1]] || laughter(w[1]) || laughterWords[w[1]])) {
			return true
		}
	}
	if strings.HasSuffix(m, "aja") {
		next := nextWords(s, end, 2)
		if particles[next[0]] {
			next[0] = next[1]
		}
		if isThing(next[0]) && !clockTimeAfter(s, end) {
			return true
		}
	}
	if !strings.HasPrefix(m, "mau") && !strings.HasPrefix(m, "mw") && !strings.HasPrefix(m, "mati") {
		return false
	}
	return thingBefore(s, start)
}

// clockTimeAfter reports whether "jam" or "pukul" and a number follow
// s[:end], past particles: a time ("pengen mati aja jam 12", "nggak ada jam
// 12"), not the clock as a device or a lesson hour.
func clockTimeAfter(s string, end int) bool {
	i := end
	for n := 0; n < 3; n++ {
		for i < len(s) && s[i] == ' ' {
			i++
		}
		j := i
		for j < len(s) && isLetter(s[j]) {
			j++
		}
		switch w := s[i:j]; {
		case w == "jam" || w == "pukul":
			for j < len(s) && s[j] == ' ' {
				j++
			}
			return j < len(s) && isDigit(s[j])
		case particles[w] || w == "aja" || w == "saja" || w == "lagi":
			i = j
		default:
			return false
		}
	}
	return false
}

// thingBefore reports whether the subject before s[start:] is a thing (see
// idiomaticDeath).
func thingBefore(s string, start int) bool { return subjectIs(s, start, isThing) }

// animalBefore reports whether the subject before s[start:] is an animal:
// only an animal can be the subject of a jump ("kucingku loncat dari atap";
// "mati lampu mau loncat dari gedung" is the speaker's).
func animalBefore(s string, start int) bool { return subjectIs(s, start, isAnimal) }

// subjectIs reports whether the subject before s[start:] is a noun that is
// accepts (see idiomaticDeath for the subject forms).
func subjectIs(s string, start int, is func(string) bool) bool {
	prev, ps := tokenBefore(s, start)
	for n := 0; n < 2 && aspectWords[prev]; n++ {
		prev, ps = tokenBefore(s, ps)
	}
	if prev == "" {
		return false
	}
	if is(prev) {
		gov, gs := tokenBefore(s, ps)
		return !deviceGovernors[gov] && !numberBefore(s, gs)
	}
	if possessive[prev] {
		// "hp gue mau mati": a bare thing noun and a possessive pronoun. A
		// noun that already has a possessive suffix ("kucingku aku mau
		// mati", "hp-ku aku mau mati aja") is followed by the speaker as a
		// new subject (review finding).
		dev, ds := tokenBefore(s, ps)
		if !is(dev) || !things[dev] {
			return false
		}
		before, _ := tokenBefore(s, ds)
		return before == "" || is(before) || deviceLeadIns[before]
	}
	// A species or owner after its kind: "bunga mawarnya", "kucing tetangga".
	kind, ks := tokenBefore(s, ps)
	if thingKinds[kind] && !firstPerson(prev) {
		gov, _ := tokenBefore(s, ks)
		return !deviceGovernors[gov]
	}
	return false
}

// thingSubject is the subject check of the jumping rules: an animal
// subject ("kucingku loncat dari atap", "kucingku mau loncat") is not the
// speaker. Wishes between the subject and the verb are skipped. A device,
// power or a plant cannot jump, so a noun such as "lampu" before the verb
// belongs to another phrase ("mati lampu mau loncat dari gedung" counts;
// review sweep).
func thingSubject(s string, start, _ int) bool {
	p := start
	for n := 0; n < 2; n++ {
		w, ws := tokenBefore(s, p)
		if !wishWords[w] {
			break
		}
		p = ws
	}
	return animalBefore(s, p)
}

var wishWords = setOf("pengen pgn pingin pengin kepengen ingin mau mw")

// jumpNotSpeaker is the post-check of the colloquial jumping phrases: a
// thing subject (see thingSubject), or a leisure jump (see leisureJump).
func jumpNotSpeaker(s string, start, end int) bool {
	return thingSubject(s, start, end) || leisureJump(s, start, end)
}

// leisureJump rejects a jump by a group or by others in a leisure context
// ("kami terjun dari tebing di bali seru banget", "anak-anak lompat dari
// atap rumah ke kolam"): both a group or third-party subject right before the
// verb and a leisure word in the phrase are needed, and a phrase that carries
// an intent, modal or habit word keeps the match.
func leisureJump(s string, start, end int) bool {
	subj, ps := tokenBefore(s, start)
	for n := 0; n < 2 && aspectWords[subj]; n++ {
		subj, ps = tokenBefore(s, ps)
	}
	if subj == "" && ps > 0 && s[ps-1] == '2' {
		// Reduplication written with a 2 ("anak2" for "anak-anak").
		if w, _ := tokenBefore(s, ps-1); w != "" {
			subj = w + "-" + w
		}
	}
	if !groupSubjects[subj] {
		return false
	}
	lo, hi := localWindow(s, start, end)
	w := s[lo:hi]
	return leisureRe.MatchString(w) && !hasRiskWord(w)
}

var (
	groupSubjects = setOf("kami kita anak-anak anakanak mereka orang-orang orangorang teman-teman temanteman " +
		"temen-temen tementemen pemain atlet peserta turis wisatawan bocah bocah-bocah bocahbocah")
	leisureRe = regexp.MustCompile(`\b(?:seru|asik|asyik|keren|kolam|berenang|renang|bungee|paralayang|parasut|paragliding|olahraga|latihan|wahana|lomba|atraksi|liburan|wisata|main|mainan|pantai)\b`)
)

// deathIdioms follow "mati" in everyday idioms: a power cut ("mati lampu",
// "mati listrik"), boredom ("mati gaya"), being stumped ("mati kutu", "mati
// langkah", "mati akal"), laughing ("mati ketawa"), dying of curiosity
// ("mati penasaran"), a stalled engine ("mati mesin") and still air ("mati
// angin"). Numbness ("mati rasa") is an idiom only when "rasa" ends the
// phrase or a particle, a preposition or laughter follows it (rasaFollow:
// "harusnya aku mati rasa lol"; review finding): in run-on
// chat "rasa" often opens a new noun phrase ("aku mau mati rasa ini terlalu
// berat"). "kata" is not an idiom noun ("pengen mati kata orang itu dosa").
var deathIdioms = setOf("lampu listrik gaya kutu suri ketawa penasaran angin langkah akal mesin")

var rasaFollow = setOf("aja saja sama ke terhadap dengan dgn sih deh lah banget bgt kok ya nih dong tuh juga jg doang")

// matianAfter does not cross a newline: "mati\n-matian" is two lines.
var matianAfter = regexp.MustCompile(`^[ \t-]*matian\b`)

// particles may stand between "mati aja" and the device it is about ("mati
// aja lah hp-ku").
var particles = setOf("lah deh sih nih dong ya tuh")

// aspectWords may stand between a device and "mati aja"/"mau mati".
var aspectWords = setOf("mau mw udah udh sudah hampir bakal akan lagi lg keburu juga jg malah sering suka tiba-tiba")

// deviceGovernors before a device noun make it an object or a place, not the
// subject of "mau mati" ("nunggu wifi mau mati", "di depan laptop").
var deviceGovernors = setOf("di depan dari ke sama dengan dgn pake pakai make nunggu nungguin menunggu main maen " +
	"mainin gara-gara garagara gegara karena karna krn liat lihat ngeliat ngeliatin natap natapin menatap scroll " +
	"buka ngecek cek benerin ngurus ngurusin ngadep ngadepin " +
	// A quantity word makes "jam" a unit of time ("tiap jam mau mati
	// rasanya"; review finding).
	"tiap setiap per beberapa berapa satu dua tiga empat lima sejam")

// numberBefore reports whether a number ends just before s[i:], past
// spaces ("2 jam mau mati": a duration, not a clock).
func numberBefore(s string, i int) bool {
	for i > 0 && s[i-1] == ' ' {
		i--
	}
	return i > 0 && isDigit(s[i-1])
}

// deviceLeadIns may open a clause before "<device> <pronoun> mau mati".
var deviceLeadIns = setOf("eh duh aduh waduh haduh yah yaah wah anjir njir anjay astaga btw mana terus trus lalu tapi tp dan")

// tokenBefore returns the word that ends just before i, separated from it by
// spaces only, and its start offset.
func tokenBefore(s string, i int) (string, int) {
	j := i
	for j > 0 && s[j-1] == ' ' {
		j--
	}
	k := j
	for k > 0 && (isLetter(s[k-1]) || s[k-1] == '-' || s[k-1] == '\'') {
		k--
	}
	return s[k:j], k
}

var possessive = map[string]bool{"aku": true, "gue": true, "gw": true, "gua": true, "saya": true, "ku": true}

// things are the device, power, plant and animal nouns that can be the
// subject of an idiomatic "mau mati"/"mati aja".
var things = setOf("hp hape handphone henpon ponsel smartphone iphone android tablet ipad baterai batre batrai batere " +
	"laptop komputer pc tv televisi lampu listrik wifi sinyal internet kipas ac kulkas mesin motor mobil charger " +
	"casan router modem aki genset senter jam kamera powerbank speaker printer server aplikasi app " +
	// plants
	"tanaman pohon bunga mawar melati anggrek kaktus sukulen monstera padi cabai cabe tomat daun rumput " +
	// animals
	"kucing anjing ikan cupang koi lele guppy burung ayam bebek hamster kelinci kura-kura kura marmut sapi kambing " +
	"kuda babi tikus ular monyet kenari lovebird perkutut hewan peliharaan anabul")

// animals are the animal nouns of things.
var animals = setOf("kucing anjing ikan cupang koi lele guppy burung ayam bebek hamster kelinci kura-kura kura " +
	"marmut sapi kambing kuda babi tikus ular monyet kenari lovebird perkutut hewan peliharaan anabul")

// isAnimal reports whether tok, without a possessive suffix, is an animal
// noun (see isThing).
func isAnimal(tok string) bool {
	if !isThing(tok) {
		return false
	}
	if animals[tok] {
		return true
	}
	for _, suf := range []string{"-nya", "-ku", "-mu", "nya", "ku", "mu"} {
		if b, ok := strings.CutSuffix(tok, suf); ok && animals[b] {
			return true
		}
	}
	return false
}

// thingKinds name a kind before a species or an owner ("bunga mawarnya",
// "kucing tetangga").
var thingKinds = setOf("bunga pohon tanaman ikan kucing burung ayam hewan")

// swearThings are animal nouns that are also interjections ("anjing, aku mau
// mati" is "damn, I want to die"): they only count as a subject with a
// possessive suffix ("anjingku", "anjingnya").
var swearThings = setOf("anjing babi monyet")

// isThing reports whether tok, without a possessive suffix, is a thing noun.
func isThing(tok string) bool {
	if things[tok] {
		return !swearThings[tok]
	}
	for _, suf := range []string{"-nya", "-ku", "-mu", "nya", "ku", "mu"} {
		if b, ok := strings.CutSuffix(tok, suf); ok && len(b) >= 2 && things[b] {
			return true
		}
	}
	return false
}
