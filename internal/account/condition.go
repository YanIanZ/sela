package account

import (
	"slices"
	"time"
)

// Optional consents added on 2026-11-01.
const (
	// PolicyConditionMap is the consent for the condition map (S-020).
	PolicyConditionMap = "condition_map"
	// PolicyCommunity is the consent for anonymous peer sharing (D-072).
	PolicyCommunity = "community"
)

// Condition stages, gentlest first. They describe how the person's own
// saved material reads to Sela's deterministic checks; they are not a
// diagnosis, a triage category or a risk score, and no number is shown.
const (
	StageEveryday  = "everyday"  // Sehari-hari
	StageAttention = "attention" // Perlu perhatian
	StageSupport   = "support"   // Butuh dukungan
	StageUrgent    = "urgent"    // Butuh bantuan segera
)

// ConditionSignal is one reason behind a stage, content-free.
type ConditionSignal struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
	Days  int    `json:"days"`
}

// ConditionMap is the computed view; nothing of it is stored.
type ConditionMap struct {
	Stage       string            `json:"stage"`
	Signals     []ConditionSignal `json:"signals"`
	DataPoints  int               `json:"dataPoints"`
	WindowDays  int               `json:"windowDays"`
	DataLimited bool              `json:"dataLimited"`
	Sources     map[string]int    `json:"sources"`
	ComputedAt  time.Time         `json:"computedAt"`
	Ruleset     string            `json:"ruleset"`
}

// ConditionRuleset identifies the stage rules; change it with the rules.
const ConditionRuleset = "sela-condition-2026.11.01-1"

const conditionWindow = 30

var heavyMoods = []string{"very_heavy", "heavy"}

// ComputeConditionMap derives the stage from saved records only: notes,
// check-ins and saved conversations (their stored safety level, the
// check-in mood and energy). Unsaved conversations never reach it, and
// memory items, summaries and follow-ups are counted as context only. The
// rules are deterministic and explainable:
//
//   - urgent: a saved entry read as crisis in the last 7 days;
//   - support: a crisis entry 8–30 days ago, three or more elevated entries
//     in 14 days, heavy moods in 60% or more of at least four check-ins in
//     14 days, or the three latest check-ins all heavy;
//   - attention: an elevated entry in 14 days, heavy moods in a third or more
//     of at least three check-ins in 14 days, two heavy check-ins in 7
//     days, or low energy in half of at least four check-ins in 14 days;
//   - everyday: otherwise.
//
// With fewer than three entries in 30 days and no safety signal the stage
// is everyday and DataLimited is set.
func ComputeConditionMap(records []PersonalRecord, now time.Time) ConditionMap {
	m := ConditionMap{WindowDays: conditionWindow, ComputedAt: now.UTC(), Ruleset: ConditionRuleset, Sources: map[string]int{}}
	days := func(r PersonalRecord) float64 { return now.Sub(r.UpdatedAt).Hours() / 24 }
	var crisis7, crisis30, elevated14, checkins14, heavy14, heavy7, lowEnergy14 int
	var checkins []PersonalRecord
	for _, r := range records {
		age := days(r)
		if age > conditionWindow || age < -1 {
			continue
		}
		switch r.Kind {
		case "note", "checkin", "conversation":
		default:
			m.Sources["context"]++
			continue
		}
		m.DataPoints++
		m.Sources[r.Kind]++
		switch r.Safety {
		case "crisis":
			if age <= 7 {
				crisis7++
			} else {
				crisis30++
			}
		case "elevated":
			if age <= 14 {
				elevated14++
			}
		}
		if r.Kind == "checkin" && age <= 14 {
			checkins14++
			checkins = append(checkins, r)
			if slices.Contains(heavyMoods, r.Mood) {
				heavy14++
				if age <= 7 {
					heavy7++
				}
			}
			if r.Energy == "low" {
				lowEnergy14++
			}
		}
	}
	slices.SortFunc(checkins, func(a, b PersonalRecord) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	streak := 0
	for _, c := range checkins {
		if !slices.Contains(heavyMoods, c.Mood) {
			break
		}
		streak++
	}
	add := func(key string, n, d int) {
		m.Signals = append(m.Signals, ConditionSignal{Key: key, Count: n, Days: d})
	}
	share := func(part, whole int) float64 {
		if whole == 0 {
			return 0
		}
		return float64(part) / float64(whole)
	}
	switch {
	case crisis7 > 0:
		m.Stage = StageUrgent
		add("crisis_entries", crisis7, 7)
	case crisis30 > 0 || elevated14 >= 3 || (checkins14 >= 4 && share(heavy14, checkins14) >= 0.6) || streak >= 3:
		m.Stage = StageSupport
		if crisis30 > 0 {
			add("crisis_entries", crisis30, 30)
		}
		if elevated14 >= 3 {
			add("heavy_words", elevated14, 14)
		}
		if checkins14 >= 4 && share(heavy14, checkins14) >= 0.6 {
			add("heavy_checkins", heavy14, 14)
		}
		if streak >= 3 {
			add("heavy_streak", streak, 14)
		}
	case elevated14 >= 1 || (checkins14 >= 3 && share(heavy14, checkins14) >= 1.0/3) || heavy7 >= 2 || (checkins14 >= 4 && share(lowEnergy14, checkins14) >= 0.5):
		m.Stage = StageAttention
		if elevated14 >= 1 {
			add("heavy_words", elevated14, 14)
		}
		if (checkins14 >= 3 && share(heavy14, checkins14) >= 1.0/3) || heavy7 >= 2 {
			add("heavy_checkins", heavy14, 14)
		}
		if checkins14 >= 4 && share(lowEnergy14, checkins14) >= 0.5 {
			add("low_energy", lowEnergy14, 14)
		}
	default:
		m.Stage = StageEveryday
		m.DataLimited = m.DataPoints < 3
	}
	if m.Signals == nil {
		m.Signals = []ConditionSignal{}
	}
	return m
}
