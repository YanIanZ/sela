package evidenceguard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fixture() Input {
	return Input{Synthetic: true, VerificationConsent: true, FixtureVersion: "fictional-id-v1", BriefVersion: 2,
		Evidence: []Evidence{{ID: "e1", Quote: "Saya merasa kewalahan saat kuliah, tapi masih mengikuti kelas."}, {ID: "e2", Quote: "Belum menyebut diagnosis."}},
		Claims:   []Claim{{ID: "c1", Text: "Saya merasa kewalahan saat kuliah.", EvidenceIDs: []string{"e1"}}}}
}

func mock(t *testing.T, answer string, status int, check func(*http.Request)) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			check(r)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	c, _ := New("test-key")
	c.endpoint = server.URL
	return c
}

func envelope(t *testing.T, findings any) string {
	t.Helper()
	x, e := json.Marshal(findings)
	if e != nil {
		t.Fatal(e)
	}
	y, e := json.Marshal(map[string]any{"model": Model, "stop_reason": "end_turn",
		"content": []map[string]string{{"type": "text", "text": string(x)}}})
	if e != nil {
		t.Fatal(e)
	}
	return string(y)
}

func TestSupportedAndCorrectRequest(t *testing.T) {
	answer := envelope(t, map[string]any{"findings": []map[string]any{{
		"claim_id": "c1", "status": "supported", "evidence_ids": []string{"e1"}, "reason": "The quote supports it."}}})
	c := mock(t, answer, 200, func(r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-key" ||
			r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Error("request headers")
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req["model"] != Model || req["output_config"] == nil {
			t.Error("missing model or schema")
		}
		if _, ok := req["temperature"]; ok {
			t.Error("invalid Sonnet 5.5 temperature")
		}
	})
	report, err := c.Verify(context.Background(), fixture())
	if err != nil {
		t.Fatal(err)
	}
	if report.ReviewStatus != "model_supported" || report.Model != Model || report.BriefVersion != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestContradictedClaimNeedsHumanReview(t *testing.T) {
	answer := envelope(t, map[string]any{"findings": []map[string]any{{
		"claim_id": "c1", "status": "contradicted", "evidence_ids": []string{"e1"}, "reason": "Negation inverted."}}})
	c := mock(t, answer, 200, nil)
	out, err := c.Verify(context.Background(), fixture())
	if err != nil || out.ReviewStatus != "needs_human_review" {
		t.Fatalf("%+v %v", out, err)
	}
}

func TestRefuseUnauthorizedInputsBeforeApi(t *testing.T) {
	count := 0
	c := mock(t, "", 200, func(r *http.Request) { count++ })
	cases := []struct {
		name   string
		change func(*Input)
	}{
		{"no verifier consent", func(x *Input) { x.VerificationConsent = false }},
		{"non synthetic", func(x *Input) { x.Synthetic = false }},
		{"unlisted citation", func(x *Input) { x.Claims[0].EvidenceIDs = []string{"not-real"} }},
		{"duplicate source", func(x *Input) { x.Evidence = append(x.Evidence, x.Evidence[0]) }},
		{"empty claim", func(x *Input) { x.Claims[0].Text = "" }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := fixture()
			tt.change(&f)
			if _, err := c.Verify(context.Background(), f); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected invalid input, got %v", err)
			}
		})
	}
	if count != 0 {
		t.Fatalf("unapproved calls=%d", count)
	}
}

func TestRejectMalformedOrForgedOutput(t *testing.T) {
	cases := []struct{ name, data string }{
		{"unknown claim", envelope(t, map[string]any{"findings": []map[string]any{{"claim_id": "x", "status": "supported", "evidence_ids": []string{"e1"}, "reason": "a"}}})},
		{"unknown evidence", envelope(t, map[string]any{"findings": []map[string]any{{"claim_id": "c1", "status": "supported", "evidence_ids": []string{"x"}, "reason": "a"}}})},
		{"wrong status", envelope(t, map[string]any{"findings": []map[string]any{{"claim_id": "c1", "status": "approved", "evidence_ids": []string{"e1"}, "reason": "a"}}})},
		{"missing claim", envelope(t, map[string]any{"findings": []any{}})},
		{"extra field", envelope(t, map[string]any{"findings": []map[string]any{{"claim_id": "c1", "status": "supported", "evidence_ids": []string{"e1"}, "reason": "a", "diagnosis": "invented"}}})},
		{"garbage", "not-json"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := mock(t, tt.data, 200, nil)
			if _, err := c.Verify(context.Background(), fixture()); !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("want invalid response, got %v", err)
			}
		})
	}
}

func TestApiFailureDoesNotLeakProviderBody(t *testing.T) {
	secret := "private narrative must not leak"
	c := mock(t, secret, 503, nil)
	_, err := c.Verify(context.Background(), fixture())
	if !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := mock(t, "", 200, func(r *http.Request) { t.Fatal("canceled request sent") })
	if _, err := c.Verify(ctx, fixture()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want unavailable: %v", err)
	}
}
