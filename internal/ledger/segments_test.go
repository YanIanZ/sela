package ledger

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func segTexts(segs []Segment) []string {
	out := make([]string, len(segs))
	for i, s := range segs {
		out[i] = s.Text
	}
	return out
}

func TestSplitSegments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"indonesian sentences", "Aku susah tidur. Kuliahku berantakan! Harus gimana?", []string{"Aku susah tidur.", "Kuliahku berantakan!", "Harus gimana?"}},
		{"lowercase sentences still split", "aku capek. aku sedih.", []string{"aku capek.", "aku sedih."}},
		{"english with ellipsis", "I can't sleep… I keep thinking. What now?!", []string{"I can't sleep…", "I keep thinking.", "What now?!"}},
		{"three dots", "Entahlah... aku bingung.", []string{"Entahlah...", "aku bingung."}},
		{"abbreviations", "Aku ketemu dr. Sari dan psikolog dll. di kampus. Lalu pulang.", []string{"Aku ketemu dr. Sari dan psikolog dll. di kampus.", "Lalu pulang."}},
		{"english abbreviations", "I tried things, e.g. journaling. It helped a bit.", []string{"I tried things, e.g. journaling.", "It helped a bit."}},
		{"decimals and closing quotes", "Nilaiku 3.5 turun. Dia bilang \"kamu lebay.\" Aku diam.", []string{"Nilaiku 3.5 turun.", "Dia bilang \"kamu lebay.\"", "Aku diam."}},
		{"newlines and blanks", "  baris satu\r\n\n  baris dua  \n\n", []string{"baris satu", "baris dua"}},
		{"punctuation-only dropped", "Aku capek. ... !!! Sudah.", []string{"Aku capek.", "Sudah."}},
		{"code-switched", "Aku overthinking terus. It's exhausting, jujur.", []string{"Aku overthinking terus.", "It's exhausting, jujur."}},
		{"empty", "   \n\t", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitSegments(tc.in, "story", 1)
			if !slices.Equal(segTexts(got), tc.want) {
				t.Errorf("got %q, want %q", segTexts(got), tc.want)
			}
		})
	}
}

func TestSplitSegmentsIDsAndOrigin(t *testing.T) {
	got := SplitSegments("Satu. Dua.", "clarification", 7)
	want := []Segment{{ID: "s7", Text: "Satu.", Origin: "clarification"}, {ID: "s8", Text: "Dua.", Origin: "clarification"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v", got)
	}
}

func TestSplitSegmentsLimits(t *testing.T) {
	many := strings.Repeat("Kalimat. ", maxSegments+10)
	if got := SplitSegments(many, "story", 1); len(got) != maxSegments || got[len(got)-1].ID != "s40" {
		t.Errorf("got %d segments", len(got))
	}

	long := strings.Repeat("aku capek banget dan susah tidur, ", 30) // ~1000 runes, one sentence
	got := SplitSegments(long, "story", 1)
	if len(got) < 3 {
		t.Fatalf("long sentence not split: %d", len(got))
	}
	for _, s := range got {
		if n := utf8.RuneCountInString(s.Text); n > maxSegmentRunes || n == 0 {
			t.Errorf("segment of %d runes", n)
		}
		if !strings.HasSuffix(s.Text, ",") && s != got[len(got)-1] {
			t.Errorf("segment not cut at a comma: %q", s.Text)
		}
	}

	noComma := strings.Repeat("kata ", 200)
	for _, s := range SplitSegments(noComma, "story", 1) {
		if n := utf8.RuneCountInString(s.Text); n > maxSegmentRunes {
			t.Errorf("segment of %d runes", n)
		}
		if strings.HasPrefix(s.Text, " ") || strings.Contains(s.Text, "kat ") {
			t.Errorf("segment cut inside a word: %q", s.Text)
		}
	}

	noSpace := strings.Repeat("é", 1000)
	for _, s := range SplitSegments(noSpace, "story", 1) {
		if n := utf8.RuneCountInString(s.Text); n > maxSegmentRunes || !utf8.ValidString(s.Text) {
			t.Errorf("bad hard cut: %d runes", n)
		}
	}
}

func TestSplitSegmentsInvalidUTF8(t *testing.T) {
	for _, s := range SplitSegments("Aku \xff capek.", "story", 1) {
		if !utf8.ValidString(s.Text) {
			t.Errorf("invalid UTF-8 in %q", s.Text)
		}
	}
}
