package main

import (
	"math"
	"testing"
)

func TestMIECasesValid(t *testing.T) {
	cases, sha, err := loadMIECases("../..")
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 120 || len(sha) != 64 {
		t.Fatalf("%d cases, sha %q", len(cases), sha)
	}
	splits := map[string]int{}
	benign, critical := 0, 0
	for _, c := range cases {
		splits[c.Split]++
		if c.ExpectedCritical {
			critical++
		} else {
			benign++
		}
	}
	if splits["heldout"] == 0 || splits["dev"] == 0 || splits["tune"] == 0 || benign == 0 || critical == 0 {
		t.Fatalf("splits %v, benign %d, critical %d", splits, benign, critical)
	}
}

func TestWilson(t *testing.T) {
	lo, hi := wilson(9, 10)
	if math.Abs(lo-0.5958) > 0.001 || math.Abs(hi-0.9821) > 0.001 {
		t.Fatalf("wilson(9,10) = %.4f-%.4f", lo, hi)
	}
	if lo, hi := wilson(0, 0); lo != 0 || hi != 0 {
		t.Fatal("empty interval")
	}
}
