package safety

import (
	"math/rand"
	"regexp"
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
		// Conjunctions inside a quotation do not split it.
		{`temanku bilang "aku capek tapi aku mau mati" kemarin`,
			[]string{`temanku bilang "aku capek tapi aku mau mati" kemarin`}, []bool{false}},
		// A quotation cannot hold two sentences (see findQuotes), so this
		// pair is not a quotation and the text splits normally.
		{`temanku bilang "aku capek. tapi aku mau mati" kemarin`,
			[]string{`temanku bilang "aku capek`, `aku mau mati" kemarin`}, []bool{false, true}},
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
	// contrastNext marks the clause before a conjunction even when nothing
	// follows the conjunction (finding: a trailing "tapi..." was not hedged).
	for in, want := range map[string][]bool{
		"aku tidak mau bunuh diri, tapi...": {true},
		"aku tidak mau bunuh diri tapi":     {true},
		"aku tidak mau bunuh diri... tapi":  {true},
		"i don't want to die but":           {true},
		"aku capek. ok. tapi":               {false, true},
		"aku capek tapi kuat":               {true, false},
		"tapi aku capek":                    {false},
	} {
		var got []bool
		for _, cl := range splitClauses(in, findQuotes(in)) {
			got = append(got, cl.contrastNext)
		}
		if !slices.Equal(got, want) {
			t.Errorf("contrastNext of %q = %v, want %v", in, got, want)
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
		{`"i want to die." she said`, []string{"i want to die."}},
		{`"aku capek?!" katanya`, []string{"aku capek?!"}},
		// A quotation cannot span a sentence boundary or a newline: the
		// first mark is stray and the closing one opens a new quotation.
		{`temanku bilang "semangat ya. aku mau bunuh diri malam ini. dia bilang "jangan"`, []string{"jangan"}},
		{`my friend said "hang in there. i am going to kill myself. she said "no" ok`, []string{"no"}},
		{"\"satu\ndua\" tiga \"empat\"", []string{" tiga "}},
		{`she said 'hang in there. i will die' ok`, nil},
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
		"kata orang, hidup itu berat", "menurut dia, ini berat", "dalam video itu ada seseorang yang",
	}
	no := []string{
		"aku bilang ke temanku", "i said it", "dia mau mati", "my friend wants to die", "di kelas aku sedih",
		"temanku mau bunuh diri", "you said that", "lalu bilang mau mati",
		// Bare "kata" opens the speaker's own phrases; classes and lectures
		// are not media; "ada" alone is not a person.
		"kata terakhirku: selamat tinggal", "kata terakhir: pamit", "kata hatiku mending mati", "kata aku",
		"menurut aku, ini berat", "di kelas tadi dosen marah", "dalam kuliah tadi dia ngejek",
		"di kelas tadi ada tugas", "di berita tadi ada tugas",
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
		// The speaker's own reporting frame owns the quotation.
		{`i told her "i want to die", she said i was being dramatic`, "want to die", false},
		{`aku cerita "aku mau mati", dia bilang aku lebay`, "mau mati", false},
		{`aku curhat ke ibu "aku pengen bunuh diri", ibu bilang aku kurang iman`, "bunuh diri", false},
		{`di diary aku tulis: "aku mau bunuh diri"`, "bunuh diri", false},
		{`aku bilang dia nulis "aku mau mati"`, "mau mati", true}, // the nearer frame is a third party's
		{`"aku mau mati," ucap saya pelan`, "mau mati", false},
		{`"aku mau mati," kata aku dalam hati`, "mau mati", false},
		{`"aku mau mati," kata hatiku`, "mau mati", false},
		{`"aku mau mati," katanya pelan`, "mau mati", true},
		{`"aku mau mati" kata temanku ke aku`, "mau mati", true},
		{`"i want to die", she said i was being dramatic`, "want to die", false},
		{`"i want to die," she told me last night`, "want to die", true},
		// Direct speech after a colon: one segment, up to a first-person reply.
		{"my mom told me: stop crying, i want to die", "want to die", false},
		{"temenku nanya: kamu kenapa aku jawab aku mau mati", "mau mati", false},
		{"temanku bilang: aku capek, aku mau mati", "mau mati", false},
		// Breakers end subjectless reported content.
		{"nyokap ngomong gitu bikin pengen mati", "pengen mati", false},
		{"pacarku bilang putus rasanya pengen mati", "pengen mati", false},
		{"dia bilang putus dan sekarang pengen bunuh diri", "bunuh diri", false},
		{"temanku bilang dia capek terus dia mau mati", "mau mati", true},
		{"temanku bilang dia terus kepikiran bunuh diri", "bunuh diri", true},
		{"temanku bilang rasanya dia pengen mati", "pengen mati", true},
		{"temanku bilang dia mau mati terus aku nangis", "mau mati", true},
		{`temanku bilang "rasanya mau mati"`, "mau mati", true},
		// -ku words are first person unless they name a third party.
		{"kata dia, hidupku tidak berarti", "tidak berarti", false},
		{"kata dia, temanku mau bunuh diri", "bunuh diri", true},
		{"kata dia, dia baca buku tentang bunuh diri", "bunuh diri", true},
		// Reported content runs to the next frame, not past it.
		{"temanku bilang dia mau mati, dia bilang aku harus nolong", "mau mati", true},
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
	r := &catalogue[ruleIndex["explicit-suicide-phrase"]]
	for in, want := range map[string]state{
		"aku tidak mau bunuh diri, tapi kadang kepikiran": stateHedged,
		"aku tidak mau bunuh diri, aku cuma capek":        stateDenied,
		// A trailing conjunction with nothing after it still hedges.
		"aku tidak mau bunuh diri, tapi...":  stateHedged,
		"aku tidak mau bunuh diri tapi":      stateHedged,
		"aku tidak mau bunuh diri... tapi":   stateHedged,
		"aku tidak mau bunuh diri. ok. tapi": stateDenied,
	} {
		sh := analyse(in)
		i := indexOf(t, sh.text, "bunuh diri")
		if got := sh.dispose(r, span{i, i + len("bunuh diri")}); got != want {
			t.Errorf("%q: got state %d, want %d", in, got, want)
		}
	}
}

func TestFirstPerson(t *testing.T) {
	yes := []string{"aku", "i said", "hidupku", "kata hatiku", "kata terakhirku", "dunia lebih baik tanpaku",
		"hp-ku", "my life", "rumahku", "ku"}
	no := []string{"", "dia mau mati", "temanku", "ibuku bilang", "teman sekelasku", "baca buku", "kuku", "perilaku",
		"she wants to die", "haiku"}
	for _, s := range yes {
		if !firstPerson(s) {
			t.Errorf("firstPerson(%q) = false", s)
		}
	}
	for _, s := range no {
		if firstPerson(s) {
			t.Errorf("firstPerson(%q) = true", s)
		}
	}
}

// TestFrameDispatch checks that frameFind, which only tries the frameRe
// branches that can begin with the first word, finds what trying every
// branch finds, on every corpus segment and on random token soup.
func TestFrameDispatch(t *testing.T) {
	if len(frameOpens) != len(frameRe.alts) {
		t.Fatalf("frameOpens has %d entries for %d branches", len(frameOpens), len(frameRe.alts))
	}
	var inputs []string
	for _, v := range loadVectors(t) {
		s := Normalize(v.Text)
		for _, cl := range splitClauses(s, findQuotes(s)) {
			for _, seg := range cl.segments {
				inputs = append(inputs, s[seg.start:cl.end])
			}
		}
	}
	words := strings.Fields("dalam di in film berita movie the character tokoh tokohnya karakter kata katanya menurut " +
		"according to her dia ia mereka orang seseorang temanku teman aku my friend she he they someone this guy a " +
		"our sister said bilang cerita pernah tadi ke kalau that , : itu yang kutonton tonton watched i")
	rng := rand.New(rand.NewSource(11))
	for n := 0; n < 4000; n++ {
		var b strings.Builder
		for k := 1 + rng.Intn(8); k > 0; k-- {
			b.WriteString(words[rng.Intn(len(words))])
			b.WriteByte(' ')
		}
		inputs = append(inputs, b.String())
	}
	matched := 0
	for _, in := range inputs {
		want := frameRe.FindStringIndex(in)
		if got := frameFind(in); !slices.Equal(got, want) {
			t.Errorf("frameFind(%q) = %v, all branches %v", in, got, want)
		}
		if want != nil {
			matched++
		}
	}
	if matched < 200 {
		t.Errorf("only %d inputs open a frame", matched)
	}
}

// TestTailFrame checks the word-start search of tailFrame against the same
// expression searched the slow way (unanchored at the start, anchored at $)
// on the corpus and on random token soup.
func TestTailFrame(t *testing.T) {
	inputs := []string{"temanku bilang", "kata dia", "my friend texted me this:", "aku cerita", "i told her",
		"aku curhat ke ibu", "di diary aku tulis:", "aku bilang dia nulis", "(dia bilang", "i'm like", "x temanku pernah bilang ke aku kalau"}
	for _, v := range loadVectors(t) {
		s := Normalize(v.Text)
		for i := 0; i <= len(s); i++ {
			if i == len(s) || s[i] == '"' {
				inputs = append(inputs, tailWindow(s, 0, max(0, i-1)))
			}
		}
	}
	words := strings.Fields("aku i dia temanku my friend she said bilang cerita kata katanya tulis nulis ke ibu told her texted " +
		"mom curhat pernah kalau like i'm was , : ( lewat chat sama gue")
	rng := rand.New(rand.NewSource(7))
	for n := 0; n < 3000; n++ {
		var b strings.Builder
		for k := 1 + rng.Intn(8); k > 0; k-- {
			b.WriteString(words[rng.Intn(len(words))])
			b.WriteByte(' ')
		}
		inputs = append(inputs, strings.TrimSpace(b.String()))
	}
	for name, f := range map[string]tailFrame{"reportingTail": reportingTailRe, "ownTail": ownTailRe} {
		ref := regexp.MustCompile(`(?:^|[\s,(])` + strings.TrimPrefix(f.expr, "^"))
		unsorted := regexp.MustCompile(f.expr)
		for _, in := range inputs {
			for i := 0; i <= len(in); i++ {
				if got, want := f.re.MatchString(in[i:]), unsorted.MatchString(in[i:]); got != want {
					t.Errorf("%s: sorted alternations give %v on %q, the expression %v", name, got, in[i:], want)
				}
			}
			want := -1
			if loc := ref.FindStringIndex(in); loc != nil {
				want = loc[0]
				if want > 0 || (in != "" && !isLetter(in[0])) {
					want++ // the reference includes the separator
				}
			}
			if got := f.leftmost(in); got != want && strings.Count(in[max(0, want):], " ") < tailWords {
				t.Errorf("%s.leftmost(%q) = %d, reference %d", name, in, got, want)
			}
		}
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
