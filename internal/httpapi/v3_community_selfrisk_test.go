package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/community"
)

type verdictModerator community.Verdict

func (v verdictModerator) Moderate(context.Context, string, string) community.Verdict {
	return community.Verdict(v)
}

// A self_risk verdict from Meta takes the crisis path: 422 crisis, nothing
// handed to the store. Other verdicts become a draft as before.
func TestCommunityModerateSelfRiskIsCrisis(t *testing.T) {
	t.Parallel()
	s := New(nil, Options{Moderator: verdictModerator{Status: community.StatusHeld, Labels: []string{community.SelfRiskLabel}, Source: "meta", SelfRisk: true}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v3/community/posts", nil)
	if _, _, ok := s.moderate(rec, req, "aku udah di tempat yang tinggi, maaf semuanya", community.MaxPostRunes, ""); ok {
		t.Fatal("self_risk produced a draft to store")
	}
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"crisis"`) {
		t.Fatalf("self_risk response = %d %s", rec.Code, rec.Body)
	}

	s = New(nil, Options{Moderator: verdictModerator{Status: community.StatusPublished, Source: "meta"}})
	rec = httptest.NewRecorder()
	d, _, ok := s.moderate(rec, req, "semangat ya, kamu nggak sendirian", community.MaxPostRunes, "")
	if !ok || d.Status != community.StatusPublished {
		t.Fatalf("published verdict = %+v ok=%v code=%d", d, ok, rec.Code)
	}
}
