package safety

import (
	"strings"
	"testing"
)

// TestBriefFarewellReviewFindings pins the two P1 cases from the independent
// PR #8 review: a positive "kembali lagi" is not a no-return phrase, and a
// "tidak akan lama" must not hide a later no-return phrase.
func TestBriefFarewellReviewFindings(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want Level
	}{
		{"A1 positive return", "aku pergi sebentar, nanti kembali lagi", LevelNone},
		{"A2 positive return with subject", "aku pergi sebentar, nanti aku kembali lagi", LevelNone},
		{"B1 brief-stay then no-return", "aku pergi sebentar, tidak akan lama, tapi tidak akan kembali", LevelCrisis},
		{"B2 soon then no-return", "aku pergi sebentar. sebentar lagi tidak akan kembali", LevelCrisis},
		{"B3 brief-stay control", "aku pergi sebentar, tidak akan lama", LevelNone},
		{"B4 no-return control", "aku pergi sebentar, tidak akan kembali", LevelCrisis},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Assess(tc.text).Level; got != tc.want {
				t.Fatalf("Assess(%q).Level = %s, want %s", tc.text, got, tc.want)
			}
		})
	}
}

func TestBriefFarewellLookaheadBoundary(t *testing.T) {
	const lead = "aku pergi sebentar"
	const tail = "tidak akan kembali"
	for _, sep := range []string{" ", ". ", "\n"} {
		for _, gap := range []int{119, 120, 121, 240} {
			// The gap is measured from the end of "sebentar" to the start of
			// the tail. Use multi-letter filler: normalization joins runs of
			// single letters, which made the old test's measured gap much
			// smaller than intended (review finding P2-1).
			pad := gap - len(sep) - 1
			if pad < 0 {
				t.Fatalf("gap %d is too small for separator %q", gap, sep)
			}
			fill := strings.Repeat("zz ", pad/3)
			switch pad % 3 {
			case 1:
				fill += "z"
			case 2:
				fill += "zz"
			}
			text := lead + sep + fill + " " + tail
			a := Assess(text)
			if a.Level != LevelCrisis {
				end := strings.Index(text, "sebentar") + len("sebentar")
				rest := restAfter(text, end)
				t.Logf("sep=%q gap=%d rest=%q noReturn=%v brief=%v", sep, gap, rest, noReturnRe.MatchString(rest), briefFarewell(text, 0, end))
				t.Fatalf("separator %q gap %d (%d bytes before tail): Assess(...).Level = %s rules %v, want crisis", sep, gap, gap, a.Level, a.RuleIDs)
			}
		}
	}
}

// TestBriefFarewellNoReturnVariants covers the P1-2 finding: after removing
// the broad "kembali lagi" alternative, the explicit no-return branches must
// still cover the common auxiliaries and informal negators.
func TestBriefFarewellNoReturnVariants(t *testing.T) {
	for _, text := range []string{
		"aku pergi sebentar, nanti tidak akan pernah kembali lagi",
		"aku pergi sebentar, nanti nggak akan pernah kembali lagi",
		"aku pergi sebentar, nanti gak bisa kembali lagi",
		"aku pergi sebentar, nanti nggak mungkin kembali lagi",
		"aku pergi sebentar, nanti ndak kembali lagi",
		"aku pergi sebentar, nanti kagak kembali lagi",
		"aku pergi sebentar, nanti tak kan kembali lagi",
		"aku mau tidur sebentar, nanti nggak bangun lagi",
		"aku mau tidur sebentar, nanti tidak bangun lagi",
		"aku mau tidur sebentar, nanti enggak bangun lagi",
		"aku mau tidur sebentar, nanti tidak akan bangun lagi",
		"aku mau tidur sebentar, nanti gak bangun lagi",
		"aku mau tidur sebentar, nanti nggak usah bangun lagi",
		"aku mau tidur sebentar, nanti tidak usah bangun lagi",
		"aku mau tidur sebentar, nanti nggak perlu bangun lagi",
		"aku mau tidur sebentar, nanti tidak perlu bangun lagi",
		"aku mau tidur sebentar, nanti nggak bangun-bangun lagi",
		"aku mau tidur sebentar, nanti tidak bangun-bangun lagi",
	} {
		if got := Assess(text).Level; got != LevelCrisis {
			t.Errorf("Assess(%q).Level = %s, want crisis", text, got)
		}
	}
	for _, text := range []string{
		"aku pergi sebentar, nanti kembali lagi",
		"aku mau tidur sebentar, nanti bangun lagi",
		"aku pergi sebentar, tidak akan lama. kita teman selamanya",
		"aku pergi sebentar, tidak akan lama, cinta kita abadi",
	} {
		if got := Assess(text).Level; got != LevelNone {
			t.Errorf("Assess(%q).Level = %s, want none", text, got)
		}
	}
}

// TestSkipEquivalenceDenialWindow covers P1-1: the denial key must include
// the same 512-byte tail as restAfter, or a derived variant can skip the
// sentence that resolves the denial.
func TestSkipEquivalenceDenialWindow(t *testing.T) {
	text := "I'm not suicidal, just need a break. " + strings.Repeat("blah ", 21) + "x with the bottlle"
	got, full := Assess(text), assess(text, false)
	if got.Level != LevelCrisis {
		t.Fatalf("Assess(...).Level = %s rules %v, want crisis", got.Level, got.RuleIDs)
	}
	if got.Level != full.Level {
		t.Fatalf("Assess(...).Level = %s, assess(text, false).Level = %s", got.Level, full.Level)
	}
}
