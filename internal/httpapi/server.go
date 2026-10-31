// Package httpapi exposes the /api/v2 JSON API and serves the static
// frontend. It validates transport concerns (size, content type, origin,
// cookies, rate limits, idempotency keys) and delegates every product rule
// to internal/journey. Logs are metadata only (SPEC V12).
package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/budget"
	"github.com/YanIanZ/sela/internal/cache"
	"github.com/YanIanZ/sela/internal/care"
	"github.com/YanIanZ/sela/internal/community"
	"github.com/YanIanZ/sela/internal/journey"
	"github.com/YanIanZ/sela/internal/reading"
	"github.com/YanIanZ/sela/internal/research"
)

// Cookie names. Paths keep each cookie on the API surface that needs it.
const (
	SessionCookie   = "sela_session"
	RecipientCookie = "sela_recipient"
)

// BudgetGate is the durable AI spend gate (D12). Implementations must not
// admit a paid call they cannot account for.
type BudgetGate interface {
	Enabled() bool
	Admit(ctx context.Context, task string, synthetic bool) (budget.Reservation, error)
	Settle(ctx context.Context, r budget.Reservation, s budget.Settlement) error
	Summary(ctx context.Context) (budget.Summary, error)
	ByTask(ctx context.Context, limit int) ([]budget.TaskSpend, error)
}

// Options configures the HTTP layer.
type Options struct {
	AIRouter *ai.Router
	// PublicOrigin, when set (e.g. https://sela.example), is the only Origin
	// accepted for mutations; otherwise the request Host is used.
	PublicOrigin string
	// SecureCookies sets the Secure attribute (required behind HTTPS).
	SecureCookies bool
	// StreamHeartbeat is the SSE keep-alive interval for a running turn;
	// zero uses 10s. It is test/ops tuning only and carries no content.
	StreamHeartbeat time.Duration
	// TrustProxy uses the last X-Forwarded-For hop (across all header
	// lines) as the client address for rate limiting. Enable only behind a
	// proxy that appends it.
	TrustProxy bool
	// WebDir is the built frontend (web/dist). Empty disables static serving.
	WebDir string
	// Research is the reviewer corpus index; nil reports it unavailable.
	Research *research.Index
	Logger   *slog.Logger
	// Version identifies the running build in /api/v2/health.
	Version string
	// MetricsToken guards GET /api/v2/metrics ("Authorization: Bearer
	// <token>"). Empty disables the route (404).
	MetricsToken string
	// Cache is the optional public Redis cache/control plane. Nil keeps the
	// authoritative embedded/DB path.
	Cache *cache.Client
	// Admin is the separate operator identity store. Nil disables the
	// /api/admin/v1 console.
	Admin *admin.Store
	// Budget admits and settles paid AI operations against a durable
	// monthly cap. Nil or disabled keeps the local/test path unmetered.
	Budget BudgetGate
	// Care is the public planned-care directory and synthetic sandbox. Nil
	// disables those routes.
	Care *care.Store
	// Community is the anonymous peer-sharing store (D-072). Nil disables
	// the community routes (503 store_unavailable).
	Community *community.Store
	// Moderator checks community posts and replies before they are shown.
	Moderator community.Moderator
	// ReadingReviews is the operator sign-off store for the embedded reading
	// library. Nil keeps the public routes on the embedded-only gate.
	ReadingReviews *reading.ReviewStore

	// Accounts is the /api/v3 store. Nil (no database configured, a
	// development setup) makes every /api/v3 route except the policies and
	// auth options answer 503 store_unavailable.
	Accounts *account.Store
	// AccountConfig drives GET /api/v3/auth/options, the policy texts and
	// the cookie lifetimes; it is the store's configuration when Accounts
	// is set.
	AccountConfig account.Config
	// Database reports readiness for /api/v2/ready: database is ok,
	// unavailable or not_configured; schema is current, pending, mismatch
	// or not_configured. Nil means not configured.
	Database func(ctx context.Context) (database, schema string)
	// DatabaseRequired makes /api/v2/ready answer 503 unless the database
	// is ok and its schema current (demo, staging, production).
	DatabaseRequired bool
	// DisableV2Journey stops serving the v2 in-memory journey routes
	// (staging and production, ADR-0002 §12).
	DisableV2Journey bool
	// Clock drives the rate limiter; nil is the wall clock (tests freeze it).
	Clock func() time.Time
	// HSTS adds Strict-Transport-Security (max-age one year); production
	// sets it (ADR-0002 §5).
	HSTS bool
}

// Server is the HTTP handler.
type Server struct {
	svc    *journey.Service
	opt    Options
	log    *slog.Logger
	mux    *http.ServeMux
	limits *limiter
	// authLimits holds the /api/v3 auth buckets, apart from the v2 LRU
	// (v3_ratelimit.go).
	authLimits *specLimiter
	start      time.Time

	closing   chan struct{} // closed by CloseStreams
	closeOnce sync.Once

	v3 v3State
}

// New builds the handler.
func New(svc *journey.Service, opt Options) *Server {
	if opt.Logger == nil {
		opt.Logger = slog.New(slog.DiscardHandler)
	}
	s := &Server{svc: svc, opt: opt, log: opt.Logger, mux: http.NewServeMux(), limits: newLimiter(), authLimits: newSpecLimiter(),
		start: time.Now(), closing: make(chan struct{})}
	if opt.Clock != nil {
		s.limits.now = opt.Clock
		s.authLimits.now = opt.Clock
	}
	s.initV3()
	s.routes()
	return s
}

// CloseStreams ends every open server-sent event stream with a final
// content-free error event, and makes later streams end at once. It is
// idempotent. Register it with http.Server.RegisterOnShutdown: graceful
// shutdown waits for handlers to return, and a stream otherwise only ends
// when its operation does.
func (s *Server) CloseStreams() { s.closeOnce.Do(func() { close(s.closing) }) }

// ServeHTTP applies common middleware.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rid := requestID()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	h := rec.Header()
	h.Set("X-Request-Id", rid)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Permissions-Policy", "camera=(), microphone=(self), geolocation=(), interest-cohort=()")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	if s.opt.HSTS {
		h.Set("Strict-Transport-Security", "max-age=31536000")
	}
	defer func() {
		if p := recover(); p != nil {
			s.log.Error("http.panic", "request_id", rid)
			if !rec.wrote {
				writeError(rec, r, http.StatusInternalServerError, "internal", "")
			}
		}
		// r.Pattern is the matched route template, never the raw path, so
		// identifiers and fragments never reach logs.
		pattern := r.Pattern
		if pattern == "" {
			pattern = "unmatched"
		}
		s.log.Info("http.request", "request_id", rid, "route", pattern, "status", rec.status, "duration_ms", time.Since(start).Milliseconds(), "bytes", rec.bytes)
	}()
	s.mux.ServeHTTP(rec, r)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status, r.wrote = code, true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wrote = true
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Flush supports SSE through the recorder.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack delegates to the underlying writer so websocket upgrades work;
// without it the upgrader answers 500.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("hijack not supported")
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func requestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) clientIP(r *http.Request) string {
	if s.opt.TrustProxy {
		if hop := lastHop(r.Header.Values("X-Forwarded-For")); hop != "" {
			return hop
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// lastHop returns the right-most X-Forwarded-For element across every
// header line, which is the address the trusted proxy saw. Earlier elements
// and earlier lines are client-controlled. A last element that is not an
// address yields "" so the caller falls back to the peer address.
func lastHop(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		parts := strings.Split(lines[i], ",")
		for j := len(parts) - 1; j >= 0; j-- {
			p := strings.TrimSpace(parts[j])
			if p == "" {
				continue
			}
			if a, err := netip.ParseAddr(p); err == nil {
				return a.String()
			}
			if ap, err := netip.ParseAddrPort(p); err == nil {
				return ap.Addr().String()
			}
			return ""
		}
	}
	return ""
}

// sameOrigin enforces the mutation guard: a custom header that a cross-site
// form cannot send, Origin (when present) equal to our origin, and no
// cross-site Fetch Metadata.
func (s *Server) sameOrigin(r *http.Request) bool {
	if r.Header.Get("X-Sela-Request") != "1" {
		return false
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return false
	}
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

// mutation wraps a state-changing handler with origin, content-type, size
// and rate-limit checks.
func (s *Server) mutation(bucket string, maxBody int64, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.sameOrigin(r) {
			writeError(w, r, http.StatusForbidden, "cross_origin", "")
			return
		}
		if r.Method != http.MethodDelete && r.ContentLength != 0 && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeError(w, r, http.StatusUnsupportedMediaType, "unsupported_media", "")
			return
		}
		if !s.limits.allow(bucket, s.clientIP(r)) {
			w.Header().Set("Retry-After", "30")
			writeError(w, r, http.StatusTooManyRequests, journey.CodeRateLimited, "")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		h(w, r)
	}
}

// decode reads a strict JSON body: one value, no trailing data, no unknown
// fields, no duplicate keys and no key whose case differs from the field it
// would fill (encoding/json alone accepts the last two silently).
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeBody(w, r, v, false)
}

// decodeOptional is decode for bodies that may be absent: an empty or
// whitespace-only body leaves v unchanged.
func decodeOptional(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeBody(w, r, v, true)
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any, optional bool) bool {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "too_large", "")
		} else {
			writeError(w, r, http.StatusBadRequest, journey.CodeInvalid, "body")
		}
		return false
	}
	if optional && len(bytes.TrimSpace(raw)) == 0 {
		return true
	}
	if err := strictJSON(raw, reflect.TypeOf(v)); err != nil {
		writeError(w, r, http.StatusBadRequest, journey.CodeInvalid, "body")
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, r, http.StatusBadRequest, journey.CodeInvalid, "body")
		return false
	}
	return true
}

func (s *Server) sessionID(r *http.Request) string {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

func (s *Server) setCookie(w http.ResponseWriter, name, value, path string, expires time.Time) {
	c := &http.Cookie{Name: name, Value: value, Path: path, HttpOnly: true, Secure: s.opt.SecureCookies, SameSite: http.SameSiteStrictMode}
	if value == "" {
		c.MaxAge = -1
	} else {
		c.Expires = expires
	}
	http.SetCookie(w, c)
}
