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

// SubmitStory replaces the story. The deterministic safety gate runs before
// anything else (SPEC V3): on a crisis route the story is not stored, the
// session is marked interrupted and no model call or handoff can follow.
// A new story starts a new ledger; earlier approvals do not carry over.
func (s *Service) SubmitStory(sessionID string, in StoryInput) (*StoryResult, error) {
	var res StoryResult
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		var text, scenarioID string
		switch in.Mode {
		case StoryScenario:
			sc, ok := s.catalog.Scenario(in.ScenarioID)
			if !ok {
				return errf(CodeInvalid, 400, "scenarioId")
			}
			text, scenarioID = sc.Story, sc.ID
		case StoryPractice:
			text = strings.TrimSpace(in.Text)
			n := utf8.RuneCountInString(text)
			if !utf8.ValidString(text) || containsControl(text) || n < MinPracticeRunes || n > MaxPracticeRunes {
				return errf(CodeInvalid, 400, "text")
			}
		default:
			return errf(CodeInvalid, 400, "mode")
		}
		segments := ledger.SplitSegments(text, "story", 1)
		texts := make([]string, 0, len(segments)+1)
		texts = append(texts, text)
		for _, seg := range segments {
			texts = append(texts, seg.Text)
		}
		a := s.assessText(sess, texts...)
		res.Safety = a.Level
		if a.Interrupts() {
			// The crisis text is not kept; routing state is.
			sess.Story, sess.Ledger, sess.Opening = nil, nil, nil
			bump(sess)
			res.View = s.buildView(sess)
			return nil
		}
		if in.Mode == StoryPractice && containsContactDetails(text) {
			return errf(CodeContactDetails, 422, "text")
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
		sess.Story = &Story{Version: version, Mode: in.Mode, ScenarioID: scenarioID, Segments: segments, Submitted: s.now()}
		prevLedger := 0
		if sess.Ledger != nil {
			prevLedger = sess.Ledger.Version
		}
		sess.Ledger = &Ledger{Version: prevLedger + 1, Items: []Item{}, Unknowns: append([]ledger.UnknownField{}, ledger.CoreFields...), Provenance: ProvenanceManual, StoryVersion: version}
		recompute(sess.Ledger)
		sess.Opening = nil
		bump(sess)
		res.View = s.buildView(sess)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &res, nil
}
