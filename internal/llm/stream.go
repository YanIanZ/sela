package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// maxLineSlack is added to MaxOutputBytes to bound one SSE line; a line
// longer than that is a protocol error.
const maxLineSlack = 64 << 10

// finishOther replaces a finish_reason outside knownFinishReasons.
const finishOther = "other"

// knownFinishReasons are the finish_reason values of OpenAI-compatible
// servers (vLLM adds "abort"). Only these are kept verbatim, so a server
// string never reaches a Result, an error message or a log.
var knownFinishReasons = map[string]bool{
	"stop": true, "length": true, "abort": true, "content_filter": true, "tool_calls": true, "function_call": true,
}

// normalizeFinishReason returns reason when it is a known value and
// finishOther otherwise.
func normalizeFinishReason(reason string) string {
	if knownFinishReasons[reason] {
		return reason
	}
	return finishOther
}

// streamChunk is one OpenAI-compatible chat.completion.chunk. Fields other
// than delta.content (for example delta.reasoning from vLLM's reasoning
// parser) are ignored.
type streamChunk struct {
	Model   string `json:"model"`
	Object  string `json:"object"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content *string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	// Error is set by servers that report a failure inside the stream, as
	// {"error": {...}} or as an object of type "error" with a "code".
	Error json.RawMessage `json:"error"`
	Code  json.RawMessage `json:"code"`
}

// streamReader assembles one streamed completion.
type streamReader struct {
	c       *client
	onDelta func(string) error
	st      *attemptState
	text    strings.Builder
	res     Result
	chunks  int
	done    bool
}

// readStream parses the server-sent events in body. It returns when the
// stream sends [DONE], ends, fails or the context ends; the caller closes
// body.
func (c *client) readStream(ctx, actx context.Context, body io.Reader, onDelta func(string) error, st *attemptState) (Result, error) {
	sr := &streamReader{c: c, onDelta: onDelta, st: st}
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 4096), c.cfg.MaxOutputBytes+maxLineSlack)
	var data []byte
	pending := false
	for !sr.done && sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			if pending {
				if err := sr.dispatch(data); err != nil {
					return Result{}, err
				}
				data, pending = data[:0], false
			}
			continue
		}
		field, value, _ := bytes.Cut(line, []byte(":"))
		if len(field) == 0 {
			continue // comment, e.g. a keep-alive
		}
		if string(field) != "data" {
			continue // event, id and retry fields carry nothing we use
		}
		value = bytes.TrimPrefix(value, []byte(" "))
		if pending {
			data = append(data, '\n')
		}
		data = append(data, value...)
		pending = true
	}
	if !sr.done {
		if err := sc.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				return Result{}, protocolError("stream line too long")
			}
			// The body failed mid-stream: the context ended or the
			// connection broke. A broken connection may be retried only
			// while no content has been delivered.
			if ctx.Err() == nil && actx.Err() == nil {
				st.retryable = true
			}
			return Result{}, transportError(ctx, actx, err)
		}
		if pending {
			if err := sr.dispatch(data); err != nil {
				return Result{}, err
			}
		}
	}
	return sr.finish()
}

// dispatch handles one SSE event's data.
func (sr *streamReader) dispatch(data []byte) error {
	if string(bytes.TrimSpace(data)) == "[DONE]" {
		sr.done = true
		return nil
	}
	var ch streamChunk
	if err := json.Unmarshal(data, &ch); err != nil {
		return protocolError("malformed stream chunk")
	}
	if e := streamError(ch); e != nil {
		return e
	}
	if ch.Model != sr.c.cfg.ServedModel {
		return &Error{Kind: KindModelMismatch, Status: sr.st.status, Err: errors.New("server reported a different model")}
	}
	sr.chunks++
	for _, choice := range ch.Choices {
		if choice.Index != 0 {
			continue
		}
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			sr.res.FinishReason = normalizeFinishReason(*choice.FinishReason)
		}
		if choice.Delta.Content == nil || *choice.Delta.Content == "" {
			continue
		}
		delta := *choice.Delta.Content
		if sr.text.Len()+len(delta) > sr.c.cfg.MaxOutputBytes {
			return &Error{Kind: KindOutputTooLong, Status: sr.st.status, Err: errors.New("output exceeds the configured limit")}
		}
		sr.text.WriteString(delta)
		sr.st.started = true
		if err := sr.onDelta(delta); err != nil {
			return err
		}
	}
	if ch.Usage != nil {
		sr.res.PromptTokens = ch.Usage.PromptTokens
		sr.res.CompletionTokens = ch.Usage.CompletionTokens
	}
	return nil
}

// finish checks that the stream completed and returns the result.
func (sr *streamReader) finish() (Result, error) {
	switch {
	case !sr.done:
		return Result{}, protocolError("stream ended before [DONE]")
	case sr.chunks == 0:
		return Result{}, protocolError("stream contained no chunks")
	case sr.res.FinishReason == "":
		return Result{}, protocolError("stream ended without a finish reason")
	}
	sr.res.Text = sr.text.String()
	if strings.TrimSpace(sr.res.Text) == "" {
		// A reasoning model that runs out of tokens while thinking returns
		// no content with finish_reason "length" (or sometimes "stop").
		// FinishReason is normalized, so the message names a known value
		// or "other", never the server's string.
		return Result{}, protocolError("model returned no content (finish reason " + sr.res.FinishReason + ")")
	}
	sr.res.Model = sr.c.cfg.ServedModel
	return sr.res, nil
}

// streamError reports an error event inside the stream, classified by its
// status code when one is given. The provider's message is never kept.
func streamError(ch streamChunk) *Error {
	hasErr := len(ch.Error) > 0 && string(ch.Error) != "null"
	if !hasErr && ch.Object != "error" {
		return nil
	}
	code := intCode(ch.Code)
	if code == 0 && hasErr {
		var inner struct {
			Code json.RawMessage `json:"code"`
		}
		if json.Unmarshal(ch.Error, &inner) == nil {
			code = intCode(inner.Code)
		}
	}
	e := &Error{Kind: KindUnavailable, Status: code, Err: errors.New("server reported an error in the stream")}
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		e.Kind = KindAuth
	case code == http.StatusTooManyRequests:
		e.Kind = KindRateLimited
	case code >= 400 && code < 500:
		e.Kind = KindRejected
	}
	return e
}

// intCode reads an HTTP-like status code given as a JSON number or string.
func intCode(raw json.RawMessage) int {
	var n int
	if json.Unmarshal(raw, &n) == nil && n >= 100 && n <= 599 {
		return n
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if json.Unmarshal([]byte(s), &n) == nil && n >= 100 && n <= 599 {
			return n
		}
	}
	return 0
}

func protocolError(msg string) *Error {
	return &Error{Kind: KindProtocol, Status: http.StatusOK, Err: errors.New(msg)}
}
