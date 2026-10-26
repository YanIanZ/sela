package ledger

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Segment limits.
const (
	maxSegments     = 40
	maxSegmentRunes = 400
	minCutRunes     = 100 // a long sentence is never cut into a piece shorter than this unless it must
)

// abbreviations end with '.' without ending a sentence (lowercase, without
// the final dot). Words with an internal dot ("e.g", "s.psi") are always
// treated as abbreviations.
var abbreviations = set(
	"dr", "drg", "dll", "dsb", "dst", "dkk", "tsb", "yth", "sdr", "sdri", "bpk", "prof", "ir", "hj", "jl", "hlm",
	"kec", "kab", "kel", "ny", "tn", "sp", "mr", "mrs", "ms", "st", "vs", "etc", "approx", "dept", "univ", "eg", "ie",
)

func isTerminator(r rune) bool {
	switch r {
	case '.', '!', '?', '…', '。', '！', '？', '．':
		return true
	}
	return false
}

func isCloser(r rune) bool {
	switch r {
	case '"', '\'', ')', ']', '}', '”', '’', '»':
		return true
	}
	return false
}

func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', '\u0085', ' ', ' ':
		return true
	}
	return false
}

// SplitSegments splits text into sentence-level segments with IDs
// "s<startIndex>", "s<startIndex+1>", ... for the given origin.
//
// Sentences end at '.', '!', '?', '…' (and their full-width forms) followed
// by whitespace or the end of a line, and at every line break. A '.' after a
// known abbreviation ("dr.", "dll.", "e.g.") or inside a number does not end
// a sentence. Segments are trimmed; segments without a letter or digit are
// dropped. A sentence longer than 400 runes is cut at the last comma, else
// the last space, before the limit. At most 40 segments are returned; text
// beyond that is not segmented. Invalid UTF-8 is replaced with U+FFFD.
func SplitSegments(text, origin string, startIndex int) []Segment {
	text = strings.ToValidUTF8(text, "�")
	out := []Segment{}
	for _, line := range strings.FieldsFunc(text, isLineBreak) {
		for _, sentence := range splitSentences(line) {
			for _, piece := range splitLong(sentence) {
				piece = strings.TrimSpace(piece)
				if strings.IndexFunc(piece, isWordRune) < 0 {
					continue
				}
				if len(out) == maxSegments {
					return out
				}
				out = append(out, Segment{ID: "s" + strconv.Itoa(startIndex+len(out)), Text: piece, Origin: origin})
			}
		}
	}
	return out
}

// splitSentences splits one line at sentence terminators.
func splitSentences(line string) []string {
	var out []string
	begin := 0
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRuneInString(line[i:])
		if !isTerminator(r) {
			i += size
			continue
		}
		dotOnly := r == '.' || r == '．'
		j := i + size
		for j < len(line) {
			r2, s2 := utf8.DecodeRuneInString(line[j:])
			if !isTerminator(r2) {
				break
			}
			if r2 != '.' && r2 != '．' {
				dotOnly = false
			}
			j += s2
		}
		for j < len(line) {
			r2, s2 := utf8.DecodeRuneInString(line[j:])
			if !isCloser(r2) {
				break
			}
			j += s2
		}
		atEnd := j == len(line)
		if !atEnd {
			next, _ := utf8.DecodeRuneInString(line[j:])
			if !unicode.IsSpace(next) {
				i = j
				continue
			}
		}
		if dotOnly && j-i == 1 && isAbbreviation(line[begin:i]) {
			i = j
			continue
		}
		out = append(out, line[begin:j])
		begin = j
		i = j
	}
	if begin < len(line) {
		out = append(out, line[begin:])
	}
	return out
}

// isAbbreviation reports whether the word ending prefix is an abbreviation.
func isAbbreviation(prefix string) bool {
	k := len(prefix)
	for k > 0 {
		r, size := utf8.DecodeLastRuneInString(prefix[:k])
		if !unicode.IsLetter(r) && r != '.' {
			break
		}
		k -= size
	}
	word := strings.ToLower(prefix[k:])
	if word == "" {
		return false
	}
	if strings.Contains(strings.Trim(word, "."), ".") {
		return true
	}
	return abbreviations[strings.ReplaceAll(word, ".", "")]
}

// splitLong cuts a sentence into pieces of at most maxSegmentRunes runes.
func splitLong(s string) []string {
	var out []string
	for utf8.RuneCountInString(s) > maxSegmentRunes {
		limit, n := 0, 0
		cutComma, cutSpace := -1, -1
		for i, r := range s {
			if n == maxSegmentRunes {
				limit = i
				break
			}
			if n >= minCutRunes {
				switch {
				case r == ',' || r == ';' || r == ':':
					cutComma = i + 1
				case unicode.IsSpace(r):
					cutSpace = i
				}
			}
			n++
		}
		cut := limit
		if cutComma > 0 {
			cut = cutComma
		} else if cutSpace > 0 {
			cut = cutSpace
		}
		out = append(out, s[:cut])
		s = strings.TrimLeftFunc(s[cut:], unicode.IsSpace)
	}
	return append(out, s)
}
