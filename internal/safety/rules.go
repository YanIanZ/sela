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
	// reject, when set, inspects an RE2 match s[start:end] and reports that it
	// must be discarded. It re-expresses v1 lookarounds.
	reject func(s string, start, end int) bool
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
	v1Rule("explicit-suicide-phrase", CategorySuicide, LevelCrisis,
		pat(`\b(bunuh\s*diri|bundir|akhiri\s*hidup(?:ku|nya)?|mengakhiri\s*hidup(?:ku|nya)?)\b`)),
	// Post-check (new, documented idiom decision): a device or plant subject
	// ("baterai hp-ku mau mati", "lampunya mati aja") and the adverb
	// "mati-matian" ("mau mati-matian belajar") are not death wishes.
	v1Rule("death-wish-phrase", CategorySuicide, LevelCrisis,
		patReject(`\b(pengen\s*mati|mau\s*mati|ingin\s*mati|mati\s*aja|mending\s*mati|lebih\s*baik\s*mati)\b`, idiomaticDeath)),
	v1Rule("no-will-to-live", CategorySuicide, LevelCrisis,
		pat(`\b(g(a|ak|k)\s*mau\s*hidup\s*lagi|tidak\s*mau\s*hidup\s*lagi|capek\s*hidup\s*pengen\s*ngilang\s*selamanya)\b`)),
	v1Rule("farewell-will-phrase", CategoryHopelessness, LevelCrisis,
		pat(`\b(surat\s*wasiat|pamit\s*selamanya|selamat\s*tinggal\s*semuanya|titip\s*pesan\s*terakhir|dunia\s*lebih\s*baik\s*tanpa(ku|aku))\b`)),

	// ---- v1: Indonesian, acute self-harm and lethal means ----
	v1Rule("self-cutting", CategorySelfHarm, LevelCrisis,
		pat(`\b(sayat|nyayat|iris)\s*(tangan|nadi|lengan|urat|leher)\b`)),
	v1Rule("self-harm-intent", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:menyakiti|melukai|mencelakai)\s*diri(?:ku| sendiri)?\b`)),
	v1Rule("first-person-euphemism", CategorySuicide, LevelCrisis,
		pat(`\b(?:aku|saya|gue|gw)\s+(?:ingin|mau|pengen|akan)\s+(?:tidur\s*selamanya|menghilang\s*selamanya|ga\s*bangun\s*lagi)\b`)),
	v1Rule("hanging-phrase", CategorySelfHarm, LevelCrisis,
		pat(`\b(gantung\s*diri|menggantung\s*diri)\b`)),
	v1Rule("ingestion-poison", CategorySelfHarm, LevelCrisis,
		pat(`\b(minum\s*(baygon|racun|pestisida|deterjen|cairan\s*pemutih))\b`)),
	v1Rule("jumping-phrases", CategorySuicide, LevelCrisis,
		pat(`\b(loncat\s*dari\s*(gedung|jembatan|lantai|atap)|nabrakin\s*diri\s*ke\s*(kereta|truk|mobil))\b`)),

	// ---- v1: coverage gaps found in manual testing ----
	v1Rule("end-everything-phrase", CategorySuicide, LevelCrisis,
		pat(`\b(?:ingin|mau|pengen|akan)?\s*(?:mengakhiri|akhiri|menyudahi|sudahi|bunuh\s*diri)\s*(?:semuanya|semua\s*ini|segalanya)\b`)),
	v1Rule("overdose-intent", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:(?:mau|ingin|pengen|akan|udah|sudah|berencana)\s*overdosis|overdosis\s*obat|minum\s*obat\s*(?:sebanyak\s*(?:mungkin|mungkin\s*biar)|banyak\s*banget\s*biar|segenggam|sekaligus\s*(?:semua|sebotol))|nelen\s*obat\s*(?:semua|sebotol|segenggam))\b`)),
	v1Rule("self-cutting-affixed", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:menyayat|nyayat|sayat|mengiris|ngiris|iris|nyilet|menyilet|silet)\s*(?:pergelangan\s*)?(?:tangan|nadi|lengan|urat|leher|paha)(?:ku|nya)?\b`)),
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
	// number right before it ("5 kms from home") rejects the match
	// (new, documented false-positive fix; free-standing digits are no longer
	// turned into letters, see Normalize).
	v1Rule("en-explicit-suicide", CategorySuicide, LevelCrisis,
		pat(`\b(?:kill(?:ing)?\s*myself|suicid(?:e|al)|take\s*my\s*(?:own\s*)?life|end(?:ing)?\s*my\s*(?:own\s*)?life|end\s*it\s*all|unalive\s*myself|off\s*myself)\b`),
		patReject(`\bkms\b`, precededByNumber)),
	v1Rule("en-death-wish", CategorySuicide, LevelCrisis,
		pat(`\b(?:(?:want|wanna|wish|wished|wanted|hope|hoping)\s*(?:to\s*)?(?:be\s*)?(?:die|dead|to\s*die)|wish\s*i\s*(?:was|were|had)\s*(?:dead|never\s*born)|better\s*off\s*dead|rather\s*be\s*dead|(?:don'?t|do\s*not)\s*(?:want|wanna)\s*to\s*(?:live|be\s*alive|exist|wake\s*up)(?:\s*anymore)?|no\s*reason\s*to\s*(?:live|go\s*on|keep\s*going)|can'?t\s*go\s*on\s*(?:living|anymore)|ready\s*to\s*die)\b`)),
	v1Rule("en-self-harm", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:(?:hurt(?:ing)?|harm(?:ing)?|cut(?:ting)?|burn(?:ing)?|injur(?:e|ing)|punish(?:ing)?)\s*myself|self[\s-]*harm(?:ing)?|self[\s-]*injur(?:y|ing)|slit(?:ting)?\s*my\s*wrists?|cut(?:ting)?\s*my\s*(?:wrists?|arms?|veins?|thighs?))\b`)),
	v1Rule("en-lethal-means", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:hang(?:ing)?\s*myself|(?:take|took|taking)\s*(?:an\s*)?overdose|overdos(?:e|ing)\s*(?:on\s*)?(?:pills|meds|medication|myself)|od\s*on\s*(?:pills|meds)|jump(?:ing)?\s*(?:off|from)\s*(?:a|the|this)?\s*(?:bridge|building|roof|window|balcony|cliff)|drink(?:ing)?\s*(?:bleach|poison)|swallow(?:ing)?\s*(?:all\s*)?(?:the|my)?\s*pills|shoot(?:ing)?\s*myself|drown(?:ing)?\s*myself|step(?:ping)?\s*in\s*front\s*of\s*(?:a|the)\s*(?:train|truck|car|bus))\b`)),
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
		patReject(`\b(?:i'?m|im|i\s*am)\s*(?:so\s*)?done\s*with\s*(?:life|living|being\s*alive)\b`, followedByAdmin)),
	v1Rule("en-overdose-means", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:(?:plan(?:ning)?\s*to|going\s*to|gonna|want\s*to|wanna|will|about\s*to|ready\s*to|trying\s*to)\s*(?:overdose|od\b)|(?:took|taken|swallowed|just\s*took)\s*(?:a\s*lot\s*of|all\s*(?:of\s*)?(?:my|the)|too\s*many|a\s*bunch\s*of|so\s*many|a\s*whole\s*(?:bottle|pack)\s*of|handfuls?\s*of)\s*(?:pills|meds|tablets|medication|medicine|sleeping\s*pills|painkillers)|(?:bought|got|have|found|made|tied)\s*(?:a\s*)?(?:rope|noose)\b|\bnoose\b|(?:stockpil\w*|saving\s*up|hoarding)\s*(?:my\s*)?(?:pills|meds|tablets|medication))\b`)),
	v1Rule("en-self-harm-ongoing", CategorySelfHarm, LevelCrisis,
		pat(`\b(?:(?:been|started|keep|kept|still)\s*(?:self[\s-]*)?(?:cutting|burning\s*my\s*skin)(?:\s*again)?|cutting\s*again|relapsed\s*(?:on|into|with)\s*(?:cutting|self[\s-]*harm))\b`)),
	// v1 en-ambiguous-distress. The anchored "^kill me$" branch is matched
	// against the whole text (v1) and against each clause.
	v1Rule("en-ambiguous-distress", CategoryHopelessness, LevelElevated,
		patAnchored(`^kill\s*me[\s.!]*$`),
		pat(`\b(?:(?:please|someone|somebody|just)\s*kill\s*me|i\s*(?:just\s*)?want\s*(?:it|this|everything)\s*to\s*(?:stop|end)|(?:i\s*)?can'?t\s*go\s*on|(?:want|wanna)\s*to\s*end\s*it)\b`)),
	v1Rule("en-severe-distress", CategoryHopelessness, LevelElevated,
		pat(`\b(?:can'?t\s*(?:take|handle|do|bear)\s*(?:this|it|life)\s*anymore|(?:i|i'?ve)\s*give(?:n)?\s*up\s*on\s*(?:life|everything|living)|nothing\s*matters\s*anymore|(?:i|i'?m|i\s*am|i\s*feel)\s*(?:completely\s*|totally\s*)?hopeless|no\s*way\s*out|i\s*hate\s*my\s*life|life\s*is\s*(?:pointless|not\s*worth\s*(?:it|living)))\b`)),

	// ---- Sela additions: Indonesian slang and colloquial spellings ----
	// Slang spellings of "want to die" (pgn, pingin, kepengen, mw) and the
	// synonyms "modar"/"mokat". Same idiom post-check as death-wish-phrase.
	newRule("id-death-wish-slang", CategorySuicide, LevelCrisis,
		patReject(`\b(?:pgn|pngn|pingin|pengin|kepengen|kepingin|mw)\s*mati\b|\b(?:pgn|pngn|pingin|pengin|kepengen|kepingin|mw|pengen|mau|ingin)\s*(?:modar|mokat)\b`, idiomaticDeath)),
	// "ga mau idup lagi", "nggak pengen hidup lagi". v1 no-will-to-live misses
	// "nggak"/"enggak" (its \bg cannot start inside "nggak") and "idup".
	newRule("id-no-will-to-live-slang", CategorySuicide, LevelCrisis,
		pat(`\b(?:ga|gak|gk|nggak|ngga|ngak|enggak|engga|tidak|tdk|tak|ndak)\s*(?:mau|mw|pengen|pgn|pingin|ingin|kepengen)\s*h?idup\s*(?:lagi|lg)\b`)),
	// "pengen ngilang aja selamanya": vanishing *forever* is treated like v1's
	// "aku mau menghilang selamanya" (crisis) without requiring a pronoun.
	newRule("id-vanish-forever", CategorySuicide, LevelCrisis,
		pat(`\b(?:ingin|mau|mw|pengen|pgn|pingin|pengin|kepengen|akan)\s*(?:menghilang|ngilang|hilang|lenyap)\s*(?:(?:aja|saja)\s*|dari\s*(?:dunia|bumi)\s*(?:ini\s*)?)*(?:untuk\s*)?selamanya\b`)),
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
	newRule("en-death-wish-modal", CategorySuicide, LevelCrisis,
		pat(`\bi\s*(?:should|deserve\s*to|ought\s*to|need\s*to|might\s*as\s*well|may\s*as\s*well)\s*(?:just\s*)?(?:die|be\s*dead)\b`)),
	newRule("en-slang-suicide", CategorySuicide, LevelCrisis,
		pat(`\b(?:sewer\s*slide|(?:want|wanna|gonna|going\s*to|plan(?:ning)?\s*to|thinking\s*(?:about|of)|think\s*about|ready\s*to|about\s*to)\s*(?:to\s*)?unaliv(?:e|ing)|unaliv(?:e|ing)\s*my\s*self|(?:wanting|wishing|hoping)\s*to\s*(?:die|be\s*dead))\b`)),

	// ---- Sela additions: third-person wording (always attributed) ----
	// "my friend wants to kill herself", "the character killed himself". The
	// level applies only if the speaker adopts it ("me too"); otherwise the
	// match is third-party and routes to elevated.
	// The "wants to die" branch has no subject in the match; selfOrThing
	// rejects it when the subject is the speaker's inner self ("part of me",
	// "my soul": counted by en-inner-death-wish instead) or a device ("my
	// laptop wants to die": an idiom).
	thirdPersonRule("en-third-person-suicide", CategorySuicide,
		pat(`\b(?:kill(?:s|ed|ing)?\s*(?:him|her|them)\s*sel(?:f|ves)|(?:end|ends|ended|ending)\s*(?:his|her|their)\s*(?:own\s*)?li(?:fe|ves)|(?:take|takes|took|taken|taking)\s*(?:his|her|their)\s*own\s*li(?:fe|ves))\b`),
		patReject(`\bwants\s*to\s*(?:die|be\s*dead|end\s*it\s*all)\b`, selfOrThing)),
	thirdPersonRule("en-third-person-self-harm", CategorySelfHarm,
		pat(`\b(?:hurt|hurts|hurting|harm|harms|harmed|harming|cut|cuts|cutting|burn|burns|burned|burnt|burning|hang|hangs|hanged|hanging|drown|drowns|drowned|drowning|shoot|shoots|shooting|starve|starves|starving)\s*(?:him|her|them)\s*sel(?:f|ves)\b`)),

	// ---- Sela additions, ruleset 2026.10.26-2 ----
	// "part of me wants to die", "my soul wants to be dead": the speaker's
	// own death wish phrased through an inner self (v1 none).
	newRule("en-inner-death-wish", CategorySuicide, LevelCrisis,
		pat(`\b(?:part\s*of\s*me|everything\s*in\s*me|something\s*in\s*me|all\s*of\s*me|(?:the\s*)?rest\s*of\s*me|my\s*(?:whole\s*)?(?:heart|soul|body|mind|brain|spirit|self|being))\s*(?:(?:just|really|still|kinda|sometimes|literally|honestly|also|actually|lowkey|always|truly|secretly|now|only)\s*)*(?:wants?|wanted)\s*to\s*(?:die|be\s*dead|end\s*it\s*all|stop\s*existing|stop\s*living)\b`)),
}

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

// precededByNumber rejects "kms" written as a distance ("5 kms").
func precededByNumber(s string, start, _ int) bool {
	j := start
	for j > 0 && s[j-1] == ' ' {
		j--
	}
	return j > 0 && isDigit(s[j-1])
}

// followedByAdmin re-expresses the v1 lookahead (?!\s*admin) of
// en-end-everything ("done with life admin").
func followedByAdmin(s string, _, end int) bool {
	return strings.HasPrefix(strings.TrimLeft(s[end:], " \t\n\r\f"), "admin")
}

// selfOrThing rejects a third-person "wants to die" whose subject is the
// speaker's inner self ("part of me", "everything in me", "my soul") or a
// device ("my laptop", "my phone battery"). Adverbs between the subject and
// "wants" are skipped ("part of me still wants to die").
func selfOrThing(s string, start, _ int) bool {
	prev, ps := tokenBefore(s, start)
	for n := 0; n < 3 && subjectAdverbs[prev]; n++ {
		prev, ps = tokenBefore(s, ps)
	}
	switch {
	case prev == "me":
		return true
	case innerSelf[prev]:
		det, _ := tokenBefore(s, ps)
		return det == "my" || det == "whole"
	}
	return enDevices[prev]
}

var subjectAdverbs = setOf("just really still kinda sometimes literally honestly also actually lowkey always truly " +
	"secretly now only even")

var innerSelf = setOf("heart soul body mind brain spirit self being")

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
//   - the adverb "mati-matian" (all-out effort), after any branch; and
//   - a device, power or plant subject before "mau mati", "mw mati", "mau
//     modar" or "mati aja", the only forms a device can take ("baterai hp-ku
//     mau mati", "lampunya mati aja"). A wish ("pengen", "ingin", "mending",
//     "lebih baik") is never a device's, so those branches are not checked.
//
// The subject is the word before the match, past aspect words such as
// "udah" ("hp-ku udah mau mati"). It is a device when it is a device noun
// ("hp-ku", "lampunya") that is not governed by a preposition or verb ("di
// depan laptop mau mati", "nunggu wifi mau mati" stay counted), or a device
// noun plus a separate possessive pronoun ("hp gue mau mati") at the start of
// a clause or after another device noun ("baterai hp aku mau mati"). In run-on
// text "<device> aku" is read as a new subject ("dari tadi nunggu wifi aku mau
// mati" stays counted). A pronoun with no device ("aku mau mati") is never
// rejected.
func idiomaticDeath(s string, start, end int) bool {
	if matianAfter.MatchString(s[end:]) {
		return true
	}
	m := s[start:end]
	if !strings.HasPrefix(m, "mau") && !strings.HasPrefix(m, "mw") && !strings.HasPrefix(m, "mati") {
		return false
	}
	prev, ps := tokenBefore(s, start)
	for n := 0; n < 2 && aspectWords[prev]; n++ {
		prev, ps = tokenBefore(s, ps)
	}
	if prev == "" {
		return false
	}
	if isDevice(prev) {
		gov, _ := tokenBefore(s, ps)
		return !deviceGovernors[gov]
	}
	if possessive[prev] {
		dev, ds := tokenBefore(s, ps)
		if !isDevice(dev) {
			return false
		}
		before, _ := tokenBefore(s, ds)
		return before == "" || isDevice(before) || deviceLeadIns[before]
	}
	return false
}

var matianAfter = regexp.MustCompile(`^[\s-]*matian\b`)

// aspectWords may stand between a device and "mati aja"/"mau mati".
var aspectWords = setOf("mau mw udah udh sudah hampir bakal akan lagi lg keburu juga jg malah sering suka tiba-tiba")

// deviceGovernors before a device noun make it an object or a place, not the
// subject of "mau mati" ("nunggu wifi mau mati", "di depan laptop").
var deviceGovernors = setOf("di depan dari ke sama dengan dgn pake pakai make nunggu nungguin menunggu main maen " +
	"mainin gara-gara garagara gegara karena karna krn liat lihat ngeliat ngeliatin natap natapin menatap scroll " +
	"buka ngecek cek benerin ngurus ngurusin ngadep ngadepin")

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

var devices = map[string]bool{
	"hp": true, "hape": true, "handphone": true, "henpon": true, "ponsel": true, "smartphone": true,
	"iphone": true, "android": true, "tablet": true, "ipad": true, "baterai": true, "batre": true,
	"batrai": true, "batere": true, "laptop": true, "komputer": true, "pc": true, "tv": true,
	"televisi": true, "lampu": true, "listrik": true, "wifi": true, "sinyal": true, "internet": true,
	"kipas": true, "ac": true, "kulkas": true, "mesin": true, "motor": true, "mobil": true,
	"charger": true, "casan": true, "router": true, "modem": true, "aki": true, "genset": true,
	"senter": true, "jam": true, "kamera": true, "powerbank": true, "speaker": true, "printer": true,
	"server": true, "aplikasi": true, "app": true, "tanaman": true, "pohon": true, "bunga": true,
}

// isDevice reports whether tok, without a possessive suffix, is a device,
// power or plant noun.
func isDevice(tok string) bool {
	if devices[tok] {
		return true
	}
	for _, suf := range []string{"-nya", "-ku", "-mu", "nya", "ku", "mu"} {
		if b, ok := strings.CutSuffix(tok, suf); ok && len(b) >= 2 && devices[b] {
			return true
		}
	}
	return false
}
