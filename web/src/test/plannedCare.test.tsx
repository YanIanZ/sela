import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { I18nProvider } from "../i18n";
import { RouterProvider } from "../lib/router";
import { PlannedCarePage } from "../v3/screens/PlannedCare";

const entries = [
  {
    id: "e1",
    kind: "facility",
    name: "Klinik Teduh (simulasi)",
    city: "Bandung",
    description: "Contoh fasilitas untuk demo.",
    contactUrl: "https://example.org/contact",
    sourceUrl: "https://example.org/source",
    sourcePublisher: "Sumber simulasi",
    sourceAccessedAt: "2026-10-20T00:00:00Z",
    synthetic: true,
    reverifyAt: "2027-01-20T00:00:00Z",
  },
];

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

it("shows sourced planned-care entries, labels simulations and sends a search", async () => {
  const calls: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      calls.push(String(input));
      return {
        ok: true,
        status: 200,
        json: async () => ({ data: { entries } }),
      } as unknown as Response;
    }),
  );
  render(
    <I18nProvider initial="id">
      <RouterProvider>
        <PlannedCarePage />
      </RouterProvider>
    </I18nProvider>,
  );
  expect(await screen.findByRole("heading", { name: "Klinik Teduh (simulasi)" })).toBeInTheDocument();
  expect(screen.getAllByText("Simulasi").length).toBeGreaterThan(0);
  expect(screen.getByText(/Sumber simulasi/)).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /Buka kontak resmi/ })).toHaveAttribute(
    "href",
    "https://example.org/contact",
  );

  fireEvent.change(screen.getByLabelText("Cari nama atau sumber"), {
    target: { value: "Teduh" },
  });
  await waitFor(() => expect(calls.some((u) => u.includes("q=Teduh"))).toBe(true));
});
