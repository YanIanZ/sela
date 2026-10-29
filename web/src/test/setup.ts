import '@testing-library/jest-dom/vitest';

// jsdom has no layout: the router's scroll-to-top is a no-op in tests.
window.scrollTo = () => undefined;

// Node 22 ships an experimental global localStorage that is unavailable
// without --localstorage-file, and jsdom does not replace it. The app's
// display-preference storage is behind try/catch in production; tests use a
// small in-memory implementation so the persistence path is exercised.
for (const name of ["localStorage", "sessionStorage"] as const) {
  if (!window[name]) {
    const data = new Map<string, string>();
    Object.defineProperty(window, name, {
      configurable: true,
      value: {
        getItem: (key: string) => data.get(key) ?? null,
        setItem: (key: string, value: string) => void data.set(key, String(value)),
        removeItem: (key: string) => void data.delete(key),
        clear: () => data.clear(),
        key: (index: number) => [...data.keys()][index] ?? null,
        get length() {
          return data.size;
        },
      },
    });
  }
}
