package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"testing/fstest"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/config"
	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/db/dbtest"
)

var bg = context.Background()

var discard = slog.New(slog.NewJSONHandler(io.Discard, nil))

func testKEK() []byte {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return b
}

func cfgFor(d *dbtest.DB, env string) config.Config {
	return config.Config{Env: env, DatabaseURL: d.URL, DBMaxConns: 2, DBStatementTimeout: 10 * time.Second,
		KEK: config.KEK{ID: config.KEKPrefix[env] + "-test", Secret: testKEK()}}
}

// runMain runs this binary's main() with args and a clean SELA_ environment
// and returns the exit code and output.
func runMain(t *testing.T, env map[string]string, args string) (int, string, string) {
	t.Helper()
	var e []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "SELA_") {
			e = append(e, kv)
		}
	}
	e = append(e, "SELA_TEST_RUN_MAIN=1", "SELA_TEST_ARGS="+args, "SELA_ADDR=127.0.0.1:0", "SELA_WEB_DIR="+t.TempDir(),
		"SELA_RESEARCH_KB="+filepath.Join(t.TempDir(), "none.jsonl"))
	for k, v := range env {
		e = append(e, k+"="+v)
	}
	ctx, cancel := context.WithTimeout(bg, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
	cmd.Env = e
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, stdout.String(), stderr.String()
}

func envFor(d *dbtest.DB, env string) map[string]string {
	return map[string]string{"SELA_ENV": env, "SELA_DATABASE_URL": d.URL, "SELA_DATA_KEK": base64.StdEncoding.EncodeToString(testKEK()),
		"SELA_DATA_KEK_ID": config.KEKPrefix[env] + "-test", "SELA_SECURE_COOKIES": "false"}
}

func TestStartRefusesSchemaMismatch(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	if _, err := d.Pool.Exec(bg, `UPDATE schema_migrations SET checksum = '\x00'::bytea WHERE version = 1`); err != nil {
		t.Fatal(err)
	}
	for _, env := range []string{"test", "development"} {
		cfg := cfgFor(d, env)
		cfg.DBAutoMigrate = true
		if err := prepareDatabase(bg, d.Pool, cfg, db.MustEmbedded(), discard); err == nil || !strings.Contains(err.Error(), "mismatch") {
			t.Fatalf("%s: started on a changed checksum: %v", env, err)
		}
	}
	// A version applied by a newer binary.
	d2 := dbtest.New(t)
	if _, err := d2.Pool.Exec(bg, `INSERT INTO schema_migrations (version, name, kind, checksum, duration_ms, binary_version)
		VALUES (150, 'newer', 'schema', '\x01'::bytea, 1, 'newer')`); err != nil {
		t.Fatal(err)
	}
	if err := prepareDatabase(bg, d2.Pool, cfgFor(d2, "test"), db.MustEmbedded(), discard); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("started on a schema ahead of the binary: %v", err)
	}
	// The real binary: the server exits, status says 4, up refuses with 4.
	code, _, stderr := runMain(t, envFor(d, "test"), "")
	if code == 0 || !strings.Contains(stderr, "schema mismatch") || strings.Contains(stderr, "sela-dev-only") {
		t.Fatalf("server start: exit %d: %s", code, stderr)
	}
	if code, out, _ := runMain(t, envFor(d, "test"), "migrate status"); code != 4 || !strings.Contains(out, "checksum_mismatch") {
		t.Fatalf("status: exit %d\n%s", code, out)
	}
	if code, _, stderr := runMain(t, envFor(d2, "test"), "migrate up"); code != 4 || !strings.Contains(stderr, "does not match this binary") {
		t.Fatalf("up: exit %d %s", code, stderr)
	}
}

func TestStartRefusesEnvironmentMismatch(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t) // marked "test"
	for _, env := range []string{"development", "staging", "production"} {
		if err := prepareDatabase(bg, d.Pool, cfgFor(d, env), db.MustEmbedded(), discard); err == nil || !strings.Contains(err.Error(), "environment") {
			t.Errorf("%s opened a test database: %v", env, err)
		}
	}
	if err := prepareDatabase(bg, d.Pool, cfgFor(d, "test"), db.MustEmbedded(), discard); err != nil {
		t.Fatalf("matching environment refused: %v", err)
	}
	// The migrator refuses as well.
	m := &db.Migrator{Pool: d.Pool, Migrations: db.MustEmbedded(), BinaryVersion: "x", Environment: "production"}
	if _, err := m.Up(bg, db.UpOptions{}); !errors.Is(err, db.ErrEnvironmentMismatch) {
		t.Fatalf("migrate up into another environment: %v", err)
	}
}

func TestStartRefusesPendingMigrations(t *testing.T) {
	t.Parallel()
	for _, env := range []string{"staging", "production", "development", "test"} {
		d := dbtest.NewEmpty(t)
		cfg := cfgFor(d, env)
		cfg.DBAutoMigrate = env == "staging" || env == "production" // ignored there
		if env == "development" || env == "test" {
			cfg.DBAutoMigrate = false
		}
		err := prepareDatabase(bg, d.Pool, cfg, db.MustEmbedded(), discard)
		if err == nil || !strings.Contains(err.Error(), "pending") || !strings.Contains(err.Error(), "sela migrate up") {
			t.Errorf("%s: started with pending migrations: %v", env, err)
		}
		if tables := dbtest.Tables(t, d.Pool); len(tables) > 1 {
			t.Errorf("%s: start applied migrations: %v", env, tables)
		}
	}
	// One pending migration on an otherwise current production-like schema.
	d := dbtest.New(t)
	if _, err := d.Pool.Exec(bg, `DELETE FROM schema_migrations WHERE version = 900; DROP TABLE ops_canary`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(bg, `UPDATE instance_meta SET environment = 'staging'`); err != nil {
		t.Fatal(err)
	}
	if err := prepareDatabase(bg, d.Pool, cfgFor(d, "staging"), db.MustEmbedded(), discard); err == nil || !strings.Contains(err.Error(), "1 database migrations are pending") {
		t.Fatalf("staging with one pending: %v", err)
	}
}

func dataMigrationSet(t *testing.T) []db.Migration {
	t.Helper()
	fsys := fstest.MapFS{}
	for _, m := range db.MustEmbedded() {
		fsys["m/"+m.Label()+".up.sql"] = &fstest.MapFile{Data: []byte(m.Up)}
		fsys["m/"+m.Label()+".down.sql"] = &fstest.MapFile{Data: []byte(m.Down)}
	}
	fsys["m/0901_fill.up.sql"] = &fstest.MapFile{Data: []byte("-- sela:kind data\nUPDATE users SET locale = 'en';")}
	fsys["m/0901_fill.down.sql"] = &fstest.MapFile{Data: []byte("SELECT 1;")}
	fsys["m/0901_fill.validate.sql"] = &fstest.MapFile{Data: []byte("SELECT id FROM users WHERE locale <> 'en'")}
	migs, err := db.Parse(fsys, "m")
	if err != nil {
		t.Fatal(err)
	}
	return migs
}

func TestAutoMigrate(t *testing.T) {
	t.Parallel()
	for _, env := range []string{"development", "test"} {
		d := dbtest.NewEmpty(t)
		cfg := cfgFor(d, env)
		cfg.DBAutoMigrate = true
		if err := prepareDatabase(bg, d.Pool, cfg, db.MustEmbedded(), discard); err != nil {
			t.Fatalf("%s auto-migrate: %v", env, err)
		}
		if got, _ := db.ReadEnvironment(bg, d.Pool); got != env {
			t.Fatalf("%s: marker %q", env, got)
		}
		// Idempotent on the next start.
		if err := prepareDatabase(bg, d.Pool, cfg, db.MustEmbedded(), discard); err != nil {
			t.Fatal(err)
		}
	}
	// A pending data migration is never applied at start, even with
	// auto-migrate; nothing else of the run is applied either.
	d := dbtest.New(t)
	cfg := cfgFor(d, "test")
	cfg.DBAutoMigrate = true
	err := prepareDatabase(bg, d.Pool, cfg, dataMigrationSet(t), discard)
	if err == nil || !strings.Contains(err.Error(), "data migration") {
		t.Fatalf("data migration at start: %v", err)
	}
	var n int
	_ = d.Pool.QueryRow(bg, `SELECT count(*) FROM schema_migrations WHERE version = 901`).Scan(&n)
	if n != 0 {
		t.Fatal("data migration applied at start")
	}
	// Auto-migrate is ignored outside development and test (config also refuses it).
	ds := dbtest.NewEmpty(t)
	cs := cfgFor(ds, "staging")
	cs.DBAutoMigrate = true
	if err := prepareDatabase(bg, ds.Pool, cs, db.MustEmbedded(), discard); err == nil {
		t.Fatal("staging auto-migrated")
	}
}

func TestMigrateCommands(t *testing.T) {
	t.Parallel()
	d := dbtest.NewEmpty(t)
	env := envFor(d, "test")
	if code, out, _ := runMain(t, env, "migrate status"); code != 3 || !strings.Contains(out, "pending") || !strings.Contains(out, "0001") {
		t.Fatalf("status on empty: %d\n%s", code, out)
	}
	if code, out, _ := runMain(t, env, "migrate dry-run"); code != 0 || !strings.Contains(out, "rolled back") {
		t.Fatalf("dry-run: %d\n%s", code, out)
	}
	if tables := dbtest.Tables(t, d.Pool); len(tables) != 0 {
		t.Fatalf("dry-run left tables: %v", tables)
	}
	if code, out, stderr := runMain(t, env, "migrate up --to 0001"); code != 0 || !strings.Contains(out, "applied 0001_foundation") {
		t.Fatalf("up --to: %d\n%s%s", code, out, stderr)
	}
	if code, out, _ := runMain(t, env, "migrate up"); code != 0 || !strings.Contains(out, "applied 0910_admin") {
		t.Fatalf("up: %d\n%s", code, out)
	}
	if code, out, _ := runMain(t, env, "migrate status"); code != 0 || !strings.Contains(out, "result: current") || !strings.Contains(out, "environment marker: test") {
		t.Fatalf("status: %d\n%s", code, out)
	}
	if code, _, stderr := runMain(t, env, "migrate down-last --confirm 0001"); code != 1 || !strings.Contains(stderr, "0910_admin") {
		t.Fatalf("down-last with the wrong confirm: %d %s", code, stderr)
	}
	if code, out, _ := runMain(t, env, "migrate down-last --confirm 0910"); code != 0 || !strings.Contains(out, "reverted 0910_admin") {
		t.Fatalf("down-last: %d\n%s", code, out)
	}
	if code, _, _ := runMain(t, env, "migrate status"); code != 3 {
		t.Fatalf("status after down-last: %d", code)
	}
	if code, _, _ := runMain(t, env, "migrate up"); code != 0 {
		t.Fatal("up again")
	}
	if code, _, _ := runMain(t, env, "migrate sideways"); code != 2 {
		t.Fatalf("unknown subcommand: %d", code)
	}
	// Another environment's binary cannot migrate this database.
	if code, _, stderr := runMain(t, envFor(d, "development"), "migrate up"); code != 4 || !strings.Contains(stderr, "environment") {
		t.Fatalf("other environment: %d %s", code, stderr)
	}
}

func TestOpsCommands(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	kek := base64.StdEncoding.EncodeToString(testKEK())
	env := envFor(d, "test")
	env["SELA_DATA_KEK"] = kek
	if code, out, stderr := runMain(t, env, "ops canary check"); code != 1 || !strings.Contains(stderr, "no canary") {
		t.Fatalf("check without canary: %d %s %s", code, out, stderr)
	}
	if code, out, stderr := runMain(t, env, "ops canary write"); code != 0 || !strings.Contains(out, "canary written") {
		t.Fatalf("write: %d %s %s", code, out, stderr)
	}
	if code, out, stderr := runMain(t, env, "ops canary check"); code != 0 || !strings.Contains(out, "canary ok") {
		t.Fatalf("check: %d %s %s", code, out, stderr)
	}
	// Another KEK cannot open it.
	other := envFor(d, "test")
	if code, _, stderr := runMain(t, other, "ops canary check"); code != 1 || !strings.Contains(stderr, "cannot be opened") {
		t.Fatalf("check with another KEK: %d %s", code, stderr)
	}
	if code, out, _ := runMain(t, env, "keys status"); code != 0 || !strings.Contains(out, "data keys under test-test: 1") {
		t.Fatalf("keys status: %d\n%s", code, out)
	}
	if code, out, _ := runMain(t, env, "restore reconcile --dry-run"); code != 0 || !strings.Contains(out, "users with a destroyed key: 0") {
		t.Fatalf("reconcile: %d\n%s", code, out)
	}
	if code, out, _ := runMain(t, map[string]string{"SELA_ENV": "staging"}, "keys generate-kek"); code != 0 || !strings.Contains(out, "SELA_DATA_KEK_ID=staging-") {
		t.Fatalf("generate-kek: %d\n%s", code, out)
	}
	// Rotation through the commands: old key moves to _PREVIOUS, rewrap, status.
	rot := envFor(d, "test")
	rot["SELA_DATA_KEK_ID"] = "test-new"
	rot["SELA_DATA_KEK_PREVIOUS"] = "test-test:" + kek
	if code, out, stderr := runMain(t, rot, "keys rewrap --batch 1"); code != 0 || !strings.Contains(out, "done: 1 data keys re-wrapped to test-new") {
		t.Fatalf("rewrap: %d %s %s", code, out, stderr)
	}
	if code, out, _ := runMain(t, rot, "keys status"); code != 0 || !strings.Contains(out, "data keys under other KEKs: 0") {
		t.Fatalf("status after rewrap: %s", out)
	}
	delete(rot, "SELA_DATA_KEK_PREVIOUS")
	if code, out, stderr := runMain(t, rot, "ops canary check"); code != 0 {
		t.Fatalf("canary after rotation: %d %s %s", code, out, stderr)
	}
}

func pgDump(t *testing.T, d *dbtest.DB, out string, args ...string) {
	t.Helper()
	bin, err := exec.LookPath("pg_dump")
	if err != nil {
		if os.Getenv(dbtest.EnvRequire) == "1" {
			t.Fatal("pg_dump not installed but database tests are required")
		}
		t.Skip("pg_dump not installed")
	}
	cmd := exec.Command(bin, append([]string{"-Fc", "--no-owner", "-f", out, "--dbname", dbtest.URL(t)}, args...)...)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pg_dump: %v %s", err, b)
	}
}

func TestBackupCheck(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	// Some real rows: an account with its key, and a deleted one (tombstone).
	cfg := cfgFor(d, "test")
	v, err := newVault(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ac := accountConfig(cfg, discard)
	ac.Hash = account.FloorHashParams
	s, err := account.New(d.Pool, v, ac)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Register(bg, account.RegisterInput{Username: "cadangan.data", Password: "kata sandi cadangan yang panjang", Locale: "id",
		AgeBand: "18_plus", TermsPolicyVersion: ac.PolicyVersions["terms_privacy"]})
	if err != nil {
		t.Fatal(err)
	}
	_ = res
	dir := t.TempDir()
	content, keys, whole := filepath.Join(dir, "content.dump"), filepath.Join(dir, "keys.dump"), filepath.Join(dir, "whole.dump")
	pgDump(t, d, content, "--schema="+d.Schema, "--exclude-table="+d.Schema+".user_data_keys")
	pgDump(t, d, keys, "--table="+d.Schema+".user_data_keys")
	pgDump(t, d, whole, "--schema="+d.Schema)
	rep, err := backupCheck(content, keys)
	if err != nil {
		t.Fatal(err)
	}
	if rep.contentTables < 10 || rep.contentData < 10 {
		t.Fatalf("content dump: %+v", rep)
	}
	for name, pair := range map[string][2]string{
		"swapped":                 {keys, content},
		"content with keys":       {whole, keys},
		"keys dump with content":  {content, whole},
		"not an archive":          {filepath.Join(dir, "missing.dump"), keys},
		"key dump not an archive": {content, "/dev/null"},
	} {
		if _, err := backupCheck(pair[0], pair[1]); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// Through the command.
	if code, out, stderr := runMain(t, nil, "backup check --content "+content+" --keys "+keys); code != 0 || !strings.Contains(out, "key dump: user_data_keys only") {
		t.Fatalf("command: %d %s %s", code, out, stderr)
	}
	if code, _, _ := runMain(t, nil, "backup check --content "+whole+" --keys "+keys); code != 1 {
		t.Fatal("command accepted a content dump with keys")
	}
}

// TestAutoMigrateServeEndToEnd starts the real server in development with
// SELA_DB_AUTO_MIGRATE on an empty schema and uses /api/v3.
func TestAutoMigrateServeEndToEnd(t *testing.T) {
	t.Parallel()
	d := dbtest.NewEmpty(t)
	p := startSela(t, "SELA_DATABASE_URL="+d.URL, "SELA_DATA_KEK="+base64.StdEncoding.EncodeToString(testKEK()),
		"SELA_DATA_KEK_ID=dev-test", "SELA_DB_AUTO_MIGRATE=true", "SELA_ARGON2_MEMORY_KIB=19456")
	res, err := http.Get(p.base + "/api/v2/ready")
	if err != nil {
		t.Fatal(err)
	}
	var ready struct {
		Data map[string]any `json:"data"`
	}
	_ = json.NewDecoder(res.Body).Decode(&ready)
	res.Body.Close()
	if res.StatusCode != 200 || ready.Data["database"] != "ok" || ready.Data["schema"] != "current" || ready.Data["store"] != "memory" {
		t.Fatalf("ready: %d %v", res.StatusCode, ready.Data)
	}
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{Jar: jar, Timeout: 30 * time.Second}
	pol := call(t, hc, "GET", p.base+"/api/v3/policies", nil)["policies"].(map[string]any)
	terms := pol["terms_privacy"].(map[string]any)["version"]
	d1 := call(t, hc, "POST", p.base+"/api/v3/auth/register", map[string]any{"username": "uji.server", "password": "kopi pagi di teras rumah sintetis",
		"locale": "id", "ageBand": "18_plus", "termsPolicyVersion": terms})
	if d1["user"].(map[string]any)["environment"] != "development" {
		t.Fatalf("register: %v", d1)
	}
	if me := call(t, hc, "GET", p.base+"/api/v3/me", nil); me["username"] != "uji.server" {
		t.Fatalf("me: %v", me)
	}
	if env, _ := db.ReadEnvironment(bg, d.Pool); env != "development" {
		t.Fatalf("marker %q", env)
	}
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	p.wait(t, time.Now(), 10*time.Second)
	logs := p.stdout.String()
	for _, secret := range []string{"uji.server", "kopi pagi di teras rumah sintetis", "sela-dev-only", d.Schema} {
		if strings.Contains(logs, secret) {
			t.Errorf("server logs contain %q", secret)
		}
	}
}
