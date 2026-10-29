package httpapi_test

import "testing"

func TestV3RoomsAreOrderedByNewestActivity(t *testing.T) {
	e := newV3(t, v3opt{})
	c := e.client()
	c.guest("18_plus")
	first := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()["id"].(string)
	c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false})
	// A newer turn in the older room makes it the one Home should continue.
	c.must(200, "POST", "/api/v3/conversations/"+first+"/turns", map[string]any{"text": "Aku ingin mengurai hari ini.", "expectedVersion": 1})
	list := c.must(200, "GET", "/api/v3/conversations", nil).data()["conversations"].([]any)
	if len(list) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(list))
	}
	if got := list[0].(map[string]any)["id"]; got != first {
		t.Fatalf("newest room is %v, want %s", got, first)
	}
}

func TestV27V33V38ListeningRoom(t *testing.T) {
	e := newV3(t, v3opt{})
	a := e.client()
	b := e.client()
	a.guest("18_plus")
	b.guest("18_plus")
	a.must(400, "POST", "/api/v3/conversations", map[string]any{})
	room := a.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()
	id := room["id"].(string)
	b.must(404, "GET", "/api/v3/conversations/"+id, nil)
	r := a.must(200, "POST", "/api/v3/conversations/"+id+"/turns", map[string]any{"text": "Aku mau bunuh diri malam ini.", "expectedVersion": 1}).data()
	if r["paused"] != true {
		t.Fatal("crisis must pause AI without losing text")
	}
	if e.count("SELECT count(*) FROM personal_records") != 0 {
		t.Fatal("unsaved conversation persisted")
	}
	a.must(200, "DELETE", "/api/v3/conversations/"+id, nil)
	a.must(404, "GET", "/api/v3/conversations/"+id, nil)
}

func TestV38SavedCrisisPauseSurvivesReopenAndV34DeletionEndsRoom(t *testing.T) {
	e := newV3(t, v3opt{})
	c := e.client()
	c.guest("18_plus")
	room := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": true}).data()
	id := room["id"].(string)
	next := c.must(200, "POST", "/api/v3/conversations/"+id+"/turns", map[string]any{"text": "Aku mau bunuh diri malam ini.", "expectedVersion": 1}).data()
	recordID := next["recordId"].(string)
	c.must(200, "DELETE", "/api/v3/conversations/"+id, nil)
	reopened := c.must(201, "POST", "/api/v3/conversations", map[string]any{"save": true, "recordId": recordID}).data()
	if reopened["paused"] != true {
		t.Fatal("reopening a saved conversation bypassed crisis pause")
	}
	record := c.must(200, "GET", "/api/v3/records/"+recordID, nil).data()
	c.must(200, "DELETE", "/api/v3/records/"+recordID, map[string]any{"expectedVersion": record["version"]})
	c.must(404, "GET", "/api/v3/conversations/"+reopened["id"].(string), nil)
}

func TestV3LogoutAllKeepsTheCurrentUnsavedRoom(t *testing.T) {
	e := newV3(t, v3opt{})
	a := e.client()
	a.register("ruang.sesi")
	b := e.client()
	b.login("ruang.sesi", v3Password)
	roomA := a.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false}).data()["id"].(string)
	b.must(201, "POST", "/api/v3/conversations", map[string]any{"save": false})
	a.must(200, "POST", "/api/v3/auth/logout-all", map[string]any{"keepCurrent": true})
	a.must(200, "GET", "/api/v3/conversations/"+roomA, nil)
	if r := b.call("GET", "/api/v3/conversations/"+"00000000-0000-4000-8000-000000000000", nil); r.status != 401 {
		t.Fatalf("other session stayed valid: %d", r.status)
	}
	a.must(200, "POST", "/api/v3/auth/logout-all", map[string]any{"keepCurrent": false})
	if r := a.call("GET", "/api/v3/conversations/"+roomA, nil); r.status != 401 {
		t.Fatalf("signed-out session reached its room: %d", r.status)
	}
}
