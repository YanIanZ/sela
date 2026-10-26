package safety

import (
	"regexp"
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

	// Filled in lazily by shape.clause.
	analysed bool
	denials  []denialSpan
	scopes   []span // text attributed to a third party
	hidden   []span // frames and attributed text, ignored by adoption checks
	shared   bool   // "juga"/"too" inside attributed indirect speech
}

// denialSpan is a segment recognised as an explicit whole-clause denial.
type denialSpan struct {
	span
	// hedged is true when the next clause opens with a contrastive
	// conjunction ("aku tidak mau bunuh diri, tapi ...").
	hedged bool
}

// shape is the clause structure of one normalised text. Clause analysis is
// lazy: only clauses that contain a rule match are analysed, and adoption is
// only computed when a match is attributed to a third party.
type shape struct {
	text    string
	quotes  []span // quoted content, without the quote marks
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
		next := i+1 < len(c.clauses) && c.clauses[i+1].afterContrast
		c.analyseClause(cl, next)
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
			if masked != "" && adoptionRe.MatchString(masked) {
				c.adoption = true
				return true
			}
		}
	}
	return false
}

func (c *shape) analyseClause(cl *clause, contrastFollows bool) {
	t := c.text
	for i, seg := range cl.segments {
		if denialRe.MatchString(t[seg.start:seg.end]) {
			last := i == len(cl.segments)-1
			cl.denials = append(cl.denials, denialSpan{span: seg, hedged: last && contrastFollows})
		}
	}

	// Quotations attributed to a third party by a reporting verb before them
	// ("temanku bilang \"...\"") or after them ("\"...\", she said").
	var quoted []span
	for _, q := range c.quotes {
		if !cl.contains(q) {
			continue
		}
		pre := tailWindow(t, cl.start, max(cl.start, q.start-1))
		post := t[min(cl.end, q.end+1):min(cl.end, q.end+1+frameWindow)]
		if reportingTailRe.MatchString(pre) || reportingHeadRe.MatchString(post) {
			quoted = append(quoted, q)
		}
	}
	cl.scopes = append(cl.scopes, quoted...)
	cl.hidden = append(cl.hidden, quoted...)

	// A frame starts at the beginning of a comma segment and may run past a
	// comma ("in the book, the character ...", "kata dia, ..."). It
	// attributes the rest of the segment where it ends (or the next segment
	// when it ends exactly at one) and any following segments that open with
	// a third-person pronoun. Direct speech after a colon ("temanku bilang:
	// aku mau mati") is treated like a quotation up to the end of the clause.
	segs := cl.segments
	for _, seg := range segs {
		fEnd, ok := matchFrame(t, seg.start, cl.end)
		if !ok {
			continue
		}
		cl.hidden = append(cl.hidden, span{seg.start, fEnd})
		k := 0
		for k < len(segs) && segs[k].end <= fEnd {
			k++
		}
		if k == len(segs) {
			continue // nothing follows the frame
		}
		if t[fEnd-1] == ':' {
			sc := span{fEnd, cl.end}
			cl.scopes = append(cl.scopes, sc)
			cl.hidden = append(cl.hidden, sc)
			continue
		}
		sc := span{fEnd, segs[k].end}
		for j := k + 1; j < len(segs) && thirdPronounRe.MatchString(t[segs[j].start:segs[j].end]); j++ {
			sc.end = segs[j].end
		}
		masked := maskSpans(t, sc, quoted)
		if firstPersonRe.MatchString(masked) {
			continue // the speaker is part of the reported content
		}
		if sharedRe.MatchString(masked) {
			cl.shared = true // "dia mau mati juga", "she wants to die too"
		}
		cl.scopes = append(cl.scopes, sc)
		cl.hidden = append(cl.hidden, sc)
	}
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
	off, ok := frameStart(t[start:limit])
	if !ok {
		return 0, false
	}
	from := start + off
	end := min(limit, from+frameWindow)
	loc := frameRe.FindStringIndex(t[from:end])
	if loc != nil && from+loc[1] == end && end < limit {
		loc = frameRe.FindStringIndex(t[from:limit]) // the match reached the window edge
	}
	if loc == nil {
		return 0, false
	}
	return from + loc[1], true
}

// frameStart skips up to three lead-in words and reports where the frame
// proper would begin, if the next word can begin one. It keeps the lead-in
// out of the expressions and frameRe off segments that cannot match.
func frameStart(seg string) (int, bool) {
	off := 0
	for n := 0; n <= 3; n++ {
		w := firstWord(seg[off:])
		if w == "" {
			return 0, false
		}
		if frameStarts[w] || hasFrameStem(w) {
			return off, true
		}
		if !leadInWords[w] {
			return 0, false
		}
		off += len(w)
		for off < len(seg) && strings.IndexByte(" ,\t\n", seg[off]) >= 0 {
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
	"he she they someone somebody this the my a our his her their")

var leadInWords = setOf("jadi terus trus lalu kemarin kemaren tadi barusan semalam btw eh oh hmm so well " +
	"yesterday today earlier recently anyway anyways and dan also guys")

func setOf(words string) map[string]bool {
	m := make(map[string]bool)
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}

// maskSpans returns text[within] with the parts covered by hide blanked out.
func maskSpans(text string, within span, hide []span) string {
	b := []byte(text[within.start:within.end])
	for _, h := range hide {
		for i := max(h.start, within.start); i < min(h.end, within.end); i++ {
			b[i-within.start] = ' '
		}
	}
	return string(b)
}

// findQuotes returns the content spans of balanced quotations, sorted by
// position. Double quotes pair in order; single quotes count only when they
// open after a space or punctuation and close before one, so apostrophes
// ("don't") never do.
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
		if open < 0 {
			open = i
		} else {
			out = append(out, span{open + 1, i})
			open = -1
		}
	}
	open = -1
	for i := 0; i < len(s); i++ {
		if s[i] != '\'' {
			continue
		}
		prevOK := i == 0 || strings.IndexByte(" \n(:,\"", s[i-1]) >= 0
		nextOK := i+1 == len(s) || strings.IndexByte(" \n.,!?;:)\"", s[i+1]) >= 0
		switch {
		case open < 0 && prevOK && i+1 < len(s) && isLetter(s[i+1]):
			open = i
		case open >= 0 && nextOK && i > 0 && s[i-1] != ' ':
			whole := span{open, i + 1}
			inside := false
			for _, d := range out {
				inside = inside || d.overlaps(whole)
			}
			if !inside {
				out = append(out, span{open + 1, i})
			}
			open = -1
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].start < out[b].start })
	return out
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

// altGuarded is an alternation split into one guarded expression per
// branch. Small expressions let RE2 use its fast backtracker and each branch
// has its own, sharper prefilter. FindStringIndex returns the first branch
// (in order) that matches.
type altGuarded struct{ alts []guarded }

func newAltGuarded(prefix, suffix string, branches ...string) altGuarded {
	var a altGuarded
	for _, b := range branches {
		a.alts = append(a.alts, mustGuarded(prefix+b+suffix))
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
	idMedia     = `(?:film|filem|movie|berita|pelajaran|artikel|buku|novel|cerita|cerpen|komik|series|serial|sinetron|drama|drakor|lagu|video|game|anime|manga|podcast|kelas|materi|seminar|webinar|dokumenter|thread|utas|postingan|konten|episode|kuliah)`
	idMediaWho  = `(?:tokoh|karakter|pemeran|pemain|penulis|penyanyi|korban|pelaku|artis|aktor|aktris|seseorang|seorang|orang|ada|dia|ia|mereka|remaja|mahasiswa|siswa|pelajar|anak|pria|wanita|lelaki|laki|perempuan|cowok|cewek|ibu|ayah|guru|dosen|pembicara|narasumber|penyiar|youtuber|influencer|selebriti|seleb|idol)\w*`
	enMedia     = `(?:movie|film|news|article|book|show|story|class|series|novel|song|video|game|episode|documentary|podcast|lecture|post|thread|comic|anime|manga|play|lesson|report|chapter|scene)`
	enMediaWho  = `(?:(?:the|a|an|this|that|one|some)\s+)?(?:(?:main|lead)\s+)?(?:characters?|he|she|they|someone|somebody|man|woman|girl|boy|guy|kid|teen|teenager|student|person|people|hero|heroine|protagonist|author|singer|victim|narrator|teacher|speaker|actor|actress|celebrity|influencer|host|youtuber|reporter|lecturer)`
)

var (
	// denialRe is the v1 contextualDiscussion denial, applied to one comma
	// segment. Sela accepts the colloquial pronouns and negators, the same
	// verbs plus "pengen"/"berniat", the self-harm objects, and trailing
	// discourse particles ("kok", "sih"). Any other content makes it a normal
	// segment ("aku tidak mau bunuh diri lagi" is not a denial).
	denialRe = mustGuarded(`^(?:` +
		`(?:aku|saya|gue|gua|gw|ku)\s+(?:tidak|ga|gak|gk|nggak|ngga|enggak|engga|tak|ndak)\s+(?:mau|ingin|akan|pengen|pingin|berniat|berencana|kepikiran\s+(?:buat|untuk))\s+` +
		`(?:bunuh\s*diri|bundir|mati|mengakhiri\s+hidup(?:ku)?|menyakiti\s+diri(?:ku|\s+sendiri)?|melukai\s+diri(?:ku|\s+sendiri)?)` +
		`(?:\s+(?:kok|sih|ya|kan|lho|loh|deh))?` +
		`|(?:i|i'm|im|i\s*am)\s+(?:do\s*not|don't|dont|am\s*not|not|won't|wont|will\s*not|would\s*never|will\s*never|never|would\s*not|wouldn't|wouldnt)\s+(?:ever\s+)?(?:going\s*to\s*|gonna\s*)?(?:want(?:ing)?\s*to\s*|wanna\s*|try(?:ing)?\s*to\s*|plan(?:ning)?\s*to\s*)?` +
		`(?:kill\s*myself|die|commit\s*suicide|hurt\s*myself|harm\s*myself|end\s*my\s*(?:own\s*)?life|take\s*my\s*(?:own\s*)?life)` +
		`|(?:i'm|im|i\s*am)\s+not\s+suicidal` +
		`)\s*[.!]?$`)

	// frameRe recognises a third-party frame at the start of a segment: the
	// v1 media frames ("dalam film itu tokoh ...", "in the movie, the
	// character ...", widened to more media and subjects) and reporting frames
	// ("temanku bilang", "kata dia", "my friend said", "according to her").
	frameRe = newAltGuarded(`^(?:`, `)`+complement+`\s*[:,]?`,
		`(?:dalam|di)\s+`+idMedia+`(?:\s+(?:itu|ini|tersebut|tadi|kemarin|kemaren))?(?:\s+yang\s+(?:aku|saya|gue|gw|kita|kami)\s+(?:tonton|baca|lihat|liat|dengar|denger))?\s*,?\s*`+idMediaWho+`\b`,
		`in\s+(?:the|that|a|an|this|my|our|today'?s|yesterday'?s)\s+`+enMedia+`(?:\s+(?:i|we)\s+(?:just\s+)?(?:watched|saw|read|heard|listened\s+to))?\s*,?\s*`+enMediaWho+`\b`,
		`(?:tokoh|karakter|pemeran)(?:nya)?(?:\s+utama(?:nya)?)?\b`,
		`the\s+(?:main\s+|lead\s+)?(?:character|protagonist|heroine|hero|villain)\b`,
		`(?:kata|katanya|menurut)(?:\s+`+idSubject+`)?\b`,
		`according\s+to\s+(?:him|her|them|`+enSubject+`)\b`,
		idSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb+anyReceiver+`\b`,
		enSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb+anyReceiver+`\b`,
	)

	// reportingTailRe recognises a reporting frame that ends just before a
	// quotation ("temanku nulis", "my friend texted me this:").
	reportingTailRe = newAltGuarded(`(?:^|[\s,(])(?:`, `)`+complement+`\s*[:,]?\s*$`,
		`(?:kata|katanya|tulis|tulisnya|ujar|ujarnya|ucap|ucapnya)(?:\s+`+idSubject+`)?`,
		idSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb+anyReceiver,
		enSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb+anyReceiver,
	)

	// reportingHeadRe recognises an attribution right after a quotation
	// ("\"...\", she said", "\"...\" kata temanku").
	reportingHeadRe = newAltGuarded(`^\s*,?\s*(?:`, `)\b`,
		`(?:kata|katanya|tulis|tulisnya|ujar|ujarnya|ucap|ucapnya)(?:\s+`+idSubject+`)?`,
		idSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb,
		enSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb,
	)

	// thirdPronounRe: a segment that continues the reported content.
	thirdPronounRe = regexp.MustCompile(`^(?:dia|ia|mereka|beliau|he|she|they|he's|she's|they're|hes|shes|theyre|his|her|their)\b`)

	// firstPersonRe: first-person markers. Inside an attributed scope (outside
	// quotations) any of them makes the attribution void: the speaker is part
	// of it ("dalam film itu tokoh bunuh diri aku mau bunuh diri").
	firstPersonRe = regexp.MustCompile(`\b(?:aku|saya|gue|gua|gw|ane|daku|ku|diriku|hidupku|nyawaku|jiwaku|tanganku|lenganku|nadiku|leherku|pahaku|tubuhku|badanku|kita|kami|i|i'm|im|i've|ive|i'd|i'll|me|my|myself|mine|we|us|our|ours|ourselves)\b`)

	// sharedRe: "juga"/"too" inside reported content is read as the speaker
	// sharing it ("tokohnya bunuh diri, jadi pengen mati juga").
	sharedRe = regexp.MustCompile(`\b(?:juga|jg|too|as\s+well)\b`)

	// adoptionRe: the speaker takes up the reported risk ("aku juga", "me too").
	adoptionRe = mustGuarded(`` +
		`\b(?:aku|saya|gue|gua|gw|ane|ku)\s+(?:juga|jg|jga)\b(?:\s*$|\s+(?:mau|mw|pengen|pgn|pingin|ingin|kepengen|kepikiran|mikir|merasa|ngerasa|rasa|gitu|gtu|begitu|sama|sih|kok|lho|loh|deh|nih|banget|bgt|kadang|sering|pernah|lagi|udah|sudah|ya|kayaknya)\b)` +
		`|\bsama\s+(?:aku|gue|saya|gw)\s+(?:juga|jg)\b` +
		`|\b(?:aku|gue|saya|gw)\s+(?:juga\s+)?(?:ngerasa|merasa|ngerasain|merasakan|ngalamin|mengalami)\s+(?:hal\s+yang\s+sama|yang\s+sama|gitu|begitu|hal\s+serupa|sama)\b` +
		`|\bme\s+(?:too|as\s+well|neither|also)\b` +
		`|\bso\s+do\s+i\b` +
		`|\bsame\s+(?:here|for\s+me|tbh|honestly)\b` +
		`|\bi\s+(?:do|did|feel|felt|want|wanted|think|am|have)\s+(?:(?:the\s+)?same\s+|that\s+|it\s+|so\s+)?(?:too|as\s+well)\b` +
		`|\bi\s+(?:feel|felt)\s+(?:the\s+same|that\s+way|like\s+that)\b` +
		`|\bi\s+(?:can\s+|totally\s+|really\s+)?relate\b|\brelat(?:able|e\s+(?:banget|bgt|sih|deh|parah))\b` +
		`|\b(?:i\s+(?:said|say|replied|answered|was\s+like)|lol|honestly|tbh|ngl|literally|fr)\s+"?\s*(?:same|me\s+too)\b` +
		`|\b(?:aku|gue|saya|gw)\s+(?:bilang|jawab|balas)\s+"?\s*(?:sama|aku\s+juga|gue\s+juga)\b` +
		`|^(?:same|relate|relatable|sama|me|aku\s+banget|gue\s+banget)$`)
)
