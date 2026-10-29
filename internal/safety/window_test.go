package safety

import (
	"math/rand"
	"slices"
	"strings"
	"testing"
)

// TestWindowEquivalence checks windowed matching (see window.go) against
// matching whole sentences: every vector on its own with windows forced on
// every sentence, and long unpunctuated texts made of the vectors (a voice
// transcript), with the default threshold.
func TestWindowEquivalence(t *testing.T) {
	saved := windowMin
	defer func() { windowMin = saved }()
	assessWith := func(min int, s string) Assessment {
		windowMin = min
		return Assess(s)
	}
	same := func(a, b Assessment) bool {
		return a.Level == b.Level && slices.Equal(a.RuleIDs, b.RuleIDs) && a.ThirdParty == b.ThirdParty && a.Denial == b.Denial
	}
	vs := loadVectors(t)
	for i, v := range vs {
		whole, win := assessWith(1<<30, v.Text), assessWith(0, v.Text)
		if !same(whole, win) {
			t.Errorf("vector %d: whole %s %v, windowed %s %v", i, whole.Level, whole.RuleIDs, win.Level, win.RuleIDs)
		}
	}
	unpunct := strings.NewReplacer(".", " ", "!", " ", "?", " ")
	texts := make([]string, len(vs))
	for i, v := range vs {
		texts[i] = unpunct.Replace(v.Text)
	}
	rng := rand.New(rand.NewSource(6))
	for round := 0; round < 2; round++ {
		if round == 1 {
			rng.Shuffle(len(texts), func(i, j int) { texts[i], texts[j] = texts[j], texts[i] })
		}
		var b strings.Builder
		flush := func() {
			s := b.String()
			b.Reset()
			whole, win := assessWith(1<<30, s), assessWith(saved, s)
			if !same(whole, win) {
				t.Errorf("long text of %d bytes: whole %s %v, windowed %s %v", len(s), whole.Level, whole.RuleIDs, win.Level, win.RuleIDs)
			}
		}
		for _, s := range texts {
			b.WriteString(s)
			b.WriteByte(' ')
			if b.Len() > 2500 {
				flush()
			}
		}
		flush()
	}
}

// TestLenBound checks the match-length bound of every windowed pattern on
// its own matches in the vectors: no match is longer than maxLen says.
func TestLenBound(t *testing.T) {
	for _, v := range loadVectors(t) {
		for _, variant := range variants(v.Text) {
			for _, sp := range sentenceSpans(variant) {
				chunk := variant[sp.start:sp.end]
				sw := newSentenceWindows(chunk)
				for i := range catalogue {
					for _, p := range catalogue[i].patterns {
						if p.win.lens == nil {
							continue
						}
						ml := p.win.lens.maxLen(sw)
						for _, loc := range p.re.FindAllStringIndex(chunk, -1) {
							if loc[1]-loc[0] > ml {
								t.Errorf("%s: match of %d bytes, bound %d", catalogue[i].id, loc[1]-loc[0], ml)
							}
						}
					}
				}
			}
		}
	}
}
