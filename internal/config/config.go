// Package config reads Sela's runtime configuration from environment
// variables. Secrets are only ever read here and passed to internal/llm; they
// are never logged or returned by String().
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

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

// Load reads and validates the environment.
func Load() (Config, error) {
	var c Config
	var errs []error
	add := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}
	var err error
	c.Env = env("SELA_ENV", "production")
	if c.Env != "production" && c.Env != "development" && c.Env != "test" {
		add(errors.New("SELA_ENV: must be production, development or test"))
	}
	c.Addr = env("SELA_ADDR", ":8080")
	c.WebDir = env("SELA_WEB_DIR", "web/dist")
	c.PublicOrigin = env("SELA_PUBLIC_ORIGIN", "")
	c.ResearchKB = env("SELA_RESEARCH_KB", "data/research/corpus/knowledge_base.jsonl")
	c.LogLevel = env("SELA_LOG_LEVEL", "info")
	c.SecureCookies, err = envBool("SELA_SECURE_COOKIES", c.Env == "production")
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

	switch c.LLMMode {
	case "disabled":
	case "modal":
		if !strings.HasPrefix(c.LLMBaseURL, "https://") || c.LLMAPIKey == "" || c.ModalKey == "" || c.ModalSecret == "" ||
			c.ServedModel == "" || c.ModelID == "" || c.ModelRevision == "" {
			add(errors.New("SELA_LLM_MODE=modal requires https SELA_LLM_BASE_URL, SELA_LLM_API_KEY, SELA_MODAL_KEY, SELA_MODAL_SECRET, SELA_LLM_SERVED_MODEL, SELA_LLM_MODEL_ID and SELA_LLM_MODEL_REVISION"))
		}
	case "test_double":
		allow, _ := envBool("SELA_ALLOW_TEST_DOUBLE", false)
		if !allow || c.Env == "production" {
			add(errors.New("SELA_LLM_MODE=test_double requires SELA_ALLOW_TEST_DOUBLE=true and SELA_ENV other than production"))
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
	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return c, nil
}

// String describes the configuration without secrets.
func (c Config) String() string {
	return fmt.Sprintf("env=%s addr=%s web=%s llm=%s model=%s@%s secure_cookies=%t max_sessions=%d metrics=%t",
		c.Env, c.Addr, c.WebDir, c.LLMMode, c.ModelID, c.ModelRevision, c.SecureCookies, c.MaxSessions, c.MetricsToken != "")
}
