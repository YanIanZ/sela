package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/config"
	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/fixtures"
	"github.com/YanIanZ/sela/internal/vault"
)

func openDatabase(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	return db.Open(ctx, db.Options{URL: cfg.DatabaseURL, StatementTimeout: cfg.DBStatementTimeout, MaxConns: int32(cfg.DBMaxConns)})
}

// prepareDatabase is the start-up schema gate (ADR-0002 §4, §12): a
// checksum mismatch or a version unknown to this binary always stops the
// server; a database marked for another environment stops it; pending
// migrations stop it in demo, staging and production, and in development
// and test are applied at start only with SELA_DB_AUTO_MIGRATE=true and
// only when they are all schema migrations (data migrations never run at
// start).
func prepareDatabase(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, migs []db.Migration, logger *slog.Logger) error {
	m := &db.Migrator{Pool: pool, Migrations: migs, BinaryVersion: version, Environment: cfg.Env, Logger: logger}
	st, err := m.Status(ctx)
	if err != nil {
		return fmt.Errorf("database: cannot read the schema state (%s)", db.Classify(err))
	}
	if st.Mismatched() {
		return fmt.Errorf("database schema mismatch, refusing to start: %v; run sela migrate status", st.Problems())
	}
	if st.Environment != "" && st.Environment != cfg.Env {
		return fmt.Errorf("database belongs to environment %q but SELA_ENV is %q; refusing to start", st.Environment, cfg.Env)
	}
	if len(st.Pending) > 0 {
		auto := cfg.DBAutoMigrate && (cfg.Env == config.EnvDevelopment || cfg.Env == config.EnvTest)
		if !auto {
			return fmt.Errorf("%d database migrations are pending, refusing to start; run sela migrate up", len(st.Pending))
		}
		if _, err := m.Up(ctx, db.UpOptions{SchemaOnly: true}); err != nil {
			return fmt.Errorf("database auto-migrate: %w", err)
		}
		if st, err = m.Status(ctx); err != nil {
			return err
		}
		if !st.Current() {
			return errors.New("database still not current after auto-migrate")
		}
	}
	if st.Environment != cfg.Env {
		return fmt.Errorf("database has no environment marker for %q; refusing to start", cfg.Env)
	}
	return nil
}

// readiness reports the database and schema state for /api/v2/ready
// without taking the migration lock.
func readiness(pool *pgxpool.Pool, migs []db.Migration) func(context.Context) (string, string) {
	m := &db.Migrator{Pool: pool, Migrations: migs}
	return func(ctx context.Context) (string, string) {
		if err := db.Ping(ctx, pool, 2*time.Second); err != nil {
			return "unavailable", "not_configured"
		}
		cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		st, err := m.Peek(cctx)
		switch {
		case err != nil:
			return "unavailable", "not_configured"
		case st.Mismatched():
			return "ok", "mismatch"
		case len(st.Pending) > 0:
			return "ok", "pending"
		}
		return "ok", "current"
	}
}

func newVault(cfg config.Config) (*vault.Vault, error) {
	if cfg.KEK.ID == "" {
		return nil, errors.New("SELA_DATA_KEK and SELA_DATA_KEK_ID are required")
	}
	ring := vault.Keyring{Current: vault.Key{ID: cfg.KEK.ID, Secret: cfg.KEK.Secret}}
	for _, p := range cfg.KEKPrevious {
		ring.Previous = append(ring.Previous, vault.Key{ID: p.ID, Secret: p.Secret})
	}
	return vault.New(ring, vault.Options{})
}

// accountConfig maps the environment configuration onto the account rules
// and the current v3 policy versions. Unset (zero) values keep the
// ADR-0002 defaults.
func accountConfig(cfg config.Config, logger *slog.Logger) account.Config {
	c := account.DefaultConfig(cfg.Env)
	if cfg.Argon2MemoryKiB > 0 {
		c.Hash = account.HashParams{MemoryKiB: uint32(cfg.Argon2MemoryKiB), Iterations: uint32(cfg.Argon2Iterations), Parallelism: uint8(cfg.Argon2Parallelism)}
	}
	setInt := func(dst *int, v int) {
		if v > 0 {
			*dst = v
		}
	}
	setDur := func(dst *time.Duration, v time.Duration) {
		if v > 0 {
			*dst = v
		}
	}
	setInt(&c.HashConcurrency, cfg.HashConcurrency)
	setInt(&c.MaxGuests, cfg.MaxGuestAccounts)
	setDur(&c.SessionIdle, cfg.SessionIdle)
	setDur(&c.SessionAbsolute, cfg.SessionAbsolute)
	setDur(&c.TrustedIdle, cfg.TrustedIdle)
	setDur(&c.TrustedAbsolute, cfg.TrustedAbsolute)
	setDur(&c.ReauthWindow, cfg.ReauthWindow)
	setDur(&c.GuestInactiveTTL, cfg.GuestInactiveTTL)
	setDur(&c.GuestMaxAge, cfg.GuestMaxAge)
	if len(cfg.AllowedAgeBands) > 0 {
		c.AllowedAgeBands = cfg.AllowedAgeBands
	}
	c.GuardianConsentRequired = cfg.GuardianConsentRequired
	c.RegionLabel = cfg.RegionLabel
	c.PolicyVersions = fixtures.V3PolicyVersions()
	c.Purposes = append(append([]account.Purpose(nil), account.FoundationPurposes...),
		account.Purpose{Name: account.PolicyPersonalMemory, Kinds: []string{account.KindGuest, account.KindAccount}})
	c.Purposes = append(c.Purposes, account.Purpose{Name: "ai_processing", Kinds: []string{account.KindGuest, account.KindAccount}}, account.Purpose{Name: "ai_provider_anthropic", Kinds: []string{account.KindGuest, account.KindAccount}}, account.Purpose{Name: "ai_provider_deepseek", Kinds: []string{account.KindGuest, account.KindAccount}})
	c.Logger = logger
	return c
}

// Background job periods (ADR-0002 §6, §7, §8).
const (
	purgeEvery     = 10 * time.Minute
	retentionEvery = time.Hour
	// hashDistributionEvery is how often the argon2id parameter
	// distribution of the stored credentials is re-read for the dummy
	// hashes of unknown usernames.
	hashDistributionEvery = 10 * time.Minute
)

// runJobs runs the guest purge and the retention job until ctx ends. Each
// takes its advisory lock, so with several instances only one runs it.
// Every instance also refreshes its own hash-parameter distribution. Only
// counts are logged.
func runJobs(ctx context.Context, s *account.Store, logger *slog.Logger) {
	purge := time.NewTicker(purgeEvery)
	retention := time.NewTicker(retentionEvery)
	hashes := time.NewTicker(hashDistributionEvery)
	defer purge.Stop()
	defer retention.Stop()
	defer hashes.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-hashes.C:
			if err := s.RefreshHashDistribution(ctx); err != nil && ctx.Err() == nil {
				logger.Warn("job.hash_distribution_failed", "class", db.Classify(err))
			}
		case <-purge.C:
			if _, err := s.PurgeGuests(ctx); err != nil && !errors.Is(err, db.ErrJobLocked) && ctx.Err() == nil {
				logger.Warn("job.guest_purge_failed", "class", db.Classify(err))
			}
		case <-retention.C:
			if _, err := s.Retention(ctx); err != nil && !errors.Is(err, db.ErrJobLocked) && ctx.Err() == nil {
				logger.Warn("job.retention_failed", "class", db.Classify(err))
			}
		}
	}
}
