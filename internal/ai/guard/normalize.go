package guard

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/YanIanZ/sela/internal/ai/textnorm"
)

// fold lowercases s, maps fullwidth ASCII to ASCII, folds accents and
// homoglyphs, drops zero-width and other format characters and removes
// apostrophes ("don't" becomes "dont"). Punctuation is kept. It is the
// normalisation the input redaction uses too (internal/ai/textnorm).
func fold(s string) string { return textnorm.Fold(s) }

// leet maps digits and symbols inside letter tokens ("d3pr3si", "4ku").
var leet = map[rune]rune{'0': 'o', '1': 'i', '3': 'e', '4': 'a', '5': 's', '7': 't', '@': 'a', '$': 's'}

// words turns folded text into space-separated letter/digit tokens. With
// deleet, digits and @/$ inside a token that also has letters become
// letters, so informal spellings match; without it digits stay (dosages).
func words(folded string, deleet bool) string {
	var out []string
	for _, raw := range strings.FieldsFunc(folded, unicode.IsSpace) {
		tok := raw
		if deleet && hasLetter(raw) {
			var b strings.Builder
			for _, r := range raw {
				if l, ok := leet[r]; ok {
					r = l
				}
				b.WriteRune(r)
			}
			tok = b.String()
		}
		var b strings.Builder
		for _, r := range tok {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(r)
			} else {
				b.WriteByte(' ')
			}
		}
		for _, f := range strings.Fields(b.String()) {
			out = append(out, f)
		}
	}
	return strings.Join(out, " ")
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// sentences splits folded text at sentence ends and line breaks.
func sentences(folded string) []string {
	return strings.FieldsFunc(folded, func(r rune) bool {
		return r == '.' || r == '!' || r == '?' || r == '\n' || r == '\r' || r == '…' || r == '。'
	})
}

// clauses splits folded text further at commas, semicolons, colons and
// dashes, so "kalau kamu butuh, aku di sini" is two clauses.
func clauses(folded string) []string {
	return strings.FieldsFunc(folded, func(r rune) bool {
		switch r {
		case '.', '!', '?', '\n', '\r', '…', '。', ',', ';', ':', '—', '–', '(', ')', '"', '“', '”':
			return true
		}
		return false
	})
}

// w returns a regexp alternation of word forms in which every letter may
// repeat ("cumaaa", "bangeeet"). Spaces inside a form match any run of
// spaces.
func w(forms ...string) string {
	parts := make([]string, 0, len(forms))
	for _, form := range forms {
		var b strings.Builder
		for i, f := range strings.Fields(form) {
			if i > 0 {
				b.WriteString(`\s+`)
			}
			for _, r := range f {
				b.WriteString(regexp.QuoteMeta(string(r)))
				if unicode.IsLetter(r) {
					b.WriteByte('+')
				}
			}
		}
		parts = append(parts, b.String())
	}
	return "(?:" + strings.Join(parts, "|") + ")"
}

// attached parts start with this prefix and are appended without a
// separator.
const attached = `(?:\s+`

// gap allows up to n arbitrary words.
func gap(n int) string { return fmt.Sprintf(`(?:\s+\S+){0,%d}`, n) }

// opt makes a part optional.
func opt(part string) string { return attached + part + `)?` }

// seq joins parts with whitespace (gap and opt parts attach directly) and
// anchors the result at word boundaries.
func seq(parts ...string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString(`\b`)
	for i, p := range parts {
		if strings.HasPrefix(p, attached) {
			b.WriteString(p)
			continue
		}
		if i > 0 {
			b.WriteString(`\s+`)
		}
		b.WriteString(p)
	}
	b.WriteString(`\b`)
	return regexp.MustCompile(b.String())
}

// start anchors a seq at the start of the sentence (imperatives).
func start(parts ...string) *regexp.Regexp {
	re := seq(parts...)
	return regexp.MustCompile(`^(?:tolong\s+|please\s+|pls\s+|yuk\s+|ayo\s+)?` + strings.TrimPrefix(re.String(), `\b`))
}
