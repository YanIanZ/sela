package config

import (
	"strings"
	"testing"
)

// clean unsets every SELA_ variable the loader reads.
func clean(t *testing.T) {
	t.Helper()
	for _, k := range []string{"SELA_ENV", "SELA_ADDR", "SELA_WEB_DIR", "SELA_PUBLIC_ORIGIN", "SELA_RESEARCH_KB", "SELA_LOG_LEVEL",
		"SELA_SECURE_COOKIES", "SELA_TRUST_PROXY", "SELA_MAX_SESSIONS", "SELA_LLM_MODE", "SELA_LLM_BASE_URL", "SELA_LLM_API_KEY",
		"SELA_MODAL_KEY", "SELA_MODAL_SECRET", "SELA_LLM_SERVED_MODEL", "SELA_LLM_MODEL_ID", "SELA_LLM_MODEL_REVISION",
		"SELA_LLM_CONCURRENCY", "SELA_LLM_OP_TIMEOUT", "SELA_LLM_WARM_WINDOW", "SELA_LLM_COLD_START_ESTIMATE",
		"SELA_MAX_AI_OPS_PER_SESSION", "SELA_ALLOW_TEST_DOUBLE", "SELA_METRICS_TOKEN"} {
		t.Setenv(k, "")
	}
}

func TestDefaultsAreProductionWithAIDisabled(t *testing.T) {
	clean(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Env != "production" || c.LLMMode != "disabled" || !c.SecureCookies || c.MaxSessions != 500 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestTestDoubleIsRefusedInProductionAndWithoutOptIn(t *testing.T) {
	clean(t)
	t.Setenv("SELA_LLM_MODE", "test_double")
	if _, err := Load(); err == nil {
		t.Fatal("test double accepted in production")
	}
	t.Setenv("SELA_ENV", "development")
	if _, err := Load(); err == nil {
		t.Fatal("test double accepted without SELA_ALLOW_TEST_DOUBLE")
	}
	t.Setenv("SELA_ALLOW_TEST_DOUBLE", "true")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ServedModel != "test-double-not-meta" || c.SecureCookies {
		t.Fatalf("test double defaults: %+v", c)
	}
}

func TestModalNeedsHTTPSAndEveryCredential(t *testing.T) {
	clean(t)
	t.Setenv("SELA_LLM_MODE", "modal")
	t.Setenv("SELA_LLM_BASE_URL", "http://example.invalid")
	t.Setenv("SELA_LLM_API_KEY", "k")
	t.Setenv("SELA_MODAL_KEY", "wk-1")
	t.Setenv("SELA_MODAL_SECRET", "ws-1")
	t.Setenv("SELA_LLM_SERVED_MODEL", "sela-glimmer-cec94fc")
	t.Setenv("SELA_LLM_MODEL_ID", "meta-models/Muse-Glimmer-30B")
	t.Setenv("SELA_LLM_MODEL_REVISION", "cec94fcfd6512f2cfe708138c32533b346ea4f69")
	if _, err := Load(); err == nil {
		t.Fatal("plain http Modal URL accepted")
	}
	t.Setenv("SELA_LLM_BASE_URL", "https://example.invalid")
	t.Setenv("SELA_MODAL_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("missing Modal secret accepted")
	}
	t.Setenv("SELA_MODAL_SECRET", "ws-secret-value")
	t.Setenv("SELA_LLM_API_KEY", "vllm-key-secret-value")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	s := c.String()
	for _, secret := range []string{"vllm-key-secret-value", "ws-secret-value", "wk-1"} {
		if strings.Contains(s, secret) {
			t.Fatalf("String() leaks a credential: %s", s)
		}
	}
}

func TestInvalidValuesAreRejected(t *testing.T) {
	for k, v := range map[string]string{
		"SELA_ENV":            "staging",
		"SELA_LLM_MODE":       "openai",
		"SELA_MAX_SESSIONS":   "0",
		"SELA_LLM_OP_TIMEOUT": "1s",
		"SELA_SECURE_COOKIES": "maybe",
	} {
		t.Run(k, func(t *testing.T) {
			clean(t)
			t.Setenv(k, v)
			if _, err := Load(); err == nil {
				t.Fatalf("%s=%s accepted", k, v)
			}
		})
	}
}

func TestMetricsTokenIsOptionalStrongAndNeverPrinted(t *testing.T) {
	clean(t)
	c, err := Load()
	if err != nil || c.MetricsToken != "" {
		t.Fatalf("metrics should be off by default: %q %v", c.MetricsToken, err)
	}
	for _, weak := range []string{"short", "has a space in the token value"} {
		t.Setenv("SELA_METRICS_TOKEN", weak)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "SELA_METRICS_TOKEN") {
			t.Fatalf("weak metrics token %q accepted: %v", weak, err)
		}
	}
	const token = "metrics-0123456789-abcdef"
	t.Setenv("SELA_METRICS_TOKEN", "  "+token+"\n")
	c, err = Load()
	if err != nil || c.MetricsToken != token {
		t.Fatalf("metrics token: %q %v", c.MetricsToken, err)
	}
	if s := c.String(); strings.Contains(s, token) || !strings.Contains(s, "metrics=true") {
		t.Fatalf("String(): %s", s)
	}
}
