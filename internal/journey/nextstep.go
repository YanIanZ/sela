package journey

import (
	"github.com/YanIanZ/sela/internal/fixtures"
)

// PreferencesInput sets explicit practical preferences (SPEC V8).
type PreferencesInput struct {
	ExpectedVersion int    `json:"expectedVersion"`
	Language        string `json:"language"`
	Format          string `json:"format"`
	Budget          string `json:"budget"`
	Availability    string `json:"availability"`
	Barrier         string `json:"barrier"`
}

func oneOf(v string, set ...string) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

// SetPreferences stores preferences. They are user choices, never a
// readiness or clinical score, and they do not change the ledger.
func (s *Service) SetPreferences(sessionID string, in PreferencesInput) (*SessionView, error) {
	switch {
	case !oneOf(in.Language, "id", "en", "either"):
		return nil, errf(CodeInvalid, 400, "language")
	case !oneOf(in.Format, "online", "in_person", "either"):
		return nil, errf(CodeInvalid, 400, "format")
	case !oneOf(in.Budget, "free_only", "low_or_free", "any"):
		return nil, errf(CodeInvalid, 400, "budget")
	case !oneOf(in.Availability, "weekday_morning", "weekday_daytime", "weekday_evening", "weekend", "any"):
		return nil, errf(CodeInvalid, 400, "availability")
	case !oneOf(in.Barrier, "cost", "schedule", "how_to_start", "privacy", "not_ready", "none"):
		return nil, errf(CodeInvalid, 400, "barrier")
	}
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		if sess.Safety.Interrupted {
			return errCrisis
		}
		v := 1
		if sess.Preferences != nil {
			v = sess.Preferences.Version + 1
		}
		sess.Preferences = &Preferences{Language: in.Language, Format: in.Format, Budget: in.Budget, Availability: in.Availability, Barrier: in.Barrier, Version: v}
		bump(sess)
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// Reason is a machine-readable explanation of a match; the UI renders it.
type Reason struct {
	Criterion string `json:"criterion"` // language | format | budget | availability | slots
	Match     bool   `json:"match"`
	Value     string `json:"value"`
}

// ProfessionalOption is one fictional professional with its rationale.
type ProfessionalOption struct {
	ProfessionalID string          `json:"professionalId"`
	Reasons        []Reason        `json:"reasons"`
	Slots          []fixtures.Slot `json:"slots"`
	Exact          bool            `json:"exact"`
}

// NextSteps is the deterministic Next Step result.
type NextSteps struct {
	Preferences *Preferences         `json:"preferences"`
	Exact       []ProfessionalOption `json:"exact"`
	// Near lists options failing exactly one criterion, each labelled.
	Near []ProfessionalOption `json:"near"`
	// Steps are non-handoff practical steps relevant to the stated barrier.
	Steps []string `json:"steps"`
	// HandoffFirst is false when the user said they are not ready: the
	// practical steps come first and professional options are secondary.
	HandoffFirst bool   `json:"handoffFirst"`
	Barrier      string `json:"barrier"`
	CatalogNote  string `json:"catalogNote"`
}

var costRank = map[string]int{"free": 0, "low": 1, "standard": 2}

func budgetAllows(budget, tier string) bool {
	switch budget {
	case "free_only":
		return tier == "free"
	case "low_or_free":
		return costRank[tier] <= 1
	}
	return true
}

// ComputeNextSteps filters the fictional catalogue by explicit preferences
// only. Order is catalogue order; nothing is ranked by clinical suitability
// and no professional category is chosen for the user (SPEC V8).
func (s *Service) ComputeNextSteps(sessionID string) (*NextSteps, error) {
	var out *NextSteps
	err := s.view(sessionID, func(sess *Session) error {
		if sess.Safety.Interrupted {
			return errCrisis
		}
		if sess.Preferences == nil {
			return errf(CodeInvalid, 409, "preferences")
		}
		p := *sess.Preferences
		out = &NextSteps{Preferences: &p, Exact: []ProfessionalOption{}, Near: []ProfessionalOption{}, Steps: []string{}, Barrier: p.Barrier, HandoffFirst: p.Barrier != "not_ready", CatalogNote: "fictional_catalog"}
		requested := map[string]bool{}
		for _, h := range sess.Handoffs {
			if h.Share == ShareActive && (h.Status == HandoffRequested || h.Status == HandoffAcknowledged) {
				requested[h.SlotID] = true
			}
		}
		for _, prof := range s.catalog.Professionals {
			reasons := []Reason{
				{Criterion: "language", Value: p.Language, Match: p.Language == "either" || oneOf(p.Language, prof.Languages...)},
				{Criterion: "format", Value: p.Format, Match: p.Format == "either" || oneOf(p.Format, prof.Formats...)},
				{Criterion: "budget", Value: p.Budget, Match: budgetAllows(p.Budget, prof.CostTier)},
				{Criterion: "availability", Value: p.Availability, Match: p.Availability == "any" || oneOf(p.Availability, prof.Availability...)},
			}
			var slots []fixtures.Slot
			for _, sl := range prof.Slots {
				if sl.Available && !requested[sl.ID] && (p.Format == "either" || sl.Format == p.Format) {
					slots = append(slots, sl)
				}
			}
			reasons = append(reasons, Reason{Criterion: "slots", Value: "", Match: len(slots) > 0})
			misses := 0
			for _, r := range reasons {
				if !r.Match {
					misses++
				}
			}
			if slots == nil {
				slots = []fixtures.Slot{}
			}
			opt := ProfessionalOption{ProfessionalID: prof.ID, Reasons: reasons, Slots: slots, Exact: misses == 0}
			switch misses {
			case 0:
				out.Exact = append(out.Exact, opt)
			case 1:
				out.Near = append(out.Near, opt)
			}
		}
		for _, st := range s.catalog.NextSteps {
			if oneOf(p.Barrier, st.Barriers...) {
				out.Steps = append(out.Steps, st.ID)
			}
		}
		return nil
	})
	return out, err
}
