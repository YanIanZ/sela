package httpapi

import (
	"testing"

	"github.com/YanIanZ/sela/internal/ai/sonic"
)

func kinds(out []voiceOut) []string {
	var s []string
	for _, o := range out {
		if o.audio != nil {
			s = append(s, "audio")
			continue
		}
		s = append(s, o.msg["type"].(string))
	}
	return s
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestVoiceGateHoldsAudioUntilTextPasses(t *testing.T) {
	g := newVoiceGate()
	if out := g.handle(sonic.Event{Kind: sonic.KindAssistantAudio, PCM: []byte{1, 0}}); len(out) != 0 {
		t.Fatalf("audio before text leaked: %v", kinds(out))
	}
	out := g.handle(sonic.Event{Kind: sonic.KindAssistantText, Speculative: true, Text: "Aku mendengarmu. Ceritakan pelan-pelan."})
	if !eq(kinds(out), []string{"reply", "audio"}) {
		t.Fatalf("release: %v", kinds(out))
	}
	if out := g.handle(sonic.Event{Kind: sonic.KindAssistantAudio, PCM: []byte{2, 0}}); !eq(kinds(out), []string{"audio"}) {
		t.Fatalf("open gate: %v", kinds(out))
	}
	if out := g.handle(sonic.Event{Kind: sonic.KindAssistantAudioEnd}); !eq(kinds(out), []string{"replyEnd"}) {
		t.Fatalf("end: %v", kinds(out))
	}
	// The next block waits for its own text again.
	if out := g.handle(sonic.Event{Kind: sonic.KindAssistantAudio, PCM: []byte{3, 0}}); len(out) != 0 {
		t.Fatalf("next block leaked: %v", kinds(out))
	}
}

func TestVoiceGateWithholdsGuardedReplies(t *testing.T) {
	for name, text := range map[string]string{
		"diagnosis":   "Kamu mengalami depresi berat.",
		"medication":  "Minum sertraline 50 mg setiap pagi.",
		"crisis text": "Aku juga ingin bunuh diri.",
	} {
		t.Run(name, func(t *testing.T) {
			g := newVoiceGate()
			g.handle(sonic.Event{Kind: sonic.KindAssistantAudio, PCM: []byte{1, 0}})
			out := g.handle(sonic.Event{Kind: sonic.KindAssistantText, Speculative: true, Text: text})
			if !eq(kinds(out), []string{"withheld"}) {
				t.Fatalf("not withheld: %v", kinds(out))
			}
			if out := g.handle(sonic.Event{Kind: sonic.KindAssistantAudio, PCM: []byte{2, 0}}); len(out) != 0 {
				t.Fatalf("withheld audio played: %v", kinds(out))
			}
			if out := g.handle(sonic.Event{Kind: sonic.KindAssistantText, Text: text}); len(out) != 0 {
				t.Fatalf("withheld final text shown: %v", kinds(out))
			}
		})
	}
}

func TestVoiceGateCrisisInTranscriptStopsEverything(t *testing.T) {
	g := newVoiceGate()
	g.handle(sonic.Event{Kind: sonic.KindAssistantAudio, PCM: []byte{1, 0}})
	out := g.handle(sonic.Event{Kind: sonic.KindUserTranscript, Text: "aku pengen mati aja, mau bunuh diri malam ini"})
	if !eq(kinds(out), []string{"transcript", "crisis"}) || !g.crisis {
		t.Fatalf("crisis: %v", kinds(out))
	}
	if out := g.handle(sonic.Event{Kind: sonic.KindAssistantText, Speculative: true, Text: "Aku di sini."}); len(out) != 0 {
		t.Fatalf("reply after crisis: %v", kinds(out))
	}
	if out := g.handle(sonic.Event{Kind: sonic.KindAssistantAudio, PCM: []byte{2, 0}}); len(out) != 0 {
		t.Fatalf("audio after crisis: %v", kinds(out))
	}
}

func TestVoiceGateAudioWithoutTextIsNeverPlayed(t *testing.T) {
	g := newVoiceGate()
	g.handle(sonic.Event{Kind: sonic.KindAssistantAudio, PCM: []byte{1, 0}})
	if out := g.handle(sonic.Event{Kind: sonic.KindAssistantAudioEnd}); !eq(kinds(out), []string{"withheld"}) {
		t.Fatalf("textless audio: %v", kinds(out))
	}
}

func TestVoiceGateUnsafeFinalTextRecallsReleasedAudio(t *testing.T) {
	// The speculative text passes and releases the turn's audio; the final
	// text then disagrees and fails the checks (Sonic orders the final text
	// after the audio). The client must get "interrupted" so it stops
	// playback and drops queued audio, and the rest of the turn stays shut.
	g := newVoiceGate()
	out := g.handle(sonic.Event{Kind: sonic.KindAssistantText, Speculative: true, Text: "Aku mendengarmu."})
	if !eq(kinds(out), []string{"reply"}) {
		t.Fatalf("release: %v", kinds(out))
	}
	if out := g.handle(sonic.Event{Kind: sonic.KindAssistantAudio, PCM: []byte{1, 0}}); !eq(kinds(out), []string{"audio"}) {
		t.Fatalf("open gate: %v", kinds(out))
	}
	out = g.handle(sonic.Event{Kind: sonic.KindAssistantText, Text: "Minum sertraline 50 mg setiap pagi."})
	if !eq(kinds(out), []string{"interrupted"}) {
		t.Fatalf("unsafe final text after release: %v", kinds(out))
	}
	if out := g.handle(sonic.Event{Kind: sonic.KindAssistantAudio, PCM: []byte{2, 0}}); len(out) != 0 {
		t.Fatalf("audio flowed after recall: %v", kinds(out))
	}
	if out := g.handle(sonic.Event{Kind: sonic.KindAssistantAudioEnd}); len(out) != 0 {
		t.Fatalf("replyEnd after recall: %v", kinds(out))
	}
	// A safe final text after a released turn still shows normally.
	g = newVoiceGate()
	g.handle(sonic.Event{Kind: sonic.KindAssistantText, Speculative: true, Text: "Aku mendengarmu."})
	g.handle(sonic.Event{Kind: sonic.KindAssistantAudio, PCM: []byte{1, 0}})
	out = g.handle(sonic.Event{Kind: sonic.KindAssistantText, Text: "Aku mendengarmu."})
	if !eq(kinds(out), []string{"reply"}) {
		t.Fatalf("safe final text: %v", kinds(out))
	}
}

func TestVoiceGateInterruptionClearsHeldAudio(t *testing.T) {
	g := newVoiceGate()
	g.handle(sonic.Event{Kind: sonic.KindAssistantAudio, PCM: []byte{1, 0}})
	if out := g.handle(sonic.Event{Kind: sonic.KindInterrupted}); !eq(kinds(out), []string{"interrupted"}) {
		t.Fatalf("interrupt: %v", kinds(out))
	}
	out := g.handle(sonic.Event{Kind: sonic.KindAssistantText, Speculative: true, Text: "Baik, aku dengar."})
	if !eq(kinds(out), []string{"reply"}) {
		t.Fatalf("held audio survived interruption: %v", kinds(out))
	}
}
