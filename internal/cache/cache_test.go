package cache

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestDisabledClientIsInert(t *testing.T) {
	c, err := New(Options{})
	if err != nil || c != nil {
		t.Fatalf("New(empty) = %v, %v", c, err)
	}
	var d *Client
	if d.Enabled() {
		t.Fatal("nil client enabled")
	}
	if ok, err := d.GetJSON(context.Background(), "k", &struct{}{}); ok || err != nil {
		t.Fatalf("nil GetJSON = %v, %v", ok, err)
	}
	if err := d.SetJSON(context.Background(), "k", map[string]string{"a": "b"}, 0); err != nil {
		t.Fatal(err)
	}
	if err := d.Delete(context.Background(), "k"); err != nil {
		t.Fatal(err)
	}
	if h := d.Health(context.Background()); h.Enabled || h.OK {
		t.Fatalf("nil health = %+v", h)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestURLValidationAndKeys(t *testing.T) {
	if _, err := New(Options{URL: "http://redis.example"}); err == nil {
		t.Fatal("http scheme accepted")
	}
	if _, err := New(Options{URL: "redis://%zz"}); err == nil {
		t.Fatal("malformed URL accepted")
	}
	c, err := New(Options{URL: "redis://127.0.0.1:1", Prefix: "sela:test"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	got := c.Key("help-content", "v1", "all")
	if want := "sela:test:public:help-content:v1:all"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	// An unreachable Redis is a miss, never a request failure.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if ok, err := c.GetJSON(ctx, c.Key("x", "v1", "y"), &struct{}{}); ok || err != nil {
		t.Fatalf("unreachable GetJSON = %v, %v", ok, err)
	}
	if h := c.Health(ctx); !h.Enabled || h.OK || h.Error != "redis_unavailable" {
		t.Fatalf("health = %+v", h)
	}
}

// TestRedisRoundTrip runs only when SELA_TEST_REDIS_URL names a disposable
// Redis (docker compose service `redis`). It never touches a production URL
// because every key is prefixed with a random test namespace and deleted.
func TestRedisRoundTrip(t *testing.T) {
	url := os.Getenv("SELA_TEST_REDIS_URL")
	if url == "" {
		t.Skip("SELA_TEST_REDIS_URL is not set")
	}
	prefix := "sela:test:" + time.Now().UTC().Format("20060102150405.000000000")
	c, err := New(Options{URL: url, Prefix: prefix, TTL: 2 * time.Second, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	if h := c.Health(ctx); !h.OK {
		t.Fatalf("health = %+v", h)
	}
	key := c.Key("help-content", "v1", "all")
	var out struct {
		Cards int `json:"cards"`
	}
	if ok, err := c.GetJSON(ctx, key, &out); ok || err != nil {
		t.Fatalf("first read = %v, %v", ok, err)
	}
	if err := c.SetJSON(ctx, key, map[string]int{"cards": 30}, 0); err != nil {
		t.Fatal(err)
	}
	if ok, err := c.GetJSON(ctx, key, &out); !ok || err != nil || out.Cards != 30 {
		t.Fatalf("read back = %v, %v, %+v", ok, err, out)
	}
	if err := c.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if ok, _ := c.GetJSON(ctx, key, &out); ok {
		t.Fatal("value survived Delete")
	}
	// TTL is bounded even when the caller asks for longer.
	if err := c.SetJSON(ctx, key, map[string]int{"cards": 1}, time.Hour); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2200 * time.Millisecond)
	if ok, _ := c.GetJSON(ctx, key, &out); ok {
		t.Fatal("value outlived the configured TTL")
	}
}
