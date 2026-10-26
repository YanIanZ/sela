package ledger

import (
	"encoding/json"
	"slices"
	"testing"
)

// schemaNode is the subset of JSON Schema the contracts use.
type schemaNode struct {
	Type                 string                 `json:"type"`
	Properties           map[string]*schemaNode `json:"properties"`
	Required             []string               `json:"required"`
	Enum                 []string               `json:"enum"`
	Items                *schemaNode            `json:"items"`
	MaxItems             *int                   `json:"maxItems"`
	MaxLength            *int                   `json:"maxLength"`
	AdditionalProperties *bool                  `json:"additionalProperties"`
}

var allowedSchemaKeys = map[string]bool{
	"type": true, "properties": true, "required": true, "enum": true, "items": true,
	"maxItems": true, "maxLength": true, "additionalProperties": true,
}

// checkSubset fails on any keyword outside the guided-decoding subset, and
// on objects that allow additional properties or leave fields optional.
func checkSubset(t *testing.T, path string, raw json.RawMessage) {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	for k := range m {
		if !allowedSchemaKeys[k] {
			t.Errorf("%s: unsupported keyword %q", path, k)
		}
	}
	var n schemaNode
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if n.Type == "object" {
		if n.AdditionalProperties == nil || *n.AdditionalProperties {
			t.Errorf("%s: additionalProperties must be false", path)
		}
		var props map[string]json.RawMessage
		_ = json.Unmarshal(m["properties"], &props)
		for name, p := range props {
			checkSubset(t, path+"."+name, p)
			if !slices.Contains(n.Required, name) {
				t.Errorf("%s: %q not required", path, name)
			}
		}
	}
	if items, ok := m["items"]; ok {
		checkSubset(t, path+"[]", items)
	}
}

func parseSchema(t *testing.T, s string) *schemaNode {
	t.Helper()
	if !json.Valid([]byte(s)) {
		t.Fatal("schema is not valid JSON")
	}
	var n schemaNode
	if err := json.Unmarshal([]byte(s), &n); err != nil {
		t.Fatal(err)
	}
	checkSubset(t, "$", json.RawMessage(s))
	return &n
}

func strs[T ~string](vs []T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

func TestDraftJSONSchemaMatchesGo(t *testing.T) {
	root := parseSchema(t, DraftJSONSchema)
	claim := root.Properties["claims"].Items
	if got := *root.Properties["claims"].MaxItems; got != MaxClaims {
		t.Errorf("claims maxItems = %d, want %d", got, MaxClaims)
	}
	enums := map[string][]string{
		"kind":      strs(AllKinds),
		"subject":   {string(SubjectSelf), string(SubjectOther)},
		"time":      {string(TimeCurrent), string(TimePast), string(TimeFuture), string(TimeUnspecified)},
		"certainty": {string(CertaintyStated), string(CertaintyUncertain), string(CertaintyHypothetical)},
	}
	for field, want := range enums {
		if got := claim.Properties[field].Enum; !slices.Equal(got, want) {
			t.Errorf("%s enum = %v, want %v", field, got, want)
		}
	}
	if *claim.Properties["quote"].MaxLength != MaxQuoteRunes || *claim.Properties["text"].MaxLength != MaxWordingRunes {
		t.Errorf("quote/text maxLength mismatch")
	}
	if claim.Properties["negated"].Type != "boolean" {
		t.Errorf("negated type = %q", claim.Properties["negated"].Type)
	}
	// Every Candidate JSON field is in the schema and vice versa.
	if len(claim.Properties) != len(candidateKeys) {
		t.Errorf("claim properties = %d, candidate fields = %d", len(claim.Properties), len(candidateKeys))
	}
	for k := range candidateKeys {
		if claim.Properties[k] == nil {
			t.Errorf("candidate field %q missing from schema", k)
		}
	}
	if got := root.Properties["unknowns"].Items.Enum; !slices.Equal(got, strs(CoreFields)) {
		t.Errorf("unknowns enum = %v", got)
	}
	if got := root.Properties["clarification"].Properties["field"].Enum; !slices.Equal(got, append(strs(CoreFields), "none")) {
		t.Errorf("clarification enum = %v", got)
	}
	// A draft that follows the schema parses.
	if _, err := ParseDraft([]byte(validDraft)); err != nil {
		t.Errorf("schema-shaped draft rejected: %v", err)
	}
}

func TestOpeningJSONSchemaMatchesGo(t *testing.T) {
	root := parseSchema(t, OpeningJSONSchema)
	lines := root.Properties["lines"]
	if lines.Type != "array" || lines.Items.Type != "string" {
		t.Errorf("lines = %+v", lines)
	}
	if *lines.MaxItems != MaxOpeningLines || *lines.Items.MaxLength != MaxOpeningRunes {
		t.Errorf("limits mismatch")
	}
}
