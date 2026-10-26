package ledger

import (
	"strings"
	"unicode/utf8"
)

// Opening constants.
const (
	// IntentBasis is the BasedOn marker for a line made only of the allowed
	// intent vocabulary ("I would like to talk to a psychologist").
	IntentBasis = "intent"
	// OpeningDuplicate rejects a line that repeats an accepted line.
	OpeningDuplicate OpeningRejectReason = "duplicate"
	// OpeningInvalidText rejects a line with control or bidi characters.
	OpeningInvalidText OpeningRejectReason = "invalid_text"

	templateValueRunes = 150
)

// intentStems are the stems of intentVocab, so inflected and cliticised
// forms ("memulainya", "started") count as intent words without the loose
// substring rule used for approved items.
var intentStems = func() map[string]bool {
	m := make(map[string]bool, len(intentVocab))
	for w := range intentVocab {
		m[stem(w)] = true
	}
	return m
}()

func isIntentWord(w string) bool { return intentVocab[w] || intentStems[stem(w)] }

// openingItem is one approved item prepared for line checks.
type openingItem struct {
	item ApprovedItem
	// words are the item's content words, each with the number of negators
	// governing it ("tidak bisa tidur" -> "tidur": 1).
	words []polarWord
}

// openingContext is the approved vocabulary a line may use.
type openingContext struct {
	items  []openingItem
	nums   map[string]bool // quantities in the items
	pct    bool            // a percent sign in the items
	labels string          // safetyText of all item text, for diagnosis labels
	parts  map[string]bool // every word part in the items, for claim markers
}

func newOpeningContext(items []ApprovedItem) *openingContext {
	ctx := &openingContext{parts: map[string]bool{}}
	texts := make([]string, 0, len(items))
	for _, it := range items {
		texts = append(texts, it.Text)
		toks := tokenize(it.Text)
		for _, t := range toks {
			for _, p := range wordParts(t.text) {
				ctx.parts[p] = true
			}
		}
		ctx.items = append(ctx.items, openingItem{item: it, words: polarWords(toks, negationCounts(toks, true), 0, len(toks))})
	}
	all := strings.Join(texts, "\n")
	ctx.nums, ctx.pct = quantities(all)
	ctx.labels = safetyText(all)
	return ctx
}

// Grounding outcomes for one line word.
const (
	groundNone    = iota // no approved item has the word
	groundItem           // an item has it with the same negation
	groundFlipped        // items have it, but only with other negation
)

// ground looks up word p, governed by neg negators in the line, in the
// approved items and marks the items that ground it.
func (ctx *openingContext) ground(p string, neg int, matched []bool) int {
	res := groundNone
	for i, it := range ctx.items {
		for _, w := range it.words {
			if !sameWord(p, w.word) {
				continue
			}
			if w.neg == neg {
				matched[i] = true
				res = groundItem
				break
			}
			if res == groundNone {
				res = groundFlipped
			}
		}
	}
	return res
}

// desireBefore reports a desire word one or two tokens before toks[i] in its
// clause ("ingin bercerita", "would like to talk").
func desireBefore(toks []token, i int) bool {
	for k := i - 1; k >= 0 && k >= i-2 && toks[k].clause == toks[i].clause; k-- {
		if desireWords[toks[k].text] {
			return true
		}
	}
	return false
}

// frameBody returns the part of line after a TemplateOpening frame and
// whether line starts with one; the neutral line is a frame with no body.
// Frames are Sela's own fixed text, so only the body is checked against the
// approved items (safety checks always cover the whole line).
func frameBody(line string) (string, bool) {
	for _, lang := range []string{"id", "en"} {
		f := templateFrames[lang]
		if line == f.neutral {
			return "", true
		}
		for _, fr := range []string{f.concern, f.impact, f.goal} {
			if prefix := strings.TrimSuffix(fr, "%s"); strings.HasPrefix(line, prefix) {
				return line[len(prefix):], true
			}
		}
	}
	return line, false
}

// check validates one whitespace-collapsed line and returns its basis.
//
// Negation is compared word by word: a line word must occur in an approved
// item governed by as many negators as in the line. A line therefore cannot
// keep one item's negator while dropping another's ("saya tidak bisa tidur
// dan ingin bunuh diri" against "aku tidak ingin bunuh diri"), nor negate a
// word an item states plainly. A hedge ("belum yakin", "not sure") negates
// only its knowing word. The one exception is a request: an intent word after
// a desire word ("ingin bercerita") may differ from an item that negates it
// ("belum pernah cerita").
func (ctx *openingContext) check(line string) ([]string, OpeningRejectReason) {
	if hasControl(line) || !utf8.ValidString(line) {
		return nil, OpeningInvalidText
	}
	if utf8.RuneCountInString(line) > MaxOpeningRunes {
		return nil, OpeningTooLong
	}
	if containsIdentifier(line) {
		return nil, OpeningIdentifier
	}
	for _, label := range diagnosisLabelRe.FindAllString(safetyText(line), -1) {
		if !strings.Contains(ctx.labels, label) {
			return nil, OpeningDiagnosis
		}
	}
	if ContainsMedicalInstruction(line) {
		return nil, OpeningMedical
	}
	if pressureLexicon.has(line) {
		return nil, OpeningManipulative
	}
	if addsQuantity(line, ctx.nums, ctx.pct) {
		return nil, OpeningNumber
	}
	body, framed := frameBody(line)
	toks := tokenize(body)
	for _, t := range toks {
		for _, p := range wordParts(t.text) {
			if claimMarkers[p] && !ctx.parts[p] {
				return nil, OpeningUngrounded // adds a time, frequency or severity claim
			}
		}
	}
	counts := negationCounts(toks, true)
	matched := make([]bool, len(ctx.items))
	intent := false
	for i, t := range toks {
		if isNegator(t.text) {
			continue
		}
		for _, p := range wordParts(t.text) {
			if !isContent(p) {
				continue
			}
			switch ctx.ground(p, counts[i], matched) {
			case groundItem:
				continue
			case groundFlipped:
				if counts[i] == 0 && isIntentWord(p) && desireBefore(toks, i) {
					intent = true
					continue
				}
				return nil, OpeningUngrounded // changes an item's negation
			}
			if isIntentWord(p) && (counts[i] == 0 || knowWords[p]) {
				intent = true
				continue
			}
			return nil, OpeningUngrounded // a word no item has, or a negation no item makes
		}
	}
	var basedOn []string
	for i, ok := range matched {
		if ok {
			basedOn = append(basedOn, ctx.items[i].item.ID)
		}
	}
	if len(basedOn) == 0 {
		if !intent && !framed {
			return nil, OpeningUngrounded
		}
		basedOn = []string{IntentBasis}
	}
	return basedOn, ""
}

// ValidateOpening keeps only grounded, safe opening lines, in order, at most
// MaxOpeningLines (callers treat zero kept lines as a failed draft and fall
// back to TemplateOpening). Whitespace is collapsed. A line is rejected when
// it is empty, beyond the limit, a repeat of a kept line (OpeningDuplicate),
// has control or bidi characters (OpeningInvalidText), is too long, or contains
// an identifier (URL, e-mail, @handle, phone-like number), a diagnosis label
// or number absent from the approved items, a medical instruction, pressure
// or guilt language, a time/frequency/severity claim the items do not make, a
// word whose negation differs from every approved item that has it, or any
// content word that is neither in an approved item (affix-insensitive) nor in
// the allowed intent vocabulary. BasedOn lists the items sharing a content
// word (with the same negation) with the line, or IntentBasis for a line of
// intent words only. A line that starts with a TemplateOpening frame has
// only its quoted part checked against the items. lang does not change the
// rules: users code-switch, so Indonesian and English lexicons always both
// apply.
func ValidateOpening(d OpeningDraft, items []ApprovedItem, lang string) OpeningResult {
	_ = lang
	ctx := newOpeningContext(items)
	res := OpeningResult{Lines: []OpeningLine{}, Rejected: []OpeningRejection{}}
	seen := map[string]bool{}
	for i, raw := range d.Lines {
		line := strings.Join(strings.Fields(raw), " ")
		reason := OpeningRejectReason("")
		var basedOn []string
		switch {
		case line == "":
			reason = OpeningEmpty
		case len(res.Lines) >= MaxOpeningLines:
			reason = OpeningLimit
		default:
			basedOn, reason = ctx.check(line)
		}
		if reason == "" {
			key := normalizeForGrounding(line)
			if seen[key] {
				reason = OpeningDuplicate
			} else {
				seen[key] = true
				res.Lines = append(res.Lines, OpeningLine{Text: line, BasedOn: basedOn})
				continue
			}
		}
		res.Rejected = append(res.Rejected, OpeningRejection{Index: i, Reason: reason})
	}
	return res
}

// clipValue collapses whitespace, drops trailing sentence punctuation and
// truncates at templateValueRunes runes on a word boundary.
func clipValue(s string) (string, bool) {
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimRight(s, ".!?…。 ")
	r := []rune(s)
	if len(r) <= templateValueRunes {
		return s, false
	}
	cut := string(r[:templateValueRunes])
	if i := strings.LastIndexByte(cut, ' '); i > 0 && utf8.RuneCountInString(cut[:i]) > 40 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, ",;: "), true
}

// templateFrames are the deterministic sentence frames per language.
var templateFrames = map[string]struct{ concern, impact, goal, neutral string }{
	"id": {
		concern: "Saya ingin membicarakan hal yang sedang saya alami: %s",
		impact:  "Ini mulai berpengaruh pada %s",
		goal:    "Yang saya harapkan dari pertemuan ini: %s",
		neutral: "Saya belum yakin bagaimana memulainya, jadi saya membawa catatan ini.",
	},
	"en": {
		concern: "I would like to talk about what I have been going through: %s",
		impact:  "It has started to affect %s",
		goal:    "What I hope for from this meeting: %s",
		neutral: "I am not sure how to start, so I brought this note.",
	},
}

// TemplateOpening builds deterministic opening lines without a model, in
// "id" (default) or "en": one line quoting the first feeling, experience or
// worry item, one quoting the first impact item and one quoting the first
// goal item, each value truncated at 150 runes on a word boundary. The frame
// words never count against the items; an item whose line would not pass
// ValidateOpening (for example one containing a medical instruction or an
// identifier) is skipped in favour of the next of the same kind. With nothing
// usable it returns one neutral line based on IntentBasis. Every returned
// line passes ValidateOpening for the same items.
func TemplateOpening(items []ApprovedItem, lang string) []OpeningLine {
	frames := templateFrames["id"]
	if strings.HasPrefix(lowerTrim(lang), "en") {
		frames = templateFrames["en"]
	}
	ctx := newOpeningContext(items)
	var lines []OpeningLine
	seen := map[string]bool{}
	build := func(frame string, kinds ...Kind) {
		for _, it := range items {
			if !hasKind(kinds, it.Kind) || seen[it.ID] {
				continue
			}
			value, clipped := clipValue(it.Text)
			if value == "" {
				continue
			}
			end := "."
			if clipped {
				end = "..."
			}
			line := strings.Replace(frame, "%s", value, 1) + end
			if _, reason := ctx.check(line); reason != "" {
				continue
			}
			seen[it.ID] = true
			lines = append(lines, OpeningLine{Text: line, BasedOn: []string{it.ID}})
			return
		}
	}
	build(frames.concern, KindFeeling, KindExperience, KindWorry)
	build(frames.impact, KindImpact)
	build(frames.goal, KindGoal)
	if len(lines) == 0 {
		lines = append(lines, OpeningLine{Text: frames.neutral, BasedOn: []string{IntentBasis}})
	}
	return lines
}

func hasKind(kinds []Kind, k Kind) bool {
	for _, v := range kinds {
		if v == k {
			return true
		}
	}
	return false
}

// PracticeCoverage reports which approved items a practice attempt mentions,
// in input order. An item with fewer than four content words is mentioned
// when the attempt shares at least one of them (affix-insensitive); an item
// with four or more needs at least half of them. Only the first
// MaxPracticeRunes runes of the attempt are read. It is never a score.
func PracticeCoverage(attempt string, items []ApprovedItem) Coverage {
	if r := []rune(attempt); len(r) > MaxPracticeRunes {
		attempt = string(r[:MaxPracticeRunes])
	}
	said := contentWords(attempt)
	cov := Coverage{Mentioned: []string{}, NotMentioned: []string{}}
	for _, it := range items {
		words := contentWords(it.Text)
		shared := 0
		for _, w := range words {
			for _, a := range said {
				if sameWord(w, a) {
					shared++
					break
				}
			}
		}
		mentioned := shared >= 1
		if len(words) >= 4 {
			mentioned = shared*2 >= len(words)
		}
		if mentioned {
			cov.Mentioned = append(cov.Mentioned, it.ID)
		} else {
			cov.NotMentioned = append(cov.NotMentioned, it.ID)
		}
	}
	return cov
}
