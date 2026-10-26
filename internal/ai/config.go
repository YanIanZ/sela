package ai

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/ai/anthropic"
	"github.com/YanIanZ/sela/internal/ai/deepseek"
	"github.com/YanIanZ/sela/internal/ai/transport"
)

// Environments (ADR-0002 §12, D-045).
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvDemo        = "demo"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// Profile selects the provider set (RR-15).
type Profile string

// Profiles.
const (
	// ProfileFull uses Meta, Claude and, when enabled, DeepSeek.
	ProfileFull Profile = "full"
	// ProfileMetaOnly is the Meta-only hackathon path (D-049): no Claude,
	// no DeepSeek; verifier-gated tasks abstain.
	ProfileMetaOnly Profile = "meta_only"
)

// ProviderMode selects what is behind a provider.
type ProviderMode string

// Provider modes.
const (
	ModeLive       ProviderMode = "live"
	ModeTestDouble ProviderMode = "test_double"
	ModeDisabled   ProviderMode = "disabled"
)

// on reports whether the mode puts a provider behind the route: live or
// a labelled test double. An empty or unknown mode is off (fail closed).
func (m ProviderMode) on() bool { return m == ModeLive || m == ModeTestDouble }

// AnthropicConfig configures the Claude verifier. The key and workspace
// are Secrets: printing or encoding the config never shows them.
type AnthropicConfig struct {
	Mode           ProviderMode
	APIKey         transport.Secret
	BaseURL        string
	WorkspaceID    transport.Secret
	Version        string
	Model          string
	InferenceGeo   string
	AttemptTimeout time.Duration
}

// DeepSeekConfig configures DeepSeek. The key is a Secret.
type DeepSeekConfig struct {
	Mode             ProviderMode
	APIKey           transport.Secret
	BaseURL          string
	Model            string
	ExpectedVersion  string
	PinServedVersion bool
	AttemptTimeout   time.Duration
}

// Config is the AI layer's configuration. Read it with LoadFromEnv. It
// never prints secrets: the keys are transport.Secret values, which every
// fmt verb and JSON show as "[redacted]" (see also String).
type Config struct {
	// Env is one of the five environments; any other value is treated as
	// production by Production and refused by Validate and NewRouter.
	Env             string
	AllowTestDouble bool
	// TestDoubleAPIKey is the only key sent to a test-double verifier or
	// DeepSeek double (SELA_TEST_DOUBLE_API_KEY, optional). Live keys are
	// never sent to a test double.
	TestDoubleAPIKey transport.Secret
	Profile          Profile
	// Fallback enables DeepSeek as fallback generator (SELA_AI_FALLBACK,
	// default off; RR-4 e).
	Fallback bool
	// AltVerifier enables DeepSeek as alternative verifier
	// (SELA_AI_ALT_VERIFIER, default off; RR-9 e).
	AltVerifier bool
	// AllowedBands are the age bands that may reach an AI provider
	// (SELA_ALLOWED_AGE_BANDS), checked per band (ADR-0003 §8.3: "only
	// where the band is enabled"). A nil set allows 18_plus only. 15_17
	// and unknown may use Meta and Claude where enabled, never DeepSeek
	// and never in production (B-10).
	AllowedBands map[AgeBand]bool
	// OpTimeout is the AT-3/AT-4 operation budget (SELA_LLM_OP_TIMEOUT),
	// at least MinOpTimeout.
	OpTimeout time.Duration
	Anthropic AnthropicConfig
	DeepSeek  DeepSeekConfig
}

// Production reports whether c is the production environment. It fails
// closed: any value other than development, test, demo or staging (an
// empty Env, "prod", "Production") is production.
func (c Config) Production() bool {
	switch c.Env {
	case EnvDevelopment, EnvTest, EnvDemo, EnvStaging:
		return false
	}
	return true
}

// VerifierEnabled reports whether a Claude route is configured. The mode
// is authoritative: SELA_VERIFIER_MODE=disabled (or an empty mode) turns
// the route off even for a client passed in Deps (RR-15).
func (c Config) VerifierEnabled() bool {
	return (c.Profile == ProfileFull || c.Profile == "") && c.Anthropic.Mode.on()
}

// DeepSeekEnabled reports whether DeepSeek is configured, as fallback
// generator or alternative verifier. SELA_DEEPSEEK_MODE=disabled turns it
// off even for clients passed in Deps.
func (c Config) DeepSeekEnabled() bool {
	return (c.Profile == ProfileFull || c.Profile == "") && c.DeepSeek.Mode.on()
}

// bandAllowed reports whether band may reach any AI provider.
func (c Config) bandAllowed(b AgeBand) bool {
	if c.AllowedBands == nil {
		return b == Band18Plus
	}
	return c.AllowedBands[b]
}

// Bands returns a band set.
func Bands(bands ...AgeBand) map[AgeBand]bool {
	m := make(map[AgeBand]bool, len(bands))
	for _, b := range bands {
		m[b] = true
	}
	return m
}

// Timeouts.
const (
	DefaultOpTimeout = 6 * time.Minute
	MaxOpTimeout     = 15 * time.Minute
)

// MinOpTimeout is the smallest SELA_LLM_OP_TIMEOUT: the AT-3/AT-4 fallback
// and verification reserves, the minimum Meta window and the reserve
// margin. A shorter budget could not wait for Meta at all.
func MinOpTimeout() time.Duration {
	var floor time.Duration
	for _, p := range policies {
		if p.Generative && p.Budget == 0 {
			floor = max(floor, p.reserves()+MinMetaWindow+reserveMargin)
		}
	}
	return floor
}

// LoadFromEnv reads the AI configuration. getenv may be nil (os.Getenv).
// It fails closed: an unknown value is an error, production defaults to
// no test doubles, no fallback and no alternative verifier, and errors
// name variables, never their values.
//
// Variables:
//
//	SELA_ENV                       development | test | demo | staging | production (default production)
//	SELA_ALLOW_TEST_DOUBLE         true to allow test doubles outside production
//	SELA_AI_PROFILE                full (default) | meta_only
//	SELA_AI_FALLBACK               on | off (default off; "deepseek" means on)
//	SELA_AI_ALT_VERIFIER           on | off (default off; "deepseek" means on)
//	SELA_AI_FALLBACK_EV5           pass: required to turn the fallback on in production
//	SELA_AI_ALT_VERIFIER_EV6       pass: required to turn the alternative verifier on in production
//	SELA_ALLOWED_AGE_BANDS         comma list of 15_17, 18_plus, unknown (production: 18_plus only)
//	SELA_LLM_OP_TIMEOUT            AT-3/AT-4 budget (default 6m, MinOpTimeout..15m)
//	SELA_TEST_DOUBLE_API_KEY       optional key sent to test doubles only (never a live key)
//	SELA_VERIFIER_MODE             anthropic | test_double | disabled (default anthropic with a key, else disabled)
//	SELA_ANTHROPIC_API_KEY         workspace-scoped server key
//	SELA_ANTHROPIC_BASE_URL        default https://api.anthropic.com
//	SELA_ANTHROPIC_WORKSPACE_ID    optional
//	SELA_ANTHROPIC_VERSION         default 2026-10-21
//	SELA_VERIFIER_MODEL            default claude-sonnet-5-5
//	SELA_VERIFIER_INFERENCE_GEO    us (default) | global (production: us only; Claude gets
//	                               personal data only with us, the consent's location)
//	SELA_ANTHROPIC_TIMEOUT         per attempt (default 30s, 1s..2m)
//	SELA_DEEPSEEK_MODE             deepseek | test_double | disabled (default deepseek with a key, else disabled)
//	SELA_DEEPSEEK_API_KEY
//	SELA_DEEPSEEK_BASE_URL         default https://api.deepseek.com
//	SELA_DEEPSEEK_MODEL            default deepseek-flash
//	SELA_DEEPSEEK_EXPECTED_VERSION default DeepSeek-V4.1-Flash-1017
//	SELA_DEEPSEEK_PIN_VERSION      on | off (default off until the response field is calibrated, B-9)
//	SELA_DEEPSEEK_TIMEOUT          per attempt (default 60s, 1s..5m)
func LoadFromEnv(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	get := func(k string) string { return strings.TrimSpace(getenv(k)) }
	var errs []error
	add := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}
	var c Config
	c.Env = get("SELA_ENV")
	if c.Env == "" {
		c.Env = EnvProduction
	}
	switch c.Env {
	case EnvDevelopment, EnvTest, EnvDemo, EnvStaging, EnvProduction:
	default:
		add(errors.New("SELA_ENV: must be development, test, demo, staging or production"))
	}
	var err error
	c.AllowTestDouble, err = boolVar(get, "SELA_ALLOW_TEST_DOUBLE")
	add(err)
	switch p := get("SELA_AI_PROFILE"); p {
	case "", string(ProfileFull):
		c.Profile = ProfileFull
	case string(ProfileMetaOnly):
		c.Profile = ProfileMetaOnly
	default:
		add(errors.New("SELA_AI_PROFILE: must be full or meta_only"))
	}
	c.Fallback, err = switchVar(get, "SELA_AI_FALLBACK", "deepseek")
	add(err)
	c.AltVerifier, err = switchVar(get, "SELA_AI_ALT_VERIFIER", "deepseek")
	add(err)
	bands := get("SELA_ALLOWED_AGE_BANDS")
	if bands == "" {
		bands = "15_17,18_plus,unknown"
		if c.Production() {
			bands = "18_plus"
		}
	}
	c.AllowedBands = map[AgeBand]bool{}
	for _, b := range strings.Split(bands, ",") {
		band := AgeBand(strings.TrimSpace(b))
		if !band.Valid() {
			add(errors.New("SELA_ALLOWED_AGE_BANDS: must list 15_17, 18_plus or unknown"))
			continue
		}
		c.AllowedBands[band] = true
	}
	c.OpTimeout, err = durationVar(get, "SELA_LLM_OP_TIMEOUT", DefaultOpTimeout, MinOpTimeout(), MaxOpTimeout)
	add(err)
	c.TestDoubleAPIKey = transport.NewSecret(getenv("SELA_TEST_DOUBLE_API_KEY"))

	a := &c.Anthropic
	a.APIKey = transport.NewSecret(getenv("SELA_ANTHROPIC_API_KEY"))
	a.BaseURL = get("SELA_ANTHROPIC_BASE_URL")
	a.WorkspaceID = transport.NewSecret(get("SELA_ANTHROPIC_WORKSPACE_ID"))
	a.Version = get("SELA_ANTHROPIC_VERSION")
	a.Model = get("SELA_VERIFIER_MODEL")
	a.InferenceGeo = get("SELA_VERIFIER_INFERENCE_GEO")
	a.AttemptTimeout, err = durationVar(get, "SELA_ANTHROPIC_TIMEOUT", anthropic.DefaultAttemptTimeout, time.Second, 2*time.Minute)
	add(err)
	switch m := get("SELA_VERIFIER_MODE"); m {
	case "":
		a.Mode = ModeDisabled
		if a.APIKey.IsSet() {
			a.Mode = ModeLive
		}
	case "anthropic", string(ModeLive):
		a.Mode = ModeLive
	case string(ModeTestDouble):
		a.Mode = ModeTestDouble
	case string(ModeDisabled):
		a.Mode = ModeDisabled
	default:
		add(errors.New("SELA_VERIFIER_MODE: must be anthropic, test_double or disabled"))
	}

	d := &c.DeepSeek
	d.APIKey = transport.NewSecret(getenv("SELA_DEEPSEEK_API_KEY"))
	d.BaseURL = get("SELA_DEEPSEEK_BASE_URL")
	d.Model = get("SELA_DEEPSEEK_MODEL")
	d.ExpectedVersion = get("SELA_DEEPSEEK_EXPECTED_VERSION")
	d.PinServedVersion, err = switchVar(get, "SELA_DEEPSEEK_PIN_VERSION", "")
	add(err)
	d.AttemptTimeout, err = durationVar(get, "SELA_DEEPSEEK_TIMEOUT", deepseek.DefaultAttemptTimeout, time.Second, 5*time.Minute)
	add(err)
	switch m := get("SELA_DEEPSEEK_MODE"); m {
	case "":
		d.Mode = ModeDisabled
		if d.APIKey.IsSet() {
			d.Mode = ModeLive
		}
	case "deepseek", string(ModeLive):
		d.Mode = ModeLive
	case string(ModeTestDouble):
		d.Mode = ModeTestDouble
	case string(ModeDisabled):
		d.Mode = ModeDisabled
	default:
		add(errors.New("SELA_DEEPSEEK_MODE: must be deepseek, test_double or disabled"))
	}

	if c.Production() {
		if c.Fallback && get("SELA_AI_FALLBACK_EV5") != "pass" {
			add(errors.New("SELA_AI_FALLBACK: stays off in production until EV-5 passes (set SELA_AI_FALLBACK_EV5=pass with the recorded result)"))
		}
		if c.AltVerifier && get("SELA_AI_ALT_VERIFIER_EV6") != "pass" {
			add(errors.New("SELA_AI_ALT_VERIFIER: stays off in production until EV-6 passes (set SELA_AI_ALT_VERIFIER_EV6=pass with the recorded result)"))
		}
	}
	if len(errs) == 0 {
		add(c.Validate())
	}
	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return c, nil
}

// validateCore checks what NewRouter relies on, without building
// clients: a known environment and profile, known modes, an operation
// budget that can hold Meta's window and the reserves, known age bands
// (production: 18_plus only), test doubles only where allowed, and in
// production Claude processing in the US only.
func (c Config) validateCore() error {
	var errs []error
	switch c.Env {
	case EnvDevelopment, EnvTest, EnvDemo, EnvStaging, EnvProduction:
	default:
		errs = append(errs, errors.New("SELA_ENV: must be development, test, demo, staging or production"))
	}
	switch c.Profile {
	case "", ProfileFull, ProfileMetaOnly:
	default:
		errs = append(errs, errors.New("SELA_AI_PROFILE: must be full or meta_only"))
	}
	for _, m := range []struct {
		name string
		mode ProviderMode
	}{{"SELA_VERIFIER_MODE", c.Anthropic.Mode}, {"SELA_DEEPSEEK_MODE", c.DeepSeek.Mode}} {
		switch m.mode {
		case "", ModeLive, ModeTestDouble, ModeDisabled:
		default:
			errs = append(errs, fmt.Errorf("%s: unknown mode", m.name))
		}
		if m.mode == ModeTestDouble && (c.Production() || !c.AllowTestDouble) {
			errs = append(errs, fmt.Errorf("%s=test_double requires SELA_ALLOW_TEST_DOUBLE=true and SELA_ENV other than production", m.name))
		}
	}
	if c.OpTimeout != 0 && (c.OpTimeout < MinOpTimeout() || c.OpTimeout > MaxOpTimeout) {
		errs = append(errs, fmt.Errorf("SELA_LLM_OP_TIMEOUT: must be within [%s, %s]: the AT-3/AT-4 fallback and verification reserves plus a minimum Meta window",
			MinOpTimeout(), MaxOpTimeout))
	}
	for b, ok := range c.AllowedBands {
		if !b.Valid() {
			errs = append(errs, errors.New("SELA_ALLOWED_AGE_BANDS: must list 15_17, 18_plus or unknown"))
		} else if ok && b != Band18Plus && c.Production() {
			// Production allows 18_plus only until the legal review (B-10):
			// minors get the honest no-AI path from this layer.
			errs = append(errs, errors.New("SELA_ALLOWED_AGE_BANDS: production allows only 18_plus for AI providers until the legal review (B-10)"))
		}
	}
	if c.Production() && c.Anthropic.InferenceGeo != "" && c.Anthropic.InferenceGeo != anthropic.DefaultInferenceGeo {
		// ai_provider_anthropic is "Anthropic, US (inference_geo: us)"; a
		// region change is a new policy version (ADR-0003 §8, §8.1).
		errs = append(errs, errors.New("SELA_VERIFIER_INFERENCE_GEO: production allows only us, the location the consent names"))
	}
	return errors.Join(errs...)
}

// Validate checks the whole configuration: validateCore, then the
// provider settings. It builds the clients' configs and lets them
// validate URLs, keys and models, so the rules live in one place;
// production refuses test doubles and any test-double base URL.
func (c Config) Validate() error {
	errs := []error{c.validateCore()}
	if c.Production() {
		for _, u := range []struct{ name, url, host string }{
			{"SELA_ANTHROPIC_BASE_URL", c.Anthropic.BaseURL, anthropic.LiveHost},
			{"SELA_DEEPSEEK_BASE_URL", c.DeepSeek.BaseURL, deepseek.LiveHost},
		} {
			// Checked even for a disabled provider: production refuses a
			// test-double base URL wherever it is configured.
			if u.url == "" {
				continue
			}
			if _, err := transport.ParseBaseURL(u.url, transport.BaseURLOptions{Production: true, LiveHosts: []string{u.host}}); err != nil {
				errs = append(errs, fmt.Errorf("%s: %v", u.name, err))
			}
		}
	}
	if c.Anthropic.Mode.on() {
		if _, err := anthropic.New(c.anthropicConfig()); err != nil {
			errs = append(errs, fmt.Errorf("SELA_ANTHROPIC_*: %v", err))
		}
	}
	if c.DeepSeek.Mode.on() {
		if _, err := deepseek.New(c.deepseekConfig()); err != nil {
			errs = append(errs, fmt.Errorf("SELA_DEEPSEEK_*: %v", err))
		}
	}
	return errors.Join(errs...)
}

func (c Config) anthropicConfig() anthropic.Config {
	return anthropic.Config{
		APIKey: c.Anthropic.APIKey, TestDoubleAPIKey: c.TestDoubleAPIKey, BaseURL: c.Anthropic.BaseURL, WorkspaceID: c.Anthropic.WorkspaceID,
		Version: c.Anthropic.Version, Model: c.Anthropic.Model, InferenceGeo: c.Anthropic.InferenceGeo,
		TestDouble: c.Anthropic.Mode == ModeTestDouble, Production: c.Production(), AttemptTimeout: c.Anthropic.AttemptTimeout,
	}
}

func (c Config) deepseekConfig() deepseek.Config {
	return deepseek.Config{
		APIKey: c.DeepSeek.APIKey, TestDoubleAPIKey: c.TestDoubleAPIKey, BaseURL: c.DeepSeek.BaseURL, Model: c.DeepSeek.Model,
		ExpectedVersion: c.DeepSeek.ExpectedVersion, PinServedVersion: c.DeepSeek.PinServedVersion,
		TestDouble: c.DeepSeek.Mode == ModeTestDouble, Production: c.Production(), AttemptTimeout: c.DeepSeek.AttemptTimeout,
	}
}

// String describes the configuration without secrets.
func (c Config) String() string {
	set := func(v transport.Secret) string {
		if !v.IsSet() {
			return "unset"
		}
		return "set"
	}
	onOff := func(b bool) string {
		if b {
			return "on"
		}
		return "off"
	}
	claudeModel, dsModel := anthropic.DefaultModel, deepseek.DefaultModel
	if c.Anthropic.Mode == ModeTestDouble {
		claudeModel = anthropic.TestDoubleModel
	}
	if c.DeepSeek.Mode == ModeTestDouble {
		dsModel = deepseek.TestDoubleModel
	}
	var bands []string
	for b, ok := range c.AllowedBands {
		if ok {
			bands = append(bands, string(b))
		}
	}
	sort.Strings(bands)
	if c.AllowedBands == nil {
		bands = []string{string(Band18Plus)}
	}
	return fmt.Sprintf("env=%s profile=%s verifier=%s(key=%s workspace=%s model=%s version=%s geo=%s) deepseek=%s(key=%s model=%s) fallback=%s alt_verifier=%s bands=%s",
		c.Env, c.Profile, c.Anthropic.Mode, set(c.Anthropic.APIKey), set(c.Anthropic.WorkspaceID), orDefault(c.Anthropic.Model, claudeModel),
		orDefault(c.Anthropic.Version, anthropic.DefaultVersion), orDefault(c.Anthropic.InferenceGeo, anthropic.DefaultInferenceGeo),
		c.DeepSeek.Mode, set(c.DeepSeek.APIKey), orDefault(c.DeepSeek.Model, dsModel),
		onOff(c.Fallback), onOff(c.AltVerifier), strings.Join(bands, ","))
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func boolVar(get func(string) string, key string) (bool, error) {
	v := get(key)
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: invalid boolean", key)
	}
	return b, nil
}

// switchVar reads on/off (also true/false, 1/0). alias, when set, is
// another spelling of "on" (the ADR writes SELA_AI_FALLBACK=deepseek).
func switchVar(get func(string) string, key, alias string) (bool, error) {
	switch v := strings.ToLower(get(key)); v {
	case "", "off", "false", "0", "no":
		return false, nil
	case "on", "true", "1", "yes":
		return true, nil
	default:
		if alias != "" && v == alias {
			return true, nil
		}
		return false, fmt.Errorf("%s: must be on or off", key)
	}
}

func durationVar(get func(string) string, key string, def, lo, hi time.Duration) (time.Duration, error) {
	v := get(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < lo || d > hi {
		return 0, fmt.Errorf("%s: must be a duration in [%s, %s]", key, lo, hi)
	}
	return d, nil
}
