package httpapi_test

import (
	"bytes"
	"testing"

	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/care"
	"github.com/YanIanZ/sela/internal/httpapi"
)

func newCareEnv(t *testing.T) (*v3env, *admin.Store, *care.Store) {
	t.Helper()
	key := bytes.Repeat([]byte{13}, 32)
	var adminStore *admin.Store
	var careStore *care.Store
	e := newV3(t, v3opt{mutateO: func(e *v3env, o *httpapi.Options) {
		s, err := admin.New(e.db.Pool, admin.Options{Hasher: e.store.Hasher(), MFAKey: key, Now: e.clock.Now})
		if err != nil {
			t.Fatal(err)
		}
		c, err := care.New(e.db.Pool, e.clock.Now)
		if err != nil {
			t.Fatal(err)
		}
		adminStore, careStore = s, c
		o.Admin, o.Care = s, c
	}})
	return e, adminStore, careStore
}

func TestCareDirectoryEditorialFlowAndSandbox(t *testing.T) {
	e, st, _ := newCareEnv(t)
	author := e.client()
	adminLogin(t, e, st, author, "care.author", adminPassword, admin.RoleOwner)
	reviewer := e.client()
	adminLogin(t, e, st, reviewer, "care.reviewer", adminPassword, admin.RoleOwner)

	created := author.must(201, "POST", "/api/admin/v1/directory", map[string]any{
		"kind": "organization", "name": "Ruang Contoh", "city": "Bandung",
		"description": "Contoh sumber publik.", "contactUrl": "https://example.org/contact",
		"sourceUrl": "https://example.org/source", "sourcePublisher": "Sumber Contoh",
		"sourceAccessedAt": "2026-10-20T00:00:00Z",
	}).data()
	id := created["id"].(string)
	if created["status"] != "draft" || created["version"].(float64) != 1 {
		t.Fatalf("created = %v", created)
	}
	author.must(200, "POST", "/api/admin/v1/directory/"+id+"/submit", map[string]any{"expectedVersion": 1})
	if r := author.call("POST", "/api/admin/v1/directory/"+id+"/verify", map[string]any{"expectedVersion": 2}); r.status != 403 || r.code() != "directory_forbidden" {
		t.Fatalf("self verify: %d %s", r.status, r.raw)
	}
	verified := reviewer.must(200, "POST", "/api/admin/v1/directory/"+id+"/verify", map[string]any{"expectedVersion": 2}).data()
	if verified["status"] != "verified" || verified["reverifyAt"] == nil {
		t.Fatalf("verified = %v", verified)
	}
	public := e.client().must(200, "GET", "/api/v3/care-directory?q=Ruang", nil).data()
	entries, _ := public["entries"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["id"] != id {
		t.Fatalf("public = %v", public)
	}

	ops := e.client()
	adminLogin(t, e, st, ops, "care.ops", adminPassword, admin.RoleOps)
	ops.must(403, "GET", "/api/admin/v1/directory", nil)

	request := author.must(201, "POST", "/api/admin/v1/sandbox/requests", map[string]any{
		"organization": "Ruang Fiktif", "professional": "Praktisi Fiktif", "service": "Konseling", "note": "demo",
	}).data()
	rid := request["id"].(string)
	author.must(200, "POST", "/api/admin/v1/sandbox/requests/"+rid+"/transition", map[string]any{
		"expectedVersion": 1, "status": "pending", "note": "queued",
	})
	if r := author.call("POST", "/api/admin/v1/sandbox/requests/"+rid+"/transition", map[string]any{
		"expectedVersion": 2, "status": "closed",
	}); r.status != 409 {
		t.Fatalf("invalid sandbox transition: %d %s", r.status, r.raw)
	}
	audit := author.must(200, "GET", "/api/admin/v1/directory/audit", nil).data()["events"].([]any)
	if len(audit) < 5 {
		t.Fatalf("care audit = %v", audit)
	}
}
