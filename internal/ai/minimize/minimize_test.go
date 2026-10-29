package minimize

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRedactContactDetails(t *testing.T) {
	cases := []struct {
		in   string
		gone []string
	}{
		{"Email aku: raka.putra@example.co.id ya", []string{"raka.putra@example.co.id"}},
		{"WA 0812-3456-7890 atau +62 812 3456 7890", []string{"0812-3456-7890", "+62 812 3456 7890"}},
		{"Call me at +1 (415) 555-0100 tonight", []string{"415", "555-0100"}},
		{"Kantor (021) 555 1234 tutup", []string{"555 1234"}},
		{"Lihat https://contoh.id/halaman?x=1, terus www.blog.com.", []string{"contoh.id", "www.blog.com"}},
		{"Akun aku @raka_putra di IG", []string{"@raka_putra"}},
		{"NIK 3174012345678901 hilang", []string{"3174012345678901"}},
		{"Kartu 4111 1111 1111 1111 ditolak", []string{"4111 1111 1111 1111"}},
		{"blog aku rakaputra.com", []string{"rakaputra.com"}},
	}
	for _, tc := range cases {
		s := NewSession(nil)
		out := s.Redact(tc.in)
		for _, g := range tc.gone {
			if strings.Contains(out, g) {
				t.Errorf("Redact(%q) = %q still contains %q", tc.in, out, g)
			}
		}
		if !strings.Contains(out, "[KONTAK-1]") {
			t.Errorf("Redact(%q) = %q has no placeholder", tc.in, out)
		}
	}
}

func TestRedactKeepsOrdinaryNumbers(t *testing.T) {
	for _, in := range []string{
		"Sudah dua bulan, sejak 12 Agustus 2026.",
		"Aku tidur cuma 3 jam.",
		"Biayanya Rp 12.000.000 per semester.",
		"Tanggal 2026-10-26 aku ujian.",
		"Skor aku 85 dari 100.",
		"Email itu penting buat kerja.",
	} {
		s := NewSession(nil)
		if out := s.Redact(in); out != in {
			t.Errorf("Redact(%q) = %q, want unchanged", in, out)
		}
	}
}

func TestRedactPrivateNamesStableAcrossSpans(t *testing.T) {
	s := NewSession([]string{"Budi", "Budi Santoso", "Sari"})
	a := s.Redact("Budi Santoso bilang aku lebay. budi memang begitu.")
	b := s.Redact("Aku cerita ke Sari, bukan ke BUDI. Budiman temanku.")
	if strings.Contains(strings.ToLower(a+b), "budi ") || strings.Contains(a+b, "Sari") {
		t.Fatalf("names leaked: %q / %q", a, b)
	}
	if !strings.Contains(b, "Budiman") {
		t.Fatalf("matched inside another word: %q", b)
	}
	if !strings.HasPrefix(a, "[NAMA-1]") || !strings.Contains(a, "[NAMA-2] memang") || !strings.Contains(b, "ke [NAMA-2]") {
		t.Fatalf("placeholders not stable: %q / %q", a, b)
	}
	r := s.Restore()
	if r["[NAMA-1]"] != "Budi Santoso" || r["[NAMA-3]"] != "Sari" {
		t.Fatalf("restore = %v", r)
	}
	// Re-redacting redacted text is a no-op.
	if again := s.Redact(a); again != a {
		t.Fatalf("second pass changed text: %q", again)
	}
	if s.Counts()[KindName] != 3 {
		t.Fatalf("counts = %v", s.Counts())
	}
}

func TestBuildMinimisesAndMaps(t *testing.T) {
	const key = "conv-7f3a/entry-91"
	s := NewSession([]string{"Raka"})
	p, err := Build([]Selection{
		{Key: key, Role: RoleUser, Text: "Raka bilang hubungi raka@example.com"},
		{Key: "entry-92", Role: RoleEarlierReply, Text: "Terima kasih sudah cerita."},
	}, Limits{MaxSpans: 4, MaxSpanRunes: 200, MaxTotalRunes: 400}, s)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(p.Spans)
	for _, forbidden := range []string{key, "entry-92", "Raka", "raka@example.com"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("payload contains %q: %s", forbidden, raw)
		}
	}
	if p.Spans[0].ID != "s1" || p.Spans[1].ID != "s2" || p.Spans[1].Role != RoleEarlierReply {
		t.Fatalf("spans = %+v", p.Spans)
	}
	if k, ok := p.Key("s1"); !ok || k != key {
		t.Fatalf("key = %q, %v", k, ok)
	}
	if txt, _ := p.Text("s1"); txt != "[NAMA-1] bilang hubungi [KONTAK-1]" {
		t.Fatalf("text = %q", txt)
	}
}

func TestBuildLimits(t *testing.T) {
	s := NewSession(nil)
	if _, err := Build(nil, Limits{}, s); !errors.Is(err, ErrNoSpans) {
		t.Fatalf("no spans: %v", err)
	}
	if _, err := Build([]Selection{{Role: RoleUser, Text: "  "}}, Limits{}, s); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty: %v", err)
	}
	if _, err := Build([]Selection{{Role: "assistant", Text: "x"}}, Limits{}, s); !errors.Is(err, ErrInvalid) {
		t.Fatalf("role: %v", err)
	}
	long := strings.Repeat("a", 101)
	if _, err := Build([]Selection{{Role: RoleUser, Text: long}}, Limits{MaxSpanRunes: 100}, s); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("span cap: %v", err)
	}
	three := []Selection{{Key: "1", Role: RoleUser, Text: "satu"}, {Key: "2", Role: RoleUser, Text: "dua"}, {Key: "3", Role: RoleUser, Text: "tiga"}}
	if _, err := Build(three, Limits{MaxSpans: 2}, s); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("count cap: %v", err)
	}
	p, err := Build(three, Limits{MaxSpans: 2, DropOldest: true}, s)
	if err != nil || len(p.Spans) != 2 || p.Dropped != 1 || p.Spans[0].Text != "dua" {
		t.Fatalf("drop oldest: %+v, %v", p, err)
	}
	p, err = Build(three, Limits{MaxTotalRunes: 7, DropOldest: true}, s)
	if err != nil || len(p.Spans) != 2 || p.Spans[1].Text != "tiga" {
		t.Fatalf("drop oldest by size: %+v, %v", p, err)
	}
	if _, err := Build(three[:1], Limits{MaxTotalRunes: 2, DropOldest: true}, s); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("last span above total cap: %v", err)
	}
}

func TestRoles(t *testing.T) {
	if !RoleUser.UserAuthored() || !RoleItem.UserAuthored() || RoleEarlierReply.UserAuthored() || RoleSource.UserAuthored() {
		t.Fatal("user-authored roles")
	}
}

// TestRedactGroupedNIK: a NIK written with spaces, hyphens or dots between
// its groups is redacted as a whole, with no region code left behind
// (ADR-0003 DM-2 names NIK).
func TestRedactGroupedNIK(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"NIK aku 3174 1234 5678 9012", "NIK aku [KONTAK-1]"},
		{"NIK: 3273 2504 9001 0003", "NIK: [KONTAK-1]"},
		{"NIK aku 3174-5612-3456-7891", "NIK aku [KONTAK-1]"},
		{"NIK aku 3174.0123.4567.8901", "NIK aku [KONTAK-1]"},
		{"no KK 3273 0101 2345 6789", "no KK [KONTAK-1]"},
		{"3174 0123 4567 8901 itu NIK-ku", "[KONTAK-1] itu NIK-ku"},
		{"partly grouped 3174 012345678901", "partly grouped [KONTAK-1]"},
		{"KTP 317456 123456 7891", "KTP [KONTAK-1]"},
		{"KTP 32.73.25.040990.0003", "KTP [KONTAK-1]"},
		{"NIK aku 3174–5612–3456–7891", "NIK aku [KONTAK-1]"},
	} {
		s := NewSession(nil)
		if got := s.Redact(tc.in); got != tc.want {
			t.Errorf("Redact(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if s.Counts()[KindNIK] != 1 {
			t.Errorf("%q: counts %v", tc.in, s.Counts())
		}
	}
	// 17 digits or a 4-4-4-4 group glued to more digits is not a NIK.
	s := NewSession(nil)
	if got := s.Redact("kode 31741234567890123"); strings.Contains(got, "KONTAK") && s.Counts()[KindNIK] > 0 {
		t.Errorf("17 digits redacted as NIK: %q", got)
	}
}

// TestRedactAddressesAndSelfIntroducedNames: the v1 address_hint and
// person_name_hint patterns, and handles named by a platform cue.
func TestRedactAddressesAndSelfIntroducedNames(t *testing.T) {
	for _, tc := range []struct {
		in   string
		gone []string
		keep []string
	}{
		{"Alamat aku Jl. Melati No. 12, RT 03/RW 05, Kel. Cempaka Putih", []string{"Melati", "12", "03", "05", "Cempaka", "Putih"}, []string{"Alamat aku"}},
		{"Rumahku di Jalan Merdeka Raya No. 5 dekat pasar", []string{"Merdeka", "No. 5"}, []string{"dekat pasar"}},
		{"Aku tinggal di Kec. Sukajadi, Kab. Bandung", []string{"Sukajadi", "Bandung"}, []string{"Aku tinggal di"}},
		{"Namaku Raka Putra, aku dari Bandung", []string{"Raka", "Putra"}, []string{"aku dari Bandung"}},
		{"Panggil aku Sari ya", []string{"Sari"}, []string{"Panggil aku", "ya"}},
		{"Aku Dimas dari Surabaya", []string{"Dimas"}, []string{"dari Surabaya"}},
		{"Hi, my name is Jordan Lee.", []string{"Jordan", "Lee"}, []string{"my name is"}},
		{"ig aku raka.putra ya, follow", []string{"raka.putra"}, []string{"ig aku", "follow"}},
		{"line: rakaputra", []string{"rakaputra"}, []string{"line"}},
		{"tiktoknya raka_99", []string{"raka_99"}, []string{"tiktoknya"}},
	} {
		s := NewSession(nil)
		got := s.Redact(tc.in)
		for _, g := range tc.gone {
			if strings.Contains(got, g) {
				t.Errorf("Redact(%q) = %q still contains %q", tc.in, got, g)
			}
		}
		for _, k := range tc.keep {
			if !strings.Contains(got, k) {
				t.Errorf("Redact(%q) = %q lost %q", tc.in, got, k)
			}
		}
	}
	// Ordinary words after a cue are not handles; lowercase words after
	// "namaku" are not names (v1 requires a capitalised name).
	for _, in := range []string{"Aku suka line art.", "Minum 3 x sehari kata ibu.", "ig aku rame banget", "Panggil aku kalau butuh."} {
		s := NewSession(nil)
		if got := s.Redact(in); got != in {
			t.Errorf("Redact(%q) = %q, want unchanged", in, got)
		}
	}
	s := NewSession(nil)
	out := s.Redact("Namaku Raka. Raka capek.")
	if !strings.HasPrefix(out, "Namaku [NAMA-1].") || s.Restore()["[NAMA-1]"] != "Raka" {
		t.Fatalf("name hint: %q %v", out, s.Restore())
	}
}

// TestRedactNormalisedForms: input redaction recognises what the output
// guard recognises: fullwidth, accented, zero-width and homoglyph
// spellings of a private name, multi-word names split by a no-break
// space, several spaces or a line break, Unicode dashes in a phone number
// and a fullwidth "@" in an e-mail address. The original bytes are
// replaced and the same person keeps one placeholder.
func TestRedactNormalisedForms(t *testing.T) {
	for _, tc := range []struct {
		names []string
		in    string
		want  string
	}{
		{[]string{"Dewi"}, "Ｄｅｗｉ marah", "[NAMA-1] marah"},
		{[]string{"Dewi"}, "Déwi marah", "[NAMA-1] marah"},
		{[]string{"Dewi"}, "De​wi marah", "[NAMA-1] marah"},
		{[]string{"Dewi"}, "Dеwi marah", "[NAMA-1] marah"}, // Cyrillic е
		{[]string{"Zoë"}, "Zoe marah", "[NAMA-1] marah"},
		{[]string{"Budi Santoso"}, "Budi Santoso bilang", "[NAMA-1] bilang"},
		{[]string{"Budi Santoso"}, "Budi  Santoso bilang", "[NAMA-1] bilang"},
		{[]string{"Budi Santoso"}, "Budi\nSantoso bilang", "[NAMA-1] bilang"},
		{nil, "WA aku 0812–3456–7890 ya", "WA aku [KONTAK-1] ya"},
		{nil, "WA aku 0812—3456—7890 ya", "WA aku [KONTAK-1] ya"},
		{nil, "email aku raka.putra＠gmail.com", "email aku [KONTAK-1]"},
		{nil, "nomorku ０８１２３４５６７８９０", "nomorku [KONTAK-1]"},
	} {
		s := NewSession(tc.names)
		if got := s.Redact(tc.in); got != tc.want {
			t.Errorf("Redact(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	s := NewSession([]string{"Dewi"})
	a := s.Redact("Dewi marah.")
	b := s.Redact("Ｄｅｗｉ pergi.")
	if a != "[NAMA-1] marah." || b != "[NAMA-1] pergi." || s.Restore()["[NAMA-1]"] != "Dewi" {
		t.Fatalf("one person, two placeholders: %q %q %v", a, b, s.Restore())
	}
	if again := s.Redact(b); again != b {
		t.Fatalf("second pass changed text: %q", again)
	}
}

// TestRedactPrivateNameAffixes: a private name with a possessive, an
// enclitic, the reduplication digit or repeated letters is redacted, and
// the suffix stays beside the placeholder.
func TestRedactPrivateNameAffixes(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"I told Raka's mom everything.", "I told [NAMA-1]'s mom everything."},
		{"Raka’s sister knows.", "[NAMA-1]’s sister knows."},
		{"Rakanya nggak bales chat aku.", "[NAMA-1]nya nggak bales chat aku."},
		{"aku kangen Dewiku.", "aku kangen [NAMA-2]ku."},
		{"Dewiii kenapa sih", "[NAMA-2] kenapa sih"},
		{"Dewi2 itu temenku", "[NAMA-2]2 itu temenku"},
		{"Rakasa itu raksasa.", "Rakasa itu raksasa."},
	} {
		s := NewSession([]string{"Raka", "Dewi"})
		s.Redact("Raka dan Dewi.") // numbers the two people
		if got := s.Redact(tc.in); got != tc.want {
			t.Errorf("Redact(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestBuildLearnsSelfIntroducedNames: Build reads every span for a
// self-introduction first, so every mention of the name and of its given
// name, in every span (also one before the introduction), gets the one
// placeholder, which restores to the full name; Names lists them for the
// output guard. An address term alone is never learned as a name.
func TestBuildLearnsSelfIntroducedNames(t *testing.T) {
	s := NewSession([]string{"Sari"})
	p, err := Build([]Selection{
		{Key: "a", Role: RoleUser, Text: "Raka capek terus."},
		{Key: "b", Role: RoleUser, Text: "Namaku Raka Putra. Jujur Raka Putra itu capek terus."},
		{Key: "c", Role: RoleUser, Text: "Teman-teman manggil aku Raka, tapi Raka nggak suka."},
		{Key: "d", Role: RoleUser, Text: "Aku dipanggil Bima. Panggil aku Kak Lintang."},
		{Key: "e", Role: RoleUser, Text: "My name is Dewi Lestari and Dewi never sleeps. Sari tahu."},
		{Key: "f", Role: RoleUser, Text: "Panggil aku Kak aja, kak."},
	}, Limits{}, s)
	if err != nil {
		t.Fatal(err)
	}
	// Placeholders are numbered in order of appearance.
	want := []string{
		"[NAMA-1] capek terus.",
		"Namaku [NAMA-1]. Jujur [NAMA-1] itu capek terus.",
		"Teman-teman manggil aku [NAMA-1], tapi [NAMA-1] nggak suka.",
		"Aku dipanggil [NAMA-2]. Panggil aku [NAMA-3].",
		"My name is [NAMA-4] and [NAMA-4] never sleeps. [NAMA-5] tahu.",
		"Panggil aku Kak aja, kak.",
	}
	for i, sp := range p.Spans {
		if sp.Text != want[i] {
			t.Errorf("span %d = %q, want %q", i+1, sp.Text, want[i])
		}
	}
	r := s.Restore()
	if r["[NAMA-1]"] != "Raka Putra" || r["[NAMA-2]"] != "Bima" || r["[NAMA-3]"] != "Kak Lintang" || r["[NAMA-4]"] != "Dewi Lestari" ||
		r["[NAMA-5]"] != "Sari" {
		t.Fatalf("restore %v", r)
	}
	names := strings.Join(s.Names(), ",")
	for _, n := range []string{"Sari", "Raka Putra", "Raka", "Bima", "Kak Lintang", "Lintang", "Dewi Lestari", "Dewi"} {
		if !strings.Contains(","+names+",", ","+n+",") {
			t.Errorf("Names() = %s lacks %q", names, n)
		}
	}
	if strings.Contains(","+strings.ToLower(names)+",", ",kak,") {
		t.Errorf("an address term was learned as a name: %s", names)
	}
}

// TestKeptPlaceholdersReserveTheirNumbers: a placeholder already in a text
// is kept, and new values are numbered after it in every span order, so
// two people or contacts never share one placeholder.
func TestKeptPlaceholdersReserveTheirNumbers(t *testing.T) {
	for _, order := range [][2]string{
		{"Kedengarannya [NAMA-1] kesal, cek [KONTAK-2].", "Terus Dewi ikut, emailnya d@example.com."},
		{"Terus Dewi ikut, emailnya d@example.com.", "Kedengarannya [NAMA-1] kesal, cek [KONTAK-2]."},
	} {
		s := NewSession([]string{"Dewi"})
		p, err := Build([]Selection{{Key: "a", Role: RoleEarlierReply, Text: order[0]}, {Key: "b", Role: RoleUser, Text: order[1]}}, Limits{}, s)
		if err != nil {
			t.Fatal(err)
		}
		all := p.Spans[0].Text + " " + p.Spans[1].Text
		if !strings.Contains(all, "Terus [NAMA-2] ikut, emailnya [KONTAK-3].") || !strings.Contains(all, "Kedengarannya [NAMA-1] kesal, cek [KONTAK-2].") {
			t.Fatalf("sent %q", all)
		}
		if r := s.Restore(); len(r) != 2 || r["[NAMA-2]"] != "Dewi" || r["[KONTAK-3]"] != "d@example.com" {
			t.Fatalf("restore %v", r)
		}
	}
	// Redact alone (no Build) also reserves the numbers it keeps.
	s := NewSession([]string{"Dewi"})
	if got := s.Redact("[NAMA-4] dan Dewi"); got != "[NAMA-4] dan [NAMA-5]" {
		t.Fatalf("Redact = %q", got)
	}
}

// TestRedactContactLinksAndAddresses: any host with a path is a URL
// whatever its top-level domain, common link hosts are URLs without a
// path, and a street cue with a house number is an address in any case;
// ordinary words, scores and abbreviations stay.
func TestRedactContactLinksAndAddresses(t *testing.T) {
	for _, tc := range []struct{ in, gone string }{
		{"cek linktr.ee/rakaputra ya", "rakaputra"},
		{"donasi di saweria.co/rakaputra", "rakaputra"},
		{"join discord.gg/abc123x", "abc123x"},
		{"live di twitch.tv/rakalive", "rakalive"},
		{"video youtu.be/xYz123", "xYz123"},
		{"profil di contoh.studio/raka", "raka"},
		{"halaman saweria.co aja", "saweria"},
		{"rumahku di jalan melati no 12 bekasi", "melati"},
		{"rumahku di JALAN MELATI NO. 12", "MELATI"},
		{"rumahku di jl melati no 12", "melati"},
		{"kosku di Gg. Mawar 3 No. 7", "Mawar"},
		{"kosku di gang mawar no 7", "mawar"},
		{"aku di jalan Sudirman", "Sudirman"},
		{"tinggal di desa Sukamaju", "Sukamaju"},
	} {
		if got := NewSession(nil).Redact(tc.in); strings.Contains(got, tc.gone) {
			t.Errorf("Redact(%q) = %q still contains %q", tc.in, got, tc.gone)
		}
	}
	for _, in := range []string{"Aku jalan kaki ke kampus.", "Jalan-jalan ke pantai yuk.", "Nilaiku 7.5/10 minggu ini.",
		"Kecepatan 5 kg.m/s itu kecil.", "Ada jalan keluar kok.", "gg banget sih.", "Jam 7.30/8.00 aku kuliah."} {
		if got := NewSession(nil).Redact(in); got != in {
			t.Errorf("Redact(%q) = %q, want unchanged", in, got)
		}
	}
}

// TestBuildRoleLimits: a per-role limit counts the whole selection after
// redaction and fails with ErrRoleLimit; nothing is dropped silently.
func TestBuildRoleLimits(t *testing.T) {
	sel := func(n, runes int) []Selection {
		var out []Selection
		for i := 0; i < n; i++ {
			out = append(out, Selection{Key: "m", Role: RoleMemory, Text: strings.Repeat("a", runes)})
		}
		return append(out, Selection{Key: "u", Role: RoleUser, Text: "Aku capek."})
	}
	lim := Limits{PerRole: map[Role]RoleLimit{RoleMemory: {MaxSpans: 5, MaxRunes: 600}}}
	if _, err := Build(sel(5, 120), lim, NewSession(nil)); err != nil {
		t.Fatal(err)
	}
	for _, s := range [][]Selection{sel(6, 10), sel(5, 121), sel(1, 601)} {
		if _, err := Build(s, lim, NewSession(nil)); !errors.Is(err, ErrRoleLimit) {
			t.Errorf("%d memory spans accepted: %v", len(s)-1, err)
		}
	}
}

// TestOverlapNeverLeaksPartOfAName (review round 3, D-029, DM-2): a
// contact match (a cue handle, an @mention, a URL path or an address) that
// covers only part of a private or self-introduced name never lets the
// rest of the name through: the overlapping matches are merged, and the
// owner's view restores the merged text exactly.
func TestOverlapNeverLeaksPartOfAName(t *testing.T) {
	for _, tc := range []struct {
		names        []string
		in, want     string
		gone         []string
		placeholder  string
		restoredFrom string
	}{
		{[]string{"Dewi Lestari"}, "Cari aja fbnya: Dewi Lestari, dia yang bikin aku sedih.",
			"Cari aja fbnya: [NAMA-1], dia yang bikin aku sedih.", []string{"Dewi", "Lestari"}, "[NAMA-1]", "Dewi Lestari"},
		{[]string{"Dewi Lestari"}, "Aku di-tag @Dewi Lestari di story itu", "Aku di-tag [KONTAK-1] di story itu",
			[]string{"Dewi", "Lestari"}, "[KONTAK-1]", "@Dewi Lestari"},
		{[]string{"Dewi Lestari"}, "Profilnya di linktr.ee/Dewi Lestari bilang dia sibuk.", "Profilnya di [KONTAK-1] bilang dia sibuk.",
			[]string{"Dewi", "Lestari", "linktr"}, "[KONTAK-1]", "linktr.ee/Dewi Lestari"},
		{[]string{"Dewi Lestari"}, "Rumahnya di Gg. Mawar Indah 3 Dewi Lestari tinggal sendirian.", "",
			[]string{"Dewi", "Lestari", "Mawar"}, "", ""},
		{[]string{"Raka Putra Wijaya"}, "line: Raka Putra Wijaya, dia mantan aku.", "line: [NAMA-1], dia mantan aku.",
			[]string{"Raka", "Putra", "Wijaya"}, "[NAMA-1]", "Raka Putra Wijaya"},
		{nil, "Namaku Raka Putra. Cari aja ig: Raka Putra ya.", "Namaku [NAMA-1]. Cari aja ig: [NAMA-1] ya.",
			[]string{"Raka", "Putra"}, "[NAMA-1]", "Raka Putra"},
		{[]string{"Dewi Lestari", "Lestari Ayu"}, "Itu Dewi Lestari Ayu yang aku ceritain.", "Itu [NAMA-1] yang aku ceritain.",
			[]string{"Dewi", "Lestari", "Ayu"}, "[NAMA-1]", "Dewi Lestari Ayu"},
	} {
		s := NewSession(tc.names)
		got := s.Redact(tc.in)
		for _, g := range tc.gone {
			if strings.Contains(got, g) {
				t.Errorf("Redact(%q) = %q still contains %q", tc.in, got, g)
			}
		}
		if tc.want != "" && got != tc.want {
			t.Errorf("Redact(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if tc.placeholder != "" && s.Restore()[tc.placeholder] != tc.restoredFrom {
			t.Errorf("Redact(%q): restore %v, want %s -> %q", tc.in, s.Restore(), tc.placeholder, tc.restoredFrom)
		}
	}
}

// TestPhoneSeparatorForms (review round 3, DM-2): whitespace around a
// separator, double spaces and Unicode dashes between digit groups are a
// phone number, as the output guard already treated them; two numbers side
// by side are both redacted; one-digit groups after a number and ordinary
// numbers stay.
func TestPhoneSeparatorForms(t *testing.T) {
	for _, in := range []string{
		"WA 0812 - 3456 - 7890", "WA 0812  3456  7890", "WA 0812 -3456- 7890", "WA 0812 – 3456 – 7890",
		"WA +62 812 - 3456 - 7890", "WA 0812 . 3456 . 7890", "WA 0812\n3456\n7890", "WA (0812) 3456 7890",
		"WA 0812-34-56-78-90", "WA 0 8 1 2 3 4 5 6 7 8 9 0",
	} {
		if got := NewSession(nil).Redact(in); got != "WA [KONTAK-1]" && got != "WA ([KONTAK-1]" {
			t.Errorf("Redact(%q) = %q", in, got)
		}
	}
	// Two numbers side by side (here also NIK-shaped across the gap): no
	// digit of either is sent.
	for _, in := range []string{"WA 0812 3456 7890 0813 1111 2222 ya", "WA 0812-3456-7890 0813-1111-2222 ya",
		"WA 081234567890 081311112222 ya"} {
		if got := NewSession(nil).Redact(in); strings.ContainsAny(placeholderRe.ReplaceAllString(got, ""), "0123456789") {
			t.Errorf("Redact(%q) = %q keeps digits", in, got)
		}
	}
	for in, want := range map[string]string{
		"WA 0812 3456 7890 2 kali":         "WA [KONTAK-1] 2 kali",
		"Tidur cuma 3 jam, 2 kali bangun.": "Tidur cuma 3 jam, 2 kali bangun.",
	} {
		if got := NewSession(nil).Redact(in); got != want {
			t.Errorf("Redact(%q) = %q, want %q", in, got, want)
		}
	}
	if got := FindPhones("WA 0812 - 3456 - 7890 ya"); len(got) != 1 || got[0] != [2]int{3, 21} {
		t.Fatalf("FindPhones = %v", got)
	}
}

// TestPlaceholderShapedNumbers (review round 3, DM-2): only a placeholder
// a session can produce is kept verbatim; a placeholder-shaped token with
// a long number is redacted by the contact matchers.
func TestPlaceholderShapedNumbers(t *testing.T) {
	for _, in := range []string{"Nomorku [KONTAK-6281234567890] kalau perlu.", "NIK [NAMA-3273250409900003] ya.",
		"Nomorku [KONTAK-081234567890] ya."} {
		got := NewSession(nil).Redact(in)
		for _, digits := range []string{"6281234567890", "3273250409900003", "081234567890", "81234567890"} {
			if strings.Contains(got, digits) {
				t.Errorf("Redact(%q) = %q keeps %s", in, got, digits)
			}
		}
	}
	s := NewSession(nil)
	if got := s.Redact("Cek [KONTAK-12] dan [NAMA-9999] ya, email a@b.co"); got != "Cek [KONTAK-12] dan [NAMA-9999] ya, email [KONTAK-13]" {
		t.Fatalf("kept placeholders: %q", got)
	}
}

// TestRedactPartsOfPrivateNames (DM-2): each word of a multi-word private
// name is redacted when it appears alone, with the full name's placeholder
// and restore value; address terms and particles are not parts, and a part
// two private names share gets its own placeholder.
func TestRedactPartsOfPrivateNames(t *testing.T) {
	s := NewSession([]string{"Budi Santoso", "Pak Joko Widodo", "Ahmad bin Yusuf"})
	out := s.Redact("Budi Santoso marah. Kata Santoso, budi salah. Pak Joko diam, Widodo pergi. Ahmad dan Yusuf bin teman.")
	for _, leak := range []string{"Budi", "budi", "Santoso", "Joko", "Widodo", "Ahmad", "Yusuf"} {
		if strings.Contains(out, leak) {
			t.Fatalf("part %q leaked: %q", leak, out)
		}
	}
	if !strings.Contains(out, "Pak [NAMA-2]") || !strings.Contains(out, "bin teman") {
		t.Fatalf("an address term or particle was redacted as a part: %q", out)
	}
	if strings.Count(out, "[NAMA-1]") != 3 {
		t.Fatalf("the parts of one name do not share its placeholder: %q", out)
	}
	r := s.Restore()
	if r["[NAMA-1]"] != "Budi Santoso" || r["[NAMA-2]"] != "Pak Joko Widodo" {
		t.Fatalf("restore = %v", r)
	}
	names := strings.Join(s.Names(), ",")
	for _, want := range []string{"Budi", "Santoso", "Joko", "Widodo", "Ahmad", "Yusuf"} {
		if !strings.Contains(names, want) {
			t.Fatalf("Names() = %s lacks the part %s (the output guard blocks Names)", names, want)
		}
	}
	for _, not := range []string{",bin", ",Pak,"} {
		if strings.Contains(","+names+",", not) {
			t.Fatalf("Names() = %s lists %s", names, not)
		}
	}

	// A shared part names neither person for sure: its own placeholder.
	s = NewSession([]string{"Dewi Lestari", "Dewi Sartika"})
	out = s.Redact("Dewi Lestari dan Dewi Sartika teman. Dewi datang, Lestari tidak.")
	if strings.Contains(out, "Dewi") || strings.Contains(out, "Lestari") || strings.Contains(out, "Sartika") {
		t.Fatalf("leak: %q", out)
	}
	r = s.Restore()
	if r["[NAMA-3]"] != "Dewi" || !strings.HasSuffix(out, "[NAMA-3] datang, [NAMA-1] tidak.") {
		t.Fatalf("shared part: %q restore %v", out, r)
	}
	// A one-word private name has no parts; words inside other words stay.
	if out := NewSession([]string{"Budi Santoso"}).Redact("Budiman dan Santosoan."); out != "Budiman dan Santosoan." {
		t.Fatalf("matched inside a word: %q", out)
	}
}

// TestPhoneMiddleDotSlashUnderscore (DM-2): the separators "·", "/" and
// "_" (and the other middle dots and bullets a View maps to '.') join a
// phone number; a slash-separated date is not a phone number.
func TestPhoneMiddleDotSlashUnderscore(t *testing.T) {
	for _, in := range []string{"WA 0812·3456·7890", "WA 0812 · 3456 · 7890", "WA 0812/3456/7890", "WA 0812 / 3456 / 7890",
		"WA 0812_3456_7890", "WA 0812•3456•7890", "WA 0812∙3456∙7890", "WA 0812・3456・7890", "WA +62·812·3456·7890",
		"WA 021/7654321"} {
		if got := NewSession(nil).Redact(in); got != "WA [KONTAK-1]" {
			t.Errorf("Redact(%q) = %q", in, got)
		}
	}
	for _, in := range []string{"Tanggal 08/10/2026 aku ujian.", "Dari 01/02/2026 10/11/2026 aku libur.", "Jam 08/09 pagi.",
		"Nilai 0/10 dan 05/2026."} {
		if got := NewSession(nil).Redact(in); got != in {
			t.Errorf("Redact(%q) = %q, want unchanged", in, got)
		}
	}
	// A date and a number in one run: the number is redacted, the date kept.
	if got := NewSession(nil).Redact("Tgl 08/10/2026 08123456789 ya"); got != "Tgl 08/10/2026 [KONTAK-1] ya" {
		t.Errorf("date then number: %q", got)
	}
}

// TestPhoneAfterDateAndInternationalSlash (review round 4): leading date
// groups are skipped and the rest of the run is read as numbers again, so
// a number right after a short date is redacted; a run that opens with "+"
// or "62" never holds a date, so its slashes join the number.
func TestPhoneAfterDateAndInternationalSlash(t *testing.T) {
	for in, want := range map[string]string{
		"kontrol 05/11 08123456789 ya":                "kontrol 05/11 [KONTAK-1] ya",
		"kontrol 05/11 0812-3456-7890 ya":             "kontrol 05/11 [KONTAK-1] ya",
		"08/10/2026 0812 345 6789":                    "08/10/2026 [KONTAK-1]",
		"Dari 01/02/2026 10/11/2026 0812 345 6789 ya": "Dari 01/02/2026 10/11/2026 [KONTAK-1] ya",
		"Tgl 5/11/26 08123456789":                     "Tgl 5/11/26 [KONTAK-1]",
		"Hubungi +62 812 3456/7890 ya":                "Hubungi [KONTAK-1] ya",
		"Hubungi +62 812/3456/7890 ya":                "Hubungi [KONTAK-1] ya",
		"Hubungi 62 812/3456/7890 ya":                 "Hubungi [KONTAK-1] ya",
		"WA 08/12/3456/7890":                          "WA [KONTAK-1]",
		"WA 08/12/3456 7890":                          "WA [KONTAK-1]",
		"Kontrol 05/11/2026 10.30 ya.":                "Kontrol 05/11/2026 10.30 ya.",
		"Jadwal 05/11 dan 12/11, tensi 120/80 ya.":    "Jadwal 05/11 dan 12/11, tensi 120/80 ya.",
		"Tanggal 08/10/2026 aku ujian.":               "Tanggal 08/10/2026 aku ujian.",
		"Dari 01/02/2026 10/11/2026 aku libur.":       "Dari 01/02/2026 10/11/2026 aku libur.",
		// Review round 5: dates with dots, dashes or spaces are skipped too,
		// so the whole number after them is replaced, never its first
		// group alone; a number with those separators stays one number.
		"Kontrol 05.11.2026 0812 3456 789 ya":  "Kontrol 05.11.2026 [KONTAK-1] ya",
		"Kontrol 05-11-2026 0812 3456 789 ya":  "Kontrol 05-11-2026 [KONTAK-1] ya",
		"Kontrol 05.11.2026 0812-3456-789":     "Kontrol 05.11.2026 [KONTAK-1]",
		"Kontrol 05.11.2026 0812 345 678":      "Kontrol 05.11.2026 [KONTAK-1]",
		"Kontrol 05 - 11 - 2026 0812 3456 789": "Kontrol 05 - 11 - 2026 [KONTAK-1]",
		"Kontrol 05 11 2026 0812 3456 789 ya":  "Kontrol 05 11 2026 [KONTAK-1] ya",
		"Kontrol 05.11 0812 3456 7890":         "Kontrol 05.11 [KONTAK-1]",
		"Kontrol 5.11.26 08123456789":          "Kontrol 5.11.26 [KONTAK-1]",
		"WA 0812-3456-7890":                    "WA [KONTAK-1]",
		"WA 0812.3456.7890":                    "WA [KONTAK-1]",
		"WA 08.12.3456.7890":                   "WA [KONTAK-1]",
		"WA 08-12-3456-7890":                   "WA [KONTAK-1]",
		"WA 08.12.34.56.78.90":                 "WA [KONTAK-1]",
		// A rest too short to be a number after a cut at 15 digits goes
		// with the number before it (a date shape dateAt does not know).
		"Kontrol 05.11.26.0812 3456 789 ya": "Kontrol [KONTAK-1] ya",
		"Kontrol 05.11.2026 jam 10.30 ya.":  "Kontrol 05.11.2026 jam 10.30 ya.",
		"Tanggal 05-11-2026 aku ujian.":     "Tanggal 05-11-2026 aku ujian.",
		"Tanggal 05 11 2026 aku ujian.":     "Tanggal 05 11 2026 aku ujian.",
	} {
		if got := NewSession(nil).Redact(in); got != want {
			t.Errorf("Redact(%q) = %q, want %q", in, got, want)
		}
	}
	// (Redact also replaces "2026 0812 3456 7890" there as a NIK-shaped
	// 4-4-4-4 group; the phone matcher alone keeps the date.)
	for in, want := range map[string]string{
		"08/10/2026 0812 3456 7890": "0812 3456 7890",
		"+62 812 3456/7890":         "+62 812 3456/7890",
		"+62 812/3456/7890":         "+62 812/3456/7890",
		"05/11 08123456789":         "08123456789",
		"05.11.2026 0812 3456 789":  "0812 3456 789",
		"05-11-2026 0812-3456-789":  "0812-3456-789",
		"05 11 2026 0812 3456 789":  "0812 3456 789",
		"0812-3456-7890":            "0812-3456-7890",
		"0812.3456.7890":            "0812.3456.7890",
	} {
		got := FindPhones(in)
		if len(got) != 1 || in[got[0][0]:got[0][1]] != want {
			t.Errorf("FindPhones(%q) = %v, want %q", in, got, want)
		}
	}
}

// TestCommonWordNamePart (review round 4): a part of a private name that
// is also an everyday word is redacted alone only capitalised and outside a
// fixed phrase; the full name and the other parts are matched as before,
// and a one-word private name keeps matching everywhere.
func TestCommonWordNamePart(t *testing.T) {
	s := NewSession([]string{"Kasih Ramadhani"})
	if got := s.Redact("Terima kasih ya, aku cinta kamu"); got != "Terima kasih ya, aku cinta kamu" {
		t.Fatalf("everyday words redacted: %q", got)
	}
	in := "Kasih bilang aku jahat. Terima Kasih, Kasih. Kasih sayang itu penting. ramadhani pergi. kasih ramadhani datang."
	want := "[NAMA-1] bilang aku jahat. Terima Kasih, [NAMA-1]. Kasih sayang itu penting. [NAMA-1] pergi. [NAMA-1] datang."
	if got := s.Redact(in); got != want {
		t.Fatalf("Redact = %q, want %q", got, want)
	}
	if r := s.Restore(); r["[NAMA-1]"] != "Kasih Ramadhani" {
		t.Fatalf("restore %v", r)
	}
	names := "," + strings.Join(s.Names(), ",") + ","
	if strings.Contains(names, ",Kasih,") || !strings.Contains(names, ",Ramadhani,") || !strings.Contains(names, ",Kasih Ramadhani,") {
		t.Fatalf("Names() = %s", names)
	}
	if parts := s.CommonNameParts(); len(parts) != 1 || parts[0] != "Kasih" {
		t.Fatalf("CommonNameParts() = %v", parts)
	}
	if got := NewSession([]string{"Kasih"}).Redact("kasih bilang aku jahat"); got != "[NAMA-1] bilang aku jahat" {
		t.Fatalf("one-word private name: %q", got)
	}
	// Review round 5: a fixed phrase keeps a capitalised part the everyday
	// word only where it opens a sentence (or follows a capitalised word
	// of the phrase, "Terima Kasih"); after a lowercase word in the middle
	// of a sentence it is the name. A part whose everyday meaning is rare
	// ("bayu", "agung", "intan") is matched in any case outside a fixed
	// phrase; one whose everyday meaning dominates ("indah", "bagus",
	// "kasih") only capitalised.
	for _, tc := range commonPartCases {
		got := NewSession([]string{tc.name}).Redact(tc.in)
		if got != tc.want {
			t.Errorf("%v: Redact(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// commonPartCases are the inputs of TestCommonWordNamePart from review
// round 5; guard.TestCommonWordNamePartParity checks NAME-1 on the same
// kind of text.
var commonPartCases = []struct{ name, in, want string }{
	{"Indah Permata", "Aku kangen Indah banget.", "Aku kangen [NAMA-1] banget."},
	{"Indah Permata", "Indah banget pemandangannya.", "Indah banget pemandangannya."},
	{"Indah Permata", "Hari ini cerah.\nIndah sekali langitnya.", "Hari ini cerah.\nIndah sekali langitnya."},
	{"Indah Permata", "pemandangannya indah banget", "pemandangannya indah banget"},
	{"Indah Permata", "aku kangen permata", "aku kangen [NAMA-1]"},
	{"Bagus Pratama", "Tadi aku cerita ke Bagus ya, terus dia diam.", "Tadi aku cerita ke [NAMA-1] ya, terus dia diam."},
	{"Bagus Pratama", "Aku udah ngomong sama Bagus kok.", "Aku udah ngomong sama [NAMA-1] kok."},
	{"Bagus Pratama", "Bagus banget idenya. Idenya bagus kok.", "Bagus banget idenya. Idenya bagus kok."},
	{"Putri Ayu", "Aku ketemu Putri kami di sekolah.", "Aku ketemu [NAMA-1] kami di sekolah."},
	{"Putri Ayu", "aku ketemu putri tadi", "aku ketemu [NAMA-1] tadi"},
	{"Putri Ayu", "Putri saya sudah sekolah, putri kami juga.", "Putri saya sudah sekolah, putri kami juga."},
	{"Bayu Pratama", "td aku berantem sama bayu, dia marah", "td aku berantem sama [NAMA-1], dia marah"},
	{"Intan Lestari", "aku kangen intan", "aku kangen [NAMA-1]"},
	{"Intan Lestari", "lestari blm bales", "[NAMA-1] blm bales"},
	{"Agung Wibowo", "agung blm bales chat aku", "[NAMA-1] blm bales chat aku"},
	{"Agung Wibowo", "Kemarin aku ke masjid agung.", "Kemarin aku ke masjid agung."},
	{"Surya Jaya", "aku sama surya, terus jaya dateng", "aku sama [NAMA-1], terus [NAMA-1] dateng"},
	{"Fajar Teguh", "fajar marah, tapi aku tetap teguh", "[NAMA-1] marah, tapi aku tetap teguh"},
	{"Kasih Ramadhani", "Aku mau bilang Terima Kasih ke kamu.", "Aku mau bilang Terima Kasih ke kamu."},
	{"Kasih Ramadhani", "\"Kasih sayang itu penting,\" katanya.", "\"Kasih sayang itu penting,\" katanya."},
	{"Kasih Ramadhani", "Aku rindu Kasih sayang banget.", "Aku rindu [NAMA-1] sayang banget."},
	{"Kasih Ramadhani", "aku kasih tau dia", "aku kasih tau dia"},
}

// TestLearnedNameUncommonsPart (review round 5): a name the user
// introduces themselves with that is a common part of a private name
// ("namaku Kasih" with "Kasih Ramadhani" private) is matched in any case,
// as in a session without private names, keeping the private name's
// placeholder; NAME-1 (Names) blocks it in any case too.
func TestLearnedNameUncommonsPart(t *testing.T) {
	const text = "Namaku Kasih. Temen-temen bilang kasih itu pendiam."
	want := "Namaku [NAMA-1]. Temen-temen bilang [NAMA-1] itu pendiam."
	if got := NewSession(nil).Redact(text); got != want {
		t.Fatalf("without private names: %q", got)
	}
	check := func(s *Session) {
		t.Helper()
		if r := s.Restore(); r["[NAMA-1]"] != "Kasih Ramadhani" {
			t.Errorf("restore %v", r)
		}
		if names := "," + strings.Join(s.Names(), ",") + ","; !strings.Contains(names, ",Kasih,") {
			t.Errorf("Names() = %s", names)
		}
		if parts := s.CommonNameParts(); len(parts) != 0 {
			t.Errorf("CommonNameParts() = %v", parts)
		}
	}
	// Learned in the same span.
	s := NewSession([]string{"Kasih Ramadhani"})
	if got := s.Redact(text); got != want {
		t.Fatalf("same span: %q, want %q", got, want)
	}
	check(s)
	// Learned in an earlier span.
	s = NewSession([]string{"Kasih Ramadhani"})
	if got := s.Redact("Namaku Kasih."); got != "Namaku [NAMA-1]." {
		t.Fatalf("first span: %q", got)
	}
	if got := s.Redact("Temen-temen bilang kasih itu pendiam."); got != "Temen-temen bilang [NAMA-1] itu pendiam." {
		t.Fatalf("later span: %q", got)
	}
	check(s)
	// Learned as the given name of a longer self-introduced name.
	s = NewSession([]string{"Kasih Ramadhani"})
	if got := s.Redact("Panggil aku Kasih Putri. kasih capek."); strings.Contains(got, "asih") {
		t.Fatalf("given name: %q", got)
	}
	// A common part nobody introduced stays capitalised only.
	if got := NewSession([]string{"Kasih Ramadhani"}).Redact("Dia kasih aku buku."); got != "Dia kasih aku buku." {
		t.Fatalf("everyday word: %q", got)
	}
}
