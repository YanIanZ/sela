package safety

import (
	"regexp"
	"strings"
)

// Post-checks added in ruleset sela-safety-2026.10.26-3. Each one receives
// the sentence s and a match s[start:end] and looks at a bounded window
// around the match only (see localWindow), so that checking every match
// stays linear in the sentence length (a wide pattern's checks also see the
// text right after the sentence; see pattern.wide); the context-marker gate
// scans the whole text once per variant, only texts of at most
// maxMarkerText bytes, and is open only when it is open on every variant
// (see markersApply and gateOpen). Every decision is recorded as a vector in
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

// wordAt is nextWords(s, i, 1)[0] without the allocation, for checks that
// run once per match of a frequent pattern (see shSlang).
func wordAt(s string, i int) string {
	lim := min(len(s), i+checkWindow)
	for i < lim && s[i] == ' ' {
		i++
	}
	k := i
	for k < lim && (isLetter(s[k]) || s[k] == '\'' || s[k] == '-') {
		k++
	}
	return s[i:k]
}

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

// sentenceReach bounds how far, in bytes, a check that reads the whole
// sentence looks on either side of a match (see sentenceWindow).
const sentenceReach = 2 * checkWindow

// sentenceWindow returns the sentence s around s[start:end] up to
// sentenceReach bytes each way, cut at word boundaries. Unlike localWindow
// it crosses commas and clause words ("I hope I'm not alive to see it, I
// can't keep going"); the bound keeps checking every match of a long
// sentence linear.
func sentenceWindow(s string, start, end int) string {
	lo, hi := max(0, start-sentenceReach), min(len(s), end+sentenceReach)
	for lo > 0 && lo < start && isLetter(s[lo-1]) {
		lo++
	}
	for hi < len(s) && hi > end && isLetter(s[hi]) {
		hi--
	}
	return s[lo:hi]
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
		return set[next[0]] && !compoundPronouns[next[1]] && !idiomGoverned(s, start, own)
	}
	// A compound noun before the match, or an English one after it, is a
	// figurative or named thing ("misi bunuh diri", "career suicide",
	// "suicide sprints"): only an intent word keeps it ("I'm on a suicide
	// mission at work" is an idiom). Before "bunuh diri karier" a
	// first-person subject keeps it too: "aku bunuh diri" is the core.
	switch {
	case strings.HasPrefix(m, "bunuh"):
		return (idCompoundBefore[prev] && !idiomGoverned(s, ps, false)) || after(idCompoundAfter, true)
	case m == "suicide":
		return (enCompoundBefore[prev] && !idiomGoverned(s, ps, false)) || after(enCompoundAfter, false)
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
		if idCompoundBefore[prev] {
			// "teroris itu siap jalanin misi bunuh diri": a compound that
			// only a third party's readiness governs is their act.
			_, third := compoundGovernor(s, ps, false)
			return third
		}
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
	gov, _ := compoundGovernor(s, at, own)
	return gov
}

// compoundGovernor is compoundGoverned that also reports whether the
// governing word is a readiness word with a third-party subject
// (readinessOf). The attribution checks (suicideNews) do not count such a
// readiness as the speaker's; the idiom checks (suicideCompound) count it,
// so that an attributed readiness never makes the compound an idiom
// ("teroris itu siap jalanin misi bunuh diri" is attributed, not none;
// review finding).
func compoundGovernor(s string, at int, own bool) (gov, third bool) {
	p := at
	for n := 0; n < 6; n++ {
		w, ws := tokenBefore(s, p)
		switch {
		case w == "":
			return false, false
		case readinessWords[w]:
			if thirdBefore(s, ws) {
				return false, true
			}
			return true, false
		case riskWords[w], strings.HasSuffix(w, "'ll"), w == "going", own && ownPronouns[w]:
			return true, false
		case w == "about":
			t, _ := tokenBefore(s, ws)
			return t == "thinking" || t == "thought" || t == "think", false
		case !compoundSkip[w] && !governSkip[w] && !idNewsBefore[w]:
			return false, false
		}
		p = ws
	}
	return false, false
}

// idiomGoverned is compoundGoverned for the idiom checks: any readiness
// word governs, whoever its subject (see compoundGovernor).
func idiomGoverned(s string, at int, own bool) bool {
	gov, third := compoundGovernor(s, at, own)
	return gov || third
}

// readinessWords ("siap", "rela", "bersedia": ready, willing) govern a
// compound like an intent word, but only for the speaker: after a
// third-party subject they report someone else's readiness ("teroris itu
// siap jadi pelaku bom bunuh diri", "mereka bersedia jadi pelaku ..."; review
// finding), while the speaker's own ("aku siap jadi pelaku ...") and a
// subjectless one count.
var readinessWords = setOf("siap rela bersedia")

// thirdBefore reports whether the subject that ends at s[:i], past aspect
// words, names someone other than the speaker: a third-person pronoun, a
// noun phrase ending in "itu"/"tersebut", a news or group noun ("teroris",
// "pelaku", "kelompok") or a relative ("temanku"). Any other word, or none,
// is read as the speaker's own subject, the safer path.
//
// The third party is not the subject, and the speaker's own reading is kept,
// when it is part of a subject that includes the speaker or the object of
// a preposition or a verb ("aku dan temanku siap ...", "demi mereka siap
// ...", "aku ikut mereka siap ...": a joining word up to three words before
// it), or when a first-person pronoun comes earlier in the clause (review
// finding). A comma right after the joining word does not end its phrase
// ("demi, mereka siap ...": a preposition needs its object, so the comma is
// read as a slip; review sweep).
func thirdBefore(s string, i int) bool {
	w, ws := tokenBefore(s, i)
	for n := 0; n < 3 && readinessAspect[w]; n++ {
		w, ws = tokenBefore(s, ws)
	}
	if !thirdPronouns[w] && !readinessThird[w] && !idNewsBefore[w] && !relativeNoun(w) {
		return false
	}
	for n, p := 0, ws; n < 8; n++ {
		t, ts := tokenBefore(s, p)
		if t == "" && n < 3 && ts > 0 && s[ts-1] == ',' {
			if j, _ := tokenBefore(s, ts-1); readinessJoin[j] {
				return false
			}
		}
		switch {
		case t == "" || conjunctions[t]:
			return true
		case ownPronouns[t] || (n < 3 && readinessJoin[t]):
			return false
		}
		p = ts
	}
	return true
}

// readinessJoin before a third party make it part of the speaker's subject
// or an object ("aku dan temanku", "demi mereka", "ikut mereka").
var readinessJoin = setOf("dan sama ama dengan dgn bersama bareng demi untuk utk buat ikut ikutan dari kayak seperti")

var readinessThird = setOf("itu tersebut orang seseorang kelompok pria wanita lelaki perempuan militan pemuda remaja " +
	"mahasiswa jaringan anggota tersangka")

var readinessAspect = setOf("udah sudah udh telah emang memang benar-benar bener-bener sangat juga jg pun pasti selalu " +
	"sudah sdh lebih")

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
		case "hit", "smacked":
			after = hitAccidentRe
		case "punched", "slapped":
			after = byAccidentRe
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
	// "I hit myself on the cabinet door", "... with the door" (review
	// finding): furniture and doors only. A wall is not listed: hitting a
	// wall is not an accident. An instrument or any other surface ("with a
	// hammer", "on the edge", "on my bed") is a marker only with an
	// accident word after it ("... with a hammer by accident").
	hitAccidentRe = regexp.MustCompile(`^\s+(?:on\s+(?:the\s+|a\s+|my\s+)?(?:cabinet|cupboard|door|doorframe|table|desk|corner|shelf|counter|chair)(?:\s+door)?|with\s+(?:the\s+|a\s+|my\s+)?(?:(?:cabinet|cupboard|car)\s+)?door|` +
		`(?:on|with)\s+(?:the\s+|a\s+|my\s+)?[a-z]{3,}\s+(?:by\s+accident|on\s+accident|accidentally|accidently)|by\s+accident|on\s+accident|accidentally|accidently)\b`)
	byAccidentRe   = regexp.MustCompile(`^\s+(?:by\s+accident|on\s+accident|accidentally|accidently)\b`)
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
// A digit or any other symbol is not benign ("all 30 of them", an emoji),
// and neither is a bracket: a wink or a smile (";)", ":)", ":-)") and a
// parenthesised accident word ("I (accidentally) cut myself") read as
// sarcasm, so they keep the match (review finding).
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
		case strings.IndexByte(" \t\n.,!?;:-", c) >= 0:
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
		"court park home weights cabinet cupboard door doorframe table desk corner shelf counter bed chair car edge " +
		"hammer racket ball bat " +
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
		// A number after a clock word is a time, not a distance ("exam at
		// 8 kms", "done by 9 kms", "jam 12 kms lagi"; review findings), and
		// then only "per" after it makes a speed ("at 5 kms per hour"): a
		// following "lagi" is "again", not "more" (review finding). "until"
		// and "till" measure a distance only after a motion verb ("I'll run
		// until 10 kms"); otherwise they name a time ("gonna wait until 10
		// kms").
		d := j
		for d > 0 && (isDigit(s[d-1]) || s[d-1] == ':' || s[d-1] == '.') {
			d--
		}
		w, ws := tokenBefore(s, d)
		after := nextWords(s, end, 1)[0]
		year := j-d == 4 && (strings.HasPrefix(s[d:j], "19") || strings.HasPrefix(s[d:j], "20"))
		switch {
		case year && kmYearWords[w]:
			// A year ("back to 2019 kms", "in 2020 kms") is not a distance.
		case kmTimeWords[w]:
			if kmsAway[after] {
				return true
			}
		case w == "until" || w == "till" || w == "til":
			if motionBefore(s, ws) || kmsAway[after] {
				return true
			}
		default:
			return true // a speed or a distance ("5 kms", "3 kms lagi")
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

// kmYearWords before a four-digit number make it a year.
var kmYearWords = setOf("to in since from until till tahun thn year")

// kmTimeWords before a number make it a time ("at 8 kms", "by 12 kms").
var kmTimeWords = setOf("at jam pukul by")

// motionBefore reports whether a motion verb is one of the three words
// before s[:i] ("run until 10 kms", "I'll keep cycling till 40 kms").
func motionBefore(s string, i int) bool {
	for n := 0; n < 3; n++ {
		w, k := tokenBefore(s, i)
		if w == "" {
			return false
		}
		if motionVerbs[w] {
			return true
		}
		i = k
	}
	return false
}

var motionVerbs = setOf("run runs running ran walk walks walking walked ride rides riding rode cycle cycles cycling cycled " +
	"bike bikes biking biked drive drives driving drove jog jogs jogging jogged swim swims swimming swam hike hikes hiking hiked " +
	"row rowing paddle paddling skate skating lari berlari jalan berjalan jalan-jalan gowes nggowes sepedaan bersepeda nyetir " +
	"renang berenang jogging")

// kmsAway after "<clock word> <number> kms" still make it a speed or a
// distance ("at 5 kms per hour", "at 3 kms away"); "lagi" (again) and
// "more" do not.
var kmsAway = setOf("per away from ahead behind further farther jauhnya")

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
	if w[0] != "dari" && w[0] != "dr" {
		return false // "dr" is the chat spelling (review finding)
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

// Post-checks added in ruleset sela-safety-2026.10.26-5.

// hopeDeathIdiom rejects an en-hope-to-die match whose "die" is spelt for
// "dye" ("I wish I could die my hair"), that hopes for a death of old age
// ("I hope I die of old age", "I hope I die old and happy"), whose "dead" is
// an intensifier ("I hope I'm dead wrong", "... dead serious") or whose
// "alive" is the idiom "not alive to see it" (also "that", "what", "how";
// a time, "the" or "this" after "see" is not the idiom: "... to see the
// morning"), unless an intent, modal or habit word comes with it anywhere
// in the sentence ("I hope I'll die of old age soon", "I hope I'm not alive
// to see it, I can't keep going" keep the match; review finding).
func hopeDeathIdiom(s string, start, end int) bool {
	if dyeObject(s, start, end) {
		return true
	}
	m := s[start:end]
	w := nextWords(s, end, 3)
	idiom := false
	sent := sentenceWindow(s, start, end)
	if gameDeath(s, sent, end) {
		// "I hope I die first in the game so I can spectate", "... don't
		// make it to the next round lol" (review finding).
		return !hasRiskWord(sent) && !deathAround(s, start, end)
	}
	switch {
	case strings.HasSuffix(m, "die"):
		idiom = (w[0] == "of" && w[1] == "old" && w[2] == "age") || w[0] == "old" ||
			(w[0] == "an" && w[1] == "old") || (w[0] == "at" && w[1] == "a" && w[2] == "ripe") ||
			(w[0] == "of" && dieOfIdioms[w[1]]) || oldAndDieRe.MatchString(m)
	case strings.HasSuffix(m, "dead"):
		idiom = deadIntensified[w[0]]
	case strings.HasSuffix(m, "see the sunrise") || strings.HasSuffix(m, "see sunrise") || strings.HasSuffix(m, "see the morning") ||
		strings.HasSuffix(m, "see morning") || strings.HasSuffix(m, "see daylight"):
		// "I hope I don't see the sunrise, I'm sleeping in all weekend".
		idiom = sleepInRe.MatchString(sent)
	case strings.HasSuffix(m, "alive"):
		idiom = w[0] == "to" && w[1] == "see" && seeObjects[w[2]]
	case strings.HasSuffix(m, "gone") || strings.HasSuffix(m, "here") || strings.HasSuffix(m, "around"):
		// Being away for a listed everyday occasion, trip or place ("I hope
		// I'm gone before traffic gets bad", "I hope I'm not here when the
		// movers come", "I wish I was gone on vacation already"; review
		// finding); "before they arrive" keeps the match (see goneOccasion).
		idiom = goneOccasion(s, end)
	case strings.HasSuffix(m, "make it"):
		// A bare "I hope I don't make it" is not surviving; a listed
		// everyday destination or occasion after it is not ("... make it to
		// the meeting", "... make it in time", "... make it on time", "...
		// make it for dinner"). Anything else keeps it: "at all", "with all
		// this pain", "in this world", "into tomorrow", "to friday", a time,
		// "home" or "there" (review finding: every preposition rejected it).
		idiom = makeItOccasion(nextWords(s, end, 5))
	}
	return idiom && !hasRiskWord(sent) && !deathAround(s, start, end)
}

// deathAround reports whether a death, method or permanence word stands in
// the sentence around s[start:end], outside the match ("I hope I don't see
// the sunrise, sleeping in forever", "... die in the game, I want real
// death"): a hope or wish idiom then keeps its match.
func deathAround(s string, start, end int) bool {
	lo, hi := max(0, start-sentenceReach), min(len(s), end+sentenceReach)
	return deathWordRe.MatchString(s[lo:start]) || deathWordRe.MatchString(s[end:hi])
}

var deathWordRe = mustGuarded(`\b(?:forever|for\s+good|permanently|eternally|never\s+wake|suicide|suicidal|kill|killing|dead|death|die|dying|overdose|pills)\b`)

// lifeEndsHappy rejects a hope that the speaker's life ends well ("semoga
// hidupku berakhir bahagia", "I hope my life ends happily", "semoga umurku
// panjang" is no match at all), unless a risk word is in the sentence.
func lifeEndsHappy(s string, start, end int) bool {
	return happyEndings[nextWords(s, end, 1)[0]] && !hasRiskWord(sentenceWindow(s, start, end))
}

// makeItOccasion reports whether the words w after a bare "make it" name a
// listed everyday destination or occasion: "to (the) work/class/meeting/
// party/...", "back to ...", "in time", "on time", "for dinner", "into the
// team", "onto the bus", "big", "work" ("make it work") or "up to you".
func makeItOccasion(w []string) bool {
	k := 1
	det := func() {
		if makeItDets[w[k]] {
			k++
		}
	}
	switch w[0] {
	case "big", "work":
		return true
	case "in", "on":
		if w[1] == "time" {
			return true
		}
		det()
		return makeItGroups[w[k]]
	case "back":
		if w[1] == "in" && w[2] == "time" {
			return true
		}
		if w[1] != "to" {
			return false
		}
		k = 2
		det()
		return makeItPlaces[w[k]]
	case "to", "for":
		det()
		return makeItPlaces[w[k]]
	case "into", "onto":
		det()
		return makeItGroups[w[k]] || makeItPlaces[w[k]]
	case "up":
		return w[1] == "to" && (w[2] == "you" || w[2] == "him" || w[2] == "her" || w[2] == "them" || w[2] == "u")
	}
	return false
}

var (
	makeItDets = setOf("the my our a an this that his her their your")
	// makeItPlaces are everyday destinations and occasions after "make it
	// to/for".
	makeItPlaces = setOf("work class classes school office meeting meetings party gym practice rehearsal interview " +
		"appointment event wedding airport station train bus flight plane dinner lunch breakfast brunch church exam " +
		"test quiz lecture finals final shift concert show match store mall movies cinema college campus uni " +
		"university library restaurant cafe beach park zoo recital ceremony conference reunion festival playoffs " +
		"semifinals semis tournament deadline kickoff start opening premiere session game games gig lesson lessons " +
		"date dates event events party's movie")
	// makeItGroups are what one makes it in, into or onto ("in the team",
	// "into the finals", "onto the list").
	makeItGroups = setOf("team band finals final top list roster squad club choir cast cut program programme news " +
		"photo picture paper papers playoffs semis semifinals group shot bus train plane")
	happyEndings = setOf("bahagia indah baik manis bagus sukses happily happy well beautifully peacefully")
)

var (
	deadIntensified = setOf("wrong serious tired last centre center asleep honest broke set silent quiet even straight sure")
	seeObjects      = setOf("it that what how")
	// dieOfIdioms: "die of happiness", "die of laughter".
	dieOfIdioms = setOf("happiness joy laughter laughing excitement embarrassment cringe cuteness boredom")
	// oldAndDieRe: "I hope I get old and die", "grow old and die".
	oldAndDieRe = regexp.MustCompile(`\b(?:get|got|grow|grew|getting|growing)\s+(?:very\s+|really\s+|super\s+)?(?:old|older|rich|famous|married|wrinkly|grey|gray)\s+and\s+(?:just\s+|then\s+)?die$`)
	sleepInRe   = regexp.MustCompile(`\b(?:sleep|sleeping|slept)\s+(?:in|through|until|till|late)\b|\boversleep(?:ing)?\b`)
	// gameNouns after "die in the ..." or "make it to the next ..." are a
	// game ("die first in the game", "the next round"), but only in a
	// sentence with a game word (gameWords): a round may be of chemo.
	gameNouns = setOf("game games round rounds level levels stage stages match matches wave waves raid lobby server boss tournament")
	gameLeads = setOf("the this that a my our")
	gameWords = setOf("game games gaming gamer tournament play playing played minecraft fortnite valorant roblox pubg " +
		"respawn respawning respawned spectate spectating lobby")
)

// gameDeath reports whether the death in a hope is a game's: the death
// happens in a game noun phrase right after the match, past at most two
// adverbs ("die first in the game", "die in this round", "make it to the
// next round"), with no "of", "called" or "named" after the noun ("the
// next round of chemo", "this game called life"), the sentence has a game
// word ("game", "tournament", "play", "spectate") and does not call life a
// game ("the game of life", "life is a game"). A game noun after a
// conjunction or "before" is not where the death happens ("I hope I die
// but the game is on", "... before the game"; review finding), and a game
// word elsewhere in the sentence alone is not enough.
func gameDeath(s, sent string, end int) bool {
	// The match may end inside the noun phrase ("make it to the next" of
	// "... make it to the next round"): its last words open the sequence.
	var pre []string
	for p, n := end, 0; n < 3; n++ {
		t, ts := tokenBefore(s, p)
		if !gamePreps[t] && !gameLeads[t] && !gameMods[t] {
			break
		}
		pre = append([]string{t}, pre...)
		p = ts
	}
	w := append(pre, nextWords(s, end, 7)...)
	i := 0
	for len(pre) == 0 && i < 2 && gameAdverbs[w[i]] {
		i++
	}
	if !gamePreps[w[i]] {
		return false
	}
	i++
	if gameLeads[w[i]] {
		i++
	}
	if gameMods[w[i]] {
		i++
	}
	if !gameNouns[w[i]] || w[i+1] == "of" || w[i+1] == "called" || w[i+1] == "named" || lifeGameRe.MatchString(sent) {
		return false
	}
	for _, t := range strings.Fields(sent) {
		if gameWords[strings.Trim(t, ".,!?;:()")] {
			return true
		}
	}
	return false
}

var (
	gameAdverbs = setOf("first last again early instantly immediately quickly fast soon already")
	gamePreps   = setOf("in on at during to into")
	gameMods    = setOf("next first final last boss same new")
	lifeGameRe  = mustGuarded(`\b(?:game|games)\s+(?:(?:that|which)\s+is\s+|of\s+|called\s+|named\s+)life\b|\blife\s+(?:is|'s)\s+(?:just\s+|only\s+|one\s+big\s+)?(?:a|an)\s+(?:[a-z]+\s+)?game\b|\blife'?s\s+game\b`)
)

// deathFormIdiom is the post-check of "I'd love to die": "die" spelt for
// "dye", and "dead" as an intensifier ("I'd love to be dead honest").
func deathFormIdiom(s string, start, end int) bool {
	if dyeObject(s, start, end) {
		return true
	}
	return strings.HasSuffix(s[start:end], "dead") && deadIntensified[nextWords(s, end, 1)[0]] && !carriesRisk(s, start, end)
}

// hopeOfOthers attributes a hope or wish to someone else when a third-party
// subject comes right before the hope verb, past adverbs ("they wish I would
// die", "mereka berharap aku mati", "ibuku berharap aku meninggal"): someone
// else's wish for the speaker's death is reported, not the speaker's own. A
// hope particle has no subject ("semoga", "moga", "hopefully": "ibuku,
// semoga aku mati" is the speaker's), and a noun with a possessive suffix
// before a pronoun is followed by a new subject ("ibuku aku berharap mati").
func hopeOfOthers(s string, start, _ int) bool {
	if hopeParticles[firstWord(s[start:])] {
		return false
	}
	prev, ps := tokenBefore(s, start)
	for n := 0; n < 3 && (subjectAdverbs[prev] || hopeAdverbs[prev]); n++ {
		prev, ps = tokenBefore(s, ps)
	}
	switch {
	case hopeOthers[prev] || relativeNoun(prev):
		if w, ws := tokenBefore(s, ps); hopeDeterminers[w] {
			ps = ws // "my mom"
		}
	case possessive[prev] || prev == "my":
		// "ibu aku berharap aku mati", "temen gue berharap ...", "my mom".
		w, ws := tokenBefore(s, ps)
		if !kuRelatives[w] && !enRelatives[w] {
			return false
		}
		ps = ws
	default:
		return false
	}
	// The third party must be the subject: it opens the clause or follows
	// a clause word. After a verb or a preposition it is an object, and
	// the hope is the speaker's own run-on clause ("aku kangen ibu berharap
	// aku meninggal", "aku capek sama mereka berharap aku mati", "i hate you
	// wish i would die", "I can't stand my mom hoping I die"; review
	// finding).
	w, _ := tokenBefore(s, ps)
	return w == "" || hopeClauseLeads[w]
}

var (
	hopeDeterminers = setOf("my our his her their the")
	// hopeClauseLeads may come right before the subject of a reported hope
	// ("terus mereka berharap aku mati", "and they wish I would die").
	hopeClauseLeads = setOf("dan terus trus lalu jadi tapi tp padahal bahkan malah sampai sampe soalnya karena krn kalau kalo klo pas " +
		"sementara sedangkan apalagi and but so then because cause cuz since when while if even now and")
)

var hopeParticles = setOf("semoga smoga moga moga-moga moga2 mudah-mudahan mudah2an hopefully harapnya")

// relativeNoun reports whether w names a relative or another person close
// to the speaker, with or without a possessive suffix ("ibuku", "temennya",
// "mom").
func relativeNoun(w string) bool {
	if kuRelatives[w] || enRelatives[w] {
		return true
	}
	for _, suf := range []string{"ku", "nya", "mu"} {
		if stem, ok := strings.CutSuffix(w, suf); ok && kuRelatives[stem] {
			return true
		}
	}
	return false
}

var (
	hopeOthers = setOf("he she they you we people everyone everybody someone somebody nobody they'd they'll he'd " +
		"she'd you'd dia ia mereka beliau kalian kamu lo lu elo doi orang-orang semua")
	enRelatives = setOf("mom mum mother dad father parents parent sister brother friend friends boyfriend girlfriend " +
		"bf gf wife husband family teacher boss classmates ex bully bullies")
	hopeAdverbs = setOf("probably secretly clearly obviously totally kinda pasti kayaknya mungkin juga jg malah " +
		"diam-diam sampai sampe bahkan")
)

// heartGerund rejects "my heart stops" followed by a listed gerund ("I hope
// my heart stops racing", "... stops hurting"): the heart is to calm down,
// not to stop. "stops beating" keeps the match.
func heartGerund(s string, start, end int) bool {
	return heartGerunds[nextWords(s, end, 1)[0]] && !carriesRisk(s, start, end)
}

var heartGerunds = setOf("racing pounding hurting aching fluttering skipping thumping breaking bleeding burning " +
	"sinking hammering")

// takeMePlace rejects a plea to be taken to an everyday place ("lord take
// me back to school", "God take me to the mall", "god take me out to
// dinner", "take me shopping"): "take me" followed by a destination word.
// "take me home", "take me to heaven", "take me to my <relative>", "take me
// out of this world", "take me there", "take me somewhere I can rest" and
// any plea in a sentence with "forever", "rest", "for good", "world",
// "away" or a risk word are pleas to die (review finding).
//
// The sentence around the plea is read, past commas and clause words, for
// an intent, modal or habit word or a self-directed method ("God take me
// to the beach and let me drown", "..., I want to drown", "... so I can
// jump"; review finding: only the local phrase was read).
func takeMePlace(s string, start, end int) bool {
	w := nextWords(s, end, 4)
	sent := sentenceWindow(s, start, end)
	if hasRiskWord(sent) || selfMethodRe.MatchString(sent) || takeDeathRe.MatchString(s[end:min(len(s), end+checkWindow)]) {
		return false
	}
	switch w[0] {
	case "back":
		// "take me back to 2019", "... to my childhood", "... to the 90s":
		// nostalgia (review finding).
		if w[1] == "to" && (pastYearRe.MatchString(s[end:min(len(s), end+checkWindow)]) || takePast[w[2]] ||
			((w[2] == "the" || w[2] == "my") && takePast[w[3]])) {
			return true
		}
		return (w[1] == "to" && takeDestinations[w[2]]) || takeDestinations[w[1]]
	case "to":
		if w[1] == "the" || w[1] == "a" {
			return takeDestinations[w[2]]
		}
		return takeDestinations[w[1]]
	case "out":
		// "take me out to dinner", "take me out for coffee".
		return (w[1] == "to" || w[1] == "for") && (takeDestinations[w[2]] || takeDestinations[w[3]])
	}
	return takeDestinations[w[0]]
}

var (
	// takeDestinations are everyday places and outings.
	takeDestinations = setOf("higher school class work office church mall store shop shopping market dinner lunch breakfast " +
		"coffee movies cinema beach park concert party gym hospital doctor dentist airport station college campus " +
		"university library restaurant cafe zoo vacation holiday disneyland disney practice " +
		// trips (review finding: "God take me to Bali please")
		"trip bali lombok jogja yogyakarta bandung hawaii paris japan tokyo korea seoul london europe italy rome " +
		"thailand bangkok singapore vegas mexico cancun maldives bahamas festival")
	// takePast after "take me back to" are nostalgia ("my childhood", "the
	// 90s", "when I was a kid"); pastYearRe is a past year ("back to 2019").
	takePast   = setOf("childhood summer 90s 80s 70s 60s 2000s 2010s nineties eighties seventies days times old good simpler when highschool")
	pastYearRe = regexp.MustCompile(`^\s+back\s+to\s+(?:19|20)\d\d\b`)
	// selfMethodRe is a self-directed method or a death word anywhere in the
	// sentence: no idiom check discards a match that comes with one.
	selfMethodRe = mustGuarded(`\b(?:drown|drowning|drowned|jump|jumping|jumped|hang|hanging|hanged|overdose|overdosing|od|pills|` +
		`wrists?|veins?|nadi|bridge|roof|rooftop|ledge|cliff|rope|noose|knife|blade|razor|bleach|poison|gun|shoot|` +
		`die|dying|dead|death|kill|killing|suicide|suicidal|end\s+it|tenggelam|loncat|lompat|terjun|gantung|racun|obat|mati|bunuh)\b`)
	// takeDeathRe after "take me" makes the plea a plea to die.
	takeDeathRe = mustGuarded(`\b(?:forever|rest|resting|for\s+good|world|away|heaven|die|dead|death|end|peace|sleep)\b`)
)

// dieOnHill rejects "let me die on this hill", the idiom for holding a
// position, unless an intent, modal or habit word comes with it.
func dieOnHill(s string, start, end int) bool {
	w := nextWords(s, end, 3)
	return w[0] == "on" && (w[1] == "this" || w[1] == "that") && w[2] == "hill" && !carriesRisk(s, start, end)
}

// letMeDieNot is the post-check of "let me die": a plea to live ("please
// don't let me die", "jangan let me die"), the idiom "let me die on this
// hill" and "die" spelt for "dye". A refused permission is the speaker's
// wish ("they wouldn't let me die") and keeps the match.
func letMeDieNot(s string, start, end int) bool {
	w, ws := tokenBefore(s, start)
	if w == "not" {
		w, _ = tokenBefore(s, ws)
		if w == "do" {
			return true // "do not let me die"
		}
	}
	return letNegators[w] || dieOnHill(s, start, end) || dyeObject(s, start, end)
}

var letNegators = setOf("don't dont jangan")

// hopeTarget is the post-check of id-hope-to-die's death pattern: the idiom
// checks of death-wish-phrase (see idiomaticDeath), and a subjectless hope
// whose death word a third party follows ("semoga cepet mati tuh orang",
// "semoga mati aja dia", "moga mampus si koruptor"): a curse on someone else,
// not the speaker's wish. An intent, modal or habit word keeps the match.
func hopeTarget(s string, start, end int) bool {
	if idiomaticDeath(s, start, end) {
		return true
	}
	if firstPerson(s[start:end]) {
		return false
	}
	w := nextWords(s, end, 6)
	if w[0] == "aja" || w[0] == "saja" || particles[w[0]] {
		w = w[1:]
	}
	if !hopeVictims[w[0]] || carriesRisk(s, start, end) {
		return false
	}
	if lead := firstWord(s[start:end]); prayLeads[lead] && addressees[w[0]] {
		// "doain cepet meninggal ya kalian": the people asked to pray are
		// addressed, not cursed.
		return false
	}
	// The victim must be the whole rest of the clause, a noun phrase of
	// particles, demonstratives and insults ("tuh orang", "si koruptor",
	// "orang itu"). A predicate after it makes it the subject of the
	// speaker's own run-on clause ("semoga cepet mati semuanya beres",
	// "... dia pasti seneng", "... mereka bisa bahagia"; review finding).
	rest := w[1:]
	if w[0] == "itu" || w[0] == "tuh" {
		// A bare "itu"/"tuh" needs a person after it ("itu orang", "tuh
		// koruptor"): "semoga cepet mati itu aja" ("that is all") is the
		// speaker's own wish (review finding).
		person := false
		for _, t := range rest {
			if t == "" {
				break
			}
			person = person || victimPersons[t]
		}
		if !person {
			return false
		}
	}
	if w[0] == "si" {
		if rest[0] == "" || ownPronouns[rest[0]] || selfWords[rest[0]] {
			return false // "si aku yang nggak berguna ini"
		}
		rest = rest[1:]
	}
	for _, t := range rest {
		if t == "" {
			return true
		}
		if !victimTail[t] {
			return false
		}
	}
	return false
}

var (
	hopeVictims = setOf("dia ia mereka beliau kalian kamu lo lu elo doi tuh itu si orang semua semuanya koruptor")
	// victimTail may follow the victim to the end of its clause.
	victimTail = setOf("lah deh sih nih dong ya tuh itu ini aja saja juga jg orang orangnya orang-orang semua semuanya " +
		"sialan brengsek bangsat bajingan jahat kampret anjing koruptor pejabat penjahat pembully tersebut")
	// victimPersons name the person a bare "itu"/"tuh" points at.
	victimPersons = setOf("orang orangnya orang-orang koruptor pejabat penjahat pembully sialan brengsek bangsat bajingan kampret")
	prayLeads     = setOf("doain doakan mendoakan berdoa")
	addressees    = setOf("kalian kamu lo lu elo semua semuanya")
	selfWords     = setOf("ku diriku saya gue gw gua aq ak")
)

// notAliveObject is the post-check of id-hope-to-die's "nggak ada" pattern:
// it rejects the match only when a listed object or everyday place
// completes "ada" ("semoga aku nggak ada kelas besok", "semoga aku nggak ada
// di kelas"; see everydayPlace) or a listed manner completes "hidup"
// ("semoga aku nggak hidup susah"). Any other
// continuation, a conjunction, a reason, a filler or a dash included,
// keeps the match ("semoga besok aku udah nggak ada tapi ...", "... nggak
// ada soalnya capek", "... nggak ada lagi di dunia ini"; review finding).
func notAliveObject(s string, start, end int) bool {
	if carriesRisk(s, start, end) {
		return false
	}
	w := nextWords(s, end, 4)
	switch {
	case w[0] == "di" || w[0] == "dari" || w[0] == "dalam":
		// Only a listed everyday place ("di kelas", "di rumah") is an
		// everyday absence, and not before "lagi" (see everydayPlace): "di
		// muka bumi ini lagi", "di alam ini", "di tahun depan", "di sini
		// lagi" and "di antara kalian" keep the match (review finding).
		return everydayPlace(w[1:])
	case strings.HasSuffix(s[start:end], "ada"):
		return aliveObjects[w[0]] && !clockTimeAfter(s, end)
	case strings.HasSuffix(s[start:end], "hidup") || strings.HasSuffix(s[start:end], "idup"):
		return aliveManners[w[0]]
	}
	return false
}

var (
	// aliveObjects complete "nggak ada" as having no ... ("nggak ada kelas").
	aliveObjects = setOf("kelas jadwal tugas pr rapat meeting masalah shift acara ujian kuis quiz ulangan kuliah kerjaan " +
		"kerja lembur les latihan janji urusan halangan hambatan utang hutang cicilan tagihan beban drama libur jam " +
		"kabar sinyal uang duit apa-apa apapun kendala gangguan")
	// aliveManners complete "nggak hidup" as not living in some way
	// ("nggak hidup susah", "nggak hidup sendirian").
	aliveManners = setOf("susah miskin sengsara melarat kekurangan sendirian sendiri kesepian boros mewah foya-foya " +
		"dalam di tanpa dengan bergantung numpang menumpang")
)

// adverbDeathIdiom is the post-check of id-death-wish-adverb: the idiom
// checks of death-wish-phrase (see idiomaticDeath), and "mau ... meninggal"
// ("about to die": "nenekku mau cepet meninggal") without a first-person
// subject or "aja".
func adverbDeathIdiom(s string, start, end int) bool {
	if idiomaticDeath(s, start, end) {
		return true
	}
	m := s[start:end]
	if !strings.HasSuffix(m, "meninggal") || (!strings.HasPrefix(m, "mau") && !strings.HasPrefix(m, "mw")) {
		return false
	}
	if strings.Contains(m, "aja") || strings.Contains(m, "saja") || nextWords(s, end, 1)[0] == "aja" {
		return false
	}
	prev, _ := tokenBefore(s, start)
	return !ownPronouns[prev] && prev != "ku"
}

// negatedIntent rejects a plan whose intent word a negator comes right
// before ("I don't want to take all my pills", "I'm not going to take ...",
// "aku nggak mau minum semua obat"): the speaker refuses the act, as in an
// explicit denial. A contrastive conjunction after it hedges the refusal
// ("I don't want to take all my pills, but ..."), which keeps the match.
func negatedIntent(s string, start, end int) bool {
	if contrastAfter(s, end) {
		return false
	}
	w, ws := tokenBefore(s, start)
	switch {
	case idNegators[w], w == "jangan", w == "bukan", w == "don't", w == "dont", w == "never", w == "not",
		w == "didn't", w == "doesn't", w == "won't", w == "wont", w == "wouldn't", w == "no":
		return true
	case w == "do" || w == "did" || w == "would":
		n, _ := tokenBefore(s, ws)
		return n == "not"
	}
	return false
}

// bottleObject rejects a past bottle that held something other than
// medicine ("I took the whole bottle of water"); a bottle of anything else,
// or of nothing named, keeps the match, and so does an intent, modal or
// habit word ("I'm going to take the whole bottle of wine" is crisis: a plan
// is never narrowed by its object).
func bottleObject(s string, start, end int) bool {
	w := nextWords(s, end, 3)
	if w[0] != "of" || carriesRisk(s, start, end) {
		return false
	}
	if bottleSkip[w[1]] {
		w[1] = w[2]
	}
	return bottleContents[w[1]]
}

// contrastAfter reports whether a contrastive conjunction follows s[end:]
// within checkWindow bytes, past any words of its sentence ("..., but", "...
// tapi"), or opens the next sentence (". but", "! tapi"; the patterns that
// use it are wide, so s runs on past the sentence; review finding).
func contrastAfter(s string, end int) bool {
	hi := min(len(s), end+checkWindow)
	for i := end; i < hi; i++ {
		switch c := s[i]; {
		case c == '.' || c == '!' || c == '?':
			return conjunctionAt(s, skipPunct(s, i))
		case isLetter(c) && (i == 0 || !isLetter(s[i-1])) && conjunctionAt(s, i):
			return true
		}
	}
	return false
}

var (
	odQuantities   = setOf("semua semuanya seluruh seluruhnya banyak sebotol segenggam sekotak sebungkus sestrip")
	bottleSkip     = setOf("my the that this red white cold sparkling mineral hot")
	bottleContents = setOf("water juice soda coke pop milk wine champagne prosecco beer sauce ketchup mayo shampoo " +
		"conditioner lotion sunscreen perfume cologne oil vinegar dressing glue paint ink gatorade kombucha tea coffee " +
		"lemonade sriracha honey ranch soap wash moisturizer toner serum")
)

// odQuestion rejects "minum obat sekaligus" in a question about taking
// medicine together that opens its sentence with "boleh", "bolehkah",
// "amankah" or "apakah boleh/aman" ("boleh minum obat sekaligus dengan
// vitamin?"), when the match has no quantity ("semua", "sebotol") and the
// sentence no risk word or digit.
func odQuestion(s string, start, end int) bool {
	m := s[start:end]
	if start > sentenceReach || strings.Contains(m, "semua") || !strings.HasSuffix(m, "sekaligus") {
		return false
	}
	if sent := sentenceWindow(s, start, end); hasRiskWord(sent) || strings.ContainsAny(sent, "0123456789") {
		return false
	}
	// A quantity after the match is not a question about taking medicine
	// together ("boleh minum obat sekaligus semuanya", "... sekaligus
	// banyak"; review finding).
	for _, x := range nextWords(s, end, 3) {
		if odQuantities[x] {
			return false
		}
	}
	w := nextWords(strings.TrimLeft(s, " \n\t"), 0, 2)
	switch w[0] {
	case "boleh", "bolehkah", "amankah":
		return true
	case "apakah", "apa":
		return w[1] == "boleh" || w[1] == "aman"
	}
	return false
}

// shSlang is the check of the bare "sh" of self-harm-slang: the slang needs
// an intent, urge, aspect, thought or relapse word right before it
// ("pengen sh", "urge to sh", "I want to sh", "gonna sh", "did sh", "relapsed
// on sh", "aku abis sh", "kepikiran sh", "trying not to sh"), a relapse,
// urge or scar noun right after it ("sh lagi", "sh again", "sh relapse", "sh
// urges", "sh scars"), a past form ("i sh'd", "i sh'ed"), or a first-person
// subject right before it with a time, habit or reason word after it ("i sh
// every night", "i sh because ...", "aku sh tadi malam"; review finding).
// "sh" spelling another word is rejected (see shNotSlang). The words are
// read as tokens around the match: a regexp over a window per match made a
// text of many "sh" slow (review finding).
func shSlang(s string, start, end int) bool {
	if shNotSlang(s, start, end) {
		return true
	}
	if end+1 < len(s) && s[end] == '\'' {
		if w := wordAt(s, end+1); w == "d" || w == "ed" || w == "ing" {
			return false // "i sh'd last night"
		}
	}
	next := wordAt(s, end)
	if shTrail[next] {
		return false
	}
	prev, ps := tokenBefore(s, start)
	for n := 0; n < 2 && shSkip[prev]; n++ {
		prev, ps = tokenBefore(s, ps)
	}
	switch {
	case shLeads[prev], strings.HasSuffix(prev, "'ll"):
		return false
	case prev == "to":
		w, _ := tokenBefore(s, ps)
		// "trying not to sh", "promised not to sh": a struggle against the
		// urge.
		return w != "not" && !shBeforeTo[w]
	case prev == "about" || prev == "of":
		w, _ := tokenBefore(s, ps)
		return w != "thinking" && w != "think" && w != "thought" && w != "thoughts"
	case shSubjects[prev]:
		return !shAfterSubject[next]
	}
	return true
}

var (
	// shLeads come right before the slang, past shSkip words.
	shLeads = setOf("pengen pgn pngn pingin pengin kepengen kepingin mau mw ingin relapse relapsed relapsing wanna gonna " +
		"gotta did might will abis habis tadi barusan udah sudah udh pernah sempet sempat kepikiran mikirin kebayang " +
		"niat berniat urge urges")
	// shSkip may stand between a lead and the slang ("relapsed on sh",
	// "pengen banget sh").
	shSkip = setOf("on into with aja lagi lg banget bgt just really so again")
	// shBeforeTo come before "to sh" ("urge to sh", "want to sh").
	shBeforeTo = setOf("urge urges tempted want wanted wanting need needed going have had mean meant trying try tried " +
		"wish ready planning plan decided about")
	// shTrail come right after the slang.
	shTrail = setOf("lagi lg again relapse relapsed relapsing urges urge scars scar cuts marks wounds thoughts habit " +
		"addiction")
	shSubjects = setOf("i aku gue gw gua saya aq ak ku i've ive")
	// shAfterSubject after "<subject> sh" make it a disclosure: a time, a
	// habit or a reason.
	shAfterSubject = setOf("last tonight today yesterday every everyday daily nightly often sometimes regularly again " +
		"when because cause cuz bc since to alot lot tadi semalam kemarin kemaren malam pagi siang sore terus trus mulu " +
		"melulu tiap setiap sering kadang karena soalnya biar pas waktu kalau kalo")
)

// shNotSlang rejects "sh" that is the start of a masked word ("sh*t", "sh t"
// in the lexical reading, "sh it").
func shNotSlang(s string, _, end int) bool {
	if end < len(s) && strings.IndexByte("*#@$%!-", s[end]) >= 0 {
		return true
	}
	w := wordAt(s, end)
	return w == "t" || w == "it" || w == "itty" || w == "ite"
}

// burnNews attributes a reported self-immolation ("aksi bakar diri",
// "pelaku bakar diri") to a third party, like a news compound (see
// suicideNews), unless an intent word or a first-person subject governs it.
func burnNews(s string, start, _ int) bool {
	prev, ps := tokenBefore(s, start)
	if prev != "aksi" && !idNewsBefore[prev] {
		return false
	}
	return !compoundGoverned(s, ps, true)
}

// drownIdiomID rejects "menenggelamkan diri dalam pekerjaan" (to bury
// oneself in work), the Indonesian counterpart of "drown myself in work"
// (see lethalIdiom): the noun after "dalam"/"di"/"ke" must be a listed idiom
// noun, and an intent, modal or habit word keeps the match.
func drownIdiomID(s string, start, end int) bool {
	w := nextWords(s, end, 3)
	if w[0] != "dalam" && w[0] != "di" && w[0] != "ke" {
		return false
	}
	k := 1
	if drownSkipID[w[k]] {
		k++
	}
	return drownIdiomsID[w[k]] && !carriesRisk(s, start, end)
}

var (
	drownSkipID   = setOf("dunia lautan tumpukan segala banyak")
	drownIdiomsID = setOf("pekerjaan kerjaan kerja tugas kesibukan buku bacaan game musik drama drakor kuliah belajar " +
		"pelajaran ibadah doa hobi")
)

// absenceOccasion is the post-check of en-passive-death-wish. It rejects a
// negated wish (see negatedIntent), an online absence ("I want to not exist
// on social media", "... online"), a stay elsewhere ("I want to not be here
// today"; "not be here" counts at the end of its clause or before
// "anymore", "forever", "at all" or "in this world"), being away for a
// listed everyday occasion ("I want to be gone by the time the party
// starts", "... gone on vacation"; see goneOccasion) and a happy or
// gerund ending ("I want it all to end happily", "I want my life to stop
// being so boring"). An intent, modal or habit word or a death word outside
// the match keeps it.
func absenceOccasion(s string, start, end int) bool {
	if negatedIntent(s, start, end) {
		return true
	}
	if riskOutside(s, start, end) || deathAround(s, start, end) {
		return false
	}
	m := s[start:end]
	w := nextWords(s, end, 3)
	switch {
	case strings.HasSuffix(m, "exist") || strings.HasSuffix(m, "existed"):
		return w[0] == "online" || ((w[0] == "on" || w[0] == "in") && (onlinePlaces[w[1]] || onlinePlaces[w[2]]))
	case strings.HasSuffix(m, "here") || strings.HasSuffix(m, "around"):
		return !absenceTail(w)
	case strings.HasSuffix(m, "gone"):
		return goneOccasion(s, end)
	case strings.HasSuffix(m, "end") || strings.HasSuffix(m, "over"):
		return happyEndings[w[0]]
	case strings.HasSuffix(m, "stop"):
		return strings.HasSuffix(w[0], "ing") || happyEndings[w[0]]
	case strings.Contains(m, "leave"):
		return leaveLegacy(s, end)
	}
	return false
}

// leaveLegacy reports whether the words after "leave this world" are the
// legacy idiom: "a better place", "a little kinder place", "better than I
// found it". Any other "a" phrase ("a little early", "a bit sooner"),
// "better off", and a legacy phrase that "without me", "early" or "soon"
// follows in the sentence ("a better place without me in it") keep the match
// (review finding: every "a" and every listed adjective was rejected).
func leaveLegacy(s string, end int) bool {
	w := nextWords(s, end, 4)
	k := 0
	if w[0] == "a" {
		k = 1
		if legacyDegrees[w[k]] {
			k++
		}
	}
	if !leaveWorldTails[w[k]] {
		return false
	}
	if next := w[k+1]; next != "place" && next != "than" && !(w[0] == "a" && next == "") {
		return false
	}
	return !leaveSoonRe.MatchString(restAfter(s, end))
}

// riskOutside reports whether an intent, modal or habit word stands in the
// phrase around s[start:end] (see localWindow), outside the match itself.
func riskOutside(s string, start, end int) bool {
	lo, hi := localWindow(s, start, end)
	return hasRiskWord(s[lo:start]) || hasRiskWord(s[end:hi])
}

// absenceTail reports whether the words w after "not be here/around" make
// it an absence from life: nothing, "anymore", "any more", "any longer",
// "forever", "ever again", "at all", "no more", "in this world", "on this
// earth".
func absenceTail(w []string) bool {
	switch {
	case w[0] == "" || absenceTails[w[0]]:
		return true
	case w[0] == "any" && (w[1] == "more" || w[1] == "longer"), w[0] == "at" && w[1] == "all", w[0] == "no" && w[1] == "more":
		return true
	case w[0] == "in" || w[0] == "on":
		return w[1] == "earth" || ((w[1] == "this" || w[1] == "the") && absenceWorlds[w[2]])
	}
	return false
}

// goneOccasion reports whether a listed everyday occasion, trip or place
// follows "gone" in a phrase that a preposition or a time word opens right
// after it ("gone by the time the party starts", "gone before traffic gets
// bad", "gone on vacation", "gone from this office", "not here when the
// movers come"), within six words and before any first-person word, hope,
// wish or clause word, so the speaker is away rather than dead. "before
// they arrive", "by the time she gets home", a run-on next sentence ("...
// gone by morning God take me back to school") and anything unlisted keep
// the match.
func goneOccasion(s string, end int) bool {
	w := nextWords(s, end, 7)
	if !goneLeads[w[0]] {
		return false
	}
	for _, x := range w[1:] {
		switch {
		case x == "" || goneStops[x] || clauseWords[x]:
			return false
		case goneOccasions[x]:
			return true
		}
	}
	return false
}

// leaveWorldTails after "leave this world" make it a legacy, not a death
// ("leave this world a better place"; see absenceOccasion).
var (
	leaveWorldTails = setOf("better cleaner greener kinder safer healthier happier nicer brighter fairer")
	legacyDegrees   = setOf("little bit slightly much lot tiny")
	// leaveSoonRe after "leave this world" is about dying early, not a
	// legacy (see leaveLegacy).
	leaveSoonRe = regexp.MustCompile(`\b(?:without\s+me|early|earlier|sooner|soon)\b`)
)

var (
	goneLeads     = setOf("by before when for on from until till til during after while at to in")
	goneStops     = setOf("i i'm im i'd i'll me my mine myself god lord hope hoping wish wishing want wanna die dead aku gue gw saya")
	onlinePlaces  = setOf("social socials media instagram ig insta twitter x tiktok facebook fb snapchat snap reddit discord internet web online group chat gc server photos pictures pics feed timeline")
	absenceTails  = setOf("anymore forever ever again permanently alive tomorrow tonight")
	absenceWorlds = setOf("world earth planet life")
	// goneOccasions are the everyday occasions, trips and places that make
	// being gone an absence from them (see goneOccasion).
	goneOccasions = setOf("vacation vacations vacay trip trips holiday holidays party parties meeting meetings office " +
		"work job shift class classes school lecture exam exams test movers traffic guests presents gifts wedding " +
		"ceremony concert match show dinner lunch breakfast rush weekend weekends airport flight train bus beach mall " +
		"store shop shopping errands errand appointment interview practice rehearsal event festival reunion visit " +
		"inspection renovation construction cleaning cleaners guests' relatives in-laws inlaws")
)

// idAbsence is the post-check of id-passive-death-wish. It rejects a
// negated wish (see negatedIntent); after "nggak ada", a listed object
// ("pengen nggak ada pr", "... nggak ada yang ganggu") or an everyday place
// ("... nggak ada di kelas"; see everydayPlace); after "nggak hidup", a
// listed manner ("pengen nggak hidup susah"); after "selamanya", leaving for
// a place ("pengen pergi selamanya dari kota ini", "... ke luar negeri");
// ending something with someone or an activity after "udahan" ("pengen
// udahan aja sama dia", "mau udahan main"); a happy ending ("pengen semuanya
// berakhir bahagia"); and for a readiness to die, the idiom checks of
// death-wish-phrase and devotion ("aku siap mati demi kamu", "aku rela mati
// buat kalian"). An intent, modal or habit word outside the match keeps
// it, except for devotion.
func idAbsence(s string, start, end int) bool {
	if negatedIntent(s, start, end) {
		return true
	}
	m := s[start:end]
	w := nextWords(s, end, 4)
	if strings.HasSuffix(m, "mati") || strings.HasSuffix(m, "meninggal") || strings.HasSuffix(m, "mampus") {
		if idiomaticDeath(s, start, end) {
			return true
		}
		return w[0] == "demi" || ((w[0] == "buat" || w[0] == "untuk" || w[0] == "utk") && (devotionTargets[w[1]] || relativeNoun(w[1])))
	}
	if riskOutside(s, start, end) {
		return false
	}
	switch {
	case strings.HasSuffix(m, "ada"):
		if w[0] == "di" || w[0] == "dari" || w[0] == "dalam" {
			return everydayPlace(w[1:])
		}
		return (aliveObjects[w[0]] || absenceObjects[w[0]]) && !clockTimeAfter(s, end)
	case strings.HasSuffix(m, "hidup") || strings.HasSuffix(m, "idup"):
		if w[0] == "di" || w[0] == "dalam" {
			return everydayPlace(w[1:])
		}
		if strings.Contains(m, "berhenti") || strings.Contains(m, "stop") {
			return stopLivingManner(w)
		}
		return aliveManners[w[0]]
	case strings.HasSuffix(m, "selamanya") || strings.HasSuffix(m, "abadi"):
		if w[0] == "ke" {
			return !afterlifePlaces[w[1]]
		}
		return vanishPlace(s, start, end)
	case strings.HasSuffix(m, " ini") || strings.HasSuffix(m, "fana"):
		// "pengen pergi dari dunia ini sebentar": a short break.
		return briefBreak(w) && !foreverRe.MatchString(restAfter(s, end))
	case strings.HasSuffix(m, "udahan"):
		k := 0
		for k < 2 && (w[k] == "aja" || w[k] == "saja" || w[k] == "ajah" || particles[w[k]]) {
			k++
		}
		if udahanWith[w[k]] {
			return !lifeObjects[w[k+1]]
		}
		return udahanActivities[w[k]]
	}
	return happyEndings[w[0]]
}

// everydayPlace reports whether w opens with a listed everyday place
// ("kelas", "rumah", "kantor", "acara itu") that no "lagi" or time word
// follows: "nggak ada di kelas besok" is about tomorrow's class, while "...
// di tahun depan", "... di sini lagi", "... di muka bumi" and "... di antara
// kalian" are about being alive (review finding).
func everydayPlace(w []string) bool {
	if !everydayPlaces[w[0]] {
		return false
	}
	for _, x := range w[1:] {
		if x == "lagi" || x == "lg" || x == "selamanya" {
			return false
		}
	}
	return true
}

// briefBreak reports whether the words w after "pergi dari dunia ini" make
// it a short break: a brief time word ("sebentar", "sejenak") that "lagi"
// ("sebentar lagi" is "soon") and an ending word ("selamanya", "abadi")
// do not follow, in the sentence (see foreverRe). "dulu" is the farewell
// particle after a leaving verb ("aku mau pergi dari dunia ini dulu ya"),
// not a short break (review finding: both were rejected).
func briefBreak(w []string) bool {
	if !briefTimes[w[0]] {
		return false
	}
	for _, x := range w[1:] {
		if x == "lagi" || x == "lg" || x == "selamanya" || x == "abadi" || x == "forever" {
			return false
		}
	}
	return true
}

// stopLivingManner reports whether the words w after "berhenti hidup" name
// a way of living that the speaker wants to stop ("pengen berhenti hidup
// boros", "... hidup numpang", "... hidup dengan boros"). Unlike
// aliveManners it has no "dengan" or "tanpa" of its own: a manner of dying
// or a method ("... dengan tenang", "... tanpa rasa sakit", "... dengan
// minum obat") keeps the match, and so does "dgn" (review finding: every
// "dengan"/"tanpa" tail was rejected).
func stopLivingManner(w []string) bool {
	switch w[0] {
	case "dengan", "dgn", "secara":
		return stopManners[w[1]]
	case "tanpa":
		return stopAimless[w[1]] // "berhenti hidup tanpa tujuan"
	}
	return stopManners[w[0]]
}

// briefFarewell rejects a brief leaving or rest match ("pergi sebentar",
// "tidur sejenak") unless the rest of the message says the person is not
// coming back: an explicit not-coming-back phrase or a forever word, even
// across a sentence break ("aku pergi sebentar. Tidak akan kembali",
// "aku mau tidur sebentar, selamanya"). A no-return phrase always wins over
// a short-break phrase: "tidak akan lama, tapi tidak akan kembali" is a
// farewell, not a short absence. A short-break phrase alone ("sebentar
// lagi", "tidak akan lama") keeps the match benign, and so does the absence
// of any farewell tail ("aku pergi sebentar ke toilet").
func briefFarewell(s string, _, end int) bool {
	// A no-return phrase always wins over a short-break phrase. Otherwise
	// "tidak akan lama, tapi tidak akan kembali" would stay benign because
	// the first listed phrase matched (review finding B).
	return !noReturnRe.MatchString(restAfter(s, end))
}

var (
	// briefTimes after "pergi dari dunia ini" make it a short break (see
	// briefBreak).
	briefTimes = setOf("sebentar bentar sejenak sesaat")
	// noReturnRe after a brief leaving or rest word says the person is not
	// coming back (see briefFarewell). "kembali lagi" is a positive return,
	// not a no-return phrase; negated forms ("tidak akan kembali lagi") are
	// already covered by the first alternative (review finding A).
	noReturnRe = regexp.MustCompile(`\b(?:tidak|tak|nggak|ngga|gak|ga|gk|enggak|engga|tdk|takkan|tidakkan)\s*(?:akan\s*)?(?:kembali|balik|pulang)\b|\b(?:nggak|ngga|gak|ga|gk|tidak|tak)\s*bakal\s*(?:kembali|balik|pulang)\b|\bga(?:k|)\s*bangun\s*lagi\b|\b(?:selamanya|selama-lamanya|abadi)\b`)
	// foreverRe later in the sentence makes "pergi dari dunia ini
	// sebentar" more than a short break (see briefBreak).
	foreverRe = regexp.MustCompile(`\b(?:selamanya|selama-lamanya|abadi|forever|for\s+good)\b`)
	// stopManners after "berhenti hidup" name a way of living (see
	// stopLivingManner).
	stopManners = setOf("susah miskin sengsara melarat kekurangan sendirian sendiri kesepian boros mewah foya-foya " +
		"bergantung numpang menumpang hedon hemat konsumtif berfoya-foya ngutang berutang")
	stopAimless = setOf("tujuan arah makna rencana aturan")
)

var (
	// absenceObjects complete "pengen nggak ada" as "there to be no ...":
	// "nggak ada yang ganggu", "nggak ada orang".
	absenceObjects = setOf("yang yg siapa siapapun orang apa beban gangguan masalah tugas kerjaan kegiatan")
	// everydayPlaces after "nggak ada di/dari" are an everyday absence.
	everydayPlaces = setOf("rumah kelas kantor sekolah kampus acara rapat kos kosan kost grup group gc pesta kondangan " +
		"kantin perpustakaan lab kampung kota jakarta sana situ tempat mall toko pasar gereja masjid jalanan desa " +
		"kontrakan apartemen asrama")
	afterlifePlaces = setOf("surga alam sisi pangkuan rahmatullah akhirat nirwana")
	udahanWith      = setOf("sama ama dengan dgn sm dari")
	lifeObjects     = setOf("hidup hidupku kehidupan dunia semuanya semua segalanya nyawa nyawaku diri diriku")
	// udahanActivities after "udahan" name what is ended ("mau udahan main").
	udahanActivities = setOf("main mainnya kerja kerjanya kuliah kuliahnya sekolah nugas belajar rapat meeting pacaran " +
		"hubungan hubungannya diet puasa langganan kontrak ngerjain ngoding latihan nonton makan chat chattan teleponan " +
		"telponan vc call")
	devotionTargets = setOf("kamu kalian dia mereka negara bangsa agama keluarga cinta indonesia tanah")
)
