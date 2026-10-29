// Package config reads Sela's runtime configuration from environment
// variables. Secrets (model credentials, the metrics token, the database
// URL and the data KEKs) are only ever read here and passed on; they are
// never logged or returned by String() or in an error.
package config

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgconn"
)

// Environments (ADR-0002 §12).
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvDemo        = "demo"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// Mode selects which guards Load applies.
type Mode int

// Modes.
const (
	// ModeServer is the HTTP server: every guard.
	ModeServer Mode = iota
	// ModeOps is an operator command (migrate, keys, restore, backup, ops):
	// environment, database, TLS and KEK guards, but not the HTTP-surface
	// ones (public origin, cookies, AI mode, v2 journey store).
	ModeOps
	// ModeModel is the evaluation tool talking to the model only: no
	// database or HTTP-surface requirement.
	ModeModel
)

// KEK is one data key-encryption key (secret).
type KEK struct {
	ID     string
	Secret []byte
}

// Config is the validated runtime configuration.
type Config struct {
	Addr          string
	WebDir        string
	PublicOrigin  string
	SecureCookies bool
	TrustProxy    bool
	MaxSessions   int
	// MaxSessionsPerClient bounds live sessions per client address (IPv6 /64).
	MaxSessionsPerClient int
	ResearchKB           string
	LogLevel             string
	Env                  string
	// MetricsToken, when set, enables GET /api/v2/metrics for requests with
	// "Authorization: Bearer <MetricsToken>". Never logged.
	MetricsToken string

	LLMMode        string // disabled | modal | test_double
	LLMBaseURL     string
	LLMAPIKey      string
	ModalKey       string
	ModalSecret    string
	ServedModel    string
	ModelID        string
	ModelRevision  string
	LLMConcurrency int
	OpTimeout      time.Duration
	WarmWindow     time.Duration
	ColdStart      time.Duration
	MaxOpsPerHour  int

	// ASRMode is the server-side dictation backend: "disabled" (mic off),
	// "meta" (Meta voice-transcribe realtime via SELA_ASR_API_KEY),
	// "bedrock" (Amazon Nova Sonic speech-to-speech via AWS credentials),
	// or "test_double" (local loopback for development/test only).
	ASRMode    string
	ASRAPIKey  string
	ASRModel   string
	ASRURL     string
	AWSRegion  string
	AWSProfile string

	// Store is the v2 journey store: "memory" (development, test, demo
	// only) or "disabled" (v2 journey routes not served).
	Store string
	// DatabaseURL is SELA_DATABASE_URL (secret); empty = no /api/v3 store.
	DatabaseURL        string
	DBAutoMigrate      bool
	DBMaxConns         int
	DBStatementTimeout time.Duration
	// KEK is the current data key-encryption key; KEKPrevious unwrap only.
	KEK         KEK
	KEKPrevious []KEK

	GuestInactiveTTL        time.Duration
	GuestMaxAge             time.Duration
	MaxGuestAccounts        int
	AllowedAgeBands         []string
	GuardianConsentRequired bool
	RegionLabel             string
	HashConcurrency         int
	Argon2MemoryKiB         int
	Argon2Iterations        int
	Argon2Parallelism       int
	SessionIdle             time.Duration
	SessionAbsolute         time.Duration
	TrustedIdle             time.Duration
	TrustedAbsolute         time.Duration
	ReauthWindow            time.Duration
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func envBool(key string, def bool) (bool, error) {
	v := env(key, "")
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: invalid boolean", key)
	}
	return b, nil
}

// Bounds of SELA_DB_MAX_CONNS. One connection is enough for the server
// and its background jobs: a request holds at most one connection at a
// time, and a job runs every statement on the connection that holds its
// advisory lock (db.WithJobLock), so it never waits for a second one.
const (
	MinDBMaxConns = 1
	MaxDBMaxConns = 200
)

func envInt(key string, def, min, max int) (int, error) {
	v := env(key, "")
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("%s: must be an integer in [%d, %d]", key, min, max)
	}
	return n, nil
}

func envDuration(key string, def, min, max time.Duration) (time.Duration, error) {
	v := env(key, "")
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < min || d > max {
		return 0, fmt.Errorf("%s: must be a duration in [%s, %s]", key, min, max)
	}
	return d, nil
}

// minMetricsToken keeps the operator's metrics token out of guessing range.
const minMetricsToken = 16

// kekIDPattern is the KEK id shape; its prefix names the environment.
var kekIDPattern = regexp.MustCompile(`^(dev|test|demo|staging|prod)-[a-z0-9-]{1,40}$`)

// KEKPrefix is the KEK id prefix an environment requires.
var KEKPrefix = map[string]string{EnvDevelopment: "dev", EnvTest: "test", EnvDemo: "demo", EnvStaging: "staging", EnvProduction: "prod"}

// AllAgeBands are the bands that can be enabled.
var AllAgeBands = []string{"15_17", "18_plus", "unknown"}

const day = 24 * time.Hour

// Load reads and validates the environment for the server.
func Load() (Config, error) { return LoadMode(ModeServer) }

// LoadMode reads and validates the environment for one kind of program.
func LoadMode(mode Mode) (Config, error) {
	var c Config
	var errs []error
	add := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}
	var err error
	c.Env = env("SELA_ENV", EnvProduction)
	if KEKPrefix[c.Env] == "" {
		add(errors.New("SELA_ENV: must be production, staging, demo, development or test"))
	}
	public := c.Env == EnvProduction || c.Env == EnvStaging || c.Env == EnvDemo
	c.Addr = env("SELA_ADDR", ":8080")
	c.WebDir = env("SELA_WEB_DIR", "web/dist")
	c.PublicOrigin = env("SELA_PUBLIC_ORIGIN", "")
	c.ResearchKB = env("SELA_RESEARCH_KB", "data/research/corpus/knowledge_base.jsonl")
	c.LogLevel = env("SELA_LOG_LEVEL", "info")
	c.SecureCookies, err = envBool("SELA_SECURE_COOKIES", public)
	add(err)
	c.TrustProxy, err = envBool("SELA_TRUST_PROXY", false)
	add(err)
	c.MaxSessions, err = envInt("SELA_MAX_SESSIONS", 500, 1, 100000)
	add(err)
	c.MaxSessionsPerClient, err = envInt("SELA_MAX_SESSIONS_PER_CLIENT", 50, 1, 100000)
	add(err)
	c.MetricsToken = env("SELA_METRICS_TOKEN", "")
	if c.MetricsToken != "" && (len(c.MetricsToken) < minMetricsToken || strings.ContainsAny(c.MetricsToken, " \t\r\n")) {
		add(fmt.Errorf("SELA_METRICS_TOKEN: must be at least %d characters without whitespace", minMetricsToken))
	}

	c.LLMMode = env("SELA_LLM_MODE", "disabled")
	c.LLMBaseURL = env("SELA_LLM_BASE_URL", "")
	c.LLMAPIKey = env("SELA_LLM_API_KEY", "")
	c.ModalKey = env("SELA_MODAL_KEY", "")
	c.ModalSecret = env("SELA_MODAL_SECRET", "")
	c.ServedModel = env("SELA_LLM_SERVED_MODEL", "")
	c.ModelID = env("SELA_LLM_MODEL_ID", "")
	c.ModelRevision = env("SELA_LLM_MODEL_REVISION", "")
	c.LLMConcurrency, err = envInt("SELA_LLM_CONCURRENCY", 2, 1, 64)
	add(err)
	c.OpTimeout, err = envDuration("SELA_LLM_OP_TIMEOUT", 6*time.Minute, 10*time.Second, 15*time.Minute)
	add(err)
	c.WarmWindow, err = envDuration("SELA_LLM_WARM_WINDOW", 5*time.Minute, time.Minute, time.Hour)
	add(err)
	c.ColdStart, err = envDuration("SELA_LLM_COLD_START_ESTIMATE", 4*time.Minute, 10*time.Second, 30*time.Minute)
	add(err)
	c.MaxOpsPerHour, err = envInt("SELA_MAX_AI_OPS_PER_SESSION", 8, 1, 1000)
	add(err)

	c.ASRMode = env("SELA_ASR_MODE", "disabled")
	c.ASRAPIKey = env("SELA_ASR_API_KEY", "")
	c.ASRModel = env("SELA_ASR_MODEL", "")
	c.ASRURL = env("SELA_ASR_URL", "wss://api.meta.ai/v1/asr/realtime")
	c.AWSRegion = env("AWS_REGION", env("AWS_DEFAULT_REGION", "us-east-1"))
	c.AWSProfile = env("AWS_PROFILE", "")
	if c.ASRMode == "meta" && c.ASRModel == "" {
		c.ASRModel = "muse-voice-transcribe-1.0"
	}
	if c.ASRMode == "bedrock" && c.ASRModel == "" {
		c.ASRModel = "amazon.nova-2-5-sonic"
	}

	switch c.LLMMode {
	case "disabled":
	case "modal":
		if !strings.HasPrefix(c.LLMBaseURL, "https://") || c.LLMAPIKey == "" || c.ModalKey == "" || c.ModalSecret == "" ||
			c.ServedModel == "" || c.ModelID == "" || c.ModelRevision == "" {
			add(errors.New("SELA_LLM_MODE=modal requires https SELA_LLM_BASE_URL, SELA_LLM_API_KEY, SELA_MODAL_KEY, SELA_MODAL_SECRET, SELA_LLM_SERVED_MODEL, SELA_LLM_MODEL_ID and SELA_LLM_MODEL_REVISION"))
		}
	case "test_double":
		allow, _ := envBool("SELA_ALLOW_TEST_DOUBLE", false)
		if mode == ModeServer && (!allow || public) {
			add(errors.New("SELA_LLM_MODE=test_double requires SELA_ALLOW_TEST_DOUBLE=true and SELA_ENV development or test (never demo, staging or production)"))
		}
		if c.LLMBaseURL == "" {
			c.LLMBaseURL = "http://127.0.0.1:8090"
		}
		if c.ServedModel == "" {
			c.ServedModel = "test-double-not-meta"
		}
	default:
		add(errors.New("SELA_LLM_MODE: must be disabled, modal or test_double"))
	}

	switch c.ASRMode {
	case "disabled":
	case "test_double":
		allow, _ := envBool("SELA_ALLOW_TEST_DOUBLE", false)
		if public || (mode == ModeServer && !allow) {
			add(errors.New("SELA_ASR_MODE=test_double is refused in demo, staging and production (V42), and in development and test requires SELA_ALLOW_TEST_DOUBLE=true"))
		}
	case "meta":
		if c.ASRAPIKey == "" || c.ASRModel == "" || c.ASRURL == "" {
			add(errors.New("SELA_ASR_MODE=meta requires SELA_ASR_API_KEY, SELA_ASR_MODEL and SELA_ASR_URL"))
		}
		if (c.Env == EnvProduction || c.Env == EnvStaging) && !strings.HasPrefix(c.ASRURL, "wss://") {
			add(errors.New("SELA_ASR_URL in staging and production must be wss:// (plain ws never carries dictation audio)"))
		}
	case "bedrock":
		if c.ASRModel == "" || c.AWSRegion == "" {
			add(errors.New("SELA_ASR_MODE=bedrock requires SELA_ASR_MODEL and AWS_REGION"))
		}
	default:
		add(errors.New("SELA_ASR_MODE: must be disabled, meta, bedrock or test_double"))
	}

	loadV3(&c, mode, public, add)

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return c, nil
}

// loadV3 reads the persistence, encryption and account settings and
// applies the per-environment guards (ADR-0002 §5, §7, §8, §12).
func loadV3(c *Config, mode Mode, public bool, add func(error)) {
	var err error
	strict := c.Env == EnvProduction || c.Env == EnvStaging

	// v2 journey store (SELA_STORE, alias SELA_V2_JOURNEY=on|off).
	defStore := "memory"
	if strict {
		defStore = "disabled"
	}
	c.Store = env("SELA_STORE", "")
	switch j := env("SELA_V2_JOURNEY", ""); j {
	case "":
	case "on", "off":
		alias := map[string]string{"on": "memory", "off": "disabled"}[j]
		if c.Store != "" && c.Store != alias {
			add(errors.New("SELA_STORE and SELA_V2_JOURNEY disagree"))
		}
		c.Store = alias
	default:
		add(errors.New("SELA_V2_JOURNEY: must be on or off"))
	}
	if c.Store == "" {
		c.Store = defStore
	}
	if c.Store != "memory" && c.Store != "disabled" {
		add(errors.New("SELA_STORE: must be memory or disabled"))
	}

	c.DatabaseURL = env("SELA_DATABASE_URL", "")
	c.DBAutoMigrate, err = envBool("SELA_DB_AUTO_MIGRATE", false)
	add(err)
	c.DBMaxConns, err = envInt("SELA_DB_MAX_CONNS", 10, MinDBMaxConns, MaxDBMaxConns)
	add(err)
	c.DBStatementTimeout, err = envDuration("SELA_DB_STATEMENT_TIMEOUT", 30*time.Second, time.Second, 10*time.Minute)
	add(err)
	if c.DatabaseURL != "" {
		add(checkDatabaseURL(c.DatabaseURL, strict))
	}
	if c.DBAutoMigrate && c.Env != EnvDevelopment && c.Env != EnvTest {
		add(errors.New("SELA_DB_AUTO_MIGRATE: allowed only in development and test; run sela migrate up"))
	}

	c.KEK, c.KEKPrevious = loadKEKs(c.Env, add)
	if c.DatabaseURL != "" && c.KEK.ID == "" {
		add(errors.New("SELA_DATABASE_URL needs SELA_DATA_KEK and SELA_DATA_KEK_ID"))
	}
	if public && mode != ModeModel {
		if c.DatabaseURL == "" {
			add(fmt.Errorf("SELA_DATABASE_URL: required when SELA_ENV=%s", c.Env))
		}
		if c.KEK.ID == "" {
			add(fmt.Errorf("SELA_DATA_KEK: required when SELA_ENV=%s", c.Env))
		}
	}

	c.GuestInactiveTTL, err = envDuration("SELA_GUEST_INACTIVE_TTL", 0, time.Hour, 30*day)
	add(err)
	alias, err := envDuration("SELA_GUEST_TTL", 0, time.Hour, 30*day)
	add(err)
	switch {
	case alias != 0 && c.GuestInactiveTTL != 0 && alias != c.GuestInactiveTTL:
		add(errors.New("SELA_GUEST_TTL and SELA_GUEST_INACTIVE_TTL disagree"))
	case c.GuestInactiveTTL == 0 && alias != 0:
		c.GuestInactiveTTL = alias
	case c.GuestInactiveTTL == 0:
		c.GuestInactiveTTL = 7 * day
	}
	c.GuestMaxAge, err = envDuration("SELA_GUEST_MAX_AGE", 30*day, day, 30*day)
	add(err)
	if c.GuestMaxAge > 0 && c.GuestInactiveTTL > c.GuestMaxAge {
		add(errors.New("SELA_GUEST_INACTIVE_TTL: must not exceed SELA_GUEST_MAX_AGE"))
	}
	c.MaxGuestAccounts, err = envInt("SELA_MAX_GUEST_ACCOUNTS", 10000, 1, 1_000_000)
	add(err)
	c.GuardianConsentRequired, err = envBool("SELA_GUARDIAN_CONSENT_REQUIRED", true)
	add(err)
	c.RegionLabel = env("SELA_DATA_REGION_LABEL", "")
	if len([]rune(c.RegionLabel)) > 64 || strings.IndexFunc(c.RegionLabel, unicode.IsControl) >= 0 {
		add(errors.New("SELA_DATA_REGION_LABEL: at most 64 printable characters"))
	}

	defBands := "15_17,18_plus,unknown"
	if c.Env == EnvProduction {
		defBands = "18_plus"
	}
	c.AllowedAgeBands = nil
	for _, b := range strings.Split(env("SELA_ALLOWED_AGE_BANDS", defBands), ",") {
		b = strings.TrimSpace(b)
		if !slices.Contains(AllAgeBands, b) {
			add(errors.New("SELA_ALLOWED_AGE_BANDS: comma-separated list of 15_17, 18_plus, unknown"))
			break
		}
		if !slices.Contains(c.AllowedAgeBands, b) {
			c.AllowedAgeBands = append(c.AllowedAgeBands, b)
		}
	}

	c.HashConcurrency, err = envInt("SELA_AUTH_HASH_CONCURRENCY", 2, 1, 64)
	add(err)
	c.Argon2MemoryKiB, err = envInt("SELA_ARGON2_MEMORY_KIB", 65536, 19456, 1<<20)
	add(err)
	c.Argon2Iterations, err = envInt("SELA_ARGON2_ITERATIONS", 2, 2, 10)
	add(err)
	c.Argon2Parallelism, err = envInt("SELA_ARGON2_PARALLELISM", 1, 1, 16)
	add(err)
	c.SessionIdle, err = envDuration("SELA_SESSION_IDLE", 2*time.Hour, 5*time.Minute, day)
	add(err)
	c.SessionAbsolute, err = envDuration("SELA_SESSION_ABSOLUTE", 12*time.Hour, time.Hour, 7*day)
	add(err)
	c.TrustedIdle, err = envDuration("SELA_SESSION_TRUSTED_IDLE", 7*day, time.Hour, 30*day)
	add(err)
	c.TrustedAbsolute, err = envDuration("SELA_SESSION_TRUSTED_ABSOLUTE", 30*day, time.Hour, 30*day)
	add(err)
	c.ReauthWindow, err = envDuration("SELA_REAUTH_WINDOW", 10*time.Minute, time.Minute, time.Hour)
	add(err)
	if c.SessionIdle > c.SessionAbsolute || c.TrustedIdle > c.TrustedAbsolute {
		add(errors.New("SELA_SESSION_*: an idle timeout must not exceed its absolute timeout"))
	}

	if mode != ModeServer {
		return
	}
	if public {
		if !c.SecureCookies {
			add(fmt.Errorf("SELA_SECURE_COOKIES=false: refused when SELA_ENV=%s", c.Env))
		}
		if u, err := url.Parse(c.PublicOrigin); c.PublicOrigin == "" || err != nil || u.Scheme != "https" || u.Host == "" {
			add(fmt.Errorf("SELA_PUBLIC_ORIGIN: an https origin is required when SELA_ENV=%s", c.Env))
		}
	}
	if strict && c.Store == "memory" {
		add(fmt.Errorf("SELA_STORE=memory (the v2 in-memory journey store) is refused when SELA_ENV=%s", c.Env))
	}
}

// checkDatabaseURL accepts a postgres:// URL and, in staging and
// production, requires certificate and host-name verification (sslmode
// verify-full) for every host that is not a unix socket or loopback
// address. It checks the settings pgx will actually connect with, not the
// URL text: a URL without a host, sslmode or with ?service= takes them
// from PGHOST, PGSSLMODE and libpq service files, so the URL alone would
// let a remote, unverified or plaintext connection through. Every host
// and every fallback (sslmode prefer adds a plaintext one) must pass.
// Errors never contain the URL.
func checkDatabaseURL(raw string, strict bool) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return errors.New("SELA_DATABASE_URL: must be a postgres:// URL")
	}
	if !strict {
		return nil
	}
	pc, err := pgconn.ParseConfig(raw)
	if err != nil {
		return errors.New("SELA_DATABASE_URL: the connection settings (with PG* variables and service files) cannot be parsed")
	}
	type target struct {
		host string
		tls  *tls.Config
	}
	targets := []target{{pc.Host, pc.TLSConfig}}
	for _, fb := range pc.Fallbacks {
		targets = append(targets, target{fb.Host, fb.TLSConfig})
	}
	for _, t := range targets {
		if isLocal(t.host) {
			continue
		}
		if t.tls == nil || t.tls.InsecureSkipVerify || t.tls.ServerName == "" {
			return errors.New("SELA_DATABASE_URL: sslmode=verify-full is required for a non-local database in staging and production " +
				"(checked on the effective settings, including PGHOST, PGSSLMODE and service files)")
		}
	}
	return nil
}

func isLocal(host string) bool {
	if host == "" || strings.HasPrefix(host, "/") || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func decodeKEK(name, s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(b) != 32 {
		return nil, fmt.Errorf("%s: must be standard base64 of exactly 32 bytes", name)
	}
	return b, nil
}

func checkKEKID(name, id, env string) error {
	if !kekIDPattern.MatchString(id) {
		return fmt.Errorf("%s: must match (dev|test|demo|staging|prod)-[a-z0-9-]{1,40}", name)
	}
	if want := KEKPrefix[env]; want != "" && !strings.HasPrefix(id, want+"-") {
		return fmt.Errorf("%s: a %s key cannot be used when SELA_ENV=%s (prefix %q required)", name, strings.SplitN(id, "-", 2)[0], env, want)
	}
	return nil
}

func loadKEKs(envName string, add func(error)) (KEK, []KEK) {
	var cur KEK
	secret, id := env("SELA_DATA_KEK", ""), env("SELA_DATA_KEK_ID", "")
	switch {
	case secret == "" && id == "":
	case secret == "" || id == "":
		add(errors.New("SELA_DATA_KEK and SELA_DATA_KEK_ID must be set together"))
	default:
		b, err := decodeKEK("SELA_DATA_KEK", secret)
		add(err)
		if e := checkKEKID("SELA_DATA_KEK_ID", id, envName); e != nil {
			add(e)
		} else if err == nil {
			cur = KEK{ID: id, Secret: b}
		}
	}
	var prev []KEK
	if p := env("SELA_DATA_KEK_PREVIOUS", ""); p != "" {
		seen := map[string]bool{cur.ID: true}
		for _, item := range strings.Split(p, ",") {
			pid, ps, ok := strings.Cut(strings.TrimSpace(item), ":")
			if !ok {
				add(errors.New("SELA_DATA_KEK_PREVIOUS: entries are id:base64, comma separated"))
				continue
			}
			if err := checkKEKID("SELA_DATA_KEK_PREVIOUS", pid, envName); err != nil {
				add(err)
				continue
			}
			b, err := decodeKEK("SELA_DATA_KEK_PREVIOUS", ps)
			if err != nil {
				add(err)
				continue
			}
			if seen[pid] {
				add(errors.New("SELA_DATA_KEK_PREVIOUS: duplicate or current key id"))
				continue
			}
			seen[pid] = true
			prev = append(prev, KEK{ID: pid, Secret: b})
		}
	}
	return cur, prev
}

// RequiresDatabase reports environments whose readiness depends on the
// database.
func (c Config) RequiresDatabase() bool {
	return c.Env == EnvProduction || c.Env == EnvStaging || c.Env == EnvDemo
}

// String describes the configuration without secrets: the database URL
// and KEKs appear only as "configured" and the KEK id.
func (c Config) String() string {
	return fmt.Sprintf("env=%s addr=%s web=%s llm=%s model=%s@%s secure_cookies=%t max_sessions=%d metrics=%t store=%s database=%t kek_id=%s kek_previous=%d age_bands=%s",
		c.Env, c.Addr, c.WebDir, c.LLMMode, c.ModelID, c.ModelRevision, c.SecureCookies, c.MaxSessions, c.MetricsToken != "",
		c.Store, c.DatabaseURL != "", c.KEK.ID, len(c.KEKPrevious), strings.Join(c.AllowedAgeBands, ","))
}
