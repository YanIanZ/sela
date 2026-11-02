package reading

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Accessibility approval protocol (sela-reading-a11y-v1).
//
//  1. The client sends the rule-set and checklist versions it previewed. A
//     different version is refused (ErrA11yVersion) so a reviewer never
//     confirms a checklist or results they did not see.
//  2. The server recomputes the automatic report from the exact embedded
//     content. Any failing blocking check refuses the approval
//     (ErrA11yBlocked); there is no override for these.
//  3. Every manual checklist item must be confirmed true
//     (ErrA11yChecklist).
//  4. When only overridable checks (readability) fail, the reviewer must
//     give a reason of 20-300 characters (ErrA11yOverrideRequired). The
//     reason is stored on the approval and written as its own
//     reading.a11y_override audit event. A reason sent when nothing failed is
//     refused (ErrA11yOverrideUnneeded) so the record stays truthful.

// A11yRequest is what the reviewer submits with an approval.
type A11yRequest struct {
	ChecksVersion  string          `json:"checksVersion"`
	ManualVersion  string          `json:"manualVersion"`
	ManualChecks   map[string]bool `json:"manualChecks"`
	OverrideReason string          `json:"overrideReason"`
}

// ApprovalAccessibility is the accessibility record stored with an
// approval. Overridden lists the failing overridable checks the reason
// covers.
type ApprovalAccessibility struct {
	ChecksVersion   string      `json:"checksVersion"`
	Result          string      `json:"result"`
	Checks          []A11yCheck `json:"checks"`
	ManualVersion   string      `json:"manualVersion"`
	ManualConfirmed []string    `json:"manualConfirmed"`
	OverrideReason  string      `json:"overrideReason,omitempty"`
	Overridden      []string    `json:"overridden,omitempty"`
}

// Recorded approval results.
const (
	A11yApprovedPass       = "pass"
	A11yApprovedOverridden = "overridden"
)

// Override reason bounds (runes, after trimming).
const (
	A11yOverrideMin = 20
	A11yOverrideMax = 300
)

// Gate errors. A11yGateError carries the request field and check ids.
var (
	ErrA11yVersion          = errors.New("reading: accessibility checks changed")
	ErrA11yBlocked          = errors.New("reading: accessibility checks failed")
	ErrA11yChecklist        = errors.New("reading: accessibility checklist incomplete")
	ErrA11yOverrideRequired = errors.New("reading: accessibility override reason required")
	ErrA11yOverrideUnneeded = errors.New("reading: accessibility override reason without a failed check")
)

// A11yGateError explains a refusal.
type A11yGateError struct {
	Err    error
	Field  string
	Checks []string
}

func (e *A11yGateError) Error() string { return e.Err.Error() + ": " + e.Field }
func (e *A11yGateError) Unwrap() error { return e.Err }

func gateErr(err error, field string, checks []string) error {
	return &A11yGateError{Err: err, Field: field, Checks: checks}
}

// GateAccessibility applies the protocol to one report. It never approves
// anything by itself; it returns the record to store with a human approval.
func GateAccessibility(report A11yReport, req *A11yRequest) (ApprovalAccessibility, error) {
	if req == nil {
		return ApprovalAccessibility{}, gateErr(ErrA11yChecklist, "accessibility", nil)
	}
	if report.Version != A11yChecksVersion || req.ChecksVersion != A11yChecksVersion {
		return ApprovalAccessibility{}, gateErr(ErrA11yVersion, "accessibility.checksVersion", nil)
	}
	if req.ManualVersion != A11yManualVersion {
		return ApprovalAccessibility{}, gateErr(ErrA11yVersion, "accessibility.manualVersion", nil)
	}
	blocking, overridable := report.Failing()
	if len(blocking) > 0 {
		return ApprovalAccessibility{}, gateErr(ErrA11yBlocked, "accessibility.checks."+blocking[0], blocking)
	}
	for _, id := range ManualCheckIDs() {
		if !req.ManualChecks[id] {
			return ApprovalAccessibility{}, gateErr(ErrA11yChecklist, "accessibility.manualChecks."+id, nil)
		}
	}
	for id := range req.ManualChecks {
		if !isManualCheck(id) {
			return ApprovalAccessibility{}, gateErr(ErrA11yChecklist, "accessibility.manualChecks."+id, nil)
		}
	}
	reason := strings.TrimSpace(req.OverrideReason)
	out := ApprovalAccessibility{
		ChecksVersion:   report.Version,
		Result:          A11yApprovedPass,
		Checks:          report.Checks,
		ManualVersion:   A11yManualVersion,
		ManualConfirmed: ManualCheckIDs(),
	}
	if len(overridable) == 0 {
		if reason != "" {
			return ApprovalAccessibility{}, gateErr(ErrA11yOverrideUnneeded, "accessibility.overrideReason", nil)
		}
		return out, nil
	}
	if n := utf8.RuneCountInString(reason); n < A11yOverrideMin || n > A11yOverrideMax {
		return ApprovalAccessibility{}, gateErr(ErrA11yOverrideRequired, "accessibility.overrideReason", overridable)
	}
	out.Result = A11yApprovedOverridden
	out.OverrideReason = reason
	out.Overridden = overridable
	return out, nil
}

func isManualCheck(id string) bool {
	for _, c := range ManualChecklist {
		if c.ID == id {
			return true
		}
	}
	return false
}

// validRecord re-checks a record before it is stored, so a caller that skips
// GateAccessibility still cannot persist an inconsistent approval.
func (a *ApprovalAccessibility) validRecord() bool {
	if a == nil || a.ChecksVersion != A11yChecksVersion || a.ManualVersion != A11yManualVersion {
		return false
	}
	if len(a.Checks) == 0 || len(a.ManualConfirmed) != len(ManualChecklist) {
		return false
	}
	for i, id := range ManualCheckIDs() {
		if a.ManualConfirmed[i] != id {
			return false
		}
	}
	blocking, overridable := A11yReport{Checks: a.Checks}.Failing()
	if len(blocking) > 0 {
		return false
	}
	reason := strings.TrimSpace(a.OverrideReason)
	n := utf8.RuneCountInString(reason)
	switch a.Result {
	case A11yApprovedPass:
		return len(overridable) == 0 && reason == ""
	case A11yApprovedOverridden:
		return len(overridable) > 0 && n >= A11yOverrideMin && n <= A11yOverrideMax
	}
	return false
}
