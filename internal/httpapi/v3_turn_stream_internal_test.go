package httpapi

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStreamPiecesStayOnRuneBoundaries(t *testing.T) {
	text := "Kamu 🌿 baik-baik saja?"
	pieces := streamPieces(text, 3)
	if len(pieces) < 2 {
		t.Fatalf("expected several pieces, got %q", pieces)
	}
	if got := strings.Join(pieces, ""); got != text {
		t.Fatalf("chunks changed text: %q", got)
	}
	for _, piece := range pieces {
		if !utf8.ValidString(piece) {
			t.Fatalf("chunk split a UTF-8 sequence: %q", piece)
		}
	}
}
