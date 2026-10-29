import { describe, it, expect } from "vitest";
import { recordsInWeek, weekWindow } from "../v3/weekly";
import type { Entry } from "../v3/api";
const entry = (id: string, over: Partial<Entry> = {}): Entry => ({
  id,
  kind: "note",
  title: "Synthetic note",
  text: "Synthetic test",
  version: 1,
  provenance: "user_original",
  reviewStatus: "confirmed",
  safety: "none",
  createdAt: "2026-10-27T12:00:00",
  updatedAt: "2026-11-09T12:00:00",
  ...over,
});
describe("weekly reflection dates", () => {
  it("uses a local Monday through Sunday window, including month boundaries", () => {
    expect(weekWindow(new Date(2026, 10, 1)).days).toEqual([
      "2026-10-26",
      "2026-10-27",
      "2026-10-28",
      "2026-10-29",
      "2026-10-30",
      "2026-10-31",
      "2026-11-01",
    ]);
  });
  it("does not pull old notes into a new week when they are edited", () => {
    expect(recordsInWeek([entry("old")], new Date(2026, 10, 10))).toEqual([]);
  });
  it("uses check-in's chosen day and excludes derived summaries and memories", () => {
    const records = [
      entry("checkin", { kind: "checkin", day: "2026-11-09" }),
      entry("summary", { kind: "summary", createdAt: "2026-11-09T12:00:00" }),
      entry("memory", { kind: "memory", createdAt: "2026-11-09T12:00:00" }),
    ];
    expect(
      recordsInWeek(records, new Date(2026, 10, 10)).map((r) => r.id),
    ).toEqual(["checkin"]);
  });
  it("can look back a full week without using a fixed millisecond offset", () => {
    expect(weekWindow(new Date(2026, 10, 2), -1).days[0]).toBe("2026-10-26");
  });
});
