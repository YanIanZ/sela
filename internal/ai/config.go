package ai

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/ai/metaverifier"
	"github.com/YanIanZ/sela/internal/ai/transport"
)

// Environments (ADR-0002 §12, D-045).
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// Profile selects the provider set (RR-15).
type Profile string

// Profiles.
const (
	// ProfileFull uses the configured Meta generator and the Meta verifier.
	ProfileFull Profile = "full"
	// ProfileMetaOnly keeps the Meta-only hackathon path (D-049). Both
	// provider slots are Meta, so the verifier still runs; verifier-gated
	// tasks abstain only when the verifier is disabled.
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

// MetaVerifierConfig configures the Meta verifier. It is always a
// different Meta model from the generator, so a model never verifies its
// own output (SPEC V36). The key is a Secret.
type MetaVerifierConfig struct {
	Mode           ProviderMode
	APIKey         transport.Secret
	BaseURL        string
	Model          string
	AttemptTimeout time.Duration
}

// Config is the AI layer's configuration. Read it with LoadFromEnv. It
// never prints secrets: the keys are transport.Secret values, which every
// fmt verb and JSON show as "[redacted]" (see also String).
type Config struct {
	// Env is one of the five environments; any other value is treated as
	// production by Production and refused by Validate and NewRouter.
	Env             string
	AllowTestDouble bool
	// TestDoubleAPIKey is the only key sent to a test-double verifier
	// (SELA_TEST_DOUBLE_API_KEY, optional). Live keys are never sent to a
	// test double.
	TestDoubleAPIKey transport.Secret
	Profile          Profile
	// AllowedBands are the age bands that may reach an AI provider
	// (SELA_ALLOWED_AGE_BANDS), checked per band (ADR-0003 §8.3: "only
	// where the band is enabled"). A nil set allows 18_plus only. 15_17
	// and unknown may use Meta where enabled, and never in production
	// (B-10).
	AllowedBands map[AgeBand]bool
	// OpTimeout is the AT-3/AT-4 operation budget (SELA_LLM_OP_TIMEOUT),
	// at least MinOpTimeout.
	OpTimeout time.Duration
	// VerifierDailyUSD is Sela's daily spend ceiling for the Meta verifier
	// (SELA_VERIFIER_DAILY_USD, ADR-0003 §11), estimated from tokens per
	// UTC day by this router; zero sets no ceiling here (a Limiter may
	// still enforce one). Reaching it is an availability failure of the
	// verifier route (spend_cap), and a verifier-gated output abstains.
	VerifierDailyUSD float64
	MetaVerifier     MetaVerifierConfig
}

// Production reports whether c is the production environment. It fails
// closed: any value other than development, test or staging (an
// empty Env, "prod", "Production") is production.
func (c Config) Production() bool {
	switch c.Env {
	case EnvDevelopment, EnvTest, EnvStaging:
		return false
	}
	return true
}

// VerifierEnabled reports whether a Meta verifier route is configured. The
// mode is authoritative: SELA_VERIFIER_MODE=disabled (or an empty mode)
// turns the route off even for a client passed in Deps (RR-15).
func (c Config) VerifierEnabled() bool {
	return c.MetaVerifier.Mode.on()
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

// MaxVerifierDailyUSD bounds SELA_VERIFIER_DAILY_USD (a typo guard).
const MaxVerifierDailyUSD = 10000

// Timeouts.
const (
	DefaultOpTimeout = 6 * time.Minute
	MaxOpTimeout     = 15 * time.Minute
)

// MinOpTimeout is the smallest SELA_LLM_OP_TIMEOUT: the verification
// reserve, the minimum Meta window and the reserve margin. A shorter budget
// could not wait for Meta at all.
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
// no test doubles, and errors name variables, never their values.
//
// Variables:
//
//	SELA_ENV                       development | test | staging | production (default production)
//	SELA_ALLOW_TEST_DOUBLE         true to allow test doubles outside production
//	SELA_AI_PROFILE                full (default) | meta_only
//	SELA_ALLOWED_AGE_BANDS         comma list of 15_17, 18_plus, unknown (production: 18_plus only)
//	SELA_LLM_OP_TIMEOUT            AT-3/AT-4 budget (default 6m, MinOpTimeout..15m)
//	SELA_VERIFIER_DAILY_USD        Meta verifier daily spend ceiling in USD (optional, e.g. 5 or 2.50)
//	SELA_TEST_DOUBLE_API_KEY       optional key sent to test doubles only (never a live key)
//	SELA_VERIFIER_MODE             meta_api | test_double | disabled (default meta_api with a key, else disabled)
//	SELA_META_VERIFIER_API_KEY     Meta API key (falls back to SELA_LLM_API_KEY)
//	SELA_META_VERIFIER_BASE_URL    default https://api.meta.ai/v1 (falls back to SELA_LLM_BASE_URL)
//	SELA_VERIFIER_MODEL            default muse-wonder-1.0 (must differ from the generator's model)
//	SELA_META_VERIFIER_TIMEOUT     per attempt (default 30s, 1s..2m)
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
	case EnvDevelopment, EnvTest, EnvStaging, EnvProduction:
	default:
		add(errors.New("SELA_ENV: must be development, test, staging or production"))
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
	if v := get("SELA_VERIFIER_DAILY_USD"); v != "" {
		usd, perr := strconv.ParseFloat(v, 64)
		if perr != nil || !(usd > 0 && usd <= MaxVerifierDailyUSD) {
			add(fmt.Errorf("SELA_VERIFIER_DAILY_USD: must be a positive amount up to %g", float64(MaxVerifierDailyUSD)))
		} else {
			c.VerifierDailyUSD = usd
		}
	}
	c.TestDoubleAPIKey = transport.NewSecret(getenv("SELA_TEST_DOUBLE_API_KEY"))

	m := &c.MetaVerifier
	m.APIKey = transport.NewSecret(getenv("SELA_META_VERIFIER_API_KEY"))
	if !m.APIKey.IsSet() {
		// One Meta API key may serve generator and verifier; the verifier
		// reads its own variable first so the two can be split later.
		m.APIKey = transport.NewSecret(getenv("SELA_LLM_API_KEY"))
	}
	m.BaseURL = get("SELA_META_VERIFIER_BASE_URL")
	if m.BaseURL == "" {
		m.BaseURL = get("SELA_LLM_BASE_URL")
	}
	m.Model = get("SELA_VERIFIER_MODEL")
	m.AttemptTimeout, err = durationVar(get, "SELA_META_VERIFIER_TIMEOUT", metaverifier.DefaultAttemptTimeout, time.Second, 2*time.Minute)
	add(err)
	switch mode := get("SELA_VERIFIER_MODE"); mode {
	case "":
		m.Mode = ModeDisabled
		if m.APIKey.IsSet() {
			m.Mode = ModeLive
		}
	case "meta_api", "meta_verifier", string(ModeLive):
		m.Mode = ModeLive
	case string(ModeTestDouble):
		m.Mode = ModeTestDouble
	case string(ModeDisabled):
		m.Mode = ModeDisabled
	default:
		add(errors.New("SELA_VERIFIER_MODE: must be meta_api, test_double or disabled"))
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
// budget that can hold Meta's window and the verification reserve, known
// age bands (production: 18_plus only), and test doubles only where
// allowed.
func (c Config) validateCore() error {
	var errs []error
	switch c.Env {
	case EnvDevelopment, EnvTest, EnvStaging, EnvProduction:
	default:
		errs = append(errs, errors.New("SELA_ENV: must be development, test, staging or production"))
	}
	switch c.Profile {
	case "", ProfileFull, ProfileMetaOnly:
	default:
		errs = append(errs, errors.New("SELA_AI_PROFILE: must be full or meta_only"))
	}
	switch c.MetaVerifier.Mode {
	case "", ModeLive, ModeTestDouble, ModeDisabled:
	default:
		errs = append(errs, errors.New("SELA_VERIFIER_MODE: unknown mode"))
	}
	if c.MetaVerifier.Mode == ModeTestDouble && (c.Production() || !c.AllowTestDouble) {
		errs = append(errs, errors.New("SELA_VERIFIER_MODE=test_double requires SELA_ALLOW_TEST_DOUBLE=true and SELA_ENV other than production"))
	}
	if c.OpTimeout != 0 && (c.OpTimeout < MinOpTimeout() || c.OpTimeout > MaxOpTimeout) {
		errs = append(errs, fmt.Errorf("SELA_LLM_OP_TIMEOUT: must be within [%s, %s]: the verification reserve plus a minimum Meta window",
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
	if c.VerifierDailyUSD < 0 || c.VerifierDailyUSD > MaxVerifierDailyUSD || math.IsNaN(c.VerifierDailyUSD) {
		errs = append(errs, fmt.Errorf("SELA_VERIFIER_DAILY_USD: must be a positive amount up to %g", float64(MaxVerifierDailyUSD)))
	}
	return errors.Join(errs...)
}

// Validate checks the whole configuration: validateCore, then the
// provider settings. It builds the client's config and lets it validate
// URLs, keys and models, so the rules live in one place; production
// refuses test doubles and any test-double base URL.
func (c Config) Validate() error {
	errs := []error{c.validateCore()}
	if c.Production() && c.MetaVerifier.BaseURL != "" {
		// Checked even for a disabled provider: production refuses a
		// test-double base URL wherever it is configured.
		if _, err := transport.ParseBaseURL(c.MetaVerifier.BaseURL, transport.BaseURLOptions{Production: true, LiveHosts: []string{metaverifier.LiveHost}}); err != nil {
			errs = append(errs, fmt.Errorf("SELA_META_VERIFIER_BASE_URL: %v", err))
		}
	}
	if c.MetaVerifier.Mode.on() {
		if _, err := metaverifier.New(c.metaVerifierConfig()); err != nil {
			errs = append(errs, fmt.Errorf("SELA_META_VERIFIER_*: %v", err))
		}
	}
	return errors.Join(errs...)
}

func (c Config) metaVerifierConfig() metaverifier.Config {
	return metaverifier.Config{
		APIKey: c.MetaVerifier.APIKey, TestDoubleAPIKey: c.TestDoubleAPIKey, BaseURL: c.MetaVerifier.BaseURL,
		Model: c.MetaVerifier.Model, TestDouble: c.MetaVerifier.Mode == ModeTestDouble,
		Production: c.Production(), AttemptTimeout: c.MetaVerifier.AttemptTimeout,
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
	model := metaverifier.DefaultModel
	if c.MetaVerifier.Mode == ModeTestDouble {
		model = metaverifier.TestDoubleModel
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
	return fmt.Sprintf("env=%s profile=%s verifier=%s(key=%s model=%s) bands=%s",
		c.Env, c.Profile, c.MetaVerifier.Mode, set(c.MetaVerifier.APIKey), orDefault(c.MetaVerifier.Model, model),
		strings.Join(bands, ","))
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
