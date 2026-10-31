import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { I18nProvider } from "../i18n";
import { RouterProvider } from "../lib/router";
import { DesignPreviewSession } from "../v3/session";
import { SharesPage } from "../v3/screens/Shares";

const shares = [
  {
    id: "s1",
    summaryId: "r1",
    summaryVersion: 1,
    label: "Dr. Sintetis",
    purpose: "consultation",
    state: "active",
    version: 1,
    expiresAt: "2026-11-03T09:00:00Z",
    createdAt: "2026-11-02T09:00:00Z",
    acknowledged: true,
    openedCount: 2,
    acknowledgedCount: 1,
    lastOpenedAt: "2026-11-02T09:20:00Z",
  },
  {
    id: "s2",
    summaryId: "r1",
    summaryVersion: 1,
    label: "Ibu Sintetis",
    purpose: "family",
    state: "revoked",
    version: 2,
    expiresAt: "2026-11-03T09:00:00Z",
    createdAt: "2026-11-02T08:00:00Z",
    acknowledged: false,
    openedCount: 0,
    acknowledgedCount: 0,
    lastOpenedAt: null,
  },
  {
    id: "s3",
    summaryId: "r1",
    summaryVersion: 1,
    label: "Rekan Sintetis",
    purpose: "other",
    state: "active",
    version: 1,
    expiresAt: "2026-11-03T09:00:00Z",
    createdAt: "2026-11-02T07:00:00Z",
    acknowledged: false,
    openedCount: 0,
    acknowledgedCount: 0,
    lastOpenedAt: null,
  },
];

const events: Record<string, { kind: string; at: string }[]> = {
  s1: [
    { kind: "acknowledged", at: "2026-11-02T09:20:00Z" },
    { kind: "opened", at: "2026-11-02T09:10:00Z" },
    { kind: "created", at: "2026-11-02T09:00:00Z" },
  ],
  s2: [
    { kind: "revoked", at: "2026-11-02T10:00:00Z" },
    { kind: "created", at: "2026-11-02T08:00:00Z" },
  ],
  s3: [],
};

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function mockApi() {
  const calls: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      calls.push(url);
      let data: unknown = {};
      if (url.includes("/consultation-summaries")) data = { summaries: [] };
      else if (url.endsWith("/shares")) data = { shares };
      else {
        const id = url.split("/shares/")[1]?.split("/")[0] ?? "";
        data = { events: events[id] ?? [] };
      }
      return { ok: true, status: 200, json: async () => ({ data }) } as unknown as Response;
    }),
  );
  return calls;
}

function renderPage() {
  return render(
    <I18nProvider initial="id">
      <RouterProvider>
        <DesignPreviewSession profile={null}>
          <SharesPage />
        </DesignPreviewSession>
      </RouterProvider>
    </I18nProvider>,
  );
}

it("loads access history on demand and shows honest counters", async () => {
  const calls = mockApi();
  const { container } = renderPage();

  expect(await screen.findByRole("heading", { name: "Dr. Sintetis" })).toBeInTheDocument();
  expect(screen.getByText(/Dibuka 2 kali/)).toBeInTheDocument();
  expect(screen.getByText(/Dikonfirmasi dibaca 1 kali/)).toBeInTheDocument();
  expect(screen.getByText(/Terakhir dibuka/)).toBeInTheDocument();
  expect(screen.getAllByText(/bukan identitas penerima/).length).toBe(shares.length);
  expect(calls.some((u) => u.includes("/shares/s1/events"))).toBe(false);

  const buttons = screen.getAllByRole("button", { name: "Riwayat akses" });
  fireEvent.click(buttons[0]!);
  expect(buttons[0]).toHaveAttribute("aria-expanded", "true");

  expect(await screen.findByText("Ditandai sudah dibaca")).toBeInTheDocument();
  expect(screen.getByText("Dibuka dengan kode akses")).toBeInTheDocument();
  expect(screen.getByText("Tautan dibuat")).toBeInTheDocument();
  expect(calls.filter((u) => u.includes("/shares/s1/events")).length).toBe(1);

  const times = [...container.querySelectorAll("time")].map((t) => t.getAttribute("dateTime"));
  expect(times).toContain("2026-11-02T09:20:00Z");
});

it("keeps a revoked share's history visible and announces an empty history", async () => {
  mockApi();
  renderPage();
  await screen.findByRole("heading", { name: "Ibu Sintetis" });

  const buttons = screen.getAllByRole("button", { name: "Riwayat akses" });
  fireEvent.click(buttons[1]!);
  expect(await screen.findByText("Akses dicabut")).toBeInTheDocument();

  fireEvent.click(buttons[2]!);
  expect(await screen.findByText("Belum ada riwayat.")).toBeInTheDocument();
});

it("keeps the access-history panel collapsed and fetches nothing until asked", async () => {
  const calls = mockApi();
  renderPage();
  await screen.findByRole("heading", { name: "Rekan Sintetis" });
  expect(screen.getAllByText(/Belum pernah dibuka/).length).toBe(2);
  expect(screen.getAllByText(/Belum ada konfirmasi dibaca/).length).toBe(2);

  const button = screen.getAllByRole("button", { name: "Riwayat akses" })[2]!;
  expect(button).toHaveAttribute("aria-expanded", "false");
  expect(calls.some((u) => u.includes("/shares/s3/events"))).toBe(false);
});
