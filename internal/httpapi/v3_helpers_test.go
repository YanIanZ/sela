package httpapi_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/db/dbtest"
	"github.com/YanIanZ/sela/internal/fixtures"
	"github.com/YanIanZ/sela/internal/httpapi"
	"github.com/YanIanZ/sela/internal/journey"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/store"
	"github.com/YanIanZ/sela/internal/vault"
)

// Synthetic markers; none is real data.
const (
	v3Password    = "Lembah Kabut Sintetis tujuh"
	v3NewPassword = "sungai biru pagi hari tanpa hujan"
)

var bg = context.Background()

type v3clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *v3clock) Now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *v3clock) Advance(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

type v3env struct {
	t     *testing.T
	srv   *httptest.Server
	logs  *syncBuffer
	h     *httpapi.Server
	db    *dbtest.DB
	store *account.Store
	clock *v3clock
	kek   vault.Key
	opt   httpapi.Options
	cfg   account.Config
	ips   atomic.Int64
}

type v3opt struct {
	secure  bool
	mutateC func(*account.Config)
	mutateO func(*httpapi.Options)
	noStore bool
}

func v3Config(c *v3clock) account.Config {
	cfg := account.DefaultConfig("test")
	cfg.Hash = account.FloorHashParams
	cfg.Now = c.Now
	cfg.PolicyVersions = fixtures.V3PolicyVersions()
	cfg.PolicyVersions["test_optional"] = "optional-test.1"
	cfg.Purposes = append(append([]account.Purpose(nil), account.FoundationPurposes...),
		account.Purpose{Name: "test_optional", Kinds: []string{account.KindGuest, account.KindAccount}})
	return cfg
}

func newV3(t *testing.T, o v3opt) *v3env {
	t.Helper()
	e := &v3env{t: t, logs: &syncBuffer{}, clock: &v3clock{now: time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)}}
	e.cfg = v3Config(e.clock)
	if o.mutateC != nil {
		o.mutateC(&e.cfg)
	}
	if !o.noStore {
		e.db = dbtest.New(t)
		secret := make([]byte, 32)
		_, _ = rand.Read(secret)
		e.kek = vault.Key{ID: "test-http", Secret: secret}
		e.store = e.newStore()
	}
	e.opt = httpapi.Options{SecureCookies: o.secure, TrustProxy: true, Accounts: e.store, AccountConfig: e.cfg, Clock: e.clock.Now}
	if o.mutateO != nil {
		o.mutateO(&e.opt)
	}
	e.start()
	return e
}

func (e *v3env) newStore() *account.Store {
	v, err := vault.New(vault.Keyring{Current: e.kek}, vault.Options{Now: e.clock.Now})
	if err != nil {
		e.t.Fatal(err)
	}
	s, err := account.New(e.db.Pool, v, e.cfg)
	if err != nil {
		e.t.Fatal(err)
	}
	// As cmd/sela does at start (ADR-0002 §6).
	if err := s.RefreshHashDistribution(bg); err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *v3env) start() {
	logger := slog.New(slog.NewJSONHandler(e.logs, nil))
	svc := journey.New(journey.Config{Logger: logger, OpTimeout: 5 * time.Second}, store.NewMemory(store.Options{}), fixtures.MustLoad(), scripted{}, llm.NewAdmission(2))
	opt := e.opt
	opt.Logger, opt.Version = logger, "test"
	e.h = httpapi.New(svc, opt)
	e.srv = httptest.NewServer(e.h)
	srv := e.srv
	e.t.Cleanup(func() { svc.Wait(); srv.Close() })
}

// restart replaces the server and store with fresh instances on the same
// database (new limiter, empty DEK cache; the login throttle is in the
// database and survives).
func (e *v3env) restart() {
	e.srv.Close()
	e.store = e.newStore()
	e.opt.Accounts = e.store
	e.start()
}

type v3client struct {
	e    *v3env
	hc   *http.Client
	ip   string
	lang string
}

func (e *v3env) client() *v3client {
	jar, _ := cookiejar.New(nil)
	n := e.ips.Add(1)
	return &v3client{e: e, hc: &http.Client{Jar: jar, Timeout: 30 * time.Second}, ip: fmt.Sprintf("198.51.%d.%d", n/250, n%250+1)}
}

type resp struct {
	status int
	header http.Header
	raw    []byte
	body   map[string]any
}

func (r resp) data() map[string]any { d, _ := r.body["data"].(map[string]any); return d }
func (r resp) code() string {
	e, _ := r.body["error"].(map[string]any)
	c, _ := e["code"].(string)
	return c
}
func (r resp) field() string {
	e, _ := r.body["error"].(map[string]any)
	c, _ := e["field"].(string)
	return c
}

// call sends a request as a same-origin browser would (X-Sela-Request,
// Origin) from the client's address. hdr pairs override headers; an empty
// value deletes one.
func (c *v3client) call(method, path string, body any, hdr ...string) resp {
	c.e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, _ := json.Marshal(b)
		rd = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set("X-Sela-Request", "1")
		req.Header.Set("Origin", c.e.srv.URL)
	}
	req.Header.Set("X-Forwarded-For", c.ip)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Mobile Safari/537.36")
	if c.lang != "" {
		req.Header.Set("X-Sela-Language", c.lang)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		if hdr[i+1] == "" {
			req.Header.Del(hdr[i])
		} else {
			req.Header.Set(hdr[i], hdr[i+1])
		}
	}
	res, err := c.hc.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := resp{status: res.StatusCode, header: res.Header, raw: raw}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func (c *v3client) must(status int, method, path string, body any, hdr ...string) resp {
	c.e.t.Helper()
	r := c.call(method, path, body, hdr...)
	if r.status != status {
		c.e.t.Fatalf("%s %s: got %d %s, want %d", method, path, r.status, r.raw, status)
	}
	return r
}

func (c *v3client) token() string {
	u, _ := url.Parse(c.e.srv.URL)
	for _, ck := range c.hc.Jar.Cookies(u) {
		if ck.Name == httpapi.InsecureAuthCookie {
			return ck.Value
		}
	}
	return ""
}

func (c *v3client) setToken(tok string) {
	u, _ := url.Parse(c.e.srv.URL)
	c.hc.Jar.SetCookies(u, []*http.Cookie{{Name: httpapi.InsecureAuthCookie, Value: tok, Path: "/"}})
}

func policyVersion(key string) string { return fixtures.V3PolicyVersions()[key] }

func (c *v3client) register(username string, extra ...map[string]any) resp {
	c.e.t.Helper()
	body := map[string]any{"username": username, "password": v3Password, "locale": "id", "ageBand": "18_plus",
		"termsPolicyVersion": policyVersion("terms_privacy"), "rememberDevice": false}
	for _, x := range extra {
		for k, v := range x {
			body[k] = v
		}
	}
	return c.must(201, "POST", "/api/v3/auth/register", body)
}

func (c *v3client) login(username, password string) resp {
	c.e.t.Helper()
	return c.call("POST", "/api/v3/auth/login", map[string]any{"username": username, "password": password})
}

func (c *v3client) guest(band string) resp {
	c.e.t.Helper()
	return c.must(201, "POST", "/api/v3/auth/guest", map[string]any{"locale": "id", "ageBand": band,
		"termsPolicyVersion": policyVersion("terms_privacy"), "guestNoticeVersion": policyVersion("guest_storage")})
}

func (c *v3client) profile() map[string]any {
	c.e.t.Helper()
	return c.must(200, "GET", "/api/v3/me", nil).data()
}

func (e *v3env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.db.Pool.QueryRow(bg, sql, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func userID(t *testing.T, r resp) account.UserID {
	t.Helper()
	d := r.data()
	if u, ok := d["user"].(map[string]any); ok {
		d = u
	}
	id, err := account.ParseUUID(fmt.Sprint(d["id"]))
	if err != nil {
		t.Fatalf("no user id in %s", r.raw)
	}
	return account.UserID(id)
}

func sessionID(t *testing.T, r resp) string {
	t.Helper()
	s, _ := r.data()["session"].(map[string]any)
	return fmt.Sprint(s["id"])
}

func setCookies(r resp) []string { return r.header.Values("Set-Cookie") }

func strs(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, fmt.Sprint(x))
	}
	return out
}
