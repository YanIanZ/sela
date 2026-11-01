// Package fixtures loads Sela's versioned synthetic catalogue: seeded
// scenarios, fictional professionals and slots, fixture-authored follow-up
// plans, check-in options, recipient reasons, clarification templates,
// practical next steps and verified help resources.
//
// Every person, professional, slot and plan here is fictional (SPEC C3, V1).
// Help resources are real services copied only from v1 entries marked
// verified_official, with their sources and access dates (SPEC V3).
// The catalogue is embedded and validated at start-up; an invalid catalogue
// fails fast instead of serving a partially translated or inconsistent demo.
package fixtures

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Version identifies the catalogue contents. Bump on any data change.
const Version = "sela-fixtures-2026.11.01-1"

//go:embed data/*.json
var dataFS embed.FS

// Text is a localized string. Both languages are mandatory (SPEC V13).
type Text struct {
	ID string `json:"id"`
	EN string `json:"en"`
}

// In returns the text in lang ("id" or "en"); unknown languages get Indonesian.
func (t Text) In(lang string) string {
	if lang == "en" {
		return t.EN
	}
	return t.ID
}

// Fact is a gold annotation of one expected ledger claim for evaluation.
type Fact struct {
	Quote     string `json:"quote"`
	Kind      string `json:"kind"`
	Negated   bool   `json:"negated"`
	Subject   string `json:"subject"`
	Time      string `json:"time"`
	Certainty string `json:"certainty"`
	Important bool   `json:"important"`
}

// Forbidden is a gold annotation of a claim a correct draft must NOT make
// (e.g. presenting a negated statement as positive).
type Forbidden struct {
	Pattern string `json:"pattern"` // case-insensitive regular expression over claim wording
	Reason  string `json:"reason"`
}

// Scenario is a seeded, fictional adult story.
type Scenario struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Kind     string `json:"kind"`     // "seeded" | "crisis"
	Language string `json:"language"` // language of the story text: "id" | "en" | "mixed"
	Title    Text   `json:"title"`
	Persona  Text   `json:"persona"`
	Summary  Text   `json:"summary"`
	// ContentWarning is shown before the story when set.
	ContentWarning *Text `json:"contentWarning,omitempty"`
	// Story is the persona's own words in the original language. It is never
	// translated: Sela shows original quotes beside any wording.
	Story     string      `json:"story"`
	Facts     []Fact      `json:"facts"`
	Forbidden []Forbidden `json:"forbidden"`
	// ExpectedSafety is the deterministic route this fixture must produce.
	ExpectedSafety string `json:"expectedSafety"`
}

// Slot is a fixed fictional appointment slot on the demo clock.
type Slot struct {
	ID        string `json:"id"`
	Start     string `json:"start"` // RFC 3339 on the fixed demo clock
	Format    string `json:"format"`
	Available bool   `json:"available"`
}

// Professional is a fictional professional or service.
type Professional struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Role         Text     `json:"role"`
	Category     string   `json:"category"` // psychologist | counselor | psychiatrist | community
	Languages    []string `json:"languages"`
	Formats      []string `json:"formats"`
	CostTier     string   `json:"costTier"` // free | low | standard
	CostLabel    Text     `json:"costLabel"`
	Availability []string `json:"availability"`
	Area         Text     `json:"area"`
	Bio          Text     `json:"bio"`
	Slots        []Slot   `json:"slots"`
	Plans        []string `json:"plans"`
}

// Plan is a fixture-authored follow-up plan attributed to a fictional
// professional. AI never writes or edits plans (SPEC V7).
type Plan struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Title       Text   `json:"title"`
	Steps       []Text `json:"steps"`
	Attribution Text   `json:"attribution"`
}

// Option is a generic localized choice (check-ins, recipient reasons).
type Option struct {
	ID    string `json:"id"`
	Label Text   `json:"label"`
	Kind  string `json:"kind,omitempty"`
}

// NextStep is a practical step that does not require a handoff.
type NextStep struct {
	ID          string   `json:"id"`
	Title       Text     `json:"title"`
	Description Text     `json:"description"`
	Action      string   `json:"action"`   // practice | save | trusted_person | help | pause
	Barriers    []string `json:"barriers"` // barriers this step addresses
}

// Channel is one way to reach a help resource.
type Channel struct {
	Kind  string `json:"kind"` // phone | whatsapp | web
	Value string `json:"value"`
}

// Source is a citation for a help-resource fact.
type Source struct {
	Label      string `json:"label"`
	URL        string `json:"url"`
	AccessedOn string `json:"accessedOn"`
}

// HelpResource is a real support service. Only verified entries are allowed.
type HelpResource struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Region             string    `json:"region"` // ISO 3166-1 alpha-2; "ID" = Indonesia
	RegionName         Text      `json:"regionName"`
	Channels           []Channel `json:"channels"`
	Hours              Text      `json:"hours"`
	Cost               Text      `json:"cost"`
	Description        Text      `json:"description"`
	Sources            []Source  `json:"sources"`
	VerificationStatus string    `json:"verificationStatus"`
	LastVerified       string    `json:"lastVerified"`
	VerifiedBy         string    `json:"verifiedBy"`
	ReverifyBy         string    `json:"reverifyBy"`
	// Urgent services lead the quick-help sheet (D-077).
	Urgent bool `json:"urgent,omitempty"`
}

// Catalog is the whole validated catalogue.
type Catalog struct {
	Version        string                  `json:"version"`
	DemoClock      string                  `json:"demoClock"`
	Scenarios      []Scenario              `json:"scenarios"`
	Professionals  []Professional          `json:"professionals"`
	Plans          []Plan                  `json:"plans"`
	CheckIns       []Option                `json:"checkIns"`
	ClarifyReasons []Option                `json:"clarifyReasons"`
	DeclineReasons []Option                `json:"declineReasons"`
	NextSteps      []NextStep              `json:"nextSteps"`
	Clarifications map[string]Text         `json:"clarifications"`
	Help           []HelpResource          `json:"help"`
	HelpNotice     Text                    `json:"helpNotice"`
	Policies       map[string]PolicyNotice `json:"policies"`

	scenarios     map[string]*Scenario
	professionals map[string]*Professional
	plans         map[string]*Plan
	slots         map[string]slotRef
	options       map[string]map[string]*Option
}

// PolicyNotice is the versioned text a consent decision refers to.
type PolicyNotice struct {
	Version string `json:"version"`
	Title   Text   `json:"title"`
	Body    []Text `json:"body"`
}

type slotRef struct {
	prof *Professional
	slot *Slot
}

// Load parses and validates the embedded catalogue.
func Load() (*Catalog, error) {
	c := &Catalog{}
	files := []struct {
		name string
		into any
	}{
		{"data/scenarios.json", &c.Scenarios},
		{"data/professionals.json", &c.Professionals},
		{"data/plans.json", &c.Plans},
		{"data/options.json", c},
		{"data/help.json", c},
		{"data/policies.json", &c.Policies},
	}
	for _, f := range files {
		raw, err := dataFS.ReadFile(f.name)
		if err != nil {
			return nil, err
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(f.into); err != nil {
			return nil, fmt.Errorf("fixtures: %s: %w", f.name, err)
		}
	}
	c.Version = Version
	if err := c.index(); err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// MustLoad panics on an invalid catalogue (start-up only).
func MustLoad() *Catalog {
	c, err := Load()
	if err != nil {
		panic(err)
	}
	return c
}

func (c *Catalog) index() error {
	c.scenarios = map[string]*Scenario{}
	c.professionals = map[string]*Professional{}
	c.plans = map[string]*Plan{}
	c.slots = map[string]slotRef{}
	c.options = map[string]map[string]*Option{}
	for i := range c.Scenarios {
		s := &c.Scenarios[i]
		if c.scenarios[s.ID] != nil {
			return fmt.Errorf("fixtures: duplicate scenario %q", s.ID)
		}
		c.scenarios[s.ID] = s
	}
	for i := range c.Professionals {
		p := &c.Professionals[i]
		if c.professionals[p.ID] != nil {
			return fmt.Errorf("fixtures: duplicate professional %q", p.ID)
		}
		c.professionals[p.ID] = p
		for j := range p.Slots {
			s := &p.Slots[j]
			if _, dup := c.slots[s.ID]; dup {
				return fmt.Errorf("fixtures: duplicate slot %q", s.ID)
			}
			c.slots[s.ID] = slotRef{prof: p, slot: s}
		}
	}
	for i := range c.Plans {
		p := &c.Plans[i]
		if c.plans[p.ID] != nil {
			return fmt.Errorf("fixtures: duplicate plan %q", p.ID)
		}
		c.plans[p.ID] = p
	}
	for name, list := range map[string][]Option{"checkIn": c.CheckIns, "clarify": c.ClarifyReasons, "decline": c.DeclineReasons} {
		m := map[string]*Option{}
		for i := range list {
			o := &list[i]
			if m[o.ID] != nil {
				return fmt.Errorf("fixtures: duplicate %s option %q", name, o.ID)
			}
			m[o.ID] = o
		}
		c.options[name] = m
	}
	return nil
}

var (
	idPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)
	datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

func checkText(where string, t Text) error {
	if strings.TrimSpace(t.ID) == "" || strings.TrimSpace(t.EN) == "" {
		return fmt.Errorf("fixtures: %s: both id and en text are required", where)
	}
	return nil
}

func (c *Catalog) validate() error {
	if !strings.HasPrefix(c.DemoClock, "20") {
		return fmt.Errorf("fixtures: demoClock missing")
	}
	var errs []string
	add := func(err error) {
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	allowed := func(v string, set ...string) bool {
		for _, s := range set {
			if v == s {
				return true
			}
		}
		return false
	}
	for _, s := range c.Scenarios {
		if !idPattern.MatchString(s.ID) {
			add(fmt.Errorf("fixtures: bad scenario id %q", s.ID))
		}
		add(checkText("scenario "+s.ID+" title", s.Title))
		add(checkText("scenario "+s.ID+" persona", s.Persona))
		add(checkText("scenario "+s.ID+" summary", s.Summary))
		if s.ContentWarning != nil {
			add(checkText("scenario "+s.ID+" warning", *s.ContentWarning))
		}
		if !allowed(s.Kind, "seeded", "crisis") || !allowed(s.Language, "id", "en", "mixed") || !allowed(s.ExpectedSafety, "none", "elevated", "crisis") {
			add(fmt.Errorf("fixtures: scenario %s: bad kind/language/expectedSafety", s.ID))
		}
		if strings.TrimSpace(s.Story) == "" || len(s.Story) > 4000 {
			add(fmt.Errorf("fixtures: scenario %s: story empty or too long", s.ID))
		}
		for _, f := range s.Facts {
			if !strings.Contains(strings.ToLower(s.Story), strings.ToLower(f.Quote)) {
				add(fmt.Errorf("fixtures: scenario %s: fact quote %q not in story", s.ID, f.Quote))
			}
		}
		for _, f := range s.Forbidden {
			if _, err := regexp.Compile("(?i)" + f.Pattern); err != nil {
				add(fmt.Errorf("fixtures: scenario %s: bad forbidden pattern: %v", s.ID, err))
			}
		}
	}
	for _, p := range c.Professionals {
		if !idPattern.MatchString(p.ID) || !strings.Contains(strings.ToLower(p.Name), "fiktif") && !strings.Contains(strings.ToLower(p.Name), "fictional") {
			add(fmt.Errorf("fixtures: professional %q must have a valid id and a visibly fictional name", p.ID))
		}
		add(checkText("professional "+p.ID+" role", p.Role))
		add(checkText("professional "+p.ID+" cost", p.CostLabel))
		add(checkText("professional "+p.ID+" area", p.Area))
		add(checkText("professional "+p.ID+" bio", p.Bio))
		if !allowed(p.Category, "psychologist", "counselor", "psychiatrist", "community") || !allowed(p.CostTier, "free", "low", "standard") {
			add(fmt.Errorf("fixtures: professional %s: bad category/cost", p.ID))
		}
		for _, l := range p.Languages {
			if !allowed(l, "id", "en") {
				add(fmt.Errorf("fixtures: professional %s: bad language %q", p.ID, l))
			}
		}
		for _, f := range p.Formats {
			if !allowed(f, "online", "in_person") {
				add(fmt.Errorf("fixtures: professional %s: bad format %q", p.ID, f))
			}
		}
		for _, a := range p.Availability {
			if !allowed(a, "weekday_morning", "weekday_daytime", "weekday_evening", "weekend") {
				add(fmt.Errorf("fixtures: professional %s: bad availability %q", p.ID, a))
			}
		}
		for _, s := range p.Slots {
			if !strings.HasPrefix(s.Start, c.DemoClock[:4]) || !allowed(s.Format, p.Formats...) {
				add(fmt.Errorf("fixtures: slot %s: bad start/format", s.ID))
			}
		}
		for _, planID := range p.Plans {
			if c.plans[planID] == nil {
				add(fmt.Errorf("fixtures: professional %s: unknown plan %q", p.ID, planID))
			}
		}
	}
	for _, p := range c.Plans {
		add(checkText("plan "+p.ID+" title", p.Title))
		add(checkText("plan "+p.ID+" attribution", p.Attribution))
		if len(p.Steps) == 0 {
			add(fmt.Errorf("fixtures: plan %s: no steps", p.ID))
		}
		for i, s := range p.Steps {
			add(checkText(fmt.Sprintf("plan %s step %d", p.ID, i), s))
		}
	}
	for _, list := range [][]Option{c.CheckIns, c.ClarifyReasons, c.DeclineReasons} {
		for _, o := range list {
			add(checkText("option "+o.ID, o.Label))
		}
	}
	for _, n := range c.NextSteps {
		add(checkText("next step "+n.ID+" title", n.Title))
		add(checkText("next step "+n.ID+" description", n.Description))
	}
	for _, field := range []string{"concern", "duration", "daily_impact", "goal", "support_preference"} {
		t, ok := c.Clarifications[field]
		if !ok {
			add(fmt.Errorf("fixtures: clarification template %q missing", field))
			continue
		}
		add(checkText("clarification "+field, t))
	}
	add(checkText("help notice", c.HelpNotice))
	for _, h := range c.Help {
		if h.VerificationStatus != "verified_official" {
			add(fmt.Errorf("fixtures: help %s: only verified_official resources may be shown", h.ID))
		}
		if len(h.Sources) == 0 || !datePattern.MatchString(h.LastVerified) || !datePattern.MatchString(h.ReverifyBy) || h.Region == "" {
			add(fmt.Errorf("fixtures: help %s: sources, region and verification dates are required", h.ID))
		}
		for _, s := range h.Sources {
			if !strings.HasPrefix(s.URL, "https://") || !datePattern.MatchString(s.AccessedOn) {
				add(fmt.Errorf("fixtures: help %s: source must be https with an access date", h.ID))
			}
		}
		add(checkText("help "+h.ID+" region", h.RegionName))
		add(checkText("help "+h.ID+" hours", h.Hours))
		add(checkText("help "+h.ID+" cost", h.Cost))
		add(checkText("help "+h.ID+" description", h.Description))
	}
	for _, key := range []string{"privacy", "ai_processing", "storage", "sharing"} {
		p, ok := c.Policies[key]
		if !ok || p.Version == "" || len(p.Body) == 0 {
			add(fmt.Errorf("fixtures: policy %q missing", key))
			continue
		}
		add(checkText("policy "+key+" title", p.Title))
		for i, b := range p.Body {
			add(checkText(fmt.Sprintf("policy %s body %d", key, i), b))
		}
	}
	if len(errs) > 0 {
		sort.Strings(errs)
		return fmt.Errorf("fixtures invalid:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

// Scenario returns a scenario by id.
func (c *Catalog) Scenario(id string) (*Scenario, bool) { s, ok := c.scenarios[id]; return s, ok }

// Professional returns a professional by id.
func (c *Catalog) Professional(id string) (*Professional, bool) {
	p, ok := c.professionals[id]
	return p, ok
}

// Slot returns a slot and its professional.
func (c *Catalog) Slot(id string) (*Professional, *Slot, bool) {
	r, ok := c.slots[id]
	return r.prof, r.slot, ok
}

// Plan returns a plan by id.
func (c *Catalog) Plan(id string) (*Plan, bool) { p, ok := c.plans[id]; return p, ok }

// CheckIn returns a check-in option by id.
func (c *Catalog) CheckIn(id string) (*Option, bool) { o, ok := c.options["checkIn"][id]; return o, ok }

// ClarifyReason returns a recipient clarification reason by id.
func (c *Catalog) ClarifyReason(id string) (*Option, bool) {
	o, ok := c.options["clarify"][id]
	return o, ok
}

// DeclineReason returns a recipient decline reason by id.
func (c *Catalog) DeclineReason(id string) (*Option, bool) {
	o, ok := c.options["decline"][id]
	return o, ok
}
