package dbtest_test

import (
	"context"
	"net/url"
	"regexp"
	"testing"

	"github.com/YanIanZ/sela/internal/db/dbtest"
)

func TestHelperDecide(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		url, require string
		want         dbtest.Action
	}{
		{"", "", dbtest.Skip},
		{"", "0", dbtest.Skip},
		{"", "1", dbtest.Fail},
		{"postgres://x", "", dbtest.Run},
		{"postgres://x", "1", dbtest.Run},
		{"  ", "1", dbtest.Fail},
	} {
		if got := dbtest.Decide(c.url, c.require); got != c.want {
			t.Errorf("Decide(%q, %q) = %v, want %v", c.url, c.require, got, c.want)
		}
	}
}

func TestHelperSchemaURL(t *testing.T) {
	t.Parallel()
	u, err := dbtest.SchemaURL("postgres://u:p@127.0.0.1:5432/db?sslmode=disable", "t_0011223344556677")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := url.Parse(u)
	if p.Query().Get("search_path") != "t_0011223344556677" || p.Query().Get("sslmode") != "disable" {
		t.Fatalf("schema URL %s", u)
	}
	if _, err := dbtest.SchemaURL("host=x dbname=y", "t_1"); err == nil {
		t.Fatal("key=value DSN accepted")
	}
}

// One schema per test, named t_<16 hex>, used as search_path, migrated by
// New, isolated from other tests and dropped on cleanup.
func TestHelperSchemaLifecycle(t *testing.T) {
	t.Parallel()
	dbtest.URL(t) // skip (or fail when required) before starting subtests
	var first, second string
	t.Run("a", func(t *testing.T) {
		t.Parallel()
		d := dbtest.New(t)
		first = d.Schema
		var current string
		if err := d.Pool.QueryRow(context.Background(), `SELECT current_schema()`).Scan(&current); err != nil {
			t.Fatal(err)
		}
		if current != d.Schema || !regexp.MustCompile(`^t_[0-9a-f]{16}$`).MatchString(d.Schema) {
			t.Fatalf("schema %q, current_schema %q", d.Schema, current)
		}
		if _, err := d.Pool.Exec(context.Background(), `INSERT INTO instance_meta (singleton, environment, created_at) VALUES (true, 'test', now()) ON CONFLICT DO NOTHING`); err != nil {
			t.Fatal(err)
		}
		if tables := dbtest.Tables(t, d.Pool); len(tables) < 10 {
			t.Fatalf("New did not migrate: %v", tables)
		}
	})
	t.Run("b", func(t *testing.T) {
		t.Parallel()
		d := dbtest.NewEmpty(t)
		second = d.Schema
		if tables := dbtest.Tables(t, d.Pool); len(tables) != 0 {
			t.Fatalf("empty schema has tables: %v", tables)
		}
	})
	t.Cleanup(func() {
		if first == "" || second == "" || first == second {
			t.Errorf("schemas %q %q", first, second)
			return
		}
		for _, s := range []string{first, second} {
			if dbtest.SchemaExists(t, s) {
				t.Errorf("schema %s survived its test", s)
			}
		}
	})
}
