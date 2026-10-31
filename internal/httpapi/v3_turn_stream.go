package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

// v3StreamTurn is the SSE form of POST /conversations/{id}/turns. It runs
// the same pipeline as v3SendTurn. No reply text exists before the router
// has released the complete validated output; only then are small
// UTF-8-safe chunks of that released text sent, followed by the final room.
//
// Frames: status (once), optional heartbeat while the router runs, zero or
// more chunk, then done, or error with a stable code. A refusal before the
// stream opens (invalid input, stale version, missing room) uses the normal
// JSON error envelope, so a client can fall back to the JSON route without
// parsing a partial stream. Heartbeats keep proxies and clients from timing
// out during a slow model call; they carry no content and are ignored by
// clients that do not know them.
func (s *Server) v3StreamTurn(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in listeningTurnInput
	if !decode(w, r, &in) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	heartbeat := s.opt.StreamHeartbeat
	if heartbeat <= 0 {
		heartbeat = 10 * time.Second
	}
	opened := false
	open := func(state string) {
		if opened {
			return
		}
		h := w.Header()
		h.Set("Content-Type", "text/event-stream; charset=utf-8")
		h.Set("Cache-Control", "no-store")
		h.Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		opened = true
		_ = writeSSE(w, "status", map[string]string{"state": state})
		flusher.Flush()
	}

	// The turn runs in a goroutine so this handler can emit keep-alives
	// while the model is working. Only this handler writes to w; the
	// pipeline signals when the router is about to start.
	started := make(chan struct{})
	var startOnce sync.Once
	type result struct {
		out    listeningTurn
		reject *turnRejection
		err    error
	}
	done := make(chan result, 1)
	go func() {
		out, reject, err := s.runListeningTurn(r, p, r.PathValue("id"), in, func() {
			startOnce.Do(func() { close(started) })
		})
		done <- result{out: out, reject: reject, err: err}
	}()

	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-started:
			started = nil
			open("thinking")
		case <-ticker.C:
			if opened {
				if writeSSE(w, "heartbeat", map[string]string{"state": "thinking"}) != nil {
					return
				}
				flusher.Flush()
			}
		case res := <-done:
			if res.reject != nil {
				if opened {
					_ = writeSSE(w, "error", map[string]string{"code": res.reject.code})
					flusher.Flush()
					return
				}
				res.reject.write(w, r)
				return
			}
			if res.err != nil {
				if opened {
					_ = writeSSE(w, "error", map[string]string{"code": "internal"})
					flusher.Flush()
					return
				}
				s.v3Error(w, r, res.err)
				return
			}
			// A turn that stopped before the router (paused, no consent, a
			// limit or no AI) is still a complete answer; open the stream
			// with its state and send the room without chunks.
			open(res.out.Room.AIState)
			if len(res.out.Released) > 0 {
				text := strings.Join(res.out.Released, "\n\n")
				for _, piece := range streamPieces(text, 16) {
					if writeSSE(w, "chunk", map[string]string{"text": piece}) != nil {
						return
					}
					flusher.Flush()
				}
			}
			if writeSSE(w, "done", res.out.Room) != nil {
				return
			}
			flusher.Flush()
			return
		case <-r.Context().Done():
			return
		}
	}
}

func writeSSE(w http.ResponseWriter, event string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
	return err
}

// streamPieces splits validated text on rune boundaries so a chunk never
// breaks a UTF-8 sequence. It never adds or reorders characters.
func streamPieces(text string, size int) []string {
	if text == "" || size < 1 {
		return nil
	}
	runes := []rune(text)
	out := make([]string, 0, (len(runes)+size-1)/size)
	for start := 0; start < len(runes); start += size {
		end := min(start+size, len(runes))
		out = append(out, string(runes[start:end]))
	}
	return out
}
