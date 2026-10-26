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
			if masked != "" && adoptionRe.MatchString(masked) {
				c.adoption = true
				return true
			}
		}
	}
	return false
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
	for i, seg := range cl.segments {
		if denialRe.MatchString(t[seg.start:seg.end]) {
			last := i == len(cl.segments)-1
			cl.denials = append(cl.denials, denialSpan{span: seg, hedged: last && cl.contrastNext})
		}
	}

	quoted := c.attributedQuotes(cl)
	cl.scopes = append(cl.scopes, quoted...)
	cl.hidden = append(cl.hidden, quoted...)

	// A frame starts at the beginning of a comma segment and may run past a
	// comma ("in the book, the character ...", "kata dia, ..."). Its reported
	// content is the rest of the segment where it ends (or the next segment
	// when it ends exactly at one), plus each following segment that opens
	// with a third-person pronoun, up to the next frame. Within each part the
	// content ends at a breaker that starts the speaker's own reaction
	// ("Nyokap bilang harus nikah rasanya pengen mati"). Each segment belongs
	// to at most a few frames, so the work is linear in the clause length.
	segs := cl.segments
	frameEnd := make([]int, len(segs)) // 0: the segment opens no frame
	for k, seg := range segs {
		if end, ok := matchFrame(t, seg.start, cl.end); ok {
			frameEnd[k] = end
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
			// segment or a first-person reply ("aku jawab ..."): an unquoted
			// quotation has no visible end, so what follows takes the safer
			// path.
			sc := span{fEnd, segs[j].end}
			if loc := ownFrameRe.FindStringIndex(maskSpans(t, sc, quoted)); loc != nil {
				sc.end = sc.start + loc[0]
			}
			if sc = trimSpan(t, sc); sc.start < sc.end {
				cl.scopes = append(cl.scopes, sc)
				cl.hidden = append(cl.hidden, sc)
			}
			continue
		}
		parts := []span{cutAtBreaker(t, span{fEnd, segs[j].end}, quoted)}
		for n := j + 1; n < len(segs) && frameEnd[n] == 0 && thirdPronounRe.MatchString(t[segs[n].start:segs[n].end]); n++ {
			parts = append(parts, cutAtBreaker(t, segs[n], quoted))
		}
		void, shared := false, false
		for _, p := range parts {
			masked := maskSpans(t, p, quoted)
			if firstPerson(masked) {
				void = true // the speaker is part of the reported content
				break
			}
			shared = shared || sharedRe.MatchString(masked) // "dia mau mati juga"
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

// cutAtBreaker shortens reported content at the first breaker word that is
// not attached to a third-person pronoun: "bikin", "rasanya", "sekarang",
// "terus", "jadi" and the like start the speaker's own reaction in
// subjectless Indonesian ("Pacarku bilang putus rasanya pengen mati"), while
// "dia terus ..." and "terus dia ..." keep the third-party subject. Breakers
// inside quotations are ignored.
func cutAtBreaker(t string, sp span, quoted []span) span {
	masked := maskSpans(t, sp, quoted)
	for _, loc := range breakerRe.FindAllStringIndex(masked, -1) {
		before, _ := tokenBefore(masked, loc[0])
		if thirdPronouns[before] || thirdPronouns[nextWord(masked, loc[1])] {
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
	if firstPersonRe.MatchString(s) {
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
	off, ok := frameStart(t[start:limit])
	if !ok {
		return 0, false
	}
	from := start + off
	end := min(limit, from+frameWindow)
	loc := frameFind(t[from:end])
	if loc != nil && from+loc[1] == end && end < limit {
		loc = frameFind(t[from:limit]) // the match reached the window edge
	}
	if loc == nil {
		return 0, false
	}
	return from + loc[1], true
}

// frameFind is frameRe.FindStringIndex restricted to the branches that can
// begin with the first word of s. A frame is tried on every comma segment,
// and trying all eight branches there dominated the cost of long texts.
func frameFind(s string) []int {
	w := firstWord(s)
	for i, g := range frameRe.alts {
		if frameOpens[i](w) {
			if loc := g.FindStringIndex(s); loc != nil {
				return loc
			}
		}
	}
	return nil
}

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
}

var (
	idSubjectStarts = setOf("dia ia mereka beliau seseorang orang")
	enSubjectStarts = setOf("he she they someone somebody this the my a our his her their")
)

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
// boundary (see oneSentence): such a pair is taken to start at a stray quote
// mark, and the closing mark is tried as a new opening one. Otherwise one
// stray mark could turn the speaker's own sentences into a quotation.
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
		case oneSentence(s[open+1 : i]):
			out = append(out, span{open + 1, i})
			open = -1
		default:
			open = i
		}
	}
	doubles := len(out)
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
				whole := span{open, i + 1}
				inside := false
				for _, d := range out[:doubles] {
					inside = inside || d.overlaps(whole)
				}
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
// frame start word, a relative or role noun, or a bare reporting verb.
func opensReport(w string) bool {
	return frameStarts[w] || hasFrameStem(w) || reportVerbs[w]
}

var reportVerbs = setOf("katanya tulisnya ujarnya ucapnya kata tulis ujar ucap")

// opensOwnReport reports whether w can open the speaker's own reporting
// frame.
func opensOwnReport(w string) bool { return ownSubjects[w] }

var ownSubjects = setOf("aku saya gue gua gw ane ku i i'm im")

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
	frameRe = newAltAnchored(`^(?:`, `)`+complement+`\s*[:,]?`,
		`(?:dalam|di)\s+`+idMedia+`(?:\s+(?:itu|ini|tersebut|tadi|kemarin|kemaren))?(?:\s+yang\s+(?:aku|saya|gue|gw|kita|kami)\s+(?:tonton|baca|lihat|liat|dengar|denger))?\s*,?\s*`+idMediaWho+`\b`,
		`in\s+(?:the|that|a|an|this|my|our|today'?s|yesterday'?s)\s+`+enMedia+`(?:\s+(?:i|we)\s+(?:just\s+)?(?:watched|saw|read|heard|listened\s+to))?\s*,?\s*`+enMediaWho+`\b`,
		`(?:tokoh|karakter|pemeran)(?:nya)?(?:\s+utama(?:nya)?)?\b`,
		`the\s+(?:main\s+|lead\s+)?(?:character|protagonist|heroine|hero|villain)\b`,
		`(?:katanya|(?:kata|menurut)\s+`+kataSubject+`)\b`,
		`according\s+to\s+(?:him|her|them|`+enSubject+`)\b`,
		idSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb+anyReceiver+`\b`,
		enSubject+`\s+(?:`+anyAdverb+`\s+)*`+anyVerb+anyReceiver+`\b`,
	)

	// reportingTailRe recognises a reporting frame that ends just before a
	// quotation ("temanku nulis", "my friend texted me this:").
	reportingTailRe = newTailFrame(opensReport,
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

	// thirdPronounRe: a segment that continues the reported content.
	thirdPronounRe = regexp.MustCompile(`^(?:dia|ia|mereka|beliau|he|she|they|he's|she's|they're|hes|shes|theyre|his|her|their)\b`)

	// firstPersonRe: first-person pronouns and common -ku forms (see
	// firstPerson, which also accepts any -ku word). Inside an attributed
	// scope (outside quotations) any of them makes the attribution void: the
	// speaker is part of it ("dalam film itu tokoh bunuh diri aku mau bunuh
	// diri").
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
