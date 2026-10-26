package fakellm

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// The negation lexicon and tokenizer mirror internal/ledger (lexicon.go and
// tokens.go), whose versions are unexported: the fake's "negated" flag must
// agree with the ledger's deterministic reading, and FaultNegationFlip must
// find every negator the ledger would. TestNegatedMatchesLedger catches
// drift between the two.

// negators are single-token negation words.
var negators = wordSet(
	// Indonesian, formal and informal.
	"tidak", "tak", "nggak", "ngga", "ngak", "gak", "ga", "enggak", "engga", "ndak", "nda", "kagak", "kaga",
	"tdk", "gk", "bukan", "bukannya", "bukanlah", "belum", "blm", "tanpa", "jangan",
	// English ("n't" contractions are matched by suffix).
	"not", "no", "never", "without", "nobody", "nothing", "neither", "nor", "none", "nowhere", "cannot",
	"noone", "no-one",
	// English contractions typed without an apostrophe, common in chat.
	"dont", "doesnt", "didnt", "cant", "wont", "isnt", "arent", "wasnt", "werent", "couldnt", "wouldnt",
	"shouldnt", "havent", "hasnt", "hadnt", "aint", "mustnt", "neednt", "dunno",
)

// Informal Indonesian negators fused with the next word ("gabisa", "gamau").
// They are listed explicitly: a "ga-" prefix rule would also catch "gagal",
// "galau" or "ganti".
var (
	fusedNegatorHeads = []string{"ga", "gak", "gk", "ngga", "nggak", "ngak", "enggak", "engga", "kagak", "kaga", "tdk", "tak", "ndak", "nda"}
	fusedNegatorTails = []string{
		"bisa", "bs", "mau", "ada", "suka", "usah", "kuat", "pernah", "boleh", "perlu", "punya", "jadi", "peduli",
		"berani", "mampu", "sanggup", "sempat", "sempet", "tega", "enak", "papa", "apa", "mood", "nafsu", "semangat",
		"pengen", "ingin",
		// Fused hedges ("gatau" = "nggak tahu") negate too.
		"tau", "tahu", "yakin", "paham", "ngerti",
	}
)

// fusedNegators maps each fused negator to the word it negates, which
// FaultNegationFlip keeps ("gabisa" becomes "bisa"). An empty tail means
// the whole word is dropped ("gpp", "it's fine").
var fusedNegators = func() map[string]string {
	m := map[string]string{"gada": "ada", "gbs": "bs", "gbisa": "bisa", "gmau": "mau", "gpp": "", "gtau": "tau"}
	for _, h := range fusedNegatorHeads {
		for _, t := range fusedNegatorTails {
			if _, dup := m[h+t]; !dup {
				m[h+t] = t
			}
		}
	}
	return m
}()

func wordSet(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// isNegator reports whether a folded token negates.
func isNegator(w string) bool {
	if negators[w] || strings.HasSuffix(w, "n't") {
		return true
	}
	_, fused := fusedNegators[w]
	return fused
}

// token is one folded word and its byte span in the original text.
type token struct {
	word       string
	start, end int
}

// foldRune lowercases r and maps typographic apostrophes and hyphens to
// their ASCII forms.
func foldRune(r rune) rune {
	switch r {
	case '‘', '’', '‚', '‛', 'ʼ':
		return '\''
	case '‐', '‑':
		return '-'
	}
	return unicode.ToLower(r)
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// tokenize splits text into folded word tokens. An apostrophe or hyphen
// between two letters stays inside a word ("don't", "no-one"), as in the
// ledger.
func tokenize(text string) []token {
	var toks []token
	var cur []rune
	start := 0
	flush := func(end int) {
		if len(cur) > 0 {
			toks = append(toks, token{word: string(cur), start: start, end: end})
			cur = cur[:0]
		}
	}
	for i, r := range text {
		f := foldRune(r)
		if isWordRune(f) {
			if len(cur) == 0 {
				start = i
			}
			cur = append(cur, f)
			continue
		}
		if (f == '\'' || f == '-') && len(cur) > 0 && unicode.IsLetter(cur[len(cur)-1]) {
			next, _ := utf8.DecodeRuneInString(text[i+utf8.RuneLen(r):])
			if unicode.IsLetter(foldRune(next)) {
				cur = append(cur, f)
				continue
			}
		}
		flush(i)
	}
	flush(len(text))
	return toks
}

// hasNegator reports whether text contains a negator.
func hasNegator(text string) bool {
	for _, t := range tokenize(text) {
		if isNegator(t.word) {
			return true
		}
	}
	return false
}

// dropNegators removes every negator from text, keeping the negated word
// of a fused negator ("Aku gabisa tidur" becomes "Aku bisa tidur"), and
// reports whether it removed any.
func dropNegators(text string) (string, bool) {
	var b strings.Builder
	last, dropped := 0, false
	for _, t := range tokenize(text) {
		if !isNegator(t.word) {
			continue
		}
		dropped = true
		if tail := fusedNegators[t.word]; tail != "" {
			b.WriteString(text[last:t.start])
			b.WriteString(matchCase(text[t.start:t.end], tail))
			last = t.end
			continue
		}
		// Drop the word and the spaces after it; at the end of a phrase,
		// drop the spaces before it instead.
		end := t.end
		for end < len(text) && text[end] == ' ' {
			end++
		}
		head := text[last:t.start]
		if end == t.end {
			head = strings.TrimRight(head, " ")
		}
		b.WriteString(head)
		last = end
	}
	b.WriteString(text[last:])
	out := b.String()
	if dropped {
		// "Tidak bisa tidur." becomes "Bisa tidur.", not "bisa tidur.".
		out = matchCase(text, out)
	}
	return out, dropped
}

// matchCase capitalizes word when orig starts with an upper-case letter.
func matchCase(orig, word string) string {
	r, _ := utf8.DecodeRuneInString(orig)
	if !unicode.IsUpper(r) || word == "" {
		return word
	}
	w, size := utf8.DecodeRuneInString(word)
	return string(unicode.ToUpper(w)) + word[size:]
}

// addNegator inserts a negator ("tidak", or "not" for English) after the
// first word of text, or before it when text has a single word.
func addNegator(text, language string) string {
	neg := "tidak"
	if language == "en" {
		neg = "not"
	}
	toks := tokenize(text)
	switch len(toks) {
	case 0:
		return strings.TrimSpace(neg + " " + text)
	case 1:
		return text[:toks[0].start] + neg + " " + text[toks[0].start:]
	}
	return text[:toks[0].end] + " " + neg + text[toks[0].end:]
}

// flipNegation reverses the polarity of text: it drops every negator or,
// when text has none, adds one. The result always reads differently from
// text, so FaultNegationFlip is never a silent no-op.
func flipNegation(text, language string) string {
	if out, dropped := dropNegators(text); dropped {
		return out
	}
	return addNegator(text, language)
}
