package ai

import (
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/ai/guard"
	"github.com/YanIanZ/sela/internal/ai/transport"
)

func okFallback() fallbackFacts {
	return fallbackFacts{Failure: transport.ClassAvailability, TaskAllows: true, FlagOn: true, Profile: ProfileFull,
		Configured: true, Remaining: time.Minute, Reserve: 20 * time.Second, Eligible: true, DataAllowed: true, VersionCheck: true}
}

// TestRR4FallbackDecision: DeepSeek replaces the generator only when every
// condition of RR-4 holds; RR-5 and RR-6 are terminal.
func TestRR4FallbackDecision(t *testing.T) {
	if r := fallbackDecision(okFallback()); r != "" {
		t.Fatalf("all conditions hold, got %q", r)
	}
	cases := map[string]struct {
		mod  func(*fallbackFacts)
		want string
	}{
		"(a) content failure (RR-5)":            {func(f *fallbackFacts) { f.Failure = transport.ClassContent }, ReasonNotAvailability},
		"(a) integrity failure":                 {func(f *fallbackFacts) { f.Failure = transport.ClassIntegrity }, ReasonNotAvailability},
		"(a) policy (user limit, kill)":         {func(f *fallbackFacts) { f.Failure = transport.ClassPolicy }, ReasonNotAvailability},
		"(a) after content started":             {func(f *fallbackFacts) { f.ContentStarted = true }, ReasonContentStarted},
		"(b) task disallows":                    {func(f *fallbackFacts) { f.TaskAllows = false }, ReasonTaskDisallows},
		"(e) flag off":                          {func(f *fallbackFacts) { f.FlagOn = false }, ReasonFlagOff},
		"(f) budget below reserve":              {func(f *fallbackFacts) { f.Remaining = 19 * time.Second }, ReasonBudget},
		"(g) not configured":                    {func(f *fallbackFacts) { f.Configured = false }, ReasonNotConfigured},
		"RR-6 one switch":                       {func(f *fallbackFacts) { f.Switched = true }, ReasonOneSwitch},
		"RR-15 meta_only":                       {func(f *fallbackFacts) { f.Profile = ProfileMetaOnly }, ReasonProfileMetaOnly},
		"(a) not an upstream Meta outage (O-1)": {func(f *fallbackFacts) { f.Eligible = false }, ReasonFallbackNotEligible},
		"V21 synthetic data only (O-3)":         {func(f *fallbackFacts) { f.DataAllowed = false }, ReasonSyntheticOnly},
		"V21 no version re-check":               {func(f *fallbackFacts) { f.VersionCheck = false }, ReasonNoVersionCheck},
	}
	for name, tc := range cases {
		f := okFallback()
		tc.mod(&f)
		if got := fallbackDecision(f); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}

func okAlt() altFacts {
	return altFacts{ClaudeFailure: transport.ReasonUnavailable, Generator: ProviderMeta, FlagOn: true, Profile: ProfileFull,
		Configured: true, Remaining: time.Minute, Attempt: 15 * time.Second, DataAllowed: true}
}

// TestRR9AltVerifierDecision and RR-10: only a Claude availability failure
// on Meta output may reach DeepSeek as alternative verifier.
func TestRR9AltVerifierDecision(t *testing.T) {
	for _, r := range []transport.Reason{transport.ReasonTimeout, transport.ReasonRateLimited, transport.ReasonSpendCap,
		transport.ReasonAuth, transport.ReasonUnavailable} {
		f := okAlt()
		f.ClaudeFailure = r
		if got := altDecision(f); got != "" {
			t.Errorf("availability %s: %q", r, got)
		}
	}
	for _, r := range []transport.Reason{transport.ReasonRefused, transport.ReasonTruncated, transport.ReasonInvalidOutput,
		transport.ReasonModelMismatch, transport.ReasonTestDouble, transport.ReasonRejected, transport.ReasonCanceled} {
		f := okAlt()
		f.ClaudeFailure = r
		if got := altDecision(f); got != ReasonNotAvailability {
			t.Errorf("RR-10 %s: %q", r, got)
		}
	}
	cases := map[string]struct {
		mod  func(*altFacts)
		want string
	}{
		"(b) DeepSeek generated (self-verification)": {func(f *altFacts) { f.Generator = ProviderDeepSeek }, ReasonSelfVerification},
		"(b) human authored":                         {func(f *altFacts) { f.Generator = ProviderNone }, ReasonSelfVerification},
		"(e) flag off":                               {func(f *altFacts) { f.FlagOn = false }, ReasonFlagOff},
		"(f) budget":                                 {func(f *altFacts) { f.Remaining = time.Second }, ReasonBudget},
		"not configured":                             {func(f *altFacts) { f.Configured = false }, ReasonNotConfigured},
		"meta_only":                                  {func(f *altFacts) { f.Profile = ProfileMetaOnly }, ReasonProfileMetaOnly},
		"V21 synthetic data only (O-3)":              {func(f *altFacts) { f.DataAllowed = false }, ReasonSyntheticOnly},
	}
	for name, tc := range cases {
		f := okAlt()
		tc.mod(&f)
		if got := altDecision(f); got != tc.want {
			t.Errorf("%s: %q, want %q", name, got, tc.want)
		}
	}
}

// TestAgeBandMatrix is ADR-0003 §8.3.
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
		{ProviderAnthropic, Band18Plus, prod, true},
		{ProviderDeepSeek, Band18Plus, prod, true},
		{ProviderMeta, Band15To17, dev, true},
		{ProviderAnthropic, Band15To17, dev, true},
		{ProviderAnthropic, BandUnknown, dev, true},
		{ProviderDeepSeek, Band15To17, dev, false},
		{ProviderDeepSeek, BandUnknown, dev, false},
		{ProviderMeta, Band15To17, devNoMinors, false},
		{ProviderAnthropic, BandUnknown, devNoMinors, false},
		{ProviderMeta, Band15To17, prod, false},
		{ProviderAnthropic, Band15To17, prod, false},
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

// TestPurposes: every provider call needs its own consent, never bundled;
// DeepSeek and Claude also need the base AI consent; AT-6 carries no
// personal data.
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
		!eq(purposesFor(ProviderAnthropic, TaskSummaryDraft, false), []Purpose{PurposeAIProcessing, PurposeAnthropic}) ||
		!eq(purposesFor(ProviderDeepSeek, TaskReflection, false), []Purpose{PurposeAIProcessing, PurposeDeepSeek}) ||
		purposesFor(ProviderAnthropic, TaskReadingVerification, false) != nil {
		t.Fatal("purposes")
	}
	// A run of the memory feature or with memory items also needs
	// personal_memory, for every provider (§3.1).
	if !eq(purposesFor(ProviderMeta, TaskMemoryProposal, true), []Purpose{PurposeAIProcessing, PurposePersonalMemory}) ||
		!eq(purposesFor(ProviderDeepSeek, TaskListeningReply, true), []Purpose{PurposeAIProcessing, PurposeDeepSeek, PurposePersonalMemory}) ||
		!eq(purposesFor(ProviderAnthropic, TaskMemoryProposal, true), []Purpose{PurposeAIProcessing, PurposeAnthropic, PurposePersonalMemory}) ||
		purposesFor(ProviderAnthropic, TaskReadingVerification, true) != nil {
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
		if got := needsVerification(pol, clean, RoutePrimary); got != want[0] {
			t.Errorf("%s clean: %v", task, got)
		}
		if got := needsVerification(pol, flagged, RoutePrimary); got != want[1] {
			t.Errorf("%s flagged: %v", task, got)
		}
		// §17 O-6: a DeepSeek AT-1 reply is always verified; the fallback
		// output of the other tasks follows their own rule.
		fb := want[0] || task == TaskListeningReply
		if got := needsVerification(pol, clean, RouteFallback); got != fb {
			t.Errorf("%s clean fallback: %v", task, got)
		}
	}
}

// TestTaxonomy pins the PM-decided structure of AT-0..AT-6 (D-029).
func TestTaxonomy(t *testing.T) {
	type row struct {
		id         string
		generative bool
		modelFree  bool
		fallback   bool
		verify     Verification
		regenerate bool
	}
	want := map[Task]row{
		TaskCrisisSupport:       {"AT-0", false, true, false, VerifyNever, false},
		TaskListeningReply:      {"AT-1", true, false, true, VerifyWhenFlagged, true},
		TaskReflection:          {"AT-2", true, false, true, VerifyWhenFlagged, true},
		TaskMemoryProposal:      {"AT-3", true, false, true, VerifyAlways, false},
		TaskSummaryDraft:        {"AT-4", true, false, true, VerifyAlways, false},
		TaskOpeningLines:        {"AT-5", true, false, true, VerifyWhenFlagged, true},
		TaskReadingVerification: {"AT-6", false, false, false, VerifyAlways, false},
	}
	for task, w := range want {
		p, ok := PolicyFor(task)
		if !ok || p.ID != w.id || p.Generative != w.generative || p.ModelFree != w.modelFree || p.FallbackAllowed != w.fallback ||
			p.Verification != w.verify || p.Regenerate != w.regenerate {
			t.Errorf("%s: %+v", task, p)
		}
	}
	if len(Tasks()) != 7 {
		t.Fatal("seven tasks")
	}
	if p, _ := PolicyFor(TaskListeningReply); p.Budget != 60*time.Second || p.FirstContentWait != 20*time.Second ||
		p.FallbackReserve != 20*time.Second || p.VerifierAttempt != 15*time.Second || p.MaxRetryAfter != 3*time.Second ||
		!p.VerifyFallback || p.FallbackVerifierReserve != 15*time.Second || p.reserves() != 35*time.Second {
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
	weekdayPeak := time.Date(2026, 10, 26, 7, 0, 0, 0, time.UTC) // Monday 07:00 UTC
	offPeak := time.Date(2026, 10, 25, 7, 0, 0, 0, time.UTC)     // Sunday
	if c := estimateCost(ProviderAnthropic, false, "us", 1_000_000, 0, offPeak); c < 2.199 || c > 2.201 {
		t.Fatalf("anthropic us %f", c)
	}
	if c := estimateCost(ProviderDeepSeek, false, "", 0, 1_000_000, weekdayPeak); c != 1.2 {
		t.Fatalf("deepseek peak %f", c)
	}
	if c := estimateCost(ProviderDeepSeek, false, "", 0, 1_000_000, offPeak); c != 0.6 {
		t.Fatalf("deepseek off-peak %f", c)
	}
	if c := estimateCost(ProviderMetaAPI, false, "", 1_000_000, 0, offPeak); c != 1.25 {
		t.Fatalf("meta api input %f", c)
	}
	if c := estimateCost(ProviderMetaAPI, false, "", 0, 1_000_000, offPeak); c != 4.25 {
		t.Fatalf("meta api output %f", c)
	}
	if estimateCost(ProviderMeta, false, "", 1000, 1000, offPeak) != 0 || estimateCost(ProviderAnthropic, true, "us", 1000, 1000, offPeak) != 0 {
		t.Fatal("self-hosted meta and test doubles carry no API cost")
	}
}
