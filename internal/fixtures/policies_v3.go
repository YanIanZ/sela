package fixtures

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// V3 policy keys: the texts consent decisions in /api/v3 refer to.
const (
	PolicyTermsPrivacy        = "terms_privacy"
	PolicyGuestStorage        = "guest_storage"
	PolicyGuardianInformation = "guardian_information"
	// PolicyPersonalMemory is the optional consent for explicit memory.
	PolicyPersonalMemory = "personal_memory"
	// PolicyASRMeta is the optional consent for server-side dictation.
	PolicyASRMeta = "asr_meta"
)

//go:embed v3/policies.json
var v3PoliciesJSON []byte

// ForbiddenGuestPhrases promise an anonymity Sela cannot give; no v3 policy
// text may contain them (ADR-0002 §7).
var ForbiddenGuestPhrases = []string{"anonim sepenuhnya", "tidak bisa dilacak", "completely anonymous", "untraceable"}

// V3Policies returns the v3 policy templates (placeholders unrendered).
func V3Policies() (map[string]PolicyNotice, error) {
	var m map[string]PolicyNotice
	if err := json.Unmarshal(v3PoliciesJSON, &m); err != nil {
		return nil, fmt.Errorf("fixtures: v3 policies: %w", err)
	}
	var errs []string
	for _, key := range []string{PolicyTermsPrivacy, PolicyGuestStorage, PolicyGuardianInformation, PolicyPersonalMemory, PolicyASRMeta} {
		p, ok := m[key]
		if !ok || p.Version == "" || len(p.Body) == 0 {
			errs = append(errs, fmt.Sprintf("v3 policy %q missing", key))
			continue
		}
		if err := checkText("v3 policy "+key+" title", p.Title); err != nil {
			errs = append(errs, err.Error())
		}
		for i, b := range p.Body {
			if err := checkText(fmt.Sprintf("v3 policy %s body %d", key, i), b); err != nil {
				errs = append(errs, err.Error())
			}
			for _, f := range ForbiddenGuestPhrases {
				if strings.Contains(strings.ToLower(b.ID+" "+b.EN), f) {
					errs = append(errs, fmt.Sprintf("v3 policy %s body %d contains a forbidden phrase", key, i))
				}
			}
		}
	}
	if len(errs) > 0 {
		sort.Strings(errs)
		return nil, fmt.Errorf("fixtures invalid:\n%s", strings.Join(errs, "\n"))
	}
	return m, nil
}

// MustV3Policies is V3Policies for program start and tests.
func MustV3Policies() map[string]PolicyNotice {
	m, err := V3Policies()
	if err != nil {
		panic(err)
	}
	return m
}

// V3PolicyVersions maps each v3 policy key to its version.
func V3PolicyVersions() map[string]string {
	out := map[string]string{}
	for k, p := range MustV3Policies() {
		out[k] = p.Version
	}
	return out
}

// Duration renders a guest retention period in one language: whole days
// as days, anything else as hours.
func Duration(d time.Duration, lang string) string {
	unitID, unitEN := "jam", "hours"
	n := int(d / time.Hour)
	if d%(24*time.Hour) == 0 {
		unitID, unitEN, n = "hari", "days", int(d/(24*time.Hour))
	}
	if lang == "en" {
		if n == 1 {
			unitEN = strings.TrimSuffix(unitEN, "s")
		}
		return fmt.Sprintf("%d %s", n, unitEN)
	}
	return fmt.Sprintf("%d %s", n, unitID)
}

// RenderV3Policies fills the guest-retention and region placeholders with
// this deployment's configuration. The version stays that of the template.
func RenderV3Policies(inactive, maxAge time.Duration, region string) map[string]PolicyNotice {
	src := MustV3Policies()
	regionID, regionEN := region, region
	if region == "" {
		regionID, regionEN = "belum ditetapkan", "not yet decided"
	}
	fill := func(s, lang string) string {
		r := regionID
		if lang == "en" {
			r = regionEN
		}
		return strings.NewReplacer("{inactive}", Duration(inactive, lang), "{maxAge}", Duration(maxAge, lang), "{region}", r).Replace(s)
	}
	out := map[string]PolicyNotice{}
	for k, p := range src {
		q := PolicyNotice{Version: p.Version, Title: Text{ID: fill(p.Title.ID, "id"), EN: fill(p.Title.EN, "en")}}
		for _, b := range p.Body {
			q.Body = append(q.Body, Text{ID: fill(b.ID, "id"), EN: fill(b.EN, "en")})
		}
		out[k] = q
	}
	return out
}
