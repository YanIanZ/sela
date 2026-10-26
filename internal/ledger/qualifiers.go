package ledger

import (
	"strings"
	"unicode/utf8"
)

// negationWindow is how many tokens before a quote, within its clause, are
// searched for a governing negator.
const negationWindow = 3

// analysis is the deterministic reading of one span in its context.
type analysis struct {
	q Qualifiers
	// negBefore: a negator within negationWindow tokens before the span, in
	// the same clause. A quote that starts after it has dropped it.
	negBefore bool
	// negInside: a negator inside the span.
	negInside bool
}

// Analyze computes source qualifiers for seg.Text[start:end] in its clause
// context. Offsets are clamped to the text and moved to rune boundaries.
//
// Negated: a negator inside the span, or within three tokens before it in the
// same clause. Subject: other when reported speech from someone else covers
// the span ("kata temanku ...", "my friend said ...") or the nearest subject
// word names a third party; otherwise self. Time and Certainty: the first
// marker inside the span, else the nearest one before it in the same
// sentence region, else one after it in the same clause; otherwise
// unspecified and stated.
func Analyze(seg Segment, start, end int) Qualifiers {
	start, end = clampSpan(seg.Text, start, end)
	return analyze(tokenize(seg.Text), start, end).q
}

// clampSpan bounds [start, end) to s and to rune boundaries.
func clampSpan(s string, start, end int) (int, int) {
	start = max(0, min(start, len(s)))
	end = max(start, min(end, len(s)))
	for start > 0 && start < len(s) && !utf8.RuneStart(s[start]) {
		start--
	}
	for end < len(s) && !utf8.RuneStart(s[end]) {
		end++
	}
	return start, end
}

// tokenRange returns the indices [qs, qe) of tokens overlapping [start, end).
func tokenRange(toks []token, start, end int) (int, int) {
	qs := 0
	for qs < len(toks) && toks[qs].end <= start {
		qs++
	}
	qe := qs
	for qe < len(toks) && toks[qe].start < end {
		qe++
	}
	return qs, qe
}

func analyze(toks []token, start, end int) analysis {
	a := analysis{q: Qualifiers{Subject: SubjectSelf, Time: TimeUnspecified, Certainty: CertaintyStated}}
	qs, qe := tokenRange(toks, start, end)
	if qs >= qe {
		return a
	}
	for _, t := range toks[qs:qe] {
		if isNegator(t.text) {
			a.negInside = true
			break
		}
	}
	clause := toks[qs].clause
	for k := qs - 1; k >= 0 && k >= qs-negationWindow && toks[k].clause == clause; k-- {
		if isNegator(toks[k].text) {
			a.negBefore = true
			break
		}
	}
	a.q.Negated = a.negInside || a.negBefore
	a.q.Subject = subjectOf(toks, qs, qe)
	if tag := markerNear(timeLexicon, toks, qs, qe); tag != "" {
		a.q.Time = Time(tag)
	}
	if tag := markerNear(certaintyLexicon, toks, qs, qe); tag != "" {
		a.q.Certainty = Certainty(tag)
	}
	return a
}

// markerNear picks the lexicon tag that governs tokens [qs, qe).
func markerNear(lx *lexicon, toks []token, qs, qe int) string {
	region := toks[qs].region
	before, after := "", ""
	for _, h := range lx.scan(toks) {
		if toks[h.start].region != region {
			continue
		}
		switch {
		case h.start < qe && h.end > qs:
			return h.tag
		case h.end <= qs:
			before = h.tag // the last one before the span is the nearest
		case after == "" && toks[h.start].clause == toks[qe-1].clause:
			after = h.tag
		}
	}
	if before != "" {
		return before
	}
	return after
}

// Subject classes for tokens.
const (
	clsNone = iota
	clsSelf
	clsOther
)

// stripPossessive removes a trailing "-ku", "-mu" or "-nya".
func stripPossessive(t string) string {
	for _, suf := range []string{"nya", "ku", "mu"} {
		if strings.HasSuffix(t, suf) && len(t) > len(suf) {
			return strings.TrimSuffix(t, suf)
		}
	}
	return t
}

// isThirdNoun reports a noun naming someone else ("temanku", "teman-teman",
// "mother").
func isThirdNoun(t string) bool {
	if thirdPartyNouns[t] || thirdPartyNouns[stripPossessive(t)] {
		return true
	}
	if parts := wordParts(t); len(parts) > 1 {
		return thirdPartyNouns[parts[0]] || thirdPartyNouns[stripPossessive(parts[0])]
	}
	return false
}

// isOrangTua reports "orang tua(ku)" at toks[i].
func isOrangTua(toks []token, i int) bool {
	return toks[i].text == "orang" && i+1 < len(toks) && toks[i+1].region == toks[i].region &&
		strings.HasPrefix(toks[i+1].text, "tua")
}

// classify returns the subject class of toks[i].
func classify(toks []token, i int) int {
	t := toks[i].text
	if possessiveDeterminers[t] && i+1 < len(toks) && toks[i+1].clause == toks[i].clause &&
		(isThirdNoun(toks[i+1].text) || frameSpeakers[toks[i+1].text]) {
		return clsNone // "my friend": the noun is the subject word
	}
	if i > 0 && toks[i-1].text == "orang" && strings.HasPrefix(t, "tua") {
		return clsNone
	}
	if firstPerson[t] {
		if i > 0 && toks[i-1].clause == toks[i].clause && (thirdPartyNouns[toks[i-1].text] || (i > 1 && isOrangTua(toks, i-2))) {
			return clsNone // "teman saya", "ibu aku": possessive
		}
		return clsSelf
	}
	if thirdPartyPronouns[t] || isThirdNoun(t) || isOrangTua(toks, i) {
		return clsOther
	}
	if strings.HasSuffix(t, "ku") && utf8.RuneCountInString(t) >= 5 && !kuWords[t] && !strings.Contains(t, "-") {
		return clsSelf // "tidurku", "pikiranku": the user's own
	}
	return clsNone
}

// maxDeterminers bounds how many possessive determiners are skipped before a
// speaker noun ("my", "our"), so adversarial runs stay linear.
const maxDeterminers = 2

// isSpeaker reports whether toks[i] can be the speaker of a reported-speech
// frame, and returns the index just past the speaker phrase.
func isSpeaker(toks []token, i int) (bool, int) {
	for n := 0; n < maxDeterminers && i+1 < len(toks) && possessiveDeterminers[toks[i].text] &&
		toks[i+1].region == toks[i].region; n++ {
		i++
	}
	if i >= len(toks) {
		return false, i
	}
	t := toks[i].text
	switch {
	case isOrangTua(toks, i):
		return true, i + 2
	case thirdPartyPronouns[t] || isThirdNoun(t) || frameSpeakers[t] || t == "orang-orang":
		// "teman saya": the possessive pronoun belongs to the speaker phrase.
		if thirdPartyNouns[t] && i+1 < len(toks) && toks[i+1].clause == toks[i].clause && firstPerson[toks[i+1].text] && !possessiveDeterminers[toks[i+1].text] {
			return true, i + 2
		}
		return true, i + 1
	}
	return false, i
}

// frame is a reported-speech frame over token indices [start, end).
type frame struct{ start, end int }

// framesIn finds reported-speech frames in the region [rs, re).
func framesIn(toks []token, rs, re int) []frame {
	var out []frame
	for i := rs; i < re; i++ {
		t := toks[i].text
		switch t {
		case "katanya":
			out = append(out, frame{i, i + 1})
			continue
		case "kata", "menurut":
			if ok, e := isSpeaker(toks, i+1); ok && e <= re {
				out = append(out, frame{i, e})
				i = e - 1
				continue
			}
		case "according":
			if i+1 < re && toks[i+1].text == "to" {
				if ok, e := isSpeaker(toks, i+2); ok && e <= re {
					out = append(out, frame{i, e})
					i = e - 1
					continue
				}
			}
		}
		if ok, e := isSpeaker(toks, i); ok && classify(toks, i) != clsSelf {
			// Speaker followed by a speech verb, allowing two adverbs between
			// ("ibuku selalu bilang", "my friend always says").
			for j := e; j < re && j <= e+2; j++ {
				if speechVerbs[toks[j].text] {
					out = append(out, frame{i, j + 1})
					i = j
					break
				}
				if classify(toks, j) != clsNone {
					break
				}
			}
		}
	}
	return out
}

// subjectOf decides who tokens [qs, qe) are about.
func subjectOf(toks []token, qs, qe int) Subject {
	region := toks[qs].region
	rs, re := qs, qe
	for rs > 0 && toks[rs-1].region == region {
		rs--
	}
	for re < len(toks) && toks[re].region == region {
		re++
	}
	frames := framesIn(toks, rs, re)
	var clauseEnd []int // clauseEnd[k-rs]: index just past the clause of toks[k]
	firstSelf := -1     // first self-referring token at or after qs
	if len(frames) > 0 {
		clauseEnd = make([]int, re-rs)
		for k := re - 1; k >= rs; k-- {
			if k+1 < re && toks[k+1].clause == toks[k].clause {
				clauseEnd[k-rs] = clauseEnd[k+1-rs]
			} else {
				clauseEnd[k-rs] = k + 1
			}
		}
		for k := qs; k < qe; k++ {
			if classify(toks, k) == clsSelf {
				firstSelf = k
				break
			}
		}
	}
	for _, f := range frames {
		switch {
		case f.end <= qs:
			// Reported content is the clause that follows the frame: "kata
			// temanku aku cuma butuh liburan", "kata temanku, aku ...".
			scope := f.end
			if scope < re {
				scope = clauseEnd[scope-rs]
			}
			if qs < scope {
				return SubjectOther
			}
		case f.start < qe && f.end > qs:
			// The quote contains the frame, unless the user is its subject
			// before the frame ("aku sedih karena kata temanku ...").
			if firstSelf < 0 || firstSelf >= f.start {
				return SubjectOther
			}
		case f.start >= qe && f.end == re && f.start > 0 && toks[qe-1].clause == toks[f.start-1].clause:
			return SubjectOther // "<quote>, kata temanku."
		}
	}
	for k := qs; k < qe && k < qs+2; k++ {
		switch classify(toks, k) {
		case clsSelf:
			return SubjectSelf
		case clsOther:
			return SubjectOther
		}
	}
	for k := qs - 1; k >= rs; k-- {
		switch classify(toks, k) {
		case clsSelf:
			return SubjectSelf
		case clsOther:
			return SubjectOther
		}
	}
	return SubjectSelf
}
