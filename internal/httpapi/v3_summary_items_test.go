package httpapi_test

import (
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/account"
)

func summaryItemsV3(t *testing.T) *v3env {
	t.Helper()
	return newV3(t, v3opt{mutateC: func(c *account.Config) {
		c.Purposes = append(c.Purposes, account.Purpose{Name: "sharing", Kinds: []string{account.KindGuest, account.KindAccount}})
	}})
}

func summaryItemBody(noteID string, noteVersion int, text, quote string) map[string]any {
	return map[string]any{
		"text": text, "quote": quote,
		"sourceRef": map[string]any{"recordId": noteID, "version": noteVersion},
	}
}

// TestV3SummaryItemsLifecycle covers add/edit/remove through the public
// routes, the approval reset and the statement-aware export.
func TestV3SummaryItemsLifecycle(t *testing.T) {
	e := summaryItemsV3(t)
	a := e.client()
	b := e.client()
	a.guest("18_plus")
	b.guest("18_plus")

	note := a.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Synthetic item source", "text": "Synthetic item source text.", "storageGranted": true,
	}).data()
	noteID, noteVersion := note["id"].(string), int(note["version"].(float64))
	sourceRefs := []map[string]any{{"recordId": noteID, "version": noteVersion}}
	created := a.must(201, "POST", "/api/v3/consultation-summaries", map[string]any{
		"title": "Synthetic item summary", "text": "", "storageGranted": true, "sourceRefs": sourceRefs,
		"items": []map[string]any{
			summaryItemBody(noteID, noteVersion, "Synthetic statement one.", "Synthetic quote one."),
			summaryItemBody(noteID, noteVersion, "Synthetic statement two.", "Synthetic quote two."),
		},
	}).data()
	id := created["id"].(string)
	items, ok := created["items"].([]any)
	if !ok || len(items) != 2 || created["summaryState"] != "draft" || created["text"] != "" {
		t.Fatalf("create with items: %v", created)
	}
	firstItem := items[0].(map[string]any)
	if firstItem["id"] == "" || firstItem["position"] != float64(0) ||
		firstItem["provenance"] != "user_original" || firstItem["reviewStatus"] != "confirmed" {
		t.Fatalf("normalized item: %v", firstItem)
	}
	firstID := firstItem["id"].(string)

	a.must(200, "POST", "/api/v3/consultation-summaries/"+id+"/approve", map[string]any{"expectedVersion": 1})
	correctedItem := summaryItemBody(noteID, noteVersion, "Synthetic corrected statement one.", "Synthetic quote one.")
	correctedItem["id"] = firstID
	edited := a.must(200, "PATCH", "/api/v3/consultation-summaries/"+id, map[string]any{
		"title": "Synthetic item summary", "text": "", "expectedVersion": 1, "storageGranted": true,
		"sourceRefs": sourceRefs,
		"items": []map[string]any{
			correctedItem,
			summaryItemBody(noteID, noteVersion, "Synthetic statement three.", "Synthetic quote three."),
		},
	}).data()
	if edited["version"] != float64(2) || edited["summaryState"] != "draft" {
		t.Fatalf("item edit must reset approval: %v", edited)
	}
	if r := a.call("POST", "/api/v3/consultation-summaries/"+id+"/approve", map[string]any{"expectedVersion": 1}); r.status != 409 || r.code() != "stale_version" {
		t.Fatalf("approve stale item version: %d %s", r.status, r.raw)
	}
	editedItems := edited["items"].([]any)
	if editedItems[0].(map[string]any)["id"] != firstID ||
		editedItems[0].(map[string]any)["provenance"] != "user_correction" ||
		editedItems[1].(map[string]any)["provenance"] != "user_original" {
		t.Fatalf("stable id or provenance: %v", editedItems)
	}
	keptID := editedItems[0].(map[string]any)["id"].(string)
	corrected := a.must(200, "PATCH", "/api/v3/consultation-summaries/"+id, map[string]any{
		"title": "Synthetic item summary", "text": "", "expectedVersion": 2, "storageGranted": true,
		"sourceRefs": sourceRefs,
		"items": []map[string]any{
			func() map[string]any {
				item := summaryItemBody(noteID, noteVersion, "Synthetic corrected statement one.", "Synthetic quote one.")
				item["id"] = keptID
				return item
			}(),
			summaryItemBody(noteID, noteVersion, "Synthetic statement four.", "Synthetic quote four."),
		},
	}).data()
	correctedItems := corrected["items"].([]any)
	if corrected["version"] != float64(3) || len(correctedItems) != 2 ||
		correctedItems[0].(map[string]any)["id"] != keptID ||
		correctedItems[0].(map[string]any)["provenance"] != "user_correction" ||
		correctedItems[1].(map[string]any)["provenance"] != "user_original" {
		t.Fatalf("kept item provenance: %v", correctedItems)
	}
	a.must(200, "POST", "/api/v3/consultation-summaries/"+id+"/approve", map[string]any{"expectedVersion": 3})

	htmlExport := a.call("GET", "/api/v3/consultation-summaries/"+id+"/export?format=html&lang=en", nil)
	htmlBody := string(htmlExport.raw)
	if htmlExport.status != 200 || !strings.Contains(htmlBody, "Statements from your sources") ||
		!strings.Contains(htmlBody, "Synthetic corrected statement one.") ||
		!strings.Contains(htmlBody, "Synthetic quote one.") ||
		!strings.Contains(htmlBody, "Synthetic item source · v1") {
		t.Fatalf("html items export: %d %s", htmlExport.status, htmlBody)
	}
	mdExport := a.call("GET", "/api/v3/consultation-summaries/"+id+"/export?format=md&lang=en", nil)
	mdBody := string(mdExport.raw)
	if mdExport.status != 200 || !strings.Contains(mdBody, "## Statements from your sources") ||
		!strings.Contains(mdBody, "> Synthetic quote one.") ||
		!strings.Contains(mdBody, "Synthetic corrected statement one.") ||
		!strings.Contains(mdBody, "_Synthetic item source · v1_") {
		t.Fatalf("markdown items export: %d %s", mdExport.status, mdBody)
	}

	// The generic record route still refuses items.
	if r := a.call("POST", "/api/v3/records", map[string]any{
		"kind": "note", "text": "Synthetic note.", "storageGranted": true,
		"items": []map[string]any{summaryItemBody(noteID, noteVersion, "Synthetic statement.", "")},
	}); r.status != 400 || r.field() != "items" {
		t.Fatalf("items on a note: %d %s", r.status, r.raw)
	}

	// Owner isolation covers item edits and exports.
	b.must(404, "GET", "/api/v3/consultation-summaries/"+id, nil)
	if r := b.call("PATCH", "/api/v3/consultation-summaries/"+id, map[string]any{
		"title": "Synthetic foreign", "text": "", "expectedVersion": 3, "storageGranted": true,
		"sourceRefs": sourceRefs,
	}); r.status != 404 {
		t.Fatalf("foreign item edit: %d %s", r.status, r.raw)
	}
	if r := b.call("GET", "/api/v3/consultation-summaries/"+id+"/export?format=html", nil); r.status != 404 {
		t.Fatalf("foreign item export: %d %s", r.status, r.raw)
	}
}

// TestV3SummaryItemsValidation exposes the stable field for bad statement
// input and the source-pin rule.
func TestV3SummaryItemsValidation(t *testing.T) {
	e := summaryItemsV3(t)
	a := e.client()
	b := e.client()
	a.guest("18_plus")
	b.guest("18_plus")

	note := a.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Synthetic item source", "text": "Synthetic item source text.", "storageGranted": true,
	}).data()
	noteID, noteVersion := note["id"].(string), int(note["version"].(float64))
	foreign := b.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "text": "Synthetic foreign source.", "storageGranted": true,
	}).data()
	base := map[string]any{
		"title": "Synthetic item summary", "text": "", "storageGranted": true,
		"sourceRefs": []map[string]any{{"recordId": noteID, "version": noteVersion}},
	}
	withItems := func(items ...map[string]any) map[string]any {
		body := map[string]any{}
		for k, v := range base {
			body[k] = v
		}
		body["items"] = items
		return body
	}

	for name, body := range map[string]map[string]any{
		"blank wording":    withItems(summaryItemBody(noteID, noteVersion, "  ", "Synthetic quote.")),
		"unpinned source":  withItems(summaryItemBody(foreign["id"].(string), int(foreign["version"].(float64)), "Synthetic statement.", "")),
		"stale pin":        withItems(summaryItemBody(noteID, noteVersion+1, "Synthetic statement.", "")),
		"missing source":   withItems(map[string]any{"text": "Synthetic statement.", "quote": ""}),
		"1201-rune quote":  withItems(summaryItemBody(noteID, noteVersion, "Synthetic statement.", strings.Repeat("a", account.SummaryItemQuoteMax+1))),
		"801-rune wording": withItems(summaryItemBody(noteID, noteVersion, strings.Repeat("a", account.SummaryItemTextMax+1), "")),
		"AI provenance": withItems(func() map[string]any {
			i := summaryItemBody(noteID, noteVersion, "Synthetic statement.", "")
			i["provenance"] = "ai_summary"
			return i
		}()),
		"unreviewed item": withItems(func() map[string]any {
			i := summaryItemBody(noteID, noteVersion, "Synthetic statement.", "")
			i["reviewStatus"] = "unreviewed"
			return i
		}()),
		"empty text": {"title": "Synthetic empty", "text": "", "storageGranted": true, "sourceRefs": base["sourceRefs"]},
		"unknown item field": withItems(func() map[string]any {
			i := summaryItemBody(noteID, noteVersion, "Synthetic statement.", "")
			i["mood"] = "x"
			return i
		}()),
	} {
		r := a.call("POST", "/api/v3/consultation-summaries", body)
		if r.status != 400 {
			t.Fatalf("%s: %d %s", name, r.status, r.raw)
		}
		wantField := "items"
		if name == "empty text" {
			wantField = "text"
		} else if name == "unknown item field" {
			wantField = "body"
		}
		if r.field() != wantField {
			t.Fatalf("%s field: want %s, got %s (%s)", name, wantField, r.field(), r.raw)
		}
	}
	var tooMany []map[string]any
	for i := 0; i < account.MaxSummaryItems+1; i++ {
		tooMany = append(tooMany, summaryItemBody(noteID, noteVersion, "Synthetic statement.", ""))
	}
	if r := a.call("POST", "/api/v3/consultation-summaries", withItems(tooMany...)); r.status != 400 || r.field() != "items" {
		t.Fatalf("37 items: %d %s", r.status, r.raw)
	}

	// A foreign source added to both the summary and its item is a
	// not-found source, not a statement error.
	body := withItems(summaryItemBody(foreign["id"].(string), int(foreign["version"].(float64)), "Synthetic statement.", ""))
	body["sourceRefs"] = []map[string]any{
		{"recordId": noteID, "version": noteVersion},
		{"recordId": foreign["id"].(string), "version": int(foreign["version"].(float64))},
	}
	if r := a.call("POST", "/api/v3/consultation-summaries", body); r.status != 404 || r.code() != "not_found" {
		t.Fatalf("foreign source ref: %d %s", r.status, r.raw)
	}
}

// TestV3SummaryItemsShareProjection confirms the frozen recipient view
// carries each statement, its quote and its source label, while a legacy
// text-only summary freezes exactly as before.
func TestV3SummaryItemsShareProjection(t *testing.T) {
	e := summaryItemsV3(t)
	owner := e.client()
	owner.guest("18_plus")
	owner.must(200, "PUT", "/api/v3/me/consents/sharing", map[string]any{
		"granted": true, "policyVersion": policyVersion("sharing"),
	})
	note := owner.must(201, "POST", "/api/v3/records", map[string]any{
		"kind": "note", "title": "Synthetic item source", "text": "Synthetic item source text.", "storageGranted": true,
	}).data()
	noteID, noteVersion := note["id"].(string), int(note["version"].(float64))
	sourceRefs := []map[string]any{{"recordId": noteID, "version": noteVersion}}
	summary := owner.must(201, "POST", "/api/v3/consultation-summaries", map[string]any{
		"title": "Synthetic item summary", "text": "Synthetic free wording.", "storageGranted": true, "sourceRefs": sourceRefs,
		"items": []map[string]any{summaryItemBody(noteID, noteVersion, "Synthetic shared statement.", "Synthetic shared quote.")},
	}).data()
	summaryID := summary["id"].(string)
	owner.must(200, "POST", "/api/v3/consultation-summaries/"+summaryID+"/approve", map[string]any{"expectedVersion": 1})

	created := owner.must(201, "POST", "/api/v3/shares", map[string]any{
		"summaryId": summaryID, "label": "Dr. Synthetic", "purpose": "consultation", "durationHours": 24,
	}).data()
	token, code := created["token"].(string), created["code"].(string)
	recipient := e.client()
	projection := recipient.must(200, "POST", "/api/v3/recipient/session", map[string]any{"token": token, "code": code}).
		data()["share"].(map[string]any)
	items, ok := projection["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("shared items: %v", projection)
	}
	item := items[0].(map[string]any)
	if item["text"] != "Synthetic shared statement." || item["quote"] != "Synthetic shared quote." ||
		item["sourceLabel"] != "Synthetic item source · v1" || item["position"] != float64(0) {
		t.Fatalf("shared item: %v", item)
	}
	if _, leaked := item["id"]; leaked {
		t.Fatal("projection leaks the item id")
	}

	legacy := owner.must(201, "POST", "/api/v3/consultation-summaries", map[string]any{
		"title": "Synthetic legacy summary", "text": "Synthetic legacy text.", "storageGranted": true, "sourceRefs": sourceRefs,
	}).data()
	legacyID := legacy["id"].(string)
	owner.must(200, "POST", "/api/v3/consultation-summaries/"+legacyID+"/approve", map[string]any{"expectedVersion": 1})
	legacyShare := owner.must(201, "POST", "/api/v3/shares", map[string]any{
		"summaryId": legacyID, "label": "Dr. Legacy", "purpose": "consultation", "durationHours": 24,
	}).data()
	legacyRecipient := e.client()
	legacyProjection := legacyRecipient.must(200, "POST", "/api/v3/recipient/session", map[string]any{
		"token": legacyShare["token"], "code": legacyShare["code"],
	}).data()["share"].(map[string]any)
	if _, ok := legacyProjection["items"]; ok || legacyProjection["text"] != "Synthetic legacy text." {
		t.Fatalf("legacy projection grew items: %v", legacyProjection)
	}

	legacyExport := owner.call("GET", "/api/v3/consultation-summaries/"+legacyID+"/export?format=html&lang=en", nil)
	if legacyExport.status != 200 || strings.Contains(string(legacyExport.raw), "Statements from your sources") ||
		!strings.Contains(string(legacyExport.raw), "Synthetic legacy text.") {
		t.Fatalf("legacy export changed: %d %s", legacyExport.status, legacyExport.raw)
	}
}
