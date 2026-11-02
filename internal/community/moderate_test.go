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

// TestRulesFraming pins S-036's rule fixes from both sides: a quoted,
// reported, negated, self-directed or thing-directed insult is left to Meta,
// while the same word aimed at the reader is still hidden by the rules.
func TestRulesFraming(t *testing.T) {
	benign := []string{
		// quoted or reported to ask for support
		"Dia bilang aku goblok di depan semua orang. Aku harus gimana?",
		"temenku ngatain aku tolol cuma gara-gara salah ngomong",
		"my boss called me an idiot in front of everyone, how do I deal with this?",
		"dad always says 'man up' whenever I open up. it hurts",
		"my ex used to say 'nobody cares about you'. I still hear it",
		"ada yang komen 'dasar lebay' di postinganku, aku jadi takut cerita lagi",
		// self-description and reclaimed words
		"goblok banget aku, lupa bawa laptop pas presentasi",
		"aku drama queen dan bangga, perasaanku valid kok",
		"proud drama queen here, my feelings are big and that's ok",
		"I feel like such a loser today",
		// inclusive banter
		"bego banget kita berdua, sama-sama lupa deadline",
		// aimed at a thing
		"goblok banget sistem antrian rumah sakit ini, nunggu 5 jam",
		// negated or reported speech, talk about words
		"I'm not saying you're stupid, I just worry about you",
		"don't let anyone tell you you're worthless",
		"bukan salahmu dia ngatain kamu goblok. itu cerminan dia, bukan kamu",
		"It's not your fault he called you an idiot.",
		"you're not a loser for asking for help",
		"the article said phrases like 'man up' make boys hide their feelings",
		"Artikel itu menyebut kata-kata seperti 'dasar lemah' bisa melukai lama.",
	}
	harmful := []string{
		"goblok lu", "lu goblok banget", "kamu tuh tolol ya", "dasar lebay", "loser", "kamu loser",
		"aku bilang kamu goblok", "gue bilang lu tolol, ngerti?", "I'm saying you're stupid",
		"nggak usah lebay banget lu", "you're an idiot and I'm not sorry", "drama queen lu",
		"man up", "stop whining, nobody cares", "you're pathetic",
		// a framed first sentence does not cover a direct insult in the next
		"dia bilang aku goblok. kamu juga goblok",
		// telling someone to die stays hidden even when quoted or reported
		"someone commented 'kys' on my post", "di film itu tokohnya bilang 'kill yourself'", "mati aja lu sana",
	}
	for _, s := range benign {
		if l := Rules(s); len(l) > 0 {
			t.Errorf("benign Rules(%q) = %v", s, l)
		}
	}
	for _, s := range harmful {
		if l := Rules(s); !slices.Contains(l, "bullying") {
			t.Errorf("harmful Rules(%q) = %v", s, l)
		}
	}
}

func TestParseModeration(t *testing.T) {
	if l, _, ok := parseModeration(`{"labels":["bullying","bullying"],"supportive":false}`); !ok || !slices.Equal(l, []string{"bullying"}) {
		t.Fatalf("labels = %v %v", l, ok)
	}
	if _, _, ok := parseModeration(`{"labels":["made_up"],"supportive":false}`); ok {
		t.Fatal("unknown label accepted")
	}
	if _, _, ok := parseModeration(`{"labels":[]}`); ok {
		t.Fatal("missing field accepted")
	}
	if l, sr, ok := parseModeration(`{"labels":["self_risk"],"supportive":false}`); !ok || !sr || len(l) != 0 {
		t.Fatalf("self_risk = %v %v %v", l, sr, ok)
	}
	if l, sr, ok := parseModeration(`{"labels":["bullying"],"supportive":false}`); !ok || sr || !slices.Equal(l, []string{"bullying"}) {
		t.Fatalf("bullying read as self_risk: %v %v %v", l, sr, ok)
	}
	if l, _, ok := parseModeration("```json\n{\"labels\":[],\"supportive\":true}\n```"); !ok || len(l) != 0 {
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

func TestMetaModeratorSelfRisk(t *testing.T) {
	m := MetaModerator{Provider: fakeProvider{text: `{"labels":["self_risk"],"supportive":false}`}}
	v := m.Moderate(context.Background(), "aku udah di tempat tinggi", "")
	if !v.SelfRisk || v.Status != StatusHeld || !slices.Contains(v.Labels, SelfRiskLabel) || v.Source != "meta" {
		t.Fatalf("self_risk verdict = %+v", v)
	}
	m = MetaModerator{Provider: fakeProvider{text: `{"labels":[],"supportive":true}`}}
	if v := m.Moderate(context.Background(), "semangat ya, kamu nggak sendiri", ""); v.SelfRisk || v.Status != StatusPublished {
		t.Fatalf("supportive verdict = %+v", v)
	}
}
