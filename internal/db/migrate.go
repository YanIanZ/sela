package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Kind classifies a migration. Data migrations rewrite or move existing
// user rows and pass extra gates (backup reference, validate file).
type Kind string

// Migration kinds.
const (
	KindSchema Kind = "schema"
	KindData   Kind = "data"
)

// Migration is one embedded, parsed migration.
type Migration struct {
	Version       int
	Name          string
	Kind          Kind
	Requires      []int
	Irreversible  bool
	NoTransaction bool
	Up            string
	Down          string
	Validate      string
	HasDown       bool
	HasValidate   bool
	// Checksum is the SHA-256 of the up file with CRLF normalised to LF.
	Checksum []byte
	// DownChecksum is the SHA-256 of the down file, nil without one.
	DownChecksum []byte
}

// Label is the file stem, e.g. 0001_foundation.
func (m Migration) Label() string { return fmt.Sprintf("%04d_%s", m.Version, m.Name) }

// Range is a reserved block of versions owned by one milestone.
type Range struct {
	Lo, Hi int
	Owner  string
}

// Ranges are the reserved version ranges (ADR-0002 §4). A version outside
// them is rejected.
var Ranges = []Range{
	{1, 99, "foundation"},
	{100, 199, "conversation"},
	{200, 299, "journal and check-in"},
	{300, 399, "memory"},
	{400, 499, "consultation summary and sharing"},
	{500, 599, "AI routing records"},
	{600, 699, "reading content"},
	{700, 799, "planned-care directory and synthetic sandbox"},
	{800, 899, "privacy controls"},
	{900, 999, "ops"},
}

// RangeOf returns the owner of a version, or "" when it is not reserved.
func RangeOf(v int) string {
	for _, r := range Ranges {
		if v >= r.Lo && v <= r.Hi {
			return r.Owner
		}
	}
	return ""
}

var migrationFile = regexp.MustCompile(`^([0-9]{4})_([a-z0-9_]{1,48})\.(up|down|validate)\.sql$`)

func normalizeNewlines(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

func checksum(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

// Parse reads every migration file in dir and enforces the naming,
// directive and range rules. All problems are reported together.
func Parse(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}
	type files struct {
		name               string
		up, down, validate *string
	}
	byVersion := map[int]*files{}
	var errs []string
	for _, e := range entries {
		if e.IsDir() {
			errs = append(errs, fmt.Sprintf("%s: unexpected directory", e.Name()))
			continue
		}
		m := migrationFile.FindStringSubmatch(e.Name())
		if m == nil {
			errs = append(errs, fmt.Sprintf("%s: file name must match NNNN_name.(up|down|validate).sql with a lower-case name of 1-48 characters", e.Name()))
			continue
		}
		v, _ := strconv.Atoi(m[1])
		raw, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		text := string(normalizeNewlines(raw))
		f := byVersion[v]
		if f == nil {
			f = &files{name: m[2]}
			byVersion[v] = f
		} else if f.name != m[2] {
			errs = append(errs, fmt.Sprintf("version %04d: duplicate version (%s and %s)", v, f.name, m[2]))
			continue
		}
		switch m[3] {
		case "up":
			f.up = &text
		case "down":
			f.down = &text
		case "validate":
			f.validate = &text
		}
	}
	versions := make([]int, 0, len(byVersion))
	for v := range byVersion {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	known := map[int]bool{}
	for _, v := range versions {
		known[v] = true
	}
	var out []Migration
	for _, v := range versions {
		f := byVersion[v]
		label := fmt.Sprintf("%04d_%s", v, f.name)
		if RangeOf(v) == "" {
			errs = append(errs, fmt.Sprintf("%s: version %04d is outside the reserved ranges", label, v))
		}
		if f.up == nil {
			errs = append(errs, fmt.Sprintf("%s: missing up file", label))
			continue
		}
		hdr, err := parseHeader(*f.up)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", label, err))
			continue
		}
		mg := Migration{Version: v, Name: f.name, Kind: hdr.kind, Requires: hdr.requires, Irreversible: hdr.irreversible,
			NoTransaction: hdr.noTx, Up: *f.up, Checksum: checksum(*f.up)}
		if f.down != nil {
			mg.Down, mg.HasDown, mg.DownChecksum = *f.down, true, checksum(*f.down)
		}
		if f.validate != nil {
			mg.Validate, mg.HasValidate = *f.validate, true
		}
		switch {
		case mg.Irreversible && mg.HasDown:
			errs = append(errs, fmt.Sprintf("%s: irreversible migrations must not have a down file", label))
		case !mg.Irreversible && !mg.HasDown:
			errs = append(errs, fmt.Sprintf("%s: needs a down file or the sela:irreversible directive", label))
		}
		if mg.Kind == KindData {
			if !mg.HasValidate {
				errs = append(errs, fmt.Sprintf("%s: data migrations need a validate file", label))
			}
			if mg.NoTransaction {
				errs = append(errs, fmt.Sprintf("%s: data migrations cannot use sela:no-transaction", label))
			}
		}
		if mg.NoTransaction && mg.HasValidate {
			errs = append(errs, fmt.Sprintf("%s: a validate file needs a transaction", label))
		}
		// A transactional migration runs inside the runner's transaction:
		// a COMMIT (or a BEGIN ... COMMIT wrapper) in one of its files would
		// end it early, so a failed validation could no longer roll back, a
		// dry run would persist its changes and nothing would be recorded.
		if !mg.NoTransaction {
			errs = append(errs, checkTransactional(label+".up.sql", mg.Up)...)
			if mg.HasDown {
				errs = append(errs, checkTransactional(label+".down.sql", mg.Down)...)
			}
		}
		if mg.HasValidate && !mg.NoTransaction {
			errs = append(errs, checkValidate(label+".validate.sql", mg.Validate)...)
		}
		for _, r := range mg.Requires {
			switch {
			case r == v:
				errs = append(errs, fmt.Sprintf("%s: requires itself", label))
			case !known[r]:
				errs = append(errs, fmt.Sprintf("%s: requires %04d, which is not embedded", label, r))
			}
		}
		out = append(out, mg)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("migrations invalid:\n%s", strings.Join(errs, "\n"))
	}
	return out, nil
}

type header struct {
	kind         Kind
	requires     []int
	irreversible bool
	noTx         bool
}

// parseHeader reads the "-- sela:" directives from the leading comment
// block of an up file. A directive after the first SQL line is an error,
// so it can never be silently ignored.
func parseHeader(up string) (header, error) {
	var h header
	seen := map[string]bool{}
	inHeader := true
	for i, line := range strings.Split(up, "\n") {
		t := strings.TrimSpace(line)
		comment := strings.HasPrefix(t, "--")
		if inHeader && t != "" && !comment {
			inHeader = false
		}
		if !comment {
			continue
		}
		c := strings.TrimSpace(strings.TrimPrefix(t, "--"))
		if !strings.HasPrefix(c, "sela:") {
			continue
		}
		if !inHeader {
			return h, fmt.Errorf("line %d: sela: directive outside the leading comment block", i+1)
		}
		name, arg, _ := strings.Cut(strings.TrimPrefix(c, "sela:"), " ")
		arg = strings.TrimSpace(arg)
		if seen[name] {
			return h, fmt.Errorf("line %d: duplicate sela:%s", i+1, name)
		}
		seen[name] = true
		switch name {
		case "kind":
			switch Kind(arg) {
			case KindSchema, KindData:
				h.kind = Kind(arg)
			default:
				return h, fmt.Errorf("line %d: sela:kind must be schema or data", i+1)
			}
		case "requires":
			if arg == "" {
				return h, fmt.Errorf("line %d: sela:requires needs versions", i+1)
			}
			for _, p := range strings.Split(arg, ",") {
				p = strings.TrimSpace(p)
				if len(p) != 4 {
					return h, fmt.Errorf("line %d: sela:requires takes 4-digit versions", i+1)
				}
				n, err := strconv.Atoi(p)
				if err != nil || n < 1 {
					return h, fmt.Errorf("line %d: sela:requires takes 4-digit versions", i+1)
				}
				h.requires = append(h.requires, n)
			}
		case "irreversible":
			if arg != "" {
				return h, fmt.Errorf("line %d: sela:irreversible takes no argument", i+1)
			}
			h.irreversible = true
		case "no-transaction":
			if arg != "" {
				return h, fmt.Errorf("line %d: sela:no-transaction takes no argument", i+1)
			}
			h.noTx = true
		default:
			return h, fmt.Errorf("line %d: unknown directive sela:%s", i+1, name)
		}
	}
	if h.kind == "" {
		return h, errors.New("missing -- sela:kind schema|data")
	}
	return h, nil
}

// State of one migration version.
type State string

// States reported by status.
const (
	StateApplied          State = "applied"
	StatePending          State = "pending"
	StateOutOfOrder       State = "out_of_order"
	StateChecksumMismatch State = "checksum_mismatch"
	StateUnknown          State = "unknown_in_database"
)

// Applied is one schema_migrations row.
type Applied struct {
	Version       int
	Name          string
	Kind          Kind
	Checksum      []byte
	DownChecksum  []byte
	AppliedSeq    int64
	AppliedAt     time.Time
	AppliedBy     string
	DurationMS    int
	BackupRef     *string
	BinaryVersion string
}

// Entry is one line of a status report.
type Entry struct {
	Version    int
	Name       string
	Kind       Kind
	State      State
	AppliedAt  *time.Time
	AppliedSeq int64
	Detail     string
}

// Status compares the embedded migrations with schema_migrations.
type Status struct {
	Entries []Entry
	// Pending lists pending and out-of-order migrations in version order.
	Pending []Migration
	// Environment is the instance_meta marker ("" when not written yet).
	Environment string
	// ExpectedEnvironment is the runner's SELA_ENV ("" for a runner that
	// does not check it, such as the readiness probe).
	ExpectedEnvironment string
	applied             map[int]bool
}

// EnvironmentMismatch reports a database marked for another environment
// than the runner's (ADR-0002 §12).
func (s Status) EnvironmentMismatch() bool {
	return s.Environment != "" && s.ExpectedEnvironment != "" && s.Environment != s.ExpectedEnvironment
}

// Mismatched reports a changed checksum or a version unknown to the binary.
func (s Status) Mismatched() bool {
	for _, e := range s.Entries {
		if e.State == StateChecksumMismatch || e.State == StateUnknown {
			return true
		}
	}
	return false
}

// Current reports nothing pending and nothing mismatched, in a database
// of the runner's environment.
func (s Status) Current() bool {
	return !s.Mismatched() && !s.EnvironmentMismatch() && len(s.Pending) == 0
}

// ExitCode is 0 when current, 3 when work is pending, 4 on a mismatch (a
// changed or unknown migration, or another environment's database).
func (s Status) ExitCode() int {
	switch {
	case s.Mismatched() || s.EnvironmentMismatch():
		return 4
	case len(s.Pending) > 0:
		return 3
	}
	return 0
}

// Problems describes each mismatched entry.
func (s Status) Problems() []string {
	var out []string
	for _, e := range s.Entries {
		if e.State == StateChecksumMismatch || e.State == StateUnknown {
			out = append(out, fmt.Sprintf("%04d_%s: %s (%s)", e.Version, e.Name, e.State, e.Detail))
		}
	}
	return out
}

// Plan compares embedded migrations with applied rows. It is pure, so the
// ordering rules are testable without a database.
func Plan(embedded []Migration, applied []Applied) Status {
	st := Status{applied: map[int]bool{}}
	byVersion := map[int]Applied{}
	maxApplied := 0
	for _, a := range applied {
		byVersion[a.Version] = a
		st.applied[a.Version] = true
		if a.Version > maxApplied {
			maxApplied = a.Version
		}
	}
	embeddedSet := map[int]bool{}
	for _, m := range embedded {
		embeddedSet[m.Version] = true
		e := Entry{Version: m.Version, Name: m.Name, Kind: m.Kind}
		if a, ok := byVersion[m.Version]; ok {
			at := a.AppliedAt
			e.AppliedAt, e.AppliedSeq = &at, a.AppliedSeq
			switch {
			case !bytes.Equal(a.Checksum, m.Checksum):
				e.State, e.Detail = StateChecksumMismatch, "the up file changed after it was applied"
			case a.Name != m.Name:
				e.State, e.Detail = StateChecksumMismatch, fmt.Sprintf("applied as %q", a.Name)
			case a.Kind != m.Kind:
				e.State, e.Detail = StateChecksumMismatch, fmt.Sprintf("applied as kind %s", a.Kind)
			default:
				e.State = StateApplied
				if a.DownChecksum != nil && !bytes.Equal(a.DownChecksum, m.DownChecksum) {
					e.Detail = "the down file changed after it was applied; down-last refuses it"
				}
			}
		} else {
			e.State = StatePending
			if m.Version < maxApplied {
				e.State, e.Detail = StateOutOfOrder, "lower than the highest applied version"
			}
			st.Pending = append(st.Pending, m)
		}
		st.Entries = append(st.Entries, e)
	}
	for _, a := range applied {
		if !embeddedSet[a.Version] {
			at := a.AppliedAt
			st.Entries = append(st.Entries, Entry{Version: a.Version, Name: a.Name, Kind: a.Kind, State: StateUnknown,
				AppliedAt: &at, AppliedSeq: a.AppliedSeq, Detail: "applied in the database but not embedded in this binary (schema ahead of binary)"})
		}
	}
	sort.Slice(st.Entries, func(i, j int) bool { return st.Entries[i].Version < st.Entries[j].Version })
	sort.Slice(st.Pending, func(i, j int) bool { return st.Pending[i].Version < st.Pending[j].Version })
	return st
}

// ErrRequiresUnmet means a pending migration needs a version that is not
// applied (an out-of-order migration is only allowed when its
// requirements are met).
var ErrRequiresUnmet = errors.New("migrate: required migration not applied")

// Sequence returns the pending migrations an up run applies, in version
// order, up to and including to (0 = all). Every required version must be
// applied already or earlier in the sequence.
func (s Status) Sequence(to int) ([]Migration, error) {
	if to > 0 {
		found := false
		for _, e := range s.Entries {
			if e.Version == to && e.State != StateUnknown {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("migrate: --to %04d is not an embedded migration", to)
		}
	}
	done := map[int]bool{}
	for v := range s.applied {
		done[v] = true
	}
	var out []Migration
	for _, m := range s.Pending {
		if to > 0 && m.Version > to {
			break
		}
		for _, r := range m.Requires {
			if !done[r] {
				return nil, fmt.Errorf("%w: %s requires %04d", ErrRequiresUnmet, m.Label(), r)
			}
		}
		done[m.Version] = true
		out = append(out, m)
	}
	return out, nil
}

// Runner errors.
var (
	ErrSchemaMismatch       = errors.New("migrate: schema does not match this binary")
	ErrEnvironmentMismatch  = errors.New("migrate: database belongs to another environment")
	ErrBackupRefRequired    = errors.New("migrate: data migrations need --backup-ref")
	ErrDataMigrationPending = errors.New("migrate: a data migration is pending; it is never applied automatically")
	ErrValidationFailed     = errors.New("migrate: validation returned rows")
	ErrNothingApplied       = errors.New("migrate: nothing is applied")
	ErrConfirmMismatch      = errors.New("migrate: --confirm must name the most recently applied migration")
	ErrIrreversible         = errors.New("migrate: migration is irreversible; roll back by restoring the backup")
	// ErrDownChecksumMismatch refuses a down file that differs from the one
	// recorded when the migration was applied (schema_migrations.down_checksum).
	ErrDownChecksumMismatch = errors.New("migrate: the down file differs from the one recorded when the migration was applied")
)

// Migrator runs migrations against one schema (the connection's
// current_schema()).
type Migrator struct {
	Pool       *pgxpool.Pool
	Migrations []Migration
	// BinaryVersion is recorded with every applied migration.
	BinaryVersion string
	// Environment is written to instance_meta by the first migration and
	// must match it afterwards.
	Environment string
	Logger      *slog.Logger
	// LockPoll is how often a waiting runner retries the lock (default 50ms).
	LockPoll time.Duration
}

const migrateLockKey = `hashtextextended('sela.migrate:' || current_schema(), 0)`

const createSchemaMigrations = `CREATE TABLE IF NOT EXISTS schema_migrations (
  version        integer PRIMARY KEY CHECK (version BETWEEN 1 AND 9999),
  name           text NOT NULL,
  kind           text NOT NULL CHECK (kind IN ('schema','data')),
  checksum       bytea NOT NULL,
  down_checksum  bytea NULL,
  applied_seq    bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
  applied_at     timestamptz NOT NULL DEFAULT now(),
  applied_by     text NOT NULL DEFAULT current_user,
  duration_ms    integer NOT NULL,
  backup_ref     text NULL,
  binary_version text NOT NULL
);
REVOKE ALL ON schema_migrations FROM PUBLIC`

func (m *Migrator) log() *slog.Logger {
	if m.Logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return m.Logger
}

// locked runs fn on a dedicated connection holding the schema-keyed
// migration advisory lock. The lock is polled with pg_try_advisory_lock so
// that a canceled waiter leaves nothing queued on the server.
func (m *Migrator) locked(ctx context.Context, fn func(c *pgx.Conn) error) error {
	pc, err := m.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer pc.Release()
	c := pc.Conn()
	if _, err := c.Exec(ctx, "SET statement_timeout = 0"); err != nil {
		return err
	}
	poll := m.LockPoll
	if poll <= 0 {
		poll = 50 * time.Millisecond
	}
	for {
		var ok bool
		if err := c.QueryRow(ctx, "SELECT pg_try_advisory_lock("+migrateLockKey+")").Scan(&ok); err != nil {
			_ = c.Close(context.Background())
			return err
		}
		if ok {
			break
		}
		select {
		case <-ctx.Done():
			_, _ = c.Exec(context.Background(), "RESET statement_timeout")
			return ctx.Err()
		case <-time.After(poll):
		}
		if poll < 500*time.Millisecond {
			poll += poll / 2
		}
	}
	defer func() {
		uctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := c.Exec(uctx, "SELECT pg_advisory_unlock("+migrateLockKey+")"); err != nil {
			_ = c.Close(uctx) // ending the session releases the lock
			return
		}
		_, _ = c.Exec(uctx, "RESET statement_timeout")
	}()
	return fn(c)
}

func tableExists(ctx context.Context, q DBTX, name string) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_tables WHERE schemaname = current_schema() AND tablename = $1)`, name).Scan(&ok)
	return ok, err
}

func loadApplied(ctx context.Context, q DBTX) ([]Applied, error) {
	ok, err := tableExists(ctx, q, "schema_migrations")
	if err != nil || !ok {
		return nil, err
	}
	rows, err := q.Query(ctx, `SELECT version, name, kind, checksum, down_checksum, applied_seq, applied_at, applied_by,
		duration_ms, backup_ref, binary_version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Applied, error) {
		var a Applied
		var kind string
		err := r.Scan(&a.Version, &a.Name, &kind, &a.Checksum, &a.DownChecksum, &a.AppliedSeq, &a.AppliedAt, &a.AppliedBy,
			&a.DurationMS, &a.BackupRef, &a.BinaryVersion)
		a.Kind = Kind(kind)
		return a, err
	})
}

// ReadEnvironment returns the instance_meta marker, "" when the table or
// its row does not exist yet.
func ReadEnvironment(ctx context.Context, q DBTX) (string, error) {
	ok, err := tableExists(ctx, q, "instance_meta")
	if err != nil || !ok {
		return "", err
	}
	var env string
	err = q.QueryRow(ctx, `SELECT environment FROM instance_meta`).Scan(&env)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return env, err
}

func (m *Migrator) inspect(ctx context.Context, q DBTX) (Status, error) {
	applied, err := loadApplied(ctx, q)
	if err != nil {
		return Status{}, err
	}
	st := Plan(m.Migrations, applied)
	st.ExpectedEnvironment = m.Environment
	st.Environment, err = ReadEnvironment(ctx, q)
	return st, err
}

// Status reports every embedded and applied version (under the lock).
func (m *Migrator) Status(ctx context.Context) (Status, error) {
	var st Status
	err := m.locked(ctx, func(c *pgx.Conn) error {
		var err error
		st, err = m.inspect(ctx, c)
		return err
	})
	return st, err
}

// Peek is Status without the lock, for readiness checks that must not
// wait behind a running migration.
func (m *Migrator) Peek(ctx context.Context) (Status, error) { return m.inspect(ctx, m.Pool) }

// UpOptions selects what an up run applies.
type UpOptions struct {
	// To stops after this version (0 = all pending).
	To int
	// BackupRef names the verified pg_dump taken before a data migration;
	// required whenever a data migration is in the run.
	BackupRef string
	// SchemaOnly refuses the run when a data migration is pending (server
	// start in development and test).
	SchemaOnly bool
}

// Step reports one applied or reverted migration.
type Step struct {
	Version  int
	Name     string
	Kind     Kind
	Duration time.Duration
}

func (m *Migrator) checkEnvironment(ctx context.Context, q DBTX) error {
	if m.Environment == "" {
		return errors.New("migrate: the environment (SELA_ENV) is required")
	}
	return CheckEnvironment(ctx, q, m.Environment)
}

// CheckEnvironment refuses a database whose instance_meta marker is set and
// differs from env (SELA_ENV), so a command configured for one environment
// never changes another environment's database. Every migration command
// and every operator command that opens SELA_DATABASE_URL calls it. A
// database without a marker (nothing migrated yet) passes.
func CheckEnvironment(ctx context.Context, q DBTX, env string) error {
	marker, err := ReadEnvironment(ctx, q)
	if err != nil {
		return err
	}
	if marker != "" && marker != env {
		return fmt.Errorf("%w: marker %q, SELA_ENV %q", ErrEnvironmentMismatch, marker, env)
	}
	return nil
}

// ErrTransactionEnded means a migration file ended the runner's
// transaction (a transaction-control statement the parser did not catch,
// for example inside a procedure), also when it then opened a new one
// (COMMIT; BEGIN;). What ran before it may be committed; nothing is
// recorded in schema_migrations.
var ErrTransactionEnded = errors.New("migrate: the migration ended the runner's transaction; statements before that point may be committed and the migration is not recorded; compare the database with the backup and fix the file")

// runnerTx identifies the runner's own transaction: its top-level
// transaction id (pg_current_xact_id assigns one when needed). A file that
// ends with COMMIT; BEGIN; leaves the connection in a transaction again,
// but in a new one with another id, and neither a COMMIT nor a ROLLBACK TO
// SAVEPOINT can bring the old id back.
func runnerTx(ctx context.Context, c *pgx.Conn) (string, error) {
	var id string
	err := c.QueryRow(ctx, `SELECT pg_current_xact_id()::text`).Scan(&id)
	return id, err
}

// stillInTx checks, after a migration file ran, that the connection is
// still inside the runner's transaction: in a transaction at all, and in
// the same one (want, from runnerTx before the file ran).
func stillInTx(ctx context.Context, c *pgx.Conn, want, what string) error {
	if st := c.PgConn().TxStatus(); st != 'T' {
		return fmt.Errorf("%w (%s, transaction status %q)", ErrTransactionEnded, what, string(st))
	}
	got, err := runnerTx(ctx, c)
	if err != nil {
		return fmt.Errorf("%w (%s: %v)", ErrTransactionEnded, what, describe(err))
	}
	if got != want {
		return fmt.Errorf("%w (%s, the file started another transaction)", ErrTransactionEnded, what)
	}
	return nil
}

// Up applies pending migrations, each in its own transaction together with
// its schema_migrations row, so a failure leaves nothing half-applied.
func (m *Migrator) Up(ctx context.Context, o UpOptions) ([]Step, error) {
	var steps []Step
	err := m.locked(ctx, func(c *pgx.Conn) error {
		if _, err := c.Exec(ctx, createSchemaMigrations); err != nil {
			return err
		}
		if err := m.checkEnvironment(ctx, c); err != nil {
			return err
		}
		st, err := m.inspect(ctx, c)
		if err != nil {
			return err
		}
		if st.Mismatched() {
			return fmt.Errorf("%w: %s", ErrSchemaMismatch, strings.Join(st.Problems(), "; "))
		}
		seq, err := st.Sequence(o.To)
		if err != nil {
			return err
		}
		for _, mg := range seq {
			if mg.Kind == KindData {
				if o.SchemaOnly {
					return fmt.Errorf("%w (%s); run sela migrate up --backup-ref REF", ErrDataMigrationPending, mg.Label())
				}
				if strings.TrimSpace(o.BackupRef) == "" {
					return fmt.Errorf("%w (%s)", ErrBackupRefRequired, mg.Label())
				}
			}
		}
		for _, mg := range seq {
			d, err := m.apply(ctx, c, mg, o.BackupRef)
			if err != nil {
				return fmt.Errorf("migration %s: %w", mg.Label(), err)
			}
			steps = append(steps, Step{Version: mg.Version, Name: mg.Name, Kind: mg.Kind, Duration: d})
			m.log().Info("migrate.applied", "version", mg.Version, "name", mg.Name, "kind", string(mg.Kind), "duration_ms", d.Milliseconds())
		}
		return nil
	})
	return steps, err
}

const writeMarker = `INSERT INTO instance_meta (singleton, environment, created_at)
SELECT true, $1, now() WHERE NOT EXISTS (SELECT 1 FROM instance_meta)`

func (m *Migrator) markEnvironment(ctx context.Context, q DBTX) error {
	ok, err := tableExists(ctx, q, "instance_meta")
	if err != nil || !ok {
		return err
	}
	_, err = q.Exec(ctx, writeMarker, m.Environment)
	return err
}

func (m *Migrator) record(ctx context.Context, q DBTX, mg Migration, d time.Duration, backupRef string) error {
	var ref *string
	if strings.TrimSpace(backupRef) != "" {
		ref = &backupRef
	}
	_, err := q.Exec(ctx, `INSERT INTO schema_migrations (version, name, kind, checksum, down_checksum, duration_ms, backup_ref, binary_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, mg.Version, mg.Name, string(mg.Kind), mg.Checksum, mg.DownChecksum,
		int(d.Milliseconds()), ref, m.BinaryVersion)
	return err
}

func countRows(ctx context.Context, q DBTX, sql string) (int, error) {
	rows, err := q.Query(ctx, sql)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	return n, rows.Err()
}

func (m *Migrator) apply(ctx context.Context, c *pgx.Conn, mg Migration, backupRef string) (time.Duration, error) {
	start := time.Now()
	if mg.NoTransaction {
		if _, err := c.Exec(ctx, mg.Up); err != nil {
			return 0, describe(err)
		}
		if err := m.markEnvironment(ctx, c); err != nil {
			return 0, err
		}
		d := time.Since(start)
		return d, m.record(ctx, c, mg, d, backupRef)
	}
	tx, err := c.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	own, err := runnerTx(ctx, c)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, mg.Up); err != nil {
		return 0, describe(err)
	}
	if err := stillInTx(ctx, c, own, "up file"); err != nil {
		return 0, err
	}
	if mg.HasValidate {
		n, err := countRows(ctx, tx, mg.Validate)
		if err != nil {
			return 0, fmt.Errorf("validate: %w", describe(err))
		}
		if err := stillInTx(ctx, c, own, "validate file"); err != nil {
			return 0, err
		}
		if n > 0 {
			return 0, fmt.Errorf("%w (%d rows); rolled back", ErrValidationFailed, n)
		}
	}
	if err := m.markEnvironment(ctx, tx); err != nil {
		return 0, err
	}
	d := time.Since(start)
	if err := m.record(ctx, tx, mg, d, backupRef); err != nil {
		return 0, err
	}
	return d, tx.Commit(ctx)
}

// describe keeps the parts of a PostgreSQL error that name schema objects
// (SQLSTATE, table, constraint, column) and the message only for schema
// errors (class 42), never row values.
func describe(err error) error {
	var pg *pgconn.PgError
	if !errors.As(err, &pg) {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "SQLSTATE %s", pg.Code)
	if strings.HasPrefix(pg.Code, "42") {
		fmt.Fprintf(&b, ": %s", pg.Message)
	}
	for _, kv := range [][2]string{{"table", pg.TableName}, {"constraint", pg.ConstraintName}, {"column", pg.ColumnName}} {
		if kv[1] != "" {
			fmt.Fprintf(&b, " %s=%s", kv[0], kv[1])
		}
	}
	return errors.New(b.String())
}

// DryRunStep is one migration in a dry run.
type DryRunStep struct {
	Version        int
	Name           string
	Kind           Kind
	Duration       time.Duration
	HasValidate    bool
	ValidationRows int
	Error          string
}

// DryRunReport is the result of a dry run.
type DryRunReport struct {
	Steps  []DryRunStep
	Failed bool
}

// DryRun applies every pending migration inside one transaction, runs the
// validate files, reports durations and validation counts, and rolls
// everything back, schema_migrations included.
func (m *Migrator) DryRun(ctx context.Context) (DryRunReport, error) {
	var rep DryRunReport
	err := m.locked(ctx, func(c *pgx.Conn) error {
		tx, err := c.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		own, err := runnerTx(ctx, c)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, createSchemaMigrations); err != nil {
			return err
		}
		if err := m.checkEnvironment(ctx, tx); err != nil {
			return err
		}
		st, err := m.inspect(ctx, tx)
		if err != nil {
			return err
		}
		if st.Mismatched() {
			return fmt.Errorf("%w: %s", ErrSchemaMismatch, strings.Join(st.Problems(), "; "))
		}
		seq, err := st.Sequence(0)
		if err != nil {
			return err
		}
		for _, mg := range seq {
			step := DryRunStep{Version: mg.Version, Name: mg.Name, Kind: mg.Kind, HasValidate: mg.HasValidate}
			if mg.NoTransaction {
				step.Error = "sela:no-transaction migrations cannot be dry-run"
				rep.Steps, rep.Failed = append(rep.Steps, step), true
				break
			}
			start := time.Now()
			if _, err := tx.Exec(ctx, mg.Up); err != nil {
				step.Error = describe(err).Error()
				rep.Steps, rep.Failed = append(rep.Steps, step), true
				break
			}
			if err := stillInTx(ctx, c, own, mg.Label()+" up file"); err != nil {
				step.Error = err.Error()
				rep.Steps, rep.Failed = append(rep.Steps, step), true
				return err
			}
			if mg.HasValidate {
				n, err := countRows(ctx, tx, mg.Validate)
				if err != nil {
					step.Error = "validate: " + describe(err).Error()
					rep.Steps, rep.Failed = append(rep.Steps, step), true
					break
				}
				if err := stillInTx(ctx, c, own, mg.Label()+" validate file"); err != nil {
					step.Error = err.Error()
					rep.Steps, rep.Failed = append(rep.Steps, step), true
					return err
				}
				step.ValidationRows = n
				if n > 0 {
					step.Error = fmt.Sprintf("validation returned %d rows", n)
					step.Duration = time.Since(start)
					rep.Steps, rep.Failed = append(rep.Steps, step), true
					break
				}
			}
			step.Duration = time.Since(start)
			rep.Steps = append(rep.Steps, step)
		}
		return nil
	})
	return rep, err
}

// DownLast reverts the most recently applied migration (by applied_seq),
// which confirm must name. Irreversible migrations are refused, data
// migrations need a backup reference, and a down file whose checksum
// differs from the down_checksum recorded at apply time is refused
// (ErrDownChecksumMismatch), so a rollback never runs another file than
// the one reviewed with the applied up file.
func (m *Migrator) DownLast(ctx context.Context, confirm int, backupRef string) (Step, error) {
	var step Step
	err := m.locked(ctx, func(c *pgx.Conn) error {
		if err := m.checkEnvironment(ctx, c); err != nil {
			return err
		}
		applied, err := loadApplied(ctx, c)
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			return ErrNothingApplied
		}
		st := Plan(m.Migrations, applied)
		if st.Mismatched() {
			return fmt.Errorf("%w: %s", ErrSchemaMismatch, strings.Join(st.Problems(), "; "))
		}
		last := applied[0]
		for _, a := range applied {
			if a.AppliedSeq > last.AppliedSeq {
				last = a
			}
		}
		if confirm != last.Version {
			return fmt.Errorf("%w (%04d_%s)", ErrConfirmMismatch, last.Version, last.Name)
		}
		var mg Migration
		for _, e := range m.Migrations {
			if e.Version == last.Version {
				mg = e
			}
		}
		if mg.Irreversible || !mg.HasDown {
			return fmt.Errorf("%w (%s)", ErrIrreversible, mg.Label())
		}
		// The down file must be the one that was shipped with the up file
		// that ran. A row without a recorded down checksum (applied without
		// a down file) has nothing to compare and is accepted.
		if last.DownChecksum != nil && !bytes.Equal(last.DownChecksum, mg.DownChecksum) {
			return fmt.Errorf("%w (%s); restore from the backup, or ship the recorded down file", ErrDownChecksumMismatch, mg.Label())
		}
		if mg.Kind == KindData && strings.TrimSpace(backupRef) == "" {
			return fmt.Errorf("%w (%s)", ErrBackupRefRequired, mg.Label())
		}
		start := time.Now()
		if mg.NoTransaction {
			if _, err := c.Exec(ctx, mg.Down); err != nil {
				return describe(err)
			}
			if _, err := c.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, mg.Version); err != nil {
				return err
			}
		} else {
			err := pgx.BeginFunc(ctx, c, func(tx pgx.Tx) error {
				own, err := runnerTx(ctx, c)
				if err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, mg.Down); err != nil {
					return describe(err)
				}
				if err := stillInTx(ctx, c, own, "down file"); err != nil {
					return err
				}
				_, err = tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, mg.Version)
				return err
			})
			if err != nil {
				return err
			}
		}
		step = Step{Version: mg.Version, Name: mg.Name, Kind: mg.Kind, Duration: time.Since(start)}
		m.log().Info("migrate.reverted", "version", mg.Version, "name", mg.Name, "duration_ms", step.Duration.Milliseconds())
		return nil
	})
	return step, err
}
