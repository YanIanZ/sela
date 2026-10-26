package safety

import (
	"regexp"
	"strings"
)

// Post-checks added in ruleset sela-safety-2026.10.26-3. Each one receives
// the sentence s and a match s[start:end] and looks at a bounded window
// around the match only (see localWindow), so that checking every match
// stays linear in the sentence length. Every decision is recorded as a
// vector in testdata/vectors.json.
//
// There are two kinds. A fixed idiom is a set phrase that completes the
// match and has no self-harm reading ("cut myself off from", "cutting carbs",
// "mati lampu", "gol bunuh diri"); it is rejected whatever surrounds it. A
// context marker is a word near the match that suggests a benign reading
// ("at the gym", "like the doctor said", "5 kms", "posyandu"). A context
// marker never rejects a match whose phrase carries an intent, modal or habit
// word (see carriesRisk), and the accident check never applies to a
// self-directed method (wrists, veins, nadi, a cut arm): the safer path wins.

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
// "will", "should", "keep", "again", "pengen", "mau", "sering", "biar". A
// context-marker post-check never rejects such a match ("I want to hurt
// myself at the gym", "aku sering iris tangan pas lagi main hp").
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
// ("suicide sprints") and a car door ("suicide doors"). A compound noun after
// the match is not read as one when an intent word governs the match or a
// pronoun follows the noun ("aku mau bunuh diri karier aku hancur").
func suicideCompound(s string, start, end int) bool {
	m := s[start:end]
	prev, _ := tokenBefore(s, start)
	next := nextWords(s, end, 2)
	after := func(set map[string]bool) bool {
		return set[next[0]] && !compoundPronouns[next[1]] && !riskGoverns(s, start)
	}
	switch {
	case strings.HasPrefix(m, "bunuh"):
		return idCompoundBefore[prev] || after(idCompoundAfter)
	case m == "suicide":
		return enCompoundBefore[prev] || after(enCompoundAfter)
	}
	return false
}

// compoundPronouns after a compound noun start a new clause ("bunuh diri
// karier aku hancur").
var compoundPronouns = setOf("aku saya gue gw gua ku dia ia kamu lo lu i i'm im he she they it")

// suicideNews attributes a news compound to a third party: a suicide bombing
// ("bom bunuh diri", "pelaku bom bunuh diri", "suicide bomber") reports
// someone else's act, so it routes to elevated like a news frame.
func suicideNews(s string, start, end int) bool {
	m := s[start:end]
	switch {
	case strings.HasPrefix(m, "bunuh"):
		prev, _ := tokenBefore(s, start)
		return idNewsBefore[prev]
	case m == "suicide":
		return enNewsAfter[nextWords(s, end, 1)[0]]
	}
	return false
}

var (
	idCompoundBefore = setOf("gol misi")
	idCompoundAfter  = setOf("karier karir politik")
	enCompoundBefore = setOf("career social political financial commercial electoral professional corporate brand")
	enCompoundAfter  = setOf("squad mission missions sprint sprints door doors squeeze run runs drill drills king kings " +
		"burpees shuttle shuttles lane")
	idNewsBefore = setOf("bom serangan pelaku pengebom peledakan ledakan teror teroris")
	enNewsAfter  = setOf("bomber bombers bombing bombings bomb bombs attack attacks attacker attackers vest vests blast blasts")
)

// legalWill rejects "surat wasiat" that is a legal will: a relative as its
// author ("kakek sudah bikin surat wasiat"; see relativeAuthor), or a legal
// word in a phrase with
// no intent, modal or habit word ("cara bikin surat wasiat yang sah", "untuk
// pembagian warisan", "notaris"), when the phrase has no first-person word. A
// farewell word or a first-person author keeps the match ("surat wasiat
// terakhirku", "aku udah nulis surat wasiat yang sah buat keluargaku").
func legalWill(s string, start, end int) bool {
	if !strings.HasPrefix(s[start:end], "surat") {
		return false
	}
	lo, hi := localWindow(s, start, end)
	w := s[lo:hi]
	switch {
	case firstPerson(w) || farewellRe.MatchString(w):
		return false
	case relativeAuthor(s, lo, start):
		return true
	}
	return legalRe.MatchString(w) && !hasRiskWord(w)
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

// accidental rejects an injury that its phrase says was an accident: "I
// accidentally cut myself while cooking", "I cut myself shaving", "I hurt
// myself at the gym", "nggak sengaja ngiris tangan pas potong bawang". The
// marker must sit right before the match or right after it, and the match
// must be an injury that can happen by accident: "hurt/cut/burn myself" or an
// Indonesian cut hand. It never applies to the wrists, veins, nadi, an arm or
// a neck, to a phrase that carries an intent, modal or habit word (see
// carriesRisk), or to a phrase with a deliberate or negating word ("on
// purpose", "not accidentally", "didn't").
func accidental(s string, start, end int) bool {
	m := s[start:end]
	var after *regexp.Regexp
	verb := firstWord(m)
	switch {
	case strings.HasSuffix(m, "myself"):
		switch {
		case strings.HasPrefix(verb, "cut"):
			after = cutAccidentRe
		case strings.HasPrefix(verb, "hurt"), strings.HasPrefix(verb, "injur"):
			after = hurtAccidentRe
		case strings.HasPrefix(verb, "burn"):
			after = burnAccidentRe
		default:
			return false
		}
	case strings.Contains(m, "tangan") && !strings.Contains(m, "pergelangan"):
		after = idAccidentRe
	default:
		return false
	}
	lo, hi := localWindow(s, start, end)
	w := s[lo:hi]
	if hasRiskWord(w) || deliberate(w) {
		return false
	}
	return accidentBefore(s[lo:start]) || after.MatchString(s[end:hi])
}

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
	idAccidentRe   = regexp.MustCompile(`^(?:ku|nya)?\s+(?:(?:pas|waktu|wkt|saat|ketika|sewaktu|sambil)\s+(?:lagi\s+|lg\s+|sedang\s+)?|lagi\s+|lg\s+)(?:masak|memasak|motong|memotong|potong|ngupas|mengupas|kupas|cukur|nyukur|mencukur|ngiris\s+[a-z]+|iris\s+[a-z]+|bikin\s+[a-z]+)\b|^(?:ku|nya)?\s+(?:nggak|ngga|gak|ga|tidak|tak|enggak|engga|gk)\s+sengaja\b`)
	// deliberateRe: a deliberate or negating word that keeps the match.
	deliberateRe = mustGuarded(`\b(?:on\s+purpose|purposely|deliberately|intentionally|not|never|didn'?t|did\s+not|wasn'?t|was\s+not|no\s+accident|bukan)\b`)
)

// deliberate reports whether phrase w has a deliberate or negating word (see
// deliberateRe) or an Indonesian "sengaja" (on purpose) that no negator
// precedes ("aku sengaja nyayat tanganku", but not "nggak sengaja").
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
		if (i > 0 && isLetter(w[i-1])) || (off < len(w) && isLetter(w[off])) {
			continue
		}
		if prev, _ := tokenBefore(w, i); !idNegators[prev] {
			return true
		}
	}
}

var idNegators = setOf("nggak ngga gak ga tidak tak ndak enggak engga gk g tanpa")

// selfHarmIdiom rejects the fixed idioms en-self-harm's "<verb> myself" also
// spells: "burning myself out", "cut myself off from" (not "off and on"),
// "cutting myself some slack", "cut myself a break"; and accidents (see
// accidental).
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
	return accidental(s, start, end)
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

// asPrescribed rejects an overdose-means "took all my pills" followed by a
// marker that the medication was taken as prescribed ("like the doctor
// said", "as prescribed", "sesuai resep"), unless the phrase carries an
// intent, modal, habit or deliberate word.
func asPrescribed(s string, start, end int) bool {
	m := s[start:end]
	if !strings.HasPrefix(m, "took") && !strings.HasPrefix(m, "taken") && !strings.HasPrefix(m, "swallowed") &&
		!strings.HasPrefix(m, "just") {
		return false
	}
	lo, hi := localWindow(s, start, end)
	w := s[lo:hi]
	return adherenceRe.MatchString(s[end:hi]) && !hasRiskWord(w) && !deliberate(w)
}

var adherenceRe = mustGuarded(`\b(?:as\s+prescribed|as\s+directed|(?:like|as)\s+(?:the|my)\s+(?:doctor|doc|dr|psychiatrist|nurse|pharmacist)\s+(?:said|told\s+me|told|ordered|prescribed|recommended|instructed|asked)|doctor'?s\s+orders|on\s+time|on\s+schedule|sesuai\s+(?:resep|anjuran|dosis|aturan|petunjuk))\b`)

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
		return true
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

// mixedIdiom is the post-check of mixed-death-wish: the idiom checks of
// death-wish-phrase ("hp-ku mau die"), "die" spelt for "dye", and English
// compounds that begin with "die" ("mau die cast", "pengen die hard fans").
func mixedIdiom(s string, start, end int) bool {
	if strings.HasSuffix(s[start:end], "die") && dieCompounds[nextWords(s, end, 1)[0]] {
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
