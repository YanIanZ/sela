// Package cache is Sela's optional ephemeral acceleration plane.
//
// It stores only public, versioned data (the help-content catalogue and
// similar non-personal records) and health metadata. It is never the source
// of truth for consent, safety, identity or private content. Redis failure
// falls back to the embedded/DB source; it never changes an authorization or
// safety decision.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Options configures the optional Redis client.
type Options struct {
	// URL is REDIS_URL/rediss URL. Empty disables Redis entirely.
	URL string
	// Prefix namespaces every key, e.g. "sela:production".
	Prefix string
	// TTL bounds every cached public value.
	TTL time.Duration
	// Timeout bounds one Redis command.
	Timeout time.Duration
}

// Client is a bounded Redis wrapper. The zero value is disabled.
type Client struct {
	rdb     *redis.Client
	prefix  string
	ttl     time.Duration
	timeout time.Duration
}

type silentLogger struct{}

func (silentLogger) Printf(context.Context, string, ...interface{}) {}

var silenceOnce sync.Once

// New opens the optional client. An empty URL returns (nil, nil) so callers
// keep the no-Redis path.
func New(o Options) (*Client, error) {
	if strings.TrimSpace(o.URL) == "" {
		return nil, nil
	}
	// go-redis logs pool failures to stderr by default. An optional cache
	// that is down is an expected state here, reported by Health and the
	// caller's own warning, not by an unstructured library log.
	silenceOnce.Do(func() { redis.SetLogger(silentLogger{}) })
	u, err := url.Parse(o.URL)
	if err != nil {
		return nil, fmt.Errorf("cache: invalid REDIS_URL: %w", err)
	}
	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return nil, errors.New("cache: REDIS_URL must be redis:// or rediss://")
	}
	if o.TTL <= 0 {
		o.TTL = 5 * time.Minute
	}
	if o.Timeout <= 0 {
		o.Timeout = 300 * time.Millisecond
	}
	if o.Prefix == "" {
		o.Prefix = "sela"
	}
	opt, err := redis.ParseURL(o.URL)
	if err != nil {
		return nil, fmt.Errorf("cache: parse REDIS_URL: %w", err)
	}
	opt.DialTimeout = o.Timeout
	opt.ReadTimeout = o.Timeout
	opt.WriteTimeout = o.Timeout
	opt.MaxRetries = 0
	return &Client{rdb: redis.NewClient(opt), prefix: o.Prefix, ttl: o.TTL, timeout: o.Timeout}, nil
}

// Enabled reports whether a Redis client is configured.
func (c *Client) Enabled() bool { return c != nil && c.rdb != nil }

// Key returns a namespaced public key.
func (c *Client) Key(kind, revision, id string) string {
	parts := []string{c.prefix, "public", kind, revision, id}
	return strings.Join(parts, ":")
}

// GetJSON reads a public value. A miss, disabled client or Redis error is
// (false, nil): callers fall back to the authoritative source. Errors are
// only returned for a malformed stored value.
func (c *Client) GetJSON(ctx context.Context, key string, dst any) (bool, error) {
	if !c.Enabled() {
		return false, nil
	}
	cctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	raw, err := c.rdb.Get(cctx, key).Bytes()
	if errors.Is(err, redis.Nil) || err != nil {
		return false, nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return false, err
	}
	return true, nil
}

// SetJSON writes a public value best-effort. The TTL is clamped to the
// client maximum. Redis errors are returned so the caller can log them, but
// callers must not fail the request on them.
func (c *Client) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if !c.Enabled() {
		return nil
	}
	if ttl <= 0 || ttl > c.ttl {
		ttl = c.ttl
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	return c.rdb.Set(cctx, key, raw, ttl).Err()
}

// Delete removes one public key. It is used by the operator invalidation
// route; it never scans for or deletes private keys.
func (c *Client) Delete(ctx context.Context, key string) error {
	if !c.Enabled() {
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	return c.rdb.Del(cctx, key).Err()
}

// Health is the operator-safe Redis health report. It carries no key names,
// values or connection secrets.
type Health struct {
	Enabled   bool   `json:"enabled"`
	OK        bool   `json:"ok"`
	LatencyMS int64  `json:"latencyMs,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Health pings Redis with the bounded timeout.
func (c *Client) Health(ctx context.Context) Health {
	if !c.Enabled() {
		return Health{Enabled: false, OK: false}
	}
	start := time.Now()
	cctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	err := c.rdb.Ping(cctx).Err()
	h := Health{Enabled: true, OK: err == nil, LatencyMS: time.Since(start).Milliseconds()}
	if err != nil {
		h.Error = "redis_unavailable"
	}
	return h
}

// Close releases the client.
func (c *Client) Close() error {
	if !c.Enabled() {
		return nil
	}
	return c.rdb.Close()
}
