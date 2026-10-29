package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/care"
	"github.com/YanIanZ/sela/internal/config"
)

// cmdCare seeds the clearly synthetic planned-care demo. It refuses
// production and staging; the rows are labelled synthetic in the database
// and the UI.
func cmdCare(ctx context.Context, args []string, out io.Writer) (int, error) {
	if len(args) < 1 || args[0] != "seed-demo" {
		return 2, errors.New("care: seed-demo --admin USERNAME")
	}
	fs := flag.NewFlagSet("care seed-demo", flag.ContinueOnError)
	fs.SetOutput(out)
	username := fs.String("admin", "", "active operator username recorded as the demo actor")
	if err := fs.Parse(args[1:]); err != nil {
		return 2, err
	}
	if *username == "" {
		return 2, errors.New("care seed-demo: --admin is required")
	}
	cfg, pool, err := opsDatabase(ctx, true)
	if err != nil {
		return exitFor(err), err
	}
	defer pool.Close()
	if cfg.Env == config.EnvProduction || cfg.Env == config.EnvStaging {
		return 1, errors.New("care seed-demo is refused in production and staging")
	}
	var id pgtype.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM admin_users WHERE username = $1 AND status = 'active'`, *username).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 1, fmt.Errorf("care seed-demo: no active operator %q; run sela admin create first", *username)
		}
		return 1, err
	}
	store, err := care.New(pool, time.Now)
	if err != nil {
		return 1, err
	}
	if err := store.SeedDemo(ctx, admin.AdminID(id.Bytes)); err != nil {
		return 1, err
	}
	fmt.Fprintln(out, "seeded synthetic planned-care demo entries and sandbox requests")
	return 0, nil
}
