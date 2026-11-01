package ledger

import "strings"

// MeaningVersion identifies the Meaning Diff rules (MIE H-004). Change it
// whenever a rule or lexicon below changes.
const MeaningVersion = "sela-meaning-2026.11.01-1"

// Meaning difference codes, in report order.
const (
	MeaningNegation       = "negation"
	MeaningSubject        = "subject"
	MeaningTime           = "time"
	MeaningCertainty      = "certainty"
	MeaningFrequency      = "frequency"
	MeaningGeneralization = "generalization"
	MeaningNumber         = "number"
	MeaningAddedContent   = "added_content"
	MeaningDiagnosis      = "diagnosis"
	MeaningMedical        = "medical"
)

// Severities.
const (
	SeverityCritical = "critical" // the meaning changed
	SeverityReview   = "review"   // worth a look, the meaning may still hold
)

// MeaningDifference is one way Sela's wording departs from the user's words.
type MeaningDifference struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Source   string `json:"source,omitempty"`
	Wording  string `json:"wording,omitempty"`
}

// MeaningCheck is the deterministic comparison of one wording with the
// user's words it rests on.
type MeaningCheck struct {
	Version     string              `json:"version"`
	Differences []MeaningDifference `json:"differences"`
	Critical    bool                `json:"critical"`
}

// Frequency classes, ordered from never to always.
const (
	freqNever     = "never"
	freqRarely    = "rarely"
	freqSometimes = "sometimes"
	freqOften     = "often"
	freqAlways    = "always"
)

var freqRank = map[string]int{freqNever: 0, freqRarely: 1, freqSometimes: 2, freqOften: 3, freqAlways: 4}

// frequencyLexicon tags frequency markers. "terus" alone is left out: it
// usually means "then". "pernah" alone is a past-time marker, not a
// frequency.
var frequencyLexicon = newLexicon(map[string][]string{
	freqNever: {
		"tidak pernah", "tak pernah", "nggak pernah", "ngga pernah", "gak pernah", "ga pernah", "enggak pernah",
		"belum pernah", "gapernah", "gakpernah", "nggakpernah", "ngapernah", "never", "not once", "not ever",
	},
	freqRarely: {"jarang", "jarang-jarang", "hampir tidak pernah", "hampir nggak pernah", "rarely", "seldom", "hardly ever"},
	freqSometimes: {
		"kadang", "kadang-kadang", "kadang2", "terkadang", "kadangkala", "kadang kala", "sesekali", "sekali-sekali",
		"sekali sekali", "sometimes", "occasionally", "at times", "now and then", "once in a while", "from time to time",
	},
	freqOften: {
		"sering", "seringkali", "sering kali", "sering-sering", "acap kali", "kerap", "kerap kali", "often",
		"frequently", "a lot of the time", "most of the time", "usually", "biasanya",
	},
	freqAlways: {
		"selalu", "senantiasa", "terus-menerus", "terus menerus", "terus-terusan", "setiap hari", "tiap hari",
		"setiap saat", "tiap saat", "setiap waktu", "tiap waktu", "sepanjang waktu", "sepanjang hari", "setiap malam",
		"tiap malam", "setiap pagi", "tiap pagi", "every night", "every morning", "always",
		"constantly", "every day", "everyday", "all the time", "every time", "continuously", "nonstop", "non-stop",
	},
})

// generalizationLexicon holds universal quantifiers: a wording that adds
// one turns a specific experience into a rule about everything or everyone.
var generalizationLexicon = newLexicon(map[string][]string{
	"universal": {
		"semua", "semuanya", "seluruh", "seluruhnya", "segala", "segalanya", "siapapun", "siapa pun", "apapun",
		"apa pun", "tidak ada yang", "nggak ada yang", "gak ada yang", "ga ada yang", "enggak ada yang",
		"tak ada yang", "tidak seorang pun", "everyone", "everybody", "everything", "all", "nobody", "no one",
		"noone", "no-one", "every", "nothing", "anyone at all", "siapa-siapa", "apa-apa",
	},
})

// certainLexicon holds markers that state certainty outright.
var certainLexicon = newLexicon(map[string][]string{
	"certain": {
		"pasti", "jelas", "jelas-jelas", "sudah pasti", "tentu saja", "tentunya", "sebenarnya kamu", "memang kamu",
		"definitely", "certainly", "clearly", "obviously", "surely", "without doubt", "no doubt", "undoubtedly",
	},
})

// implicitNegators carry negative polarity without a negation word ("sulit
// tidur" ≈ "tidak bisa tidur"). They count as negators only for the
// polarity comparison of CompareMeaning.
var implicitNegators = set(
	"sulit", "susah", "sukar", "kesulitan", "kesusahan", "kurang", "unable", "difficult", "hard", "struggle",
	"struggles", "struggling", "trouble",
)

// universalWords are quantity words that are generalizations, reported as
// such and not again as invented numbers.
var universalWords = set("semua", "seluruh", "all", "every", "everyone", "everybody", "nothing", "nobody")

// CompareMeaning compares Sela's wording (an AI interpretation or summary
// sentence) with the user's own words it rests on (a quote or a whole
// turn). It is deterministic and does no I/O. It catches known failure
// classes only: a paraphrase it cannot read may still be flagged for
// review, and the user remains the authority on what they meant.
//
// Sela addresses the user as "kamu"/"you"; that is the user ("aku"/"I"),
// not a subject shift.
func CompareMeaning(source, wording string) MeaningCheck {
	out := MeaningCheck{Version: MeaningVersion, Differences: []MeaningDifference{}}
	add := func(code, severity, src, w string) {
		out.Differences = append(out.Differences, MeaningDifference{Code: code, Severity: severity, Source: src, Wording: w})
		if severity == SeverityCritical {
			out.Critical = true
		}
	}
	if strings.TrimSpace(wording) == "" {
		return out
	}
	stoks, wtoks := tokenize(source), tokenize(wording)

	// Markers each rule reports on its own are kept out of the polarity
	// and added-content comparisons.
	sFreq, wFreq := frequencyLexicon.scan(stoks), frequencyLexicon.scan(wtoks)
	wGen := generalizationLexicon.scan(wtoks)
	sGen := generalizationLexicon.scan(stoks)
	wGen = withoutOverlap(wGen, wFreq)
	sGen = withoutOverlap(sGen, sFreq)

	// negation
	if src, w, changed := polarityChange(stoks, wtoks, sFreq, wFreq); changed {
		add(MeaningNegation, SeverityCritical, src, w)
	}

	// subject
	sSub, wSub := wholeSubject(stoks), wholeSubject(wtoks)
	switch {
	case sSub != wSub:
		add(MeaningSubject, SeverityCritical, string(sSub), string(wSub))
	case namesOther(wtoks, 0, len(wtoks)) && !namesOther(stoks, 0, len(stoks)):
		add(MeaningSubject, SeverityCritical, string(sSub), "other")
	}

	// time: a time frame the user did not state
	sTime := tagSet(timeLexicon.scan(stoks))
	for _, h := range timeLexicon.scan(wtoks) {
		if !sTime[h.tag] && !insideAny(h, wFreq) {
			add(MeaningTime, SeverityReview, firstTag(timeLexicon.scan(stoks)), phraseText(wtoks, h))
			break
		}
	}

	// certainty
	sCert, wCert := certaintyLexicon.scan(stoks), certaintyLexicon.scan(wtoks)
	sCertain, wCertain := certainLexicon.scan(stoks), certainLexicon.scan(wtoks)
	switch {
	case len(sCert) > 0 && len(wCert) == 0:
		add(MeaningCertainty, SeverityCritical, phraseText(stoks, sCert[0]), string(CertaintyStated))
	case len(wCertain) > 0 && len(sCertain) == 0:
		add(MeaningCertainty, SeverityCritical, certaintyName(stoks, sCert), phraseText(wtoks, wCertain[0]))
	case len(wCert) > 0 && len(sCert) == 0:
		add(MeaningCertainty, SeverityReview, string(CertaintyStated), phraseText(wtoks, wCert[0]))
	case len(wCert) > 0 && len(sCert) > 0 && wCert[0].tag != sCert[0].tag:
		add(MeaningCertainty, SeverityReview, phraseText(stoks, sCert[0]), phraseText(wtoks, wCert[0]))
	}

	// frequency
	if d, ok := frequencyChange(stoks, wtoks, sFreq, wFreq); ok {
		add(MeaningFrequency, d.Severity, d.Source, d.Wording)
	}

	// generalization
	if len(wGen) > 0 && len(sGen) == 0 {
		add(MeaningGeneralization, SeverityCritical, "", phraseText(wtoks, wGen[0]))
	}

	// number: an invented quantity that is not a generalization
	have, srcPct := quantities(source)
	got, pct := quantities(wording)
	for n := range got {
		if !have[n] && !universalWords[n] {
			add(MeaningNumber, SeverityReview, "", n)
			pct = false
			break
		}
	}
	if pct && !srcPct {
		add(MeaningNumber, SeverityReview, "", "%")
	}

	// added content, outside the markers reported above and second-person
	// address
	if w, ok := addedContent(stoks, wtoks, [][]hit{wFreq, wGen, wCert, wCertain, timeLexicon.scan(wtoks)}); ok {
		add(MeaningAddedContent, SeverityReview, "", w)
	}

	// diagnosis and medical instruction
	if ContainsDiagnosisLabel(wording) && !ContainsDiagnosisLabel(source) {
		add(MeaningDiagnosis, SeverityCritical, "", "")
	}
	if ContainsMedicalInstruction(wording) && !ContainsMedicalInstruction(source) {
		add(MeaningMedical, SeverityCritical, "", "")
	}
	return out
}

// polarityChange compares negation: every content word the two texts share
// must keep one of its negation counts, and no word only in the wording may
// be negated. Frequency markers ("tidak pernah") are left to the frequency
// rule. Implicit negators ("sulit", "unable") count as negators here.
// rootForms are the spellings a word may take once one Indonesian prefix
// and one clitic are removed ("bertemu", "ketemu" -> "temu";
// "pekerjaanmu" -> "pekerjaan").
func rootForms(w string) []string {
	forms := []string{w, stem(w)}
	base := []string{w}
	for _, suf := range []string{"nya", "ku", "mu"} {
		if strings.HasSuffix(w, suf) && runeLen(w)-runeLen(suf) >= 4 {
			base = append(base, strings.TrimSuffix(w, suf))
		}
	}
	for _, b := range base {
		forms = append(forms, b)
		for _, pre := range []string{"meng", "meny", "mem", "men", "me", "ber", "ter", "ke", "di", "se", "pe", "per"} {
			if strings.HasPrefix(b, pre) && runeLen(b)-runeLen(pre) >= 4 {
				forms = append(forms, strings.TrimPrefix(b, pre))
			}
		}
	}
	return forms
}

// sameMeaningWord reports whether two content words are spellings of one
// word for the meaning comparison: sameWord, or a shared root form.
func sameMeaningWord(a, b string) bool {
	if sameWord(a, b) {
		return true
	}
	for _, x := range rootForms(a) {
		for _, y := range rootForms(b) {
			if x == y || covers(x, y) {
				return true
			}
		}
	}
	return false
}

// polarityKeptMeaning is polarityKept with sameMeaningWord.
func polarityKeptMeaning(src, wording []polarWord) bool {
	for _, ww := range wording {
		found, same := false, false
		for _, sw := range src {
			if sameMeaningWord(ww.word, sw.word) {
				found = true
				if sw.neg == ww.neg {
					same = true
					break
				}
			}
		}
		if found != same || (!found && ww.neg > 0) {
			return false
		}
	}
	return true
}

func polarityChange(stoks, wtoks []token, sFreq, wFreq []hit) (string, string, bool) {
	sw := polarWordsExt(stoks, sFreq)
	ww := polarWordsExt(wtoks, wFreq)
	if polarityKeptMeaning(sw, ww) {
		return "", "", false
	}
	return firstNegator(stoks, sFreq), firstNegator(wtoks, wFreq), true
}

func isPolar(t string) bool { return isNegator(t) || implicitNegators[t] }

// polarWordsExt is polarWords over negationCounts with implicit negators,
// skipping the tokens of the given marker hits.
func polarWordsExt(toks []token, skip []hit) []polarWord {
	counts := make([]int, len(toks))
	masked := maskOf(len(toks), skip)
	for i, t := range toks {
		if masked[i] || !isPolar(t.text) || (isNegator(t.text) && isHedge(toks, i)) {
			continue
		}
		// A negator governs its head: the first content word after it in
		// the clause, within three words ("susah tidur", "tidak bisa
		// tidur"). Pronouns and
		// connective words ("berarti", "means") are passed over. Comparing
		// heads, not every word in the window, keeps "tidak dihargai di
		// rumah" from negating "rumah".
		window := negationWindow
		// After the head, one directly following content word is governed
		// too ("tidak suka keramaian"), unless a function word or a
		// preposition ("di", "menjelang", "after") comes first.
		head, governed := false, 0
		for k, seen := i+1, 0; k < len(toks) && seen < window && toks[k].clause == t.clause; k++ {
			w := toks[k].text
			if firstPerson[w] || addressWords[w] || negTransparent[w] {
				continue
			}
			seen++
			if isPolar(w) {
				break
			}
			if !hasContent(w) || negBreak[w] {
				if head {
					break
				}
				continue
			}
			counts[k]++
			head = true
			if governed++; governed == 2 {
				break
			}
		}
	}
	var out []polarWord
	for i := range toks {
		if masked[i] || isPolar(toks[i].text) {
			continue
		}
		for _, p := range wordParts(toks[i].text) {
			if isContent(p) && !addressWords[p] {
				out = append(out, polarWord{word: p, neg: counts[i]})
			}
		}
	}
	return out
}

// negTransparent words sit between a negator and what it negates without
// being negated themselves ("bukan berarti membenci", "doesn't mean I hate").
var negTransparent = set("berarti", "artinya", "maksudnya", "mean", "means", "meaning", "really", "benar-benar", "terlalu", "too")

// negBreak are prepositions and connectives that end what a negator governs.
var negBreak = set("menjelang", "sebelum", "sesudah", "setelah", "selama", "sejak", "karena", "akibat", "sampai",
	"after", "before", "during", "since", "because", "around", "near", "until")

func hasContent(t string) bool {
	for _, p := range wordParts(t) {
		if isContent(p) {
			return true
		}
	}
	return false
}

func firstNegator(toks []token, skip []hit) string {
	masked := maskOf(len(toks), skip)
	for i, t := range toks {
		if !masked[i] && isPolar(t.text) && !(isNegator(t.text) && isHedge(toks, i)) {
			return t.text
		}
	}
	return ""
}

// addressWords are Sela addressing the user.
var addressWords = set("kamu", "anda", "kau", "engkau", "dirimu", "you", "your", "yours", "yourself")

// wholeSubject reads who a whole text is about: other when a reported-
// speech frame from someone else covers it or its first subject word names
// a third party; otherwise the user.
func wholeSubject(toks []token) Subject {
	if len(toks) == 0 {
		return SubjectSelf
	}
	return subjectOf(toks, 0, len(toks))
}

type freqDiff struct{ Severity, Source, Wording string }

// frequencyChange compares frequency classes. Escalation or de-escalation
// across classes is critical; an absolute ("always", "never") where the user
// gave no frequency is critical; a softer class where the user gave none,
// or a dropped frequency, is for review.
func frequencyChange(stoks, wtoks []token, sFreq, wFreq []hit) (freqDiff, bool) {
	sClasses := tagSet(sFreq)
	for _, h := range wFreq {
		if sClasses[h.tag] {
			continue
		}
		w := phraseText(wtoks, h)
		if len(sFreq) == 0 {
			if h.tag == freqAlways || h.tag == freqNever {
				return freqDiff{SeverityCritical, "", w}, true
			}
			return freqDiff{SeverityReview, "", w}, true
		}
		return freqDiff{SeverityCritical, phraseText(stoks, sFreq[0]), w}, true
	}
	if len(sFreq) > 0 && len(wFreq) == 0 {
		return freqDiff{SeverityReview, phraseText(stoks, sFreq[0]), ""}, true
	}
	return freqDiff{}, false
}

// addedContent reports the first content word of the wording that is not a
// spelling of a source word, outside the masked marker hits, second-person
// address, negators, third-party nouns and paraphrase words.
func addedContent(stoks, wtoks []token, masks [][]hit) (string, bool) {
	var all []hit
	for _, m := range masks {
		all = append(all, m...)
	}
	masked := maskOf(len(wtoks), all)
	var srcParts []string
	for _, t := range stoks {
		srcParts = append(srcParts, wordParts(t.text)...)
	}
	for i, t := range wtoks {
		if masked[i] || isPolar(t.text) || isThirdNoun(t.text) || addressWords[t.text] {
			continue
		}
	parts:
		for _, p := range wordParts(t.text) {
			if !isContent(p) || paraphraseWords[p] || addressWords[p] || addressWords[stripPossessive(p)] {
				continue
			}
			for _, s := range srcParts {
				if sameMeaningWord(p, s) {
					continue parts
				}
			}
			return p, true
		}
	}
	return "", false
}

func maskOf(n int, hits []hit) []bool {
	m := make([]bool, n)
	for _, h := range hits {
		for k := h.start; k < h.end && k < n; k++ {
			m[k] = true
		}
	}
	return m
}

func withoutOverlap(hits, other []hit) []hit {
	var out []hit
	for _, h := range hits {
		if !insideAny(h, other) {
			out = append(out, h)
		}
	}
	return out
}

func insideAny(h hit, others []hit) bool {
	for _, o := range others {
		if h.start < o.end && o.start < h.end {
			return true
		}
	}
	return false
}

func tagSet(hits []hit) map[string]bool {
	m := map[string]bool{}
	for _, h := range hits {
		m[h.tag] = true
	}
	return m
}

func firstTag(hits []hit) string {
	if len(hits) == 0 {
		return ""
	}
	return hits[0].tag
}

func phraseText(toks []token, h hit) string {
	words := make([]string, 0, h.end-h.start)
	for _, t := range toks[h.start:h.end] {
		words = append(words, t.text)
	}
	return strings.Join(words, " ")
}

func certaintyName(toks []token, hits []hit) string {
	if len(hits) == 0 {
		return string(CertaintyStated)
	}
	return phraseText(toks, hits[0])
}
