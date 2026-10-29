package sonic

import (
	"context"
	"encoding/binary"
	"math"
	"sync"
	"time"
)

// TestDouble is the SELA_VOICE_MODE=test_double backend. It never calls AWS.
// A "turn" is speech energy followed by a short quiet spell; for each turn
// it emits a transcript and a clearly labelled assistant reply with a soft
// tone as audio, so the voice room can be exercised without credentials.
type TestDouble struct {
	// Transcript returns the transcript for turn n (0-based). Tests use it to
	// script crisis language; nil means a fixed labelled transcript.
	Transcript func(n int) string
	// Reply returns the assistant text for turn n; nil means a fixed reply.
	Reply func(n int) string
}

// TestDoubleReply is the fixed assistant reply; it says what it is.
const TestDoubleReply = "TEST DOUBLE — bukan Amazon Nova Sonic. Aku mendengarmu."

// Model implements Opener.
func (TestDouble) Model() string { return TestDoubleModel }

// Open implements Opener.
func (d TestDouble) Open(_ context.Context, _ SessionOptions) (Session, error) {
	s := &doubleSession{d: d, events: make(chan Event, 64), stop: make(chan struct{})}
	go s.watch()
	return s, nil
}

type doubleSession struct {
	d      TestDouble
	events chan Event

	mu       sync.Mutex
	closed   bool
	voiced   time.Duration
	lastLoud time.Time
	turn     int
	stop     chan struct{}
	once     sync.Once
}

// SendAudio implements Session.
func (s *doubleSession) SendAudio(pcm []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if rms(pcm) > 500 {
		s.voiced += time.Duration(len(pcm)/2) * time.Second / InputSampleRate
		s.lastLoud = time.Now()
	}
	return nil
}

func (s *doubleSession) watch() {
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.mu.Lock()
			ready := s.voiced > 300*time.Millisecond && time.Since(s.lastLoud) > 700*time.Millisecond
			n := s.turn
			if ready {
				s.voiced = 0
				s.turn++
			}
			s.mu.Unlock()
			if ready {
				s.emitTurn(n)
			}
		}
	}
}

func (s *doubleSession) emitTurn(n int) {
	transcript := "Transkrip uji (TEST DOUBLE, bukan Sonic)."
	if s.d.Transcript != nil {
		transcript = s.d.Transcript(n)
	}
	reply := TestDoubleReply
	if s.d.Reply != nil {
		reply = s.d.Reply(n)
	}
	id := "double-" + string(rune('a'+n%26))
	evs := []Event{
		{Kind: KindUserTranscript, ContentID: id + "-u", Text: transcript},
		{Kind: KindAssistantText, ContentID: id + "-s", Text: reply, Speculative: true},
		{Kind: KindAssistantAudio, ContentID: id + "-a", PCM: tone(600 * time.Millisecond)},
		{Kind: KindAssistantAudioEnd, ContentID: id + "-a", StopReason: "END_TURN"},
		{Kind: KindAssistantText, ContentID: id + "-f", Text: reply},
	}
	for _, e := range evs {
		select {
		case s.events <- e:
		case <-s.stop:
			return
		}
	}
}

// Inject delivers one scripted turn immediately (tests only).
func (s *doubleSession) Inject(n int) { s.emitTurn(n) }

// Events implements Session.
func (s *doubleSession) Events() <-chan Event { return s.events }

// Err implements Session.
func (s *doubleSession) Err() error { return nil }

// Close implements Session.
func (s *doubleSession) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.once.Do(func() { close(s.stop) })
	return nil
}

func rms(pcm []byte) float64 {
	n := len(pcm) / 2
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		v := float64(int16(binary.LittleEndian.Uint16(pcm[2*i:])))
		sum += v * v
	}
	return math.Sqrt(sum / float64(n))
}

// tone is a quiet 440 Hz sine at the output rate, so playback is audible
// in development without pretending to be speech.
func tone(d time.Duration) []byte {
	n := int(d.Seconds() * OutputSampleRate)
	out := make([]byte, 2*n)
	for i := 0; i < n; i++ {
		v := int16(1500 * math.Sin(2*math.Pi*440*float64(i)/OutputSampleRate))
		binary.LittleEndian.PutUint16(out[2*i:], uint16(v))
	}
	return out
}
