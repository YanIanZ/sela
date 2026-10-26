package journey

import (
	"time"

	"github.com/YanIanZ/sela/internal/ledger"
)

// SessionView is the owner's complete, user-visible state. It contains the
// owner's own data only and never credentials (the session id is a cookie).
// It shares no memory with the session: callers encode it after the session
// lock is released while AI operations keep updating the session.
type SessionView struct {
	Version       int                     `json:"version"`
	Synthetic     bool                    `json:"synthetic"`
	Language      string                  `json:"language"`
	Identity      Identity                `json:"identity"`
	CreatedAt     time.Time               `json:"createdAt"`
	ExpiresAt     time.Time               `json:"expiresAt"`
	IdleExpiresAt time.Time               `json:"idleExpiresAt"`
	Retention     Retention               `json:"retention"`
	Consents      map[ConsentKind]Consent `json:"consents"`
	Policies      map[string]string       `json:"policies"`
	Safety        SafetyView              `json:"safety"`
	Stage         string                  `json:"stage"`
	Story         *StoryView              `json:"story,omitempty"`
	Ledger        *LedgerView             `json:"ledger,omitempty"`
	Preferences   *Preferences            `json:"preferences,omitempty"`
	Opening       *Opening                `json:"opening,omitempty"`
	Handoffs      []HandoffView           `json:"handoffs"`
	CheckIns      []CheckIn               `json:"checkIns"`
	Ops           []*OpRecord             `json:"ops"`
	AI            AIStatus                `json:"ai"`
	FixturesVer   string                  `json:"fixturesVersion"`
}

// SafetyView is the routing state shown to the user.
type SafetyView struct {
	Level       string `json:"level"`
	Interrupted bool   `json:"interrupted"`
	ThirdParty  bool   `json:"thirdParty,omitempty"`
}

// StoryView is the current story.
type StoryView struct {
	Version    int              `json:"version"`
	Mode       StoryMode        `json:"mode"`
	ScenarioID string           `json:"scenarioId,omitempty"`
	Segments   []ledger.Segment `json:"segments"`
}

// LedgerView is the Story Ledger with review progress.
type LedgerView struct {
	Version       int                   `json:"version"`
	Items         []Item                `json:"items"`
	Unknowns      []ledger.UnknownField `json:"unknowns"`
	Skipped       []ledger.UnknownField `json:"skipped"`
	Clarification ledger.UnknownField   `json:"clarification,omitempty"`
	Provenance    Provenance            `json:"provenance"`
	Model         string                `json:"model,omitempty"`
	Approved      *Approval             `json:"approved,omitempty"`
	Reviewed      int                   `json:"reviewed"`
	Rejected      map[string]int        `json:"rejected,omitempty"`
}

// HandoffView is the owner's receipt.
type HandoffView struct {
	ID             string          `json:"id"`
	Version        int             `json:"version"`
	ProfessionalID string          `json:"professionalId"`
	SlotID         string          `json:"slotId"`
	BriefVersion   int             `json:"briefVersion"`
	Fields         []ShareField    `json:"fields"`
	Projection     Projection      `json:"projection"`
	Status         HandoffStatus   `json:"status"`
	Share          ShareState      `json:"share"`
	ShareReason    string          `json:"shareReason,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	ExpiresAt      time.Time       `json:"expiresAt"`
	Timeline       []TimelineEvent `json:"timeline"`
	ClarifyReason  string          `json:"clarifyReason,omitempty"`
	DeclineReason  string          `json:"declineReason,omitempty"`
	PlanID         string          `json:"planId,omitempty"`
	ReplacesID     string          `json:"replacesId,omitempty"`
	// Opened reports whether the recipient has opened the share.
	Opened bool `json:"opened"`
	// CareReceived is always false: a simulated request or consultation in
	// this demo is never care received (SPEC V1).
	CareReceived bool `json:"careReceived"`
}

func (s *Service) handoffView(sess *Session, h *Handoff) HandoffView {
	opened := false
	for _, e := range h.Timeline {
		opened = opened || e.Kind == "opened"
	}
	return HandoffView{
		ID: h.ID, Version: h.Version, ProfessionalID: h.ProfessionalID, SlotID: h.SlotID, BriefVersion: h.Brief.Version,
		Fields: h.Fields, Projection: h.Projection, Status: h.Status, Share: s.shareState(h, s.now()), ShareReason: h.ShareReason,
		CreatedAt: h.CreatedAt, ExpiresAt: shareExpiry(sess, h), Timeline: append([]TimelineEvent{}, h.Timeline...),
		ClarifyReason: h.ClarifyReason, DeclineReason: h.DeclineReason, PlanID: h.PlanID, ReplacesID: h.ReplacesID, Opened: opened,
	}
}

// stage is a coarse journey position for navigation; it never gates rules.
func stage(sess *Session) string {
	switch {
	case sess.Safety.Interrupted:
		return "crisis"
	case len(sess.Handoffs) > 0:
		return "journey"
	case sess.Ledger != nil && sess.Ledger.Approved != nil && sess.Ledger.Approved.Version == sess.Ledger.Version:
		return "next_step"
	case sess.Story != nil:
		return "ledger"
	case sess.consent(ConsentAI).DecidedAt.IsZero():
		return "consent"
	default:
		return "story"
	}
}

func (s *Service) buildView(sess *Session) *SessionView {
	v := &SessionView{
		Version: sess.Version, Synthetic: true, Language: sess.Language, Identity: sess.Identity,
		CreatedAt: sess.CreatedAt, ExpiresAt: sess.ExpiresAt, IdleExpiresAt: sess.IdleDeadline(),
		Retention: sess.Retention, Consents: map[ConsentKind]Consent{},
		Policies: map[string]string{
			"privacy": s.policy("privacy"), "ai_processing": s.policy("ai_processing"),
			"storage": s.policy("storage"), "sharing": s.policy("sharing"),
		},
		Safety:      SafetyView{Level: string(maxLevel(sess.Safety.Level, "")), Interrupted: sess.Safety.Interrupted, ThirdParty: sess.Safety.ThirdParty},
		Stage:       stage(sess),
		Handoffs:    []HandoffView{},
		CheckIns:    append([]CheckIn{}, sess.CheckIns...),
		Ops:         make([]*OpRecord, 0, len(sess.Ops)),
		AI:          s.AIStatus(),
		FixturesVer: s.catalog.Version,
	}
	for k, c := range sess.Consents {
		v.Consents[k] = c
	}
	// Records are copied by value: the operation goroutine updates them
	// under the session lock while the view is encoded without it.
	for _, o := range sess.Ops {
		rec := *o
		v.Ops = append(v.Ops, &rec)
	}
	if sess.Preferences != nil {
		p := *sess.Preferences
		v.Preferences = &p
	}
	if sess.Opening != nil {
		o := *sess.Opening
		o.Lines = append([]ledger.OpeningLine{}, o.Lines...)
		v.Opening = &o
	}
	if sess.Story != nil {
		v.Story = &StoryView{Version: sess.Story.Version, Mode: sess.Story.Mode, ScenarioID: sess.Story.ScenarioID, Segments: append([]ledger.Segment{}, sess.Story.Segments...)}
	}
	if l := sess.Ledger; l != nil {
		lv := &LedgerView{Version: l.Version, Items: append([]Item{}, l.Items...), Unknowns: append([]ledger.UnknownField{}, l.Unknowns...),
			Skipped: append([]ledger.UnknownField{}, l.Skipped...), Clarification: l.Clarification, Provenance: l.Provenance, Model: l.Model}
		if l.Rejected != nil {
			lv.Rejected = make(map[string]int, len(l.Rejected))
			for k, n := range l.Rejected {
				lv.Rejected[k] = n
			}
		}
		if l.Approved != nil && l.Approved.Version == l.Version {
			a := *l.Approved
			lv.Approved = &a
		}
		for _, it := range l.Items {
			if it.Status == ItemConfirmed {
				lv.Reviewed++
			}
		}
		v.Ledger = lv
	}
	for _, h := range sess.Handoffs {
		v.Handoffs = append(v.Handoffs, s.handoffView(sess, h))
	}
	return v
}
