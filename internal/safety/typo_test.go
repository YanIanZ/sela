package safety

import "testing"

func TestEditDistanceOne(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"bnuh", "bunuh", true},     // deletion
		{"bunnuh", "bunuh", true},   // insertion
		{"bunah", "bunuh", true},    // substitution
		{"bnuuh", "bunuh", true},    // adjacent transposition
		{"gantug", "gantung", true}, // deletion
		{"bunuh", "bunuh", false},   // identical
		{"bnh", "bunuh", false},     // two deletions
		{"bnauh", "bunuh", false},   // two substitutions
		{"ubnhu", "bunuh", false},   // two separate swaps
		{"bunüh", "bunuh", false},   // non-ASCII is never corrected
	}
	for _, c := range cases {
		if got := editDistanceOne(c.a, c.b); got != c.want {
			t.Errorf("editDistanceOne(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestTypoVariant(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		changed bool
	}{
		// abbreviations expand, whatever their length
		{"aku mau bnuh dri", "aku mau bunuh diri", true},
		{"pengen mtii aja", "pengen mati aja", true},
		{"i want to kil myslf", "i want to kill myself", true},
		// one edit from a long risk word is corrected; punctuation stays
		{"aku mau gantug diri.", "aku mau gantung diri.", true},
		{"(bunnuh)", "(bunuh)", true},
		// everyday words one edit away are kept
		{"aku butuh teman", "", false},
		{"aku mampu kok", "", false},
		{"he hurt himself", "", false},
		// short words are never corrected by distance
		{"aku mau mata", "", false},
		{"semua baik", "", false},
	}
	for _, c := range cases {
		got, changed := typoVariant(c.in)
		if changed != c.changed || got != c.want {
			t.Errorf("typoVariant(%q) = (%q, %v), want (%q, %v)", c.in, got, changed, c.want, c.changed)
		}
	}
}
