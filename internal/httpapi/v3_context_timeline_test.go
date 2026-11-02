package httpapi_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/httpapi"
)

// countingGenerator counts provider calls; the timeline must make none.
type countingGenerator struct{ calls atomic.Int64 }

func (g *countingGenerator) Identity() ai.GeneratorIdentity {
	return ai.GeneratorIdentity{Role: ai.ProviderMeta, ModelID: "test-generator", Revision: "test"}
}

func (g *countingGenerator) Generate(context.Context, ai.GenerateRequest) (ai.GenerateResult, error) {
	g.calls.Add(1)
	return ai.GenerateResult{}, fmt.Errorf("the timeline must not call a provider")
}

func timelineItems(t *testing.T, r resp) []map[string]any {
	t.Helper()
	raw, ok := r.data()["items"].([]any)
	if !ok {
		t.Fatalf("no items in %s", r.raw)
	}
	out := make([]map[string]any, len(raw))
	for i, x := range raw {
		out[i] = x.(map[string]any)
	}
	return out
}

func itemID(it map[string]any) string { return it["source"].(map[string]any)["recordId"].(string) }

// LT-01, LT-02, LT-09, BW-T01, BW-T08, BW-T12: AI consent granted and a
// provider wired, yet a timeline read sends nothing to it; owners are
// isolated; unknown and foreign cursors are indistinguishable; the answer is
// never cached.
func TestV3ContextTimelineOwnerScopeNoAI(t *testing.T) {
	gen := &countingGenerator{}
	router, err := ai.NewRouter(ai.Config{
		Env: ai.EnvTest, AllowedBands: ai.Bands(ai.Band18Plus), OpTimeout: ai.DefaultOpTimeout,
	}, ai.Deps{Meta: gen, Consent: allowAI{}})
	if err != nil {
		t.Fatal(err)
	}
	e := newV3(t, v3opt{
		mutateC: func(c *account.Config) {
			c.Purposes = append(c.Purposes,
				account.Purpose{Name: "ai_processing", Kinds: []string{account.KindGuest, account.KindAccount}})
		},
		mutateO: func(_ *v3env, o *httpapi.Options) { o.AIRouter = router },
	})
	if r := e.client().call("GET", "/api/v3/context-timeline", nil); r.status != 401 || r.code() != "not_signed_in" {
		t.Fatalf("signed out: %d %s", r.status, r.raw)
	}
	a, b := e.client(), e.client()
	a.guest("18_plus")
	b.guest("18_plus")
	a.must(200, "PUT", "/api/v3/me/consents/ai_processing", map[string]any{"granted": true, "policyVersion": policyVersion("ai_processing")})

	empty := a.must(200, "GET", "/api/v3/context-timeline", nil)
	if len(timelineItems(t, empty)) != 0 || empty.data()["nextCursor"] != nil || empty.data()["aiUsed"] != false ||
		empty.data()["count"] != float64(0) {
		t.Fatalf("empty: %s", empty.raw)
	}
	if cc := empty.header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control %q", cc)
	}

	secret := a.must(201, "POST", "/api/v3/records", map[string]any{"kind": "note", "text": "Synthetic secret of owner A.", "storageGranted": true}).data()
	e.clock.Advance(time.Minute)
	b.must(201, "POST", "/api/v3/records", map[string]any{"kind": "note", "text": "Synthetic note of owner B.", "storageGranted": true})
	for i := 0; i < 3; i++ {
		a.must(201, "POST", "/api/v3/records", map[string]any{"kind": "note", "text": fmt.Sprintf("Synthetic note %d.", i), "storageGranted": true})
	}

	got := timelineItems(t, a.must(200, "GET", "/api/v3/context-timeline?days=7&kinds=note,checkin,conversation", nil))
	if len(got) != 4 {
		t.Fatalf("owner A items: %d", len(got))
	}
	bItems := timelineItems(t, b.must(200, "GET", "/api/v3/context-timeline", nil))
	if len(bItems) != 1 || strings.Contains(fmt.Sprint(bItems), "owner A") {
		t.Fatalf("owner B: %v", bItems)
	}
	cursor := func(id string) string {
		raw := fmt.Sprintf("v1|desc|%d|%s", e.clock.Now().Add(time.Hour).UnixMicro(), id)
		return url.QueryEscape(base64.RawURLEncoding.EncodeToString([]byte(raw)))
	}
	foreign := b.must(200, "GET", "/api/v3/context-timeline?cursor="+cursor(secret["id"].(string)), nil)
	unknown := b.must(200, "GET", "/api/v3/context-timeline?cursor="+cursor("6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11"), nil)
	if string(foreign.raw) != string(unknown.raw) {
		t.Fatalf("foreign and unknown cursors differ:\n%s\n%s", foreign.raw, unknown.raw)
	}

	// Paging over HTTP follows the cursor to the end.
	var paged []string
	next := ""
	for i := 0; i < 10; i++ {
		path := "/api/v3/context-timeline?limit=1"
		if next != "" {
			path += "&cursor=" + url.QueryEscape(next)
		}
		r := a.must(200, "GET", path, nil)
		for _, it := range timelineItems(t, r) {
			paged = append(paged, itemID(it))
		}
		c, _ := r.data()["nextCursor"].(string)
		if c == "" {
			break
		}
		next = c
	}
	if len(paged) != 4 || paged[0] != itemID(got[0]) || paged[3] != itemID(got[3]) {
		t.Fatalf("paged %v vs %v", paged, got)
	}
	if n := gen.calls.Load(); n != 0 {
		t.Fatalf("provider called %d times by timeline reads", n)
	}
}

func TestV3ContextTimelineValidation(t *testing.T) {
	e := newV3(t, v3opt{})
	c := e.client()
	c.guest("18_plus")
	for q, field := range map[string]string{
		"limit=0":                          "limit",
		"limit=51":                         "limit",
		"limit=abc":                        "limit",
		"kinds=memory":                     "kinds",
		"kinds=note,summary":               "kinds",
		"days=14":                          "days",
		"days=7&from=2026-11-01T00:00:00Z": "days",
		"from=yesterday":                   "from",
		"to=2026-13-01":                    "to",
		"from=2026-11-02T00:00:00Z&to=2026-11-01T00:00:00Z": "to",
		"order=sideways": "order",
		"cursor=%25%25":  "cursor",
	} {
		r := c.call("GET", "/api/v3/context-timeline?"+q, nil)
		if r.status != 400 || r.code() != "invalid_input" || r.field() != field {
			t.Errorf("%s: %d %s", q, r.status, r.raw)
		}
	}
}

// LT-06, LT-07: after a source edit the timeline shows the new version as a
// correction, marked stale in text while the summary built on the old
// version is stale; deleting the source removes it on the next read.
func TestV3ContextTimelineStaleAfterEdit(t *testing.T) {
	e := newV3(t, v3opt{})
	a := e.client()
	a.guest("18_plus")
	note := a.must(201, "POST", "/api/v3/records", map[string]any{"kind": "note", "text": "Synthetic source v1.", "storageGranted": true}).data()
	id := note["id"].(string)
	a.must(201, "POST", "/api/v3/consultation-summaries", map[string]any{
		"title": "Synthetic summary", "text": "Synthetic summary.", "storageGranted": true,
		"sourceRefs": []map[string]any{{"recordId": id, "version": 1}},
		"prepared":   map[string]any{"goal": "g", "questions": "q", "bring": "b"},
	})
	it := timelineItems(t, a.must(200, "GET", "/api/v3/context-timeline", nil))[0]
	if it["meaningStatus"] != "current" || it["source"].(map[string]any)["version"] != float64(1) || it["excerpt"] != "Synthetic source v1." {
		t.Fatalf("before: %v", it)
	}
	a.must(200, "PATCH", "/api/v3/records/"+id, map[string]any{"kind": "note", "text": "Synthetic source v2.", "storageGranted": true, "expectedVersion": 1})
	it = timelineItems(t, a.must(200, "GET", "/api/v3/context-timeline", nil))[0]
	stale, _ := it["stale"].(map[string]any)
	if it["meaningStatus"] != "stale" || it["provenance"] != "user_correction" || it["source"].(map[string]any)["version"] != float64(2) ||
		it["excerpt"] != "Synthetic source v2." || stale["reason"] != "source_edited" {
		t.Fatalf("after edit: %v", it)
	}
	a.must(200, "DELETE", "/api/v3/records/"+id, map[string]any{"expectedVersion": 2})
	if items := timelineItems(t, a.must(200, "GET", "/api/v3/context-timeline", nil)); len(items) != 0 {
		t.Fatalf("after delete: %v", items)
	}
}
