// Package dbtest gives each PostgreSQL test its own schema so database
// tests run in parallel. It reads SELA_TEST_DATABASE_URL; when the variable
// is unset the test is skipped, unless SELA_REQUIRE_DB_TESTS=1 (CI), in
// which case it fails. Every schema is dropped in t.Cleanup.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YanIanZ/sela/internal/db"
)

// Environment variables read by the helper.
const (
	EnvURL     = "SELA_TEST_DATABASE_URL"
	EnvRequire = "SELA_REQUIRE_DB_TESTS"
)

// Action is what a database test does in the current environment.
type Action int

// Actions.
const (
	Run Action = iota
	Skip
	Fail
)

// Decide maps the two variables to an action: a URL runs the test; without
// one the test is skipped unless database tests are required.
func Decide(url, require string) Action {
	switch {
	case strings.TrimSpace(url) != "":
		return Run
	case strings.TrimSpace(require) == "1":
		return Fail
	}
	return Skip
}

// URL returns the base test database URL, or skips/fails the test.
func URL(t testing.TB) string {
	t.Helper()
	u := os.Getenv(EnvURL)
	switch Decide(u, os.Getenv(EnvRequire)) {
	case Skip:
		t.Skip(EnvURL + " is not set; skipping PostgreSQL test")
	case Fail:
		t.Fatal(EnvURL + " is not set but " + EnvRequire + "=1")
	}
	return u
}

// DB is one test's schema.
type DB struct {
	Pool   *pgxpool.Pool
	Schema string
	// URL is the test database URL with search_path set to Schema, for
	// code under test that opens its own pool (commands, subprocesses).
	URL string
}

// SchemaURL adds search_path=schema to a postgres:// URL.
func SchemaURL(base, schema string) (string, error) {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return "", fmt.Errorf("dbtest: %s must be a postgres:// URL", EnvURL)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// NewEmpty creates an empty schema t_<16 hex> and a pool whose sessions use
// it as search_path. No migration is applied.
func NewEmpty(t testing.TB) *DB {
	t.Helper()
	base := URL(t)
	var b [8]byte
	_, _ = rand.Read(b[:])
	schema := "t_" + hex.EncodeToString(b[:])
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("dbtest: connect: %s", db.Classify(err))
	}
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+db.QuoteIdent(schema))
	_ = admin.Close(ctx)
	if err != nil {
		t.Fatalf("dbtest: create schema: %s", db.Classify(err))
	}
	su, err := SchemaURL(base, schema)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.Open(ctx, db.Options{URL: base, SearchPath: schema, MaxConns: 3})
	if err != nil {
		dropSchema(t, base, schema)
		t.Fatalf("dbtest: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		dropSchema(t, base, schema)
	})
	return &DB{Pool: pool, Schema: schema, URL: su}
}

func dropSchema(t testing.TB, base, schema string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Errorf("dbtest: drop %s: connect: %s", schema, db.Classify(err))
		return
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, "DROP SCHEMA IF EXISTS "+db.QuoteIdent(schema)+" CASCADE"); err != nil {
		t.Errorf("dbtest: drop %s: %s", schema, db.Classify(err))
	}
}

// New creates a schema and applies every embedded migration (environment
// "test").
func New(t testing.TB) *DB {
	t.Helper()
	d := NewEmpty(t)
	m := d.Migrator(db.MustEmbedded())
	if _, err := m.Up(context.Background(), db.UpOptions{}); err != nil {
		t.Fatalf("dbtest: migrate up: %v", err)
	}
	return d
}

// Migrator returns a runner for this schema with the given migrations.
func (d *DB) Migrator(migs []db.Migration) *db.Migrator {
	return &db.Migrator{Pool: d.Pool, Migrations: migs, BinaryVersion: "test", Environment: "test"}
}

// OpenPool opens another pool on the same schema (a second instance).
func (d *DB) OpenPool(t testing.TB, maxConns int32) *pgxpool.Pool {
	t.Helper()
	p, err := db.Open(context.Background(), db.Options{URL: URL(t), SearchPath: d.Schema, MaxConns: maxConns})
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

// SchemaExists reports whether a schema exists (cleanup checks).
func SchemaExists(t testing.TB, schema string) bool {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, URL(t))
	if err != nil {
		t.Fatalf("dbtest: connect: %s", db.Classify(err))
	}
	defer c.Close(ctx)
	var ok bool
	if err := c.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)`, schema).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

// Catalog is a deterministic text snapshot of the schema's objects:
// relations, columns, constraints, indexes, triggers, functions and
// privileges. Row contents are not included.
func Catalog(t testing.TB, q db.DBTX, schema string) string {
	t.Helper()
	ctx := context.Background()
	queries := []string{
		`SELECT 'rel', c.relname, c.relkind::text, coalesce(c.relacl::text, '') FROM pg_class c
		  JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = current_schema() ORDER BY 2`,
		`SELECT 'col', table_name || '.' || column_name, data_type || ' ' || is_nullable || ' ' || is_identity,
		  coalesce(column_default, '') FROM information_schema.columns WHERE table_schema = current_schema() ORDER BY 2`,
		`SELECT 'con', conrelid::regclass::text || '.' || conname, contype::text, pg_get_constraintdef(oid) FROM pg_constraint
		  WHERE connamespace = (SELECT oid FROM pg_namespace WHERE nspname = current_schema()) ORDER BY 2`,
		`SELECT 'idx', tablename || '.' || indexname, '', indexdef FROM pg_indexes WHERE schemaname = current_schema() ORDER BY 2`,
		`SELECT 'trg', tgrelid::regclass::text || '.' || tgname, '', pg_get_triggerdef(t.oid) FROM pg_trigger t
		  JOIN pg_class c ON c.oid = t.tgrelid JOIN pg_namespace n ON n.oid = c.relnamespace
		  WHERE NOT t.tgisinternal AND n.nspname = current_schema() ORDER BY 2`,
		`SELECT 'fn', p.proname, '', pg_get_functiondef(p.oid) FROM pg_proc p
		  JOIN pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname = current_schema() ORDER BY 2`,
		`SELECT 'grant', table_name || ':' || grantee, privilege_type, '' FROM information_schema.role_table_grants
		  WHERE table_schema = current_schema() ORDER BY 2, 3`,
	}
	var b strings.Builder
	for _, sql := range queries {
		rows, err := q.Query(ctx, sql)
		if err != nil {
			t.Fatalf("dbtest: catalog: %v", err)
		}
		for rows.Next() {
			var a, k, v, d string
			if err := rows.Scan(&a, &k, &v, &d); err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&b, "%s %s %s %s\n", a, k, v, d)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
	}
	return strings.ReplaceAll(b.String(), schema, "<schema>")
}

// Tables lists the tables in the schema.
func Tables(t testing.TB, q db.DBTX) []string {
	t.Helper()
	rows, err := q.Query(context.Background(), `SELECT tablename FROM pg_tables WHERE schemaname = current_schema() ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// OwnedTables lists the tables that have a user_id column.
func OwnedTables(t testing.TB, q db.DBTX) []string {
	t.Helper()
	rows, err := q.Query(context.Background(), `SELECT c.table_name FROM information_schema.columns c
		JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name AND t.table_type = 'BASE TABLE'
		WHERE c.table_schema = current_schema() AND c.column_name = 'user_id' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// CountOwned counts one user's rows in every table with a user_id column.
func CountOwned(t testing.TB, q db.DBTX, userID any) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, tbl := range OwnedTables(t, q) {
		var n int
		if err := q.QueryRow(context.Background(), "SELECT count(*) FROM "+db.QuoteIdent(tbl)+" WHERE user_id = $1", userID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out[tbl] = n
	}
	return out
}

// DumpText returns every row of every table cast to text, one line per
// row, for at-rest scans (bytea shows as \x hex).
func DumpText(t testing.TB, q db.DBTX) string {
	t.Helper()
	var b strings.Builder
	for _, tbl := range Tables(t, q) {
		rows, err := q.Query(context.Background(), "SELECT t::text FROM "+db.QuoteIdent(tbl)+" t")
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			b.WriteString(tbl + " " + s + "\n")
		}
		rows.Close()
	}
	return b.String()
}
