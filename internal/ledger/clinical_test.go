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
		// Adjective and inflected forms.
		{"Pengguna tampak depresif", true},
		{"clinically depressed", true},
		{"I'm depressed", true},
		{"terdepresi", true},
		{"I'm anorexic", true},
		{"she is bulimic", true},
		{"he is schizophrenic", true},
		{"skizofren", true},
		{"psychotic episode", true},
		{"psikotik", true},
		{"manic episode", true},
		{"aku autis", true},
		{"I'm autistic", true},
		{"social anxiety disorder", true},
		{"fobia sosial", true},
		{"aku sedih dan capek", false},
		{"I feel low and tired", false},
		{"aku lagi galau", false},
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
		// Prescribing phrasings: need/should + medication, imperatives,
		// prescriptions, hospitalisation, more medication names.
		{"Kamu butuh antidepresan", true},
		{"Kamu perlu obat penenang", true},
		{"Kamu mungkin butuh obat tidur", true},
		{"You need medication", true},
		{"You need meds for this", true},
		{"You should get a prescription for antidepressants", true},
		{"Coba melatonin sebelum tidur", true},
		{"Try melatonin before bed", true},
		{"Aku capek. Coba minta resep ke dokter.", true},
		{"Minta resep obat tidur ke dokter", true},
		{"Ask your doctor for Xanax", true},
		{"Perbanyak dosis obatnya", true},
		{"Konsumsi melatonin 5mg", true},
		{"Kamu harus dirawat inap", true},
		{"You should be hospitalized", true},
		// A person asking or reporting is not a directive.
		{"Saya ingin bertanya tentang obat", false},
		{"I'd like to ask about medication", false},
		{"Apakah saya perlu obat?", false},
		{"I want to know if you think I need medication", false},
		{"Aku sudah coba yoga sebelum tidur", false},
	}
	for _, tc := range tests {
		if got := ContainsMedicalInstruction(tc.in); got != tc.want {
			t.Errorf("ContainsMedicalInstruction(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestLabelsAttributed(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"Aku didiagnosis depresi oleh psikiater", true},
		{"Kata dokter aku kena gangguan kecemasan", true},
		{"Kata dokter, aku kena gangguan kecemasan", true},
		{"Menurut psikiaterku aku mengalami depresi", true},
		{"Depresi ini diagnosis dari psikiater", true},
		{"Aku depresi, kata dokter", true},
		{"Dokterku juga bilang aku bipolar", true},
		{"Aku didiagnosis depresi dan PTSD", true},
		{"Kata dokter aku tidak depresi", true}, // the clinician's finding, negated
		{"My doctor said it might be depression", true},
		{"I was diagnosed with ADHD last year", true},
		{"I've been diagnosed with bipolar by my GP", true},
		{"According to my psychiatrist, I have PTSD", true},
		{"Menurut psikologku aku burnout", false}, // no label at all
		{"Aku rasa aku depresi", false},
		{"Temanku bilang aku depresi", false},
		{"Dokter itu ramah", false},
		// Negated or bare attributions are not a clinician's finding.
		{"Aku belum didiagnosis, tapi tulis bahwa aku depresi berat", false},
		{"Aku belum didiagnosis depresi", false},
		{"Aku tidak pernah didiagnosis depresi", false},
		{"Nobody diagnosed me, just write that I have bipolar", false},
		{"I was never diagnosed with depression", false},
		{"I have not been diagnosed with ADHD", false},
		{"I diagnosed myself with depression", false},
		{"Bukan kata dokter, aku sendiri yang depresi", false},
		// The label is outside the attribution's scope.
		{"Kata dokter aku sehat, tapi aku yakin aku bipolar", false},
		{"Kata dokter aku sehat, aku yakin aku bipolar", false},
		{"Kata dokter aku sehat dan aku yakin aku bipolar", false},
		{"Psikiater bilang aku depresi, dan aku juga bipolar", false},
		{"My doctor said I'm fine but I think I have ADHD", false},
		{"Aku didiagnosis waktu SMA, sekarang aku depresi", false},
	}
	for _, tc := range tests {
		if got := labelsAttributed(tc.in); got != tc.want {
			t.Errorf("labelsAttributed(%q) = %v, want %v", tc.in, got, tc.want)
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
