package budget

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/db/dbtest"
)

var bg = context.Background()

func newTestGate(t *testing.T, monthly, reserve float64) (*Gate, *time.Time) {
	t.Helper()
	db := dbtest.New(t)
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	g, err := New(db.Pool, Options{MonthlyUSD: monthly, ReserveUSD: reserve, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return g, &now
}

func TestDisabledGateAdmitsWithoutDatabase(t *testing.T) {
	g, err := New(nil, Options{MonthlyUSD: 0})
	if err != nil {
		t.Fatal(err)
	}
	if g.Enabled() {
		t.Fatal("zero cap enabled")
	}
	r, err := g.Admit(bg, "listening_reply", false)
	if err != nil || r.Admitted() {
		t.Fatalf("Admit = %+v, %v", r, err)
	}
	if err := g.Settle(bg, r, Settlement{ActualUSD: 1}); err != nil {
		t.Fatal(err)
	}
	s, err := g.Summary(bg)
	if err != nil || s.CommittedUSD != 0 || s.MonthlyLimitUSD != 0 {
		t.Fatalf("Summary = %+v, %v", s, err)
	}
}

func TestValidation(t *testing.T) {
	if _, err := New(nil, Options{MonthlyUSD: 10, ReserveUSD: 0.25}); err == nil {
		t.Fatal("enabled cap without a pool accepted")
	}
	if _, err := New(nil, Options{MonthlyUSD: 10, ReserveUSD: 0}); err == nil {
		t.Fatal("zero reserve accepted")
	}
	if _, err := New(nil, Options{MonthlyUSD: -1, ReserveUSD: 0.25}); err == nil {
		t.Fatal("negative cap accepted")
	}
	if _, err := New(nil, Options{MonthlyUSD: 0, ReserveUSD: -1}); err != nil {
		t.Fatalf("disabled gate with negative reserve: %v", err)
	}
}

func TestAdmissionIsDurableAndSerialized(t *testing.T) {
	g, _ := newTestGate(t, 1.00, 0.25)
	const workers = 12
	var ok atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r, err := g.Admit(bg, "listening_reply", false); err == nil && r.Admitted() {
				ok.Add(1)
			} else if err != nil && !errors.Is(err, ErrExceeded) {
				t.Errorf("Admit: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := ok.Load(); got != 4 {
		t.Fatalf("admitted %d operations, want 4", got)
	}
	if _, err := g.Admit(bg, "listening_reply", false); !errors.Is(err, ErrExceeded) {
		t.Fatalf("over-cap Admit: %v", err)
	}

	// A second gate instance on the same database sees the same committed
	// spend: a Redis/process restart cannot reset the month.
	again, err := New(g.pool, Options{MonthlyUSD: 1.00, ReserveUSD: 0.25, Now: g.now})
	if err != nil {
		t.Fatal(err)
	}
	s, err := again.Summary(bg)
	if err != nil {
		t.Fatal(err)
	}
	if s.CommittedUSD != 1.00 || s.OpenReservations != 4 || s.Operations != 4 {
		t.Fatalf("Summary = %+v", s)
	}
}

func TestSettlementReleasesCapacityAndCountsActual(t *testing.T) {
	g, _ := newTestGate(t, 1.10, 0.25)
	var first Reservation
	for i := 0; i < 4; i++ {
		r, err := g.Admit(bg, "listening_reply", false)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = r
		}
	}
	if err := g.Settle(bg, first, Settlement{ActualUSD: 0.05, InputTokens: 900, OutputTokens: 120, Provider: "anthropic", Outcome: "ok"}); err != nil {
		t.Fatal(err)
	}
	// Repeated settlement is a no-op.
	if err := g.Settle(bg, first, Settlement{ActualUSD: 99}); err != nil {
		t.Fatal(err)
	}
	r, err := g.Admit(bg, "listening_reply", false)
	if err != nil {
		t.Fatalf("Admit after settlement: %v", err)
	}
	// 3 open reservations (0.75) + actual 0.05 + the new 0.25 = 1.05, so
	// one more fits under 1.10 and the second is refused.
	if _, err := g.Admit(bg, "listening_reply", false); !errors.Is(err, ErrExceeded) {
		t.Fatalf("Admit past actual+reserved: %v", err)
	}
	if err := g.Settle(bg, r, Settlement{ActualUSD: -5, Outcome: "failed"}); err != nil {
		t.Fatal(err)
	}
	s, err := g.Summary(bg)
	if err != nil {
		t.Fatal(err)
	}
	if s.ActualUSD != 0.30 || s.OpenReservations != 3 || s.Operations != 5 {
		t.Fatalf("Summary = %+v", s)
	}
	rows, err := g.ByTask(bg, 10)
	if err != nil || len(rows) != 1 || rows[0].Task != "listening_reply" || rows[0].Operations != 5 {
		t.Fatalf("ByTask = %+v, %v", rows, err)
	}
}
