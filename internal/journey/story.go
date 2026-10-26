package journey

import (
	"strings"
	"unicode/utf8"

	"github.com/YanIanZ/sela/internal/ledger"
	"github.com/YanIanZ/sela/internal/safety"
)

// Practice-text bounds (SPEC V2, amended by D-003).
const (
	MinPracticeRunes = 20
	MaxPracticeRunes = 3000
)

// Per-session bounds on what reaches the model (SPEC V9, V19): the story with
// its clarification answers, and the ledger items (CodeLimitReached).
const (
	MaxStoryRunes    = MaxPracticeRunes + 5*ledger.MaxEditRunes // one answer per core field
	MaxStorySegments = 60
	MaxLedgerItems   = ledger.MaxClaims * 2
)

func storyRunes(segs []ledger.Segment) int {
	n := 0
	for _, seg := range segs {
		n += utf8.RuneCountInString(seg.Text)
	}
	return n
}

// screenTexts lists a story and each of its segments for the safety gate.
func screenTexts(text string, segs []ledger.Segment) []string {
	texts := make([]string, 0, len(segs)+1)
	texts = append(texts, text)
	for _, seg := range segs {
		texts = append(texts, seg.Text)
	}
	return texts
}

// StoryInput provides the story: a seeded scenario id or practice text.
type StoryInput struct {
	ExpectedVersion int       `json:"expectedVersion"`
	Mode            StoryMode `json:"mode"`
	ScenarioID      string    `json:"scenarioId,omitempty"`
	Text            string    `json:"text,omitempty"`
}

// StoryResult reports the routing decision with the new state.
type StoryResult struct {
	Safety safety.Level `json:"safety"`
	View   *SessionView `json:"session"`
}

// SubmitStory replaces the story. The deterministic safety gate runs over any
// text the user sent before the request is validated, its mode included
// (SPEC V3): on a crisis route the story is not stored, the session is marked
// interrupted and no model call or handoff can follow; an interrupted session
// accepts no new story (crisis_interrupted).
// A new story starts a new ledger; earlier approvals do not carry over, and
// the event logs of earlier operations are dropped.
func (s *Service) SubmitStory(sessionID string, in StoryInput) (*StoryResult, error) {
	var res StoryResult
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		if sess.Safety.Interrupted {
			return errCrisis
		}
		// crisis reports whether the gate routes text to the crisis route;
		// the text is then not kept, routing state is.
		crisis := func(text string, segs []ledger.Segment) bool {
			a := s.assessText(sess, screenTexts(text, segs)...)
			res.Safety = maxLevel(res.Safety, a.Level)
			if !a.Interrupts() {
				return false
			}
			bump(sess)
			res.View = s.buildView(sess)
			return true
		}
		// The user's own text is screened whatever the mode and before the
		// mode itself is checked, so crisis language is never turned away
		// as an input error nor ignored beside a scenario. A seeded story is
		// assessed too (crisis scenarios demonstrate the route).
		text := strings.TrimSpace(in.Text)
		segments := ledger.SplitSegments(text, "story", 1)
		if text != "" && crisis(text, segments) {
			return nil
		}
		var scenarioID string
		switch in.Mode {
		case StoryScenario:
			sc, ok := s.catalog.Scenario(in.ScenarioID)
			if !ok {
				return errf(CodeInvalid, 400, "scenarioId")
			}
			text, scenarioID = sc.Story, sc.ID
			segments = ledger.SplitSegments(text, "story", 1)
			if crisis(text, segments) {
				return nil
			}
		case StoryPractice:
		default:
			return errf(CodeInvalid, 400, "mode")
		}
		if in.Mode == StoryPractice {
			n := utf8.RuneCountInString(text)
			if !utf8.ValidString(text) || containsControl(text) || n < MinPracticeRunes || n > MaxPracticeRunes {
				return errf(CodeInvalid, 400, "text")
			}
			if containsContactDetails(text) {
				return errf(CodeContactDetails, 422, "text")
			}
		}
		if len(segments) == 0 {
			return errf(CodeInvalid, 400, "text")
		}
		version := 1
		if sess.Story != nil {
			version = sess.Story.Version + 1
		}
		s.cancelKind(sess, OpLedgerDraft, "story_replaced")
		s.cancelKind(sess, OpOpeningDraft, "story_replaced")
		s.purgeRuntimes(sess.ID, "story_replaced", false)
		sess.Story = &Story{Version: version, Mode: in.Mode, ScenarioID: scenarioID, Segments: segments, Submitted: s.now()}
		prevLedger := 0
		if sess.Ledger != nil {
			prevLedger = sess.Ledger.Version
		}
		sess.Ledger = &Ledger{Version: prevLedger + 1, Items: []Item{}, Unknowns: append([]ledger.UnknownField{}, ledger.CoreFields...), Provenance: ProvenanceManual, StoryVersion: version}
		recompute(sess.Ledger)
		sess.Opening = nil
		sess.ledgerRev++
		sess.openingRev++
		bump(sess)
		res.View = s.buildView(sess)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}
