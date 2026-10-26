package safety

import (
	"slices"
	"sort"
	"strconv"
	"strings"
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
func Assess(text string) Assessment { return assess(text, true) }

// assess matches every variant of text (see variants) and folds the
// outcomes. With skip, a derived variant only re-matches its context units
// that differ from the canonical form's (see contextUnits): every other unit
// has exactly the same matches and dispositions there. TestSkipEquivalence
// checks this against assess(text, false).
func assess(text string, skip bool) Assessment {
	vs := variants(text)
	acc := &accumulator{states: make([]state, len(catalogue)), attributed: make([]bool, len(catalogue))}
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
	shapes  []*shape
}

// hit is one accepted match of catalogue[rule]. third is set when the
// pattern's attribution check names someone else's act ("bom bunuh diri").
type hit struct {
	rule int
	span
	third bool
}

// evaluate matches every rule against the given sentences of one normalised
// variant and records how each match counts.
func (acc *accumulator) evaluate(c *shape, sentences []span) {
	hits := matchAll(c, sentences)
	for _, h := range hits {
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
				if p.filter != nil && p.filter.op != 'a' && !p.filter.match(chunk) {
					continue
				}
				p.each(chunk, func(start, end int, third bool) bool {
					hits = append(hits, hit{i, span{sent.start + start, sent.start + end}, third})
					return p.maximal
				})
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
func (c *shape) sharedAfter(m span) bool {
	lo, hi := c.overlapping(m)
	for i := lo; i < hi; i++ {
		segs := c.clauses[i].segments
		k := sort.Search(len(segs), func(n int) bool { return segs[n].end >= m.end })
		if k < len(segs) && segs[k].start <= m.end {
			return sharedRe.MatchString(c.text[m.end:segs[k].end])
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
// whether a contrastive conjunction follows it, the whole-text denial check
// and adoption. The key holds the first three; assess compares the fourth
// and folds adoption over all variants.
func contextUnits(c *shape) []unit {
	t := c.text
	var out []unit
	q, first := 0, 0 // q: quotation at or after i; first: first one of the unit
	start := 0
	var key []byte
	emit := func(end int) {
		key = append(key[:0], t[start:end]...)
		key = append(key, 0)
		for ; first < len(c.quotes) && c.quotes[first].start < end; first++ {
			key = strconv.AppendInt(key, int64(c.quotes[first].start-start), 10)
			key = append(key, ':')
			key = strconv.AppendInt(key, int64(c.quotes[first].end-start), 10)
			key = append(key, ' ')
		}
		if conjunctionFollows(t, end) {
			key = append(key, 0, 'c')
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

// conjunctionFollows reports whether a contrastive conjunction follows
// position i of t with only punctuation and spaces in between, which is what
// marks the clause before it contrastNext in splitClauses.
func conjunctionFollows(t string, i int) bool {
	for i < len(t) && strings.IndexByte(" \t\n,.!?;", t[i]) >= 0 {
		i++
	}
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
// for atoms shared by several patterns.
type atomIndex struct {
	text      string
	sentences []span
	cache     map[string][]int
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
			if containsAny(x.text[sp.start:sp.end], p.atoms) {
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
// word start keeps the boundary semantics of the full string. third reports
// the pattern's attribution check (see pattern.third).
func (p *pattern) each(s string, fn func(start, end int, third bool) bool) {
	for pos := 0; pos <= len(s); {
		loc := p.re.FindStringIndex(s[pos:])
		if loc == nil {
			return
		}
		start, end := pos+loc[0], pos+loc[1]
		if p.reject == nil || !p.reject(s, start, end) {
			if fn(start, end, p.third != nil && p.third(s, start, end)) {
				return
			}
		}
		pos = nextWordStart(s, start+1)
	}
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
