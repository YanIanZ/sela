import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "../i18n";
import { RouterProvider } from "../lib/router";
import { V3App } from "../v3/App";
import type { Profile } from "../v3/api";
import type { TimelineItem, TimelinePage } from "../v3/screens/ContextTimeline";
import { clearSummarySelection } from "../v3/summarySelection";

// Life Context Timeline (BW-003). All content is synthetic.

const user: Profile = {
  id: "user-test",
  kind: "guest",
  username: null,
  locale: "id",
  ageBand: "18_plus",
  version: 1,
  createdAt: "2026-10-27T00:00:00Z",
  lastActiveAt: "2026-10-27T00:00:00Z",
  guest: {
    inactivePurgeAt: "2026-11-03T00:00:00Z",
    maxAgePurgeAt: "2026-11-26T00:00:00Z",
    inactiveTtlHours: 168,
    maxAgeDays: 30,
    storage: { location: "server_database", region: null, encrypted: true, boundTo: "this_browser_cookie" },
  },
  recovery: null,
  guardianConsent: null,
  actionsRequired: [],
  environment: "development",
  synthetic: false,
} as unknown as Profile;

const options = {
  username: { minLength: 3, maxLength: 32 },
  password: { minLength: 15, maxLength: 256 },
  recoveryCodes: { count: 10, groups: 4, groupLength: 4 },
  ageBands: { allowed: ["18_plus", "15_17", "unknown"] },
  guest: { available: true, inactiveTtlHours: 168, maxAgeDays: 30 },
  sessions: { idleHours: 2, absoluteHours: 12, trustedIdleDays: 7, trustedAbsoluteDays: 30 },
  guardianConsentRequired: true,
  environment: "development",
  synthetic: false,
};

let me: Profile = user;

const NOTE_TEXT = "Sepulang kuliah aku merasa lelah, catatan sintetis.";

function item(over: Partial<TimelineItem> & { id: string }): TimelineItem {
  const { id, ...rest } = over;
  return {
    source: { recordId: id, version: 1, kind: "note" },
    occurredAt: "2026-10-27T09:00:00Z",
    updatedAt: "2026-10-27T09:00:00Z",
    title: "",
    excerpt: NOTE_TEXT,
    truncated: false,
    provenance: "user_original",
    meaningStatus: "current",
    ...rest,
  };
}

const items: TimelineItem[] = [
  item({
    id: "note-1",
    title: "Catatan sintetis",
    source: { recordId: "note-1", version: 2, kind: "note" },
    provenance: "user_correction",
    meaningStatus: "stale",
    stale: { reason: "source_edited", at: "2026-10-28T09:00:00Z" },
    occurredAt: "2026-10-28T09:00:00Z",
  }),
  item({
    id: "conv-1",
    title: "Percakapan sintetis",
    source: { recordId: "conv-1", version: 1, kind: "conversation" },
    meaningStatus: "needs_review",
    pendingReflections: 1,
    excerpt: "Kata-kataku di percakapan sintetis.",
    occurredAt: "2026-10-27T10:00:00Z",
  }),
  item({
    id: "check-1",
    source: { recordId: "check-1", version: 1, kind: "checkin" },
    day: "2026-10-26",
    mood: "okay",
    excerpt: "",
    occurredAt: "2026-10-26T08:00:00Z",
  }),
];

const page = (list: TimelineItem[], nextCursor: string | null = null): TimelinePage => ({
  items: list,
  nextCursor,
  count: list.length,
  aiUsed: false,
});

type Call = { method: string; path: string; search: URLSearchParams };
type Handler = (url: URL) => { status?: number; data?: unknown; error?: string };

function json(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), { status, headers: { "Content-Type": "application/json" } });
}

function mockApi(handlers: Record<string, Handler>) {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
      const url = new URL(String(input), "http://localhost");
      const method = (init.method ?? "GET").toUpperCase();
      const path = url.pathname.replace(/^\/api\/v3/, "");
      calls.push({ method, path, search: url.searchParams });
      const handler = handlers[`${method} ${path}`];
      if (handler) {
        const r = handler(url);
        if (r.error) return json({ error: { code: r.error, message: r.error } }, r.status ?? 400);
        return json({ data: r.data ?? null }, r.status ?? 200);
      }
      if (path === "/me") return json({ data: me });
      if (path === "/policies") return json({ data: { policies: {} } });
      if (path === "/auth/options") return json({ data: options });
      return json({ error: { code: "not_found", message: "Not found" } }, 404);
    }),
  );
  return calls;
}

function renderPath(path: string, lang: "id" | "en" = "id") {
  window.history.replaceState({}, "", path);
  return render(
    <I18nProvider initial={lang}>
      <RouterProvider>
        <V3App />
      </RouterProvider>
    </I18nProvider>,
  );
}

const timelineCalls = (calls: Call[]) => calls.filter((c) => c.path === "/context-timeline");

beforeEach(() => {
  me = user;
  clearSummarySelection();
  try {
    window.localStorage.clear();
    window.sessionStorage.clear();
  } catch {
    // jsdom without storage
  }
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Life Context Timeline", () => {
  it("shows the empty state with no AI request and a link to write (LT-01, BW-T01)", async () => {
    const calls = mockApi({ "GET /context-timeline": () => ({ data: page([]) }) });
    renderPath("/notes/context");
    expect(await screen.findByRole("heading", { level: 1, name: "Perjalanan ceritaku" })).toBeInTheDocument();
    expect(await screen.findByText("Belum ada cerita tersimpan.")).toBeInTheDocument();
    expect(screen.getByText("Kamu bisa menulis atau memilih untuk menyimpan percakapan.")).toBeInTheDocument();
    expect(within(screen.getByRole("main")).getByRole("link", { name: "Tulis catatan" })).toHaveAttribute("href", "/notes/write");
    expect(calls.every((c) => c.method === "GET")).toBe(true);
    expect(calls.some((c) => /conversations|reflections|memory\/proposals|turns/.test(c.path))).toBe(false);
    const first = timelineCalls(calls)[0]!;
    expect(first.search.get("days")).toBe("90");
    expect(first.search.get("kinds")).toBe("note,checkin,conversation");
  });

  it("lists sources by local day with version, provenance and text-labelled status (LT-05, LT-06)", async () => {
    mockApi({ "GET /context-timeline": () => ({ data: page(items) }) });
    const { container } = renderPath("/notes/context");
    const stale = await screen.findByRole("article", { name: "Catatan sintetis" });
    // Status is text, not colour only.
    expect(within(stale).getByText("Perlu ditinjau ulang")).toBeInTheDocument();
    expect(within(stale).getByText("Versi 2")).toBeInTheDocument();
    expect(within(stale).getByText("Koreksimu")).toBeInTheDocument();
    expect(within(stale).getByText(/setelah ada ringkasan atau ingatan yang memakai versi lamanya/)).toBeInTheDocument();
    expect(within(stale).getByText(NOTE_TEXT)).toBeInTheDocument();
    expect(within(stale).getByRole("link", { name: "Buka catatan" })).toHaveAttribute("href", "/notes/note-1");
    expect(stale.querySelector("time")).toHaveAttribute("dateTime", "2026-10-28T09:00:00Z");

    const conv = screen.getByRole("article", { name: "Percakapan sintetis" });
    expect(within(conv).getByText("Menunggu tinjauanmu")).toBeInTheDocument();
    expect(within(conv).getByRole("link", { name: "Buka percakapan" })).toHaveAttribute("href", "/talk?record=conv-1");

    const check = screen.getByRole("article", { name: "Cek perasaan" });
    expect(within(check).getByText("Terkini")).toBeInTheDocument();
    expect(within(check).getByText(/Biasa saja/)).toBeInTheDocument();

    // Three day headings, newest first; no score or diagnosis wording.
    expect(screen.getAllByRole("heading", { level: 2 }).length).toBeGreaterThanOrEqual(3);
    const pageText = container.querySelector(".ct-page")?.textContent ?? "";
    expect(pageText).toMatch(/tidak memakai AI, tidak memberi skor/);
    expect(pageText).not.toMatch(/skormu|skor kamu|risiko|diagnosismu|memburuk|membaik|karena kamu/i);

    const results = await axe.run(container, { rules: { "color-contrast": { enabled: false } } });
    expect(results.violations.filter((v) => ["serious", "critical"].includes(v.impact ?? ""))).toEqual([]);
  });

  it("sends kind and window filters, never text, and resets paging (BW-T02)", async () => {
    const calls = mockApi({ "GET /context-timeline": () => ({ data: page(items) }) });
    renderPath("/notes/context");
    await screen.findByRole("article", { name: "Catatan sintetis" });

    fireEvent.click(screen.getByRole("radio", { name: "7 hari terakhir" }));
    await waitFor(() => expect(timelineCalls(calls).at(-1)!.search.get("days")).toBe("7"));
    expect(window.location.search).toBe("?days=7");

    fireEvent.click(screen.getByRole("checkbox", { name: "Cek perasaan" }));
    await waitFor(() => expect(timelineCalls(calls).at(-1)!.search.get("kinds")).toBe("note,conversation"));
    for (const c of timelineCalls(calls)) {
      expect([...c.search.keys()].every((k) => ["days", "kinds", "limit", "cursor"].includes(k))).toBe(true);
      expect(c.search.has("cursor")).toBe(false);
    }
  });

  it("filters to an empty choice without claiming nothing is saved", async () => {
    mockApi({
      "GET /context-timeline": (url) =>
        url.searchParams.get("days") === "7" ? { data: page([]) } : { data: page(items.slice(0, 1)) },
    });
    renderPath("/notes/context?days=7");
    expect(await screen.findByText("Tidak ada cerita dalam pilihan ini.")).toBeInTheDocument();
    expect(screen.queryByText("Belum ada cerita tersimpan.")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Tampilkan semua, 90 hari" }));
    expect(await screen.findByRole("article", { name: "Catatan sintetis" })).toBeInTheDocument();
  });

  it("loads earlier stories with the cursor and keeps keyboard focus on the first new one (LT-04, LT-08)", async () => {
    const calls = mockApi({
      "GET /context-timeline": (url) =>
        url.searchParams.get("cursor") === "c1"
          ? { data: page([item({ id: "note-old", title: "Catatan lama sintetis", occurredAt: "2026-10-20T09:00:00Z" })]) }
          : { data: page(items.slice(0, 2), "c1") },
    });
    renderPath("/notes/context");
    const more = await screen.findByRole("button", { name: "Muat cerita sebelumnya" });
    const u = userEvent.setup();
    more.focus();
    await u.keyboard("{Enter}");
    const old = await screen.findByRole("article", { name: "Catatan lama sintetis" });
    await waitFor(() => expect(document.activeElement).toBe(old));
    expect(timelineCalls(calls).at(-1)!.search.get("cursor")).toBe("c1");
    expect(screen.queryByRole("button", { name: "Muat cerita sebelumnya" })).not.toBeInTheDocument();
    expect(screen.getAllByRole("article")).toHaveLength(3);
  });

  it("expands the exact original source from the owner API, by keyboard", async () => {
    const calls = mockApi({
      "GET /context-timeline": () => ({ data: page(items) }),
      "GET /records/note-1": () => ({
        data: {
          id: "note-1", kind: "note", title: "Catatan sintetis", text: NOTE_TEXT + " Lanjutan lengkap.",
          version: 3, provenance: "user_correction", reviewStatus: "confirmed", safety: "none",
          createdAt: "2026-10-28T09:00:00Z", updatedAt: "2026-10-29T09:00:00Z",
        },
      }),
    });
    renderPath("/notes/context");
    const article = await screen.findByRole("article", { name: "Catatan sintetis" });
    const button = within(article).getByRole("button", { name: "Lihat sumber asli" });
    expect(button).toHaveAttribute("aria-expanded", "false");
    const u = userEvent.setup();
    button.focus();
    await u.keyboard("{Enter}");
    expect(await within(article).findByText(NOTE_TEXT + " Lanjutan lengkap.")).toBeInTheDocument();
    expect(within(article).getByRole("button", { name: "Tutup sumber asli" })).toHaveAttribute("aria-expanded", "true");
    // The list showed version 2; the source is now version 3.
    expect(within(article).getByText(/sudah berubah ke versi 3/)).toBeInTheDocument();
    expect(calls.some((c) => c.path === "/records/note-1")).toBe(true);
  });

  it("shows an error with retry", async () => {
    let fail = true;
    mockApi({
      "GET /context-timeline": () => (fail ? { status: 500, error: "internal" } : { data: page(items) }),
    });
    renderPath("/notes/context");
    const retry = await screen.findByRole("button", { name: /Coba lagi/ });
    fail = false;
    fireEvent.click(retry);
    expect(await screen.findByRole("article", { name: "Catatan sintetis" })).toBeInTheDocument();
  });

  it("hands ids and versions, never text, to the summary composer (LT-11)", async () => {
    const calls = mockApi({
      "GET /context-timeline": () => ({ data: page(items) }),
      "GET /records": () => ({
        data: {
          records: [
            { id: "note-1", kind: "note", title: "Catatan sintetis", text: NOTE_TEXT, version: 2, provenance: "user_correction",
              reviewStatus: "confirmed", safety: "none", createdAt: "2026-10-28T09:00:00Z", updatedAt: "2026-10-28T09:00:00Z" },
            // Revised since the timeline loaded: not preselected.
            { id: "conv-1", kind: "conversation", title: "Percakapan sintetis", text: "", version: 2, provenance: "user_original",
              reviewStatus: "confirmed", safety: "none", createdAt: "2026-10-27T10:00:00Z", updatedAt: "2026-10-27T10:00:00Z",
              turns: [{ role: "user", text: "Kata-kataku di percakapan sintetis.", provenance: "user_original", createdAt: "2026-10-27T10:00:00Z" }] },
          ],
        },
      }),
      "GET /memory": () => ({ data: { enabled: false, items: [] } }),
    });
    renderPath("/notes/context");
    await screen.findByRole("article", { name: "Catatan sintetis" });
    const selectButton = screen.getByRole("button", { name: "Pilih untuk ringkasan" });
    fireEvent.click(selectButton);
    expect(screen.getByText("Pilih setidaknya satu cerita dulu.")).toBeInTheDocument();
    expect(window.location.pathname).toBe("/notes/context");

    const boxes = screen.getAllByRole("checkbox", { name: "Bawa ke ringkasan" });
    fireEvent.click(boxes[0]!);
    fireEvent.click(boxes[1]!);
    expect(screen.getByText("2 cerita dipilih.")).toBeInTheDocument();
    fireEvent.click(selectButton);

    await waitFor(() => expect(window.location.pathname).toBe("/support/summaries/new"));
    expect(window.location.search).toBe("");
    expect(window.location.href).not.toContain("lelah");
    expect(await screen.findByRole("checkbox", { name: /Catatan sintetis/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Percakapan sintetis/ })).not.toBeChecked();
    expect(screen.getByText(/1 sumber dari Perjalanan ceritaku sudah dipilih/)).toBeInTheDocument();
    expect(screen.getByText(/1 lainnya sudah berubah atau dihapus/)).toBeInTheDocument();
    expect(calls.every((c) => c.method === "GET")).toBe(true);
    for (const c of calls) expect(c.search.toString()).not.toContain("lelah");
  });

  it("renders in English and is reachable from the notes timeline", async () => {
    mockApi({
      "GET /context-timeline": () => ({ data: page(items) }),
      "GET /records": () => ({ data: { records: [] } }),
    });
    me = { ...user, locale: "en" } as Profile;
    renderPath("/notes", "en");
    const link = await screen.findByRole("link", { name: "Story over time" });
    expect(link).toHaveAttribute("href", "/notes/context");
    await act(async () => {
      fireEvent.click(link);
    });
    expect(await screen.findByRole("heading", { level: 1, name: "My story over time" })).toBeInTheDocument();
    const stale = await screen.findByRole("article", { name: "Catatan sintetis" });
    expect(within(stale).getByText("Needs review again")).toBeInTheDocument();
    expect(within(stale).getByText("Version 2")).toBeInTheDocument();
  });
});
