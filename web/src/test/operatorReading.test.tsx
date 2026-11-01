import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "../i18n";
import { OperatorApp } from "../operator/OperatorApp";
import type {
  Admin,
  ReadingAdminItem,
  ReadingApproval,
  ReadingDetail,
} from "../operator/api";

const owner: Admin = {
  username: "reading.owner",
  role: "owner",
  orgId: "",
  permissions: ["ops/read", "research/review", "safety-content/review", "staff/manage"],
};

const detail: ReadingDetail = {
  id: "draft-one",
  version: 2,
  kind: "exercise",
  topics: ["stress"],
  audience: "adult",
  title: { id: "Judul Latihan", en: "Exercise title" },
  summary: { id: "Ringkasan Indonesia.", en: "English summary." },
  steps: { id: ["Langkah satu"], en: ["Step one"] },
  why: { id: "Alasan Indonesia.", en: "English reason." },
  seekHelpWhen: { id: "Cari bantuan.", en: "Seek help." },
  cautions: { id: "Perhatian.", en: "Caution." },
  readingTimeMinutes: { id: 2, en: 2 },
  source: {
    relation: "sela_original",
    locator: "Whole item",
    sources: [
      {
        id: "src-1",
        title: "Source one",
        publisher: "Publisher",
        url: "https://example.org/source",
        licence: "MIT",
      },
    ],
  },
  licence: {
    itemTextLicence: "MIT",
    sourceLicence: "MIT",
    commercialUse: "allowed",
  },
  attribution: { id: "Sela", en: "Sela" },
  adaptationNote: "Adapted for Sela.",
  evidenceNote: "Evidence note.",
  review: {
    status: "machine_drafted_unreviewed",
    reviewedOn: "",
    verifier: "not_run",
  },
};

const hash = "a".repeat(64);

function approval(revoked = false): ReadingApproval {
  return {
    slug: "draft-one",
    reviewerName: "Editor Manusia",
    reviewedOn: "2026-11-01",
    note: "read the current text",
    verifierStatus: "supported",
    contentHash: hash,
    approvedBy: "11111111-1111-4111-8111-111111111111",
    approvedAt: "2026-11-02T09:00:00Z",
    revokedAt: revoked ? "2026-11-03T09:00:00Z" : undefined,
    version: revoked ? 2 : 1,
  };
}

function item(state: "draft" | "approved" | "revoked"): ReadingAdminItem {
  const ap = state === "draft" ? undefined : approval(state === "revoked");
  return {
    slug: "draft-one",
    detail,
    contentHash: hash,
    embeddedReview: {
      status: "machine_drafted_unreviewed",
      verifierStatus: "not_run",
      verifierNote: "Verifier has not run.",
      publishable: false,
    },
    approval: ap,
    hashMatches: Boolean(ap),
    published: state === "approved",
  };
}

function reply(status: number, body: unknown): Promise<Response> {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as unknown as Response);
}

type Call = { url: string; init: RequestInit };

function stubReading(
  session: Admin,
  start: "draft" | "approved" = "draft",
): { calls: Call[]; state: { value: "draft" | "approved" | "revoked" } } {
  const calls: Call[] = [];
  const state = { value: start as "draft" | "approved" | "revoked" };
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
      const url = String(input);
      calls.push({ url, init });
      if (url.endsWith("/api/admin/v1/me")) {
        return reply(200, { data: { admin: session } });
      }
      if (url.endsWith("/api/admin/v1/reading/audit")) {
        return reply(200, { data: { events: [] } });
      }
      if (url.endsWith("/api/admin/v1/reading/draft-one/approve")) {
        state.value = "approved";
        return reply(200, {
          data: { slug: "draft-one", approval: approval() },
        });
      }
      if (url.endsWith("/api/admin/v1/reading/draft-one/revoke")) {
        state.value = "revoked";
        return reply(200, {
          data: { slug: "draft-one", approval: approval(true) },
        });
      }
      if (url.endsWith("/api/admin/v1/reading")) {
        return reply(200, { data: { items: [item(state.value)] } });
      }
      throw new Error(`unexpected request ${url}`);
    }),
  );
  return { calls, state };
}

function renderReading() {
  window.history.replaceState(null, "", "/operator/reading");
  return render(
    <I18nProvider initial="en">
      <OperatorApp />
    </I18nProvider>,
  );
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  window.history.replaceState(null, "", "/");
  window.localStorage.clear();
});

async function openItem() {
  await screen.findByRole("heading", { name: "Reading reviews" });
  const row = await screen.findByRole("button", { name: /Exercise title/ });
  fireEvent.click(row);
  await screen.findByRole("heading", { name: /Exercise title/ });
}

describe("operator reading review", () => {
  it("shows the full bilingual text and approves only with an explicit confirmation", async () => {
    const { calls } = stubReading(owner);
    const { container } = renderReading();
    await openItem();

    expect(screen.getByText("Source one")).toBeInTheDocument();
    expect(screen.getAllByText("MIT").length).toBeGreaterThan(0);
    expect(screen.getByText("English summary.")).toBeInTheDocument();
    expect(screen.getByText("Ringkasan Indonesia.")).toBeInTheDocument();
    expect(screen.getByText("Step one")).toBeInTheDocument();
    expect(screen.getByText("English reason.")).toBeInTheDocument();
    expect(screen.getAllByText("Machine draft, unreviewed").length).toBeGreaterThan(0);
    expect(screen.getByText(/Verifier has not run/)).toBeInTheDocument();
    expect(screen.getByLabelText("Reviewer name")).toHaveValue("");

    const submit = screen.getByRole("button", {
      name: "Approve for publication",
    });
    expect(submit).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Reviewer name"), {
      target: { value: "Editor Manusia" },
    });
    fireEvent.change(screen.getByLabelText("Review date"), {
      target: { value: "2026-11-01" },
    });
    fireEvent.change(screen.getByLabelText("Note (optional)"), {
      target: { value: "read it" },
    });
    expect(submit).toBeDisabled();
    fireEvent.click(screen.getByRole("checkbox"));
    expect(submit).toBeEnabled();
    fireEvent.click(submit);

    await screen.findByText(/draft-one is approved/);
    const call = calls.find((c) =>
      c.url.endsWith("/api/admin/v1/reading/draft-one/approve"),
    );
    expect(call?.init.method).toBe("POST");
    expect(JSON.parse(String(call?.init.body))).toEqual({
      reviewerName: "Editor Manusia",
      reviewedOn: "2026-11-01",
      note: "read it",
      verifierStatus: "supported",
      contentHash: hash,
      confirm: true,
    });
    expect(new Headers(call?.init.headers).get("X-Sela-Request")).toBe("1");
    await waitFor(() => expect(screen.getAllByText("Published").length).toBeGreaterThan(0));

    const results = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });
    expect(
      results.violations.filter((v) =>
        ["serious", "critical"].includes(v.impact ?? ""),
      ),
    ).toEqual([]);
  });

  it("ends publication after an inline confirmation and sends the reason", async () => {
    const { calls, state } = stubReading(owner, "approved");
    renderReading();
    await openItem();
    expect(screen.getByText("Current for this text")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "End publication" }));
    expect(
      await screen.findByText(
        "End publication of this item? The public library stops serving it immediately.",
      ),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Reason (optional)"), {
      target: { value: "withdrawn" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Yes, end publication" }));

    await screen.findByText(/is no longer publicly served/);
    expect(state.value).toBe("revoked");
    const call = calls.find((c) =>
      c.url.endsWith("/api/admin/v1/reading/draft-one/revoke"),
    );
    expect(call?.init.method).toBe("POST");
    expect(JSON.parse(String(call?.init.body))).toEqual({ reason: "withdrawn" });
    await waitFor(() => expect(screen.getAllByText("Revoked").length).toBeGreaterThan(0));
  });

  it("marks the nav entry and sends a role without a review permission to Home", async () => {
    stubReading(owner);
    renderReading();
    await screen.findByRole("heading", { name: "Reading reviews" });
    expect(
      screen.getByRole("link", { name: "Reading reviews" }),
    ).toHaveAttribute("aria-current", "page");

    cleanup();
    vi.unstubAllGlobals();
    const ops: Admin = {
      username: "ops.one",
      role: "ops",
      orgId: "",
      permissions: ["ops/read"],
    };
    const { calls } = stubReading(ops);
    renderReading();
    // A page the role may not use falls back to Home (operator shell).
    await screen.findByRole("heading", { name: "Home" });
    await waitFor(() => expect(window.location.pathname).toBe("/operator"));
    expect(
      screen.queryByRole("link", { name: "Reading reviews" }),
    ).not.toBeInTheDocument();
    expect(calls.some((c) => c.url.endsWith("/api/admin/v1/reading"))).toBe(false);
  });
});
