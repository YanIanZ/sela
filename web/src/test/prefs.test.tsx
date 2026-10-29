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

it("drops a colour mood saved by an earlier build: Sampul is the only look", () => {
  vi.stubGlobal("matchMedia", () => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
  window.localStorage.setItem("sela.prefs", JSON.stringify({ palette: "teduh", textScale: "130" }));
  function Read() {
    const { prefs, set } = usePrefs();
    return <button onClick={() => set("spacing", "relaxed")}>{"palette" in prefs ? "has-palette" : prefs.textScale}</button>;
  }
  render(<PrefsProvider><Read /></PrefsProvider>);
  expect(screen.getByRole("button")).toHaveTextContent("130");
  expect(document.documentElement).not.toHaveAttribute("data-palette");
  act(() => screen.getByRole("button").click());
  expect(JSON.parse(window.localStorage.getItem("sela.prefs")!)).not.toHaveProperty("palette");
});
