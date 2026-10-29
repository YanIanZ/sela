//go:build !race

package account_test

// raceEnabled reports a -race build, whose instrumentation makes latency
// measurements meaningless (see timingAssertions).
const raceEnabled = false
