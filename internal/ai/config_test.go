package ai

import (
	"strings"
	"testing"
	"time"
)

const (
	anthropicKey = "sk-ant-CONFIG-CANARY"
	deepseekKey  = "sk-ds-CONFIG-CANARY"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaultsFailClosed(t *testing.T) {
	c, err := LoadFromEnv(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Env != EnvProduction || c.Profile != ProfileFull || c.Fallback || c.AltVerifier || len(c.AllowedBands) != 1 || !c.AllowedBands[Band18Plus] ||
		c.Anthropic.Mode != ModeDisabled || c.DeepSeek.Mode != ModeDisabled || c.OpTimeout != 6*time.Minute {
		t.Fatalf("defaults %+v", c)
	}
	if c.VerifierEnabled() || c.DeepSeekEnabled() {
		t.Fatal("providers enabled by default")
	}
}

func TestLoadLiveProduction(t *testing.T) {
	c, err := LoadFromEnv(env(map[string]string{
		"SELA_ANTHROPIC_API_KEY": anthropicKey, "SELA_ANTHROPIC_WORKSPACE_ID": "wrkspc_1",
		"SELA_DEEPSEEK_API_KEY": deepseekKey, "SELA_ANTHROPIC_TIMEOUT": "20s", "SELA_DEEPSEEK_TIMEOUT": "45s",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Anthropic.Mode != ModeLive || c.DeepSeek.Mode != ModeLive || c.Anthropic.AttemptTimeout != 20*time.Second ||
		c.DeepSeek.AttemptTimeout != 45*time.Second || c.Fallback || c.AltVerifier {
		t.Fatalf("config %+v", c)
	}
	s := c.String()
	if strings.Contains(s, anthropicKey) || strings.Contains(s, deepseekKey) || strings.Contains(s, "wrkspc_1") {
		t.Fatalf("String leaks a secret: %s", s)
	}
	if !strings.Contains(s, "key=set") || !strings.Contains(s, "model=claude-sonnet-5-5") || !strings.Contains(s, "fallback=off") {
		t.Fatalf("String = %s", s)
	}
}

// TestProductionRefusals: production refuses test doubles, test-double
// base URLs (also for a disabled provider), non-official hosts, minors for
// AI providers, and the fallback or alternative verifier before their
// evaluations pass.
func TestProductionRefusals(t *testing.T) {
	cases := map[string]map[string]string{
		"verifier test double":          {"SELA_VERIFIER_MODE": "test_double", "SELA_ALLOW_TEST_DOUBLE": "true"},
		"deepseek test double":          {"SELA_DEEPSEEK_MODE": "test_double", "SELA_ALLOW_TEST_DOUBLE": "true"},
		"test-double anthropic url":     {"SELA_ANTHROPIC_API_KEY": anthropicKey, "SELA_ANTHROPIC_BASE_URL": "http://127.0.0.1:8090"},
		"test-double url https":         {"SELA_ANTHROPIC_API_KEY": anthropicKey, "SELA_ANTHROPIC_BASE_URL": "https://localhost:8443"},
		"test-double url disabled mode": {"SELA_DEEPSEEK_BASE_URL": "http://127.0.0.1:8090"},
		"fakellm host":                  {"SELA_DEEPSEEK_API_KEY": deepseekKey, "SELA_DEEPSEEK_BASE_URL": "https://fakellm.sela.example"},
		"non-official host":             {"SELA_ANTHROPIC_API_KEY": anthropicKey, "SELA_ANTHROPIC_BASE_URL": "https://anthropic-proxy.example-corp.com"},
		"fallback before EV-5":          {"SELA_AI_FALLBACK": "on", "SELA_DEEPSEEK_API_KEY": deepseekKey},
		"alt verifier before EV-6":      {"SELA_AI_ALT_VERIFIER": "deepseek", "SELA_DEEPSEEK_API_KEY": deepseekKey},
		"minor bands":                   {"SELA_ALLOWED_AGE_BANDS": "15_17,18_plus"},
	}
	for name, vars := range cases {
		vars["SELA_ENV"] = "production"
		_, err := LoadFromEnv(env(vars))
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		for _, secret := range []string{anthropicKey, deepseekKey} {
			if strings.Contains(err.Error(), secret) {
				t.Errorf("%s: error leaks a key", name)
			}
		}
	}
	// With the recorded evaluation results the switches may be turned on.
	c, err := LoadFromEnv(env(map[string]string{"SELA_ENV": "production", "SELA_AI_FALLBACK": "on", "SELA_AI_FALLBACK_EV5": "pass",
		"SELA_AI_ALT_VERIFIER": "on", "SELA_AI_ALT_VERIFIER_EV6": "pass", "SELA_DEEPSEEK_API_KEY": deepseekKey}))
	if err != nil || !c.Fallback || !c.AltVerifier {
		t.Fatalf("after EV-5/EV-6: %+v %v", c, err)
	}
}

func TestTestDoublesOutsideProduction(t *testing.T) {
	vars := map[string]string{"SELA_ENV": "development", "SELA_VERIFIER_MODE": "test_double", "SELA_DEEPSEEK_MODE": "test_double"}
	if _, err := LoadFromEnv(env(vars)); err == nil {
		t.Fatal("test double without SELA_ALLOW_TEST_DOUBLE accepted")
	}
	vars["SELA_ALLOW_TEST_DOUBLE"] = "true"
	vars["SELA_AI_FALLBACK"] = "deepseek"
	c, err := LoadFromEnv(env(vars))
	if err != nil {
		t.Fatal(err)
	}
	if c.Anthropic.Mode != ModeTestDouble || c.DeepSeek.Mode != ModeTestDouble || !c.Fallback || len(c.AllowedBands) != 3 {
		t.Fatalf("config %+v", c)
	}
	if a := c.anthropicConfig(); !a.TestDouble || a.Production {
		t.Fatalf("anthropic config %+v", a)
	}
	for _, e := range []string{EnvTest, EnvDemo, EnvStaging} {
		vars["SELA_ENV"] = e
		if _, err := LoadFromEnv(env(vars)); err != nil {
			t.Errorf("%s: %v", e, err)
		}
	}
}

func TestInvalidValues(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"env":         {"SELA_ENV": "prod"},
		"profile":     {"SELA_AI_PROFILE": "all"},
		"fallback":    {"SELA_AI_FALLBACK": "maybe"},
		"alt":         {"SELA_AI_ALT_VERIFIER": "claude"},
		"bands":       {"SELA_ALLOWED_AGE_BANDS": "under_15"},
		"timeout":     {"SELA_ANTHROPIC_TIMEOUT": "10m"},
		"ds timeout":  {"SELA_DEEPSEEK_TIMEOUT": "abc"},
		"op":          {"SELA_LLM_OP_TIMEOUT": "1s"},
		"op short":    {"SELA_LLM_OP_TIMEOUT": "2m"},
		"op long":     {"SELA_LLM_OP_TIMEOUT": "16m"},
		"verifier":    {"SELA_VERIFIER_MODE": "claude"},
		"deepseek":    {"SELA_DEEPSEEK_MODE": "on"},
		"allow":       {"SELA_ALLOW_TEST_DOUBLE": "sure"},
		"live no key": {"SELA_VERIFIER_MODE": "anthropic"},
		"version":     {"SELA_ANTHROPIC_API_KEY": anthropicKey, "SELA_ANTHROPIC_VERSION": "v1"},
		"geo":         {"SELA_ANTHROPIC_API_KEY": anthropicKey, "SELA_VERIFIER_INFERENCE_GEO": "eu"},
		"pin":         {"SELA_DEEPSEEK_PIN_VERSION": "yes please"},
	} {
		if _, err := LoadFromEnv(env(vars)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestMetaOnlyProfile(t *testing.T) {
	c, err := LoadFromEnv(env(map[string]string{"SELA_AI_PROFILE": "meta_only", "SELA_ANTHROPIC_API_KEY": anthropicKey,
		"SELA_DEEPSEEK_API_KEY": deepseekKey}))
	if err != nil {
		t.Fatal(err)
	}
	if c.VerifierEnabled() || c.DeepSeekEnabled() {
		t.Fatal("meta_only enables Claude or DeepSeek")
	}
}
