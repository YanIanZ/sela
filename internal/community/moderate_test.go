package community

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/YanIanZ/sela/internal/llm"
)

func TestRules(t *testing.T) {
	harmful := []string{"dasar cengeng, gitu aja nangis", "mati aja lu sana", "kill yourself", "you're pathetic", "hubungi aku 0812-3456-7890", "lebay banget lu"}
	ok := []string{"aku juga pernah merasa gitu, pelan-pelan ya", "semangat, kamu nggak sendirian", "aku capek banget hari ini", "kayaknya kamu perlu istirahat"}
	for _, s := range harmful {
		if len(Rules(s)) == 0 {
			t.Errorf("Rules(%q) found nothing", s)
		}
	}
	for _, s := range ok {
		if l := Rules(s); len(l) > 0 {
			t.Errorf("Rules(%q) = %v", s, l)
		}
	}
}

func TestParseModeration(t *testing.T) {
	if l, ok := parseModeration(`{"labels":["bullying","bullying"],"supportive":false}`); !ok || !slices.Equal(l, []string{"bullying"}) {
		t.Fatalf("labels = %v %v", l, ok)
	}
	if _, ok := parseModeration(`{"labels":["made_up"],"supportive":false}`); ok {
		t.Fatal("unknown label accepted")
	}
	if _, ok := parseModeration(`{"labels":[]}`); ok {
		t.Fatal("missing field accepted")
	}
	if l, ok := parseModeration("```json\n{\"labels\":[],\"supportive\":true}\n```"); !ok || len(l) != 0 {
		t.Fatal("fenced JSON refused")
	}
}

type fakeProvider struct {
	text string
	err  error
}

func (f fakeProvider) Info() llm.Info { return llm.Info{Mode: llm.ModeTestDouble} }
func (f fakeProvider) Complete(context.Context, llm.Request, func(string) error) (llm.Result, error) {
	return llm.Result{Text: f.text}, f.err
}
func (f fakeProvider) Health(context.Context) llm.Health { return llm.Health{} }

func TestMetaModeratorOutcomes(t *testing.T) {
	ctx := context.Background()
	if v := (MetaModerator{Provider: fakeProvider{text: `{"labels":[],"supportive":true}`}}).Moderate(ctx, "semangat ya", ""); v.Status != StatusPublished {
		t.Fatalf("clean = %+v", v)
	}
	if v := (MetaModerator{Provider: fakeProvider{text: `{"labels":["harassment"],"supportive":false}`}}).Moderate(ctx, "aku tau rumahmu", ""); v.Status != StatusHidden {
		t.Fatalf("harmful = %+v", v)
	}
	if v := (MetaModerator{Provider: fakeProvider{err: errors.New("down")}}).Moderate(ctx, "halo", ""); v.Status != StatusHeld {
		t.Fatalf("unavailable = %+v", v)
	}
	if v := (MetaModerator{Provider: fakeProvider{text: "not json"}}).Moderate(ctx, "halo", ""); v.Status != StatusHeld {
		t.Fatalf("unclear = %+v", v)
	}
	if v := (MetaModerator{}).Moderate(ctx, "dasar lemah", ""); v.Status != StatusHidden || v.Source != "rules" {
		t.Fatalf("rules first = %+v", v)
	}
}

func TestPrepareStopsCrisis(t *testing.T) {
	if _, _, err := Prepare("aku mau bunuh diri malam ini", 2000); !errors.Is(err, ErrCrisis) {
		t.Fatalf("crisis = %v", err)
	}
	if _, _, err := Prepare("", 2000); !errors.Is(err, ErrInvalid) {
		t.Fatal("empty accepted")
	}
}
