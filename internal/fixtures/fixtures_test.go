package fixtures

import (
	"regexp"
	"strings"
	"testing"
)

func TestLoadValidCatalog(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Scenarios) < 4 || len(c.Professionals) < 5 || len(c.Help) == 0 {
		t.Fatalf("catalogue too small: %d scenarios, %d professionals, %d help", len(c.Scenarios), len(c.Professionals), len(c.Help))
	}
	if _, ok := c.Scenario("id-crisis-interrupt"); !ok {
		t.Fatal("crisis fixture missing")
	}
}

// Every forbidden pattern must NOT match the facts' own quotes for the
// non-negated, self-subject facts... but must be meaningful: at least one
// pattern per seeded scenario.
func TestSeededScenariosHaveGoldAnnotations(t *testing.T) {
	c := MustLoad()
	for _, s := range c.Scenarios {
		if s.Kind != "seeded" {
			continue
		}
		if len(s.Facts) < 5 || len(s.Forbidden) < 2 {
			t.Errorf("%s: needs >=5 facts and >=2 forbidden patterns", s.ID)
		}
		negated := false
		for _, f := range s.Facts {
			negated = negated || f.Negated
		}
		if !negated {
			t.Errorf("%s: needs at least one negated fact for Meaning Lock evaluation", s.ID)
		}
	}
}

func TestNoRealContactDetailsInFictionalData(t *testing.T) {
	c := MustLoad()
	phone := regexp.MustCompile(`\+?\d[\d -]{7,}\d`)
	email := regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	for _, p := range c.Professionals {
		for _, s := range []string{p.Name, p.Bio.ID, p.Bio.EN, p.Area.ID, p.Area.EN} {
			if phone.MatchString(s) || email.MatchString(s) || strings.Contains(s, "http") {
				t.Errorf("%s: fictional professional contains contact-like data: %q", p.ID, s)
			}
		}
	}
	for _, s := range c.Scenarios {
		if phone.MatchString(s.Story) || email.MatchString(s.Story) {
			t.Errorf("%s: story contains contact-like data", s.ID)
		}
	}
}

func TestRejectsUnverifiedHelp(t *testing.T) {
	c := MustLoad()
	c.Help[0].VerificationStatus = "todo_human_verification"
	if err := c.validate(); err == nil || !strings.Contains(err.Error(), "verified_official") {
		t.Fatalf("expected verification error, got %v", err)
	}
}

func TestRejectsMissingTranslation(t *testing.T) {
	c := MustLoad()
	c.Plans[0].Steps[0].EN = " "
	if err := c.validate(); err == nil {
		t.Fatal("expected missing translation error")
	}
}
