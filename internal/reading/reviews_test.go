package reading

import (
	"encoding/json"
	"testing"
	"testing/fstest"
)

func unreviewedItem(id string) map[string]any {
	return fixtureItem(id, review(map[string]any{
		"status":       "machine_drafted_unreviewed",
		"human_editor": nil,
		"reviewed_on":  nil,
		"verifier":     map[string]string{"status": "not_run", "note": "not checked"},
		"publishable":  false,
	}))
}

func approvalFor(t *testing.T, lib *Library, slug string) Approval {
	t.Helper()
	hash, ok := lib.ContentHash(slug)
	if !ok {
		t.Fatalf("no content hash for %q", slug)
	}
	return Approval{
		Slug:           slug,
		ReviewerName:   "Editor Manusia",
		ReviewedOn:     "2026-11-01",
		VerifierStatus: "supported",
		ContentHash:    hash,
		Version:        1,
	}
}

func TestContentHashBindsTheExactItemBytes(t *testing.T) {
	t.Parallel()
	lib, err := LoadFS(fixtureFS(unreviewedItem("stable")))
	if err != nil {
		t.Fatal(err)
	}
	hash, ok := lib.ContentHash("stable")
	if !ok || len(hash) != 64 {
		t.Fatalf("hash = %q, %t", hash, ok)
	}
	// Reformatting the same value is a different stored text: the hash must
	// change, which is what invalidates an approval.
	set := []byte(`{"version":"1.0.0","files":[{"path":"cards.json"}]}`)
	sources := []byte(`{"sources":[]}`)
	compact, _ := json.Marshal(map[string]any{"items": []map[string]any{unreviewedItem("stable")}})
	indented, _ := json.MarshalIndent(map[string]any{"items": []map[string]any{unreviewedItem("stable")}}, "", "  ")
	lib2, err := LoadFS(fstest.MapFS{
		"set.json":     {Data: set},
		"sources.json": {Data: sources},
		"cards.json":   {Data: compact},
	})
	if err != nil {
		t.Fatal(err)
	}
	lib3, err := LoadFS(fstest.MapFS{
		"set.json":     {Data: set},
		"sources.json": {Data: sources},
		"cards.json":   {Data: indented},
	})
	if err != nil {
		t.Fatal(err)
	}
	hash2, _ := lib2.ContentHash("stable")
	hash3, _ := lib3.ContentHash("stable")
	if hash != hash2 {
		t.Fatal("the same compact JSON produced different hashes")
	}
	if hash == hash3 {
		t.Fatal("a reformatted item kept the same hash")
	}
}

func TestOperatorApprovalPublishesOnlyMatchingSupportedContent(t *testing.T) {
	t.Parallel()
	lib, err := LoadFS(fixtureFS(unreviewedItem("draft-one")))
	if err != nil {
		t.Fatal(err)
	}
	if got := lib.ListWith(nil); len(got) != 0 {
		t.Fatalf("unreviewed item is public: %+v", got)
	}
	approval := approvalFor(t, lib, "draft-one")
	approvals := map[string]Approval{"draft-one": approval}

	list := lib.ListWith(approvals)
	if len(list) != 1 || list[0].ID != "draft-one" || list[0].ReviewedOn != "2026-11-01" {
		t.Fatalf("list = %+v", list)
	}
	detail, ok := lib.GetWith("draft-one", approvals)
	if !ok {
		t.Fatal("approved draft is absent from detail")
	}
	if detail.Review.Status != StatusHumanApproved || detail.Review.ReviewedOn != "2026-11-01" || detail.Review.Verifier != "supported" {
		t.Fatalf("review = %+v", detail.Review)
	}
	if detail.Steps == nil {
		t.Fatal("detail lost its text")
	}

	cases := map[string]func(*Approval){
		"revoked": func(a *Approval) {
			revoked := a.ApprovedAt.Add(1)
			a.RevokedAt = &revoked
		},
		"content changed": func(a *Approval) {
			last := byte('0')
			if a.ContentHash[len(a.ContentHash)-1] == '0' {
				last = '1'
			}
			a.ContentHash = a.ContentHash[:len(a.ContentHash)-1] + string(last)
		},
		"verifier not supported": func(a *Approval) { a.VerifierStatus = "not_run" },
		"blank reviewer":         func(a *Approval) { a.ReviewerName = "  " },
		"invalid date":           func(a *Approval) { a.ReviewedOn = "1 November 2026" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			a := approval
			mutate(&a)
			if got := lib.ListWith(map[string]Approval{"draft-one": a}); len(got) != 0 {
				t.Fatalf("list = %+v, want empty", got)
			}
			if _, ok := lib.GetWith("draft-one", map[string]Approval{"draft-one": a}); ok {
				t.Fatal("detail served an invalid approval")
			}
		})
	}
}

func TestRevocationVetoesEmbeddedApproval(t *testing.T) {
	t.Parallel()
	lib, err := LoadFS(fixtureFS(fixtureItem("embedded-approved", review(nil))))
	if err != nil {
		t.Fatal(err)
	}
	if got := lib.List(); len(got) != 1 {
		t.Fatalf("embedded approval not served: %+v", got)
	}
	a := approvalFor(t, lib, "embedded-approved")
	revoked := a.ApprovedAt.Add(1)
	a.RevokedAt = &revoked
	approvals := map[string]Approval{"embedded-approved": a}
	if got := lib.ListWith(approvals); len(got) != 0 {
		t.Fatalf("revoked item stayed public: %+v", got)
	}
	if _, ok := lib.GetWith("embedded-approved", approvals); ok {
		t.Fatal("revoked item stayed on detail")
	}
}

func TestAdminItemsShowEveryItemAndHashMatch(t *testing.T) {
	t.Parallel()
	lib, err := LoadFS(fixtureFS(unreviewedItem("draft-one"), fixtureItem("embedded-approved", review(nil))))
	if err != nil {
		t.Fatal(err)
	}
	items := lib.AdminItems(nil)
	if len(items) != 2 {
		t.Fatalf("admin items = %d, want every embedded item", len(items))
	}
	for _, it := range items {
		if it.Published != (it.Detail.ID == "embedded-approved") {
			t.Fatalf("unapproved admin row %q published=%t", it.Detail.ID, it.Published)
		}
	}
	approval := approvalFor(t, lib, "draft-one")
	items = lib.AdminItems(map[string]Approval{"draft-one": approval})
	byID := map[string]AdminItem{}
	for _, it := range items {
		byID[it.Detail.ID] = it
	}
	draft := byID["draft-one"]
	if !draft.Published || !draft.HashMatches || draft.Approval == nil {
		t.Fatalf("draft row = %+v", draft)
	}
	if draft.EmbeddedReview.Status != "machine_drafted_unreviewed" || draft.EmbeddedReview.HumanEditor != "" {
		t.Fatalf("embedded review = %+v", draft.EmbeddedReview)
	}
	if draft.Detail.Title.EN != "Title draft-one" || draft.Detail.Summary.ID == "" {
		t.Fatalf("admin preview lost content: %+v", draft.Detail)
	}
	embedded := byID["embedded-approved"]
	if !embedded.Published || embedded.Approval != nil {
		t.Fatalf("embedded-approved row = %+v", embedded)
	}

	stale := approval
	stale.ContentHash = stale.ContentHash[:len(stale.ContentHash)-1] + "0"
	if stale.ContentHash == approval.ContentHash {
		stale.ContentHash = stale.ContentHash[:len(stale.ContentHash)-1] + "1"
	}
	for _, it := range lib.AdminItems(map[string]Approval{"draft-one": stale}) {
		if it.Detail.ID == "draft-one" && (it.HashMatches || it.Published) {
			t.Fatalf("stale approval reported as valid: %+v", it)
		}
	}
}
