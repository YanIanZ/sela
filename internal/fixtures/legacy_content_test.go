package fixtures

import "testing"

func TestLegacyContent(t *testing.T) {
	c, err := LoadLegacyContent()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(c.SelfHelpCards), 30; got != want {
		t.Errorf("self-help cards = %d, want %d", got, want)
	}
	if got, want := len(c.HelpDestinations), 4; got != want {
		t.Errorf("help destinations = %d, want %d", got, want)
	}
	if got, want := len(c.SupportOptions), 6; got != want {
		t.Errorf("support options = %d, want %d", got, want)
	}
	for _, card := range c.SelfHelpCards {
		if card.EvidenceNote == "" {
			t.Errorf("self-help %q has no evidence note", card.ID)
		}
	}
	for _, d := range c.HelpDestinations {
		for _, item := range d.Checklist {
			if item.Unverified && len(item.SourceIDs) > 0 {
				t.Errorf("destination %q item %q is both cited and unverified", d.ID, item.Text)
			}
		}
	}
}
