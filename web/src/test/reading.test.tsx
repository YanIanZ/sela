import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "../i18n";
import { RouterProvider } from "../lib/router";
import type { ReadingItem, ReadingListItem } from "../v3/api";
import { ReadingItemPage, ReadingPage } from "../v3/screens/Reading";

function json(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function renderPage(node: ReactNode) {
  return render(
    <I18nProvider initial="id">
      <RouterProvider>{node}</RouterProvider>
    </I18nProvider>,
  );
}

const localized = (value: string) => ({ id: value, en: value });

const listItem: ReadingListItem = {
  id: "approved-one",
  version: 1,
  kind: "exercise",
  topics: ["stress"],
  audience: "adult",
  title: { id: "Latihan napas singkat", en: "A short breathing exercise" },
  summary: { id: "Latihan satu menit untuk jeda.", en: "A one-minute exercise for a pause." },
  readingTimeMinutes: { id: 1, en: 1 },
  sources: [
    {
      id: "src-1",
      title: "Source one",
      publisher: "WHO",
      url: "https://example.org/source",
      licence: "CC BY-NC-SA 3.0 IGO",
    },
  ],
  reviewedOn: "2026-11-01",
};

const detail: ReadingItem = {
  ...listItem,
  steps: { id: ["Tarik napas.", "Hembuskan."], en: ["Breathe in.", "Breathe out."] },
  why: { id: "Menjeda membantu tubuh menenangkan diri.", en: "A pause helps the body settle." },
  source: {
    relation: "adapted",
    locator: "Whole item",
    sources: listItem.sources,
  },
  licence: {
    itemTextLicence: "MIT",
    sourceLicence: "CC BY-NC-SA 3.0 IGO",
    commercialUse: "blocked",
    note: "Not legal advice.",
  },
  attribution: localized("Diadaptasi dari WHO."),
  adaptationNotice: localized("WHO tidak bertanggung jawab atas adaptasi ini."),
  adaptationNote: "Adapted for Sela.",
  review: { status: "human_approved", reviewedOn: "2026-11-01", verifier: "supported" },
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("reading library", () => {
  it("shows an honest empty state and keeps the official sources", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ data: { items: [] } })));
    renderPage(<ReadingPage />);

    expect(await screen.findByText(/Kartu latihan Sela masih menunggu tinjauan editor manusia/)).toBeInTheDocument();
    expect(screen.getByText("Sumber resmi WHO dan NHS")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Doing What Matters in Times of Stress/ })).toHaveAttribute(
      "href",
      "https://www.who.int/publications/i/item/9789240003927",
    );
  });

  it("links an approved item to its detail screen", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ data: { items: [listItem] } })));
    renderPage(<ReadingPage />);

    const link = await screen.findByRole("link", { name: "Latihan napas singkat" });
    expect(link).toHaveAttribute("href", "/reading/approved-one");
    expect(screen.getByText(/Ditinjau 1 November 2026/)).toBeInTheDocument();
  });

  it("shows the approved detail with review date, licence and limits", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json({ data: detail })));
    renderPage(<ReadingItemPage slug="approved-one" />);

    expect(await screen.findByRole("heading", { name: "Latihan napas singkat" })).toBeInTheDocument();
    expect(screen.getByText(/Disetujui setelah tinjauan editor manusia/)).toBeInTheDocument();
    expect(screen.getAllByText(/1 November 2026/).length).toBeGreaterThan(0);
    expect(screen.getByText(/bukan nasihat medis/)).toBeInTheDocument();
    expect(screen.getByText(/CC BY-NC-SA 3.0 IGO/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Source one" })).toHaveAttribute("href", "https://example.org/source");
  });

  it("explains that an unpublished slug is absent", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => json({ error: { code: "not_found", message: "Not found" } }, 404)),
    );
    renderPage(<ReadingItemPage slug="unpublished" />);

    expect(await screen.findByText(/Sela hanya menampilkan bacaan yang sudah disetujui editor manusia/)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Bacaan tidak tersedia" })).toBeInTheDocument();
  });
});
