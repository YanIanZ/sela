package ai

import (
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/guard"
)

// TestAgeBandMatrix is ADR-0003 §8.3: Meta and the Meta verifier serve a
// minor band only where it is enabled, and never in production (B-10).
func TestAgeBandMatrix(t *testing.T) {
	dev := Config{Env: EnvDevelopment, AllowedBands: Bands(Band18Plus, Band15To17, BandUnknown)}
	devNoMinors := Config{Env: EnvDevelopment}
	// Refused by Validate; checked here as defence in depth.
	prod := Config{Env: EnvProduction, AllowedBands: Bands(Band18Plus, Band15To17, BandUnknown)}
	cases := []struct {
		role ProviderName
		band AgeBand
		cfg  Config
		want bool
	}{
		{ProviderMeta, Band18Plus, prod, true},
		{ProviderMetaVerifier, Band18Plus, prod, true},
		{ProviderMeta, Band15To17, dev, true},
		{ProviderMetaVerifier, Band15To17, dev, true},
		{ProviderMetaVerifier, BandUnknown, dev, true},
		{ProviderMeta, Band15To17, devNoMinors, false},
		{ProviderMetaVerifier, BandUnknown, devNoMinors, false},
		{ProviderMeta, Band15To17, prod, false},
		{ProviderMetaVerifier, Band15To17, prod, false},
		{ProviderMeta, "under_15", dev, false},
		{ProviderMeta, "", dev, false},
	}
	for _, tc := range cases {
		if got := providerAllowed(tc.role, tc.band, tc.cfg); got != tc.want {
			t.Errorf("%s/%s/%s: %v", tc.role, tc.band, tc.cfg.Env, got)
		}
	}
	if BandUnknown.Adult() || BandUnknown.Audience() != "young_person" || Band18Plus.Audience() != "adult" {
		t.Fatal("unknown band must be treated as not 18+")
	}
}

// TestPurposes: every provider call needs the base AI consent; memory also
// needs personal_memory; AT-6 carries no personal data.
func TestPurposes(t *testing.T) {
	eq := func(a, b []Purpose) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	if !eq(purposesFor(ProviderMeta, TaskListeningReply, false), []Purpose{PurposeAIProcessing}) ||
		!eq(purposesFor(ProviderMetaVerifier, TaskSummaryDraft, false), []Purpose{PurposeAIProcessing}) ||
		purposesFor(ProviderMetaVerifier, TaskReadingVerification, false) != nil {
		t.Fatal("purposes")
	}
	if !eq(purposesFor(ProviderMeta, TaskMemoryProposal, true), []Purpose{PurposeAIProcessing, PurposePersonalMemory}) ||
		!eq(purposesFor(ProviderMetaVerifier, TaskListeningReply, true), []Purpose{PurposeAIProcessing, PurposePersonalMemory}) {
		t.Fatal("memory purposes")
	}
}

// TestRR8NeedsVerification: always for AT-3/AT-4/AT-6, when flagged for
// AT-1/AT-2/AT-5, never for AT-0; guards can only add a requirement.
func TestRR8NeedsVerification(t *testing.T) {
	flagged := guard.Check("Penelitian menunjukkan menulis jurnal membantu.", guard.Options{})
	clean := guard.Check("Terima kasih sudah cerita.", guard.Options{})
	for _, task := range Tasks() {
		pol, _ := PolicyFor(task)
		want := map[Task][2]bool{
			TaskCrisisSupport: {false, false}, TaskListeningReply: {false, true}, TaskReflection: {false, true},
			TaskMemoryProposal: {true, true}, TaskSummaryDraft: {true, true}, TaskOpeningLines: {false, true},
			TaskReadingVerification: {true, true},
		}[task]
		if got := needsVerification(pol, clean); got != want[0] {
			t.Errorf("%s clean: %v", task, got)
		}
		if got := needsVerification(pol, flagged); got != want[1] {
			t.Errorf("%s flagged: %v", task, got)
		}
	}
}

// TestTaxonomy pins the PM-decided structure of AT-0..AT-6 (D-029).
func TestTaxonomy(t *testing.T) {
	type row struct {
		id         string
		generative bool
		modelFree  bool
		verify     Verification
		regenerate bool
	}
	want := map[Task]row{
		TaskCrisisSupport:       {"AT-0", false, true, VerifyNever, false},
		TaskListeningReply:      {"AT-1", true, false, VerifyWhenFlagged, true},
		TaskReflection:          {"AT-2", true, false, VerifyWhenFlagged, true},
		TaskMemoryProposal:      {"AT-3", true, false, VerifyAlways, false},
		TaskSummaryDraft:        {"AT-4", true, false, VerifyAlways, false},
		TaskOpeningLines:        {"AT-5", true, false, VerifyWhenFlagged, true},
		TaskReadingVerification: {"AT-6", false, false, VerifyAlways, false},
	}
	for task, w := range want {
		p, ok := PolicyFor(task)
		if !ok || p.ID != w.id || p.Generative != w.generative || p.ModelFree != w.modelFree ||
			p.Verification != w.verify || p.Regenerate != w.regenerate {
			t.Errorf("%s: %+v", task, p)
		}
	}
	if len(Tasks()) != 7 {
		t.Fatal("seven tasks")
	}
	if p, _ := PolicyFor(TaskListeningReply); p.Budget != 60*time.Second || p.FirstContentWait != 20*time.Second ||
		p.VerifierAttempt != 15*time.Second || p.MaxRetryAfter != 3*time.Second || p.reserves() != 0 {
		t.Fatalf("AT-1 budgets %+v", p)
	}
	if p, _ := PolicyFor(TaskSummaryDraft); p.MaxVerifierCalls != 3 || p.Budget != 0 || p.VerificationReserve != 90*time.Second {
		t.Fatalf("AT-4 budgets %+v", p)
	}
}

// TestRR7RiskOnlyRaises.
func TestRR7RiskOnlyRaises(t *testing.T) {
	if raise(RiskElevated, RiskNone) != RiskElevated || raise(RiskCrisis, RiskElevated) != RiskCrisis ||
		raise(RiskNone, RiskCrisis) != RiskCrisis || raise("", RiskNone) != RiskNone {
		t.Fatal("raise lowered a risk")
	}
}

// TestRR14Breaker.
func TestRR14Breaker(t *testing.T) {
	var b breaker
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	b.failure(now)
	b.failure(now.Add(30 * time.Second))
	if b.isOpen(now.Add(31 * time.Second)) {
		t.Fatal("open after two failures")
	}
	b.success()
	b.failure(now.Add(40 * time.Second))
	b.failure(now.Add(50 * time.Second))
	if b.isOpen(now.Add(51 * time.Second)) {
		t.Fatal("success did not reset")
	}
	b.failure(now.Add(60 * time.Second))
	if !b.isOpen(now.Add(61*time.Second)) || b.isOpen(now.Add(121*time.Second)) {
		t.Fatal("breaker must open for 60 s after three failures")
	}
	var c breaker
	c.failure(now)
	c.failure(now.Add(time.Minute))
	c.failure(now.Add(3 * time.Minute)) // the first is outside the 2-minute window
	if c.isOpen(now.Add(3 * time.Minute)) {
		t.Fatal("failures outside the window counted")
	}
}

func TestCostEstimate(t *testing.T) {
	if c := estimateCost(ProviderMetaAPI, false, 1_000_000, 0); c != 1.25 {
		t.Fatalf("meta api input %f", c)
	}
	if c := estimateCost(ProviderMetaVerifier, false, 0, 1_000_000); c != 4.25 {
		t.Fatalf("meta verifier output %f", c)
	}
	if estimateCost(ProviderMeta, false, 1000, 1000) != 0 || estimateCost(ProviderMetaVerifier, true, 1000, 1000) != 0 {
		t.Fatal("self-hosted meta and test doubles carry no API cost")
	}
}
