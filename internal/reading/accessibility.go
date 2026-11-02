package reading

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// Accessibility checks for the operator approval gate (sela-reading-a11y-v1).
//
// Every check is deterministic and computed from the exact embedded item, so
// the same content hash always yields the same report. A check is either
// blocking (a failure cannot be approved at all; the text must change) or
// overridable (a failure can be approved only with an explicit, audited
// reviewer reason). Readability is overridable on purpose: CONTENT_STYLE 3.2
// says the scores are a check, never a target to game by chopping sentences,
// and there is no validated Indonesian formula, so a human may decide that a
// 21-word sentence reads better than two fragments. Structural problems that
// a screen reader or a reader without colour vision cannot work around are
// blocking.
//
// The readability metrics reproduce sela-plain-language-v1 from
// tests/content/reading_validator.py (same word and sentence rules, same
// syllable heuristic, same thresholds), so the server report agrees with the
// authoring-time checks stored in each item.

// A11yChecksVersion names the automatic rule set. Changing any rule or
// threshold must change this string; the approve endpoint refuses a client
// that previewed a different version.
const A11yChecksVersion = "sela-reading-a11y-v1"

// A11yManualVersion names the manual checklist the reviewer confirms.
const A11yManualVersion = "sela-reading-a11y-manual-v1"

// Thresholds (CONTENT_STYLE 3.1 rule 1 and 3.2).
const (
	a11yMaxWordsPerSentence = 20
	a11yMaxAvgWords         = 15.0
	a11yMaxFKGrade          = 8.0
	a11yMaxBlockWords       = 80
	a11yMaxBlockSentences   = 5
	a11yMaxTitleWords       = 15
	a11yMaxHeadingWords     = 12
	a11yMaxSteps            = 10
)

// Check statuses and report results.
const (
	A11yPass          = "pass"
	A11yFail          = "fail"
	A11yNotApplicable = "not_applicable"

	A11yResultPass          = "pass"
	A11yResultNeedsOverride = "needs_override"
	A11yResultBlocked       = "blocked"
)

// A11yFinding is one machine-readable problem. Code is stable; the operator
// UI turns it into words. Path names the content field (as the authoring
// validator does), Value carries a count when one applies.
type A11yFinding struct {
	Code  string  `json:"code"`
	Path  string  `json:"path,omitempty"`
	Lang  string  `json:"lang,omitempty"`
	Value float64 `json:"value,omitempty"`
}

// A11yCheck is one automatic check.
type A11yCheck struct {
	ID       string        `json:"id"`
	Status   string        `json:"status"`
	Blocking bool          `json:"blocking"`
	Findings []A11yFinding `json:"findings"`
}

// A11yMetrics is the per-language readability summary.
type A11yMetrics struct {
	Sentences            int      `json:"sentences"`
	Words                int      `json:"words"`
	AvgWordsPerSentence  float64  `json:"avgWordsPerSentence"`
	MaxWordsPerSentence  int      `json:"maxWordsPerSentence"`
	SentencesOver20Words int      `json:"sentencesOver20Words"`
	FKGrade              *float64 `json:"fkGrade"`
}

// A11yReport is the full automatic result for one item.
type A11yReport struct {
	Version string                 `json:"version"`
	Result  string                 `json:"result"`
	Checks  []A11yCheck            `json:"checks"`
	Metrics map[string]A11yMetrics `json:"metrics"`
}

// Failing returns the ids of failing checks, split by kind.
func (r A11yReport) Failing() (blocking, overridable []string) {
	for _, c := range r.Checks {
		if c.Status != A11yFail {
			continue
		}
		if c.Blocking {
			blocking = append(blocking, c.ID)
		} else {
			overridable = append(overridable, c.ID)
		}
	}
	return blocking, overridable
}

// ManualCheck is one item of the reviewer's checklist.
type ManualCheck struct {
	ID    string    `json:"id"`
	Label Localized `json:"label"`
}

// ManualChecklist is what a human must confirm for every approval. These are
// the things a machine cannot judge (CONTENT_STYLE 3.2 asks for human
// readers; WCAG 1.3.3 and 2.4.6 need judgement about meaning).
var ManualChecklist = []ManualCheck{
	{
		ID: "read_aloud_plain_language",
		Label: Localized{
			ID: "Saya membaca teks Indonesia dan Inggris dengan suara keras. Kalimatnya mudah diikuti, istilah sulit dijelaskan, dan singkatan ditulis lengkap saat pertama muncul.",
			EN: "I read the Indonesian and English text aloud. The sentences are easy to follow, hard terms are explained and abbreviations are spelled out the first time.",
		},
	},
	{
		ID: "meaning_without_visuals",
		Label: Localized{
			ID: "Maknanya tetap jelas tanpa warna, bentuk, posisi, atau gambar. Setiap langkah dan judul bagian bisa dipahami jika dibacakan sendiri.",
			EN: "The meaning stays clear without colour, shape, position or images. Each step and section heading makes sense when read out on its own.",
		},
	},
	{
		ID: "names_and_order",
		Label: Localized{
			ID: "Judul, judul bagian, nama sumber, dan urutan baca pada pratinjau menggambarkan isinya dengan jelas.",
			EN: "The title, section headings, source names and reading order in the preview describe the content clearly.",
		},
	},
}

// ManualCheckIDs lists the checklist ids in order.
func ManualCheckIDs() []string {
	out := make([]string, len(ManualChecklist))
	for i, c := range ManualChecklist {
		out[i] = c.ID
	}
	return out
}

var (
	a11yWord      = regexp.MustCompile(`[0-9A-Za-z\x{00C0}-\x{024F}]+(?:['’-][0-9A-Za-z\x{00C0}-\x{024F}]+)*`)
	a11yNonAlpha  = regexp.MustCompile(`[^a-z]`)
	a11ySilentEnd = regexp.MustCompile(`(?:[^laeiouy]es|ed|[^laeiouy]e)$`)
	a11yLeadingY  = regexp.MustCompile(`^y`)
	a11yVowels    = regexp.MustCompile(`[aeiouy]{1,2}`)

	a11yBareURL     = regexp.MustCompile(`(?i)\bhttps?://\S+|\bwww\.\S+`)
	a11yMarkdownLnk = regexp.MustCompile(`(^|[^!])\[([^\]]*)\]\(([^)]*)\)`)
	a11yHTMLAnchor  = regexp.MustCompile(`(?is)<a\b[^>]*>(.*?)</a>`)
	a11yGenericLink = regexp.MustCompile(`(?i)\b(klik di sini|tekan di sini|ketuk di sini|baca selengkapnya|selengkapnya|click here|tap here|read more|learn more|more info)\b`)
	a11yMarkdownImg = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]*)\)`)
	a11yHTMLImg     = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	a11yImgAlt      = regexp.MustCompile(`(?is)\balt\s*=\s*("([^"]*)"|'([^']*)')`)
	a11yTags        = regexp.MustCompile(`(?s)<[^>]*>`)

	a11yStepNumber = regexp.MustCompile(`^\s*(?:\(?\d+[.):]|[-•*–])\s`)

	a11yQuoted = regexp.MustCompile(`["“][^"”]{2,}["”]|(?:^|\s)'[^']{2,}'`)
	// "berwarna", or a colour named as the identifying property ("warna
	// merah", "coloured green", "in red"). "tema warna" alone is not.
	a11yColour = regexp.MustCompile(`(?i)\bberwarna\b|\bwarna\s+(?:merah|hijau|biru|kuning|oranye|jingga|ungu|abu-abu|hitam|putih)\b|\b(?:colou?red|in)\s+(?:red|green|blue|yellow|orange|purple|grey|gray|black|white)\b`)
)

// Words near a control name that make an instruction depend on sight.
var (
	a11yUINouns = map[string]bool{
		"tombol": true, "ikon": true, "tautan": true, "kotak": true, "kolom": true, "panel": true, "menu": true,
		"button": true, "buttons": true, "icon": true, "icons": true, "link": true, "box": true,
	}
	a11ySensory = map[string]bool{
		// colour
		"merah": true, "hijau": true, "biru": true, "kuning": true, "oranye": true, "jingga": true, "ungu": true,
		"abu-abu": true, "hitam": true, "putih": true,
		"red": true, "green": true, "blue": true, "yellow": true, "orange": true, "purple": true, "grey": true,
		"gray": true, "black": true, "white": true,
		// shape
		"bulat": true, "bundar": true, "segitiga": true, "persegi": true, "lingkaran": true, "bintang": true,
		"round": true, "circle": true, "circular": true, "square": true, "triangle": true, "triangular": true,
		// position
		"kanan": true, "kiri": true, "pojok": true, "sudut": true,
		"right": true, "left": true, "corner": true, "top": true, "bottom": true,
	}
)

const a11yNearWords = 4

// a11yText is one user-facing text with its validator-style path.
type a11yText struct {
	path, lang, text string
	body             bool // counted for readability (not a title/heading)
	kind             string
}

func (it item) a11yTexts() []a11yText {
	c := it.Content
	var out []a11yText
	add := func(path string, body bool, kind string, v *Localized) {
		if v == nil {
			return
		}
		out = append(out,
			a11yText{path: path + ".id", lang: "id", text: v.ID, body: body, kind: kind},
			a11yText{path: path + ".en", lang: "en", text: v.EN, body: body, kind: kind})
	}
	add("content.title", false, "title", &c.Title)
	add("content.summary", true, "block", &c.Summary)
	add("content.why", true, "block", c.Why)
	add("content.seek_help_when", true, "block", c.SeekHelpWhen)
	add("content.cautions", true, "block", c.Cautions)
	if c.Steps != nil {
		for _, lang := range []string{"id", "en"} {
			list := c.Steps.ID
			if lang == "en" {
				list = c.Steps.EN
			}
			for i, s := range list {
				out = append(out, a11yText{path: fmt.Sprintf("content.steps.%s[%d]", lang, i), lang: lang, text: s, body: true, kind: "step"})
			}
		}
	}
	for si, sec := range c.Sections {
		h := sec.Heading
		add(fmt.Sprintf("content.sections[%d].heading", si), false, "heading", &h)
		for pi, p := range sec.Paragraphs {
			pl := Localized{ID: p.ID, EN: p.EN}
			add(fmt.Sprintf("content.sections[%d].paragraphs[%d]", si, pi), true, "block", &pl)
		}
	}
	return out
}

func a11yWords(text string) []string { return a11yWord.FindAllString(text, -1) }

// a11ySentences splits like the validator: after . ! or ?, optional closing
// quotes or brackets, then whitespace. Fragments without words are dropped.
func a11ySentences(text string) []string {
	text = strings.TrimSpace(text)
	var out []string
	rs := []rune(text)
	start := 0
	for i := 0; i < len(rs); i++ {
		if rs[i] != '.' && rs[i] != '!' && rs[i] != '?' {
			continue
		}
		j := i + 1
		for j < len(rs) && strings.ContainsRune("\"'”’)", rs[j]) {
			j++
		}
		k := j
		for k < len(rs) && (rs[k] == ' ' || rs[k] == '\t' || rs[k] == '\n' || rs[k] == '\r' || rs[k] == ' ') {
			k++
		}
		if k == j {
			continue // no whitespace after the mark: not a boundary
		}
		out = append(out, string(rs[start:j]))
		start = k
		i = k - 1
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	kept := out[:0]
	for _, s := range out {
		if len(a11yWords(s)) > 0 {
			kept = append(kept, s)
		}
	}
	return kept
}

// a11ySyllables is the validator's English heuristic, approximate by design.
func a11ySyllables(word string) int {
	w := a11yNonAlpha.ReplaceAllString(strings.ToLower(word), "")
	if w == "" || len(w) <= 3 {
		return 1
	}
	w = a11ySilentEnd.ReplaceAllString(w, "")
	w = a11yLeadingY.ReplaceAllString(w, "")
	n := len(a11yVowels.FindAllString(w, -1))
	if n < 1 {
		return 1
	}
	return n
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }

// CheckAccessibility computes the report for one item.
func (it item) checkAccessibility(sources map[string]sourceEntry) A11yReport {
	texts := it.a11yTexts()
	r := A11yReport{Version: A11yChecksVersion, Metrics: map[string]A11yMetrics{}}

	r.Checks = append(r.Checks,
		it.checkLanguageParity(texts),
		it.checkHeadings(),
		it.checkSteps(),
		checkLinks(texts, it.Source.SourceIDs, sources),
		checkImages(texts),
		checkSensory(texts),
	)
	sentence, readability, blocks := checkReadability(texts, r.Metrics)
	r.Checks = append(r.Checks, sentence, readability, blocks)

	blocking, overridable := r.Failing()
	switch {
	case len(blocking) > 0:
		r.Result = A11yResultBlocked
	case len(overridable) > 0:
		r.Result = A11yResultNeedsOverride
	default:
		r.Result = A11yResultPass
	}
	return r
}

func newCheck(id string, blocking bool) A11yCheck {
	return A11yCheck{ID: id, Blocking: blocking, Status: A11yPass, Findings: []A11yFinding{}}
}

func (c *A11yCheck) fail(f A11yFinding) {
	c.Status = A11yFail
	c.Findings = append(c.Findings, f)
}

// language_parity: every text exists in both languages, so the lang-tagged
// preview and the public page never fall back to an untagged blank.
func (it item) checkLanguageParity(texts []a11yText) A11yCheck {
	c := newCheck("language_parity", true)
	for _, t := range texts {
		if strings.TrimSpace(t.text) == "" {
			c.fail(A11yFinding{Code: "empty_text", Path: t.path, Lang: t.lang})
		}
	}
	if s := it.Content.Steps; s != nil && len(s.ID) != len(s.EN) {
		c.fail(A11yFinding{Code: "step_count_differs", Path: "content.steps", Value: float64(len(s.ID) - len(s.EN))})
	}
	return c
}

// heading_structure: a short, unique, descriptive heading for every section,
// no empty sections, and guides organised by headings (WCAG 1.3.1, 2.4.6).
func (it item) checkHeadings() A11yCheck {
	c := newCheck("heading_structure", true)
	title := it.Content.Title
	for _, lang := range []string{"id", "en"} {
		t := pick(title, lang)
		if n := len(a11yWords(t)); n > a11yMaxTitleWords {
			c.fail(A11yFinding{Code: "title_too_long", Path: "content.title." + lang, Lang: lang, Value: float64(n)})
		}
	}
	if it.Kind == "guide" && len(it.Content.Sections) == 0 {
		c.fail(A11yFinding{Code: "guide_without_sections", Path: "content.sections"})
	}
	seen := map[string]bool{}
	for si, sec := range it.Content.Sections {
		for _, lang := range []string{"id", "en"} {
			path := fmt.Sprintf("content.sections[%d].heading.%s", si, lang)
			h := strings.TrimSpace(pick(sec.Heading, lang))
			if h == "" {
				continue // reported by language_parity
			}
			if n := len(a11yWords(h)); n > a11yMaxHeadingWords {
				c.fail(A11yFinding{Code: "heading_too_long", Path: path, Lang: lang, Value: float64(n)})
			}
			key := lang + ":" + strings.ToLower(h)
			if seen[key] {
				c.fail(A11yFinding{Code: "heading_duplicate", Path: path, Lang: lang})
			}
			seen[key] = true
			if strings.EqualFold(h, strings.TrimSpace(pick(title, lang))) {
				c.fail(A11yFinding{Code: "heading_repeats_title", Path: path, Lang: lang})
			}
		}
		if len(sec.Paragraphs) == 0 {
			c.fail(A11yFinding{Code: "section_without_text", Path: fmt.Sprintf("content.sections[%d]", si)})
		}
	}
	return c
}

// list_structure: steps are a real ordered list, so they carry no typed
// numbers or bullets (a screen reader would announce "1. 1.").
func (it item) checkSteps() A11yCheck {
	c := newCheck("list_structure", true)
	s := it.Content.Steps
	if s == nil {
		if it.Kind == "exercise" || it.Kind == "safety_plan" {
			c.fail(A11yFinding{Code: "steps_missing", Path: "content.steps"})
			return c
		}
		c.Status = A11yNotApplicable
		return c
	}
	for _, lang := range []string{"id", "en"} {
		list := s.ID
		if lang == "en" {
			list = s.EN
		}
		if len(list) > a11yMaxSteps {
			c.fail(A11yFinding{Code: "too_many_steps", Path: "content.steps." + lang, Lang: lang, Value: float64(len(list))})
		}
		for i, step := range list {
			if a11yStepNumber.MatchString(step) {
				c.fail(A11yFinding{Code: "typed_list_marker", Path: fmt.Sprintf("content.steps.%s[%d]", lang, i), Lang: lang})
			}
		}
	}
	return c
}

// link_text: no bare URLs or generic link words in the reading text, and
// every cited source has a real title for its link (WCAG 2.4.4). Attribution
// and licence notices are not scanned for bare URLs: the CC BY-NC-SA licence
// requires the URI verbatim.
func checkLinks(texts []a11yText, sourceIDs []string, sources map[string]sourceEntry) A11yCheck {
	c := newCheck("link_text", true)
	seenAny := false
	for _, t := range texts {
		if a11yBareURL.MatchString(stripMarkup(t.text)) {
			seenAny = true
			c.fail(A11yFinding{Code: "bare_url", Path: t.path, Lang: t.lang})
		}
		for _, m := range a11yMarkdownLnk.FindAllStringSubmatch(t.text, -1) {
			seenAny = true
			if !descriptiveLinkText(m[2]) {
				c.fail(A11yFinding{Code: "link_text_not_descriptive", Path: t.path, Lang: t.lang})
			}
		}
		for _, m := range a11yHTMLAnchor.FindAllStringSubmatch(t.text, -1) {
			seenAny = true
			if !descriptiveLinkText(a11yTags.ReplaceAllString(m[1], "")) {
				c.fail(A11yFinding{Code: "link_text_not_descriptive", Path: t.path, Lang: t.lang})
			}
		}
		if a11yGenericLink.MatchString(t.text) {
			c.fail(A11yFinding{Code: "generic_link_phrase", Path: t.path, Lang: t.lang})
		}
	}
	for i, id := range sourceIDs {
		seenAny = true
		s, ok := sources[id]
		path := fmt.Sprintf("source.source_ids[%d]", i)
		if !ok {
			c.fail(A11yFinding{Code: "source_unknown", Path: path})
			continue
		}
		if !descriptiveLinkText(s.Title) {
			c.fail(A11yFinding{Code: "source_title_not_descriptive", Path: path})
		}
	}
	if !seenAny {
		c.Status = A11yNotApplicable
	}
	return c
}

// a11yGenericOnly are link texts that say nothing out of context.
var a11yGenericOnly = map[string]bool{
	"di sini": true, "sini": true, "ini": true, "tautan": true, "tautan ini": true, "klik": true,
	"here": true, "this": true, "link": true, "this link": true, "click": true, "more": true,
}

func descriptiveLinkText(text string) bool {
	t := strings.TrimSpace(text)
	if len(a11yWords(t)) == 0 || a11yBareURL.MatchString(t) {
		return false
	}
	norm := strings.ToLower(strings.Join(a11yWords(t), " "))
	if a11yGenericOnly[norm] {
		return false
	}
	return !a11yGenericLink.MatchString(t) || len(a11yWords(t)) > 4
}

func stripMarkup(text string) string {
	text = a11yMarkdownLnk.ReplaceAllString(text, "$1$2")
	return a11yHTMLAnchor.ReplaceAllString(text, "$1")
}

// image_alt: any image must carry a text alternative (WCAG 1.1.1). The v1
// content schema has no images, so this is normally not applicable; it
// exists so an image added by hand cannot slip through.
func checkImages(texts []a11yText) A11yCheck {
	c := newCheck("image_alt", true)
	seen := false
	for _, t := range texts {
		for _, m := range a11yMarkdownImg.FindAllStringSubmatch(t.text, -1) {
			seen = true
			if strings.TrimSpace(m[1]) == "" {
				c.fail(A11yFinding{Code: "image_without_alt", Path: t.path, Lang: t.lang})
			}
		}
		for _, tag := range a11yHTMLImg.FindAllString(t.text, -1) {
			seen = true
			alt := a11yImgAlt.FindStringSubmatch(tag)
			if alt == nil || strings.TrimSpace(alt[2]+alt[3]) == "" {
				c.fail(A11yFinding{Code: "image_without_alt", Path: t.path, Lang: t.lang})
			}
		}
	}
	if !seen {
		c.Status = A11yNotApplicable
	}
	return c
}

// sensory_instructions: an instruction must not rely only on colour, shape
// or position (WCAG 1.3.3, 1.4.1). A sentence that names a control by its
// quoted label passes; "tekan tombol hijau di kanan" does not.
func checkSensory(texts []a11yText) A11yCheck {
	c := newCheck("sensory_instructions", true)
	for _, t := range texts {
		for _, s := range a11ySentences(t.text) {
			if a11yQuoted.MatchString(s) {
				continue
			}
			if a11yColour.MatchString(s) || nearUINoun(s) {
				c.fail(A11yFinding{Code: "sensory_only_instruction", Path: t.path, Lang: t.lang})
				break
			}
		}
	}
	return c
}

func nearUINoun(sentence string) bool {
	ws := a11yWords(strings.ToLower(sentence))
	for i, w := range ws {
		if !a11yUINouns[w] {
			continue
		}
		lo, hi := max(0, i-a11yNearWords), min(len(ws)-1, i+a11yNearWords)
		for j := lo; j <= hi; j++ {
			if j != i && a11ySensory[ws[j]] {
				return true
			}
		}
	}
	return false
}

// checkReadability returns sentence_length, readability and block_length,
// all overridable, and fills the per-language metrics.
func checkReadability(texts []a11yText, metrics map[string]A11yMetrics) (A11yCheck, A11yCheck, A11yCheck) {
	sentence := newCheck("sentence_length", false)
	readability := newCheck("readability", false)
	blocks := newCheck("block_length", false)
	for _, lang := range []string{"id", "en"} {
		var lens []int
		syll := 0
		for _, t := range texts {
			if t.lang != lang || !t.body {
				continue
			}
			sents := a11ySentences(t.text)
			if n := len(a11yWords(t.text)); n > a11yMaxBlockWords {
				blocks.fail(A11yFinding{Code: "block_over_80_words", Path: t.path, Lang: lang, Value: float64(n)})
			}
			if len(sents) > a11yMaxBlockSentences {
				blocks.fail(A11yFinding{Code: "block_over_5_sentences", Path: t.path, Lang: lang, Value: float64(len(sents))})
			}
			for _, s := range sents {
				ws := a11yWords(s)
				lens = append(lens, len(ws))
				if len(ws) > a11yMaxWordsPerSentence {
					sentence.fail(A11yFinding{Code: "sentence_over_20_words", Path: t.path, Lang: lang, Value: float64(len(ws))})
				}
				if lang == "en" {
					for _, w := range ws {
						syll += a11ySyllables(w)
					}
				}
			}
		}
		m := A11yMetrics{Sentences: len(lens)}
		for _, n := range lens {
			m.Words += n
			if n > m.MaxWordsPerSentence {
				m.MaxWordsPerSentence = n
			}
			if n > a11yMaxWordsPerSentence {
				m.SentencesOver20Words++
			}
		}
		if m.Sentences > 0 {
			m.AvgWordsPerSentence = round1(float64(m.Words) / float64(m.Sentences))
			if m.AvgWordsPerSentence > a11yMaxAvgWords {
				readability.fail(A11yFinding{Code: "avg_words_per_sentence_over_15", Lang: lang, Value: m.AvgWordsPerSentence})
			}
			if lang == "en" && m.Words > 0 {
				fk := round1(0.39*float64(m.Words)/float64(m.Sentences) + 11.8*float64(syll)/float64(m.Words) - 15.59)
				m.FKGrade = &fk
				if fk > a11yMaxFKGrade {
					readability.fail(A11yFinding{Code: "fk_grade_over_8", Lang: "en", Value: fk})
				}
			}
		}
		metrics[lang] = m
	}
	return sentence, readability, blocks
}

func pick(l Localized, lang string) string {
	if lang == "en" {
		return l.EN
	}
	return l.ID
}
