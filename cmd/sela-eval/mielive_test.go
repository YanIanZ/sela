package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/ai"
)

func liveTestCase(critical bool) mieCase {
	c := mieCase{CaseID: "X1-1", FamilyID: "X1", Lang: "id", Split: "heldout", Synthetic: true,
		Source:    "Kadang aku ingin sendiri, tapi aku tetap suka bersama teman.",
		Candidate: "Kamu sesekali butuh waktu sendiri."}
	if critical {
		c.Candidate, c.ExpectedCritical, c.ExpectedCodes = "Kamu selalu ingin sendiri.", true, []string{"frequency"}
	}
	return c
}

// T's post-rejection request must be the p1 request (graph exclusion), and
// the no-graph p2 request must differ from it.
func TestMIELiveTRequestIsP1Request(t *testing.T) {
	pol, _ := ai.PolicyFor(ai.TaskReflection)
	lr := &liveRunner{pol: pol}
	c := liveTestCase(true)
	p1, err := lr.genSpans(c, "gen1")
	if err != nil {
		t.Fatal(err)
	}
	p1b, _ := lr.genSpans(c, "gen1")
	p2, err := lr.genSpans(c, "gen2")
	if err != nil {
		t.Fatal(err)
	}
	_, _, h1, err := requestHash(c, p1)
	if err != nil {
		t.Fatal(err)
	}
	_, _, h1b, _ := requestHash(c, p1b)
	sys2, user2, h2, _ := requestHash(c, p2)
	if h1 != h1b || h1 == h2 {
		t.Fatalf("hashes p1 %s p1' %s p2 %s", h1, h1b, h2)
	}
	if len(p2.Spans) != 3 || p2.Spans[1].Role != "earlier_reply" || !strings.Contains(user2, correctionText["id"]) || sys2 == "" {
		t.Fatalf("p2 payload %+v", p2.Spans)
	}
}

func TestMIELiveReleased(t *testing.T) {
	yes, no := true, false
	ok := &liveRecord{Outcome: "valid", B1Pass: &yes}
	flagged := &liveRecord{Outcome: "valid", B1Pass: &no}
	invalid := &liveRecord{Outcome: "invalid"}
	rel := &liveRecord{Outcome: "release"}
	wh := &liveRecord{Outcome: "withhold"}
	failed := &liveRecord{Outcome: "failed"}
	cases := []struct {
		arm  string
		g, v *liveRecord
		want bool
	}{
		{"B0", ok, nil, true}, {"B0", flagged, nil, true}, {"B0", invalid, rel, false}, {"B0", nil, nil, false},
		{"B1", ok, nil, true}, {"B1", flagged, rel, false},
		{"B2", ok, rel, true}, {"B2", ok, wh, false}, {"B2", ok, failed, false}, {"B2", ok, nil, false}, {"B2", flagged, rel, false},
		{"T", ok, rel, true},
	}
	for i, c := range cases {
		if got := released(c.arm, c.g, c.v); got != c.want {
			t.Errorf("case %d %s: got %v", i, c.arm, got)
		}
	}
}

func TestMIELiveBudgetCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	b, err := openBudget(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.reserve(2); err != nil {
		t.Fatal(err)
	}
	if err := b.reserve(2); err != nil {
		t.Fatal(err)
	}
	if err := b.reserve(1); err != errBudget {
		t.Fatalf("over-reservation: %v", err)
	}
	b.settle(2, 1, "gen1", "a")
	b.settle(2, 2, "ver1", "a")
	again, err := openBudget(path, 4)
	if err != nil || again.total() != 3 {
		t.Fatalf("ledger reload: %d %v", again.total(), err)
	}
	if err := again.reserve(2); err != errBudget {
		t.Fatalf("cap across runs: %v", err)
	}
}

func TestMIELiveReadKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(path, []byte("# x\nMETA_LLAMA_BASE_URL=https://example\nexport META_LLAMA_API_KEY=\"k-123\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	k, err := readKey(path)
	if err != nil || k != "k-123" {
		t.Fatalf("got %q %v", k, err)
	}
	if err := os.WriteFile(path, []byte("OTHER=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readKey(path); err == nil || strings.Contains(err.Error(), "OTHER") {
		t.Fatalf("missing key error %v", err)
	}
}

func TestMIELiveThresholdsRegistered(t *testing.T) {
	th, _, err := liveThresholds("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"mie_live.heldout.p1.B2.uir", "mie_live.heldout.p2.T.correction_leakage", "mie_live.heldout.gold.B2.false_acceptance_critical"} {
		if _, ok := th.Metrics[k]; !ok {
			t.Errorf("%s not registered", k)
		}
	}
	// The normal sela-eval harness must not pick up the live MIE metrics.
	raw, err := os.ReadFile("../../eval/thresholds.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), mieDatasetSHA) {
		t.Fatal("registered dataset hash missing")
	}
	var all thresholds
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	all.merge()
	for k := range all.Metrics {
		if strings.HasPrefix(k, "mie_live.") {
			t.Fatalf("offline harness merged %s", k)
		}
	}
}

// scoreGroup on a hand-built group: B0 releases an unsupported output that
// B1 blocks; one faithful output is withheld by the verifier; T avoids the
// leak that the no-graph p2 generation shows.
func TestMIELiveScoreGroup(t *testing.T) {
	yes, no := true, false
	leak := true
	benign := liveTestCase(false)
	crit := liveTestCase(true)
	crit.CaseID, crit.FamilyID = "X2-1", "X2"
	cs := []*liveCase{
		{c: benign, g1: &liveRecord{Outcome: "valid", B1Pass: &yes}, v1: &liveRecord{Outcome: "withhold"},
			l1: &liveLabel{Label: "faithful"}, vg: &liveRecord{Outcome: "release"}, goldB1: true},
		{c: crit, g1: &liveRecord{Outcome: "valid", B1Pass: &no}, v1: &liveRecord{Outcome: "release"},
			l1: &liveLabel{Label: "unsupported"}, vg: &liveRecord{Outcome: "withhold"}, goldB1: false,
			g2: &liveRecord{Outcome: "valid", B1Pass: &yes}, v2: &liveRecord{Outcome: "release"},
			l2: &liveLabel{Label: "unsupported", Leak: &leak}},
	}
	rep := &Report{}
	scoreGroup(rep, "t", cs, true)
	want := map[string]float64{
		"t.p1.B0.uir": 0.5, "t.p1.B1.uir": 0, "t.p1.B2.uir": 0,
		"t.p1.B2.unnecessary_abstention": 1, "t.p1.B1.unnecessary_abstention": 0,
		"t.p1.uir_benefit_b2_vs_b0":  0.5,
		"t.p2.B0.correction_leakage": 1, "t.p2.B2.correction_leakage": 1, "t.p2.T.correction_leakage": 0,
		"t.gold.B0.false_acceptance_critical": 1, "t.gold.B2.false_acceptance_critical": 0,
		"t.gold.B2.benign_unnecessary_abstention": 0,
	}
	for k, v := range want {
		m, ok := rep.measured[k]
		if !ok || m.Value != v {
			t.Errorf("%s = %v (measured %v), want %v", k, m.Value, ok, v)
		}
	}
	if m := rep.measured["t.p1.B0.uir"]; m.N != 2 {
		t.Errorf("uir denominator %d, want 2 released", m.N)
	}
}

func TestMIELiveRawOutsideRepo(t *testing.T) {
	err := runMIELiveCalls(mieLiveOpts{root: "../..", raw: "../../eval/results/raw", envFile: "/nonexistent", maxCalls: 1, concurrency: 1})
	if err == nil || !strings.Contains(err.Error(), "outside the repository") {
		t.Fatalf("raw inside repo accepted: %v", err)
	}
}
