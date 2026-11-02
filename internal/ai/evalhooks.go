package ai

import (
	"errors"

	"github.com/YanIanZ/sela/internal/ai/minimize"
)

// EvalPrompt returns the exact system and user messages the router sends
// to the generator for a first (not regenerated) draft of task, with no
// intent or purpose. It exists for cmd/sela-eval's live MIE benchmark, so
// the benchmark sends the production prompt instead of a copy that could
// drift.
func EvalPrompt(task Task, language, audience string, spans []minimize.Span) (system, user string, err error) {
	if _, ok := taskRules[task]; !ok {
		return "", "", errors.New("ai: no prompt for task")
	}
	return prompt(task, language, audience, "", "", nil, spans)
}

// EvalParse validates a generation with the production parser of task
// (parseDraft) against the payload that was sent.
func EvalParse(task Task, text string, payload minimize.Payload) (Draft, error) {
	pol, ok := PolicyFor(task)
	if !ok || !pol.Generative {
		return Draft{}, errors.New("ai: not a generative task")
	}
	return parseDraft(text, pol, payload)
}
