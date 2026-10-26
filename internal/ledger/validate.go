package ledger

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxOccurrences bounds how many matches of one quote are tried.
const maxOccurrences = 16

// User-wording errors returned by ValidateUserWording.
var (
	ErrWordingEmpty   = errors.New("ledger: wording is empty")
	ErrWordingTooLong = errors.New("ledger: wording exceeds MaxEditRunes")
	ErrWordingInvalid = errors.New("ledger: wording contains invalid characters")
)

// segmentData holds the lazily computed matching state of one segment.
type segmentData struct {
	seg   Segment
	ready bool
	form  groundForm
	toks  []token
}

func (sd *segmentData) prepare() {
	if !sd.ready {
		sd.form = newGroundForm(sd.seg.Text)
		sd.toks = tokenize(sd.seg.Text)
		sd.ready = true
	}
}

func validKind(k Kind) bool {
	for _, v := range AllKinds {
		if k == v {
			return true
		}
	}
	return false
}

// selfKinds are the kinds that present a statement as the user's own state.
func selfKind(k Kind) bool {
	return k == KindFeeling || k == KindExperience || k == KindImpact || k == KindDuration
}

func lowerTrim(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// ValidateDraft applies Meaning Lock to a parsed draft.
//
// Candidates are checked in order. A candidate is rejected when its kind is
// invalid, its segment does not exist, its quote is empty, shorter than
// MinQuoteRunes or longer than MaxQuoteRunes, or not found in the segment
// (case, width, quote style and whitespace are ignored); when the quote
// starts after a negator that governs it, even if it keeps another one
// (negation_dropped); when the source is about someone else but the claim
// presents it as the user's own feeling, experience, impact or duration
// (subject_mismatch); when the quote carries a clinical label, except a
// context claim whose every label is attributed to a clinician in its own
// clause, or the model's wording adds one (diagnosis_label); when the model's
// wording contains a medication or treatment directive the quote does not
// (medical_instruction); when it repeats an accepted (segment, quote) pair
// (duplicate); or beyond MaxClaims.
//
// Accepted claims always carry the deterministic source qualifiers. Their
// quote is the verbatim segment span, widened to whole words. The model's
// wording is kept only when it fits MaxWordingRunes (longer wording is never
// analysed) and passes every check of wordingNotes: negation word by word,
// subject, every time and certainty marker, no new number, no added content,
// and no unsafe content (a clinician-attributed label or a directive the user
// reported is shown only in the user's words). Otherwise the quote is used
// and a note is recorded per failed check. Unknowns are recomputed from the
// accepted claims; the model's unknowns are ignored and its clarification
// pick survives only if it is still unknown.
func ValidateDraft(d Draft, segments []Segment) Result {
	res := Result{Accepted: []Claim{}, Rejected: []Rejection{}}
	byID := make(map[string]*segmentData, len(segments))
	for _, s := range segments {
		if _, dup := byID[s.ID]; !dup {
			byID[s.ID] = &segmentData{seg: s}
		}
	}
	seen := map[string]bool{}
	for i, c := range d.Claims {
		claim, reason := validateCandidate(c, byID)
		if reason == "" {
			key := claim.SegmentID + "\x00" + normalizeForGrounding(claim.Quote)
			switch {
			case seen[key]:
				reason = RejectDuplicate
			case len(res.Accepted) >= MaxClaims:
				reason = RejectLimit
			default:
				seen[key] = true
				res.Accepted = append(res.Accepted, claim)
				continue
			}
		}
		res.Rejected = append(res.Rejected, Rejection{Index: i, Reason: reason})
	}
	res.Unknowns = ComputeUnknowns(res.Accepted)
	if d.Clarification != nil {
		pick := UnknownField(lowerTrim(d.Clarification.Field))
		for _, u := range res.Unknowns {
			if u == pick {
				res.Clarification = pick
			}
		}
	}
	return res
}

// validateCandidate checks one candidate; a non-empty reason rejects it.
func validateCandidate(c Candidate, byID map[string]*segmentData) (Claim, RejectReason) {
	kind := Kind(lowerTrim(c.Kind))
	if !validKind(kind) {
		return Claim{}, RejectInvalidKind
	}
	sd := byID[strings.TrimSpace(c.SegmentID)]
	if sd == nil {
		return Claim{}, RejectUnknownSegment
	}
	q := normalizeForGrounding(c.Quote)
	switch n := runeLen(q); {
	case n == 0:
		return Claim{}, RejectEmpty
	case n < MinQuoteRunes:
		return Claim{}, RejectQuoteTooShort
	case n > MaxQuoteRunes:
		return Claim{}, RejectQuoteTooLong
	}
	sd.prepare()
	text := sd.seg.Text

	// Try each occurrence; the first one that keeps its meaning wins.
	var first RejectReason
	start, end := -1, -1
	var src analysis
	for _, span := range sd.form.occurrences(q, maxOccurrences) {
		s, e := widenToWords(sd.toks, span[0], span[1])
		if !utf8.ValidString(text[s:e]) {
			continue
		}
		a := analyze(sd.toks, s, e)
		if r := meaningReason(a, c, kind); r != "" {
			if first == "" {
				first = r
			}
			continue
		}
		start, end, src = s, e, a
		break
	}
	if start < 0 {
		if first != "" {
			return Claim{}, first
		}
		return Claim{}, RejectQuoteNotFound
	}
	quote := text[start:end]

	wording := strings.Join(strings.Fields(c.Text), " ")
	verbatim := wording == "" || normalizeForGrounding(wording) == normalizeForGrounding(quote)
	quoteLabel := ContainsDiagnosisLabel(quote)
	if quoteLabel && !(kind == KindContext && labelsAttributed(quote)) {
		return Claim{}, RejectDiagnosis
	}
	// The label and directive patterns are linear-time regular expressions,
	// so they run on wording of any length.
	wordingLabel := !verbatim && ContainsDiagnosisLabel(wording)
	if wordingLabel && !quoteLabel {
		return Claim{}, RejectDiagnosis
	}
	wordingMedical := !verbatim && ContainsMedicalInstruction(wording)
	if wordingMedical && !ContainsMedicalInstruction(quote) {
		return Claim{}, RejectMedical
	}

	claim := Claim{
		SegmentID:  sd.seg.ID,
		Quote:      quote,
		Start:      start,
		End:        end,
		Kind:       kind,
		Wording:    quote,
		FromQuote:  true,
		Qualifiers: src.q,
	}
	if qualifiersDiffer(c, src.q) {
		claim.Notes = append(claim.Notes, NoteQualifierFixed)
	}
	if verbatim {
		return claim, ""
	}
	if runeLen(wording) > MaxWordingRunes {
		// Never tokenized: analysing oversized text costs CPU for nothing.
		claim.Notes = append(claim.Notes, NoteWordingTooLong)
		return claim, ""
	}
	notes := wordingNotes(wording, quote, src.q, sd.toks, start, end)
	if quoteLabel || wordingLabel || wordingMedical || hasControl(wording) ||
		(containsIdentifier(wording) && !containsIdentifier(quote)) {
		// A clinician-attributed label, or a directive the user wrote
		// themselves, is shown only in the user's words.
		notes = append(notes, NoteWordingUnsafe)
	}
	if len(notes) > 0 {
		claim.Notes = append(claim.Notes, notes...)
		return claim, ""
	}
	claim.Wording = wording
	claim.FromQuote = false
	return claim, ""
}

// wordingNotes compares the model's wording with the accepted quote
// segToks[start:end] and returns one note per failed check.
//
//   - Negation: every content word the wording shares with the quote is
//     governed by as many negators as one of its occurrences in the quote,
//     the wording negates no word the quote lacks, and both are negated or
//     neither is.
//   - Subject: the same subject, and no third party the quote does not name.
//   - Time and certainty: every marker in the wording has the source's tag;
//     a wording without markers matches only "unspecified" or "stated".
//   - Number: no digit, number word or percentage absent from the quote.
//   - Added content: every other content word is a spelling of a quote word,
//     one of the segment's own time or certainty marker words, a generic
//     marker (freeMarkerLexicon; its tag is checked above), a reporting verb
//     (paraphraseWords) or a third party (checked as subject). Frequency and
//     severity words ("setiap", "parah"), new facts and translations are
//     therefore reverted.
func wordingNotes(wording, quote string, src Qualifiers, segToks []token, start, end int) []string {
	wtoks := tokenize(wording)
	w := analyze(wtoks, 0, len(wording)).q
	qs, qe := tokenRange(segToks, start, end)
	srcWords := polarWords(segToks, negationCounts(segToks, false), qs, qe)
	wWords := polarWords(wtoks, negationCounts(wtoks, false), 0, len(wtoks))

	var notes []string
	if w.Negated != src.Negated || !polarityKept(srcWords, wWords) {
		notes = append(notes, NoteWordingNegation)
	}
	if w.Subject != src.Subject || (namesOther(wtoks, 0, len(wtoks)) && !namesOther(segToks, qs, qe)) {
		notes = append(notes, NoteWordingSubject)
	}
	if !onlyTag(timeLexicon, wtoks, string(src.Time), string(TimeUnspecified)) {
		notes = append(notes, NoteWordingTime)
	}
	if !onlyTag(certaintyLexicon, wtoks, string(src.Certainty), string(CertaintyStated)) {
		notes = append(notes, NoteWordingCertainty)
	}
	if inventsNumber(wording, quote) {
		notes = append(notes, NoteWordingNumber)
	}
	if addsContent(wtoks, segToks[qs:qe], srcWords, markerParts(segToks)) {
		notes = append(notes, NoteWordingAddedContent)
	}
	return notes
}

// polarityKept reports whether every wording word that also occurs in the
// quote has the negation count of one of its quote occurrences, and no
// wording word outside the quote is negated.
func polarityKept(src, wording []polarWord) bool {
	for _, ww := range wording {
		found, same := false, false
		for _, sw := range src {
			if sameWord(ww.word, sw.word) {
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

// namesOther reports a token in toks[from:to] naming someone other than the
// user ("temanku", "dia", "my mom").
func namesOther(toks []token, from, to int) bool {
	for i := from; i < to; i++ {
		if classify(toks, i) == clsOther {
			return true
		}
	}
	return false
}

// onlyTag reports whether every lexicon match in toks is tagged want; with no
// match, want must be none.
func onlyTag(lx *lexicon, toks []token, want, none string) bool {
	hits := lx.scan(toks)
	if len(hits) == 0 {
		return want == none
	}
	for _, h := range hits {
		if h.tag != want {
			return false
		}
	}
	return true
}

// markerParts returns the word parts of the time and certainty markers in
// toks ("akhir" of "akhir-akhir ini", "lama" of "sudah lama").
func markerParts(toks []token) map[string]bool {
	out := map[string]bool{}
	for _, lx := range []*lexicon{timeLexicon, certaintyLexicon} {
		for _, h := range lx.scan(toks) {
			for _, t := range toks[h.start:h.end] {
				for _, p := range wordParts(t.text) {
					out[p] = true
				}
			}
		}
	}
	return out
}

// addsContent reports a wording word that adds to the quote: a content word
// or a time, frequency or severity marker (claimMarkers, including stopwords
// such as "lama") that is neither a spelling of a word of quoteToks, nor one
// of the segment's marker parts, nor part of a generic marker, nor a
// paraphrase word or third party.
func addsContent(wtoks, quoteToks []token, quote []polarWord, markers map[string]bool) bool {
	free := make([]bool, len(wtoks))
	for _, h := range freeMarkerLexicon.scan(wtoks) {
		for k := h.start; k < h.end; k++ {
			free[k] = true
		}
	}
	quoteParts := map[string]bool{}
	for _, t := range quoteToks {
		for _, p := range wordParts(t.text) {
			quoteParts[p] = true
		}
	}
	for i, t := range wtoks {
		if free[i] || isNegator(t.text) || isThirdNoun(t.text) {
			continue
		}
	parts:
		for _, p := range wordParts(t.text) {
			if markers[p] || quoteParts[p] {
				continue
			}
			if !claimMarkers[p] && (!isContent(p) || paraphraseWords[p]) {
				continue
			}
			for _, q := range quote {
				if sameWord(p, q.word) {
					continue parts
				}
			}
			return true
		}
	}
	return false
}

// meaningReason applies the quote-level Meaning Lock checks to one occurrence.
// A quote that starts after a governing negator is rejected even when it
// keeps another negator: "It's not that I don't care" quoted as "I don't
// care" reverses the meaning.
func meaningReason(a analysis, c Candidate, kind Kind) RejectReason {
	if a.negBefore {
		return RejectNegationDropped
	}
	if a.q.Subject == SubjectOther && lowerTrim(c.Subject) != string(SubjectOther) && selfKind(kind) {
		return RejectSubjectMismatch
	}
	return ""
}

// qualifiersDiffer reports whether the model's flags disagree with q.
func qualifiersDiffer(c Candidate, q Qualifiers) bool {
	return c.Negated != q.Negated || lowerTrim(c.Subject) != string(q.Subject) ||
		lowerTrim(c.Time) != string(q.Time) || lowerTrim(c.Certainty) != string(q.Certainty)
}

// widenToWords extends [start, end) so it does not cut a word.
func widenToWords(toks []token, start, end int) (int, int) {
	qs, qe := tokenRange(toks, start, end)
	if qs < qe {
		start = min(start, toks[qs].start)
		end = max(end, toks[qe-1].end)
	}
	return start, end
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) || isBidiControl(r) {
			return true
		}
	}
	return false
}

func isBidiControl(r rune) bool {
	return (r >= '‪' && r <= '‮') || (r >= '⁦' && r <= '⁩') ||
		r == '‎' || r == '‏' || r == '؜'
}

// ManualClaim builds a claim from a whole segment chosen by the user: the
// trimmed segment text is both quote and wording, with Analyze qualifiers.
// The caller is responsible for passing a valid kind.
func ManualClaim(seg Segment, kind Kind) Claim {
	text := seg.Text
	start := len(text) - len(strings.TrimLeftFunc(text, unicode.IsSpace))
	end := len(strings.TrimRightFunc(text, unicode.IsSpace))
	if end < start {
		start, end = 0, 0
	}
	quote := text[start:end]
	return Claim{
		SegmentID:  seg.ID,
		Quote:      quote,
		Start:      start,
		End:        end,
		Kind:       kind,
		Wording:    quote,
		FromQuote:  true,
		Qualifiers: Analyze(seg, start, end),
	}
}

// ComputeUnknowns lists, in CoreFields order, the core fields not covered by
// claims: concern (no feeling, experience or worry), duration, daily_impact
// (no impact), goal and support_preference (no preference). The result is
// never nil.
func ComputeUnknowns(claims []Claim) []UnknownField {
	has := map[Kind]bool{}
	for _, c := range claims {
		has[c.Kind] = true
	}
	covered := map[UnknownField]bool{
		UnknownConcern:    has[KindFeeling] || has[KindExperience] || has[KindWorry],
		UnknownDuration:   has[KindDuration],
		UnknownImpact:     has[KindImpact],
		UnknownGoal:       has[KindGoal],
		UnknownPreference: has[KindPreference],
	}
	out := []UnknownField{}
	for _, f := range CoreFields {
		if !covered[f] {
			out = append(out, f)
		}
	}
	return out
}

// ValidateUserWording checks a user's own edit: valid UTF-8, non-empty after
// trimming, at most MaxEditRunes runes, no control or bidi-override
// characters except single line breaks ("\n"; blank lines are rejected).
// User wording may mention a diagnosis: it is the user's own words. Callers
// store strings.TrimSpace(text).
func ValidateUserWording(text string) error {
	if !utf8.ValidString(text) {
		return ErrWordingInvalid
	}
	t := strings.TrimSpace(text)
	if t == "" {
		return ErrWordingEmpty
	}
	if runeLen(t) > MaxEditRunes {
		return ErrWordingTooLong
	}
	newline := false
	for _, r := range t {
		switch {
		case r == '\n':
			if newline {
				return ErrWordingInvalid
			}
			newline = true
		case r == ' ':
		case unicode.IsControl(r) || isBidiControl(r):
			return ErrWordingInvalid
		default:
			newline = false
		}
	}
	return nil
}
