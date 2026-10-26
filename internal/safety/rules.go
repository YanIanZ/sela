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
	// sentence.
	reject func(s string, start, end int) bool
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
}

func pat(expr string) *pattern {
	f := compileFilter(expr)
	return &pattern{re: regexp.MustCompile(expr), filter: f, atoms: f.flatAtoms()}
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
		pat(`\b(?:ingin|mau|mw|pengen|pgn|pingin|pengin|kepengen|semoga|moga|moga-moga|mudah-mudahan|berharap)\s*(?:(?:aku|saya|gue|gw|gua|ku)\s*)?tidur\s*(?:(?:aja|saja|dan|terus|trus|lalu|abis\s*itu)\s*)*(?:ga|gak|gk|nggak|ngga|tidak|tak|enggak)\s*(?:usah\s*|perlu\s*)?bangun\s*(?:lagi|lg)\b`),
		// A hope: "semoga aku ga bangun lagi besok", "berharap tidur
		// selamanya" (review finding). "bangun kesiangan lagi" (oversleeping
		// again) does not match.
		pat(`\b(?:semoga|moga|moga-moga|mudah-mudahan|berharap|harap(?:nya)?)\s*`+idHopeTime+`(?:(?:aku|saya|gue|gw|gua|ku)\s*)?`+idHopeTime+
			`(?:(?:ga|gak|gk|nggak|ngga|tidak|tak|enggak|engga)\s*(?:usah\s*|perlu\s*)?bangun(?:[\s-]*bangun)?\s*(?:lagi|lg|besok|esok|selamanya)|tidur\s*(?:untuk\s*)?selamanya)\b`),
		// "semoga tidurku malam ini yang terakhir" (review finding).
		pat(`\b(?:semoga|moga|moga-moga|mudah-mudahan|berharap|harap(?:nya)?)\s*tidur(?:ku|\s*(?:aku|saya|gue|gw|gua))?\s*`+idHopeTime+`(?:ini\s*)?(?:yang|yg|jadi|menjadi|adalah)\s*(?:yang\s*|yg\s*)?(?:terakhir|penghabisan)\b`)),
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
		pat(`\b(?:want|wanna|wish|hope|hoping|like|ready|pray|praying)\s*(?:that\s*)?(?:i(?:'ll)?\s*(?:(?:can|could|will|would|just|finally)\s*)*)?(?:to\s*)?(?:(?:just|go\s*to|fall|falls|fell)\s*)*(?:sleep|asleep)\s*(?:tonight\s*)?(?:and|&|then|n)\s*(?:never|not|don'?t|won'?t|wouldn'?t)\s*(?:ever\s*)?wake\s*(?:up)?\b`),
		// "I hope I never wake up", "I wish I wouldn't wake up" (review
		// finding). A routine ("I hope I don't wake up late") is rejected
		// (see wakeRoutine).
		// "I hope I sleep forever", "I wish I could sleep forever".
		pat(`\b(?:hope|hoping|hopefully|wish|wishing|pray|praying)\s*(?:that\s*)?i(?:'ll)?\s*(?:(?:can|could|will|would|just)\s*)*(?:sleep|go\s*to\s*sleep|fall\s*asleep)\s*(?:and\s*never\s*wake\s*up\s*)?forever\b`),
		// A time word may come first: "I hope tonight I don't wake up".
		patReject(`\b(?:hope|hoping|hopefully|wish|wishing|pray|praying)\s*(?:that\s*)?(?:(?:tonight|tomorrow|today|someday|one\s*day|this\s*time)\s*)?(?:i|i'?ll)\s*(?:just\s*)?(?:never|don'?t|do\s*not|won'?t|will\s*not|wouldn'?t|would\s*not|didn'?t)\s*(?:ever\s*)?wake\s*(?:up)?\b`, wakeRoutine)),

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
		pat(`\b(?:hit(?:s|ting)?|punch(?:es|ed|ing)?|slap(?:s|ped|ping)?|smack(?:s|ed|ing)?)\s*myself\b|\b(?:bang(?:s|ed|ing)?|hit(?:s|ting)?|slam(?:s|med|ming)?|smash(?:es|ed|ing)?)\s*my\s*head\s*(?:(?:against|on|into)\s*(?:the\s*|a\s*)?(?:wall|walls|floor|desk|table|door|mirror)\s*)?(?:until|till|til|so\s*hard\s*(?:that\s*)?)\s*(?:it|i)\s*(?:bleed|bleeds|bled|bleeding|bruise|bruised|passed?\s*out)\b|\bscratch(?:es|ed|ing)?\s*myself\s*(?:raw|bloody|(?:until|till|til)\s*(?:i|it)\s*(?:bleed|bleeds|bled|bleeding))\b`)),
	// Past hanging and vehicles: "I hanged myself but the rope broke", "I
	// want to jump in front of a train", "throw myself under a bus" (review
	// finding: v1 none).
	newRule("en-lethal-means-forms", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:hanged|hung)\s*myself\b|\b(?:step(?:ped|ping|s)?|jump(?:ed|ing|s)?|(?:throw(?:ing|n|s)?|threw)\s*myself)\s*in\s*front\s*of\s*(?:a|an|the|some)?\s*(?:(?:moving|speeding|oncoming)\s*)?(?:train|truck|car|bus|lorry|subway|tram|vehicle)s?\b|\b(?:throw(?:ing|n|s)?|threw)\s*myself\s*(?:under|onto|on\s*to)\s*(?:a|an|the)?\s*(?:train|truck|car|bus|lorry|subway|tram|tracks)\b|\bjump(?:ed|ing|s)?\s*(?:onto|on\s*to|into)\s*(?:the\s*)?(?:train\s*|subway\s*|railway\s*)?tracks\b`)),
	// Colloquial "menyakiti/melukai diri": "nyakitin diri sendiri",
	// "ngelukain diri" (review finding: v1 none).
	newRule("id-self-harm-colloquial", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:nyakitin|nyakiti|ngelukain|ngelukai|lukain|ngelukaiin|nyelakain|celakain)\s*diri(?:ku|\s*sendiri)?\b`),
		// Hitting oneself: "aku mukul diriku sendiri", "aku sering mukulin
		// diri sendiri", "mukulin kepalaku sendiri sampai berdarah" (review
		// finding: v1 none). A head needs "-ku" or "sendiri" ("mukul kepala
		// adikku" is someone else's).
		pat(`\b(?:mukul|mukulin|memukul|memukuli|mukulan|nonjok|nonjokin|ninju|ninjuin|nampar|namparin|menampar)\s*(?:diri(?:ku)?(?:\s*sendiri)?|kepala(?:ku|\s*(?:aku|gue|gw|saya))(?:\s*sendiri)?|kepala\s*sendiri)\b`)),
	// A wish to be hit by a vehicle: "pengen ditabrak kereta aja",
	// "pengen ketabrak truk aja biar selesai". "mau" ("about to": "tadi
	// mau ketabrak mobil") counts only with "aja" after the vehicle (review
	// finding: v1 none).
	newRule("id-vehicle-death-wish", CategorySuicide, LevelCrisis,
		pat(`\b(?:(?:pengen|pgn|pingin|pengin|kepengen|kepingin|ingin|mending|mendingan)|(?:semoga|moga|moga-moga|mudah-mudahan|berharap)\s*`+idHopeTime+`(?:(?:aku|saya|gue|gw|gua|ku)\s*)?`+idHopeTime+`)\s*(?:(?:aja|saja)\s*)?(?:ditabrak|ketabrak|tertabrak|dilindes|kelindes|dilindas|kelindas|terlindas)\s*(?:(?:sama|oleh)\s*)?(?:kereta(?:\s*api)?|krl|mrt|truk|truck|mobil|bus|bis|motor)\b|\b(?:mau|mw)\s*(?:ditabrak|ketabrak|tertabrak|dilindes|kelindes|dilindas|kelindas|terlindas)\s*(?:(?:sama|oleh)\s*)?(?:kereta(?:\s*api)?|krl|mrt|truk|truck|mobil|bus|bis|motor)\s*(?:aja|saja|ajah)\b`)),
	// The English counterpart of id-vehicle-death-wish: "I want to get hit by
	// a car", "I want to get run over by a truck", "i wish a car would hit
	// me", "i hope a bus hits me tomorrow" (review finding: v1 none). A
	// negated hope ("I hope a car doesn't hit me") does not match.
	newRule("en-vehicle-death-wish", CategorySuicide, LevelCrisis,
		pat(`\b(?:want|wanna|wish|wished|hope|hoping|hopefully|pray|praying|would\s*love|love|ready)\s*(?:to\s*)?(?:that\s*)?(?:i(?:'d|'ll|\s*would|\s*could|\s*will|\s*can|\s*just)*\s*)?(?:just\s*)?(?:get|be|getting|being)\s*(?:hit|run\s*over|ran\s*over|struck|killed|flattened|crushed|mowed\s*down)\s*by\s*(?:a|an|the|some)?\s*(?:(?:moving|speeding|passing|oncoming)\s*)?(?:train|truck|car|bus|lorry|vehicle|semi|tram|subway|motorbike|motorcycle)s?\b`),
		pat(`\b(?:wish|wished|hope|hoping|pray|praying)\s*(?:that\s*)?(?:a|an|the|some)\s*(?:(?:moving|speeding|passing|oncoming)\s*)?(?:train|truck|car|bus|lorry|vehicle|semi|tram|subway|motorbike|motorcycle)\s*(?:would\s*|will\s*|could\s*|just\s*)*(?:(?:hit|hits|run\s*over|runs\s*over|ran\s*over|kill|kills|crush|crushes)\s*me|(?:run|runs|ran)\s*me\s*over|mows?\s*me\s*down)\b`),
		// "i want a car to hit me".
		pat(`\b(?:want|wanted|need)\s*(?:a|an|the|some)\s*(?:train|truck|car|bus|lorry|vehicle|semi|tram|subway)\s*to\s*(?:(?:hit|run\s*over|kill|crush)\s*me|run\s*me\s*over)\b`)),
}

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
// clause ("late tomorrow", "early again", "early for work"). A bare or final
// "too", "at all", "again", "ever", "sick" or "with this pain" is not a
// routine ("I hope I don't wake up at all", "I hope I never wake up sick
// again"), and an intent, modal or habit word or a death word anywhere else
// in the sentence keeps the match ("I hope I don't wake up during the night,
// I want it to be over", "... before my alarm so I can die in my sleep").
// Like every post-check it reads at most checkWindow bytes on either side of
// the match.
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
		case strings.IndexByte(",;:\n", c) >= 0:
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
	case t == "before" || t == "until" || t == "till" || t == "at" || t == "by":
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
	wakeTrail = setOf("tomorrow today for work school class the exam test interview lol haha lmao pls please ugh tbh")
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
		if deathIdioms[w[0]] || (w[0] == "rasa" && (w[1] == "" || rasaFollow[w[1]])) {
			return true
		}
	}
	if strings.HasSuffix(m, "aja") {
		next := nextWords(s, end, 2)
		if particles[next[0]] {
			next[0] = next[1]
		}
		if isThing(next[0]) {
			return true
		}
	}
	if !strings.HasPrefix(m, "mau") && !strings.HasPrefix(m, "mw") && !strings.HasPrefix(m, "mati") {
		return false
	}
	return thingBefore(s, start)
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
// phrase or a particle or preposition follows it (rasaFollow): in run-on
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
