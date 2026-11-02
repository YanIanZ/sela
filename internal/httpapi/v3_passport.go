package httpapi

import (
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

// Care Passport (BW-005/006) is a preset of the consultation summary: the
// preview route and the export preset both render the recipient projection
// that share and care-request creation freeze (account.projectSummary).
// Nothing here builds a second projection or a new record.

// v3PassportPreview returns the owner's Care Passport preview: effective
// state, pinned sources with their currency, and the exact recipient
// content. The summary detail alone cannot give the recipient content: the
// owner's private names are masked server-side and source labels come
// from the sealed source records.
func (s *Server) v3PassportPreview(w http.ResponseWriter, r *http.Request, p account.Principal) {
	preview, err := s.opt.Accounts.PassportPreview(r.Context(), p.UserID, r.PathValue("id"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"passport": preview})
}

// v3ExportPassport renders the passport preset of an approved, current
// summary. A draft, stale (MIE) or source-changed version is refused with
// 409 invalid_transition, exactly like the default export.
func (s *Server) v3ExportPassport(w http.ResponseWriter, r *http.Request, p account.Principal, format, lang string) {
	preview, err := s.opt.Accounts.PassportPreview(r.Context(), p.UserID, r.PathValue("id"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	if !preview.Shareable {
		writeError(w, r, http.StatusConflict, "invalid_transition", "")
		return
	}
	var body string
	contentType := "text/html; charset=utf-8"
	if format == "html" {
		body = renderPassportHTML(preview.Recipient, preview.UpdatedAt, lang)
	} else {
		contentType = "text/markdown; charset=utf-8"
		body = renderPassportMarkdown(preview.Recipient, preview.UpdatedAt, lang)
	}
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Content-Disposition", `attachment; filename="sela-persiapan-konsultasi.`+format+`"`)
	h.Set("Cache-Control", "no-store")
	if format == "html" {
		h.Set("Content-Security-Policy", exportCSP)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

type passportWords struct {
	heading, disclaimer, version, purpose, share, words, story, ask, bring, footer string
}

func passportCopy(lang string) passportWords {
	if lang == "en" {
		return passportWords{
			heading: "My consultation preparation",
			disclaimer: "A personal consultation preparation sheet, written and approved by its owner. " +
				"It is not a medical passport, an official health record, a diagnosis or a document from a professional. " +
				"Sela does not check or set any diagnosis.",
			version: "Approved version",
			purpose: "My purpose", share: "What I chose to share", words: "In my words",
			story: "My summary", ask: "What I want to ask", bring: "What I will bring",
			footer: "Sela · Care Passport is a nickname for this preparation sheet. " +
				"A recipient may keep a copy they have seen; revoking blocks later access, not copies already downloaded.",
		}
	}
	return passportWords{
		heading: "Ringkasan untuk dibawa",
		disclaimer: "Ringkasan persiapan konsultasi pribadi, ditulis dan disetujui pemiliknya. " +
			"Ini bukan paspor medis, rekam medis resmi, diagnosis, atau dokumen dari tenaga profesional. " +
			"Sela tidak memeriksa atau menetapkan diagnosis.",
		version: "Versi yang disetujui",
		purpose: "Tujuanku", share: "Yang ingin kubagikan", words: "Dengan kata-kataku",
		story: "Ringkasanku", ask: "Yang ingin kutanyakan", bring: "Yang kubawa",
		footer: "Sela · Care Passport adalah nama lain lembar persiapan ini. " +
			"Penerima bisa menyimpan salinan yang sudah dilihat; pencabutan memblokir akses berikutnya, bukan salinan yang sudah diunduh.",
	}
}

// passportQuestions splits the owner's questions into one per non-blank
// line, preserving each line's exact text.
func passportQuestions(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}

func renderPassportHTML(c account.RecipientContent, updated time.Time, lang string) string {
	t := passportCopy(lang)
	esc := html.EscapeString
	title := strings.TrimSpace(c.Title)
	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"" + lang + "\"><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	b.WriteString("<title>" + esc(t.heading) + "</title><style>")
	b.WriteString("body{font:16px/1.6 system-ui,-apple-system,Segoe UI,sans-serif;max-width:46rem;margin:2rem auto;padding:0 1.25rem;color:#1d1d1f}")
	b.WriteString("h1{font-size:1.6rem;line-height:1.25}h2{font-size:1.1rem;margin-top:2rem}")
	b.WriteString("blockquote{margin:1rem 0;padding:.5rem 1rem;border-left:3px solid #888;color:#444}")
	b.WriteString("p,li{white-space:pre-wrap;overflow-wrap:anywhere}.note{border:1px solid #888;padding:.75rem 1rem}")
	b.WriteString("footer{margin-top:3rem;font-size:.875rem;color:#555}")
	b.WriteString("</style></head><body>")
	b.WriteString("<h1>" + esc(t.heading) + "</h1>")
	if title != "" {
		b.WriteString("<p><strong>" + esc(title) + "</strong></p>")
	}
	b.WriteString("<p class=\"note\">" + esc(t.disclaimer) + "</p>")
	b.WriteString("<p><small>" + esc(t.version) + " v" + strconv.Itoa(c.SummaryVersion) + " · " + esc(updated.Format(time.DateOnly)) + "</small></p>")
	if c.Prepared != nil && strings.TrimSpace(c.Prepared.Goal) != "" {
		b.WriteString("<h2>" + esc(t.purpose) + "</h2><p>" + esc(c.Prepared.Goal) + "</p>")
	}
	b.WriteString("<h2>" + esc(t.share) + "</h2><ul>")
	for _, src := range c.Sources {
		b.WriteString("<li>" + esc(src) + "</li>")
	}
	b.WriteString("</ul>")
	if len(c.Items) > 0 {
		b.WriteString("<h2>" + esc(t.words) + "</h2>")
		for _, item := range c.Items {
			b.WriteString("<section>")
			b.WriteString("<p>" + esc(item.Text) + "</p>")
			if strings.TrimSpace(item.Quote) != "" {
				b.WriteString("<blockquote><p>" + esc(item.Quote) + "</p></blockquote>")
			}
			b.WriteString("<p><small>" + esc(item.SourceLabel) + "</small></p>")
			b.WriteString("</section>")
		}
	}
	if strings.TrimSpace(c.Text) != "" {
		b.WriteString("<h2>" + esc(t.story) + "</h2><p>" + esc(c.Text) + "</p>")
	}
	if c.Prepared != nil {
		if qs := passportQuestions(c.Prepared.Questions); len(qs) > 0 {
			b.WriteString("<h2>" + esc(t.ask) + "</h2><ol>")
			for _, q := range qs {
				b.WriteString("<li>" + esc(q) + "</li>")
			}
			b.WriteString("</ol>")
		}
		if strings.TrimSpace(c.Prepared.Bring) != "" {
			b.WriteString("<h2>" + esc(t.bring) + "</h2><p>" + esc(c.Prepared.Bring) + "</p>")
		}
	}
	b.WriteString("<footer>" + esc(t.footer) + "</footer></body></html>")
	return b.String()
}

// mdText makes one line of owner text inert in Markdown while keeping its
// rendered text exact: HTML is entity-escaped (no raw tag or comment
// survives), inline punctuation is backslash-escaped (no link, image,
// emphasis or code span) and a leading block marker is escaped (no
// heading, list, quote, rule or fence). Line breaks become spaces; use
// mdBlock for multi-line text.
func mdText(s string) string {
	s = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
			continue
		case '<':
			b.WriteString("&lt;")
			continue
		case '>':
			b.WriteString("&gt;")
			continue
		case '\\', '`', '*', '_', '[', ']', '~', '|':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	out := b.String()
	trimmed := strings.TrimLeft(out, " \t")
	// Leading indentation as entities: four spaces must not start a code
	// block, where the escapes above would show literally.
	lead := strings.NewReplacer(" ", "&#32;", "\t", "&#9;").Replace(out[:len(out)-len(trimmed)])
	out = lead + trimmed
	if trimmed != "" {
		switch trimmed[0] {
		case '#', '-', '+', '=':
			return lead + "\\" + trimmed
		}
		digits := len(trimmed) - len(strings.TrimLeft(trimmed, "0123456789"))
		if digits > 0 && digits < len(trimmed) && (trimmed[digits] == '.' || trimmed[digits] == ')') {
			return lead + trimmed[:digits] + "\\" + trimmed[digits:]
		}
	}
	return out
}

// mdBlock renders a multi-line owner text as hard-broken Markdown lines.
func mdBlock(s, prefix string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		lines[i] = prefix + mdText(strings.TrimRight(line, " \t\r"))
	}
	return strings.Join(lines, "  \n") + "\n\n"
}

func renderPassportMarkdown(c account.RecipientContent, updated time.Time, lang string) string {
	t := passportCopy(lang)
	var b strings.Builder
	b.WriteString("# " + t.heading + "\n\n")
	if strings.TrimSpace(c.Title) != "" {
		b.WriteString("**" + mdText(strings.TrimSpace(c.Title)) + "**\n\n")
	}
	b.WriteString("> " + t.disclaimer + "\n\n")
	b.WriteString("_" + t.version + " v" + strconv.Itoa(c.SummaryVersion) + " · " + updated.Format(time.DateOnly) + "_\n\n")
	if c.Prepared != nil && strings.TrimSpace(c.Prepared.Goal) != "" {
		b.WriteString("## " + t.purpose + "\n\n" + mdBlock(c.Prepared.Goal, ""))
	}
	b.WriteString("## " + t.share + "\n\n")
	for _, src := range c.Sources {
		b.WriteString("- " + mdText(src) + "\n")
	}
	b.WriteString("\n")
	if len(c.Items) > 0 {
		b.WriteString("## " + t.words + "\n\n")
		for _, item := range c.Items {
			b.WriteString(mdBlock(item.Text, ""))
			if strings.TrimSpace(item.Quote) != "" {
				b.WriteString(mdBlock(item.Quote, "> "))
			}
			b.WriteString("_" + mdText(item.SourceLabel) + "_\n\n")
		}
	}
	if strings.TrimSpace(c.Text) != "" {
		b.WriteString("## " + t.story + "\n\n" + mdBlock(c.Text, ""))
	}
	if c.Prepared != nil {
		if qs := passportQuestions(c.Prepared.Questions); len(qs) > 0 {
			b.WriteString("## " + t.ask + "\n\n")
			for i, q := range qs {
				b.WriteString(strconv.Itoa(i+1) + ". " + mdText(q) + "\n")
			}
			b.WriteString("\n")
		}
		if strings.TrimSpace(c.Prepared.Bring) != "" {
			b.WriteString("## " + t.bring + "\n\n" + mdBlock(c.Prepared.Bring, ""))
		}
	}
	b.WriteString("---\n\n" + t.footer + "\n")
	return b.String()
}
