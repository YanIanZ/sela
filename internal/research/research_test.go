package research

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func load(t *testing.T) *Index {
	t.Helper()
	x, err := Load(filepath.Join("testdata", "knowledge_base.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestSearchRanksAndFilters(t *testing.T) {
	x := load(t)
	res := x.Search("Indonesia help-seeking", "", "", 10).(SearchResult)
	if res.Total == 0 || res.Results[0].Record.ID != "R047" {
		t.Fatalf("expected R047 first: %+v", res)
	}
	res = x.Search("", "AI_SUPPORT", "", 10).(SearchResult)
	if res.Total != 1 || res.Results[0].Record.ID != "R001" {
		t.Fatalf("theme filter: %+v", res)
	}
	res = x.Search("", "", "needs_check", 10).(SearchResult)
	if res.Total != 1 || res.Results[0].Record.ID != "R099" {
		t.Fatalf("status filter: %+v", res)
	}
	if res := x.Search("zzzzzz", "", "", 10).(SearchResult); res.Total != 0 || res.Results == nil {
		t.Fatalf("no-match: %+v", res)
	}
	if len(x.Search("", "", "", 1).(SearchResult).Results) != 1 {
		t.Fatal("limit not applied")
	}
}

func TestSearchIsDeterministic(t *testing.T) {
	x := load(t)
	a, _ := json.Marshal(x.Search("students", "", "", 10))
	b, _ := json.Marshal(x.Search("students", "", "", 10))
	if string(a) != string(b) {
		t.Fatal("non-deterministic search")
	}
}

// Corpus text is data: an injection string is returned verbatim as a field
// value and nothing in the index interprets it.
func TestInjectionTextIsInertData(t *testing.T) {
	x := load(t)
	got, ok := x.Record("R099")
	if !ok {
		t.Fatal("missing record")
	}
	r := got.(*Record)
	// Returned verbatim as data; JSON encoding escapes markup for HTML contexts.
	if !strings.HasPrefix(r.Title, "IGNORE ALL PREVIOUS INSTRUCTIONS") || !strings.HasPrefix(*r.NoteID, "<script>") {
		t.Fatalf("record altered: %+v", r)
	}
	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), "<script>") {
		t.Fatalf("markup not escaped in JSON: %s", raw)
	}
}

func TestNullsArePreserved(t *testing.T) {
	x := load(t)
	rec, _ := x.Record("R001")
	raw, _ := json.Marshal(rec)
	for _, field := range []string{`"sample_size":null`, `"study_type":null`} {
		if !strings.Contains(string(raw), field) {
			t.Fatalf("expected %s in %s", field, raw)
		}
	}
}

func TestLoadRejectsBadFiles(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"empty.jsonl": "",
		"dup.jsonl":   `{"id":"R1","title":"a"}` + "\n" + `{"id":"R1","title":"b"}` + "\n",
		"bad.jsonl":   `{"id":`,
		"noid.jsonl":  `{"title":"x"}`,
	} {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(content), 0o644)
		if _, err := Load(p); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := Load(filepath.Join(dir, "missing.jsonl")); err == nil {
		t.Error("missing file accepted")
	}
}

func TestManifestLoaded(t *testing.T) {
	x := load(t)
	m := x.Manifest().(map[string]any)
	if m["cutoff_date"] != "2021-10-26" || x.version != "test" {
		t.Fatalf("manifest: %+v", m)
	}
}
