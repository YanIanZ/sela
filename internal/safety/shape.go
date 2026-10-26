package safety

import (
	"regexp"
	"regexp/syntax"
	"slices"
	"sort"
	"strings"
)

// span is a half-open byte range [start, end) of the normalised text.
type span struct{ start, end int }

func (s span) contains(o span) bool { return s.start <= o.start && o.end <= s.end }

func (s span) overlaps(o span) bool { return s.start < o.end && o.start < s.end }

// clause is a stretch of text between sentence punctuation, newlines and
// contrastive conjunctions, never split inside quotation marks.
type clause struct {
	span
	// segments are the comma-separated parts of the clause (outside quotes).
	segments []span
	// afterContrast is true when the clause follows a contrastive
	// conjunction ("tapi", "but").
	afterContrast bool
	// contrastNext is true when a contrastive conjunction follows the clause
	// with nothing but punctuation in between, even when nothing comes after
	// it ("aku tidak mau bunuh diri, tapi...").
	contrastNext bool

	// Filled in lazily by shape.clause.
	analysed bool
	denials  []denialSpan
	scopes   []span // text attributed to a third party
	hidden   []span // frames and attributed text, sorted and merged
	shared   bool   // "juga"/"too" inside attributed indirect speech
	// lastShared holds, per segment, the start of its last "juga"/"too"
	// (-1: none); filled in lazily by shape.sharedAfter.
	lastShared []int
}

// denialSpan is a segment recognised as an explicit whole-clause denial.
type denialSpan struct {
	span
	// hedged is true when a contrastive conjunction follows the denial
	// ("aku tidak mau bunuh diri, tapi ...").
	hedged bool
}

// shape is the clause structure of one normalised text. Clause analysis is
// lazy: only clauses that contain a rule match are analysed, and adoption is
// only computed when a match is attributed to a third party.
type shape struct {
	text    string
	quotes  []span // quoted content, without the quote marks, sorted
	clauses []clause
	// wholeDenial is v1's denial check applied to the entire text, where
	// spaces also match newlines ("aku tidak mau\nmati").
	wholeDenial bool

	adoptionKnown bool
	adoption      bool
}

func analyse(text string) *shape {
	c := &shape{text: text, quotes: findQuotes(text)}
	c.clauses = splitClauses(text, c.quotes)
	c.wholeDenial = denialRe.MatchString(text)
	return c
}

// overlapping returns the index range [lo, hi) of clauses overlapping m.
func (c *shape) overlapping(m span) (lo, hi int) {
	lo = sort.Search(len(c.clauses), func(i int) bool { return c.clauses[i].end > m.start })
	hi = lo
	for hi < len(c.clauses) && c.clauses[hi].start < m.end {
		hi++
	}
	return lo, hi
}

// clause returns clause i, analysing it on first use.
func (c *shape) clause(i int) *clause {
	cl := &c.clauses[i]
	if !cl.analysed {
		c.analyseClause(cl)
		cl.analysed = true
	}
	return cl
}

// adopted reports whether the speaker adopts third-party risk language
// anywhere in the text ("aku juga", "me too", "same here", "I relate", or
// "juga"/"too" inside reported content).
func (c *shape) adopted() bool {
	if c.adoptionKnown {
		return c.adoption
	}
	c.adoptionKnown = true
	for i := range c.clauses {
		cl := c.clause(i)
		if cl.shared {
			c.adoption = true
			return true
		}
		for _, seg := range cl.segments {
			masked := strings.TrimSpace(maskSpans(c.text, seg, cl.hidden))
			if masked != "" && adopts(masked) {
				c.adoption = true
				return true
			}
		}
		// "i do, too" and "me, too" put a comma inside the form.
		if len(cl.segments) > 1 && commaAdoptionRe.MatchString(maskSpans(c.text, cl.span, cl.hidden)) {
			c.adoption = true
			return true
		}
	}
	return false
}

// adopts reports whether the speaker's own words s (one comma segment with
// attributed text masked) take up reported risk language: a full form of
// adoptionRe ("me too", "same here", "pengen nyusul", "kepikiran buat
// ngelakuin hal yang sama", "I want to too"), or a short form of
// adoptionCoreRe that its continuation keeps one (see adoptionTail):
//   - "aku juga", "gw jg", "aku pun", "saya juga" are adoption unless an
//     ordinary continuation follows ("aku juga nonton beritanya", "aku pun
//     kaget"; see ordinaryAfterJuga): the words also mean "I also ...", so
//     an unlisted continuation takes the safer path;
//   - "aku sama", "so am I", "I'm the same", "me to", "same" opening its
//     segment and an elliptical wish ("aku pengen", "I want to") are
//     adoption when nothing but trailing words follows them (laughter,
//     emoticons, emoji, particles, time and degree adverbs: "aku sama
//     wkwk", "so am I :(", "aku pengen banget"; see trailingWords), a listed
//     follower ("aku sama kayak dia"), or for the English forms a
//     preposition or conjunction ("so am I at night"); before any other
//     word they are ordinary ("aku sama dia", "I'm the same age", "aku
//     pengen makan"). A negated wish is never adoption ("aku nggak pengen").
//
// Each tail scan stops at the first content word, so the work is linear.
func adopts(s string) bool {
	for _, g := range adoptionAlts {
		if g.MatchString(s) {
			return true
		}
	}
	for _, loc := range adoptionCoreRe.FindAllStringIndex(s, -1) {
		m := s[loc[0]:loc[1]]
		kind := coreKind(m)
		if kind == coreWish && negatedWish(s, loc[0]) {
			continue // "aku nggak pengen", "I don't want to"
		}
		if adoptionTail(s[loc[1]:], kind) {
			return true
		}
	}
	for _, g := range adoptionTrailAlts {
		for _, loc := range g.FindAllStringIndex(s, -1) {
			if adoptionTail(s[loc[1]:], coreTrail) {
				return true
			}
		}
	}
	return false
}

// adoptionStart returns the offset in unquoted direct speech s of the first
// adoption form after its opening (see adopts), or -1: "me too", "so do I"
// or "aku juga wkwk" after the quoted words are the speaker's ("lirik
// lagunya: aku ingin mati so do i lol"). An elliptical wish is left to the
// quoted person ("temanku bilang: aku capek pengen").
func adoptionStart(s string) int {
	best := -1
	for _, loc := range adoptionRe.FindAllStringIndex(s, -1) {
		if loc[0] > 0 {
			best = loc[0]
			break
		}
	}
	for _, loc := range adoptionCoreRe.FindAllStringIndex(s, -1) {
		if best >= 0 && loc[0] >= best {
			break
		}
		kind := coreKind(s[loc[0]:loc[1]])
		if loc[0] > 0 && kind != coreWish && adoptionTail(s[loc[1]:], kind) {
			best = loc[0]
			break
		}
	}
	for _, g := range adoptionTrailAlts {
		for _, loc := range g.FindAllStringIndex(s, -1) {
			if best >= 0 && loc[0] >= best {
				break
			}
			if loc[0] > 0 && adoptionTail(s[loc[1]:], coreTrail) {
				best = loc[0]
				break
			}
		}
	}
	return best
}

// The kinds of short adoption forms (see adopts).
const (
	coreJuga    = iota // "aku juga", "aku pun"
	coreSama           // "aku sama", "aku juga sama"
	coreEnglish        // "so am I", "I'm the same", "me to", "same"
	coreWish           // an elliptical "aku pengen", "I want to"
	coreTrail          // "gw banget", "kayak aku", "big mood", "I'm next": trailing words only
)

func coreKind(m string) int {
	w := firstWord(m)
	switch {
	case speechWishes[w] || w == "want":
		return coreWish
	case strings.HasSuffix(m, "sama"):
		return coreSama
	case corePronouns[w]:
		return coreJuga
	}
	for p := range corePronouns {
		if rest, ok := strings.CutPrefix(w, p); ok && (rest == "juga" || rest == "jg" || rest == "jga" || rest == "pun") {
			return coreJuga // "akupun"
		}
	}
	return coreEnglish
}

// corePronouns are the first-person pronouns of the Indonesian short forms,
// chat spellings included ("aq juga", "ak juga", "w juga", "sy juga").
var corePronouns = setOf("aku saya gue gua gw ane ku aq ak w sy gwe")

// negatedWish reports whether a negator comes right before the wish at i
// ("aku nggak pengen", "i don't want to", "i do not wanna").
func negatedWish(s string, i int) bool {
	w, ws := tokenBefore(s, i)
	if w == "i" {
		w, ws = tokenBefore(s, ws)
	}
	switch {
	case idNegators[w], w == "don't", w == "dont", w == "never", w == "not", w == "didn't", w == "won't",
		w == "doesn't", w == "no", w == "jangan", w == "bukan":
		return true
	case w == "do" || w == "did":
		n, _ := tokenBefore(s, ws)
		return n == "not"
	}
	return false
}

// adoptionTail reports whether the text after a short adoption form of the
// given kind keeps it adoption (see adopts). Laughter, an emoticon, an
// emoji, a dash or a time adverb right after the form closes it, so any
// words after that are a new thought ("so am i lol idk what to do", "i'm the
// same :( aku juga udah tahu", "so am I lately ..."), and so does a pronoun
// after an English form ("so am i i guess") or an Indonesian pronoun after a
// trailing-only form ("gw banget aku juga ..."); an elliptical wish is not
// closed this way ("aku pengen banget bantu dia" has an object).
func adoptionTail(rest string, kind int) bool {
	if kind == coreJuga {
		return jugaTail(rest)
	}
	closed := false
	for i := 0; i < len(rest); {
		if !isLetter(rest[i]) {
			closed = closed || closesForm(rest[i])
			i++
			continue
		}
		j := i
		for j < len(rest) && (isLetter(rest[j]) || rest[j] == '\'' || rest[j] == '-') {
			j++
		}
		w := rest[i:j]
		switch {
		case laughter(w), laughterWords[w], closingAdverbs[w]:
			closed = true
			i = j
			continue
		case trailingWords[w]:
			i = j
			continue
		case closed && kind != coreWish:
			return true
		case kind == coreSama:
			return samaFollowers[w]
		case kind == coreEnglish:
			return englishContinuations[w] || corePronouns[w] || w == "i"
		case kind == coreTrail:
			// An Indonesian pronoun starts a new clause ("gw banget aku
			// juga ..."); "i feel that i should help her" is an opinion.
			return idPronounsNew[w]
		}
		return false
	}
	return true
}

// closesForm reports whether byte c, after a short adoption form, closes it:
// an emoticon, an emoji, a dash or other symbol, not a space or an
// apostrophe. (A hyphen inside a word is read with the word.)
func closesForm(c byte) bool { return strings.IndexByte(" \t\n'", c) < 0 }

// idPronounsNew are the Indonesian first-person pronouns that start a new
// clause after a trailing-only form.
var idPronounsNew = setOf("aku saya gue gua gw ane aq ak sy gwe")

// closingAdverbs are the time adverbs that close a short adoption form.
var closingAdverbs = setOf("sekarang skrg skrng akhir-akhir belakangan lately rn now today tonight kemarin kemaren " +
	"tadi sometimes kadang kadang-kadang")

// laughterWords are written laughter and sighs that laughter does not
// recognise by shape.
var laughterWords = setOf("lol lmao lmfao rofl haha hehe huhu hiks hix xixi wkwk hmm huft hufft")

// jugaTail reports whether the text after "aku juga"/"aku pun" keeps it
// adoption. Only an ordinary word right after the form makes it an ordinary
// "I also ..." ("aku juga nonton beritanya", "aku pun kaget"; see
// ordinaryAfterJuga): particles and laughter are skipped, but after an
// aspect, time or degree word ("udah", "lagi", "kadang") only a knowing or
// perceiving verb is ordinary ("aku juga udah tahu"), and after a negator or
// limiter ("nggak", "cuma", "ada", "punya", "baru") only one that such a
// verb follows ("aku juga nggak tahu"). A continuation with a wish, thought,
// intent, hopelessness or death word is never ordinary ("aku juga udah
// nggak sanggup", "aku juga takut aku bakal begitu", "aku juga punya
// rencana"; only the next jugaWindow bytes are read, so a segment with many
// forms stays linear). Any other word takes the safer path.
func jugaTail(rest string) bool {
	if jugaRiskRe.MatchString(rest[:min(len(rest), jugaWindow)]) {
		return true
	}
	adverb, closed := false, false
	for i := 0; i < len(rest); {
		if !isLetter(rest[i]) {
			closed = closed || closesForm(rest[i])
			i++
			continue
		}
		j := i
		for j < len(rest) && (isLetter(rest[j]) || rest[j] == '\'' || rest[j] == '-') {
			j++
		}
		w := rest[i:j]
		i = j
		switch {
		case laughter(w), laughterWords[w]:
			closed = true
			continue
		case jugaParticles[w]:
			continue
		case closed:
			return true // "aku juga wkwk", then a new thought
		case trailingWords[w]:
			adverb = true
			continue
		case jugaLimiters[w]:
			return !knowingVerbs[nextWord(rest, j)]
		case adverb:
			return !knowingVerbs[w]
		}
		return !ordinaryAfterJuga[w]
	}
	return true
}

// jugaWindow bounds, in bytes, the continuation jugaTail reads for a risk
// word.
const jugaWindow = 80

// laughter reports whether w is written laughter ("wkwk", "wkwkwk",
// "awokwok", "hahaha", "hehe", "huhu", "xixi").
func laughter(w string) bool {
	if len(w) < 4 {
		return false
	}
	if strings.Trim(w, "wk") == "" || (w[0] == 'a' && strings.Trim(w[1:], "wok") == "") {
		return strings.Contains(w, "w") && strings.Contains(w, "k")
	}
	for _, u := range []string{"ha", "he", "hi", "hu", "xi"} {
		if strings.Count(w, u) >= 2 {
			if r := strings.ReplaceAll(w, u, ""); r == "" || r == "h" || r == "a" {
				return true
			}
		}
	}
	return false
}

var (
	// ordinaryAfterJuga continue "aku juga"/"aku pun" as an ordinary "I
	// also ...": perceiving, knowing, reacting, telling, helping and
	// everyday activities, right after the form (see jugaTail). Negators
	// and limiters are not ordinary on their own (jugaLimiters), and a
	// preposition is not one ("aku juga dari dulu" is adoption).
	ordinaryAfterJuga = setOf("nonton menonton baca membaca lihat liat ngeliat melihat denger dengar mendengar " +
		"ngedenger tahu tau ngerti mengerti paham kenal ingat inget lupa sadar nyadar percaya yakin heran ragu " +
		"kaget terkejut panik takut khawatir kuatir cemas bingung sedih nangis menangis syok shock kasihan " +
		"prihatin peduli sayang cinta marah kesal kesel emosi malu senang seneng kecewa setuju bangga bilang " +
		"ngomong bicara nanya tanya bertanya jawab cerita curhat ngabarin kasih kasi ngasih nelpon telpon nelfon " +
		"chat ngechat ngajak minta nyuruh ngelarang nasehatin nasihatin bantu membantu nolong menolong nemenin " +
		"menemani temenin jagain menjaga jaga peluk meluk dukung mendukung doain mendoakan berdoa nyemangatin " +
		"semangatin kerja bekerja kuliah sekolah tinggal pulang datang dateng " +
		"langsung segera buru-buru tetap tetep temannya temennya sahabatnya pacarnya sibuk makan minum masak beli " +
		"main jalan nyari cari dapet dapat nemu kirim ngirim nulis ngetik nunggu nungguin")
	// jugaLimiters after "aku juga" keep it ordinary only before a knowing
	// or perceiving verb ("aku juga nggak tahu", "aku juga baru denger").
	jugaLimiters = setOf("nggak ga gak tidak enggak engga ngga gk tak bukan jangan nggk ndak belum baru cuma hanya " +
		"ada punya masih sempat")
	// knowingVerbs are the knowing and perceiving verbs (see jugaTail).
	knowingVerbs = setOf("tahu tau ngerti mengerti paham kenal ingat inget lupa sadar nyadar nyangka menyangka " +
		"percaya yakin denger dengar mendengar ngedenger lihat liat ngeliat melihat nonton menonton baca membaca")
	// jugaParticles are skipped after "aku juga" (see jugaTail).
	jugaParticles = setOf("sih kok deh ya yah yaa yaaa lho loh nih dong tuh kan lah ah eh oh")
	// jugaRiskRe: a wish, thought, intent, hopelessness or death word after
	// "aku juga" keeps it adoption.
	jugaRiskRe = mustGuarded(`\b(?:pengen|pgn|pngn|pingin|pengin|kepengen|kepingin|ingin|mau|mw|keinginan|pikiran|kepikiran|` +
		`mikir|mikirin|kebayang|niat|berniat|rencana|berencana|bakal|akan|nekat|mati|meninggal|bunuh|hidup|idup|kuat|` +
		`sanggup|tahan|capek|cape|lelah|nyerah|menyerah|putus\s+asa|selesai|ngilang|menghilang|lenyap|want|wanna|` +
		`die|dead|suicide|kill|tired|thinking|thought)\b`)
	// samaFollowers continue "aku sama" as "I'm the same" ("aku sama kayak
	// dia"); any other word makes it "me and ..." or "with".
	samaFollowers = setOf("persis kayak kaya kek kyk seperti gitu gtu begitu gini begini aja saja kok sih deh " +
		"banget bgt juga jg")
	// englishContinuations continue an English short form as adoption: a
	// preposition or conjunction opens an adverbial ("so am I at night").
	englishContinuations = setOf("at in on when while like by with during after before since as because " +
		"cause sometimes too also lately rn now right")
	// trailingWords carry no content after a short adoption form: particles,
	// laughter spelt as a word, time and degree adverbs and address terms.
	trailingWords = setOf("sih kok deh ya yah yaa yaaa lho loh nih dong tuh kan lah ah eh oh aja saja banget bgt " +
		"juga jg jga pun kali kayaknya sekarang skrg skrng akhir-akhir belakangan ini hari malam beneran " +
		"bener serius sumpah jujur sebenernya sebenarnya emang memang lagi udah tbh honestly fr ngl too also " +
		"sometimes kadang kadang-kadang actually literally really so bro sis guys lately rn now right today " +
		"tonight kemarin kemaren tadi tho though lowkey lol lmao lmfao rofl haha hehe huhu hiks hix xixi " +
		"wkwk wkwkwk hmm huft hufft " +
		// Chat fillers and swear interjections (review finding).
		"idk idc ugh omg omfg af asf fr frfr srsly smh ikr ong fml much bad badly anjir njir anjay anjrit anjrot " +
		"jir bjir anjim asli cuy coy buset")
)

// splitAlternation returns one guarded expression per top-level branch of
// expr (expr itself when it is not an alternation).
func splitAlternation(expr string) []guarded {
	re, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		panic("safety: invalid expression " + expr + ": " + err.Error())
	}
	if re.Op != syntax.OpAlternate {
		return []guarded{mustGuarded(expr)}
	}
	out := make([]guarded, 0, len(re.Sub))
	for _, sub := range re.Sub {
		out = append(out, mustGuarded(sub.String()))
	}
	return out
}

// quotesIn returns the quotations inside cl, in order.
func (c *shape) quotesIn(cl *clause) []span {
	lo := sort.Search(len(c.quotes), func(i int) bool { return c.quotes[i].start >= cl.start })
	hi := lo
	for hi < len(c.quotes) && c.quotes[hi].end <= cl.end {
		hi++
	}
	return c.quotes[lo:hi]
}

func (c *shape) analyseClause(cl *clause) {
	t := c.text
	segs := cl.segments
	for i, seg := range segs {
		last := i == len(segs)-1
		d := denialSpan{span: seg}
		if !denialRe.MatchString(t[seg.start:seg.end]) {
			// "I'm not suicidal just tired of living": the denial ends
			// before "just", and what follows counts (review finding). Only
			// a listed tiredness or passive-ideation phrase continues a
			// denial this way; any other continuation ("just yet", "just
			// kidding", "cuma belum berani") leaves the segment the
			// speaker's own words, as in v1.
			loc := denialJustRe.FindStringSubmatchIndex(t[seg.start:seg.end])
			if loc == nil {
				continue
			}
			if rest := t[seg.start+loc[1] : seg.end]; !endsDenial(rest, sentenceAfter(t, seg.end)) {
				continue
			}
			d.end = seg.start + loc[3]
		}
		d.hedged = last && cl.contrastNext
		// A comma segment, or the next clause, that "just"/"cuma" opens
		// qualifies the denial before it ("I don't want to kill myself,
		// just yet", "I'm not suicidal. just kidding"; see justQualifier),
		// and so does a retraction or a deferral that opens it without
		// "just", after laughter or a filler ("I'm not suicidal, jk", "...,
		// haha just kidding", "aku gak mau bunuh diri, boong deng", "...,
		// yet"; see retraction).
		nsp := span{-1, -1}
		if !last {
			nsp = segs[i+1]
		} else {
			nsp = c.nextClauseSpan(cl)
		}
		next := ""
		if nsp.start >= 0 && (!last || !cl.contrastNext) {
			next = t[nsp.start:nsp.end]
		}
		next = stripFillers(next)
		if rest, ok := justRest(next); ok {
			switch laterKind(rest, sentenceAfter(t, nsp.end), continuationKind(rest)) {
			case qualVoid:
				continue
			case qualHedge:
				d.hedged = true
			default:
				d.hedged = d.hedged || (i+1 == len(segs)-1 && cl.contrastNext)
			}
		} else if retraction(next) {
			continue
		} else if nsp.start >= 0 && laterOwnAct(wordPrefix(t[nsp.start:nsp.end], checkWindow)+sentenceAfter(t, nsp.end)) {
			// "I'm not suicidal, I promise I'll do it tonight", "...,
			// I've got the pills ready", "aku gak mau bunuh diri, serius
			// besok aku lakuin" (review finding: v1 crisis, none since
			// e9711af).
			continue
		}
		cl.denials = append(cl.denials, d)
	}

	quoted := c.attributedQuotes(cl)
	cl.scopes = append(cl.scopes, quoted...)
	cl.hidden = append(cl.hidden, quoted...)
	// Every quotation in the clause is masked when reported content is
	// checked for the speaker's words and breakers: a quotation inside a
	// frame ("di lagu itu penyanyinya bilang \"aku ingin mati\"") is part of
	// the report even when no reporting frame sits right before it.
	allQuotes := c.quotesIn(cl)

	// A frame starts at the beginning of a comma segment and may run past a
	// comma ("in the book, the character ...", "kata dia, ..."). Its reported
	// content is the rest of the segment where it ends (or the next segment
	// when it ends exactly at one), plus each following segment that opens
	// with a third-person pronoun, up to the next frame. Within each part the
	// content ends at a breaker that starts the speaker's own reaction
	// ("Nyokap bilang harus nikah rasanya pengen mati"). Content that opens
	// with a closed report, a quotation or an anaphoric "gitu" ("bilang
	// \"semangat ya\" pengen mati", "ngomong gitu pengen mati"), ends there
	// unless a third-person pronoun opens what follows. Each segment belongs
	// to at most a few frames, so the work is linear in the clause length.
	frameEnd := make([]int, len(segs)) // 0: the segment opens no frame
	media := make([]bool, len(segs))   // the frame is a media frame
	for k, seg := range segs {
		if end, branch, ok := matchFrameBranch(t, seg.start, cl.end); ok {
			frameEnd[k] = end
			media[k] = mediaBranches[branch]
		}
	}
	for k, fEnd := range frameEnd {
		if fEnd == 0 {
			continue
		}
		cl.hidden = append(cl.hidden, span{segs[k].start, fEnd})
		j := k
		for j < len(segs) && segs[j].end <= fEnd {
			j++
		}
		if j == len(segs) {
			continue // nothing follows the frame
		}
		if t[fEnd-1] == ':' {
			// Direct speech after a colon ("temanku bilang: aku mau mati")
			// is treated like a quotation, but only up to the end of its
			// segment, the end of a quotation that opens it, a first-person
			// reply ("aku jawab ...") or the start of the speaker's own words
			// after its opening words (see colonSpeechEnd: "mama bilang:
			// makan dulu aku mau mati", "makan dulu rasanya pengen mati",
			// "stop crying wanna die"; see adoptionStart: "... me too"): an
			// unquoted quotation has no visible end, so what follows takes
			// the safer path.
			sc := span{fEnd, segs[j].end}
			if end, ok := c.quotedReport(allQuotes, fEnd, sc.end); ok {
				sc.end = end
			}
			masked := maskSpans(t, sc, allQuotes)
			if loc := ownFrameRe.FindStringIndex(masked); loc != nil {
				sc.end = sc.start + loc[0]
				masked = masked[:loc[0]]
			}
			end := len(masked)
			if k := colonSpeechEnd(masked); k >= 0 {
				end = k
			}
			if k := adoptionStart(masked); k > 0 && k < end {
				end = k // "lirik lagunya: aku ingin mati me too :("
			}
			sc.end = sc.start + end
			if media[k] {
				// Song lyrics after a colon end, like any media report, at
				// a subjectless wish after the risk phrase ("lirik lagunya:
				// aku ingin mati pengen ngelakuin itu juga").
				sc = cutAtOwnWish(t, sc, allQuotes)
			}
			if sc = trimSpan(t, sc); sc.start < sc.end {
				cl.scopes = append(cl.scopes, sc)
				cl.hidden = append(cl.hidden, sc)
			}
			continue
		}
		var parts []span
		if end, ok := c.closedReport(allQuotes, fEnd, segs[j].end); ok {
			parts = append(parts, span{fEnd, end})
			if rest := trimSpan(t, span{end, segs[j].end}); rest.start < rest.end && thirdContinuationRe.MatchString(t[rest.start:rest.end]) {
				parts = append(parts, cutAtBreaker(t, rest, allQuotes))
			}
		} else {
			p := cutAtBreaker(t, span{fEnd, segs[j].end}, allQuotes)
			if media[k] {
				p = cutAtOwnWish(t, p, allQuotes)
			}
			parts = append(parts, p)
		}
		for n := j + 1; n < len(segs) && frameEnd[n] == 0 && thirdPronounRe.MatchString(t[segs[n].start:segs[n].end]); n++ {
			parts = append(parts, cutAtBreaker(t, segs[n], allQuotes))
		}
		void, shared := false, false
		for _, p := range parts {
			masked := maskSpans(t, p, allQuotes)
			if firstPerson(masked) {
				void = true // the speaker is part of the reported content
				break
			}
			// "dia mau mati juga", or doing "the same" as the reported
			// act ("dia mau bunuh diri kepikiran buat ngelakuin hal yg
			// sama"): only the speaker can copy it.
			shared = shared || sharedRe.MatchString(masked) || sameActRe.MatchString(masked)
		}
		if void {
			continue
		}
		cl.shared = cl.shared || shared
		for _, p := range parts {
			if p.start < p.end {
				cl.scopes = append(cl.scopes, p)
				cl.hidden = append(cl.hidden, p)
			}
		}
	}
	cl.hidden = mergeSpans(cl.hidden)
}

// nextClauseSpan returns the first segment of the clause after cl, or
// {-1, -1} when cl is the last one. The first segment is enough for
// justRest.
func (c *shape) nextClauseSpan(cl *clause) span {
	k := sort.Search(len(c.clauses), func(i int) bool { return c.clauses[i].start >= cl.end })
	if k == len(c.clauses) || len(c.clauses[k].segments) == 0 {
		return span{-1, -1}
	}
	return c.clauses[k].segments[0]
}

// sentenceAfter returns the text of t after offset i up to the end of its
// sentence ('.', '!', '?' or a newline), at most checkWindow bytes and cut
// at a word boundary, so that reading it once per denial or match stays
// linear.
func sentenceAfter(t string, i int) string {
	if i < 0 || i >= len(t) {
		return ""
	}
	if k := strings.IndexAny(t[i:min(len(t), i+checkWindow)], ".!?\n"); k >= 0 {
		return t[i : i+k]
	}
	return wordPrefix(t[i:], checkWindow)
}

// wordPrefix returns s cut to at most n bytes at a word boundary.
func wordPrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && isLetter(s[n]) {
		n--
	}
	return s[:n]
}

// qualifierLead reports whether s opens with words that may qualify a
// denial before it: "just"/"cuma" or a retraction, after fillers (see
// analyseClause and contextUnits).
func qualifierLead(s string) bool {
	s = stripFillers(s)
	return justLeadRe.MatchString(s) || retraction(s)
}

// justRest returns the words after a "just"/"cuma" that opens seg.
func justRest(seg string) (string, bool) {
	loc := justLeadRe.FindStringIndex(seg)
	if loc == nil {
		return "", false
	}
	return seg[loc[1]:], true
}

// Kinds of qualification a "just"/"cuma" continuation gives a denial (see
// justQualifier).
const (
	qualNone  = iota // an ordinary continuation ("just tired", "cuma capek")
	qualHedge        // thinking, fear or a bare negation: the denial is hedged
	qualVoid         // a retraction, deferral, condition or lack of courage
)

// justQualifier reads the words after "just"/"cuma" that continue a denial.
// A retraction ("just kidding", "cuma bercanda"), a deferral ("just yet",
// "just not today", "only for now", "cuma belum berani", "untuk sekarang"),
// a condition ("only because of my mom", "hanya karena ibu") or a lack of
// courage ("just not brave enough", "merely because I'm a coward") takes
// the denial back: it does not count, and the risk phrase is the
// speaker's own. Thinking about it, fear or a negation ("cuma kadang
// kepikiran", "just scared", "just not okay") hedges it (review finding).
func justQualifier(rest string) int {
	words := strings.Fields(strings.ToLower(rest))
	if len(words) == 0 {
		return qualNone
	}
	if lim := rest[:min(len(rest), 2*checkWindow)]; justMeansRe.MatchString(lim) || justScaredRe.MatchString(lim) {
		return qualVoid
	}
	first := strings.Trim(words[0], ",.!?;:")
	if justDeferStart[first] || justCondition[first] {
		return qualVoid
	}
	if len(words) > 1 {
		pair := first + "_" + strings.Trim(words[1], ",.!?;:")
		if justDeferPairs[pair] {
			return qualVoid
		}
	}
	kind := qualNone
	if idNegators[first] || first == "not" || first == "no" || first == "never" {
		kind = qualHedge
	}
	for _, w := range words {
		w = strings.Trim(w, ",.!?;:'\"()")
		switch {
		case justVoidWords[w]:
			return qualVoid
		case justHedgeWords[w]:
			kind = qualHedge
		}
	}
	return kind
}

// continuationKind is the qualification the words after "just"/"cuma" give a
// denial: the stronger of justQualifier and wishQualifier.
func continuationKind(rest string) int {
	return max(justQualifier(rest), wishQualifier(rest))
}

// endsDenial reports whether the words after a "just"/"cuma" that continues
// a denial without a comma end the denial before them: a listed tiredness,
// sadness or venting phrase (see tiredRe), or a wish for a listed benign
// thing ("just need a break", "cuma pengen istirahat", "just want someone to
// talk to"; see benignWantRe), that nothing in the continuation qualifies
// (see continuationKind). A wish for anything else ("just want to not
// exist", "cuma pengen nggak ada", "just want to be left alone") leaves the
// whole segment the speaker's own words, as in v1 (review finding: a
// generic "want"/"pengen" ended the denial, so a passive death wish that no
// rule lists routed none).
//
// after is the rest of the sentence after the segment. The denial does not
// end either when the continuation or the rest of its sentence names a
// self-directed method, a deferral or a threat ("just need a break with my
// pills", "... before I end it", "... or I'll do something", "cuma pengen
// tidur, pakai semua obatku"), an absence or an ending in a later segment
// ("..., permanently") or another wish ("..., i want out"; see laterKind;
// review finding: the denial stayed exempt and the rest matched nothing).
func endsDenial(rest, after string) bool {
	if !tiredRe.MatchString(rest) && !benignWantRe.MatchString(strings.TrimLeft(rest, " ")) {
		return false
	}
	return laterKind(rest, after, continuationKind(rest)) == qualNone
}

// laterKind raises kind, the qualification that the words rest after a
// "just"/"cuma" give a denial (see continuationKind), by what follows them:
// after, the rest of the sentence after the segment of rest. A deferral to
// the act or a threat ("before I end it", "or I'll do something", "atau aku
// bakal nekat"), a later act ("I'll do it tonight"; see laterOwnAct), a
// self-directed method in a later segment, or in rest when nothing else
// qualifies it ("just want some rest with the whole bottle of pills"; "just
// need my meds" stays a hedge), and an absence or an ending in a later
// segment ("..., permanently", "..., selamanya") take the denial back. Any
// other wish in a later segment that is not for a listed benign thing
// ("..., i want out") hedges it.
func laterKind(rest, after string, kind int) int {
	if kind == qualVoid {
		return kind
	}
	rest = wordPrefix(rest, 2*checkWindow)
	both := rest + " " + after
	switch {
	case denialThreatRe.MatchString(both), laterOwnAct(both), denialMethodRe.MatchString(after), absenceRe.MatchString(after):
		return qualVoid
	case denialMethodRe.MatchString(benignTail(rest, kind)):
		return qualVoid
	}
	return max(kind, wishQualifier(after))
}

// benignTail returns the words of rest after its listed tiredness phrase or
// benign wish ("need a break| and the rope is ready"), the whole of rest
// when nothing qualifies it, and "" otherwise: a method noun there is not
// the object of another wish ("just need my meds" stays a hedge).
func benignTail(rest string, kind int) string {
	if loc := tiredRe.FindStringIndex(rest); loc != nil {
		return rest[loc[1]:]
	}
	t := strings.TrimLeft(rest, " ")
	if loc := benignWantRe.FindStringIndex(t); loc != nil {
		return t[loc[1]:]
	}
	if kind == qualNone {
		return rest
	}
	return ""
}

// laterOwnAct reports whether s, the words after a denial, report a later
// act of the speaker's: a first-person future or progressive "do it" or
// "end it" with a time or sincerity word ("I promise I'll do it tonight",
// "for real this time I'm going to do it", "serius besok aku lakuin"), a
// prepared means ("I've got the pills ready", "udah siapin obat"; see
// justMeansRe), or a set time ("tonight is the night"). Such words take
// the denial back.
func laterOwnAct(s string) bool {
	if justMeansRe.MatchString(s) {
		return true
	}
	if loc := setTimeRe.FindStringIndex(s); loc != nil && onlyFillers(s[loc[1]:]) {
		return true
	}
	if !laterTimeRe.MatchString(s) {
		return false
	}
	for _, loc := range laterActEnRe.FindAllStringIndex(s, -1) {
		w := nextWords(s, loc[1], 2)
		if laterActOthers[w[0]] || (w[0] == "for" && w[1] != "real" && w[1] != "good") {
			continue // "I'll end it with him tonight", "doing it for my kids"
		}
		if timedSegment(s, loc[0], loc[1]) {
			return true
		}
	}
	for _, loc := range laterActIdRe.FindAllStringIndex(s, -1) {
		if w := nextWords(s, loc[1], 1)[0]; (w == "" || laterActEnds[w]) && timedSegment(s, loc[0], loc[1]) {
			return true
		}
	}
	return false
}

// timedSegment reports whether a time or sincerity word stands in the comma
// segment of s around s[start:end] ("for real this time I'm going to do
// it"; not "I'm finally doing it, I'm moving out tonight").
func timedSegment(s string, start, end int) bool {
	lo, hi := start, end
	for lo > 0 && strings.IndexByte(",;:", s[lo-1]) < 0 {
		lo--
	}
	for hi < len(s) && strings.IndexByte(",;:", s[hi]) < 0 {
		hi++
	}
	return laterTimeRe.MatchString(s[lo:hi])
}

// onlyFillers reports whether s has no words but laughter and fillers.
func onlyFillers(s string) bool {
	for _, w := range strings.Fields(s) {
		w = strings.Trim(w, ",.!?;:'\"()")
		if w != "" && !laughter(w) && !laughterWords[w] && !fillerWords[w] {
			return false
		}
	}
	return true
}

// wishQualifier reads a wish in the continuation of a denial. A wish for an
// absence or an ending ("just want to be gone", "just need everything to
// stop", "cuma pengen nggak ada", "cuma mau udahan aja", "just want to rest
// forever"; see absenceRe) takes the denial back. Any other wish that is not
// for a listed benign thing ("just want the pain to stop", "cuma pengen
// pergi") hedges it. A wish for a listed benign thing ("just need a break",
// "cuma pengen curhat") leaves it (review finding).
func wishQualifier(rest string) int {
	if len(rest) > 2*checkWindow {
		rest = rest[:2*checkWindow]
	}
	any, other := false, false
	for _, loc := range desireRe.FindAllStringIndex(rest, -1) {
		any = true
		if !benignWantRe.MatchString(rest[loc[0]:]) {
			other = true
		}
	}
	switch {
	case any && absenceRe.MatchString(rest):
		return qualVoid
	case other:
		return qualHedge
	}
	return qualNone
}

// stripFillers drops up to three leading laughter or filler words ("haha",
// "lol", "eh", "ok") from the words that follow a denial, so "..., haha just
// kidding" reads like "..., just kidding".
func stripFillers(s string) string {
	for n := 0; n < 3; n++ {
		i := 0
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == ',') {
			i++
		}
		j := i
		for j < len(s) && isLetter(s[j]) {
			j++
		}
		if j == i || (j < len(s) && s[j] != ' ' && s[j] != ',' && s[j] != '\t') {
			return s[i:]
		}
		if w := s[i:j]; !laughter(w) && !laughterWords[w] && !fillerWords[w] {
			return s[i:]
		}
		s = s[j:]
	}
	return strings.TrimLeft(s, " \t,")
}

// retraction reports whether the words that open the segment or clause
// right after a denial, without "just"/"cuma", take it back: a retraction
// ("jk", "kidding", "not really", "that's a lie", "bercanda", "boong deng",
// "gak deng", "or do i") or a deferral ("yet", "not yet", "for now",
// "belum", "untuk sekarang"; review finding: only a "just"/"cuma"
// continuation was read, so "I'm not suicidal, jk" routed none). "yet" with
// words after it is the conjunction and does not retract.
func retraction(s string) bool {
	if len(s) > 64 {
		s = s[:64]
	}
	w := strings.Fields(s)
	for i := range w {
		w[i] = strings.Trim(w[i], ",.!?;:'\"()")
	}
	if len(w) == 0 {
		return false
	}
	if w[0] == "yet" {
		return len(w) == 1
	}
	if retractStart[w[0]] {
		return true
	}
	if len(w) > 1 {
		if retractPairs[w[0]+"_"+w[1]] {
			return true
		}
		if len(w) > 2 && retractTriples[w[0]+"_"+w[1]+"_"+w[2]] {
			return true
		}
	}
	return false
}

var (
	// fillerWords may open the words after a denial (see stripFillers).
	fillerWords = setOf("ok okay oke okey well um uh umm uhh hmm hm eh ah oh nah ya yah yaa sih deh lho loh tbh honestly ngl")
	// retractStart, retractPairs and retractTriples open a retraction or a
	// deferral (see retraction). Pairs and triples are written with "_".
	retractStart = setOf("jk jking jkjk kidding kiddin joking bercanda becanda bcanda canda guyon gurau boong bohong bo'ong " +
		"ngibul dusta deng belum blm prank lies lie lying psych psyche")
	retractPairs = setOf("not_really not_true a_lie i_lied i'm_lying im_lying i_lie or_am or_do or_will just_kidding " +
		"gak_deng ga_deng nggak_deng ngga_deng enggak_deng engga_deng gk_deng g_deng tidak_deng boong_ding bohong_ding " +
		"not_yet for_now untuk_sekarang buat_sekarang utk_sekarang not_today not_tonight no_wait wait_no")
	retractTriples = setOf("that's_a_lie thats_a_lie that's_not_true thats_not_true who_am_i at_least_not at_least_for " +
		"that_was_a it's_a_lie its_a_lie")
)

var (
	// desireRe finds a wish in the continuation of a denial (see
	// wishQualifier).
	desireRe = mustGuarded(`\b(?:want|wants|wanting|wanna|need|needs|needing|wish|wishing|hope|hoping|ready|` +
		`pengen|pgn|pngn|pingin|pengin|kepengen|kepingin|mau|mw|ingin|butuh|perlu|siap|rela|berharap|semoga)\b`)
	// benignWantRe is a wish for a listed benign thing: rest, sleep, a break,
	// someone to talk to, time alone (see endsDenial).
	benignWantRe = regexp.MustCompile(`^(?:want|wants|wanting|wanna|need|needs|needing|` +
		`pengen|pgn|pngn|pingin|pengin|kepengen|kepingin|mau|mw|ingin|butuh|perlu)\s+` +
		`(?:(?:to|for|a|an|some|more|my|the|little|bit|of|good|long|quick|real|proper|nice|hot|warm|big|someone|somebody|` +
		`be|get|go|take|have|aja|saja|banget|bgt|sih|deh|dulu|sebentar|bentar|buat|untuk|teman|temen|orang|yang|bisa)\s+){0,4}` +
		`(?:break|breather|rest|nap|sleep|vacation|holiday|hug|hugs|vent|talk|talking|space|time|quiet|peace|help|support|` +
		`advice|food|coffee|tea|friend|friends|mom|mum|dad|cry|scream|relax|chill|unwind|breathe|heard|alone|listen|` +
		`listened|weekend|shower|bath|walk|cuddle|therapy|therapist|counselor|counsellor|` +
		`istirahat|rehat|tidur|libur|liburan|cuti|healing|refreshing|jalan-jalan|curhat|cerita|ngobrol|didengerin|` +
		`didengar|didengarkan|dipeluk|pelukan|ditemenin|ditemani|sendiri|sendirian|nangis|menangis|teriak|makan|` +
		`waktu|ruang|bantuan|dukungan|psikolog|konseling)\b`)
	// absenceRe is an absence or an ending (see wishQualifier).
	absenceRe = mustGuarded(`\b(?:exist|existing|existence|alive|gone|vanish|vanishing|disappear|disappearing|` +
		`fade\s+away|fading\s+away|forever|for\s+good|permanently|eternally|dead|die|dying|death|never\s+wake|not\s+wake|` +
		`(?:not|never|no\s+longer)\s+(?:be\s+)?(?:here|around)|(?:be|being)\s+(?:here|around)\s+any\s*more|` +
		`(?:it|it\s+all|everything|all\s+of\s+(?:it|this)|my\s+life|this\s+life|life|things)\s+(?:to\s+)?(?:(?:just|finally|all)\s+)?(?:end|ends|stop|stops|be\s+over|is\s+over)|` +
		`hilang|ngilang|menghilang|lenyap|selamanya|abadi|udahan|tiada|mati|meninggal|` +
		`(?:nggak|ngga|gak|ga|gk|tidak|tdk|tak|enggak|engga)\s+(?:(?:usah|perlu)\s+)?(?:bangun|hidup|idup|bernapas|bernafas)|` +
		`(?:nggak|ngga|gak|ga|gk|tidak|tdk|tak|enggak|engga)\s+ada(?:\s+lagi)?(?:\s*$|\s+(?:lagi|lg|aja|saja|di\s+dunia|dari\s+dunia|selamanya|deh|ya))|` +
		`(?:semua(?:nya)?(?:\s+ini)?|segalanya|hidup(?:ku)?)\s+(?:(?:cepet|cepat|segera)\s+)?(?:berakhir|selesai|tamat|usai))`)

	// justVoidWords anywhere in the continuation take the denial back: a
	// retraction, a deferral, a plan or a lack of courage.
	justVoidWords = setOf("kidding kiddin jk jking joking joke jokes bercanda becanda bcanda canda candain guyon gurau bohong boong prank " +
		"yet belum blm blom blum lom brave courage courageous guts coward cowardly berani nyali pengecut cemen " +
		// waiting and planning (review finding: "just waiting for the right
		// moment", "just planning")
		"waiting wait nunggu nungguin menunggu tunggu planning plan plans planned rencana berencana ngerencanain merencanakan")
	// justMeansRe: preparing a means ("cuma udah siapin obat", "just got the
	// pills ready") takes the denial back (review finding).
	justMeansRe = mustGuarded(`\b(?:siapin|nyiapin|nyiapain|menyiapkan|disiapin|siapkan|ngumpulin|kumpulin|mengumpulkan|nimbun|beli|` +
		`prepared|preparing|prepping|ready|got|bought|stockpiling|stockpiled|saving|saved|collecting|collected|hoarding)\b.*\b(?:obat|pil|tali|pisau|silet|racun|` +
		`pills|meds|medication|rope|noose|knife|blade|razor|poison|gun|note|letter)\b|` +
		// The means first: "the pills are ready", "talinya udah siap".
		`\b(?:(?:obat|pil|tali|pisau|silet|racun)(?:ku|nya)?|pills|meds|rope|noose|knife|blade|razor|poison|gun|note|letter)\s+` +
		`(?:(?:is|are|'s|was|were|all|udah|sudah|sdh|dah|udh)\s+){0,2}(?:ready|prepared|set|siap|disiapin|disiapkan)\b`)
	// denialMethodRe is a self-directed method after a denial and its
	// continuation (see laterKind): the means of justMeansRe and a bottle.
	denialMethodRe = mustGuarded(`\b(?:(?:obat|pil|tali|pisau|silet|racun|botol|pistol)(?:ku|nya|mu)?|` +
		`pills?|meds|medications?|rope|noose|knife|blades?|razors?|poison|gun|bottle|overdose|overdosis)\b`)
	// denialThreatRe is a deferral to the act or a threat after a denial
	// and its continuation (see laterKind): "before I end it", "or I'll do
	// something", "atau aku bakal nekat".
	denialThreatRe = mustGuarded(`\b(?:before|or|otherwise|else|sebelum|atau|ato|kalau\s+nggak|kalo\s+nggak|kalo\s+gak|kalau\s+tidak)\s+` +
		`(?:(?:else|then|maybe|mungkin|nanti|ntar)\s+)?(?:i|i'll|ill|i'm|im|i'd|aku|gw|gue|gua|saya|ku)\s+` +
		`(?:(?:will|would|might|gonna|going\s+to|just|finally|actually|bakal|akan|mau|bisa|langsung|jadi|pasti)\s+){0,2}` +
		`(?:(?:do|doing)\s+(?:it|something|anything|this|that)|(?:end|ending)\s+(?:it|this|that|everything|things|myself|my\s+life)|` +
		`act\s+on\s+it|(?:kill|hurt)\s+(?:myself|me)|jump|go\s+through\s+with\s+it|nekat|bunuh\s+diri|` +
		`(?:lakuin|ngelakuin|melakukan|lakukan|akhiri|akhirin|ngakhirin|mengakhiri)(?:nya)?(?:\s+(?:itu|semua|semuanya|hidup|hidupku|aja|deh|beneran|malam|besok|sekarang|nanti)\b|\s*(?:$|[,.!?;])))`)
	// laterTimeRe is a time or sincerity word that makes a later act
	// definite (see laterOwnAct).
	laterTimeRe = mustGuarded(`\b(?:tonight|today|tomorrow|now|soon|later|this\s+time|this\s+week(?:end)?|midnight|for\s+real|no\s+lie|promise|seriously|` +
		`malam\s+ini|nanti\s+malam|ntar\s+malam|besok|bsk|sekarang|skrg|hari\s+ini|minggu\s+ini|segera|sebentar\s+lagi|bentar\s+lagi|nanti|beneran|serius|janji)\b`)
	// laterActEnRe is a first-person future or progressive "do it" or "end
	// it" (see laterOwnAct).
	laterActEnRe = mustGuarded(`\b(?:i'?ll|ill|i\s+will|i'?m|im|i\s+am)\s+(?:(?:going\s+to|gonna|gna|finally|actually|really|definitely|totally|seriously|just)\s+){0,2}` +
		`(?:(?:do|doing|go\s+through\s+with|going\s+through\s+with)\s+it|(?:end|ending)\s+(?:it|this|that))\b`)
	// laterActIdRe is an Indonesian "do it" without an object (see
	// laterOwnAct and laterActEnds): "besok aku lakuin", "malam ini
	// kulakukan".
	laterActIdRe = mustGuarded(`\b(?:(?:aku|gw|gue|gua|saya)\s+|ku)?(?:(?:akan|bakal|mau|pasti|langsung)\s+)?` +
		`(?:lakuin|ngelakuin|lakukan|melakukan|ngelakukan|eksekusi|akhiri|akhirin|ngakhirin|mengakhiri)(?:nya)?\b`)
	laterActEnds = setOf("itu aja saja deh kok sih lah ya yah dong beneran serius malam besok sekarang skrg nanti hari pas")
	// laterActOthers after "do it" or "end it" make it about someone or
	// something else (see laterOwnAct).
	laterActOthers = setOf("with between together")
	// setTimeRe is a set time for the act after a denial ("tonight is the
	// night"; see laterOwnAct).
	setTimeRe = mustGuarded(`\b(?:(?:tonight|today|tomorrow)(?:\s+is|'s|\s+will\s+be)\s+the\s+(?:night|day)|` +
		`(?:malam\s+ini|hari\s+ini|besok)\s+(?:adalah\s+)?(?:saatnya|waktunya|harinya|malamnya))\b`)
	// justScaredRe: fear of the act itself is a lack of courage ("just too
	// scared to do it", "cuma terlalu takut"; review finding).
	justScaredRe = mustGuarded(`\b(?:too|terlalu|masih|still)\s+(?:(?:much\s+of\s+a\s+)?(?:scared|afraid|takut|chicken|cowardly|weak)|of\s+a\s+(?:coward|chicken))\b|` +
		`\b(?:scared|afraid|takut)\s+(?:to\s+(?:do\s+it|do\s+that|try|actually|go\s+through)|(?:buat|untuk|utk)\s+(?:ngelakuin|melakukan|nyoba|mencoba)|ngelakuin|nyoba)\b`)
	// justDeferStart and justDeferPairs open a deferral ("just now", "just
	// not today", "cuma untuk sekarang").
	justDeferStart = setOf("now rn today tonight tomorrow sekarang skrg skrng sekarng saat hari malam besok")
	// Pairs are written with "_" for the space.
	justDeferPairs = setOf("not_now not_today not_tonight not_tomorrow not_rn not_yet for_now for_today for_tonight right_now " +
		"this_time this_week this_year untuk_sekarang buat_sekarang utk_sekarang untuk_saat buat_saat untuk_hari buat_hari " +
		"nggak_sekarang gak_sekarang ga_sekarang bukan_sekarang ngga_sekarang tidak_sekarang")
	// justCondition opens a condition for staying alive ("only because of
	// my mom", "hanya karena ibu", "cuma demi anakku").
	justCondition = setOf("because cause cuz coz bc since karena krn karna gara gara-gara garagara demi soalnya sebab")
	// justHedgeWords hedge the denial: thinking about it, sometimes, fear.
	justHedgeWords = setOf("think thinking thought thoughts kepikiran kpikiran mikir mikirin pikiran sometimes kadang kadang-kadang " +
		"scared afraid terrified frightened fear takut")
)

// attributedQuotes returns the quotations in cl that a third party said:
// those with a third-party reporting frame right before them ("temanku
// bilang \"...\"") or an attribution right after them ("\"...\", she said").
// A first-person reporting frame before the quotation ("aku cerita \"...\"",
// "I told her \"...\"") makes it the speaker's own words, whatever follows.
// An attribution after it counts only when no first-person word follows it
// before the next comma: in "\"I want to die\", she said I was dramatic"
// "she said" opens a new report rather than naming the quotation's speaker.
func (c *shape) attributedQuotes(cl *clause) []span {
	t := c.text
	var quoted []span
	for _, q := range c.quotesIn(cl) {
		pre := tailWindow(t, cl.start, max(cl.start, q.start-1))
		own := ownTailRe.leftmost(pre)
		if tp := reportingTailRe.leftmost(pre); tp >= 0 && tp > own {
			quoted = append(quoted, q)
			continue
		}
		if own >= 0 {
			continue
		}
		post := t[min(cl.end, q.end+1):min(cl.end, q.end+1+frameWindow)]
		loc := reportingHeadRe.FindStringIndex(post)
		if loc == nil {
			continue
		}
		rest := post[loc[1]:]
		if i := strings.IndexAny(rest, ",:\"'"); i >= 0 {
			rest = rest[:i]
		}
		if !firstPerson(rest) {
			quoted = append(quoted, q)
		}
	}
	return quoted
}

// closedReport reports where reported content that opens with a closed
// report ends: a quotation, optionally after the reporting verb ("bilang
// \"semangat ya\"", the content of a media frame such as "tokohnya bilang
// \"...\""), or an anaphoric "gitu"/"begitu" ("ngomong gitu", "bilang gitu
// doang"). Content that is not closed this way runs on (see analyseClause).
func (c *shape) closedReport(quotes []span, from, limit int) (int, bool) {
	if end, ok := c.quotedReport(quotes, from, limit); ok {
		return end, true
	}
	if loc := anaphorRe.FindStringIndex(c.text[from:min(limit, from+frameWindow)]); loc != nil {
		return from + loc[1], true
	}
	return 0, false
}

// quotedReport reports the end (past the closing mark) of a quotation that
// opens t[from:limit], optionally after a reporting verb and up to three
// words of manner or aspect ("bilang sambil marah \"...\"", "bilang lagi
// \"...\"").
func (c *shape) quotedReport(quotes []span, from, limit int) (int, bool) {
	t := c.text
	i := from
	for i < limit && (t[i] == ' ' || t[i] == ':') {
		i++
	}
	if loc := quoteVerbRe.FindStringIndex(t[i:min(limit, i+frameWindow)]); loc != nil {
		i += loc[1]
	}
	for n := 0; ; n++ {
		k := sort.Search(len(quotes), func(n int) bool { return quotes[n].start > i })
		if k < len(quotes) && quotes[k].start == i+1 && quotes[k].end < limit {
			return quotes[k].end + 1, true
		}
		if n == 3 {
			return 0, false
		}
		// Skip one word and the spaces after it.
		j := i
		for j < limit && (isLetter(t[j]) || t[j] == '-' || t[j] == '\'') {
			j++
		}
		if j == i {
			return 0, false
		}
		for j < limit && t[j] == ' ' {
			j++
		}
		i = j
	}
}

// laterOwnSubject returns the offset in colon speech s of a first-person
// subject pronoun that does not open it, or -1. The speech may open with the
// pronoun after a few discourse words ("aku capek", "kadang aku ..."): that
// one is the quoted person's "I". A later one ("aku capek aku mau mati",
// "makan dulu aku mau mati", "stop crying I want to die") starts the
// speaker's own words.
func laterOwnSubject(s string) int {
	opening := true
	for i := 0; i < len(s); {
		if !isLetter(s[i]) || (i > 0 && (isAlnum(s[i-1]) || s[i-1] == '\'')) {
			i++
			continue
		}
		j := i
		for j < len(s) && (isLetter(s[j]) || s[j] == '\'') {
			j++
		}
		w := s[i:j]
		switch {
		case ownPronouns[w]:
			if !opening {
				return i
			}
			opening = false
		case !speechLeads[w]:
			opening = false
		}
		i = j
	}
	return -1
}

// colonSpeechEnd returns the offset in colon speech s where the speaker's own
// words start, or -1: a later first-person subject (see laterOwnSubject),
// or, when the speech does not open with a first-person pronoun and no
// third-person pronoun comes first, a breaker word (see cutAtBreaker) or a
// subjectless wish after at least two words ("makan dulu rasanya pengen
// mati", "makan dulu pengen mati aja", "stop crying wanna die"). Speech that
// opens with the quoted person's "aku" keeps its subjectless continuation
// ("temanku bilang: aku capek terus pengen mati").
func colonSpeechEnd(s string) int {
	end := laterOwnSubject(s)
	cut := func(k int) {
		if end < 0 || k < end {
			end = k
		}
	}
	n := 0
	for i := 0; i < len(s) && (end < 0 || i < end); {
		if !isLetter(s[i]) || (i > 0 && (isAlnum(s[i-1]) || s[i-1] == '\'')) {
			i++
			continue
		}
		j := i
		for j < len(s) && (isLetter(s[j]) || s[j] == '\'' || s[j] == '-') {
			j++
		}
		w := s[i:j]
		switch {
		case n == 0 && ownPronouns[w], thirdPronouns[w]:
			return end
		case n >= 1 && breakerRe.MatchString(w) && !thirdPronouns[nextWord(s, j)]:
			cut(i)
		case n >= 1 && (w == "makes" || w == "made" || w == "making") && nextWord(s, j) == "me":
			cut(i) // "stop crying makes me want to die" (review finding)
		case n >= 2 && (speechWishes[w] || (w == "want" && nextWord(s, j) == "to")):
			cut(i) // "stop crying want to die" (review finding)
		}
		n++
		i = j
	}
	return end
}

// speechWishes are the wish words that start a subjectless clause.
var speechWishes = setOf("pengen pgn pingin pengin kepengen kepingin mau mw ingin wanna")

// cutAtOwnWish shortens the reported content of a media frame at a
// subjectless wish or thought that follows a complete risk phrase ("aku baca
// berita ada mahasiswa bunuh diri pengen nyusul"): the wish is the speaker's
// reaction.
// A wish before the risk phrase ("ada mahasiswa mau bunuh diri") or tied to
// the reported person ("yang pengen", "dia pengen", "karena pengen") stays
// in the report.
func cutAtOwnWish(t string, sp span, quoted []span) span {
	masked := maskSpans(t, sp, quoted)
	risk := false
	prev := ""
	for i := 0; i < len(masked); {
		if !isLetter(masked[i]) || (i > 0 && isAlnum(masked[i-1])) {
			i++
			continue
		}
		j := i
		for j < len(masked) && (isLetter(masked[j]) || masked[j] == '-') {
			j++
		}
		w := masked[i:j]
		if risk && (speechWishes[w] || reactionWishes[w]) && !wishOwners[prev] {
			return trimSpan(t, span{sp.start, sp.start + i})
		}
		risk = risk || riskEnds[w]
		prev = w
		i = j
	}
	return sp
}

var (
	// reactionWishes also start the speaker's reaction after a reported act
	// ("... bunuh diri kepikiran buat ngelakuin hal yang sama").
	reactionWishes = setOf("kepikiran mikir kebayang berniat niat pengennya want thinking")
	riskEnds       = setOf("diri mati meninggal tewas hidupnya gedung jembatan atap lantai himself herself themselves life dead " +
		"die suicide bridge building roof cliff window balcony train tracks")
	wishOwners = setOf("yang dia ia mereka beliau doi who that he she they karena soalnya because since biar supaya untuk buat")
)

// ownPronouns are first-person subject pronouns.
var ownPronouns = setOf("aku saya gue gua gw ane i i'm im i've i'd i'll")

// speechLeads may come before the quoted person's own "I" at the start of
// colon speech.
var speechLeads = setOf("kadang kadang-kadang sebenernya sebenarnya jujur aduh duh ya sih kayaknya mungkin kalau kalo " +
	"klo pas waktu honestly sometimes maybe actually really well so oh ugh if when and dan hmm um eh")

// cutAtBreaker shortens reported content at the first breaker word that is
// not attached to a third-person pronoun: "bikin", "rasanya", "sekarang",
// "terus", "jadi" and the like start the speaker's own reaction in
// subjectless Indonesian ("Pacarku bilang putus rasanya pengen mati"), while
// "dia terus ...", "terus dia ..." and "gitu terus dia ..." keep the
// third-party subject. Breakers inside quotations are ignored.
func cutAtBreaker(t string, sp span, quoted []span) span {
	masked := maskSpans(t, sp, quoted)
	for _, loc := range breakerRe.FindAllStringIndex(masked, -1) {
		before, _ := tokenBefore(masked, loc[0])
		after := loc[1]
		for {
			next := breakerRe.FindStringIndex(masked[after:])
			if next == nil || strings.TrimSpace(masked[after:after+next[0]]) != "" {
				break
			}
			after += next[1]
		}
		if thirdPronouns[before] || thirdPronouns[nextWord(masked, after)] {
			continue
		}
		return trimSpan(t, span{sp.start, sp.start + loc[0]})
	}
	return sp
}

// firstPerson reports whether s contains a first-person word: a pronoun
// (firstPersonRe) or a word with the possessive enclitic -ku ("hatiku",
// "terakhirku"), except relatives and roles that name a third party
// ("temanku", "ibuku") and words that merely end in "ku" ("buku").
func firstPerson(s string) bool {
	if hasFirstPersonWord(s) {
		return true
	}
	for i := 0; i < len(s); {
		if !isLetter(s[i]) {
			i++
			continue
		}
		j := i
		for j < len(s) && (isLetter(s[j]) || s[j] == '-') {
			j++
		}
		w := s[i:j]
		i = j
		stem, ok := strings.CutSuffix(w, "ku")
		if !ok || kuWords[w] {
			continue
		}
		stem = strings.TrimSuffix(stem, "-")
		if len(stem) >= 2 && !kuRelatives[stem] {
			return true
		}
	}
	return false
}

// firstPersonList holds the words of firstPersonRe. The contractions ("i'm",
// "i've") are matched through their "i", as \b allows.
const firstPersonList = "aku saya gue gua gw ane daku ku diriku hidupku nyawaku jiwaku tanganku lenganku nadiku " +
	"leherku pahaku tubuhku badanku kita kami i i'm im i've ive i'd i'll me my myself mine we us our ours ourselves"

var firstPersonWords = setOf(firstPersonList)

// hasFirstPersonWord reports whether firstPersonRe matches s, by looking up
// each \w word of s (a scan is much faster than the alternation on the many
// short windows the post-checks look at).
func hasFirstPersonWord(s string) bool {
	for i := 0; i < len(s); {
		if !isWordByte(s[i]) {
			i++
			continue
		}
		j := i
		for j < len(s) && isWordByte(s[j]) {
			j++
		}
		if firstPersonWords[s[i:j]] {
			return true
		}
		i = j
	}
	return false
}

// kuRelatives are nouns whose -ku form names a third party ("temanku"), so
// it is not a first-person marker.
var kuRelatives = setOf("teman temen sahabat pacar adik adek kakak abang ibu ayah mama papa mamah papah mami papi " +
	"bapak nyokap bokap sepupu tetangga rekan kolega om tante paman bibi anak suami istri mantan bestie kenalan " +
	"murid klien pasien senior junior kakek nenek cucu keponakan ponakan saudara sodara kawan gebetan dokter " +
	"psikolog guru dosen bos atasan sekelas sekantor dekat kecil tokoh karakter")

// kuWords end in "ku" without the enclitic.
var kuWords = setOf("buku kuku paku saku suku laku berlaku perilaku baku kaku beku membeku siku liku tungku selaku " +
	"haiku sudoku")

// thirdPronouns keep a breaker inside reported content ("dia terus ...").
var thirdPronouns = setOf("dia ia mereka beliau doi she he they")

// mergeSpans sorts spans and merges overlapping or touching ones.
func mergeSpans(in []span) []span {
	if len(in) < 2 {
		return in
	}
	sort.Slice(in, func(a, b int) bool { return in[a].start < in[b].start })
	out := in[:1]
	for _, s := range in[1:] {
		last := &out[len(out)-1]
		if s.start <= last.end {
			last.end = max(last.end, s.end)
			continue
		}
		out = append(out, s)
	}
	return out
}

// frameWindow bounds the text a frame expression looks at. Frames are a few
// words long (about 110 bytes at most), so this keeps the work per segment
// constant on very long clauses.
const frameWindow = 160

// tailWindow returns t[start:end], cut to its last frameWindow bytes at a
// word start, for expressions anchored at the end.
func tailWindow(t string, start, end int) string {
	if end-start <= frameWindow {
		return t[start:end]
	}
	w := end - frameWindow
	for w < end && t[w-1] != ' ' {
		w++
	}
	return t[w:end]
}

// matchFrame matches a third-party frame that starts at t[start], after up
// to three lead-in words ("jadi, kemarin temanku bilang ..."), reading no
// further than limit. It returns the end of the frame.
func matchFrame(t string, start, limit int) (int, bool) {
	end, _, ok := matchFrameBranch(t, start, limit)
	return end, ok
}

// matchFrameBranch is matchFrame that also returns the frameRe branch that
// matched.
func matchFrameBranch(t string, start, limit int) (int, int, bool) {
	off, ok := frameStart(t[start:limit])
	if !ok {
		return 0, 0, false
	}
	from := start + off
	end := min(limit, from+frameWindow)
	loc, branch := frameFindBranch(t[from:end])
	if loc != nil && from+loc[1] == end && end < limit {
		loc, branch = frameFindBranch(t[from:limit]) // the match reached the window edge
	}
	if loc == nil {
		return 0, 0, false
	}
	// Trailing spaces are not part of the frame, so a frame that ends in a
	// colon ends at it ("lirik lagunya: ...").
	end = from + loc[1]
	for end > from && t[end-1] == ' ' {
		end--
	}
	return end, branch, true
}

// frameFind is frameRe.FindStringIndex restricted to the branches that can
// begin with the first word of s. A frame is tried on every comma segment,
// and trying all eight branches there dominated the cost of long texts.
func frameFind(s string) []int {
	loc, _ := frameFindBranch(s)
	return loc
}

// frameFindBranch is frameFind that also returns the index of the branch.
func frameFindBranch(s string) ([]int, int) {
	w := firstWord(s)
	for i, g := range frameRe.alts {
		if frameOpens[i](w) {
			if loc := g.FindStringIndex(s); loc != nil {
				return loc, i
			}
		}
	}
	return nil, -1
}

// mediaBranches are the frameRe branches of media frames: "dalam film itu
// tokoh ...", "in the movie, the character ...", the speaker reading or
// watching a report about someone else, and song lyrics before a colon.
var mediaBranches = map[int]bool{0: true, 1: true, 8: true, 9: true}

// frameOpens[i] reports whether branch i of frameRe can begin with word w.
// TestFrameDispatch checks it against trying every branch.
var frameOpens = []func(w string) bool{
	func(w string) bool { return w == "dalam" || w == "di" },
	func(w string) bool { return w == "in" },
	func(w string) bool {
		return strings.HasPrefix(w, "tokoh") || strings.HasPrefix(w, "karakter") || strings.HasPrefix(w, "pemeran")
	},
	func(w string) bool { return w == "the" },
	func(w string) bool { return w == "katanya" || w == "kata" || w == "menurut" },
	func(w string) bool { return w == "according" },
	func(w string) bool { return idSubjectStarts[w] || hasFrameStem(w) },
	func(w string) bool { return enSubjectStarts[w] },
	func(w string) bool { return ownSubjects[w] },
	func(w string) bool { return lyricStarts[w] },
}

var lyricStarts = setOf("the this that song lyrics lyric lirik liriknya")

var (
	idSubjectStarts = setOf("dia ia mereka beliau seseorang orang")
	enSubjectStarts = setOf("he she they someone somebody this the my a our his her their")
)

// frameStart skips opening brackets and emphasis marks, up to three lead-in
// words ("honestly", "jadi", "kemarin") or one-word labels with a colon
// ("Curhat: my friend said ..."), and reports where the frame proper would
// begin, if the next word can begin one. It keeps the lead-in out of the
// expressions and frameRe off segments that cannot match.
func frameStart(seg string) (int, bool) {
	off := 0
	for off < len(seg) && strings.IndexByte("([{*~>- ", seg[off]) >= 0 {
		off++
	}
	for n := 0; n <= 3; n++ {
		w := firstWord(seg[off:])
		if w == "" {
			return 0, false
		}
		if frameStarts[w] || hasFrameStem(w) {
			return off, true
		}
		label := off+len(w) < len(seg) && seg[off+len(w)] == ':'
		if !leadInWords[w] && !label {
			return 0, false
		}
		off += len(w)
		for off < len(seg) && strings.IndexByte(" ,:\t\n", seg[off]) >= 0 {
			off++
		}
	}
	return 0, false
}

func firstWord(s string) string {
	i := 0
	for i < len(s) && (isLetter(s[i]) || s[i] == '\'') {
		i++
	}
	return s[:i]
}

// hasFrameStem reports whether w is a relative or role noun with a
// possessive suffix ("temanku", "kakaknya", "tokohnya").
func hasFrameStem(w string) bool {
	for _, stem := range frameStems {
		if strings.HasPrefix(w, stem) {
			return true
		}
	}
	return false
}

var frameStems = []string{
	"teman", "temen", "sahabat", "pacar", "adik", "adek", "kakak", "abang", "ibu", "ayah",
	"mama", "papa", "bapak", "nyokap", "bokap", "sepupu", "tetangga", "rekan", "kolega",
	"om", "tante", "paman", "bibi", "anak", "suami", "istri", "mantan", "bestie", "kenalan",
	"murid", "klien", "pasien", "senior", "junior", "tokoh", "karakter", "pemeran", "kata",
}

var frameStarts = setOf("dalam di in kata katanya menurut according dia ia mereka beliau seseorang orang " +
	"he she they someone somebody this the my a our his her their " +
	"aku saya gue gua gw ane ku i i'm im that song lyrics lyric lirik liriknya")

var leadInWords = setOf("jadi terus trus lalu kemarin kemaren tadi barusan semalam btw eh oh hmm so well " +
	"yesterday today earlier recently anyway anyways and dan also guys honestly tbh idk ngl fyi lol omg ok okay " +
	"oke um uh like basically literally jujur pokoknya sebenernya sebenarnya intinya nah ya yah duh aduh wkwk")

func setOf(words string) map[string]bool {
	m := make(map[string]bool)
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}

// maskSpans returns text[within] with the parts covered by hide blanked out.
// hide must be sorted by start and free of overlaps (quotations and merged
// hidden spans are), so only the spans that meet within are visited.
func maskSpans(text string, within span, hide []span) string {
	s := text[within.start:within.end]
	k := sort.Search(len(hide), func(i int) bool { return hide[i].end > within.start })
	if k == len(hide) || hide[k].start >= within.end {
		return s
	}
	b := []byte(s)
	for ; k < len(hide) && hide[k].start < within.end; k++ {
		for i := max(hide[k].start, within.start); i < min(hide[k].end, within.end); i++ {
			b[i-within.start] = ' '
		}
	}
	return string(b)
}

// findQuotes returns the content spans of balanced quotations, sorted by
// position. Double quotes pair in order; single quotes count only when they
// open after a space or punctuation and close before one, so apostrophes
// ("don't") never do. A quotation never spans a newline or a sentence
// boundary (see oneSentence), and a double-quoted one never ends with a
// third-party reporting frame after a comma (see endsInReport): such a pair
// is taken to start at a stray quote mark, and the closing mark is tried as
// a new opening one. Otherwise one stray mark could turn the speaker's own
// sentences or clauses into a quotation.
func findQuotes(s string) []span {
	if strings.IndexAny(s, "\"'") < 0 {
		return nil
	}
	var out []span
	open := -1
	for i := 0; i < len(s); i++ {
		if s[i] != '"' {
			continue
		}
		switch {
		case open < 0:
			open = i
		case oneSentence(s[open+1:i]) && !endsInReport(s[open+1:i]):
			out = append(out, span{open + 1, i})
			open = -1
		default:
			open = i
		}
	}
	doubles := len(out)
	d := 0 // first double-quoted pair that may overlap the next candidate
	open = -1
	for i := 0; i < len(s); i++ {
		if s[i] != '\'' {
			continue
		}
		prevOK := i == 0 || strings.IndexByte(" \n(:,\"", s[i-1]) >= 0
		nextOK := i+1 == len(s) || strings.IndexByte(" \n.,!?;:)\"", s[i+1]) >= 0
		canOpen := prevOK && i+1 < len(s) && isLetter(s[i+1])
		if open >= 0 && nextOK && i > 0 && s[i-1] != ' ' {
			if oneSentence(s[open+1 : i]) {
				// The double-quoted pairs are in order and so are the
				// single-quoted candidates, so one moving index finds the
				// pairs that could overlap this one.
				whole := span{open, i + 1}
				for d < doubles && out[d].end <= whole.start {
					d++
				}
				inside := d < doubles && out[d].start < whole.end
				if !inside {
					out = append(out, span{open + 1, i})
				}
				open = -1
				continue
			}
			open = -1
		}
		if open < 0 && canOpen {
			open = i
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].start < out[b].start })
	return out
}

// endsInReport reports whether quoted content q ends with a third-party
// reporting frame ("semangat ya, aku mau bunuh diri malam ini, dia bilang "):
// the closing mark then opens the next quotation, and the opening mark was
// stray, as when a quotation would span a sentence boundary.
func endsInReport(q string) bool {
	if !strings.Contains(q, ",") {
		return false // a one-clause quotation cannot hold a report and more
	}
	return reportingTailRe.leftmost(tailWindow(q, 0, len(q))) >= 0
}

// oneSentence reports whether quoted content q is a single line holding at
// most one sentence: no newline, and no '.', '!' or '?' followed by more
// words ("i want to die." and "aku capek..." qualify; "semangat ya. aku mau
// bunuh diri malam ini. dia bilang " does not).
func oneSentence(q string) bool {
	if strings.IndexByte(q, '\n') >= 0 {
		return false
	}
	end := false
	for i := 0; i < len(q); i++ {
		switch c := q[i]; {
		case c == '.' || c == '!' || c == '?':
			end = true
		case end && isAlnum(c):
			return false
		case c != ' ' && c != '\'' && c != ')':
			end = false
		}
	}
	return true
}

// conjunctions split clauses ("aku tidak mau bunuh diri tapi ...").
var conjunctions = setOf("tapi tetapi tp tpi namun padahal melainkan sedangkan but however although whereas")

// splitClauses splits s at sentence punctuation (. ! ? ;), newlines and
// contrastive conjunctions outside quotations, then each clause at commas
// outside quotations.
func splitClauses(s string, quotes []span) []clause {
	var quoted []bool
	if len(quotes) > 0 {
		quoted = make([]bool, len(s))
		for _, q := range quotes {
			for i := q.start; i < q.end; i++ {
				quoted[i] = true
			}
		}
	}
	inQuote := func(i int) bool { return quoted != nil && quoted[i] }

	var out []clause
	contrast := false // the clause about to be added follows a conjunction
	add := func(start, end int) {
		sp := trimSpan(s, span{start, end})
		if sp.start >= sp.end {
			return
		}
		cl := clause{span: sp, afterContrast: contrast}
		segStart := sp.start
		for i := sp.start; i < sp.end; i++ {
			if s[i] == ',' && !inQuote(i) {
				if seg := trimSpan(s, span{segStart, i}); seg.start < seg.end {
					cl.segments = append(cl.segments, seg)
				}
				segStart = i + 1
			}
		}
		if seg := trimSpan(s, span{segStart, sp.end}); seg.start < seg.end {
			cl.segments = append(cl.segments, seg)
		}
		out = append(out, cl)
		contrast = false
	}
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case inQuote(i):
			i++
		case c == '.' || c == '!' || c == '?' || c == ';' || c == '\n':
			add(start, i)
			i++
			start = i
		case isLetter(c) && (i == 0 || !isWordByte(s[i-1])):
			j := i
			for j < len(s) && isWordByte(s[j]) {
				j++
			}
			if conjunctions[s[i:j]] {
				add(start, i)
				// Any clause before the conjunction ends just before it:
				// clauses are only separated by punctuation and spaces.
				if len(out) > 0 {
					out[len(out)-1].contrastNext = true
				}
				contrast = true
				start = j
			}
			i = j
		default:
			i++
		}
	}
	add(start, len(s))
	return out
}

// trimSpan shrinks sp past surrounding spaces and commas.
func trimSpan(s string, sp span) span {
	for sp.start < sp.end && strings.IndexByte(" \n\t,", s[sp.start]) >= 0 {
		sp.start++
	}
	for sp.end > sp.start && strings.IndexByte(" \n\t,", s[sp.end-1]) >= 0 {
		sp.end--
	}
	return sp
}

// sentenceSpans splits s at '.', '!' and '?'. No rule pattern can consume
// those characters, so matching each sentence separately finds exactly the
// matches a whole-text search would, while keeping regexp inputs short.
func sentenceSpans(s string) []span {
	var out []span
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.', '!', '?':
			if i > start {
				out = append(out, span{start, i})
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, span{start, len(s)})
	}
	return out
}

// guarded is a regexp with a prefilter derived from the same expression
// (see prefilter.go); the regexp only runs on inputs the filter admits.
type guarded struct {
	re *regexp.Regexp
	f  *filter
}

func mustGuarded(expr string) guarded {
	return guarded{re: regexp.MustCompile(expr), f: compileFilter(expr)}
}

func (g guarded) MatchString(s string) bool {
	return (g.f == nil || g.f.match(s)) && g.re.MatchString(s)
}

func (g guarded) FindStringIndex(s string) []int {
	if g.f != nil && !g.f.match(s) {
		return nil
	}
	return g.re.FindStringIndex(s)
}

func (g guarded) FindAllStringIndex(s string, n int) [][]int {
	if g.f != nil && !g.f.match(s) {
		return nil
	}
	return g.re.FindAllStringIndex(s, n)
}

// altGuarded is an alternation split into one expression per branch. Small
// expressions let RE2 use its fast backtracker. FindStringIndex returns the
// first branch (in order) that matches.
type altGuarded struct{ alts []guarded }

// newAltAnchored builds an alternation whose branches are anchored at the
// start of the input. They run without a prefilter: an anchored expression
// fails within a few bytes, while a prefilter scans the whole input for each
// of its atoms, which on many short frame windows costs more than it saves.
func newAltAnchored(prefix, suffix string, branches ...string) altGuarded {
	var a altGuarded
	for _, b := range branches {
		a.alts = append(a.alts, guarded{re: regexp.MustCompile(prefix + b + suffix)})
	}
	return a
}

func (a altGuarded) MatchString(s string) bool {
	for _, g := range a.alts {
		if g.MatchString(s) {
			return true
		}
	}
	return false
}

func (a altGuarded) FindStringIndex(s string) []int {
	for _, g := range a.alts {
		if loc := g.FindStringIndex(s); loc != nil {
			return loc
		}
	}
	return nil
}

// tailFrame is a frame expression that must end where its input ends (just
// before a quotation). RE2 cannot search backwards, and an unanchored search
// for a match ending at $ runs the expression over the whole window. A frame
// is at most a dozen words and begins at a few known words, so instead the
// expression is anchored at both ends and tried only at the starts (after a
// space, comma or parenthesis) of the last tailWords words whose word can
// open the frame.
type tailFrame struct {
	re     *regexp.Regexp // expr with sorted alternations
	expr   string
	starts func(word string) bool
}

// tailWords bounds the length of a tail frame in words. The longest frames
// ("teman sekelas aku pernah tiba-tiba bilang ke aku kalau", "aku curhat ke
// ibu lewat chat") have about ten.
const tailWords = 14

func newTailFrame(starts func(string) bool, branches ...string) tailFrame {
	expr := `^(?:` + strings.Join(branches, `|`) + `)` + complement + `\s*[:,]?\s*$`
	return tailFrame{re: regexp.MustCompile(sortAlternations(expr)), expr: expr, starts: starts}
}

// sortAlternations rewrites expr with the branches of every alternation in
// sorted order, so that the parser factors their common prefixes ("bilang",
// "bicara" become "bi(?:lang|cara)") and RE2's NFA follows a few threads
// where the verb lists would otherwise start hundreds. Branch order only
// decides which match is reported, never whether one exists, so the result
// is equivalent for MatchString (not for FindStringIndex).
func sortAlternations(expr string) string {
	re, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		panic("safety: invalid expression " + expr + ": " + err.Error())
	}
	var walk func(*syntax.Regexp)
	walk = func(r *syntax.Regexp) {
		for _, sub := range r.Sub {
			walk(sub)
		}
		if r.Op == syntax.OpAlternate {
			slices.SortStableFunc(r.Sub, func(a, b *syntax.Regexp) int { return strings.Compare(a.String(), b.String()) })
		}
	}
	walk(re)
	return re.String()
}

// leftmost returns the start of the longest frame that ends at the end of
// s, or -1 when there is none.
func (f tailFrame) leftmost(s string) int {
	var at [tailWords]int
	n := 0
	for i := len(s) - 1; i >= 0 && n < tailWords; i-- {
		if isLetter(s[i]) && (i == 0 || strings.IndexByte(" \t\n,(", s[i-1]) >= 0) {
			at[n] = i
			n++
		}
	}
	for k := n - 1; k >= 0; k-- {
		if i := at[k]; f.starts(firstWord(s[i:])) && f.re.MatchString(s[i:]) {
			return i
		}
	}
	return -1
}

// opensReport reports whether w can open a third-party reporting frame: a
// frame start word, a relative or role noun, a bare reporting verb or a word
// that opens a lyrics frame.
func opensReport(w string) bool {
	return frameStarts[w] || hasFrameStem(w) || reportVerbs[w] || lyricStarts[w]
}

var reportVerbs = setOf("katanya tulisnya ujarnya ucapnya kata tulis ujar ucap")

// opensOwnReport reports whether w can open the speaker's own reporting
// frame.
func opensOwnReport(w string) bool { return ownSubjects[w] }

var ownSubjects = setOf("aku saya gue gua gw ane ku i i'm im")

// The parts of the denial expressions (see denialRe).
const (
	justWord   = `(?:just|only|merely|simply|cuma|cuman|cma|hanya)`
	denialLead = `(?:(?:honestly|tbh|ngl|jujur(?:\s+aja)?|sebenernya|sebenarnya|serius|seriously|actually|well|no|nah|nope|ok|okay|oke|` +
		`i\s+mean|to\s+be\s+honest|frankly|tenang(?:\s+aja)?|santai(?:\s+aja)?|beneran|sumpah)\s+)?`
	denialCore = `(?:` +
		`(?:aku|saya|gue|gua|gw|ku)\s+(?:tidak|ga|gak|gk|nggak|ngga|enggak|engga|tak|ndak)\s+(?:mau|ingin|akan|pengen|pingin|berniat|berencana|kepikiran\s+(?:buat|untuk))\s+` +
		`(?:bunuh\s*diri|bundir|mati|mengakhiri\s+hidup(?:ku)?|menyakiti\s+diri(?:ku|\s+sendiri)?|melukai\s+diri(?:ku|\s+sendiri)?)` +
		`(?:\s+(?:kok|sih|ya|kan|lho|loh|deh))?` +
		`|(?:i|i'm|im|i\s*am)\s+(?:do\s*not|don't|dont|am\s*not|not|won't|wont|will\s*not|would\s*never|will\s*never|never|would\s*not|wouldn't|wouldnt)\s+(?:ever\s+)?(?:going\s*to\s*|gonna\s*)?(?:want(?:ing)?\s*to\s*|wanna\s*|try(?:ing)?\s*to\s*|plan(?:ning)?\s*to\s*)?` +
		`(?:kill\s*myself|die|commit\s*suicide|hurt\s*myself|harm\s*myself|end\s*my\s*(?:own\s*)?life|take\s*my\s*(?:own\s*)?life)` +
		`|(?:i'm|im|i\s*am)\s+not\s+suicidal` +
		`)`
)

// Vocabulary shared by the frame expressions.
const (
	idSubject = `(?:dia|ia|mereka|beliau|seseorang|orang\s+itu|orang\s+lain|tokoh(?:nya)?|karakter(?:nya)?|` +
		`(?:teman|temen|sahabat|pacar|adik|adek|kakak|abang|ibu|ayah|mama|papa|mamah|papah|bapak|nyokap|bokap|sepupu|tetangga|rekan|kolega|om|tante|paman|bibi|anak|suami|istri|mantan|bestie|kenalan|murid|klien|pasien|senior|junior)` +
		`(?:\s+(?:sekelas|sekantor|kerja|kos|kosan|kantor|dekat|baik|kuliah|sekolah|lama|kecil))?(?:ku|nya|\s+(?:aku|saya|gue|gw|gua))?)`
	idAdverb   = `(?:pernah|tadi|barusan|baru|aja|saja|udah|sudah|sempat|sering|selalu|terus|juga|kemarin|kemaren|tiba-tiba|tibatiba|lagi|pas|semalam|katanya)`
	idVerb     = `(?:bilang|berkata|ngomong|bicara|cerita|bercerita|curhat|nulis|menulis|posting|ngepost|ngepos|nge-?post|update\s+status|bikin\s+status|nge-?chat|chat|ngetik|mengetik|kirim\s+pesan|ngirim\s+pesan|ngirim\s+chat|nge-?tweet|ngetwit|mengaku|ngaku|ngeluh|mengeluh|teriak|berteriak|nanya|bertanya|sms|nge-?dm|dm|nge-?wa|ngabarin|mengabari|share|ngeshare|ngirim|mengirim)`
	idReceiver = `(?:\s+(?:ke|sama|kepada|pada|ama)\s+(?:aku|saya|gue|gw|gua|ku|kami|kita))?`
	enSubject  = `(?:he|she|they|someone|somebody|this\s+(?:guy|girl|person|kid|friend|boy|man|woman)|the\s+(?:character|author|singer|victim|protagonist|narrator|hero|girl|boy|guy)|` +
		`(?:my|a|our|his|her|their)\s+(?:(?:best|close|old|good)\s+)?(?:friend|bestie|brother|bro|sister|sis|sibling|mom|mum|mother|dad|father|parent|cousin|partner|boyfriend|girlfriend|bf|gf|roommate|flatmate|housemate|classmate|co-?worker|colleague|son|daughter|kid|wife|husband|neighbou?r|student|client|patient|ex|teammate|aunt|uncle|niece|nephew|grandma|grandpa|teacher|boss)(?:\s+of\s+mine)?)`
	enAdverb   = `(?:just|once|recently|always|also|literally|actually|even|then|finally|suddenly|kept|keeps)`
	enVerb     = `(?:said|says|told|tells|texted|texts|wrote|writes|posted|posts|messaged|messages|tweeted|admitted|admits|mentioned|mentions|confessed|confided|shared|whispered|yelled|screamed|joked|jokes|dm'?d|dmed|(?:keeps|kept|was|is|has\s+been|have\s+been|been)\s+(?:saying|talking\s+about|posting\s+about|writing\s+about|joking\s+about|texting)|talks\s+about|talked\s+about)`
	enReceiver = `(?:\s+(?:me|us|to\s+me|to\s+us|everyone|people))?`
	complement = `(?:\s+(?:kalau|kalo|klo|bahwa|that|like|this|ini|gini|begini))?`
	// Code-switched frames mix the two vocabularies ("my bestie bilang ...",
	// "temanku said ...").
	anyAdverb   = `(?:` + idAdverb + `|` + enAdverb + `)`
	anyVerb     = `(?:` + idVerb + `|` + enVerb + `)`
	anyReceiver = `(?:` + idReceiver + enReceiver + `)`
	// idMedia names recorded or published content. Classes, lectures and
	// seminars are events the speaker attends, not content, so they are not
	// media frames ("di kelas tadi dosen marah terus pengen mati aja").
	idMedia = `(?:film|filem|movie|berita|pelajaran|artikel|buku|novel|cerita|cerpen|komik|series|serial|sinetron|drama|drakor|lagu|video|game|anime|manga|podcast|dokumenter|thread|utas|postingan|konten|episode)`
	// idMediaWho is a person inside the media, optionally introduced by
	// "ada" ("di berita tadi ada mahasiswa ...").
	idMediaWho = `(?:ada\s+)?(?:tokoh|karakter|pemeran|pemain|penulis|penyanyi|korban|pelaku|artis|aktor|aktris|seseorang|seorang|orang|dia|ia|mereka|remaja|mahasiswa|siswa|pelajar|anak|pria|wanita|lelaki|laki|perempuan|cowok|cewek|ibu|ayah|pembicara|narasumber|penyiar|youtuber|influencer|selebriti|seleb|idol)(?:nya)?`
	// kataSubject is the third party after "kata"/"menurut" ("kata dia",
	// "kata orang"). It is required: bare "kata" opens the speaker's own
	// phrases too ("kata hatiku", "kata terakhirku").
	kataSubject = `(?:` + idSubject + `|orang(?:[\s-]+orang)?)`
	// ownSubject and ownVerb describe the speaker's own reporting frame
	// ("aku cerita", "I told her", "aku tulis:").
	ownSubject = `(?:aku|saya|gue|gua|gw|ane|ku|i)`
	ownVerb    = `(?:` + anyVerb + `|tulis|jawab|balas|bales|tanya|nanya|bisik|ketik|ucap|ujar|kirim|ngechat|bikin\s+status|reply|replied|answer|answered|ask|asked|type|typed|say|tell|text|write|post|message|scream|yell|whisper|cry|cried|went|go|was\s+like|am\s+like)`
	// ownReceiver is up to two short phrases after the verb ("ke ibu", "my
	// mom", "lewat chat", "in my diary").
	ownReceiver = `(?:\s+(?:(?:ke|sama|kepada|pada|ama|buat|lewat|via|di|dalam|to|at|on|in|with|into)\s+)?(?:(?:my|our|the|a|this|that|his|her|their|si|sang)\s+)?[a-z']+){0,2}`
	enMedia     = `(?:movie|film|news|article|book|show|story|class|series|novel|song|video|game|episode|documentary|podcast|lecture|post|thread|comic|anime|manga|play|lesson|report|chapter|scene)`
	enMediaWho  = `(?:(?:the|a|an|this|that|one|some)\s+)?(?:(?:main|lead|young|local|famous)\s+)?(?:characters?|he|she|they|someone|somebody|man|woman|girl|boy|guy|kid|teen|teenager|student|person|people|hero|heroine|protagonist|author|singer|victim|narrator|teacher|speaker|actor|actress|celebrity|influencer|host|youtuber|reporter|lecturer|mother|father|soldier|officer|worker|patient|rapper|musician|athlete|player)`
	// mediaReport is the speaker reading, watching or hearing a report
	// about a third party.
	mediaReport = `(?:aku|saya|gue|gua|gw|i)\s+(?:(?:baru|tadi|barusan|sempat|just|recently|once)\s+)?(?:baca|membaca|nonton|menonton|denger|dengar|mendengar|lihat|liat|melihat|read|watched|saw|heard)\s+(?:(?:a|an|the|this|that|sebuah|satu)\s+)?(?:` +
		idMedia + `|` + enMedia + `|news\s+(?:article|story|report|piece))(?:\s+(?:itu|ini|tadi|kemarin|kemaren|today|yesterday|online))?` +
		`(?:\s+(?:tentang|soal|kalau|bahwa|about|that|where|of|on|in\s+which))?\s*,?\s*(?:` + idMediaWho + `|` + enMediaWho + `)`
	// lyricsFrame introduces song lyrics ("the lyrics say", "lirik
	// lagunya:"); with a colon or a quotation it attributes them.
	lyricsFrame = `(?:(?:the|this|that)\s+)?(?:song\s+)?lyrics?(?:\s+(?:say|says|go|goes|read|reads|are|is))?|lirik(?:\s+lagu)?(?:nya)?(?:\s+(?:itu|ini))?(?:\s+(?:bilang|bunyinya|berbunyi))?`
)

var (
	// denialRe is the v1 contextualDiscussion denial, applied to one comma
	// segment. Sela accepts the colloquial pronouns and negators, the same
	// verbs plus "pengen"/"berniat", the self-harm objects, trailing
	// discourse particles ("kok", "sih") and a lead-in word ("honestly",
	// "jujur"; review finding). Any other content makes it a normal segment
	// ("aku tidak mau bunuh diri lagi" is not a denial).
	denialRe = mustGuarded(`^` + denialLead + denialCore + `\s*[.!]?$`)

	// denialJustRe is a denial that "just"/"cuma" continues without a comma
	// ("I'm not suicidal just tired of living"): its first group is the
	// denial, and the continuation counts as the speaker's words.
	denialJustRe = regexp.MustCompile(`^(` + denialLead + denialCore + `)\s+` + justWord + `\b`)

	// justLeadRe is "just"/"cuma" opening a comma segment after a denial.
	justLeadRe = regexp.MustCompile(`^` + justWord + `\b`)

	// tiredRe is a tiredness, sadness or passive-ideation phrase after
	// "just"/"cuma" ("tired of living", "capek banget", "need a break",
	// "pengen istirahat"): the only continuation that ends a denial without
	// a comma (see analyseClause).
	//
	// A wish ("want", "need", "pengen", "butuh") is not listed: only a wish
	// for a listed benign thing ends a denial (see benignWantRe and
	// endsDenial; review finding).
	tiredRe = regexp.MustCompile(`^\s*(?:(?:so|really|very|kinda|kind\s+of|a\s+bit|a\s+little|pretty|super|extremely|feeling|feel|i'?m|im|am|lagi|lg|agak|rada|sangat|terlalu|udah|sudah|udh|bener|bgt|banget|aja|aj|sih|ngerasa|merasa)\s+)*` +
		`(?:tired|exhausted|sad|drained|burnt|burned|burnout|worn|stressed|stress|overwhelmed|lonely|numb|empty|depressed|down|low|upset|frustrated|hurt|hurting|broken|struggling|sick|venting|` +
		`having\s+(?:a|an|such\s+a|one\s+of\s+those)\s+(?:[a-z]+\s+){0,2}(?:day|days|time|week|night|month|year|moment|morning)|` +
		`capek|cape|capai|cpk|cpek|lelah|letih|sedih|stres|setres|pusing|jenuh|bosen|bosan|kesepian|hampa|kosong|berat|galau|bete|kesel|kecewa|hancur|rapuh|putus|curhat|cerita)\b`)

	// frameRe recognises a third-party frame at the start of a segment: the
	// v1 media frames ("dalam film itu tokoh ...", "in the movie, the
	// character ...", widened to more media and subjects) and reporting frames
	// ("temanku bilang", "kata dia", "my friend said", "according to her").
	frameRe = newAltAnchored(`^(?:`, `)`+complement+`\s*[:,]?`,
		`(?:dalam|di)\s+`+idMedia+`(?:\s+(?:itu|ini|tersebut|tadi|kemarin|kemaren))?(?:\s+yang\s+(?:aku|saya|gue|gw|kita|kami)\s+(?:tonton|baca|lihat|liat|dengar|denger))?\s*,?\s*`+idMediaWho+`\b`,
		`in\s+(?:the|that|a|an|this|my|our|today'?s|yesterday'?s)\s+`+enMedia+`(?:\s+(?:i|we)\s+(?:just\s+)?(?:watched|saw|read|heard|listened\s+to))?\s*,?\s*`+enMediaWho+`\b`,
		`(?:tokoh|karakter|pemeran)(?:nya)?(?:\s+utama(?:nya)?)?\b`,
		`the\s+(?:main\s+|lead\s+)?(?:character|protagonist|heroine|hero|villain)\b`,
		`(?:katanya|(?:kata|menurut)\s+`+kataSubject+`)\b`,
		`according\s+to\s+(?:him|her|them|`+enSubject+`)\b`,
		idSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb+anyReceiver+`\b`,
		enSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb+anyReceiver+`\b`,
		// The speaker reading or watching a report about someone else ("aku
		// baca berita ada mahasiswa ...", "I read a news article about a
		// student ..."): the frame needs the third party, so "aku baca
		// berita, pengen mati" stays the speaker's own.
		mediaReport+`\b`,
		// Song lyrics frame only before a colon ("lirik lagunya: ..."); as a
		// plain frame "lirik lagunya sedih banget pengen mati" would hide the
		// speaker's own reaction. Before a quotation see reportingTailRe.
		`(?:`+lyricsFrame+`)\s*:`,
	)

	// reportingTailRe recognises a reporting frame that ends just before a
	// quotation ("temanku nulis", "my friend texted me this:").
	reportingTailRe = newTailFrame(opensReport,
		`(?:`+lyricsFrame+`)`,
		`(?:katanya|tulisnya|ujarnya|ucapnya|(?:kata|tulis|ujar|ucap)\s+`+kataSubject+`)`,
		idSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb+anyReceiver,
		enSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb+anyReceiver,
	)

	// reportingHeadRe recognises an attribution right after a quotation
	// ("\"...\", she said", "\"...\" kata temanku ke aku"). Bare "kata",
	// "ucap" and the like may name anyone ("kata Budi"); attributedQuotes
	// rejects the match when a first-person word follows ("ucap saya").
	reportingHeadRe = newAltAnchored(`^\s*,?\s*(?:`, `)\b`,
		`(?:katanya|tulisnya|ujarnya|ucapnya|(?:kata|tulis|ujar|ucap)(?:\s+`+kataSubject+`)?)`+anyReceiver,
		idSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb+anyReceiver,
		enSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb+anyReceiver,
	)

	// ownTailRe recognises the speaker's own reporting frame just before a
	// quotation ("aku cerita", "aku curhat ke ibu", "I texted my mom",
	// "di diary aku tulis:", "I was like"). The quotation is then the
	// speaker's words even if a third-party tag follows it.
	ownTailRe = newTailFrame(opensOwnReport,
		ownSubject+`\s+(?:(?:`+anyAdverb+`|cuma|hanya|langsung|pun|then)\s+)*`+ownVerb+ownReceiver,
		`(?:i'm|im)\s+(?:like|all\s+like)`,
	)

	// ownFrameRe finds a first-person reply inside unquoted direct speech
	// ("temenku nanya: kamu kenapa aku jawab aku mau mati").
	ownFrameRe = regexp.MustCompile(`\b` + ownSubject + `\s+(?:(?:` + anyAdverb + `|cuma|langsung|then)\s+)*(?:jawab|balas|bales|bilang|tanya|nanya|cerita|curhat|replied|answered|said|told|texted|wrote|asked|say|tell|was\s+like)\b`)

	// breakerRe lists the words that end subjectless reported content (see
	// cutAtBreaker).
	breakerRe = regexp.MustCompile(`\b(?:bikin|bikinin|membuat|rasanya|rasa|sekarang|skrg|skrang|jadi|jadinya|makanya|akhirnya|sampai|sampe|sehingga|terus|trus|lalu|(?:abis|habis|setelah|sesudah)\s+itu)\b`)

	// anaphorRe: an anaphoric complement right after a reporting verb, which
	// closes the report ("nyokap ngomong gitu", "bilang begitu doang").
	anaphorRe = regexp.MustCompile(`^\s*(?:(?:kayak|kaya|kek|kyk|seperti)\s+)?(?:gitu(?:2|-gitu|an)?|begitu|gtu|gt|bgitu|(?:hal\s+)?itu)(?:\s+(?:doang|aja|deh|sih|lho|loh))?\b`)

	// quoteVerbRe: a reporting verb at the start of a media frame's content,
	// before its quotation ("tokohnya bilang \"...\"").
	quoteVerbRe = regexp.MustCompile(`^(?:` + anyVerb + `|sings|sang|singing|nyanyi|menyanyi|bernyanyi|screams|screamed|says|said)` + anyReceiver + `\s*:?\s*`)

	// thirdContinuationRe: text after a closed report that continues it,
	// a third-person pronoun after an optional connective ("\"aku capek\"
	// dan dia mau mati", "gitu terus dia ...").
	thirdContinuationRe = regexp.MustCompile(`^(?:(?:dan|terus|trus|lalu|abis\s+itu|habis\s+itu|and|then|so|jadi)\s+)?(?:dia|ia|mereka|beliau|he|she|they|he's|she's|they're|hes|shes|theyre)\b`)

	// thirdPronounRe: a segment that continues the reported content.
	thirdPronounRe = regexp.MustCompile(`^(?:dia|ia|mereka|beliau|he|she|they|he's|she's|they're|hes|shes|theyre|his|her|their)\b`)

	// firstPersonRe: first-person pronouns and common -ku forms (see
	// firstPerson, which also accepts any -ku word). Inside an attributed
	// scope (outside quotations) any of them makes the attribution void: the
	// speaker is part of it ("dalam film itu tokoh bunuh diri aku mau bunuh
	// diri").
	firstPersonRe = regexp.MustCompile(`\b(?:` + strings.Join(strings.Fields(firstPersonList), "|") + `)\b`)

	// sharedRe: "juga"/"too" inside reported content is read as the speaker
	// sharing it ("tokohnya bunuh diri, jadi pengen mati juga").
	sharedRe = regexp.MustCompile(`\b(?:juga|jg|too|as\s+well)\b`)

	// sameActRe: doing "the same" inside reported content is the speaker
	// copying the reported act.
	sameActRe = regexp.MustCompile(`\b(?:ngelakuin|nglakuin|lakuin|ngelakukan|melakukan|lakukan|doing|do|did)\s+(?:(?:hal\s+)?y(?:an)?g\s+sama|the\s+same)\b`)

	// adoptionRe: the speaker takes up the reported risk ("aku juga", "me too").
	adoptionRe = mustGuarded(`` +
		`\bsama\s+(?:aku|gue|saya|gw)\s+(?:juga|jg)\b` +
		`|\b(?:aku|gue|saya|gw)\s+(?:juga\s+)?(?:ngerasa|merasa|ngerasain|merasakan|ngalamin|mengalami)\s+(?:hal\s+yang\s+sama|yang\s+sama|gitu|begitu|hal\s+serupa|sama)\b` +
		`|\bme{1,2}\s+(?:too|as\s+well|neither|also)\b` +
		`|\bso\s+do\s+i\b` +
		`|\bsame\s+(?:here|for\s+me|tbh|honestly|lol|lmao|lmfao|fr|ngl|bro|sis|tho|though)\b` +
		`|\b(?:i'?m|i\s+am)\s+the\s+same\s+(?:way|too)\b` +
		`|\bi\s+(?:(?:kinda|really|totally|honestly|also|lowkey|sometimes|literally|actually|so|definitely)\s+)*(?:do|did|feel|felt|want|wanted|think|am|have)\s+(?:(?:the\s+)?same\s+|that\s+|it\s+|so\s+)?(?:too|as\s+well)\b` +
		`|\bi(?:'?ve|\s+have)?\s+(?:(?:been|also|kinda|really|lowkey|always)\s+)*(?:feel|feeling|felt)\s+(?:the\s+same|that\s+way|like\s+that|it\s+too|that\s+too)\b` +
		// "i've thought about it too", "i think about it too".
		`|\bi(?:'?ve|\s+have)?\s+(?:(?:also|kinda|sometimes|often|been|lowkey|really|honestly|always)\s+)*(?:think|thought|thinking)\s+(?:about|of)\s+(?:it|that|this|doing\s+(?:it|that|the\s+same))\s+(?:too|as\s+well|also|sometimes|a\s+lot|every\s+day|all\s+the\s+time|lately|before)\b` +
		`|\bi\s+(?:can\s+|totally\s+|really\s+)?relate\b|\brelat(?:able|e\s+(?:banget|bgt|sih|deh|parah))\b` +
		`|\b(?:i\s+(?:said|say|replied|answered|was\s+like)|lol|honestly|tbh|ngl|literally|fr|omg|ugh|so|lmao|damn|oh|yeah|yes|ya|bruh)\s+"?\s*(?:same|me\s+too)\b` +
		`|\bliterally\s+me\b|\bthat'?s\s+(?:so\s+|literally\s+|exactly\s+|just\s+)?(?:me|how\s+i\s+feel)\b` +
		`|\b(?:aku|gue|gw|gua|saya)\s+(?:juga\s+)?sama\s+(?:persis|kayak\s+dia|kaya\s+dia|kek\s+dia)\b` +
		`|\bsama\s+(?:kayak|kaya|kek|kyk|seperti|dengan|dgn|ama)\s+(?:aku|gue|gw|gua|saya)\b` +
		// Imitation of the reported act ("pengen nyusul", "aku pengen kayak
		// dia", "kok aku iri", "I want to try it").
		`|\b(?:pengen|pgn|pingin|pengin|kepengen|mau|mw|ingin|kepikiran|mikir)\s+(?:(?:buat|untuk)\s+)?(?:nyusul(?:in|nya)?|menyusul(?:nya)?|ikutan|nyoba(?:in)?|mencoba(?:nya)?|ngikutin)\b` +
		// A wish or a thought that refers back to the reported act ("jadi
		// pengen ngelakuin juga", "kepikiran buat ngelakuin hal yg sama",
		// "I'm thinking of doing the same", "I want to too"), with at most
		// 40 bytes in between.
		`|\b(?:pengen|pgn|pingin|pengin|kepengen|kepingin|mau|mw|ingin|kepikiran|mikir|mikirin|kebayang|berniat|niat|pengennya|want|wanna|wanted|thinking|considering|tempted|gonna|planning)\b[^,.!?;:\n"]{0,40}?\b(?:juga|jg|too|as\s+well|(?:hal\s+)?y(?:an)?g\s+sama|the\s+same|same\s+thing)\b` +
		`|\b(?:ngelakuin|nglakuin|lakuin|ngelakukan|melakukan|lakukan)\s+(?:(?:hal\s+)?itu\s+)?(?:juga|jg)\b` +
		`|\bi\s+(?:did|do|have\s+done|will\s+do|did\s+it|do\s+it)\s+(?:the\s+same|it\s+too|that\s+too|too)\b` +
		`|\b(?:pengen|pgn|pingin|pengin|kepengen|mau|mw|ingin)\s+(?:ikut|ngikut)(?:\s*$|\s+(?:aja|juga|jg|deh|sih|ya|lho|loh|kok|dia|mereka|nih|lah|dong|rasanya|mati|bunuh|banget|bgt|kayaknya)\b)` +
		`|\b(?:pengen|pgn|pingin|pengin|kepengen|mau|mw|ingin)\s+(?:(?:banget|bgt|bngt|juga|jg|sih|deh|aja)\s+)?(?:jadi\s+)?(?:kayak|kaya|kek|kyk|seperti)\s+(?:dia|gitu|mereka|itu|begitu)\b` +
		// "pengen ngelakuin itu", "pengen banget ngelakuin itu" (review
		// finding): a wish that refers back to the reported act.
		`|\b(?:pengen|pgn|pingin|pengin|kepengen|kepingin|mau|mw|ingin|kepikiran|mikir)\s+(?:(?:banget|bgt|bngt|juga|jg|sih|deh|aja|buat|untuk)\s+){0,2}(?:ngelakuin|nglakuin|lakuin|ngelakukan|melakukan|lakukan)(?:nya)?\s*(?:(?:hal\s+)?(?:itu|gitu|begitu)|kayak\s+(?:gitu|dia|itu)|kaya\s+(?:gitu|dia|itu))?\b` +
		`|\b(?:aku|gue|gw|gua|saya)\s+(?:jadi\s+|kok\s+|agak\s+|sedikit\s+)?iri\b` +
		`|\b(?:ngelakuin|nglakuin|melakukan|lakuin|ngelakukan|lakukan)\s+(?:hal\s+)?y(?:an)?g\s+sama\b` +
		`|\b(?:want|wanna|wanted)\s+(?:to\s+)?(?:try\s+(?:it|that)|do\s+(?:the\s+same|that|it)|join\s+(?:him|her|them)|follow\s+(?:him|her|them)|be\s+(?:next|like\s+(?:him|her|them)))\b` +
		`|\bi(?:'m|\s+am|m)\s+(?:so\s+|kinda\s+|a\s+bit\s+)?jealous\b|\bi\s+envy\s+(?:him|her|them)\b` +
		`|\b(?:aku|gue|saya|gw)\s+(?:bilang|jawab|balas)\s+"?\s*(?:sama|aku\s+juga|gue\s+juga)\b` +
		// "aku relate", "sama aja aku" (same for me).
		`|\b(?:aku|gue|gw|gua|saya|aq|ak|sy|w)\s+(?:juga\s+|jg\s+|sangat\s+|bener-bener\s+|beneran\s+)?relate\b` +
		`|^sama\s+(?:(?:aja|saja|kayak|kaya|kek|kyk|kok|sih|deh|juga|jg|dengan|dgn|ama)\s+)*(?:aku|gue|gw|gua|saya|aq|ak|sy)\b` +
		// A wish or envy the speaker attaches to the reported act, or the
		// speaker next in line: "I wish that was me", "andai itu aku",
		// "semoga aku berikutnya", "I wish I had the guts", "pengen punya
		// keberanian kayak dia", "kenapa bukan aku", "why not me", "itu yang
		// aku mau", "that's what I want".
		`|\b(?:i\s+)?(?:just\s+|really\s+)?wish\s+(?:that|it)\s+(?:was|were|had\s+been|could\s+(?:have\s+)?been)\s+me\b` +
		`|\bi\s+(?:just\s+|really\s+)?wish\s+i\s+(?:was|were|could\s+be)\s+(?:as\s+|so\s+)?(?:brave|strong|lucky|free|fearless)\s+(?:like|as)\s+(?:her|him|them)\b` +
		`|\blucky\s+(?:her|him|them|bastard|girl|guy)\b|\b(?:she|he|they)(?:'s|'re|\s+is|\s+was|\s+are|\s+were)\s+(?:so\s+)?lucky\b` +
		`|\b(?:andai|andaikan|seandainya|coba|kalo\s+aja|kalau\s+saja|kalau\s+aja)\s+(?:itu|yang\s+itu|yg\s+itu|dia\s+itu)\s+(?:aku|gue|gw|gua|saya|aq)\b` +
		`|\b(?:semoga|moga|mudah-mudahan)\s+(?:aku|gue|gw|gua|saya|aq)\s+(?:yang\s+|yg\s+)?(?:berikutnya|selanjutnya|next|nyusul)\b` +
		`|\bi\s+(?:just\s+|really\s+)?wish\s+i\s+(?:had|have|could\s+find|could\s+have)\s+(?:the\s+)?(?:guts|courage|balls|nerve|strength|bravery)\b` +
		`|\b(?:pengen|pgn|pingin|pengin|kepengen|ingin|mau|mw)\s+(?:punya\s+)?(?:keberanian|nyali|berani)\s+(?:kayak|kaya|kek|kyk|seperti|kaya\s+gitu|kayak\s+gitu|gitu|begitu)\b` +
		`|\b(?:kenapa|knp|kok)\s+(?:bukan|ga|gak|nggak|enggak)\s+(?:aku|gue|gw|gua|saya|aq)\b|\bwhy\s+(?:not|wasn'?t\s+it|couldn'?t\s+it\s+(?:have\s+)?been|isn'?t\s+it)\s+me\b` +
		`|\bitu\s+(?:yang|yg)\s+(?:aku|gue|gw|gua|saya|aq)\s+(?:mau|pengen|pgn|ingin|cari|butuh)(?:in)?\b|\bthat'?s\s+(?:what|all)\s+i\s+(?:want|wanna|need|wanted|needed)\b` +
		`|^(?:same|relate|relatable|sama|me|aku\s+banget|gue\s+banget)$`)

	// adoptionTrailRe: short adoption forms that only trailing words may
	// follow (see adopts): "gw banget", "kayak aku", "metoo", "me 2", "big
	// mood", "felt", "this is me", "i feel that", "I'm next", "aku
	// selanjutnya". Before any other word they are ordinary ("this is me at
	// the beach", "I'm next to her").
	adoptionTrailRe = mustGuarded(`` +
		`\b(?:aku|gue|gw|gua|saya|aq|w)\s+(?:banget|bgt|bngt)\b` +
		`|^(?:kayak|kaya|kek|kyk|seperti|persis\s+kayak)\s+(?:aku|gue|gw|gua|saya|aq)\b` +
		`|\bmetoo\b|\bme\s*2\b` +
		`|^(?:big\s+|such\s+a\s+|huge\s+|a\s+)?mood\b|^felt\b(?:\s+(?:that|this|it))?` +
		`|\bthis\s+(?:is|was)\s+(?:so\s+|literally\s+|exactly\s+|just\s+)?me\b` +
		`|\bi\s+(?:(?:really|totally|so|kinda|lowkey|honestly|deeply)\s+)*feel\s+(?:that|this|it|u|you|her|him|them)\b` +
		`|\b(?:i'?m|i\s+am|im)\s+(?:the\s+)?next\b|\b(?:aku|gue|gw|gua|saya|aq)\s+(?:yang\s+|yg\s+)?(?:berikutnya|selanjutnya)\b` +
		// Adoption idioms (review finding): "i know the feeling", "i know
		// how she feels", "been there", "same boat", "join the club",
		// "ditto", "it me", "I want that", "aku tau rasanya", "senasib",
		// all ordinary before a content word ("i know the feeling of
		// losing someone", "been there last summer", "I want that dress").
		`|\bi\s+(?:(?:really|totally|so|kinda|honestly|definitely)\s+)*know\s+(?:the|that|this)\s+feeling\b` +
		`|\bi\s+(?:(?:really|totally|so|kinda|honestly|definitely)\s+)*know\s+(?:how|what)\s+(?:she|he|they|you|it|that)\s+(?:feels?|means?|feels\s+like|is\s+like)\b` +
		`|\b(?:i'?ve\s+|ive\s+|i\s+have\s+)?been\s+there(?:\s+(?:before|too|done\s+that))?\b` +
		`|\b(?:(?:we'?re|we\s+are|i'?m|im|i\s+am)\s+)?(?:in\s+)?(?:the\s+)?same\s+boat\b|\bjoin\s+the\s+club\b|\bditto\b|^it'?s?\s+me\b` +
		`|\bi\s+(?:(?:really|just|also|kinda|lowkey|honestly|so)\s+)*(?:want|wanted|need)\s+(?:that|it|this)\b` +
		`|\b(?:aku|gue|gw|gua|saya|aq)\s+(?:juga\s+|jg\s+)?(?:tau|tahu|ngerti|paham)\s+(?:banget\s+|bgt\s+)?(?:rasanya|perasaannya|perasaan\s+itu|gimana\s+rasanya)\b` +
		`|\b(?:(?:aku|gue|gw|gua|saya|kita|kami)\s+)?(?:juga\s+)?senasib(?:\s+(?:sama|dengan|dgn|ama)\s+(?:dia|kamu|mereka|lo|lu))?\b`)

	// adoptionTrailAlts are the branches of adoptionTrailRe, each with its
	// own prefilter, so that a segment is only searched for the branches
	// whose literals it contains (one alternation of all the forms
	// dominated the cost of long texts of attributed reports).
	adoptionTrailAlts = splitAlternation(adoptionTrailRe.re.String())

	// adoptionAlts are the branches of adoptionRe, each with its own
	// prefilter (see adoptionTrailAlts): one search of the whole alternation
	// dominated long texts of attributed compounds (review finding).
	adoptionAlts = splitAlternation(adoptionRe.re.String())

	// commaAdoptionRe: adoption forms with a comma inside ("i do, too",
	// "me, too"), matched on the whole clause (see shape.adopted).
	commaAdoptionRe = regexp.MustCompile(`\bi\s+(?:(?:kinda|really|totally|honestly|also|lowkey|sometimes|literally|actually|so)\s+)*(?:do|did|am|have|feel|want|was)\s*,\s*(?:too|as\s+well|also)\b|\bme\s*,\s*(?:too|as\s+well)\b`)

	// adoptionCoreRe: short adoption forms that are also ordinary phrases
	// ("aku juga", "so am I", an elliptical "aku pengen"); see adopts.
	adoptionCoreRe = mustGuarded(`` +
		`\b(?:aku|saya|gue|gua|gw|ane|ku|aq|ak|w|sy|gwe)\s*(?:(?:sih|tuh|mah|sendiri|pribadi)\s+)?(?:juga|jg|jga|pun)\b` +
		`|\b(?:aku|gue|gw|gua|saya|aq|ak|sy|kita|kami)\s+(?:juga\s+)?sama\b|^sama\b` +
		`|\bso\s+(?:am|did|have|would)\s+i\b|\bme\s+to\b|\b(?:i'?m|i\s+am)\s+the\s+same\b` +
		`|^(?:(?:omg|ugh|lol|honestly|tbh|ngl|literally|fr|so|lmao|damn|oh|yeah|yes|ya|bruh|same|lowkey|highkey|kinda|also|big)\s+)*same\b` +
		`|\b(?:pengen|pgn|pingin|pengin|kepengen|kepingin|ingin|want\s+to|wanna)\b`)
)
