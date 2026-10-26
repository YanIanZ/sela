package fakellm

import (
	"reflect"
	"testing"
)

func TestTokenizeKeepsInnerApostrophesAndHyphens(t *testing.T) {
	text := "I DON’T know, no-one-ish 'quoted' gak-bisa 3-4."
	var got []string
	for _, tk := range tokenize(text) {
		got = append(got, tk.word)
		if text[tk.start:tk.end] == "" {
			t.Fatalf("empty span for %q", tk.word)
		}
	}
	want := []string{"i", "don't", "know", "no-one-ish", "quoted", "gak-bisa", "3", "4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokenize = %q, want %q", got, want)
	}
}

func TestIsNegatorMirrorsLedgerLexicon(t *testing.T) {
	yes := []string{
		"tidak", "nggak", "ngak", "nda", "kaga", "bukannya", "belum", "no-one", "noone", "nowhere", "neither", "nor",
		"dont", "cant", "wont", "dunno", "don't", "can't", "isn't", "gabisa", "gamau", "gaada", "gatau", "nggakbisa",
		"ngakmau", "ndakuat", "kagakada", "tdkbisa", "gbisa", "gada", "gpp", "gtau", "gayakin",
	}
	no := []string{"gagal", "galau", "ganti", "gakbisa-bisa", "tidakk", "notes", "nono", "knot", "nation", "bisa", "gak-bisa"}
	for _, w := range yes {
		if !isNegator(w) {
			t.Errorf("isNegator(%q) = false", w)
		}
	}
	for _, w := range no {
		if isNegator(w) {
			t.Errorf("isNegator(%q) = true", w)
		}
	}
}

func TestDropNegators(t *testing.T) {
	cases := []struct {
		in, want string
		dropped  bool
	}{
		{"Aku tidak bisa tidur.", "Aku bisa tidur.", true},
		{"Aku gabisa tidur nyenyak.", "Aku bisa tidur nyenyak.", true},
		{"Aku Gamau ketemu siapa-siapa.", "Aku Mau ketemu siapa-siapa.", true},
		{"I dont sleep well anymore.", "I sleep well anymore.", true},
		{"I don’t feel like going.", "I feel like going.", true},
		{"Tidak bisa tidur.", "Bisa tidur.", true},
		{"Neither my friends nor family understand.", "My friends family understand.", true},
		{"Aku capek, tidak.", "Aku capek,.", true},
		{"Aku gpp kok.", "Aku kok.", true},
		{"Aku capek banget.", "Aku capek banget.", false},
		{"Aku gagal lagi.", "Aku gagal lagi.", false},
	}
	for _, tc := range cases {
		got, dropped := dropNegators(tc.in)
		if got != tc.want || dropped != tc.dropped {
			t.Errorf("dropNegators(%q) = %q, %v; want %q, %v", tc.in, got, dropped, tc.want, tc.dropped)
		}
		if dropped && hasNegator(got) {
			t.Errorf("dropNegators(%q) = %q still has a negator", tc.in, got)
		}
	}
}

func TestFlipNegationNeverNoOp(t *testing.T) {
	cases := []struct{ in, lang, want string }{
		{"Aku tidak bisa tidur.", "id", "Aku bisa tidur."},
		{"Aku capek banget.", "id", "Aku tidak capek banget."},
		{"I feel overwhelmed.", "en", "I not feel overwhelmed."},
		{"Capek.", "id", "tidak Capek."},
		{"...", "en", "not ..."},
	}
	for _, tc := range cases {
		got := flipNegation(tc.in, tc.lang)
		if got != tc.want {
			t.Errorf("flipNegation(%q, %q) = %q, want %q", tc.in, tc.lang, got, tc.want)
		}
		if got == tc.in || hasNegator(got) == hasNegator(tc.in) {
			t.Errorf("flipNegation(%q) = %q did not reverse polarity", tc.in, got)
		}
	}
}
