package db_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/db/dbtest"
)

var ctx = context.Background()

func embeddedPlus(t *testing.T, extra map[string]string) []db.Migration {
	t.Helper()
	fsys := fstest.MapFS{}
	for _, m := range db.MustEmbedded() {
		fsys["m/"+m.Label()+".up.sql"] = &fstest.MapFile{Data: []byte(m.Up)}
		if m.HasDown {
			fsys["m/"+m.Label()+".down.sql"] = &fstest.MapFile{Data: []byte(m.Down)}
		}
		if m.HasValidate {
			fsys["m/"+m.Label()+".validate.sql"] = &fstest.MapFile{Data: []byte(m.Validate)}
		}
	}
	for name, body := range extra {
		fsys["m/"+name] = &fstest.MapFile{Data: []byte(body)}
	}
	migs, err := db.Parse(fsys, "m")
	if err != nil {
		t.Fatal(err)
	}
	return migs
}

func TestMigrateUp(t *testing.T) {
	t.Parallel()
	d := dbtest.NewEmpty(t)
	m := d.Migrator(db.MustEmbedded())
	m.BinaryVersion = "test-build-7"
	steps, err := m.Up(ctx, db.UpOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != len(db.MustEmbedded()) {
		t.Fatalf("applied %d steps", len(steps))
	}
	rows, err := d.Pool.Query(ctx, `SELECT version, name, kind, checksum, applied_seq, duration_ms, binary_version, applied_by FROM schema_migrations ORDER BY applied_seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	i := 0
	var lastSeq int64
	for rows.Next() {
		var v, dur int
		var name, kind, bin, by string
		var sum []byte
		var seq int64
		if err := rows.Scan(&v, &name, &kind, &sum, &seq, &dur, &bin, &by); err != nil {
			t.Fatal(err)
		}
		want := db.MustEmbedded()[i]
		if v != want.Version || name != want.Name || kind != string(want.Kind) || string(sum) != string(want.Checksum) ||
			seq <= lastSeq || dur < 0 || bin != "test-build-7" || by == "" {
			t.Errorf("row %d: %d %s %s seq=%d dur=%d bin=%s by=%s", i, v, name, kind, seq, dur, bin, by)
		}
		lastSeq = seq
		i++
	}
	if i != len(db.MustEmbedded()) {
		t.Fatalf("%d schema_migrations rows", i)
	}
	env, err := db.ReadEnvironment(ctx, d.Pool)
	if err != nil || env != "test" {
		t.Fatalf("instance_meta marker %q %v", env, err)
	}
	// A second run applies nothing.
	if steps, err := m.Up(ctx, db.UpOptions{}); err != nil || len(steps) != 0 {
		t.Fatalf("second up: %v %v", steps, err)
	}
	// Another environment cannot migrate this database.
	m.Environment = "production"
	if _, err := m.Up(ctx, db.UpOptions{}); !errors.Is(err, db.ErrEnvironmentMismatch) {
		t.Fatalf("environment mismatch not refused: %v", err)
	}
}

func TestMigrateStatus(t *testing.T) {
	t.Parallel()
	d := dbtest.NewEmpty(t)
	m := d.Migrator(db.MustEmbedded())
	st, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.ExitCode() != 3 || len(st.Pending) != len(db.MustEmbedded()) || st.Environment != "" {
		t.Fatalf("empty schema: exit %d pending %d env %q", st.ExitCode(), len(st.Pending), st.Environment)
	}
	// Status never creates anything.
	if tables := dbtest.Tables(t, d.Pool); len(tables) != 0 {
		t.Fatalf("status created tables: %v", tables)
	}
	if _, err := m.Up(ctx, db.UpOptions{To: 1}); err != nil {
		t.Fatal(err)
	}
	st, _ = m.Status(ctx)
	if st.ExitCode() != 3 || st.Entries[0].State != db.StateApplied || st.Entries[0].AppliedAt == nil || st.Entries[1].State != db.StatePending {
		t.Fatalf("partial: %+v", st.Entries)
	}
	if _, err := m.Up(ctx, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	st, _ = m.Status(ctx)
	if st.ExitCode() != 0 || !st.Current() || st.Environment != "test" {
		t.Fatalf("current: exit %d %+v", st.ExitCode(), st.Entries)
	}
	if peek, err := m.Peek(ctx); err != nil || !peek.Current() {
		t.Fatalf("peek: %v %v", peek, err)
	}
}

func TestMigrateChecksumMismatch(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	m := d.Migrator(db.MustEmbedded())
	if _, err := d.Pool.Exec(ctx, `UPDATE schema_migrations SET checksum = '\x00'::bytea WHERE version = 1`); err != nil {
		t.Fatal(err)
	}
	st, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.ExitCode() != 4 || st.Entries[0].State != db.StateChecksumMismatch {
		t.Fatalf("checksum: exit %d %+v", st.ExitCode(), st.Entries[0])
	}
	if _, err := m.Up(ctx, db.UpOptions{}); !errors.Is(err, db.ErrSchemaMismatch) {
		t.Fatalf("up accepted a mismatch: %v", err)
	}
	if _, err := m.DryRun(ctx); !errors.Is(err, db.ErrSchemaMismatch) {
		t.Fatalf("dry-run accepted a mismatch: %v", err)
	}

	// An applied version the binary does not embed: schema ahead of binary.
	d2 := dbtest.New(t)
	m2 := d2.Migrator(db.MustEmbedded())
	if _, err := d2.Pool.Exec(ctx, `INSERT INTO schema_migrations (version, name, kind, checksum, duration_ms, binary_version)
		VALUES (150, 'from_a_newer_binary', 'schema', '\x01'::bytea, 1, 'newer')`); err != nil {
		t.Fatal(err)
	}
	st, err = m2.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range st.Entries {
		found = found || (e.Version == 150 && e.State == db.StateUnknown)
	}
	if st.ExitCode() != 4 || !found {
		t.Fatalf("unknown version: exit %d %+v", st.ExitCode(), st.Entries)
	}
	if _, err := m2.Up(ctx, db.UpOptions{}); !errors.Is(err, db.ErrSchemaMismatch) {
		t.Fatalf("up accepted an unknown version: %v", err)
	}
	if _, err := m2.DownLast(ctx, 150, ""); !errors.Is(err, db.ErrSchemaMismatch) {
		t.Fatalf("down-last accepted an unknown version: %v", err)
	}
}

func TestMigrateConcurrentRunners(t *testing.T) {
	t.Parallel()
	d := dbtest.NewEmpty(t)
	// Four runners (separate pools, like separate instances) race on one
	// schema: each version is applied exactly once.
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	applied := make(chan int, 4)
	for i := 0; i < 4; i++ {
		pool := d.OpenPool(t, 2)
		m := &db.Migrator{Pool: pool, Migrations: db.MustEmbedded(), BinaryVersion: "test", Environment: "test", LockPoll: 5 * time.Millisecond}
		wg.Add(1)
		go func() {
			defer wg.Done()
			steps, err := m.Up(ctx, db.UpOptions{})
			errs <- err
			applied <- len(steps)
		}()
	}
	wg.Wait()
	close(errs)
	close(applied)
	total := 0
	for err := range errs {
		if err != nil {
			t.Fatalf("runner failed: %v", err)
		}
	}
	for n := range applied {
		total += n
	}
	var rows int
	_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&rows)
	if total != len(db.MustEmbedded()) || rows != total {
		t.Fatalf("applied %d steps, %d rows", total, rows)
	}

	// A held lock blocks another runner on the same schema, but not a
	// runner on a different schema.
	holder, err := d.Pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended('sela.migrate:' || current_schema(), 0))`); err != nil {
		t.Fatal(err)
	}
	blocked := d.Migrator(db.MustEmbedded())
	short, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	_, err = blocked.Status(short)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("runner on the locked schema did not wait: %v", err)
	}
	other := dbtest.NewEmpty(t)
	start := time.Now()
	if _, err := other.Migrator(db.MustEmbedded()).Up(ctx, db.UpOptions{}); err != nil {
		t.Fatalf("runner on another schema: %v", err)
	}
	if el := time.Since(start); el > 20*time.Second {
		t.Fatalf("other schema waited %s", el)
	}
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_unlock(hashtextextended('sela.migrate:' || current_schema(), 0))`); err != nil {
		t.Fatal(err)
	}
	if st, err := blocked.Status(ctx); err != nil || !st.Current() {
		t.Fatalf("after release: %v %v", st, err)
	}
}

func TestMigrateDownLast(t *testing.T) {
	t.Parallel()
	migs := embeddedPlus(t, map[string]string{
		"0100_talk.up.sql":   "-- sela:kind schema\n-- sela:requires 0001\nCREATE TABLE talk (id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE);\nREVOKE ALL ON talk FROM PUBLIC;",
		"0100_talk.down.sql": "DROP TABLE talk;",
	})
	d := dbtest.NewEmpty(t)
	base := d.Migrator(db.MustEmbedded())
	if _, err := base.Up(ctx, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	// 0100 is applied after 0900 (out of order), so it is the most recently
	// applied migration even though 0900 is the highest number.
	m := d.Migrator(migs)
	if st, _ := m.Status(ctx); st.ExitCode() != 3 || st.Pending[0].Version != 100 {
		t.Fatalf("0100 not pending: %+v", st)
	}
	if _, err := m.Up(ctx, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.DownLast(ctx, 900, ""); !errors.Is(err, db.ErrConfirmMismatch) || !strings.Contains(err.Error(), "0100_talk") {
		t.Fatalf("confirm of the highest number accepted: %v", err)
	}
	step, err := m.DownLast(ctx, 100, "")
	if err != nil || step.Version != 100 {
		t.Fatalf("down-last: %v %v", step, err)
	}
	var exists bool
	_ = d.Pool.QueryRow(ctx, `SELECT to_regclass('talk') IS NOT NULL`).Scan(&exists)
	if exists {
		t.Fatal("talk table still exists")
	}
	st, _ := m.Status(ctx)
	states := map[int]db.State{}
	for _, e := range st.Entries {
		states[e.Version] = e.State
	}
	if states[1] != db.StateApplied || states[900] != db.StateApplied || states[100] != db.StateOutOfOrder {
		t.Fatalf("only 0100 should be reverted: %v", states)
	}

	// Irreversible migrations are refused.
	irr := embeddedPlus(t, map[string]string{"0901_irr.up.sql": "-- sela:kind schema\n-- sela:irreversible\nCREATE TABLE irr (id int);"})
	mi := d.Migrator(irr)
	if _, err := mi.Up(ctx, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := mi.DownLast(ctx, 901, ""); !errors.Is(err, db.ErrIrreversible) {
		t.Fatalf("irreversible reverted: %v", err)
	}
}

func TestMigrateRoundTrip(t *testing.T) {
	t.Parallel()
	d := dbtest.NewEmpty(t)
	m := d.Migrator(db.MustEmbedded())
	if _, err := m.Up(ctx, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	before := dbtest.Catalog(t, d.Pool, d.Schema)
	for i := len(db.MustEmbedded()) - 1; i >= 0; i-- {
		v := db.MustEmbedded()[i].Version
		if _, err := m.DownLast(ctx, v, ""); err != nil {
			t.Fatalf("down-last %04d: %v", v, err)
		}
	}
	if _, err := m.DownLast(ctx, 1, ""); !errors.Is(err, db.ErrNothingApplied) {
		t.Fatalf("down-last on an empty history: %v", err)
	}
	if tables := dbtest.Tables(t, d.Pool); len(tables) != 1 || tables[0] != "schema_migrations" {
		t.Fatalf("after all downs: %v", tables)
	}
	if _, err := m.Up(ctx, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	after := dbtest.Catalog(t, d.Pool, d.Schema)
	if before != after {
		t.Fatalf("catalog differs after up -> down-last (all) -> up:\n--- before\n%s\n--- after\n%s", before, after)
	}
}

func TestMigrateDryRun(t *testing.T) {
	t.Parallel()
	d := dbtest.NewEmpty(t)
	m := d.Migrator(db.MustEmbedded())
	empty := dbtest.Catalog(t, d.Pool, d.Schema)
	rep, err := m.DryRun(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed || len(rep.Steps) != len(db.MustEmbedded()) {
		t.Fatalf("dry-run report: %+v", rep)
	}
	if got := dbtest.Catalog(t, d.Pool, d.Schema); got != empty {
		t.Fatalf("dry-run changed an empty schema:\n%s", got)
	}

	// On a partly migrated schema with a pending data migration.
	migs := embeddedPlus(t, map[string]string{
		"0901_fill.up.sql":       "-- sela:kind data\nUPDATE users SET locale = 'en' WHERE locale = 'id';",
		"0901_fill.down.sql":     "SELECT 1;",
		"0901_fill.validate.sql": "SELECT id FROM users WHERE locale <> 'en'",
	})
	if _, err := m.Up(ctx, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	insertUser(t, d)
	m2 := d.Migrator(migs)
	beforeCat := dbtest.Catalog(t, d.Pool, d.Schema)
	beforeRows := dumpTable(t, d, "SELECT version::text || ':' || applied_seq::text FROM schema_migrations ORDER BY 1")
	beforeUsers := dumpTable(t, d, "SELECT id::text || ':' || locale FROM users ORDER BY 1")
	rep, err = m2.DryRun(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed || len(rep.Steps) != 1 || rep.Steps[0].Version != 901 || !rep.Steps[0].HasValidate || rep.Steps[0].ValidationRows != 0 {
		t.Fatalf("dry-run of the data migration: %+v", rep)
	}
	if dbtest.Catalog(t, d.Pool, d.Schema) != beforeCat ||
		dumpTable(t, d, "SELECT version::text || ':' || applied_seq::text FROM schema_migrations ORDER BY 1") != beforeRows ||
		dumpTable(t, d, "SELECT id::text || ':' || locale FROM users ORDER BY 1") != beforeUsers {
		t.Fatal("dry-run changed the database")
	}
	if st, _ := m2.Status(ctx); st.ExitCode() != 3 {
		t.Fatal("dry-run recorded the migration")
	}

	// A no-transaction migration is reported as not dry-runnable.
	nt := embeddedPlus(t, map[string]string{
		"0902_idx.up.sql":   "-- sela:kind schema\n-- sela:no-transaction\nCREATE INDEX CONCURRENTLY users_locale ON users (locale);",
		"0902_idx.down.sql": "DROP INDEX users_locale;",
	})
	rep, err = d.Migrator(nt).DryRun(ctx)
	if err != nil || !rep.Failed || !strings.Contains(rep.Steps[len(rep.Steps)-1].Error, "cannot be dry-run") {
		t.Fatalf("no-transaction dry-run: %+v %v", rep, err)
	}
}

func insertUser(t *testing.T, d *dbtest.DB) {
	t.Helper()
	if _, err := d.Pool.Exec(ctx, `INSERT INTO users (id, kind, age_band, age_band_source, age_band_confirmed_at, locale, created_at, last_active_at)
		VALUES ('6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11', 'account', '18_plus', 'self_declared', now(), 'id', now(), now())`); err != nil {
		t.Fatal(err)
	}
}

func dumpTable(t *testing.T, d *dbtest.DB, sql string) string {
	t.Helper()
	rows, err := d.Pool.Query(ctx, sql)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(out, ",")
}

func TestDataMigrationGates(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	insertUser(t, d)
	fill := embeddedPlus(t, map[string]string{
		"0901_fill.up.sql":       "-- sela:kind data\nUPDATE users SET locale = 'en';",
		"0901_fill.down.sql":     "UPDATE users SET locale = 'id';",
		"0901_fill.validate.sql": "SELECT id FROM users WHERE locale <> 'en'",
	})
	m := d.Migrator(fill)
	// Refused without a backup reference.
	if _, err := m.Up(ctx, db.UpOptions{}); !errors.Is(err, db.ErrBackupRefRequired) {
		t.Fatalf("data migration without --backup-ref: %v", err)
	}
	// Never applied at server start (schema-only runs).
	if _, err := m.Up(ctx, db.UpOptions{SchemaOnly: true, BackupRef: "x"}); !errors.Is(err, db.ErrDataMigrationPending) {
		t.Fatalf("schema-only run applied a data migration: %v", err)
	}
	if _, err := m.Up(ctx, db.UpOptions{BackupRef: "dump-2026-10-26T12:00Z.sha256-abc"}); err != nil {
		t.Fatal(err)
	}
	var ref string
	_ = d.Pool.QueryRow(ctx, `SELECT backup_ref FROM schema_migrations WHERE version = 901`).Scan(&ref)
	if ref != "dump-2026-10-26T12:00Z.sha256-abc" {
		t.Fatalf("backup_ref %q", ref)
	}
	// Reverting a data migration also needs a backup reference.
	if _, err := m.DownLast(ctx, 901, ""); !errors.Is(err, db.ErrBackupRefRequired) {
		t.Fatalf("down-last of a data migration without --backup-ref: %v", err)
	}
	if _, err := m.DownLast(ctx, 901, "dump-2"); err != nil {
		t.Fatal(err)
	}

	// A validate file that returns rows rolls the migration back.
	bad := embeddedPlus(t, map[string]string{
		"0902_bad.up.sql":       "-- sela:kind data\nUPDATE users SET locale = 'en';",
		"0902_bad.down.sql":     "SELECT 1;",
		"0902_bad.validate.sql": "SELECT id FROM users WHERE locale = 'en'",
	})
	if _, err := d.Migrator(bad).Up(ctx, db.UpOptions{BackupRef: "dump-3"}); !errors.Is(err, db.ErrValidationFailed) {
		t.Fatalf("validation rows did not fail the run: %v", err)
	}
	var locale string
	_ = d.Pool.QueryRow(ctx, `SELECT locale FROM users`).Scan(&locale)
	var n int
	_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version = 902`).Scan(&n)
	if locale != "id" || n != 0 {
		t.Fatalf("failed validation left changes: locale %s rows %d", locale, n)
	}
	// The validate file is mandatory and no-transaction is refused, at parse time.
	for name, files := range map[string]fstest.MapFS{
		"no validate": {"m/0901_x.up.sql": {Data: []byte("-- sela:kind data\nSELECT 1;")}, "m/0901_x.down.sql": {Data: []byte("SELECT 1;")}},
		"no-transaction": {"m/0901_x.up.sql": {Data: []byte("-- sela:kind data\n-- sela:no-transaction\nSELECT 1;")},
			"m/0901_x.down.sql": {Data: []byte("SELECT 1;")}, "m/0901_x.validate.sql": {Data: []byte("SELECT 1 WHERE false")}},
	} {
		if _, err := db.Parse(files, "m"); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestSchemaOwnedTablesCascade(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	allow := map[string]bool{"user_data_keys": true}
	owned := dbtest.OwnedTables(t, d.Pool)
	if len(owned) < 8 {
		t.Fatalf("owned tables: %v", owned)
	}
	for _, tbl := range owned {
		var rule string
		err := d.Pool.QueryRow(ctx, `SELECT rc.delete_rule FROM information_schema.key_column_usage k
			JOIN information_schema.referential_constraints rc ON rc.constraint_schema = k.constraint_schema AND rc.constraint_name = k.constraint_name
			JOIN information_schema.constraint_column_usage u ON u.constraint_schema = rc.unique_constraint_schema AND u.constraint_name = rc.unique_constraint_name
			WHERE k.table_schema = current_schema() AND k.table_name = $1 AND k.column_name = 'user_id' AND u.table_name = 'users' AND u.column_name = 'id'`, tbl).Scan(&rule)
		switch {
		case allow[tbl]:
			if err == nil {
				t.Errorf("%s: allow-listed table must not reference users (it is the deletion tombstone)", tbl)
			}
		case err != nil:
			t.Errorf("%s: user_id does not reference users(id): %v", tbl, err)
		case rule != "CASCADE":
			t.Errorf("%s: ON DELETE %s, want CASCADE", tbl, rule)
		}
	}
	for _, tbl := range dbtest.Tables(t, d.Pool) {
		var acl *string
		if err := d.Pool.QueryRow(ctx, `SELECT relacl::text FROM pg_class WHERE oid = $1::regclass`, tbl).Scan(&acl); err != nil {
			t.Fatal(err)
		}
		// NULL means default privileges (owner only, PUBLIC has nothing on tables);
		// an explicit ACL must not grant PUBLIC ("=..." entries).
		if acl != nil && (strings.HasPrefix(*acl, "{=") || strings.Contains(*acl, ",=")) {
			t.Errorf("%s grants PUBLIC: %s", tbl, *acl)
		}
		var publicPrivs int
		if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.role_table_grants WHERE table_schema = current_schema()
			AND table_name = $1 AND grantee = 'PUBLIC'`, tbl).Scan(&publicPrivs); err != nil || publicPrivs != 0 {
			t.Errorf("%s: %d PUBLIC privileges (%v)", tbl, publicPrivs, err)
		}
		if !strings.Contains(migrationSQL(), "REVOKE ALL ON") || !revokes(tbl) {
			t.Errorf("%s: no REVOKE ... FROM PUBLIC in its migration", tbl)
		}
	}
}

func migrationSQL() string {
	var b strings.Builder
	for _, m := range db.MustEmbedded() {
		b.WriteString(m.Up)
	}
	return b.String()
}

// revokes checks the migrations name the table in a REVOKE ... FROM PUBLIC.
func revokes(tbl string) bool {
	if tbl == "schema_migrations" {
		return true // the runner revokes it
	}
	sql := migrationSQL()
	for _, stmt := range strings.Split(sql, ";") {
		s := strings.Join(strings.Fields(stmt), " ")
		if strings.HasPrefix(s, "REVOKE ALL ON ") && strings.HasSuffix(s, " FROM PUBLIC") {
			list := strings.TrimSuffix(strings.TrimPrefix(s, "REVOKE ALL ON "), " FROM PUBLIC")
			for _, name := range strings.Split(list, ",") {
				if strings.TrimSpace(name) == tbl {
					return true
				}
			}
		}
	}
	return false
}

func TestAppendOnlyTriggers(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	insertUser(t, d)
	uid := "6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11"
	if _, err := d.Pool.Exec(ctx, `INSERT INTO audit_events (id, user_id, at, type, actor) VALUES ('00000000-0000-4000-8000-000000000001', $1, now(), 'auth.logout', 'user')`, uid); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(ctx, `INSERT INTO consent_events (id, user_id, purpose, decision, policy_version, source, decided_at)
		VALUES ('00000000-0000-4000-8000-000000000002', $1, 'terms_privacy', 'granted', 'v1', 'onboarding', now())`, uid); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{`UPDATE audit_events SET type = 'auth.login_succeeded'`, `UPDATE consent_events SET decision = 'withdrawn'`} {
		_, err := d.Pool.Exec(ctx, sql)
		if err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s: %v", sql, err)
		}
	}
	// Deleting the user still removes both through the cascade.
	if _, err := d.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = d.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM audit_events) + (SELECT count(*) FROM consent_events)`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d append-only rows left after the cascade", n)
	}
}

// R1-1: down-last checks the environment marker like up and dry-run, and
// status reports another environment's database as a mismatch (exit 4).
func TestEnvironmentMarkerEveryCommand(t *testing.T) {
	t.Parallel()
	d := dbtest.NewEmpty(t)
	prod := &db.Migrator{Pool: d.Pool, Migrations: db.MustEmbedded(), BinaryVersion: "test", Environment: "production"}
	if _, err := prod.Up(ctx, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	staging := &db.Migrator{Pool: d.Pool, Migrations: db.MustEmbedded(), BinaryVersion: "test", Environment: "staging"}
	if _, err := staging.Up(ctx, db.UpOptions{}); !errors.Is(err, db.ErrEnvironmentMismatch) {
		t.Fatalf("up: %v", err)
	}
	if _, err := staging.DryRun(ctx); !errors.Is(err, db.ErrEnvironmentMismatch) {
		t.Fatalf("dry-run: %v", err)
	}
	for _, v := range []int{900, 2, 1} {
		if _, err := staging.DownLast(ctx, v, "ref"); !errors.Is(err, db.ErrEnvironmentMismatch) {
			t.Fatalf("down-last %04d on another environment's database: %v", v, err)
		}
	}
	if tables := dbtest.Tables(t, d.Pool); !slicesContains(tables, "users") || !slicesContains(tables, "ops_canary") {
		t.Fatalf("tables after refused down-last: %v", tables)
	}
	st, err := staging.Status(ctx)
	if err != nil || !st.EnvironmentMismatch() || st.ExitCode() != 4 || st.Current() {
		t.Fatalf("status: mismatch=%t exit=%d current=%t %v", st.EnvironmentMismatch(), st.ExitCode(), st.Current(), err)
	}
	if st, err := prod.Status(ctx); err != nil || st.EnvironmentMismatch() || st.ExitCode() != 0 {
		t.Fatalf("own environment: %+v %v", st, err)
	}
	if err := db.CheckEnvironment(ctx, d.Pool, "staging"); !errors.Is(err, db.ErrEnvironmentMismatch) {
		t.Fatalf("CheckEnvironment: %v", err)
	}
	if err := db.CheckEnvironment(ctx, dbtest.NewEmpty(t).Pool, "staging"); err != nil {
		t.Fatalf("an unmarked database: %v", err)
	}
	// The production runner can still revert its own migration.
	if _, err := prod.DownLast(ctx, 900, ""); err != nil {
		t.Fatal(err)
	}
}

func slicesContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// R1-6: should a transaction-control statement get past the parser (here
// a hand-built migration), up, dry-run and down-last stop with
// ErrTransactionEnded instead of reporting a rollback that did not happen.
func TestRuntimeGuardTransactionEnded(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	insertUser(t, d)
	sum := make([]byte, 32)
	bad := append(append([]db.Migration(nil), db.MustEmbedded()...), db.Migration{Version: 901, Name: "sneaky", Kind: db.KindData,
		Up: "UPDATE users SET locale = 'en';\nCOMMIT;\n", Validate: "SELECT id FROM users WHERE locale = 'en'", HasValidate: true,
		Down: "SELECT 1;", HasDown: true, Checksum: sum})
	m := d.Migrator(bad)
	rep, err := m.DryRun(ctx)
	if !errors.Is(err, db.ErrTransactionEnded) || !rep.Failed {
		t.Fatalf("dry-run: %+v %v", rep, err)
	}
	if _, err := m.Up(ctx, db.UpOptions{BackupRef: "ref"}); !errors.Is(err, db.ErrTransactionEnded) {
		t.Fatalf("up: %v", err)
	}
	var n int
	_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version = 901`).Scan(&n)
	if n != 0 {
		t.Fatal("the migration was recorded")
	}
	// down-last of a migration whose down file commits.
	good := append(append([]db.Migration(nil), db.MustEmbedded()...), db.Migration{Version: 902, Name: "down_commits", Kind: db.KindSchema,
		Up: "CREATE TABLE extra (id int);", Down: "DROP TABLE extra; COMMIT;", HasDown: true, Checksum: sum})
	m2 := d.Migrator(good)
	if _, err := m2.Up(ctx, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := m2.DownLast(ctx, 902, ""); !errors.Is(err, db.ErrTransactionEnded) {
		t.Fatalf("down-last: %v", err)
	}
}

// R2-1: a file that ends with COMMIT; BEGIN; is in a transaction again,
// just not the runner's. The runner compares the transaction id before
// and after every file, so dry-run, up and down-last stop with
// ErrTransactionEnded instead of reporting a rollback, and nothing is
// recorded.
func TestRuntimeGuardDetectsNewTransaction(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	sum := make([]byte, 32)
	rebegin := append(append([]db.Migration(nil), db.MustEmbedded()...), db.Migration{Version: 901, Name: "rebegin", Kind: db.KindData,
		Up: "CREATE TABLE r2_rebegin (id int);\nCOMMIT;\nBEGIN;\n", Validate: "SELECT 1", HasValidate: true,
		Down: "SELECT 1;", HasDown: true, Checksum: sum})
	m := d.Migrator(rebegin)
	rep, err := m.DryRun(ctx)
	if !errors.Is(err, db.ErrTransactionEnded) || !rep.Failed {
		t.Fatalf("dry-run: %+v %v", rep, err)
	}
	if _, err := d.Pool.Exec(ctx, `DROP TABLE IF EXISTS r2_rebegin`); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Up(ctx, db.UpOptions{BackupRef: "ref"}); !errors.Is(err, db.ErrTransactionEnded) || errors.Is(err, db.ErrValidationFailed) {
		t.Fatalf("up: %v", err)
	}
	var n int
	_ = d.Pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version = 901`).Scan(&n)
	if n != 0 {
		t.Fatal("the migration was recorded")
	}
	// The validate file is checked the same way.
	viaValidate := append(append([]db.Migration(nil), db.MustEmbedded()...), db.Migration{Version: 902, Name: "validate_commits", Kind: db.KindData,
		Up: "SELECT 1;", Validate: "COMMIT", HasValidate: true, Down: "SELECT 1;", HasDown: true, Checksum: sum})
	if _, err := d.Migrator(viaValidate).Up(ctx, db.UpOptions{BackupRef: "ref"}); !errors.Is(err, db.ErrTransactionEnded) {
		t.Fatalf("up with a committing validate file: %v", err)
	}
	// down-last of a migration whose down file commits and begins again.
	good := append(append([]db.Migration(nil), db.MustEmbedded()...), db.Migration{Version: 903, Name: "down_rebegins", Kind: db.KindSchema,
		Up: "CREATE TABLE extra_r2 (id int);", Down: "DROP TABLE extra_r2; COMMIT; BEGIN;", HasDown: true, Checksum: sum})
	m2 := d.Migrator(good)
	if _, err := m2.Up(ctx, db.UpOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := m2.DownLast(ctx, 903, ""); !errors.Is(err, db.ErrTransactionEnded) {
		t.Fatalf("down-last: %v", err)
	}
	// The probe of the review: the parser refuses the hidden variant before
	// anything runs.
	if _, err := db.Parse(fstest.MapFS{
		"m/0003_r2_hidden.up.sql":       {Data: []byte("-- sela:kind data\nCREATE FUNCTION begin() RETURNS int LANGUAGE sql AS 'SELECT 1';\nCREATE TABLE r2_persisted (id int);\nCOMMIT;\nBEGIN;\nCREATE TABLE r2_after (id int);\n")},
		"m/0003_r2_hidden.down.sql":     {Data: []byte("DROP TABLE IF EXISTS r2_after;\n")},
		"m/0003_r2_hidden.validate.sql": {Data: []byte("SELECT 1\n")},
	}, "m"); err == nil {
		t.Fatal("the parser accepted COMMIT; BEGIN; hidden behind a function named begin")
	}
}
