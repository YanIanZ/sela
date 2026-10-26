package safety

import (
	"slices"
	"sort"
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
func Assess(text string) Assessment {
	states := make([]state, len(catalogue))
	denial := false
	vs := variants(text)
	var seen map[string]bool
	for i, v := range vs {
		// A derived variant only re-matches the sentences it changed: the
		// others were already matched, in context, in the canonical form.
		if i == 1 {
			seen = map[string]bool{}
			for _, sp := range sentenceSpans(vs[0]) {
				seen[vs[0][sp.start:sp.end]] = true
			}
		}
		if evaluate(v, states, seen) {
			denial = true
		}
	}
	return assemble(states, denial)
}

// hit is one accepted match of catalogue[rule].
type hit struct {
	rule int
	span
}

// evaluate matches every rule against one normalised variant and raises
// states accordingly, skipping sentences whose text is in skip. It reports
// whether an explicit denial discarded or hedged a match.
func evaluate(text string, states []state, skip map[string]bool) (denial bool) {
	if text == "" {
		return false
	}
	var sh *shape
	shapeOf := func() *shape {
		if sh == nil {
			sh = analyse(text)
		}
		return sh
	}
	var hits []hit
	sentences := sentenceSpans(text)
	if skip != nil {
		kept := sentences[:0]
		for _, sp := range sentences {
			if !skip[text[sp.start:sp.end]] {
				kept = append(kept, sp)
			}
		}
		sentences = kept
	}
	idx := &atomIndex{text: text, sentences: sentences}
	var cand []int
	for i := range catalogue {
		for _, p := range catalogue[i].patterns {
			if p.anchored {
				if !p.mayMatch(text) {
					continue
				}
				if p.re.MatchString(text) {
					hits = append(hits, hit{i, span{0, len(text)}})
				}
				for _, cl := range shapeOf().clauses {
					if p.re.MatchString(text[cl.start:cl.end]) {
						hits = append(hits, hit{i, cl.span})
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
				p.each(chunk, func(start, end int) bool {
					hits = append(hits, hit{i, span{sent.start + start, sent.start + end}})
					return false
				})
			}
		}
	}
	if len(hits) == 0 {
		return false
	}
	c := shapeOf()
	local := make([]state, len(catalogue))
	for _, h := range hits {
		d := c.dispose(&catalogue[h.rule], h.span)
		if d == stateDenied || d == stateHedged {
			denial = true
		}
		if d > local[h.rule] {
			local[h.rule] = d
		}
	}
	adopt := false
	for _, st := range local {
		if st == stateAttributed {
			adopt = c.adopted()
			break
		}
	}
	for i, st := range local {
		if st == stateAttributed && adopt {
			st = stateCounted
		}
		if st > states[i] {
			states[i] = st
		}
	}
	return denial
}

// dispose decides how one match of r counts: inside an explicit denial it is
// discarded (or hedged), third-person wording and attributed text make it
// third-party, and anything else is the speaker's own.
func (c *shape) dispose(r *rule, m span) state {
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
	if r.thirdPerson {
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
		if x.sentences[si].start <= pos {
			out = append(out, si)
		}
		off = max(pos+1, x.sentences[si].end)
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
// word start keeps the boundary semantics of the full string.
func (p *pattern) each(s string, fn func(start, end int) bool) {
	for pos := 0; pos <= len(s); {
		loc := p.re.FindStringIndex(s[pos:])
		if loc == nil {
			return
		}
		start, end := pos+loc[0], pos+loc[1]
		if p.reject == nil || !p.reject(s, start, end) {
			if fn(start, end) {
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

// assemble turns per-rule states into an Assessment. RuleIDs and Categories
// follow catalogue order; rules discarded by a denial are not listed.
func assemble(states []state, denial bool) Assessment {
	a := Assessment{Level: LevelNone, Denial: denial, Ruleset: RulesetVersion}
	counted, attributed := false, false
	for i, st := range states {
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
