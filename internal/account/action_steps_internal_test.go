package account

import (
	"errors"
	"testing"
)

// TestActionTransitions pins the BW-3 lifecycle: discard is always
// allowed, needs_review leaves only through a review onto chosen, a
// discarded step can only be restored, and tried/paused go back to chosen.
func TestActionTransitions(t *testing.T) {
	s := func(v string) *string { return &v }
	cases := []struct {
		from      string
		to        *string
		resolving bool
		want      string
		err       error
	}{
		{ActionChosen, s(ActionTried), false, ActionTried, nil},
		{ActionChosen, s(ActionPaused), false, ActionPaused, nil},
		{ActionChosen, nil, false, ActionChosen, nil},
		{ActionTried, s(ActionChosen), false, ActionChosen, nil},
		{ActionTried, nil, false, ActionTried, nil},
		{ActionTried, s(ActionPaused), false, "", ErrInvalidTransition},
		{ActionPaused, s(ActionChosen), false, ActionChosen, nil},
		{ActionPaused, s(ActionTried), false, "", ErrInvalidTransition},
		{ActionPaused, s(ActionDiscarded), false, ActionDiscarded, nil},
		{ActionDiscarded, s(ActionChosen), false, ActionChosen, nil},
		{ActionDiscarded, s(ActionTried), false, "", ErrInvalidTransition},
		{ActionDiscarded, nil, false, "", ErrInvalidTransition},
		{ActionNeedsReview, s(ActionTried), false, "", ErrInvalidTransition},
		{ActionNeedsReview, nil, false, "", ErrInvalidTransition},
		{ActionNeedsReview, nil, true, ActionChosen, nil},
		{ActionNeedsReview, s(ActionChosen), true, ActionChosen, nil},
		{ActionNeedsReview, s(ActionPaused), true, "", ErrInvalidTransition},
		{ActionNeedsReview, s(ActionDiscarded), false, ActionDiscarded, nil},
		{ActionChosen, s(ActionNeedsReview), false, "", ErrInvalidInput},
		{ActionChosen, s("done"), false, "", ErrInvalidInput},
	}
	for _, c := range cases {
		got, err := actionTransition(c.from, c.to, c.resolving)
		if got != c.want || (c.err == nil) != (err == nil) || (c.err != nil && !errors.Is(err, c.err)) {
			to := "<nil>"
			if c.to != nil {
				to = *c.to
			}
			t.Errorf("%s -> %s (resolving %v): %q %v, want %q %v", c.from, to, c.resolving, got, err, c.want, c.err)
		}
	}
}
