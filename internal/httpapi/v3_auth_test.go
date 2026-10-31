package httpapi_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/httpapi"
)

var codeFormat = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{4}(-[0-9A-HJKMNP-TV-Z]{4}){3}$`)

func TestV3Register(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	c := e.client()
	r := c.register("  Pelangi.Senja ")
	d := r.data()
	user := d["user"].(map[string]any)
	if user["kind"] != "account" || user["username"] != "pelangi.senja" || user["ageBand"] != "18_plus" || user["version"].(float64) != 1 ||
		user["environment"] != "test" || user["synthetic"] != true || user["guest"] != nil {
		t.Fatalf("user: %v", user)
	}
	rec := user["recovery"].(map[string]any)
	if rec["remaining"].(float64) != 10 || rec["acknowledged"] != false || !slices.Contains(strs(user["actionsRequired"]), "acknowledge_recovery_codes") {
		t.Fatalf("recovery: %v %v", rec, user["actionsRequired"])
	}
	codes := strs(d["recoveryCodes"])
	if len(codes) != 10 {
		t.Fatalf("%d codes", len(codes))
	}
	for _, code := range codes {
		if !codeFormat.MatchString(code) {
			t.Fatalf("code %q", code)
		}
	}
	if s := d["session"].(map[string]any); s["current"] != true || s["kind"] != "account" || s["trustedDevice"] != false {
		t.Fatalf("session: %v", s)
	}
	if c.token() == "" {
		t.Fatal("no cookie")
	}
	id := userID(t, r)
	// One transaction created all of it.
	for sql, want := range map[string]int{
		`SELECT count(*) FROM users WHERE id = $1 AND kind = 'account'`:                                                     1,
		`SELECT count(*) FROM credentials WHERE user_id = $1`:                                                               1,
		`SELECT count(*) FROM user_data_keys WHERE user_id = $1 AND wrapped_dek IS NOT NULL AND kek_id = 'test-http'`:       1,
		`SELECT count(*) FROM recovery_codes WHERE user_id = $1 AND used_at IS NULL`:                                        10,
		`SELECT count(*) FROM consents WHERE user_id = $1 AND purpose = 'terms_privacy' AND granted`:                        1,
		`SELECT count(*) FROM consent_events WHERE user_id = $1 AND source = 'onboarding' AND decision = 'granted'`:         1,
		`SELECT count(*) FROM sessions WHERE user_id = $1 AND revoked_at IS NULL`:                                           1,
		`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'account.created' AND detail->>'kind' = 'account'`: 1,
	} {
		if n := e.count(sql, id); n != want {
			t.Errorf("%s: %d, want %d", sql, n, want)
		}
	}
	// The codes are returned only once: no later response carries them.
	me := c.must(200, "GET", "/api/v3/me", nil)
	for _, code := range codes {
		if bytes.Contains(me.raw, []byte(code)) {
			t.Fatal("profile repeats a recovery code")
		}
	}
	// Errors: nothing is stored on any of them.
	users := e.count(`SELECT count(*) FROM users`)
	for name, tc := range map[string]struct {
		body   map[string]any
		status int
		code   string
		field  string
	}{
		"taken (case-insensitive)": {map[string]any{"username": "PELANGI.senja"}, 409, "username_taken", "username"},
		"invalid username":         {map[string]any{"username": "admin"}, 400, "username_invalid", "username"},
		"email username":           {map[string]any{"username": "budi.gmail.com"}, 400, "username_invalid", "username"},
		"short password":           {map[string]any{"username": "baru.satu", "password": "pendek sekali"}, 400, "password_too_short", "password"},
		"weak password":            {map[string]any{"username": "baru.dua", "password": "passwordpassword"}, 400, "password_weak", "password"},
		"username in password":     {map[string]any{"username": "baru.tiga", "password": "baru.tiga suka hujan"}, 400, "password_weak", "password"},
		"old terms":                {map[string]any{"username": "baru.empat", "termsPolicyVersion": "terms-privacy-2020-01-01.1"}, 409, "policy_outdated", "termsPolicyVersion"},
		"under 15":                 {map[string]any{"username": "baru.lima", "ageBand": "under_15"}, 403, "age_not_supported", "ageBand"},
		"explicit 15":              {map[string]any{"username": "baru.lima.lima", "ageBand": "under_16"}, 403, "age_not_supported", "ageBand"},
		"bad band":                 {map[string]any{"username": "baru.enam", "ageBand": "adult"}, 400, "invalid_input", "ageBand"},
		"bad locale":               {map[string]any{"username": "baru.tujuh", "locale": "fr"}, 400, "invalid_input", "locale"},
	} {
		body := map[string]any{"username": "x", "password": v3Password, "locale": "id", "ageBand": "18_plus", "termsPolicyVersion": policyVersion("terms_privacy")}
		for k, v := range tc.body {
			body[k] = v
		}
		r := e.client().call("POST", "/api/v3/auth/register", body)
		if r.status != tc.status || r.code() != tc.code || r.field() != tc.field {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	// Strict decoding: an unknown or case-variant key is invalid.
	for _, raw := range []string{
		`{"username":"x.y.z","password":"a long enough password","locale":"id","ageBand":"18_plus","termsPolicyVersion":"t","userId":"6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11"}`,
		`{"Username":"x.y.z","password":"a long enough password","locale":"id","ageBand":"18_plus","termsPolicyVersion":"t"}`,
		`{"username":"x.y.z","username":"y.y.y","password":"a long enough password","locale":"id","ageBand":"18_plus","termsPolicyVersion":"t"}`,
	} {
		if r := e.client().call("POST", "/api/v3/auth/register", raw); r.status != 400 || r.code() != "invalid_input" {
			t.Errorf("%s: %d %s", raw, r.status, r.raw)
		}
	}
	if n := e.count(`SELECT count(*) FROM users`); n != users {
		t.Fatalf("failed registrations stored %d users", n-users)
	}
	// A band this deployment does not enable.
	e2 := newV3(t, v3opt{mutateC: func(c *account.Config) { c.AllowedAgeBands = []string{"18_plus"} }})
	r = e2.client().call("POST", "/api/v3/auth/register", map[string]any{"username": "remaja.satu", "password": v3Password,
		"locale": "id", "ageBand": "15_17", "termsPolicyVersion": policyVersion("terms_privacy")})
	if r.status != 403 || r.code() != "age_band_not_enabled" || e2.count(`SELECT count(*) FROM users`) != 0 {
		t.Fatalf("band not enabled: %d %s", r.status, r.raw)
	}
}

func TestV3LoginUniformFailure(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	e.client().register("nama.ada")
	h := e.store.Hasher()
	before := h.Verifications()
	unknown := e.client().login("nama.tidak.ada", v3Password)
	mid := h.Verifications()
	wrong := e.client().login("nama.ada", "kata sandi yang salah sama sekali")
	after := h.Verifications()
	if unknown.status != 401 || wrong.status != 401 || unknown.code() != "invalid_credentials" {
		t.Fatalf("statuses %d %d", unknown.status, wrong.status)
	}
	if !bytes.Equal(unknown.raw, wrong.raw) {
		t.Fatalf("bodies differ:\n%s\n%s", unknown.raw, wrong.raw)
	}
	if mid-before != 1 || after-mid != 1 {
		t.Fatalf("argon2id verifications: unknown %d, wrong %d", mid-before, after-mid)
	}
	if len(setCookies(unknown)) != len(setCookies(wrong)) {
		t.Fatal("cookies differ")
	}
	// An invalid username shape gets the same answer too.
	if r := e.client().login("A D M I N", v3Password); !bytes.Equal(r.raw, unknown.raw) {
		t.Fatalf("malformed username: %s", r.raw)
	}
	en := e.client()
	en.lang = "en"
	if r := en.login("nama.tidak.ada", v3Password); !strings.Contains(string(r.raw), "The username or password does not match.") {
		t.Fatalf("English message: %s", r.raw)
	}
	if r := e.client().login("nama.ada", v3Password); r.status != 200 {
		t.Fatalf("login: %d %s", r.status, r.raw)
	}
}

func retry(t *testing.T, r resp) int {
	t.Helper()
	n, err := strconv.Atoi(r.header.Get("Retry-After"))
	if err != nil || n < 1 {
		t.Fatalf("Retry-After %q", r.header.Get("Retry-After"))
	}
	return n
}

func TestV3AuthRateLimits(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	// Per client: 20/min, burst 10. The eleventh login from one address
	// is refused; another address is not.
	c := e.client()
	for i := 0; i < 10; i++ {
		if r := c.login("tidak.ada."+strconv.Itoa(i), v3Password); r.status != 401 {
			t.Fatalf("attempt %d: %d", i, r.status)
		}
	}
	r := c.login("tidak.ada.lagi", v3Password)
	if r.status != 429 || r.code() != "rate_limited" || retry(t, r) != 3 {
		t.Fatalf("client limit: %d %s %s", r.status, r.header.Get("Retry-After"), r.raw)
	}
	if r := e.client().login("tidak.ada.lagi", v3Password); r.status != 401 {
		t.Fatalf("other client refused: %d", r.status)
	}
	// Per username (exists or not): 10/hour, burst 5, across addresses.
	e.client().register("batas.nama")
	for i := 0; i < 5; i++ {
		if r := e.client().login("batas.nama", v3Password); r.status != 200 {
			t.Fatalf("login %d: %d %s", i, r.status, r.raw)
		}
	}
	r = e.client().login("Batas.Nama", v3Password)
	if r.status != 429 || retry(t, r) != 360 {
		t.Fatalf("username limit: %d %s", r.status, r.header.Get("Retry-After"))
	}
	for i := 0; i < 5; i++ {
		e.client().login("hantu.nama", v3Password)
	}
	if r := e.client().login("hantu.nama", v3Password); r.status != 429 {
		t.Fatalf("unknown username not limited: %d", r.status)
	}
	// Registration: 20/hour per client, burst 5.
	reg := e.client()
	for i := 0; i < 4; i++ {
		reg.register("daftar.ke" + strconv.Itoa(i))
	}
	r = reg.call("POST", "/api/v3/auth/register", map[string]any{"username": "daftar.ke9", "password": v3Password, "locale": "id",
		"ageBand": "18_plus", "termsPolicyVersion": policyVersion("terms_privacy")})
	if r.status != 201 {
		t.Fatalf("fifth registration: %d", r.status)
	}
	r = reg.call("POST", "/api/v3/auth/register", map[string]any{"username": "daftar.ke10", "password": v3Password, "locale": "id",
		"ageBand": "18_plus", "termsPolicyVersion": policyVersion("terms_privacy")})
	if r.status != 429 || retry(t, r) != 180 {
		t.Fatalf("registration limit: %d %s", r.status, r.header.Get("Retry-After"))
	}
	// Guest start: 30/hour, burst 10.
	g := e.client()
	for i := 0; i < 10; i++ {
		g.guest("18_plus")
	}
	if r := g.call("POST", "/api/v3/auth/guest", map[string]any{"locale": "id", "ageBand": "18_plus", "termsPolicyVersion": policyVersion("terms_privacy"),
		"guestNoticeVersion": policyVersion("guest_storage")}); r.status != 429 || retry(t, r) != 120 {
		t.Fatalf("guest limit: %d %s", r.status, r.header.Get("Retry-After"))
	}
	// Recovery: 10/hour per client (burst 3) and 5/hour per username (burst 3).
	rc := e.client()
	for i := 0; i < 3; i++ {
		rc.call("POST", "/api/v3/auth/recovery", map[string]any{"username": "u" + strconv.Itoa(i) + "x", "recoveryCode": "0000-0000-0000-0000", "newPassword": v3NewPassword})
	}
	if r := rc.call("POST", "/api/v3/auth/recovery", map[string]any{"username": "u9x", "recoveryCode": "0000-0000-0000-0000", "newPassword": v3NewPassword}); r.status != 429 {
		t.Fatalf("recovery client limit: %d", r.status)
	}
	for i := 0; i < 3; i++ {
		e.client().call("POST", "/api/v3/auth/recovery", map[string]any{"username": "pulih.target", "recoveryCode": "0000-0000-0000-0000", "newPassword": v3NewPassword})
	}
	if r := e.client().call("POST", "/api/v3/auth/recovery", map[string]any{"username": "pulih.target", "recoveryCode": "0000-0000-0000-0000", "newPassword": v3NewPassword}); r.status != 429 {
		t.Fatalf("recovery username limit: %d", r.status)
	}

	// Backoff persists across a new server instance; 100 failures disable
	// password sign-in until recovery.
	e2 := newV3(t, v3opt{})
	owner := e2.client()
	reg2 := owner.register("tahan.lama")
	codes := strs(reg2.data()["recoveryCodes"])
	for i := 0; i < 5; i++ {
		if r := e2.client().login("tahan.lama", "salah salah salah salah"); r.status != 401 {
			t.Fatalf("failure %d: %d", i, r.status)
		}
	}
	e2.restart()
	r = e2.client().login("tahan.lama", v3Password)
	if r.status != 429 || r.code() != "rate_limited" || retry(t, r) != 1 {
		t.Fatalf("persisted backoff after restart: %d %s %s", r.status, r.header.Get("Retry-After"), r.raw)
	}
	if _, err := e2.db.Pool.Exec(bg, `UPDATE credentials SET failed_count = 99`); err != nil {
		t.Fatal(err)
	}
	e2.clock.Advance(time.Second) // the persisted 1 s backoff has passed
	e2.restart()
	if r := e2.client().login("tahan.lama", "salah salah salah salah"); r.status != 401 {
		t.Fatalf("100th failure: %d", r.status)
	}
	e2.clock.Advance(16 * time.Minute)
	e2.restart()
	r = e2.client().login("tahan.lama", v3Password)
	if r.status != 403 || r.code() != "password_login_disabled" {
		t.Fatalf("after 100 failures: %d %s", r.status, r.raw)
	}
	e2.client().must(200, "POST", "/api/v3/auth/recovery", map[string]any{"username": "tahan.lama", "recoveryCode": codes[0], "newPassword": v3NewPassword})
	if r := e2.client().login("tahan.lama", v3NewPassword); r.status != 200 {
		t.Fatalf("login after recovery: %d %s", r.status, r.raw)
	}
}

func TestV3Recovery(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	a := e.client()
	codes := strs(a.register("pulih.saja").data()["recoveryCodes"])
	other := e.client()
	other.login("pulih.saja", v3Password)
	oldA, oldOther := a.token(), other.token()
	// Tolerant entry: lower case, spaces instead of hyphens.
	entry := strings.ToLower(strings.ReplaceAll(codes[0], "-", " "))
	rc := e.client()
	r := rc.must(200, "POST", "/api/v3/auth/recovery", map[string]any{"username": "pulih.saja", "recoveryCode": entry, "newPassword": v3NewPassword})
	if r.data()["remainingRecoveryCodes"].(float64) != 9 || rc.token() == "" {
		t.Fatalf("recovery: %s", r.raw)
	}
	rc.must(200, "GET", "/api/v3/me", nil)
	// Every earlier session is revoked.
	for _, tok := range []string{oldA, oldOther} {
		x := e.client()
		x.setToken(tok)
		if r := x.call("GET", "/api/v3/me", nil); r.status != 401 || r.code() != "not_signed_in" {
			t.Fatalf("old session after recovery: %d", r.status)
		}
	}
	if r := e.client().login("pulih.saja", v3Password); r.status != 401 {
		t.Fatal("old password still works")
	}
	if r := e.client().login("pulih.saja", v3NewPassword); r.status != 200 {
		t.Fatalf("new password: %d", r.status)
	}
	id := userID(t, r)
	if e.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'recovery.used' AND (detail->>'remaining')::int = 9`, id) != 1 {
		t.Fatal("recovery.used not audited")
	}
	// Failures are identical: used code, wrong code, unknown user. (A new
	// instance first: the per-username bucket allows a burst of 3.)
	e.restart()
	var bodies [][]byte
	for _, in := range []map[string]any{
		{"username": "pulih.saja", "recoveryCode": codes[0], "newPassword": v3NewPassword + " lagi"},
		{"username": "pulih.saja", "recoveryCode": "0000-0000-0000-0000", "newPassword": v3NewPassword + " lagi"},
		{"username": "tidak.pernah.ada", "recoveryCode": codes[1], "newPassword": v3NewPassword + " lagi"},
	} {
		r := e.client().call("POST", "/api/v3/auth/recovery", in)
		if r.status != 401 || r.code() != "recovery_failed" {
			t.Fatalf("failure: %d %s", r.status, r.raw)
		}
		bodies = append(bodies, r.raw)
	}
	if !bytes.Equal(bodies[0], bodies[1]) || !bytes.Equal(bodies[1], bodies[2]) {
		t.Fatal("recovery failures differ")
	}
	// The new password is validated before any code is consumed.
	e.restart()
	if r := e.client().call("POST", "/api/v3/auth/recovery", map[string]any{"username": "pulih.saja", "recoveryCode": codes[1], "newPassword": "pendek"}); r.status != 400 || r.code() != "password_too_short" || r.field() != "newPassword" {
		t.Fatalf("short new password: %d %s", r.status, r.raw)
	}
	if n := e.count(`SELECT count(*) FROM recovery_codes WHERE user_id = $1 AND used_at IS NULL`, id); n != 9 {
		t.Fatalf("%d unused codes", n)
	}
}

func TestV3RecoveryCodes(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	c := e.client()
	first := strs(c.register("kode.baru").data()["recoveryCodes"])
	if r := c.call("POST", "/api/v3/auth/recovery-codes", map[string]any{"password": "bukan kata sandinya sama sekali"}); r.status != 403 || r.code() != "current_password_incorrect" {
		t.Fatalf("wrong password: %d %s", r.status, r.raw)
	}
	c.must(200, "GET", "/api/v3/me", nil) // session stays valid
	r := c.must(200, "POST", "/api/v3/auth/recovery-codes", map[string]any{"password": v3Password})
	second := strs(r.data()["recoveryCodes"])
	if len(second) != 10 || r.data()["generatedAt"] == nil || slices.Contains(second, first[0]) {
		t.Fatalf("regenerated: %s", r.raw)
	}
	if p := c.profile(); p["recovery"].(map[string]any)["acknowledged"] != false {
		t.Fatal("acknowledgement not reset")
	}
	// The previous batch is invalid.
	if r := e.client().call("POST", "/api/v3/auth/recovery", map[string]any{"username": "kode.baru", "recoveryCode": first[1], "newPassword": v3NewPassword}); r.status != 401 {
		t.Fatalf("old code accepted: %d", r.status)
	}
	// Acknowledge is idempotent.
	for i := 0; i < 2; i++ {
		p := c.must(200, "POST", "/api/v3/me/recovery-codes/acknowledge", map[string]any{}).data()
		if p["recovery"].(map[string]any)["acknowledged"] != true || slices.Contains(strs(p["actionsRequired"]), "acknowledge_recovery_codes") {
			t.Fatalf("acknowledge %d: %v", i, p)
		}
	}
	id := userID(t, c.must(200, "GET", "/api/v3/me", nil))
	if e.count(`SELECT count(*) FROM audit_events WHERE user_id = $1 AND type = 'recovery.acknowledged'`, id) != 1 {
		t.Fatal("acknowledgement audited more than once")
	}
	// Guests have no codes.
	g := e.client()
	g.guest("18_plus")
	if r := g.call("POST", "/api/v3/auth/recovery-codes", map[string]any{"password": v3Password}); r.status != 403 || r.code() != "account_required" {
		t.Fatalf("regenerate as guest: %d %s", r.status, r.raw)
	}
	if r := g.call("POST", "/api/v3/me/recovery-codes/acknowledge", map[string]any{}); r.status != 403 || r.code() != "account_required" {
		t.Fatalf("acknowledge as guest: %d %s", r.status, r.raw)
	}
}

func TestV3ChangePassword(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	a := e.client()
	a.register("ganti.sandi")
	b := e.client()
	b.login("ganti.sandi", v3Password)
	old := a.token()
	if r := a.call("POST", "/api/v3/auth/password", map[string]any{"currentPassword": "salah salah salah salah", "newPassword": v3NewPassword}); r.status != 403 || r.code() != "current_password_incorrect" {
		t.Fatalf("wrong current: %d %s", r.status, r.raw)
	}
	if r := a.call("POST", "/api/v3/auth/password", map[string]any{"currentPassword": v3Password, "newPassword": "ganti.sandi lagi ya"}); r.status != 400 || r.code() != "password_weak" || r.field() != "newPassword" {
		t.Fatalf("weak new: %d %s", r.status, r.raw)
	}
	r := a.must(200, "POST", "/api/v3/auth/password", map[string]any{"currentPassword": v3Password, "newPassword": v3NewPassword})
	if r.data()["revokedSessions"].(float64) != 1 || a.token() == old || a.token() == "" {
		t.Fatalf("change: %s", r.raw)
	}
	a.must(200, "GET", "/api/v3/me", nil)
	x := e.client()
	x.setToken(old)
	if r := x.call("GET", "/api/v3/me", nil); r.status != 401 {
		t.Fatalf("old token after change: %d", r.status)
	}
	if r := b.call("GET", "/api/v3/me", nil); r.status != 401 {
		t.Fatalf("other session after change: %d", r.status)
	}
	if r := e.client().login("ganti.sandi", v3NewPassword); r.status != 200 {
		t.Fatalf("login with the new password: %d", r.status)
	}
	g := e.client()
	g.guest("18_plus")
	if r := g.call("POST", "/api/v3/auth/password", map[string]any{"currentPassword": v3Password, "newPassword": v3NewPassword}); r.status != 403 || r.code() != "account_required" {
		t.Fatalf("guest: %d %s", r.status, r.raw)
	}
}

func TestV3ReauthRequired(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	a := e.client()
	a.register("bukti.sandi")
	// A fresh sign-in is a password proof.
	a.must(200, "GET", "/api/v3/me/export?format=json", nil)
	e.clock.Advance(11 * time.Minute)
	for _, rq := range []struct{ method, path string }{{"GET", "/api/v3/me/export?format=json"}, {"DELETE", "/api/v3/me"}} {
		var body any
		if rq.method == "DELETE" {
			body = map[string]any{"confirm": true}
		}
		if r := a.call(rq.method, rq.path, body); r.status != 403 || r.code() != "reauth_required" {
			t.Fatalf("%s %s without reauth: %d %s", rq.method, rq.path, r.status, r.raw)
		}
	}
	if r := a.call("POST", "/api/v3/auth/reauth", map[string]any{"password": "salah salah salah salah"}); r.status != 403 || r.code() != "current_password_incorrect" {
		t.Fatalf("wrong reauth: %d", r.status)
	}
	r := a.must(200, "POST", "/api/v3/auth/reauth", map[string]any{"password": v3Password})
	until, err := time.Parse(time.RFC3339, r.data()["reauthenticatedUntil"].(string))
	if err != nil || !until.Equal(e.clock.Now().Add(10*time.Minute).Truncate(time.Second)) {
		t.Fatalf("until %v %v", until, err)
	}
	a.must(200, "GET", "/api/v3/me/export?format=json", nil)
	e.clock.Advance(9 * time.Minute)
	a.must(200, "DELETE", "/api/v3/me", map[string]any{"confirm": true})
	// Guests: the cookie is the proof; reauth itself needs an account.
	g := e.client()
	g.guest("18_plus")
	e.clock.Advance(time.Hour)
	g.must(200, "GET", "/api/v3/me/export?format=json", nil)
	if r := g.call("POST", "/api/v3/auth/reauth", map[string]any{"password": v3Password}); r.status != 403 || r.code() != "account_required" {
		t.Fatalf("guest reauth: %d %s", r.status, r.raw)
	}
	g.must(200, "DELETE", "/api/v3/me", map[string]any{"confirm": true})
}

func cookieNamed(r resp, name string) string {
	for _, c := range setCookies(r) {
		if strings.HasPrefix(c, name+"=") {
			return c
		}
	}
	return ""
}

func TestV3CookieAttributes(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{secure: true})
	c := e.client()
	r := c.call("POST", "/api/v3/auth/register", map[string]any{"username": "kue.aman", "password": v3Password, "locale": "id",
		"ageBand": "18_plus", "termsPolicyVersion": policyVersion("terms_privacy")})
	if r.status != 201 {
		t.Fatalf("register: %d %s", r.status, r.raw)
	}
	ck := cookieNamed(r, httpapi.AuthCookie)
	parts := strings.Split(ck, "; ")
	tok := strings.TrimPrefix(parts[0], httpapi.AuthCookie+"=")
	if len(tok) != 43 || !regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`).MatchString(tok) {
		t.Fatalf("token %q", tok)
	}
	for _, want := range []string{"Path=/", "HttpOnly", "Secure", "SameSite=Strict"} {
		if !slices.Contains(parts, want) {
			t.Errorf("cookie %q lacks %s", ck, want)
		}
	}
	if strings.Contains(ck, "Domain=") || strings.Contains(ck, "Max-Age") || strings.Contains(ck, "Expires") {
		t.Fatalf("untrusted session cookie must be a browser-session cookie without Domain: %q", ck)
	}
	// Only SHA-256(token) is stored.
	h := sha256.Sum256([]byte(tok))
	if e.count(`SELECT count(*) FROM sessions WHERE token_hash = $1`, h[:]) != 1 {
		t.Fatal("token hash not stored")
	}
	if strings.Contains(e.dump(), tok) {
		t.Fatal("raw token at rest")
	}
	// Trusted device and guest sessions get Max-Age up to the absolute expiry.
	r = c.call("POST", "/api/v3/auth/login", map[string]any{"username": "kue.aman", "password": v3Password, "rememberDevice": true})
	if ck := cookieNamed(r, httpapi.AuthCookie); !strings.Contains(ck, "Max-Age=2592000") || !strings.Contains(ck, "Secure") {
		t.Fatalf("trusted cookie: %q", ck)
	}
	r = c.call("POST", "/api/v3/auth/guest", map[string]any{"locale": "id", "ageBand": "18_plus", "termsPolicyVersion": policyVersion("terms_privacy"),
		"guestNoticeVersion": policyVersion("guest_storage")})
	if ck := cookieNamed(r, httpapi.AuthCookie); !strings.Contains(ck, "Max-Age=2592000") {
		t.Fatalf("guest cookie: %q", ck)
	}
	// Logout clears it.
	r = c.call("POST", "/api/v3/auth/logout", nil)
	if ck := cookieNamed(r, httpapi.AuthCookie); !strings.Contains(ck, "Max-Age=0") {
		t.Fatalf("logout cookie: %q", ck)
	}
	// Plain-HTTP development: sela_auth without Secure.
	e2 := newV3(t, v3opt{})
	r = e2.client().register("kue.biasa")
	if ck := cookieNamed(r, httpapi.InsecureAuthCookie); ck == "" || strings.Contains(ck, "Secure") || !strings.Contains(ck, "HttpOnly") {
		t.Fatalf("insecure cookie: %q", setCookies(r))
	}
}

func (e *v3env) dump() string {
	var b strings.Builder
	rows, err := e.db.Pool.Query(bg, `SELECT tablename FROM pg_tables WHERE schemaname = current_schema()`)
	if err != nil {
		e.t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		tables = append(tables, s)
	}
	rows.Close()
	for _, tbl := range tables {
		rows, err := e.db.Pool.Query(bg, "SELECT t::text FROM "+tbl+" t")
		if err != nil {
			e.t.Fatal(err)
		}
		for rows.Next() {
			var s string
			_ = rows.Scan(&s)
			b.WriteString(s + "\n")
		}
		rows.Close()
	}
	return b.String()
}

func TestV3SessionRotation(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	e.client().register("putar.sesi")
	// Fixation: an attacker plants their own session cookie in the victim's
	// browser; signing in revokes it and issues a new one.
	attacker := e.client()
	attacker.guest("18_plus")
	planted := attacker.token()
	victim := e.client()
	victim.setToken(planted)
	r := victim.login("putar.sesi", v3Password)
	if r.status != 200 || victim.token() == planted {
		t.Fatalf("login with a planted cookie: %d", r.status)
	}
	if r := attacker.call("GET", "/api/v3/me", nil); r.status != 401 {
		t.Fatalf("planted session survived the login: %d", r.status)
	}
	// Login with an own existing cookie rotates too.
	before := victim.token()
	victim.login("putar.sesi", v3Password)
	x := e.client()
	x.setToken(before)
	if r := x.call("GET", "/api/v3/me", nil); r.status != 401 {
		t.Fatal("previous token valid after login")
	}
	// Guest upgrade.
	g := e.client()
	g.guest("18_plus")
	gt := g.token()
	g.must(200, "POST", "/api/v3/auth/guest/upgrade", map[string]any{"username": "dari.tamu", "password": v3Password})
	x.setToken(gt)
	if r := x.call("GET", "/api/v3/me", nil); r.status != 401 || g.token() == gt {
		t.Fatal("guest token valid after upgrade")
	}
	g.must(200, "GET", "/api/v3/me", nil)
	// Password change.
	pt := g.token()
	g.must(200, "POST", "/api/v3/auth/password", map[string]any{"currentPassword": v3Password, "newPassword": v3NewPassword})
	x.setToken(pt)
	if r := x.call("GET", "/api/v3/me", nil); r.status != 401 {
		t.Fatal("token valid after password change")
	}
	// Recovery.
	codes := strs(e.client().register("pulih.putar").data()["recoveryCodes"])
	rc := e.client()
	rc.login("pulih.putar", v3Password)
	rt := rc.token()
	rc.must(200, "POST", "/api/v3/auth/recovery", map[string]any{"username": "pulih.putar", "recoveryCode": codes[0], "newPassword": v3NewPassword})
	x.setToken(rt)
	if r := x.call("GET", "/api/v3/me", nil); r.status != 401 || rc.token() == rt {
		t.Fatal("token valid after recovery")
	}
	// The rotation is recorded.
	if e.count(`SELECT count(*) FROM sessions WHERE rotated_from IS NOT NULL`) < 2 {
		t.Fatal("rotated_from not recorded")
	}
}

func TestV3SessionsListRevoke(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	a := e.client()
	a.register("daftar.sesi")
	a2 := e.client()
	r2 := a2.login("daftar.sesi", v3Password)
	other := sessionID(t, r2)
	e.clock.Advance(90 * time.Minute)
	list := a.must(200, "GET", "/api/v3/me/sessions", nil).data()["sessions"].([]any)
	if len(list) != 2 {
		t.Fatalf("%d sessions", len(list))
	}
	first := list[0].(map[string]any)
	dev := first["device"].(map[string]any)
	if first["current"] != true || dev["browser"] != "chrome" || dev["os"] != "android" || !strings.HasSuffix(first["lastSeenAt"].(string), ":00:00Z") {
		t.Fatalf("session view: %v", first)
	}
	// Another user cannot revoke it: 404, and it stays live.
	b := e.client()
	b.register("orang.lain")
	if r := b.call("DELETE", "/api/v3/me/sessions/"+other, nil); r.status != 404 || r.code() != "not_found" {
		t.Fatalf("cross-user revoke: %d", r.status)
	}
	a2.must(200, "GET", "/api/v3/me", nil)
	for _, bad := range []string{"6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11", "not-a-uuid"} {
		if r := a.call("DELETE", "/api/v3/me/sessions/"+bad, nil); r.status != 404 {
			t.Fatalf("unknown id %s: %d", bad, r.status)
		}
	}
	r := a.must(200, "DELETE", "/api/v3/me/sessions/"+other, nil)
	if r.data()["revoked"] != true || r.data()["current"] != false {
		t.Fatalf("revoke: %s", r.raw)
	}
	if r := a2.call("GET", "/api/v3/me", nil); r.status != 401 {
		t.Fatalf("revoked session still valid: %d", r.status)
	}
	a.must(200, "DELETE", "/api/v3/me/sessions/"+other, nil) // idempotent
	// Revoking the current session clears the cookie.
	cur := a.must(200, "GET", "/api/v3/me/sessions", nil).data()["sessions"].([]any)[0].(map[string]any)["id"].(string)
	r = a.must(200, "DELETE", "/api/v3/me/sessions/"+cur, nil)
	if r.data()["current"] != true || !strings.Contains(cookieNamed(r, httpapi.InsecureAuthCookie), "Max-Age=0") {
		t.Fatalf("revoke current: %s %v", r.raw, setCookies(r))
	}
	if r := a.call("GET", "/api/v3/me", nil); r.status != 401 {
		t.Fatal("current session survived")
	}
	// Logout is idempotent, with or without a session.
	l := e.client()
	l.login("orang.lain", v3Password)
	for i := 0; i < 2; i++ {
		r := l.must(200, "POST", "/api/v3/auth/logout", nil)
		if r.data()["signedOut"] != true || !strings.Contains(cookieNamed(r, httpapi.InsecureAuthCookie), "Max-Age=0") {
			t.Fatalf("logout %d: %s", i, r.raw)
		}
	}
	e.client().must(200, "POST", "/api/v3/auth/logout", map[string]any{})
}

func TestV3LogoutAll(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	a := e.client()
	a.register("keluar.semua")
	b, c := e.client(), e.client()
	b.login("keluar.semua", v3Password)
	c.login("keluar.semua", v3Password)
	r := a.must(200, "POST", "/api/v3/auth/logout-all", map[string]any{"keepCurrent": true})
	if r.data()["revoked"].(float64) != 2 || r.data()["signedOut"] != false || cookieNamed(r, httpapi.InsecureAuthCookie) != "" {
		t.Fatalf("keepCurrent: %s %v", r.raw, setCookies(r))
	}
	a.must(200, "GET", "/api/v3/me", nil)
	for _, x := range []*v3client{b, c} {
		if r := x.call("GET", "/api/v3/me", nil); r.status != 401 {
			t.Fatal("other session survived logout-all")
		}
	}
	d := e.client()
	d.login("keluar.semua", v3Password)
	r = a.must(200, "POST", "/api/v3/auth/logout-all", nil)
	if r.data()["revoked"].(float64) != 2 || r.data()["signedOut"] != true || !strings.Contains(cookieNamed(r, httpapi.InsecureAuthCookie), "Max-Age=0") {
		t.Fatalf("all: %s", r.raw)
	}
	for _, x := range []*v3client{a, d} {
		if r := x.call("GET", "/api/v3/me", nil); r.status != 401 {
			t.Fatal("session survived logout-all")
		}
	}
	if r := a.call("POST", "/api/v3/auth/logout-all", nil); r.status != 401 || r.code() != "not_signed_in" {
		t.Fatalf("without a session: %d", r.status)
	}
	if e.count(`SELECT count(*) FROM audit_events WHERE type = 'auth.logout_all'`) != 2 {
		t.Fatal("auth.logout_all not audited twice")
	}
}

func TestV3GuestStart(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{mutateC: func(c *account.Config) {
		c.GuestInactiveTTL, c.GuestMaxAge, c.RegionLabel = 48*time.Hour, 10*24*time.Hour, "Wilayah Contoh"
	}})
	g := e.client()
	r := g.guest("15_17")
	u := r.data()["user"].(map[string]any)
	gi := u["guest"].(map[string]any)
	created, _ := time.Parse(time.RFC3339, u["createdAt"].(string))
	inactive, _ := time.Parse(time.RFC3339, gi["inactivePurgeAt"].(string))
	maxAge, _ := time.Parse(time.RFC3339, gi["maxAgePurgeAt"].(string))
	st := gi["storage"].(map[string]any)
	if u["kind"] != "guest" || u["username"] != nil || u["recovery"] != nil || inactive.Sub(created) != 48*time.Hour ||
		maxAge.Sub(created) != 10*24*time.Hour || gi["inactiveTtlHours"].(float64) != 48 || gi["maxAgeDays"].(float64) != 10 ||
		st["location"] != "server_database" || st["region"] != "Wilayah Contoh" || st["encrypted"] != true || st["boundTo"] != "this_browser_cookie" {
		t.Fatalf("guest profile: %v", u)
	}
	if gc := u["guardianConsent"].(map[string]any); gc["status"] != "missing" || gc["required"] != true {
		t.Fatalf("guardian status: %v", gc)
	}
	if ck := cookieNamed(r, httpapi.InsecureAuthCookie); !strings.Contains(ck, "Max-Age=864000") || !strings.Contains(ck, "HttpOnly") {
		t.Fatalf("guest cookie: %q", ck)
	}
	id := userID(t, r)
	if e.count(`SELECT count(*) FROM consents WHERE user_id = $1 AND granted`, id) != 2 || e.count(`SELECT count(*) FROM users WHERE id = $1 AND kind = 'guest' AND guest_expires_at IS NOT NULL`, id) != 1 {
		t.Fatal("guest rows")
	}
	g.must(200, "GET", "/api/v3/me", nil)
	if r := e.client().call("GET", "/api/v3/me", nil); r.status != 401 {
		t.Fatal("no cookie, still signed in")
	}
	// Both versions are required and must be current.
	for name, body := range map[string]map[string]any{
		"no guest notice":  {"locale": "id", "ageBand": "18_plus", "termsPolicyVersion": policyVersion("terms_privacy")},
		"old guest notice": {"locale": "id", "ageBand": "18_plus", "termsPolicyVersion": policyVersion("terms_privacy"), "guestNoticeVersion": "guest-storage-old"},
		"old terms":        {"locale": "id", "ageBand": "18_plus", "termsPolicyVersion": "x", "guestNoticeVersion": policyVersion("guest_storage")},
	} {
		if r := e.client().call("POST", "/api/v3/auth/guest", body); r.status != 409 || r.code() != "policy_outdated" {
			t.Errorf("%s: %d %s", name, r.status, r.raw)
		}
	}
	for _, band := range []string{"under_15", "under_16"} {
		if r := e.client().call("POST", "/api/v3/auth/guest", map[string]any{"locale": "id", "ageBand": band, "termsPolicyVersion": policyVersion("terms_privacy"),
			"guestNoticeVersion": policyVersion("guest_storage")}); r.status != 403 || r.code() != "age_not_supported" {
			t.Fatalf("%s: %d", band, r.status)
		}
	}
	// Live-guest capacity.
	e2 := newV3(t, v3opt{mutateC: func(c *account.Config) { c.MaxGuests = 1 }})
	e2.client().guest("18_plus")
	r = e2.client().call("POST", "/api/v3/auth/guest", map[string]any{"locale": "id", "ageBand": "18_plus", "termsPolicyVersion": policyVersion("terms_privacy"),
		"guestNoticeVersion": policyVersion("guest_storage")})
	if r.status != 503 || r.code() != "server_busy" || retry(t, r) != 5 {
		t.Fatalf("capacity: %d %s", r.status, r.raw)
	}
}

func TestV3GuestUpgradeKeepsData(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	g := e.client()
	r := g.guest("15_17")
	id := userID(t, r)
	g.must(200, "PUT", "/api/v3/me/guardian-consent", map[string]any{"relationship": "parent", "method": "guardian_present_attestation",
		"guardianName": "Ibu Sintetis", "policyVersion": policyVersion("guardian_information")})
	g.must(200, "PUT", "/api/v3/me/consents/test_optional", map[string]any{"granted": true, "policyVersion": "optional-test.1"})
	before := e.owned(id)
	gt := g.token()
	up := g.must(200, "POST", "/api/v3/auth/guest/upgrade", map[string]any{"username": "tamu.naik", "password": v3Password, "rememberDevice": true})
	if userID(t, up) != id {
		t.Fatal("upgrade changed the user id")
	}
	u := up.data()["user"].(map[string]any)
	if u["kind"] != "account" || u["guest"] != nil || u["username"] != "tamu.naik" || len(strs(up.data()["recoveryCodes"])) != 10 || g.token() == gt {
		t.Fatalf("upgrade: %s", up.raw)
	}
	after := e.owned(id)
	grew := map[string]int{"credentials": 1, "recovery_codes": 10, "sessions": 1, "audit_events": 1}
	for tbl, n := range before {
		if after[tbl] != n+grew[tbl] {
			t.Errorf("%s: %d before, %d after", tbl, n, after[tbl])
		}
	}
	if e.count(`SELECT count(*) FROM users WHERE id = $1 AND kind = 'account' AND upgraded_at IS NOT NULL AND guest_expires_at IS NULL`, id) != 1 {
		t.Fatal("users row not upgraded")
	}
	gc := g.must(200, "GET", "/api/v3/me/guardian-consent", nil).data()
	if gc["active"].(map[string]any)["guardianName"] != "Ibu Sintetis" {
		t.Fatal("encrypted data unreadable after upgrade")
	}
	if r := g.call("POST", "/api/v3/auth/guest/upgrade", map[string]any{"username": "tamu.lagi", "password": v3Password}); r.status != 409 || r.code() != "not_guest" {
		t.Fatalf("second upgrade: %d %s", r.status, r.raw)
	}
	if r := e.client().login("tamu.naik", v3Password); r.status != 200 {
		t.Fatalf("login after upgrade: %d", r.status)
	}
	// Username taken during upgrade.
	g2 := e.client()
	g2.guest("18_plus")
	if r := g2.call("POST", "/api/v3/auth/guest/upgrade", map[string]any{"username": "tamu.naik", "password": v3Password}); r.status != 409 || r.code() != "username_taken" {
		t.Fatalf("taken: %d %s", r.status, r.raw)
	}
	g2.must(200, "GET", "/api/v3/me", nil)
}

func (e *v3env) owned(id account.UserID) map[string]int {
	out := map[string]int{}
	for _, tbl := range []string{"users", "credentials", "recovery_codes", "sessions", "user_data_keys", "consents", "consent_events", "guardian_consents", "audit_events"} {
		col := "user_id"
		if tbl == "users" {
			col = "id"
		}
		out[tbl] = e.count("SELECT count(*) FROM "+tbl+" WHERE "+col+" = $1", id)
	}
	return out
}

func hexOf(b []byte) string { return hex.EncodeToString(b) }

// TestV3SessionExpiryClearsCookie: an expired session answers 401
// not_signed_in and clears the cookie (the account tests cover the
// timeouts per session kind).
func TestV3SessionExpiryClearsCookie(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{mutateO: func(_ *v3env, o *httpapi.Options) { o.HSTS = true }})
	c := e.client()
	c.register("sesi.habis")
	r := c.must(200, "GET", "/api/v3/me", nil)
	if r.header.Get("Strict-Transport-Security") != "max-age=31536000" || r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("headers: %v", r.header)
	}
	e.clock.Advance(2*time.Hour + time.Second)
	r = c.call("GET", "/api/v3/me", nil)
	if r.status != 401 || r.code() != "not_signed_in" || !strings.Contains(cookieNamed(r, httpapi.InsecureAuthCookie), "Max-Age=0") {
		t.Fatalf("expired: %d %s %v", r.status, r.raw, setCookies(r))
	}
	// A garbage cookie is the same 401.
	x := e.client()
	x.setToken("not-a-token")
	if r := x.call("GET", "/api/v3/me/sessions", nil); r.status != 401 || r.code() != "not_signed_in" {
		t.Fatalf("garbage cookie: %d", r.status)
	}
}
