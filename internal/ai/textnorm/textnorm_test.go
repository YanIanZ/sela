package textnorm

import "testing"

func TestFold(t *testing.T) {
	for in, want := range map[string]string{
		"Ｄｅｗｉ":  "dewi",
		"Déwi":  "dewi",
		"De​wi": "dewi",
		"Dеwi":  "dewi", // Cyrillic е
		"Don't": "dont",
		"A–B":   "a–b", // Fold keeps dashes and spaces as they are
	} {
		if got := Fold(in); got != want {
			t.Errorf("Fold(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestViewOffsets: the view maps Unicode spaces and dashes to ASCII, keeps
// case, drops zero-width characters, and maps every range back to the
// original bytes.
func TestViewOffsets(t *testing.T) {
	in := "WA 0812–3456 789 Ｄｅ​wi"
	v := NewView(in)
	if v.Text != "WA 0812-3456 789 Dewi" {
		t.Fatalf("view %q", v.Text)
	}
	start, end := v.Orig(3, 16)
	if in[start:end] != "0812–3456 789" {
		t.Fatalf("orig %q", in[start:end])
	}
	start, end = v.Orig(17, len(v.Text))
	if in[start:end] != "Ｄｅ​wi" {
		t.Fatalf("orig %q", in[start:end])
	}
	if v.Original() != in {
		t.Fatal("original")
	}
}

func TestFindName(t *testing.T) {
	for _, tc := range []struct {
		text, name string
		n          int
	}{
		{NewView("Budi Santoso bilang").Text, "Budi Santoso", 1},
		{NewView("Budi  Santoso dan budi\nsantoso").Text, "budi santoso", 2},
		{NewView("Budiman temanku").Text, "Budi", 0},
		{NewView("Zoë marah").Text, "Zoe", 1},
		{NewView("Zoe marah").Text, "Zoë", 1},
		{Fold("ALI dan Alis"), "Ali", 1}, // "Alis" is another word
		{NewView("A marah").Text, "A", 0},
		{NewView("x").Text, "  ", 0},
	} {
		if got := len(FindName(tc.text, tc.name)); got != tc.n {
			t.Errorf("FindName(%q, %q) = %d, want %d", tc.text, tc.name, got, tc.n)
		}
	}
}

// TestFindNameForms: a name followed by a possessive, an Indonesian
// enclitic or particle, the reduplication digit, or with its last letter
// repeated, is the name; another word that starts with the name is not.
// The match covers the name (and repeated letters), never the suffix.
func TestFindNameForms(t *testing.T) {
	for _, tc := range []struct {
		text, name, want string
	}{
		{"I told Raka's mom", "Raka", "Raka"},
		{"Raka’s family", "Raka", "Raka"},
		{"Raka‘s family", "Raka", "Raka"},
		{"Rakanya nggak bales", "Raka", "Raka"},
		{"aku kangen Dewiku", "Dewi", "Dewi"},
		{"Dewimu marah", "Dewi", "Dewi"},
		{"Rakalah yang salah", "Raka", "Raka"},
		{"Rakapun tahu", "Raka", "Raka"},
		{"Rakakah itu?", "Raka", "Raka"},
		{"Dewinyalah yang salah", "Dewi", "Dewi"},
		{"Dewi2 itu temenku", "Dewi", "Dewi"},
		{"Dewiii kenapa sih", "Dewi", "Dewiii"},
		{"Dewiiiku kenapa", "Dewi", "Dewiii"},
		{"Ma'ruf datang", "Maruf", "Ma'ruf"},
		{"Ma’ruf datang", "Ma'ruf", "Ma’ruf"},
		{"Raka Putranya bilang", "Raka Putra", "Raka Putra"},
		{"dewi_lestari bilang", "Dewi", "dewi"},
	} {
		v := NewNameView(tc.text)
		got := FindName(v.Text, tc.name)
		if len(got) != 1 {
			t.Errorf("FindName(%q, %q) = %v, want one match", tc.text, tc.name, got)
			continue
		}
		start, end := v.Orig(got[0].Start, got[0].End)
		if tc.text[start:end] != tc.want {
			t.Errorf("FindName(%q, %q) matched %q, want %q", tc.text, tc.name, tc.text[start:end], tc.want)
		}
	}
	for _, tc := range []struct{ text, name string }{
		{"Budiman temanku", "Budi"},
		{"Rakasa itu", "Raka"},
		{"Rakanyaman", "Raka"},
		{"kaku sekali", "Ka"}, // no suffixes for names under three letters
		{"Bungalow", "Bunga"},
	} {
		if got := FindName(NewNameView(tc.text).Text, tc.name); len(got) != 0 {
			t.Errorf("FindName(%q, %q) = %v, want none", tc.text, tc.name, got)
		}
	}
}

// TestNameViewKeepsApostrophes: the name view maps every apostrophe to
// ASCII and keeps it; the contact view and Fold drop it.
func TestNameViewKeepsApostrophes(t *testing.T) {
	in := "Raka’s ０８１２'3456"
	if v := NewNameView(in); v.Text != "Raka's 0812'3456" {
		t.Fatalf("name view %q", v.Text)
	}
	if v := NewView(in); v.Text != "Rakas 08123456" {
		t.Fatalf("view %q", v.Text)
	}
	if got := Fold("Don´t ＇x"); got != "dont x" {
		t.Fatalf("Fold %q", got)
	}
}
