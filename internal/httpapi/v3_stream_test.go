package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/httpapi"
)

type streamFrame struct {
	event string
	data  string
}

// parseStreamFrames reads the server's "event: …\ndata: …\n\n" frames in
// order. It fails on an incomplete frame.
func parseStreamFrames(t *testing.T, raw []byte) []streamFrame {
	t.Helper()
	var out []streamFrame
	for _, block := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n\n") {
		if strings.TrimSpace(block) == "" {
			continue
		}
		var frame streamFrame
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				frame.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				frame.data = strings.TrimPrefix(line, "data: ")
			}
		}
		if frame.event == "" || frame.data == "" {
			t.Fatalf("incomplete SSE frame %q", block)
		}
		out = append(out, frame)
	}
	return out
}

func TestV3StreamTurnOnlyReleasesValidatedText(t *testing.T) {
	router, err := ai.NewRouter(ai.Config{Env: ai.EnvTest, AllowedBands: ai.Bands(ai.Band18Plus),
		OpTimeout: ai.DefaultOpTimeout}, ai.Deps{Meta: reflectionGenerator{}, Consent: reflectionConsent{}})
	if err != nil {
		t.Fatal(err)
	}
	e := newV3(t, v3opt{
		mutateC: func(c *account.Config) {
			c.Purposes = append(c.Purposes, account.Purpose{Name: "ai_processing", Kinds: []string{account.KindGuest, account.KindAccount}})
		},
		mutateO: func(_ *v3env, o *httpapi.Options) { o.AIRouter = router },
	})
	c := e.client()
	c.guest("18_plus")
	c.must(200, "PUT", "/api/v3/me/consents/ai_processing", map[string]any{
		"granted": true, "policyVersion": policyVersion("ai_processing"),
	})
	room := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()
	id := room["id"].(string)
	if ttl, _ := room["holdExpiresIn"].(float64); ttl <= 0 || ttl > 1800 {
		t.Fatalf("holdExpiresIn = %v, want a short positive hold", room["holdExpiresIn"])
	}

	res := c.call("POST", "/api/v3/conversations/"+id+"/turns/stream", map[string]any{
		"text": "Aku capek banget sama kuliahku.", "expectedVersion": 1,
	}, "Accept", "text/event-stream")
	if res.status != 200 {
		t.Fatalf("stream status %d: %s", res.status, res.raw)
	}
	if ct := res.header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type %q", ct)
	}
	frames := parseStreamFrames(t, res.raw)
	if len(frames) < 3 {
		t.Fatalf("frames: %+v", frames)
	}
	if frames[0].event != "status" {
		t.Fatalf("first frame %q, want status", frames[0].event)
	}
	var status struct{ State string }
	if err := json.Unmarshal([]byte(frames[0].data), &status); err != nil || status.State != "thinking" {
		t.Fatalf("status: %s (%v)", frames[0].data, err)
	}
	var chunks []string
	for _, frame := range frames[1 : len(frames)-1] {
		if frame.event != "chunk" {
			t.Fatalf("middle frame %q, want chunk", frame.event)
		}
		var piece struct{ Text string }
		if err := json.Unmarshal([]byte(frame.data), &piece); err != nil {
			t.Fatalf("chunk: %s (%v)", frame.data, err)
		}
		chunks = append(chunks, piece.Text)
	}
	if got := strings.Join(chunks, ""); got != "Terima kasih sudah cerita." {
		t.Fatalf("streamed %q", got)
	}
	if last := frames[len(frames)-1]; last.event != "done" {
		t.Fatalf("last frame %q, want done", last.event)
	}
	var done map[string]any
	if err := json.Unmarshal([]byte(frames[len(frames)-1].data), &done); err != nil {
		t.Fatalf("done: %s (%v)", frames[len(frames)-1].data, err)
	}
	turns, _ := done["turns"].([]any)
	if len(turns) != 2 {
		t.Fatalf("done room turns: %v", done["turns"])
	}
	last, _ := turns[1].(map[string]any)
	if last["role"] != "assistant" || last["text"] != "Terima kasih sudah cerita." {
		t.Fatalf("done assistant turn: %v", last)
	}
	if done["aiState"] != "ready" {
		t.Fatalf("aiState: %v", done["aiState"])
	}

	stored := c.must(200, "GET", "/api/v3/conversations/"+id, nil).data()
	storedTurns, _ := stored["turns"].([]any)
	if len(storedTurns) != 2 {
		t.Fatalf("stored turns: %v", stored["turns"])
	}
	assistant := 0
	for _, raw := range storedTurns {
		turn, _ := raw.(map[string]any)
		if turn["role"] == "assistant" {
			assistant++
		}
	}
	if assistant != 1 {
		t.Fatalf("stored assistant turns = %d, want 1", assistant)
	}
}

func TestV3StreamTurnPreflightStaysJSON(t *testing.T) {
	e := newV3(t, v3opt{})
	c := e.client()
	c.guest("18_plus")
	id := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()["id"].(string)

	res := c.call("POST", "/api/v3/conversations/"+id+"/turns/stream", map[string]any{
		"text": "Halo.", "expectedVersion": 99,
	}, "Accept", "text/event-stream")
	if res.status != http.StatusConflict || res.code() != "stale_version" {
		t.Fatalf("stale stream: %d %s", res.status, res.raw)
	}
	if ct := res.header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("preflight content type %q", ct)
	}
}

func TestV3StreamTurnCrisisHasNoChunks(t *testing.T) {
	router, err := ai.NewRouter(ai.Config{Env: ai.EnvTest, AllowedBands: ai.Bands(ai.Band18Plus),
		OpTimeout: ai.DefaultOpTimeout}, ai.Deps{Meta: reflectionGenerator{}, Consent: reflectionConsent{}})
	if err != nil {
		t.Fatal(err)
	}
	e := newV3(t, v3opt{
		mutateC: func(c *account.Config) {
			c.Purposes = append(c.Purposes, account.Purpose{Name: "ai_processing", Kinds: []string{account.KindGuest, account.KindAccount}})
		},
		mutateO: func(_ *v3env, o *httpapi.Options) { o.AIRouter = router },
	})
	c := e.client()
	c.guest("18_plus")
	c.must(200, "PUT", "/api/v3/me/consents/ai_processing", map[string]any{
		"granted": true, "policyVersion": policyVersion("ai_processing"),
	})
	id := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()["id"].(string)
	res := c.call("POST", "/api/v3/conversations/"+id+"/turns/stream", map[string]any{
		"text": "Aku mau bunuh diri malam ini.", "expectedVersion": 1,
	}, "Accept", "text/event-stream")
	if res.status != 200 {
		t.Fatalf("crisis stream: %d %s", res.status, res.raw)
	}
	frames := parseStreamFrames(t, res.raw)
	if len(frames) != 2 || frames[0].event != "status" || frames[1].event != "done" {
		t.Fatalf("crisis frames: %+v", frames)
	}
	var done map[string]any
	if err := json.Unmarshal([]byte(frames[1].data), &done); err != nil {
		t.Fatal(err)
	}
	if done["paused"] != true || done["aiState"] != "paused" {
		t.Fatalf("crisis room: %v", done)
	}
}

type slowStreamGenerator struct{ delay time.Duration }

func (g slowStreamGenerator) Identity() ai.GeneratorIdentity { return reflectionGenerator{}.Identity() }
func (g slowStreamGenerator) Generate(ctx context.Context, req ai.GenerateRequest) (ai.GenerateResult, error) {
	select {
	case <-time.After(g.delay):
	case <-ctx.Done():
		return ai.GenerateResult{}, ctx.Err()
	}
	return reflectionGenerator{}.Generate(ctx, req)
}

func TestV3StreamTurnKeepsSlowCallsAlive(t *testing.T) {
	router, err := ai.NewRouter(ai.Config{Env: ai.EnvTest, AllowedBands: ai.Bands(ai.Band18Plus),
		OpTimeout: ai.DefaultOpTimeout}, ai.Deps{Meta: slowStreamGenerator{delay: 80 * time.Millisecond}, Consent: reflectionConsent{}})
	if err != nil {
		t.Fatal(err)
	}
	e := newV3(t, v3opt{
		mutateC: func(c *account.Config) {
			c.Purposes = append(c.Purposes, account.Purpose{Name: "ai_processing", Kinds: []string{account.KindGuest, account.KindAccount}})
		},
		mutateO: func(_ *v3env, o *httpapi.Options) {
			o.AIRouter = router
			o.StreamHeartbeat = 5 * time.Millisecond
		},
	})
	c := e.client()
	c.guest("18_plus")
	c.must(200, "PUT", "/api/v3/me/consents/ai_processing", map[string]any{
		"granted": true, "policyVersion": policyVersion("ai_processing"),
	})
	id := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()["id"].(string)
	res := c.call("POST", "/api/v3/conversations/"+id+"/turns/stream", map[string]any{
		"text": "Aku capek banget sama kuliahku.", "expectedVersion": 1,
	}, "Accept", "text/event-stream")
	if res.status != 200 {
		t.Fatalf("stream status %d: %s", res.status, res.raw)
	}
	frames := parseStreamFrames(t, res.raw)
	heartbeats := 0
	for _, f := range frames {
		if f.event == "heartbeat" {
			heartbeats++
		}
	}
	if heartbeats == 0 {
		t.Fatalf("no heartbeat in %d frames", len(frames))
	}
	if last := frames[len(frames)-1]; last.event != "done" {
		t.Fatalf("last frame %q, want done", last.event)
	}
}
