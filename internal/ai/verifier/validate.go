package verifier

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrInvalidOutput marks a report that violates VC-6. The whole report is
// rejected; partial reports are never used.
var ErrInvalidOutput = errors.New("verifier: invalid output")

// outputError is a VC-6 violation; errors.Is(err, ErrInvalidOutput) holds.
type outputError struct{ msg string }

func (e *outputError) Error() string        { return ErrInvalidOutput.Error() + ": " + e.msg }
func (e *outputError) Is(target error) bool { return target == ErrInvalidOutput }

func bad(msg string) error { return &outputError{msg: msg} }

// rawReport mirrors the VC-2 schema with plain strings, so enum casing
// can be checked case-insensitively after strict decoding.
type rawReport struct {
	Claims []rawClaim `json:"claims"`
	Issues []rawIssue `json:"issues"`
}

type rawClaim struct {
	ClaimID *string  `json:"claim_id"`
	Verdict *string  `json:"verdict"`
	SpanIDs []string `json:"span_ids"`
	Reason  *string  `json:"reason"`
}

type rawIssue struct {
	Category *string  `json:"category"`
	ClaimID  *string  `json:"claim_id"`
	SpanIDs  []string `json:"span_ids"`
	Reason   *string  `json:"reason"`
}

// Parse decodes and validates a report for req (VC-6):
//   - strict JSON: unknown fields, missing required fields, duplicate keys,
//     keys not spelled exactly as in the schema (encoding/json would match
//     "Verdict" to verdict), trailing data and a body above
//     MaxResponseBytes are rejected;
//   - the claim-id set equals the request's exactly (none missing, extra
//     or duplicated);
//   - verdicts and categories match the enums case-insensitively;
//   - span_ids are a subset of the supplied spans without repeats, and a
//     "supported" verdict cites at least one of the claim's own spans;
//   - every reason is non-empty and at most MaxReasonRunes runes after
//     control characters are stripped;
//   - an issue's claim_id is "" or a known claim.
//
// Error messages never contain report or request text.
func Parse(text string, req Request) (Report, error) {
	if len(text) > MaxResponseBytes {
		return Report{}, bad("report too large")
	}
	if !utf8.ValidString(text) {
		return Report{}, bad("report is not UTF-8")
	}
	if err := CheckKeys(text, reportKeys); err != nil {
		return Report{}, bad("report has a duplicate, misspelled or malformed key")
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	var raw rawReport
	if err := dec.Decode(&raw); err != nil {
		return Report{}, bad("report is not the schema's JSON object")
	}
	if dec.More() || trailing(dec) {
		return Report{}, bad("trailing data after the report")
	}
	if raw.Claims == nil || raw.Issues == nil {
		return Report{}, bad("claims and issues are required")
	}
	spans := make(map[string]bool, len(req.Spans))
	for _, s := range req.Spans {
		spans[s.ID] = true
	}
	own := make(map[string]map[string]bool, len(req.Claims))
	for _, c := range req.Claims {
		cited := make(map[string]bool, len(c.SpanIDs))
		for _, id := range c.SpanIDs {
			cited[id] = true
		}
		own[c.ID] = cited
	}
	if len(raw.Claims) != len(req.Claims) {
		return Report{}, bad("claim count differs from the request")
	}
	byID := make(map[string]ClaimVerdict, len(raw.Claims))
	for _, rc := range raw.Claims {
		if rc.ClaimID == nil || rc.Verdict == nil || rc.SpanIDs == nil || rc.Reason == nil {
			return Report{}, bad("a claim finding lacks a required field")
		}
		cited, ok := own[*rc.ClaimID]
		if !ok {
			return Report{}, bad("unknown claim id")
		}
		if _, dup := byID[*rc.ClaimID]; dup {
			return Report{}, bad("duplicate claim id")
		}
		v, ok := parseVerdict(*rc.Verdict)
		if !ok {
			return Report{}, bad("verdict outside the enum")
		}
		ids, err := checkSpans(rc.SpanIDs, spans)
		if err != nil {
			return Report{}, err
		}
		if v == Supported && !citesOwn(ids, cited) {
			return Report{}, bad("a supported verdict cites none of the claim's spans")
		}
		reason, err := cleanReason(*rc.Reason)
		if err != nil {
			return Report{}, err
		}
		byID[*rc.ClaimID] = ClaimVerdict{ClaimID: *rc.ClaimID, Verdict: v, SpanIDs: ids, Reason: reason}
	}
	rep := Report{Claims: make([]ClaimVerdict, 0, len(req.Claims)), Issues: make([]Issue, 0, len(raw.Issues))}
	for _, c := range req.Claims {
		rep.Claims = append(rep.Claims, byID[c.ID])
	}
	for _, ri := range raw.Issues {
		if ri.Category == nil || ri.ClaimID == nil || ri.SpanIDs == nil || ri.Reason == nil {
			return Report{}, bad("an issue lacks a required field")
		}
		cat, ok := parseCategory(*ri.Category)
		if !ok {
			return Report{}, bad("issue category outside the enum")
		}
		if *ri.ClaimID != "" {
			if _, ok := own[*ri.ClaimID]; !ok {
				return Report{}, bad("issue names an unknown claim")
			}
		}
		ids, err := checkSpans(ri.SpanIDs, spans)
		if err != nil {
			return Report{}, err
		}
		reason, err := cleanReason(*ri.Reason)
		if err != nil {
			return Report{}, err
		}
		rep.Issues = append(rep.Issues, Issue{Category: cat, ClaimID: *ri.ClaimID, SpanIDs: ids, Reason: reason})
	}
	return rep, nil
}

// trailing reports non-whitespace data after the first JSON value.
func trailing(dec *json.Decoder) bool {
	rest, err := io.ReadAll(dec.Buffered())
	if err != nil {
		return true
	}
	return len(bytes.TrimSpace(rest)) > 0
}

func parseVerdict(s string) (Verdict, bool) {
	for _, v := range Verdicts() {
		if strings.EqualFold(s, string(v)) {
			return v, true
		}
	}
	return "", false
}

func parseCategory(s string) (Category, bool) {
	for _, c := range Categories() {
		if strings.EqualFold(s, string(c)) {
			return c, true
		}
	}
	return "", false
}

func checkSpans(ids []string, spans map[string]bool) ([]string, error) {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !spans[id] {
			return nil, bad("span id outside the supplied spans")
		}
		if seen[id] {
			return nil, bad("repeated span id")
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

func citesOwn(ids []string, own map[string]bool) bool {
	for _, id := range ids {
		if own[id] {
			return true
		}
	}
	return false
}

// cleanReason strips control characters (newlines and tabs become
// spaces), trims, and enforces 1..MaxReasonRunes runes. The result is
// rendered as text, never as HTML.
func cleanReason(s string) (string, error) {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteByte(' ')
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r):
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	if out == "" {
		return "", bad("empty reason")
	}
	if utf8.RuneCountInString(out) > MaxReasonRunes {
		return "", bad("reason too long")
	}
	return out, nil
}
