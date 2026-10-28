package httpapi_test

import "testing"

func TestV3RecordsOwnedStorageAndReload(t *testing.T) {
	e := newV3(t, v3opt{})
	a := e.client()
	b := e.client()
	a.guest("18_plus")
	b.guest("18_plus")
	body := map[string]any{"kind": "note", "text": "Synthetic private note about a busy week.", "storageGranted": false}
	a.must(403, "POST", "/api/v3/records", body)
	body["storageGranted"] = true
	r := a.must(201, "POST", "/api/v3/records", body).data()
	id := r["id"].(string)
	b.must(404, "GET", "/api/v3/records/"+id, nil)
	b.must(404, "DELETE", "/api/v3/records/"+id, map[string]any{"expectedVersion": 1})
	a.must(200, "GET", "/api/v3/records", nil)
	a.must(200, "DELETE", "/api/v3/records/"+id, map[string]any{"expectedVersion": 1})
	a.must(404, "GET", "/api/v3/records/"+id, nil)
}
