// Command sela runs Sela: one Go process serving /api/v3 (accounts, guests
// and personal records in PostgreSQL), the /api/v2 demo journey (bounded
// in-memory sessions, development, test and demo only) and the static
// bilingual frontend. Operator subcommands live in the same binary so the
// distroless image needs nothing else:
//
//	sela                       serve
//	sela migrate status|up|dry-run|down-last
//	sela keys generate-kek|status|rewrap
//	sela restore reconcile [--dry-run]
//	sela backup check --content FILE --keys FILE
//	sela ops canary write|check
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/budget"
	"github.com/YanIanZ/sela/internal/cache"
	"github.com/YanIanZ/sela/internal/care"
	"github.com/YanIanZ/sela/internal/community"
	"github.com/YanIanZ/sela/internal/config"
	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/fixtures"
	"github.com/YanIanZ/sela/internal/httpapi"
	"github.com/YanIanZ/sela/internal/journey"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/reading"
	"github.com/YanIanZ/sela/internal/research"
	"github.com/YanIanZ/sela/internal/store"
)

// version is set at build time with -ldflags "-X main.version=<commit>".
var version = "dev"

func main() {
	code, err := dispatch(os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sela:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	level := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	catalog, err := fixtures.Load()
	if err != nil {
		return err
	}

	provider := llm.Disabled()
	if cfg.LLMMode != "disabled" {
		provider, err = llm.New(llm.Config{
			Mode: llm.Mode(cfg.LLMMode), BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey,
			ModalKey: cfg.ModalKey, ModalSecret: cfg.ModalSecret,
			ServedModel: cfg.ServedModel, ModelID: cfg.ModelID, Revision: cfg.ModelRevision,
			Observer: func(e llm.Event) {
				logger.Info("llm.call", "operation", e.Operation, "outcome", e.Outcome, "status", e.Status, "attempts", e.Attempts,
					"latency_ms", e.LatencyMS, "first_token_ms", e.FirstTokenMS, "prompt_tokens", e.PromptTokens, "completion_tokens", e.CompletionTokens)
			},
		})
		if err != nil {
			return err
		}
	}

	var index *research.Index
	if idx, err := research.Load(cfg.ResearchKB); err == nil {
		index = idx
	} else {
		logger.Warn("research.unavailable", "reason", err.Error())
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Sela v3 store: PostgreSQL, checked before anything is served.
	acctCfg := accountConfig(cfg, logger)
	var accounts *account.Store
	var adminStore *admin.Store
	var careStore *care.Store
	var communityStore *community.Store
	var readingReviews *reading.ReviewStore
	var budgetGate *budget.Gate
	var dbStatus func(context.Context) (string, string)
	if cfg.DatabaseURL != "" {
		pool, err := openDatabase(ctx, cfg)
		if err != nil {
			return err
		}
		defer pool.Close()
		migs := db.MustEmbedded()
		if err := prepareDatabase(ctx, pool, cfg, migs, logger); err != nil {
			return err
		}
		v, err := newVault(cfg)
		if err != nil {
			return err
		}
		if accounts, err = account.New(pool, v, acctCfg); err != nil {
			return err
		}
		// Unknown usernames are verified with the stored argon2id
		// parameter distribution from the first request on (ADR-0002 §6).
		if err := accounts.RefreshHashDistribution(ctx); err != nil {
			return fmt.Errorf("database: cannot read the password hash parameters (%s)", db.Classify(err))
		}
		careStore, err = care.New(pool, time.Now)
		if err != nil {
			return err
		}
		communityStore, err = community.New(pool, time.Now)
		if err != nil {
			return err
		}
		readingReviews, err = reading.NewReviewStore(pool, time.Now)
		if err != nil {
			return err
		}
		if len(cfg.AdminMFAKey) == 32 {
			adminStore, err = admin.New(pool, admin.Options{Hasher: accounts.Hasher(), MFAKey: cfg.AdminMFAKey, Now: time.Now})
			if err != nil {
				return err
			}
		}
		if cfg.AIMonthlyBudgetUSD > 0 {
			budgetGate, err = budget.New(pool, budget.Options{MonthlyUSD: cfg.AIMonthlyBudgetUSD, ReserveUSD: cfg.AIReserveUSD})
			if err != nil {
				return err
			}
		}
		dbStatus = readiness(pool, migs)
		go runJobs(ctx, accounts, logger)
	}
	if cfg.AIMonthlyBudgetUSD > 0 && budgetGate == nil {
		return errors.New("SELA_AI_MONTHLY_BUDGET_USD requires SELA_DATABASE_URL: refusing to run unmetered")
	}

	var aiRouter *ai.Router
	if accounts != nil && cfg.LLMMode != "disabled" {
		aiConfig, e := ai.LoadFromEnv(nil)
		if e != nil {
			return e
		}
		aiRouter, e = ai.Build(aiConfig, llm.Config{Mode: llm.Mode(cfg.LLMMode), BaseURL: cfg.LLMBaseURL, APIKey: cfg.LLMAPIKey, ModalKey: cfg.ModalKey, ModalSecret: cfg.ModalSecret, ServedModel: cfg.ServedModel, ModelID: cfg.ModelID, Revision: cfg.ModelRevision, ReasoningEffort: cfg.LLMReasoningEffort}, ai.Deps{Consent: accountAIConsent{accounts}, MetaAdmission: llm.NewAdmission(cfg.LLMConcurrency)})
		if e != nil {
			return e
		}
	}
	pubCache, err := cache.New(cache.Options{URL: cfg.RedisURL, Prefix: "sela:" + cfg.Env, TTL: 10 * time.Minute})
	if err != nil {
		return err
	}
	if pubCache != nil {
		defer pubCache.Close()
	}
	st := store.NewMemory(store.Options{MaxSessions: cfg.MaxSessions, MaxSessionsPerClient: cfg.MaxSessionsPerClient})
	svc := journey.New(journey.Config{
		OpTimeout: cfg.OpTimeout, MaxOpsPerWindow: cfg.MaxOpsPerHour, WarmWindow: cfg.WarmWindow, ColdStartEstimate: cfg.ColdStart, Logger: logger,
	}, st, catalog, provider, llm.NewAdmission(cfg.LLMConcurrency))
	handler := httpapi.New(svc, httpapi.Options{
		PublicOrigin: cfg.PublicOrigin, SecureCookies: cfg.SecureCookies, TrustProxy: cfg.TrustProxy,
		WebDir: cfg.WebDir, Research: index, Logger: logger, Version: version, MetricsToken: cfg.MetricsToken,
		Cache: pubCache, Admin: adminStore, Budget: budgetGate, Care: careStore,
		Community: communityStore, Moderator: community.MetaModerator{Provider: provider},
		ReadingReviews: readingReviews,
		AIRouter:       aiRouter, Accounts: accounts, AccountConfig: acctCfg, Database: dbStatus, DatabaseRequired: cfg.RequiresDatabase(),
		DisableV2Journey: cfg.Store != "memory", HSTS: cfg.Env == config.EnvProduction,
	})

	srv := &http.Server{
		Addr: cfg.Addr, Handler: handler,
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		// No WriteTimeout: SSE streams outlive it; each operation has its own deadline.
		IdleTimeout: 120 * time.Second, MaxHeaderBytes: 32 << 10,
	}
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if n := svc.Sweep(); n > 0 {
					logger.Info("store.sweep", "purged", n)
				}
			}
		}
	}()
	// Only named, non-secret settings are logged; the Config value itself
	// (which holds credentials) never reaches a log call.
	info := provider.Info()
	logger.Info("sela.start", "version", version, "env", cfg.Env, "addr", cfg.Addr, "llm_mode", string(info.Mode),
		"model_id", info.ModelID, "model_revision", info.Revision, "secure_cookies", cfg.SecureCookies,
		"max_sessions", cfg.MaxSessions, "metrics", cfg.MetricsToken != "", "fixtures", catalog.Version,
		"store", cfg.Store, "database", cfg.DatabaseURL != "", "kek_id", cfg.KEK.ID)
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	// Open event streams only end with their operation; close them when
	// shutdown starts so it is not held up for the whole grace period.
	srv.RegisterOnShutdown(handler.CloseStreams)
	return serve(ctx, stop, srv, ln, shutdownGrace, logger)
}

// shutdownGrace is how long in-flight requests get after a stop signal.
const shutdownGrace = 10 * time.Second

// serve runs srv on ln until ctx ends (the first SIGINT or SIGTERM), then
// shuts down gracefully. stop is called first so that a second signal
// terminates the process at once. Connections still busy after grace (for
// example a stalled upload) are closed; that is still a clean stop, because
// in-memory sessions are discarded either way.
func serve(ctx context.Context, stop context.CancelFunc, srv *http.Server, ln net.Listener, grace time.Duration, logger *slog.Logger) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	stop()
	shutdown, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	err := srv.Shutdown(shutdown)
	if errors.Is(err, context.DeadlineExceeded) {
		logger.Warn("sela.stop_forced", "grace_ms", grace.Milliseconds())
		// The listener is already closed; Close only drops the busy
		// connections, and its error (a closed listener) is not a failure.
		_ = srv.Close()
		err = nil
	}
	logger.Info("sela.stop", "note", "in-memory sessions are discarded on shutdown")
	return err
}
