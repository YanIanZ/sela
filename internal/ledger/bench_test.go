package ledger

import (
	"strings"
	"testing"
)

func BenchmarkValidateDraft(b *testing.B) {
	story := strings.Repeat("Aku tidak bisa tidur sejak awal semester, kata temanku aku cuma butuh liburan. ", 20)
	segs := SplitSegments(story, "story", 1)
	var claims []Candidate
	for _, s := range segs[:MaxClaims] {
		claims = append(claims, cand(s.ID, "tidak bisa tidur sejak awal semester", KindExperience, "Tidak bisa tidur sejak awal semester"))
	}
	d := Draft{Claims: claims}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateDraft(d, segs)
	}
}

func BenchmarkValidateOpening(b *testing.B) {
	var items []ApprovedItem
	for i := 0; i < 12; i++ {
		items = append(items, ApprovedItem{ID: "i", Kind: KindFeeling, Text: strings.Repeat("Aku susah tidur dan sering cemas sebelum ujian kuliah ", 5)})
	}
	d := OpeningDraft{Lines: []string{"Saya susah tidur dan sering cemas sebelum ujian.", "Saya ingin berbicara dengan psikolog.", "Kuliah saya berantakan."}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateOpening(d, items, "id")
		TemplateOpening(items, "id")
	}
}
