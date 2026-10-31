package account

import (
	"testing"
	"time"
)

func TestComputeConditionMap(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	ago := func(d int) time.Time { return now.Add(-time.Duration(d) * 24 * time.Hour) }
	check := func(d int, mood, energy string) PersonalRecord {
		return PersonalRecord{Kind: "checkin", Mood: mood, Energy: energy, Safety: "none", UpdatedAt: ago(d)}
	}
	cases := []struct {
		name    string
		records []PersonalRecord
		stage   string
		limited bool
	}{
		{"empty", nil, StageEveryday, true},
		{"calm", []PersonalRecord{check(1, "good", "medium"), check(3, "okay", "high"), {Kind: "note", Safety: "none", UpdatedAt: ago(2)}}, StageEveryday, false},
		{"elevated note", []PersonalRecord{{Kind: "note", Safety: "elevated", UpdatedAt: ago(4)}}, StageAttention, false},
		{"heavy share", []PersonalRecord{check(1, "heavy", ""), check(2, "okay", ""), check(5, "good", "")}, StageAttention, false},
		{"heavy streak", []PersonalRecord{check(1, "very_heavy", ""), check(2, "heavy", ""), check(3, "heavy", ""), check(6, "good", "")}, StageSupport, false},
		{"old crisis", []PersonalRecord{{Kind: "conversation", Safety: "crisis", UpdatedAt: ago(20)}}, StageSupport, false},
		{"recent crisis", []PersonalRecord{{Kind: "note", Safety: "crisis", UpdatedAt: ago(2)}, check(1, "good", "")}, StageUrgent, false},
		{"out of window", []PersonalRecord{{Kind: "note", Safety: "crisis", UpdatedAt: ago(45)}}, StageEveryday, true},
		{"memory is context only", []PersonalRecord{{Kind: "memory", Safety: "crisis", UpdatedAt: ago(1)}}, StageEveryday, true},
	}
	for _, c := range cases {
		m := ComputeConditionMap(c.records, now)
		if m.Stage != c.stage || m.DataLimited != c.limited {
			t.Errorf("%s: stage %s limited %v, want %s %v (signals %+v)", c.name, m.Stage, m.DataLimited, c.stage, c.limited, m.Signals)
		}
		if m.Stage != StageEveryday && len(m.Signals) == 0 {
			t.Errorf("%s: a stage above everyday must show its signals", c.name)
		}
	}
}
