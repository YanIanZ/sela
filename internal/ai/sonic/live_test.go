package sonic

import (
	"context"
	"encoding/binary"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLiveBedrock talks to the real Amazon Nova 2.5 Sonic. It runs only with
// SELA_LIVE_SONIC=1 and IAM credentials in the environment, feeds a recorded
// Indonesian utterance (SELA_LIVE_SONIC_WAV, 24 kHz mono PCM16 WAV) as the
// microphone, and expects a transcript and assistant audio back.
func TestLiveBedrock(t *testing.T) {
	if os.Getenv("SELA_LIVE_SONIC") != "1" {
		t.Skip("live Sonic test: set SELA_LIVE_SONIC=1 with IAM credentials")
	}
	wav, err := os.ReadFile(os.Getenv("SELA_LIVE_SONIC_WAV"))
	if err != nil || len(wav) < 44 {
		t.Fatalf("SELA_LIVE_SONIC_WAV: %v", err)
	}
	mic := resample24to16(wav[44:])
	b, err := NewBedrock(Config{
		Region: os.Getenv("AWS_REGION"), AccessKeyID: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"), SessionToken: os.Getenv("AWS_SESSION_TOKEN"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s, err := b.Open(ctx, SessionOptions{System: "Kamu pendengar yang tenang. Jawab singkat dalam bahasa Indonesia."})
	if err != nil {
		t.Fatalf("open (SigV4 must be used even if AWS_BEARER_TOKEN_BEDROCK is set): %v", err)
	}
	defer s.Close()
	go func() {
		frame := InputSampleRate * 2 * 32 / 1000
		for i := 0; i < len(mic); i += frame {
			end := min(i+frame, len(mic))
			_ = s.SendAudio(mic[i:end])
			time.Sleep(32 * time.Millisecond)
		}
	}()
	var transcript, reply strings.Builder
	audio := 0
	for e := range s.Events() {
		switch e.Kind {
		case KindUserTranscript:
			transcript.WriteString(e.Text)
		case KindAssistantText:
			if !e.Speculative {
				reply.WriteString(e.Text)
			}
		case KindAssistantAudio:
			audio += len(e.PCM)
		}
		if audio > 0 && reply.Len() > 0 {
			break
		}
	}
	t.Logf("transcript: %q", transcript.String())
	t.Logf("reply: %q (audio %.1fs)", reply.String(), float64(audio)/float64(OutputSampleRate*2))
	if transcript.Len() == 0 || audio == 0 {
		t.Fatalf("no transcript or audio (err=%v)", s.Err())
	}
}

// resample24to16 converts 24 kHz PCM16 to 16 kHz by linear interpolation.
func resample24to16(in []byte) []byte {
	n := len(in) / 2
	outN := n * 2 / 3
	out := make([]byte, 2*outN)
	for i := 0; i < outN; i++ {
		pos := float64(i) * 1.5
		j := int(pos)
		f := pos - float64(j)
		a := float64(int16(binary.LittleEndian.Uint16(in[2*j:])))
		bv := a
		if j+1 < n {
			bv = float64(int16(binary.LittleEndian.Uint16(in[2*(j+1):])))
		}
		binary.LittleEndian.PutUint16(out[2*i:], uint16(int16(a+(bv-a)*f)))
	}
	return out
}
