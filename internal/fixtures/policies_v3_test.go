package fixtures

import (
	"strings"
	"testing"
	"time"
)

// TestGuestStoragePolicyText: the guest notice states where the data is,
// both deletion periods, the cookie dependency and that there is no
// absolute anonymity, in Indonesian and English, without forbidden
// phrases (ADR-0002 §7, F-33).
func TestGuestStoragePolicyText(t *testing.T) {
	p := RenderV3Policies(7*24*time.Hour, 30*24*time.Hour, "")
	g := p[PolicyGuestStorage]
	var id, en strings.Builder
	for _, b := range g.Body {
		id.WriteString(strings.ToLower(b.ID) + " ")
		en.WriteString(strings.ToLower(b.EN) + " ")
	}
	for lang, c := range map[string]struct {
		text string
		want []string
	}{
		"id": {id.String(), []string{"server sela", "basis data", "wilayah hosting", "7 hari", "30 hari", "cookie", "bukan anonimitas mutlak", "alamat ip", "dienkripsi"}},
		"en": {en.String(), []string{"sela's server", "database", "hosting region", "7 days", "30 days", "cookie", "not absolute anonymity", "ip address", "encrypted"}},
	} {
		for _, w := range c.want {
			if !strings.Contains(c.text, w) {
				t.Errorf("%s guest notice does not state %q", lang, w)
			}
		}
		if strings.Contains(c.text, "{") {
			t.Errorf("%s guest notice has an unrendered placeholder", lang)
		}
	}
	for key, pol := range p {
		for _, b := range pol.Body {
			for _, f := range ForbiddenGuestPhrases {
				if strings.Contains(strings.ToLower(b.ID), f) || strings.Contains(strings.ToLower(b.EN), f) {
					t.Errorf("%s contains forbidden phrase %q", key, f)
				}
			}
		}
	}
	if g.Version != "guest-storage-2026-11-01.1" {
		t.Fatalf("version %s", g.Version)
	}
}

func TestV3PoliciesRenderConfiguredDurations(t *testing.T) {
	p := RenderV3Policies(36*time.Hour, 24*time.Hour, "Jakarta (contoh)")
	body := p[PolicyGuestStorage].Body
	joined := ""
	for _, b := range body {
		joined += b.ID + "|" + b.EN + "|"
	}
	for _, w := range []string{"36 jam", "36 hours", "1 hari", "1 day ", "Jakarta (contoh)"} {
		if !strings.Contains(joined, w) {
			t.Errorf("rendered text lacks %q", w)
		}
	}
	if len(V3PolicyVersions()) != 3 {
		t.Fatal("v3 policy versions")
	}
}

func TestV3PoliciesRejectForbiddenPhrases(t *testing.T) {
	saved := v3PoliciesJSON
	defer func() { v3PoliciesJSON = saved }()
	v3PoliciesJSON = []byte(strings.Replace(string(saved), "Mode tamu tidak meminta nama", "Mode tamu anonim sepenuhnya, tidak meminta nama", 1))
	if _, err := V3Policies(); err == nil || !strings.Contains(err.Error(), "forbidden phrase") {
		t.Fatalf("forbidden phrase accepted: %v", err)
	}
}
