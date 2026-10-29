package sonic

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
	"time"
)

func mustDecode(t *testing.T, d *decoder, s string) []Event {
	t.Helper()
	evs, err := d.decode([]byte(s))
	if err != nil {
		t.Fatalf("decode %s: %v", s, err)
	}
	return evs
}

func TestDecoderRolesStagesAndAudio(t *testing.T) {
	d := newDecoder()
	mustDecode(t, d, `{"event":{"contentStart":{"contentId":"u1","role":"USER","type":"TEXT"}}}`)
	evs := mustDecode(t, d, `{"event":{"textOutput":{"contentId":"u1","role":"USER","content":"aku capek"}}}`)
	if len(evs) != 1 || evs[0].Kind != KindUserTranscript || evs[0].Text != "aku capek" {
		t.Fatalf("user transcript: %+v", evs)
	}
	mustDecode(t, d, `{"event":{"contentStart":{"contentId":"s1","role":"ASSISTANT","type":"TEXT","additionalModelFields":"{\"generationStage\":\"SPECULATIVE\"}"}}}`)
	evs = mustDecode(t, d, `{"event":{"textOutput":{"contentId":"s1","role":"ASSISTANT","content":"Aku dengar."}}}`)
	if len(evs) != 1 || evs[0].Kind != KindAssistantText || !evs[0].Speculative {
		t.Fatalf("speculative assistant text: %+v", evs)
	}
	mustDecode(t, d, `{"event":{"contentStart":{"contentId":"a1","role":"ASSISTANT","type":"AUDIO"}}}`)
	pcm := []byte{1, 0, 2, 0}
	evs = mustDecode(t, d, `{"event":{"audioOutput":{"contentId":"a1","content":"`+base64.StdEncoding.EncodeToString(pcm)+`"}}}`)
	if len(evs) != 1 || evs[0].Kind != KindAssistantAudio || len(evs[0].PCM) != 4 {
		t.Fatalf("audio: %+v", evs)
	}
	evs = mustDecode(t, d, `{"event":{"contentEnd":{"contentId":"a1","type":"AUDIO","stopReason":"END_TURN"}}}`)
	if len(evs) != 1 || evs[0].Kind != KindAssistantAudioEnd || evs[0].StopReason != "END_TURN" {
		t.Fatalf("audio end: %+v", evs)
	}
	mustDecode(t, d, `{"event":{"contentStart":{"contentId":"f1","role":"ASSISTANT","type":"TEXT","additionalModelFields":"{\"generationStage\":\"FINAL\"}"}}}`)
	evs = mustDecode(t, d, `{"event":{"textOutput":{"contentId":"f1","role":"ASSISTANT","content":"Aku dengar."}}}`)
	if evs[0].Speculative {
		t.Fatal("final text marked speculative")
	}
}

func TestDecoderInterruption(t *testing.T) {
	d := newDecoder()
	evs := mustDecode(t, d, `{"event":{"textOutput":{"contentId":"x","role":"ASSISTANT","content":"{ \"interrupted\" : true }"}}}`)
	if len(evs) != 1 || evs[0].Kind != KindInterrupted {
		t.Fatalf("barge-in marker: %+v", evs)
	}
	mustDecode(t, d, `{"event":{"contentStart":{"contentId":"a","role":"ASSISTANT","type":"AUDIO"}}}`)
	evs = mustDecode(t, d, `{"event":{"contentEnd":{"contentId":"a","type":"AUDIO","stopReason":"INTERRUPTED"}}}`)
	if len(evs) != 1 || evs[0].Kind != KindInterrupted {
		t.Fatalf("interrupted end: %+v", evs)
	}
}

func TestDecoderIgnoresUnknownEvents(t *testing.T) {
	d := newDecoder()
	if evs := mustDecode(t, d, `{"event":{"usageEvent":{"totalTokens":3}}}`); len(evs) != 0 {
		t.Fatalf("unexpected events: %+v", evs)
	}
}

func TestConfigNeedsIAMCredentialsNamedNotValued(t *testing.T) {
	err := Config{AccessKeyID: "AKIASECRETVALUE"}.Validate()
	if err == nil || !strings.Contains(err.Error(), "AWS_SECRET_ACCESS_KEY") || strings.Contains(err.Error(), "AKIASECRETVALUE") {
		t.Fatalf("validate: %v", err)
	}
	if _, err := NewBedrock(Config{}); err == nil {
		t.Fatal("NewBedrock accepted empty credentials")
	}
}

func loud(ms int) []byte {
	n := InputSampleRate * ms / 1000
	b := make([]byte, 2*n)
	for i := 0; i < n; i++ {
		v := int16(8000)
		if i%2 == 0 {
			v = -8000
		}
		binary.LittleEndian.PutUint16(b[2*i:], uint16(v))
	}
	return b
}

func TestTestDoubleEmitsLabelledTurnAfterSpeechAndQuiet(t *testing.T) {
	s, _ := TestDouble{}.Open(context.Background(), SessionOptions{})
	defer s.Close()
	_ = s.SendAudio(loud(500))
	var kinds []Kind
	timeout := time.After(3 * time.Second)
	for len(kinds) < 5 {
		select {
		case e := <-s.Events():
			kinds = append(kinds, e.Kind)
			if e.Kind == KindAssistantText && !strings.Contains(e.Text, "TEST DOUBLE") {
				t.Fatalf("unlabelled double reply: %q", e.Text)
			}
		case <-timeout:
			t.Fatalf("no turn; got %v", kinds)
		}
	}
	if kinds[0] != KindUserTranscript || kinds[2] != KindAssistantAudio {
		t.Fatalf("order: %v", kinds)
	}
	if err := s.SendAudio(loud(10)); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	if err := s.SendAudio(loud(10)); err != ErrClosed {
		t.Fatalf("send after close: %v", err)
	}
}
