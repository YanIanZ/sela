package httpapi_test

// Regression tests for review round 2 over HTTP (auth R2-A, R2-D).

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

// R2-A over HTTP: accounts registered at the floor parameters; the
// operator raises SELA_ARGON2_MEMORY_KIB and restarts. A wrong-password
// login for an existing username and a login for an unknown one, each
// from a fresh client, cost the same: both verify with the stored
// parameters.
func TestV3UnknownUsernameCostAfterParameterChange(t *testing.T) {
	e := newV3(t, v3opt{})
	const n = 8
	for i := 0; i < n; i++ {
		e.client().register(fmt.Sprintf("ada.akun%02d", i))
	}
	raised := account.HashParams{MemoryKiB: 65536, Iterations: 2, Parallelism: 1}
	e.cfg.Hash = raised
	e.restart()
	h := e.store.Hasher()
	var known, unknown []time.Duration
	for i := 0; i < n; i++ {
		for _, k := range []bool{i%2 == 0, i%2 != 0} {
			u := fmt.Sprintf("tak.ada%02d", i)
			if k {
				u = fmt.Sprintf("ada.akun%02d", i)
			}
			start := time.Now()
			r := e.client().login(u, "salah salah salah salah")
			d := time.Since(start)
			if r.status != 401 || r.code() != "invalid_credentials" {
				t.Fatalf("%s: %d %s", u, r.status, r.raw)
			}
			if k {
				known = append(known, d)
			} else {
				unknown = append(unknown, d)
			}
		}
	}
	if h.VerificationsAt(account.FloorHashParams) != 2*n || h.VerificationsAt(raised) != 0 {
		t.Fatalf("verifications at the stored parameters %d, at the configured ones %d", h.VerificationsAt(account.FloorHashParams), h.VerificationsAt(raised))
	}
	slices.Sort(known)
	slices.Sort(unknown)
	t.Logf("existing usernames (401): %v", known)
	t.Logf("unknown usernames  (401): %v", unknown)
	// A measurement unless timingAssertions (review round 3, R3-5); the
	// VerificationsAt check above carries the property.
	if timingAssertions() && (known[n-1] < unknown[0] || unknown[n-1] < known[0]) {
		t.Fatalf("the two sides do not overlap: known %v, unknown %v", known, unknown)
	}
}

// timingAssertions reports whether latency assertions run: only when
// SELA_TIMING_TESTS=1 and not under -race (review round 3, R3-5).
func timingAssertions() bool { return os.Getenv("SELA_TIMING_TESTS") == "1" && !raceEnabled }

func v3Cursor(micros int64, v4 bool) string {
	b := binary.BigEndian.AppendUint64(nil, uint64(micros))
	id := make([]byte, 16)
	if v4 {
		id[6], id[8] = 0x40, 0x80
	}
	return base64.RawURLEncoding.EncodeToString(append(b, id...))
}

// R2-D: a crafted, well-formed cursor (a time PostgreSQL cannot hold, a
// time in the future, or an id that is no version-4 uuid) is 400
// invalid_input with field cursor on both paginated routes, and nothing
// reaches the database (no v3.error in the log).
func TestV3CraftedCursorIsInvalidInput(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	c := e.client()
	c.register("kursor.uji")
	for _, cur := range []string{
		v3Cursor(-377_000_000_000_000_000, true),
		v3Cursor(e.clock.Now().Add(72*time.Hour).UnixMicro(), true),
		v3Cursor(e.clock.Now().UnixMicro(), false),
	} {
		for _, path := range []string{"/api/v3/me/audit?cursor=", "/api/v3/me/consents/history?cursor="} {
			r := c.call("GET", path+cur, nil)
			if r.status != 400 || r.code() != "invalid_input" || r.field() != "cursor" {
				t.Errorf("GET %s%s: %d %s", path, cur, r.status, r.raw)
			}
		}
	}
	if strings.Contains(e.logs.String(), `"v3.error"`) {
		t.Fatal("a crafted cursor reached the database")
	}
}
