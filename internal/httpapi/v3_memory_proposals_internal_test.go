package httpapi

import (
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
)

// TestMemoryProposalSpansKeepsOnlySendableUserTurns: AT-3 sees the newest
// user turns that fit the span cap, never an assistant reply, a crisis
// turn or an over-long turn, and only confirmed/corrected memory.
func TestMemoryProposalSpansKeepsOnlySendableUserTurns(t *testing.T) {
	turns := []account.ConversationTurn{
		{Role: "user", Text: "Aku suka menulis."},
		{Role: "assistant", Text: "Sela reply, never a proposal source."},
		{Role: "user", Text: "Aku mau bunuh diri malam ini."},
		{Role: "user", Text: strings.Repeat("a", 1601)},
		{Role: "user", Text: "Aku ingin tidur lebih awal."},
	}
	items := []account.PersonalRecord{
		{Text: "Tidur lebih awal.", ReviewStatus: account.ReviewConfirmed},
		{Text: "unreviewed memory", ReviewStatus: account.ReviewUnreviewed},
		{Text: "rejected memory", ReviewStatus: account.ReviewRejected},
	}
	spans, quotes := memoryProposalSpans(turns, items)
	if len(spans) != 3 {
		t.Fatalf("spans = %+v, want one memory item and two user turns", spans)
	}
	if spans[0].Role != ai.RoleMemory || spans[0].Text != "Tidur lebih awal." {
		t.Fatalf("memory span: %+v", spans[0])
	}
	if spans[1].Key != "turn-0" || spans[1].Role != ai.RoleUser || spans[2].Key != "turn-4" || spans[2].Role != ai.RoleUser {
		t.Fatalf("turn spans: %+v", spans)
	}
	if quotes["turn-0"] != "Aku suka menulis." || quotes["turn-4"] != "Aku ingin tidur lebih awal." {
		t.Fatalf("quotes: %v", quotes)
	}
	for _, forbidden := range []string{"turn-2", "turn-3"} {
		if _, ok := quotes[forbidden]; ok {
			t.Fatalf("unsafe or too long turn %s reached AT-3", forbidden)
		}
	}
	for _, s := range spans {
		if s.Role == ai.RoleUser && (strings.Contains(s.Text, "bunuh diri") || len(s.Text) > 1600) {
			t.Fatalf("unsafe turn reached AT-3: %+v", s)
		}
	}
	if spans, quotes := memoryProposalSpans([]account.ConversationTurn{{Role: "assistant", Text: "x"}}, nil); spans != nil || quotes != nil {
		t.Fatalf("no user turn should mean no input: %v %v", spans, quotes)
	}
}
