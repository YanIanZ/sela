package safety

import "testing"

// TestDangerLexicon pins the danger lexicon of ruleset 2026.11.01-1: threats
// to others, the speaker in danger, prepared means and farewells interrupt;
// a denial or a game context only raises support; figurative and everyday
// phrasings stay ordinary.
func TestDangerLexicon(t *testing.T) {
	cases := []struct {
		text string
		want Level
	}{
		// threats to others
		{"aku mau bunuh dia", LevelCrisis},
		{"gw mo nusuk org itu", LevelCrisis},
		{"aku pengen nusuk orang itu", LevelCrisis},
		{"aku bakal bikin dia mati", LevelCrisis},
		{"aku mau bakar rumahnya", LevelCrisis},
		{"aku bawa pisau buat dia besok", LevelCrisis},
		{"aku mau nembak orang di sekolah", LevelCrisis},
		{"aku mau ngebom kantor", LevelCrisis},
		{"aku mau habisin nyawa dia", LevelCrisis},
		{"I'm going to kill him", LevelCrisis},
		{"I'll shoot everyone at school", LevelCrisis},
		{"I'm bringing a gun to school", LevelCrisis},
		// the speaker in danger
		{"suamiku mau bunuh aku", LevelCrisis},
		{"aku dipukulin terus sama ayah", LevelCrisis},
		{"aku diperkosa", LevelCrisis},
		{"pacarku cekik aku tadi", LevelCrisis},
		{"aku nggak aman di rumah", LevelCrisis},
		{"aku korban kdrt", LevelCrisis},
		{"my dad beats me every night", LevelCrisis},
		{"someone is threatening to kill me", LevelCrisis},
		{"I was raped", LevelCrisis},
		{"I'm not safe at home", LevelCrisis},
		// prepared means, farewells, vanishing, self-harm
		{"aku udah nyiapin tali", LevelCrisis},
		{"ini pesan terakhirku", LevelCrisis},
		{"besok aku udah nggak ada", LevelCrisis},
		{"aku mau nyusul almarhum ibu", LevelCrisis},
		{"lebih baik aku pergi selamanya", LevelCrisis},
		{"rasanya pengen ilang dari dunia ini", LevelCrisis},
		{"aku barusan mukul-mukul kepala ke tembok", LevelCrisis},
		{"I have pills ready", LevelCrisis},
		{"this is my last message", LevelCrisis},
		// denial and play: support only
		{"aku nggak mau bunuh dia", LevelElevated},
		{"aku nggak pernah dipukul ortu", LevelElevated},
		{"aku mau habisin dia di ranked", LevelElevated},
		{"I'm gonna kill him in fortnite", LevelElevated},
		// everyday and figurative
		{"aku mau nembak dia besok", LevelNone},
		{"dia mau nembak aku katanya", LevelNone},
		{"aku mau bunuh nyamuk", LevelNone},
		{"aku pengen bunuh waktu", LevelNone},
		{"aku mau bakar sate", LevelNone},
		{"aku bawa pisau buat masak", LevelNone},
		{"aku dipukul bola tadi pas futsal", LevelNone},
		{"aku ditampar realita", LevelNone},
		{"aku dihajar tugas", LevelNone},
		{"dia ngancam aku putus", LevelNone},
		{"besok aku udah nggak ada di kantor", LevelNone},
		{"aku beli tali sepatu", LevelNone},
		{"aku nggak aman sama kerjaanku", LevelNone},
		{"aku pamit pergi selamanya dari grup ini", LevelNone},
		{"aku mau bom chat dia", LevelNone},
		{"I'm going to kill it at the interview", LevelNone},
		{"I killed the presentation", LevelNone},
		{"my boss hits me with deadlines", LevelNone},
		{"I'm not suicidal just need a break or i'll hurt someone's feelings", LevelNone},
	}
	for _, c := range cases {
		if got := Assess(c.text).Level; got != c.want {
			t.Errorf("Assess(%q) = %s, want %s", c.text, got, c.want)
		}
	}
}
