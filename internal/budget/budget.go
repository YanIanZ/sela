// Package budget is Sela's durable AI spend gate (D12).
//
// PostgreSQL is the authority: every operation reserves its worst-case cost
// inside one transaction serialized by an advisory lock, and settles the
// actual estimate afterwards. Redis is an optional cache for public data
// only and is never consulted here, so a Redis outage or restart can neither
// reset nor bypass a budget. Rows contain task/provider metadata, token
// counts and cost estimates; never a user id, session id, prompt or reply.
package budget

import (
	"context"
	"crypto/rand"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrExceeded means admitting the operation would exceed the monthly cap.
// Callers decline the paid call; they never fall back to an unmetered path.
var ErrExceeded = errors.New("budget: monthly AI budget exhausted")

// The advisory lock serializes every reservation for the month, so two
// concurrent instances cannot both spend the last slot.
const budgetLockKey int64 = 0x53454C4142494C4C // "SELABILL"

// Options configures the gate. MonthlyUSD <= 0 disables it.
type Options struct {
	MonthlyUSD float64
	ReserveUSD float64
	Now        func() time.Time
}

// Gate admits and settles AI operations against a durable monthly cap.
type Gate struct {
	pool    *pgxpool.Pool
	monthly float64
	reserve float64
	now     func() time.Time
}

// New validates the budget configuration. A nil pool with a positive cap is
// an error: an unmetered paid call is never acceptable in that setup.
func New(pool *pgxpool.Pool, o Options) (*Gate, error) {
	if o.MonthlyUSD < 0 || math.IsNaN(o.MonthlyUSD) || math.IsInf(o.MonthlyUSD, 0) {
		return nil, errors.New("budget: monthly cap must be a finite non-negative number")
	}
	if o.MonthlyUSD > 0 {
		if pool == nil {
			return nil, errors.New("budget: a database pool is required when the cap is enabled")
		}
		if o.ReserveUSD <= 0 || math.IsNaN(o.ReserveUSD) || math.IsInf(o.ReserveUSD, 0) {
			return nil, errors.New("budget: reserve must be a finite positive number")
		}
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Gate{pool: pool, monthly: o.MonthlyUSD, reserve: o.ReserveUSD, now: o.Now}, nil
}

// Enabled reports whether a monthly cap is configured.
func (g *Gate) Enabled() bool { return g != nil && g.monthly > 0 }

// LimitUSD is the configured monthly cap (0 when disabled).
func (g *Gate) LimitUSD() float64 {
	if g == nil {
		return 0
	}
	return g.monthly
}

// Reservation is one admitted operation. Its zero value means "not
// admitted" (the gate was disabled).
type Reservation struct {
	ID          [16]byte
	ReservedUSD float64
}

// Admitted reports whether the reservation must be settled.
func (r Reservation) Admitted() bool { return r.ID != [16]byte{} }

// Settlement is the measured result of an admitted operation. ActualUSD is
// never below zero; a failed operation settles at its reservation instead,
// because provider work may already have been billed.
type Settlement struct {
	ActualUSD    float64
	InputTokens  int
	OutputTokens int
	Provider     string
	Outcome      string
}

// Admit reserves the configured worst-case cost for one operation.
func (g *Gate) Admit(ctx context.Context, task string, synthetic bool) (Reservation, error) {
	if !g.Enabled() {
		return Reservation{}, nil
	}
	if task == "" || len(task) > 80 {
		return Reservation{}, errors.New("budget: invalid task")
	}
	id := newUUID()
	now := g.now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	err := g.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, budgetLockKey); err != nil {
			return err
		}
		var committed float64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(COALESCE(actual_usd, reserved_usd)),0)
			FROM ai_spend WHERE reserved_at >= $1`, monthStart).Scan(&committed); err != nil {
			return err
		}
		if committed+g.reserve > g.monthly+1e-9 {
			return ErrExceeded
		}
		_, err := tx.Exec(ctx, `INSERT INTO ai_spend
			(id, reserved_at, task, reserved_usd, synthetic) VALUES ($1,$2,$3,$4,$5)`,
			id, now, task, g.reserve, synthetic)
		return err
	})
	if err != nil {
		return Reservation{}, err
	}
	return Reservation{ID: id, ReservedUSD: g.reserve}, nil
}

// Settle records the measured cost once. A repeated settlement is a no-op.
func (g *Gate) Settle(ctx context.Context, r Reservation, s Settlement) error {
	if !r.Admitted() {
		return nil
	}
	actual := s.ActualUSD
	if actual < 0 || math.IsNaN(actual) || math.IsInf(actual, 0) {
		actual = r.ReservedUSD
	}
	if s.InputTokens < 0 || s.OutputTokens < 0 {
		return errors.New("budget: negative token count")
	}
	_, err := g.pool.Exec(ctx, `UPDATE ai_spend
		SET settled_at = $2, actual_usd = $3, input_tokens = $4, output_tokens = $5,
		    provider = NULLIF($6,''), outcome = NULLIF($7,'')
		WHERE id = $1 AND settled_at IS NULL`,
		r.ID, g.now().UTC(), actual, s.InputTokens, s.OutputTokens, s.Provider, s.Outcome)
	return err
}

// Summary is a privacy-safe aggregate for the operator console.
type Summary struct {
	MonthlyLimitUSD  float64   `json:"monthlyLimitUsd"`
	ReserveUSD       float64   `json:"reserveUsd"`
	MonthStart       time.Time `json:"monthStart"`
	CommittedUSD     float64   `json:"committedUsd"`
	ActualUSD        float64   `json:"actualUsd"`
	OpenReservations int64     `json:"openReservations"`
	Operations       int64     `json:"operations"`
	GeneratedAt      time.Time `json:"generatedAt"`
}

// Summary reads the current month's committed spend.
func (g *Gate) Summary(ctx context.Context) (Summary, error) {
	if g == nil {
		return Summary{}, nil
	}
	now := g.now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	out := Summary{MonthlyLimitUSD: g.monthly, ReserveUSD: g.reserve, MonthStart: monthStart, GeneratedAt: now}
	if !g.Enabled() {
		return out, nil
	}
	err := g.pool.QueryRow(ctx, `SELECT
		COALESCE(SUM(COALESCE(actual_usd, reserved_usd)),0),
		COALESCE(SUM(actual_usd),0),
		count(*) FILTER (WHERE settled_at IS NULL),
		count(*)
		FROM ai_spend WHERE reserved_at >= $1`, monthStart).Scan(
		&out.CommittedUSD, &out.ActualUSD, &out.OpenReservations, &out.Operations)
	return out, err
}

// TaskSpend is one task's share of the month.
type TaskSpend struct {
	Task         string  `json:"task"`
	CommittedUSD float64 `json:"committedUsd"`
	ActualUSD    float64 `json:"actualUsd"`
	Operations   int64   `json:"operations"`
}

// ByTask lists the largest task shares of the month, at most limit rows.
func (g *Gate) ByTask(ctx context.Context, limit int) ([]TaskSpend, error) {
	if !g.Enabled() {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	now := g.now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	rows, err := g.pool.Query(ctx, `SELECT task,
		COALESCE(SUM(COALESCE(actual_usd, reserved_usd)),0),
		COALESCE(SUM(actual_usd),0),
		count(*)
		FROM ai_spend WHERE reserved_at >= $1
		GROUP BY task ORDER BY 2 DESC LIMIT $2`, monthStart, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TaskSpend
	for rows.Next() {
		var t TaskSpend
		if err := rows.Scan(&t.Task, &t.CommittedUSD, &t.ActualUSD, &t.Operations); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (g *Gate) tx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := g.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func newUUID() [16]byte {
	var u [16]byte
	if _, err := rand.Read(u[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	return u
}

// UUID returns the reservation id as text.
func (r Reservation) UUID() string { return pgtype.UUID{Bytes: r.ID, Valid: true}.String() }
