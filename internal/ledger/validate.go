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
// starts after a negator that governs it (negation_dropped); when the source
// is about someone else but the claim presents it as the user's own feeling,
// experience, impact or duration (subject_mismatch); when the quote or the
// model's wording carries a clinical label, except a clinician-attributed
// context claim (diagnosis_label); when the model's wording contains a
// medication or treatment directive (medical_instruction); when it repeats an
// accepted (segment, quote) pair (duplicate); or beyond MaxClaims.
//
// Accepted claims always carry the deterministic source qualifiers. Their
// quote is the verbatim segment span, widened to whole words. The model's
// wording is kept only when it preserves negation, subject, time and
// certainty, adds no number and no unsafe content, and fits MaxWordingRunes;
// otherwise the quote is used and a note is recorded per failed check.
// Unknowns are recomputed from the accepted claims; the model's unknowns are
// ignored and its clarification pick survives only if it is still unknown.
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
	clinicianContext := kind == KindContext && clinicianAttributed(quote)
	quoteLabel := ContainsDiagnosisLabel(quote)
	wordingLabel := !verbatim && ContainsDiagnosisLabel(wording)
	if (quoteLabel || wordingLabel) && !clinicianContext {
		return Claim{}, RejectDiagnosis
	}
	wordingMedical := !verbatim && ContainsMedicalInstruction(wording)
	if wordingMedical && !clinicianContext {
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

	var notes []string
	if runeLen(wording) > MaxWordingRunes {
		notes = append(notes, NoteWordingTooLong)
	}
	w := analyze(tokenize(wording), 0, len(wording)).q
	if w.Negated != src.q.Negated {
		notes = append(notes, NoteWordingNegation)
	}
	if w.Subject != src.q.Subject {
		notes = append(notes, NoteWordingSubject)
	}
	if w.Time != src.q.Time {
		notes = append(notes, NoteWordingTime)
	}
	if w.Certainty != src.q.Certainty {
		notes = append(notes, NoteWordingCertainty)
	}
	if inventsNumber(wording, quote) {
		notes = append(notes, NoteWordingNumber)
	}
	if quoteLabel || wordingLabel || wordingMedical || hasControl(wording) ||
		(containsIdentifier(wording) && !containsIdentifier(quote)) {
		// A clinician-attributed label is shown only in the user's words.
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

// meaningReason applies the quote-level Meaning Lock checks to one occurrence.
func meaningReason(a analysis, c Candidate, kind Kind) RejectReason {
	if a.negBefore && !a.negInside {
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
