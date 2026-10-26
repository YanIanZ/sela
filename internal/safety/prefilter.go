package safety

import (
	"regexp/syntax"
	"strings"
)

// A prefilter is derived from each pattern's parsed syntax tree, so it cannot
// drift from the expression. It is an AND/OR tree of literal atoms that every
// match must satisfy; a sentence that fails it cannot match and the regexp is
// not run. Each node of the syntax tree yields either an exact set (every
// match is one of a few known strings) or a filter:
//   - literals, small character classes and zero-width assertions are exact;
//   - a concatenation joins neighbouring exact parts by cross product and
//     requires every part (AND); a prefix that the parser factored out of an
//     alternation ("m" in "m(?:au\s*mati|ati\s*aja)") is also distributed
//     back over the branches;
//   - an alternation requires one of its branches (OR);
//   - a repetition requires one occurrence; optional parts, stars and
//     anything that can match the empty string require nothing.

// filter is a prefilter node: op 'a' needs one of atoms, '&' needs every
// sub-filter and '|' needs one of them.
type filter struct {
	op    byte
	atoms []string
	subs  []*filter
}

// match reports whether s can contain a match.
func (f *filter) match(s string) bool {
	switch f.op {
	case 'a':
		for _, a := range f.atoms {
			if strings.Contains(s, a) {
				return true
			}
		}
		return false
	case '&':
		for _, sub := range f.subs {
			if !sub.match(s) {
				return false
			}
		}
		return true
	default:
		for _, sub := range f.subs {
			if sub.match(s) {
				return true
			}
		}
		return false
	}
}

// flatAtoms returns one set of atoms such that every match contains one of
// them (the strongest set the filter implies), or nil.
func (f *filter) flatAtoms() []string {
	if f == nil {
		return nil
	}
	switch f.op {
	case 'a':
		return f.atoms
	case '|':
		var union []string
		for _, sub := range f.subs {
			a := sub.flatAtoms()
			if a == nil {
				return nil
			}
			union = append(union, a...)
		}
		return dedupeStrings(union)
	default:
		var best []string
		for _, sub := range f.subs {
			if a := sub.flatAtoms(); a != nil && (best == nil || strongerAtoms(a, best)) {
				best = a
			}
		}
		return best
	}
}

// compileFilter derives the prefilter for expr.
func compileFilter(expr string) *filter {
	re, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		panic("safety: invalid pattern " + expr + ": " + err.Error())
	}
	return infoOf(re.Simplify()).filter()
}

// maxExact bounds exact sets and cross products.
const maxExact = 16

// info describes what a syntax node can match.
type info struct {
	exact   []string // every match is one of these (when isExact)
	isExact bool
	f       *filter // otherwise every match satisfies f; nil: unknown
}

func exactInfo(set []string) info { return info{exact: dedupeStrings(set), isExact: true} }

// filter turns an info into a filter, or nil when nothing is required.
func (i info) filter() *filter {
	if !i.isExact {
		return i.f
	}
	return atomsFilter(i.exact)
}

func atomsFilter(set []string) *filter {
	if len(set) == 0 {
		return nil
	}
	for _, s := range set {
		if s == "" {
			return nil // the node can match the empty string
		}
	}
	return &filter{op: 'a', atoms: set}
}

func combine(op byte, subs []*filter) *filter {
	var flat []*filter
	for _, s := range subs {
		if s.op == op {
			flat = append(flat, s.subs...)
		} else {
			flat = append(flat, s)
		}
	}
	if len(flat) == 1 {
		return flat[0]
	}
	return &filter{op: op, subs: flat}
}

func infoOf(re *syntax.Regexp) info {
	switch re.Op {
	case syntax.OpLiteral:
		if re.Flags&syntax.FoldCase != 0 {
			return info{}
		}
		return exactInfo([]string{string(re.Rune)})
	case syntax.OpCharClass:
		var set []string
		for i := 0; i+1 < len(re.Rune); i += 2 {
			for r := re.Rune[i]; r <= re.Rune[i+1]; r++ {
				if len(set) == 8 {
					return info{}
				}
				set = append(set, string(r))
			}
		}
		if len(set) == 0 {
			return info{}
		}
		return exactInfo(set)
	case syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText,
		syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return exactInfo([]string{""})
	case syntax.OpCapture:
		return infoOf(re.Sub[0])
	case syntax.OpQuest:
		if sub := infoOf(re.Sub[0]); sub.isExact && len(sub.exact) < maxExact {
			return exactInfo(append(append([]string{}, sub.exact...), ""))
		}
		return info{}
	case syntax.OpPlus:
		return info{f: infoOf(re.Sub[0]).filter()}
	case syntax.OpRepeat:
		if re.Min >= 1 {
			return info{f: infoOf(re.Sub[0]).filter()}
		}
		return info{}
	case syntax.OpAlternate:
		var infos []info
		var union []string
		allExact := true
		for _, sub := range re.Sub {
			si := infoOf(sub)
			infos = append(infos, si)
			allExact = allExact && si.isExact
			union = append(union, si.exact...)
		}
		if allExact && len(union) <= maxExact {
			return exactInfo(union)
		}
		var ors []*filter
		for _, si := range infos {
			f := si.filter()
			if f == nil {
				return info{}
			}
			ors = append(ors, f)
		}
		return info{f: combine('|', ors)}
	case syntax.OpConcat:
		return concatInfo(re.Sub, []string{""})
	}
	return info{}
}

// concatInfo describes the concatenation of subs, given that the match so
// far is exactly one of prefix.
func concatInfo(subs []*syntax.Regexp, prefix []string) info {
	cur := prefix // exact strings for the current run of exact parts
	allExact := true
	var ands []*filter
	flush := func() {
		if f := atomsFilter(cur); f != nil {
			ands = append(ands, f)
		}
	}
	for _, sub := range subs {
		si := infoOf(sub)
		if si.isExact {
			if len(cur)*len(si.exact) <= maxExact {
				cur = cross(cur, si.exact)
			} else {
				flush()
				cur = si.exact
				allExact = false
			}
			continue
		}
		allExact = false
		if si.f != nil {
			ands = append(ands, si.f)
		}
		// Distribute the exact run over a following alternation, so a
		// prefix factored out by the parser joins each branch again.
		if alt := unwrapCapture(sub); alt.Op == syntax.OpAlternate && hasNonEmpty(cur) {
			var ors []*filter
			for _, b := range alt.Sub {
				parts := []*syntax.Regexp{b}
				if b.Op == syntax.OpConcat {
					parts = b.Sub
				}
				f := concatInfo(parts, cur).filter()
				if f == nil {
					ors = nil
					break
				}
				ors = append(ors, f)
			}
			if ors != nil {
				ands = append(ands, combine('|', ors))
			}
		}
		flush()
		cur = []string{""}
	}
	if allExact {
		return exactInfo(cur)
	}
	flush()
	if len(ands) == 0 {
		return info{}
	}
	return info{f: combine('&', ands)}
}

func unwrapCapture(re *syntax.Regexp) *syntax.Regexp {
	for re.Op == syntax.OpCapture {
		re = re.Sub[0]
	}
	return re
}

func hasNonEmpty(set []string) bool {
	for _, s := range set {
		if s != "" {
			return true
		}
	}
	return false
}

func cross(a, b []string) []string {
	out := make([]string, 0, len(a)*len(b))
	for _, x := range a {
		for _, y := range b {
			out = append(out, x+y)
		}
	}
	return dedupeStrings(out)
}

// strongerAtoms prefers the set whose shortest atom is longest, then the
// smaller set.
func strongerAtoms(a, b []string) bool {
	ma, mb := minLen(a), minLen(b)
	if ma != mb {
		return ma > mb
	}
	return len(a) < len(b)
}

func minLen(set []string) int {
	m := -1
	for _, s := range set {
		if m < 0 || len(s) < m {
			m = len(s)
		}
	}
	return m
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// mayMatch reports whether s passes the pattern's prefilter.
func (p *pattern) mayMatch(s string) bool {
	return p.filter == nil || p.filter.match(s)
}
