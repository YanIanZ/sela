// Package textnorm is the one text normalisation that Sela's input
// redaction (internal/ai/minimize) and output guards (internal/ai/guard)
// share, so a name or contact detail that the output guard recognises is
// also recognised, and redacted, before any provider call.
//
// Normalisation maps fullwidth ASCII to ASCII, folds accented letters and
// Cyrillic and Greek homoglyphs to ASCII, drops zero-width and other
// format characters and combining marks, and removes apostrophes. A View
// additionally maps Unicode spaces to ' ', Unicode dashes to '-' and middle
// dots and bullets to '.', and keeps a byte-offset map back to the
// original text, so a match found in
// the normalised view replaces the original bytes. A NameView keeps
// apostrophes (as ASCII '), so a name followed by a possessive ("Raka's",
// "Raka’s") still ends at a word boundary; FindName runs on it.
//
// The standard library has no NFKC; the fold table covers the Latin-1 and
// Latin Extended-A letters seen in Indonesian and English text. Nothing
// here logs.
package textnorm

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// foldTable maps look-alike and accented lowercase letters to ASCII.
var foldTable = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a', 'ā': 'a', 'ă': 'a', 'ą': 'a',
	'ç': 'c', 'ć': 'c', 'č': 'c', 'ď': 'd', 'đ': 'd',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e', 'ē': 'e', 'ė': 'e', 'ę': 'e', 'ě': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i', 'ī': 'i', 'į': 'i', 'ı': 'i',
	'ñ': 'n', 'ń': 'n', 'ň': 'n',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'ø': 'o', 'ō': 'o', 'ő': 'o',
	'ŕ': 'r', 'ř': 'r', 'ś': 's', 'š': 's', 'ş': 's', 'ť': 't', 'ţ': 't',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u', 'ū': 'u', 'ů': 'u', 'ű': 'u', 'ų': 'u',
	'ý': 'y', 'ÿ': 'y', 'ź': 'z', 'ż': 'z', 'ž': 'z', 'ß': 's',
	// Cyrillic homoglyphs.
	'а': 'a', 'в': 'b', 'е': 'e', 'ё': 'e', 'к': 'k', 'м': 'm', 'н': 'h', 'о': 'o', 'р': 'p',
	'с': 'c', 'т': 't', 'у': 'y', 'х': 'x', 'і': 'i', 'ј': 'j', 'ѕ': 's', 'ԁ': 'd',
	// Greek homoglyphs.
	'α': 'a', 'ε': 'e', 'ι': 'i', 'κ': 'k', 'ν': 'v', 'ο': 'o', 'ρ': 'p', 'τ': 't', 'υ': 'u', 'χ': 'x',
}

// IsApostrophe reports an apostrophe or a character used as one.
func IsApostrophe(r rune) bool {
	return r == '\'' || r == '’' || r == '‘' || r == '`' || r == 'ʼ' || r == '´' || r == '＇'
}

// IsDash reports a Unicode dash or minus sign (mapped to '-' in a View).
func IsDash(r rune) bool {
	switch r {
	case '‐', '‑', '‒', '–', '—', '―', '−', '﹘', '﹣', '⁃':
		return true
	}
	return false
}

// IsMiddleDot reports a middle dot or bullet used as a separator (mapped
// to '.' in a View): "0812·3456·7890" is one phone number.
func IsMiddleDot(r rune) bool {
	switch r {
	case '·', '•', '∙', '・', '･', '‧', '⋅':
		return true
	}
	return false
}

// IsSpace reports a Unicode space other than a line break (mapped to ' '
// in a View): no-break, en/em, thin, narrow and ideographic spaces.
func IsSpace(r rune) bool {
	return r != '\n' && r != '\r' && (unicode.Is(unicode.Zs, r) || r == '\t' || r == '\v' || r == '\f')
}

// mapRune returns the normalised form of r and false when r is dropped.
// lower selects lowercasing; otherwise the case of r is kept.
func mapRune(r rune, lower bool) (rune, bool) {
	switch {
	case IsApostrophe(r):
		return 0, false
	case r >= 0xFF01 && r <= 0xFF5E:
		r -= 0xFEE0
	case unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Mn, r):
		return 0, false
	}
	lr := unicode.ToLower(r)
	if f, ok := foldTable[lr]; ok {
		if !lower && unicode.IsUpper(r) {
			return unicode.ToUpper(f), true
		}
		return f, true
	}
	if lower {
		return lr, true
	}
	return r, true
}

// Fold lowercases s, maps fullwidth ASCII to ASCII, folds accents and
// homoglyphs, drops zero-width and other format characters and combining
// marks and removes apostrophes ("don't" becomes "dont"). Punctuation,
// spaces and dashes are kept as they are.
func Fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if m, ok := mapRune(r, true); ok {
			b.WriteRune(m)
		}
	}
	return b.String()
}

// View is a normalised view of a text: Fold's mapping without
// lowercasing, plus Unicode spaces mapped to ' ', Unicode dashes to '-'
// and middle dots to '.'. It maps byte offsets of the view back to the
// original text.
type View struct {
	// Text is the normalised text with the original case.
	Text string
	orig string
	// off[i] is the original byte offset of the rune that produced byte i
	// of Text; off[len(Text)] is len(orig).
	off []int
}

// NewView normalises s; apostrophes are dropped, so a contact detail split
// by one ("0812'3456'7890") is still one run.
func NewView(s string) View { return newView(s, false) }

// NewNameView normalises s as NewView does, but keeps every apostrophe
// (as ASCII '): names are matched on it, so "Raka's" and "Raka’s" end the
// name "Raka" at a word boundary instead of becoming the other word
// "Rakas".
func NewNameView(s string) View { return newView(s, true) }

func newView(s string, keepApostrophes bool) View {
	var b strings.Builder
	b.Grow(len(s))
	off := make([]int, 0, len(s)+1)
	for i, r := range s {
		var m rune
		switch {
		case IsSpace(r):
			m = ' '
		case IsDash(r):
			m = '-'
		case IsMiddleDot(r):
			m = '.'
		case keepApostrophes && IsApostrophe(r):
			m = '\''
		default:
			var ok bool
			if m, ok = mapRune(r, false); !ok {
				continue
			}
		}
		n := utf8.RuneLen(m)
		if n < 0 {
			m, n = utf8.RuneError, 3
		}
		b.WriteRune(m)
		for k := 0; k < n; k++ {
			off = append(off, i)
		}
	}
	off = append(off, len(s))
	return View{Text: b.String(), orig: s, off: off}
}

// Orig returns the original byte range of the view range [start, end).
// Characters the view dropped inside the range (zero-width characters)
// are part of the original range.
func (v View) Orig(start, end int) (int, int) {
	if start < 0 {
		start = 0
	}
	if end > len(v.Text) {
		end = len(v.Text)
	}
	if start >= end {
		return v.off[start], v.off[start]
	}
	return v.off[start], v.off[end]
}

// Original returns the original text.
func (v View) Original() string { return v.orig }

// Span is a byte range.
type Span struct{ Start, End int }

// NameSuffixes are what may follow a name in the same word without making
// it another word: the Indonesian enclitics and particles ("Rakanya",
// "Dewiku", "Dewimu", "Rakalah", "Rakapun", "Rakakah", "Rakatah") and the
// reduplication digit ("Dewi2"). An English possessive needs none: the
// apostrophe a NameView keeps ends the word ("Raka's").
var NameSuffixes = []string{"nya", "ku", "mu", "lah", "pun", "kah", "tah", "2"}

// FindName returns the whole-word, case-insensitive occurrences of name in
// text (normalised by NewNameView, or by Fold). The name's words may be
// separated in text by any run of whitespace (spaces, no-break spaces
// after a View, tabs or line breaks), and an apostrophe inside a word is
// skipped ("Ma'ruf" is the name "Maruf"). The word may go on past the name
// with repeats of the name's last letter ("Dewiii", part of the match)
// and, for a name of at least three letters, with up to two NameSuffixes
// ("Rakanya", "Dewiku", "Dewinyalah", "Dewi2"; the suffix is not part of
// the match, so it stays beside a placeholder). A name shorter than two
// letters is ignored.
func FindName(text, name string) []Span {
	toks := strings.Fields(Fold(name))
	letters := utf8.RuneCountInString(strings.Join(toks, ""))
	if len(toks) == 0 || letters < 2 {
		return nil
	}
	last, _ := utf8.DecodeLastRuneInString(toks[len(toks)-1])
	var out []Span
	for at := 0; at < len(text); {
		if end, ok := matchTokens(text, at, toks); ok && wordStart(text, at) {
			end = repeats(text, end, last)
			if wordEnd(text, end, letters >= 3) {
				out = append(out, Span{at, end})
				at = end
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(text[at:])
		at += size
	}
	return out
}

// matchTokens matches toks at text[at:], case-insensitively rune by rune,
// with at least one whitespace rune between tokens; apostrophes inside a
// token are skipped.
func matchTokens(text string, at int, toks []string) (int, bool) {
	i := at
	for k, tok := range toks {
		if k > 0 {
			n := 0
			for i < len(text) {
				r, size := utf8.DecodeRuneInString(text[i:])
				if !unicode.IsSpace(r) {
					break
				}
				i += size
				n++
			}
			if n == 0 {
				return 0, false
			}
		}
		for j, nr := range tok {
			if j > 0 {
				for i < len(text) && text[i] == '\'' {
					i++
				}
			}
			if i >= len(text) {
				return 0, false
			}
			r, size := utf8.DecodeRuneInString(text[i:])
			if m, _ := mapRune(r, true); m != nr {
				return 0, false
			}
			i += size
		}
	}
	return i, true
}

// repeats extends end over repeats of the letter last ("Dewiii").
func repeats(text string, end int, last rune) int {
	if !unicode.IsLetter(last) {
		return end
	}
	for end < len(text) {
		r, size := utf8.DecodeRuneInString(text[end:])
		if m, _ := mapRune(r, true); m != last {
			break
		}
		end += size
	}
	return end
}

func wordStart(text string, at int) bool {
	before, _ := utf8.DecodeLastRuneInString(text[:at])
	return at == 0 || !WordRune(before)
}

// wordEnd reports whether the word ends at end, or, with suffixes, after
// at most two NameSuffixes.
func wordEnd(text string, end int, suffixes bool) bool {
	ends := func(i int) bool {
		r, _ := utf8.DecodeRuneInString(text[i:])
		return i == len(text) || !WordRune(r)
	}
	if ends(end) {
		return true
	}
	if !suffixes {
		return false
	}
	for i, n := end, 0; n < 2; n++ {
		j, ok := suffixAt(text, i)
		if !ok {
			return false
		}
		if ends(j) {
			return true
		}
		i = j
	}
	return false
}

// suffixAt matches one of NameSuffixes at text[i:], case-insensitively.
func suffixAt(text string, i int) (int, bool) {
	for _, s := range NameSuffixes {
		j, ok := i, true
		for _, sr := range s {
			if j >= len(text) {
				ok = false
				break
			}
			r, size := utf8.DecodeRuneInString(text[j:])
			if m, _ := mapRune(r, true); m != sr {
				ok = false
				break
			}
			j += size
		}
		if ok {
			return j, true
		}
	}
	return 0, false
}

// WordRune reports a letter or digit.
func WordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
