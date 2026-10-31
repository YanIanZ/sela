package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/YanIanZ/sela/internal/reading"
)

func readingReview(status string, publishable bool, editor, reviewedOn, verifier string) map[string]any {
	var editorValue any
	if editor != "" {
		editorValue = editor
	}
	var dateValue any
	if reviewedOn != "" {
		dateValue = reviewedOn
	}
	return map[string]any{
		"status":               status,
		"human_editor":         editorValue,
		"reviewed_on":          dateValue,
		"approved_text_sha256": strings.Repeat("a", 64),
		"teen_review":          "not_applicable",
		"verifier":             map[string]string{"status": verifier, "note": "fixture"},
		"publishable":          publishable,
	}
}

func readingItem(id string, review map[string]any) map[string]any {
	return map[string]any{
		"id":       id,
		"version":  1,
		"kind":     "exercise",
		"topics":   []string{"stress"},
		"audience": "adult",
		"content": map[string]any{
			"title":   map[string]string{"id": "Judul " + id, "en": "Title " + id},
			"summary": map[string]string{"id": "Ringkasan " + id, "en": "Summary " + id},
			"steps":   map[string]any{"id": []string{"Satu"}, "en": []string{"One"}},
		},
		"reading_time_minutes": map[string]int{"id": 1, "en": 1},
		"source": map[string]any{
			"source_ids": []string{"src-1"},
			"relation":   "sela_original",
			"locator":    "Whole item",
		},
		"licence": map[string]any{
			"source_licence":             "MIT",
			"item_text_licence":          "MIT",
			"commercial_use":             "allowed",
			"share_alike":                false,
			"attribution_required":       true,
			"adaptation_notice_required": false,
		},
		"attribution":     map[string]string{"id": "Sela", "en": "Sela"},
		"adaptation_note": "Written for Sela.",
		"review":          review,
	}
}

func readingFixture(t *testing.T) *reading.Library {
	t.Helper()
	items := []map[string]any{
		readingItem("approved-one", readingReview("human_approved", true, "Editor Uji", "2026-11-01", "supported")),
		readingItem("unpublished", readingReview("machine_drafted_unreviewed", false, "", "", "not_run")),
		readingItem("forged", readingReview("human_approved", true, "", "", "not_run")),
	}
	set, _ := json.Marshal(map[string]any{
		"version": "1.0.0",
		"files":   []map[string]any{{"path": "cards.json"}},
	})
	sources, _ := json.Marshal(map[string]any{
		"sources": []map[string]string{{
			"id": "src-1", "title": "Source one", "publisher": "Publisher", "url": "https://example.org/source", "licence": "MIT",
		}},
	})
	cards, _ := json.Marshal(map[string]any{"items": items})
	lib, err := reading.LoadFS(fstest.MapFS{
		"set.json":     {Data: set},
		"sources.json": {Data: sources},
		"cards.json":   {Data: cards},
	})
	if err != nil {
		t.Fatal(err)
	}
	return lib
}

func readingGet(t *testing.T, s *Server, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestV3ReadingGateServesOnlyApprovedFixture(t *testing.T) {
	t.Parallel()
	s := New(nil, Options{})
	s.v3.readingLib = readingFixture(t)

	rec, body := readingGet(t, s, "/api/v3/reading")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	data, _ := body["data"].(map[string]any)
	items, _ := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %v, want only the approved fixture", items)
	}
	first, _ := items[0].(map[string]any)
	if first["id"] != "approved-one" {
		t.Fatalf("served %v, want approved-one", first["id"])
	}
	if strings.Contains(rec.Body.String(), "Editor Uji") {
		t.Fatal("the reviewer's name leaked into the public response")
	}

	rec, body = readingGet(t, s, "/api/v3/reading/approved-one")
	if rec.Code != http.StatusOK {
		t.Fatalf("approved detail: %d %s", rec.Code, rec.Body)
	}
	data, _ = body["data"].(map[string]any)
	if data["id"] != "approved-one" || data["title"].(map[string]any)["en"] != "Title approved-one" {
		t.Fatalf("detail = %v", data)
	}
	review, _ := data["review"].(map[string]any)
	if review["status"] != "human_approved" || review["reviewedOn"] != "2026-11-01" || review["verifier"] != "supported" {
		t.Fatalf("review = %v", review)
	}

	for _, slug := range []string{"unpublished", "forged"} {
		rec, body = readingGet(t, s, "/api/v3/reading/"+slug)
		errBody, _ := body["error"].(map[string]any)
		if rec.Code != http.StatusNotFound || errBody["code"] != "not_found" {
			t.Fatalf("%s: %d %s, want 404 not_found", slug, rec.Code, rec.Body)
		}
	}
}
