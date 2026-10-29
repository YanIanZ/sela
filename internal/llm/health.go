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
// ServedModel and whose root is exactly the pinned snapshot (vLLM reports
// the local path, .../models--<org>--<name>/snapshots/<revision>; see
// pinnedRoot). Revision must be the full commit the snapshot is named
// after; a short form does not match.
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
	for _, m := range list.Data {
		if m.ID == c.cfg.ServedModel {
			// Meta Model API lists hosted model ids without a Hugging Face
			// snapshot root; the served id is the identity there.
			if c.cfg.Mode == ModeMeta {
				return true
			}
			return pinnedRoot(m.Root, c.cfg.ModelID, c.cfg.Revision)
		}
	}
	return false
}

// pinnedRoot reports whether root, the "root" of a /v1/models entry, names
// exactly repository modelID at revision: either "<modelID>@<revision>" or
// a Hugging Face cache snapshot directory whose last three path segments
// are "models--<org>--<name>", "snapshots" and "<revision>" (vLLM reports
// the local path it serves). Matching is exact, never by substring or
// prefix: a repository whose name merely starts with modelID, a path that
// merely contains revision, or a short revision for a full one does not
// match.
func pinnedRoot(root, modelID, revision string) bool {
	if modelID == "" || revision == "" {
		return false
	}
	if root == modelID+"@"+revision {
		return true
	}
	var segs []string
	for _, s := range strings.Split(root, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	n := len(segs)
	hubDir := "models--" + strings.ReplaceAll(modelID, "/", "--")
	return n >= 3 && segs[n-3] == hubDir && segs[n-2] == "snapshots" && segs[n-1] == revision
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
