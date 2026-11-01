import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "../i18n";
import { OperatorApp } from "../operator/OperatorApp";
import type { Admin, AuditEvent, Overview, Staff } from "../operator/api";

const admin: Admin = {
  username: "ops.one",
  role: "ops",
  orgId: "",
  permissions: ["ops/read", "ops/cache/write"],
};

const overview: Overview = {
  activeStaff: 2,
  liveSessions: 1,
  auditLast24h: 7,
  generatedAt: "2026-10-30T09:00:00Z",
  cacheEnabled: true,
  databaseReady: true,
  version: "test",
  aiSpend: {
    monthlyLimitUsd: 1.5,
    reserveUsd: 0.25,
    monthStart: "2026-10-01T00:00:00Z",
    committedUsd: 0.4,
    actualUsd: 0.2,
    openReservations: 1,
    operations: 3,
    generatedAt: "2026-10-30T09:00:00Z",
  },
  aiSpendByTask: [
    { task: "listening_reply", committedUsd: 0.4, actualUsd: 0.2, operations: 3 },
  ],
};

const events: AuditEvent[] = [
  {
    id: "a1",
    actor: "ops.one",
    action: "admin.login",
    createdAt: "2026-10-30T09:00:00Z",
  },
  {
    id: "a2",
    actor: "ops.one",
    action: "admin.create",
    objectKind: "admin_user",
    objectId: "ops.two",
    oldStatus: "",
    newStatus: "active",
    createdAt: "2026-10-30T08:00:00Z",
  },
];

function reply(status: number, body: unknown): Promise<Response> {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as unknown as Response);
}

type Call = { url: string; init: RequestInit };

function stubApi(
  session: Admin | null,
  options: {
    loginStatus?: number;
    audit?: boolean;
    logoutStatus?: number;
    staff?: Staff[];
    directory?: unknown[];
    sandbox?: unknown[];
  } = {},
): Call[] {
  const calls: Call[] = [];
  const revoked = new Set<string>();
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
      const url = String(input);
      calls.push({ url, init });
      if (url.endsWith("/api/admin/v1/me")) {
        return session
          ? reply(200, { data: { admin: session } })
          : reply(401, { error: { code: "admin_not_signed_in" } });
      }
      if (url.endsWith("/api/admin/v1/sessions") && init.method === "POST") {
        const status = options.loginStatus ?? 200;
        return status === 200
          ? reply(200, { data: { admin: session ?? admin } })
          : reply(status, { error: { code: "admin_invalid_credentials" } });
      }
      if (url.endsWith("/api/admin/v1/sessions/current")) {
        return reply(options.logoutStatus ?? 200, { data: { ok: true } });
      }
      if (url.endsWith("/api/admin/v1/overview")) {
        return reply(200, { data: overview });
      }
      if (url.endsWith("/api/admin/v1/cache/health")) {
        return reply(200, { data: { cache: { enabled: true, ok: true, latencyMs: 3 } } });
      }
      if (url.includes("/api/admin/v1/cache/invalidate")) {
        return reply(200, { data: { invalidated: "help-content" } });
      }
      if (url.endsWith("/api/admin/v1/staff") && init.method !== "POST") {
        return reply(200, {
          data: {
            staff: (options.staff ?? []).map((s) =>
              revoked.has(s.username)
                ? { ...s, status: "revoked", liveSessions: 0 }
                : s,
            ),
          },
        });
      }
      if (url.includes("/api/admin/v1/staff/") && url.endsWith("/revoke")) {
        const rest = url.split("/staff/")[1] ?? "";
        const username = decodeURIComponent(rest.split("/")[0] ?? "");
        revoked.add(username);
        return reply(200, { data: { username, status: "revoked" } });
      }
      if (url.endsWith("/api/admin/v1/directory/audit")) {
        return reply(200, { data: { events: [] } });
      }
      if (url.endsWith("/api/admin/v1/directory") && init.method !== "POST") {
        return reply(200, { data: { entries: options.directory ?? [] } });
      }
      if (url.endsWith("/api/admin/v1/directory") && init.method === "POST") {
        return reply(201, { data: { id: "d1", status: "draft", version: 1 } });
      }
      if (url.includes("/api/admin/v1/directory/") && url.endsWith("/verify")) {
        return reply(200, { data: { id: "d1", status: "verified", version: 3 } });
      }
      if (url.endsWith("/api/admin/v1/sandbox/requests") && init.method !== "POST") {
        return reply(200, { data: { requests: options.sandbox ?? [] } });
      }
      if (url.includes("/api/admin/v1/sandbox/requests") && url.endsWith("/transition")) {
        return reply(200, { data: { id: "s1", status: "pending", version: 2 } });
      }
      if (url.includes("/api/admin/v1/audit")) {
        return reply(200, { data: { events } });
      }
      throw new Error(`unexpected request ${url}`);
    }),
  );
  return calls;
}

function renderOperator() {
  return render(
    <I18nProvider initial="en">
      <OperatorApp />
    </I18nProvider>,
  );
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  window.localStorage.clear();
});

describe("operator console", () => {
  it("signs a staff member in with password and TOTP, then loads the dashboard", async () => {
    const calls = stubApi(null);
    renderOperator();

    await screen.findByRole("heading", { name: "Sign in to the operator console" });
    fireEvent.change(screen.getByLabelText("Username"), {
      target: { value: "ops.one" },
    });
    fireEvent.change(screen.getByLabelText("Password"), {
      target: { value: "sungai biru deh" },
    });
    fireEvent.change(screen.getByLabelText("MFA code"), {
      target: { value: "123456" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    await screen.findByRole("heading", { name: "Operator console" });
    const login = calls.find(
      (c) => c.url.endsWith("/sessions") && c.init.method === "POST",
    );
    expect(login).toBeDefined();
    expect(JSON.parse(String(login?.init.body))).toEqual({
      username: "ops.one",
      password: "sungai biru deh",
      totp: "123456",
    });
    expect(new Headers(login?.init.headers).get("X-Sela-Request")).toBe("1");
    await waitFor(() =>
      expect(screen.getByText("Active staff").closest(".op-card")).toHaveTextContent("2"),
    );
    expect(screen.getByText(/\$1\.50/)).toBeInTheDocument();
  });

  it("invalidates the public cache with the mutation marker", async () => {
    const calls = stubApi(admin);
    renderOperator();

    await screen.findByRole("heading", { name: "Operator console" });
    const button = await screen.findByRole("button", {
      name: "Invalidate help cache",
    });
    fireEvent.click(button);

    await screen.findByText(/Public cache invalidated/);
    const call = calls.find((c) => c.url.endsWith("/cache/invalidate"));
    expect(call?.init.method).toBe("POST");
    expect(new Headers(call?.init.headers).get("X-Sela-Request")).toBe("1");
    expect(
      screen.queryByText("Your role isn't allowed to read the audit trail."),
    ).toBeInTheDocument();
  });

  it("keeps audit hidden without permission and cache read-only for a security admin", async () => {
    const security: Admin = {
      username: "security.one",
      role: "security_admin",
      orgId: "org-a",
      permissions: ["ops/read", "security/audit/read"],
    };
    const calls = stubApi(security);
    renderOperator();

    await screen.findByRole("heading", { name: "Operator console" });
    expect(
      screen.queryByRole("button", { name: "Invalidate help cache" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText("Your role can't invalidate the cache."),
    ).toBeInTheDocument();
    expect(await screen.findByText("Staff created")).toBeInTheDocument();
    expect(calls.some((c) => c.url.includes("/cache/invalidate"))).toBe(false);
  });

  it("shows a uniform sign-in error and keeps the values", async () => {
    stubApi(null, { loginStatus: 401 });
    renderOperator();

    await screen.findByRole("heading", { name: "Sign in to the operator console" });
    fireEvent.change(screen.getByLabelText("Username"), {
      target: { value: "ops.one" },
    });
    fireEvent.change(screen.getByLabelText("Password"), {
      target: { value: "wrong password" },
    });
    fireEvent.change(screen.getByLabelText("MFA code"), {
      target: { value: "000000" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "The username, password or MFA code doesn't match.",
    );
    expect(screen.getByLabelText("Username")).toHaveValue("ops.one");
    expect(
      screen.getByRole("heading", { name: "Sign in to the operator console" }),
    ).toBeInTheDocument();
  });

  it("signs out and returns to the login screen", async () => {
    const calls = stubApi(admin);
    renderOperator();

    await screen.findByRole("heading", { name: "Operator console" });
    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));

    await screen.findByRole("heading", { name: "Sign in to the operator console" });
    expect(
      calls.some(
        (c) => c.url.endsWith("/sessions/current") && c.init.method === "DELETE",
      ),
    ).toBe(true);
  });

  it("has no serious or critical axe findings on the dashboard", async () => {
    stubApi(admin);
    const { container } = renderOperator();
    await screen.findByRole("heading", { name: "Operator console" });
    await waitFor(() => expect(screen.getByText("Active staff")).toBeInTheDocument());
    const results = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });
    expect(
      results.violations.filter((v) =>
        ["serious", "critical"].includes(v.impact ?? ""),
      ),
    ).toEqual([]);
  });
});

const owner: Admin = {
  username: "owner.one",
  role: "owner",
  orgId: "",
  permissions: [
    "ops/read",
    "security/audit/read",
    "ops/cache/write",
    "directory/write",
    "directory/verify",
    "research/review",
    "safety-content/review",
    "staff/manage",
    "sandbox/write",
  ],
};

const staffRows: Staff[] = [
  {
    username: "owner.one",
    role: "owner",
    status: "active",
    createdAt: "2026-10-01T09:00:00Z",
    lastLoginAt: "2026-10-30T09:00:00Z",
    liveSessions: 1,
  },
  {
    username: "ops.two",
    role: "ops",
    orgId: "org-a",
    status: "active",
    createdAt: "2026-10-02T09:00:00Z",
    lastLoginAt: "2026-10-29T09:00:00Z",
    liveSessions: 2,
  },
  {
    username: "ops.gone",
    role: "ops",
    status: "revoked",
    createdAt: "2026-10-03T09:00:00Z",
    liveSessions: 0,
  },
];

describe("operator staff", () => {
  it("lists staff and revokes another operator after an inline confirmation", async () => {
    const calls = stubApi(owner, { staff: staffRows });
    renderOperator();

    await screen.findByRole("heading", { name: "Operator staff" });
    expect(await screen.findByText("ops.two")).toBeInTheDocument();
    expect(
      screen.getByText("You can't revoke your own account"),
    ).toBeInTheDocument();
    expect(screen.getByText("Access revoked")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Revoke access" }));
    expect(
      await screen.findByText("Revoke ops.two? All sessions will end."),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Yes, revoke" }));

    await screen.findByText(/ops\.two was revoked/);
    const call = calls.find((c) => c.url.includes("/staff/ops.two/revoke"));
    expect(call?.init.method).toBe("POST");
    expect(JSON.parse(String(call?.init.body))).toEqual({
      reason: "operator console",
    });
    expect(new Headers(call?.init.headers).get("X-Sela-Request")).toBe("1");
    await waitFor(() => expect(screen.getAllByText("Access revoked")).toHaveLength(2));
  });

  it("shows the last-owner guard instead of a revoke button", async () => {
    const security: Admin = {
      username: "security.one",
      role: "security_admin",
      orgId: "",
      permissions: ["ops/read", "security/audit/read", "staff/manage"],
    };
    stubApi(security, { staff: [staffRows[0]!, staffRows[1]!] });
    renderOperator();

    await screen.findByRole("heading", { name: "Operator staff" });
    expect(await screen.findByText("Last active owner")).toBeInTheDocument();
    // Only the non-owner row offers revocation.
    expect(screen.getAllByRole("button", { name: "Revoke access" })).toHaveLength(1);
  });
});

describe("operator planned care", () => {
  it("shows the sourced directory and lets a reviewer verify a pending entry", async () => {
    const directory = [
      {
        id: "d1",
        kind: "organization",
        name: "Ruang Contoh",
        city: "Bandung",
        description: "Contoh sumber.",
        sourceUrl: "https://example.org/source",
        sourcePublisher: "Sumber Contoh",
        sourceAccessedAt: "2026-10-20T00:00:00Z",
        status: "pending",
        synthetic: false,
        version: 2,
        createdAt: "2026-10-20T00:00:00Z",
        updatedAt: "2026-10-20T00:00:00Z",
      },
    ];
    const calls = stubApi(owner, { directory, sandbox: [] });
    renderOperator();
    await screen.findByRole("heading", { name: "Planned support directory" });
    expect(await screen.findByText("Ruang Contoh")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Verify" }));
    await waitFor(() =>
      expect(calls.some((c) => c.url.endsWith("/directory/d1/verify"))).toBe(true),
    );
    const call = calls.find((c) => c.url.endsWith("/directory/d1/verify"));
    expect(JSON.parse(String(call?.init.body))).toEqual({ expectedVersion: 2 });
    expect(new Headers(call?.init.headers).get("X-Sela-Request")).toBe("1");
  });

  it("labels the planned-care sandbox as a simulation and transitions it", async () => {
    const sandbox = [
      {
        id: "s1",
        organization: "Ruang Fiktif",
        professional: "Praktisi Fiktif",
        service: "Konseling",
        status: "prepared",
        synthetic: true,
        version: 1,
        createdAt: "2026-10-20T00:00:00Z",
        updatedAt: "2026-10-20T00:00:00Z",
      },
    ];
    const calls = stubApi(owner, { directory: [], sandbox });
    renderOperator();
    await screen.findByRole("heading", { name: "Planned support sandbox" });
    expect(await screen.findByText("Ruang Fiktif")).toBeInTheDocument();
    expect(screen.getAllByText("Simulation").length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole("button", { name: "Submit (simulation)" }));
    await waitFor(() => expect(calls.some((c) => c.url.endsWith("/sandbox/requests/s1/transition"))).toBe(true));
    const call = calls.find((c) => c.url.endsWith("/sandbox/requests/s1/transition"));
    expect(JSON.parse(String(call?.init.body))).toEqual({
      expectedVersion: 1,
      status: "pending",
      note: "",
    });
  });
});

describe("professional care network (D-076)", () => {
  const networkOwner: Admin = {
    username: "owner.net",
    role: "owner",
    orgId: "",
    permissions: ["ops/read", "network/manage", "network/verify"],
  };
  const practitioner: Admin = {
    username: "psy.demo",
    role: "practitioner",
    orgId: "",
    careOrgId: "org-1",
    displayName: "Psikolog Demo",
    partner: true,
    permissions: ["care-requests/read", "care-requests/content", "care-requests/respond"],
  };
  const staff: Admin = {
    ...practitioner,
    username: "desk.demo",
    role: "partner_staff",
    displayName: "Front desk",
    permissions: ["care-requests/read"],
  };
  const org = {
    id: "org-1",
    kind: "clinic",
    name: "Klinik Teduh (simulasi)",
    city: "Bandung",
    modes: ["online"],
    stages: ["support"],
    synthetic: true,
    status: "pending",
    acceptsRequests: true,
    createdAt: "2026-11-01T00:00:00Z",
    members: 1,
  };
  const request = {
    id: "r1",
    reference: "SR-ABC234",
    practitionerName: "Psikolog Demo",
    mode: "online",
    state: "sent",
    synthetic: true,
    version: 1,
    contentAvailable: true,
    expiresAt: "2026-11-09T00:00:00Z",
    createdAt: "2026-11-02T00:00:00Z",
    updatedAt: "2026-11-02T00:00:00Z",
  };
  const summary = {
    label: "Klinik Teduh (simulasi)",
    purpose: "consultation",
    summaryVersion: 1,
    title: "Rangkuman sintetis",
    text: "Teks rangkuman sintetis.",
    prepared: { goal: "Tujuan sintetis", questions: ["Pertanyaan sintetis?"] },
    items: [{ text: "Pernyataan sintetis", quote: "Kutipan sintetis", sourceLabel: "Catatan · v1", position: 0 }],
    sources: ["Catatan · v1"],
    createdAt: "2026-11-02T00:00:00Z",
    expiresAt: "2026-11-09T00:00:00Z",
  };

  function stubNetwork(session: Admin): Call[] {
    const calls: Call[] = [];
    let currentOrg = { ...org };
    let members = [
      { username: "psy.demo", role: "practitioner", displayName: "Psikolog Demo", professionalType: "psychologist", licenseNo: "STR-SIM-1", licenseVerified: false, status: "active" },
    ];
    let current = { ...request } as Record<string, unknown>;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
        const url = String(input);
        calls.push({ url, init });
        const method = init.method ?? "GET";
        if (url.endsWith("/api/admin/v1/me")) return reply(200, { data: { admin: session } });
        if (url.endsWith("/api/admin/v1/overview")) return reply(200, { data: overview });
        if (url.endsWith("/api/admin/v1/cache/health")) return reply(200, { data: { cache: { enabled: false, ok: true } } });
        if (url.endsWith("/api/admin/v1/network/orgs") && method === "GET") {
          return reply(200, { data: { orgs: [currentOrg], realPartners: false } });
        }
        if (url.endsWith("/api/admin/v1/network/orgs") && method === "POST") {
          return reply(201, { data: { org: { ...org, id: "org-2", name: "Praktik Baru" } } });
        }
        if (url.endsWith("/api/admin/v1/network/orgs/org-1/status")) {
          currentOrg = { ...currentOrg, status: "verified" };
          return reply(200, { data: { org: currentOrg } });
        }
        if (url.endsWith("/api/admin/v1/network/orgs/org-1/members")) return reply(200, { data: { members } });
        if (url.endsWith("/api/admin/v1/network/members/psy.demo/verify-license")) {
          members = members.map((m) => ({ ...m, licenseVerified: true }));
          return reply(200, { data: { username: "psy.demo", licenseVerified: true } });
        }
        if (url.endsWith("/api/admin/v1/partner/org")) {
          return reply(200, { data: { org: { ...org, status: "verified" }, displayName: session.displayName } });
        }
        if (url.endsWith("/api/admin/v1/partner/requests")) return reply(200, { data: { requests: [current] } });
        if (url.endsWith("/api/admin/v1/partner/requests/r1")) {
          const content = session.permissions.includes("care-requests/content");
          return reply(200, { data: { request: content ? { ...current, summary } : current, canReadContent: content } });
        }
        if (url.endsWith("/api/admin/v1/partner/requests/r1/respond")) {
          const body = JSON.parse(String(init.body));
          current = { ...current, state: "proposed", version: 2, proposal: body.proposal };
          return reply(200, { data: { request: current } });
        }
        throw new Error(`unexpected request ${method} ${url}`);
      }),
    );
    return calls;
  }

  it("lets an internal owner create and verify a simulated clinic and verify a licence", async () => {
    const calls = stubNetwork(networkOwner);
    renderOperator();
    await screen.findByRole("heading", { name: "Professional network" });
    expect(await screen.findByText("Klinik Teduh (simulasi)")).toBeInTheDocument();
    expect(screen.getByText(/allows simulated organisations only/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Add organisation" }));
    const simulated = screen.getByRole("checkbox", { name: /Simulated organisation/ });
    expect(simulated).toBeChecked();
    expect(simulated).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Organisation name"), { target: { value: "Praktik Baru" } });
    fireEvent.click(screen.getByRole("button", { name: "Create organisation" }));
    await waitFor(() => expect(calls.some((c) => c.url.endsWith("/network/orgs") && c.init.method === "POST")).toBe(true));
    const created = calls.find((c) => c.url.endsWith("/network/orgs") && c.init.method === "POST");
    expect(JSON.parse(String(created?.init.body))).toMatchObject({ kind: "clinic", name: "Praktik Baru", modes: ["online"], synthetic: true });
    expect(new Headers(created?.init.headers).get("X-Sela-Request")).toBe("1");

    fireEvent.click(await screen.findByRole("button", { name: "Verify" }));
    await waitFor(() => expect(calls.some((c) => c.url.endsWith("/network/orgs/org-1/status"))).toBe(true));
    expect(JSON.parse(String(calls.find((c) => c.url.endsWith("/status"))?.init.body))).toEqual({ status: "verified" });

    fireEvent.click(screen.getByRole("button", { name: "Show accounts" }));
    expect(await screen.findByText(/STR-SIM-1/)).toBeInTheDocument();
    expect(screen.getByText(/--org org-1/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Verify licence" }));
    await waitFor(() => expect(calls.some((c) => c.url.endsWith("/network/members/psy.demo/verify-license"))).toBe(true));
    expect(await screen.findByText(/verified$/)).toBeInTheDocument();
  });

  it("shows a practitioner only the workspace, the shared summary and a proposal form", async () => {
    const calls = stubNetwork(practitioner);
    renderOperator();
    await screen.findByRole("heading", { name: "Professional workspace" });
    expect(screen.queryByRole("heading", { name: "Overview" })).not.toBeInTheDocument();
    expect(calls.some((c) => c.url.endsWith("/overview"))).toBe(false);
    expect(await screen.findByText("Klinik Teduh (simulasi)")).toBeInTheDocument();

    fireEvent.click(await screen.findByRole("button", { name: /SR-ABC234/ }));
    expect(await screen.findByRole("heading", { name: "Rangkuman sintetis" })).toBeInTheDocument();
    expect(screen.getByText("Kutipan sintetis")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Time 1"), { target: { value: "2026-11-05T10:00" } });
    fireEvent.change(screen.getByLabelText("How to reach the clinic"), { target: { value: "WhatsApp klinik, sebut SR-ABC234" } });
    fireEvent.click(screen.getByRole("button", { name: "Send proposal" }));
    await waitFor(() => expect(calls.some((c) => c.url.endsWith("/partner/requests/r1/respond"))).toBe(true));
    const sent = JSON.parse(String(calls.find((c) => c.url.endsWith("/respond"))?.init.body));
    expect(sent.decision).toBe("propose");
    expect(sent.expectedVersion).toBe(1);
    expect(sent.proposal.mode).toBe("online");
    expect(sent.proposal.contact).toBe("WhatsApp klinik, sebut SR-ABC234");
    expect(sent.proposal.slots).toEqual([new Date("2026-11-05T10:00").toISOString()]);
    expect(await screen.findByText(/Proposed times sent/)).toBeInTheDocument();
  });

  it("has no serious or critical axe findings in the partner workspace", async () => {
    stubNetwork(practitioner);
    const { container } = renderOperator();
    fireEvent.click(await screen.findByRole("button", { name: /SR-ABC234/ }));
    await screen.findByRole("heading", { name: "Rangkuman sintetis" });
    const results = await axe.run(container, { rules: { "color-contrast": { enabled: false } } });
    expect(results.violations.filter((v) => ["serious", "critical"].includes(v.impact ?? "")).map((v) => v.id)).toEqual([]);
  });

  it("never shows clinic staff the summary or a respond form", async () => {
    stubNetwork(staff);
    renderOperator();
    fireEvent.click(await screen.findByRole("button", { name: /SR-ABC234/ }));
    expect(await screen.findByText(/only for practitioners and clinic administrators/)).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Rangkuman sintetis" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Send proposal" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Decline request" })).not.toBeInTheDocument();
  });
});
