package account

import (
	"crypto/rand"
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/db/dbtest"
)

// R2-A: each unknown username is mapped to one stored parameter tuple in
// proportion to the counts, and to the same one every time.
func TestDummyForIsProportionalAndStable(t *testing.T) {
	t.Parallel()
	a := FloorHashParams
	b := HashParams{MemoryKiB: 19456, Iterations: 3, Parallelism: 1}
	h, err := NewHasher(a, 2, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	h.SetDistribution([]ParamCount{{Params: a, Count: 3}, {Params: b, Count: 1}})
	const n = 20000
	atB := 0
	for i := 0; i < n; i++ {
		var seed [32]byte
		_, _ = rand.Read(seed[:])
		p := h.dummyFor(seed).Params
		if h.dummyFor(seed).Params != p {
			t.Fatal("the same key mapped to two tuples")
		}
		if p == b {
			atB++
		}
	}
	if share := float64(atB) / n; share < 0.23 || share > 0.27 {
		t.Fatalf("share of the 1-in-4 tuple: %.3f", share)
	}
}

// R3-1: when the counts change by one (a registration, an upgrade, a
// deletion), only the keys between the old and the new boundary change
// tuple, as many as the boundary moved. Mapping by remainder (key % total)
// re-drew every key, so about 2p(1-p) of all unknown usernames changed
// cost at each refresh while existing accounts kept theirs.
func TestDummyForMovesOnlyBoundaryKeys(t *testing.T) {
	t.Parallel()
	a := FloorHashParams
	b := HashParams{MemoryKiB: 19456, Iterations: 3, Parallelism: 1}
	h, err := NewHasher(a, 2, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ before, after [2]int64 }{
		{[2]int64{8, 8}, [2]int64{8, 9}},         // one registration at the new parameters
		{[2]int64{500, 500}, [2]int64{499, 501}}, // one account re-hashed on sign-in
		{[2]int64{40, 0}, [2]int64{40, 1}},       // the first account at new parameters
		{[2]int64{9, 30}, [2]int64{8, 30}},       // one old account deleted
	} {
		dist := func(c [2]int64) []ParamCount {
			return []ParamCount{{Params: a, Count: c[0]}, {Params: b, Count: c[1]}}
		}
		const n = 40000
		seeds := make([][32]byte, n)
		before := make([]HashParams, n)
		h.SetDistribution(dist(tc.before))
		for i := range seeds {
			_, _ = rand.Read(seeds[i][:])
			before[i] = h.dummyFor(seeds[i]).Params
		}
		h.SetDistribution(dist(tc.after))
		lo := float64(tc.before[0]) / float64(tc.before[0]+tc.before[1])
		hi := float64(tc.after[0]) / float64(tc.after[0]+tc.after[1])
		if lo > hi {
			lo, hi = hi, lo
		}
		moved := 0
		for i, seed := range seeds {
			if h.dummyFor(seed).Params == before[i] {
				continue
			}
			moved++
			// The fraction the key stands for lies between the two boundaries.
			f := float64(binary.BigEndian.Uint64(seed[:8])) / math.Exp2(64)
			if f < lo-1e-9 || f >= hi+1e-9 {
				t.Fatalf("%v -> %v: a key at %.6f moved, outside the boundary shift [%.6f, %.6f)", tc.before, tc.after, f, lo, hi)
			}
		}
		if want := (hi - lo) * n; float64(moved) > want+6*math.Sqrt(want)+5 {
			t.Fatalf("%v -> %v: %d of %d keys changed tuple, the boundary moved by %.4f (about %.0f keys)", tc.before, tc.after, moved, n, hi-lo, want)
		}
	}
}

// The backoff schedule in SQL (reserveSQL) is BackoffDelay.
func TestBackoffSQLMatchesBackoffDelay(t *testing.T) {
	t.Parallel()
	d := dbtest.New(t)
	rows, err := d.Pool.Query(t.Context(), `SELECT n, extract(epoch FROM `+backoffSQL("n")+`)::float8 FROM generate_series(0, 300) AS n`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var n int
		var secs float64
		if err := rows.Scan(&n, &secs); err != nil {
			t.Fatal(err)
		}
		if got, want := time.Duration(secs*float64(time.Second)), BackoffDelay(n); got != want {
			t.Errorf("n=%d: SQL %s, BackoffDelay %s", n, got, want)
		}
		seen++
	}
	if rows.Err() != nil || seen != 301 {
		t.Fatalf("rows %d: %v", seen, rows.Err())
	}
}
