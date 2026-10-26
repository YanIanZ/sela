// Command sela runs the Sela v2 demo: one Go process serving the /api/v2 API
// and the static bilingual frontend, with bounded in-memory sessions.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/YanIanZ/sela/internal/config"
	"github.com/YanIanZ/sela/internal/fixtures"
	"github.com/YanIanZ/sela/internal/httpapi"
	"github.com/YanIanZ/sela/internal/journey"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/research"
	"github.com/YanIanZ/sela/internal/store"
)

// version is set at build time with -ldflags "-X main.version=<commit>".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "sela:", err)
		os.Exit(1)
	}
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

	st := store.NewMemory(store.Options{MaxSessions: cfg.MaxSessions})
	svc := journey.New(journey.Config{
		OpTimeout: cfg.OpTimeout, MaxOpsPerWindow: cfg.MaxOpsPerHour, WarmWindow: cfg.WarmWindow, ColdStartEstimate: cfg.ColdStart, Logger: logger,
	}, st, catalog, provider, llm.NewAdmission(cfg.LLMConcurrency))
	handler := httpapi.New(svc, httpapi.Options{
		PublicOrigin: cfg.PublicOrigin, SecureCookies: cfg.SecureCookies, TrustProxy: cfg.TrustProxy,
		WebDir: cfg.WebDir, Research: index, Logger: logger, Version: version,
	})

	srv := &http.Server{
		Addr: cfg.Addr, Handler: handler,
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		// No WriteTimeout: SSE streams outlive it; each operation has its own deadline.
		IdleTimeout: 120 * time.Second, MaxHeaderBytes: 32 << 10,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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
	logger.Info("sela.start", "version", version, "config", cfg.String(), "fixtures", catalog.Version, "provider_mode", string(provider.Info().Mode))
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdown)
	logger.Info("sela.stop", "note", "in-memory sessions are discarded on shutdown")
	return err
}
