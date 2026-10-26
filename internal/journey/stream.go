package journey

import "encoding/json"

// arrayStreamer incrementally extracts complete elements of one top-level
// JSON array field (e.g. "claims") from a model's streamed JSON text, so
// validated items can be previewed before generation ends. It never trusts
// partial elements; the final commit always re-parses the whole output.
type arrayStreamer struct {
	field   string
	buf     []byte
	pos     int // scan position in buf
	state   int // 0 searching key, 1 searching '[', 2 in array, 3 done
	depth   int // nesting depth inside the array (0 = between elements)
	inStr   bool
	escape  bool
	elStart int
	emitted int
	// keyDepth tracks object depth outside the array to only match the
	// top-level key.
	objDepth int
	lastStr  []byte
	strStart int
}

func newArrayStreamer(field string) *arrayStreamer { return &arrayStreamer{field: field, elStart: -1} }

// Feed appends text and returns any newly completed raw array elements.
func (a *arrayStreamer) Feed(chunk string) []json.RawMessage {
	a.buf = append(a.buf, chunk...)
	var out []json.RawMessage
	for ; a.pos < len(a.buf); a.pos++ {
		c := a.buf[a.pos]
		if a.state == 3 {
			break
		}
		if a.inStr {
			if a.escape {
				a.escape = false
				continue
			}
			switch c {
			case '\\':
				a.escape = true
			case '"':
				a.inStr = false
				if a.state == 0 && a.objDepth == 1 {
					a.lastStr = a.buf[a.strStart:a.pos]
				}
			}
			continue
		}
		switch a.state {
		case 0:
			switch c {
			case '"':
				a.inStr, a.strStart = true, a.pos+1
			case '{', '[':
				a.objDepth++
			case '}', ']':
				a.objDepth--
			case ':':
				if a.objDepth == 1 && string(a.lastStr) == a.field {
					a.state = 1
				}
				a.lastStr = nil
			case ',':
				a.lastStr = nil
			}
		case 1:
			switch c {
			case '[':
				a.state, a.depth = 2, 0
			case ' ', '\n', '\r', '\t':
			default:
				a.state = 3 // not an array: give up previewing
			}
		case 2:
			switch c {
			case '"':
				a.inStr = true
				if a.depth == 0 && a.elStart < 0 {
					a.elStart = a.pos
				}
			case '{', '[':
				if a.depth == 0 {
					a.elStart = a.pos
				}
				a.depth++
			case '}', ']':
				if a.depth == 0 && c == ']' {
					if a.elStart >= 0 {
						out = append(out, json.RawMessage(append([]byte{}, a.buf[a.elStart:a.pos]...)))
						a.elStart = -1
						a.emitted++
					}
					a.state = 3
					continue
				}
				a.depth--
				if a.depth == 0 && a.elStart >= 0 {
					out = append(out, json.RawMessage(append([]byte{}, a.buf[a.elStart:a.pos+1]...)))
					a.elStart = -1
					a.emitted++
				}
			case ',':
				if a.depth == 0 && a.elStart >= 0 {
					// a completed scalar element (e.g. a string line)
					out = append(out, json.RawMessage(append([]byte{}, a.buf[a.elStart:a.pos]...)))
					a.elStart = -1
					a.emitted++
				}
			}
		}
	}
	return out
}
