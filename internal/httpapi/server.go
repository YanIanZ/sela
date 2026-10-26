// Package httpapi exposes the /api/v2 JSON API and serves the static
// frontend. It validates transport concerns (size, content type, origin,
// cookies, rate limits, idempotency keys) and delegates every product rule
// to internal/journey. Logs are metadata only (SPEC V12).
package httpapi

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/journey"
	"github.com/YanIanZ/sela/internal/research"
)

// Cookie names. Paths keep each cookie on the API surface that needs it.
const (
	SessionCookie   = "sela_session"
	RecipientCookie = "sela_recipient"
)

// Options configures the HTTP layer.
type Options struct {
	// PublicOrigin, when set (e.g. https://sela.example), is the only Origin
	// accepted for mutations; otherwise the request Host is used.
	PublicOrigin string
	// SecureCookies sets the Secure attribute (required behind HTTPS).
	SecureCookies bool
	// TrustProxy uses the last X-Forwarded-For hop as the client address
	// for rate limiting. Enable only behind a proxy that sets it.
	TrustProxy bool
	// WebDir is the built frontend (web/dist). Empty disables static serving.
	WebDir string
	// Research is the reviewer corpus index; nil reports it unavailable.
	Research *research.Index
	Logger   *slog.Logger
	// Version identifies the running build in /api/v2/health.
	Version string
}

// Server is the HTTP handler.
type Server struct {
	svc    *journey.Service
	opt    Options
	log    *slog.Logger
	mux    *http.ServeMux
	limits *limiter
	start  time.Time
}

// New builds the handler.
func New(svc *journey.Service, opt Options) *Server {
	if opt.Logger == nil {
		opt.Logger = slog.New(slog.DiscardHandler)
	}
	s := &Server{svc: svc, opt: opt, log: opt.Logger, mux: http.NewServeMux(), limits: newLimiter(), start: time.Now()}
	s.routes()
	return s
}

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
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
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

// Hijack is not supported; present so wrappers do not hide it silently.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
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
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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

// decode reads a strict JSON body (unknown fields rejected).
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "too_large", "")
		} else {
			writeError(w, r, http.StatusBadRequest, journey.CodeInvalid, "body")
		}
		return false
	}
	if _, err := dec.Token(); err != io.EOF {
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
