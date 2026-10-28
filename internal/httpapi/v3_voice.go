package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai/guard"
	"github.com/YanIanZ/sela/internal/ai/sonic"
	"github.com/YanIanZ/sela/internal/safety"
	"github.com/gorilla/websocket"
)

// Voice room over Amazon Nova 2.5 Sonic (SPEC C27, owner decision D-058).
// The browser streams microphone PCM (16 kHz mono int16) to Sela; Sela holds
// the IAM credentials and relays to Sonic. Nothing is stored: transcripts
// live in the tab only.
//
// Safety order, enforced here and not in the client:
//   - no audio is accepted before the mode, consent (ai_provider_aws_voice)
//     and age band (18_plus only) checks pass;
//   - every user transcript goes through the deterministic crisis router;
//     a crisis ends the session at once, drops any pending assistant audio
//     and tells the client to show human help (SPEC V3);
//   - assistant audio is held until the matching assistant text has passed
//     the crisis router and the output guards (hard guards and the
//     health-claim flag both withhold, because no verifier runs on voice);
//     withheld audio is dropped, never played.
const (
	// PolicyVoiceAWS is the consent purpose for the voice room.
	PolicyVoiceAWS = "ai_provider_aws_voice"
	// voiceSessionMax stays under Sonic's 8-minute connection limit.
	voiceSessionMax = 7*time.Minute + 50*time.Second
	// voiceHoldMax bounds audio held while waiting for its text.
	voiceHoldMax  = 30 * sonic.OutputSampleRate * 2
	voiceAudioMax = 16 << 20
	voiceSystem   = `Kamu adalah Sela, ruang untuk didengarkan. Kamu adalah AI, bukan psikolog, dokter, atau konselor, dan kamu tidak pernah mengaku manusia atau tenaga profesional.
Bicaralah dalam bahasa yang dipakai orang itu (biasanya bahasa Indonesia sehari-hari), hangat, tenang, dan singkat: satu sampai tiga kalimat.
Dengarkan dan cerminkan perasaannya dengan kata-katanya sendiri. Ajukan paling banyak satu pertanyaan terbuka.
Jangan memberi diagnosis, nama gangguan, obat, dosis, terapi, statistik, atau nasihat medis. Jangan menjanjikan kerahasiaan, jangan membuat orang bergantung padamu, dan jangan menekan.
Kalau orang itu menyebut ingin menyakiti diri, bunuh diri, atau sedang dalam bahaya, katakan dengan lembut bahwa ia layak mendapat bantuan manusia sekarang juga dan sarankan menghubungi layanan darurat atau orang terdekat.`
)

// Close codes reported inside the upgraded socket (browsers cannot read
// the HTTP status of a refused upgrade).
const (
	voiceCloseUnavailable = 4413
	voiceCloseConsent     = 4403
	voiceCloseAgeBand     = 4451
	voiceCloseBusy        = 4429
	voiceCloseBackend     = 4500
	voiceCloseCrisis      = 4409
	voiceCloseTimeLimit   = 4408
)

// voiceActive allows one voice session per person per server.
var voiceActive sync.Map // voiceKey -> struct{}

type voiceKey struct {
	srv  *Server
	user account.UserID
}

// v3VoiceRealtime is GET /api/v3/voice/realtime (owner).
func (s *Server) v3VoiceRealtime(w http.ResponseWriter, r *http.Request, p account.Principal) {
	if !s.asrOriginOK(r) {
		writeError(w, r, http.StatusForbidden, "cross_origin", "")
		return
	}
	up := asrUpgraderBase
	up.CheckOrigin = s.asrOriginOK
	front, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer front.Close()
	if s.opt.VoiceOpener == nil {
		s.asrReject(front, "voice_unavailable", voiceCloseUnavailable)
		return
	}
	prof, err := s.opt.Accounts.Profile(r.Context(), p.UserID)
	if err != nil || prof.AgeBand != "18_plus" {
		s.asrReject(front, "voice_age_band", voiceCloseAgeBand)
		return
	}
	if !s.voiceConsented(r.Context(), p.UserID) {
		s.asrReject(front, "voice_consent_required", voiceCloseConsent)
		return
	}
	key := voiceKey{s, p.UserID}
	if _, busy := voiceActive.LoadOrStore(key, struct{}{}); busy {
		s.asrReject(front, "voice_busy", voiceCloseBusy)
		return
	}
	defer voiceActive.Delete(key)
	s.voiceRelay(front)
}

func (s *Server) voiceConsented(ctx context.Context, owner account.UserID) bool {
	list, err := s.opt.Accounts.Consents(ctx, owner)
	if err != nil {
		return false
	}
	for _, c := range list {
		if c.Purpose == PolicyVoiceAWS {
			return c.Granted && !c.Outdated
		}
	}
	return false
}

// voiceRelay runs one session: browser audio in, gated events and audio out.
func (s *Server) voiceRelay(front *websocket.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), voiceSessionMax)
	defer cancel()
	sess, err := s.opt.VoiceOpener.Open(ctx, sonic.SessionOptions{System: voiceSystem})
	if err != nil {
		s.asrReject(front, "voice_backend_unreachable", voiceCloseBackend)
		return
	}
	defer sess.Close()

	var mu sync.Mutex // serialises writes to the browser
	write := func(mt int, b []byte) error {
		mu.Lock()
		defer mu.Unlock()
		_ = front.SetWriteDeadline(time.Now().Add(asrWriteWait))
		return front.WriteMessage(mt, b)
	}
	writeJSON := func(v any) error {
		b, _ := json.Marshal(v)
		return write(websocket.TextMessage, b)
	}
	closeWith := func(code int) {
		mu.Lock()
		defer mu.Unlock()
		_ = front.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, ""), time.Now().Add(asrWriteWait))
	}

	model := s.opt.VoiceOpener.Model()
	_ = writeJSON(map[string]any{
		"type": "ready", "provider": "aws", "model": model,
		"testDouble": model == sonic.TestDoubleModel, "maxSeconds": int(voiceSessionMax.Seconds()),
		"inputSampleRate": sonic.InputSampleRate, "outputSampleRate": sonic.OutputSampleRate,
	})

	// Browser -> Sonic.
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		front.SetReadLimit(64 << 10)
		total := 0
		for {
			mt, msg, err := front.ReadMessage()
			if err != nil {
				return
			}
			switch mt {
			case websocket.BinaryMessage:
				total += len(msg)
				if total > voiceAudioMax || len(msg)%2 != 0 {
					return
				}
				if err := sess.SendAudio(msg); err != nil {
					return
				}
			case websocket.TextMessage:
				var ctl struct {
					Type string `json:"type"`
				}
				if json.Unmarshal(msg, &ctl) == nil && ctl.Type == "end" {
					return
				}
			}
		}
	}()

	// Sonic -> browser, through the gate.
	g := newVoiceGate()
	for {
		select {
		case <-clientDone:
			closeWith(websocket.CloseNormalClosure)
			return
		case <-ctx.Done():
			_ = writeJSON(map[string]any{"type": "error", "message": "voice_time_limit"})
			closeWith(voiceCloseTimeLimit)
			return
		case e, ok := <-sess.Events():
			if !ok {
				_ = writeJSON(map[string]any{"type": "error", "message": "voice_backend_ended"})
				closeWith(voiceCloseBackend)
				return
			}
			for _, out := range g.handle(e) {
				if out.audio != nil {
					if write(websocket.BinaryMessage, out.audio) != nil {
						return
					}
					continue
				}
				if writeJSON(out.msg) != nil {
					return
				}
			}
			if g.crisis {
				closeWith(voiceCloseCrisis)
				return
			}
		}
	}
}

// voiceOut is one item for the browser: a JSON message or PCM audio.
type voiceOut struct {
	msg   map[string]any
	audio []byte
}

// voiceGate decides what reaches the browser. It is pure and unit-tested.
type voiceGate struct {
	state  gateState
	held   [][]byte
	heldN  int
	crisis bool
}

type gateState int

const (
	gatePending gateState = iota // audio waits for its text
	gateOpen                     // text passed: audio flows
	gateShut                     // text failed: audio is dropped
)

func newVoiceGate() *voiceGate { return &voiceGate{} }

// assistantTextOK runs the crisis router and the output guards. A health
// claim is withheld too: no verifier runs on spoken replies.
func assistantTextOK(text string) bool {
	if safety.Assess(text).Level == safety.LevelCrisis {
		return false
	}
	res := guard.Check(text, guard.Options{})
	return !res.Blocked() && !res.Flagged()
}

func (g *voiceGate) handle(e sonic.Event) []voiceOut {
	if g.crisis {
		return nil
	}
	switch e.Kind {
	case sonic.KindUserTranscript:
		out := []voiceOut{{msg: map[string]any{"type": "transcript", "text": e.Text}}}
		if safety.Assess(e.Text).Level == safety.LevelCrisis {
			g.crisis = true
			g.reset()
			out = append(out, voiceOut{msg: map[string]any{"type": "crisis"}})
		}
		return out
	case sonic.KindAssistantText:
		if !e.Speculative {
			// Final text follows its audio; show it only if the turn
			// was released.
			if g.state == gateShut || !assistantTextOK(e.Text) {
				return nil
			}
			return []voiceOut{{msg: map[string]any{"type": "reply", "text": e.Text, "final": true}}}
		}
		if !assistantTextOK(e.Text) {
			g.state = gateShut
			g.dropHeld()
			return []voiceOut{{msg: map[string]any{"type": "withheld"}}}
		}
		if g.state == gateShut {
			return nil
		}
		g.state = gateOpen
		out := []voiceOut{{msg: map[string]any{"type": "reply", "text": e.Text, "final": false}}}
		for _, a := range g.held {
			out = append(out, voiceOut{audio: a})
		}
		g.dropHeld()
		return out
	case sonic.KindAssistantAudio:
		switch g.state {
		case gateOpen:
			return []voiceOut{{audio: e.PCM}}
		case gatePending:
			if g.heldN+len(e.PCM) <= voiceHoldMax {
				g.held = append(g.held, e.PCM)
				g.heldN += len(e.PCM)
			}
		}
		return nil
	case sonic.KindAssistantAudioEnd:
		var out []voiceOut
		if g.state == gateOpen {
			out = append(out, voiceOut{msg: map[string]any{"type": "replyEnd"}})
		} else if g.state == gatePending && g.heldN > 0 {
			// Audio ended without any text to check: never play it.
			out = append(out, voiceOut{msg: map[string]any{"type": "withheld"}})
		}
		g.reset()
		return out
	case sonic.KindInterrupted:
		g.reset()
		return []voiceOut{{msg: map[string]any{"type": "interrupted"}}}
	}
	return nil
}

func (g *voiceGate) dropHeld() { g.held, g.heldN = nil, 0 }

func (g *voiceGate) reset() {
	g.state = gatePending
	g.dropHeld()
}

// The voice refusal codes are documented in API_SPEC_V3 §2; they reach the
// browser inside the upgraded socket, with the documented status recorded
// for completeness.
func init() {
	for code, m := range map[string]message{
		"voice_unavailable":      {"Ruang suara belum diaktifkan di server ini.", "The voice room isn't enabled on this server."},
		"voice_consent_required": {"Izin percakapan suara belum diberikan.", "Voice conversation permission hasn't been given."},
		"voice_age_band":         {"Ruang suara hanya untuk usia 18 tahun ke atas.", "The voice room is for people aged 18 and over only."},
		"voice_busy":             {"Ruang suara sedang terbuka di tab atau perangkat lain.", "The voice room is already open in another tab or device."},
	} {
		messages[code] = m
	}
	v3Status["voice_unavailable"] = http.StatusServiceUnavailable
	v3Status["voice_consent_required"] = http.StatusForbidden
	v3Status["voice_age_band"] = http.StatusForbidden
	v3Status["voice_busy"] = http.StatusConflict
}
