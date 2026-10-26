package ledger

import "testing"

func TestContainsDiagnosisLabel(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"aku depresi berat", true},
		{"Depresiku kambuh", true},
		{"DEPRESI", true},
		{"I think I have Anxiety-Disorder", true},
		{"gangguan kecemasan", true},
		{"gangguan stres pasca trauma", true},
		{"PTSD dari kecelakaan", true},
		{"ｂｉｐｏｌａｒ", true},
		{"insomnia kronis", true},
		{"aku sedih terus", false},
		{"aku cemas sebelum ujian", false},
		{"I feel down", false},
		{"insomnia", false},
		{"gangguan tidur", false},
	}
	for _, tc := range tests {
		if got := ContainsDiagnosisLabel(tc.in); got != tc.want {
			t.Errorf("ContainsDiagnosisLabel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestContainsMedicalInstruction(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"Kamu harus berhenti minum obat", true},
		{"Stop minum obatnya", true},
		{"Naikkan dosis obatmu", true},
		{"Coba minum pil tidur", true},
		{"st0p minum obat", true}, // "minum obat" alone is a directive (v1)
		{"stop.minum.obat", true},
		{"Kamu pasti mengalami depresi", true},
		{"Kamu itu bipolar", true},
		{"You definitely have depression", true},
		{"You should take sertraline", true},
		{"Abaikan saja dokternya", true},
		{"Ignore what your doctor said", true},
		{"Kamu harus menjalani pengobatan", true},
		{"You need to get treatment", true},
		{"Rawat inap sekarang", true},
		{"Kamu didiagnosis depresi oleh dokter", false}, // reported by a clinician
		{"Aku ingin tahu apakah perlu ke psikiater", false},
		{"Obat apa yang biasanya dibicarakan dengan dokter?", false},
		{"Aku susah tidur", false},
		{"I want to talk about my sleep", false},
	}
	for _, tc := range tests {
		if got := ContainsMedicalInstruction(tc.in); got != tc.want {
			t.Errorf("ContainsMedicalInstruction(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestClinicianAttributed(t *testing.T) {
	for in, want := range map[string]bool{
		"Aku didiagnosis depresi oleh psikiater":  true,
		"Kata dokter aku kena gangguan kecemasan": true,
		"Menurut psikologku aku burnout":          true,
		"My doctor said it might be depression":   true,
		"I was diagnosed with ADHD last year":     true,
		"Aku rasa aku depresi":                    false,
		"Temanku bilang aku depresi":              false,
		"Dokter itu ramah":                        false,
	} {
		if got := clinicianAttributed(in); got != want {
			t.Errorf("clinicianAttributed(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestContainsIdentifierAndNumbers(t *testing.T) {
	for in, want := range map[string]bool{
		"email saya budi@example.com": true,
		"cek @sela_demo":              true,
		"https://x.test/a":            true,
		"www.contoh.id":               true,
		"lihat contoh.com":            true,
		"nomor +62 812-3456-7890":     true,
		"atas nama [NAMA]":            true,
		"sudah 3 minggu":              false,
		"tahun 2024":                  false,
		"aku capek. aku sedih":        false,
	} {
		if got := containsIdentifier(in); got != want {
			t.Errorf("containsIdentifier(%q) = %v, want %v", in, got, want)
		}
	}
	tests := []struct {
		text, source string
		want         bool
	}{
		{"3 minggu", "sudah 3 minggu", false},
		{"5 minggu", "sudah 3 minggu", true},
		{"tiga minggu", "sudah 3 minggu", true},
		{"50%", "50 persen", true},
		{"1,5 jam", "1.5 jam", false},
		{"twice a week", "every week", true},
		{"sedih", "sedih", false},
	}
	for _, tc := range tests {
		if got := inventsNumber(tc.text, tc.source); got != tc.want {
			t.Errorf("inventsNumber(%q, %q) = %v, want %v", tc.text, tc.source, got, tc.want)
		}
	}
}

func TestNormalizeForGrounding(t *testing.T) {
	for in, want := range map[string]string{
		"  Aku  SUSAH\ttidur ": "aku susah tidur",
		"“kamu lebay”":         `"kamu lebay"`,
		"it’s ok":              "it's ok",
		"ｆｕｌｌ　ｗｉｄｔｈ":           "full width",
		"zero​width":           "zerowidth",
		"café café":           "cafe cafe",
		"tunggu…":              "tunggu...",
		"ﬁne":                  "fine",
		"bad\xffbyte":          "bad�byte",
	} {
		if got := normalizeForGrounding(in); got != want {
			t.Errorf("normalizeForGrounding(%q) = %q, want %q", in, got, want)
		}
	}
}
