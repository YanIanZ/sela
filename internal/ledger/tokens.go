package ledger

import (
	"sort"
	"strings"
	"unicode"
)

// token is one folded word with its original byte span and clause context.
//
// clause changes at commas, colons and everything that changes region;
// region changes at sentence-final punctuation, semicolons, line breaks and
// contrastive conjunctions. Negation uses the narrow clause; subject, time
// and certainty use the wider region.
type token struct {
	text       string
	start, end int
	clause     int
	region     int
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

func isHardBoundary(r rune) bool {
	switch r {
	case '.', '!', '?', ';', '\n', '\r', '\v', '\f', '\u0085', ' ', ' ', '。':
		return true
	}
	return false
}

// isSoftBoundary reports clause punctuation inside a sentence. Quote marks,
// brackets, slashes, dashes and bars are deliberately transparent: they set
// words off inside one clause and must not shield them from a negator
// (`tidak "baik-baik saja"`, "nggak (terlalu) cemas", "nggak sedih/cemas").
func isSoftBoundary(r rune) bool {
	switch r {
	case ',', ':', '、':
		return true
	}
	return false
}

// tokenize splits s into folded word tokens. Apostrophes and hyphens between
// letters stay inside a word ("don't", "akhir-akhir"), as do '.' and ','
// between digits ("1,5").
func tokenize(s string) []token {
	folded := foldSpans(s)
	var toks []token
	clause, region := 0, 0
	var cur []rune
	curStart, curEnd := 0, 0
	flush := func() {
		if len(cur) > 0 {
			toks = append(toks, token{text: string(cur), start: curStart, end: curEnd, clause: clause, region: region})
			cur = cur[:0]
		}
	}
	for i, f := range folded {
		r := f.r
		if isWordRune(r) {
			if len(cur) == 0 {
				curStart = f.start
			}
			cur = append(cur, r)
			curEnd = f.end
			continue
		}
		if len(cur) > 0 && i+1 < len(folded) && isWordRune(folded[i+1].r) {
			last := cur[len(cur)-1]
			next := folded[i+1].r
			if (r == '\'' || r == '-') && unicode.IsLetter(last) && unicode.IsLetter(next) {
				cur = append(cur, r)
				continue
			}
			if (r == '.' || r == ',') && unicode.IsDigit(last) && unicode.IsDigit(next) {
				cur = append(cur, r)
				continue
			}
		}
		flush()
		switch {
		case isHardBoundary(r):
			region++
			clause++
		case isSoftBoundary(r):
			clause++
		}
	}
	flush()
	// A contrastive conjunction opens a new clause and region at itself.
	shift := 0
	for i := range toks {
		if contrastive[toks[i].text] {
			shift++
		}
		toks[i].clause += shift
		toks[i].region += shift
	}
	return toks
}

// phrase is a lexicon entry: consecutive folded tokens and a tag.
type phrase struct {
	words []string
	tag   string
}

// lexicon matches multi-word phrases over tokens, longest first.
type lexicon struct {
	byFirst map[string][]phrase
}

// newLexicon builds a lexicon from tag -> space-separated phrases.
func newLexicon(entries map[string][]string) *lexicon {
	lx := &lexicon{byFirst: map[string][]phrase{}}
	for tag, list := range entries {
		for _, p := range list {
			words := strings.Fields(p)
			if len(words) == 0 {
				continue
			}
			lx.byFirst[words[0]] = append(lx.byFirst[words[0]], phrase{words: words, tag: tag})
		}
	}
	for k, ps := range lx.byFirst {
		sort.SliceStable(ps, func(i, j int) bool {
			if len(ps[i].words) != len(ps[j].words) {
				return len(ps[i].words) > len(ps[j].words)
			}
			return ps[i].tag < ps[j].tag
		})
		lx.byFirst[k] = ps
	}
	return lx
}

// matchAt returns the length in tokens and tag of the longest phrase starting
// at toks[i] whose tokens all lie in one region, or 0.
func (lx *lexicon) matchAt(toks []token, i int) (int, string) {
	for _, p := range lx.byFirst[toks[i].text] {
		if i+len(p.words) > len(toks) {
			continue
		}
		ok := true
		for k, w := range p.words {
			t := toks[i+k]
			if t.text != w || t.region != toks[i].region {
				ok = false
				break
			}
		}
		if ok {
			return len(p.words), p.tag
		}
	}
	return 0, ""
}

// hit is a phrase match over token indices [start, end).
type hit struct {
	start, end int
	tag        string
}

// scan returns non-overlapping, longest-first matches from left to right.
func (lx *lexicon) scan(toks []token) []hit {
	var hits []hit
	for i := 0; i < len(toks); {
		if n, tag := lx.matchAt(toks, i); n > 0 {
			hits = append(hits, hit{start: i, end: i + n, tag: tag})
			i += n
			continue
		}
		i++
	}
	return hits
}

// has reports whether any phrase of the lexicon occurs in s.
func (lx *lexicon) has(s string) bool { return len(lx.scan(tokenize(s))) > 0 }

// wordParts splits a token into its hyphen/apostrophe parts ("tugas-tugas"
// -> "tugas", "friend's" -> "friend", "s").
func wordParts(t string) []string {
	return strings.FieldsFunc(t, func(r rune) bool { return r == '-' || r == '\'' })
}

// isContent reports whether a word part carries content: at least three
// runes and not a stopword.
func isContent(p string) bool { return runeLen(p) >= 3 && !stopwords[p] }

// contentWords returns the distinct content words of s in order of first
// appearance: word parts of at least three runes that are not stopwords.
// Negators are not content words; their effect is tracked per word by
// negationCounts.
func contentWords(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tokenize(s) {
		if isNegator(t.text) {
			continue
		}
		for _, p := range wordParts(t.text) {
			if !isContent(p) || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// negationCounts returns, for every token, how many negators govern it: the
// negators within negationWindow tokens before it in the same clause. With
// narrowHedge, a hedging negator ("not sure", "belum tahu") governs only the
// knowing word right after it, and a fused hedge ("gatau") governs nothing.
func negationCounts(toks []token, narrowHedge bool) []int {
	counts := make([]int, len(toks))
	for i, t := range toks {
		if !isNegator(t.text) {
			continue
		}
		window := negationWindow
		if narrowHedge && isHedge(toks, i) {
			window = 1
			if fusedNegators[t.text] {
				window = 0
			}
		}
		for k := i + 1; k < len(toks) && k <= i+window && toks[k].clause == t.clause; k++ {
			counts[k]++
		}
	}
	return counts
}

// polarWord is one content word with the number of negators governing it.
type polarWord struct {
	word string
	neg  int
}

// polarWords lists the content words of toks[from:to], in order and with
// repeats, each with its count from negationCounts.
func polarWords(toks []token, counts []int, from, to int) []polarWord {
	var out []polarWord
	for i := from; i < to; i++ {
		if isNegator(toks[i].text) {
			continue
		}
		for _, p := range wordParts(toks[i].text) {
			if isContent(p) {
				out = append(out, polarWord{word: p, neg: counts[i]})
			}
		}
	}
	return out
}

// stem strips at most one common Indonesian clitic or particle suffix, one
// English inflection and one Indonesian prefix, each only when at least four
// runes remain. It is deliberately conservative: it is used only to decide
// whether two spellings of a word are the same word.
func stem(w string) string {
	strip := func(s, affix string, suffix bool) (string, bool) {
		if suffix && strings.HasSuffix(s, affix) && runeLen(s)-runeLen(affix) >= 4 {
			return strings.TrimSuffix(s, affix), true
		}
		if !suffix && strings.HasPrefix(s, affix) && runeLen(s)-runeLen(affix) >= 4 {
			return strings.TrimPrefix(s, affix), true
		}
		return s, false
	}
	for _, suf := range []string{"nya", "lah", "kah", "ku", "mu"} {
		if s, ok := strip(w, suf, true); ok {
			w = s
			break
		}
	}
	for _, suf := range []string{"ing", "ed", "es", "s"} {
		if s, ok := strip(w, suf, true); ok {
			w = s
			break
		}
	}
	for _, pre := range []string{"meng", "meny", "mem", "men", "ber", "ter", "me", "di"} {
		if s, ok := strip(w, pre, false); ok {
			w = s
			break
		}
	}
	return w
}

// covers is v1's loose morphology: one word contains the other and the
// shorter has at least four runes ("berfokus" covers "fokus").
func covers(a, b string) bool {
	if a == b {
		return true
	}
	if min(runeLen(a), runeLen(b)) < 4 {
		return false
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}

// sameWord reports whether two content words are spellings of one word.
func sameWord(a, b string) bool {
	if a == b {
		return true
	}
	sa, sb := stem(a), stem(b)
	return sa == sb || covers(a, b) || covers(sa, sb)
}
