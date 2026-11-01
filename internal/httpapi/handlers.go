package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/YanIanZ/sela/internal/journey"
)

const (
	smallBody = 16 << 10
	storyBody = 32 << 10
)

func (s *Server) routes() {
	m := s.mux
	// Public, session-free.
	m.HandleFunc("GET /api/v2/health", s.health)
	m.HandleFunc("GET /api/v2/ready", s.ready)
	m.HandleFunc("GET /api/v2/catalog", s.catalog)
	m.HandleFunc("GET /api/v2/ai/status", s.aiStatus)
	// Operator only, and only registered when a token is configured, so an
	// unconfigured deployment answers 404 for every method (no 405 + Allow).
	if s.opt.MetricsToken != "" {
		m.HandleFunc("GET /api/v2/metrics", s.limited("read", s.metrics))
	}

	// The v2 in-memory journey (owner session and fictional recipient) is
	// served only in development, test and demo (ADR-0002 §12).
	if !s.opt.DisableV2Journey {
		// Owner session.
		m.HandleFunc("POST /api/v2/sessions", s.mutation("session", smallBody, s.createSession))
		m.HandleFunc("GET /api/v2/session", s.read(s.getSession))
		m.HandleFunc("PATCH /api/v2/session", s.mutation("mutation", smallBody, s.patchSession))
		m.HandleFunc("DELETE /api/v2/session", s.mutation("mutation", smallBody, s.deleteSession))
		m.HandleFunc("PUT /api/v2/consents/{kind}", s.mutation("mutation", smallBody, s.putConsent))
		m.HandleFunc("PUT /api/v2/story", s.mutation("mutation", storyBody, s.putStory))
		m.HandleFunc("POST /api/v2/ledger/draft", s.mutation("ai", smallBody, s.startLedgerDraft))
		m.HandleFunc("POST /api/v2/ledger/items", s.mutation("mutation", smallBody, s.addItem))
		m.HandleFunc("PATCH /api/v2/ledger/items/{id}", s.mutation("mutation", smallBody, s.reviewItem))
		m.HandleFunc("POST /api/v2/ledger/clarifications", s.mutation("mutation", smallBody, s.clarify))
		m.HandleFunc("POST /api/v2/ledger/approval", s.mutation("mutation", smallBody, s.approve))
		m.HandleFunc("GET /api/v2/operations/{id}/events", s.read(s.opEvents))
		m.HandleFunc("DELETE /api/v2/operations/{id}", s.mutation("mutation", smallBody, s.cancelOp))
		m.HandleFunc("PUT /api/v2/preferences", s.mutation("mutation", smallBody, s.putPreferences))
		m.HandleFunc("GET /api/v2/next-steps", s.read(s.nextSteps))
		m.HandleFunc("POST /api/v2/opening/template", s.mutation("mutation", smallBody, s.openingTemplate))
		m.HandleFunc("POST /api/v2/opening/draft", s.mutation("ai", smallBody, s.openingDraft))
		m.HandleFunc("PUT /api/v2/opening", s.mutation("mutation", smallBody, s.putOpening))
		m.HandleFunc("POST /api/v2/opening/practice", s.mutation("mutation", smallBody, s.practice))
		m.HandleFunc("POST /api/v2/handoffs/preview", s.mutation("mutation", smallBody, s.previewHandoff))
		m.HandleFunc("POST /api/v2/handoffs", s.mutation("mutation", smallBody, s.createHandoff))
		m.HandleFunc("DELETE /api/v2/handoffs/{id}/share", s.mutation("mutation", smallBody, s.revokeShare))
		m.HandleFunc("POST /api/v2/follow-up/check-ins", s.mutation("mutation", smallBody, s.checkIn))

		// Fictional recipient (separate principal and cookie).
		m.HandleFunc("POST /api/v2/recipient/sessions", s.mutation("recipient", smallBody, s.recipientLogin))
		m.HandleFunc("GET /api/v2/recipient/handoff", s.read(s.recipientGet))
		m.HandleFunc("POST /api/v2/recipient/handoff/actions", s.mutation("mutation", smallBody, s.recipientAct))
		m.HandleFunc("DELETE /api/v2/recipient/session", s.mutation("mutation", smallBody, s.recipientLogout))
	}

	// Reviewer research corpus (never touches sessions).
	m.HandleFunc("GET /api/v2/research/manifest", s.limited("research", s.researchManifest))
	m.HandleFunc("GET /api/v2/research/search", s.limited("research", s.researchSearch))
	m.HandleFunc("GET /api/v2/research/records/{id}", s.limited("research", s.researchRecord))

	// Sela v3: accounts, guests, sessions, consents and the signed-in user.
	s.v3Routes()
	// Separate operator identity, audit, cache and planned-care controls.
	s.adminRoutes()
	s.adminCareRoutes()
	s.adminReadingRoutes()
	s.adminCommunityRoutes()
	s.adminNetworkRoutes()
	s.adminInviteRoutes()

	m.HandleFunc("/api/", s.apiFallback)
	m.Handle("/", s.static())
}

// routeMethods are the methods probed for an Allow header.
var routeMethods = []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

// apiFallback answers API paths no route matched: 405 with Allow when the
// path exists for other methods, otherwise a JSON 404.
func (s *Server) apiFallback(w http.ResponseWriter, r *http.Request) {
	var allow []string
	for _, m := range routeMethods {
		probe := r.Clone(r.Context())
		probe.Method = m
		if _, pattern := s.mux.Handler(probe); pattern != "" && pattern != "/api/" && pattern != "/" {
			allow = append(allow, m)
		}
	}
	if len(allow) > 0 {
		w.Header().Set("Allow", strings.Join(allow, ", "))
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "")
		return
	}
	writeError(w, r, http.StatusNotFound, journey.CodeNotFound, "")
}

func (s *Server) read(h http.HandlerFunc) http.HandlerFunc { return s.limited("read", h) }

func (s *Server) limited(bucket string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.limits.allow(bucket, s.clientIP(r)) {
			w.Header().Set("Retry-After", "30")
			writeError(w, r, http.StatusTooManyRequests, journey.CodeRateLimited, "")
			return
		}
		h(w, r)
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	mode := s.opt.AccountConfig.Environment
	if mode == "" {
		mode = "production"
	}
	writeData(w, http.StatusOK, map[string]any{"status": "ok", "version": s.opt.Version, "mode": mode, "uptimeSeconds": int(time.Since(s.start).Seconds())})
}

// ready keeps its v2 fields and adds database and schema. It answers 503
// only when this environment requires the database and it is unavailable
// or its schema is not current; "store" still describes the v2 journey
// store.
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ai := s.svc.AIStatus()
	database, schema := "not_configured", "not_configured"
	if s.opt.Database != nil {
		database, schema = s.opt.Database(r.Context())
	}
	store := "memory"
	if s.opt.DisableV2Journey {
		store = "disabled"
	}
	status, code := "ready", http.StatusOK
	if s.opt.DatabaseRequired && (database != "ok" || schema != "current") {
		status, code = "not_ready", http.StatusServiceUnavailable
	}
	writeData(w, code, map[string]any{
		"status":     status,
		"fixtures":   s.svc.Catalog().Version,
		"ai":         ai.Info.Mode,
		"research":   s.opt.Research != nil,
		"frontend":   s.frontendBuilt(),
		"store":      store,
		"persistent": false,
		"database":   database,
		"schema":     schema,
	})
}

// metrics is for the operator only: live counters include safety routing
// and consent outcomes, which in a quiet demo can reveal what one visitor
// just did. Without a configured token the route is not registered; the
// empty-token check below is a defensive second guard.
func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	if s.opt.MetricsToken == "" {
		writeError(w, r, http.StatusNotFound, journey.CodeNotFound, "")
		return
	}
	got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	// Hashing first makes the comparison constant-time in the length too.
	gh, wh := sha256.Sum256([]byte(got)), sha256.Sum256([]byte(s.opt.MetricsToken))
	if !ok || subtle.ConstantTimeCompare(gh[:], wh[:]) != 1 {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, r, http.StatusUnauthorized, journey.CodeForbidden, "")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"counters": s.svc.Metrics(), "store": s.svc.StoreStats()})
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	c := s.svc.Catalog()
	scenarios := make([]map[string]any, 0, len(c.Scenarios))
	for _, sc := range c.Scenarios {
		scenarios = append(scenarios, map[string]any{
			"id": sc.ID, "version": sc.Version, "kind": sc.Kind, "language": sc.Language, "title": sc.Title,
			"persona": sc.Persona, "summary": sc.Summary, "contentWarning": sc.ContentWarning, "story": sc.Story,
		})
	}
	writeData(w, http.StatusOK, map[string]any{
		"version": c.Version, "demoClock": c.DemoClock, "synthetic": true,
		"scenarios": scenarios, "professionals": c.Professionals, "plans": c.Plans,
		"checkIns": c.CheckIns, "clarifyReasons": c.ClarifyReasons, "declineReasons": c.DeclineReasons,
		"nextSteps": c.NextSteps, "clarifications": c.Clarifications, "help": c.Help, "helpNotice": c.HelpNotice,
		"policies": c.Policies,
	})
}

func (s *Server) aiStatus(w http.ResponseWriter, r *http.Request) {
	writeData(w, http.StatusOK, s.svc.AIStatus())
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var in journey.CreateInput
	if !decode(w, r, &in) {
		return
	}
	in.ClientKey = limitKey(s.clientIP(r))
	id, view, err := s.svc.CreateSession(in)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	s.setCookie(w, SessionCookie, id, "/api/v2", view.ExpiresAt)
	writeData(w, http.StatusCreated, view)
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	view, err := s.svc.State(s.sessionID(r))
	if err != nil {
		s.clearIfMissing(w, err)
		writeAppError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, view)
}

func (s *Server) clearIfMissing(w http.ResponseWriter, err error) {
	if e, ok := err.(*journey.Error); ok && e.Code == journey.CodeSessionMissing {
		s.setCookie(w, SessionCookie, "", "/api/v2", time.Time{})
	}
}

func (s *Server) respond(w http.ResponseWriter, r *http.Request, view any, err error) {
	if err != nil {
		s.clearIfMissing(w, err)
		writeAppError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, view)
}

func (s *Server) patchSession(w http.ResponseWriter, r *http.Request) {
	var in journey.SettingsInput
	if !decode(w, r, &in) {
		return
	}
	view, err := s.svc.UpdateSettings(s.sessionID(r), in)
	if err == nil {
		s.setCookie(w, SessionCookie, s.sessionID(r), "/api/v2", view.ExpiresAt)
	}
	s.respond(w, r, view, err)
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	err := s.svc.DeleteSession(s.sessionID(r))
	s.setCookie(w, SessionCookie, "", "/api/v2", time.Time{})
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) putConsent(w http.ResponseWriter, r *http.Request) {
	var in journey.ConsentInput
	if !decode(w, r, &in) {
		return
	}
	view, err := s.svc.SetConsent(s.sessionID(r), journey.ConsentKind(r.PathValue("kind")), in)
	if err == nil {
		// Retention changes move the session expiry; keep the cookie in step.
		s.setCookie(w, SessionCookie, s.sessionID(r), "/api/v2", view.ExpiresAt)
	}
	s.respond(w, r, view, err)
}

func (s *Server) putStory(w http.ResponseWriter, r *http.Request) {
	var in journey.StoryInput
	if !decode(w, r, &in) {
		return
	}
	res, err := s.svc.SubmitStory(s.sessionID(r), in)
	s.respond(w, r, res, err)
}

func (s *Server) startLedgerDraft(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ExpectedVersion int `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	op, err := s.svc.StartLedgerDraft(s.sessionID(r), in.ExpectedVersion)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	view, err := s.svc.State(s.sessionID(r))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	writeData(w, http.StatusAccepted, map[string]any{"operation": op, "session": view})
}

func (s *Server) addItem(w http.ResponseWriter, r *http.Request) {
	var in journey.AddItemInput
	if !decode(w, r, &in) {
		return
	}
	view, err := s.svc.AddManualItem(s.sessionID(r), in)
	s.respond(w, r, view, err)
}

func (s *Server) reviewItem(w http.ResponseWriter, r *http.Request) {
	var in journey.ItemInput
	if !decode(w, r, &in) {
		return
	}
	view, err := s.svc.ReviewItem(s.sessionID(r), r.PathValue("id"), in)
	s.respond(w, r, view, err)
}

func (s *Server) clarify(w http.ResponseWriter, r *http.Request) {
	var in journey.ClarifyInput
	if !decode(w, r, &in) {
		return
	}
	view, err := s.svc.Clarify(s.sessionID(r), in)
	s.respond(w, r, view, err)
}

func (s *Server) approve(w http.ResponseWriter, r *http.Request) {
	var in journey.ApproveInput
	if !decode(w, r, &in) {
		return
	}
	view, err := s.svc.Approve(s.sessionID(r), in)
	s.respond(w, r, view, err)
}

func (s *Server) cancelOp(w http.ResponseWriter, r *http.Request) {
	view, err := s.svc.CancelOp(s.sessionID(r), r.PathValue("id"))
	s.respond(w, r, view, err)
}

func (s *Server) putPreferences(w http.ResponseWriter, r *http.Request) {
	var in journey.PreferencesInput
	if !decode(w, r, &in) {
		return
	}
	view, err := s.svc.SetPreferences(s.sessionID(r), in)
	s.respond(w, r, view, err)
}

func (s *Server) nextSteps(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.ComputeNextSteps(s.sessionID(r))
	s.respond(w, r, out, err)
}

func (s *Server) openingTemplate(w http.ResponseWriter, r *http.Request) {
	var in journey.OpeningInput
	if !decode(w, r, &in) {
		return
	}
	view, err := s.svc.TemplateOpening(s.sessionID(r), in)
	s.respond(w, r, view, err)
}

func (s *Server) openingDraft(w http.ResponseWriter, r *http.Request) {
	var in journey.OpeningInput
	if !decode(w, r, &in) {
		return
	}
	op, err := s.svc.StartOpeningDraft(s.sessionID(r), in)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	view, err := s.svc.State(s.sessionID(r))
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	writeData(w, http.StatusAccepted, map[string]any{"operation": op, "session": view})
}

func (s *Server) putOpening(w http.ResponseWriter, r *http.Request) {
	var in journey.EditOpeningInput
	if !decode(w, r, &in) {
		return
	}
	view, err := s.svc.EditOpening(s.sessionID(r), in)
	s.respond(w, r, view, err)
}

func (s *Server) practice(w http.ResponseWriter, r *http.Request) {
	var in journey.PracticeInput
	if !decode(w, r, &in) {
		return
	}
	out, err := s.svc.Practice(s.sessionID(r), in)
	s.respond(w, r, out, err)
}

func (s *Server) createHandoff(w http.ResponseWriter, r *http.Request) {
	var in journey.HandoffInput
	if !decode(w, r, &in) {
		return
	}
	out, err := s.svc.CreateHandoff(s.sessionID(r), r.Header.Get("Idempotency-Key"), in)
	if err != nil {
		s.respond(w, r, nil, err)
		return
	}
	status := http.StatusCreated
	if out.Replay {
		status = http.StatusOK
	}
	writeData(w, status, out)
}

func (s *Server) previewHandoff(w http.ResponseWriter, r *http.Request) {
	var in journey.PreviewInput
	if !decode(w, r, &in) {
		return
	}
	out, err := s.svc.PreviewHandoff(s.sessionID(r), in)
	s.respond(w, r, out, err)
}

// revokeShare is a privacy action: the body ({} or {"expectedVersion":n})
// is optional, including when it arrives chunked and empty.
func (s *Server) revokeShare(w http.ResponseWriter, r *http.Request) {
	var in journey.RevokeInput
	if !decodeOptional(w, r, &in) {
		return
	}
	view, err := s.svc.RevokeShare(s.sessionID(r), r.PathValue("id"), in)
	s.respond(w, r, view, err)
}

func (s *Server) checkIn(w http.ResponseWriter, r *http.Request) {
	var in journey.CheckInInput
	if !decode(w, r, &in) {
		return
	}
	view, err := s.svc.CheckIn(s.sessionID(r), in)
	s.respond(w, r, view, err)
}

func (s *Server) recipientID(r *http.Request) string {
	c, err := r.Cookie(RecipientCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

func (s *Server) recipientLogin(w http.ResponseWriter, r *http.Request) {
	var in journey.RecipientAuthInput
	if !decode(w, r, &in) {
		return
	}
	id, err := s.svc.AuthenticateRecipient(in)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	h, err := s.svc.RecipientGet(id)
	if err != nil {
		writeAppError(w, r, err)
		return
	}
	s.setCookie(w, RecipientCookie, id, "/api/v2/recipient", h.ExpiresAt)
	writeData(w, http.StatusCreated, h)
}

func (s *Server) recipientGet(w http.ResponseWriter, r *http.Request) {
	h, err := s.svc.RecipientGet(s.recipientID(r))
	if err != nil {
		s.setCookie(w, RecipientCookie, "", "/api/v2/recipient", time.Time{})
		writeAppError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, h)
}

func (s *Server) recipientAct(w http.ResponseWriter, r *http.Request) {
	var in journey.RecipientActionInput
	if !decode(w, r, &in) {
		return
	}
	h, err := s.svc.RecipientAct(s.recipientID(r), in)
	if err != nil {
		if e, ok := err.(*journey.Error); ok && e.Code == journey.CodeAccessEnded {
			s.setCookie(w, RecipientCookie, "", "/api/v2/recipient", time.Time{})
		}
		writeAppError(w, r, err)
		return
	}
	writeData(w, http.StatusOK, h)
}

func (s *Server) recipientLogout(w http.ResponseWriter, r *http.Request) {
	s.svc.RecipientLogout(s.recipientID(r))
	s.setCookie(w, RecipientCookie, "", "/api/v2/recipient", time.Time{})
	writeData(w, http.StatusOK, map[string]any{"signedOut": true})
}

func (s *Server) researchManifest(w http.ResponseWriter, r *http.Request) {
	if s.opt.Research == nil {
		writeError(w, r, http.StatusServiceUnavailable, "research_unavailable", "")
		return
	}
	writeData(w, http.StatusOK, s.opt.Research.Manifest())
}

func (s *Server) researchSearch(w http.ResponseWriter, r *http.Request) {
	if s.opt.Research == nil {
		writeError(w, r, http.StatusServiceUnavailable, "research_unavailable", "")
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	// 200 characters, as the API spec and the input's maxLength say; a
	// UTF-16 unit count is never below the rune count.
	if utf8.RuneCountInString(q.Get("q")) > 200 {
		writeError(w, r, http.StatusBadRequest, journey.CodeInvalid, "q")
		return
	}
	writeData(w, http.StatusOK, s.opt.Research.Search(q.Get("q"), q.Get("theme"), q.Get("status"), limit))
}

func (s *Server) researchRecord(w http.ResponseWriter, r *http.Request) {
	if s.opt.Research == nil {
		writeError(w, r, http.StatusServiceUnavailable, "research_unavailable", "")
		return
	}
	rec, ok := s.opt.Research.Record(r.PathValue("id"))
	if !ok {
		writeError(w, r, http.StatusNotFound, journey.CodeNotFound, "id")
		return
	}
	writeData(w, http.StatusOK, rec)
}
