package httpapi_test

import (
	"bytes"
	"net/http"
	"net/url"
	"testing"

	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/budget"
	"github.com/YanIanZ/sela/internal/httpapi"
)

const adminPassword = "sungai biru deh"

func newAdminEnv(t *testing.T, gates ...httpapi.BudgetGate) (*v3env, *admin.Store) {
	t.Helper()
	key := bytes.Repeat([]byte{9}, 32)
	var st *admin.Store
	e := newV3(t, v3opt{mutateO: func(e *v3env, o *httpapi.Options) {
		s, err := admin.New(e.db.Pool, admin.Options{Hasher: e.store.Hasher(), MFAKey: key, Now: e.clock.Now})
		if err != nil {
			t.Fatal(err)
		}
		st = s
		o.Admin = s
		if len(gates) > 0 {
			o.Budget = gates[0]
		}
	}})
	return e, st
}

func adminLogin(t *testing.T, e *v3env, st *admin.Store, c *v3client, username, password string, role admin.Role) {
	t.Helper()
	_, secretText, err := st.Create(bg, username, password, role, "")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := admin.DecodeTOTPSecret(secretText)
	if err != nil {
		t.Fatal(err)
	}
	code := admin.CodeAt(secret, e.clock.Now())
	c.must(200, "POST", "/api/admin/v1/sessions", map[string]any{
		"username": username, "password": password, "totp": code,
	})
}

func setAdminCookie(t *testing.T, c *v3client, value string) {
	t.Helper()
	u, _ := url.Parse(c.e.srv.URL)
	c.hc.Jar.SetCookies(u, []*http.Cookie{{Name: httpapi.InsecureAdminCookie, Value: value, Path: "/"}})
}

func TestAdminConsoleSeparateDomainAndPermissions(t *testing.T) {
	e, st := newAdminEnv(t)

	c := e.client()

	// Nothing is served before a staff sign-in.
	r := c.must(401, "GET", "/api/admin/v1/me", nil)
	if r.code() != "admin_not_signed_in" {
		t.Fatalf("code = %q", r.code())
	}

	// A consumer session token is not an operator token.
	c.register("langit.pagi")
	consumer := c.token()
	if consumer == "" {
		t.Fatal("consumer session missing")
	}
	setAdminCookie(t, c, consumer)
	c.must(401, "GET", "/api/admin/v1/me", nil)

	adminLogin(t, e, st, c, "ops.web", adminPassword, admin.RoleOps)
	me := c.must(200, "GET", "/api/admin/v1/me", nil)
	view, _ := me.data()["admin"].(map[string]any)
	if view["role"] != "ops" {
		t.Fatalf("role = %v", view["role"])
	}
	perms, _ := view["permissions"].([]any)
	foundCache := false
	for _, p := range perms {
		if p == admin.PermCacheWrite {
			foundCache = true
		}
	}
	if !foundCache {
		t.Fatalf("ops permissions = %v", perms)
	}

	// Role scoping is enforced server-side.
	c.must(403, "GET", "/api/admin/v1/audit", nil)
	ov := c.must(200, "GET", "/api/admin/v1/overview", nil)
	if ov.data()["activeStaff"].(float64) != 1 {
		t.Fatalf("overview = %v", ov.data())
	}
	// Redis is not configured in this environment.
	ch := c.must(200, "GET", "/api/admin/v1/cache/health", nil)
	cache, _ := ch.data()["cache"].(map[string]any)
	if cache["enabled"] != false {
		t.Fatalf("cache health = %v", cache)
	}
	c.must(503, "POST", "/api/admin/v1/cache/invalidate", nil)

	// The consumer session survives an operator logout, and only the
	// operator session ends.
	c.must(200, "DELETE", "/api/admin/v1/sessions/current", nil)
	c.must(401, "GET", "/api/admin/v1/me", nil)
	c.must(200, "GET", "/api/v3/me", nil)
}

func TestAdminConsoleRoleScopingAndDisabled(t *testing.T) {
	e, st := newAdminEnv(t)

	c := e.client()
	adminLogin(t, e, st, c, "security.one", adminPassword, admin.RoleSecurityAdmin)
	c.must(200, "GET", "/api/admin/v1/audit", nil)
	c.must(403, "POST", "/api/admin/v1/cache/invalidate", nil)

	// Without SELA_ADMIN_MFA_KEY the whole surface answers 503.
	off := newV3(t, v3opt{})
	d := off.client()
	r := d.must(503, "GET", "/api/admin/v1/me", nil)
	if r.code() != "admin_unavailable" {
		t.Fatalf("code = %q", r.code())
	}
	d.must(503, "POST", "/api/admin/v1/sessions", map[string]any{"username": "x", "password": "y", "totp": "z"})
}

func TestAdminOverviewShowsDurableAISpend(t *testing.T) {
	gate := &fakeGate{enabled: true, summary: budget.Summary{MonthlyLimitUSD: 1.5, CommittedUSD: 0.4, ActualUSD: 0.1, Operations: 2}}
	e, st := newAdminEnv(t, gate)
	c := e.client()
	adminLogin(t, e, st, c, "ops.spend", adminPassword, admin.RoleOps)
	view := c.must(200, "GET", "/api/admin/v1/overview", nil).data()
	spend, _ := view["aiSpend"].(map[string]any)
	if spend["monthlyLimitUsd"] != 1.5 || spend["committedUsd"] != 0.4 {
		t.Fatalf("aiSpend = %v", spend)
	}
}
