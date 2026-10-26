package safety

import (
	"regexp/syntax"
	"sort"
	"strings"
	"unicode/utf8"
)

// Matching a long sentence in windows (review finding: a single long
// sentence, such as an unpunctuated voice transcript, ran every pattern whose
// prefilter it passed over its whole length, so a 32 KB story took about
// 2s). Every match of a pattern contains one of its atoms (see
// filter.flatAtoms) and is at most maxLen bytes long, so it lies inside
// [p+len(atom)-maxLen, p+maxLen] for an occurrence p of an atom. A pattern
// is therefore run only in those windows, widened to word boundaries and
// merged where they overlap: inside a window every assertion (\b, \B) reads
// exactly what it reads in the whole sentence, a match found there is a
// match of the sentence with the same preferred end, and the matches are
// visited in the same order (see eachIn). TestWindowEquivalence checks this
// against matching whole sentences.
//
// maxLen is bounded per sentence: a repetition ("\s*", "[a-z']+",
// "(?:(?:aja|saja)\s*)*") consumes a run of the bytes it can match, so it is
// at most as long as the longest such run in the sentence (see lenNode).
// A pattern with a line or text anchor, or a maximal pattern, is matched on
// the whole sentence as before.

// windowMin is the sentence length, in bytes, from which patterns are
// matched in windows. Shorter sentences are matched whole.
var windowMin = 768

// lenNode is a syntax tree reduced to what bounds the length of a match.
type lenNode struct {
	op   byte // 'f' fixed length n, 'c' concatenation, 'a' alternation, 'r' repetition of subs[0]
	n    int  // 'f': bytes; 'r': maximum count, -1 unbounded
	set  int  // 'r': index in runSets of the bytes subs[0] can consume
	subs []*lenNode
}

// runSets are the distinct byte sets of the repetitions in the catalogue.
var runSets [][256]bool

// windowInfo holds what windowed matching needs to know about a pattern.
type windowInfo struct {
	lens *lenNode // nil: the pattern is matched whole
}

// newWindowInfo analyses expr (see lenNode). Patterns with a line or text
// anchor are not windowed.
func newWindowInfo(expr string) windowInfo {
	re, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		panic("safety: invalid pattern " + expr + ": " + err.Error())
	}
	n, ok := buildLen(re)
	if !ok {
		return windowInfo{}
	}
	return windowInfo{lens: n}
}

// buildLen reduces re to a lenNode; ok is false when re has an anchor that
// reads beyond a window.
func buildLen(re *syntax.Regexp) (*lenNode, bool) {
	switch re.Op {
	case syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText:
		return nil, false
	case syntax.OpEmptyMatch, syntax.OpWordBoundary, syntax.OpNoWordBoundary, syntax.OpNoMatch:
		return &lenNode{op: 'f'}, true
	case syntax.OpLiteral:
		n := 0
		for _, r := range re.Rune {
			if re.Flags&syntax.FoldCase != 0 && r >= utf8.RuneSelf {
				n += utf8.UTFMax
			} else {
				n += utf8.RuneLen(r)
			}
		}
		return &lenNode{op: 'f', n: n}, true
	case syntax.OpCharClass:
		n := 1
		for i := 1; i < len(re.Rune); i += 2 {
			n = max(n, utf8.RuneLen(re.Rune[i]))
		}
		return &lenNode{op: 'f', n: n}, true
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		return &lenNode{op: 'f', n: utf8.UTFMax}, true
	case syntax.OpCapture:
		return buildLen(re.Sub[0])
	case syntax.OpConcat, syntax.OpAlternate:
		op := byte('c')
		if re.Op == syntax.OpAlternate {
			op = 'a'
		}
		out := &lenNode{op: op}
		for _, sub := range re.Sub {
			n, ok := buildLen(sub)
			if !ok {
				return nil, false
			}
			out.subs = append(out.subs, n)
		}
		return out, true
	case syntax.OpQuest:
		return buildLen(re.Sub[0])
	case syntax.OpStar, syntax.OpPlus, syntax.OpRepeat:
		sub, ok := buildLen(re.Sub[0])
		if !ok {
			return nil, false
		}
		count := -1
		if re.Op == syntax.OpRepeat {
			count = re.Max
		}
		var set [256]bool
		byteSet(re.Sub[0], &set)
		return &lenNode{op: 'r', n: count, set: runSetIndex(set), subs: []*lenNode{sub}}, true
	}
	return nil, false
}

// byteSet adds to set every byte that re can consume.
func byteSet(re *syntax.Regexp, set *[256]bool) {
	addRune := func(r rune) {
		if r < utf8.RuneSelf {
			set[r] = true
			return
		}
		for b := 0x80; b < 0x100; b++ {
			set[b] = true
		}
	}
	switch re.Op {
	case syntax.OpLiteral:
		for _, r := range re.Rune {
			addRune(r)
			if re.Flags&syntax.FoldCase != 0 {
				for f := syntaxSimpleFold(r); f != r; f = syntaxSimpleFold(f) {
					addRune(f)
				}
			}
		}
	case syntax.OpCharClass:
		for i := 0; i+1 < len(re.Rune); i += 2 {
			lo, hi := re.Rune[i], re.Rune[i+1]
			for r := lo; r <= min(hi, utf8.RuneSelf-1); r++ {
				set[r] = true
			}
			if hi >= utf8.RuneSelf {
				addRune(hi)
			}
		}
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		for b := range set {
			set[b] = re.Op == syntax.OpAnyChar || b != '\n'
		}
	default:
		for _, sub := range re.Sub {
			byteSet(sub, set)
		}
	}
}

// syntaxSimpleFold is unicode.SimpleFold restricted to ASCII letters, which
// is all case folding can mean in the normalised text's patterns; other
// runes are their own fold here (their bytes are all added anyway).
func syntaxSimpleFold(r rune) rune {
	switch {
	case r >= 'a' && r <= 'z':
		return r - 'a' + 'A'
	case r >= 'A' && r <= 'Z':
		return r - 'A' + 'a'
	}
	return r
}

// runSetIndex returns the index of set in runSets, adding it when new.
func runSetIndex(set [256]bool) int {
	for i, s := range runSets {
		if s == set {
			return i
		}
	}
	runSets = append(runSets, set)
	return len(runSets) - 1
}

// maxLen returns the longest match n allows, given the longest run in the
// sentence of each byte set in runSets (see sentenceWindows.run).
func (n *lenNode) maxLen(sw *sentenceWindows) int {
	switch n.op {
	case 'f':
		return n.n
	case 'c':
		t := 0
		for _, s := range n.subs {
			t += s.maxLen(sw)
		}
		return t
	case 'a':
		t := 0
		for _, s := range n.subs {
			t = max(t, s.maxLen(sw))
		}
		return t
	}
	sub := n.subs[0].maxLen(sw)
	if sub == 0 {
		return 0
	}
	b := sw.run(n.set)
	if n.n >= 0 {
		b = min(b, n.n*sub)
	}
	return b
}

// sentenceWindows caches, for one long sentence, the atom occurrences and
// the longest runs that windows are computed from. Occurrences are found
// through an index of the sentence's three-byte grams, so that looking up
// the many atoms of the catalogue stays cheap.
type sentenceWindows struct {
	text  string
	runs  []int // -1: not computed yet
	occ   map[string][]int
	grams map[uint32][]int32
	pairs map[uint16][]int32
}

// hasAny reports whether one of atoms occurs in the sentence.
func (sw *sentenceWindows) hasAny(atoms []string) bool {
	for _, a := range atoms {
		if len(sw.occurrences(a)) > 0 {
			return true
		}
	}
	return false
}

func newSentenceWindows(s string) *sentenceWindows {
	sw := &sentenceWindows{text: s, runs: make([]int, len(runSets)), occ: map[string][]int{}}
	for i := range sw.runs {
		sw.runs[i] = -1
	}
	return sw
}

// run returns the longest run in the sentence of the bytes of runSets[i].
func (sw *sentenceWindows) run(i int) int {
	if sw.runs[i] >= 0 {
		return sw.runs[i]
	}
	set := &runSets[i]
	s := sw.text
	best, cur := 0, 0
	for j := 0; j < len(s); j++ {
		if set[s[j]] {
			cur++
			best = max(best, cur)
		} else {
			cur = 0
		}
	}
	sw.runs[i] = best
	return best
}

func gram(s string, i int) uint32 {
	return uint32(s[i])<<16 | uint32(s[i+1])<<8 | uint32(s[i+2])
}

// occurrences returns the start offsets of atom in the sentence, in order.
func (sw *sentenceWindows) occurrences(atom string) []int {
	if got, ok := sw.occ[atom]; ok {
		return got
	}
	var out []int
	s := sw.text
	if len(atom) == 2 {
		if sw.pairs == nil {
			sw.pairs = make(map[uint16][]int32, len(s)/4)
			for i := 0; i+2 <= len(s); i++ {
				g := uint16(s[i])<<8 | uint16(s[i+1])
				sw.pairs[g] = append(sw.pairs[g], int32(i))
			}
		}
		for _, i := range sw.pairs[uint16(atom[0])<<8|uint16(atom[1])] {
			out = append(out, int(i))
		}
	} else if len(atom) < 3 {
		for off := 0; off < len(s); {
			k := indexFrom(s, atom, off)
			if k < 0 {
				break
			}
			out = append(out, k)
			off = k + 1
		}
	} else {
		if sw.grams == nil {
			sw.grams = make(map[uint32][]int32, len(s)/2)
			for i := 0; i+3 <= len(s); i++ {
				g := gram(s, i)
				sw.grams[g] = append(sw.grams[g], int32(i))
			}
		}
		for _, i := range sw.grams[gram(atom, 0)] {
			if strings.HasPrefix(s[i:], atom) {
				out = append(out, int(i))
			}
		}
	}
	sw.occ[atom] = out
	return out
}

// windows returns the merged windows of the sentence in which p can match,
// or nil when p is to be matched on the whole sentence (no atoms, no length
// bound, or windows that would cover most of it anyway).
func (sw *sentenceWindows) windows(p *pattern) []span {
	if p.win.lens == nil || p.atoms == nil {
		return nil
	}
	s := sw.text
	ml := p.win.lens.maxLen(sw)
	if 2*ml >= len(s) {
		return nil
	}
	var ws []span
	for _, a := range p.atoms {
		for _, at := range sw.occurrences(a) {
			lo, hi := max(0, at+len(a)-ml), min(len(s), at+ml)
			for lo > 0 && isWordByte(s[lo-1]) {
				lo--
			}
			for hi < len(s) && isWordByte(s[hi]) {
				hi++
			}
			// The match lies inside the window, so the window must pass the
			// whole prefilter (a hope needs its death word too).
			if p.filter == nil || sw.filterIn(p.filter, lo, hi) {
				ws = append(ws, span{lo, hi})
			}
		}
	}
	if len(ws) == 0 {
		return []span{} // no atom: no match
	}
	sort.Slice(ws, func(i, j int) bool { return ws[i].start < ws[j].start })
	out := ws[:1]
	covered := 0
	for _, w := range ws[1:] {
		last := &out[len(out)-1]
		if w.start <= last.end {
			last.end = max(last.end, w.end)
			continue
		}
		out = append(out, w)
	}
	for _, w := range out {
		covered += w.end - w.start
	}
	if 4*covered >= 3*len(s) {
		return nil
	}
	return out
}

// filterIn reports whether f can be satisfied by the text s[lo:hi] of the
// sentence: every atom it needs occurs wholly inside it.
func (sw *sentenceWindows) filterIn(f *filter, lo, hi int) bool {
	switch f.op {
	case 'a':
		for _, a := range f.atoms {
			occ := sw.occurrences(a)
			k := sort.SearchInts(occ, lo)
			if k < len(occ) && occ[k]+len(a) <= hi {
				return true
			}
		}
		return false
	case '&':
		for _, sub := range f.subs {
			if !sw.filterIn(sub, lo, hi) {
				return false
			}
		}
		return true
	default:
		for _, sub := range f.subs {
			if sw.filterIn(sub, lo, hi) {
				return true
			}
		}
		return false
	}
}

// indexFrom returns the index of sub in s at or after off, or -1.
func indexFrom(s, sub string, off int) int {
	k := strings.Index(s[off:], sub)
	if k < 0 {
		return -1
	}
	return off + k
}

// eachIn is each restricted to the windows ws of s (see sentenceWindows):
// it visits exactly the matches each visits, in the same order, because
// every match lies inside one window and the next search position carries
// over from one window to the next.
func (p *pattern) eachIn(s, ctx string, ws []span, fn func(start, end int, third, marked bool) bool) {
	pos := 0
	for _, w := range ws {
		pos = max(pos, w.start)
		for pos < w.end {
			loc := p.re.FindStringIndex(s[pos:w.end])
			if loc == nil {
				break
			}
			start, end := pos+loc[0], pos+loc[1]
			if p.reject == nil || !p.reject(ctx, start, end) {
				third := p.third != nil && p.third(ctx, start, end)
				if fn(start, end, third, p.marker != nil && p.marker(ctx, start, end)) {
					return
				}
			}
			pos = nextWordStart(s, start+1)
		}
	}
}
