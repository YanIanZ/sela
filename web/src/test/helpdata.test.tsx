import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { I18nProvider } from "../i18n";
import { RouterProvider } from "../lib/router";
import { EmergencySheet } from "../v3/emergency";
import { channelHref, helpData, telHref, waNumber } from "../v3/helpdata";

afterEach(cleanup);

it("dials the main number and opens WhatsApp in international form", () => {
  expect(telHref("119 ext 8")).toBe("tel:119");
  expect(telHref("112")).toBe("tel:112");
  expect(waNumber("0811-843-6633")).toBe("628118436633");
  expect(waNumber("+62 811-1129-129")).toBe("628111129129");
  expect(channelHref({ kind: "whatsapp", value: "0811-843-6633" })).toBe("https://wa.me/628118436633");
});

it("keeps every help entry sourced and dated, and the urgent set explicit", () => {
  for (const h of helpData.help) {
    expect(h.verificationStatus).toBe("verified_official");
    expect(h.sources.length).toBeGreaterThan(0);
    for (const s of h.sources) expect(s.url.startsWith("https://")).toBe(true);
    expect(h.reverifyBy > h.lastVerified).toBe(true);
  }
  expect(helpData.help.filter((h) => h.urgent).map((h) => h.id)).toEqual(["darurat-112", "ambulans-119", "healing119-kemenkes"]);
});

it("quick-help offers the emergency number, the ambulance and the mental-health line", () => {
  render(
    <I18nProvider initial="id">
      <RouterProvider>
        <EmergencySheet open onClose={() => {}} />
      </RouterProvider>
    </I18nProvider>,
  );
  const sheet = screen.getByRole("alertdialog", { name: "Kamu tidak sendirian" });
  expect(within(sheet).getByRole("link", { name: /Telepon 112/ })).toHaveAttribute("href", "tel:112");
  expect(within(sheet).getByRole("link", { name: /^Telepon 119$/ })).toHaveAttribute("href", "tel:119");
  expect(within(sheet).getByRole("link", { name: /Telepon 119 ext 8/ })).toHaveAttribute("href", "tel:119");
  expect(within(sheet).getByText("Layanan Darurat 112")).toBeInTheDocument();
});
