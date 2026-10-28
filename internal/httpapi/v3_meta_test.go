package httpapi_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/db/dbtest"
	"github.com/YanIanZ/sela/internal/fixtures"
	"github.com/YanIanZ/sela/internal/httpapi"
)

func fillPath(p string) string {
	return strings.NewReplacer("{id}", "6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11", "{purpose}", "terms_privacy").Replace(p)
}

func TestV3MutationGuardAllRoutes(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	routes := httpapi.V3RouteTable(e.h)
	mutations := 0
	for _, rt := range routes {
		if !rt.Mutation {
			if rt.Method != "GET" {
				t.Errorf("%s %s is not guarded", rt.Method, rt.Path)
			}
			continue
		}
		mutations++
		path := fillPath(rt.Path)
		for name, hdr := range map[string][]string{
			"missing X-Sela-Request": {"X-Sela-Request", ""},
			"foreign Origin":         {"Origin", "https://evil.example"},
			"cross-site fetch":       {"Sec-Fetch-Site", "cross-site"},
			"same-site fetch":        {"Sec-Fetch-Site", "same-site"},
		} {
			r := e.client().call(rt.Method, path, map[string]any{}, hdr...)
			if r.status != 403 || r.code() != "cross_origin" {
				t.Errorf("%s %s with %s: %d %s", rt.Method, rt.Path, name, r.status, r.raw)
			}
		}
		if rt.Method != "DELETE" {
			r := e.client().call(rt.Method, path, "username=x", "Content-Type", "text/plain")
			if r.status != 415 || r.code() != "unsupported_media" {
				t.Errorf("%s %s with a non-JSON body: %d %s", rt.Method, rt.Path, r.status, r.raw)
			}
		}
		// Over the 16 KiB body limit.
		big := map[string]any{"username": strings.Repeat("a", 17<<10)}
		if r := e.client().call(rt.Method, path, big); r.status != 413 && r.status != 401 {
			t.Errorf("%s %s with a 17 KiB body: %d", rt.Method, rt.Path, r.status)
		}
	}
	if mutations < 17 {
		t.Fatalf("only %d mutation routes", mutations)
	}
}

// specRoutes parses API_SPEC_V3 §4-§6 for "METHOD /api/v3/..." routes.
func specRoutes(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("../../docs/API_SPEC_V3.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	start, end := strings.Index(doc, "\n## 4."), strings.Index(doc, "\n## 7.")
	if start < 0 || end < start {
		t.Fatal("API_SPEC_V3.md sections 4-6 not found")
	}
	re := regexp.MustCompile("`(GET|POST|PUT|PATCH|DELETE) (/api/v3/[^`?\\s]*)")
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(doc[start:end], -1) {
		seen[m[1]+" "+m[2]] = true
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestV3RoutesMatchSpec(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{noStore: true})
	spec := specRoutes(t)
	if len(spec) < 25 {
		t.Fatalf("parsed only %d routes from the spec: %v", len(spec), spec)
	}
	var registered []string
	for _, rt := range httpapi.V3RouteTable(e.h) {
		registered = append(registered, rt.Method+" "+rt.Path)
	}
	sort.Strings(registered)
	if !slices.Equal(spec, registered) {
		for _, s := range spec {
			if !slices.Contains(registered, s) {
				t.Errorf("in the spec, not registered: %s", s)
			}
		}
		for _, r := range registered {
			if !slices.Contains(spec, r) {
				t.Errorf("registered, not in the spec: %s", r)
			}
		}
	}
	// Each one is really served with its method: no 404 or 405.
	for _, s := range spec {
		method, path, _ := strings.Cut(s, " ")
		var body any
		if method != "GET" {
			body = map[string]any{}
		}
		r := e.client().call(method, fillPath(path), body)
		if r.status == 404 || r.status == 405 {
			t.Errorf("%s: %d", s, r.status)
		}
	}
	// Anything else under /api/v3 is a JSON 404; a wrong method a 405.
	for _, p := range []string{"/api/v3/admin", "/api/v3/me/secrets", "/api/v3/guardian", "/api/v3/auth/token"} {
		if r := e.client().call("GET", p, nil); r.status != 404 || r.code() != "not_found" {
			t.Errorf("%s: %d", p, r.status)
		}
	}
	if r := e.client().call("PUT", "/api/v3/auth/login", map[string]any{}); r.status != 405 || r.header.Get("Allow") != "POST" {
		t.Errorf("wrong method: %d %q", r.status, r.header.Get("Allow"))
	}
}

func TestV3ErrorMessagesComplete(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../docs/API_SPEC_V3.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	sec := doc[strings.Index(doc, "## 2. Error codes"):strings.Index(doc, "## 3. Shared objects")]
	documented := map[string]int{}
	for _, m := range regexp.MustCompile("(?m)^\\| `([a-z_]+)` \\| (\\d{3}) \\|").FindAllStringSubmatch(sec, -1) {
		var n int
		fmt.Sscan(m[2], &n)
		documented[m[1]] = n
	}
	for _, m := range regexp.MustCompile("`([a-z_]+)` (\\d{3})").FindAllStringSubmatch(strings.SplitN(sec, "\n\n", 3)[1], -1) {
		var n int
		fmt.Sscan(m[2], &n)
		documented[m[1]] = n
	}
	if len(documented) < 28 {
		t.Fatalf("parsed %d codes: %v", len(documented), documented)
	}
	for code, status := range documented {
		id, en, ok := httpapi.Message(code)
		if !ok || strings.TrimSpace(id) == "" || strings.TrimSpace(en) == "" || id == en {
			t.Errorf("%s: messages %q / %q", code, id, en)
		}
		if got := httpapi.V3Status(code); got != status {
			t.Errorf("%s: status %d, documented %d", code, got, status)
		}
	}
	// Every code the account store can return is documented.
	for _, code := range account.Codes() {
		if _, ok := documented[code]; !ok {
			t.Errorf("account code %s is not in API_SPEC_V3 §2", code)
		}
	}
	// The messages are served in both languages.
	e := newV3(t, v3opt{noStore: true})
	for lang, want := range map[string]string{"id": "Penyimpanan sedang tidak tersedia", "en": "Storage is unavailable right now"} {
		c := e.client()
		c.lang = lang
		if r := c.call("GET", "/api/v3/me", nil); !strings.Contains(string(r.raw), want) {
			t.Errorf("%s: %s", lang, r.raw)
		}
	}
}

func TestReadyReportsDatabase(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		database, schema string
		required         bool
		status           int
		state            string
	}{
		{"current", "ok", "current", true, 200, "ready"},
		{"required and unavailable", "unavailable", "not_configured", true, 503, "not_ready"},
		{"required and pending", "ok", "pending", true, 503, "not_ready"},
		{"required and mismatched", "ok", "mismatch", true, 503, "not_ready"},
		{"optional and unavailable", "unavailable", "not_configured", false, 200, "ready"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newV3(t, v3opt{noStore: true, mutateO: func(o *httpapi.Options) {
				o.Database = func(context.Context) (string, string) { return tc.database, tc.schema }
				o.DatabaseRequired = tc.required
			}})
			r := e.client().call("GET", "/api/v2/ready", nil)
			d := r.data()
			if r.status != tc.status || d["status"] != tc.state || d["database"] != tc.database || d["schema"] != tc.schema {
				t.Fatalf("%d %s", r.status, r.raw)
			}
			for _, k := range []string{"fixtures", "ai", "research", "frontend", "store", "persistent"} {
				if _, ok := d[k]; !ok {
					t.Errorf("ready lost field %s", k)
				}
			}
		})
	}
	e := newV3(t, v3opt{noStore: true})
	r := e.client().call("GET", "/api/v2/ready", nil)
	if d := r.data(); r.status != 200 || d["database"] != "not_configured" || d["schema"] != "not_configured" || d["store"] != "memory" {
		t.Fatalf("not configured: %s", r.raw)
	}
	h := e.client().call("GET", "/api/v2/health", nil).data()
	var keys []string
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !slices.Equal(keys, []string{"mode", "status", "uptimeSeconds", "version"}) || h["status"] != "ok" {
		t.Fatalf("health changed: %v", h)
	}
	// Staging and production do not serve the v2 journey store.
	off := newV3(t, v3opt{noStore: true, mutateO: func(o *httpapi.Options) { o.DisableV2Journey = true }})
	if d := off.client().call("GET", "/api/v2/ready", nil).data(); d["store"] != "disabled" {
		t.Fatalf("store: %v", d["store"])
	}
	if r := off.client().call("POST", "/api/v2/sessions", map[string]any{}); r.status != 404 {
		t.Fatalf("v2 journey served: %d", r.status)
	}
	if r := off.client().call("GET", "/api/v2/health", nil); r.status != 200 {
		t.Fatal("health")
	}
}

func TestV3StoreUnavailable(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{noStore: true})
	c := e.client()
	if r := c.must(200, "GET", "/api/v3/policies", nil); len(r.data()["policies"].(map[string]any)) != len(fixtures.V3PolicyVersions()) {
		t.Fatalf("policies: %s", r.raw)
	}
	if o := c.must(200, "GET", "/api/v3/auth/options", nil).data(); o["guest"].(map[string]any)["available"] != false {
		t.Fatalf("options: %v", o)
	}
	for _, rt := range httpapi.V3RouteTable(e.h) {
		if rt.Public {
			continue
		}
		var body any
		if rt.Mutation {
			body = map[string]any{}
		}
		r := c.call(rt.Method, fillPath(rt.Path), body)
		if r.status != 503 || r.code() != "store_unavailable" || r.header.Get("Retry-After") == "" {
			t.Errorf("%s %s: %d %s", rt.Method, rt.Path, r.status, r.raw)
		}
	}
	// The mutation guard still comes first.
	if r := c.call("POST", "/api/v3/auth/register", map[string]any{}, "Origin", "https://evil.example"); r.status != 403 {
		t.Fatalf("guard: %d", r.status)
	}
}

func TestV3PoliciesAndOptions(t *testing.T) {
	t.Parallel()
	e := newV3(t, v3opt{})
	p := e.client().must(200, "GET", "/api/v3/policies", nil).data()["policies"].(map[string]any)
	for _, k := range []string{"terms_privacy", "guest_storage", "guardian_information"} {
		pol := p[k].(map[string]any)
		if pol["version"] != policyVersion(k) || len(pol["body"].([]any)) == 0 || pol["title"].(map[string]any)["en"] == "" {
			t.Errorf("%s: %v", k, pol)
		}
	}
	if g := fmt.Sprint(p["guest_storage"]); !strings.Contains(g, "7 hari") || !strings.Contains(g, "30 days") || strings.Contains(g, "{inactive}") {
		t.Fatalf("guest notice not rendered: %s", g)
	}
	o := e.client().must(200, "GET", "/api/v3/auth/options", nil).data()
	want := "map[ageBands:map[allowed:[15_17 18_plus unknown]] environment:test guardianConsentRequired:true " +
		"guest:map[available:true inactiveTtlHours:168 maxAgeDays:30] password:map[maxLength:256 minLength:15] " +
		"recoveryCodes:map[count:10 groupLength:4 groups:4] sessions:map[absoluteHours:12 idleHours:2 trustedAbsoluteDays:30 trustedIdleDays:7] " +
		"synthetic:true username:map[maxLength:32 minLength:3]]"
	if got := fmt.Sprint(o); got != want {
		t.Fatalf("options:\n got %s\nwant %s", got, want)
	}
}

// TestV3LogsContainNoSecrets runs every v3 flow and checks the captured
// logs for the markers.
func TestV3LogsContainNoSecrets(t *testing.T) {
	t.Parallel()
	const marker = "penanda.rahasia"
	const guardianName = "Ibu Penanda Rahasia"
	e := newV3(t, v3opt{})
	var tokens []string
	keep := func(c *v3client) {
		if tok := c.token(); tok != "" {
			tokens = append(tokens, tok)
		}
	}
	a := e.client()
	reg := a.register(marker, map[string]any{"ageBand": "15_17"})
	keep(a)
	codes := strs(reg.data()["recoveryCodes"])
	id := userID(t, reg)
	sessions := []string{sessionID(t, reg)}
	e.client().login(marker, "kata sandi yang salah sekali")
	b := e.client()
	sessions = append(sessions, sessionID(t, b.login(marker, v3Password)))
	keep(b)
	a.must(200, "PUT", "/api/v3/me/guardian-consent", map[string]any{"relationship": "parent", "method": "guardian_present_attestation",
		"guardianName": guardianName, "policyVersion": policyVersion("guardian_information")})
	a.must(200, "GET", "/api/v3/me/guardian-consent", nil)
	a.must(200, "PATCH", "/api/v3/me", map[string]any{"expectedVersion": 1, "locale": "en"})
	a.must(200, "PUT", "/api/v3/me/consents/test_optional", map[string]any{"granted": true, "policyVersion": "optional-test.1"})
	a.must(200, "GET", "/api/v3/me/consents/history", nil)
	a.must(200, "GET", "/api/v3/me/audit", nil)
	a.must(200, "GET", "/api/v3/me/sessions", nil)
	a.must(200, "POST", "/api/v3/auth/reauth", map[string]any{"password": v3Password})
	a.must(200, "POST", "/api/v3/me/recovery-codes/acknowledge", nil)
	codes = append(codes, strs(a.must(200, "POST", "/api/v3/auth/recovery-codes", map[string]any{"password": v3Password}).data()["recoveryCodes"])...)
	for _, f := range []string{"json", "html", "md"} {
		a.must(200, "GET", "/api/v3/me/export?format="+f, nil)
	}
	a.must(200, "DELETE", "/api/v3/me/sessions/"+sessions[1], nil)
	a.must(200, "POST", "/api/v3/auth/password", map[string]any{"currentPassword": v3Password, "newPassword": v3NewPassword})
	keep(a)
	rc := e.client()
	rc.must(200, "POST", "/api/v3/auth/recovery", map[string]any{"username": marker, "recoveryCode": codes[10], "newPassword": v3Password})
	keep(rc)
	rc.call("POST", "/api/v3/auth/recovery", map[string]any{"username": marker, "recoveryCode": codes[11] + "X", "newPassword": v3Password})
	rc.must(200, "POST", "/api/v3/auth/logout-all", map[string]any{"keepCurrent": true})
	g := e.client()
	g.guest("18_plus")
	keep(g)
	g.must(200, "POST", "/api/v3/auth/guest/upgrade", map[string]any{"username": marker + ".dua", "password": v3NewPassword})
	keep(g)
	g.must(200, "POST", "/api/v3/auth/logout", nil)
	rc.must(200, "DELETE", "/api/v3/me", map[string]any{"confirm": true})

	logs := e.logs.String()
	if !strings.Contains(logs, `"msg":"auth.event"`) || !strings.Contains(logs, `"outcome":"invalid"`) || !strings.Contains(logs, `"outcome":"success"`) {
		t.Fatalf("auth outcomes not logged: %s", logs)
	}
	secrets := map[string]string{"username": marker, "password": v3Password, "new password": v3NewPassword, "guardian name": guardianName,
		"user id": id.String(), "database URL": e.db.URL, "test database URL": os.Getenv(dbtest.EnvURL), "KEK": base64.StdEncoding.EncodeToString(e.kek.Secret)}
	for i, s := range sessions {
		secrets[fmt.Sprintf("session id %d", i)] = s
	}
	for i, c := range codes {
		n, _ := account.NormalizeRecoveryCode(c)
		secrets[fmt.Sprintf("code %d", i)] = c
		secrets[fmt.Sprintf("normalised code %d", i)] = n
	}
	for i, tok := range tokens {
		h := sha256.Sum256([]byte(tok))
		secrets[fmt.Sprintf("token %d", i)] = tok
		secrets[fmt.Sprintf("token hash %d", i)] = hex.EncodeToString(h[:])
		secrets[fmt.Sprintf("token hash b64 %d", i)] = base64.StdEncoding.EncodeToString(h[:])
	}
	for what, v := range secrets {
		if v != "" && strings.Contains(logs, v) {
			t.Errorf("logs contain the %s", what)
		}
	}
	if len(tokens) < 5 {
		t.Fatalf("only %d tokens captured", len(tokens))
	}
}
