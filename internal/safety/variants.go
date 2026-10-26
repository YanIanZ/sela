package safety

import (
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// variants returns the distinct strings Assess evaluates for text. The first
// is always Normalize(text). The others only ever add matches (each is
// matched in its own context, and the strongest outcome per rule wins):
//   - compact: hyphens, underscores and dots between two letters removed, so
//     "gan-tung di-ri" and "mau-mati" are seen as "gantung diri" and
//     "maumati" (patterns already allow optional spaces between words);
//   - doubled: elongation collapsed to two runes instead of one, so
//     "killll myself" is seen as "kill myself" rather than "kil myself";
//   - pronoun: a free-standing "1" read as the pronoun "i" (v1 mapped every
//     1 to i), so "1 hate my life" is seen as "i hate my life" while the
//     canonical form keeps numbers such as "jam 1" and "5 kms";
//   - lexical (see lexical): every elongated word resolved against the
//     catalogue's vocabulary, two-letter elongation inside a word included
//     ("killll myselfff" → "kill myself", "matii" → "mati", "pengeen" →
//     "pengen", "kiill" → "kill"), fused spaced-out words split ("akumaumati"
//     → "aku mau mati"), symbols, emoji, emphasis marks and pauses between
//     words read as spaces ("aku pengen... mati", "aku pengen... 😭 mati",
//     "I want to . . . die", "aku pengen - mati", "I want to *die*", "bunuh
//     🙂 diri"), contractions and common misspellings spelt out ("wanna" →
//     "want to", "wana" → "want to", "my self" → "myself", "sucidal" →
//     "suicidal", "pngen" → "pengen", "bunuh dri" → "bunuh diri", "want 2
//     die" → "want to die"), and the pronoun reading of "1";
//   - compact lexical: the lexical variant with compact applied;
//   - alternative leetspeak: the lexical variant of a second reading of the
//     raw text in which 7 and + are t, @ is a, $ is s, ! inside a word is i,
//     a capital I inside a lowercase word is l, and each 1 or | is i or l,
//     whichever spells a vocabulary word ("ki11", "kiII", "suicidaI", "m@t!",
//     "$uicide", "$u1c1d3", "k1ll my$3lf", "m@t1"); a final "!" stays a mark
//     after a whole word ("$u1c1d3!").
func variants(text string) []string {
	base := prepare(text)
	primary := finish(base, 1)
	out := []string{primary}
	add := func(v string) {
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	add(compactJoin(primary))
	if hasRun3(base) {
		add(finish(base, 2))
	}
	add(oneAsPronoun(primary))
	lex := lexical(base)
	add(lex)
	add(compactJoin(lex))
	if alt := prepareAlt(text); alt != "" && alt != base {
		add(lexical(alt))
	}
	return out
}

// lexical returns the lexical reading of a prepared text (see variants).
func lexical(base string) string {
	s := collapseRunsLex(pipeAsI(base))
	s = symbolGaps(s)
	s = rejoinBunuhDiri(s)
	s = joinLetters(s, segmentWord)
	s = strings.TrimSpace(collapseSpaces(s))
	s = spelling(s)
	s = twoAsTo(s)
	return oneAsPronoun(s)
}

// pipeAsI reads a "|" between two letters as the i it imitates ("k|ll");
// the alternative leetspeak variant reads it as l ("ki||").
func pipeAsI(s string) string {
	if !strings.Contains(s, "|") {
		return s
	}
	b := []byte(s)
	for i := 1; i+1 < len(b); i++ {
		if b[i] == '|' && isLetter(b[i-1]) && (isLetter(b[i+1]) || b[i+1] == '|') {
			b[i] = 'i'
		}
	}
	return string(b)
}

// collapseRunsLex collapses elongation word by word: each word with a run
// of two or more identical letters takes the first of its possible spellings
// that is in the vocabulary (see resolveWord), or else the canonical
// spelling. Runs of dots keep two, so hesitation dots stay visible to
// symbolGaps; other runs collapse to one as in Normalize, and spaces are kept.
func collapseRunsLex(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if isLetter(s[i]) {
			j := i
			for j < len(s) && isLetter(s[j]) {
				j++
			}
			b.WriteString(resolveWord(s[i:j]))
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		j, n := i+size, 1
		for j < len(s) {
			r2, size2 := utf8.DecodeRuneInString(s[j:])
			if r2 != r {
				break
			}
			j += size2
			n++
		}
		if n >= 3 && r != ' ' {
			n = 1
			if r == '.' {
				n = 2
			}
		}
		for k := 0; k < n; k++ {
			b.WriteString(s[i : i+size])
		}
		i = j
	}
	return b.String()
}

// maxRuns bounds the repeated-letter runs resolveWord tries combinations of.
const maxRuns = 5

// resolveWord picks the spelling of an elongated word (see collapseRunsLex).
// Every run of two or more identical letters may be read with one or two of
// them, so a two-letter elongation inside a word ("pengeen", "bunuuh",
// "kiill", "diieee") resolves as well as a long one ("matiii"). The canonical
// spelling (runs of three or more as one letter, doubles kept, as Normalize
// writes them) is preferred, then the doubled one ("killll" → "kill"), then
// any other combination; a word with no spelling in the vocabulary keeps the
// canonical one.
func resolveWord(w string) string {
	type run struct{ start, n int }
	var runs []run
	long := 0 // runs of three or more
	for i := 0; i < len(w); {
		j := i
		for j < len(w) && w[j] == w[i] {
			j++
		}
		if j-i >= 2 {
			runs = append(runs, run{i, j - i})
			if j-i >= 3 {
				long |= 1 << (len(runs) - 1)
			}
		}
		i = j
	}
	if len(runs) == 0 {
		return w
	}
	if len(runs) > maxRuns {
		return collapseRuns(w, 1)
	}
	// Bit r of mask flips run r from its canonical length (one letter for a
	// long run, two for a double) to the other one.
	build := func(mask int) string {
		var b strings.Builder
		last := 0
		for r, ru := range runs {
			b.WriteString(w[last:ru.start])
			keep := 2
			if long&(1<<r) != 0 {
				keep = 1
			}
			if mask&(1<<r) != 0 {
				keep = 3 - keep
			}
			b.WriteString(w[ru.start : ru.start+keep])
			last = ru.start + ru.n
		}
		b.WriteString(w[last:])
		return b.String()
	}
	canonical := build(0)
	if vocabulary[canonical] {
		return canonical
	}
	if long != 0 {
		if d := build(long); vocabulary[d] {
			return d
		}
	}
	for mask := 1; mask < 1<<len(runs); mask++ {
		if c := build(mask); vocabulary[c] {
			return c
		}
	}
	return canonical
}

// segmentWord splits a joined spaced-out word that is not in the
// vocabulary into vocabulary words when it can be split completely
// ("akumaumati" → "aku mau mati", "killi" → "kill i").
func segmentWord(w string) string {
	if len(w) < 3 || vocabulary[w] {
		return w
	}
	if s := segment(w); s != "" {
		return s
	}
	return w
}

// segment splits w into vocabulary words and unknown stretches at the
// lowest cost (a word costs 1, an unknown letter unknownCost), so that
// "tolongakumaumati" becomes "tolong aku mau mati". It returns "" unless the
// split finds a vocabulary word of three or more letters. "i" is the only
// single-letter word. It runs in time linear in len(w) (each position tries
// at most maxVocabWord lengths).
func segment(w string) string {
	const unknownCost = 3
	n := len(w)
	cost := make([]int, n+1) // lowest cost of w[:i]
	prev := make([]int, n+1) // start of the last piece of w[:i]
	known := make([]bool, n+1)
	for i := 1; i <= n; i++ {
		cost[i], prev[i] = cost[i-1]+unknownCost, i-1
		for l := 1; l <= min(i, maxVocabWord); l++ {
			j := i - l
			if (l == 1 && w[j] != 'i') || !vocabulary[w[j:i]] {
				continue
			}
			if c := cost[j] + 1; c < cost[i] || (c == cost[i] && !known[i]) {
				cost[i], prev[i], known[i] = c, j, true
			}
		}
	}
	var parts []string
	long := false
	end := n // end of the pending unknown stretch
	for i := n; i > 0; i = prev[i] {
		if !known[i] {
			continue
		}
		if end > i {
			parts = append(parts, w[i:end])
		}
		parts = append(parts, w[prev[i]:i])
		long = long || i-prev[i] >= 3
		end = prev[i]
	}
	if end > 0 {
		parts = append(parts, w[:end])
	}
	if !long {
		return ""
	}
	slices.Reverse(parts)
	return strings.Join(parts, " ")
}

// symbolGaps reads symbols, emoji, emphasis marks and brackets as spaces
// ("aku pengen😭mati", "I want to *die*", "aku pengen (mati)"), drops the
// quotation marks around a single word ("I want to \"die\""), and reads a
// run of two or more dots between two words as a space ("aku pengen...
// mati"). Sentence and clause punctuation, apostrophes, hyphens and the
// quotation marks of longer quotations are kept, so frames and quotations
// keep their meaning.
//
// Symbols are read first, so a pause that an emoji follows is still seen
// between two words ("aku pengen... 😭 mati"). A pause is a run of dots and
// spaces with two or more dots ("...", ". . .") or a dash with spaces on both
// sides (" - ", " -- "; em dashes are folded to '-').
func symbolGaps(s string) string {
	s = symbolSpaces(s)
	var b strings.Builder
	changed := false
	for i := 0; i < len(s); {
		c := s[i]
		size := 1
		if c == '.' || (c == '-' && i > 0 && s[i-1] == ' ') {
			j, dots, dashes := i, 0, 0
			for j < len(s) && strings.IndexByte(". -", s[j]) >= 0 {
				switch s[j] {
				case '.':
					dots++
				case '-':
					dashes++
				}
				j++
			}
			pause := dots >= 2 || (dots == 0 && dashes > 0 && s[j-1] == ' ')
			if pause && wordBefore(s, i) && wordAfter(s, j) {
				if !changed {
					b.Grow(len(s))
					b.WriteString(s[:i])
					changed = true
				}
				b.WriteByte(' ')
				i = j
				continue
			}
			size = max(1, j-i)
			if c == '-' {
				size = 1
			}
		}
		if changed {
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	if !changed {
		return s
	}
	return strings.TrimSpace(collapseSpaces(b.String()))
}

// symbolSpaces is the first step of symbolGaps: symbols, emoji, emphasis
// marks and brackets become spaces, and so do the quotation marks around a
// single word.
func symbolSpaces(s string) string {
	drop := singleWordQuotes(s)
	var b strings.Builder
	changed := false
	for i := 0; i < len(s); {
		c := s[i]
		size := 1
		gap := false
		switch {
		case c >= utf8.RuneSelf:
			r, sz := utf8.DecodeRuneInString(s[i:])
			size = sz
			gap = !unicode.IsLetter(r) && !unicode.IsDigit(r)
		case strings.IndexByte("*_~#^=|/\\()[]{}<>@$%&+", c) >= 0:
			gap = true
		case c == '"' || c == '\'':
			gap = drop[i]
		}
		switch {
		case gap:
			if !changed {
				b.Grow(len(s))
				b.WriteString(s[:i])
				changed = true
			}
			b.WriteByte(' ')
		case changed:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	if !changed {
		return s
	}
	return collapseSpaces(b.String())
}

// wordBefore reports whether a letter precedes s[:i], past spaces.
func wordBefore(s string, i int) bool {
	for i > 0 && s[i-1] == ' ' {
		i--
	}
	return i > 0 && isLetter(s[i-1])
}

// wordAfter reports whether a letter follows s[j:], past spaces.
func wordAfter(s string, j int) bool {
	for j < len(s) && s[j] == ' ' {
		j++
	}
	return j < len(s) && isLetter(s[j])
}

// singleWordQuotes returns the positions of quotation marks that enclose a
// single word ("\"die\"", "'die'").
func singleWordQuotes(s string) map[int]bool {
	var drop map[int]bool
	for i := 0; i < len(s); i++ {
		q := s[i]
		if q != '"' && q != '\'' {
			continue
		}
		if q == '\'' && i > 0 && isAlnum(s[i-1]) {
			continue // an apostrophe ("don't")
		}
		j := i + 1
		for j < len(s) && (isLetter(s[j]) || s[j] == '-' || (q == '"' && s[j] == '\'')) {
			j++
		}
		if j == i+1 || j == len(s) || s[j] != q || (q == '\'' && j+1 < len(s) && isAlnum(s[j+1])) {
			continue
		}
		if drop == nil {
			drop = map[int]bool{}
		}
		drop[i], drop[j] = true, true
		i = j
	}
	return drop
}

// spellingWords spell out contractions and common misspellings.
var spellingWords = map[string]string{
	"wanna": "want to", "gonna": "going to", "gotta": "got to",
	"myslef": "myself", "mysefl": "myself", "mysef": "myself", "myselg": "myself",
	"sucidal": "suicidal", "suicidial": "suicidal", "suicdal": "suicidal", "suisidal": "suicidal",
	"sucidial": "suicidal", "suicidel": "suicidal", "suicidle": "suicidal",
	"sucide": "suicide", "suiside": "suicide", "suicde": "suicide", "sucicide": "suicide",
	"wana": "want to", "wantto": "want to", "myslf": "myself", "meself": "myself", "mysel": "myself",
	"pngen": "pengen", "pengn": "pengen", "pegen": "pengen",
}

var (
	mySelfRe = regexp.MustCompile(`\bmy\s+s(?:elf|lef)\b`)
	driRe    = regexp.MustCompile(`\b(bunuh|gantung)\s+dri\b`)
)

// spelling applies spellingWords and the phrase spellings "my self" →
// "myself" and "bunuh dri"/"gantung dri" → "... diri".
func spelling(s string) string {
	if strings.Contains(s, "my s") {
		s = mySelfRe.ReplaceAllString(s, "myself")
	}
	if strings.Contains(s, " dri") {
		s = driRe.ReplaceAllString(s, "$1 diri")
	}
	var b strings.Builder
	changed := false
	for i := 0; i < len(s); {
		if !isLetter(s[i]) || (i > 0 && (isAlnum(s[i-1]) || s[i-1] == '\'')) {
			if changed {
				b.WriteByte(s[i])
			}
			i++
			continue
		}
		j := i
		for j < len(s) && isLetter(s[j]) {
			j++
		}
		repl, ok := spellingWords[s[i:j]]
		if ok && (j == len(s) || (!isAlnum(s[j]) && s[j] != '\'')) {
			if !changed {
				b.Grow(len(s) + 8)
				b.WriteString(s[:i])
				changed = true
			}
			b.WriteString(repl)
		} else if changed {
			b.WriteString(s[i:j])
		}
		i = j
	}
	if !changed {
		return s
	}
	return b.String()
}

// twoAsTo reads a free-standing "2" between two words as "to" ("i want 2
// die").
func twoAsTo(s string) string {
	if !strings.Contains(s, " 2 ") {
		return s
	}
	var b strings.Builder
	last := 0
	for i := 1; i+2 < len(s); i++ {
		if s[i] == '2' && s[i-1] == ' ' && s[i+1] == ' ' && wordBefore(s, i) && isLetter(s[i+2]) {
			b.WriteString(s[last:i])
			b.WriteString("to")
			last = i + 1
		}
	}
	if last == 0 {
		return s
	}
	b.WriteString(s[last:])
	return b.String()
}

// oneAsPronoun replaces every free-standing "1" (no letter, digit or number
// punctuation next to it) with "i", unless a unit follows ("1 kms", "1 jam").
func oneAsPronoun(s string) string {
	if !strings.Contains(s, "1") {
		return s
	}
	var b []byte
	for i := 0; i < len(s); i++ {
		if s[i] != '1' {
			continue
		}
		if i > 0 && (isAlnum(s[i-1]) || strings.IndexByte(".,:/+-", s[i-1]) >= 0) {
			continue
		}
		if i+1 < len(s) && (isAlnum(s[i+1]) || ((s[i+1] == '.' || s[i+1] == ',') && i+2 < len(s) && isDigit(s[i+2]))) {
			continue
		}
		if numberUnits[nextWord(s, i+1)] {
			continue
		}
		if b == nil {
			b = []byte(s)
		}
		b[i] = 'i'
	}
	if b == nil {
		return s
	}
	return string(b)
}

// nextWord returns the letters of the word that starts after the spaces at
// s[i:].
func nextWord(s string, i int) string {
	for i < len(s) && s[i] == ' ' {
		i++
	}
	j := i
	for j < len(s) && (isLetter(s[j]) || s[j] == '%') {
		j++
	}
	return s[i:j]
}

// numberUnits are words that mark a preceding "1" as a number.
var numberUnits = setOf("km kms kg gr g m cm mm l ml jam menit mnt detik hari hr minggu mgg bulan bln tahun thn " +
	"x kali orang org buah biji lembar botol butir tablet pil hour hours hrs min mins minute minutes sec secs " +
	"second seconds day days week weeks month months year years yr yrs time times pm am k rb ribu juta jt % " +
	"st nd rd th of out")

// prepareAlt is the alternative leetspeak reading of text (see variants), or
// "" when text has none of the characters it reads differently.
func prepareAlt(text string) string {
	if !strings.ContainsAny(text, "@$!|+17I") {
		return ""
	}
	return leetAlt(mapRunesMode(text, true))
}

// leetAlt applies the alternative leetspeak reading inside every word that
// has a letter (a word here is a run of letters, digits and @ $ ! | +):
// 1 and | read as l, 7 and + as t, @ as a, $ as s, 0 3 4 5 8 as in leetWords,
// and ! as i when it sits between two other characters of the word ("d!r!",
// "k!ll"), or anywhere in a word with another substitution ("m@t!"). A word
// whose only symbol is a trailing "!" ("mati!") keeps it.
func leetAlt(s string) string {
	b := []byte(s)
	for i := 0; i < len(b); {
		if !isAltWordByte(b[i]) {
			i++
			continue
		}
		j := i
		letters, subs := 0, 0
		for j < len(b) && isAltWordByte(b[j]) {
			switch c := b[j]; {
			case isLetter(c):
				letters++
			case c == '!':
				if j > i && j+1 < len(b) && isAltWordByte(b[j+1]) && b[j-1] != '!' && b[j+1] != '!' {
					subs++
				}
			default:
				subs++
			}
			j++
		}
		if letters > 0 {
			var ambiguous []int // the 1s and |s, read as l by default
			trailing := -1      // a final "!" read as i
			for k := i; k < j; k++ {
				switch c := b[k]; {
				case c == '!':
					interior := k > i && k+1 < j && b[k-1] != '!' && b[k+1] != '!'
					if interior || subs > 0 && (k == j-1 || b[k+1] != '!') && (k == i || b[k-1] != '!') {
						b[k] = 'i'
						if k == j-1 && !interior {
							trailing = k
						}
					}
				case !isLetter(c):
					if c == '1' || c == '|' {
						ambiguous = append(ambiguous, k)
					}
					b[k] = altLeet(c)
				}
			}
			e := j // the word proper ends before a run of final "!" kept as marks
			for e > i && b[e-1] == '!' {
				e--
			}
			resolveAmbiguous(b[i:e], ambiguous, i)
			switch {
			case trailing > i && !vocabulary[resolveWord(string(b[i:e]))]:
				// The final "!" may be punctuation after a whole word
				// ("$u1c1d3!"): keep it when the word before it is one.
				b[trailing] = '!'
				resolveAmbiguous(b[i:trailing], ambiguous, i)
				if !vocabulary[resolveWord(string(b[i:trailing]))] {
					b[trailing] = 'i'
					resolveAmbiguous(b[i:e], ambiguous, i)
				}
			case e > i && e < j && !vocabulary[resolveWord(string(b[i:e]))]:
				// A run of final marks may begin with the i it imitates
				// ("m@t!!").
				b[e] = 'i'
				resolveAmbiguous(b[i:e+1], ambiguous, i)
				if !vocabulary[resolveWord(string(b[i:e+1]))] {
					b[e] = '!'
					resolveAmbiguous(b[i:e], ambiguous, i)
				}
			}
		}
		i = j
	}
	return string(b)
}

// maxAmbiguous bounds the 1s and |s of one word resolveAmbiguous tries
// combinations of.
const maxAmbiguous = 6

// resolveAmbiguous reads each 1 or | of the word w (at the offsets pos, minus
// base, already read as l) as i or l, keeping the first combination whose
// spelling is a vocabulary word ("k1ll" → "kill", "$u1c1d3" → "suicide",
// "m@t1" → "mati"), or l everywhere when none is.
func resolveAmbiguous(w []byte, pos []int, base int) {
	if len(pos) == 0 || len(pos) > maxAmbiguous || vocabulary[resolveWord(string(w))] {
		return
	}
	for mask := 1; mask < 1<<len(pos); mask++ {
		for n, p := range pos {
			w[p-base] = 'l'
			if mask&(1<<n) != 0 {
				w[p-base] = 'i'
			}
		}
		if vocabulary[resolveWord(string(w))] {
			return
		}
	}
	for _, p := range pos {
		w[p-base] = 'l'
	}
}

func isAltWordByte(c byte) bool {
	return isLetter(c) || isDigit(c) || c == '@' || c == '$' || c == '!' || c == '|' || c == '+'
}

func altLeet(c byte) byte {
	switch c {
	case '1', '|':
		return 'l'
	case '7', '+':
		return 't'
	case '@':
		return 'a'
	case '$':
		return 's'
	}
	return byte(leet(rune(c)))
}

// vocabulary holds every word the catalogue and the context expressions
// spell out (plus a few common chat words). The lexical variant resolves
// elongation and splits fused words against it.
var vocabulary, maxVocabWord = buildVocabulary()

// extraVocabulary lists words that no expression spells but that are often
// elongated or fused next to risk words.
const extraVocabulary = "too all so see need feel still will well off good soon sorry please really just " +
	"aja deh sih dong lah ya banget bgt udah lagi nih tuh kok gitu gini aku kamu dia"

func buildVocabulary() (map[string]bool, int) {
	v := map[string]bool{}
	longest := 0
	addText := func(s string) {
		for _, w := range strings.FieldsFunc(s, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r == '\'') }) {
			w = strings.Trim(w, "'")
			if w != "" {
				v[w] = true
				longest = max(longest, len(w))
			}
		}
	}
	addExpr := func(expr string) {
		re, err := syntax.Parse(expr, syntax.Perl)
		if err != nil {
			panic("safety: invalid expression " + expr + ": " + err.Error())
		}
		collectWords(re.Simplify(), addText)
	}
	for _, r := range catalogue {
		for _, p := range r.patterns {
			addExpr(p.re.String())
		}
	}
	for _, re := range []*regexp.Regexp{denialRe.re, adoptionRe.re, sharedRe, firstPersonRe, breakerRe, thirdPronounRe, ownFrameRe} {
		addExpr(re.String())
	}
	for _, g := range frameRe.alts {
		addExpr(g.re.String())
	}
	addText(extraVocabulary)
	return v, longest
}

// maxVocabSet bounds the string sets collectWords builds.
const maxVocabSet = 64

// collectWords passes to add the literal text of every run of re that can
// only match a few known strings (each alternative of "kill(?:ing)?" in
// "kill(?:ing)?\s*myself", then "myself").
func collectWords(re *syntax.Regexp, add func(string)) {
	if set, ok := exactStrings(re); ok {
		for _, s := range set {
			add(s)
		}
		return
	}
	if re.Op != syntax.OpConcat {
		for _, sub := range re.Sub {
			collectWords(sub, add)
		}
		return
	}
	collectSeq(re.Sub, []string{""}, add)
}

// collectSeq is collectWords for the concatenation subs after the exact
// prefix cur. The parser factors common prefixes out of alternations
// ("bunuh\s*diri|bundir" becomes "bun(?:uh\s*diri|dir)"), so an exact prefix
// is joined to each branch of the alternation that follows it again.
func collectSeq(subs []*syntax.Regexp, cur []string, add func(string)) {
	flush := func() {
		for _, s := range cur {
			add(s)
		}
		cur = []string{""}
	}
	for _, sub := range subs {
		set, ok := exactStrings(sub)
		switch {
		case ok && len(cur)*len(set) <= maxVocabSet:
			cur = cross(cur, set)
			continue
		case ok:
			flush()
			cur = set
			continue
		}
		if alt := unwrapCapture(sub); alt.Op == syntax.OpAlternate && hasNonEmpty(cur) {
			for _, b := range alt.Sub {
				parts := []*syntax.Regexp{b}
				if b.Op == syntax.OpConcat {
					parts = b.Sub
				}
				collectSeq(parts, cur, add)
			}
			cur = []string{""}
			continue
		}
		flush()
		collectWords(sub, add)
	}
	flush()
}

// exactStrings returns every string re can match when that is a small
// finite set; a class of spaces stands for one space.
func exactStrings(re *syntax.Regexp) ([]string, bool) {
	switch re.Op {
	case syntax.OpLiteral:
		if re.Flags&syntax.FoldCase != 0 {
			return nil, false
		}
		return []string{string(re.Rune)}, true
	case syntax.OpCharClass:
		var set []string
		spaces := true
		for i := 0; i+1 < len(re.Rune); i += 2 {
			for r := re.Rune[i]; r <= re.Rune[i+1]; r++ {
				spaces = spaces && unicode.IsSpace(r)
				if len(set) == 8 {
					return nil, false
				}
				set = append(set, string(r))
			}
		}
		if spaces && len(set) > 0 {
			return []string{" "}, true
		}
		return set, len(set) > 0
	case syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText,
		syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return []string{""}, true
	case syntax.OpCapture:
		return exactStrings(re.Sub[0])
	case syntax.OpQuest:
		sub, ok := exactStrings(re.Sub[0])
		if !ok || len(sub) >= maxVocabSet {
			return nil, false
		}
		return append(sub, ""), true
	case syntax.OpAlternate:
		var out []string
		for _, sub := range re.Sub {
			set, ok := exactStrings(sub)
			if !ok || len(out)+len(set) > maxVocabSet {
				return nil, false
			}
			out = append(out, set...)
		}
		return out, true
	case syntax.OpConcat:
		out := []string{""}
		for _, sub := range re.Sub {
			set, ok := exactStrings(sub)
			if !ok || len(out)*len(set) > maxVocabSet {
				return nil, false
			}
			out = cross(out, set)
		}
		return out, true
	}
	return nil, false
}
