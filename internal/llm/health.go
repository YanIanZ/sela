package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	healthTimeout   = 10 * time.Second
	maxHealthBytes  = 64 << 10
	healthStateOK   = "ok"
	healthStateDown = "unavailable"
)

// healthBody is the identity document served at /health by cmd/fakellm and
// by an identity-aware Modal server.
type healthBody struct {
	Status      *string `json:"status"`
	ModelID     *string `json:"model_id"`
	Revision    *string `json:"revision"`
	ServedModel *string `json:"served_model"`
}

// Health performs GET {BaseURL}/health with the configured credentials and
// a 10 s timeout. State is "ok" only when the server proves the configured
// identity; any failure or mismatch yields "unavailable". The response body
// is never returned or logged.
//
// The server proves its identity either with a JSON body
// {"status":"ok","model_id":...,"revision":...,"served_model":...} matching
// Config, or, when /health answers 200 without that document (vLLM's own
// /health has an empty body), with a /v1/models entry whose id is
// ServedModel and whose root names ModelID and Revision (vLLM reports the
// local snapshot path, .../models--<org>--<name>/snapshots/<revision>).
func (c *client) Health(ctx context.Context) Health {
	start := time.Now()
	hctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	ok := c.checkHealth(hctx)
	h := Health{State: healthStateDown, Latency: time.Since(start).Milliseconds(), CheckedAt: start}
	if ok {
		h.State, h.ModelID, h.Revision = healthStateOK, c.cfg.ModelID, c.cfg.Revision
	}
	return h
}

func (c *client) checkHealth(ctx context.Context) bool {
	body, ok := c.get(ctx, c.healthURL)
	if !ok {
		return false
	}
	var hb healthBody
	if json.Unmarshal(body, &hb) == nil && hb.ServedModel != nil {
		return hb.Status != nil && *hb.Status == "ok" &&
			*hb.ServedModel == c.cfg.ServedModel &&
			hb.ModelID != nil && *hb.ModelID == c.cfg.ModelID &&
			hb.Revision != nil && *hb.Revision == c.cfg.Revision
	}
	return c.checkModels(ctx)
}

// checkModels verifies the served identity through /v1/models.
func (c *client) checkModels(ctx context.Context) bool {
	body, ok := c.get(ctx, c.modelsURL)
	if !ok {
		return false
	}
	var list struct {
		Data []struct {
			ID   string `json:"id"`
			Root string `json:"root"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &list) != nil || c.cfg.Revision == "" || c.cfg.ModelID == "" {
		return false
	}
	hubDir := "models--" + strings.ReplaceAll(c.cfg.ModelID, "/", "--")
	for _, m := range list.Data {
		if m.ID != c.cfg.ServedModel {
			continue
		}
		hasModel := m.Root == c.cfg.ModelID || strings.Contains(m.Root, hubDir) || strings.HasPrefix(m.Root, c.cfg.ModelID+"@")
		return hasModel && strings.Contains(m.Root, c.cfg.Revision)
	}
	return false
}

// get performs an authenticated GET and returns a bounded body for 200.
func (c *client) get(ctx context.Context, u string) ([]byte, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false
	}
	c.setAuth(req)
	req.Header.Set("Accept", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.CopyN(io.Discard, resp.Body, maxDrainBytes)
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHealthBytes+1))
	if err != nil || len(body) > maxHealthBytes {
		return nil, false
	}
	return bytes.TrimSpace(body), true
}
