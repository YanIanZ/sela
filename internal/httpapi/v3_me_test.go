package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/httpapi"
)

func TestV3CrossUserNotFound(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	a := e.client()
	ra := a.register("pemilik.satu")
	aID, aSession := userID(t, ra), sessionID(t, ra)
	b := e.client()
	rb := b.register("pemilik.dua")
	bID := userID(t, rb)
	// A valid id of another user's session is indistinguishable from a missing one.
	missing := b.call("DELETE", "/api/v3/me/sessions/6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11", nil)
	cross := b.call("DELETE", "/api/v3/me/sessions/"+aSession, nil)
	if cross.status != 404 || !bytes.Equal(cross.raw, missing.raw) {
		t.Fatalf("cross-user: %d %s vs %s", cross.status, cross.raw, missing.raw)
	}
	a.must(200, "GET", "/api/v3/me", nil)
	// A user id in the query, body or path is never used.
	if p := b.must(200, "GET", "/api/v3/me?userId="+aID.String()+"&id="+aID.String(), nil).data(); p["id"] != bID.String() {
		t.Fatalf("query user id honoured: %v", p["id"])
	}
	if r := b.call("PATCH", "/api/v3/me", map[string]any{"expectedVersion": 1, "locale": "en", "userId": aID.String()}); r.status != 400 || r.code() != "invalid_input" {
		t.Fatalf("body user id: %d %s", r.status, r.raw)
	}
	if r := b.call("GET", "/api/v3/users/"+aID.String(), nil); r.status != 404 {
		t.Fatalf("users route exists: %d", r.status)
	}
	for _, rt := range httpapi.V3RouteTable(e.h) {
		for _, seg := range strings.Split(rt.Path, "/") {
			if strings.HasPrefix(seg, "{") && seg != "{id}" && seg != "{purpose}" && seg != "{reflectionId}" {
				t.Errorf("%s %s takes a path parameter %s", rt.Method, rt.Path, seg)
			}
		}
		if strings.Contains(strings.ToLower(rt.Path), "user") {
			t.Errorf("%s %s names a user in the path", rt.Method, rt.Path)
		}
	}
	// B's lists never show A's data.
	for _, path := range []string{"/api/v3/me/sessions", "/api/v3/me/audit", "/api/v3/me/consents/history", "/api/v3/me/export?format=json"} {
		r := b.must(200, "GET", path, nil)
		if bytes.Contains(r.raw, []byte(aID.String())) || bytes.Contains(r.raw, []byte(aSession)) || bytes.Contains(r.raw, []byte("pemilik.satu")) {
			t.Errorf("%s shows A's data", path)
		}
	}
}

func TestV3Consents(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	c := e.client()
	c.register("setuju.saja")
	list := c.must(200, "GET", "/api/v3/me/consents", nil).data()["consents"].([]any)
	var names []string
	for _, x := range list {
		m := x.(map[string]any)
		names = append(names, m["purpose"].(string))
		if m["purpose"] == "terms_privacy" && (m["granted"] != true || m["required"] != true || m["outdated"] != false || m["revision"].(float64) != 1) {
			t.Fatalf("terms: %v", m)
		}
		if m["purpose"] == "test_optional" && (m["granted"] != false || m["policyVersion"] != nil || m["decidedAt"] != nil || m["revision"].(float64) != 0) {
			t.Fatalf("undecided: %v", m)
		}
	}
	if !slices.Equal(names, []string{"terms_privacy", "test_optional", "personal_memory"}) {
		t.Fatalf("purposes for an account: %v (guest_storage must not apply)", names)
	}
	// A required purpose cannot be withdrawn.
	if r := c.call("PUT", "/api/v3/me/consents/terms_privacy", map[string]any{"granted": false, "policyVersion": policyVersion("terms_privacy")}); r.status != 409 || r.code() != "required_consent" {
		t.Fatalf("withdraw required: %d %s", r.status, r.raw)
	}
	// Granting needs the current version.
	if r := c.call("PUT", "/api/v3/me/consents/test_optional", map[string]any{"granted": true, "policyVersion": "optional-test.0"}); r.status != 409 || r.code() != "policy_outdated" {
		t.Fatalf("grant old version: %d %s", r.status, r.raw)
	}
	e.clock.Advance(time.Second)
	r := c.must(200, "PUT", "/api/v3/me/consents/test_optional", map[string]any{"granted": true, "policyVersion": "optional-test.1"})
	if r.data()["changed"] != true || r.data()["consent"].(map[string]any)["revision"].(float64) != 1 {
		t.Fatalf("grant: %s", r.raw)
	}
	id := userID(t, c.must(200, "GET", "/api/v3/me", nil))
	events := e.count(`SELECT count(*) FROM consent_events WHERE user_id = $1`, id)
	// The same decision on the same version adds nothing.
	if r := c.must(200, "PUT", "/api/v3/me/consents/test_optional", map[string]any{"granted": true, "policyVersion": "optional-test.1"}); r.data()["changed"] != false {
		t.Fatalf("repeat: %s", r.raw)
	}
	if n := e.count(`SELECT count(*) FROM consent_events WHERE user_id = $1`, id); n != events {
		t.Fatal("repeated decision added an event")
	}
	// Withdrawal is never blocked by a version mismatch; it records the
	// version last agreed to, never the text the client sent.
	e.clock.Advance(time.Second)
	if r := c.call("PUT", "/api/v3/me/consents/test_optional", map[string]any{"granted": false, "policyVersion": "aku depresi, hubungi 0812 3456 7890"}); r.status != 400 || r.code() != "invalid_input" || r.field() != "policyVersion" {
		t.Fatalf("free-text version: %d %s", r.status, r.raw)
	}
	r = c.must(200, "PUT", "/api/v3/me/consents/test_optional", map[string]any{"granted": false, "policyVersion": "optional-test.0"})
	if con := r.data()["consent"].(map[string]any); con["granted"] != false || con["policyVersion"] != "optional-test.1" || con["revision"].(float64) != 2 {
		t.Fatalf("withdraw: %s", r.raw)
	}
	if n := e.count(`SELECT count(*) FROM consent_events WHERE policy_version = 'optional-test.0'`) +
		e.count(`SELECT count(*) FROM audit_events WHERE detail->>'policyVersion' = 'optional-test.0'`); n != 0 {
		t.Fatalf("the client's version was stored %d times", n)
	}
	for path, want := range map[string]int{"/api/v3/me/consents/nonexistent": 404, "/api/v3/me/consents/guest_storage": 404, "/api/v3/me/consents/ai_processing": 404} {
		if r := c.call("PUT", path, map[string]any{"granted": true, "policyVersion": "x"}); r.status != want || r.code() != "not_found" {
			t.Errorf("%s: %d", path, r.status)
		}
	}
	if r := c.call("PUT", "/api/v3/me/consents/test_optional", map[string]any{"policyVersion": "x"}); r.status != 400 || r.field() != "granted" {
		t.Fatalf("missing granted: %d %s", r.status, r.raw)
	}
	// History: newest first, paginated, filterable.
	e.clock.Advance(time.Second)
	c.must(200, "PUT", "/api/v3/me/consents/test_optional", map[string]any{"granted": true, "policyVersion": "optional-test.1"})
	h := c.must(200, "GET", "/api/v3/me/consents/history?limit=2", nil).data()
	ev := h["events"].([]any)
	if len(ev) != 2 || h["nextCursor"] == nil || ev[0].(map[string]any)["decision"] != "granted" || ev[1].(map[string]any)["decision"] != "withdrawn" {
		t.Fatalf("page 1: %v", h)
	}
	h2 := c.must(200, "GET", "/api/v3/me/consents/history?limit=2&cursor="+url.QueryEscape(h["nextCursor"].(string)), nil).data()
	ev2 := h2["events"].([]any)
	if len(ev2) != 2 || h2["nextCursor"] != nil {
		t.Fatalf("page 2: %v", h2)
	}
	seen := map[string]bool{}
	for _, x := range append(ev, ev2...) {
		seen[x.(map[string]any)["id"].(string)] = true
	}
	if len(seen) != 4 {
		t.Fatal("pages overlap")
	}
	f := c.must(200, "GET", "/api/v3/me/consents/history?purpose=terms_privacy", nil).data()["events"].([]any)
	if len(f) != 1 || f[0].(map[string]any)["source"] != "onboarding" {
		t.Fatalf("filter: %v", f)
	}
	for _, q := range []string{"cursor=not*base64", "cursor=AAAA", "limit=0", "limit=101", "limit=x", "purpose=BAD"} {
		if r := c.call("GET", "/api/v3/me/consents/history?"+q, nil); r.status != 400 || r.code() != "invalid_input" {
			t.Errorf("%s: %d", q, r.status)
		}
	}
	// Audit events follow each change.
	if n := e.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type IN ('consent.granted', 'consent.withdrawn')`, id); n != 3 {
		t.Fatalf("%d consent audit events", n)
	}
	// Guests see guest_storage, which they cannot withdraw.
	g := e.client()
	g.guest("18_plus")
	gl := g.must(200, "GET", "/api/v3/me/consents", nil).data()["consents"].([]any)
	if len(gl) != 4 {
		t.Fatalf("guest purposes: %v", gl)
	}
	if r := g.call("PUT", "/api/v3/me/consents/guest_storage", map[string]any{"granted": false, "policyVersion": policyVersion("guest_storage")}); r.status != 409 || r.code() != "required_consent" {
		t.Fatalf("guest withdraw: %d", r.status)
	}
}

func TestV3AgeBand(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	// under_15 is refused everywhere with nothing stored.
	for path, body := range map[string]map[string]any{
		"/api/v3/auth/register": {"username": "anak.kecil", "password": v3Password, "locale": "id", "ageBand": "under_15",
			"termsPolicyVersion": policyVersion("terms_privacy")},
		"/api/v3/auth/guest": {"locale": "id", "ageBand": "under_15", "termsPolicyVersion": policyVersion("terms_privacy"),
			"guestNoticeVersion": policyVersion("guest_storage")},
	} {
		r := e.client().call("POST", path, body)
		if r.status != 403 || r.code() != "age_not_supported" || !strings.Contains(string(r.raw), "15 tahun") {
			t.Fatalf("%s: %d %s", path, r.status, r.raw)
		}
	}
	if n := e.count(`SELECT count(*) FROM users`); n != 0 {
		t.Fatalf("%d users stored for under 15", n)
	}
	c := e.client()
	c.register("usia.saya")
	p := c.profile()
	id := userID(t, c.must(200, "GET", "/api/v3/me", nil))
	if r := c.call("PATCH", "/api/v3/me", map[string]any{"expectedVersion": 1, "ageBand": "under_15"}); r.status != 403 || r.code() != "age_not_supported" {
		t.Fatalf("patch under 15: %d", r.status)
	}
	if q := c.profile(); q["ageBand"] != "18_plus" || q["version"].(float64) != 1 {
		t.Fatalf("under_15 changed the profile: %v", q)
	}
	// Repeating the band confirms it.
	e.clock.Advance(time.Hour)
	r := c.must(200, "PATCH", "/api/v3/me", map[string]any{"expectedVersion": 1, "ageBand": "18_plus"})
	if r.data()["version"].(float64) != 2 || r.data()["ageBandConfirmedAt"] == p["ageBandConfirmedAt"] {
		t.Fatalf("confirm: %s", r.raw)
	}
	r = c.must(200, "PATCH", "/api/v3/me", map[string]any{"expectedVersion": 2, "ageBand": "15_17", "locale": "en"})
	if r.data()["ageBand"] != "15_17" || r.data()["locale"] != "en" || r.data()["guardianConsent"].(map[string]any)["status"] != "missing" ||
		!slices.Contains(strs(r.data()["actionsRequired"]), "guardian_consent") {
		t.Fatalf("change: %s", r.raw)
	}
	if r := c.call("PATCH", "/api/v3/me", map[string]any{"expectedVersion": 2, "locale": "id"}); r.status != 409 || r.code() != "stale_version" {
		t.Fatalf("stale: %d", r.status)
	}
	for body, field := range map[string]string{`{"expectedVersion":3}`: "body", `{"locale":"id"}`: "expectedVersion", `{"expectedVersion":3,"locale":"jv"}`: "locale", `{"expectedVersion":3,"ageBand":"adult"}`: "ageBand"} {
		if r := c.call("PATCH", "/api/v3/me", body); r.status != 400 || r.field() != field {
			t.Errorf("%s: %d %s", body, r.status, r.raw)
		}
	}
	for typ, want := range map[string]int{"profile.age_band_confirmed": 1, "profile.age_band_changed": 1, "profile.locale_changed": 1} {
		if n := e.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = $2`, id, typ); n != want {
			t.Errorf("%s: %d", typ, n)
		}
	}
	if e.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'profile.age_band_changed' AND detail->>'from' = '18_plus' AND detail->>'to' = '15_17'`, id) != 1 {
		t.Fatal("age band change detail")
	}
	// A band outside SELA_ALLOWED_AGE_BANDS.
	e2 := newV3(t, v3opt{mutateC: func(c *account.Config) { c.AllowedAgeBands = []string{"18_plus"} }})
	c2 := e2.client()
	c2.register("dewasa.saja")
	for _, band := range []string{"15_17", "unknown"} {
		if r := c2.call("PATCH", "/api/v3/me", map[string]any{"expectedVersion": 1, "ageBand": band}); r.status != 403 || r.code() != "age_band_not_enabled" {
			t.Fatalf("%s: %d %s", band, r.status, r.raw)
		}
	}
	if r := e2.client().call("POST", "/api/v3/auth/guest", map[string]any{"locale": "id", "ageBand": "unknown", "termsPolicyVersion": policyVersion("terms_privacy"),
		"guestNoticeVersion": policyVersion("guest_storage")}); r.status != 403 || r.code() != "age_band_not_enabled" {
		t.Fatalf("guest band: %d", r.status)
	}
	if opts := e2.client().must(200, "GET", "/api/v3/auth/options", nil).data(); fmt.Sprint(opts["ageBands"]) != "map[allowed:[18_plus]]" {
		t.Fatalf("options: %v", opts["ageBands"])
	}
}

func TestV3GuardianConsent(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	adult := e.client()
	adult.register("bukan.remaja")
	put := map[string]any{"relationship": "parent", "method": "guardian_present_attestation", "policyVersion": policyVersion("guardian_information")}
	if r := adult.call("PUT", "/api/v3/me/guardian-consent", put); r.status != 409 || r.code() != "invalid_transition" {
		t.Fatalf("18_plus: %d %s", r.status, r.raw)
	}
	if g := adult.must(200, "GET", "/api/v3/me/guardian-consent", nil).data(); g["required"] != false || g["status"] != "not_applicable" {
		t.Fatalf("adult view: %v", g)
	}
	teen := e.client()
	r := teen.register("remaja.ok", map[string]any{"ageBand": "15_17"})
	id := userID(t, r)
	for name, tc := range map[string]struct {
		body  map[string]any
		code  string
		field string
	}{
		"old version":  {map[string]any{"policyVersion": "guardian-information-old"}, "policy_outdated", "policyVersion"},
		"phone name":   {map[string]any{"guardianName": "Bu Sari 0812 3456 7890"}, "invalid_input", "guardianName"},
		"email name":   {map[string]any{"guardianName": "sari@contoh.id"}, "invalid_input", "guardianName"},
		"long name":    {map[string]any{"guardianName": strings.Repeat("a", 81)}, "invalid_input", "guardianName"},
		"relationship": {map[string]any{"relationship": "friend"}, "invalid_input", "relationship"},
		"method":       {map[string]any{"method": "sms"}, "invalid_input", "method"},
	} {
		body := map[string]any{}
		for k, v := range put {
			body[k] = v
		}
		for k, v := range tc.body {
			body[k] = v
		}
		if r := teen.call("PUT", "/api/v3/me/guardian-consent", body); r.code() != tc.code || r.field() != tc.field {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	withName := map[string]any{"relationship": "legal_guardian", "method": "guardian_present_attestation", "guardianName": " Pak Wali Sintetis ",
		"policyVersion": policyVersion("guardian_information")}
	r = teen.must(200, "PUT", "/api/v3/me/guardian-consent", withName)
	active := r.data()["active"].(map[string]any)
	if active["guardianName"] != "Pak Wali Sintetis" || active["relationship"] != "legal_guardian" || active["withdrawnAt"] != nil {
		t.Fatalf("recorded: %s", r.raw)
	}
	if e.count(`SELECT count(*) FROM guardian_consents WHERE user_id = $1 AND guardian_name_ct IS NOT NULL AND position(convert_to('Wali', 'UTF8') in guardian_name_ct) = 0`, id) != 1 {
		t.Fatal("guardian name not encrypted at rest")
	}
	if strings.Contains(e.dump(), "Pak Wali Sintetis") {
		t.Fatal("guardian name plaintext at rest")
	}
	// Replacing withdraws the active record in the same transaction.
	e.clock.Advance(time.Second)
	teen.must(200, "PUT", "/api/v3/me/guardian-consent", put)
	g := teen.must(200, "GET", "/api/v3/me/guardian-consent", nil).data()
	hist := g["history"].([]any)
	if g["required"] != true || g["status"] != "active" || len(hist) != 2 || g["active"].(map[string]any)["guardianName"] != nil {
		t.Fatalf("after replace: %v", g)
	}
	if old := hist[1].(map[string]any); old["withdrawnBy"] != "user" || old["guardianName"] != "Pak Wali Sintetis" {
		t.Fatalf("replaced record: %v", old)
	}
	if e.count(`SELECT count(*) FROM guardian_consents WHERE user_id = $1 AND withdrawn_at IS NULL`, id) != 1 {
		t.Fatal("more than one active record")
	}
	// Withdrawal is idempotent.
	for i := 0; i < 2; i++ {
		if r := teen.must(200, "DELETE", "/api/v3/me/guardian-consent", nil); r.data()["active"] != nil {
			t.Fatalf("withdraw: %s", r.raw)
		}
	}
	if g := teen.must(200, "GET", "/api/v3/me/guardian-consent", nil).data(); g["status"] != "withdrawn" || g["active"] != nil {
		t.Fatalf("after withdraw: %v", g)
	}
	if n := e.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'guardian_consent.withdrawn'`, id); n != 1 {
		t.Fatalf("%d withdrawal events", n)
	}
	// Consent is not access: no guardian principal, route or cookie exists.
	for _, rt := range httpapi.V3RouteTable(e.h) {
		if strings.Contains(rt.Path, "guardian") && rt.Path != "/api/v3/me/guardian-consent" {
			t.Errorf("guardian route %s", rt.Path)
		}
	}
	for _, p := range []string{"/api/v3/guardian", "/api/v3/guardian/login", "/api/v3/guardians/me"} {
		if r := e.client().call("GET", p, nil); r.status != 404 {
			t.Errorf("%s: %d", p, r.status)
		}
	}
	if n := e.count(`SELECT count(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'guardian_consents' AND column_name ~ '(email|phone|contact|token)'`); n != 0 {
		t.Fatal("guardian contact or access columns exist")
	}
}

func TestV3AuditEvents(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	a := e.client()
	codes := strs(a.register("jejak.aman", map[string]any{"ageBand": "15_17"}).data()["recoveryCodes"])
	id := userID(t, a.must(200, "GET", "/api/v3/me", nil))
	var types []string
	ids := func() map[string]string {
		out := map[string]string{}
		rows, err := e.db.Pool.Query(bg, `SELECT id::text, type FROM audit_events WHERE user_id = $1`, id)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var i, typ string
			_ = rows.Scan(&i, &typ)
			out[i] = typ
		}
		rows.Close()
		return out
	}
	expect := func(typ string, do func()) {
		t.Helper()
		before := ids()
		do()
		var got []string
		for i, ty := range ids() {
			if _, ok := before[i]; !ok {
				got = append(got, ty)
			}
		}
		if len(got) != 1 || got[0] != typ {
			t.Errorf("%s: wrote %v", typ, got)
		}
		types = append(types, typ)
	}
	b := e.client()
	expect("auth.login_succeeded", func() { b.login("jejak.aman", v3Password) })
	expect("auth.login_failed", func() {
		for i := 0; i < 3; i++ {
			e.client().login("jejak.aman", "salah salah salah salah")
		}
	})
	e.clock.Advance(time.Minute)
	expect("auth.reauthenticated", func() { a.must(200, "POST", "/api/v3/auth/reauth", map[string]any{"password": v3Password}) })
	expect("profile.locale_changed", func() { a.must(200, "PATCH", "/api/v3/me", map[string]any{"expectedVersion": 1, "locale": "en"}) })
	expect("profile.age_band_confirmed", func() { a.must(200, "PATCH", "/api/v3/me", map[string]any{"expectedVersion": 2, "ageBand": "15_17"}) })
	expect("profile.age_band_changed", func() { a.must(200, "PATCH", "/api/v3/me", map[string]any{"expectedVersion": 3, "ageBand": "unknown"}) })
	a.must(200, "PATCH", "/api/v3/me", map[string]any{"expectedVersion": 4, "ageBand": "15_17"})
	expect("guardian_consent.recorded", func() {
		a.must(200, "PUT", "/api/v3/me/guardian-consent", map[string]any{"relationship": "parent", "method": "guardian_present_attestation", "policyVersion": policyVersion("guardian_information")})
	})
	expect("guardian_consent.withdrawn", func() { a.must(200, "DELETE", "/api/v3/me/guardian-consent", nil) })
	expect("consent.granted", func() {
		a.must(200, "PUT", "/api/v3/me/consents/test_optional", map[string]any{"granted": true, "policyVersion": "optional-test.1"})
	})
	expect("consent.withdrawn", func() {
		a.must(200, "PUT", "/api/v3/me/consents/test_optional", map[string]any{"granted": false, "policyVersion": "optional-test.1"})
	})
	expect("recovery.acknowledged", func() { a.must(200, "POST", "/api/v3/me/recovery-codes/acknowledge", nil) })
	expect("recovery.regenerated", func() {
		codes = strs(a.must(200, "POST", "/api/v3/auth/recovery-codes", map[string]any{"password": v3Password}).data()["recoveryCodes"])
	})
	expect("export.created", func() { a.must(200, "GET", "/api/v3/me/export?format=md", nil) })
	expect("session.revoked", func() {
		a.must(200, "DELETE", "/api/v3/me/sessions/"+b.must(200, "GET", "/api/v3/me/sessions", nil).data()["sessions"].([]any)[0].(map[string]any)["id"].(string), nil)
	})
	expect("password.changed", func() {
		a.must(200, "POST", "/api/v3/auth/password", map[string]any{"currentPassword": v3Password, "newPassword": v3NewPassword})
	})
	c := e.client()
	expect("recovery.used", func() {
		c.must(200, "POST", "/api/v3/auth/recovery", map[string]any{"username": "jejak.aman", "recoveryCode": codes[0], "newPassword": v3Password + " baru"})
	})
	expect("auth.logout_all", func() { c.must(200, "POST", "/api/v3/auth/logout-all", map[string]any{"keepCurrent": true}) })
	expect("auth.logout", func() { c.must(200, "POST", "/api/v3/auth/logout", nil) })
	if _, err := e.db.Pool.Exec(bg, `UPDATE credentials SET failed_count = 99 WHERE user_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(2 * time.Hour)
	e.restart()
	before := e.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'password_login.disabled'`, id)
	e.client().login("jejak.aman", "salah salah salah salah")
	if e.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'password_login.disabled'`, id) != before+1 {
		t.Error("password_login.disabled not written")
	}
	g := e.client()
	gid := userID(t, g.guest("18_plus"))
	e.clock.Advance(time.Second)
	g.must(200, "POST", "/api/v3/auth/guest/upgrade", map[string]any{"username": "tamu.jejak", "password": v3Password})
	e.clock.Advance(time.Second)
	if e.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'account.upgraded'`, gid) != 1 ||
		e.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'account.created' AND detail->>'kind' = 'guest'`, gid) != 1 {
		t.Error("guest lifecycle not audited")
	}
	// Every event in the database uses an allowed type and allow-listed detail keys only.
	rows, err := e.db.Pool.Query(bg, `SELECT type, detail FROM audit_events`)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for rows.Next() {
		var typ string
		var detail map[string]any
		if err := rows.Scan(&typ, &detail); err != nil {
			t.Fatal(err)
		}
		seen[typ] = true
		if !slices.Contains(account.AuditTypes, typ) {
			t.Errorf("unknown type %s", typ)
		}
		for k := range detail {
			if !slices.Contains(account.AuditDetailKeys, k) {
				t.Errorf("%s: detail key %s", typ, k)
			}
		}
	}
	rows.Close()
	for _, typ := range account.AuditTypes {
		if !seen[typ] {
			t.Errorf("type %s never written by this test", typ)
		}
	}
	// The list shows own events only, newest first, paginated.
	lc := e.client()
	lc.login("tamu.jejak", v3Password)
	page1 := lc.must(200, "GET", "/api/v3/me/audit?limit=2", nil).data()
	evs := page1["events"].([]any)
	if len(evs) != 2 || page1["nextCursor"] == nil || evs[0].(map[string]any)["type"] != "auth.login_succeeded" || evs[0].(map[string]any)["currentSession"] != true {
		t.Fatalf("audit page: %v", page1)
	}
	all := lc.must(200, "GET", "/api/v3/me/audit?limit=100", nil)
	if bytes.Contains(all.raw, []byte(id.String())) || len(all.data()["events"].([]any)) != e.count(`SELECT count(*) FROM audit_events WHERE user_id = $1`, gid) {
		t.Fatal("audit list not limited to own events")
	}
	page2 := lc.must(200, "GET", "/api/v3/me/audit?limit=2&cursor="+url.QueryEscape(page1["nextCursor"].(string)), nil).data()
	if ev := page2["events"].([]any); len(ev) == 0 || ev[0].(map[string]any)["id"] == evs[1].(map[string]any)["id"] {
		t.Fatalf("page 2: %v", page2)
	}
	_ = types
}

func TestV3ExportFormats(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	c := e.client()
	reg := c.register("ekspor.data", map[string]any{"ageBand": "15_17"})
	codes := strs(reg.data()["recoveryCodes"])
	tok := c.token()
	c.must(200, "PUT", "/api/v3/me/guardian-consent", map[string]any{"relationship": "parent", "method": "guardian_present_attestation",
		"guardianName": "Ibu <b>Sintetis</b> & [tautan](x)", "policyVersion": policyVersion("guardian_information")})
	r := c.must(200, "GET", "/api/v3/me/export?format=json", nil)
	day := e.clock.Now().UTC().Format("20060102")
	if r.header.Get("Content-Type") != "application/json; charset=utf-8" || r.header.Get("Cache-Control") != "no-store" ||
		r.header.Get("Content-Disposition") != `attachment; filename="sela-export-`+day+`.json"` {
		t.Fatalf("json headers: %v", r.header)
	}
	var doc map[string]any
	if err := json.Unmarshal(r.raw, &doc); err != nil {
		t.Fatal(err)
	}
	sections := doc["sections"].(map[string]any)
	var keys []string
	for k := range sections {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var want []string
	for _, ex := range e.store.Exporters() {
		want = append(want, ex.Section())
	}
	slices.Sort(want)
	if doc["format"] != "sela-export/1" || !slices.Equal(keys, want) || doc["environment"] != "test" || doc["synthetic"] != true {
		t.Fatalf("export: format %v sections %v want %v", doc["format"], keys, want)
	}
	if acct := sections["account"].(map[string]any); acct["username"] != "ekspor.data" || acct["recoveryCodes"].(map[string]any)["remaining"].(float64) != 10 {
		t.Fatalf("account section: %v", acct)
	}
	if g := sections["guardianConsents"].([]any); len(g) != 1 || !strings.Contains(g[0].(map[string]any)["guardianName"].(string), "Sintetis") {
		t.Fatalf("guardian section: %v", g)
	}
	notices := doc["notices"].(map[string]any)
	if !strings.Contains(fmt.Sprint(notices["id"]), "tidak dapat ditarik kembali") || !strings.Contains(fmt.Sprint(notices["en"]), "cannot be recalled") {
		t.Fatalf("notices: %v", notices)
	}
	// Never hashes, keys or tokens.
	var secrets []string
	rows, _ := e.db.Pool.Query(bg, `SELECT encode(token_hash, 'hex') FROM sessions UNION ALL SELECT encode(code_hash, 'hex') FROM recovery_codes
		UNION ALL SELECT encode(password_hash, 'hex') FROM credentials UNION ALL SELECT encode(password_salt, 'hex') FROM credentials
		UNION ALL SELECT encode(wrapped_dek, 'hex') FROM user_data_keys WHERE wrapped_dek IS NOT NULL UNION ALL SELECT encode(guardian_name_ct, 'hex') FROM guardian_consents`)
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		secrets = append(secrets, s)
	}
	rows.Close()
	secrets = append(secrets, tok, v3Password)
	secrets = append(secrets, codes...)
	for _, format := range []string{"json", "html", "md"} {
		r := c.must(200, "GET", "/api/v3/me/export?format="+format+"&lang=en", nil)
		for _, s := range secrets {
			if bytes.Contains(r.raw, []byte(s)) || bytes.Contains(r.raw, []byte(strings.ToUpper(s))) {
				t.Errorf("%s export contains a secret", format)
			}
		}
		for _, k := range []string{"token_hash", "password_hash", "wrapped_dek", "code_hash"} {
			if bytes.Contains(r.raw, []byte(k)) {
				t.Errorf("%s export names %s", format, k)
			}
		}
	}
	h := c.must(200, "GET", "/api/v3/me/export?format=html&lang=id", nil)
	html := string(h.raw)
	if h.header.Get("Content-Type") != "text/html; charset=utf-8" || h.header.Get("Content-Security-Policy") != "default-src 'none'; style-src 'unsafe-inline'; img-src data:; sandbox" ||
		!strings.HasSuffix(h.header.Get("Content-Disposition"), `.html"`) {
		t.Fatalf("html headers: %v", h.header)
	}
	if strings.Contains(strings.ToLower(html), "<script") || strings.Contains(html, "<b>Sintetis</b>") || !strings.Contains(html, "&lt;b&gt;Sintetis&lt;/b&gt;") ||
		!strings.Contains(html, `<html lang="id">`) || !strings.Contains(html, "Persetujuan orang tua atau wali") || strings.Contains(strings.ToLower(html), "<a ") {
		t.Fatalf("html body: %s", html)
	}
	md := c.must(200, "GET", "/api/v3/me/export?format=md&lang=en", nil)
	if md.header.Get("Content-Type") != "text/markdown; charset=utf-8" || !strings.HasPrefix(string(md.raw), "# Sela data export") ||
		strings.Contains(string(md.raw), "[tautan](x)") || !strings.Contains(string(md.raw), "## Parent or guardian consent") {
		t.Fatalf("markdown: %s", md.raw)
	}
	for _, q := range []string{"format=pdf", "format=json&lang=fr"} {
		if r := c.call("GET", "/api/v3/me/export?"+q, nil); r.status != 400 || r.code() != "invalid_input" {
			t.Errorf("%s: %d", q, r.status)
		}
	}
	// 6 exports per hour per user: five more were made above (json, 3 formats, html, md = 6).
	if r := c.call("GET", "/api/v3/me/export?format=json", nil); r.status != 429 || retry(t, r) != 600 {
		t.Fatalf("export limit: %d %s", r.status, r.header.Get("Retry-After"))
	}
}

func TestV3DeleteAccount(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	a := e.client()
	aID := userID(t, a.register("hapus.saya", map[string]any{"ageBand": "15_17"}))
	a.must(200, "PUT", "/api/v3/me/guardian-consent", map[string]any{"relationship": "parent", "method": "guardian_present_attestation",
		"guardianName": "Ibu Sintetis", "policyVersion": policyVersion("guardian_information")})
	a.must(200, "PUT", "/api/v3/me/consents/test_optional", map[string]any{"granted": true, "policyVersion": "optional-test.1"})
	second := e.client()
	second.login("hapus.saya", v3Password)
	b := e.client()
	bID := userID(t, b.register("tetap.aman"))
	b.must(200, "PUT", "/api/v3/me/consents/test_optional", map[string]any{"granted": true, "policyVersion": "optional-test.1"})
	bBefore := e.owned(bID)
	for _, body := range []any{nil, map[string]any{}, map[string]any{"confirm": false}} {
		if r := a.call("DELETE", "/api/v3/me", body); r.status != 400 || r.field() != "confirm" {
			t.Fatalf("confirm %v: %d %s", body, r.status, r.raw)
		}
	}
	r := a.must(200, "DELETE", "/api/v3/me", map[string]any{"confirm": true})
	keys, _ := time.Parse(time.RFC3339, r.data()["keysUnrecoverableBy"].(string))
	backups, _ := time.Parse(time.RFC3339, r.data()["backupsExpireBy"].(string))
	now := e.clock.Now()
	if r.data()["deleted"] != true || !keys.Equal(now.Add(7*24*time.Hour)) || !backups.Equal(now.Add(30*24*time.Hour)) {
		t.Fatalf("receipt: %s", r.raw)
	}
	if !strings.Contains(cookieNamed(r, httpapi.InsecureAuthCookie), "Max-Age=0") {
		t.Fatal("cookie not cleared")
	}
	for tbl, n := range e.owned(aID) {
		if tbl == "user_data_keys" {
			continue
		}
		if n != 0 {
			t.Errorf("%s: %d rows left", tbl, n)
		}
	}
	if e.count(`SELECT count(*) FROM user_data_keys WHERE user_id = $1 AND wrapped_dek IS NULL AND kek_id IS NULL AND destroyed_at IS NOT NULL`, aID) != 1 {
		t.Fatal("no key tombstone")
	}
	if e.count(`SELECT count(*) FROM sessions WHERE user_id = $1`, aID) != 0 || e.store.Vault().Cached(aID) {
		t.Fatal("sessions or cached key left")
	}
	// A retry is 401 not_signed_in, which the client treats as done.
	if r := second.call("DELETE", "/api/v3/me", map[string]any{"confirm": true}); r.status != 401 || r.code() != "not_signed_in" {
		t.Fatalf("retry: %d %s", r.status, r.raw)
	}
	// Another user's data is untouched.
	if after := e.owned(bID); fmt.Sprint(after) != fmt.Sprint(bBefore) {
		t.Fatalf("B changed: %v -> %v", bBefore, after)
	}
	b.must(200, "GET", "/api/v3/me", nil)
}
