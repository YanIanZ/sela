package ledger

import (
	"slices"
	"strings"
	"testing"
)

var idItems = []ApprovedItem{
	{ID: "i1", Kind: KindFeeling, Text: "Aku susah tidur dan sering cemas sebelum ujian"},
	{ID: "i2", Kind: KindImpact, Text: "Kuliahku jadi berantakan"},
	{ID: "i3", Kind: KindGoal, Text: "Aku ingin tahu langkah selanjutnya"},
	{ID: "i4", Kind: KindDuration, Text: "Sudah 3 minggu"},
}

var enItems = []ApprovedItem{
	{ID: "e1", Kind: KindExperience, Text: "I can't sleep before exams"},
	{ID: "e2", Kind: KindImpact, Text: "My grades are slipping"},
	{ID: "e3", Kind: KindGoal, Text: "I want to know my options for counseling"},
}

var mixItems = []ApprovedItem{
	{ID: "m1", Kind: KindFeeling, Text: "Aku overthinking terus soal skripsi"},
	{ID: "m2", Kind: KindImpact, Text: "Jadi nggak bisa fokus kerja"},
}

func TestValidateOpening(t *testing.T) {
	tests := []struct {
		name    string
		items   []ApprovedItem
		line    string
		reason  OpeningRejectReason
		basedOn []string
	}{
		{"ID grounded", idItems, "Saya susah tidur dan sering cemas sebelum ujian.", "", []string{"i1"}},
		{"ID affix-insensitive", idItems, "Kuliah saya jadi berantakan.", "", []string{"i2"}},
		{"ID goal words also intent", idItems, "Saya ingin tahu langkah selanjutnya.", "", []string{"i3"}},
		{"ID number the user wrote", idItems, "Sudah 3 minggu saya susah tidur.", "", []string{"i1", "i4"}},
		{"ID intent only", idItems, "Saya ingin berbicara dengan psikolog.", "", []string{IntentBasis}},
		{"ID ungrounded content", idItems, "Saya juga sering bertengkar dengan pacar.", OpeningUngrounded, nil},
		{"ID added severity", idItems, "Saya sangat susah tidur.", OpeningUngrounded, nil},
		{"ID pressure", idItems, "Anda harus segera membantu saya.", OpeningManipulative, nil},
		{"ID guarantee", idItems, "Saya pasti sembuh kalau dibantu.", OpeningManipulative, nil},
		{"ID invented number", idItems, "Saya susah tidur selama 5 minggu.", OpeningNumber, nil},
		{"ID invented number word", idItems, "Saya susah tidur dua kali.", OpeningNumber, nil},
		{"ID identifier email", idItems, "Hubungi saya di budi@example.com.", OpeningIdentifier, nil},
		{"ID identifier phone", idItems, "Nomor saya 0812 3456 7890.", OpeningIdentifier, nil},
		{"ID diagnosis absent from items", idItems, "Saya rasa saya depresi.", OpeningDiagnosis, nil},
		{"ID medical instruction", idItems, "Saya ingin berhenti minum obat.", OpeningMedical, nil},
		{"ID flips negation", []ApprovedItem{{ID: "n1", Kind: KindExperience, Text: "Aku tidak bisa tidur"}}, "Saya bisa tidur.", OpeningUngrounded, nil},
		{"ID keeps negation", []ApprovedItem{{ID: "n1", Kind: KindExperience, Text: "Aku tidak bisa tidur"}}, "Saya tidak bisa tidur.", "", []string{"n1"}},
		{"ID adds negation", idItems, "Saya tidak cemas sebelum ujian.", OpeningUngrounded, nil},
		{"ID no content", idItems, "Halo, selamat pagi.", OpeningUngrounded, nil},
		{"EN grounded", enItems, "I can't sleep before exams.", "", []string{"e1"}},
		{"EN impact", enItems, "My grades are slipping.", "", []string{"e2"}},
		{"EN intent only", enItems, "I would like to talk to a psychologist.", "", []string{IntentBasis}},
		{"EN ungrounded", enItems, "My boss yells at me every day.", OpeningUngrounded, nil},
		{"EN pressure", enItems, "You must help me right now or I will leave.", OpeningManipulative, nil},
		{"EN guilt", enItems, "If you cared you would help me.", OpeningManipulative, nil},
		{"EN invented percentage", enItems, "My grades are slipping by 40%.", OpeningNumber, nil},
		{"EN URL", enItems, "See https://example.org for my notes.", OpeningIdentifier, nil},
		{"EN handle", enItems, "Message me at @sleepless.", OpeningIdentifier, nil},
		{"EN diagnosis", enItems, "I think I have anxiety disorder.", OpeningDiagnosis, nil},
		{"EN treatment directive", enItems, "You should take sleeping pills.", OpeningMedical, nil},
		{"code-switched grounded", mixItems, "Saya overthinking terus soal skripsi, and I want to talk about it.", "", []string{"m1"}},
		{"code-switched impact", mixItems, "Jadi I nggak bisa fokus kerja.", "", []string{"m2"}},
		{"code-switched spelling is not stemmed", mixItems, "I can't focus on kerja.", OpeningUngrounded, nil},
		{"code-switched ungrounded", mixItems, "Skripsi saya ditolak dosen.", OpeningUngrounded, nil},
		{"too long", idItems, strings.Repeat("susah tidur ", 30), OpeningTooLong, nil},
		{"empty", idItems, "   ", OpeningEmpty, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ValidateOpening(OpeningDraft{Lines: []string{tc.line}}, tc.items, "id")
			if tc.reason != "" {
				if len(r.Rejected) != 1 || r.Rejected[0].Reason != tc.reason {
					t.Fatalf("got %+v, want rejection %q", r, tc.reason)
				}
				return
			}
			if len(r.Lines) != 1 {
				t.Fatalf("got %+v, want accepted", r)
			}
			if !slices.Equal(r.Lines[0].BasedOn, tc.basedOn) {
				t.Errorf("basedOn = %v, want %v", r.Lines[0].BasedOn, tc.basedOn)
			}
		})
	}
}

func TestValidateOpeningLimitAndDuplicate(t *testing.T) {
	d := OpeningDraft{Lines: []string{
		"Saya susah tidur.",
		"saya  SUSAH tidur.",
		"Kuliah saya jadi berantakan.",
		"Saya ingin tahu langkah selanjutnya.",
		"Saya ingin bercerita.",
	}}
	r := ValidateOpening(d, idItems, "id")
	if len(r.Lines) != MaxOpeningLines {
		t.Fatalf("lines = %+v", r.Lines)
	}
	want := []OpeningRejection{{1, OpeningDuplicate}, {4, OpeningLimit}}
	if !slices.Equal(r.Rejected, want) {
		t.Errorf("rejected = %v, want %v", r.Rejected, want)
	}
}

func TestValidateOpeningAllowsDiagnosisFromItems(t *testing.T) {
	items := []ApprovedItem{{ID: "c1", Kind: KindContext, Text: "Aku didiagnosis depresi oleh psikiater"}}
	r := ValidateOpening(OpeningDraft{Lines: []string{"Saya didiagnosis depresi oleh psikiater."}}, items, "id")
	if len(r.Lines) != 1 {
		t.Fatalf("got %+v", r)
	}
}

func TestTemplateOpening(t *testing.T) {
	long := ApprovedItem{ID: "l1", Kind: KindWorry, Text: strings.Repeat("aku takut mengecewakan keluarga ", 10)}
	tests := []struct {
		name    string
		items   []ApprovedItem
		lang    string
		lines   int
		basedOn [][]string
		prefix  string
	}{
		{"ID full", idItems, "id", 3, [][]string{{"i1"}, {"i2"}, {"i3"}}, "Saya ingin membicarakan hal yang sedang saya alami: Aku susah tidur"},
		{"EN full", enItems, "en", 3, [][]string{{"e1"}, {"e2"}, {"e3"}}, "I would like to talk about what I have been going through: I can't sleep"},
		{"code-switched", mixItems, "id", 2, [][]string{{"m1"}, {"m2"}}, "Saya ingin membicarakan"},
		{"no items ID", nil, "id", 1, [][]string{{IntentBasis}}, "Saya belum yakin bagaimana memulainya"},
		{"no items EN", nil, "en-US", 1, [][]string{{IntentBasis}}, "I am not sure how to start"},
		{"only preferences", []ApprovedItem{{ID: "p1", Kind: KindPreference, Text: "Online saja"}}, "id", 1, [][]string{{IntentBasis}}, "Saya belum yakin"},
		{"unsafe item skipped", []ApprovedItem{
			{ID: "u1", Kind: KindFeeling, Text: "Hubungi aku di budi@example.com"},
			{ID: "u2", Kind: KindFeeling, Text: "Aku sering cemas"},
		}, "id", 1, [][]string{{"u2"}}, "Saya ingin membicarakan hal yang sedang saya alami: Aku sering cemas."},
		{"long value truncated", []ApprovedItem{long}, "id", 1, [][]string{{"l1"}}, "Saya ingin membicarakan"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lines := TemplateOpening(tc.items, tc.lang)
			if len(lines) != tc.lines {
				t.Fatalf("lines = %+v", lines)
			}
			if !strings.HasPrefix(lines[0].Text, tc.prefix) {
				t.Errorf("first line %q lacks prefix %q", lines[0].Text, tc.prefix)
			}
			var texts []string
			for i, l := range lines {
				texts = append(texts, l.Text)
				if !slices.Equal(l.BasedOn, tc.basedOn[i]) {
					t.Errorf("line %d basedOn = %v, want %v", i, l.BasedOn, tc.basedOn[i])
				}
				if n := runeLen(l.Text); n > MaxOpeningRunes {
					t.Errorf("line %d has %d runes", i, n)
				}
			}
			r := ValidateOpening(OpeningDraft{Lines: texts}, tc.items, tc.lang)
			if len(r.Rejected) != 0 || len(r.Lines) != len(lines) {
				t.Errorf("template does not pass ValidateOpening: %+v", r.Rejected)
			}
		})
	}
	if l := TemplateOpening([]ApprovedItem{long}, "id")[0].Text; !strings.HasSuffix(l, "...") || runeLen(l) > 52+templateValueRunes+3 {
		t.Errorf("truncated line = %q", l)
	}
}

func TestPracticeCoverage(t *testing.T) {
	items := []ApprovedItem{
		{ID: "a", Kind: KindFeeling, Text: "Aku susah tidur"},
		{ID: "b", Kind: KindImpact, Text: "Kuliahku jadi berantakan"},
		{ID: "c", Kind: KindGoal, Text: "Aku ingin tahu pilihan konseling kampus yang gratis dan rahasia"},
	}
	tests := []struct {
		attempt string
		want    []string
	}{
		{"Halo, saya susah tidur akhir-akhir ini.", []string{"a"}},
		{"Kuliah saya berantakan dan saya tidak bisa tidur.", []string{"a", "b"}},
		{"Saya ingin tahu soal konseling.", nil},                               // 2 of 6 content words for c
		{"Saya mau tahu pilihan konseling kampus yang gratis.", []string{"c"}}, // 5 of 6
		{"I can't sleep and my studies are a mess.", nil},                      // different language, no shared word
		{"", nil},
	}
	for _, tc := range tests {
		got := PracticeCoverage(tc.attempt, items)
		if tc.want == nil {
			tc.want = []string{}
		}
		if !slices.Equal(got.Mentioned, tc.want) {
			t.Errorf("PracticeCoverage(%q).Mentioned = %v, want %v", tc.attempt, got.Mentioned, tc.want)
		}
		if len(got.Mentioned)+len(got.NotMentioned) != len(items) {
			t.Errorf("coverage does not partition items: %+v", got)
		}
	}
	long := strings.Repeat("x ", MaxPracticeRunes) + "susah tidur"
	if got := PracticeCoverage(long, items); slices.Contains(got.Mentioned, "a") {
		t.Errorf("text beyond MaxPracticeRunes was read")
	}
}
