package ledger

import (
	"errors"
	"strings"
	"testing"
)

const validDraft = `{"claims":[{"segment_id":"s1","quote":"susah tidur","kind":"experience","text":"Susah tidur","subject":"self","time":"current","certainty":"stated","negated":false}],"unknowns":["goal"],"clarification":{"field":"goal"}}`

func TestParseDraftAccepts(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"bare", validDraft},
		{"whitespace", "\n  " + validDraft + "\n"},
		{"json fence", "```json\n" + validDraft + "\n```"},
		{"plain fence", "```\n" + validDraft + "\n```"},
		{"prose around", "Here is the draft:\n" + validDraft + "\nHope this helps."},
		{"prose and fence", "Sure!\n```json\n" + validDraft + "\n```\nDone."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, err := ParseDraft([]byte(tc.raw))
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if len(d.Claims) != 1 || d.Claims[0].Quote != "susah tidur" || d.Clarification == nil || d.Clarification.Field != "goal" {
				t.Errorf("draft = %+v", d)
			}
		})
	}
	d, err := ParseDraft([]byte(`{"claims":[]}`))
	if err != nil || d.Claims == nil || len(d.Claims) != 0 || d.Clarification != nil {
		t.Errorf("empty claims: %+v, %v", d, err)
	}
}

func TestParseDraftRejects(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want error
	}{
		{"unknown top-level field", `{"claims":[],"approved":true}`, ErrMalformed},
		{"unknown claim field", `{"claims":[{"segment_id":"s1","quote":"abc","kind":"feeling","text":"","subject":"self","time":"current","certainty":"stated","negated":false,"confidence":0.9}]}`, ErrMalformed},
		{"unknown clarification field", `{"claims":[],"clarification":{"field":"goal","question":"Sejak kapan?"}}`, ErrMalformed},
		{"case-folded field", `{"Claims":[]}`, ErrMalformed},
		{"case-folded claim field", `{"claims":[{"Quote":"abc"}]}`, ErrMalformed},
		{"null claim", `{"claims":[null]}`, ErrMalformed},
		{"missing claims", `{"unknowns":[]}`, ErrMissingField},
		{"null claims", `{"claims":null}`, ErrMissingField},
		{"wrong type", `{"claims":"none"}`, ErrMalformed},
		{"trailing object", `{"claims":[]} {"claims":[]}`, ErrTrailingData},
		{"trailing garbage inside braces", `{"claims":[]} garbage }`, ErrTrailingData},
		{"syntax error", `{"claims":[}`, ErrMalformed},
		{"no object", `I cannot help with that.`, ErrNoJSONObject},
		{"array only", `[]`, ErrNoJSONObject},
		{"oversize", `{"claims":[],"unknowns":["` + strings.Repeat("a", MaxDraftBytes) + `"]}`, ErrDraftTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseDraft([]byte(tc.raw))
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if strings.Contains(err.Error(), "approved") || strings.Contains(err.Error(), "confidence") || strings.Contains(err.Error(), "Sejak") {
				t.Errorf("error echoes model output: %v", err)
			}
		})
	}
}

func TestParseOpening(t *testing.T) {
	d, err := ParseOpening([]byte("```json\n{\"lines\":[\"Saya susah tidur.\",\"Saya ingin bercerita.\"]}\n```"))
	if err != nil || len(d.Lines) != 2 {
		t.Fatalf("got %+v, %v", d, err)
	}
	for _, raw := range []string{`{"lines":[],"tone":"warm"}`, `{"Lines":[]}`, `{}`, `{"lines":[]}x}`, `nothing`} {
		if _, err := ParseOpening([]byte(raw)); err == nil {
			t.Errorf("ParseOpening(%q) accepted", raw)
		}
	}
}
