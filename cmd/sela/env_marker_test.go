package main

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/config"
	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/db/dbtest"
)

// R1-1: every operator command that opens SELA_DATABASE_URL refuses a
// database whose environment marker names another environment (exit 4),
// and changes nothing in it; migrate status reports the mismatch.
func TestOperatorCommandsRefuseAnotherEnvironment(t *testing.T) {
	t.Parallel()
	d := dbtest.NewEmpty(t)
	prod := &db.Migrator{Pool: d.Pool, Migrations: db.MustEmbedded(), BinaryVersion: "test", Environment: config.EnvProduction}
	if _, err := prod.Up(bg, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	prodEnv := envFor(d, config.EnvProduction)
	if code, out, stderr := runMain(t, prodEnv, "ops canary write"); code != 0 {
		t.Fatalf("production canary write: %d %s %s", code, out, stderr)
	}
	before := dbtest.DumpText(t, d.Pool)
	catalog := dbtest.Catalog(t, d.Pool, d.Schema)

	staging := map[string]string{"SELA_ENV": config.EnvStaging, "SELA_DATABASE_URL": d.URL,
		"SELA_DATA_KEK": base64.StdEncoding.EncodeToString(testKEK()), "SELA_DATA_KEK_ID": "staging-test"}
	for _, args := range []string{
		"keys status", "keys rewrap", "keys rewrap --batch 1",
		"restore reconcile --dry-run", "restore reconcile",
		"ops canary write", "ops canary check",
		"migrate up", "migrate dry-run", "migrate down-last --confirm 0900", "migrate down-last --confirm 0001",
	} {
		code, out, stderr := runMain(t, staging, args)
		if code != 4 || !strings.Contains(stderr, "environment") {
			t.Errorf("%s: exit %d, want 4\n%s%s", args, code, out, stderr)
		}
		if strings.Contains(out+stderr, "sela-dev-only") {
			t.Errorf("%s: output contains the database password", args)
		}
	}
	code, out, _ := runMain(t, staging, "migrate status")
	if code != 4 || !strings.Contains(out, "result: mismatch") || !strings.Contains(out, "SELA_ENV is staging") {
		t.Errorf("migrate status on another environment's database: exit %d\n%s", code, out)
	}
	if got := dbtest.DumpText(t, d.Pool); got != before {
		t.Error("a refused command changed rows")
	}
	if got := dbtest.Catalog(t, d.Pool, d.Schema); got != catalog {
		t.Error("a refused command changed the schema")
	}
	// The production operator still works on it.
	if code, out, stderr := runMain(t, prodEnv, "ops canary check"); code != 0 {
		t.Fatalf("production canary check: %d %s %s", code, out, stderr)
	}
	if code, out, _ := runMain(t, prodEnv, "migrate status"); code != 0 || !strings.Contains(out, "environment marker: production\n") {
		t.Fatalf("production status: %d\n%s", code, out)
	}
}
