package httpapi_test

import "testing"

func TestV3ReadingIsPublic(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{noStore: true})
	r := e.client().call("GET", "/api/v3/reading", nil)
	if r.status != 200 {
		t.Fatalf("list status %d: %s", r.status, r.raw)
	}
	if _, ok := r.data()["items"].([]any); !ok {
		t.Fatalf("items is %T, want an array: %s", r.data()["items"], r.raw)
	}
	r = e.client().call("GET", "/api/v3/reading/tidak-ada-di-pustaka", nil)
	if r.status != 404 || r.code() != "not_found" {
		t.Fatalf("unknown slug: %d %s", r.status, r.raw)
	}
}
