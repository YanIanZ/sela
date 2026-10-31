package ai

// Regression tests for the second review round of the AI layer (policy and
// privacy, 2026-10-26). Each test names the rule it pins and reproduces
// the reviewer's probe.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/transport"
	"github.com/YanIanZ/sela/internal/ai/verifier"
	"github.com/YanIanZ/sela/internal/llm"
)

// queuedAdmission is a saturated llm.Admission (one slot, held by another
// call) that reports when a caller starts waiting for it.
type queuedAdmission struct {
	adm     *llm.Admission
	entered chan struct{}
	once    sync.Once
}

func (a *queuedAdmission) Acquire(ctx context.Context) (func(), error) {
	a.once.Do(func() { close(a.entered) })
	return a.adm.Acquire(ctx)
}

// RR-2, §8.2, D-029 ("withdrawing one stops its use at once"): a
// withdrawal or a band change while the Meta call waits in a saturated
// admission queue stops the call before anything is sent.
func TestMetaRecheckedAfterAdmissionWait(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*consentFake)
		calls  int32
		want   Outcome
	}{
		{"ai_processing withdrawn", func(c *consentFake) { c.withdraw(PurposeAIProcessing) }, 0, OutcomeStale},
		{"band no longer allowed", func(c *consentFake) { c.setBand(Band15To17) }, 0, OutcomeStale},
		{"nothing changed", func(*consentFake) {}, 1, OutcomeReleased},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := allConsent()
			var calls atomic.Int32
			stub := &llmStub{mode: llm.ModeTestDouble, fn: func(ctx context.Context, _ llm.Request, onDelta func(string) error) (llm.Result, error) {
				calls.Add(1)
				_ = onDelta("x")
				return llm.Result{Text: reply("Terima kasih sudah cerita."), Model: llm.TestDoubleServedModel, Attempts: 1}, nil
			}}
			adm := &queuedAdmission{adm: llm.NewAdmission(1), entered: make(chan struct{})}
			release, err := adm.adm.Acquire(context.Background()) // another call holds the only slot
			if err != nil {
				t.Fatal(err)
			}
			go func() {
				<-adm.entered
				tc.change(c)
				release()
			}()
			cfg := testConfig()
			cfg.AllowedBands = Bands(Band18Plus)
			r, err := NewRouter(cfg, Deps{Meta: newMetaGenerator(stub, adm, time.Millisecond), Consent: c, Screener: screenFake{}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = r.Run(context.Background(), TaskListeningReply, inReply())
			if calls.Load() != tc.calls || OutcomeOf(err) != tc.want {
				t.Fatalf("outcome %s (%v), Meta calls %d, want %s and %d", OutcomeOf(err), err, calls.Load(), tc.want, tc.calls)
			}
		})
	}
}

// VC-1 (AT-1): the user turn a flagged reply answers is in every VC-1
// request, whole. Earlier turns are added newest first and only whole, and
// a current turn too long for one claim abstains instead of being cut.
func TestAT1CurrentTurnAlwaysVerified(t *testing.T) {
	old := "Dulu dokter bilang aku kena insomnia. " + strings.Repeat("Aku cerita panjang soal kuliah dan kos. ", 120)
	latest := "LATEST-TURN Ternyata dokternya salah, aku nggak insomnia sama sekali."
	h := newHarness(t)
	h.meta.steps = []genStep{{text: reply("Insomnia memang berat ya. Kamu boleh cerita lagi.")}}
	h.verifier.steps = []verStep{{report: func(r verifier.Request) verifier.Report {
		// The verifier sees the latest turn and finds the reply contradicts it.
		return withVerdicts(r, map[int]verifier.Verdict{0: verifier.Contradicted})
	}}}
	_, re := h.run(TaskListeningReply, inReply(old, "Oke.", latest))
	if h.verifier.count() != 1 {
		t.Fatalf("verifier calls %d (%v)", h.verifier.count(), re)
	}
	for _, req := range h.verifier.reqs {
		text := map[string]string{}
		for _, s := range req.Spans {
			text[s.ID] = s.Text
		}
		for _, c := range req.Claims {
			if strings.Join(c.SpanIDs, ",") != "s2,s3" || text["s3"] != latest {
				t.Fatalf("claim %s cites %v; spans %v", c.ID, c.SpanIDs, req.Spans)
			}
		}
	}
	expect(t, re, OutcomeAbstain, ReasonWithheld)

	// Every turn fits: all are cited, oldest first.
	h2 := newHarness(t)
	h2.meta.steps = []genStep{{text: reply("Insomnia memang berat ya.")}}
	if _, re := h2.run(TaskListeningReply, inReply("Aku sering susah tidur.", "Oke.", latest)); re != nil {
		t.Fatal(re)
	}
	if got := strings.Join(h2.verifier.reqs[0].Claims[0].SpanIDs, ","); got != "s1,s2,s3" {
		t.Fatalf("cited %s", got)
	}

	// A current turn of more than MaxCitedSpans pieces is never cut.
	h3 := newHarness(t)
	h3.meta.steps = []genStep{{text: reply("Insomnia memang berat ya.")}}
	_, re = h3.run(TaskListeningReply, inReply(strings.Repeat("ééééé ", 1000)))
	expect(t, re, OutcomeAbstain, ReasonInputTooLarge)
	if h3.verifier.count() != 0 || re.Provenance.Verification.Status != StatusAbstained {
		t.Fatalf("verifier calls %d status %s", h3.verifier.count(), re.Provenance.Verification.Status)
	}
}

// Evidence is never cut for any task: a claim whose cited spans do not fit
// VC-1 whole abstains (verification_input_too_large). (AT-4 items cite
// exactly one span each, so the multi-span claim is an AT-3 proposal.)
func TestVerifierEvidenceNeverTruncated(t *testing.T) {
	long := strings.Repeat("ééééé ", 260) // about 2,800 bytes: two pieces per span
	h := newHarness(t)
	in := inReply(long, long, long)
	h.meta.steps = []genStep{{text: genJSON("none", false, [2]string{"Saya sulit tidur.", "s1,s2,s3"})}}
	_, re := h.run(TaskMemoryProposal, in)
	expect(t, re, OutcomeAbstain, ReasonInputTooLarge)
	if h.verifier.count() != 0 {
		t.Fatal("partial evidence was sent to the verifier")
	}
}

// CK-1, VC-3, RR-9(b): Sela's own earlier, unverified reply is never
// evidence for a flagged AT-2 card. Only the user's spans go to the
// verifier, and a card that rests on an earlier reply alone is withheld
// without a verifier call.
func TestAT2EarlierReplyIsNotEvidence(t *testing.T) {
	spans := []Span{
		{Key: "turn-a", Role: RoleUser, Text: "Aku capek banget sama skripsi."},
		{Key: "reply-a", Role: RoleEarlierReply, Text: "Sepertinya kamu sedang burnout dan mengalami insomnia karena skripsi."},
	}
	in := Input{Subject: subject, Language: "id", Spans: spans}
	h := newHarness(t)
	h.meta.steps = []genStep{{text: genJSON("none", false, [2]string{"Kamu sedang burnout dan mengalami insomnia karena skripsi.", "s2"})}}
	out, re := h.run(TaskReflection, in)
	expect(t, re, OutcomeAbstain, ReasonNoUserEvidence)
	if h.verifier.count() != 0 || len(out.Items) != 0 || re.Provenance.Verification.Status != StatusAbstained {
		t.Fatalf("verifier calls %d, items %d, status %s", h.verifier.count(), len(out.Items), re.Provenance.Verification.Status)
	}

	// A card citing both: only the user's span is sent and cited, and the
	// released card points at the user's words only.
	h2 := newHarness(t)
	h2.meta.steps = []genStep{{text: genJSON("none", false, [2]string{"Kamu sedang burnout karena skripsi.", "s1,s2"})}}
	out, re = h2.run(TaskReflection, in)
	if re != nil {
		t.Fatal(re)
	}
	req := h2.verifier.reqs[0]
	if len(req.Spans) != 1 || req.Spans[0].ID != "s1" || strings.Join(req.Claims[0].SpanIDs, ",") != "s1" {
		t.Fatalf("VC-1 spans %+v claim %+v", req.Spans, req.Claims[0])
	}
	if strings.Contains(fmtJSON(t, h2.verifier.reqs), "Sepertinya") {
		t.Fatal("the earlier reply reached the verifier")
	}
	if len(out.Items) != 1 || strings.Join(out.Items[0].SpanKeys, ",") != "turn-a" {
		t.Fatalf("items %+v", out.Items)
	}
}

func fmtJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// VC-8, §3 AT-6 ("the item stays unpublished"): an output-level issue or a
// safety signal withholds the reading item, and its record says so.
func TestAT6WithholdsOnOutputLevelIssue(t *testing.T) {
	in := Input{Language: "id", Spans: []Span{{Key: "R-012#p3", Role: RoleSource, Text: "Menulis ekspresif selama 15 menit mengurangi stres pada mahasiswa dalam studi ini."}},
		Claims: []ClaimInput{{Key: "draft-1", Text: "Menulis ekspresif mengurangi stres pada mahasiswa.", SpanKeys: []string{"R-012#p3"}}}}
	for _, tc := range []struct {
		cat   verifier.Category
		claim string
	}{
		{verifier.CitationInvalid, ""}, {verifier.InformationLeak, ""}, {verifier.MedicalLanguage, ""}, {verifier.SafetySignal, ""},
		{verifier.Manipulation, ""}, {verifier.AgeInappropriate, ""}, {verifier.LostContext, ""},
		{verifier.MedicalLanguage, "c1"}, {verifier.InformationLeak, "c1"}, {verifier.SafetySignal, "c1"},
	} {
		t.Run(string(tc.cat)+"/"+tc.claim, func(t *testing.T) {
			h := newHarness(t)
			h.verifier.consent = nil
			h.verifier.steps = []verStep{{report: func(r verifier.Request) verifier.Report {
				return withVerdicts(r, nil, verifier.Issue{Category: tc.cat, ClaimID: tc.claim, SpanIDs: []string{}, Reason: "whole item"})
			}}}
			out, re := h.run(TaskReadingVerification, in)
			expect(t, re, OutcomeAbstain, ReasonWithheld)
			v := re.Provenance.Verification
			if len(out.Items) != 0 || !v.WithheldAll || v.Released != 0 || v.Status != StatusChecked || !strings.Contains(strings.Join(re.Reasons, ","), string(tc.cat)) {
				t.Fatalf("items %d, verification %+v, reasons %v", len(out.Items), v, re.Reasons)
			}
		})
	}
}

// VC-8: medical_language, manipulation, age_inappropriate and
// information_leak withhold the whole output wherever the verifier
// attached them; lost_context on one claim falls back for that claim only.
func TestClaimScopedOutputLevelIssueWithholdsAll(t *testing.T) {
	items := []string{"Aku susah tidur sejak dua bulan.", "Aku sering pusing di kampus.", "Aku jarang makan siang."}
	for _, cat := range []verifier.Category{verifier.Manipulation, verifier.InformationLeak, verifier.MedicalLanguage, verifier.AgeInappropriate} {
		h, in := summaryHarness(t, items...)
		h.verifier.steps = []verStep{{report: func(r verifier.Request) verifier.Report {
			return withVerdicts(r, nil, verifier.Issue{Category: cat, ClaimID: "c2", SpanIDs: []string{"s2"}, Reason: "x"})
		}}}
		out, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeAbstain, ReasonWithheld)
		if len(out.Items) != 0 || !re.Provenance.Verification.WithheldAll || !strings.Contains(strings.Join(re.Reasons, ","), string(cat)) {
			t.Fatalf("%s: items %d reasons %v", cat, len(out.Items), re.Reasons)
		}
	}
	h, in := summaryHarness(t, items...)
	h.verifier.steps = []verStep{{report: func(r verifier.Request) verifier.Report {
		return withVerdicts(r, nil, verifier.Issue{Category: verifier.LostContext, ClaimID: "c2", SpanIDs: []string{"s2"}, Reason: "x"})
	}}}
	out, re := h.run(TaskSummaryDraft, in)
	if re != nil || len(out.Items) != 3 || out.Items[1].Source != SourceUserQuote || out.Items[0].Source != SourceAIWording {
		t.Fatalf("lost_context: %v %+v", re, out.Items)
	}
}

// cutServer answers 200 with the start of a body (a refusal and a
// contradicted verdict are in it), then breaks the connection.
func cutServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Header().Set(transport.HeaderTestDouble, "true")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "5000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"type":"message","model":"test-double-not-claude","stop_reason":"refusal","content":[{"type":"text","text":"{\"claims\":[{\"claim_id\":\"c1\",\"verdict\":\"contradicted\"`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("server cannot hijack")
			return
		}
		if conn, _, err := hj.Hijack(); err == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

// countLimiter counts Allow per provider.
type countLimiter struct {
	mu sync.Mutex
	n  map[ProviderName]int
}

func (l *countLimiter) Allow(_ context.Context, _ Subject, p ProviderName, _ Task) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.n[p]++
	return nil
}

// Check never counts.
func (l *countLimiter) Check(context.Context, Subject, ProviderName, Task) error { return nil }

func (l *countLimiter) get(p ProviderName) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.n[p]
}

// §11, the Limiter contract ("called right before every provider call"):
// Allow is asked once per generator or verifier call.
func TestLimiterAskedOncePerProviderCall(t *testing.T) {
	h, in := summaryHarness(t, "Aku susah tidur sejak dua bulan.")
	lim := &countLimiter{n: map[ProviderName]int{}}
	h.lim = lim
	if _, re := h.run(TaskSummaryDraft, in); re != nil {
		t.Fatal(re)
	}
	for p, calls := range map[ProviderName]int{ProviderMeta: h.meta.count(), ProviderMetaVerifier: h.verifier.count()} {
		if calls != 1 || lim.get(p) != calls {
			t.Errorf("%s: %d calls, Allow asked %d times", p, calls, lim.get(p))
		}
	}
}

// sentText is everything the fakes received: the generators' prompts and
// the VC-1 requests.
func sentText(t *testing.T, h *harness) string {
	t.Helper()
	var b strings.Builder
	for i := 0; i < h.meta.count(); i++ {
		b.WriteString(h.meta.req(i).User)
	}
	b.WriteString(fmtJSON(t, h.verifier.reqs))
	return b.String()
}

// DM-2, NAME-1: a private name followed by a possessive, an enclitic or the
// reduplication digit, or with repeated letters, never reaches Meta, the
// DeepSeek fallback or Claude, and a generated text naming it that way is
// blocked.
func TestPrivateNameFormsNeverReachProviders(t *testing.T) {
	texts := []string{
		"I told Raka's mom everything and now I feel awful.",
		"Aku cerita ke Raka’s family, terus nyesel.",
		"Rakanya nggak bales chat aku dari kemarin.",
		"Aku kangen Dewiku, Dewiii kenapa sih.",
		"Dewi2 itu temenku, DEWI marah, dewi_lestari bilang, Dewinyalah yang salah.",
	}
	h := newHarness(t)
	var gen [][2]string
	for i := range texts {
		gen = append(gen, [2]string{"Saya menulis hal nomor " + itoa(i+1) + ".", "s" + itoa(i+1)})
	}
	h.meta.steps = []genStep{{text: genJSON("none", false, gen...)}}
	in := inItems(texts...)
	in.PrivateNames = []string{"Raka", "Dewi"}
	if _, re := h.run(TaskSummaryDraft, in); re != nil {
		t.Fatal(re)
	}
	if h.meta.count() != 1 || h.verifier.count() != 1 {
		t.Fatalf("calls: meta %d verifier %d", h.meta.count(), h.verifier.count())
	}
	sent := strings.ToLower(sentText(t, h))
	for _, name := range []string{"raka", "dewi"} {
		if strings.Contains(sent, name) {
			t.Fatalf("a provider received %q", name)
		}
	}
	for _, out := range []string{"I told Raka's mom everything.", "Raka’s family tahu.", "Saya kesal sama Rakanya.",
		"Dewiii marah.", "Dewiku baik.", "Saya sudah cerita ke Raka."} {
		h := newHarness(t)
		h.meta.steps = []genStep{{text: genJSON("none", false, [2]string{out, "s1"})}}
		in := inItems("Aku sudah cerita ke [NAMA-1] soal ini.")
		in.PrivateNames = []string{"Raka", "Dewi"}
		_, re := h.run(TaskSummaryDraft, in)
		expect(t, re, OutcomeAbstain, "guard:private_name")
	}
}

// DM-2: a name the user introduced themselves with is redacted everywhere
// in the request (every span, also one before the introduction), with one
// placeholder for the full name and the given name, and NAME-1 blocks it
// in the output.
func TestSelfIntroducedNameRedactedEverywhere(t *testing.T) {
	h := newHarness(t)
	in := inReply(
		"Raka capek terus akhir-akhir ini.",
		"Namaku Raka Putra. Jujur Raka Putra itu capek terus.",
		"Teman-teman manggil aku Raka, tapi Raka nggak suka.",
		"Aku biasa dipanggil Bima, Bimanya sih santai.",
		"My name is Dewi Lestari and Dewi never sleeps.",
	)
	out, re := h.run(TaskListeningReply, in)
	if re != nil {
		t.Fatal(re)
	}
	sent := h.meta.req(0).User
	for _, leak := range []string{"Raka", "Putra", "Bima", "Dewi", "Lestari"} {
		if strings.Contains(sent, leak) {
			t.Fatalf("Meta received %q", leak)
		}
	}
	var gi genInput
	if err := json.Unmarshal([]byte(sent[strings.Index(sent, "{"):strings.LastIndex(sent, "}")+1]), &gi); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"[NAMA-1] capek terus akhir-akhir ini.",
		"Namaku [NAMA-1]. Jujur [NAMA-1] itu capek terus.",
		"Teman-teman manggil aku [NAMA-1], tapi [NAMA-1] nggak suka.",
		"Aku biasa dipanggil [NAMA-2], [NAMA-2]nya sih santai.",
		"My name is [NAMA-3] and [NAMA-3] never sleeps.",
	}
	for i, s := range gi.Spans {
		if s.Text != want[i] {
			t.Errorf("span %d sent as %q, want %q", i+1, s.Text, want[i])
		}
	}
	if out.Restore["[NAMA-1]"] != "Raka Putra" || out.Restore["[NAMA-2]"] != "Bima" || out.Restore["[NAMA-3]"] != "Dewi Lestari" {
		t.Fatalf("restore %v", out.Restore)
	}
	for _, gen := range []string{"Raka Putra, kamu nggak sendirian.", "Raka, kamu nggak sendirian."} {
		h := newHarness(t)
		h.meta.steps = []genStep{{text: reply(gen)}}
		_, re := h.run(TaskListeningReply, inReply("Namaku Raka Putra. Aku capek."))
		expect(t, re, OutcomeAbstain, "guard:private_name")
	}
}

// Placeholders already in a span (an earlier reply that was redacted
// before) keep their numbers, and the session numbers new values after
// them, whatever the span order: two people never share a placeholder.
func TestPlaceholderNumbersNeverCollide(t *testing.T) {
	for _, order := range []string{"earlier reply first", "user turn first"} {
		h := newHarness(t)
		spans := []Span{
			{Key: "r1", Role: RoleEarlierReply, Text: "Kedengarannya [NAMA-1] lagi kesal sama kamu, cek [KONTAK-3] ya."},
			{Key: "u2", Role: RoleUser, Text: "Iya, terus Dewi malah ikut-ikutan, emailnya dewi@example.com."},
		}
		if order == "user turn first" {
			spans[0], spans[1] = spans[1], spans[0]
		}
		in := Input{Subject: subject, Language: "id", PrivateNames: []string{"Raka", "Dewi"}, Spans: spans}
		out, re := h.run(TaskListeningReply, in)
		if re != nil {
			t.Fatal(re)
		}
		sent := h.meta.req(0).User
		if !strings.Contains(sent, "terus [NAMA-2] malah") || !strings.Contains(sent, "emailnya [KONTAK-4]") ||
			!strings.Contains(sent, "Kedengarannya [NAMA-1] lagi") {
			t.Fatalf("%s: sent %s", order, sent[strings.Index(sent, "{"):])
		}
		if len(out.Restore) != 2 || out.Restore["[NAMA-2]"] != "Dewi" || out.Restore["[KONTAK-4]"] != "dewi@example.com" {
			t.Fatalf("%s: restore %v", order, out.Restore)
		}
	}
}

// §3.1: AT-1 sends at most 5 confirmed memory items of at most 600
// characters together, AT-3 at most 10 memory items; more is invalid input
// and nothing is sent.
func TestMemoryCapsPerTask(t *testing.T) {
	mem := func(n, runes int) []Span {
		var out []Span
		for i := 0; i < n; i++ {
			out = append(out, Span{Key: "mem" + itoa(i), Role: RoleMemory, Text: "Aku perawat " + strings.Repeat("a", runes-12)})
		}
		return out
	}
	turn := Span{Key: "u", Role: RoleUser, Text: "Aku capek."}
	for _, tc := range []struct {
		name string
		task Task
		mem  []Span
		ok   bool
	}{
		{"AT-1 five items of 120", TaskListeningReply, mem(5, 120), true},
		{"AT-1 six items", TaskListeningReply, mem(6, 20), false},
		{"AT-1 five items of 130 (650 characters)", TaskListeningReply, mem(5, 130), false},
		{"AT-1 one item of 860", TaskListeningReply, mem(1, 860), false},
		{"AT-3 ten items", TaskMemoryProposal, mem(10, 100), true},
		{"AT-3 eleven items", TaskMemoryProposal, mem(11, 100), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			in := Input{Subject: subject, Language: "id", Spans: append(append([]Span(nil), tc.mem...), turn)}
			_, re := h.run(tc.task, in)
			if tc.ok {
				if h.meta.count() != 1 {
					t.Fatalf("Meta not called: %v", re)
				}
				return
			}
			expect(t, re, OutcomeInvalidInput, ReasonRoleLimit)
			noCalls(t, h)
		})
	}
}

// DM-2, CON-1: contact links without a scheme (any host with a path,
// common link hosts) and lowercase street addresses never reach a
// provider and are blocked in generated output; ordinary words and
// numbers stay.
func TestContactLinksNeverReachProviders(t *testing.T) {
	texts := []string{
		"Kalau mau dukung, cek saweria.co/rakaputra atau linktr.ee/rakaputra, server discord.gg/rk7Qx2.",
		"Video aku di youtu.be/xYz123 dan twitch.tv/rakalive, profil di beacons.ai/rakap.",
		"Rumahku di jalan melati no 12 bekasi, kosku di Gg. Mawar 3 No. 7.",
		"Aku jalan kaki ke kampus, nilaiku 7.5/10 minggu ini.",
		// Review round 3: whitespace around a phone separator.
		"WA 0812 - 3456 - 7890 atau 0813  5555  6666.",
		"WA 0812 -4444- 7891, kantor 0812 – 2222 – 7892.",
		"WA +62 812 - 1111 - 7893 atau 0812 . 3333 . 7894.",
	}
	h, in := summaryHarness(t, texts...)
	var gen [][2]string
	for i := range texts {
		gen = append(gen, [2]string{"Saya menulis hal nomor " + itoa(i+1) + ".", "s" + itoa(i+1)})
	}
	h.meta.steps = []genStep{{text: genJSON("none", false, gen...)}}
	if _, re := h.run(TaskSummaryDraft, in); re != nil {
		t.Fatal(re)
	}
	sent := sentText(t, h)
	for _, leak := range []string{"rakaputra", "rk7Qx2", "xYz123", "rakalive", "rakap", "melati", "Mawar", "3456", "5555", "4444",
		"2222", "1111", "3333", "789"} {
		if strings.Contains(sent, leak) {
			t.Fatalf("a provider received %q", leak)
		}
	}
	for _, keep := range []string{"jalan kaki ke kampus", "7.5/10"} {
		if !strings.Contains(h.meta.req(0).User, keep) {
			t.Fatalf("ordinary text %q was redacted", keep)
		}
	}
	for _, gen := range []string{"Dukung saya di saweria.co/rakaputra.", "Saya di linktr.ee/raka.", "Rumah saya di jalan melati no 12.",
		"WA saya 0812 - 3456 - 7890.", "WA saya 0812 . 3456 . 7890."} {
		h := newHarness(t)
		h.meta.steps = []genStep{{text: genJSON("none", false, [2]string{gen, "s1"})}}
		_, re := h.run(TaskSummaryDraft, inItems("Aku punya halaman dukungan."))
		expect(t, re, OutcomeAbstain, "guard:contact_detail")
	}
}

// Finding 17's rules for Meta: a test double never receives the live vLLM
// key or the Modal pair (only Config.TestDoubleAPIKey) and never runs at a
// Modal or other live host; production requires an https Modal host.
func TestMetaTestDoubleNeverGetsLiveKeys(t *testing.T) {
	var mu sync.Mutex
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = r.Header.Clone()
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	live := func(base string) llm.Config {
		return llm.Config{Mode: llm.ModeTestDouble, BaseURL: base, ServedModel: llm.TestDoubleServedModel,
			APIKey: "LIVE-VLLM-KEY", ModalKey: "LIVE-MODAL-KEY", ModalSecret: "LIVE-MODAL-SECRET"}
	}
	headers := func(g Generator) http.Header {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = g.Generate(ctx, GenerateRequest{Task: TaskListeningReply, System: "s", User: "u", MaxTokens: 10})
		mu.Lock()
		defer mu.Unlock()
		return got
	}
	g, err := NewMetaGenerator(live(srv.URL), nil)
	if err != nil {
		t.Fatal(err)
	}
	if h := headers(g); h.Get("Authorization") != "" || h.Get("Modal-Key") != "" || h.Get("Modal-Secret") != "" {
		t.Fatalf("test double received %v", h)
	}
	cfg := testConfig()
	cfg.MetaVerifier.Mode = ModeDisabled
	cfg.TestDoubleAPIKey = transport.NewSecret("td-key")
	r, err := Build(cfg, live(srv.URL), Deps{Consent: allConsent()})
	if err != nil {
		t.Fatal(err)
	}
	if h := headers(r.meta); h.Get("Authorization") != "Bearer td-key" || h.Get("Modal-Key") != "" || h.Get("Modal-Secret") != "" {
		t.Fatalf("test double received %v", h)
	}
	for _, base := range []string{"https://sela--glimmer.modal.run", "https://sela-fakellm.modal.run", "https://sela--glimmer.modal.direct",
		"https://api.example.com"} {
		if _, err := NewMetaGenerator(live(base), nil); err == nil {
			t.Errorf("Meta test double accepted at %s", base)
		}
		staging := cfg
		staging.Env = EnvStaging
		if _, err := Build(staging, live(base), Deps{Consent: allConsent()}); err == nil {
			t.Errorf("Build accepted a Meta test double at %s", base)
		}
	}
	prod := Config{Env: EnvProduction, Profile: ProfileFull}
	modal := func(base string) llm.Config {
		return llm.Config{Mode: llm.ModeModal, BaseURL: base, APIKey: "k", ModalKey: "mk", ModalSecret: "ms",
			ServedModel: "sela-glimmer-cec94fc", ModelID: "meta-models/Muse-Glimmer-30B", Revision: "cec94fc"}
	}
	for base, ok := range map[string]bool{"https://sela--glimmer.modal.run": true, "https://ws--sela.modal.direct": true,
		"https://glimmer.example.com": false, "https://sela--glimmer.modal.run:8443": false} {
		if _, err := Build(prod, modal(base), Deps{Consent: allConsent()}); (err == nil) != ok {
			t.Errorf("production Meta at %s: err %v", base, err)
		}
	}
}
