package httpapi_test

import "testing"

func TestV3HelpContent(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{noStore: true})
	r := e.client().call("GET", "/api/v3/help-content", nil)
	if r.status != 200 {
		t.Fatalf("status %d: %s", r.status, r.raw)
	}
	data := r.data()
	for key, want := range map[string]int{"selfHelpCards": 30, "helpDestinations": 4, "supportOptions": 6} {
		list, ok := data[key].([]any)
		if !ok || len(list) != want {
			t.Fatalf("%s = %T with %d entries, want %d", key, data[key], len(list), want)
		}
	}
}
