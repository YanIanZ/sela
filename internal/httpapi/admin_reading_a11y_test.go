package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YanIanZ/sela/internal/reading"
)

// TestWriteA11yGateError pins the status and code of every accessibility
// refusal, including the blocked case that no embedded item triggers today.
func TestWriteA11yGateError(t *testing.T) {
	items := readingFixture(t).AdminItems(nil)
	if len(items) == 0 {
		t.Fatal("empty fixture library")
	}
	report := items[0].Accessibility
	blocked := report
	blocked.Checks = append([]reading.A11yCheck(nil), report.Checks...)
	for i := range blocked.Checks {
		if blocked.Checks[i].ID == "sensory_instructions" {
			blocked.Checks[i].Status = reading.A11yFail
		}
	}
	full := map[string]bool{}
	for _, id := range reading.ManualCheckIDs() {
		full[id] = true
	}
	req := &reading.A11yRequest{ChecksVersion: reading.A11yChecksVersion, ManualVersion: reading.A11yManualVersion, ManualChecks: full}

	cases := []struct {
		name   string
		report reading.A11yReport
		req    *reading.A11yRequest
		status int
		code   string
		field  string
	}{
		{"blocked", blocked, req, http.StatusUnprocessableEntity, "accessibility_checks_failed", "accessibility.checks.sensory_instructions"},
		{"missing", report, nil, http.StatusBadRequest, "accessibility_checklist_incomplete", "accessibility"},
		{"version", report, &reading.A11yRequest{ChecksVersion: "old", ManualVersion: reading.A11yManualVersion}, http.StatusConflict, "accessibility_checks_changed", "accessibility.checksVersion"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := reading.GateAccessibility(c.report, c.req)
			if err == nil {
				t.Fatal("gate accepted")
			}
			rec := httptest.NewRecorder()
			writeA11yGateError(rec, httptest.NewRequest("POST", "/api/admin/v1/reading/x/approve", nil), err)
			var body struct {
				Error struct{ Code, Field, Message string } `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if rec.Code != c.status || body.Error.Code != c.code || body.Error.Field != c.field || body.Error.Message == "" {
				t.Fatalf("got %d %+v", rec.Code, body.Error)
			}
			if body.Error.Message == messages["internal"].id {
				t.Fatalf("%s has no localized message", c.code)
			}
		})
	}
}
