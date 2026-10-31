package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testKEK is a synthetic 32-byte key used only by these tests.
var testKEK = base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))

var otherKEK = base64.StdEncoding.EncodeToString([]byte("fedcba9876543210fedcba9876543210"))

var v3Keys = []string{"SELA_STORE", "SELA_V2_JOURNEY", "SELA_DATABASE_URL", "SELA_DB_AUTO_MIGRATE", "SELA_DB_MAX_CONNS",
	"SELA_DB_STATEMENT_TIMEOUT", "SELA_DATA_KEK", "SELA_DATA_KEK_ID", "SELA_DATA_KEK_PREVIOUS", "SELA_GUEST_INACTIVE_TTL",
	"SELA_GUEST_TTL", "SELA_GUEST_MAX_AGE", "SELA_MAX_GUEST_ACCOUNTS", "SELA_GUARDIAN_CONSENT_REQUIRED", "SELA_DATA_REGION_LABEL",
	"SELA_ALLOWED_AGE_BANDS", "SELA_AUTH_HASH_CONCURRENCY", "SELA_ARGON2_MEMORY_KIB", "SELA_ARGON2_ITERATIONS",
	"SELA_ARGON2_PARALLELISM", "SELA_SESSION_IDLE", "SELA_SESSION_ABSOLUTE", "SELA_SESSION_TRUSTED_IDLE",
	"SELA_SESSION_TRUSTED_ABSOLUTE", "SELA_REAUTH_WINDOW"}

var prefixes = map[string]string{"development": "dev", "test": "test", "staging": "staging", "production": "prod"}

// valid sets a complete valid configuration for an environment.
func valid(t *testing.T, env string) {
	t.Helper()
	clean(t)
	t.Setenv("SELA_ENV", env)
	t.Setenv("SELA_DATABASE_URL", "postgres://sela:secret-db-password@db.internal.example:5432/sela?sslmode=verify-full")
	t.Setenv("SELA_DATA_KEK", testKEK)
	t.Setenv("SELA_DATA_KEK_ID", prefixes[env]+"-k1")
	if env == "development" || env == "test" {
		t.Setenv("SELA_SECURE_COOKIES", "false")
	} else {
		t.Setenv("SELA_PUBLIC_ORIGIN", "https://sela.example")
	}
}

func TestLoadEnvironmentMatrix(t *testing.T) {
	type rule struct {
		name string
		set  map[string]string
		// refused lists the environments that must refuse the setting.
		refused []string
	}
	all := []string{"development", "test", "staging", "production"}
	rules := []rule{
		{"no database", map[string]string{"SELA_DATABASE_URL": "", "SELA_DATA_KEK": "", "SELA_DATA_KEK_ID": ""}, []string{"staging", "production"}},
		{"no KEK with a database", map[string]string{"SELA_DATA_KEK": "", "SELA_DATA_KEK_ID": ""}, all},
		{"plain-text remote database", map[string]string{"SELA_DATABASE_URL": "postgres://u:p@db.example:5432/sela?sslmode=require"}, []string{"staging", "production"}},
		{"remote database without sslmode", map[string]string{"SELA_DATABASE_URL": "postgres://u:p@10.0.0.5/sela"}, []string{"staging", "production"}},
		{"loopback database", map[string]string{"SELA_DATABASE_URL": "postgres://u:p@127.0.0.1:5432/sela?sslmode=disable"}, nil},
		{"unix socket database", map[string]string{"SELA_DATABASE_URL": "postgres://u:p@/sela?host=/var/run/postgresql"}, nil},
		{"not a postgres URL", map[string]string{"SELA_DATABASE_URL": "mysql://u:p@127.0.0.1/sela"}, all},
		{"insecure cookies", map[string]string{"SELA_SECURE_COOKIES": "false"}, []string{"staging", "production"}},
		{"http public origin", map[string]string{"SELA_PUBLIC_ORIGIN": "http://sela.example"}, []string{"staging", "production"}},
		{"no public origin", map[string]string{"SELA_PUBLIC_ORIGIN": ""}, []string{"staging", "production"}},
		{"test double", map[string]string{"SELA_LLM_MODE": "test_double", "SELA_ALLOW_TEST_DOUBLE": "true"}, []string{"staging", "production"}},
		{"memory journey store", map[string]string{"SELA_STORE": "memory"}, []string{"staging", "production"}},
		{"memory journey store (alias)", map[string]string{"SELA_V2_JOURNEY": "on"}, []string{"staging", "production"}},
		{"auto-migrate", map[string]string{"SELA_DB_AUTO_MIGRATE": "true"}, []string{"staging", "production"}},
		{"bad band", map[string]string{"SELA_ALLOWED_AGE_BANDS": "under_15"}, all},
		{"bad explicit 15 band", map[string]string{"SELA_ALLOWED_AGE_BANDS": "under_16"}, all},
		{"argon2 below floor", map[string]string{"SELA_ARGON2_MEMORY_KIB": "19455"}, all},
		{"argon2 one pass", map[string]string{"SELA_ARGON2_ITERATIONS": "1"}, all},
		{"guest TTL beyond max age", map[string]string{"SELA_GUEST_INACTIVE_TTL": "240h", "SELA_GUEST_MAX_AGE": "168h"}, all},
		{"guest max age over 30 days", map[string]string{"SELA_GUEST_MAX_AGE": "721h"}, all},
		{"trusted session over 30 days", map[string]string{"SELA_SESSION_TRUSTED_ABSOLUTE": "721h"}, all},
		{"idle beyond absolute", map[string]string{"SELA_SESSION_IDLE": "13h"}, all},
		{"conflicting guest TTL alias", map[string]string{"SELA_GUEST_TTL": "24h", "SELA_GUEST_INACTIVE_TTL": "48h"}, all},
		{"region label with control character", map[string]string{"SELA_DATA_REGION_LABEL": "a\x01b"}, all},
	}
	for _, env := range all {
		t.Run(env+"/baseline", func(t *testing.T) {
			valid(t, env)
			if _, err := Load(); err != nil {
				t.Fatalf("valid %s configuration refused: %v", env, err)
			}
		})
		for _, r := range rules {
			t.Run(env+"/"+r.name, func(t *testing.T) {
				valid(t, env)
				for k, v := range r.set {
					t.Setenv(k, v)
				}
				_, err := Load()
				if want := slices.Contains(r.refused, env); want != (err != nil) {
					t.Fatalf("refused=%t, want %t (%v)", err != nil, want, err)
				}
				if err != nil && strings.Contains(err.Error(), "secret-db-password") {
					t.Fatalf("error leaks the database URL: %v", err)
				}
			})
		}
	}
}

func TestMetaAPIProductionAck(t *testing.T) {
	valid(t, "production")
	t.Setenv("SELA_LLM_MODE", "meta_api")
	t.Setenv("SELA_LLM_API_KEY", "test-key")
	t.Setenv("SELA_LLM_SERVED_MODEL", "muse-spark-1.3")
	t.Setenv("SELA_LLM_MODEL_ID", "muse-spark-1.3")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted meta_api without the owner acknowledgement")
	}
	t.Setenv("SELA_META_API_PRODUCTION_ACK", "B-10-owner")
	if _, err := Load(); err != nil {
		t.Fatalf("production with the owner acknowledgement: %v", err)
	}
}

func TestLoadEnvironmentDefaults(t *testing.T) {
	for env, want := range map[string]struct {
		bands  []string
		store  string
		secure bool
	}{
		"production":  {[]string{"18_plus"}, "disabled", true},
		"staging":     {[]string{"15_17", "18_plus", "unknown"}, "disabled", true},
		"development": {[]string{"15_17", "18_plus", "unknown"}, "memory", false},
		"test":        {[]string{"15_17", "18_plus", "unknown"}, "memory", false},
	} {
		t.Run(env, func(t *testing.T) {
			valid(t, env)
			t.Setenv("SELA_SECURE_COOKIES", "")
			if env == "development" || env == "test" {
				t.Setenv("SELA_DATABASE_URL", "")
				t.Setenv("SELA_DATA_KEK", "")
				t.Setenv("SELA_DATA_KEK_ID", "")
			}
			c, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(c.AllowedAgeBands, want.bands) || c.Store != want.store || c.SecureCookies != want.secure {
				t.Fatalf("bands %v store %s secure %t", c.AllowedAgeBands, c.Store, c.SecureCookies)
			}
			if c.GuestInactiveTTL != 168*time.Hour || c.GuestMaxAge != 720*time.Hour || c.SessionIdle != 2*time.Hour ||
				c.SessionAbsolute != 12*time.Hour || c.TrustedIdle != 168*time.Hour || c.TrustedAbsolute != 720*time.Hour ||
				c.ReauthWindow != 10*time.Minute || c.Argon2MemoryKiB != 65536 || c.Argon2Iterations != 2 || c.Argon2Parallelism != 1 ||
				c.HashConcurrency != 2 || c.MaxGuestAccounts != 10000 || !c.GuardianConsentRequired {
				t.Fatalf("defaults: %+v", c)
			}
		})
	}
	// Production may enable minors explicitly (D-031, after legal review).
	valid(t, "production")
	t.Setenv("SELA_ALLOWED_AGE_BANDS", "15_17, 18_plus")
	c, err := Load()
	if err != nil || !slices.Equal(c.AllowedAgeBands, []string{"15_17", "18_plus"}) {
		t.Fatalf("explicit bands: %v %v", c.AllowedAgeBands, err)
	}
	// The guest TTL alias.
	valid(t, "test")
	t.Setenv("SELA_GUEST_TTL", "48h")
	if c, err := Load(); err != nil || c.GuestInactiveTTL != 48*time.Hour {
		t.Fatalf("SELA_GUEST_TTL: %v %v", c.GuestInactiveTTL, err)
	}
}

func TestKEKConfig(t *testing.T) {
	// The KEK must be standard base64 of exactly 32 bytes.
	for name, v := range map[string]string{
		"16 bytes":  base64.StdEncoding.EncodeToString(make([]byte, 16)),
		"33 bytes":  base64.StdEncoding.EncodeToString(make([]byte, 33)),
		"not b64":   "not base64 at all!!",
		"url b64":   base64.URLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcd\xff\xfe")),
		"hex value": "30313233343536373839616263646566303132333435363738396162636465",
	} {
		t.Run(name, func(t *testing.T) {
			valid(t, "staging")
			t.Setenv("SELA_DATA_KEK", v)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), "SELA_DATA_KEK") {
				t.Fatalf("accepted: %v", err)
			}
		})
	}
	// The id prefix must match SELA_ENV, so a staging key never loads in
	// production and the reverse.
	for _, env := range []string{"development", "test", "staging", "production"} {
		for envKey, prefix := range prefixes {
			t.Run(env+"/"+prefix, func(t *testing.T) {
				valid(t, env)
				t.Setenv("SELA_DATA_KEK_ID", prefix+"-key-1")
				_, err := Load()
				if (envKey == env) != (err == nil) {
					t.Fatalf("env %s with %s key: %v", env, prefix, err)
				}
			})
		}
	}
	for _, id := range []string{"prod", "prod-", "prod_2026", "PROD-2026", "prod-" + strings.Repeat("a", 41), "live-2026"} {
		valid(t, "production")
		t.Setenv("SELA_DATA_KEK_ID", id)
		if _, err := Load(); err == nil {
			t.Errorf("KEK id %q accepted", id)
		}
	}
	// Both or neither.
	valid(t, "development")
	t.Setenv("SELA_DATA_KEK_ID", "")
	if _, err := Load(); err == nil {
		t.Error("KEK without id accepted")
	}
	// Previous keys: id:base64 pairs, same environment, no duplicates.
	valid(t, "production")
	t.Setenv("SELA_DATA_KEK_PREVIOUS", "prod-old-1:"+otherKEK)
	c, err := Load()
	if err != nil || len(c.KEKPrevious) != 1 || c.KEKPrevious[0].ID != "prod-old-1" || len(c.KEKPrevious[0].Secret) != 32 {
		t.Fatalf("previous: %+v %v", c.KEKPrevious, err)
	}
	for _, bad := range []string{"demo-old:" + otherKEK, "prod-old-1", "prod-old-1:short", "prod-k1:" + otherKEK,
		"prod-a:" + otherKEK + ",prod-a:" + otherKEK} {
		t.Setenv("SELA_DATA_KEK_PREVIOUS", bad)
		if _, err := Load(); err == nil {
			t.Errorf("previous %q accepted", bad)
		}
	}
	// Secrets never appear in String() or in errors.
	valid(t, "production")
	t.Setenv("SELA_DATA_KEK_PREVIOUS", "prod-old-1:"+otherKEK)
	c, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	s := c.String()
	for _, secret := range []string{testKEK, otherKEK, "secret-db-password", "db.internal.example", c.DatabaseURL} {
		if strings.Contains(s, secret) {
			t.Fatalf("String() leaks a secret: %s", s)
		}
	}
	if !strings.Contains(s, "kek_id=prod-k1") || !strings.Contains(s, "database=true") {
		t.Fatalf("String(): %s", s)
	}
	t.Setenv("SELA_DATA_KEK", "bm90IDMyIGJ5dGVz")
	if _, err := Load(); err == nil || strings.Contains(err.Error(), "bm90IDMyIGJ5dGVz") {
		t.Fatalf("error leaks the KEK value: %v", err)
	}
}

func TestLoadModes(t *testing.T) {
	// Operator commands keep the database and KEK guards but not the HTTP
	// surface ones; the model tool needs neither.
	valid(t, "production")
	t.Setenv("SELA_PUBLIC_ORIGIN", "")
	if _, err := LoadMode(ModeOps); err != nil {
		t.Fatalf("ops mode needs the public origin: %v", err)
	}
	t.Setenv("SELA_DATABASE_URL", "postgres://u:p@db.example/sela?sslmode=require")
	if _, err := LoadMode(ModeOps); err == nil {
		t.Fatal("ops mode accepted a non-verify-full remote database")
	}
	clean(t)
	if _, err := LoadMode(ModeModel); err != nil {
		t.Fatalf("model mode needs a database: %v", err)
	}
	if !(Config{Env: "staging"}).RequiresDatabase() || (Config{Env: "development"}).RequiresDatabase() {
		t.Fatal("RequiresDatabase")
	}
}

// R1-3: in staging and production the TLS rule applies to the settings pgx
// will connect with, so a host or sslmode taken from PGHOST, PGSSLMODE or a
// libpq service file cannot bypass it.
func TestDatabaseTLSUsesEffectiveSettings(t *testing.T) {
	dir := t.TempDir()
	service := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	plain := service("plain.conf", "[sela]\nhost=db.example.org\nport=5432\nsslmode=disable\n")
	verified := service("verified.conf", "[sela]\nhost=db.example.org\nsslmode=verify-full\n")
	local := service("local.conf", "[sela]\nhost=127.0.0.1\nsslmode=disable\n")
	cases := []struct {
		name   string
		url    string
		env    map[string]string
		strict bool // refused in staging and production
	}{
		{"host from PGHOST, sslmode prefer", "postgres://sela:pw-secret@/sela", map[string]string{"PGHOST": "db.example.org"}, true},
		{"host from PGHOST, sslmode=disable from PGSSLMODE", "postgres://sela:pw-secret@/sela", map[string]string{"PGHOST": "db.example.org", "PGSSLMODE": "disable"}, true},
		{"host from PGHOST, verify-full from PGSSLMODE", "postgres://sela:pw-secret@/sela", map[string]string{"PGHOST": "db.example.org", "PGSSLMODE": "verify-full"}, false},
		{"service file without TLS", "postgres://sela:pw-secret@/sela?service=sela", map[string]string{"PGSERVICEFILE": plain}, true},
		{"service file with verify-full", "postgres://sela:pw-secret@/sela?service=sela", map[string]string{"PGSERVICEFILE": verified}, false},
		{"service file to loopback", "postgres://sela:pw-secret@/sela?service=sela", map[string]string{"PGSERVICEFILE": local}, false},
		{"PGSERVICE variable", "postgres://sela:pw-secret@/sela", map[string]string{"PGSERVICE": "sela", "PGSERVICEFILE": plain}, true},
		{"missing service file", "postgres://sela:pw-secret@/sela?service=sela", map[string]string{"PGSERVICEFILE": filepath.Join(dir, "none.conf")}, true},
		{"host parameter", "postgres://sela:pw-secret@/sela?host=db.example.org", nil, true},
		{"verify-ca", "postgres://sela:pw-secret@db.example.org/sela?sslmode=verify-ca", nil, true},
		{"allow", "postgres://sela:pw-secret@db.example.org/sela?sslmode=allow", nil, true},
		{"second host unverified", "postgres://sela:pw-secret@127.0.0.1,db.example.org/sela?sslmode=require", nil, true},
		{"verify-full with system roots", "postgres://sela:pw-secret@db.example.org/sela?sslmode=verify-full&sslrootcert=system", nil, false},
		{"URL verify-full wins over PGSSLMODE", "postgres://sela:pw-secret@db.example.org/sela?sslmode=verify-full", map[string]string{"PGSSLMODE": "disable"}, false},
		{"loopback with PGSSLMODE=disable", "postgres://sela:pw-secret@127.0.0.1/sela", map[string]string{"PGSSLMODE": "disable"}, false},
		{"unix socket from PGHOST", "postgres://sela:pw-secret@/sela", map[string]string{"PGHOST": "/var/run/postgresql"}, false},
	}
	for _, env := range []string{"production", "staging", "development"} {
		for _, c := range cases {
			t.Run(env+"/"+c.name, func(t *testing.T) {
				valid(t, env)
				t.Setenv("SELA_DATABASE_URL", c.url)
				for k, v := range c.env {
					t.Setenv(k, v)
				}
				_, err := Load()
				want := c.strict && env != "development"
				if (err != nil) != want {
					t.Fatalf("refused=%t, want %t (%v)", err != nil, want, err)
				}
				if err != nil && (strings.Contains(err.Error(), "pw-secret") || strings.Contains(err.Error(), "db.example.org")) {
					t.Fatalf("error leaks the connection settings: %v", err)
				}
			})
		}
	}
}

// PR #9 review, F2: every pool size from MinDBMaxConns on is accepted (one
// connection is enough: a background job runs on the connection that
// holds its lock, db.WithJobLock), and the error names the bounds.
func TestDBMaxConnsBounds(t *testing.T) {
	if MinDBMaxConns != 1 || MaxDBMaxConns != 200 {
		t.Fatalf("bounds [%d, %d]; update RUNBOOK §3 and this test together", MinDBMaxConns, MaxDBMaxConns)
	}
	for v, want := range map[string]int{"": 10, strconv.Itoa(MinDBMaxConns): MinDBMaxConns, "2": 2, strconv.Itoa(MaxDBMaxConns): MaxDBMaxConns} {
		valid(t, "production")
		t.Setenv("SELA_DB_MAX_CONNS", v)
		c, err := Load()
		if err != nil || c.DBMaxConns != want {
			t.Fatalf("SELA_DB_MAX_CONNS=%q: %d, %v; want %d", v, c.DBMaxConns, err, want)
		}
	}
	for _, v := range []string{strconv.Itoa(MinDBMaxConns - 1), "-1", strconv.Itoa(MaxDBMaxConns + 1), "ten"} {
		valid(t, "production")
		t.Setenv("SELA_DB_MAX_CONNS", v)
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "SELA_DB_MAX_CONNS: must be an integer in [1, 200]") {
			t.Fatalf("SELA_DB_MAX_CONNS=%q: %v, want the bounds error", v, err)
		}
	}
}
