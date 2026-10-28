package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai/sonic"
	"github.com/gorilla/websocket"
)

// Dictation (SPEC C25, backlog S-025): the browser streams raw PCM audio
// to Sela over this socket and Sela relays it to the configured provider
// with server-held credentials, which never reach the browser. Two relays
// exist behind SELA_ASR_MODE: Amazon Nova Sonic ("bedrock", the pilot
// configuration) and the legacy Meta voice-transcribe proxy ("meta").
// The transcript stays editable client-side; nothing is auto-submitted.
const (
	// asrAudioEncoding is the Meta-mode handshake encoding; the Sonic
	// relay declares its own input config in internal/ai/sonic.
	asrAudioEncoding = "PCM_24KHZ"
	// asrSessionMax bounds one dictation session.
	asrSessionMax = 5 * time.Minute
	// asrAudioMaxBytes caps upstream audio (~5 min at 48 kB/s).
	asrAudioMaxBytes = 15 << 20
	asrWriteWait     = 10 * time.Second
	asrHandshakeWait = 15 * time.Second
	// asrFlushWait is how long the relay waits for the provider's final
	// answer after the browser ends input before closing the socket.
	asrFlushWait = 15 * time.Second
)

// asrInputCap is the per-byte cap applied to every PCM frame the browser
// sends: 48 kB per second at 24 kHz 16-bit mono, so 2 400 ms of audio is
// one chunk. Frames larger than this are not audio and are rejected.
const asrInputCap = 115_200

var asrUpgraderBase = websocket.Upgrader{
	// Browsers cannot set custom headers on a WS handshake, so the
	// X-Sela-Request mutation guard cannot apply here. The Origin check
	// below still rejects cross-site sockets; same-origin pages pass.
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

func (s *Server) asrOriginOK(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if s.opt.PublicOrigin != "" {
		return strings.EqualFold(strings.TrimRight(s.opt.PublicOrigin, "/"), u.Scheme+"://"+u.Host)
	}
	return strings.EqualFold(u.Host, r.Host)
}

// v3ASRRealtime is GET /api/v3/asr/realtime (owner). It upgrades the
// client socket first and then reports every failure as an error event
// plus an application close code, because browsers cannot read the HTTP
// status of a rejected upgrade. No audio is accepted before the mode
// and consent checks pass.
func (s *Server) v3ASRRealtime(w http.ResponseWriter, r *http.Request, p account.Principal) {
	if !s.asrOriginOK(r) {
		writeError(w, r, http.StatusForbidden, "cross_origin", "")
		return
	}
	mode := s.opt.ASRMode
	if mode == "" {
		mode = "disabled"
	}
	if mode == "disabled" {
		asrUpgrader := asrUpgraderBase
		asrUpgrader.CheckOrigin = s.asrOriginOK
		front, err := asrUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer front.Close()
		s.asrReject(front, "asr_unavailable", 4413)
		return
	}
	if !s.asrConsented(r.Context(), p.UserID) {
		asrUpgrader := asrUpgraderBase
		asrUpgrader.CheckOrigin = s.asrOriginOK
		front, err := asrUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer front.Close()
		s.asrReject(front, "asr_consent_required", 4403)
		return
	}
	asrUpgrader := asrUpgraderBase
	asrUpgrader.CheckOrigin = s.asrOriginOK
	front, err := asrUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer front.Close()
	if mode == "test_double" {
		s.asrLoopback(front)
		return
	}
	if mode == "bedrock" {
		s.asrSonic(front)
		return
	}
	s.asrProxy(front)
}

// asrSonic relays one dictation session to Amazon Nova Sonic, keeping only
// the user transcript: the assistant text and audio are dropped (a
// transcript goes into the editable composer, never to a voice reply).
// Sonic runs a full-duplex speech-to-speech session, not push-to-talk:
// the mic stream stays open until endStream ends the browser side, the
// flush window collects the final transcript, and the deferred close
// ends the provider session. The final transcript reaches the browser
// before close.
func (s *Server) asrSonic(front *websocket.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), asrSessionMax)
	defer cancel()

	opener := s.opt.ASROpener
	if opener == nil {
		region := s.opt.AWSRegion
		if region == "" {
			region = sonic.DefaultRegion
		}
		o, err := sonic.NewBedrock(sonic.Config{
			Region:       region,
			Model:        s.opt.ASRModel,
			AccessKeyID:  os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretKey:    os.Getenv("AWS_SECRET_ACCESS_KEY"),
			SessionToken: os.Getenv("AWS_SESSION_TOKEN"),
		})
		if err != nil {
			s.asrFail(front, "asr_backend_unreachable")
			return
		}
		opener = o
	}
	sess, err := opener.Open(ctx, sonic.SessionOptions{System: "Transkripsikan ucapan pengguna apa adanya dalam bahasa yang ia pakai. Tidak ada komentar."})
	if err != nil {
		s.asrFail(front, "asr_backend_unreachable")
		return
	}
	defer sess.Close()

	var mu sync.Mutex // serialises writes to the browser
	writeJSON := func(v any) error {
		b, _ := json.Marshal(v)
		mu.Lock()
		defer mu.Unlock()
		_ = front.SetWriteDeadline(time.Now().Add(asrWriteWait))
		return front.WriteMessage(websocket.TextMessage, b)
	}

	// Browser -> Sonic (16 kHz PCM). No assistant audio is ever forwarded.
	upDone := make(chan struct{})
	go func() {
		defer close(upDone)
		front.SetReadLimit(asrInputCap)
		var sent int64
		for {
			mt, msg, err := front.ReadMessage()
			if err != nil {
				return
			}
			switch mt {
			case websocket.BinaryMessage:
				if len(msg)%2 != 0 {
					return
				}
				sent += int64(len(msg))
				if sent > asrAudioMaxBytes {
					return
				}
				if err := sess.SendAudio(msg); err != nil {
					return
				}
			case websocket.TextMessage:
				var ctl struct {
					Type string `json:"type"`
				}
				if json.Unmarshal(msg, &ctl) == nil && ctl.Type == "endStream" {
					// endStream ends input; Sonic finishes the turn and the
					// final transcript arrives below.
					return
				}
			}
		}
	}()

	// Sonic -> browser: transcripts only (dictation; nothing is spoken back).
	var last string
	for {
		select {
		case <-upDone:
			// Wait briefly for the final transcript.
			deadline := time.After(asrFlushWait)
		flush:
			for {
				select {
				case e := <-sess.Events():
					if e.Kind == sonic.KindUserTranscript && e.Text != "" {
						last = e.Text
					}
				case <-deadline:
					break flush
				}
			}
			// Exactly one terminal outcome: a final transcript, or the
			// no-transcript signal — never both.
			if last != "" {
				_ = writeJSON(map[string]any{"type": "transcript", "transcript": last, "final": true})
			} else {
				_ = writeJSON(map[string]any{"type": "error", "message": "asr_no_transcript"})
			}
			return
		case <-ctx.Done():
			if last != "" {
				_ = writeJSON(map[string]any{"type": "transcript", "transcript": last, "final": true})
			}
			return
		case e, ok := <-sess.Events():
			if !ok {
				if last != "" {
					_ = writeJSON(map[string]any{"type": "transcript", "transcript": last, "final": true})
				}
				return
			}
			if e.Kind == sonic.KindUserTranscript && e.Text != "" {
				last = e.Text
				_ = writeJSON(map[string]any{"type": "transcript", "transcript": e.Text, "final": false})
			}
		}
	}
}

// asrReject reports a pre-session failure inside the upgraded socket.
func (s *Server) asrReject(front *websocket.Conn, msg string, code int) {
	_ = front.SetWriteDeadline(time.Now().Add(asrWriteWait))
	_ = front.WriteJSON(map[string]any{"type": "error", "message": msg})
	_ = front.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(code, ""),
		time.Now().Add(asrWriteWait))
}

func (s *Server) asrConsented(ctx context.Context, owner account.UserID) bool {
	list, err := s.opt.Accounts.Consents(ctx, owner)
	if err != nil {
		return false
	}
	for _, c := range list {
		if c.Purpose == account.PolicyASRMeta {
			return c.Granted && !c.Outdated
		}
	}
	return false
}

// asrLoopback is the SELA_ASR_MODE=test_double backend: it accepts audio
// and answers endStream with a fixed, clearly-labelled test transcript so
// the frontend flow is exercisable without a provider key.
func (s *Server) asrLoopback(front *websocket.Conn) {
	_ = front.SetReadDeadline(time.Now().Add(asrSessionMax))
	for {
		mt, msg, err := front.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.TextMessage {
			continue
		}
		var ctl struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(msg, &ctl); err != nil || ctl.Type != "endStream" {
			continue
		}
		_ = front.SetWriteDeadline(time.Now().Add(asrWriteWait))
		_ = front.WriteJSON(map[string]any{
			"type": "transcript", "transcript": "Hasil dikte uji, bukan transkripsi Meta.",
			"final": true,
		})
		_ = front.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(asrWriteWait))
		return
	}
}

// asrProxy relays one session to the Meta realtime endpoint.
func (s *Server) asrProxy(front *websocket.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), asrSessionMax)
	defer cancel()

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	back, _, err := dialer.DialContext(ctx, s.opt.ASRURL, nil)
	if err != nil {
		s.asrFail(front, "asr_backend_unreachable")
		return
	}
	defer back.Close()

	model := s.opt.ASRModel
	if model == "" {
		model = "muse-voice-transcribe-1.0"
	}
	handshake, _ := json.Marshal(map[string]any{
		"authorization":     map[string]any{"accessToken": "Bearer " + s.opt.ASRAPIKey},
		"audioEncoding":     asrAudioEncoding,
		"model":             model,
		"mode":              "PUSH_TO_TALK",
		"partialMode":       "CUMULATIVE",
		"emitAudioProgress": false,
	})
	_ = back.SetWriteDeadline(time.Now().Add(asrWriteWait))
	if err := back.WriteMessage(websocket.TextMessage, handshake); err != nil {
		s.asrFail(front, "asr_backend_unreachable")
		return
	}
	_ = back.SetReadDeadline(time.Now().Add(asrHandshakeWait))
	mt, ack, err := back.ReadMessage()
	if err != nil || mt != websocket.TextMessage || !isASRAck(ack) {
		s.asrFail(front, "asr_handshake_failed")
		return
	}
	_ = back.SetReadDeadline(time.Time{})

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-done:
		case <-ctx.Done():
		}
		_ = front.Close()
		_ = back.Close()
	}()

	// frontMu serialises every write to the browser: forwarded events
	// and the final close frame.
	var frontMu sync.Mutex
	writeFront := func(mt int, msg []byte) error {
		frontMu.Lock()
		defer frontMu.Unlock()
		_ = front.SetWriteDeadline(time.Now().Add(asrWriteWait))
		return front.WriteMessage(mt, msg)
	}

	// Browser to Meta: raw PCM binary passes through; only endStream
	// control text is forwarded. Anything else is dropped.
	upDone := make(chan struct{})
	downDone := make(chan struct{})
	var upstreamEnded atomic.Bool
	go func() {
		defer close(upDone)
		var sent int64
		for {
			mt, msg, err := front.ReadMessage()
			if err != nil {
				return
			}
			_ = back.SetWriteDeadline(time.Now().Add(asrWriteWait))
			switch mt {
			case websocket.BinaryMessage:
				sent += int64(len(msg))
				if len(msg) > asrInputCap {
					// A frame too large to be live PCM (≥ ~2.4 s of audio)
					// is not audio: drop it and end this side.
					upstreamEnded.Store(true)
					return
				}
				if sent > asrAudioMaxBytes {
					return
				}
				if err := back.WriteMessage(websocket.BinaryMessage, msg); err != nil {
					return
				}
			case websocket.TextMessage:
				var ctl struct {
					Type string `json:"type"`
				}
				if json.Unmarshal(msg, &ctl) == nil && ctl.Type == "endStream" {
					_ = back.WriteMessage(websocket.TextMessage, msg)
					return
				}
			}
		}
	}()
	// Meta to browser: transcript events pass through untouched.
	go func() {
		defer close(downDone)
		for {
			mt, msg, err := back.ReadMessage()
			if err != nil {
				return
			}
			if mt != websocket.TextMessage {
				continue
			}
			if err := writeFront(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()
	select {
	case <-upDone:
		// endStream is on its way to Meta; wait for the final answer
		// before closing so it still reaches the browser. An upstream
		// that ended early (oversized frame) has nothing to flush.
		if !upstreamEnded.Load() {
			select {
			case <-downDone:
			case <-time.After(asrFlushWait):
			case <-ctx.Done():
			}
		}
	case <-downDone:
		// Meta is gone; there is nothing more to forward.
	case <-ctx.Done():
	}
	// Deliver a normal close frame before the socket goes away.
	frontMu.Lock()
	_ = front.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(asrWriteWait))
	frontMu.Unlock()
}

func isASRAck(raw []byte) bool {
	var ack map[string]any
	if err := json.Unmarshal(raw, &ack); err != nil {
		return false
	}
	if _, ok := ack["type"]; ok {
		return false
	}
	_, ok := ack["sessionId"]
	return ok
}

func (s *Server) asrFail(front *websocket.Conn, msg string) {
	_ = front.SetWriteDeadline(time.Now().Add(asrWriteWait))
	_ = front.WriteJSON(map[string]any{"type": "error", "message": msg})
	_ = front.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(asrWriteWait))
}
