package account

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/vault"
)

// Age bands. "under_15" is accepted as input only and always refused.
const (
	AgeBand15to17  = "15_17"
	AgeBand18Plus  = "18_plus"
	AgeBandUnknown = "unknown"
	AgeBandUnder15 = "under_15"
	KindGuest      = "guest"
	KindAccount    = "account"
	PolicyTerms    = "terms_privacy"
	PolicyGuest    = "guest_storage"
	PolicyGuardian = "guardian_information"
	// PolicyPersonalMemory gates the explicit, user-created memory items.
	// It is not part of FoundationPurposes: it is optional and off until
	// the owner grants it (default OFF).
	PolicyPersonalMemory = "personal_memory"
	actorUser            = "user"
	actorSystem          = "system"
	recoveryLowMark      = 3
)

// Purpose is one consent purpose in the code registry.
type Purpose struct {
	Name string
	// Kinds lists the user kinds the purpose applies to.
	Kinds []string
	// Required purposes cannot be withdrawn (409 required_consent).
	Required bool
}

func (p Purpose) appliesTo(kind string) bool { return slices.Contains(p.Kinds, kind) }

// FoundationPurposes are the purposes the foundation activates.
var FoundationPurposes = []Purpose{
	{Name: PolicyTerms, Kinds: []string{KindGuest, KindAccount}, Required: true},
	{Name: PolicyGuest, Kinds: []string{KindGuest}, Required: true},
}

// Config holds the account rules of one deployment.
type Config struct {
	// Environment is SELA_ENV; anything but production is synthetic data.
	Environment string

	Hash            HashParams
	HashConcurrency int
	HashWait        time.Duration

	SessionIdle             time.Duration
	SessionAbsolute         time.Duration
	TrustedIdle             time.Duration
	TrustedAbsolute         time.Duration
	ReauthWindow            time.Duration
	GuestInactiveTTL        time.Duration
	GuestMaxAge             time.Duration
	MaxGuests               int
	AllowedAgeBands         []string
	GuardianConsentRequired bool
	// RegionLabel names the hosting region once decided (B-3); "" = null.
	RegionLabel string
	// PolicyVersions maps policy keys (terms_privacy, guest_storage,
	// guardian_information and every purpose) to the current version.
	PolicyVersions map[string]string
	// Purposes is the consent registry (FoundationPurposes when nil).
	Purposes []Purpose
	// Now is the clock (tests inject one).
	Now    func() time.Time
	Logger *slog.Logger
}

// DefaultConfig returns the ADR-0002 defaults for an environment.
func DefaultConfig(env string) Config {
	bands := []string{AgeBand15to17, AgeBand18Plus, AgeBandUnknown}
	if env == "production" {
		bands = []string{AgeBand18Plus}
	}
	return Config{
		Environment: env, Hash: DefaultHashParams, HashConcurrency: 2, HashWait: 5 * time.Second,
		SessionIdle: 2 * time.Hour, SessionAbsolute: 12 * time.Hour, TrustedIdle: 7 * 24 * time.Hour, TrustedAbsolute: 30 * 24 * time.Hour,
		ReauthWindow: 10 * time.Minute, GuestInactiveTTL: 7 * 24 * time.Hour, GuestMaxAge: 30 * 24 * time.Hour,
		MaxGuests: 10000, AllowedAgeBands: bands, GuardianConsentRequired: true,
	}
}

// Synthetic reports whether the environment holds synthetic data only.
func (c Config) Synthetic() bool { return c.Environment != "production" }

func (c Config) bandAllowed(b string) bool { return slices.Contains(c.AllowedAgeBands, b) }

func (c Config) purposes() []Purpose {
	if c.Purposes != nil {
		return c.Purposes
	}
	return FoundationPurposes
}

func (c Config) purpose(name string) (Purpose, bool) {
	for _, p := range c.purposes() {
		if p.Name == name {
			return p, true
		}
	}
	return Purpose{}, false
}

// OwnerStore is every owner-facing operation. Each method takes the
// context and then the owner, and scopes every statement to that owner;
// a reflection test enforces the signature.
type OwnerStore interface {
	CreateRecord(context.Context, UserID, RecordInput) (PersonalRecord, error)
	Records(context.Context, UserID) ([]PersonalRecord, error)
	Record(context.Context, UserID, string) (PersonalRecord, error)
	RecordHistory(context.Context, UserID, string) ([]PersonalRecord, error)
	UpdateRecord(context.Context, UserID, string, RecordInput) (PersonalRecord, error)
	DeleteRecord(context.Context, UserID, string, int) error

	Memory(context.Context, UserID) (MemoryView, error)
	MemoryItem(context.Context, UserID, string) (PersonalRecord, error)
	CreateMemory(context.Context, UserID, RecordInput) (PersonalRecord, error)
	UpdateMemory(context.Context, UserID, string, RecordInput) (PersonalRecord, error)
	DeleteMemory(context.Context, UserID, string, int) error
	Summaries(context.Context, UserID) ([]PersonalRecord, error)
	Summary(context.Context, UserID, string) (PersonalRecord, error)
	CreateSummary(context.Context, UserID, RecordInput) (PersonalRecord, error)
	UpdateSummary(context.Context, UserID, string, RecordInput) (PersonalRecord, error)
	ApproveSummary(context.Context, UserID, string, int) (PersonalRecord, error)
	DeleteSummary(context.Context, UserID, string, int) error
	CreateFollowup(context.Context, UserID, string, RecordInput) (PersonalRecord, error)
	Followups(context.Context, UserID, string) ([]PersonalRecord, error)
	Shares(context.Context, UserID) ([]Share, error)
	CreateShare(context.Context, UserID, string, string, string, int) (Share, string, string, error)
	RevokeShare(context.Context, UserID, string, int) (Share, error)

	Profile(ctx context.Context, owner UserID) (Profile, error)
	UpdateProfile(ctx context.Context, owner UserID, current SessionID, in ProfileUpdate) (Profile, error)
	AcknowledgeRecoveryCodes(ctx context.Context, owner UserID, current SessionID) (Profile, error)
	Reauthenticate(ctx context.Context, owner UserID, current SessionID, password string) (time.Time, error)
	ChangePassword(ctx context.Context, owner UserID, current SessionID, currentPassword, newPassword, userAgent string) (AuthResult, error)
	RegenerateRecoveryCodes(ctx context.Context, owner UserID, current SessionID, password string) ([]string, time.Time, error)
	UpgradeGuest(ctx context.Context, owner UserID, current SessionID, in UpgradeInput) (AuthResult, error)
	Sessions(ctx context.Context, owner UserID, current SessionID) ([]SessionView, error)
	RevokeSession(ctx context.Context, owner UserID, current SessionID, id SessionID) (bool, error)
	Logout(ctx context.Context, owner UserID, current SessionID) error
	LogoutAll(ctx context.Context, owner UserID, current SessionID, keepCurrent bool) (int, error)
	Consents(ctx context.Context, owner UserID) ([]ConsentView, error)
	SetConsent(ctx context.Context, owner UserID, current SessionID, purpose string, granted bool, policyVersion string) (ConsentView, bool, error)
	ConsentHistory(ctx context.Context, owner UserID, purpose string, page Page) ([]ConsentEvent, string, error)
	GuardianConsent(ctx context.Context, owner UserID) (GuardianView, error)
	SetGuardianConsent(ctx context.Context, owner UserID, current SessionID, in GuardianInput) (GuardianRecord, error)
	WithdrawGuardianConsent(ctx context.Context, owner UserID, current SessionID) error
	AuditEvents(ctx context.Context, owner UserID, current SessionID, page Page) ([]AuditEvent, string, error)
	Export(ctx context.Context, owner UserID, current SessionID, format, lang string) (ExportFile, error)
	DeleteAccount(ctx context.Context, owner UserID, current SessionID) (DeletionReceipt, error)
}

// SystemStore is the allow-listed set of operations that are not scoped to
// an already-known owner: sign-in paths, session lookup by token hash, and
// the purge, retention and restore jobs.
type SystemStore interface {
	Register(ctx context.Context, in RegisterInput) (AuthResult, error)
	StartGuest(ctx context.Context, in GuestInput) (AuthResult, error)
	Login(ctx context.Context, in LoginInput) (AuthResult, error)
	Recover(ctx context.Context, in RecoveryInput) (AuthResult, error)
	Authenticate(ctx context.Context, token string) (Principal, error)
	PurgeGuests(ctx context.Context) (int, error)
	Retention(ctx context.Context) (RetentionReport, error)
	ReconcileRestore(ctx context.Context, dryRun bool) (ReconcileReport, error)
	RefreshHashDistribution(ctx context.Context) error
	OpenShare(ctx context.Context, token, code string) (SharedProjection, string, error)
	RecipientShare(ctx context.Context, session string) (SharedProjection, error)
	AcknowledgeShare(ctx context.Context, session string) error
}

var (
	_ OwnerStore  = (*Store)(nil)
	_ SystemStore = (*Store)(nil)
)

// Store is the PostgreSQL-backed account store.
type Store struct {
	pool      *pgxpool.Pool
	vault     *vault.Vault
	hasher    *Hasher
	cfg       Config
	now       func() time.Time
	log       *slog.Logger
	mu        sync.RWMutex
	exporters []Exporter
	// throttleSecret keys UsernameKey (vault.DeriveKey of
	// ThrottleKeyPurpose); it never reaches the database.
	throttleSecret []byte
	// deadlocks counts statements and transactions PostgreSQL aborted to
	// break a deadlock and the store ran again (db.Retry).
	deadlocks atomic.Uint64
}

// New validates the configuration and builds a store with the foundation
// exporters registered.
func New(pool *pgxpool.Pool, v *vault.Vault, cfg Config) (*Store, error) {
	if pool == nil || v == nil {
		return nil, errors.New("account: pool and vault are required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	for _, key := range []string{PolicyTerms, PolicyGuest, PolicyGuardian} {
		if cfg.PolicyVersions[key] == "" {
			return nil, fmt.Errorf("account: policy version %s missing", key)
		}
	}
	for _, p := range cfg.purposes() {
		if cfg.PolicyVersions[p.Name] == "" {
			return nil, fmt.Errorf("account: purpose %s has no policy version", p.Name)
		}
	}
	h, err := NewHasher(cfg.Hash, cfg.HashConcurrency, cfg.HashWait)
	if err != nil {
		return nil, err
	}
	if err := checkPolicyVersions(cfg); err != nil {
		return nil, err
	}
	secret, err := v.DeriveKey(ThrottleKeyPurpose)
	if err != nil {
		return nil, err
	}
	s := &Store{pool: pool, vault: v, hasher: h, cfg: cfg, now: cfg.Now, log: cfg.Logger, throttleSecret: secret}
	for _, e := range foundationExporters() {
		if err := s.RegisterExporter(e); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// checkPolicyVersions refuses a configured policy version that is not a
// short identifier: versions are stored in plaintext and in the audit
// trail, which hold no free text.
func checkPolicyVersions(cfg Config) error {
	for key, v := range cfg.PolicyVersions {
		if !policyVersionPattern.MatchString(v) {
			return fmt.Errorf("account: policy version of %s must match %s", key, policyVersionPattern)
		}
	}
	return nil
}

// Config returns the store's configuration.
func (s *Store) Config() Config { return s.cfg }

// Hasher exposes the argon2id hasher (tests and metrics).
func (s *Store) Hasher() *Hasher { return s.hasher }

// Vault exposes the vault (tools and tests).
func (s *Store) Vault() *vault.Vault { return s.vault }

// UsernameKey is the keyed rate-limit and backoff key of a typed username
// (HMAC-SHA256 under the server-held throttle secret; see usernameKey).
// The HTTP layer keys its per-username buckets with it.
func (s *Store) UsernameKey(raw string) [32]byte {
	k, _, _ := usernameKey(s.throttleSecret, raw)
	return k
}

// RefreshHashDistribution reads how many credentials use each argon2id
// parameter tuple and makes unknown-username verifications follow that
// distribution (Hasher.SetDistribution). The server runs it at start and
// periodically, outside any request, so the statements of a login stay
// the same for known and unknown usernames.
func (s *Store) RefreshHashDistribution(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT argon2_memory_kib, argon2_iterations, argon2_parallelism, count(*)
		FROM credentials GROUP BY 1, 2, 3`)
	if err != nil {
		return err
	}
	dist, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (ParamCount, error) {
		var mem, iter int32
		var par int16
		var n int64
		err := r.Scan(&mem, &iter, &par, &n)
		return ParamCount{Params: HashParams{MemoryKiB: uint32(mem), Iterations: uint32(iter), Parallelism: uint8(par)}, Count: n}, err
	})
	if err != nil {
		return err
	}
	s.hasher.SetDistribution(dist)
	return nil
}

// clock returns the current time truncated to PostgreSQL's microsecond
// precision, so values compare equal after a round trip.
func (s *Store) clock() time.Time { return s.now().UTC().Truncate(time.Microsecond) }

// tx runs fn in a transaction. When PostgreSQL aborts it to break a
// deadlock it runs again, a bounded number of times (db.Retry): every
// transaction body builds its results from scratch. The row locks are
// taken in one order (credentials, then login_throttle, then sessions;
// throttle.go), so this is a backstop that DeadlockRetries makes visible.
func (s *Store) tx(ctx context.Context, fn func(pgx.Tx) error) error { return s.txOn(ctx, s.pool, fn) }

// txOn is tx on b instead of the pool: a background job runs its
// transactions on the connection that holds its lock (db.WithJobLock).
func (s *Store) txOn(ctx context.Context, b db.Beginner, fn func(pgx.Tx) error) error {
	return db.Retry(ctx, s.onDeadlock, func() error { return db.InTx(ctx, b, fn) })
}

func (s *Store) onDeadlock() { s.deadlocks.Add(1) }

// DeadlockRetries reports how many statements and transactions were run
// again after a deadlock (tests and metrics; zero is expected).
func (s *Store) DeadlockRetries() uint64 { return s.deadlocks.Load() }
