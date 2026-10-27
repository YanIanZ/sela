package deepseek

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YanIanZ/sela/internal/ai/transport"
)

// TestServedModelCheck: until the response field is calibrated (B-9) the
// response model may be the alias or the expected version; with the pin
// only the version; anything else is a model mismatch.
func TestServedModelCheck(t *testing.T) {
	for _, tc := range []struct {
		model string
		pin   bool
		ok    bool
	}{
		{"deepseek-flash", false, true},
		{"DeepSeek-V4.1-Flash-1017", false, true},
		{"deepseek-v4.1-flash-1017", false, true},
		{"deepseek-v4-pro", false, false},
		{"deepseek-flash", true, false},
		{"DeepSeek-V4.1-Flash-1017", true, true},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"model":"`+tc.model+`","choices":[{"index":0,"message":{"role":"assistant","content":"{}"},"finish_reason":"stop"}]}`)
		}))
		c := &Client{cfg: Config{Model: DefaultModel, ExpectedVersion: DefaultExpectedVersion, PinServedVersion: tc.pin,
			AttemptTimeout: DefaultAttemptTimeout, MaxOutputBytes: DefaultMaxOutputBytes, MaxRetries: -1},
			hc: transport.HTTPClient(nil), endpoint: srv.URL + "/chat/completions"}
		_, err := c.Complete(context.Background(), Request{System: "json", User: "x"})
		srv.Close()
		if (err == nil) != tc.ok {
			t.Errorf("model %s pin %v: err %v", tc.model, tc.pin, err)
		}
		if err != nil && transport.ReasonOf(err) != transport.ReasonModelMismatch {
			t.Errorf("model %s: reason %v", tc.model, err)
		}
	}
}

// TestLiveClientRefusesTestDoubleLabel: a live client answered by a
// labelled test double fails with an integrity reason.
func TestLiveClientRefusesTestDoubleLabel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(transport.HeaderTestDouble, "true")
		_, _ = io.WriteString(w, `{"model":"deepseek-flash","choices":[{"index":0,"message":{"role":"assistant","content":"{}"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	c := &Client{cfg: Config{Model: DefaultModel, ExpectedVersion: DefaultExpectedVersion, AttemptTimeout: DefaultAttemptTimeout,
		MaxOutputBytes: DefaultMaxOutputBytes, MaxRetries: -1}, hc: transport.HTTPClient(nil), endpoint: srv.URL + "/chat/completions"}
	if _, err := c.Complete(context.Background(), Request{System: "json", User: "x"}); transport.ReasonOf(err) != transport.ReasonTestDouble {
		t.Fatalf("err %v", err)
	}
}

func TestShapeChecks(t *testing.T) {
	for name, tc := range map[string]struct {
		body   string
		reason transport.Reason
	}{
		"two choices":  {`{"model":"test-double-not-deepseek","choices":[{"index":0,"message":{"content":"{}"},"finish_reason":"stop"},{"index":1,"message":{"content":"{}"},"finish_reason":"stop"}]}`, transport.ReasonInvalidOutput},
		"no finish":    {`{"model":"test-double-not-deepseek","choices":[{"index":0,"message":{"content":"{}"}}]}`, transport.ReasonInvalidOutput},
		"odd finish":   {`{"model":"test-double-not-deepseek","choices":[{"index":0,"message":{"content":"{}"},"finish_reason":"tool_calls"}]}`, transport.ReasonInvalidOutput},
		"resource":     {`{"model":"test-double-not-deepseek","choices":[{"index":0,"message":{"content":"{}"},"finish_reason":"insufficient_system_resource"}]}`, transport.ReasonTruncated},
		"null content": {`{"model":"test-double-not-deepseek","choices":[{"index":0,"message":{"content":null},"finish_reason":"stop"}]}`, transport.ReasonEmptyContent},
		"not json":     {`<html>`, transport.ReasonInvalidOutput},
		"oversize":     {`{"model":"test-double-not-deepseek","choices":[{"index":0,"message":{"content":"` + stringsRepeat("a", DefaultMaxOutputBytes+1) + `"},"finish_reason":"stop"}]}`, transport.ReasonInvalidOutput},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(transport.HeaderTestDouble, "true")
			_, _ = io.WriteString(w, tc.body)
		}))
		c, err := New(Config{TestDouble: true, BaseURL: srv.URL, MaxRetries: -1})
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.Complete(context.Background(), Request{System: "json", User: "x"})
		srv.Close()
		if transport.ReasonOf(err) != tc.reason {
			t.Errorf("%s: err %v, want %s", name, err, tc.reason)
		}
	}
}

func stringsRepeat(s string, n int) string {
	b := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}
