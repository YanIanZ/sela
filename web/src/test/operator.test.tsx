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
