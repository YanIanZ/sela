package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/YanIanZ/sela/internal/ledger"
)

// mieCase is one synthetic Meaning Diff case (eval/mie/cases.jsonl). Only
// counts, rates and ids leave this file; case text never does.
type mieCase struct {
	CaseID           string   `json:"case_id"`
	FamilyID         string   `json:"family_id"`
	Lang             string   `json:"lang"`
	Split            string   `json:"split"`
	Synthetic        bool     `json:"synthetic"`
	Source           string   `json:"source"`
	Candidate        string   `json:"candidate"`
	ExpectedCritical bool     `json:"expected_critical"`
	ExpectedCodes    []string `json:"expected_codes"`
}

// loadMIECases reads and validates the dataset and returns its SHA-256.
func loadMIECases(root string) ([]mieCase, string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "eval", "mie", "cases.jsonl"))
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	var cases []mieCase
	ids := map[string]bool{}
	familySplit := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for line := 1; sc.Scan(); line++ {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var c mieCase
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			return nil, "", fmt.Errorf("cases.jsonl line %d: %w", line, err)
		}
		switch {
		case !c.Synthetic:
			return nil, "", fmt.Errorf("case %s: not marked synthetic", c.CaseID)
		case c.CaseID == "" || ids[c.CaseID]:
			return nil, "", fmt.Errorf("line %d: missing or duplicate case_id %q", line, c.CaseID)
		case c.Split != "dev" && c.Split != "tune" && c.Split != "heldout":
			return nil, "", fmt.Errorf("case %s: split %q", c.CaseID, c.Split)
		case c.Lang != "id" && c.Lang != "en" && c.Lang != "mixed":
			return nil, "", fmt.Errorf("case %s: lang %q", c.CaseID, c.Lang)
		case c.ExpectedCritical != (len(c.ExpectedCodes) > 0):
			return nil, "", fmt.Errorf("case %s: expected_critical and expected_codes disagree", c.CaseID)
		}
		if s, ok := familySplit[c.FamilyID]; ok && s != c.Split {
			return nil, "", fmt.Errorf("family %s spans splits %s and %s", c.FamilyID, s, c.Split)
		}
		familySplit[c.FamilyID] = c.Split
		ids[c.CaseID] = true
		cases = append(cases, c)
	}
	return cases, hex.EncodeToString(sum[:]), sc.Err()
}

// wilson returns the 95% Wilson score interval for k successes in n.
func wilson(k, n int) (float64, float64) {
	if n == 0 {
		return 0, 0
	}
	const z = 1.959963984540054
	p := float64(k) / float64(n)
	den := 1 + z*z/float64(n)
	centre := (p + z*z/(2*float64(n))) / den
	half := z * math.Sqrt(p*(1-p)/float64(n)+z*z/(4*float64(n)*float64(n))) / den
	return math.Max(0, centre-half), math.Min(1, centre+half)
}

type mieTally struct {
	n, tp, fp, tn, fn int
	codeHit, codeAll  map[string]int
	missed, flagged   []string
}

func (t *mieTally) add(c mieCase, got ledger.MeaningCheck) {
	if t.codeHit == nil {
		t.codeHit, t.codeAll = map[string]int{}, map[string]int{}
	}
	t.n++
	switch {
	case c.ExpectedCritical && got.Critical:
		t.tp++
	case c.ExpectedCritical:
		t.fn++
		t.missed = append(t.missed, c.CaseID)
	case got.Critical:
		t.fp++
		t.flagged = append(t.flagged, c.CaseID)
	default:
		t.tn++
	}
	for _, code := range c.ExpectedCodes {
		t.codeAll[code]++
		if slices.ContainsFunc(got.Differences, func(d ledger.MeaningDifference) bool {
			return d.Code == code && d.Severity == ledger.SeverityCritical
		}) {
			t.codeHit[code]++
		}
	}
}

func interval(k, n int) string {
	lo, hi := wilson(k, n)
	return fmt.Sprintf("%d/%d, 95%% CI %.2f-%.2f", k, n, lo, hi)
}

// runMIE scores the deterministic Meaning Diff (MIE config B1) on the
// synthetic cases, per split and per language. The held-out metrics are
// judged against the pre-registered thresholds; dev and tune are reported
// as info because the lexicons were tuned on them.
func runMIE(rep *Report, root string) error {
	cases, sha, err := loadMIECases(root)
	if err != nil {
		return err
	}
	rep.Versions["meaning_diff"] = ledger.MeaningVersion
	rep.Versions["mie_cases_sha256"] = sha
	bySplit := map[string]*mieTally{}
	byLang := map[string]*mieTally{}
	for _, c := range cases {
		got := ledger.CompareMeaning(c.Source, c.Candidate)
		for _, key := range []string{c.Split, "all"} {
			if bySplit[key] == nil {
				bySplit[key] = &mieTally{}
			}
			bySplit[key].add(c, got)
		}
		if c.Split == "heldout" {
			if byLang[c.Lang] == nil {
				byLang[c.Lang] = &mieTally{}
			}
			byLang[c.Lang].add(c, got)
		}
	}
	for _, split := range []string{"dev", "tune", "heldout", "all"} {
		t := bySplit[split]
		if t == nil {
			continue
		}
		rep.put(fmt.Sprintf("mie.%s.critical_recall", split), ratio(t.tp, t.tp+t.fn), t.tp+t.fn, interval(t.tp, t.tp+t.fn))
		rep.put(fmt.Sprintf("mie.%s.benign_false_flag", split), ratio(t.fp, t.fp+t.tn), t.fp+t.tn, interval(t.fp, t.fp+t.tn))
		rep.put(fmt.Sprintf("mie.%s.precision", split), ratio(t.tp, t.tp+t.fp), t.tp+t.fp, interval(t.tp, t.tp+t.fp))
		if split == "heldout" || split == "all" {
			codes := make([]string, 0, len(t.codeAll))
			for code := range t.codeAll {
				codes = append(codes, code)
			}
			sort.Strings(codes)
			for _, code := range codes {
				rep.put(fmt.Sprintf("mie.%s.code_recall.%s", split, code), ratio(t.codeHit[code], t.codeAll[code]), t.codeAll[code], interval(t.codeHit[code], t.codeAll[code]))
			}
		}
		if split == "heldout" {
			if len(t.missed) > 0 {
				rep.note("mie held-out missed critical cases: %v", t.missed)
			}
			if len(t.flagged) > 0 {
				rep.note("mie held-out benign cases flagged critical: %v", t.flagged)
			}
		}
	}
	for _, lang := range []string{"id", "en", "mixed"} {
		t := byLang[lang]
		if t == nil {
			continue
		}
		rep.put("mie.heldout.lang."+lang+".critical_recall", ratio(t.tp, t.tp+t.fn), t.tp+t.fn, interval(t.tp, t.tp+t.fn))
		rep.put("mie.heldout.lang."+lang+".benign_false_flag", ratio(t.fp, t.fp+t.tn), t.fp+t.tn, interval(t.fp, t.fp+t.tn))
	}
	rep.note("mie cases sha256 %s (%d synthetic cases; lexicons tuned on dev/tune only; held-out judged against thresholds version sela-eval-thresholds-2026.11.01-1)", sha, len(cases))
	return nil
}
