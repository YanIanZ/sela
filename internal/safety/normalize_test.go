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
		// Review findings: NFKD compatibility forms v1 folded, and evasions.
		{"\u1d43\u1d4f\u1d58 \u1d50\u1d43\u1d58 \u1d50\u1d43\u1d57\u2071", "aku mau mati", "modifier letters"},
		{"\u1d47\u1d58\u207f\u1d58\u02b0 \u1d48\u2071\u02b3\u2071", "bunuh diri", "modifier and superscript letters"},
		{"\u2090\u2096\u1d64", "aku", "subscript letters"},
		{"\u0299\u1d1c\u0274\u1d1c\u029c \u1d05\u026a\u0280\u026a", "bunuh diri", "small capitals"},
		{"kill \u1d0dyself", "kill myself", "small capital m"},
		{"bunuh\u3164diri", "bunuh diri", "Hangul filler"},
		{"mau\u2800mati", "mau mati", "blank Braille pattern"},
		{"pengen\uffa0mati", "pengen mati", "halfwidth Hangul filler"},
		{"\u115f\u1160", "", "Hangul choseong and jungseong fillers"},
		{"\u0412UNU\u041d \u041c\u0410\u0422\u0406", "bunuh mati", "Cyrillic capitals VE, EN, EM, TE"},
		{"\u039d\u0397\u039c\u0392\u03a5\u0396", "nhmbyz", "Greek capitals fold by shape before lowercasing"},
		{"m\u2074t\u00b9 \u2074\u2083", "mati 43", "superscript and subscript digits fold to digits; leet applies inside words"},
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
		{"1 hate my life", []string{"1 hate my life", "i hate my life"}},
		{"1'm done", []string{"1'm done", "i'm done"}},
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

func TestOneAsPronoun(t *testing.T) {
	cases := map[string]string{
		"1 hate my life":            "i hate my life",
		"if 1 was gone":             "if i was gone",
		"1 wish 1 could":            "i wish i could",
		"\"1\" said":                "\"i\" said",
		"5 kms":                     "5 kms",
		"1 kms from home":           "1 kms from home",
		"jam 1 pagi":                "jam i pagi",
		"jam 1":                     "jam i",
		"1 jam lagi":                "1 jam lagi",
		"10 orang":                  "10 orang",
		"1.5 km":                    "1.5 km",
		"page-1":                    "page-1",
		"a1":                        "a1",
		"rp 1,000":                  "rp 1,000",
		"no ones here, just 1 of 3": "no ones here, just 1 of 3",
	}
	for in, want := range cases {
		if got := oneAsPronoun(in); got != want {
			t.Errorf("oneAsPronoun(%q) = %q, want %q", in, got, want)
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

// TestLexicalVariant covers the readings only the derived variants make
// (review findings): two-letter elongation inside a word, a pause that an
// emoji follows, spaced dots and dashes, and mixed leetspeak.
func TestLexicalVariant(t *testing.T) {
	lex := map[string]string{
		"aku pengeen mati":       "aku pengen mati",
		"pengen bunuuh diri":     "pengen bunuh diri",
		"i want to kiill myself": "i want to kill myself",
		"i want to diieee":       "i want to die",
		// A dot pause or a pause with a newline keeps the canonical
		// form's clause boundary as a newline, which the patterns' \s*
		// crosses.
		"aku pengen... 😭 mati": "aku pengen\nmati",
		"aku pengen…😭mati":     "aku pengen\nmati",
		"i want to . . . die":  "i want to\ndie",
		"aku pengen...\nmati":  "aku pengen\nmati",
		"aku pengen —\nmati":   "aku pengen\nmati",
		"i want to — die":      "i want to die",
		"aku pengen - mati":    "aku pengen mati",
		"hp-ku mati":           "hp-ku mati",
		"i wana die":           "i want to die",
		"pngen mati":           "pengen mati",
		"good coffee":          "good coffee",
	}
	for in, want := range lex {
		if got := lexical(prepare(in)); got != want {
			t.Errorf("lexical(%q) = %q, want %q", in, got, want)
		}
	}
	alt := map[string]string{
		"$u1c1d3":     "suicide",
		"$u1c1d@l":    "suicidal",
		"k1ll my$3lf": "kill myself",
		"pengen m@t1": "pengen mati",
		// A final "!" after a whole word is punctuation (review finding).
		"pengen m@ti! lampu kamar mati": "pengen mati! lampu kamar mati",
		"m@t!":                          "mati",
		"bunuh d!r1":                    "bunuh diri",
		"ki11 myself":                   "kill myself",
		"i w@nt t0 d1e":                 "i want to die",
	}
	for in, want := range alt {
		if got := lexical(prepareAlt(in)); got != want {
			t.Errorf("lexical(prepareAlt(%q)) = %q, want %q", in, got, want)
		}
	}
}
