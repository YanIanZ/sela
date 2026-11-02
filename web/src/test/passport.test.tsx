import { readFileSync } from "node:fs";
import { join } from "node:path";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { I18nProvider } from "../i18n";
import { RouterProvider } from "../lib/router";
import { DesignPreviewSession } from "../v3/session";
import { PassportPage } from "../v3/screens/Passport";

// Synthetic Care Passport fixtures; none is real data.
const noteA = {
  id: "n1",
  kind: "note",
  title: "Catatan A",
  text: "Kadang aku ingin sendiri dulu. Wulan Prameswari bilang itu wajar.",
  version: 1,
  provenance: "user_original",
  reviewStatus: "confirmed",
  safety: "ok",
  ruleset: "r1",
  createdAt: "2026-11-01T09:00:00Z",
  updatedAt: "2026-11-01T09:00:00Z",
};

function summary(over: Record<string, unknown> = {}) {
  return {
    id: "s1",
    kind: "summary",
    title: "Persiapan sintetis",
    text: "",
    version: 1,
    provenance: "user_original",
    reviewStatus: "confirmed",
    safety: "ok",
    ruleset: "r1",
    summaryState: "draft",
    sourceRefs: [{ recordId: "n1", version: 1 }],
    prepared: { goal: "Aku ingin bisa menjelaskan rasa capekku.", questions: "Apa yang dibahas dulu?\nBagaimana cerita ke Wulan Prameswari?", bring: "" },
    items: [
      {
        id: "i1",
        text: "Temanku menganggapnya wajar.",
        quote: "Wulan Prameswari bilang itu wajar.",
        sourceRef: { recordId: "n1", version: 1 },
        provenance: "user_original",
        reviewStatus: "confirmed",
        position: 0,
      },
    ],
    createdAt: "2026-11-02T09:00:00Z",
    updatedAt: "2026-11-02T09:00:00Z",
    ...over,
  };
}

function passport(over: Record<string, unknown> = {}) {
  return {
    summaryId: "s1",
    summaryVersion: 1,
    state: "draft",
    shareable: false,
    sources: [{ recordId: "n1", version: 1, kind: "note", label: "Catatan A", current: true }],
    recipient: {
      summaryVersion: 1,
      title: "Persiapan sintetis",
      text: "",
      prepared: { goal: "Aku ingin bisa menjelaskan rasa capekku.", questions: "Apa yang dibahas dulu?\nBagaimana cerita ke [NAMA-1]?", bring: "" },
      items: [{ text: "Temanku menganggapnya wajar.", quote: "[NAMA-1] bilang itu wajar.", sourceLabel: "Catatan A · v1", position: 0 }],
      sources: ["Catatan A · v1"],
    },
    updatedAt: "2026-11-02T09:00:00Z",
    ...over,
  };
}

type Reply = { status?: number; data?: unknown; error?: { code: string } };
type Route = (url: string, method: string, body: unknown) => Reply | undefined;

function mockApi(route: Route) {
  const calls: { url: string; method: string; body: unknown }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      const body = init?.body ? JSON.parse(String(init.body)) : undefined;
      calls.push({ url, method, body });
      const reply = route(url, method, body) ?? { data: {} };
      const status = reply.status ?? 200;
      return {
        ok: status < 400,
        status,
        json: async () => (reply.error ? { error: reply.error } : { data: reply.data }),
      } as unknown as Response;
    }),
  );
  return calls;
}

function renderAt(path: string) {
  window.history.replaceState(null, "", path);
  return render(
    <I18nProvider initial="id">
      <RouterProvider>
        <DesignPreviewSession profile={null}>
          <PassportPage path={path} />
        </DesignPreviewSession>
      </RouterProvider>
    </I18nProvider>,
  );
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

it("lists summaries with worded statuses, never colour alone", async () => {
  mockApi((url) => {
    if (url.endsWith("/consultation-summaries"))
      return {
        data: {
          summaries: [
            summary({ id: "d1", title: "Draf sintetis" }),
            summary({ id: "a1", title: "Disetujui sintetis", version: 2, summaryState: "approved", approvedVersion: 2 }),
            summary({ id: "x1", title: "Basi sintetis", stale: { reason: "source_edited", eventId: "e1", at: "2026-11-02T10:00:00Z" } }),
            summary({ id: "y1", title: "Turun sintetis", version: 3, approvedVersion: 3 }),
          ],
        },
      };
    if (url.endsWith("/shares"))
      return { data: { shares: [{ id: "sh1", summaryId: "a1", summaryVersion: 2, label: "Konselor", state: "revoked", endReason: "owner_revoked" }] } };
    return undefined;
  });
  renderAt("/support/passport");
  const draft = (await screen.findByRole("heading", { name: "Draf sintetis" })).closest("li")!;
  const approved = screen.getByRole("heading", { name: "Disetujui sintetis" }).closest("li")!;
  const stale = screen.getByRole("heading", { name: "Basi sintetis" }).closest("li")!;
  const demoted = screen.getByRole("heading", { name: "Turun sintetis" }).closest("li")!;
  expect(within(draft).getByText("Draf · v1")).toBeInTheDocument();
  expect(within(approved).getByText("Versi disetujui · v2")).toBeInTheDocument();
  expect(within(approved).getByText("Berbagi sudah berakhir")).toBeInTheDocument();
  for (const card of [stale, demoted]) {
    expect(within(card).getByText("Perlu ditinjau ulang")).toBeInTheDocument();
    expect(within(card).getByRole("link", { name: "Tinjau ulang" })).toHaveAttribute("href", expect.stringMatching(/\/support\/summaries\/(x1|y1)$/));
  }
  // Every status chip carries an icon and words.
  for (const chip of document.querySelectorAll(".c-passport-status")) {
    expect(chip.querySelector("svg")).not.toBeNull();
    expect(chip.textContent?.trim().length).toBeGreaterThan(3);
  }
  expect(screen.getByText("Bukan paspor medis")).toBeInTheDocument();
});

it("previews exactly the recipient projection and approves the exact version", async () => {
  let approved = false;
  const calls = mockApi((url, method) => {
    if (url.endsWith("/consultation-summaries/s1/approve") && method === "POST") {
      approved = true;
      return { data: summary({ summaryState: "approved", approvedVersion: 1 }) };
    }
    if (url.endsWith("/consultation-summaries/s1/passport"))
      return { data: { passport: approved ? passport({ state: "approved", shareable: true, approvedVersion: 1 }) : passport() } };
    if (url.endsWith("/consultation-summaries/s1"))
      return { data: approved ? summary({ summaryState: "approved", approvedVersion: 1 }) : summary() };
    if (url.endsWith("/records")) return { data: { records: [noteA] } };
    if (url.endsWith("/shares")) return { data: { shares: [] } };
    if (url.endsWith("/care-requests")) return { data: { requests: [] } };
    return undefined;
  });
  renderAt("/support/passport/s1");
  expect(await screen.findByText("Draf · v1")).toBeInTheDocument();
  for (const name of ["Tujuanku", "Yang ingin kubagikan", "Dengan kata-kataku", "Yang ingin kutanyakan", "Yang diterima penerima"]) {
    expect(screen.getByRole("heading", { level: 2, name })).toBeInTheDocument();
  }
  // The recipient preview shows the masked projection, the owner view the original.
  const preview = screen.getByRole("article", { name: "Pratinjau penerima" });
  expect(within(preview).getByText("[NAMA-1] bilang itu wajar.")).toBeInTheDocument();
  expect(within(preview).queryByText(/Wulan/)).toBeNull();
  expect(screen.getByText(/Untuk penerima, nama pribadi disamarkan/)).toBeInTheDocument();
  expect(await screen.findByRole("link", { name: "Periksa sumber" })).toHaveAttribute("href", "/notes/n1");
  expect(screen.queryByRole("button", { name: /Unduh HTML/ })).toBeNull();

  const approve = screen.getByRole("button", { name: "Setujui versi ini" });
  expect(approve).toBeDisabled();
  fireEvent.click(screen.getByRole("checkbox", { name: /memeriksa versi 1/ }));
  fireEvent.click(approve);
  await waitFor(() => expect(calls.some((c) => c.url.endsWith("/approve"))).toBe(true));
  const post = calls.find((c) => c.url.endsWith("/approve"))!;
  expect(post.body).toEqual({ expectedVersion: 1 });
  expect(await screen.findByText("Versi disetujui · v1")).toBeInTheDocument();
  expect(screen.getByText("Versi 1 disetujui.")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /Unduh HTML/ })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Bagikan dengan tautan dan kode" })).toHaveAttribute("href", "/support/shares?summary=s1");
});

it("reports a version conflict instead of a false success", async () => {
  mockApi((url, method) => {
    if (url.endsWith("/approve") && method === "POST") return { status: 409, error: { code: "stale_version" } };
    if (url.endsWith("/consultation-summaries/s1/passport")) return { data: { passport: passport() } };
    if (url.endsWith("/consultation-summaries/s1")) return { data: summary() };
    if (url.endsWith("/records")) return { data: { records: [noteA] } };
    return { data: { shares: [], requests: [] } };
  });
  renderAt("/support/passport/s1");
  fireEvent.click(await screen.findByRole("checkbox", { name: /memeriksa versi 1/ }));
  fireEvent.click(screen.getByRole("button", { name: "Setujui versi ini" }));
  expect(await screen.findByText(/Versi ini sudah berubah sejak kamu membukanya/)).toBeInTheDocument();
  expect(screen.queryByText("Versi 1 disetujui.")).toBeNull();
  expect(screen.queryByText(/Versi disetujui/)).toBeNull();
});

it("blocks a stale passport and routes to review", async () => {
  mockApi((url) => {
    if (url.endsWith("/consultation-summaries/s1/passport"))
      return {
        data: {
          passport: passport({
            state: "needs_review",
            shareable: false,
            approvedVersion: 1,
            stale: { reason: "source_edited", eventId: "e1", at: "2026-11-02T10:00:00Z" },
            sources: [{ recordId: "n1", version: 1, kind: "note", label: "Catatan A", current: false }],
          }),
        },
      };
    if (url.endsWith("/consultation-summaries/s1"))
      return { data: summary({ approvedVersion: 1, stale: { reason: "source_edited", eventId: "e1", at: "2026-11-02T10:00:00Z" } }) };
    if (url.endsWith("/records")) return { data: { records: [{ ...noteA, version: 2 }] } };
    return { data: { shares: [], requests: [] } };
  });
  renderAt("/support/passport/s1");
  expect((await screen.findAllByText("Perlu ditinjau ulang")).length).toBeGreaterThan(0);
  expect(screen.getByRole("link", { name: "Tinjau ulang ringkasan" })).toHaveAttribute("href", "/support/summaries/s1");
  expect(await screen.findByText(/Sumber sudah berubah sejak dipilih/)).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Setujui versi ini" })).toBeNull();
  expect(screen.queryByRole("button", { name: /Unduh/ })).toBeNull();
  expect(screen.queryByRole("link", { name: "Bagikan dengan tautan dan kode" })).toBeNull();
});

it("labels a synthetic care partner as SIMULASI", async () => {
  mockApi((url) => {
    if (url.endsWith("/consultation-summaries/s1/passport"))
      return { data: { passport: passport({ state: "approved", shareable: true, approvedVersion: 1 }) } };
    if (url.endsWith("/consultation-summaries/s1")) return { data: summary({ summaryState: "approved", approvedVersion: 1 }) };
    if (url.endsWith("/records")) return { data: { records: [noteA] } };
    if (url.endsWith("/shares")) return { data: { shares: [] } };
    if (url.endsWith("/care-requests"))
      return {
        data: {
          requests: [
            { id: "c1", summaryId: "s1", summaryVersion: 1, orgName: "Klinik Paspor", orgSynthetic: true, state: "sent" },
            { id: "c2", summaryId: "other", summaryVersion: 1, orgName: "Lain", orgSynthetic: false, state: "sent" },
          ],
        },
      };
    return undefined;
  });
  renderAt("/support/passport/s1");
  expect(await screen.findByText("Klinik Paspor")).toBeInTheDocument();
  expect(screen.getAllByText("SIMULASI — penerima fiktif; tidak ada konsultasi sungguhan terjadi.").length).toBeGreaterThanOrEqual(2);
  expect(screen.queryByText("Lain")).toBeNull();
  expect(screen.getByText(/tidak berarti janji temu sudah dipesan/)).toBeInTheDocument();
});

it("keeps markup and styles safe at 320px", async () => {
  mockApi((url) => {
    if (url.endsWith("/consultation-summaries/s1/passport")) return { data: { passport: passport() } };
    if (url.endsWith("/consultation-summaries/s1")) return { data: summary() };
    if (url.endsWith("/records")) return { data: { records: [noteA] } };
    return { data: { shares: [], requests: [] } };
  });
  const { container } = renderAt("/support/passport/s1");
  await screen.findByText("Draf · v1");
  // No tables, no fixed inline widths, every form control labelled.
  expect(container.querySelector("table")).toBeNull();
  for (const el of container.querySelectorAll<HTMLElement>("[style]")) {
    expect(el.getAttribute("style") ?? "").not.toMatch(/(min-)?width\s*:\s*\d{3,}px/);
  }
  for (const input of container.querySelectorAll("input, textarea, select")) {
    expect(input.id && container.querySelector(`label[for="${input.id}"]`)).toBeTruthy();
  }
  const css = readFileSync(join(__dirname, "../v3/passport.css"), "utf8");
  expect(css).not.toMatch(/(min-)?width\s*:\s*(3[2-9]\d|[4-9]\d\d|\d{4,})px/);
  expect(css).toMatch(/overflow-wrap:\s*anywhere/);
  expect(css).toMatch(/flex-wrap:\s*wrap/);
});
