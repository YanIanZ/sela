package ai

import (
	"strings"
	"testing"
	"time"
)

const (
	metaKey     = "sk-meta-CONFIG-CANARY"
	metaTestKey = "sk-meta-test-CONFIG-CANARY"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaultsFailClosed(t *testing.T) {
	c, err := LoadFromEnv(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Env != EnvProduction || c.Profile != ProfileFull || len(c.AllowedBands) != 1 || !c.AllowedBands[Band18Plus] ||
		c.MetaVerifier.Mode != ModeDisabled || c.OpTimeout != 6*time.Minute {
		t.Fatalf("defaults %+v", c)
	}
	if c.VerifierEnabled() {
		t.Fatal("verifier enabled by default")
	}
}

func TestLoadLiveProduction(t *testing.T) {
	c, err := LoadFromEnv(env(map[string]string{
		"SELA_META_VERIFIER_API_KEY": metaKey, "SELA_META_VERIFIER_TIMEOUT": "20s",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.MetaVerifier.Mode != ModeLive || c.MetaVerifier.AttemptTimeout != 20*time.Second {
		t.Fatalf("config %+v", c)
	}
	s := c.String()
	if strings.Contains(s, metaKey) {
		t.Fatalf("String leaks a secret: %s", s)
	}
	if !strings.Contains(s, "key=set") || !strings.Contains(s, "model=muse-wonder-1.0") {
		t.Fatalf("String = %s", s)
	}
}

// TestProductionRefusals: production refuses test doubles, test-double
// base URLs (also for a disabled provider), non-official hosts and minors
// for AI providers.
func TestProductionRefusals(t *testing.T) {
	cases := map[string]map[string]string{
		"verifier test double":     {"SELA_VERIFIER_MODE": "test_double", "SELA_ALLOW_TEST_DOUBLE": "true"},
		"test-double meta url":     {"SELA_META_VERIFIER_API_KEY": metaKey, "SELA_META_VERIFIER_BASE_URL": "http://127.0.0.1:8090"},
		"test-double url https":    {"SELA_META_VERIFIER_API_KEY": metaKey, "SELA_META_VERIFIER_BASE_URL": "https://localhost:8443"},
		"test-double url disabled": {"SELA_META_VERIFIER_BASE_URL": "http://127.0.0.1:8090"},
		"fakellm host":             {"SELA_META_VERIFIER_API_KEY": metaKey, "SELA_META_VERIFIER_BASE_URL": "https://fakellm.sela.example"},
		"non-official host":        {"SELA_META_VERIFIER_API_KEY": metaKey, "SELA_META_VERIFIER_BASE_URL": "https://meta-proxy.example-corp.com"},
		"minor bands":              {"SELA_ALLOWED_AGE_BANDS": "15_17,18_plus"},
	}
	for name, vars := range cases {
		vars["SELA_ENV"] = "production"
		_, err := LoadFromEnv(env(vars))
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if strings.Contains(err.Error(), metaKey) {
			t.Errorf("%s: error leaks a key", name)
		}
	}
}

func TestTestDoublesOutsideProduction(t *testing.T) {
	vars := map[string]string{"SELA_ENV": "development", "SELA_VERIFIER_MODE": "test_double"}
	if _, err := LoadFromEnv(env(vars)); err == nil {
		t.Fatal("test double without SELA_ALLOW_TEST_DOUBLE accepted")
	}
	vars["SELA_ALLOW_TEST_DOUBLE"] = "true"
	c, err := LoadFromEnv(env(vars))
	if err != nil {
		t.Fatal(err)
	}
	if c.MetaVerifier.Mode != ModeTestDouble || len(c.AllowedBands) != 3 {
		t.Fatalf("config %+v", c)
	}
	if v := c.metaVerifierConfig(); !v.TestDouble || v.Production {
		t.Fatalf("verifier config %+v", v)
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
		"env":          {"SELA_ENV": "prod"},
		"profile":      {"SELA_AI_PROFILE": "all"},
		"bands":        {"SELA_ALLOWED_AGE_BANDS": "under_15"},
		"explicit 15":  {"SELA_ALLOWED_AGE_BANDS": "under_16"},
		"timeout":      {"SELA_META_VERIFIER_TIMEOUT": "10m"},
		"op":           {"SELA_LLM_OP_TIMEOUT": "1s"},
		"op long":      {"SELA_LLM_OP_TIMEOUT": "16m"},
		"verifier":     {"SELA_VERIFIER_MODE": "claude"},
		"allow":        {"SELA_ALLOW_TEST_DOUBLE": "sure"},
		"live no key":  {"SELA_VERIFIER_MODE": "meta_api"},
		"verifier key": {"SELA_META_VERIFIER_API_KEY": "line\nbreak"},
	} {
		if _, err := LoadFromEnv(env(vars)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestMetaOnlyProfileKeepsTheMetaVerifier(t *testing.T) {
	c, err := LoadFromEnv(env(map[string]string{"SELA_AI_PROFILE": "meta_only", "SELA_META_VERIFIER_API_KEY": metaKey}))
	if err != nil {
		t.Fatal(err)
	}
	if !c.VerifierEnabled() {
		t.Fatal("meta_only must keep the Meta verifier (both slots are Meta)")
	}
}
