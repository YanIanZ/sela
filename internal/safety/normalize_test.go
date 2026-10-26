package safety

import (
	"slices"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want, why string }{
		// v1 crisisEngine.test.mjs, "Text Normalization & Anti-Evasion".
		{"bund1r", "bundir", "v1 leetspeak"},
		{"b4yg0n", "baygon", "v1 leetspeak"},
		{"m4t1", "mati", "v1 leetspeak"},
		{"matiiiiii", "mati", "v1 elongation"},
		{"buuundir", "bundir", "v1 elongation"},
		{"", "", "v1 empty input"},
		// v1 crisisRegression.test.mjs normalisation inputs.
		{"Aku b u n u h d i r i malam ini", "aku bunuh diri malam ini", "v1 spaced letters"},
		{"Aku bu-nuh di-ri malam ini", "aku bunuh diri malam ini", "v1 dashed letters"},
		{"Aku ingin mengakhiri hid\u200bupku malam ini", "aku ingin mengakhiri hidupku malam ini", "v1 zero-width space"},
		{"I don\u2019t want to live anymore", "i don't want to live anymore", "v1 curly apostrophe"},
		{"I want to   die", "i want to die", "v1 elongated spaces"},
		{"k1ll mys3lf", "kill myself", "v1 leetspeak"},
		// Sela additions.
		{"    ", "", "whitespace only"},
		{"\u200b\u200d\ufeff", "", "invisible characters only"},
		{"BUNUH DIRI", "bunuh diri", "lowercase"},
		{"b\u00fan\u00fah d\u00edr\u00ed", "bunuh diri", "precomposed accents"},
		{"bu\u0301nuh diri", "bunuh diri", "combining accent"},
		{"\uff42\uff55\uff4e\uff55\uff48 \uff44\uff49\uff52\uff49", "bunuh diri", "fullwidth"},
		{"\U0001D41B\U0001D42E\U0001D427\U0001D42E\U0001D421", "bunuh", "mathematical bold"},
		{"\u24b7\u24ca\u24c3\u24ca\u24bd", "bunuh", "circled letters"},
		{"bunuh dir\u0456", "bunuh diri", "Cyrillic look-alike"},
		{"\ufb01ne", "fine", "ligature"},
		{"bunuh\u00a0diri", "bunuh diri", "no-break space"},
		{"bunuh\u2013diri", "bunuh diri", "en dash joins like a dash"},
		{"\u201caku\u201d", "\"aku\"", "curly double quotes"},
		{"tab\there", "tab here", "tab"},
		{"line\r\nbreak", "line\n\nbreak", "CR becomes a newline"},
		{"k i l l myself", "kill myself", "single-letter run"},
		{"m.a.t.i", "mati", "dotted single letters"},
		{"b u n u h d 1 r 1", "bunuh diri", "spaced leetspeak"},
		{"m 4 t 1 aja", "mati aja", "spaced leetspeak"},
		{"a b", "a b", "two single letters are left alone"},
		{"jam 1 pagi", "jam 1 pagi", "free-standing digits stay"},
		{"5 kms", "5 kms", "free-standing digits stay"},
		{"su1c1d4l", "suicidal", "digits inside a word"},
		{"aku   capek", "aku capek", "space runs"},
		{"hp-ku mati", "hp-ku mati", "hyphens inside words stay in the canonical form"},
		{"killll", "kil", "v1 collapses to one rune (see variants)"},
		{"\xff", "\ufffd", "invalid UTF-8 becomes U+FFFD"},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q (%s)", c.in, got, c.want, c.why)
		}
	}
}

func TestVariants(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"aku capek", []string{"aku capek"}},
		{"gan-tung di-ri", []string{"gan-tung di-ri", "gantung diri"}},
		{"killll myself", []string{"kil myself", "kill myself"}},
		{"mau-matiii", []string{"mau-mati", "maumati", "mau-matii"}},
		{"", []string{""}},
	}
	for _, c := range cases {
		got := variants(c.in)
		if !slices.Equal(got, c.want) {
			t.Errorf("variants(%q) = %q, want %q", c.in, got, c.want)
		}
		if got[0] != Normalize(c.in) {
			t.Errorf("variants(%q)[0] is not Normalize", c.in)
		}
	}
}

func TestNormalizeHelpers(t *testing.T) {
	if got := collapseRuns("aaabbbbcc", 2); got != "aabbcc" {
		t.Errorf("collapseRuns keep 2 = %q", got)
	}
	if got := collapseRuns("\u00e9\u00e9\u00e9!", 1); got != "\u00e9!" {
		t.Errorf("collapseRuns multibyte = %q", got)
	}
	if hasRun3("aabbcc") || !hasRun3("abccc") {
		t.Error("hasRun3")
	}
	if got := rejoinBunuhDiri("xbunuh diri"); got != "xbunuh diri" {
		t.Errorf("rejoin must respect the word boundary: %q", got)
	}
	if got := rejoinBunuhDiri("b_u.n-u h\nd i r i!"); got != "bunuh diri!" {
		t.Errorf("rejoin separators: %q", got)
	}
	if got := rejoinBunuhDiri("bunuh dirinya"); got != "bunuh dirinya" {
		t.Errorf("rejoin must respect the trailing boundary: %q", got)
	}
	if got := compactJoin("e-mail ok. next"); got != "email ok. next" {
		t.Errorf("compactJoin = %q", got)
	}
}
