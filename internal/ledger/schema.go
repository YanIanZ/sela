package ledger

// DraftJSONSchema is the JSON Schema sent for guided decoding of ledger
// drafts. It uses only the subset of draft 2020-12 that vLLM/xgrammar guided
// decoding supports (type, object, properties, required, enum, array, items,
// maxItems, maxLength, additionalProperties: false; no regex patterns).
// Every enum mirrors a Go constant list; schema_test.go checks this.
const DraftJSONSchema = `{
  "type": "object",
  "properties": {
    "claims": {
      "type": "array",
      "maxItems": 12,
      "items": {
        "type": "object",
        "properties": {
          "segment_id": {"type": "string", "maxLength": 16},
          "quote": {"type": "string", "maxLength": 300},
          "kind": {"type": "string", "enum": ["feeling", "experience", "impact", "duration", "context", "worry", "goal", "preference", "question"]},
          "text": {"type": "string", "maxLength": 300},
          "subject": {"type": "string", "enum": ["self", "other"]},
          "time": {"type": "string", "enum": ["current", "past", "future", "unspecified"]},
          "certainty": {"type": "string", "enum": ["stated", "uncertain", "hypothetical"]},
          "negated": {"type": "boolean"}
        },
        "required": ["segment_id", "quote", "kind", "text", "subject", "time", "certainty", "negated"],
        "additionalProperties": false
      }
    },
    "unknowns": {
      "type": "array",
      "maxItems": 5,
      "items": {"type": "string", "enum": ["concern", "duration", "daily_impact", "goal", "support_preference"]}
    },
    "clarification": {
      "type": "object",
      "properties": {
        "field": {"type": "string", "enum": ["concern", "duration", "daily_impact", "goal", "support_preference", "none"]}
      },
      "required": ["field"],
      "additionalProperties": false
    }
  },
  "required": ["claims", "unknowns", "clarification"],
  "additionalProperties": false
}`

// OpeningJSONSchema is the JSON Schema sent for guided decoding of opening
// lines (same supported subset as DraftJSONSchema).
const OpeningJSONSchema = `{
  "type": "object",
  "properties": {
    "lines": {
      "type": "array",
      "maxItems": 3,
      "items": {"type": "string", "maxLength": 240}
    }
  },
  "required": ["lines"],
  "additionalProperties": false
}`
