// Package ledger implements Story Ledger grounding ("Meaning Lock") and the
// Opening Practice checks.
//
// The model only proposes. Every claim it keeps must quote one story segment
// verbatim, keep the segment's negation, subject, time and certainty, and
// must not add a diagnosis, medication instruction or invented number. When
// only the model's wording is wrong, the claim survives with the user's own
// words and a note; when the quote itself is wrong, the claim is rejected.
// Unknown facts stay unknown.
//
// This ports and extends the inherited Dengar.in Care Passport validator
// (services/validator/src/carePassport.ts) and opening-lines validator
// (services/validator/src/openingLines.ts). Deterministic checks catch known
// failure classes only; the user's review remains the final check.
//
// Nothing here logs or performs I/O.
package ledger

// SchemaVersion is the version of the model draft contract (DraftJSONSchema).
const SchemaVersion = "sela-ledger-draft-v1"

// ValidatorVersion identifies the validation rules.
const ValidatorVersion = "sela-meaning-lock-2026.10.26-1"

// Limits on model drafts and user edits.
const (
	MaxClaims        = 12
	MaxQuoteRunes    = 300
	MaxWordingRunes  = 300
	MaxEditRunes     = 400
	MinQuoteRunes    = 3
	MaxDraftBytes    = 64 * 1024
	MaxOpeningLines  = 3
	MinOpeningLines  = 1
	MaxOpeningRunes  = 240
	MaxPracticeRunes = 1200
)

// Segment is one immutable sentence-level span of the user's story or of a
// clarification answer. IDs are assigned by the journey service ("s1"...).
type Segment struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Origin string `json:"origin"` // "story" | "clarification"
}

// Kind classifies a claim. It describes what the user said; it is never a
// clinical category.
type Kind string

const (
	KindFeeling    Kind = "feeling"    // emotional state the user reports
	KindExperience Kind = "experience" // something the user says happens/happened
	KindImpact     Kind = "impact"     // effect on daily life (study, work, sleep, relationships)
	KindDuration   Kind = "duration"   // since when / how long, as stated
	KindContext    Kind = "context"    // circumstances, including things others said
	KindWorry      Kind = "worry"      // fear or worry about something that may happen
	KindGoal       Kind = "goal"       // what the user wants from support
	KindPreference Kind = "preference" // practical preference (language, format, cost, time)
	KindQuestion   Kind = "question"   // question the user wants to ask a professional
)

// AllKinds lists valid kinds in display order.
var AllKinds = []Kind{KindFeeling, KindExperience, KindImpact, KindDuration, KindContext, KindWorry, KindGoal, KindPreference, KindQuestion}

// Subject is who the statement is about.
type Subject string

const (
	SubjectSelf  Subject = "self"
	SubjectOther Subject = "other"
)

// Time is the temporal frame stated in the source.
type Time string

const (
	TimeCurrent     Time = "current"
	TimePast        Time = "past"
	TimeFuture      Time = "future"
	TimeUnspecified Time = "unspecified"
)

// Certainty is how certain the user sounded.
type Certainty string

const (
	CertaintyStated       Certainty = "stated"
	CertaintyUncertain    Certainty = "uncertain"    // mungkin, kayaknya, not sure
	CertaintyHypothetical Certainty = "hypothetical" // kalau nanti, what if, takut kalau
)

// Qualifiers are the meaning-bearing properties Meaning Lock protects.
type Qualifiers struct {
	Negated   bool      `json:"negated"`
	Subject   Subject   `json:"subject"`
	Time      Time      `json:"time"`
	Certainty Certainty `json:"certainty"`
}

// Candidate is one claim exactly as the model proposed it (untrusted).
type Candidate struct {
	SegmentID string `json:"segment_id"`
	Quote     string `json:"quote"`
	Kind      string `json:"kind"`
	Text      string `json:"text"`
	Subject   string `json:"subject"`
	Time      string `json:"time"`
	Certainty string `json:"certainty"`
	Negated   bool   `json:"negated"`
}

// ClarificationCandidate is the model's single optional clarification pick.
// The question text always comes from Sela's own templates, never the model.
type ClarificationCandidate struct {
	Field string `json:"field"` // an UnknownField value, or "none"
}

// Draft is the parsed model output.
type Draft struct {
	Claims        []Candidate             `json:"claims"`
	Unknowns      []string                `json:"unknowns"`
	Clarification *ClarificationCandidate `json:"clarification"`
}

// UnknownField names a core fact the brief does not have yet.
type UnknownField string

const (
	UnknownConcern    UnknownField = "concern"
	UnknownDuration   UnknownField = "duration"
	UnknownImpact     UnknownField = "daily_impact"
	UnknownGoal       UnknownField = "goal"
	UnknownPreference UnknownField = "support_preference"
)

// CoreFields lists the unknown fields in display order.
var CoreFields = []UnknownField{UnknownConcern, UnknownDuration, UnknownImpact, UnknownGoal, UnknownPreference}

// Note codes recorded on accepted claims whose model wording was replaced by
// the user's own quote.
const (
	NoteWordingNegation  = "wording_reverted:negation"
	NoteWordingSubject   = "wording_reverted:subject"
	NoteWordingTime      = "wording_reverted:time"
	NoteWordingCertainty = "wording_reverted:certainty"
	NoteWordingNumber    = "wording_reverted:invented_number"
	NoteWordingUnsafe    = "wording_reverted:unsafe_content"
	NoteWordingTooLong   = "wording_reverted:too_long"
	NoteQualifierFixed   = "qualifier_corrected" // model's qualifier flags disagreed with the source and were replaced
)

// RejectReason is a stable code for a rejected candidate.
type RejectReason string

const (
	RejectUnknownSegment  RejectReason = "unknown_segment"
	RejectQuoteNotFound   RejectReason = "quote_not_found"
	RejectQuoteTooShort   RejectReason = "quote_too_short"
	RejectQuoteTooLong    RejectReason = "quote_too_long"
	RejectNegationDropped RejectReason = "negation_dropped" // quote omits a negator governing it in the source clause
	RejectSubjectMismatch RejectReason = "subject_mismatch" // source is about someone else, claim presented as the user's own
	RejectDiagnosis       RejectReason = "diagnosis_label"
	RejectMedical         RejectReason = "medical_instruction"
	RejectInvalidKind     RejectReason = "invalid_kind"
	RejectDuplicate       RejectReason = "duplicate"
	RejectLimit           RejectReason = "limit_exceeded"
	RejectEmpty           RejectReason = "empty"
)

// Claim is a validated claim. Quote is always a verbatim substring of the
// referenced segment; Start/End are byte offsets of Quote in Segment.Text.
type Claim struct {
	SegmentID  string     `json:"segmentId"`
	Quote      string     `json:"quote"`
	Start      int        `json:"start"`
	End        int        `json:"end"`
	Kind       Kind       `json:"kind"`
	Wording    string     `json:"wording"`
	FromQuote  bool       `json:"fromQuote"` // Wording is the verbatim quote (model wording reverted or absent)
	Qualifiers Qualifiers `json:"qualifiers"`
	Notes      []string   `json:"notes,omitempty"`
}

// Rejection records why a candidate (by index in Draft.Claims) was dropped.
type Rejection struct {
	Index  int          `json:"index"`
	Reason RejectReason `json:"reason"`
}

// Result is the outcome of ValidateDraft.
type Result struct {
	Accepted      []Claim        `json:"accepted"`
	Rejected      []Rejection    `json:"rejected"`
	Unknowns      []UnknownField `json:"unknowns"`
	Clarification UnknownField   `json:"clarification,omitempty"` // "" when none
}

// ApprovedItem is one item of an approved brief, used by Opening Practice.
type ApprovedItem struct {
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`
	Text string `json:"text"`
}

// OpeningLine is a validated opening sentence.
type OpeningLine struct {
	Text    string   `json:"text"`
	BasedOn []string `json:"basedOn"` // ApprovedItem IDs, recomputed server-side
}

// OpeningRejectReason is a stable code for a rejected opening line.
type OpeningRejectReason string

const (
	OpeningUngrounded   OpeningRejectReason = "ungrounded" // adds content words not in approved items or the allowed intent vocabulary
	OpeningNumber       OpeningRejectReason = "invented_number"
	OpeningDiagnosis    OpeningRejectReason = "diagnosis_label"
	OpeningMedical      OpeningRejectReason = "medical_instruction"
	OpeningIdentifier   OpeningRejectReason = "identifier"
	OpeningTooLong      OpeningRejectReason = "too_long"
	OpeningEmpty        OpeningRejectReason = "empty"
	OpeningLimit        OpeningRejectReason = "limit_exceeded"
	OpeningManipulative OpeningRejectReason = "pressure_language" // urgency, guilt, guarantees ("pasti sembuh", "you must")
)

// OpeningRejection records a dropped line by index.
type OpeningRejection struct {
	Index  int                 `json:"index"`
	Reason OpeningRejectReason `json:"reason"`
}

// OpeningResult is the outcome of ValidateOpening.
type OpeningResult struct {
	Lines    []OpeningLine      `json:"lines"`
	Rejected []OpeningRejection `json:"rejected"`
}

// OpeningDraft is the parsed model output for Opening Practice.
type OpeningDraft struct {
	Lines []string `json:"lines"`
}

// Coverage is the deterministic, non-judgemental practice feedback: which
// approved items the user's own practice attempt mentions. It is not a score.
type Coverage struct {
	Mentioned    []string `json:"mentioned"`    // ApprovedItem IDs
	NotMentioned []string `json:"notMentioned"` // ApprovedItem IDs
}
