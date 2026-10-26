//go:build !race

package safety

// raceEnabled reports whether the race detector is on; it slows regexp
// matching by an order of magnitude.
const raceEnabled = false
