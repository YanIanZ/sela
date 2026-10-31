package httpapi

import (
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

// v3ExportSummary renders an approved consultation summary as a
// self-contained HTML or Markdown file. Only the owner's own words and the
// pinned source labels are included; nothing is fetched from a provider.
func (s *Server) v3ExportSummary(w http.ResponseWriter, r *http.Request, p account.Principal) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "html"
	}
	if format != "html" && format != "md" {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "format")
		return
	}
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = r.Header.Get("X-Sela-Language")
	}
	if lang != "en" {
		lang = "id"
	}
	rec, err := s.opt.Accounts.Summary(r.Context(), p.UserID, r.PathValue("id"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	if rec.SummaryState != "approved" || rec.ApprovedVersion != rec.Version {
		writeError(w, r, http.StatusConflict, "invalid_transition", "")
		return
	}
	labels := make([]string, 0, len(rec.SourceRefs))
	labelsByRef := make(map[string]string, len(rec.SourceRefs))
	for _, ref := range rec.SourceRefs {
		label := ref.RecordID
		if src, err := s.opt.Accounts.Record(r.Context(), p.UserID, ref.RecordID); err == nil {
			if src.Title != "" {
				label = src.Title
			} else if src.Kind != "" {
				label = src.Kind
			}
		}
		display := label + " · v" + strconv.Itoa(ref.Version)
		labels = append(labels, display)
		labelsByRef[ref.RecordID+"/"+strconv.Itoa(ref.Version)] = display
	}
	itemLabels := make([]string, len(rec.Items))
	for i, item := range rec.Items {
		itemLabels[i] = labelsByRef[item.SourceRef.RecordID+"/"+strconv.Itoa(item.SourceRef.Version)]
		if itemLabels[i] == "" {
			itemLabels[i] = item.SourceRef.RecordID + " · v" + strconv.Itoa(item.SourceRef.Version)
		}
	}
	var body []byte
	contentType, ext := "text/html; charset=utf-8", "html"
	if format == "html" {
		body = []byte(renderSummaryHTML(rec, labels, itemLabels, lang))
	} else {
		contentType, ext = "text/markdown; charset=utf-8", "md"
		body = []byte(renderSummaryMarkdown(rec, labels, itemLabels, lang))
	}
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Content-Disposition", `attachment; filename="sela-ringkasan-`+ext+`"`)
	h.Set("Cache-Control", "no-store")
	if format == "html" {
		h.Set("Content-Security-Policy", exportCSP)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func summaryCopy(lang string) (title, note, summary, goal, questions, bring, sources, footer string) {
	if lang == "en" {
		return "Personal summary", "Written and approved by the user · not a diagnosis or an official professional record",
			"Summary", "What I want to talk about", "Questions I want to ask", "What I will bring",
			"Chosen sources", "Sela · a little space to feel heard. This file is yours; downloaded copies cannot be recalled."
	}
	return "Ringkasan pribadi", "Ditulis dan disetujui pengguna · bukan diagnosis atau catatan resmi tenaga profesional",
		"Ringkasan", "Yang ingin saya ceritakan", "Pertanyaan yang ingin saya ajukan", "Yang akan saya bawa",
		"Sumber pilihan", "Sela · ruang sejenak untuk didengar. Berkas ini milikmu; salinan yang diunduh tidak bisa ditarik kembali."
}

func summaryTitle(rec account.PersonalRecord) string {
	if strings.TrimSpace(rec.Title) != "" {
		return rec.Title
	}
	return "Ringkasan pribadi"
}

func summaryItemsTitle(lang string) string {
	if lang == "en" {
		return "Statements from your sources"
	}
	return "Bagian dari sumbermu"
}

func renderSummaryHTML(rec account.PersonalRecord, labels, itemLabels []string, lang string) string {
	_, note, summary, goalTitle, questionsTitle, bringTitle, sourcesTitle, footer := summaryCopy(lang)
	display := html.EscapeString(summaryTitle(rec))
	var b strings.Builder
	b.WriteString("<!doctype html><html lang=\"" + lang + "\"><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	b.WriteString("<title>" + display + "</title><style>")
	b.WriteString("body{font:16px/1.6 system-ui,-apple-system,Segoe UI,sans-serif;max-width:46rem;margin:2rem auto;padding:0 1.25rem;color:#1d1d1f}")
	b.WriteString("h1{font-size:1.6rem;line-height:1.25}h2{font-size:1.1rem;margin-top:2rem}blockquote{margin:1rem 0;padding:.5rem 1rem;border-left:3px solid #888;color:#444}")
	b.WriteString("ul{padding-left:1.25rem}footer{margin-top:3rem;font-size:.875rem;color:#555}")
	b.WriteString(".statement{margin:1.5rem 0}.statement p{white-space:pre-wrap;overflow-wrap:anywhere}")
	b.WriteString("</style></head><body>")
	b.WriteString("<h1>" + display + "</h1><blockquote>" + html.EscapeString(note) + "</blockquote>")
	b.WriteString("<p><small>v" + strconv.Itoa(rec.Version) + " · " + html.EscapeString(rec.UpdatedAt.Format(time.DateOnly)) + "</small></p>")
	if strings.TrimSpace(rec.Text) != "" {
		b.WriteString("<h2>" + html.EscapeString(summary) + "</h2><p>" + html.EscapeString(rec.Text) + "</p>")
	}
	if len(rec.Items) > 0 {
		b.WriteString("<h2>" + html.EscapeString(summaryItemsTitle(lang)) + "</h2>")
		for i, item := range rec.Items {
			b.WriteString("<section class=\"statement\">")
			if strings.TrimSpace(item.Quote) != "" {
				b.WriteString("<blockquote><p>" + html.EscapeString(item.Quote) + "</p></blockquote>")
			}
			b.WriteString("<p>" + html.EscapeString(item.Text) + "</p>")
			if i < len(itemLabels) && itemLabels[i] != "" {
				b.WriteString("<p><small>" + html.EscapeString(itemLabels[i]) + "</small></p>")
			}
			b.WriteString("</section>")
		}
	}
	if rec.Prepared != nil {
		for _, section := range []struct{ title, text string }{
			{goalTitle, rec.Prepared.Goal}, {questionsTitle, rec.Prepared.Questions}, {bringTitle, rec.Prepared.Bring},
		} {
			if strings.TrimSpace(section.text) != "" {
				b.WriteString("<h2>" + html.EscapeString(section.title) + "</h2><p>" + html.EscapeString(section.text) + "</p>")
			}
		}
	}
	b.WriteString("<h2>" + html.EscapeString(sourcesTitle) + "</h2><ul>")
	for _, label := range labels {
		b.WriteString("<li>" + html.EscapeString(label) + "</li>")
	}
	b.WriteString("</ul><footer>" + html.EscapeString(footer) + "</footer></body></html>")
	return b.String()
}

func renderSummaryMarkdown(rec account.PersonalRecord, labels, itemLabels []string, lang string) string {
	_, note, summary, goalTitle, questionsTitle, bringTitle, sourcesTitle, footer := summaryCopy(lang)
	var b strings.Builder
	b.WriteString("# " + summaryTitle(rec) + "\n\n> " + note + "\n\n")
	b.WriteString("_v" + strconv.Itoa(rec.Version) + " · " + rec.UpdatedAt.Format(time.DateOnly) + "_\n\n")
	if strings.TrimSpace(rec.Text) != "" {
		b.WriteString("## " + summary + "\n\n" + rec.Text + "\n\n")
	}
	if len(rec.Items) > 0 {
		b.WriteString("## " + summaryItemsTitle(lang) + "\n\n")
		for i, item := range rec.Items {
			if strings.TrimSpace(item.Quote) != "" {
				for _, line := range strings.Split(strings.TrimSpace(item.Quote), "\n") {
					b.WriteString("> " + line + "\n")
				}
				b.WriteString("\n")
			}
			b.WriteString(item.Text + "\n\n")
			if i < len(itemLabels) && itemLabels[i] != "" {
				b.WriteString("_" + itemLabels[i] + "_\n\n")
			}
		}
	}
	if rec.Prepared != nil {
		for _, section := range []struct{ title, text string }{
			{goalTitle, rec.Prepared.Goal}, {questionsTitle, rec.Prepared.Questions}, {bringTitle, rec.Prepared.Bring},
		} {
			if strings.TrimSpace(section.text) != "" {
				b.WriteString("## " + section.title + "\n\n" + section.text + "\n\n")
			}
		}
	}
	b.WriteString("## " + sourcesTitle + "\n\n")
	for _, label := range labels {
		b.WriteString("- " + label + "\n")
	}
	b.WriteString("\n---\n\n" + footer + "\n")
	return b.String()
}
