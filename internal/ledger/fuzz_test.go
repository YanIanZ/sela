package ledger

import (
	"slices"
	"testing"
	"unicode/utf8"
)

var knownNotes = []string{
	NoteWordingNegation, NoteWordingSubject, NoteWordingTime, NoteWordingCertainty,
	NoteWordingNumber, NoteWordingUnsafe, NoteWordingTooLong, NoteWordingAddedContent, NoteQualifierFixed,
}

func FuzzParseDraft(f *testing.F) {
	f.Add([]byte(validDraft))
	f.Add([]byte("```json\n" + validDraft + "\n```"))
	f.Add([]byte(`{"claims":[]} trailing`))
	f.Add([]byte(`{"claims":[{"Quote":"x"}]}`))
	f.Add([]byte("{\"claims\":[{\"quote\":\"\xff\"}]}"))
	f.Add([]byte("```"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		d, err := ParseDraft(raw)
		if err != nil {
			return
		}
		if d.Claims == nil {
			t.Fatal("accepted draft without claims")
		}
		if len(raw) > MaxDraftBytes {
			t.Fatal("accepted oversize draft")
		}
		_, _ = ParseOpening(raw)
	})
}

func FuzzValidateDraft(f *testing.F) {
	f.Add("Aku tidak susah tidur. Kata temanku aku cuma butuh liburan.", "susah tidur", "experience", "sleeps badly", false)
	f.Add("Dulu aku sering cemas…", "aku sering cemas", "feeling", "sering cemas sekarang 3 kali", true)
	f.Add("Ｆｕｌｌ “quoted” text​ here", `full "quoted" text here`, "context", "", false)
	f.Add("bad \xff bytes \xc3 here", "� bytes", "context", "", false)
	f.Add("café enak", "café", "context", "café", false)
	f.Fuzz(func(t *testing.T, text, quote, kind, wording string, negated bool) {
		segs := append(SplitSegments(text, "story", 1), Segment{ID: "raw", Text: text, Origin: "story"})
		var claims []Candidate
		for _, s := range segs {
			for _, q := range []string{quote, s.Text, firstRunes(s.Text, 5)} {
				claims = append(claims, Candidate{SegmentID: s.ID, Quote: q, Kind: kind, Text: wording, Subject: "self", Time: "current", Certainty: "stated", Negated: negated})
				claims = append(claims, Candidate{SegmentID: s.ID, Quote: q, Kind: "context", Text: wording, Subject: "other"})
			}
		}
		r := ValidateDraft(Draft{Claims: claims, Clarification: &ClarificationCandidate{Field: kind}}, segs)
		if len(r.Accepted) > MaxClaims || len(r.Accepted)+len(r.Rejected) != len(claims) {
			t.Fatalf("accepted %d rejected %d of %d", len(r.Accepted), len(r.Rejected), len(claims))
		}
		byID := map[string]string{}
		for _, s := range segs {
			if _, ok := byID[s.ID]; !ok {
				byID[s.ID] = s.Text
			}
		}
		for _, c := range r.Accepted {
			st, ok := byID[c.SegmentID]
			if !ok {
				t.Fatalf("claim for unknown segment %q", c.SegmentID)
			}
			if c.Start < 0 || c.Start > c.End || c.End > len(st) {
				t.Fatalf("offsets %d:%d out of range %d", c.Start, c.End, len(st))
			}
			if st[c.Start:c.End] != c.Quote {
				t.Fatalf("quote %q is not text[%d:%d]", c.Quote, c.Start, c.End)
			}
			if !utf8.ValidString(c.Quote) {
				t.Fatalf("quote %q is not valid UTF-8", c.Quote)
			}
			if b := runeBoundaries(st); !b[c.Start] || !b[c.End] {
				t.Fatalf("offsets %d:%d split a rune", c.Start, c.End)
			}
			if r, size := utf8.DecodeRuneInString(st[c.Start:]); r == utf8.RuneError && size <= 1 {
				t.Fatalf("quote starts at an invalid byte")
			}
			if c.FromQuote != (c.Wording == c.Quote) && c.FromQuote {
				t.Fatalf("fromQuote wording %q != quote %q", c.Wording, c.Quote)
			}
			if !c.FromQuote && (runeLen(c.Wording) > MaxWordingRunes || ContainsDiagnosisLabel(c.Wording) || ContainsMedicalInstruction(c.Wording)) {
				t.Fatalf("unsafe model wording kept: %q", c.Wording)
			}
			if ContainsDiagnosisLabel(c.Quote) && (c.Kind != KindContext || !labelsAttributed(c.Quote)) {
				t.Fatalf("label accepted: %+v", c)
			}
			if !validKind(c.Kind) || (c.Qualifiers.Subject != SubjectSelf && c.Qualifiers.Subject != SubjectOther) {
				t.Fatalf("invalid claim %+v", c)
			}
			for _, n := range c.Notes {
				if !slices.Contains(knownNotes, n) {
					t.Fatalf("unknown note %q", n)
				}
			}
		}
		if r.Clarification != "" && !slices.Contains(r.Unknowns, r.Clarification) {
			t.Fatalf("clarification %q not unknown", r.Clarification)
		}
	})
}

func FuzzSplitSegments(f *testing.F) {
	f.Add("Aku capek. Kuliahku berantakan!\nHarus gimana?")
	f.Add("dr. Sari bilang… ok.")
	f.Add("\xff\xfe.")
	f.Fuzz(func(t *testing.T, text string) {
		segs := SplitSegments(text, "story", 1)
		if len(segs) > maxSegments {
			t.Fatalf("%d segments", len(segs))
		}
		for _, s := range segs {
			if !utf8.ValidString(s.Text) || s.Text == "" || runeLen(s.Text) > maxSegmentRunes {
				t.Fatalf("bad segment %q", s.Text)
			}
		}
	})
}

func FuzzValidateOpening(f *testing.F) {
	f.Add("Aku susah tidur dan sering cemas", "Saya susah tidur.", "I would like to talk.")
	f.Add("I can't sleep", "You must help me now", "Call 0812345678")
	f.Fuzz(func(t *testing.T, item, a, b string) {
		items := []ApprovedItem{{ID: "x", Kind: KindFeeling, Text: item}, {ID: "y", Kind: KindGoal, Text: a}}
		r := ValidateOpening(OpeningDraft{Lines: []string{a, b, item, a}}, items, "id")
		if len(r.Lines) > MaxOpeningLines || len(r.Lines)+len(r.Rejected) != 4 {
			t.Fatalf("result %+v", r)
		}
		for _, l := range r.Lines {
			if len(l.BasedOn) == 0 || runeLen(l.Text) > MaxOpeningRunes {
				t.Fatalf("line %+v", l)
			}
		}
		for _, lang := range []string{"id", "en"} {
			lines := TemplateOpening(items, lang)
			texts := make([]string, len(lines))
			for i, l := range lines {
				texts[i] = l.Text
			}
			if v := ValidateOpening(OpeningDraft{Lines: texts}, items, lang); len(v.Rejected) != 0 {
				t.Fatalf("template rejected: %+v for %q", v.Rejected, texts)
			}
		}
		_ = PracticeCoverage(b, items)
	})
}

// runeBoundaries returns the byte offsets where Go's UTF-8 decoder starts a
// rune (an invalid byte counts as a one-byte rune), plus len(s).
func runeBoundaries(s string) map[int]bool {
	b := map[int]bool{len(s): true}
	for i := 0; i < len(s); {
		b[i] = true
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return b
}

func firstRunes(s string, n int) string {
	for i := range s {
		if n == 0 {
			return s[:i]
		}
		n--
	}
	return s
}
