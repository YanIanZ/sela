// Polish pass captures: populated states, menu, onboarding, room. Synthetic guest only.
import { chromium } from "playwright";
import { existsSync, writeFileSync } from "node:fs";

const base = process.env.BASE ?? "http://localhost:5179";
const out = new URL("./wellness/polish-", import.meta.url).pathname;
const statePath = "/private/var/folders/1v/m_hbw0l522z4fhqbj91_6h7w0000gn/T/opencode/sela-state.json";
const only = process.env.ONLY ? process.env.ONLY.split(",") : null;
const browser = await chromium.launch({ headless: true });
const errors = [];
const H = { "Content-Type": "application/json", "X-Sela-Request": "1", "X-Sela-Language": "id" };

const seed = await browser.newContext(existsSync(statePath) ? { storageState: statePath } : {});
const sp = await seed.newPage();
await sp.goto(base + "/", { waitUntil: "networkidle" });
const ok = await sp.evaluate(async (H) => {
  if (!(await fetch("/api/v3/me", { headers: H })).ok) {
    const list = (await (await fetch("/api/v3/policies")).json()).data.policies;
    const g = await fetch("/api/v3/auth/guest", { method: "POST", headers: H, body: JSON.stringify({ locale: "id", ageBand: "18_plus", termsPolicyVersion: list.terms_privacy.version, guestNoticeVersion: list.guest_storage.version }) });
    if (!g.ok) return "guest " + g.status;
  }
  const recs = (await (await fetch("/api/v3/records", { headers: H })).json()).data?.records ?? [];
  if (recs.length < 2) {
    const day = new Date().toLocaleDateString("en-CA");
    await fetch("/api/v3/records", { method: "POST", headers: H, body: JSON.stringify({ kind: "note", title: "Pagi yang pelan", text: "Bangun lebih awal dan minum teh di teras. Rasanya lebih tenang daripada kemarin, walau pikiran soal tugas kantor masih muter-muter. (contoh sintetis)", mood: "", energy: "", tags: [], day: "", storageGranted: true, expectedVersion: 0 }) });
    await fetch("/api/v3/records", { method: "POST", headers: H, body: JSON.stringify({ kind: "checkin", title: "", text: "Agak capek tapi oke.", mood: "okay", energy: "medium", tags: ["work", "sleep"], day, storageGranted: true, expectedVersion: 0 }) });
  }
  return "ok";
}, H);
if (ok !== "ok") { console.log(ok); process.exit(1); }
const state = await seed.storageState();
writeFileSync(statePath, JSON.stringify(state));
await seed.close();

async function go(page, name, path, { full = true, after } = {}) {
  if (only && !only.includes(name)) return;
  await page.goto(base + path, { waitUntil: "networkidle" });
  await page.waitForTimeout(450);
  if (after) await after(page);
  if ((await page.locator("h1").first().textContent().catch(() => ""))?.includes("Masuk ke akunmu") && !name.includes("signin")) errors.push(`${name} landed on sign-in`);
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1);
  if (overflow) errors.push(`overflow ${name}`);
  if (full && page.viewportSize().width < 600) await page.addStyleTag({ content: ".s-place-nav{display:none!important}" });
  await page.screenshot({ path: `${out}${name}.png`, fullPage: full });
}

for (const [label, viewport] of [["desktop", { width: 1440, height: 960 }], ["mobile", { width: 390, height: 844 }]]) {
  const ctx = await browser.newContext({ viewport, locale: "id-ID", storageState: state, deviceScaleFactor: label === "mobile" ? 2 : 1 });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => errors.push(`${label} pageerror ${e.message}`));
  page.on("console", (m) => { if (m.type() === "error" && !/40[13]/.test(m.text())) errors.push(`${label} console ${m.text()}`); });
  await go(page, `${label}-home`, "/home");
  await go(page, `${label}-notes`, "/notes");
  const first = await page.evaluate(async (H) => ((await (await fetch("/api/v3/records", { headers: H })).json()).data.records.find((r) => r.kind === "note") ?? {}).id, H);
  if (first) await go(page, `${label}-note`, "/notes/" + first);
  await go(page, `${label}-talk`, "/talk");
  await go(page, `${label}-memory`, "/notes/memory");
  await go(page, `${label}-weekly`, "/notes/weekly");
  await go(page, `${label}-summaries`, "/support/summaries");
  await go(page, `${label}-privacy`, "/settings/privacy");
  await go(page, `${label}-menu`, "/home", { full: false, after: async (p) => { await p.locator(".s-menu-button").click(); await p.waitForTimeout(300); } });
  await go(page, `${label}-focus`, "/home", { full: false, after: async (p) => { for (let i = 0; i < 6; i++) await p.keyboard.press("Tab"); } });
  await ctx.close();
  const anon = await browser.newContext({ viewport, locale: "id-ID", deviceScaleFactor: label === "mobile" ? 2 : 1 });
  const ap = await anon.newPage();
  await go(ap, `${label}-age`, "/start/age");
  await go(ap, `${label}-about`, "/about");
  await go(ap, `${label}-policies`, "/policies");
  await anon.close();
}
await browser.close();
console.log(errors.length ? errors.join("\n") : "no errors");
