package ledger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Parse errors. Messages never contain model output.
var (
	ErrDraftTooLarge = errors.New("ledger: model output exceeds MaxDraftBytes")
	ErrNoJSONObject  = errors.New("ledger: model output contains no JSON object")
	ErrMalformed     = errors.New("ledger: malformed model output")
	ErrMissingField  = errors.New("ledger: required field missing")
	ErrTrailingData  = errors.New("ledger: trailing data after JSON object")
)

var (
	draftKeys         = set("claims", "unknowns", "clarification")
	candidateKeys     = set("segment_id", "quote", "kind", "text", "subject", "time", "certainty", "negated")
	clarificationKeys = set("field")
	openingKeys       = set("lines")
)

// extractObject applies the v1 parseModelJson tolerance: when the output
// does not start with '{', the body of the first markdown fence is used if
// there is one; then the outermost {...} span is taken, which drops leading
// and trailing prose.
func extractObject(raw []byte) ([]byte, error) {
	if len(raw) > MaxDraftBytes {
		return nil, ErrDraftTooLarge
	}
	s := bytes.TrimSpace(raw)
	if !bytes.HasPrefix(s, []byte("{")) {
		if open := bytes.Index(s, []byte("```")); open >= 0 {
			body := s[open+3:]
			// Skip an info string such as "json".
			if nl := bytes.IndexAny(body, "\r\n"); nl >= 0 && !bytes.ContainsRune(body[:nl], '{') {
				body = body[nl:]
			}
			if end := bytes.Index(body, []byte("```")); end >= 0 {
				body = body[:end]
			}
			s = body
		}
	}
	start := bytes.IndexByte(s, '{')
	end := bytes.LastIndexByte(s, '}')
	if start < 0 || end <= start {
		return nil, ErrNoJSONObject
	}
	return s[start : end+1], nil
}

// decodeStrict decodes the single JSON object in obj into v with unknown
// fields disallowed, and rejects anything after it. It returns the bytes of
// the decoded value.
func decodeStrict(obj []byte, v any) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(obj))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return nil, decodeErr(err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, ErrTrailingData
	}
	return obj[:dec.InputOffset()], nil
}

// decodeErr maps a decoding error to ErrMalformed without echoing input.
func decodeErr(err error) error {
	var syntax *json.SyntaxError
	var typ *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return fmt.Errorf("%w: syntax error at byte %d", ErrMalformed, syntax.Offset)
	case errors.As(err, &typ):
		return fmt.Errorf("%w: wrong type for field %q", ErrMalformed, typ.Field)
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return fmt.Errorf("%w: truncated", ErrMalformed)
	case strings.HasPrefix(err.Error(), "json: unknown field"):
		return fmt.Errorf("%w: unknown field", ErrMalformed)
	}
	return ErrMalformed
}

// exactKeys rejects object keys that encoding/json would accept only through
// its case-insensitive matching ("Claims"), and non-object values.
func exactKeys(raw json.RawMessage, allowed map[string]bool) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return fmt.Errorf("%w: expected an object", ErrMalformed)
	}
	for k := range m {
		if !allowed[k] {
			return fmt.Errorf("%w: unknown field", ErrMalformed)
		}
	}
	return nil
}

// ParseDraft strictly decodes a model draft: at most MaxDraftBytes, one JSON
// object (optionally inside one markdown fence or surrounded by prose), no
// unknown or differently-cased fields, no trailing data, and a "claims"
// array (possibly empty). Errors never include the model output.
func ParseDraft(raw []byte) (Draft, error) {
	obj, err := extractObject(raw)
	if err != nil {
		return Draft{}, err
	}
	var aux struct {
		Claims        *[]Candidate            `json:"claims"`
		Unknowns      []string                `json:"unknowns"`
		Clarification *ClarificationCandidate `json:"clarification"`
	}
	value, err := decodeStrict(obj, &aux)
	if err != nil {
		return Draft{}, err
	}
	if aux.Claims == nil {
		return Draft{}, fmt.Errorf("%w: claims", ErrMissingField)
	}
	if err := exactKeys(value, draftKeys); err != nil {
		return Draft{}, err
	}
	var parts struct {
		Claims        []json.RawMessage `json:"claims"`
		Clarification json.RawMessage   `json:"clarification"`
	}
	if err := json.Unmarshal(value, &parts); err != nil {
		return Draft{}, decodeErr(err)
	}
	for _, c := range parts.Claims {
		if err := exactKeys(c, candidateKeys); err != nil {
			return Draft{}, err
		}
	}
	if len(parts.Clarification) > 0 && string(parts.Clarification) != "null" {
		if err := exactKeys(parts.Clarification, clarificationKeys); err != nil {
			return Draft{}, err
		}
	}
	return Draft{Claims: *aux.Claims, Unknowns: aux.Unknowns, Clarification: aux.Clarification}, nil
}

// ParseOpening strictly decodes a model opening draft with the same rules as
// ParseDraft; "lines" is required.
func ParseOpening(raw []byte) (OpeningDraft, error) {
	obj, err := extractObject(raw)
	if err != nil {
		return OpeningDraft{}, err
	}
	var aux struct {
		Lines *[]string `json:"lines"`
	}
	value, err := decodeStrict(obj, &aux)
	if err != nil {
		return OpeningDraft{}, err
	}
	if aux.Lines == nil {
		return OpeningDraft{}, fmt.Errorf("%w: lines", ErrMissingField)
	}
	if err := exactKeys(value, openingKeys); err != nil {
		return OpeningDraft{}, err
	}
	return OpeningDraft{Lines: *aux.Lines}, nil
}
