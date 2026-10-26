package safety

import (
	"regexp"
	"strings"
)

// Post-checks added in ruleset sela-safety-2026.10.26-3. Each one receives
// the sentence s and a match s[start:end] and looks at a bounded window
// around the match only (see localWindow), so that checking every match
// stays linear in the sentence length; the context-marker gate scans the
// whole text once per variant, and only texts of at most maxMarkerText
// bytes (see markersApply). Every decision is recorded as a vector in
// testdata/vectors.json.
//
// There are two kinds. A fixed idiom is a set phrase that completes the
// match and has no self-harm reading ("cut myself off from", "cutting carbs",
// "mati lampu", "gol bunuh diri"); it is a pattern's reject check and
// discards the match whatever surrounds it. A context marker is a word next
// to the match that suggests a benign reading: an accident ("I cut myself
// shaving"), medication taken as prescribed ("like the doctor said") or a
// legal will ("surat wasiat yang sah"). It is a pattern's marker check, and
// its gate is closed by default (see markersApply): it discards the match
// only when the whole text is short, has no quotation mark, carries no risk
// signal anywhere (no intent, modal, habit, purpose, feeling, deliberate,
// method or farewell word, in any sentence) and has no word outside a closed
// benign vocabulary. So any continuation after a comma, a clause word or a
// full stop that the vocabulary does not list ("I cut myself while cooking
// because I wanted to", "I cut myself shaving. it helped", "bikin nagih",
// "all 30 of them") keeps the match, and the accident check never applies to
// a self-directed method (wrists, veins, nadi, a cut arm): the safer path
// wins. "kms" after a
// number, a group's leisure jump and the "dari <place>" of id-vanish-forever
// are read locally (see kmsNotSlang, leisureJump, vanishPlace).

// checkWindow bounds how far, in bytes, a post-check looks on either side of
// a match.
const checkWindow = 120

// localWindow returns the phrase around s[start:end]: it extends at most
// checkWindow bytes each way and stops at clause punctuation and at words
// that start a new clause ("but", "dan", "terus"), so a marker in a later
// clause ("I cut myself by accident, and now I cut myself on purpose") does
// not reach the match.
func localWindow(s string, start, end int) (lo, hi int) {
	lo = start
	for lo > 0 && start-lo < checkWindow {
		c := s[lo-1]
		if strings.IndexByte(",;:\n", c) >= 0 {
			break
		}
		if isLetter(c) && (lo-1 == 0 || !isLetter(s[lo-2])) {
			// s[lo-1:] starts a word: stop before a clause word.
			w := firstWord(s[lo-1:])
			if clauseWords[w] {
				break
			}
		}
		lo--
	}
	hi = end
	for hi < len(s) && hi-end < checkWindow {
		c := s[hi]
		if strings.IndexByte(",;:\n", c) >= 0 {
			break
		}
		if isLetter(c) && !isLetter(s[hi-1]) && clauseWords[firstWord(s[hi:])] {
			break
		}
		hi++
	}
	return lo, hi
}

// clauseWords start a new clause for localWindow.
var clauseWords = setOf("but tapi tetapi namun and dan terus trus lalu then so jadi because karena soalnya abis habis " +
	"although whereas padahal sedangkan")

// nextWords returns up to n words that follow s[i:], separated by spaces
// only, within checkWindow bytes.
func nextWords(s string, i, n int) []string {
	lim := min(len(s), i+checkWindow)
	var out []string
	for len(out) < n {
		j := i
		for j < lim && s[j] == ' ' {
			j++
		}
		k := j
		for k < lim && (isLetter(s[k]) || s[k] == '\'' || s[k] == '-') {
			k++
		}
		if k == j {
			break
		}
		out = append(out, s[j:k])
		i = k
	}
	for len(out) < n {
		out = append(out, "")
	}
	return out
}

// carriesRisk reports whether the phrase around s[start:end] (see
// localWindow) carries an intent, modal or habit word: "want", "going to",
// "will", "should", "keep", "again", "pengen", "mau", "sering", "biar". The
// idioms that read the words around the match keep such a match ("I want to
// drown myself in work"); context markers look at the whole text instead
// (see markersApply).
func carriesRisk(s string, start, end int) bool {
	lo, hi := localWindow(s, start, end)
	return hasRiskWord(s[lo:hi])
}

// hasRiskWord reports whether w contains an intent, modal or habit word.
func hasRiskWord(w string) bool {
	prev := ""
	for i := 0; i < len(w); {
		if !isLetter(w[i]) || (i > 0 && (isLetter(w[i-1]) || w[i-1] == '\'')) {
			i++
			continue
		}
		j := i
		for j < len(w) && (isLetter(w[j]) || w[j] == '\'' || w[j] == '-') {
			j++
		}
		tok := w[i:j]
		switch {
		case riskWords[tok], strings.HasSuffix(tok, "'ll"):
			return true
		case tok == "to" && riskBeforeTo[prev]:
			return true
		case (tok == "about" || tok == "of") && (prev == "thinking" || prev == "thought" || prev == "think"):
			return true
		}
		prev = tok
		i = j
	}
	return false
}

// riskWords are intent, modal and habit words. "would" and "'d" are left
// out: they mark a conditional ("I'd be shooting myself in the foot if I
// quit"), not an intent.
var riskWords = setOf("want wants wanted wanting wanna gonna gotta will won't shall plan plans planning planned " +
	"ready decided decide try tries trying tried attempt attempts attempting attempted need needs needed should " +
	"siap rela bersedia " +
	"must might could keep keeps kept keeping still again always often usually sometimes constantly " +
	"regularly repeatedly every everyday daily nightly started start starting begin began urge urges tempted considering " +
	"pengen pgn pngn pingin pengin kepengen kepingin mau mw ingin akan bakal coba nyoba mencoba biar supaya agar " +
	"berencana rencana niat berniat sering selalu suka kadang kadang-kadang tiap setiap terus-terusan masih mulai " +
	"kepikiran mikir harus mesti pengennya")

// riskBeforeTo are words that make an intent with a following "to" ("going
// to", "about to", "used to", "have to").
var riskBeforeTo = setOf("going about used have has had able")

// riskGoverns reports whether an intent or modal word governs the match at
// start directly: it is one of the few words before it, past pronouns, "to"
// and adverbs ("i want to kms", "aku mau bunuh diri karier"), so that "aku
// mau bawa kms ke posyandu" (the intent governs "bawa") is not counted.
func riskGoverns(s string, start int) bool {
	p := start
	for n := 0; n < 4; n++ {
		w, ws := tokenBefore(s, p)
		switch {
		case w == "":
			return false
		case riskWords[w], strings.HasSuffix(w, "'ll"), w == "going":
			return true
		case w == "about":
			t, _ := tokenBefore(s, ws)
			return t == "thinking" || t == "thought" || t == "think"
		case !governSkip[w]:
			return false
		}
		p = ws
	}
	return false
}

var governSkip = setOf("to i i'm im just really literally so honestly actually seriously gonna aku gue gw gua saya aja " +
	"saja rasanya udah sudah lagi beneran bener commit")

// suicideCompound rejects "bunuh diri"/"suicide" used as the modifier of a
// fixed compound: an own goal ("gol bunuh diri"), a suicide mission ("misi
// bunuh diri", "suicide mission"), a career move ("bunuh diri karier",
// "career suicide"), a film title ("Suicide Squad"), sports drills
// ("suicide sprints") and a car door ("suicide doors"). The compound is the
// speaker's own plan, and the match is kept, when an intent word governs it
// (see compoundGoverned: "aku mau jalanin misi bunuh diri", "I want to go on
// a suicide mission", "I'm planning a suicide run"), or a first-person
// subject governs "bunuh diri karier" ("aku bunuh diri karier"), and a
// compound noun after the match is not read as one before a pronoun ("aku
// mau bunuh diri karier aku hancur").
func suicideCompound(s string, start, end int) bool {
	m := s[start:end]
	prev, ps := tokenBefore(s, start)
	next := nextWords(s, end, 2)
	after := func(set map[string]bool, own bool) bool {
		return set[next[0]] && !compoundPronouns[next[1]] && !compoundGoverned(s, start, own)
	}
	// A compound noun before the match, or an English one after it, is a
	// figurative or named thing ("misi bunuh diri", "career suicide",
	// "suicide sprints"): only an intent word keeps it ("I'm on a suicide
	// mission at work" is an idiom). Before "bunuh diri karier" a
	// first-person subject keeps it too: "aku bunuh diri" is the core.
	switch {
	case strings.HasPrefix(m, "bunuh"):
		return (idCompoundBefore[prev] && !compoundGoverned(s, ps, false)) || after(idCompoundAfter, true)
	case m == "suicide":
		return (enCompoundBefore[prev] && !compoundGoverned(s, ps, false)) || after(enCompoundAfter, false)
	}
	return false
}

// compoundPronouns after a compound noun start a new clause ("bunuh diri
// karier aku hancur").
var compoundPronouns = setOf("aku saya gue gw gua ku dia ia kamu lo lu i i'm im he she they it")

// suicideNews attributes a news compound to a third party: a suicide bombing
// ("bom bunuh diri", "pelaku bom bunuh diri", "suicide bomber") reports
// someone else's act, so it routes to elevated like a news frame. When an
// intent word or a first-person subject governs the compound ("aku mau bom
// bunuh diri", "aku pengen jadi pelaku bom bunuh diri", "I want to be a
// suicide bomber", "I'm going to wear a suicide vest"; see compoundGoverned)
// it is the speaker's own plan and counts.
func suicideNews(s string, start, end int) bool {
	m := s[start:end]
	switch {
	case strings.HasPrefix(m, "bunuh"):
		prev, ps := tokenBefore(s, start)
		if !idNewsBefore[prev] {
			return false
		}
		// The compound may chain news words ("pelaku bom bunuh diri").
		for n := 0; n < 3; n++ {
			w, ws := tokenBefore(s, ps)
			if !idNewsBefore[w] {
				break
			}
			ps = ws
		}
		return !compoundGoverned(s, ps, true)
	case m == "suicide":
		return enNewsAfter[nextWords(s, end, 1)[0]] && !compoundGoverned(s, start, true)
	}
	return false
}

// compoundGoverned reports whether the compound that starts at s[at:] is
// governed by an intent, modal or habit word or, with own, by a first-person
// subject, past articles, light verbs, adverbs, news words and "to" ("aku
// mau jalanin misi ...", "aku pengen jadi pelaku ...", "I want to be a
// suicide ...", "I'm going to wear a suicide ...", "I am a suicide ...").
func compoundGoverned(s string, at int, own bool) bool {
	p := at
	for n := 0; n < 6; n++ {
		w, ws := tokenBefore(s, p)
		switch {
		case w == "":
			return false
		case riskWords[w], strings.HasSuffix(w, "'ll"), w == "going", own && ownPronouns[w]:
			return true
		case w == "about":
			t, _ := tokenBefore(s, ws)
			return t == "thinking" || t == "thought" || t == "think"
		case !compoundSkip[w] && !governSkip[w] && !idNewsBefore[w]:
			return false
		}
		p = ws
	}
	return false
}

// compoundSkip are the articles and light verbs between an intent word and
// a compound ("jadi pelaku", "go on a", "wear a").
var compoundSkip = setOf("a an the my this that some one jadi menjadi jalanin menjalankan ngejalanin ngelakuin " +
	"melakukan lakuin ngelakukan bikin buat ikut ikutan be become becoming go on do doing commit run running " +
	"wear wearing carry carrying make making launch join am is are was were")

var (
	idCompoundBefore = setOf("gol misi")
	idCompoundAfter  = setOf("karier karir politik")
	enCompoundBefore = setOf("career social political financial commercial electoral professional corporate brand")
	enCompoundAfter  = setOf("squad mission missions sprint sprints door doors squeeze run runs drill drills king kings " +
		"burpees shuttle shuttles lane")
	idNewsBefore = setOf("bom serangan pelaku pengebom peledakan ledakan teror teroris")
	enNewsAfter  = setOf("bomber bombers bombing bombings bomb bombs attack attacks attacker attackers vest vests blast blasts")
)

// relativeWill rejects "surat wasiat" written by a relative ("kakek sudah
// bikin surat wasiat", "nenek ninggalin surat wasiat"; see relativeAuthor):
// someone else's legal will, not the speaker's farewell note, whatever else
// the text says. A first-person or farewell word in its phrase keeps the
// match ("kakek bikin surat wasiat, aku juga mau bikin surat wasiat
// terakhirku" keeps the second).
func relativeWill(s string, start, end int) bool {
	if !strings.HasPrefix(s[start:end], "surat") {
		return false
	}
	lo, hi := localWindow(s, start, end)
	w := s[lo:hi]
	return !firstPerson(w) && !farewellRe.MatchString(w) && relativeAuthor(s, lo, start)
}

// legalWill is the context marker of farewell-will-phrase: "surat wasiat"
// next to a legal word ("cara bikin surat wasiat yang sah", "untuk
// pembagian warisan", "notaris"). A first-person or farewell word in its
// phrase keeps the match ("surat wasiat terakhirku", "aku udah nulis surat
// wasiat yang sah buat keluargaku"), and so does a risk signal anywhere in
// the text ("lagi bikin surat wasiat yang sah, abis ini aku pamit"; see
// markersApply).
func legalWill(s string, start, end int) bool {
	if !strings.HasPrefix(s[start:end], "surat") {
		return false
	}
	lo, hi := localWindow(s, start, end)
	w := s[lo:hi]
	return !firstPerson(w) && !farewellRe.MatchString(w) && legalRe.MatchString(w)
}

// relativeAuthor reports whether a relative is the subject right before the
// will at start, past aspect, intent and writing verbs ("kakek sudah bikin
// surat wasiat", "nenek ninggalin surat wasiat"), within s[lo:].
func relativeAuthor(s string, lo, start int) bool {
	p := start
	for n := 0; n < 5; n++ {
		w, ws := tokenBefore(s, p)
		if w == "" || ws < lo {
			return false
		}
		if relativeRe.MatchString(w) {
			return true
		}
		if !willVerbs[w] {
			return false
		}
		p = ws
	}
	return false
}

var willVerbs = setOf("mau mw akan sudah udah udh lagi baru sempat bikin buat membuat nulis menulis tulis ninggalin " +
	"meninggalkan punya menyiapkan nyiapin siapin ingin pengen juga telah dulu pernah")

var (
	farewellRe = mustGuarded(`\b(?:terakhir\w*|pamit|selamat\s+tinggal|maaf\w*|goodbye|selamanya)\b`)
	legalRe    = mustGuarded(`\b(?:warisan|waris|notaris|pengacara|hukum|legal|sah|pembagian|harta|aset|properti|akta|cara\s+(?:bikin|buat|membuat|menulis|nulis)|contoh|format|template|draf|draft)\b`)
	relativeRe = mustGuarded(`\b(?:kakek|nenek|ayah|ibu|papa|mama|bapak|almarhum(?:ah)?|mendiang|om|tante|paman|bibi|opa|oma|eyang|bokap|nyokap)\w*\b`)
)

// accidental is the context marker of the cutting rules and en-self-harm:
// an injury that its phrase says was an accident ("I accidentally cut myself
// while cooking", "I cut myself shaving", "I hurt myself at the gym",
// "nggak sengaja ngiris tangan pas potong bawang"). The marker must sit
// right before the match or right after it, and the match must be an injury
// that can happen by accident, told as a single past event: "cut", "hurt",
// "burned"/"burnt" or "injured" myself (not "cutting", "hurting myself" or
// a present "burn myself", which tell a habit), or an Indonesian sliced hand
// ("iris"/"ngiris"/"mengiris tangan"; "sayat" and "silet" are cuts made on
// purpose). It never applies to the wrists, veins, nadi, an arm or a neck,
// or to a phrase with a negating word ("I didn't cut myself accidentally",
// "I cut myself not accidentally"); a risk signal anywhere in the text keeps
// the match too (see markersApply).
func accidental(s string, start, end int) bool {
	m := s[start:end]
	var after *regexp.Regexp
	verb := firstWord(m)
	switch {
	case strings.HasSuffix(m, "myself"):
		switch verb {
		case "cut":
			after = cutAccidentRe
		case "hurt", "injured":
			after = hurtAccidentRe
		case "burned", "burnt":
			after = burnAccidentRe
		default:
			return false
		}
	case idSliceVerbs[verb] && strings.Contains(m, "tangan") && !strings.Contains(m, "pergelangan"):
		after = idAccidentRe
	default:
		return false
	}
	lo, hi := localWindow(s, start, end)
	if negatorRe.MatchString(s[lo:hi]) {
		return false
	}
	return accidentBefore(s[lo:start]) || after.MatchString(s[end:hi])
}

// idSliceVerbs slice by accident as well as on purpose ("ngiris bawang").
var idSliceVerbs = setOf("iris ngiris mengiris")

// accidentBefore reports whether an accident adverb ends w, past up to two
// subject words ("I accidentally cut myself", "nggak sengaja ngiris tangan",
// "tanpa sengaja aku ...").
func accidentBefore(w string) bool {
	p := len(w)
	for n := 0; n <= 2; n++ {
		tok, ts := tokenBefore(w, p)
		switch {
		case tok == "accidentally" || tok == "accidently":
			return true
		case tok == "sengaja":
			neg, _ := tokenBefore(w, ts)
			return idNegators[neg]
		case !accidentSubjects[tok]:
			return false
		}
		p = ts
	}
	return false
}

var accidentSubjects = setOf("i aku gue gw gua saya tadi")

var (
	// The accident markers right after an injury.
	cutAccidentRe  = regexp.MustCompile(`^\s+(?:shaving|(?:while|when)\s+(?:i\s+was\s+|i'?m\s+)?(?:shaving|cooking|chopping|slicing|peeling|dicing|carving|gardening|cutting\s+[a-z]+|opening\s+[a-z]+|making\s+[a-z]+|fixing\s+[a-z]+|washing\s+[a-z]+|cleaning\s+[a-z]+)|on\s+(?:a\s+|the\s+|some\s+)?(?:paper|tin\s+can|can|broken\s+glass|glass|kitchen\s+knife|nail|fence)|by\s+accident|on\s+accident|accidentally|accidently)\b`)
	hurtAccidentRe = regexp.MustCompile(`^\s+(?:at\s+(?:the\s+)?(?:gym|practice|training)|working\s+out|lifting|running|falling|playing\s+[a-z]+|(?:while|when)\s+(?:i\s+was\s+|i'?m\s+)?(?:working\s+out|playing|running|lifting|exercising|training|skating|cycling|biking|hiking|climbing|dancing|moving\s+[a-z]+|cleaning|gardening|cooking|surfing|skiing)|in\s+(?:a|the)\s+(?:fall|accident|crash|game|match)|during\s+(?:practice|training|the\s+game|a\s+game|the\s+match|a\s+match|football|soccer|basketball|futsal|pe)|by\s+accident|on\s+accident|accidentally|accidently)\b`)
	burnAccidentRe = regexp.MustCompile(`^\s+(?:(?:while|when)\s+(?:i\s+was\s+|i'?m\s+)?(?:cooking|ironing|baking|frying|grilling|making\s+[a-z]+)|on\s+(?:the\s+|a\s+|my\s+)?(?:stove|oven|pan|iron|kettle|grill|stovetop|hob)|by\s+accident|on\s+accident|accidentally|accidently)\b`)
	// A bare "lagi"/"lg" is not a marker: after the injury it is "again"
	// ("ngiris tangan lagi bikin aku tenang"; review finding).
	idAccidentRe = regexp.MustCompile(`^(?:ku|nya)?\s+(?:pas|waktu|wkt|saat|ketika|sewaktu|sambil)\s+(?:lagi\s+|lg\s+|sedang\s+)?(?:masak|memasak|motong|memotong|potong|ngupas|mengupas|kupas|cukur|nyukur|mencukur|ngiris\s+[a-z]+|iris\s+[a-z]+|bikin\s+[a-z]+)\b|^(?:ku|nya)?\s+(?:nggak|ngga|gak|ga|tidak|tak|enggak|engga|gk)\s+sengaja\b`)
	// negatorRe: a negating word in the phrase keeps the match ("I didn't
	// cut myself accidentally", "I cut myself not accidentally").
	negatorRe = mustGuarded(`\b(?:not|never|didn'?t|did\s+not|wasn'?t|was\s+not|bukan)\b`)
)

// deliberate reports whether w has a deliberate word ("on purpose",
// "deliberately", "intentional", "no accident", "not an accident") or an
// Indonesian "sengaja"/"disengaja" (on purpose) that no negator precedes
// ("aku sengaja nyayat tanganku", "sebenernya disengaja", but not "nggak
// sengaja", "tidak disengaja" or "tanpa sengaja").
func deliberate(w string) bool {
	if deliberateRe.MatchString(w) {
		return true
	}
	for off := 0; ; {
		k := strings.Index(w[off:], "sengaja")
		if k < 0 {
			return false
		}
		i := off + k
		off = i + len("sengaja")
		if i >= 2 && w[i-2:i] == "di" && (i == 2 || !isLetter(w[i-3])) {
			i -= 2 // "disengaja"
		}
		if (i > 0 && isLetter(w[i-1])) || (off < len(w) && isLetter(w[off])) {
			continue
		}
		if prev, _ := tokenBefore(w, i); !idNegators[prev] || bukanBefore(w, i) {
			return true
		}
	}
}

// bukanBefore reports whether "bukan" negates the negated "sengaja" at i
// ("bukan nggak sengaja": not by accident).
func bukanBefore(w string, i int) bool {
	_, ns := tokenBefore(w, i)
	b, _ := tokenBefore(w, ns)
	return b == "bukan"
}

var deliberateRe = mustGuarded(`\b(?:on\s+purpose|purposely|purposefully|deliberate|deliberately|intentional|intentionally|meant\s+to|no\s+accident|` +
	`(?:not|wasn'?t|isn'?t|was\s+not|is\s+not|never)\s+(?:really\s+)?(?:an?\s+)?accident(?:al|ally)?)\b`)

var idNegators = setOf("nggak ngga gak ga tidak tak ndak enggak engga gk g tanpa")

// selfHarmIdiom rejects the fixed idioms en-self-harm's "<verb> myself" also
// spells: "burning myself out", "cut myself off from" (not "off and on"),
// "cutting myself some slack", "cut myself a break". Accidents are its
// context marker (see accidental).
func selfHarmIdiom(s string, start, end int) bool {
	m := s[start:end]
	if strings.HasSuffix(m, "myself") {
		verb := firstWord(m)
		w := nextWords(s, end, 3)
		switch {
		case strings.HasPrefix(verb, "burn"):
			if w[0] == "out" && w[1] != "of" {
				return true
			}
		case strings.HasPrefix(verb, "cut"):
			switch w[0] {
			case "off":
				if w[1] != "and" {
					return true
				}
			case "slack", "short", "loose", "free":
				return true
			case "some":
				if w[1] == "slack" {
					return true
				}
			case "a":
				if w[1] == "break" || w[1] == "slice" || w[1] == "piece" || w[1] == "deal" {
					return true
				}
			}
		}
	}
	return false
}

// lethalIdiom rejects "shooting myself in the foot" (self-sabotage) and
// "drown myself in work/coffee/my sorrows" (overwork, overindulgence): the
// noun after "in" must be a listed idiom noun, so drowning in water of any
// kind ("in the nearest river", "in cold water") keeps the match, and so does
// a phrase that carries an intent, modal or habit word ("I want to drown
// myself in work").
func lethalIdiom(s string, start, end int) bool {
	m := s[start:end]
	w := nextWords(s, end, 4)
	idiom := false
	switch {
	case strings.HasPrefix(m, "shoot"):
		idiom = w[0] == "in" && w[1] == "the" && (w[2] == "foot" || w[2] == "feet")
	case strings.HasPrefix(m, "drown"):
		if w[0] == "in" {
			k := 1
			for k < 3 && drownSkip[w[k]] {
				k++
			}
			idiom = drownIdioms[w[k]]
		}
	}
	return idiom && !carriesRisk(s, start, end)
}

var (
	drownSkip   = setOf("my the a an all this that some so much endless more")
	drownIdioms = setOf("work homework schoolwork assignments assignment tasks deadlines projects paperwork emails " +
		"studies books music coffee tea caffeine alcohol booze wine beer liquor vodka whiskey drinks sorrows sorrow " +
		"tears grief self-pity debt debts bills games videogames netflix series anime food snacks junk sugar " +
		"chocolate distractions busywork chores responsibilities perfume cologne sauce details love")
)

// cuttingObject rejects en-self-harm-ongoing's "cutting" when a fixed idiom
// or an everyday object completes it: "been cutting carbs", "started cutting
// back on coffee", "cutting down on sugar", "keep cutting corners", "been
// cutting class", "cutting in line", "cutting it close", "cutting off toxic
// people", "cutting my hair". A bare "been cutting", "cutting again",
// "cutting in high school", "cutting up again" and "cutting off and on" stay
// crisis.
func cuttingObject(s string, start, end int) bool {
	if !strings.HasSuffix(s[start:end], "cutting") {
		return false
	}
	w := nextWords(s, end, 2)
	switch w[0] {
	case "back", "down", "out", "up":
		return w[1] == "on" || cutObjects[w[1]] || (w[0] != "up" && w[1] == "the")
	case "in":
		return w[1] == "line" || w[1] == "lines" || w[1] == "queue"
	case "it":
		return w[1] == "close" || w[1] == "fine"
	case "off":
		return cutOffObjects[w[1]]
	}
	return cutObjects[w[0]] || (dyeOwners[w[0]] && trimObjects[w[1]])
}

// trimObjects are what "cutting my/the ..." trims rather than injures.
var trimObjects = setOf("hair bangs fringe beard nails toenails fingernails grass lawn hedge hedges budget costs")

var cutObjects = setOf("corners corner class classes school carbs carb sugar sugars coffee caffeine calories meat " +
	"gluten dairy alcohol booze junk snacks onions onion vegetables veggies wood paper grass weight costs cost " +
	"coupons ties expenses spending spend bread meat fat salt soda")

// cutOffObjects are who or what "cutting off" separates from ("cutting off
// toxic people").
var cutOffObjects = setOf("people toxic contact contacts friends everyone someone him her them ties my the communication")

// asPrescribed is the context marker of en-overdose-means: "took all (of)
// my pills" (no other quantity) followed in its phrase by a marker that the
// medication was taken as prescribed ("like the doctor said", "as
// prescribed", "sesuai resep"). A
// risk signal anywhere in the text keeps the match ("..., all of them at
// once so I won't wake up", "..., then I swallowed the rest"; see
// markersApply). A time is not a marker ("on time", "this morning"):
// overdose disclosures carry one too.
func asPrescribed(s string, start, end int) bool {
	m := s[start:end]
	if !strings.HasPrefix(m, "took") && !strings.HasPrefix(m, "taken") && !strings.HasPrefix(m, "swallowed") &&
		!strings.HasPrefix(m, "just") {
		return false
	}
	// Only "all (of) my pills" is an adherence report; "a lot of", "too
	// many", "a whole bottle of" or "handfuls of" pills never is.
	if w := strings.Fields(m); len(w) < 2 || (w[1] != "all" && (len(w) < 3 || w[2] != "all")) {
		return false
	}
	_, hi := localWindow(s, start, end)
	return adherenceRe.MatchString(s[end:hi])
}

var adherenceRe = mustGuarded(`\b(?:as\s+prescribed|as\s+directed|(?:like|as)\s+(?:the|my)\s+(?:doctor|doc|dr|psychiatrist|nurse|pharmacist)\s+(?:said|told\s+me|told|ordered|prescribed|recommended|instructed|asked)|doctor'?s\s+orders|sesuai\s+(?:resep|anjuran|dosis|aturan|petunjuk))\b`)

// maxMarkerText bounds, in bytes, the texts in which a context marker may
// discard a match. Markers are for short messages; in a longer text (a
// story body) a risk signal may sit anywhere, and the match is kept.
const maxMarkerText = 1000

// markersApply reports whether context markers may discard matches in the
// normalised text t. The gate is closed by default: t must be at most
// maxMarkerText bytes long, carry no risk signal (see riskSignal) and no
// quotation mark (scare quotes: "I 'accidentally' cut myself"), and every
// word of it outside the matches of the marker patterns must come from a
// closed benign vocabulary (see benignRest). So any continuation the
// vocabulary does not list keeps the match, whatever it says ("it helped",
// "bikin nagih", "all 30 of them", "udah waktunya", "bye everyone").
func markersApply(t string) bool {
	return len(t) <= maxMarkerText && !riskSignal(t) && !hasQuoteMark(t) && benignRest(t)
}

// hasQuoteMark reports whether t has a double quote, or a single quote that
// is not an apostrophe between two letters ("doctor's", "i'm").
func hasQuoteMark(t string) bool {
	for i := 0; i < len(t); i++ {
		switch t[i] {
		case '"':
			return true
		case '\'':
			if i == 0 || i+1 == len(t) || !isLetter(t[i-1]) || !isLetter(t[i+1]) {
				return true
			}
		}
	}
	return false
}

// markerPatterns are the catalogue patterns that have a context marker.
var markerPatterns = func() []*pattern {
	var out []*pattern
	for i := range catalogue {
		for _, p := range catalogue[i].patterns {
			if p.marker != nil {
				out = append(out, p)
			}
		}
	}
	return out
}()

// benignRest reports whether every word of t, once the matches of the marker
// patterns ("cut myself", "took all my pills", "ngiris tangan", "surat
// wasiat") and the benign phrases (benignPhraseRe: "not a big deal", "nggak
// sengaja", "pas lagi") are blanked out, is a benign word (benignWords:
// pronouns and articles, the accident activities, places and objects, times,
// first aid and pain, the adherence and legal words, particles, laughter).
// A digit or any other symbol is not benign ("all 30 of them", an emoji).
func benignRest(t string) bool {
	b := []byte(t)
	blank := func(loc []int) {
		for i := loc[0]; i < loc[1]; i++ {
			b[i] = ' '
		}
	}
	for _, p := range markerPatterns {
		for _, loc := range p.re.FindAllStringIndex(t, -1) {
			blank(loc)
		}
	}
	for _, loc := range benignPhraseRe.FindAllStringIndex(string(b), -1) {
		blank(loc)
	}
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case isLetter(c):
			j := i
			for j < len(b) && (isLetter(b[j]) || b[j] == '\'' || b[j] == '-') {
				j++
			}
			if w := string(b[i:j]); !benignWords[w] && !laughter(w) {
				return false
			}
			i = j
		case strings.IndexByte(" \t\n.,!?;:()-", c) >= 0:
			i++
		default:
			return false
		}
	}
	return true
}

var (
	// benignPhraseRe are phrases whose words are not benign on their own: a
	// reassurance ("not a big deal", "I'm fine", "gapapa"), a negated
	// "sengaja" (by accident) and "pas lagi" (while; a bare "lagi" is
	// "again").
	benignPhraseRe = mustGuarded(`\b(?:(?:not|no)\s+(?:a\s+)?big\s+deal|(?:it'?s|i'?m|im|i\s+am)\s+(?:fine|ok|okay|alright|all\s+right)|` +
		`(?:nggak|ngga|gak|ga|g|tidak|tak|enggak|engga|gk|tanpa)\s+(?:di)?sengaja|(?:nggak|ngga|gak|ga|tidak|enggak|engga|gk)\s*(?:apa-?apa|papa|parah)|gapapa|gpp|` +
		`(?:pas|waktu|wkt|saat|ketika|sewaktu|sambil)\s+(?:lagi|lg|sedang))\b`)
	// benignWords is the closed vocabulary of benign words around a context
	// marker (see benignRest). It has no intent, habit, quantity, feeling,
	// farewell or method word, and no "again", "too", "lagi", "juga".
	benignWords = setOf("i i'm im i've ive me my the a an at on in with while when was it it's its this and from " +
		"for during after before just now today yesterday morning afternoon evening earlier " +
		// accidents and their activities, places and objects
		"accidentally accidently accident by oops ouch ow oof ugh shaving cooking chopping slicing peeling dicing " +
		"carving gardening opening making fixing washing cleaning working out workout playing running lifting " +
		"exercising training practice skating cycling biking hiking climbing dancing moving surfing skiing ironing " +
		"baking frying grilling football soccer basketball futsal volleyball tennis badminton rugby hockey pe game " +
		"match dinner lunch breakfast food onion onions vegetables veggies carrots potatoes apples bread meat paper " +
		"glass broken can tin nail fence stove oven pan iron kettle grill stovetop hob kitchen knife gym field " +
		"court park home weights " +
		// first aid and pain
		"little bit small tiny minor bleeding bled bandage band-aid bandaid plaster hurts stings sore " +
		// medication taken as prescribed
		"like as doctor doc dr psychiatrist nurse pharmacist said told ordered prescribed recommended instructed " +
		"asked directed doctor's orders yes yep already " +
		// laughter (see also laughter)
		"lol lmao haha hehe " +
		// Indonesian: subjects, times, accident activities, objects, first aid
		"aku saya gue gw gua tadi kemarin kemaren pagi siang sore barusan hari ini pas waktu wkt saat ketika " +
		"sewaktu sambil masak memasak motong memotong potong ngupas mengupas kupas cukur nyukur mencukur bikin " +
		"bawang sayur sayuran buah daging wortel kentang tomat cabe cabai apel roti kue pisau dapur dikit sedikit " +
		"kecil luka berdarah perih sakit banget plester hansaplast betadine sih deh nih dong ya kok aduh duh waduh " +
		"dokter sesuai resep anjuran dosis aturan petunjuk " +
		// a legal will
		"gimana bagaimana cara buat membuat menulis nulis yang sah notaris bantu membantu warisan waris pembagian " +
		"harta hukum legal pengacara akta contoh format template draf draft apa apakah untuk di ke dari pakai pake " +
		"dengan dan atau itu")
)

// riskSignal reports whether t carries a word that keeps a match a context
// marker would discard: an intent, modal or habit word (see hasRiskWord), a
// deliberate word (see deliberate), or a purpose, motive, feeling, method or
// farewell word (signalWords, signalRe: "because", "to feel", "so I",
// "liked", "deserve", "whenever", "biar", "soalnya", "lega", "tenang",
// "wrists", "traffic", "the rest", "at once", "wake", "pamit", "selamanya").
// "when" counts unless an accident activity follows it ("when I was
// cooking" is the marker itself; "when I'm sad" is a habit).
func riskSignal(t string) bool {
	if hasRiskWord(t) || deliberate(t) || signalRe.MatchString(t) {
		return true
	}
	for i := 0; i < len(t); {
		if !isLetter(t[i]) || (i > 0 && (isLetter(t[i-1]) || t[i-1] == '\'')) {
			i++
			continue
		}
		j := i
		for j < len(t) && (isLetter(t[j]) || t[j] == '\'' || t[j] == '-') {
			j++
		}
		switch tok := t[i:j]; {
		case signalWords[tok]:
			return true
		case tok == "when":
			w := nextWords(t, j, 3)
			k := 0
			for k < 2 && (w[k] == "i" || w[k] == "i'm" || w[k] == "im" || w[k] == "was") {
				k++
			}
			if !accidentActivities[w[k]] {
				return true
			}
		}
		i = j
	}
	return false
}

var (
	signalWords = setOf("because cause 'cause cuz coz bc bcs deserve deserved deserves deserving punish punished " +
		"punishing punishment feel feels felt feeling relief relieved release calm calmer numb liked loved enjoy " +
		"enjoyed enjoying satisfying addicted addiction addictive crave craving whenever sad sadness depressed " +
		"depression hopeless worthless empty lonely stressed overwhelmed forever goodbye wake traffic wrist wrists " +
		"vein veins rope noose overdose od rest whole entire supply bottle bottles extra double hope hoping die " +
		"dead suicide suicidal kill killing " +
		"soalnya karena karna krn sebab gara-gara gegara rasanya lega tenang plong puas pantas pantes sedih " +
		"kesel kesal stres stress depresi hampa ketagihan candu menghukum selamanya pamit terakhir terakhirku " +
		"nadi urat sebotol botol sekaligus semuanya overdosis racun mati bunuh")
	signalRe = mustGuarded(`\b(?:so\s+(?:i|i'?d|i'?ll|i'?m|that|it)|to\s+(?:feel|punish|forget|cope|calm|numb|distract|release|relieve|stop|see\s+(?:if|how|what))|` +
		`(?:like|love)\s+(?:it|that|the\s+(?:pain|feeling|blood|sting))|each\s+time|(?:i'?m|i\s+am|im)\s+(?:so\s+|just\s+)?done|` +
		`at\s+once|all\s+of\s+them|more\s+than|in\s+front\s+of|into\s+(?:the\s+)?(?:road|street|traffic)|bangun\s+lagi|selamat\s+tinggal)\b`)
	// accidentActivities may follow "when" in an accident marker ("when I
	// was cooking").
	accidentActivities = setOf("shaving cooking chopping slicing peeling dicing carving gardening cutting opening " +
		"making fixing washing cleaning working playing running lifting exercising training skating cycling biking " +
		"hiking climbing dancing moving surfing skiing ironing baking frying grilling")
)

// kmsNotSlang rejects "kms" that is a distance or the child health card
// (Kartu Menuju Sehat). A distance needs a number or a quantity word before
// it ("5 kms", "a few kms away", "hundreds of kms", "berapa kms"); the card
// needs a card word or a child's name for it next to it ("buku KMS", "KMS
// anakku", but not "kms anak-anak") or a health-post
// word in the phrase ("bawa KMS ke posyandu"). An intent word that governs
// it keeps the slang ("i want to kms", "pengen kms dari kemarin").
func kmsNotSlang(s string, start, end int) bool {
	if riskGoverns(s, start) {
		return false
	}
	j := start
	for j > 0 && s[j-1] == ' ' {
		j--
	}
	if j > 0 && isDigit(s[j-1]) {
		// A number after a time word is a time, not a distance ("exam at
		// 8 kms"; review finding).
		d := j
		for d > 0 && (isDigit(s[d-1]) || s[d-1] == ':' || s[d-1] == '.') {
			d--
		}
		if w, _ := tokenBefore(s, d); !kmTimeWords[w] {
			return true
		}
	}
	prev, ps := tokenBefore(s, start)
	if kmQuantity[prev] {
		return true
	}
	if prev == "of" {
		if q, _ := tokenBefore(s, ps); kmQuantity[q] || q == "lots" || q == "couple" {
			return true
		}
	}
	if prev == "buku" || prev == "kartu" || prev == "card" {
		return true
	}
	if next := nextWords(s, end, 2); cardOwners[next[0]] || (cardChildren[next[0]] && ownerPronouns[next[1]]) {
		return true
	}
	lo, hi := localWindow(s, start, end)
	return healthPostRe.MatchString(s[lo:hi])
}

// kmTimeWords before a number make it a time ("at 8 kms").
var kmTimeWords = setOf("at jam pukul by until till")

var (
	kmQuantity = setOf("few many several hundreds thousands hundred thousand dozens tens berapa beberapa ratusan " +
		"puluhan ribuan bbrp more less extra total couple")
	// The card belongs to a child: "KMS anakku", "KMS anak saya".
	cardOwners    = setOf("anakku anaknya anakmu bayiku bayinya bayimu balitaku balitanya adekku adeknya adikku adiknya")
	cardChildren  = setOf("anak bayi balita adek adik")
	ownerPronouns = setOf("aku saya gue gw gua ku kamu dia")
	healthPostRe  = mustGuarded(`\b(?:posyandu|puskesmas|bidan|imunisasi|penimbangan|timbang|kia)\b`)
)

// dyeObject rejects "die" spelt for "dye": "I need to die my hair black",
// "she wants to die her hair", "want to die it red". The hair noun must end
// the phrase or be followed by a colour or a time; a verb after it starts a
// new clause ("I want to die my hair is falling out" keeps the match).
func dyeObject(s string, start, end int) bool {
	if !strings.HasSuffix(s[start:end], "die") {
		return false
	}
	w := nextWords(s, end, 3)
	switch {
	case dyeOwners[w[0]]:
		return dyeTargets[w[1]] && (w[2] == "" || colours[w[2]] || dyeFollow[w[2]])
	case w[0] == "it" || w[0] == "them":
		return colours[w[1]]
	}
	return false
}

var (
	dyeOwners  = setOf("my her his your their the our")
	dyeTargets = setOf("hair beard eyebrows eyebrow roots clothes shirt shirts jeans shoes nails")
	colours    = setOf("black blonde blond red blue pink purple green brown white grey gray silver orange")
	dyeFollow  = setOf("for again today tomorrow tonight soon this next back")
)

// vanishPlace rejects id-vanish-forever when "selamanya" is followed by a
// place, an online space or someone else's life: leaving social media, a
// group or a partner ("pengen ngilang selamanya dari sosmed", "dari grup
// itu", "dari hidupnya") is not a death wish. Anything else keeps the match:
// the world ("dari dunia ini"), a time ("dari kemarin") and people ("dari
// kalian").
func vanishPlace(s string, _, end int) bool {
	w := nextWords(s, end, 3)
	if w[0] != "dari" {
		return false
	}
	switch {
	case vanishPlaces[w[1]]:
		return true
	case w[1] == "dunia" && w[2] == "maya", w[1] == "media" && w[2] == "sosial":
		return true
	case w[1] == "hidup" || w[1] == "kehidupan":
		return otherOwners[w[2]]
	case strings.HasPrefix(w[1], "hidup") || strings.HasPrefix(w[1], "kehidupan"):
		return strings.HasSuffix(w[1], "nya") || strings.HasSuffix(w[1], "mu")
	}
	return false
}

var (
	vanishPlaces = setOf("sosmed medsos socmed internet online grup group gc chat percakapan circle tongkrongan " +
		"instagram ig twitter tiktok facebook fb wa whatsapp line discord telegram youtube timeline tl kelas kantor " +
		"sekolah kampus kosan kos kota kampung rumah radar peredaran hadapannya hadapanmu pandangannya")
	otherOwners = setOf("dia kamu mereka lo lu elo doi dirinya dirimu orang")
)

// ownEuphemism rejects an id-sleep-forever match that v1
// first-person-euphemism already counts ("aku pengen tidur selamanya"), so
// the same phrase is not reported twice.
func ownEuphemism(s string, start, _ int) bool {
	_, ps := tokenBefore(s, start)
	return ps < start && v1Euphemism.MatchString(s[ps:min(len(s), start+checkWindow)])
}

// v1Euphemism is v1 first-person-euphemism anchored at a pronoun.
var v1Euphemism = regexp.MustCompile(`^(?:aku|saya|gue|gw)\s+(?:ingin|mau|pengen|akan)\s+(?:tidur\s*selamanya|menghilang\s*selamanya|ga\s*bangun\s*lagi)\b`)

// mixedIdiom is the post-check of mixed-death-wish: for "die", the idiom
// checks of death-wish-phrase ("hp-ku mau die"), "die" spelt for "dye", and
// English compounds that begin with "die" ("mau die cast", "pengen die hard
// fans"). A device cannot "unalive" or "kill myself", so the other
// branches have no subject check ("mati lampu mau unalive aja" counts).
func mixedIdiom(s string, start, end int) bool {
	if !strings.HasSuffix(s[start:end], "die") {
		return false
	}
	if dieCompounds[nextWords(s, end, 1)[0]] {
		return true
	}
	return idiomaticDeath(s, start, end) || dyeObject(s, start, end)
}

var dieCompounds = setOf("cast casting casts hard hardfan hardfans cut")

// circumstantialLifeEn rejects "tired of living" when a neutral
// circumstance follows ("tired of living like this", "sick of life in the
// city", "tired of living with my parents", "sick of life's drama"). Pain is
// not a circumstance: "tired of living in pain" and "tired of living with
// this depression" keep the match.
func circumstantialLifeEn(s string, _, end int) bool {
	if strings.HasPrefix(s[end:], "'s") {
		return true
	}
	w := nextWords(s, end, 5)
	if !lifeCircumstance[w[0]] {
		return false
	}
	for _, x := range w[1:] {
		if painWords[x] {
			return false
		}
		if !painSkip[x] {
			break
		}
	}
	return true
}

var (
	lifeCircumstance = setOf("like in with here there paycheck alone on at under as off for from out without near " +
		"by abroad overseas together admin")
	painWords = setOf("pain agony misery suffering depression anxiety sadness grief fear darkness trauma illness " +
		"sickness hell torment loneliness emptiness guilt shame struggle struggles hurt ptsd bpd bipolar despair")
	painSkip = setOf("this my the so much constant chronic all such endless daily this")
)
