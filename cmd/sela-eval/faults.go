package main

import (
	"fmt"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/fixtures"
	"github.com/YanIanZ/sela/internal/journey"
	"github.com/YanIanZ/sela/internal/ledger"
	"github.com/YanIanZ/sela/internal/llm"
	"github.com/YanIanZ/sela/internal/llm/fakellm"
	"github.com/YanIanZ/sela/internal/store"
)

// faultScenario is the story every fault run drafts.
const faultScenario = "id-nadia-overwhelm"

// testDouble starts the in-process test double with one fault applied to
// every completion. It is reported as test_double, never as Meta.
func testDouble(fault string) (llm.Provider, func(), error) {
	h, err := fakellm.NewHandler(fakellm.Config{Fault: fakellm.Fault(fault), SlowDelay: 3 * time.Second})
	if err != nil {
		return nil, nil, err
	}
	srv := httptest.NewServer(h)
	p, err := llm.New(llm.Config{Mode: llm.ModeTestDouble, BaseURL: srv.URL, ServedModel: fakellm.ServedModel,
		AttemptTimeout: time.Second, MaxRetries: 1})
	if err != nil {
		srv.Close()
		return nil, nil, err
	}
	return p, srv.Close, nil
}

var digitsRE = regexp.MustCompile(`\d+`)

// faultCheck decides whether a run under a fault ended safely: either the
// operation failed with the expected honest code, or it completed and no
// committed item carries the injected defect.
type faultCheck struct {
	wantCode string
	clean    func(it journey.Item, segs map[string]string) bool
}

func grounded(it journey.Item, segs map[string]string) bool {
	txt, ok := segs[it.SegmentID]
	return ok && it.Start >= 0 && it.End <= len(txt) && it.Start <= it.End && txt[it.Start:it.End] == it.Quote
}

var faultChecks = map[fakellm.Fault]faultCheck{
	fakellm.FaultNone:             {clean: grounded},
	fakellm.FaultMalformedJSON:    {wantCode: "ai_invalid_output"},
	fakellm.FaultFabricatedSource: {clean: grounded},
	fakellm.FaultNegationFlip: {clean: func(it journey.Item, segs map[string]string) bool {
		w := ledger.Analyze(ledger.Segment{ID: "w", Text: it.Wording}, 0, len(it.Wording))
		return grounded(it, segs) && w.Negated == it.Qualifiers.Negated
	}},
	fakellm.FaultDiagnosis: {clean: func(it journey.Item, segs map[string]string) bool {
		return grounded(it, segs) && (!ledger.ContainsDiagnosisLabel(it.Wording) || ledger.ContainsDiagnosisLabel(it.Quote))
	}},
	fakellm.FaultInventedNumber: {clean: func(it journey.Item, segs map[string]string) bool {
		have := map[string]bool{}
		for _, d := range digitsRE.FindAllString(it.Quote, -1) {
			have[d] = true
		}
		for _, d := range digitsRE.FindAllString(it.Wording, -1) {
			if !have[d] {
				return false
			}
		}
		return grounded(it, segs)
	}},
	fakellm.FaultPromptInjectionEcho: {clean: func(it journey.Item, segs map[string]string) bool {
		return grounded(it, segs) && !strings.Contains(strings.ToUpper(it.Wording), "IGNORE PREVIOUS")
	}},
	fakellm.FaultWrongModel:      {wantCode: "ai_model_mismatch"},
	fakellm.FaultHTTP503:         {wantCode: journey.CodeAIUnavailable},
	fakellm.FaultHTTP429:         {wantCode: "ai_busy"},
	fakellm.FaultHTTP401:         {wantCode: "ai_auth"},
	fakellm.FaultSlow:            {wantCode: "ai_timeout"},
	fakellm.FaultStall:           {wantCode: "ai_timeout"},
	fakellm.FaultOversize:        {wantCode: "ai_invalid_output"},
	fakellm.FaultTruncatedStream: {wantCode: "ai_invalid_output"},
}

// runFaults drafts the same story once per fault and counts how many runs
// ended safely (meaning_lock.fault_catch_rate). The no-fault run is a
// control and is not counted.
func runFaults(rep *Report, cat *fixtures.Catalog) error {
	faults := fakellm.Faults()
	sort.Slice(faults, func(i, j int) bool { return faults[i] < faults[j] })
	caught, total := 0, 0
	var missed []string
	for _, f := range faults {
		check, ok := faultChecks[f]
		if !ok {
			missed = append(missed, string(f)+": no check defined")
			total++
			continue
		}
		p, cleanup, err := testDouble(string(f))
		if err != nil {
			return err
		}
		svc := journey.New(journey.Config{OpTimeout: 30 * time.Second, MaxOpsPerWindow: 100}, store.NewMemory(store.Options{}), cat, p, llm.NewAdmission(1))
		v, out, err := runDraft(svc, cat, faultScenario, "id")
		cleanup()
		if err != nil {
			return fmt.Errorf("fault %s: %w", f, err)
		}
		pass, why := judgeFault(check, v, out)
		if f == fakellm.FaultNone {
			if !pass {
				rep.note("control run without fault failed: %s", why)
			}
			continue
		}
		total++
		if pass {
			caught++
		} else {
			missed = append(missed, string(f)+": "+why)
		}
	}
	rep.put("meaning_lock.fault_catch_rate", ratio(caught, total), total, "TEST DOUBLE fault injection: each fault must end in an honest error or a ledger without the defect")
	if len(missed) > 0 {
		rep.note("faults not caught: %s", strings.Join(missed, "; "))
	}
	return nil
}

func judgeFault(c faultCheck, v *journey.SessionView, out opOutcome) (bool, string) {
	if c.wantCode != "" {
		if strings.HasPrefix(out.errCode, c.wantCode) {
			return true, ""
		}
		return false, fmt.Sprintf("want error %s, got %q", c.wantCode, out.errCode)
	}
	if out.errCode != "" {
		// An honest failure is still safe for content faults.
		return true, ""
	}
	if v == nil || v.Ledger == nil || v.Story == nil {
		return false, "no ledger after completed operation"
	}
	segs := map[string]string{}
	for _, s := range v.Story.Segments {
		segs[s.ID] = s.Text
	}
	for _, it := range v.Ledger.Items {
		if !c.clean(it, segs) {
			return false, fmt.Sprintf("defect committed in item %s", it.ID)
		}
	}
	return true, ""
}
