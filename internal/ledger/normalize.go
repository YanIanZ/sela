package ledger

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// foldedRune is one rune of folded text together with the byte span
// [start, end) of the original rune it came from. A rune that folds into
// several runes (an ellipsis, a ligature) gives each of them the same span,
// so any match maps back to whole original runes.
type foldedRune struct {
	r          rune
	start, end int
}

// latin1Fold maps U+00C0..U+00FF to an unaccented lowercase base letter;
// 0 keeps the rune (lowercased).
var latin1Fold = [64]rune{
	'a', 'a', 'a', 'a', 'a', 'a', 0, 'c', // À Á Â Ã Ä Å Æ Ç
	'e', 'e', 'e', 'e', 'i', 'i', 'i', 'i', // È É Ê Ë Ì Í Î Ï
	'd', 'n', 'o', 'o', 'o', 'o', 'o', 0, // Ð Ñ Ò Ó Ô Õ Ö ×
	'o', 'u', 'u', 'u', 'u', 'y', 0, 0, // Ø Ù Ú Û Ü Ý Þ ß
	'a', 'a', 'a', 'a', 'a', 'a', 0, 'c', // à á â ã ä å æ ç
	'e', 'e', 'e', 'e', 'i', 'i', 'i', 'i', // è é ê ë ì í î ï
	'd', 'n', 'o', 'o', 'o', 'o', 'o', 0, // ð ñ ò ó ô õ ö ÷
	'o', 'u', 'u', 'u', 'u', 'y', 0, 'y', // ø ù ú û ü ý þ ÿ
}

// foldRune appends the folded form of r to dst.
//
// The standard library has no Unicode normalisation tables, so this is an
// NFKC-like fold covering what matters for matching user text: full-width
// forms, compatibility spaces, ellipsis and ligatures, typographic quotes and
// hyphens, superscript digits, format characters (zero-width, soft hyphen,
// BOM, bidi controls) and combining marks are removed, Latin-1 accented
// letters lose their accent (so precomposed and combining spellings compare
// equal), and everything is lowercased.
func foldRune(dst []rune, r rune) []rune {
	if r < 0x80 { // ASCII fast path
		if 'A' <= r && r <= 'Z' {
			r += 'a' - 'A'
		}
		return append(dst, r)
	}
	switch r {
	case utf8.RuneError:
		return append(dst, r)
	case '…': // …
		return append(dst, '.', '.', '.')
	case '‘', '’', '‚', '‛', 'ʼ':
		return append(dst, '\'')
	case '“', '”', '„', '‟':
		return append(dst, '"')
	case '‐', '‑':
		return append(dst, '-')
	case 'ﬀ':
		return append(dst, 'f', 'f')
	case 'ﬁ':
		return append(dst, 'f', 'i')
	case 'ﬂ':
		return append(dst, 'f', 'l')
	case 'ﬃ':
		return append(dst, 'f', 'f', 'i')
	case 'ﬄ':
		return append(dst, 'f', 'f', 'l')
	case 'ﬅ', 'ﬆ':
		return append(dst, 's', 't')
	case '²':
		return append(dst, '2')
	case '³':
		return append(dst, '3')
	case '¹':
		return append(dst, '1')
	case '　':
		return append(dst, ' ')
	}
	switch {
	case r >= 0xFF01 && r <= 0xFF5E: // full-width ASCII
		return append(dst, unicode.ToLower(r-0xFEE0))
	case r >= 0xC0 && r <= 0xFF:
		if f := latin1Fold[r-0xC0]; f != 0 {
			return append(dst, f)
		}
		return append(dst, unicode.ToLower(r))
	case unicode.Is(unicode.Cf, r), unicode.Is(unicode.Mn, r):
		return dst
	case unicode.Is(unicode.Zs, r):
		return append(dst, ' ')
	}
	return append(dst, unicode.ToLower(r))
}

// foldSpans folds s rune by rune, keeping the original byte span of every
// output rune. Invalid UTF-8 bytes become U+FFFD with a one-byte span.
func foldSpans(s string) []foldedRune {
	out := make([]foldedRune, 0, len(s))
	var buf [4]rune
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		for _, f := range foldRune(buf[:0], r) {
			out = append(out, foldedRune{r: f, start: i, end: i + size})
		}
		i += size
	}
	return out
}

// groundForm is text prepared for quote matching: folded, whitespace runs
// collapsed to one space, and trimmed. offs[i] is the byte offset in text of
// runes[i]; runes keep their original spans.
type groundForm struct {
	text  string
	offs  []int
	runes []foldedRune
}

func newGroundForm(s string) groundForm {
	folded := foldSpans(s)
	runes := make([]foldedRune, 0, len(folded))
	pending := false
	var space foldedRune
	for _, f := range folded {
		if unicode.IsSpace(f.r) {
			if len(runes) == 0 {
				continue // leading whitespace
			}
			if !pending {
				pending = true
				space = foldedRune{r: ' ', start: f.start}
			}
			space.end = f.end
			continue
		}
		if pending {
			runes = append(runes, space)
			pending = false
		}
		runes = append(runes, f)
	}
	var b strings.Builder
	b.Grow(len(runes))
	offs := make([]int, len(runes))
	for i, f := range runes {
		offs[i] = b.Len()
		b.WriteRune(f.r)
	}
	return groundForm{text: b.String(), offs: offs, runes: runes}
}

// normalizeForGrounding mirrors v1 normalizeForGrounding: case, width, quote
// style and whitespace differences never decide grounding.
func normalizeForGrounding(s string) string { return newGroundForm(s).text }

// occurrences returns the original byte spans of up to max matches of the
// normalised needle q, in order of appearance.
func (g groundForm) occurrences(q string, max int) [][2]int {
	if q == "" {
		return nil
	}
	var res [][2]int
	for from := 0; len(res) < max && from < len(g.text); {
		k := strings.Index(g.text[from:], q)
		if k < 0 {
			break
		}
		at := from + k
		i := sort.SearchInts(g.offs, at)
		j := sort.SearchInts(g.offs, at+len(q)) - 1
		if i < len(g.offs) && g.offs[i] == at && j >= i {
			res = append(res, [2]int{g.runes[i].start, g.runes[j].end})
		}
		_, size := utf8.DecodeRuneInString(g.text[at:])
		from = at + size
	}
	return res
}

// runeLen is utf8.RuneCountInString, named for readability.
func runeLen(s string) int { return utf8.RuneCountInString(s) }
