package journey

import (
	"time"

	"github.com/YanIanZ/sela/internal/ledger"
	"github.com/YanIanZ/sela/internal/safety"
)

// Provenance records where content came from (SPEC V10).
type Provenance string

const (
	// ProvenanceLiveMeta: a successful request to the configured Meta
	// open-weight model on Modal whose reported model matched the pin.
	ProvenanceLiveMeta Provenance = "live_meta"
	// ProvenanceRecorded: a recorded synthetic result, labelled as such.
	ProvenanceRecorded Provenance = "recorded_demo"
	// ProvenanceManual: selected or written by the user without a model.
	ProvenanceManual Provenance = "manual"
	// ProvenanceTestDouble: produced by cmd/fakellm in tests/CI/dev.
	// Never presented as Meta inference.
	ProvenanceTestDouble Provenance = "test_double"
)

// ConsentKind identifies an independent decision (SPEC V4, amended).
type ConsentKind string

const (
	ConsentAI      ConsentKind = "ai_processing"
	ConsentStorage ConsentKind = "storage"
)

// Consent is one versioned decision.
type Consent struct {
	Granted       bool      `json:"granted"`
	PolicyVersion string    `json:"policyVersion"`
	DecidedAt     time.Time `json:"decidedAt"`
	// Revision increments on every decision; AI operations capture it and
	// discard their result if it changed.
	Revision int `json:"revision"`
}

// IdentityMode is how the user appears to recipients.
type IdentityMode string

const (
	IdentityAnonymous IdentityMode = "anonymous"
	IdentityPseudonym IdentityMode = "pseudonym"
)

// Identity is the user's chosen presentation. No real identity is collected.
type Identity struct {
	Mode      IdentityMode `json:"mode"`
	Pseudonym string       `json:"pseudonym,omitempty"`
}

// StoryMode is how the story was provided.
type StoryMode string

const (
	StoryScenario StoryMode = "scenario" // seeded synthetic fixture
	StoryPractice StoryMode = "practice" // user-written practice text (bounded, labelled fictional)
)

// Story is the current story version. Segments are immutable per version.
type Story struct {
	Version    int              `json:"version"`
	Mode       StoryMode        `json:"mode"`
	ScenarioID string           `json:"scenarioId,omitempty"`
	Segments   []ledger.Segment `json:"segments"`
	Submitted  time.Time        `json:"submittedAt"`
}

// SafetyState is the session's routing state (SPEC V3).
type SafetyState struct {
	Level   safety.Level `json:"level"`
	RuleIDs []string     `json:"ruleIds,omitempty"`
	// Interrupted stays true once a crisis route was taken in this session:
	// no further model call, handoff or follow-up plan is created.
	Interrupted bool      `json:"interrupted"`
	At          time.Time `json:"at,omitempty"`
	ThirdParty  bool      `json:"thirdParty,omitempty"`
}

// ItemStatus is the user's review state for a ledger item.
type ItemStatus string

const (
	ItemUnreviewed ItemStatus = "unreviewed"
	ItemConfirmed  ItemStatus = "confirmed"
)

// WordingOrigin says who wrote the displayed wording.
type WordingOrigin string

const (
	WordingModel WordingOrigin = "model" // Sela's AI-proposed paraphrase (interpretation)
	WordingQuote WordingOrigin = "quote" // the user's own words verbatim
	WordingUser  WordingOrigin = "user"  // the user's own edit
)

// Item is one Story Ledger entry. Quote is always the user's own statement;
// Wording is either that quote, the model's interpretation or the user's edit.
type Item struct {
	ID            string            `json:"id"`
	Kind          ledger.Kind       `json:"kind"`
	SegmentID     string            `json:"segmentId"`
	Quote         string            `json:"quote"`
	Start         int               `json:"start"`
	End           int               `json:"end"`
	Wording       string            `json:"wording"`
	WordingOrigin WordingOrigin     `json:"wordingOrigin"`
	Qualifiers    ledger.Qualifiers `json:"qualifiers"`
	Status        ItemStatus        `json:"status"`
	Provenance    Provenance        `json:"provenance"`
	Notes         []string          `json:"notes,omitempty"`
}

// Approval binds a user approval to one exact brief version.
type Approval struct {
	Version int       `json:"version"`
	Hash    string    `json:"hash"`
	At      time.Time `json:"at"`
}

// Ledger is the Story Ledger (the brief draft) for the current story.
type Ledger struct {
	// Version increments on every content change; approval is cleared.
	Version  int                   `json:"version"`
	Items    []Item                `json:"items"`
	Unknowns []ledger.UnknownField `json:"unknowns"`
	// Skipped lists unknown fields the user chose not to answer.
	Skipped []ledger.UnknownField `json:"skipped,omitempty"`
	// Clarification is the single suggested question (field), if any.
	Clarification ledger.UnknownField `json:"clarification,omitempty"`
	Provenance    Provenance          `json:"provenance"`
	Model         string              `json:"model,omitempty"`
	StoryVersion  int                 `json:"storyVersion"`
	Approved      *Approval           `json:"approved,omitempty"`
	// Rejected counts model claims Meaning Lock dropped, by reason.
	Rejected map[string]int `json:"rejected,omitempty"`
	nextItem int
}

// BriefSnapshot is an immutable copy of an approved ledger version.
type BriefSnapshot struct {
	Version  int                   `json:"version"`
	Hash     string                `json:"hash"`
	Language string                `json:"language"`
	Items    []Item                `json:"items"`
	Unknowns []ledger.UnknownField `json:"unknowns"`
	At       time.Time             `json:"at"`
}

// Preferences are explicit practical preferences (SPEC V8).
type Preferences struct {
	Language     string `json:"language"`     // id | en | either
	Format       string `json:"format"`       // online | in_person | either
	Budget       string `json:"budget"`       // free_only | low_or_free | any
	Availability string `json:"availability"` // weekday_morning | weekday_daytime | weekday_evening | weekend | any
	Barrier      string `json:"barrier"`      // cost | schedule | how_to_start | privacy | not_ready | none
	Version      int    `json:"version"`
}

// Opening holds Opening Practice lines.
type Opening struct {
	Lines        []ledger.OpeningLine `json:"lines"`
	Provenance   Provenance           `json:"provenance"`
	Model        string               `json:"model,omitempty"`
	BriefVersion int                  `json:"briefVersion"`
	UserEdited   bool                 `json:"userEdited"`
	Version      int                  `json:"version"`
}

// HandoffStatus is the simulated request state (SPEC V5, amended).
type HandoffStatus string

const (
	HandoffRequested     HandoffStatus = "requested"
	HandoffAcknowledged  HandoffStatus = "acknowledged"
	HandoffClarification HandoffStatus = "clarification_requested"
	HandoffDeclined      HandoffStatus = "declined"
	HandoffCompleted     HandoffStatus = "completed"
)

// ShareState is the access state of a handoff's shared projection.
type ShareState string

const (
	ShareActive  ShareState = "active"
	ShareRevoked ShareState = "revoked"
	ShareExpired ShareState = "expired"
	ShareLocked  ShareState = "locked" // too many wrong access codes
)

// ShareField is a section the user chose to share.
type ShareField string

const (
	FieldItems       ShareField = "items"        // approved ledger wording (required)
	FieldQuotes      ShareField = "quotes"       // the user's original words beside each item
	FieldUnknowns    ShareField = "unknowns"     // what is not known yet
	FieldOpening     ShareField = "opening"      // opening lines
	FieldPreferences ShareField = "preferences"  // practical preferences
	FieldDisplayName ShareField = "display_name" // pseudonym (or "anonymous")
	FieldAgeRange    ShareField = "age_range"    // "adult (18+)", attested
	FieldLanguage    ShareField = "language"     // preferred language
)

// AllShareFields lists share fields in display order.
var AllShareFields = []ShareField{FieldItems, FieldQuotes, FieldUnknowns, FieldOpening, FieldPreferences, FieldDisplayName, FieldAgeRange, FieldLanguage}

// TimelineEvent is a receipt timeline entry. Metadata only.
type TimelineEvent struct {
	Kind   string    `json:"kind"`
	At     time.Time `json:"at"`
	Actor  string    `json:"actor"` // user | recipient | system
	Detail string    `json:"detail,omitempty"`
}

// Handoff is a simulated request to a fictional recipient with its share grant.
type Handoff struct {
	ID             string          `json:"id"`
	Version        int             `json:"version"`
	ProfessionalID string          `json:"professionalId"`
	SlotID         string          `json:"slotId"`
	Brief          BriefSnapshot   `json:"brief"`
	Fields         []ShareField    `json:"fields"`
	Projection     Projection      `json:"projection"`
	Status         HandoffStatus   `json:"status"`
	Share          ShareState      `json:"share"`
	ShareReason    string          `json:"shareReason,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	ExpiresAt      time.Time       `json:"expiresAt"`
	ConsentPolicy  string          `json:"consentPolicy"`
	Timeline       []TimelineEvent `json:"timeline"`
	ClarifyReason  string          `json:"clarifyReason,omitempty"`
	DeclineReason  string          `json:"declineReason,omitempty"`
	PlanID         string          `json:"planId,omitempty"`
	ReplacesID     string          `json:"replacesId,omitempty"`
	// Token and Code are capability secrets kept only in the owner's
	// memory-only session so the owner can copy the link again. The
	// recipient index stores only their SHA-256.
	Token        string `json:"-"`
	Code         string `json:"-"`
	CodeAttempts int    `json:"-"`
	idemKey      string
}

// Projection is exactly what the recipient may see; computed once at share
// time from the approved snapshot and the selected fields (SPEC V5).
type Projection struct {
	Synthetic    bool                  `json:"synthetic"`
	Language     string                `json:"language"`
	BriefVersion int                   `json:"briefVersion"`
	Items        []ProjectedItem       `json:"items"`
	Unknowns     []ledger.UnknownField `json:"unknowns,omitempty"`
	Opening      []string              `json:"opening,omitempty"`
	Preferences  *Preferences          `json:"preferences,omitempty"`
	DisplayName  string                `json:"displayName,omitempty"`
	AgeRange     string                `json:"ageRange,omitempty"`
	Preferred    string                `json:"preferredLanguage,omitempty"`
	SlotID       string                `json:"slotId"`
	Provenance   Provenance            `json:"provenance"`
}

// ProjectedItem is a shared ledger item.
type ProjectedItem struct {
	Kind       ledger.Kind       `json:"kind"`
	Text       string            `json:"text"`
	Quote      string            `json:"quote,omitempty"`
	Qualifiers ledger.Qualifiers `json:"qualifiers"`
}

// CheckIn is a seeded follow-up check-in.
type CheckIn struct {
	ID        string    `json:"id"`
	HandoffID string    `json:"handoffId"`
	OptionID  string    `json:"optionId"`
	At        time.Time `json:"at"`
}

// OpKind is an AI operation type.
type OpKind string

const (
	OpLedgerDraft  OpKind = "ledger_draft"
	OpOpeningDraft OpKind = "opening_draft"
)

// OpState is an AI operation lifecycle state.
type OpState string

const (
	OpQueued    OpState = "queued"
	OpRunning   OpState = "running"
	OpSucceeded OpState = "succeeded"
	OpFailed    OpState = "failed"
	OpCanceled  OpState = "canceled"
)

// OpRecord is the session-visible record of an AI operation.
type OpRecord struct {
	ID         string     `json:"id"`
	Kind       OpKind     `json:"kind"`
	State      OpState    `json:"state"`
	ErrorCode  string     `json:"errorCode,omitempty"`
	Provenance Provenance `json:"provenance,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	EndedAt    time.Time  `json:"endedAt,omitempty"`
	// Captured state; the result commits only if these still match.
	StoryVersion    int `json:"-"`
	ConsentRevision int `json:"-"`
	BriefVersion    int `json:"-"`
}

// Retention is the active retention policy.
type Retention string

const (
	RetentionSession Retention = "session" // 30 min idle, 2 h max
	RetentionDay     Retention = "day"     // up to 24 h (storage consent)
)

// Session is one isolated demo visitor's state. It is only accessed through
// Store.View/Store.Update, which serialise access per session.
type Session struct {
	ID            string                  `json:"-"`
	Version       int                     `json:"version"`
	CreatedAt     time.Time               `json:"createdAt"`
	LastSeen      time.Time               `json:"lastSeen"`
	ExpiresAt     time.Time               `json:"expiresAt"`
	Language      string                  `json:"language"`
	Identity      Identity                `json:"identity"`
	AdultAttested bool                    `json:"adultAttested"`
	PolicyVersion string                  `json:"policyVersion"`
	Consents      map[ConsentKind]Consent `json:"consents"`
	Retention     Retention               `json:"retention"`
	Safety        SafetyState             `json:"safety"`
	Story         *Story                  `json:"story,omitempty"`
	Ledger        *Ledger                 `json:"ledger,omitempty"`
	Snapshots     []BriefSnapshot         `json:"-"`
	Preferences   *Preferences            `json:"preferences,omitempty"`
	Opening       *Opening                `json:"opening,omitempty"`
	Handoffs      []*Handoff              `json:"handoffs"`
	CheckIns      []CheckIn               `json:"checkIns"`
	Ops           []*OpRecord             `json:"ops"`
	idem          map[string]idemRecord
	nextHandoff   int
}

type idemRecord struct {
	payloadHash string
	handoffID   string
}

// consentRevision returns the AI consent revision (0 if never decided).
func (s *Session) consent(kind ConsentKind) Consent {
	if s.Consents == nil {
		return Consent{}
	}
	return s.Consents[kind]
}

// handoff finds a handoff by id.
func (s *Session) handoff(id string) *Handoff {
	for _, h := range s.Handoffs {
		if h.ID == id {
			return h
		}
	}
	return nil
}

// op finds an operation record by id.
func (s *Session) op(id string) *OpRecord {
	for _, o := range s.Ops {
		if o.ID == id {
			return o
		}
	}
	return nil
}
