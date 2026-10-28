// Edge captures with one reused guest session: 320px at 150% text, high
// contrast (light and dark), and mobile check-in with and without the nav.
import { chromium } from "playwright";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";

const base = process.env.BASE ?? "http://localhost:5179";
const out = new URL("./wellness/", import.meta.url).pathname;
const browser = await chromium.launch({ headless: true });
const errors = [];

// One guest for every context.
const seed = await browser.newContext({ storageState: "/private/var/folders/1v/m_hbw0l522z4fhqbj91_6h7w0000gn/T/opencode/sela-state.json" });
const sp = await seed.newPage();
await sp.goto(base + "/", { waitUntil: "networkidle" });
const status = await sp.evaluate(async () => {
  if ((await fetch("/api/v3/me", { headers: { "X-Sela-Request": "1" } })).ok) return "200 reused";
  const h = { "Content-Type": "application/json", "X-Sela-Request": "1" };
  const list = (await (await fetch("/api/v3/policies")).json()).data.policies;
  const g = await fetch("/api/v3/auth/guest", { method: "POST", headers: h, body: JSON.stringify({ locale: "id", ageBand: "18_plus", termsPolicyVersion: list.terms_privacy.version, guestNoticeVersion: list.guest_storage.version }) });
  return g.status + " " + (g.ok ? "" : await g.text());
});
if (!status.startsWith("2")) { console.log("guest failed:", status); process.exit(1); }
const state = await seed.storageState();
await seed.close();

const hashes = new Map();
async function shot(page, file, heading, opts = {}) {
  const h1 = (await page.locator("h1").first().textContent())?.trim();
  if (h1 !== heading) { errors.push(`${file}: expected "${heading}", got "${h1}"`); return; }
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1);
  if (overflow) errors.push(`overflow ${file}`);
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  const path = `${out}${file}.png`;
  await page.screenshot({ path, ...opts });
  const md5 = createHash("md5").update(readFileSync(path)).digest("hex");
  if (hashes.has(md5)) errors.push(`${file} identical to ${hashes.get(md5)}`);
  hashes.set(md5, file);
}

const hideNav = ".s-place-nav { display: none !important; }";
for (const [label, viewport, prefs, scale] of [
  ["edge-320-150", { width: 320, height: 720 }, { textScale: "150" }, 2],
  ["edge-contrast", { width: 1280, height: 900 }, { contrast: "more" }, 1],
  ["edge-contrast-dark", { width: 390, height: 844 }, { contrast: "more", theme: "dark" }, 2],
  ["mobile-nav", { width: 390, height: 844 }, {}, 2],
]) {
  const ctx = await browser.newContext({ viewport, locale: "id-ID", storageState: state, deviceScaleFactor: scale });
  await ctx.addInitScript((p) => localStorage.setItem("sela.prefs", JSON.stringify(p)), prefs);
  const page = await ctx.newPage();
  for (const [path, heading] of [["/home", "Beranda"], ["/notes/check-in", "Cek perasaan"]]) {
    await page.goto(base + path, { waitUntil: "networkidle" });
    await page.waitForTimeout(500);
    const name = `${label}${path.replaceAll("/", "-")}`;
    if (label === "mobile-nav") {
      await shot(page, `${name}-viewport`, heading);
      await page.addStyleTag({ content: hideNav });
      await shot(page, `${name}-full`, heading, { fullPage: true });
    } else {
      if (viewport.width < 600) await page.addStyleTag({ content: hideNav });
      await shot(page, name, heading, { fullPage: true });
    }
  }
  if (label === "edge-320-150") {
    // Sign-in reflow, signed out.
    const anon = await browser.newContext({ viewport, locale: "id-ID", deviceScaleFactor: 2 });
    await anon.addInitScript((p) => localStorage.setItem("sela.prefs", JSON.stringify(p)), prefs);
    const ap = await anon.newPage();
    await ap.goto(base + "/signin", { waitUntil: "networkidle" });
    await ap.waitForTimeout(400);
    await shot(ap, "edge-320-150-signin", "Masuk ke akunmu", { fullPage: true });
    await anon.close();
  }
  await ctx.close();
}
await browser.close();
console.log(errors.length ? errors.join("\n") : "no errors");
