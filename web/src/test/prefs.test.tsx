import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { PrefsProvider, usePrefs } from "../v3/prefs";

afterEach(() => { cleanup(); vi.unstubAllGlobals(); window.localStorage.clear(); });

it("V13 follows system theme and contrast changes without changing saved preferences", () => {
  const queries = new Map<string, { matches: boolean; listeners: Set<() => void> }>();
  vi.stubGlobal("matchMedia", (query: string) => {
    const state = queries.get(query) ?? { matches: false, listeners: new Set<() => void>() };
    queries.set(query, state);
    return { get matches() { return state.matches; }, addEventListener: (_: string, listener: () => void) => state.listeners.add(listener), removeEventListener: (_: string, listener: () => void) => state.listeners.delete(listener) };
  });
  window.localStorage.clear();
  function ReadScheme() { const { scheme } = usePrefs(); return <output>{scheme}</output>; }
  render(<PrefsProvider><ReadScheme /></PrefsProvider>);
  expect(screen.getByText("light")).toBeInTheDocument();
  const change = (query: string) => act(() => {
    const state = queries.get(query)!; state.matches = true;
    state.listeners.forEach(listener => listener());
  });
  change("(prefers-color-scheme: dark)");
  expect(document.documentElement).toHaveAttribute("data-scheme", "dark");
  expect(screen.getByText("dark")).toBeInTheDocument();
  change("(prefers-contrast: more)");
  expect(document.documentElement).toHaveAttribute("data-scheme", "contrast-dark");
  expect(screen.getByText("contrast-dark")).toBeInTheDocument();
  expect(window.localStorage.getItem("sela.prefs")).toBeNull();
});

it("applies the calm default palette, persists a chosen one and ignores unknown values", () => {
  vi.stubGlobal("matchMedia", () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
  window.localStorage.setItem("sela.prefs", JSON.stringify({ palette: "neon" }));
  function Pick() {
    const { prefs, set } = usePrefs();
    return <button onClick={() => set("palette", "teduh")}>{prefs.palette}</button>;
  }
  render(<PrefsProvider><Pick /></PrefsProvider>);
  expect(document.documentElement).toHaveAttribute("data-palette", "embun");
  act(() => screen.getByRole("button").click());
  expect(document.documentElement).toHaveAttribute("data-palette", "teduh");
  expect(JSON.parse(window.localStorage.getItem("sela.prefs")!).palette).toBe("teduh");
});
