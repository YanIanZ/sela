package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// opEvents streams an AI operation's events as server-sent events. Events
// are replayed from Last-Event-ID (or ?after=) so a reconnecting browser
// does not lose items. Only events of the caller's own session are visible.
func (s *Server) opEvents(w http.ResponseWriter, r *http.Request) {
	sub, err := s.svc.Subscribe(s.sessionID(r), r.PathValue("id"))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	after := 0
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		after, _ = strconv.Atoi(v)
	} else if v := r.URL.Query().Get("after"); v != "" {
		after, _ = strconv.Atoi(v)
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 2000\n\n")
	flusher.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		events, done, wake := sub.Events(after)
		for _, ev := range events {
			raw, err := json.Marshal(ev.Data)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.Seq, ev.Type, raw)
			after = ev.Seq
		}
		flusher.Flush()
		if done {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-wake:
		case <-heartbeat.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}
