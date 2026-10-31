package reading

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

func review(over map[string]any) map[string]any {
	r := map[string]any{
		"status":               "human_approved",
		"human_editor":         "Editor Uji",
		"reviewed_on":          "2026-11-01",
		"approved_text_sha256": strings.Repeat("a", 64),
		"teen_review":          "not_applicable",
		"verifier":             map[string]string{"status": "supported", "note": "checked"},
		"publishable":          true,
	}
	for k, v := range over {
		if v == nil {
			delete(r, k)
			continue
		}
		r[k] = v
	}
	return r
}

func fixtureItem(id string, r map[string]any) map[string]any {
	return map[string]any{
		"id":       id,
		"version":  1,
		"kind":     "exercise",
		"topics":   []string{"stress"},
		"audience": "adult",
		"content": map[string]any{
			"title":   map[string]string{"id": "Judul " + id, "en": "Title " + id},
			"summary": map[string]string{"id": "Ringkasan " + id, "en": "Summary " + id},
			"steps":   map[string]any{"id": []string{"Langkah satu"}, "en": []string{"Step one"}},
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
		"review":          r,
	}
}

func fixtureFS(items ...map[string]any) fstest.MapFS {
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
	return fstest.MapFS{
		"set.json":     {Data: set},
		"sources.json": {Data: sources},
		"cards.json":   {Data: cards},
	}
}

func TestApprovedFixtureIsServed(t *testing.T) {
	t.Parallel()
	lib, err := LoadFS(fixtureFS(fixtureItem("approved-one", review(nil))))
	if err != nil {
		t.Fatal(err)
	}
	list := lib.List()
	if len(list) != 1 || list[0].ID != "approved-one" {
		t.Fatalf("list = %+v, want the approved fixture", list)
	}
	if list[0].Title.EN != "Title approved-one" || list[0].ReviewedOn != "2026-11-01" {
		t.Fatalf("list item lost its content or review date: %+v", list[0])
	}
	detail, ok := lib.Get("approved-one")
	if !ok {
		t.Fatal("approved fixture is absent from detail")
	}
	if detail.Steps == nil || len(detail.Steps.EN) != 1 || detail.Source.Relation != "sela_original" {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.Review.Status != "human_approved" || detail.Review.Verifier != "supported" {
		t.Fatalf("review = %+v", detail.Review)
	}
	if len(detail.Source.Sources) != 1 || detail.Source.Sources[0].Publisher != "Publisher" {
		t.Fatalf("sources = %+v", detail.Source.Sources)
	}
}

func TestUnpublishedItemsAreHidden(t *testing.T) {
	t.Parallel()
	cases := map[string]map[string]any{
		"changes requested": review(map[string]any{"status": "human_changes_requested", "publishable": false}),
		"not publishable":   review(map[string]any{"publishable": false}),
		"not reviewed":      review(map[string]any{"status": "machine_drafted_unreviewed", "publishable": false}),
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			lib, err := LoadFS(fixtureFS(fixtureItem("hidden", r)))
			if err != nil {
				t.Fatal(err)
			}
			if got := lib.List(); len(got) != 0 {
				t.Fatalf("list = %+v, want empty", got)
			}
			if _, ok := lib.Get("hidden"); ok {
				t.Fatal("unpublished item is served on detail")
			}
		})
	}
}

func TestForgedPublishableItemsAreRejected(t *testing.T) {
	t.Parallel()
	cases := map[string]map[string]any{
		"no editor":            review(map[string]any{"human_editor": nil}),
		"blank editor":         review(map[string]any{"human_editor": "  "}),
		"no review date":       review(map[string]any{"reviewed_on": nil}),
		"invalid review date":  review(map[string]any{"reviewed_on": "1 November 2026"}),
		"verifier not run":     review(map[string]any{"verifier": map[string]string{"status": "not_run", "note": "not checked"}}),
		"verifier unsupported": review(map[string]any{"verifier": map[string]string{"status": "unsupported", "note": "contradicted"}}),
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			lib, err := LoadFS(fixtureFS(fixtureItem("forged", r)))
			if err != nil {
				t.Fatal(err)
			}
			if got := lib.List(); len(got) != 0 {
				t.Fatalf("forged item appears in list: %+v", got)
			}
			if _, ok := lib.Get("forged"); ok {
				t.Fatal("forged item is served on detail")
			}
		})
	}
}

func TestMissingAndUnknownSlugsAreAbsent(t *testing.T) {
	t.Parallel()
	lib, err := LoadFS(fixtureFS(fixtureItem("approved-one", review(nil))))
	if err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"", "unknown", "approved-one "} {
		if _, ok := lib.Get(slug); ok {
			t.Fatalf("Get(%q) unexpectedly served an item", slug)
		}
	}
}

func TestDuplicateIDsAreRejected(t *testing.T) {
	t.Parallel()
	_, err := LoadFS(fixtureFS(fixtureItem("same", review(nil)), fixtureItem("same", review(nil))))
	if err == nil {
		t.Fatal("duplicate ids loaded without an error")
	}
}

func TestEmbeddedContentSetLoads(t *testing.T) {
	t.Parallel()
	lib := Embedded()
	if lib == nil {
		t.Fatal("embedded library is nil")
	}
	// The gate, not the parser, decides what is served. Every returned item
	// must therefore carry a complete approval record.
	for _, it := range lib.List() {
		if it.ID == "" || it.ReviewedOn == "" {
			t.Fatalf("embedded list item is incomplete: %+v", it)
		}
		if _, ok := lib.Get(it.ID); !ok {
			t.Fatalf("listed item %q is absent from detail", it.ID)
		}
	}
}
