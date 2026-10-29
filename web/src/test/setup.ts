import '@testing-library/jest-dom/vitest';

// jsdom implements the <dialog> element but not its imperative API. The
// shell's menu and the shared dialog use it; this minimal polyfill mirrors
// the open attribute and the close event.
if (typeof HTMLDialogElement !== "undefined") {
  const proto = HTMLDialogElement.prototype as unknown as {
    show?: () => void;
    showModal?: () => void;
    close?: () => void;
  };
  if (!proto.show) {
    proto.show = function (this: HTMLDialogElement) {
      this.setAttribute("open", "");
    };
  }
  if (!proto.showModal) {
    proto.showModal = function (this: HTMLDialogElement) {
      this.setAttribute("open", "");
      this.setAttribute("data-modal", "");
    };
  }
  if (!proto.close) {
    proto.close = function (this: HTMLDialogElement) {
      if (!this.hasAttribute("open")) return;
      this.removeAttribute("open");
      this.dispatchEvent(new Event("close"));
    };
  }
}

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
