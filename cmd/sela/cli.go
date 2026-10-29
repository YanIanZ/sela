package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/config"
	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/vault"
)

const usage = `usage:
  sela                                   serve (reads SELA_* from the environment)
  sela migrate status                    exit 0 current, 3 pending, 4 mismatch
  sela migrate up [--to NNNN] [--backup-ref REF]
  sela migrate dry-run                   apply pending migrations in one transaction, then roll back
  sela migrate down-last --confirm NNNN [--backup-ref REF]
  sela keys generate-kek [--env ENV]     print a new KEK for the secret manager
  sela keys status                       data keys per KEK id
  sela keys rewrap [--batch 500]         re-wrap every DEK with the current KEK
  sela restore reconcile [--dry-run]     delete restored users whose key is destroyed or missing
  sela backup check --content FILE --keys FILE
  sela admin create --username NAME --role ROLE [--org ORG]   (password on stdin)
  sela admin revoke --username NAME [--reason TEXT]
  sela ops canary write|check`

// dispatch runs the server or one operator subcommand and returns the
// process exit code.
func dispatch(args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 || args[0] == "serve" {
		return 0, run()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch args[0] {
	case "migrate":
		return cmdMigrate(ctx, args[1:], stdout)
	case "keys":
		return cmdKeys(ctx, args[1:], stdout)
	case "restore":
		return cmdRestore(ctx, args[1:], stdout)
	case "backup":
		return cmdBackup(args[1:], stdout)
	case "admin":
		return cmdAdmin(ctx, args[1:], os.Stdin, stdout)
	case "ops":
		return cmdOps(ctx, args[1:], stdout)
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, usage)
		return 0, nil
	}
	fmt.Fprintln(stderr, usage)
	return 2, fmt.Errorf("unknown command %q", args[0])
}

func opsLogger(w io.Writer) *slog.Logger { return slog.New(slog.NewJSONHandler(w, nil)) }

// opsDatabase loads the operator configuration and opens the pool. With
// checkMarker it also refuses a database whose instance_meta marker names
// another environment than SELA_ENV (ADR-0002 §12), so a command configured
// for staging can never write keys or a canary into production's database,
// or reconcile it. The migrate commands pass false: their runner checks the
// marker itself, under the migration lock, and status reports it.
func opsDatabase(ctx context.Context, checkMarker bool) (config.Config, *pgxpool.Pool, error) {
	cfg, err := config.LoadMode(config.ModeOps)
	if err != nil {
		return cfg, nil, err
	}
	if cfg.DatabaseURL == "" {
		return cfg, nil, errors.New("SELA_DATABASE_URL is required")
	}
	pool, err := openDatabase(ctx, cfg)
	if err != nil {
		return cfg, nil, err
	}
	if checkMarker {
		if err := db.CheckEnvironment(ctx, pool, cfg.Env); err != nil {
			pool.Close()
			if errors.Is(err, db.ErrEnvironmentMismatch) {
				return cfg, nil, fmt.Errorf("refusing to touch another environment's database: %w", err)
			}
			return cfg, nil, fmt.Errorf("cannot read the environment marker (%s)", db.Classify(err))
		}
	}
	return cfg, pool, nil
}

// exitFor maps an operator error to the exit code: 4 for a schema or
// environment mismatch, 1 otherwise.
func exitFor(err error) int {
	if errors.Is(err, db.ErrSchemaMismatch) || errors.Is(err, db.ErrEnvironmentMismatch) || errors.Is(err, db.ErrDownChecksumMismatch) {
		return 4
	}
	return 1
}

func parseVersion(s string) (int, error) {
	if len(s) != 4 {
		return 0, fmt.Errorf("version %q: use four digits, e.g. 0001", s)
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("version %q: use four digits, e.g. 0001", s)
	}
	return n, nil
}

func cmdMigrate(ctx context.Context, args []string, out io.Writer) (int, error) {
	if len(args) == 0 {
		return 2, errors.New("migrate: status, up, dry-run or down-last")
	}
	fs := flag.NewFlagSet("migrate "+args[0], flag.ContinueOnError)
	fs.SetOutput(out)
	to := fs.String("to", "", "stop after this version")
	backupRef := fs.String("backup-ref", "", "reference of the verified pg_dump (data migrations)")
	confirm := fs.String("confirm", "", "the most recently applied version")
	if err := fs.Parse(args[1:]); err != nil {
		return 2, err
	}
	cfg, pool, err := opsDatabase(ctx, false)
	if err != nil {
		return exitFor(err), err
	}
	defer pool.Close()
	m := &db.Migrator{Pool: pool, Migrations: db.MustEmbedded(), BinaryVersion: version, Environment: cfg.Env, Logger: opsLogger(io.Discard)}
	switch args[0] {
	case "status":
		st, err := m.Status(ctx)
		if err != nil {
			return 1, err
		}
		tw := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "VERSION\tNAME\tKIND\tSTATE\tAPPLIED_AT\tDETAIL")
		for _, e := range st.Entries {
			at := "-"
			if e.AppliedAt != nil {
				at = e.AppliedAt.UTC().Format(time.RFC3339)
			}
			fmt.Fprintf(tw, "%04d\t%s\t%s\t%s\t%s\t%s\n", e.Version, e.Name, e.Kind, e.State, at, e.Detail)
		}
		_ = tw.Flush()
		env := st.Environment
		if env == "" {
			env = "(not written yet)"
		}
		if st.EnvironmentMismatch() {
			env += fmt.Sprintf(" (SELA_ENV is %s: another environment's database)", cfg.Env)
		}
		fmt.Fprintf(out, "environment marker: %s\nresult: %s\n", env, map[int]string{0: "current", 3: "pending", 4: "mismatch"}[st.ExitCode()])
		return st.ExitCode(), nil
	case "up":
		n := 0
		if *to != "" {
			if n, err = parseVersion(*to); err != nil {
				return 2, err
			}
		}
		steps, err := m.Up(ctx, db.UpOptions{To: n, BackupRef: *backupRef})
		for _, s := range steps {
			fmt.Fprintf(out, "applied %04d_%s (%s) in %d ms\n", s.Version, s.Name, s.Kind, s.Duration.Milliseconds())
		}
		if err != nil {
			return exitFor(err), err
		}
		if len(steps) == 0 {
			fmt.Fprintln(out, "nothing to apply")
		}
		return 0, nil
	case "dry-run":
		rep, err := m.DryRun(ctx)
		if err != nil {
			return exitFor(err), err
		}
		for _, s := range rep.Steps {
			v := "no validate file"
			if s.HasValidate {
				v = fmt.Sprintf("validate: %d rows", s.ValidationRows)
			}
			status := "ok"
			if s.Error != "" {
				status = "FAILED: " + s.Error
			}
			fmt.Fprintf(out, "%04d_%s (%s) %d ms, %s, %s\n", s.Version, s.Name, s.Kind, s.Duration.Milliseconds(), v, status)
		}
		fmt.Fprintf(out, "dry run: %d pending, rolled back; nothing changed\n", len(rep.Steps))
		if rep.Failed {
			return 1, errors.New("dry run failed")
		}
		return 0, nil
	case "down-last":
		n, err := parseVersion(*confirm)
		if err != nil {
			return 2, fmt.Errorf("down-last needs --confirm NNNN: %w", err)
		}
		s, err := m.DownLast(ctx, n, *backupRef)
		if err != nil {
			return exitFor(err), err
		}
		fmt.Fprintf(out, "reverted %04d_%s in %d ms\n", s.Version, s.Name, s.Duration.Milliseconds())
		return 0, nil
	}
	return 2, fmt.Errorf("migrate: unknown subcommand %q", args[0])
}

func cmdKeys(ctx context.Context, args []string, out io.Writer) (int, error) {
	if len(args) == 0 {
		return 2, errors.New("keys: generate-kek, status or rewrap")
	}
	fs := flag.NewFlagSet("keys "+args[0], flag.ContinueOnError)
	fs.SetOutput(out)
	envName := fs.String("env", os.Getenv("SELA_ENV"), "environment the key is for")
	batch := fs.Int("batch", 500, "rows per transaction")
	if err := fs.Parse(args[1:]); err != nil {
		return 2, err
	}
	if args[0] == "generate-kek" {
		if *envName == "" {
			*envName = config.EnvProduction
		}
		prefix, ok := config.KEKPrefix[*envName]
		if !ok {
			return 2, fmt.Errorf("unknown environment %q", *envName)
		}
		k, err := vault.GenerateKEK()
		if err != nil {
			return 1, err
		}
		fmt.Fprintf(out, "# Store both values in the secret manager; never commit them.\nSELA_DATA_KEK=%s\nSELA_DATA_KEK_ID=%s-%s\n",
			k, prefix, time.Now().UTC().Format("20060102"))
		return 0, nil
	}
	cfg, pool, err := opsDatabase(ctx, true)
	if err != nil {
		return exitFor(err), err
	}
	defer pool.Close()
	v, err := newVault(cfg)
	if err != nil {
		return 1, err
	}
	switch args[0] {
	case "status":
		st, err := v.KeyStatus(ctx, pool)
		if err != nil {
			return 1, err
		}
		ids := make([]string, 0, len(st.ByKEK))
		for id := range st.ByKEK {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		fmt.Fprintf(out, "current KEK: %s\n", st.CurrentKEKID)
		for _, id := range ids {
			fmt.Fprintf(out, "data keys under %s: %d\n", id, st.ByKEK[id])
		}
		fmt.Fprintf(out, "data keys under other KEKs: %d\ndestroyed (tombstones): %d\n", st.UnderOtherKEKs(), st.Destroyed)
		return 0, nil
	case "rewrap":
		n, err := v.Rewrap(ctx, pool, *batch, func(done int) { fmt.Fprintf(out, "re-wrapped %d\n", done) })
		if err != nil {
			return 1, err
		}
		fmt.Fprintf(out, "done: %d data keys re-wrapped to %s\n", n, v.CurrentKEKID())
		return 0, nil
	}
	return 2, fmt.Errorf("keys: unknown subcommand %q", args[0])
}

func opsStore(ctx context.Context, cfg config.Config, pool *pgxpool.Pool) (*account.Store, error) {
	v, err := newVault(cfg)
	if err != nil {
		return nil, err
	}
	return account.New(pool, v, accountConfig(cfg, opsLogger(io.Discard)))
}

func cmdRestore(ctx context.Context, args []string, out io.Writer) (int, error) {
	if len(args) == 0 || args[0] != "reconcile" {
		return 2, errors.New("restore: reconcile [--dry-run]")
	}
	fs := flag.NewFlagSet("restore reconcile", flag.ContinueOnError)
	fs.SetOutput(out)
	dry := fs.Bool("dry-run", false, "count only")
	if err := fs.Parse(args[1:]); err != nil {
		return 2, err
	}
	cfg, pool, err := opsDatabase(ctx, true)
	if err != nil {
		return exitFor(err), err
	}
	defer pool.Close()
	s, err := opsStore(ctx, cfg, pool)
	if err != nil {
		return 1, err
	}
	rep, err := s.ReconcileRestore(ctx, *dry)
	if err != nil {
		return 1, err
	}
	fmt.Fprintf(out, "users with a destroyed key: %d\nusers with a missing key: %d\nkeys without a user: %d\nsessions not ended: %d\n",
		rep.DestroyedKey, rep.MissingKey, rep.OrphanKeys, rep.LiveSessions)
	fmt.Fprintln(out, "note: only deletions recorded in the restored key table are seen; restore the newest key dump (RUNBOOK §8)")
	if *dry {
		fmt.Fprintln(out, "dry run: nothing deleted, destroyed or revoked")
		return 0, nil
	}
	fmt.Fprintf(out, "deleted through the deletion path: %d\nkeys without a user destroyed: %d\nsessions revoked (everyone signs in again): %d\n",
		rep.Deleted, rep.KeysDestroyed, rep.SessionsRevoked)
	fmt.Fprintln(out, "tell users: password changes, recoveries and recovery-code regenerations made after the content dump was taken were lost; repeat them (RUNBOOK §8)")
	return 0, nil
}

// canary is the synthetic record "sela ops canary" seals: its key row has
// no users row, its text is not user data.
var canaryRef = vault.Ref{Kind: "ops_canary", RecordID: "1", Field: "canary"}

const canaryPrefix = "sela-canary-v1:"

func cmdOps(ctx context.Context, args []string, out io.Writer) (int, error) {
	if len(args) != 2 || args[0] != "canary" || (args[1] != "write" && args[1] != "check") {
		return 2, errors.New("ops: canary write|check")
	}
	cfg, pool, err := opsDatabase(ctx, true)
	if err != nil {
		return exitFor(err), err
	}
	defer pool.Close()
	v, err := newVault(cfg)
	if err != nil {
		return 1, err
	}
	if args[1] == "write" {
		if err := canaryWrite(ctx, pool, v, cfg.Env); err != nil {
			return 1, err
		}
		fmt.Fprintln(out, "canary written")
		return 0, nil
	}
	if err := canaryCheck(ctx, pool, v); err != nil {
		return 1, err
	}
	fmt.Fprintln(out, "canary ok: the KEK opened the data key and the data key opened the record")
	return 0, nil
}

func canaryWrite(ctx context.Context, pool *pgxpool.Pool, v *vault.Vault, env string) error {
	var keyID [16]byte
	_, _ = rand.Read(keyID[:])
	keyID[6], keyID[8] = keyID[6]&0x0f|0x40, keyID[8]&0x3f|0x80
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	pt := []byte(canaryPrefix + env + ":" + hex.EncodeToString(nonce))
	digest := sha256.Sum256(pt)
	return db.InTx(ctx, pool, func(tx pgx.Tx) error {
		var old pgtype.UUID
		err := tx.QueryRow(ctx, `SELECT key_id FROM ops_canary FOR UPDATE`).Scan(&old)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if old.Valid {
			if _, err := tx.Exec(ctx, `DELETE FROM user_data_keys WHERE user_id = $1`, old); err != nil {
				return err
			}
		}
		if err := v.CreateUserKey(ctx, tx, keyID); err != nil {
			return err
		}
		sealed, err := v.Seal(ctx, tx, keyID, canaryRef, pt)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO ops_canary (singleton, key_id, sealed, digest, created_at) VALUES (true, $1, $2, $3, now())
			ON CONFLICT (singleton) DO UPDATE SET key_id = EXCLUDED.key_id, sealed = EXCLUDED.sealed, digest = EXCLUDED.digest,
			created_at = EXCLUDED.created_at`, pgtype.UUID{Bytes: keyID, Valid: true}, sealed, digest[:])
		return err
	})
}

func canaryCheck(ctx context.Context, pool *pgxpool.Pool, v *vault.Vault) error {
	var keyID pgtype.UUID
	var sealed, digest []byte
	err := pool.QueryRow(ctx, `SELECT key_id, sealed, digest FROM ops_canary`).Scan(&keyID, &sealed, &digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("no canary in this database; run sela ops canary write")
	}
	if err != nil {
		return err
	}
	pt, err := v.Open(ctx, pool, keyID.Bytes, canaryRef, sealed)
	if err != nil {
		return fmt.Errorf("canary cannot be opened: %w", err)
	}
	got := sha256.Sum256(pt)
	if !bytes.Equal(got[:], digest) || !bytes.HasPrefix(pt, []byte(canaryPrefix)) {
		return errors.New("canary content does not match its digest")
	}
	return nil
}

func cmdBackup(args []string, out io.Writer) (int, error) {
	if len(args) == 0 || args[0] != "check" {
		return 2, errors.New("backup: check --content FILE --keys FILE")
	}
	fs := flag.NewFlagSet("backup check", flag.ContinueOnError)
	fs.SetOutput(out)
	content := fs.String("content", "", "content dump (pg_dump -Fc --exclude-table=user_data_keys)")
	keys := fs.String("keys", "", "key dump (pg_dump -Fc --table=user_data_keys)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2, err
	}
	if *content == "" || *keys == "" {
		return 2, errors.New("backup check needs --content and --keys")
	}
	rep, err := backupCheck(*content, *keys)
	if err != nil {
		return 1, err
	}
	fmt.Fprintf(out, "content dump: %d tables, %d table data entries, no user_data_keys (taken %s)\nkey dump: user_data_keys only (taken %s)\n",
		rep.contentTables, rep.contentData, rep.contentCreated, rep.keysCreated)
	return 0, nil
}

type tocEntry struct{ kind, schema, name string }

var (
	tocLine        = regexp.MustCompile(`^\d+; \d+ \d+ (TABLE DATA|TABLE) (\S+) (\S+) `)
	archiveCreated = regexp.MustCompile(`^;\s+Archive created at (\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`)
)

// archive is what pg_restore --list reports about a dump: its TABLE and
// TABLE DATA entries and when it was taken. The time is the dumping
// host's wall clock (scripts/ops/backup.sh dumps with TZ=UTC), so two
// dumps of one backup job compare correctly as strings.
type archive struct {
	entries []tocEntry
	created string
}

// pgRestoreList runs pg_restore --list, which also proves the archive is
// readable.
func pgRestoreList(path string) (archive, error) {
	var a archive
	bin, err := exec.LookPath("pg_restore")
	if err != nil {
		return a, errors.New("pg_restore not found in PATH")
	}
	var stderr bytes.Buffer
	cmd := exec.Command(bin, "--list", path)
	cmd.Stderr = &stderr
	outb, err := cmd.Output()
	if err != nil {
		return a, fmt.Errorf("pg_restore --list %s failed: %v", path, strings.TrimSpace(stderr.String()))
	}
	sc := bufio.NewScanner(bytes.NewReader(outb))
	for sc.Scan() {
		if m := tocLine.FindStringSubmatch(sc.Text()); m != nil {
			a.entries = append(a.entries, tocEntry{kind: m[1], schema: m[2], name: m[3]})
		} else if m := archiveCreated.FindStringSubmatch(sc.Text()); m != nil && a.created == "" {
			a.created = m[1]
		}
	}
	if err := sc.Err(); err != nil {
		return a, err
	}
	if a.created == "" {
		return a, fmt.Errorf("pg_restore --list %s: no archive creation time", path)
	}
	return a, nil
}

type backupReport struct {
	contentTables, contentData  int
	contentCreated, keysCreated string
}

// backupCheck verifies the two-dump layout (ADR-0002 §11): both archives
// pass pg_restore --list; the content dump has tables and data but no
// user_data_keys; the key dump has user_data_keys (definition and data)
// and nothing else; and the key dump is not older than the content dump.
// Restore reconcile only sees deletions recorded in the key table it is
// given, so a key dump older than the content would let accounts deleted
// in between come back.
func backupCheck(contentPath, keysPath string) (backupReport, error) {
	var rep backupReport
	ca, err := pgRestoreList(contentPath)
	if err != nil {
		return rep, err
	}
	ka, err := pgRestoreList(keysPath)
	if err != nil {
		return rep, err
	}
	content, keys := ca.entries, ka.entries
	rep.contentCreated, rep.keysCreated = ca.created, ka.created
	if ka.created < ca.created {
		return rep, fmt.Errorf("the key dump (taken %s) is older than the content dump (taken %s); restore with the newest key dump", ka.created, ca.created)
	}
	for _, e := range content {
		if e.name == "user_data_keys" {
			return rep, errors.New("content dump contains user_data_keys; dump it with --exclude-table=user_data_keys")
		}
		if e.kind == "TABLE" {
			rep.contentTables++
		} else {
			rep.contentData++
		}
	}
	if rep.contentTables == 0 || rep.contentData == 0 {
		return rep, errors.New("content dump has no tables or no table data")
	}
	var table, data bool
	for _, e := range keys {
		if e.name != "user_data_keys" {
			return rep, fmt.Errorf("key dump contains %s %s; dump only --table=user_data_keys", e.kind, e.name)
		}
		table = table || e.kind == "TABLE"
		data = data || e.kind == "TABLE DATA"
	}
	if !table || !data {
		return rep, errors.New("key dump lacks the user_data_keys table or its data")
	}
	return rep, nil
}
