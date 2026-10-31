package httpapi

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
)

func TestMemoryContextSpansHonoursAT1Limits(t *testing.T) {
	items := []account.PersonalRecord{}
	for i := 0; i < 6; i++ {
		items = append(items, account.PersonalRecord{
			Title: "m", Text: strings.Repeat("a", 100), ReviewStatus: "confirmed",
		})
	}
	items = append(items, account.PersonalRecord{Text: "unconfirmed memory", ReviewStatus: "unreviewed"})
	spans := memoryContextSpans(items)
	if len(spans) != 5 {
		t.Fatalf("memory spans = %d, want 5", len(spans))
	}
	total := 0
	for _, s := range spans {
		if s.Role != ai.RoleMemory {
			t.Fatalf("memory role = %q", s.Role)
		}
		total += utf8.RuneCountInString(s.Text)
	}
	if total > 600 {
		t.Fatalf("memory runes = %d, want <= 600", total)
	}
	if got := memoryContextSpans([]account.PersonalRecord{{Text: strings.Repeat("a", 601), ReviewStatus: "confirmed"}}); len(got) != 0 {
		t.Fatalf("oversized memory was sent: %v", got)
	}
	if got := memoryContextSpans([]account.PersonalRecord{{Text: "unreviewed", ReviewStatus: "unreviewed"}}); len(got) != 0 {
		t.Fatalf("unreviewed memory was sent: %v", got)
	}
	if got := memoryContextSpans([]account.PersonalRecord{{Text: "rejected", ReviewStatus: "rejected"}}); len(got) != 0 {
		t.Fatalf("rejected memory was sent: %v", got)
	}
}

func TestListeningSpansKeepsMemoryInsideAT1(t *testing.T) {
	memory := []ai.Span{}
	for i := 0; i < 5; i++ {
		memory = append(memory, ai.Span{Key: "mem", Role: ai.RoleMemory, Text: "memory"})
	}
	turns := []account.ConversationTurn{}
	for i := 0; i < 12; i++ {
		turns = append(turns, account.ConversationTurn{Role: "user", Text: "turn"})
	}
	spans := listeningSpans(turns, memory)
	if len(spans) != 7 {
		t.Fatalf("spans = %d, want the AT-1 seven-span cap", len(spans))
	}
	for i := range memory {
		if spans[i].Role != ai.RoleMemory {
			t.Fatalf("span %d role = %q, want memory first", i, spans[i].Role)
		}
	}
	if spans[len(spans)-1].Text != "turn" {
		t.Fatalf("last span = %+v, want the current turn", spans[len(spans)-1])
	}
}
