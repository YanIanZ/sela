package safety

import (
	"slices"
	"strings"
	"testing"
)

func clauseTexts(s string) (texts []string, contrast []bool) {
	for _, cl := range splitClauses(s, findQuotes(s)) {
		texts = append(texts, s[cl.start:cl.end])
		contrast = append(contrast, cl.afterContrast)
	}
	return texts, contrast
}

func TestSplitClauses(t *testing.T) {
	cases := []struct {
		in       string
		clauses  []string
		contrast []bool
	}{
		{"aku tidak mau bunuh diri tapi aku ingin mati",
			[]string{"aku tidak mau bunuh diri", "aku ingin mati"}, []bool{false, true}},
		{"i don't want to die, but i am tired. ok!",
			[]string{"i don't want to die", "i am tired", "ok"}, []bool{false, true, false}},
		{"aku capek. tapi masih kuat",
			[]string{"aku capek", "masih kuat"}, []bool{false, true}},
		{"satu\ndua; tiga? empat",
			[]string{"satu", "dua", "tiga", "empat"}, []bool{false, false, false, false}},
		{`temanku bilang "aku capek. tapi aku mau mati" kemarin`,
			[]string{`temanku bilang "aku capek. tapi aku mau mati" kemarin`}, []bool{false}},
		{"button but-ton tapinya", []string{"button", "-ton tapinya"}, []bool{false, true}},
		{"", nil, nil},
		{" . , ", nil, nil},
	}
	for _, c := range cases {
		texts, contrast := clauseTexts(c.in)
		if !slices.Equal(texts, c.clauses) || !slices.Equal(contrast, c.contrast) {
			t.Errorf("splitClauses(%q) = %q %v, want %q %v", c.in, texts, contrast, c.clauses, c.contrast)
		}
	}
	cl := splitClauses("a, b, c", nil)
	if len(cl) != 1 || len(cl[0].segments) != 3 {
		t.Errorf("segments of %q: %+v", "a, b, c", cl)
	}
}

func TestFindQuotes(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`dia bilang "aku mau mati" tadi`, []string{"aku mau mati"}},
		{`"satu" dan "dua"`, []string{"satu", "dua"}},
		{`unbalanced "quote`, nil},
		{`she said 'i want to die' yesterday`, []string{"i want to die"}},
		{`i don't think it's fine, isn't it`, nil},
		{`the kids' toys`, nil},
		{`"it's 'fine'" ok`, []string{"it's 'fine'"}},
	}
	for _, c := range cases {
		var got []string
		for _, q := range findQuotes(c.in) {
			got = append(got, c.in[q.start:q.end])
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("findQuotes(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDenialShape(t *testing.T) {
	yes := []string{
		"aku tidak mau bunuh diri", "saya ga ingin mati", "gue gak mau mati", "aku nggak akan bunuh diri.",
		"aku tidak mau bunuh diri kok", "aku tidak berniat menyakiti diriku", "aku tidak kepikiran buat bunuh diri",
		"i don't want to kill myself", "i do not want to die", "i'm not going to kill myself!",
		"i am not gonna hurt myself", "i would never kill myself", "i won't end my life", "i'm not suicidal",
	}
	no := []string{
		"aku tidak mau bunuh diri lagi", "aku tidak mau mati sendirian", "bukan berarti aku mau bunuh diri",
		"aku mau bunuh diri", "kamu tidak mau mati", "i don't want to kill myself but", "i want to die",
		"i don't want to live", "she doesn't want to die",
	}
	for _, s := range yes {
		if !denialRe.MatchString(s) {
			t.Errorf("denialRe should match %q", s)
		}
	}
	for _, s := range no {
		if denialRe.MatchString(s) {
			t.Errorf("denialRe should not match %q", s)
		}
	}
}

func TestFrames(t *testing.T) {
	yes := []string{
		"temanku bilang dia mau mati", "teman aku pernah bilang ke aku kalau dia capek", "kata dia, dia capek",
		"katanya mau pergi", "dalam film itu tokoh berkata", "di berita tadi ada mahasiswa", "tokohnya sedih",
		"my friend said she is sad", "my best friend told me that", "according to her it is fine",
		"in the movie, the character dies", "in today's news a student", "the main character dies",
		"kemarin temanku cerita", "yesterday my sister texted me", "she keeps saying it",
	}
	no := []string{
		"aku bilang ke temanku", "i said it", "dia mau mati", "my friend wants to die", "di kelas aku sedih",
		"temanku mau bunuh diri", "you said that", "lalu bilang mau mati",
	}
	for _, s := range yes {
		if _, ok := matchFrame(s, 0, len(s)); !ok {
			t.Errorf("frame expected at the start of %q", s)
		}
	}
	for _, s := range no {
		if end, ok := matchFrame(s, 0, len(s)); ok {
			t.Errorf("no frame expected at the start of %q (frame %q)", s, s[:end])
		}
	}
	lead := "jadi, kemarin temanku bilang dia capek"
	if end, ok := matchFrame(lead, 0, len(lead)); !ok || strings.TrimSpace(lead[:end]) != "jadi, kemarin temanku bilang" {
		t.Errorf("lead-in words: end %d ok %v", end, ok)
	}
}

func TestAttribution(t *testing.T) {
	cases := []struct {
		in, phrase string
		attributed bool
	}{
		{"temanku bilang dia mau bunuh diri", "bunuh diri", true},
		{"temanku bilang aku mau bunuh diri", "bunuh diri", false},
		{`temanku bilang "aku mau mati"`, "mau mati", true},
		{`aku bilang "aku mau mati"`, "mau mati", false},
		{`"aku mau mati" kata temanku`, "mau mati", true},
		{`"aku mau mati"`, "mau mati", false},
		{"temanku bilang: aku mau mati", "mau mati", true},
		{"kata dia, dia pengen mati", "pengen mati", true},
		{"temanku bilang, mau bunuh diri", "bunuh diri", true},
		{"aku khawatir, temanku bilang dia mau bunuh diri", "bunuh diri", true},
		{"tokohnya bunuh diri, jadi pengen mati", "pengen mati", false},
		{"dalam film itu tokoh bunuh diri aku mau bunuh diri", "aku mau bunuh diri", false},
	}
	for _, c := range cases {
		sh := analyse(c.in)
		i := indexOf(t, c.in, c.phrase)
		m := span{i, i + len(c.phrase)}
		lo, hi := sh.overlapping(m)
		got := false
		for k := lo; k < hi; k++ {
			for _, s := range sh.clause(k).scopes {
				got = got || s.contains(m)
			}
		}
		if got != c.attributed {
			t.Errorf("%q in %q: attributed = %v, want %v", c.phrase, c.in, got, c.attributed)
		}
	}
}

func TestAdoption(t *testing.T) {
	yes := []string{
		"temanku bilang dia mau mati. aku juga.", "my friend said she wants to die. me too",
		"my friend said she wants to die. same here", "she wants to die. i relate",
		"dia bilang mau mati, sama aku juga", "my friend said she wants to die too",
		"the character died. relatable", "she said she wants to die and i said same",
	}
	no := []string{
		"temanku bilang dia mau mati. aku khawatir.", "my friend said she wants to die. what do i do",
		"temanku bilang dia mau mati. aku juga nonton beritanya.", "the same thing happened",
	}
	for _, s := range yes {
		if !analyse(s).adopted() {
			t.Errorf("adoption expected in %q", s)
		}
	}
	for _, s := range no {
		if analyse(s).adopted() {
			t.Errorf("no adoption expected in %q", s)
		}
	}
}

func TestHedgedDenial(t *testing.T) {
	sh := analyse("aku tidak mau bunuh diri, tapi kadang kepikiran")
	i := indexOf(t, sh.text, "bunuh diri")
	if got := sh.dispose(&catalogue[ruleIndex["explicit-suicide-phrase"]], span{i, i + len("bunuh diri")}); got != stateHedged {
		t.Errorf("hedged denial: got state %d", got)
	}
	sh = analyse("aku tidak mau bunuh diri, aku cuma capek")
	i = indexOf(t, sh.text, "bunuh diri")
	if got := sh.dispose(&catalogue[ruleIndex["explicit-suicide-phrase"]], span{i, i + len("bunuh diri")}); got != stateDenied {
		t.Errorf("plain denial: got state %d", got)
	}
}

func indexOf(t *testing.T, s, sub string) int {
	t.Helper()
	i := strings.Index(s, sub)
	if i < 0 {
		t.Fatalf("%q not in %q", sub, s)
	}
	return i
}
