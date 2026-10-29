// Local capture for the Taman Napas redesign. Synthetic guest data only.
import { chromium } from "playwright";
import { mkdirSync, readFileSync } from "node:fs";
const axeSource = readFileSync(new URL("../../node_modules/axe-core/axe.min.js", import.meta.url), "utf8");

const base = process.env.BASE ?? "http://127.0.0.1:5179";
const out = process.env.OUT ?? new URL("./wellness/", import.meta.url).pathname;
const only = process.env.ONLY ? process.env.ONLY.split(",") : null;
mkdirSync(out, { recursive: true });

const browser = await chromium.launch({ headless: true });
const errors = [];

async function guest(page) {
  await page.goto(base + "/", { waitUntil: "networkidle" });
  await page.evaluate(async () => {
    const h = { "Content-Type": "application/json", "X-Sela-Request": "1", "X-Sela-Language": "id" };
    const p = await (await fetch("/api/v3/policies", { credentials: "same-origin" })).json();
    const list = p.data?.policies ?? p.policies ?? p;
    const find = (k) => (Array.isArray(list) ? list.find((x) => x.kind === k || x.id === k)?.version : list[k]?.version) ?? "";
    await fetch("/api/v3/auth/guest", {
      method: "POST", credentials: "same-origin", headers: h,
      body: JSON.stringify({ locale: "id", ageBand: "18_plus", termsPolicyVersion: find("terms_privacy"), guestNoticeVersion: find("guest_storage") }),
    });
  });
}

const viewport0 = (page) => page.viewportSize()?.width ?? 1440;
async function shoot(page, name, path, full = true) {
  if (only && !only.includes(name)) return;
  await page.goto(base + path, { waitUntil: "networkidle" });
  await page.waitForTimeout(500);
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  const want = expected[name.split("-").slice(1).join("-")];
  const h1 = (await page.locator("h1").first().textContent())?.trim();
  if (want && h1 !== want) errors.push(`${name}: expected "${want}", got "${h1}"`);
  if (/Masuk ke akunmu/.test(h1 ?? "")) errors.push(`${name}: landed on sign-in`);
  if (viewport0(page) < 600) await page.addStyleTag({ content: ".s-place-nav { display: none !important; }" });
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1);
  if (overflow) errors.push(`overflow ${name}`);
  await page.screenshot({ path: `${out}${name}.png`, fullPage: full });
  await page.evaluate(axeSource);
  const v = await page.evaluate(async () => (await window.axe.run(document, { runOnly: { type: "tag", values: ["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"] } })).violations.filter((x) => x.impact === "serious" || x.impact === "critical").map((x) => x.id + ": " + x.nodes.slice(0, 3).map((n) => n.target.join(" ")).join(" | ")));
  if (v.length) errors.push(`axe ${name}: ${v.join("; ")}`);
}

// One guest, reused by every context so the rate limit is never hit twice.
const statePath = "/private/var/folders/1v/m_hbw0l522z4fhqbj91_6h7w0000gn/T/opencode/sela-state.json";
const { existsSync, writeFileSync } = await import("node:fs");
let seedCtx = await browser.newContext(existsSync(statePath) ? { storageState: statePath } : {});
let seedPage = await seedCtx.newPage();
await seedPage.goto(base + "/", { waitUntil: "networkidle" });
if ((await seedPage.evaluate(async () => (await fetch("/api/v3/me", { headers: { "X-Sela-Request": "1" } })).status)) !== 200) await guest(seedPage);
const me = await seedPage.evaluate(async () => (await fetch("/api/v3/me", { headers: { "X-Sela-Request": "1" } })).status);
if (me !== 200) { console.log("guest session failed:", me); process.exit(1); }
const state = await seedCtx.storageState();
writeFileSync(statePath, JSON.stringify(state));
await seedCtx.close();
const expected = { portal: "Dashboard Sela", home: "Beranda", talk: null, notes: "Catatan pribadiku", checkin: "Cek perasaan", write: "Tulis catatan", help: "Bantuan sekarang", display: "Tampilan & aksesibilitas", room: null, support: null };

for (const [label, viewport, scheme] of [
  ["desktop", { width: 1440, height: 960 }, "light"],
  ["mobile", { width: 390, height: 844 }, "light"],
  ["dark", { width: 1440, height: 960 }, "dark"],
]) {
  const ctx = await browser.newContext({ viewport, locale: "id-ID", colorScheme: scheme, deviceScaleFactor: label === "mobile" ? 2 : 1, storageState: state });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => errors.push(`${label} pageerror: ${e.message}`));
  const anon = await browser.newContext({ viewport, locale: "id-ID", colorScheme: scheme, deviceScaleFactor: label === "mobile" ? 2 : 1 });
  await shoot(await anon.newPage(), `${label}-portal`, "/");
  await anon.close();
  await shoot(page, `${label}-home`, "/home");
  if (label === "dark") { await ctx.close(); continue; }
  await shoot(page, `${label}-talk`, "/talk");
  await shoot(page, `${label}-notes`, "/notes");
  await shoot(page, `${label}-checkin`, "/notes/check-in?mood=good");
  await shoot(page, `${label}-write`, "/notes/write");
  await shoot(page, `${label}-support`, "/support");
  await shoot(page, `${label}-help`, "/help");
  await shoot(page, `${label}-display`, "/settings/display");
  await shoot(page, `${label}-room`, "/talk/new", false);
  await ctx.close();
}
await browser.close();
console.log(errors.length ? errors.join("\n") : "no errors");
