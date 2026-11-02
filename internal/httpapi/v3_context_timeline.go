package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

// v3ContextTimeline is the Life Context Timeline (BW-002): a read-only,
// owner-scoped chronological projection of saved source records with each
// source's current version and meaning status. It writes nothing and never
// calls an AI provider; aiUsed is always false.
//
// Query: kinds=note,checkin,conversation (default all), either days=7|30|90
// (a window ending now, on the server clock) or from/to as RFC 3339
// (from inclusive, to exclusive), limit=1..50 (default 20), order=desc|asc
// (default desc) and cursor (the previous page's nextCursor).
func (s *Server) v3ContextTimeline(w http.ResponseWriter, r *http.Request, p account.Principal) {
	q, field := s.parseTimelineQuery(r)
	if field != "" {
		writeError(w, r, http.StatusBadRequest, "invalid_input", field)
		return
	}
	page, err := s.opt.Accounts.ContextTimeline(r.Context(), p.UserID, q)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, page)
}

func (s *Server) parseTimelineQuery(r *http.Request) (account.TimelineQuery, string) {
	v := r.URL.Query()
	var q account.TimelineQuery
	if raw := v.Get("kinds"); raw != "" {
		q.Kinds = strings.Split(raw, ",")
	}
	if raw := v.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > account.TimelineMaxLimit {
			return q, "limit"
		}
		q.Limit = n
	}
	q.Order = v.Get("order")
	q.Cursor = v.Get("cursor")
	if raw := v.Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || (n != 7 && n != 30 && n != 90) || v.Get("from") != "" || v.Get("to") != "" {
			return q, "days"
		}
		q.Days = n
		return q, ""
	}
	for _, b := range []struct {
		name string
		dst  *time.Time
	}{{"from", &q.From}, {"to", &q.To}} {
		raw := v.Get(b.name)
		if raw == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return q, b.name
		}
		*b.dst = t.UTC()
	}
	return q, ""
}
