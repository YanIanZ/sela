package fixtures

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// LegacyContent preserves the v1 content assets that still have product value
// after the Next.js runtime is removed: the reviewed self-help card library,
// the source-cited help-destination checklists and the support-options
// directory. The data is embedded in the Go binary and validated at load
// time; no user data or model output is involved.
type LegacyContent struct {
	SelfHelpCards    []LegacySelfHelpCard    `json:"selfHelpCards"`
	HelpDestinations []LegacyHelpDestination `json:"helpDestinations"`
	SupportOptions   []LegacySupportOption   `json:"supportOptions"`
}

// LocalizedList is a bilingual list from the v1 content files.
type LocalizedList struct {
	ID []string `json:"id"`
	EN []string `json:"en"`
}

// LegacySelfHelpCard is one reviewed v1 self-help card. The model may only
// pick a card id; step text always comes from this data.
type LegacySelfHelpCard struct {
	ID           string          `json:"id"`
	Topics       []string        `json:"topics"`
	Title        Text            `json:"title"`
	Summary      Text            `json:"summary"`
	Steps        LocalizedList   `json:"steps"`
	Why          Text            `json:"why"`
	SeekHelpWhen Text            `json:"seekHelpWhen"`
	Cautions     Text            `json:"cautions"`
	Source       json.RawMessage `json:"source"`
	EvidenceNote string          `json:"evidenceNote"`
}

// LegacyHelpChecklistItem is one source-cited destination checklist line.
type LegacyHelpChecklistItem struct {
	Text       string   `json:"text"`
	SourceIDs  []string `json:"sourceIds"`
	Unverified bool     `json:"unverified"`
}

// LegacyHelpDestination is one v1 "what to bring / ask" pathway.
type LegacyHelpDestination struct {
	ID          string                    `json:"id"`
	Label       string                    `json:"label"`
	Description string                    `json:"description"`
	SpeakingTo  string                    `json:"speakingTo"`
	Vocative    string                    `json:"vocative"`
	Ask         string                    `json:"ask"`
	Checklist   []LegacyHelpChecklistItem `json:"checklist"`
	Sources     []json.RawMessage         `json:"sources"`
	Caveat      string                    `json:"caveat"`
	VerifiedOn  string                    `json:"verifiedOn"`
}

// LegacySupportOption is one entry of the v1 informational directory.
type LegacySupportOption struct {
	ID           string          `json:"id"`
	Category     string          `json:"category"`
	Name         Text            `json:"name"`
	Summary      Text            `json:"summary"`
	Phone        string          `json:"phone"`
	WhatsApp     string          `json:"whatsapp"`
	Website      string          `json:"website"`
	Hours        Text            `json:"hours"`
	Cost         Text            `json:"cost"`
	Limits       LocalizedList   `json:"limits"`
	Source       json.RawMessage `json:"source"`
	LastVerified string          `json:"lastVerified"`
}

// LoadLegacyContent parses and validates the embedded v1 content assets.
func LoadLegacyContent() (*LegacyContent, error) {
	c := &LegacyContent{}
	files := []struct {
		name string
		into any
	}{
		{"data/self_help_cards.v1.json", &c.SelfHelpCards},
		{"data/help_destinations.json", &c.HelpDestinations},
		{"data/support_options.json", &c.SupportOptions},
	}
	for _, f := range files {
		raw, err := dataFS.ReadFile(f.name)
		if err != nil {
			return nil, fmt.Errorf("fixtures: legacy %s: %w", f.name, err)
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(f.into); err != nil {
			return nil, fmt.Errorf("fixtures: legacy %s: %w", f.name, err)
		}
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *LegacyContent) validate() error {
	if len(c.SelfHelpCards) == 0 || len(c.HelpDestinations) == 0 || len(c.SupportOptions) == 0 {
		return fmt.Errorf("fixtures: legacy content is empty")
	}
	seen := map[string]bool{}
	check := func(kind, id string) error {
		if id == "" {
			return fmt.Errorf("fixtures: legacy %s has an empty id", kind)
		}
		if seen[kind+"/"+id] {
			return fmt.Errorf("fixtures: legacy %s id %q is duplicated", kind, id)
		}
		seen[kind+"/"+id] = true
		return nil
	}
	for _, card := range c.SelfHelpCards {
		if err := check("self-help", card.ID); err != nil {
			return err
		}
		if card.Title.ID == "" || card.Title.EN == "" || len(card.Steps.ID) == 0 || len(card.Steps.EN) == 0 || len(card.Source) == 0 {
			return fmt.Errorf("fixtures: legacy self-help %q is incomplete", card.ID)
		}
	}
	for _, d := range c.HelpDestinations {
		if err := check("destination", d.ID); err != nil {
			return err
		}
		if d.Label == "" || d.VerifiedOn == "" || len(d.Checklist) == 0 || len(d.Sources) == 0 {
			return fmt.Errorf("fixtures: legacy destination %q is incomplete", d.ID)
		}
		for _, item := range d.Checklist {
			if item.Text == "" || (len(item.SourceIDs) == 0 && !item.Unverified) {
				return fmt.Errorf("fixtures: legacy destination %q has an uncited item", d.ID)
			}
		}
	}
	for _, o := range c.SupportOptions {
		if err := check("support", o.ID); err != nil {
			return err
		}
		if o.Name.ID == "" || o.Name.EN == "" || o.LastVerified == "" || len(o.Source) == 0 {
			return fmt.Errorf("fixtures: legacy support option %q is incomplete", o.ID)
		}
	}
	return nil
}
