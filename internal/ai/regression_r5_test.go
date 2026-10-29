package ai

import (
	"strings"
	"testing"
)

// DM-2, NAME-1 (review round 5): a part of a private name that is also an
// everyday word never reaches Meta or DeepSeek where it names the person:
// capitalised after a lowercase word in the middle of a sentence, also
// inside a fixed phrase ("kangen Indah banget", "ke Bagus ya"), and in
// lowercase for a part whose everyday meaning is rare ("bayu", "agung").
// A reply that repeats it is blocked.
func TestR5CommonNamePartNeverReachesProviders(t *testing.T) {
	for _, tc := range []struct{ priv, turn, part string }{
		{"Indah Permata", "Aku kangen Indah banget.", "indah"},
		{"Bagus Pratama", "Tadi aku cerita ke Bagus ya, terus dia diam.", "bagus"},
		{"Bagus Pratama", "Aku udah ngomong sama Bagus kok.", "bagus"},
		{"Putri Ayu", "Aku ketemu Putri kami di sekolah.", "putri"},
		{"Bayu Pratama", "td aku berantem sama bayu, dia marah", "bayu"},
		{"Intan Lestari", "aku kangen intan", "intan"},
		{"Agung Wibowo", "agung blm bales chat aku", "agung"},
	} {
		h := newHarness(t)
		h.meta.steps = []genStep{unavailableErr()}
		in := inReply(tc.turn)
		in.PrivateNames = []string{tc.priv}
		if _, re := h.run(TaskListeningReply, in); re != nil {
			t.Fatalf("%q: %s %v", tc.turn, re.Outcome, re.Reasons)
		}
		if h.meta.count() != 1 || h.ds.count() != 1 {
			t.Fatalf("%q: calls meta %d deepseek %d", tc.turn, h.meta.count(), h.ds.count())
		}
		for _, req := range []GenerateRequest{h.meta.req(0), h.ds.req(0)} {
			if strings.Contains(strings.ToLower(req.User), tc.part) {
				t.Fatalf("%q: a provider received %q", tc.turn, tc.part)
			}
		}
	}
	for _, tc := range []struct{ priv, out string }{
		{"Indah Permata", "Kamu kangen Indah banget ya."},
		{"Bagus Pratama", "Kamu sudah cerita ke Bagus ya."},
		{"Bayu Pratama", "Mungkin bayu juga lagi capek."},
	} {
		h := newHarness(t)
		h.meta.steps = []genStep{{text: reply(tc.out)}}
		in := inReply("Aku capek.")
		in.PrivateNames = []string{tc.priv}
		_, re := h.run(TaskListeningReply, in)
		expect(t, re, OutcomeAbstain, ReasonGuardRejected)
		if !hasReason(re, "guard:private_name") || h.ds.count() != 0 {
			t.Fatalf("%q: reasons %v deepseek %d", tc.out, re.Reasons, h.ds.count())
		}
	}
	// The everyday words still pass: a reply that uses them is not blocked.
	h := newHarness(t)
	h.meta.steps = []genStep{{text: reply("Indah banget caramu bercerita, itu bagus kok.")}}
	in := inReply("Aku capek.")
	in.PrivateNames = []string{"Indah Permata", "Bagus Pratama"}
	if _, re := h.run(TaskListeningReply, in); re != nil {
		t.Fatalf("everyday words blocked: %s %v", re.Outcome, re.Reasons)
	}
}

// DM-2, CON-1 (review round 5): a phone number right after a date written
// with dots, dashes or spaces reaches neither Meta nor DeepSeek, not even
// its last groups.
func TestR5PhoneAfterDottedDateNeverReachesProviders(t *testing.T) {
	for _, turn := range []string{"Jadwal kontrol 05.11.2026 0812 3456 789 ya.", "Kontrol 05-11-2026 0812-3456-789 ya.",
		"Kontrol 05 11 2026 0812 3456 789 ya.", "Kontrol 05.11.2026 0812 345 678 ya."} {
		h := newHarness(t)
		h.meta.steps = []genStep{unavailableErr()}
		if _, re := h.run(TaskListeningReply, inReply(turn)); re != nil {
			t.Fatalf("%q: %s %v", turn, re.Outcome, re.Reasons)
		}
		if h.meta.count() != 1 || h.ds.count() != 1 {
			t.Fatalf("%q: calls meta %d deepseek %d", turn, h.meta.count(), h.ds.count())
		}
		for _, req := range []GenerateRequest{h.meta.req(0), h.ds.req(0)} {
			for _, leak := range []string{"0812", "3456", "789", "345 678"} {
				if strings.Contains(req.User, leak) {
					t.Fatalf("%q: a provider received %q", turn, leak)
				}
			}
		}
	}
}

// DM-2 (review round 5): a self-introduction that repeats a common part of
// a private name ("Namaku Kasih" with "Kasih Ramadhani" private) makes it
// a name in any case, in every span of the request, as in a request
// without private names (so a reply with "terima kasih" is blocked too).
func TestR5SelfIntroducedCommonPartRedactedInAnyCase(t *testing.T) {
	h := newHarness(t)
	h.meta.steps = []genStep{unavailableErr()}
	h.ds.steps = []genStep{{text: reply("Aku dengar kamu.")}}
	in := inReply("Namaku Kasih.", "Temen-temen bilang kasih itu pendiam.")
	in.PrivateNames = []string{"Kasih Ramadhani"}
	if _, re := h.run(TaskListeningReply, in); re != nil {
		t.Fatalf("%s %v", re.Outcome, re.Reasons)
	}
	if sent := strings.ToLower(sentText(t, h)); strings.Contains(sent, "kasih") {
		t.Fatal("a provider received the self-introduced name")
	}
	h2 := newHarness(t)
	h2.meta.steps = []genStep{{text: reply("Semoga kasih bisa lebih terbuka.")}}
	in2 := inReply("Namaku Kasih.", "Aku capek.")
	in2.PrivateNames = []string{"Kasih Ramadhani"}
	_, re := h2.run(TaskListeningReply, in2)
	expect(t, re, OutcomeAbstain, ReasonGuardRejected)
	if !hasReason(re, "guard:private_name") {
		t.Fatalf("reasons %v", re.Reasons)
	}
}
