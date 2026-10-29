package httpapi_test

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai/sonic"
	"github.com/YanIanZ/sela/internal/httpapi"
	"github.com/gorilla/websocket"
)

func newVoiceEnv(t *testing.T, opener sonic.Opener) *v3env {
	return newV3(t, v3opt{
		mutateC: func(c *account.Config) {
			c.Purposes = append(c.Purposes, account.Purpose{Name: httpapi.PolicyVoiceAWS, Kinds: []string{account.KindGuest, account.KindAccount}})
		},
		mutateO: func(o *httpapi.Options) {
			if opener != nil {
				o.VoiceOpener = opener
			}
		},
	})
}

func voiceGuest(t *testing.T, e *v3env, band string, consent bool) *v3client {
	t.Helper()
	c := e.client()
	c.guest(band)
	if consent {
		c.must(200, "PUT", "/api/v3/me/consents/"+httpapi.PolicyVoiceAWS,
			map[string]any{"granted": true, "policyVersion": policyVersion(httpapi.PolicyVoiceAWS)})
	}
	return c
}

func dialVoice(t *testing.T, e *v3env, c *v3client) *websocket.Conn {
	t.Helper()
	h := http.Header{}
	h.Set("Cookie", "sela_auth="+c.token())
	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(e.srv.URL, "http")+"/api/v3/voice/realtime", h)
	if err != nil {
		status := -1
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial voice: %v (status %d)", err, status)
	}
	return conn
}

type voiceMsg struct {
	Type       string `json:"type"`
	Text       string `json:"text"`
	Message    string `json:"message"`
	Model      string `json:"model"`
	TestDouble bool   `json:"testDouble"`
}

// readVoice collects messages until the socket closes or until stop says so.
func readVoice(t *testing.T, conn *websocket.Conn, stop func([]voiceMsg, int) bool) ([]voiceMsg, int, int) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var msgs []voiceMsg
	audio := 0
	for {
		mt, raw, err := conn.ReadMessage()
		if err != nil {
			if ce, ok := err.(*websocket.CloseError); ok {
				return msgs, audio, ce.Code
			}
			t.Fatalf("read: %v (got %+v)", err, msgs)
		}
		if mt == websocket.BinaryMessage {
			audio += len(raw)
		} else {
			var m voiceMsg
			_ = json.Unmarshal(raw, &m)
			msgs = append(msgs, m)
		}
		if stop != nil && stop(msgs, audio) {
			return msgs, audio, 0
		}
	}
}

func voiceSpeech(ms int) []byte {
	n := sonic.InputSampleRate * ms / 1000
	b := make([]byte, 2*n)
	for i := 0; i < n; i++ {
		v := int16(9000)
		if i%2 == 0 {
			v = -9000
		}
		binary.LittleEndian.PutUint16(b[2*i:], uint16(v))
	}
	return b
}

func voiceTypes(msgs []voiceMsg) []string {
	var s []string
	for _, m := range msgs {
		s = append(s, m.Type)
	}
	return s
}

func TestVoiceRefusesBeforeAnyAudio(t *testing.T) {
	cases := []struct {
		name    string
		opener  sonic.Opener
		band    string
		consent bool
		code    int
		msg     string
	}{
		{"disabled", nil, "18_plus", true, 4413, "voice_unavailable"},
		{"no consent", sonic.TestDouble{}, "18_plus", false, 4403, "voice_consent_required"},
		{"minor", sonic.TestDouble{}, "15_17", true, 4451, "voice_age_band"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newVoiceEnv(t, tc.opener)
			c := voiceGuest(t, e, tc.band, tc.consent && tc.band == "18_plus")
			if tc.consent && tc.band != "18_plus" {
				// A minor cannot even hold the consent in a useful way; the
				// band check runs first regardless.
				c.call("PUT", "/api/v3/me/consents/"+httpapi.PolicyVoiceAWS,
					map[string]any{"granted": true, "policyVersion": policyVersion(httpapi.PolicyVoiceAWS)})
			}
			conn := dialVoice(t, e, c)
			defer conn.Close()
			msgs, _, code := readVoice(t, conn, nil)
			if code != tc.code || len(msgs) != 1 || msgs[0].Message != tc.msg {
				t.Fatalf("got code %d msgs %+v, want %d %s", code, msgs, tc.code, tc.msg)
			}
		})
	}
}

func TestVoiceTurnIsLabelledAndAudioFollowsCheckedText(t *testing.T) {
	e := newVoiceEnv(t, sonic.TestDouble{})
	c := voiceGuest(t, e, "18_plus", true)
	conn := dialVoice(t, e, c)
	defer conn.Close()
	msgs, _, _ := readVoice(t, conn, func(m []voiceMsg, _ int) bool { return len(m) == 1 })
	if msgs[0].Type != "ready" || !msgs[0].TestDouble || msgs[0].Model != sonic.TestDoubleModel {
		t.Fatalf("ready: %+v", msgs)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, voiceSpeech(500)); err != nil {
		t.Fatal(err)
	}
	msgs, audio, _ := readVoice(t, conn, func(m []voiceMsg, _ int) bool {
		return len(m) > 0 && m[len(m)-1].Type == "reply" && strings.Contains(m[len(m)-1].Text, "TEST DOUBLE") && len(m) >= 4
	})
	got := strings.Join(voiceTypes(msgs), ",")
	if got != "transcript,reply,replyEnd,reply" || audio == 0 {
		t.Fatalf("turn: %s audio=%d", got, audio)
	}
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"end"}`))
	_, _, code := readVoice(t, conn, nil)
	if code != websocket.CloseNormalClosure {
		t.Fatalf("end close code %d", code)
	}
}

func TestVoiceCrisisEndsSessionWithoutReplyAudio(t *testing.T) {
	e := newVoiceEnv(t, sonic.TestDouble{Transcript: func(int) string { return "aku mau bunuh diri malam ini" }})
	c := voiceGuest(t, e, "18_plus", true)
	conn := dialVoice(t, e, c)
	defer conn.Close()
	readVoice(t, conn, func(m []voiceMsg, _ int) bool { return len(m) == 1 })
	_ = conn.WriteMessage(websocket.BinaryMessage, voiceSpeech(500))
	msgs, audio, code := readVoice(t, conn, nil)
	if code != 4409 || audio != 0 || strings.Join(voiceTypes(msgs), ",") != "transcript,crisis" {
		t.Fatalf("crisis: code %d audio %d msgs %v", code, audio, voiceTypes(msgs))
	}
}

func TestVoiceOneSessionPerPerson(t *testing.T) {
	e := newVoiceEnv(t, sonic.TestDouble{})
	c := voiceGuest(t, e, "18_plus", true)
	first := dialVoice(t, e, c)
	defer first.Close()
	readVoice(t, first, func(m []voiceMsg, _ int) bool { return len(m) == 1 })
	second := dialVoice(t, e, c)
	defer second.Close()
	msgs, _, code := readVoice(t, second, nil)
	if code != 4429 || len(msgs) != 1 || msgs[0].Message != "voice_busy" {
		t.Fatalf("second session: code %d msgs %+v", code, msgs)
	}
}
