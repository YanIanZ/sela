package main

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/db/dbtest"
	"github.com/YanIanZ/sela/internal/vault"
)

// needPGTools skips (or, when database tests are required, fails) without
// the PostgreSQL client tools the ops scripts call.
func needPGTools(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"pg_dump", "pg_restore", "psql", "bash"} {
		if _, err := exec.LookPath(bin); err != nil {
			if os.Getenv(dbtest.EnvRequire) == "1" {
				t.Fatalf("%s not installed but database tests are required", bin)
			}
			t.Skipf("%s not installed", bin)
		}
	}
}

func opsScript(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "scripts", "ops", name)
}

// argvLogger puts wrappers for pg_dump, pg_restore and psql first in PATH;
// each appends its full argument list to the returned log file and then
// runs the real tool.
func argvLogger(t *testing.T) (pathEnv, logFile string) {
	t.Helper()
	dir := t.TempDir()
	logFile = filepath.Join(dir, "argv.log")
	for _, bin := range []string{"pg_dump", "pg_restore", "psql"} {
		real, err := exec.LookPath(bin)
		if err != nil {
			t.Fatal(err)
		}
		w := "#!/bin/sh\nprintf '%s\\n' \"" + bin + " $*\" >> '" + logFile + "'\nexec '" + real + "' \"$@\"\n"
		if err := os.WriteFile(filepath.Join(dir, bin), []byte(w), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir + string(os.PathListSeparator) + os.Getenv("PATH"), logFile
}

// runScript runs an ops script under umask 022 (a common default) with the
// given environment additions and returns its combined output.
func runScript(t *testing.T, env []string, script string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"-c", `umask 022; exec "$0" "$@"`, opsScript(script)}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func scratchDatabase(t *testing.T) string {
	t.Helper()
	admin := dbtest.URL(t)
	c, err := pgx.Connect(bg, admin)
	if err != nil {
		t.Fatal(db.Classify(err))
	}
	name := "sela_ops_" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "_"))
	name = name[:min(len(name), 40)] + "_" + time.Now().UTC().Format("150405.000000")[7:]
	if _, err := c.Exec(bg, "CREATE DATABASE "+db.QuoteIdent(name)); err != nil {
		t.Fatal(db.Classify(err))
	}
	_ = c.Close(bg)
	t.Cleanup(func() {
		if c, err := pgx.Connect(bg, admin); err == nil {
			_, _ = c.Exec(bg, "DROP DATABASE IF EXISTS "+db.QuoteIdent(name)+" WITH (FORCE)")
			_ = c.Close(bg)
		}
	})
	u, _ := url.Parse(admin)
	u.Path = "/" + name
	return u.String()
}

// R1-4: dumps are private (0600 in a 0700 directory) whatever the umask,
// no partial file is left, and the database password never appears in the
// argument list of pg_dump, pg_restore or psql, or in the output.
func TestBackupScriptsKeepDumpsAndPasswordPrivate(t *testing.T) {
	t.Parallel()
	needPGTools(t)
	d := dbtest.New(t)
	base := dbtest.URL(t)
	u, _ := url.Parse(base)
	password, _ := u.User.Password()
	// A throttle row (the key of a typed username): never in a backup.
	if _, err := d.Pool.Exec(bg, `INSERT INTO login_throttle (key_hash, failures, last_failed_at) VALUES (sha256('u:penanda'::bytea), 3, now())`); err != nil {
		t.Fatal(err)
	}
	pathEnv, argv := argvLogger(t)
	out := filepath.Join(t.TempDir(), "backups", "daily")
	env := []string{"PATH=" + pathEnv, "SELA_DATABASE_URL=" + base}
	if o, err := runScript(t, env, "backup.sh", "--out-dir", out, "--schema", d.Schema); err != nil {
		t.Fatalf("backup.sh: %v\n%s", err, o)
	} else if password != "" && strings.Contains(o, password) {
		t.Fatal("backup.sh printed the password")
	}
	time.Sleep(1100 * time.Millisecond) // a new timestamp for the key-only dump
	if o, err := runScript(t, env, "backup.sh", "--keys-only", "--out-dir", out, "--schema", d.Schema); err != nil {
		t.Fatalf("backup.sh --keys-only: %v\n%s", err, o)
	}
	for _, dir := range []string{filepath.Dir(out), out} {
		if st, err := os.Stat(dir); err != nil || st.Mode().Perm() != 0o700 {
			t.Fatalf("%s: mode %v (%v), want 0700", dir, st.Mode().Perm(), err)
		}
	}
	entries, _ := os.ReadDir(out)
	var dumps, sums int
	for _, e := range entries {
		st, _ := e.Info()
		if strings.HasSuffix(e.Name(), ".partial") {
			t.Errorf("partial file left: %s", e.Name())
		}
		if st.Mode().Perm() != 0o600 {
			t.Errorf("%s: mode %v, want 0600", e.Name(), st.Mode().Perm())
		}
		switch filepath.Ext(e.Name()) {
		case ".dump":
			dumps++
		case ".sha256":
			sums++
		}
	}
	if dumps != 3 || sums != 2 {
		t.Fatalf("%d dumps and %d checksum files: %v", dumps, sums, entries)
	}
	content, _ := filepath.Glob(filepath.Join(out, "sela-content-*.dump"))
	a, err := pgRestoreList(content[0])
	if err != nil {
		t.Fatal(err)
	}
	var throttleTable, throttleData bool
	for _, e := range a.entries {
		throttleTable = throttleTable || (e.name == "login_throttle" && e.kind == "TABLE")
		throttleData = throttleData || (e.name == "login_throttle" && e.kind == "TABLE DATA")
	}
	if !throttleTable || throttleData {
		t.Fatalf("content dump: login_throttle definition %t, data %t (want the definition only)", throttleTable, throttleData)
	}
	// Restore into a scratch database: the password reaches psql and
	// pg_restore only through the private password file too.
	dst := scratchDatabase(t)
	if o, err := runScript(t, []string{"PATH=" + pathEnv, "SELA_RESTORE_DATABASE_URL=" + dst}, "restore.sh", "--content", content[0], "--keys", out); err != nil {
		t.Fatalf("restore.sh: %v\n%s", err, o)
	}
	raw, err := os.ReadFile(argv)
	if err != nil {
		t.Fatal(err)
	}
	log := string(raw)
	for _, tool := range []string{"pg_dump ", "pg_restore ", "psql "} {
		if !strings.Contains(log, tool) {
			t.Fatalf("%s was not called through the wrapper:\n%s", tool, log)
		}
	}
	if password != "" && strings.Contains(log, password) {
		t.Fatalf("the database password appeared in a process argument list:\n%s", log)
	}
}

// R1-2: the documented procedure (backup.sh, a later backup.sh --keys-only,
// restore.sh with the newest key dump, sela restore reconcile) removes an
// account deleted after the content backup; its encrypted field cannot be
// read and it cannot sign in. A key dump older than the content dump is
// refused by restore.sh and by sela backup check.
func TestRestoreProcedureRemovesAccountDeletedAfterBackup(t *testing.T) {
	t.Parallel()
	needPGTools(t)
	d := dbtest.New(t)
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
	const pw = "Lembah-Kabut-Sintetis 7 marker"
	res, err := s.Register(bg, account.RegisterInput{Username: "akan.dihapus", Password: pw, Locale: "id", AgeBand: account.AgeBand15to17,
		TermsPolicyVersion: ac.PolicyVersions[account.PolicyTerms]})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Authenticate(bg, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	name := "Ibu Penanda Sintetis Revival"
	if _, err := s.SetGuardianConsent(bg, p.UserID, p.SessionID, account.GuardianInput{Relationship: "parent",
		Method: "guardian_present_attestation", GuardianName: &name, PolicyVersion: ac.PolicyVersions[account.PolicyGuardian]}); err != nil {
		t.Fatal(err)
	}
	env := []string{"SELA_DATABASE_URL=" + dbtest.URL(t)}
	work := t.TempDir()
	stale, daily := filepath.Join(work, "stale"), filepath.Join(work, "backups")
	if o, err := runScript(t, env, "backup.sh", "--keys-only", "--out-dir", stale, "--schema", d.Schema); err != nil {
		t.Fatalf("stale key dump: %v\n%s", err, o)
	}
	time.Sleep(1100 * time.Millisecond)
	if o, err := runScript(t, env, "backup.sh", "--out-dir", daily, "--schema", d.Schema); err != nil {
		t.Fatalf("daily backup: %v\n%s", err, o)
	}
	content, _ := filepath.Glob(filepath.Join(daily, "sela-content-*.dump"))
	sameRunKeys, _ := filepath.Glob(filepath.Join(daily, "sela-keys-*.dump"))
	staleKeys, _ := filepath.Glob(filepath.Join(stale, "sela-keys-*.dump"))
	// The user deletes the account after the backup.
	if _, err := s.DeleteAccount(bg, p.UserID, p.SessionID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	if o, err := runScript(t, env, "backup.sh", "--keys-only", "--out-dir", daily, "--schema", d.Schema); err != nil {
		t.Fatalf("hourly key dump: %v\n%s", err, o)
	}

	// A key dump older than the content dump is refused.
	dst := scratchDatabase(t)
	if o, err := runScript(t, []string{"SELA_RESTORE_DATABASE_URL=" + dst}, "restore.sh", "--content", content[0], "--keys", staleKeys[0], "--dry-run"); err == nil ||
		!strings.Contains(o, "older than the content dump") {
		t.Fatalf("restore.sh accepted an older key dump: %v\n%s", err, o)
	}
	// So is the content dump's own key dump when a newer one lies next to it.
	if o, err := runScript(t, []string{"SELA_RESTORE_DATABASE_URL=" + dst}, "restore.sh", "--content", content[0], "--keys", sameRunKeys[0], "--dry-run"); err == nil ||
		!strings.Contains(o, "a newer key dump exists") {
		t.Fatalf("restore.sh accepted the same-run key dump while a newer one exists: %v\n%s", err, o)
	}
	if _, err := backupCheck(content[0], staleKeys[0]); err == nil || !strings.Contains(err.Error(), "older than the content dump") {
		t.Fatalf("backup check accepted an older key dump: %v", err)
	}
	if _, err := backupCheck(content[0], sameRunKeys[0]); err != nil {
		t.Fatalf("backup check refused the same-run pair: %v", err)
	}

	// Disaster: restore the content dump with the newest key dump.
	o, err := runScript(t, []string{"SELA_RESTORE_DATABASE_URL=" + dst}, "restore.sh", "--content", content[0], "--keys", daily)
	if err != nil {
		t.Fatalf("restore.sh: %v\n%s", err, o)
	}
	if newest, _ := filepath.Glob(filepath.Join(daily, "sela-keys-*.dump")); len(newest) != 2 || !strings.Contains(o, filepath.Base(newest[1])) {
		t.Fatalf("restore.sh did not use the newest key dump %v:\n%s", newest, o)
	}
	pool, err := db.Open(bg, db.Options{URL: dst, SearchPath: d.Schema, MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	v2, _ := newVault(cfg)
	r, err := account.New(pool, v2, ac)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := r.ReconcileRestore(bg, true)
	if err != nil || rep.DestroyedKey != 1 || rep.MissingKey != 0 {
		t.Fatalf("reconcile dry run: %+v %v", rep, err)
	}
	if rep, err := r.ReconcileRestore(bg, false); err != nil || rep.Deleted != 1 {
		t.Fatalf("reconcile: %+v %v", rep, err)
	}
	if _, err := r.GuardianConsent(bg, p.UserID); !errors.Is(err, account.ErrNotFound) {
		t.Fatalf("the deleted account's guardian record is readable after the restore: %v", err)
	}
	if _, err := r.Login(bg, account.LoginInput{Username: "akan.dihapus", Password: pw}); !errors.Is(err, account.ErrInvalidCredentials) {
		t.Fatalf("the deleted account can sign in after the restore: %v", err)
	}
}

// R2-4: an early exit (here: an output directory that cannot be created,
// and a database server that cannot be reached) leaves no password file
// behind: the scripts install their cleanup trap before sela_pg_url
// writes it.
func TestOpsScriptsRemovePasswordFileOnEarlyExit(t *testing.T) {
	t.Parallel()
	needPGTools(t)
	const secret = "r2-SECRET-pw"
	scratch := t.TempDir()
	notDir := filepath.Join(scratch, "not-a-dir")
	if err := os.WriteFile(notDir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// A go command that fails: restore-drill.sh builds sela right after
	// writing the password file when --sela is not given.
	stub := filepath.Join(scratch, "bin")
	if err := os.MkdirAll(stub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stub, "go"), []byte("#!/bin/sh\necho 'go: build failed (test stub)' >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, run := range map[string]struct {
		script string
		env    []string
		args   []string
	}{
		"backup.sh, output directory under a file": {"backup.sh", []string{"SELA_DATABASE_URL=postgres://sela:" + secret + "@127.0.0.1:5432/sela_test?sslmode=disable"},
			[]string{"--out-dir", filepath.Join(notDir, "backups")}},
		"restore-drill.sh, go build fails": {"restore-drill.sh", []string{"SELA_DRILL_DATABASE_URL=postgres://sela:" + secret + "@127.0.0.1:5432/sela_test?sslmode=disable",
			"PATH=" + stub + string(os.PathListSeparator) + os.Getenv("PATH")}, nil},
		"restore-drill.sh, server unreachable": {"restore-drill.sh", []string{"SELA_DRILL_DATABASE_URL=postgres://sela:" + secret + "@127.0.0.1:1/sela_test?sslmode=disable"},
			[]string{"--sela", "/bin/false"}},
		"restore.sh, unreadable archive": {"restore.sh", []string{"SELA_RESTORE_DATABASE_URL=postgres://sela:" + secret + "@127.0.0.1:1/sela_test?sslmode=disable"},
			[]string{"--content", notDir, "--keys", notDir}},
	} {
		tmp := filepath.Join(scratch, strings.NewReplacer(".", "_", " ", "_", ",", "").Replace(name)+"_tmp")
		if err := os.MkdirAll(tmp, 0o700); err != nil {
			t.Fatal(err)
		}
		o, err := runScript(t, append([]string{"TMPDIR=" + tmp}, run.env...), run.script, run.args...)
		if err == nil {
			t.Fatalf("%s: succeeded:\n%s", name, o)
		}
		if strings.Contains(o, secret) {
			t.Fatalf("%s: printed the password", name)
		}
		left, _ := os.ReadDir(tmp)
		if len(left) != 0 {
			t.Errorf("%s: %d entries left in TMPDIR after exit (%v); the password file must be removed", name, len(left), left[0].Name())
		}
	}
}

// restoreFixture registers through a store on a fresh schema and returns
// what the restore probes need.
func restoreFixture(t *testing.T) (*dbtest.DB, *account.Store, account.Config) {
	t.Helper()
	needPGTools(t)
	d := dbtest.New(t)
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
	return d, s, ac
}

// restoreInto runs the documented restore (restore.sh with the newest key
// dump) into a scratch database and returns a store on it after
// sela restore reconcile.
func restoreInto(t *testing.T, d *dbtest.DB, ac account.Config, content, keysDir string) (*account.Store, *pgxpool.Pool, account.ReconcileReport) {
	t.Helper()
	dst := scratchDatabase(t)
	if o, err := runScript(t, []string{"SELA_RESTORE_DATABASE_URL=" + dst}, "restore.sh", "--content", content, "--keys", keysDir); err != nil {
		t.Fatalf("restore.sh: %v\n%s", err, o)
	}
	pool, err := db.Open(bg, db.Options{URL: dst, SearchPath: d.Schema, MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	v, err := newVault(cfgFor(d, "test"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := account.New(pool, v, ac)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := r.ReconcileRestore(bg, false)
	if err != nil {
		t.Fatal(err)
	}
	return r, pool, rep
}

// R2-2: a session revoked after the content dump (here by a password
// change, the response to a stolen cookie) is not live again after the
// documented restore procedure: reconcile revokes every restored session.
func TestRestoreProcedureDoesNotReviveRevokedSessions(t *testing.T) {
	t.Parallel()
	d, s, ac := restoreFixture(t)
	const oldPW = "Lembah-Kabut-Sintetis 7 lama"
	const newPW = "Samudra-Senja-Sintetis 9 baru"
	stolen, err := s.Register(bg, account.RegisterInput{Username: "korban.sintetis", Password: oldPW, Locale: "id", AgeBand: account.AgeBand18Plus,
		TermsPolicyVersion: ac.PolicyVersions[account.PolicyTerms], RememberDevice: true})
	if err != nil {
		t.Fatal(err)
	}
	own, err := s.Login(bg, account.LoginInput{Username: "korban.sintetis", Password: oldPW})
	if err != nil {
		t.Fatal(err)
	}
	env := []string{"SELA_DATABASE_URL=" + dbtest.URL(t)}
	daily := filepath.Join(t.TempDir(), "backups")
	if o, err := runScript(t, env, "backup.sh", "--out-dir", daily, "--schema", d.Schema); err != nil {
		t.Fatalf("daily backup: %v\n%s", err, o)
	}
	content, _ := filepath.Glob(filepath.Join(daily, "sela-content-*.dump"))
	p, err := s.Authenticate(bg, own.Token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChangePassword(bg, p.UserID, p.SessionID, oldPW, newPW, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(bg, stolen.Token); !errors.Is(err, account.ErrNotSignedIn) {
		t.Fatalf("precondition: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if o, err := runScript(t, env, "backup.sh", "--keys-only", "--out-dir", daily, "--schema", d.Schema); err != nil {
		t.Fatalf("hourly key dump: %v\n%s", err, o)
	}
	r, _, rep := restoreInto(t, d, ac, content[0], daily)
	if rep.SessionsRevoked != 2 {
		t.Fatalf("reconcile: %+v (both restored sessions must be revoked)", rep)
	}
	for _, tok := range []string{stolen.Token, own.Token} {
		if p, err := r.Authenticate(bg, tok); !errors.Is(err, account.ErrNotSignedIn) {
			t.Fatalf("a restored session is live after restore + reconcile (kind %s, trusted %t): %v", p.Kind, p.TrustedDevice, err)
		}
	}
	// What a restore cannot undo, and RUNBOOK §8 tells users to repeat: the
	// password changed after the content dump is the old one again.
	if _, err := r.Login(bg, account.LoginInput{Username: "korban.sintetis", Password: oldPW}); err != nil {
		t.Fatalf("the restored account (old password, as documented): %v", err)
	}
}

// R2-3: the key of an account created after the content dump comes back
// live with the newer key dump; reconcile destroys it, so a copy of that
// person's ciphertext cannot be opened from the restored key table.
func TestRestoreProcedureDestroysKeysWithoutUser(t *testing.T) {
	t.Parallel()
	d, s, ac := restoreFixture(t)
	env := []string{"SELA_DATABASE_URL=" + dbtest.URL(t)}
	daily := filepath.Join(t.TempDir(), "backups")
	if o, err := runScript(t, env, "backup.sh", "--out-dir", daily, "--schema", d.Schema); err != nil {
		t.Fatalf("daily backup: %v\n%s", err, o)
	}
	content, _ := filepath.Glob(filepath.Join(daily, "sela-content-*.dump"))
	res, err := s.Register(bg, account.RegisterInput{Username: "setelah.cadangan", Password: "Lembah-Kabut-Sintetis 7 orphan", Locale: "id",
		AgeBand: account.AgeBand15to17, TermsPolicyVersion: ac.PolicyVersions[account.PolicyTerms]})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Authenticate(bg, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	name := "Ibu Penanda Sintetis Orphan"
	rec, err := s.SetGuardianConsent(bg, p.UserID, p.SessionID, account.GuardianInput{Relationship: "parent",
		Method: "guardian_present_attestation", GuardianName: &name, PolicyVersion: ac.PolicyVersions[account.PolicyGuardian]})
	if err != nil {
		t.Fatal(err)
	}
	var ct []byte
	if err := d.Pool.QueryRow(bg, `SELECT guardian_name_ct FROM guardian_consents WHERE user_id = $1`, p.UserID).Scan(&ct); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	if o, err := runScript(t, env, "backup.sh", "--keys-only", "--out-dir", daily, "--schema", d.Schema); err != nil {
		t.Fatalf("hourly key dump: %v\n%s", err, o)
	}
	r, pool, rep := restoreInto(t, d, ac, content[0], daily)
	if rep.OrphanKeys != 1 || rep.KeysDestroyed != 1 {
		t.Fatalf("reconcile: %+v", rep)
	}
	var orphans int
	if err := pool.QueryRow(bg, `SELECT count(*) FROM user_data_keys k WHERE k.destroyed_at IS NULL
		AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = k.user_id)`).Scan(&orphans); err != nil || orphans != 0 {
		t.Fatalf("live key rows without a user after reconcile: %d %v", orphans, err)
	}
	if pt, err := r.Vault().Open(bg, pool, p.UserID, vault.Ref{Kind: "guardian_consent", RecordID: rec.ID, Field: "guardian_name"}, ct); !errors.Is(err, vault.ErrKeyDestroyed) {
		t.Fatalf("the ciphertext of a user who no longer exists still opens (%d bytes): %v", len(pt), err)
	}
}
