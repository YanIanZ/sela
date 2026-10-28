package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/httpapi"
	"github.com/gorilla/websocket"
)

// mockMeta is a fake Meta realtime endpoint: it records the handshake,
// answers the ack, then turns endStream into one final transcript.
type mockMeta struct {
	handshake []byte
}

func (m *mockMeta) handler() http.HandlerFunc {
	up := websocket.Upgrader{}
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
		mt, hs, err := c.ReadMessage()
		if err != nil || mt != websocket.TextMessage {
			return
		}
		m.handshake = hs
		_ = c.WriteMessage(websocket.TextMessage, []byte(`{"sessionId":"mock-session"}`))
		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			if mt != websocket.TextMessage {
				continue
			}
			var ctl struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(msg, &ctl) != nil || ctl.Type != "endStream" {
				continue
			}
			_ = c.WriteMessage(websocket.TextMessage, []byte(`{"type":"transcript","transcript":"Halo dunia","final":true}`))
			return
		}
	}
}

func asrGuest(t *testing.T, e *v3env) *v3client {
	t.Helper()
	c := e.client()
	c.guest("18_plus")
	c.must(200, "PUT", "/api/v3/me/consents/asr_meta",
		map[string]any{"granted": true, "policyVersion": policyVersion("asr_meta")})
	return c
}

func dialASRRaw(wsURL string, tok string, origin string) (*websocket.Conn, *http.Response, error) {
	h := http.Header{}
	if tok != "" {
		h.Set("Cookie", "sela_auth="+tok)
	}
	if origin != "" {
		h.Set("Origin", origin)
	}
	return websocket.DefaultDialer.Dial(wsURL, h)
}

func dialASR(t *testing.T, wsURL string, tok string, origin string) (*websocket.Conn, *http.Response, error) {
	conn, resp, err := dialASRRaw(wsURL, tok, origin)
	if err != nil {
		status := -1
		body := ""
		if resp != nil {
			status = resp.StatusCode
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			body = string(raw)
			if len(body) > 200 {
				body = body[:200]
			}
		}
		t.Fatalf("dial: %v (status %d body %q)", err, status, body)
	}
	return conn, resp, err
}

func asrURL(e *v3env) string {
	return "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/api/v3/asr/realtime"
}

func readFinal(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if mt != websocket.TextMessage {
			continue
		}
		var ev struct {
			Type       string `json:"type"`
			Transcript string `json:"transcript"`
			Final      bool   `json:"final"`
		}
		if json.Unmarshal(msg, &ev) != nil || ev.Type != "transcript" {
			continue
		}
		if ev.Final {
			return ev.Transcript
		}
	}
}

// readError reads one error event plus the application close code.
func readError(t *testing.T, conn *websocket.Conn) (string, int) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	msg := ""
	for {
		mt, raw, err := conn.ReadMessage()
		if err != nil {
			if ce, ok := err.(*websocket.CloseError); ok {
				return msg, ce.Code
			}
			t.Fatalf("read: %v", err)
		}
		if mt != websocket.TextMessage {
			continue
		}
		var ev struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &ev) == nil && ev.Type == "error" {
			msg = ev.Message
		}
	}
}

func TestASRProxyForwardsToMeta(t *testing.T) {
	meta := &mockMeta{}
	metaSrv := httptest.NewServer(meta.handler())
	defer metaSrv.Close()
	e := newV3(t, v3opt{mutateO: func(o *httpapi.Options) {
		o.ASRMode = "meta"
		o.ASRAPIKey = "test-key"
		o.ASRModel = "muse-voice-transcribe-1.0"
		o.ASRURL = "ws" + strings.TrimPrefix(metaSrv.URL, "http")
	}})
	c := asrGuest(t, e)
	conn, _, err := dialASR(t, asrURL(e), c.token(), e.srv.URL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.WriteMessage(websocket.BinaryMessage, []byte{1, 2, 3, 4})
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"endStream"}`))
	if got := readFinal(t, conn); got != "Halo dunia" {
		t.Fatalf("transcript = %q", got)
	}
	var hs struct {
		Authorization map[string]string `json:"authorization"`
		Model         string            `json:"model"`
		Mode          string            `json:"mode"`
	}
	if err := json.Unmarshal(meta.handshake, &hs); err != nil {
		t.Fatalf("handshake not JSON: %v", err)
	}
	if hs.Authorization["accessToken"] != "Bearer test-key" {
		t.Fatalf("handshake auth = %v", hs.Authorization)
	}
	if hs.Model != "muse-voice-transcribe-1.0" || hs.Mode != "PUSH_TO_TALK" {
		t.Fatalf("handshake model/mode = %v", meta.handshake)
	}
}

func TestASRSonicModeWithoutCredentialsFailsBackend(t *testing.T) {
	// Bedrock mode with no IAM credentials in the environment: the opener
	// fails validation and the browser is told the backend is unreachable.
	// This exercises the mode dispatch + consent gate without needing AWS.
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SESSION_TOKEN", "")
	e := newV3(t, v3opt{mutateO: func(o *httpapi.Options) {
		o.ASRMode = "bedrock"
		o.ASRModel = "amazon.nova-2-5-sonic"
		o.AWSRegion = "us-east-1"
	}})
	c := asrGuest(t, e)
	conn, _, err := dialASR(t, asrURL(e), c.token(), e.srv.URL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	msg, _ := readError(t, conn)
	if !strings.Contains(msg, "asr_backend_unreachable") {
		t.Fatalf("got %q, want asr_backend_unreachable", msg)
	}
}

func TestASRRequiresConsent(t *testing.T) {
	e := newV3(t, v3opt{mutateO: func(o *httpapi.Options) { o.ASRMode = "test_double" }})
	c := e.client()
	c.guest("18_plus")
	conn, _, err := dialASR(t, asrURL(e), c.token(), e.srv.URL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	msg, code := readError(t, conn)
	if msg != "asr_consent_required" || code != 4403 {
		t.Fatalf("got %q code %d", msg, code)
	}
}

func TestASRDisabledByDefault(t *testing.T) {
	e := newV3(t, v3opt{})
	c := e.client()
	c.guest("18_plus")
	conn, _, err := dialASR(t, asrURL(e), c.token(), e.srv.URL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	msg, code := readError(t, conn)
	if msg != "asr_unavailable" || code != 4413 {
		t.Fatalf("got %q code %d", msg, code)
	}
}

func TestASRLoopbackAnswersEndStream(t *testing.T) {
	e := newV3(t, v3opt{mutateO: func(o *httpapi.Options) { o.ASRMode = "test_double" }})
	c := asrGuest(t, e)
	conn, _, err := dialASR(t, asrURL(e), c.token(), e.srv.URL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.WriteMessage(websocket.BinaryMessage, []byte{9, 9})
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"endStream"}`))
	if got := readFinal(t, conn); !strings.Contains(got, "uji") {
		t.Fatalf("loopback transcript = %q", got)
	}
}

func TestASRRejectsCrossOrigin(t *testing.T) {
	e := newV3(t, v3opt{mutateO: func(o *httpapi.Options) { o.ASRMode = "test_double" }})
	c := asrGuest(t, e)
	_, resp, err := dialASRRaw(asrURL(e), c.token(), "https://evil.example")
	if err == nil {
		t.Fatal("cross-origin dial succeeded")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %v, err = %v", resp, err)
	}
}

func TestASROversizedFrameEndsSide(t *testing.T) {
	meta := &mockMeta{}
	metaSrv := httptest.NewServer(meta.handler())
	defer metaSrv.Close()
	e := newV3(t, v3opt{mutateO: func(o *httpapi.Options) {
		o.ASRMode = "meta"
		o.ASRAPIKey = "test-key"
		o.ASRModel = "muse-voice-transcribe-1.0"
		o.ASRURL = "ws" + strings.TrimPrefix(metaSrv.URL, "http")
	}})
	c := asrGuest(t, e)
	conn, _, err := dialASR(t, asrURL(e), c.token(), e.srv.URL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	// One 128 KiB frame is ~2.7 s of 24 kHz PCM: above the per-frame cap.
	big := make([]byte, 128*1024)
	if err := conn.WriteMessage(websocket.BinaryMessage, big); err != nil {
		t.Fatalf("write big frame: %v", err)
	}
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"endStream"}`))
	// The upstream side ends at the cap, so the socket closes with a
	// normal close frame before any transcript can be forwarded.
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		mt, _, err := conn.ReadMessage()
		if err != nil {
			if ce, ok := err.(*websocket.CloseError); ok && ce.Code == websocket.CloseNormalClosure {
				return
			}
			t.Fatalf("read: %v", err)
		}
		_ = mt
	}
}
