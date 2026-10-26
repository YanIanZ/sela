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
//     mathematical alphanumerics (with the Letterlike Symbols that fill their
//     holes, ℂ ℍ ℕ ℙ ℝ ℐ ℰ), Roman-numeral letters (ⅰ ⅴ ⅼ ⅾ ⅿ), circled
//     letters, small capitals and modifier letters map to plain ASCII, and a
//     handful of Cyrillic/Greek look-alikes map to their Latin twins.
//  3. Curly apostrophes become "'" (v1), curly and angle double quotes
//     become '"', typographic dashes become '-', and every Unicode space
//     becomes ' ' (line separators become '\n').
//  4. Leetspeak digits are replaced inside words: 0→o 1→i 3→e 4→a 5→s 8→b
//     ("b4yg0n"→"baygon", "k1ll"→"kill"). v1 replaced every digit; Sela
//     leaves free-standing numbers alone, so "jam 1" is not read as the
//     pronoun "i" and "5 kms" stays a distance.
//  5. Runs of three or more identical characters other than spaces collapse
//     to one (v1: "matiiiii"→"mati").
//  6. "bunuh diri" written with spaces, dots, dashes or underscores between
//     its letters is rejoined (v1: "b u n u h d i r i", "bu-nuh di-ri").
//  7. Spaced-out words are joined: runs of three or more single letters
//     separated by spaces, dots, dashes or underscores ("k i l l"→"kill",
//     "m.a.t.i"→"mati"). When a run uses two kinds of separator, the more
//     frequent one joins letters and the other separates words ("a k u  m a
//     u  m a t i"→"aku mau mati", "a.k.u p.e.n.g.e.n"→"aku pengen").
//  8. Runs of spaces collapse to one.
//
// The result is trimmed. Normalize is pure and safe for concurrent use.
// Assess additionally matches derived variants (see variants), so Normalize
// alone does not describe everything Assess can see.
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

// finish applies steps 5–8, collapsing runs of three or more identical runes
// down to keep runes (1 for the canonical form, 2 for the variant that
// preserves genuine double letters such as "kill" in "killll").
func finish(s string, keep int) string {
	s = collapseRuns(s, keep)
	s = rejoinBunuhDiri(s)
	s = joinLetterRuns(s)
	s = collapseSpaces(s)
	return strings.TrimSpace(s)
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
func mapRunes(text string) string { return mapRunesMode(text, false) }

// mapRunesMode is mapRunes; with capitalL, a capital I inside a lowercase word
// is read as the l it imitates ("kiII", "suicidaI"), which lowercasing
// would otherwise turn into an i. It is used by the alternative leetspeak
// variant only (see prepareAlt).
func mapRunesMode(text string, capitalL bool) string {
	var b strings.Builder
	b.Grow(len(text))
	lowerBefore := false // the previous ASCII byte was a lowercase letter or a capital I read as l
	for i := 0; i < len(text); {
		if c := text[i]; c < utf8.RuneSelf {
			l := lowerBefore
			lowerBefore = 'a' <= c && c <= 'z'
			switch {
			case c == '\n' || c == '\r' || c == '\v' || c == '\f':
				b.WriteByte('\n')
			case c < 0x20 || c == 0x7f:
				b.WriteByte(' ') // includes \t
			case c == 'I' && capitalL && l:
				b.WriteByte('l')
				lowerBefore = true
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
		lowerBefore = false
		switch {
		case r == '\n' || r == '\r' || r == '\v' || r == '\f' || r == 0x85 || r == 0x2028 || r == 0x2029:
			b.WriteByte('\n')
			continue
		case unicode.IsSpace(r) || isBlankFiller(r):
			b.WriteByte(' ')
			continue
		case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
			continue
		case unicode.IsControl(r):
			b.WriteByte(' ')
			continue
		}
		if f, ok := upperFold[r]; ok {
			b.WriteString(f)
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

// isBlankFiller reports whether r renders as blank space although Unicode
// does not class it as a space: the Hangul fillers and the blank Braille
// pattern, all used to split words invisibly ("bunuhㅤdiri").
func isBlankFiller(r rune) bool {
	return r == 0x115F || r == 0x1160 || r == 0x3164 || r == 0xFFA0 || r == 0x2800
}

// upperFold maps capital Greek letters that look like a Latin capital whose
// lowercase form looks like a different Latin letter (Ν is "N" but ν is "v",
// Η is "H" but η is "n"). It is consulted before lowercasing.
var upperFold = map[rune]string{
	'Β': "b", 'Η': "h", 'Μ': "m", 'Ν': "n", 'Υ': "y", 'Ζ': "z",
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
	// Letterlike Symbols (U+2100–U+214F) that fill the holes of the
	// mathematical alphabets: the double-struck, script and fraktur capitals
	// and a few small letters. They have no lowercase forms, so they are
	// mapped as they are.
	add("b", "ℬ")
	add("c", "ℂℭ")
	add("d", "ⅅⅆ")
	add("e", "ℯℰℇⅇ")
	add("f", "ℱ")
	add("g", "ℊ")
	add("h", "ℋℌℍℎℏ")
	add("i", "ℐℑℹⅈ")
	add("j", "ⅉ")
	add("l", "ℒℓ")
	add("m", "ℳ")
	add("n", "ℕ")
	add("o", "ℴ")
	add("p", "ℙ")
	add("q", "ℚ")
	add("r", "ℛℜℝ")
	add("z", "ℤℨ")
	// Roman-numeral letters (U+2160–U+217F; the capitals lowercase to the
	// small forms first), read as the letters they are written with.
	m['ⅰ'] = "i"
	m['ⅱ'] = "ii"
	m['ⅲ'] = "iii"
	m['ⅳ'] = "iv"
	m['ⅴ'] = "v"
	m['ⅵ'] = "vi"
	m['ⅶ'] = "vii"
	m['ⅷ'] = "viii"
	m['ⅸ'] = "ix"
	m['ⅹ'] = "x"
	m['ⅺ'] = "xi"
	m['ⅻ'] = "xii"
	m['ⅼ'] = "l"
	m['ⅽ'] = "c"
	m['ⅾ'] = "d"
	m['ⅿ'] = "m"
	// Cyrillic and Greek look-alikes (lowercase forms; input is lowercased).
	// в, н, м and т look like small capitals, so their capitals В Н М Т,
	// which look like Latin B H M T, fold the same way.
	add("a", "аα")
	add("b", "вβ")
	add("c", "сϲ")
	add("d", "ԁ")
	add("e", "еёε")
	add("h", "һн")
	add("i", "іїι")
	add("j", "ј")
	add("k", "кκ")
	add("l", "ӏ")
	add("m", "м")
	add("n", "η")
	add("o", "оο")
	add("p", "рρ")
	add("q", "ԛ")
	add("s", "ѕ")
	add("t", "τт")
	add("u", "υμ\u00b5")
	add("v", "ν")
	add("w", "ԝ")
	add("x", "хχ")
	add("y", "у")
	// Small capitals (U+1D00–U+1D22 and the IPA ones), which NFKD leaves
	// alone but which read as plain letters.
	add("a", "ᴀ")
	add("ae", "ᴁ")
	add("b", "ʙᴃ")
	add("c", "ᴄ")
	add("d", "ᴅᴆ")
	add("e", "ᴇ")
	add("f", "ꜰ")
	add("g", "ɢ")
	add("h", "ʜ")
	add("i", "ɪᴉ")
	add("j", "ᴊ")
	add("k", "ᴋ")
	add("l", "ʟᴌ")
	add("m", "ᴍ")
	add("n", "ɴᴎ")
	add("o", "ᴏᴐᴑ")
	add("p", "ᴘ")
	add("q", "ꞯ")
	add("r", "ʀᴙᴚ")
	add("s", "ꜱ")
	add("t", "ᴛ")
	add("u", "ᴜ")
	add("v", "ᴠ")
	add("w", "ᴡ")
	add("y", "ʏ")
	add("z", "ᴢ")
	// Modifier, superscript and subscript letters (NFKD compatibility forms
	// that v1 folded through normalize('NFKD'); "tiny text" generators use
	// them): U+02B0–U+02B8, U+02E1–U+02E3, U+1D2C–U+1D6A, U+1D9B–U+1DBF,
	// U+2071, U+207F, U+2090–U+209C and U+2C7C.
	add("a", "ᴬᵃₐᵄᵅᶛ")
	add("ae", "ᴭ")
	add("b", "ᴮᴯᵇᵝᵦ")
	add("c", "ᶜᶝ")
	add("d", "ᴰᵈᶞᵟ")
	add("e", "ᴱᵉₑᴲᵋᵌᶟ")
	add("f", "ᶠ")
	add("g", "ᴳᵍᶢᵞᵧ")
	add("h", "ʰʱᴴₕᶣ")
	add("i", "ᴵⁱᵢᵎᶤᶥᶦᶧ")
	add("j", "ʲᴶⱼᶡᶨ")
	add("k", "ᴷᵏₖ")
	add("l", "ˡᴸₗᶩᶪᶫ")
	add("m", "ᴹᵐₘᶬᶭᵚ")
	add("n", "ᴺⁿₙᴻᵑᶮᶯᶰ")
	add("o", "ᴼᵒₒᵓᵔᵕᶱ")
	add("ou", "ᴽ")
	add("p", "ᴾᵖₚᵨ")
	add("r", "ʳʴʵʶᴿᵣ")
	add("s", "ˢₛᶳ")
	add("t", "ᵀᵗₜᶵ")
	add("u", "ᵁᵘᵤᵙᶶᶷᶸ")
	add("v", "ᵛᵥᶹᶺⱽ")
	add("w", "ʷᵂ")
	add("x", "ˣₓᵡᵪ")
	add("y", "ʸ")
	add("z", "ᶻᶼᶽ")
	add("e", "ₔᵊ")
	// Superscript and subscript digits (leetspeak then applies to them).
	add("0", "⁰₀")
	add("1", "¹₁")
	add("2", "²₂")
	add("3", "³₃")
	add("4", "⁴₄")
	add("5", "⁵₅")
	add("6", "⁶₆")
	add("7", "⁷₇")
	add("8", "⁸₈")
	add("9", "⁹₉")
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

// collapseRuns shortens every run of three or more identical runes other
// than spaces to keep runes (v1: /(.)\1{2,}/g → "$1"). Space runs are left to
// collapseSpaces, after joinLetterRuns has read them as word breaks.
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
		if n >= 3 && r != ' ' {
			n = keep
		}
		for k := 0; k < n; k++ {
			b.WriteString(s[i : i+size])
		}
		i = j
	}
	return b.String()
}

// hasRun3 reports whether s contains three identical runes other than
// spaces in a row.
func hasRun3(s string) bool {
	var p1, p2 rune = -1, -1
	for _, r := range s {
		if r == p1 && r == p2 && r != ' ' {
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

// joinLetterRuns joins spaced-out words (step 7 of Normalize).
func joinLetterRuns(s string) string { return joinLetters(s, nil) }

// maxLetterSep is the longest separator between two letters of a spaced-out
// run ("a k u   m a u" uses three spaces between words).
const maxLetterSep = 4

// joinLetters joins runs of single letters separated by short runs of
// spaces, dots, dashes or underscores. A run with one kind of separator is
// one word when it has three or more letters ("k i l l"); a run with two or
// more kinds is a phrase whose most frequent separator joins letters and
// whose other separators break words ("i  w a n t  t o"→"i want to",
// "a.k.u p.e.n"→"aku pen"). When seg is set, each joined word is passed
// through it (the lexical variant splits fused words with it).
func joinLetters(s string, seg func(string) string) string {
	var b strings.Builder
	changed := false
	var letters []int
	var seps []string
	for i := 0; i < len(s); {
		if !isSingleLetter(s, i) {
			if changed {
				b.WriteByte(s[i])
			}
			i++
			continue
		}
		letters = append(letters[:0], i)
		seps = seps[:0]
		j := i + 1
		for {
			k := j
			for k < len(s) && k-j < maxLetterSep && isLetterSep(s[k]) {
				k++
			}
			if k == j || !isSingleLetter(s, k) {
				break
			}
			seps = append(seps, s[j:k])
			letters = append(letters, k)
			j = k + 1
		}
		words := groupLetters(s, letters, seps)
		if words == nil {
			if changed {
				b.WriteString(s[i:j])
			}
			i = j
			continue
		}
		if !changed {
			b.Grow(len(s))
			b.WriteString(s[:i])
			changed = true
		}
		for k, w := range words {
			if k > 0 {
				b.WriteByte(' ')
			}
			if seg != nil {
				w = seg(w)
			}
			b.WriteString(w)
		}
		i = j
	}
	if !changed {
		return s
	}
	return b.String()
}

// groupLetters returns the words of a run of single letters at the given
// positions with the given separators between them, or nil when the run is
// left as it is (fewer than three letters).
func groupLetters(s string, letters []int, seps []string) []string {
	if len(letters) < 3 {
		return nil
	}
	count := map[string]int{}
	for _, sep := range seps {
		count[sep]++
	}
	intra := seps[0]
	for sep, n := range count {
		if n > count[intra] || (n == count[intra] && (len(sep) < len(intra) || (len(sep) == len(intra) && sep < intra))) {
			intra = sep
		}
	}
	var words []string
	word := []byte{s[letters[0]]}
	for k, sep := range seps {
		if sep != intra {
			words = append(words, string(word))
			word = word[:0:0]
		}
		word = append(word, s[letters[k+1]])
	}
	return append(words, string(word))
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
