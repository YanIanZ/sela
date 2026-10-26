package safety

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Normalize returns the canonical matching form of text. It ports the v1
// normalizeIndonesianText pipeline and adds a few evasion-resistant steps:
//
//  1. Invisible format characters (zero-width space/joiners, BOM, word
//     joiner, soft hyphen, bidi controls) and combining marks are removed.
//     v1 removed U+200B–U+200D and U+FEFF and stripped U+0300–U+036F after
//     NFKD; Sela removes every Cf, Mn and Me rune.
//  2. Text is lowercased and folded with a small explicit table, because
//     golang.org/x/text is not available: precomposed Latin letters lose their
//     accents (é→e, ñ→n), ligatures expand (ﬁ→fi, æ→ae), fullwidth ASCII,
//     mathematical alphanumerics and circled letters map to plain ASCII, and a
//     handful of Cyrillic/Greek look-alikes map to their Latin twins.
//  3. Curly apostrophes become "'" (v1), curly and angle double quotes
//     become '"', typographic dashes become '-', and every Unicode space
//     becomes ' ' (line separators become '\n').
//  4. Leetspeak digits are replaced inside words: 0→o 1→i 3→e 4→a 5→s 8→b
//     ("b4yg0n"→"baygon", "k1ll"→"kill"). v1 replaced every digit; Sela
//     leaves free-standing numbers alone, so "jam 1" is not read as the
//     pronoun "i" and "5 kms" stays a distance.
//  5. Runs of three or more identical characters collapse to one (v1:
//     "matiiiii"→"mati"), then runs of spaces collapse to one.
//  6. "bunuh diri" written with spaces, dots, dashes or underscores between
//     its letters is rejoined (v1: "b u n u h d i r i", "bu-nuh di-ri").
//  7. Runs of three or more single letters separated by one space, dot,
//     dash or underscore are joined ("k i l l"→"kill", "m.a.t.i"→"mati").
//
// The result is trimmed. Normalize is pure and safe for concurrent use.
// Assess additionally matches two derived variants (see variants), so
// Normalize alone does not describe everything Assess can see.
func Normalize(text string) string {
	return finish(prepare(text), 1)
}

// prepare applies steps 1–4 of Normalize.
func prepare(text string) string {
	if text == "" {
		return ""
	}
	return leetWords(mapRunes(text))
}

// finish applies steps 5–7, collapsing runs of three or more identical runes
// down to keep runes (1 for the canonical form, 2 for the variant that
// preserves genuine double letters such as "kill" in "killll").
func finish(s string, keep int) string {
	s = collapseRuns(s, keep)
	s = collapseSpaces(s)
	s = rejoinBunuhDiri(s)
	s = joinLetterRuns(s)
	return strings.TrimSpace(s)
}

// variants returns the distinct strings Assess evaluates for text. The first
// is always Normalize(text). The others only ever add matches:
//   - compact: hyphens, underscores and dots between two letters removed, so
//     "gan-tung di-ri" and "mau-mati" are seen as "gantung diri" and
//     "maumati" (patterns already allow optional spaces between words);
//   - doubled: elongation collapsed to two runes instead of one, so
//     "killll myself" is seen as "kill myself" rather than "kil myself".
func variants(text string) []string {
	base := prepare(text)
	primary := finish(base, 1)
	out := []string{primary}
	if c := compactJoin(primary); c != primary {
		out = append(out, c)
	}
	if hasRun3(base) {
		if d := finish(base, 2); d != primary {
			out = append(out, d)
		}
	}
	return out
}

// rejoinBunuhDiri is the v1 rejoin of "bunuh diri" written with separators
// between its letters, v1's
// /\bb[\s._-]*u[\s._-]*n[\s._-]*u[\s._-]*h[\s._-]*d[\s._-]*i[\s._-]*r[\s._-]*i\b/g
// written as a linear scan.
func rejoinBunuhDiri(s string) string {
	const letters = "bunuhdiri"
	var b strings.Builder
	last := 0
	for i := 0; i < len(s); i++ {
		if s[i] != 'b' || (i > 0 && isWordByte(s[i-1])) {
			continue
		}
		j, k := i, 0
		for k < len(letters) {
			if k > 0 {
				for j < len(s) && isRejoinSep(s[j]) {
					j++
				}
			}
			if j >= len(s) || s[j] != letters[k] {
				break
			}
			j++
			k++
		}
		if k < len(letters) || (j < len(s) && isWordByte(s[j])) {
			continue
		}
		b.WriteString(s[last:i])
		b.WriteString("bunuh diri")
		last = j
		i = j - 1
	}
	if last == 0 {
		return s
	}
	b.WriteString(s[last:])
	return b.String()
}

// isRejoinSep mirrors the v1 class [\s._-] (RE2 \s is [\t\n\f\r ]).
func isRejoinSep(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r' || c == '.' || c == '_' || c == '-'
}

// mapRunes applies steps 1–3 of Normalize rune by rune.
func mapRunes(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); {
		if c := text[i]; c < utf8.RuneSelf {
			switch {
			case c == '\n' || c == '\r' || c == '\v' || c == '\f':
				b.WriteByte('\n')
			case c < 0x20 || c == 0x7f:
				b.WriteByte(' ') // includes \t
			case 'A' <= c && c <= 'Z':
				b.WriteByte(c + 'a' - 'A')
			case c == '`':
				b.WriteByte('\'')
			default:
				b.WriteByte(c)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size
		switch {
		case r == '\n' || r == '\r' || r == '\v' || r == '\f' || r == 0x85 || r == 0x2028 || r == 0x2029:
			b.WriteByte('\n')
			continue
		case unicode.IsSpace(r):
			b.WriteByte(' ')
			continue
		case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
			continue
		case unicode.IsControl(r):
			b.WriteByte(' ')
			continue
		}
		r = unicode.ToLower(r)
		if f, ok := foldRune(r); ok {
			b.WriteString(f)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// leetWords replaces leetspeak digits in every run of digits that touches an
// ASCII letter, and in lone digits that sit inside a spaced-out word.
// Free-standing numbers are kept.
func leetWords(s string) string {
	if strings.IndexAny(s, "013458") < 0 {
		return s
	}
	b := []byte(s)
	for i := 0; i < len(b); {
		if !isDigit(b[i]) {
			i++
			continue
		}
		j := i
		for j < len(b) && isDigit(b[j]) {
			j++
		}
		touches := (i > 0 && isLetter(b[i-1])) || (j < len(b) && isLetter(b[j]))
		// A lone digit inside a spaced-out word ("d 1 r 1") also counts.
		spaced := j == i+1 &&
			((i >= 2 && isLetterSep(b[i-1]) && isSingleLetter(s, i-2)) ||
				(j+1 < len(b) && isLetterSep(b[j]) && isSingleLetter(s, j+1)))
		if touches || spaced {
			for k := i; k < j; k++ {
				b[k] = byte(leet(rune(b[k])))
			}
		}
		i = j
	}
	return string(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// leet maps the v1 leetspeak digits to letters.
func leet(r rune) rune {
	switch r {
	case '0':
		return 'o'
	case '1':
		return 'i'
	case '3':
		return 'e'
	case '4':
		return 'a'
	case '5':
		return 's'
	case '8':
		return 'b'
	}
	return r
}

// foldRune returns the ASCII replacement for r, if any. r is already
// lowercased.
func foldRune(r rune) (string, bool) {
	switch {
	case r < 0x80:
		return "", false
	case r >= 0xFF01 && r <= 0xFF5E: // fullwidth ASCII
		return string(unicode.ToLower(r - 0xFF01 + 0x21)), true
	case r >= 0x1D400 && r <= 0x1D6A3: // mathematical alphanumeric letters
		off := (r - 0x1D400) % 52
		if off >= 26 {
			off -= 26
		}
		return string('a' + off), true
	case r >= 0x1D7CE && r <= 0x1D7FF: // mathematical digits
		return string('0' + (r-0x1D7CE)%10), true
	case r >= 0x24D0 && r <= 0x24E9: // circled small letters (ToLower maps Ⓐ here)
		return string('a' + r - 0x24D0), true
	case r >= 0x249C && r <= 0x24B5: // parenthesized small letters
		return string('a' + r - 0x249C), true
	case r >= 0x1F130 && r <= 0x1F149: // squared capital letters
		return string('a' + r - 0x1F130), true
	case r >= 0x1F150 && r <= 0x1F169: // negative circled capital letters
		return string('a' + r - 0x1F150), true
	case r >= 0x1F170 && r <= 0x1F189: // negative squared capital letters
		return string('a' + r - 0x1F170), true
	}
	f, ok := foldTable[r]
	return f, ok
}

// foldTable is the explicit replacement table used instead of NFKD.
var foldTable = buildFoldTable()

func buildFoldTable() map[rune]string {
	m := make(map[rune]string, 512)
	add := func(to, from string) {
		for _, r := range from {
			m[r] = to
		}
	}
	// Precomposed Latin letters (Latin-1, Extended-A/B, Extended Additional).
	add("a", "àáâãäåāăąǎǟǡǻȁȃȧạảấầẩẫậắằẳẵặḁẚɑ")
	add("b", "ḃḅḇ")
	add("c", "çćĉċčḉ")
	add("d", "ďđḋḍḏḑḓð")
	add("e", "èéêëēĕėęěȅȇȩẹẻẽếềểễệḕḗḙḛḝ")
	add("f", "ḟ")
	add("g", "ĝğġģǧǵḡɡ")
	add("h", "ĥħȟḣḥḧḩḫẖ")
	add("i", "ìíîïĩīĭįıǐȉȋỉịḭḯ")
	add("j", "ĵǰ")
	add("k", "ķǩḱḳḵ")
	add("l", "ĺļľŀłḷḹḻḽ")
	add("m", "ḿṁṃ")
	add("n", "ñńņňŉǹṅṇṉṋ")
	add("o", "òóôõöøōŏőơǒǫǭǿȍȏȫȭȯȱọỏốồổỗộớờởỡợṍṏṑṓ")
	add("p", "ṕṗ")
	add("r", "ŕŗřȑȓṙṛṝṟ")
	add("s", "śŝşšșṡṣṥṧṩſ")
	add("t", "ţťŧțṫṭṯṱẗ")
	add("u", "ùúûüũūŭůűųưǔǖǘǚǜȕȗụủứừửữựṳṵṷṹṻ")
	add("v", "ṽṿ")
	add("w", "ŵẁẃẅẇẉẘ")
	add("x", "ẋẍ")
	add("y", "ýÿŷȳẏẙỳỵỷỹ")
	add("z", "źżžẑẓẕ")
	// Ligatures and digraphs.
	m['æ'] = "ae"
	m['œ'] = "oe"
	m['ß'] = "ss"
	m['ĳ'] = "ij"
	m['ǆ'] = "dz"
	m['ǅ'] = "dz"
	m['ǉ'] = "lj"
	m['ǈ'] = "lj"
	m['ǌ'] = "nj"
	m['ǋ'] = "nj"
	m['ﬀ'] = "ff"
	m['ﬁ'] = "fi"
	m['ﬂ'] = "fl"
	m['ﬃ'] = "ffi"
	m['ﬄ'] = "ffl"
	m['ﬅ'] = "st"
	m['ﬆ'] = "st"
	// Letterlike symbols that fill holes in the mathematical alphabets.
	m['ℎ'] = "h"
	m['ℓ'] = "l"
	m['ℯ'] = "e"
	m['ℊ'] = "g"
	m['ℴ'] = "o"
	// Cyrillic and Greek look-alikes (lowercase forms; input is lowercased).
	add("a", "аα")
	add("c", "сϲ")
	add("d", "ԁ")
	add("e", "еёε")
	add("h", "һ")
	add("i", "іїι")
	add("j", "ј")
	add("k", "кκ")
	add("l", "ӏ")
	add("o", "оο")
	add("p", "рρ")
	add("q", "ԛ")
	add("s", "ѕ")
	add("t", "τ")
	add("u", "υ")
	add("v", "ν")
	add("w", "ԝ")
	add("x", "хχ")
	add("y", "у")
	// Punctuation.
	add("'", "\u2018\u2019\u201bʼ\u2032`\u00b4")
	add("\"", "\u201c\u201d\u201e\u201f\u00ab\u00bb\u2033\u2036\u301d\u301e\u300c\u300d")
	add("-", "\u2010\u2011\u2012\u2013\u2014\u2015\u2212\ufe58\ufe63")
	m['\u2026'] = "..."
	m['\u2025'] = ".."
	m['\u2024'] = "."
	m['\u3002'] = "."
	return m
}

// collapseRuns shortens every run of three or more identical runes to keep
// runes (v1: /(.)\1{2,}/g → "$1").
func collapseRuns(s string, keep int) string {
	if !hasRun3(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
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
		if n >= 3 {
			n = keep
		}
		for k := 0; k < n; k++ {
			b.WriteString(s[i : i+size])
		}
		i = j
	}
	return b.String()
}

// hasRun3 reports whether s contains three identical runes in a row.
func hasRun3(s string) bool {
	var p1, p2 rune = -1, -1
	for _, r := range s {
		if r == p1 && r == p2 {
			return true
		}
		p2, p1 = p1, r
	}
	return false
}

// collapseSpaces turns every run of ' ' into a single space.
func collapseSpaces(s string) string {
	if !strings.Contains(s, "  ") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	prev := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' && prev == ' ' {
			continue
		}
		b.WriteByte(c)
		prev = c
	}
	return b.String()
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' }

func isAlnum(c byte) bool { return isLetter(c) || isDigit(c) }

// isWordByte mirrors RE2's ASCII \w, which is what \b uses.
func isWordByte(c byte) bool {
	return isAlnum(c) || c == '_' || (c >= 'A' && c <= 'Z')
}

func isLetterSep(c byte) bool { return c == ' ' || c == '.' || c == '_' || c == '-' }

// isSingleLetter reports whether s[i] is an ASCII letter with no letter or
// digit directly before or after it.
func isSingleLetter(s string, i int) bool {
	if i < 0 || i >= len(s) || !isLetter(s[i]) {
		return false
	}
	if i > 0 && isAlnum(s[i-1]) {
		return false
	}
	return i+1 == len(s) || !isAlnum(s[i+1])
}

// joinLetterRuns joins runs of three or more single letters separated by
// exactly one space, dot, dash or underscore ("k i l l" → "kill").
func joinLetterRuns(s string) string {
	var b strings.Builder
	changed := false
	for i := 0; i < len(s); {
		if isSingleLetter(s, i) {
			j, n := i+1, 1
			for j+1 < len(s) && isLetterSep(s[j]) && isSingleLetter(s, j+1) {
				j += 2
				n++
			}
			if n >= 3 {
				if !changed {
					b.Grow(len(s))
					b.WriteString(s[:i])
					changed = true
				}
				for k := i; k < j; k += 2 {
					b.WriteByte(s[k])
				}
				i = j
				continue
			}
		}
		if changed {
			b.WriteByte(s[i])
		}
		i++
	}
	if !changed {
		return s
	}
	return b.String()
}

// compactJoin removes '-', '_' and '.' when they sit between two letters.
func compactJoin(s string) string {
	var b strings.Builder
	changed := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c == '-' || c == '_' || c == '.') && i > 0 && i+1 < len(s) && isLetter(s[i-1]) && isLetter(s[i+1]) {
			if !changed {
				b.Grow(len(s))
				b.WriteString(s[:i])
				changed = true
			}
			continue
		}
		if changed {
			b.WriteByte(c)
		}
	}
	if !changed {
		return s
	}
	return b.String()
}
