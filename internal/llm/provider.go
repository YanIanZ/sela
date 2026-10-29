package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Defaults applied by New when the corresponding Config field is zero.
const (
	DefaultAttemptTimeout = 90 * time.Second
	DefaultMaxRetries     = 2
	DefaultMaxOutputBytes = 32 << 10
	// DefaultMaxTokens is sent when a Request leaves MaxTokens unset, so every
	// request stays bounded.
	DefaultMaxTokens = 1024
)

// Identity of cmd/fakellm, the deterministic test double. ModeTestDouble
// requires TestDoubleServedModel, so test-double output can never be
// mistaken for Meta inference.
const (
	TestDoubleServedModel = "test-double-not-meta"
	TestDoubleModelID     = "test-double/not-meta"
	TestDoubleRevision    = "test-double"
)

// Config configures an OpenAI-compatible provider.
type Config struct {
	Mode Mode
	// BaseURL is the server root as printed by `modal deploy` (Modal Servers
	// use *.modal.direct hosts; copy the URL, do not construct it). The
	// client appends /v1/chat/completions, /health and /v1/models. It must not
	// carry credentials, a query or a fragment.
	BaseURL string
	// APIKey is sent as "Authorization: Bearer" (vLLM --api-key).
	APIKey string
	// ModalKey/ModalSecret are a Modal proxy-auth token pair sent as the
	// Modal-Key and Modal-Secret headers.
	ModalKey    string
	ModalSecret string
	ServedModel string
	// ModelID and Revision are the pinned Hugging Face repository and commit.
	// In ModeTestDouble they default to TestDoubleModelID and
	// TestDoubleRevision.
	ModelID  string
	Revision string
	// AttemptTimeout bounds one HTTP attempt; the caller's context bounds the
	// whole operation. Zero means DefaultAttemptTimeout.
	AttemptTimeout time.Duration
	// MaxRetries is the number of retries after the first attempt. Zero means
	// DefaultMaxRetries; a negative value disables retries.
	MaxRetries int
	// MaxOutputBytes bounds the generated text. Zero means
	// DefaultMaxOutputBytes.
	MaxOutputBytes int
	// HTTPClient is used for all requests (nil means a dedicated client).
	// Redirects are never followed, so credentials cannot leak to another
	// host; a Timeout on it would cut streams and should be left zero.
	HTTPClient *http.Client
	// ReasoningEffort is the Meta Model API reasoning_effort value
	// (minimal, low, medium, high or xhigh). It applies only in ModeMeta;
	// Muse Spark rejects "none". Empty defaults to minimal.
	ReasoningEffort string
	// Observer, when set, receives one metadata-only Event per Complete
	// call. It is called synchronously and possibly concurrently.
	Observer Observer
}

// New returns a provider for cfg.Mode after validating the configuration.
// Validation errors name the offending fields but never include their values.
func New(cfg Config) (Provider, error) {
	switch cfg.Mode {
	case ModeDisabled:
		return Disabled(), nil
	case ModeModal, ModeMeta, ModeTestDouble:
	default:
		return nil, errors.New("llm: invalid config: mode must be modal, meta_api, test_double or disabled")
	}
	if cfg.Mode == ModeMeta {
		if cfg.ModelID != "" && cfg.Revision == "" {
			cfg.Revision = cfg.ModelID
		}
		if cfg.ReasoningEffort == "" {
			cfg.ReasoningEffort = "minimal"
		}
	}
	if cfg.Mode == ModeTestDouble {
		if cfg.ModelID == "" {
			cfg.ModelID = TestDoubleModelID
		}
		if cfg.Revision == "" {
			cfg.Revision = TestDoubleRevision
		}
	}
	base, err := validate(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.AttemptTimeout <= 0 {
		cfg.AttemptTimeout = DefaultAttemptTimeout
	}
	switch {
	case cfg.MaxRetries == 0:
		cfg.MaxRetries = DefaultMaxRetries
	case cfg.MaxRetries < 0:
		cfg.MaxRetries = 0
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = DefaultMaxOutputBytes
	}
	root := strings.TrimRight(base.String(), "/")
	// Meta Model API documents its base URL as https://api.meta.ai/v1;
	// this client appends /v1/chat/completions itself. Accept the documented
	// form without producing /v1/v1/...
	if cfg.Mode == ModeMeta {
		root = strings.TrimSuffix(root, "/v1")
	}
	return &client{
		cfg:       cfg,
		hc:        httpClient(cfg.HTTPClient),
		chatURL:   root + "/v1/chat/completions",
		healthURL: root + "/health",
		modelsURL: root + "/v1/models",
		endpoint:  base.Host,
	}, nil
}

// validate checks the mode-specific requirements and returns the parsed
// base URL.
func validate(cfg Config) (*url.URL, error) {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	base, err := url.Parse(cfg.BaseURL)
	switch {
	case cfg.BaseURL == "":
		add("BaseURL is required")
	case err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http"):
		add("BaseURL must be an absolute http(s) URL")
	case base.User != nil || base.RawQuery != "" || base.Fragment != "" || base.Opaque != "":
		add("BaseURL must not contain credentials, a query or a fragment")
	case cfg.Mode == ModeModal && base.Scheme != "https":
		add("BaseURL must use https in modal mode")
	case cfg.Mode == ModeMeta && (base.Scheme != "https" || !strings.EqualFold(base.Hostname(), "api.meta.ai")):
		add("BaseURL must be https://api.meta.ai/v1 in meta_api mode")
	case cfg.Mode == ModeTestDouble && base.Scheme == "http" && !isLoopback(base.Hostname()):
		add("BaseURL may use http only for a loopback host in test_double mode")
	}

	switch cfg.Mode {
	case ModeModal:
		for _, f := range []struct{ name, value string }{
			{"APIKey", cfg.APIKey}, {"ModalKey", cfg.ModalKey}, {"ModalSecret", cfg.ModalSecret},
			{"ServedModel", cfg.ServedModel}, {"ModelID", cfg.ModelID}, {"Revision", cfg.Revision},
		} {
			if strings.TrimSpace(f.value) == "" {
				add("%s is required in modal mode", f.name)
			}
		}
		if cfg.ServedModel == TestDoubleServedModel {
			add("ServedModel must not be the test-double name in modal mode")
		}
	case ModeMeta:
		for _, f := range []struct{ name, value string }{
			{"APIKey", cfg.APIKey}, {"ServedModel", cfg.ServedModel}, {"ModelID", cfg.ModelID},
		} {
			if strings.TrimSpace(f.value) == "" {
				add("%s is required in meta_api mode", f.name)
			}
		}
		if cfg.ServedModel == TestDoubleServedModel {
			add("ServedModel must not be the test-double name in meta_api mode")
		}
		switch cfg.ReasoningEffort {
		case "minimal", "low", "medium", "high", "xhigh":
		default:
			add("ReasoningEffort must be minimal, low, medium, high or xhigh in meta_api mode")
		}
	case ModeTestDouble:
		if cfg.ServedModel != TestDoubleServedModel {
			add("ServedModel must be %q in test_double mode", TestDoubleServedModel)
		}
	}
	for _, v := range []string{cfg.APIKey, cfg.ModalKey, cfg.ModalSecret} {
		if strings.ContainsAny(v, "\r\n") {
			add("credentials must not contain line breaks")
			break
		}
	}
	if cfg.AttemptTimeout < 0 {
		add("AttemptTimeout must not be negative")
	}
	if cfg.MaxOutputBytes < 0 {
		add("MaxOutputBytes must not be negative")
	}
	if len(problems) > 0 {
		return nil, errors.New("llm: invalid config: " + strings.Join(problems, "; "))
	}
	return base, nil
}

// isLoopback reports whether host is "localhost" or a loopback IP literal.
func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// httpClient returns a copy of hc (or a dedicated client) that never follows
// redirects: Go forwards custom headers such as Modal-Secret on redirect, so
// a redirect could leak credentials to another host.
func httpClient(hc *http.Client) *http.Client {
	var c http.Client
	if hc != nil {
		c = *hc
	} else {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.MaxIdleConnsPerHost = 16
		c.Transport = t
	}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

// Disabled returns a provider whose calls fail with KindDisabled.
func Disabled() Provider { return disabled{} }

type disabled struct{}

func (disabled) Info() Info { return Info{Mode: ModeDisabled} }

func (disabled) Complete(context.Context, Request, func(string) error) (Result, error) {
	return Result{}, &Error{Kind: KindDisabled}
}

func (disabled) Health(context.Context) Health {
	return Health{State: "disabled", CheckedAt: time.Now()}
}
