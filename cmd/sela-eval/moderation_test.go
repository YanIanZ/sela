package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/community"
	"github.com/YanIanZ/sela/internal/llm"
)

func TestModerationCasesLoad(t *testing.T) {
	cases, sha, err := loadModerationCases(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 150 || len(sha) != 64 {
		t.Fatalf("cases = %d, sha = %q", len(cases), sha)
	}
	seen := map[string]bool{}
	for _, c := range cases {
		seen[c.Stratum] = true
	}
	for s := range modStrata {
		if !seen[s] {
			t.Errorf("stratum %s has no case", s)
		}
	}
}

func TestModerationCasesRejectFamilyAcrossSplits(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "eval", "moderation")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines := `{"case_id":"X01-1","family_id":"X01","stratum":"banter","lang":"id","register":"slang","split":"dev","synthetic":true,"kind":"post","text":"a","expected":"publish"}
{"case_id":"X01-2","family_id":"X01","stratum":"banter","lang":"id","register":"slang","split":"heldout","synthetic":true,"kind":"post","text":"b","expected":"publish"}
`
	if err := os.WriteFile(filepath.Join(dir, "cases.jsonl"), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadModerationCases(root); err == nil || !strings.Contains(err.Error(), "spans splits") {
		t.Fatalf("err = %v", err)
	}
}

type modFake struct {
	text string
	err  error
}

func (f modFake) Info() llm.Info { return llm.Info{Mode: llm.ModeTestDouble} }
func (f modFake) Complete(context.Context, llm.Request, func(string) error) (llm.Result, error) {
	return llm.Result{Text: f.text}, f.err
}
func (f modFake) Health(context.Context) llm.Health { return llm.Health{} }

func TestClassifyModeration(t *testing.T) {
	ctx := context.Background()
	crisis := modCase{CaseID: "C-1", Kind: "post", Text: "aku mau bunuh diri malam ini", Expected: "crisis"}
	if o := classifyModeration(ctx, crisis, nil); o.Got != "crisis" || o.Stage != "safety" || !o.Correct {
		t.Fatalf("crisis = %+v", o)
	}
	bully := modCase{CaseID: "B-1", Kind: "reply", Text: "dasar cengeng, gitu aja nangis", Expected: "hide"}
	if o := classifyModeration(ctx, bully, nil); o.Got != "hide" || o.Stage != "rules" || !o.Correct {
		t.Fatalf("rules = %+v", o)
	}
	benign := modCase{CaseID: "P-1", Kind: "post", Text: "semangat ya", Expected: "publish"}
	if o := classifyModeration(ctx, benign, nil); o.Got != "publish" || o.Stage != "none" || !o.Correct {
		t.Fatalf("rules pass = %+v", o)
	}
	hidden := community.MetaModerator{Provider: modFake{text: `{"labels":["bullying"],"supportive":false}`}}
	if o := classifyModeration(ctx, benign, hidden); o.Got != "hide" || o.Stage != "meta" || o.errorClass() != "false_positive" {
		t.Fatalf("meta fp = %+v", o)
	}
	down := community.MetaModerator{Provider: modFake{err: errors.New("down")}}
	if o := classifyModeration(ctx, benign, down); o.Got != "hold" || o.errorClass() != "false_positive" {
		t.Fatalf("held = %+v", o)
	}
	harm := modCase{CaseID: "H-1", Kind: "reply", Text: "kirim foto dong", Expected: "hide"}
	clean := community.MetaModerator{Provider: modFake{text: `{"labels":[],"supportive":false}`}}
	if o := classifyModeration(ctx, harm, clean); o.Got != "publish" || o.errorClass() != "false_negative" {
		t.Fatalf("fn = %+v", o)
	}
}

type countRT struct{ n int }

func (c *countRT) RoundTrip(*http.Request) (*http.Response, error) {
	c.n++
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
}

func TestCallBudgetRefusesBeyondMax(t *testing.T) {
	next := &countRT{}
	b := &modCallBudget{max: 2, next: next}
	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest(http.MethodPost, "https://api.meta.ai/v1/chat/completions", nil)
		resp, err := b.RoundTrip(req)
		if i < 2 && err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if i >= 2 && !errors.Is(err, errCallBudget) {
			t.Fatalf("call %d not refused: %v", i, err)
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	if next.n != 2 || b.Used() != 2 {
		t.Fatalf("sent %d, used %d", next.n, b.Used())
	}
}
