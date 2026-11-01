package httpapi_test

import (
	"strings"
	"testing"
)

func TestV3DerivedImpactAndDeleteModes(t *testing.T) {
	e := newV3(t, v3opt{})
	owner := e.client()
	ownerAuth := owner.guest("18_plus")
	ownerID := userID(t, ownerAuth)
	other := e.client()
	other.guest("18_plus")

	createNote := func(c *v3client, text string) map[string]any {
		t.Helper()
		return c.must(201, "POST", "/api/v3/records", map[string]any{
			"kind": "note", "title": "Synthetic derived source", "text": text, "storageGranted": true,
		}).data()
	}
	createSummary := func(c *v3client, source map[string]any, text string) map[string]any {
		t.Helper()
		summary := c.must(201, "POST", "/api/v3/consultation-summaries", map[string]any{
			"title": "Synthetic derived summary", "text": text, "storageGranted": true,
			"sourceRefs": []map[string]any{{"recordId": source["id"], "version": source["version"]}},
		}).data()
		c.must(200, "POST", "/api/v3/consultation-summaries/"+summary["id"].(string)+"/approve",
			map[string]any{"expectedVersion": summary["version"]})
		return summary
	}
	createFollowup := func(c *v3client, summaryID, text string) map[string]any {
		t.Helper()
		return c.must(201, "POST", "/api/v3/consultation-summaries/"+summaryID+"/followups", map[string]any{
			"title": "Synthetic derived follow-up", "text": text, "storageGranted": true,
		}).data()
	}

	source := createNote(owner, "Synthetic derived source body.")
	sourceID := source["id"].(string)
	summary := createSummary(owner, source, "Synthetic derived summary body.")
	summaryID := summary["id"].(string)
	followup := createFollowup(owner, summaryID, "Synthetic derived follow-up body.")
	share, _, _, err := e.store.CreateShare(bg, ownerID, summaryID, "Dr. Derived", "consultation", 24)
	if err != nil {
		t.Fatal(err)
	}

	impact := owner.must(200, "GET", "/api/v3/derived?source="+sourceID, nil)
	data := impact.data()
	if data["sourceId"] != sourceID {
		t.Fatalf("impact source: %s", impact.raw)
	}
	summaries := data["summaries"].([]any)
	if len(summaries) != 1 {
		t.Fatalf("impact summaries: %s", impact.raw)
	}
	one := summaries[0].(map[string]any)
	if one["id"] != summaryID || one["title"] != "Synthetic derived summary" ||
		one["version"] != float64(1) || one["approved"] != true || one["shareCount"] != float64(1) {
		t.Fatalf("impact summary: %v", one)
	}
	totals := data["totals"].(map[string]any)
	if totals["summaries"] != float64(1) || totals["shares"] != float64(1) {
		t.Fatalf("impact totals: %v", totals)
	}
	// The impact answer is content-free apart from owner-visible metadata.
	if strings.Contains(string(impact.raw), "Synthetic derived source body") ||
		strings.Contains(string(impact.raw), "Synthetic derived summary body") ||
		strings.Contains(string(impact.raw), "Synthetic derived follow-up body") {
		t.Fatalf("impact leaks content: %s", impact.raw)
	}

	for name, path := range map[string]string{
		"foreign":   "/api/v3/derived?source=" + sourceID,
		"unknown":   "/api/v3/derived?source=00000000-0000-4000-8000-000000000000",
		"malformed": "/api/v3/derived?source=not-a-uuid",
		"missing":   "/api/v3/derived",
	} {
		caller := owner
		if name == "foreign" {
			caller = other
		}
		if r := caller.call("GET", path, nil); r.status != 404 || r.code() != "not_found" {
			t.Fatalf("%s impact: %d %s", name, r.status, r.raw)
		}
	}

	if r := owner.call("DELETE", "/api/v3/records/"+sourceID+"?derived=shred",
		map[string]any{"expectedVersion": 1}); r.status != 400 || r.code() != "invalid_input" || r.field() != "derived" {
		t.Fatalf("invalid mode: %d %s", r.status, r.raw)
	}
	owner.must(409, "DELETE", "/api/v3/records/"+sourceID+"?derived=delete",
		map[string]any{"expectedVersion": 99})
	if got := owner.must(200, "GET", "/api/v3/derived?source="+sourceID, nil).data(); len(got["summaries"].([]any)) != 1 {
		t.Fatalf("stale delete changed impact: %v", got)
	}

	// Keep: the source goes; the summary stays as owner history and its
	// frozen share stays listed but revoked for future readers (D-078).
	owner.must(200, "DELETE", "/api/v3/records/"+sourceID+"?derived=keep",
		map[string]any{"expectedVersion": source["version"]})
	kept := owner.must(200, "GET", "/api/v3/consultation-summaries/"+summaryID, nil).data()
	if kept["summaryState"] != "draft" {
		t.Fatalf("kept summary state: %v", kept)
	}
	owner.must(200, "GET", "/api/v3/records/"+followup["id"].(string), nil)
	shares := owner.must(200, "GET", "/api/v3/shares", nil).data()["shares"].([]any)
	if len(shares) != 1 || shares[0].(map[string]any)["id"] != share.ID || shares[0].(map[string]any)["state"] != "revoked" {
		t.Fatalf("kept share: %v", shares)
	}

	// Delete: the derived summary, its follow-up and its share go in the
	// same transaction as the source; the earlier share survives.
	source2 := createNote(owner, "Synthetic second source body.")
	summary2 := createSummary(owner, source2, "Synthetic second summary body.")
	summary2ID := summary2["id"].(string)
	followup2 := createFollowup(owner, summary2ID, "Synthetic second follow-up body.")
	share2, _, _, err := e.store.CreateShare(bg, ownerID, summary2ID, "Dr. Delete", "consultation", 24)
	if err != nil {
		t.Fatal(err)
	}

	foreignSource := createNote(other, "Synthetic foreign source body.")
	foreignSummary := createSummary(other, foreignSource, "Synthetic foreign summary body.")

	owner.must(200, "DELETE", "/api/v3/records/"+source2["id"].(string)+"?derived=delete",
		map[string]any{"expectedVersion": source2["version"]})
	owner.must(404, "GET", "/api/v3/consultation-summaries/"+summary2ID, nil)
	owner.must(404, "GET", "/api/v3/records/"+followup2["id"].(string), nil)
	if n := e.count("SELECT count(*) FROM shares WHERE id=$1", share2.ID); n != 0 {
		t.Fatalf("derived share rows: %d", n)
	}
	if n := e.count("SELECT count(*) FROM personal_records WHERE id=$1", summary2ID); n != 0 {
		t.Fatalf("derived summary rows: %d", n)
	}
	other.must(200, "GET", "/api/v3/consultation-summaries/"+foreignSummary["id"].(string), nil)
	shares = owner.must(200, "GET", "/api/v3/shares", nil).data()["shares"].([]any)
	if len(shares) != 1 || shares[0].(map[string]any)["id"] != share.ID {
		t.Fatalf("surviving shares: %v", shares)
	}

	// The default mode is keep.
	source3 := createNote(owner, "Synthetic default source body.")
	summary3 := createSummary(owner, source3, "Synthetic default summary body.")
	owner.must(200, "DELETE", "/api/v3/records/"+source3["id"].(string),
		map[string]any{"expectedVersion": source3["version"]})
	kept3 := owner.must(200, "GET", "/api/v3/consultation-summaries/"+summary3["id"].(string), nil).data()
	if kept3["summaryState"] != "draft" {
		t.Fatalf("default keep state: %v", kept3)
	}
}
