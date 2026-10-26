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
	item    ApprovedItem
	content []string
	// negatedWords are content words within the negation window after a
	// negator in the item ("tidak bisa tidur" -> "tidur").
	negatedWords []string
}

// openingContext is the approved vocabulary a line may use.
type openingContext struct {
	items      []openingItem
	nums       map[string]bool // quantities in the items
	pct        bool            // a percent sign in the items
	labels     string          // safetyText of all item text, for diagnosis labels
	parts      map[string]bool // every word part in the items, for claim markers
	hasNegator bool
}

func newOpeningContext(items []ApprovedItem) *openingContext {
	ctx := &openingContext{parts: map[string]bool{}}
	texts := make([]string, 0, len(items))
	for _, it := range items {
		texts = append(texts, it.Text)
		oi := openingItem{item: it, content: contentWords(it.Text)}
		toks := tokenize(it.Text)
		for i, t := range toks {
			for _, p := range wordParts(t.text) {
				ctx.parts[p] = true
			}
			if !isNegator(t.text) {
				continue
			}
			ctx.hasNegator = true
			for k := i + 1; k < len(toks) && k <= i+negationWindow && toks[k].clause == t.clause; k++ {
				oi.negatedWords = append(oi.negatedWords, contentWords(toks[k].text)...)
			}
		}
		ctx.items = append(ctx.items, oi)
	}
	all := strings.Join(texts, "\n")
	ctx.nums, ctx.pct = quantities(all)
	ctx.labels = safetyText(all)
	return ctx
}

// lineNegators reports whether the line has a negator that states a fact,
// i.e. not a hedge such as "not sure" or "belum tahu".
func lineNegators(toks []token) bool {
	for i, t := range toks {
		if !isNegator(t.text) {
			continue
		}
		if i+1 < len(toks) && knowWords[toks[i+1].text] {
			continue
		}
		return true
	}
	return false
}

// check validates one whitespace-collapsed line and returns its basis.
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
	toks := tokenize(line)
	for _, t := range toks {
		for _, p := range wordParts(t.text) {
			if claimMarkers[p] && !ctx.parts[p] {
				return nil, OpeningUngrounded // adds a time, frequency or severity claim
			}
		}
	}
	negates := lineNegators(toks)
	if negates && !ctx.hasNegator {
		return nil, OpeningUngrounded // adds a negation
	}
	words := contentWords(line)
	matched := make([]bool, len(ctx.items))
	intent := false
	for _, w := range words {
		found := false
		for i, it := range ctx.items {
			hit := false
			for _, iw := range it.content {
				if sameWord(w, iw) {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
			matched[i], found = true, true
			if !negates {
				for _, nw := range it.negatedWords {
					if sameWord(w, nw) {
						return nil, OpeningUngrounded // drops the item's negation
					}
				}
			}
		}
		if found {
			continue
		}
		if isIntentWord(w) {
			intent = true
			continue
		}
		return nil, OpeningUngrounded
	}
	var basedOn []string
	for i, ok := range matched {
		if ok {
			basedOn = append(basedOn, ctx.items[i].item.ID)
		}
	}
	if len(basedOn) == 0 {
		if !intent {
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
// or guilt language, a time/frequency/severity claim or negation the items do
// not make, or any content word that is neither in an approved item
// (affix-insensitive) nor in the allowed intent vocabulary. BasedOn lists the
// items sharing a content word with the line, or IntentBasis for a line of
// intent words only. lang does not change the rules: users code-switch, so
// Indonesian and English lexicons always both apply.
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
// goal item, each value truncated at 150 runes on a word boundary. An item
// whose line would not pass ValidateOpening (for example one containing a
// medical instruction or an identifier) is skipped in favour of the next of
// the same kind. With nothing usable it returns one neutral line based on
// IntentBasis. Every returned line passes ValidateOpening for the same items.
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
