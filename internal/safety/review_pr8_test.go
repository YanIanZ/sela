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
			// the tail. The filler uses one-letter words so the lookahead
			// crosses a word boundary.
			pad := gap - len(sep) - 1
			if pad < 0 {
				t.Fatalf("gap %d is too small for separator %q", gap, sep)
			}
			fill := strings.Repeat("a ", pad/2)
			if pad%2 == 1 {
				fill += "a"
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
