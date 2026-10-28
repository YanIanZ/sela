package fixtures

import (
	"strings"
	"testing"
	"time"
)

// TestGuestStoragePolicyText: the guest notice states where the data is,
// both deletion periods, the cookie dependency, that a restore from a
// backup ends guest access (every session is revoked and a guest has no
// other credential; ADR-0002 §11) and is announced, and that there is no
// absolute anonymity, in Indonesian and English, without forbidden
// phrases (ADR-0002 §7, F-33). Changing the text means a new version.
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
		"id": {id.String(), []string{"server sela", "basis data", "wilayah hosting", "7 hari", "30 hari", "cookie", "bukan anonimitas mutlak", "alamat ip", "dienkripsi",
			"memulihkan basis datanya dari cadangan", "semua sesi ikut berakhir", "tidak bisa dibuka lagi", "satu jam sebelum kegagalan", "mengumumkan setiap pemulihan"}},
		"en": {en.String(), []string{"sela's server", "database", "hosting region", "7 days", "30 days", "cookie", "not absolute anonymity", "ip address", "encrypted",
			"restore its database from a backup", "every session ends", "can then no longer be opened", "the hour before the failure", "announces every such restore"}},
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
	if g.Version != "guest-storage-2026-11-01.2" {
		t.Fatalf("version %s: a changed text needs a new version", g.Version)
	}
}

// TestTermsPrivacyStatesBackupWindows: the privacy text states the backup
// windows of ADR-0002 §11 and the restore window accepted for a controlled
// pilot (D-055): an account deleted in the hour before a total database
// loss may come back with a restore and need to be deleted again, and
// restores are announced. Changing the text means a new policy version.
func TestTermsPrivacyStatesBackupWindows(t *testing.T) {
	p := MustV3Policies()[PolicyTermsPrivacy]
	var id, en strings.Builder
	for _, b := range p.Body {
		id.WriteString(strings.ToLower(b.ID) + " ")
		en.WriteString(strings.ToLower(b.EN) + " ")
	}
	for lang, c := range map[string]struct {
		text string
		want []string
	}{
		"id": {id.String(), []string{"7 hari", "30 hari", "memulihkan basis data dari cadangan", "satu jam sebelum kegagalan", "perlu dihapus lagi", "mengumumkan setiap pemulihan"}},
		"en": {en.String(), []string{"7 days", "30 days", "restore the database from a backup", "the hour before the failure", "deleted again", "announces every such restore"}},
	} {
		for _, w := range c.want {
			if !strings.Contains(c.text, w) {
				t.Errorf("%s terms and privacy text does not state %q", lang, w)
			}
		}
	}
	if p.Version != "terms-privacy-2026-11-01.2" {
		t.Fatalf("version %s: a changed text needs a new version", p.Version)
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
	if len(V3PolicyVersions()) != 9 {
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
