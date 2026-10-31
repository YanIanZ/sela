// Package readingdata embeds the reading content set at its authoring path,
// so the Go runtime and tests/content/reading_validator.py read the same
// files. The runtime still applies the editorial gate in internal/reading:
// embedding makes an item available to the gate, not publishable.
package readingdata

import "embed"

//go:embed *.json
var FS embed.FS
