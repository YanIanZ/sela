package safety

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Typo tolerance (ruleset 2026.11.01-1, S-011). A person in distress types
// fast and badly: "bnuh dri", "gantug diri", "kil myself", "mtii aja". The
// normalised text already resolves leetspeak, spacing and repeated letters;
// this pass additionally reads each word that is one edit (insertion,
// deletion, substitution or transposition) away from a risk word of five
// letters or more as that word, and expands a short list of chat
// abbreviations. Assess then judges the corrected text too and keeps the
// stronger outcome, so a correction can only ever add safety, never remove
// it. Short words (four letters or fewer, such as "mati", "kill", "diri")
// are never corrected by distance — too many everyday words sit one edit
// away — only through the explicit abbreviations.

// typoTargets are the risk words a misspelling is read as.
var typoTargets = []string{
	"bunuh", "bundir", "gantung", "racun", "overdosis", "nyilet", "mengakhiri", "hidupku", "loncat", "terjun",
	"mampus", "suicide", "suicidal", "myself", "overdose", "murder", "strangle", "membunuh", "bacok",
}

// typoAbbrev expands chat abbreviations and frequent short misspellings of
// risk words.
var typoAbbrev = map[string]string{
	"dri": "diri", "dr1": "diri", "mti": "mati", "mtii": "mati", "matii": "mati", "bnh": "bunuh", "bnuh": "bunuh",
	"bndr": "bundir", "kil": "kill", "kll": "kill", "myslf": "myself", "mysef": "myself", "slf": "self",
	"sucid": "suicide", "suicid": "suicide", "gntng": "gantung", "rcn": "racun",
}

// typoKeep are everyday words one edit away from a target; never corrected.
var typoKeep = map[string]bool{
	"bunuk": true, "butuh": true, "racik": true, "hidupmu": true, "hidupnya": true, "mampu": true, "bacot": true, "bacod": true,
	"border": true, "burden": true, "herself": true, "himself": true, "itself": true, "yourself": true,
}

// typoVariant returns text with misspelt risk words corrected in place,
// and whether anything changed. Only the corrected words are replaced; the
// rest of the text (case, quotes, punctuation, line breaks) stays exactly
// as written, so the catalogue's context checks read it the same way.
func typoVariant(text string) (string, bool) {
	changed := false
	out := typoWordRe.ReplaceAllStringFunc(text, func(tok string) string {
		core, pre, post := trimPunct(Normalize(tok))
		if core == "" || strings.ContainsRune(core, ' ') {
			return tok
		}
		if fix, ok := typoAbbrev[core]; ok {
			changed = true
			return pre + fix + post
		}
		if typoKeep[core] || utf8.RuneCountInString(core) < 5 {
			return tok
		}
		for _, t := range typoTargets {
			if core != t && editDistanceOne(core, t) {
				changed = true
				return pre + t + post
			}
		}
		return tok
	})
	if !changed {
		return "", false
	}
	return out, true
}

var typoWordRe = regexp.MustCompile(`[\p{L}\p{N}'@$!|]+`)

func trimPunct(w string) (core, pre, post string) {
	start, end := 0, len(w)
	for start < end && strings.ContainsRune(".,!?;:\"'()[]", rune(w[start])) {
		start++
	}
	for end > start && strings.ContainsRune(".,!?;:\"'()[]", rune(w[end-1])) {
		end--
	}
	return w[start:end], w[:start], w[end:]
}

// editDistanceOne reports whether a and b differ by exactly one insertion,
// deletion, substitution or adjacent transposition (ASCII words only).
func editDistanceOne(a, b string) bool {
	la, lb := len(a), len(b)
	if la-lb > 1 || lb-la > 1 || a == b {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] >= 0x80 {
			return false
		}
	}
	if la == lb {
		diff := -1
		for i := 0; i < la; i++ {
			if a[i] != b[i] {
				if diff >= 0 {
					// a second difference: only an adjacent swap is allowed
					return diff == i-1 && a[diff] == b[i] && a[i] == b[diff] && a[i+1:] == b[i+1:]
				}
				diff = i
			}
		}
		return true
	}
	if la > lb {
		a, b = b, a
		la, lb = lb, la
	}
	// b is one longer: skip one byte of b
	i := 0
	for i < la && a[i] == b[i] {
		i++
	}
	return a[i:] == b[i+1:]
}
