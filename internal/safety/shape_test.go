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
		// A closed report ends the content, past manner words and chat
		// spellings of "gitu" (review findings).
		{`mama bilang sambil marah "sabar ya" pengen bunuh diri`, "bunuh diri", false},
		{`mama bilang lagi "sabar ya" pengen bunuh diri`, "bunuh diri", false},
		{"nyokap ngomong gt pengen mati aja", "pengen mati", false},
		{"nyokap ngomong gitu2 pengen mati aja", "pengen mati", false},
		{"nyokap ngomong itu pengen mati aja", "pengen mati", false},
		// Colon speech ends where the speaker's own words start.
		{"mama bilang: makan dulu rasanya pengen mati", "pengen mati", false},
		{"mama bilang: makan dulu pengen mati aja", "pengen mati", false},
		{"my mom told me: stop crying wanna die", "wanna die", false},
		{"temanku bilang: rasanya pengen mati", "pengen mati", true},
		{"temanku bilang: aku capek terus pengen mati", "pengen mati", true},
		{"temanku bilang: capek pengen mati", "pengen mati", true},
		// Lyrics frame only before a colon or a quotation.
		{"lirik lagunya sedih banget pengen mati", "pengen mati", false},
		{"lirik lagunya: aku ingin mati", "ingin mati", true},
		{`the lyrics say "i want to die"`, "want to die", true},
		// A media report ends at the speaker's subjectless reaction.
		{"aku baca berita ada mahasiswa bunuh diri pengen nyusul", "bunuh diri", true},
		{"aku baca berita ada mahasiswa mau bunuh diri", "bunuh diri", true},
		{"di film itu tokohnya bunuh diri karena pengen mati", "pengen mati", true},
		{"di film itu tokohnya bunuh diri pengen mati juga", "pengen mati", false},
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
		// Review findings: more adoption and imitation forms.
		"my friend said she wants to die. me to", "my friend said she wants to die. omg same",
		"my friend said she wants to die. same tho", "my friend said she wants to die. literally me",
		"my friend said she wants to die. that's me", "temanku bilang dia mau mati. aku sama",
		"temanku bilang dia mau mati. sama kayak aku", "aku baca berita ada mahasiswa bunuh diri, pengen nyusul",
		"temanku bilang dia mau bunuh diri, aku mau ikut", "aku liat berita ada orang loncat, aku pengen kayak dia",
		"aku baca berita ada anak bunuh diri, kok aku iri ya", "kepikiran buat ngelakuin hal yang sama",
		"i watched a video of a guy jumping off a bridge, i want to try it",
		// Round-2 review findings: trailing laughter, emoticons and adverbs,
		// "aku juga/pun" before an unlisted word, references back to the
		// act, elliptical wishes, and "so do i" before anything.
		"temanku bilang dia mau bunuh diri. aku juga wkwk", "temanku bilang dia mau bunuh diri. gw jg :(",
		"temanku bilang dia mau bunuh diri. aku pun hiks", "temanku bilang dia mau bunuh diri. saya juga huhu",
		"temanku bilang dia mau bunuh diri. aku sama hehe", "temanku bilang dia mau bunuh diri. aku juga sebenernya",
		"temanku bilang dia mau bunuh diri. kalau aku sih juga", "temanku bilang dia mau bunuh diri. aku juga dari dulu",
		"temanku bilang dia mau bunuh diri, aku juga sempat", "temanku bilang dia mau bunuh diri, aku pengen juga",
		"my friend said she wants to die. so do i haha", "my friend said she wants to die. so do i right now",
		"my friend said she wants to die. so am i :(", "my friend said she wants to die. i'm the same :(",
		"my friend said she wants to die. i want to too", "my friend said she wants to die. i kinda want to too",
		"my friend said she wants to die. honestly i want to", "temanku bilang dia mau bunuh diri. aku pengen banget",
		"aku baca berita ada mahasiswa bunuh diri, jadi pengen ngelakuin juga",
		"aku baca berita ada mahasiswa bunuh diri, kepikiran buat ngelakuin hal yg sama",
		"aku baca berita ada mahasiswa bunuh diri kepikiran buat ngelakuin hal yang sama",
		"aku nonton video orang gantung diri. pengen ngelakuin itu juga",
		"i read about a student who killed himself, i'm thinking of doing the same",
		// The doubled variant of "meee tooo".
		"my friend said she wants to die. mee too shaving",
		// Round-3 review findings: chat pronouns, more short forms, comma
		// forms, attached envy, a closed short form, and "aku juga" before
		// an aspect word and a non-knowing continuation.
		"temanku bilang dia mau bunuh diri. aq juga", "temanku bilang dia mau bunuh diri. gw banget",
		"temanku bilang dia mau bunuh diri. sama aja aku", "my friend said she wants to die. metoo",
		"my friend said she wants to die. big mood", "my friend said she wants to die. i do, too",
		"my friend said she wants to die. i've thought about it too", "my friend said she wants to die. so am i lol idk",
		"aku baca berita ada mahasiswa bunuh diri, kenapa bukan aku", "i read about a student who killed himself. i wish that was me",
		"temanku bilang dia mau bunuh diri. aku juga udah nggak sanggup", "temanku bilang dia mau bunuh diri. aku juga punya rencana",
	}
	no := []string{
		"temanku bilang dia mau mati. aku khawatir.", "my friend said she wants to die. what do i do",
		"temanku bilang dia mau mati. aku juga nonton beritanya.", "the same thing happened",
		"temanku bilang dia mau mati. besok aku mau ikut nemenin dia", "temanku bilang dia mau mati. aku sama dia sahabatan",
		"she gave it to me to keep",
		// An ordinary continuation, a comparison, a negated wish or a wish
		// with an object is not adoption.
		"temanku bilang dia mau mati, aku juga panik", "temanku bilang dia mau mati, aku pun langsung nelpon dia",
		"temanku bilang dia mau mati, aku juga nggak tau harus ngapain", "my friend said she wants to die. i'm the same age",
		"my friend said she wants to die. every day it's the same", "temanku bilang dia mau mati. aku nggak pengen",
		"my friend said she wants to die. i don't want to", "temanku bilang dia mau mati, aku pengen bantu dia",
		"aku baca berita ada mahasiswa bunuh diri, pengen nangis", "she asked me to come",
		"my friend said she wants to die. this is me at the beach", "my friend said she wants to die. i'm next to her now",
		"my friend said she wants to die. i feel that she needs help", "temanku bilang dia mau bunuh diri. aku juga udah tahu",
		"temanku bilang dia mau bunuh diri. aku juga nggak nyangka",
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
		if got := sh.dispose(r, span{i, i + len("bunuh diri")}, false); got != want {
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

// TestAdoptionCrossProduct checks every combination of a reported wish, a
// short Indonesian adoption form, an aspect word and a continuation, and of
// a media or lyric frame with an attached wish or envy (round-3 review
// findings: 144 and 132 inputs; vectors.json holds a sample of each).
func TestAdoptionCrossProduct(t *testing.T) {
	check := func(s string) {
		t.Helper()
		if a := Assess(s); a.Level != LevelCrisis {
			t.Errorf("Assess(%q) = %s %v, want crisis", s, a.Level, a.RuleIDs)
		}
	}
	for _, report := range []string{"temanku bilang dia mau bunuh diri. ", "my friend said she wants to die. "} {
		for _, form := range []string{"aku juga", "gw jg", "aku pun", "saya juga"} {
			for _, aspect := range []string{"udah", "lagi", "kadang"} {
				for _, tail := range []string{"nggak sanggup", "nggak kuat", "ga kuat lagi", "nggak pengen hidup",
					"cuma pengen pergi", "ada pikiran itu"} {
					check(report + form + " " + aspect + " " + tail)
				}
			}
		}
	}
	frames := []string{"aku baca berita ada mahasiswa bunuh diri", "aku nonton video orang gantung diri",
		"I watched a video of a guy jumping off a bridge", "I read a news article about a student who killed himself",
		"Lirik lagunya: aku ingin mati", `the lyrics say "I want to die"`}
	forms := []string{"I wish that was me", "andai itu aku", "semoga aku berikutnya", "I'm next", "aku selanjutnya",
		"I wish I had the guts", "pengen punya keberanian kayak dia", "kenapa bukan aku", "why not me",
		"itu yang aku mau", "that's what I want"}
	for _, f := range frames {
		for _, form := range forms {
			for _, join := range []string{", ", ". "} {
				check(f + join + form)
			}
		}
	}
}

func TestJustQualifier(t *testing.T) {
	for _, c := range []struct {
		rest string
		want int
	}{
		{"tired", qualNone}, {"tired of living", qualNone}, {"need someone to talk to", qualNone},
		{"yet", qualVoid}, {"kidding", qualVoid}, {"not today", qualVoid}, {"for now", qualVoid},
		{"untuk sekarang", qualVoid}, {"because of my mom", qualVoid}, {"karena ibu", qualVoid},
		{"belum berani", qualVoid}, {"not brave enough", qualVoid}, {"bercanda", qualVoid},
		{"kadang kepikiran", qualHedge}, {"scared", qualHedge}, {"not doing great", qualHedge},
	} {
		if got := justQualifier(c.rest); got != c.want {
			t.Errorf("justQualifier(%q) = %d, want %d", c.rest, got, c.want)
		}
	}
	for _, s := range []string{"tired", "capek banget", "so tired of everything", "having a rough day"} {
		if !tiredRe.MatchString(" " + s) {
			t.Errorf("tiredRe does not match %q", s)
		}
	}
	for _, s := range []string{"yet", "kidding", "kepikiran aja", "not today", "want to die", "pengen nggak ada"} {
		if tiredRe.MatchString(" " + s) {
			t.Errorf("tiredRe matches %q", s)
		}
	}
	// Only a wish for a listed benign thing ends a denial without a comma
	// (review finding).
	for _, c := range []struct {
		rest string
		want bool
	}{
		{" tired", true}, {" need a break", true}, {" pengen istirahat", true}, {" want someone to talk to", true},
		{" want to not exist", false}, {" want to be gone", false}, {" pengen nggak ada", false},
		{" mau udahan aja", false}, {" want to be left", false}, {" need a break forever", false},
		{" tired and want to disappear", false},
	} {
		if got := endsDenial(c.rest); got != c.want {
			t.Errorf("endsDenial(%q) = %v, want %v", c.rest, got, c.want)
		}
	}
	for _, c := range []struct {
		rest string
		want int
	}{
		{"want to be gone", qualVoid}, {"pengen nggak ada", qualVoid}, {"need everything to stop", qualVoid},
		{"want the pain to stop", qualHedge}, {"need a break", qualNone}, {"waiting for the right time", qualVoid},
		{"udah siapin obat", qualVoid}, {"too scared to do it", qualVoid}, {"planning", qualVoid},
	} {
		if got := continuationKind(c.rest); got != c.want {
			t.Errorf("continuationKind(%q) = %d, want %d", c.rest, got, c.want)
		}
	}
	for _, c := range []struct {
		s    string
		want bool
	}{
		{"jk", true}, {"not really", true}, {"that's a lie", true}, {"boong deng", true}, {"gak deng", true},
		{"yet", true}, {"yet i feel fine", false}, {"i feel fine", false}, {"really", false},
	} {
		if got := retraction(stripFillers(c.s)); got != c.want {
			t.Errorf("retraction(%q) = %v, want %v", c.s, got, c.want)
		}
	}
	if got := stripFillers("haha lol just kidding"); got != "just kidding" {
		t.Errorf("stripFillers = %q", got)
	}
}
