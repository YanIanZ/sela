package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/fixtures"
)

// v3 cookie names: the __Host- prefix makes browsers insist on Secure,
// Path=/ and no Domain. Plain-HTTP development uses the unprefixed name.
const (
	AuthCookie         = "__Host-sela_auth"
	InsecureAuthCookie = "sela_auth"
)

// v3State is what the /api/v3 surface needs beyond Options.
type v3State struct {
	listening *listeningStore
	cfg       account.Config
	policies  map[string]fixtures.PolicyNotice
	routes    []v3Route
}

func (s *Server) initV3() {
	s.v3.listening = newListeningStore()
	cfg := s.opt.AccountConfig
	if s.opt.Accounts != nil {
		cfg = s.opt.Accounts.Config()
	}
	if cfg.Environment == "" {
		cfg = account.DefaultConfig("development")
	}
	s.v3.cfg = cfg
	s.v3.policies = fixtures.RenderV3Policies(cfg.GuestInactiveTTL, cfg.GuestMaxAge, cfg.RegionLabel)
}

// ownerHandler is a handler that runs with the principal resolved from
// the session cookie; it never reads a user id from the client.
type ownerHandler func(w http.ResponseWriter, r *http.Request, p account.Principal)

// v3Route is one /api/v3 route; the table drives registration and the
// route-level tests (guards, spec coverage).
type v3Route struct {
	method   string
	path     string
	mutation bool
	// public routes never need a store (policies, auth options).
	public bool
	h      http.HandlerFunc
	owner  ownerHandler
}

func (rt v3Route) pattern() string { return rt.method + " " + rt.path }

// v3Routes registers every /api/v3 route (API_SPEC_V3 §4-§6).
func (s *Server) v3Routes() {
	s.v3.routes = []v3Route{
		{method: "GET", path: "/api/v3/conversations", owner: s.v3Rooms},
		{method: "POST", path: "/api/v3/conversations", mutation: true, owner: s.v3CreateRoom},
		{method: "GET", path: "/api/v3/conversations/{id}", owner: s.v3Room},
		{method: "DELETE", path: "/api/v3/conversations/{id}", mutation: true, owner: s.v3EndRoom},
		{method: "POST", path: "/api/v3/conversations/{id}/turns", mutation: true, owner: s.v3SendTurn},
		{method: "DELETE", path: "/api/v3/conversations/{id}/operation", mutation: true, owner: s.v3StopRoom},
		{method: "POST", path: "/api/v3/conversations/{id}/resume", mutation: true, owner: s.v3ResumeRoom},
		{method: "PATCH", path: "/api/v3/conversations/{id}/storage", mutation: true, owner: s.v3RoomStorage},
		{method: "GET", path: "/api/v3/records", owner: s.v3Records},
		{method: "POST", path: "/api/v3/records", mutation: true, owner: s.v3CreateRecord},
		{method: "GET", path: "/api/v3/records/{id}", owner: s.v3Record},
		{method: "PATCH", path: "/api/v3/records/{id}", mutation: true, owner: s.v3UpdateRecord},
		{method: "DELETE", path: "/api/v3/records/{id}", mutation: true, owner: s.v3DeleteRecord},
		{method: "GET", path: "/api/v3/records/{id}/versions", owner: s.v3RecordHistory},

		{method: "GET", path: "/api/v3/memory", owner: s.v3Memory},
		{method: "POST", path: "/api/v3/memory", mutation: true, owner: s.v3CreateMemory},
		{method: "PATCH", path: "/api/v3/memory/{id}", mutation: true, owner: s.v3UpdateMemory},
		{method: "DELETE", path: "/api/v3/memory/{id}", mutation: true, owner: s.v3DeleteMemory},
		{method: "GET", path: "/api/v3/consultation-summaries", owner: s.v3Summaries},
		{method: "POST", path: "/api/v3/consultation-summaries", mutation: true, owner: s.v3CreateSummary},
		{method: "GET", path: "/api/v3/consultation-summaries/{id}", owner: s.v3Summary},
		{method: "PATCH", path: "/api/v3/consultation-summaries/{id}", mutation: true, owner: s.v3UpdateSummary},
		{method: "DELETE", path: "/api/v3/consultation-summaries/{id}", mutation: true, owner: s.v3DeleteSummary},
		{method: "POST", path: "/api/v3/consultation-summaries/{id}/approve", mutation: true, owner: s.v3ApproveSummary},
		{method: "POST", path: "/api/v3/consultation-summaries/{id}/followups", mutation: true, owner: s.v3CreateFollowup},
		{method: "GET", path: "/api/v3/consultation-summaries/{id}/followups", owner: s.v3Followups},

		{method: "GET", path: "/api/v3/policies", public: true, h: s.v3Policies},
		{method: "GET", path: "/api/v3/auth/options", public: true, h: s.v3AuthOptions},

		{method: "POST", path: "/api/v3/auth/register", mutation: true, h: s.v3Register},
		{method: "POST", path: "/api/v3/auth/login", mutation: true, h: s.v3Login},
		{method: "POST", path: "/api/v3/auth/logout", mutation: true, h: s.v3Logout},
		{method: "POST", path: "/api/v3/auth/logout-all", mutation: true, owner: s.v3LogoutAll},
		{method: "POST", path: "/api/v3/auth/guest", mutation: true, h: s.v3Guest},
		{method: "POST", path: "/api/v3/auth/guest/upgrade", mutation: true, owner: s.v3Upgrade},
		{method: "POST", path: "/api/v3/auth/recovery", mutation: true, h: s.v3Recovery},
		{method: "POST", path: "/api/v3/auth/reauth", mutation: true, owner: s.v3Reauth},
		{method: "POST", path: "/api/v3/auth/password", mutation: true, owner: s.v3ChangePassword},
		{method: "POST", path: "/api/v3/auth/recovery-codes", mutation: true, owner: s.v3RegenerateCodes},

		{method: "GET", path: "/api/v3/me", owner: s.v3Me},
		{method: "PATCH", path: "/api/v3/me", mutation: true, owner: s.v3PatchMe},
		{method: "DELETE", path: "/api/v3/me", mutation: true, owner: s.v3DeleteMe},
		{method: "POST", path: "/api/v3/me/recovery-codes/acknowledge", mutation: true, owner: s.v3AcknowledgeCodes},
		{method: "GET", path: "/api/v3/me/sessions", owner: s.v3Sessions},
		{method: "DELETE", path: "/api/v3/me/sessions/{id}", mutation: true, owner: s.v3RevokeSession},
		{method: "GET", path: "/api/v3/me/consents", owner: s.v3Consents},
		{method: "PUT", path: "/api/v3/me/consents/{purpose}", mutation: true, owner: s.v3PutConsent},
		{method: "GET", path: "/api/v3/me/consents/history", owner: s.v3ConsentHistory},
		{method: "GET", path: "/api/v3/me/guardian-consent", owner: s.v3GuardianConsent},
		{method: "PUT", path: "/api/v3/me/guardian-consent", mutation: true, owner: s.v3PutGuardianConsent},
		{method: "DELETE", path: "/api/v3/me/guardian-consent", mutation: true, owner: s.v3DeleteGuardianConsent},
		{method: "GET", path: "/api/v3/me/audit", owner: s.v3Audit},
		{method: "GET", path: "/api/v3/me/export", owner: s.v3Export},
		{method: "GET", path: "/api/v3/asr/realtime", owner: s.v3ASRRealtime},
		{method: "GET", path: "/api/v3/voice/realtime", owner: s.v3VoiceRealtime},
	}
	for _, rt := range s.v3.routes {
		s.mux.HandleFunc(rt.pattern(), s.v3Wrap(rt))
	}
}

// v3Wrap applies, in order: the existing mutation guard (X-Sela-Request,
// Origin, Fetch Metadata, JSON content type, body limit, mutation bucket)
// or the read bucket; 503 store_unavailable without a database; and, for
// owner routes, the principal from the cookie.
func (s *Server) v3Wrap(rt v3Route) http.HandlerFunc {
	inner := rt.h
	if rt.owner != nil {
		inner = s.withOwner(rt.owner)
	}
	if !rt.public {
		next := inner
		inner = func(w http.ResponseWriter, r *http.Request) {
			if s.opt.Accounts == nil {
				retryAfter(w, 30*time.Second)
				writeError(w, r, http.StatusServiceUnavailable, "store_unavailable", "")
				return
			}
			next(w, r)
		}
	}
	if rt.mutation {
		return s.mutation("mutation", smallBody, inner)
	}
	return s.read(inner)
}

func (s *Server) authCookieName() string {
	if s.opt.SecureCookies {
		return AuthCookie
	}
	return InsecureAuthCookie
}

func (s *Server) authToken(r *http.Request) string {
	c, err := r.Cookie(s.authCookieName())
	if err != nil {
		return ""
	}
	return c.Value
}

// setAuthCookie sets the session cookie: HttpOnly, SameSite=Strict,
// Path=/, no Domain, Secure (with the __Host- name) unless insecure
// development; Max-Age only for trusted-device and guest sessions.
func (s *Server) setAuthCookie(w http.ResponseWriter, token string, maxAge time.Duration) {
	c := &http.Cookie{Name: s.authCookieName(), Value: token, Path: "/", HttpOnly: true, Secure: s.opt.SecureCookies, SameSite: http.SameSiteStrictMode}
	if maxAge > 0 {
		c.MaxAge = int(maxAge / time.Second)
	}
	http.SetCookie(w, c)
}

func (s *Server) clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: s.authCookieName(), Value: "", Path: "/", HttpOnly: true, Secure: s.opt.SecureCookies,
		SameSite: http.SameSiteStrictMode, MaxAge: -1})
}

func (s *Server) withOwner(h ownerHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := s.authToken(r)
		if tok == "" {
			s.clearAuthCookie(w)
			writeError(w, r, http.StatusUnauthorized, "not_signed_in", "")
			return
		}
		p, err := s.opt.Accounts.Authenticate(r.Context(), tok)
		if err != nil {
			s.v3Error(w, r, err)
			return
		}
		h(w, r, p)
	}
}

// limitBy applies one v3 bucket and answers 429 with Retry-After.
func (s *Server) limitBy(w http.ResponseWriter, r *http.Request, sp rateSpec, key string) bool {
	ok, wait := s.authLimits.allow(sp, key)
	if !ok {
		retryAfter(w, wait)
		writeError(w, r, http.StatusTooManyRequests, "rate_limited", "")
	}
	return ok
}

func (s *Server) clientKey(r *http.Request) string { return limitKey(s.clientIP(r)) }

// usernameKey is the per-username bucket key: the store's keyed username
// key (HMAC under a server-held secret), the same value the persisted
// login throttle uses, so a memory dump of the limiter does not reveal
// what was typed either. v3Wrap answers 503 before any handler runs
// without a store.
func (s *Server) usernameKey(raw string) string {
	k := s.opt.Accounts.UsernameKey(raw)
	return hex.EncodeToString(k[:])
}

func idKey(id [16]byte) string {
	h := sha256.Sum256(id[:])
	return hex.EncodeToString(h[:16])
}

// authOutcome is the fixed, content-free outcome logged for auth routes.
func authOutcome(err error) string {
	switch account.CodeOf(err) {
	case "":
		if err == nil {
			return "success"
		}
		return "error"
	case "rate_limited":
		return "throttled"
	case "password_login_disabled":
		return "disabled"
	case "server_busy":
		return "busy"
	}
	return "invalid"
}

func (s *Server) logAuth(w http.ResponseWriter, r *http.Request, action string, err error) {
	s.log.Info("auth.event", "request_id", w.Header().Get("X-Request-Id"), "route", r.Pattern, "action", action, "outcome", authOutcome(err))
}
