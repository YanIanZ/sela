package httpapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

// passportFixture is synthetic owner text full of HTML and Markdown
// control characters.
func passportFixture() account.RecipientContent {
	return account.RecipientContent{
		SummaryVersion: 3,
		Title:          `<script>alert("t")</script> & judul`,
		Text:           "Baris satu <b>tebal</b>\n# bukan judul\n    bukan kode",
		Prepared: &account.SummaryPrepared{
			Goal:      `Aku ingin menjelaskan "capek" & <img src=x onerror=alert(1)>`,
			Questions: "1. Apa ini [tautan](javascript:alert(1))?\n\n- Kenapa *aku* begini?\n<!-- komentar -->",
			Bring:     "`kode` dan \\ garis miring",
		},
		Items: []account.SharedSummaryItem{{
			Text: "Aku > kamu? _tidak_ selalu.", Quote: "Kadang aku ingin sendiri.\n> kutipan palsu",
			SourceLabel: "Catatan <A> · v2", Position: 0,
		}},
		Sources: []string{"Catatan <A> · v2", "![gambar](http://x) · v1"},
	}
}

// TestPassportExportSnapshot pins the Care Passport preset byte for byte
// (CP-T07): owner text with HTML and Markdown control characters is inert
// and every owner word is still there. SELA_UPDATE_GOLDEN=1 rewrites the
// golden files; review the diff before keeping it.
func TestPassportExportSnapshot(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	cases := []struct{ file, got string }{
		{"passport_export_id.golden.html", renderPassportHTML(passportFixture(), at, "id")},
		{"passport_export_en.golden.md", renderPassportMarkdown(passportFixture(), at, "en")},
	}
	for _, c := range cases {
		path := filepath.Join("testdata", c.file)
		if os.Getenv("SELA_UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(path, []byte(c.got), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if c.got != string(want) {
			t.Errorf("%s changed:\n got %q\nwant %q", c.file, c.got, want)
		}
	}
	html := cases[0].got
	for _, bad := range []string{"<script", "<img", "<b>", "<!--"} {
		if strings.Contains(html, bad) {
			t.Errorf("html export carries raw %q", bad)
		}
	}
	md := cases[1].got
	for _, bad := range []string{"<script", "<img", "<b>", "<!--", "ini [tautan]", "![gambar]", "\n# bukan", "\n    bukan"} {
		if strings.Contains(md, bad) {
			t.Errorf("markdown export carries active %q", bad)
		}
	}
	for _, must := range []string{"bukan paspor medis", "Tujuanku", "Yang ingin kubagikan", "Dengan kata-kataku", "Yang ingin kutanyakan"} {
		if !strings.Contains(html, must) {
			t.Errorf("html export lacks %q", must)
		}
	}
	if !strings.Contains(md, "not a medical passport") {
		t.Error("markdown export lacks the nonclinical disclaimer")
	}
}

func TestMdTextKeepsRenderedText(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"# judul":      `\# judul`,
		"  - butir":    `&#32;&#32;\- butir`,
		"12) nomor":    `12\) nomor`,
		"2026 tahun.":  "2026 tahun.",
		"a <b> & c":    "a &lt;b&gt; &amp; c",
		"[x](y) *z* _": `\[x\](y) \*z\* \_`,
		"baris\nbaru":  "baris baru",
	} {
		if got := mdText(in); got != want {
			t.Errorf("mdText(%q) = %q, want %q", in, got, want)
		}
	}
}
