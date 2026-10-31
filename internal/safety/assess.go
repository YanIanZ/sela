package safety

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// state is how a rule ended up for one text, ordered by strength.
type state uint8

const (
	stateNone       state = iota
	stateDenied           // matched only inside explicit denials
	stateAttributed       // matched only in third-party text (elevated)
	stateHedged           // matched in a denial followed by "tapi"/"but" (elevated)
	stateCounted          // matched in the speaker's own words
)

// Assess classifies text and returns the routing decision. It is
// deterministic, free of I/O and safe for concurrent use.
// The routing policy is described in the package documentation.
func Assess(text string) Assessment { return withDanger(assess(text, true), text) }

// assess matches every variant of text (see variants) and folds the
// outcomes. With skip, a derived variant only re-matches its context units
// that differ from the canonical form's (see contextUnits): every other unit
// has exactly the same matches and dispositions there, unless the whole-text
// denial differs, in which case it is matched in full. TestSkipEquivalence
// checks this against assess(text, false).
//
// The context-marker gate (see markersApply) is decided once for the text:
// it is open only when it is open on every variant, so a signal that one
// reading keeps and another drops (an emoji, an emphasis mark or a quotation
// mark that the lexical variant reads as a space, while only the lexical
// variant spells "cut my self" as "cut myself") keeps every match.
func assess(text string, skip bool) Assessment {
	vs := variants(text)
	acc := &accumulator{states: make([]state, len(catalogue)), attributed: make([]bool, len(catalogue)), gate: plainText(text) && gateOpen(vs)}
	var known map[string]bool
	for i, v := range vs {
		if v == "" {
			continue
		}
		sh := analyse(v)
		sentences := sentenceSpans(v)
		if skip && len(vs) > 1 {
			units := contextUnits(sh)
			switch {
			case i == 0:
				known = make(map[string]bool, len(units))
				for _, u := range units {
					known[u.key] = true
				}
			case sh.wholeDenial == acc.shapes[0].wholeDenial:
				sentences = changedSentences(sentences, units, known)
			}
		}
		acc.evaluate(sh, sentences)
		acc.shapes = append(acc.shapes, sh)
	}
	return acc.assemble()
}

// accumulator folds the matches of every variant of one text.
type accumulator struct {
	// states holds, per rule, the strongest disposition other than
	// stateAttributed; attributed records attributed matches separately, so
	// that adoption anywhere in any variant can promote them at the end.
	states     []state
	attributed []bool
	denial     bool
	// adopted is set when a third-person match is shared in place ("she
	// wants to die too"); shape adoption is checked in assemble.
	adopted bool
	// gate is the context-marker gate of the text (see gateOpen).
	gate   bool
	shapes []*shape
}

// plainText reports whether the original text, before normalisation, has
// only letters, digits, spaces and plain punctuation (. , ! ? ; : - and an
// apostrophe). An emoji, an emoticon's bracket or symbol, a quotation mark
// of any script or an invisible format character that normalisation would
// remove closes the context-marker gate (review finding), so the gate
// reads the original text as well as every variant.
func plainText(t string) bool {
	if len(t) > maxMarkerText {
		return false
	}
	for _, r := range t {
		switch {
		case r < utf8.RuneSelf:
			if !isLetter(byte(r|0x20)) && !isDigit(byte(r)) && strings.IndexByte(" \t\n\r.,!?;:-'", byte(r)) < 0 {
				return false
			}
		case !unicode.IsLetter(r) && !unicode.IsSpace(r) && r != '\u2019':
			return false
		}
	}
	return true
}

// gateOpen reports whether context markers may discard matches: the gate
// (see markersApply) must be open on every variant of the text.
func gateOpen(vs []string) bool {
	for _, v := range vs {
		if v != "" && !markersApply(v) {
			return false
		}
	}
	return true
}

// hit is one accepted match of catalogue[rule]. third is set when the
// pattern's attribution check names someone else's act ("bom bunuh diri"),
// marked when its context marker suggests a benign reading ("I cut myself
// shaving"; see markersApply).
type hit struct {
	rule int
	span
	third, marked bool
}

// evaluate matches every rule against the given sentences of one normalised
// variant and records how each match counts.
func (acc *accumulator) evaluate(c *shape, sentences []span) {
	hits := matchAll(c, sentences)
	for _, h := range hits {
		if h.marked && acc.gate {
			continue
		}
		r := &catalogue[h.rule]
		d := c.dispose(r, h.span, h.third)
		if d == stateDenied || d == stateHedged {
			acc.denial = true
		}
		if d == stateAttributed {
			acc.attributed[h.rule] = true
			if r.thirdPerson && sharedWish(c.text, h.span) && c.sharedAfter(h.span) {
				acc.adopted = true
			}
			continue
		}
		if d > acc.states[h.rule] {
			acc.states[h.rule] = d
		}
	}
}

// matchAll returns every accepted match in the given sentences of c.text,
// plus the anchored patterns' matches on the whole text and on each clause.
func matchAll(c *shape, sentences []span) []hit {
	text := c.text
	var hits []hit
	idx := &atomIndex{text: text, sentences: sentences}
	var cand []int
	for i := range catalogue {
		for _, p := range catalogue[i].patterns {
			if p.anchored {
				if !p.mayMatch(text) {
					continue
				}
				if p.re.MatchString(text) {
					hits = append(hits, hit{rule: i, span: span{0, len(text)}})
				}
				for _, cl := range c.clauses {
					if p.re.MatchString(text[cl.start:cl.end]) {
						hits = append(hits, hit{rule: i, span: cl.span})
					}
				}
				continue
			}
			cand = p.candidates(idx, cand)
			for _, si := range cand {
				sent := sentences[si]
				chunk := text[sent.start:sent.end]
				if p.filter != nil && p.filter.op != 'a' {
					if len(chunk) >= windowMin {
						if !idx.windowsOf(si, chunk).filterIn(p.filter, 0, len(chunk)) {
							continue
						}
					} else if !p.filter.match(chunk) {
						continue
					}
				}
				ctx := chunk
				if p.wide {
					ctx = text[sent.start:min(len(text), sent.end+restWindow)]
				}
				add := func(start, end int, third, marked bool) bool {
					hits = append(hits, hit{i, span{sent.start + start, sent.start + end}, third, marked})
					return p.maximal
				}
				if !p.maximal && len(chunk) >= windowMin {
					if ws := idx.windowsOf(si, chunk).windows(p); ws != nil {
						p.eachIn(chunk, ctx, ws, add)
						continue
					}
				}
				p.each(chunk, ctx, add)
			}
		}
	}
	return hits
}

// assemble turns the folded states into an Assessment. Adoption anywhere in
// any variant ("aku juga", "me too", or "too" after a third-person match)
// promotes every attributed match, so that "my friend said she wants to die.
// me tooooo" counts although only the doubled variant spells "me too". It is
// applied per rule after all variants, so a hedged denial of the same rule
// cannot mask it either. RuleIDs and Categories follow catalogue order;
// rules discarded by a denial are not listed.
func (acc *accumulator) assemble() Assessment {
	adopt := acc.adopted
	if !adopt && acc.pendingAttribution() {
		for _, sh := range acc.shapes {
			if sh.adopted() {
				adopt = true
				break
			}
		}
	}
	a := Assessment{Level: LevelNone, Denial: acc.denial, Ruleset: RulesetVersion}
	counted, attributed := false, false
	for i, st := range acc.states {
		if acc.attributed[i] {
			if adopt {
				st = stateCounted
			} else {
				st = max(st, stateAttributed)
			}
		}
		if st < stateAttributed {
			continue
		}
		r := &catalogue[i]
		a.RuleIDs = append(a.RuleIDs, r.id)
		if !slices.Contains(a.Categories, r.category) {
			a.Categories = append(a.Categories, r.category)
		}
		lvl := r.level
		switch st {
		case stateAttributed:
			lvl = LevelElevated
			attributed = true
		case stateHedged:
			lvl = LevelElevated
			counted = true
		default:
			counted = true
		}
		if levelRank[lvl] > levelRank[a.Level] {
			a.Level = lvl
		}
	}
	a.ThirdParty = attributed && !counted
	return a
}

// pendingAttribution reports whether adoption could still change the result:
// some rule is attributed without having counted in full elsewhere.
func (acc *accumulator) pendingAttribution() bool {
	for i, at := range acc.attributed {
		if at && acc.states[i] < stateCounted {
			return true
		}
	}
	return false
}

// dispose decides how one match of r counts: inside an explicit denial it is
// discarded (or hedged), third-person wording, a third-party compound
// (third) and attributed text make it third-party, and anything else is the
// speaker's own.
func (c *shape) dispose(r *rule, m span, third bool) state {
	if c.wholeDenial {
		return stateDenied
	}
	lo, hi := c.overlapping(m)
	denied := false
	for i := lo; i < hi; i++ {
		for _, d := range c.clause(i).denials {
			if d.contains(m) {
				if d.hedged {
					return stateHedged
				}
				denied = true
			}
		}
	}
	if denied {
		return stateDenied
	}
	if r.thirdPerson || third {
		return stateAttributed
	}
	for i := lo; i < hi; i++ {
		for _, s := range c.clause(i).scopes {
			if s.contains(m) {
				return stateAttributed
			}
		}
	}
	return stateCounted
}

// sharedAfter reports whether "juga"/"too" follows the match m in its comma
// segment ("honestly my friend said she wants to die too"): the speaker
// shares a third-person wish even when no frame was recognised around it.
// The last "juga"/"too" of each segment is found once per clause (see
// clause.lastShared), so many matches in one segment stay linear.
func (c *shape) sharedAfter(m span) bool {
	lo, hi := c.overlapping(m)
	for i := lo; i < hi; i++ {
		cl := &c.clauses[i]
		segs := cl.segments
		k := sort.Search(len(segs), func(n int) bool { return segs[n].end >= m.end })
		if k < len(segs) && segs[k].start <= m.end {
			if cl.lastShared == nil {
				cl.lastShared = make([]int, len(segs))
				for n, seg := range segs {
					cl.lastShared[n] = -1
					for _, loc := range sharedRe.FindAllStringIndex(c.text[seg.start:seg.end], -1) {
						cl.lastShared[n] = seg.start + loc[0]
					}
				}
			}
			return cl.lastShared[k] >= m.end
		}
	}
	return false
}

// sharedWish reports whether the third-person match m is a present or
// future wish or intent ("she wants to die", "he wants to kill himself",
// "she's going to hurt herself"), the only kind that "too" after it shares
// with the speaker. After a past act ("my uncle killed himself too", "the
// singer killed himself too") "too" adds another person, not the speaker.
func sharedWish(t string, m span) bool {
	s := t[m.start:m.end]
	if strings.HasPrefix(s, "wants") {
		return true
	}
	if !bareActs[firstWord(s)] {
		return false
	}
	prev, ps := tokenBefore(t, m.start)
	if prev == "to" {
		w, _ := tokenBefore(t, ps)
		return wishBeforeTo[w]
	}
	return wishModals[prev] || strings.HasSuffix(prev, "'ll")
}

var (
	bareActs     = setOf("kill end take hurt harm cut burn hang drown shoot starve")
	wishBeforeTo = setOf("want wants wanted wanting going planning plans planned trying tries tried ready about decided threatening threatened")
	wishModals   = setOf("wanna gonna will might could shall")
)

// unit is a context unit of a normalised text (see contextUnits).
type unit struct {
	span
	key string
}

// contextUnits splits c.text at sentence punctuation outside quotations.
// Every clause, quotation and frame lies inside one unit, and so does every
// match (matching is per sentence, and sentences only split units further).
// How a match counts depends only on its unit's text, the quotations in it,
// whether a contrastive conjunction follows it, a "just"/"cuma" clause that
// opens the next unit, the whole-text denial check and adoption. The key
// holds the first four; assess compares the fifth and folds adoption over
// all variants.
func contextUnits(c *shape) []unit {
	t := c.text
	var out []unit
	q, first := 0, 0 // q: quotation at or after i; first: first one of the unit
	start := 0
	var key []byte
	// next is the first byte at or after the last unit end that is not
	// punctuation or space (see skipPunct). Unit ends increase, so
	// it only moves forward: rescanning a run of punctuation from each of
	// its marks made "... ... ..." quadratic (review finding).
	next := 0
	emit := func(end int) {
		key = append(key[:0], t[start:end]...)
		key = append(key, 0)
		for ; first < len(c.quotes) && c.quotes[first].start < end; first++ {
			key = strconv.AppendInt(key, int64(c.quotes[first].start-start), 10)
			key = append(key, ':')
			key = strconv.AppendInt(key, int64(c.quotes[first].end-start), 10)
			key = append(key, ' ')
		}
		if next < end {
			next = skipPunct(t, end)
		}
		if conjunctionAt(t, next) {
			key = append(key, 0, 'c')
		}
		if next < len(t) && qualifierLead(t[next:min(len(t), next+checkWindow)]) {
			// A denial at the end of the unit is qualified by a "just"/"cuma"
			// or a retraction that opens the next one ("I'm not suicidal.
			// just kidding", "... . jk"; see analyseClause).
			key = append(key, 0, 'j')
			key = append(key, t[next:min(len(t), next+checkWindow)]...)
		}
		if mayHoldWide(t[start:end]) {
			// A wide pattern's check reads past the unit (see pattern.wide).
			key = append(key, 0, 'w')
			key = append(key, t[end:min(len(t), end+restWindow)]...)
		}
		if mayHoldDenial(t[start:end]) {
			// A denial's disposition reads the rest of the message past the
			// unit (see restAfter): a method, threat, later act or absence
			// that follows in a NEW sentence still takes the denial back,
			// so the key must change when that tail does. The key uses the
			// same restWindow as restAfter; a shorter key lets a derived
			// variant skip the sentence that would resolve the denial
			// (review finding P1-1).
			key = append(key, 0, 'd')
			key = append(key, t[end:min(len(t), end+restWindow)]...)
		}
		out = append(out, unit{span{start, end}, string(key)})
	}
	for i := 0; i < len(t); i++ {
		for q < len(c.quotes) && c.quotes[q].end <= i {
			q++
		}
		if q < len(c.quotes) && c.quotes[q].start <= i {
			continue // inside a quotation
		}
		switch t[i] {
		case '.', '!', '?':
			emit(i)
			start = i + 1
		}
	}
	emit(len(t))
	return out
}

// wideAtoms are the atoms of the wide patterns (see pattern.wide); nil
// entries stand for a wide pattern without atoms, which any unit may hold.
var wideAtoms = func() [][]string {
	var out [][]string
	for i := range catalogue {
		for _, p := range catalogue[i].patterns {
			if p.wide {
				out = append(out, p.atoms)
			}
		}
	}
	return out
}()

// mayHoldWide reports whether the unit text u may hold a match of a wide
// pattern.
func mayHoldWide(u string) bool {
	for _, atoms := range wideAtoms {
		if atoms == nil || containsAny(u, atoms) {
			return true
		}
	}
	return false
}

// denialObjects are the self-harm objects of the denial core (see
// denialCore); denialNegations are the negators that can stand in one. A
// unit holding both may hold a denial; a unit missing either cannot.
var denialObjects = []string{
	"bunuh diri", "bundir", "mengakhiri hidup", "menyakiti diri", "melukai diri",
	"kill myself", "commit suicide", "hurt myself", "harm myself",
	"end my life", "end my own life", "take my life", "take my own life", "suicidal",
}
var denialNegations = []string{
	"tidak", "ga ", "gak ", "gk ", "nggak", "ngga ", "enggak", "engga ", "tak ", "ndak ",
	"do not", "don't", "dont", "am not", " not ", "won't", "wont", "will not", "would never",
	"will never", "never", "would not", "wouldn't", "wouldnt",
}

// mayHoldDenial reports whether the unit text u may hold a denial (see
// analyseClause). Over-reporting only re-assesses the unit; under-reporting
// would break skip equivalence, so the check is deliberately conservative.
func mayHoldDenial(u string) bool {
	return containsAny(u, denialObjects) && containsAny(u, denialNegations)
}

// skipPunct returns the first index >= i of t that is not punctuation or
// space. A contrastive conjunction there (see conjunctionAt) follows
// position i with only punctuation and spaces in between, which is what
// marks the clause before it contrastNext in splitClauses.
func skipPunct(t string, i int) int {
	for i < len(t) && strings.IndexByte(" \t\n,.!?;", t[i]) >= 0 {
		i++
	}
	return i
}

// conjunctionAt reports whether a contrastive conjunction starts at t[i].
func conjunctionAt(t string, i int) bool {
	j := i
	for j < len(t) && isWordByte(t[j]) {
		j++
	}
	return j > i && isLetter(t[i]) && conjunctions[t[i:j]]
}

// changedSentences keeps the sentences that lie in a unit whose key is not
// in known.
func changedSentences(sentences []span, units []unit, known map[string]bool) []span {
	kept := sentences[:0]
	u := 0
	for _, sp := range sentences {
		for u < len(units) && units[u].end < sp.end {
			u++
		}
		if u < len(units) && !known[units[u].key] {
			kept = append(kept, sp)
		}
	}
	return kept
}

// atomIndex finds the sentences that contain an atom, caching the answer
// for atoms shared by several patterns, and the windows of the long
// sentences (see window.go).
type atomIndex struct {
	text      string
	sentences []span
	cache     map[string][]int
	long      map[int]*sentenceWindows
}

// windowsOf returns the window cache of sentence si, whose text is chunk.
func (x *atomIndex) windowsOf(si int, chunk string) *sentenceWindows {
	if sw, ok := x.long[si]; ok {
		return sw
	}
	if x.long == nil {
		x.long = map[int]*sentenceWindows{}
	}
	sw := newSentenceWindows(chunk)
	x.long[si] = sw
	return sw
}

// sentencesWith returns, in order, the indices of the sentences containing
// atom. Atoms never contain sentence punctuation, so an occurrence always
// lies inside one sentence.
func (x *atomIndex) sentencesWith(atom string) []int {
	if got, ok := x.cache[atom]; ok {
		return got
	}
	var out []int
	for off := 0; off < len(x.text); {
		k := strings.Index(x.text[off:], atom)
		if k < 0 {
			break
		}
		pos := off + k
		si := sort.Search(len(x.sentences), func(j int) bool { return x.sentences[j].end > pos })
		if si == len(x.sentences) {
			break
		}
		if x.sentences[si].start > pos {
			// pos lies between kept sentences (in a skipped sentence or in
			// punctuation): search on from the next kept sentence's start.
			off = x.sentences[si].start
			continue
		}
		out = append(out, si)
		off = x.sentences[si].end
	}
	if x.cache == nil {
		x.cache = map[string][]int{}
	}
	x.cache[atom] = out
	return out
}

// directCheck is the number of sentences up to which candidates tests each
// sentence directly instead of locating atoms in the whole text.
const directCheck = 6

func containsAny(s string, atoms []string) bool {
	for _, a := range atoms {
		if strings.Contains(s, a) {
			return true
		}
	}
	return false
}

// candidates returns, in order, the indices of the sentences that contain
// one of p's atoms (every sentence when p has no atoms).
func (p *pattern) candidates(x *atomIndex, buf []int) []int {
	buf = buf[:0]
	if p.atoms == nil {
		for i := range x.sentences {
			buf = append(buf, i)
		}
		return buf
	}
	if len(x.sentences) <= directCheck {
		for i, sp := range x.sentences {
			chunk := x.text[sp.start:sp.end]
			if len(chunk) >= windowMin {
				if x.windowsOf(i, chunk).hasAny(p.atoms) {
					buf = append(buf, i)
				}
			} else if containsAny(chunk, p.atoms) {
				buf = append(buf, i)
			}
		}
		return buf
	}
	for _, a := range p.atoms {
		buf = append(buf, x.sentencesWith(a)...)
	}
	if len(p.atoms) > 1 {
		sort.Ints(buf)
		buf = compactInts(buf)
	}
	return buf
}

func compactInts(s []int) []int {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// each calls fn for every accepted match in s, including overlapping ones,
// until fn returns true. Patterns start with \b, so searching again from a
// word start keeps the boundary semantics of the full string. third and
// marked report the pattern's attribution and context-marker checks (see
// pattern.third and pattern.marker), which read ctx: s itself, or for a wide
// pattern s followed by the text after it (see pattern.wide).
func (p *pattern) each(s, ctx string, fn func(start, end int, third, marked bool) bool) {
	for pos := 0; pos <= len(s); {
		start, end, ok := p.find(s, pos, len(s))
		if !ok {
			return
		}
		if p.reject == nil || !p.reject(ctx, start, end) {
			third := p.third != nil && p.third(ctx, start, end)
			if fn(start, end, third, p.marker != nil && p.marker(ctx, start, end)) {
				return
			}
		}
		pos = nextWordStart(s, start+1)
	}
}

// find returns the leftmost match of p in s[pos:hi], as offsets in s. pos
// starts a word or follows a non-word byte, and hi ends the text or comes
// before a non-word byte (see each and eachIn), so a word boundary reads the
// same in s[pos:hi] as in s.
func (p *pattern) find(s string, pos, hi int) (int, int, bool) {
	if p.word == "" {
		loc := p.re.FindStringIndex(s[pos:hi])
		if loc == nil {
			return 0, 0, false
		}
		return pos + loc[0], pos + loc[1], true
	}
	for pos < hi {
		k := strings.Index(s[pos:hi], p.word)
		if k < 0 {
			break
		}
		start, end := pos+k, pos+k+len(p.word)
		if (start == 0 || !isWordByte(s[start-1])) && (end == len(s) || !isWordByte(s[end])) {
			return start, end, true
		}
		pos = start + 1
	}
	return 0, 0, false
}

// nextWordStart returns the first index >= i that starts a word, or
// len(s)+1 when there is none.
func nextWordStart(s string, i int) int {
	for j := i; j < len(s); j++ {
		if isWordByte(s[j]) && (j == 0 || !isWordByte(s[j-1])) {
			return j
		}
	}
	return len(s) + 1
}

var levelRank = map[Level]int{LevelNone: 0, LevelElevated: 1, LevelCrisis: 2}
