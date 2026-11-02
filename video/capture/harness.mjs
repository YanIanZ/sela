// Shared Playwright harness: serves the built web app and answers /api/v3
// from the synthetic fixture router. Nothing leaves the machine: every
// request to the fake origin is fulfilled locally and anything else is aborted.
import { existsSync, readFileSync } from "node:fs";
import { extname, join, resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const here = dirname(fileURLToPath(import.meta.url));
export const VIDEO = resolve(here, "..");
export const ORIGIN = "http://sela.test";
const WEB = join(VIDEO, ".cache/web");

const require = createRequire(import.meta.url);
export function loadPlaywright() {
  const tries = [join(VIDEO, "node_modules/playwright-core"), process.env.SELA_NODE_PATH && join(process.env.SELA_NODE_PATH, "playwright-core"), "playwright-core"];
  for (const t of tries.filter(Boolean)) {
    try {
      return require(t);
    } catch {}
  }
  throw new Error("playwright-core not found (npm i, or set SELA_NODE_PATH)");
}

export const CHROME = process.env.SELA_CHROME || ["/usr/bin/google-chrome", "/usr/bin/chromium", "/usr/bin/chromium-browser"].find((p) => existsSync(p));

const TYPES = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png", ".woff2": "font/woff2", ".json": "application/json", ".txt": "text/plain" };

export async function attach(page, router, log = []) {
  await page.route("**/*", async (route) => {
    const req = route.request();
    const url = new URL(req.url());
    if (url.origin !== ORIGIN) return route.abort();
    if (url.pathname.startsWith("/api/v3/")) {
      const path = url.pathname.slice("/api/v3".length) + url.search;
      let body;
      try {
        body = req.postData() ? JSON.parse(req.postData()) : undefined;
      } catch {}
      const reply = router(req.method(), path, body) ?? { status: 404, error: { code: "not_found" } };
      log.push(`${req.method()} ${path} -> ${reply.status ?? 200}`);
      const status = reply.status ?? 200;
      return route.fulfill({
        status,
        contentType: "application/json",
        body: JSON.stringify(reply.error ? { error: reply.error } : { data: reply.data ?? {} }),
      });
    }
    if (url.pathname.startsWith("/api/")) return route.fulfill({ status: 404, body: "{}" });
    let file = join(WEB, decodeURIComponent(url.pathname));
    if (!file.startsWith(WEB) || !existsSync(file) || url.pathname === "/" || !extname(file)) file = join(WEB, "index.html");
    return route.fulfill({ status: 200, contentType: TYPES[extname(file)] ?? "application/octet-stream", body: readFileSync(file) });
  });
}
