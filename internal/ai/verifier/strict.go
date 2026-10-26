package verifier

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// ErrKeys marks JSON with a duplicate key or a key that is not spelled
// exactly as the schema spells it.
var ErrKeys = errors.New("verifier: duplicate or misspelled JSON key")

// CheckKeys walks the first JSON value in text and rejects any object that
// repeats a key or uses a key that is not exactly one of names. It runs
// before a struct decode: encoding/json matches keys case-insensitively
// and lets the last duplicate win, so {"verdict":"contradicted",
// "Verdict":"supported"} would otherwise decode as supported (VC-6 strict
// decoding). Syntax errors are returned as they are; messages never
// contain the text.
func CheckKeys(text string, names map[string]bool) error {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	type frame struct {
		object, wantKey bool
		keys            map[string]bool
	}
	var stack []frame
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return errors.New("verifier: incomplete JSON value")
		}
		if err != nil {
			return errors.New("verifier: malformed JSON")
		}
		if d, ok := tok.(json.Delim); ok {
			switch d {
			case '{':
				stack = append(stack, frame{object: true, wantKey: true, keys: map[string]bool{}})
				continue
			case '[':
				stack = append(stack, frame{})
				continue
			default: // '}' or ']'
				stack = stack[:len(stack)-1]
			}
		} else if key, ok := tok.(string); ok && len(stack) > 0 && stack[len(stack)-1].object && stack[len(stack)-1].wantKey {
			top := &stack[len(stack)-1]
			if !names[key] || top.keys[key] {
				return ErrKeys
			}
			top.keys[key] = true
			top.wantKey = false
			continue
		}
		// A value was completed.
		if len(stack) == 0 {
			return nil
		}
		if top := &stack[len(stack)-1]; top.object {
			top.wantKey = true
		}
	}
}

// reportKeys are the exact key spellings of the VC-2 schema.
var reportKeys = map[string]bool{"claims": true, "issues": true, "claim_id": true, "verdict": true, "span_ids": true,
	"reason": true, "category": true}
