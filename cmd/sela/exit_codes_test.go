package main

import (
	"fmt"
	"testing"

	"github.com/YanIanZ/sela/internal/db"
)

// DOWN-CHECKSUM-UNVERIFIED: a down file that differs from the recorded one
// is a mismatch for operators (exit 4), like a changed up file.
func TestExitForDownChecksumMismatch(t *testing.T) {
	for err, want := range map[error]int{
		fmt.Errorf("%w (0100_x)", db.ErrDownChecksumMismatch): 4,
		fmt.Errorf("%w: x", db.ErrSchemaMismatch):             4,
		db.ErrIrreversible: 1,
	} {
		if got := exitFor(err); got != want {
			t.Errorf("exitFor(%v) = %d, want %d", err, got, want)
		}
	}
}
