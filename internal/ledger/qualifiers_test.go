package ledger

import (
	"strings"
	"testing"
)

// analyzeQuote runs Analyze on the first occurrence of quote in text.
func analyzeQuote(t *testing.T, text, quote string) Qualifiers {
	t.Helper()
	i := strings.Index(text, quote)
	if i < 0 {
		t.Fatalf("%q not in %q", quote, text)
	}
	return Analyze(seg("s1", text), i, i+len(quote))
}

func TestAnalyzeNegation(t *testing.T) {
	tests := []struct {
		text, quote string
		want        bool
	}{
		{"Aku tidak susah tidur.", "susah tidur", true},
		{"Aku tidak susah tidur.", "tidak susah tidur", true},
		{"Aku gak bisa fokus.", "bisa fokus", true},
		{"Aku belum pernah cerita ke siapa pun.", "cerita ke siapa pun", true},
		{"Aku tidak tahu kenapa aku susah tidur.", "susah tidur", false}, // negator is four tokens back
		{"Aku tidak sedih, tapi capek.", "capek", false},
		{"Aku tidak sedih tapi capek.", "capek", false},
		{"I don't sleep badly.", "sleep badly", true},
		{"I never feel rested.", "feel rested", true},
		{"Nobody listens to me at home.", "listens to me at home", true},
		{"I feel tired without reason.", "I feel tired", false},
		{"Jangan-jangan aku yang salah.", "aku yang salah", false},
		{"Rasanya ngga ada yang ngerti.", "ada yang ngerti", true},
		// Quote marks, brackets, slashes and dashes do not shield a word.
		{`Aku tidak "baik-baik saja".`, "baik-baik saja", true},
		{`I'm not "fine".`, "fine", true},
		{"I'm not “okay” at all.", "okay", true},
		{"Aku nggak (terlalu) cemas.", "cemas", true},
		{"Aku nggak sedih/cemas.", "cemas", true},
		{"Aku nggak - jujur - baik.", "baik", true},
		{"Aku tidak sedih, cuma capek.", "cuma capek", false}, // a comma still ends the clause
		// Fused informal negators and apostrophe-less contractions.
		{"Aku gabisa tidur.", "tidur", true},
		{"Aku gamau makan.", "makan", true},
		// A hedge is certainty, not polarity (D-023); validation still
		// rejects a quote that cuts the hedge off (negBefore).
		{"Aku gatau harus gimana.", "harus gimana", false},
		{"I'm not sure if it's the rotating schedule or something else", "I'm not sure if it's the rotating schedule or something else", false},
		{"Aku belum yakin ini masalah besar.", "Aku belum yakin ini masalah besar", false},
		{"Aku bukannya malas.", "malas", true},
		{"I dont sleep well.", "sleep well", true},
		{"I cant focus at all.", "focus at all", true},
		{"I didnt eat today.", "eat today", true},
		// No "ga-" prefix rule.
		{"Aku gagal ujian lagi.", "ujian lagi", false},
		{"Aku lagi galau.", "galau", false},
		{"Aku ganti jurusan.", "jurusan", false},
	}
	for _, tc := range tests {
		if got := analyzeQuote(t, tc.text, tc.quote).Negated; got != tc.want {
			t.Errorf("Negated(%q in %q) = %v, want %v", tc.quote, tc.text, got, tc.want)
		}
	}
}

func TestAnalyzeSubject(t *testing.T) {
	tests := []struct {
		text, quote string
		want        Subject
	}{
		{"Aku sering cemas sebelum ujian.", "sering cemas", SubjectSelf},
		{"Temanku sering cemas sebelum ujian.", "sering cemas", SubjectOther},
		{"Temanku sering cemas sebelum ujian.", "Temanku sering cemas", SubjectOther},
		{"Kata temanku aku cuma butuh liburan.", "aku cuma butuh liburan", SubjectOther},
		{"Kata temanku, aku cuma butuh liburan.", "aku cuma butuh liburan", SubjectOther},
		{"Temanku bilang aku lebay.", "aku lebay", SubjectOther},
		{"Teman saya bilang saya terlalu sensitif.", "saya terlalu sensitif", SubjectOther},
		{"Aku cuma butuh liburan, kata temanku.", "Aku cuma butuh liburan", SubjectOther},
		{"Kata temanku aku lebay, tapi aku merasa capek banget.", "aku merasa capek banget", SubjectSelf},
		{"Temanku bilang aku lebay, aku jadi sedih.", "aku jadi sedih", SubjectSelf},
		{"Ibuku sakit dan aku jadi sedih.", "jadi sedih", SubjectSelf},
		{"Aku khawatir karena ibuku sering sakit.", "sering sakit", SubjectOther},
		{"Ibu saya sering marah.", "sering marah", SubjectOther},
		{"Orang tuaku sering bertengkar.", "sering bertengkar", SubjectOther},
		{"Pikiranku kacau terus.", "kacau terus", SubjectSelf},
		{"Dia bilang aku harus lebih kuat.", "aku harus lebih kuat", SubjectOther},
		{"Katanya aku cuma kurang bersyukur.", "aku cuma kurang bersyukur", SubjectOther},
		{"Aku takut ketemu orang.", "takut ketemu orang", SubjectSelf},
		{"Aku pinjam buku temanku.", "pinjam buku", SubjectSelf},
		{"My friend said I'm just being dramatic.", "I'm just being dramatic", SubjectOther},
		{"My mom always says I overthink.", "I overthink", SubjectOther},
		{"She cries every night.", "cries every night", SubjectOther},
		{"I feel anxious around people.", "anxious around people", SubjectSelf},
		{"I told my friend that I feel lost.", "I feel lost", SubjectSelf},
		{"According to my doctor I need more rest.", "I need more rest", SubjectOther},
	}
	for _, tc := range tests {
		if got := analyzeQuote(t, tc.text, tc.quote).Subject; got != tc.want {
			t.Errorf("Subject(%q in %q) = %v, want %v", tc.quote, tc.text, got, tc.want)
		}
	}
}

func TestAnalyzeTime(t *testing.T) {
	tests := []struct {
		text, quote string
		want        Time
	}{
		{"Dulu aku sering cemas.", "aku sering cemas", TimePast},
		{"Dulu aku sering cemas, sekarang sudah lebih tenang.", "sudah lebih tenang", TimeCurrent},
		{"Dulu aku sering cemas, sekarang sudah lebih tenang.", "aku sering cemas", TimePast},
		{"Waktu kecil, aku sering dimarahi.", "aku sering dimarahi", TimePast},
		{"Aku pernah panik di kelas.", "panik di kelas", TimePast},
		{"Aku sudah lama tidak tidur nyenyak.", "tidur nyenyak", TimeCurrent},
		{"Akhir-akhir ini aku susah fokus.", "aku susah fokus", TimeCurrent},
		{"Aku susah fokus belakangan ini.", "Aku susah fokus", TimeCurrent},
		{"Besok aku presentasi.", "aku presentasi", TimeFuture},
		{"Aku akan sidang minggu depan.", "sidang", TimeFuture},
		{"Aku susah tidur.", "susah tidur", TimeUnspecified},
		{"I used to love drawing.", "love drawing", TimePast},
		{"Last year I stopped going out.", "I stopped going out", TimePast},
		{"I can't focus lately.", "I can't focus", TimeCurrent},
		{"I am going to talk to my lecturer tomorrow.", "talk to my lecturer", TimeFuture},
		{"I feel sad.", "I feel sad", TimeUnspecified},
		{"Aku sudah lama ngerasa capek, mungkin sejak dulu.", "ngerasa capek", TimeCurrent},
	}
	for _, tc := range tests {
		if got := analyzeQuote(t, tc.text, tc.quote).Time; got != tc.want {
			t.Errorf("Time(%q in %q) = %v, want %v", tc.quote, tc.text, got, tc.want)
		}
	}
}

func TestAnalyzeCertainty(t *testing.T) {
	tests := []struct {
		text, quote string
		want        Certainty
	}{
		{"Mungkin sejak awal semester.", "sejak awal semester", CertaintyUncertain},
		{"Aku susah tidur, mungkin sejak awal semester.", "sejak awal semester", CertaintyUncertain},
		{"Aku capek, mungkin karena tugas.", "Aku capek", CertaintyStated},
		{"Kayaknya aku burnout.", "aku burnout", CertaintyUncertain},
		{"Aku belum yakin ini masalah besar.", "ini masalah besar", CertaintyUncertain},
		{"Aku takut kalau nanti aku gagal ujian.", "aku gagal ujian", CertaintyHypothetical},
		{"Seandainya aku cerita, mereka marah.", "mereka marah", CertaintyHypothetical},
		{"Aku sedih.", "Aku sedih", CertaintyStated},
		{"Maybe it started in March.", "it started in March", CertaintyUncertain},
		{"I'm not sure it is that bad.", "it is that bad", CertaintyUncertain},
		{"I guess I'm just tired.", "I'm just tired", CertaintyUncertain},
		{"What if I fail again?", "I fail again", CertaintyHypothetical},
		{"I'm afraid that I will lose my job.", "I will lose my job", CertaintyHypothetical},
		{"I feel tired probably.", "I feel tired", CertaintyUncertain},
	}
	for _, tc := range tests {
		if got := analyzeQuote(t, tc.text, tc.quote).Certainty; got != tc.want {
			t.Errorf("Certainty(%q in %q) = %v, want %v", tc.quote, tc.text, got, tc.want)
		}
	}
}

func TestAnalyzeOffsetsAreClamped(t *testing.T) {
	s := seg("s1", "Aku sedih é.")
	for _, span := range [][2]int{{-5, 100}, {9, 3}, {11, 12}, {100, 200}} {
		_ = Analyze(s, span[0], span[1]) // must not panic
	}
	if got := Analyze(s, -1, 1000); got.Subject != SubjectSelf {
		t.Errorf("whole-text subject = %v", got.Subject)
	}
	if got := Analyze(seg("s1", ""), 0, 0); got != (Qualifiers{Subject: SubjectSelf, Time: TimeUnspecified, Certainty: CertaintyStated}) {
		t.Errorf("empty = %+v", got)
	}
}
