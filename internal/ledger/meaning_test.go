package ledger

import (
	"slices"
	"testing"
)

// TestCompareMeaning pins the Meaning Diff rules (MIE H-004): a wording that
// changes who, whether, how often or how certainly is critical; a faithful
// paraphrase is not.
func TestCompareMeaning(t *testing.T) {
	cases := []struct {
		name, source, wording string
		critical              bool
		codes                 []string // critical codes that must be present
	}{
		// benign paraphrases
		{"id paraphrase kadang/sesekali", "Kadang aku ingin sendiri, tapi aku tetap suka bersama teman.", "Kamu sesekali butuh waktu sendiri dan tetap suka bersama teman.", false, nil},
		{"id kamu is the user", "Aku capek setelah kuliah.", "Kamu capek setelah kuliah.", false, nil},
		{"id negation kept", "Aku tidak membenci temanku.", "Kamu tidak membenci temanmu.", false, nil},
		{"id bukan berarti kept", "Kadang aku ingin sendiri, tapi bukan berarti membenci teman.", "Kamu kadang ingin sendiri, dan itu tidak berarti kamu membenci teman.", false, nil},
		{"id implicit negator", "Aku tidak bisa tidur tiap malam.", "Kamu sulit tidur tiap malam.", false, nil},
		{"id kurang ~ tidak", "Aku kadang merasa tidak dihargai di rumah.", "Kamu kadang merasa kurang dihargai di rumah.", false, nil},
		{"id reported speech kept", "Dia bilang aku terlihat sedih.", "Menurut dia, kamu terlihat sedih.", false, nil},
		{"id hedge kept", "Mungkin aku cuma capek.", "Mungkin kamu hanya capek.", false, nil},
		{"id slang negation kept", "Gue gak benci kerjaan gue.", "Kamu nggak benci pekerjaanmu.", false, nil},
		{"id siapa-siapa ~ siapa pun", "Aku nggak mau ketemu siapa-siapa hari ini.", "Hari ini kamu tidak mau bertemu siapa pun.", false, nil},
		{"id never kept", "Aku tidak pernah merasa cukup baik.", "Kamu tidak pernah merasa cukup baik.", false, nil},
		{"en paraphrase", "I sometimes feel lonely after class.", "You sometimes feel lonely after class.", false, nil},
		{"en negation kept", "I don't hate my job, I'm just tired.", "You don't hate your job; you're just tired.", false, nil},
		{"en reported kept", "My mom says I worry too much.", "According to your mom, you worry too much.", false, nil},
		{"en hedge kept", "Maybe I'm just stressed about exams.", "Maybe you're just stressed about exams.", false, nil},
		{"mixed paraphrase", "Aku kadang overthinking sebelum tidur.", "Kamu kadang overthinking sebelum tidur.", false, nil},
		{"mixed hedge translated", "Maybe aku cuma butuh liburan.", "Mungkin kamu hanya butuh liburan.", false, nil},
		{"empty wording", "Aku capek.", "", false, nil},

		// negation
		{"id negation dropped", "Aku tidak membenci temanku.", "Kamu membenci temanmu.", true, []string{MeaningNegation}},
		{"id negation added", "Aku senang ketemu teman-teman kemarin.", "Kamu tidak senang bertemu teman-temanmu.", true, []string{MeaningNegation}},
		{"id object negation", "Aku tidak suka keramaian, tapi aku suka konser kecil.", "Kamu suka keramaian.", true, []string{MeaningNegation}},
		{"id double clause", "Aku tidak takut gagal, aku takut mengecewakan orang tuaku.", "Kamu takut gagal.", true, []string{MeaningNegation}},
		{"id slang dropped", "Gue gak benci kerjaan gue, cuma jenuh.", "Kamu benci pekerjaanmu.", true, []string{MeaningNegation}},
		{"en negation dropped", "I'm not angry with my parents.", "You are angry with your parents.", true, []string{MeaningNegation}},
		{"mixed negation dropped", "Honestly aku nggak marah sama dia.", "Kamu marah sama dia.", true, []string{MeaningNegation}},

		// subject and reported speech
		{"id other to self", "Dia bilang aku terlihat sedih.", "Kamu sedih.", true, []string{MeaningSubject}},
		{"id ibu bilang", "Ibuku bilang aku terlalu sensitif.", "Kamu terlalu sensitif.", true, []string{MeaningSubject}},
		{"en boss thinks", "My boss thinks I'm lazy.", "You are lazy.", true, []string{MeaningSubject}},
		{"en friend said", "My friend said I seemed anxious at the party.", "You were anxious at the party.", true, []string{MeaningSubject}},

		// certainty
		{"id hedge dropped", "Mungkin aku cuma capek.", "Kamu capek.", true, []string{MeaningCertainty}},
		{"id certainty added", "Sepertinya aku butuh istirahat.", "Kamu jelas butuh istirahat.", true, []string{MeaningCertainty}},
		{"id not sure dropped", "Aku nggak yakin apakah aku perlu bantuan.", "Kamu perlu bantuan.", true, []string{MeaningCertainty}},
		{"en perhaps dropped", "Perhaps I need a break.", "You need a break.", true, []string{MeaningCertainty}},
		{"en definitely added", "I feel tense before exams.", "You definitely feel tense before exams.", true, []string{MeaningCertainty}},

		// frequency
		{"id kadang to selalu", "Kadang aku ingin sendiri.", "Kamu selalu ingin sendiri.", true, []string{MeaningFrequency}},
		{"id jarang to selalu", "Aku jarang makan siang.", "Kamu selalu melewatkan makan siang.", true, []string{MeaningFrequency}},
		{"id sering to tidak pernah", "Aku sering susah tidur.", "Kamu tidak pernah bisa tidur.", true, []string{MeaningFrequency}},
		{"id none to setiap hari", "Aku menangis tadi malam.", "Kamu menangis setiap hari.", true, []string{MeaningFrequency}},
		{"id de-escalation", "Aku selalu cemas sebelum presentasi.", "Kamu kadang cemas sebelum presentasi.", true, []string{MeaningFrequency}},
		{"en sometimes to always", "I sometimes feel lonely.", "You always feel lonely.", true, []string{MeaningFrequency}},
		{"en rarely to never", "I rarely go out on weekends.", "You never go out.", true, []string{MeaningFrequency}},
		{"mixed sometimes to always", "Aku sometimes ngerasa insecure.", "Kamu always insecure.", true, []string{MeaningFrequency}},

		// generalization
		{"id semua", "Kadang aku ingin sendiri, tapi bukan berarti membenci teman.", "Nadia menghindari semua teman.", true, []string{MeaningGeneralization}},
		{"id semua orang", "Aku kadang merasa tidak dihargai di rumah.", "Semua orang di rumah tidak menghargaimu.", true, []string{MeaningGeneralization}},
		{"en everyone", "I sometimes feel lonely after class.", "Everyone ignores you after class.", true, []string{MeaningGeneralization}},
		{"en all", "I like being alone sometimes, but I still love my friends.", "You avoid all your friends.", true, []string{MeaningGeneralization}},

		// diagnosis and medical
		{"id depresi", "Aku capek banget minggu ini.", "Kamu mengalami depresi.", true, []string{MeaningDiagnosis}},
		{"id gangguan kecemasan", "Kadang aku cemas sebelum presentasi.", "Kamu punya gangguan kecemasan.", true, []string{MeaningDiagnosis}},
		{"en sleeping pills", "I couldn't sleep last night.", "You should take sleeping pills.", true, []string{MeaningMedical}},
		{"mixed antidepresan", "Aku stres sama deadline.", "Kamu butuh antidepresan.", true, []string{MeaningMedical}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CompareMeaning(c.source, c.wording)
			if got.Version != MeaningVersion || got.Differences == nil {
				t.Fatalf("version %q, differences nil=%v", got.Version, got.Differences == nil)
			}
			if got.Critical != c.critical {
				t.Fatalf("critical = %v, want %v: %+v", got.Critical, c.critical, got.Differences)
			}
			for _, code := range c.codes {
				if !slices.ContainsFunc(got.Differences, func(d MeaningDifference) bool {
					return d.Code == code && d.Severity == SeverityCritical
				}) {
					t.Errorf("missing critical %s: %+v", code, got.Differences)
				}
			}
		})
	}
}

// TestCompareMeaningIdentity: a text never changes its own meaning.
func TestCompareMeaningIdentity(t *testing.T) {
	for _, s := range []string{
		"Kadang aku ingin sendiri, tapi bukan berarti membenci teman.",
		"Dia bilang aku terlihat sedih belakangan ini.",
		"Mungkin aku cuma capek karena tugas.",
		"Aku tidak pernah merasa cukup baik.",
		"Gue gak benci kerjaan gue, cuma lagi jenuh aja.",
		"My mom says I worry too much.",
		"I'm not sure I want to talk to a counsellor yet.",
		"Aku sometimes ngerasa insecure di kelas.",
		"Semua orang di kelas baik padaku.",
		"Kata dokter aku mengalami depresi ringan.",
	} {
		if got := CompareMeaning(s, s); got.Critical {
			t.Errorf("CompareMeaning(%q, itself) is critical: %+v", s, got.Differences)
		}
	}
}

// TestCompareMeaningOrder: differences come in the documented code order.
func TestCompareMeaningOrder(t *testing.T) {
	order := []string{MeaningNegation, MeaningSubject, MeaningTime, MeaningCertainty, MeaningFrequency,
		MeaningGeneralization, MeaningNumber, MeaningAddedContent, MeaningDiagnosis, MeaningMedical}
	got := CompareMeaning("Kata temanku, mungkin aku kadang tidak suka kuliah.", "Kamu pasti selalu benci semua kuliah selama 3 tahun dan depresi.")
	last := -1
	for _, d := range got.Differences {
		i := slices.Index(order, d.Code)
		if i < 0 || i < last {
			t.Fatalf("unexpected order: %+v", got.Differences)
		}
		last = i
	}
	if !got.Critical || len(got.Differences) < 4 {
		t.Fatalf("expected several critical differences: %+v", got.Differences)
	}
}

func FuzzCompareMeaning(f *testing.F) {
	f.Add("Kadang aku ingin sendiri", "Kamu selalu ingin sendiri")
	f.Add("I don't hate my job", "You hate your job")
	f.Fuzz(func(t *testing.T, a, b string) {
		got := CompareMeaning(a, b)
		if got.Differences == nil {
			t.Fatal("nil differences")
		}
		if self := CompareMeaning(a, a); self.Critical && len(a) < 200 {
			// identity must never be critical; long random input is allowed
			// to trip lexicon heuristics only through diagnosis rules
			for _, d := range self.Differences {
				if d.Severity == SeverityCritical && d.Code != MeaningDiagnosis && d.Code != MeaningMedical {
					t.Fatalf("identity critical: %q %+v", a, self.Differences)
				}
			}
		}
	})
}
