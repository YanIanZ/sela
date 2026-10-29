// Mobile functional pass: real taps through the main flows on phone viewports.
// Synthetic guest only; reuses the saved session.
import { chromium, devices } from "playwright";
const base = process.env.BASE ?? "http://localhost:5179";
const out = new URL("./wellness/flow-", import.meta.url).pathname;
const state = process.env.STATE ?? "/private/var/folders/1v/m_hbw0l522z4fhqbj91_6h7w0000gn/T/opencode/sela-state.json";
const browser = await chromium.launch();
const results = [];
const check = (name, ok, detail = "") => { results.push({ name, ok }); console.log(`${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`); };

const pages = ["/home", "/talk", "/notes", "/notes/write", "/notes/check-in", "/notes/weekly", "/notes/memory", "/support", "/support/summaries", "/reading", "/help", "/settings/privacy", "/settings/display", "/settings/account", "/about", "/policies"];

for (const [label, device] of [["390", devices["iPhone 13"]], ["360", { ...devices["Galaxy S9+"], viewport: { width: 360, height: 740 } }]]) {
  const ctx = await browser.newContext({ ...device, locale: "id-ID", storageState: state });
  const page = await ctx.newPage();
  const errs = [];
  page.on("pageerror", (e) => errs.push(e.message));
  page.on("console", (m) => { if (m.type() === "error" && !/40[139]|429/.test(m.text())) errs.push(m.text()); });

  // Every page: no sideways scroll, and the floating nav never hides the end of the page.
  for (const path of pages) {
    await page.goto(base + path, { waitUntil: "networkidle" });
    await page.waitForTimeout(250);
    const r = await page.evaluate(async () => {
      const overflow = document.documentElement.scrollWidth > document.documentElement.clientWidth + 1;
      window.scrollTo(0, document.documentElement.scrollHeight);
      await new Promise((res) => setTimeout(res, 150));
      const nav = document.querySelector(".s-place-nav");
      const last = [...document.querySelectorAll("main a, main button, main input, main textarea, footer a")].filter((e) => e.offsetParent).at(-1);
      const navTop = nav && getComputedStyle(nav).display !== "none" ? nav.getBoundingClientRect().top : Infinity;
      const lastBottom = last ? last.getBoundingClientRect().bottom : 0;
      return { overflow, covered: lastBottom > navTop + 1, h1: document.querySelector("h1")?.textContent?.trim() };
    });
    check(`${label} ${path} no overflow`, !r.overflow);
    check(`${label} ${path} last control above nav`, !r.covered);
    if (/Masuk ke akunmu/.test(r.h1 ?? "")) check(`${label} ${path} signed in`, false);
  }

  // Bottom nav taps.
  await page.goto(base + "/home", { waitUntil: "networkidle" });
  for (const [name, path] of [["Cerita", "/talk"], ["Catatanku", "/notes"], ["Dukungan", "/support"], ["Beranda", "/home"]]) {
    await page.locator(".s-place-nav").getByRole("link", { name }).tap();
    await page.waitForURL("**" + path);
    const current = await page.locator(`.s-place-nav a[aria-current="page"]`).textContent();
    check(`${label} nav tap ${name}`, page.url().endsWith(path) && current?.includes(name));
  }

  // Menu opens, traps focus, closes.
  await page.locator(".s-menu-button").tap();
  await page.waitForTimeout(250);
  check(`${label} menu opens`, await page.locator("dialog.s-menu-sheet[open]").isVisible());
  await page.screenshot({ path: `${out}${label}-menu.png` });
  await page.locator("dialog.s-menu-sheet").getByRole("button", { name: "Tutup" }).tap();
  await page.waitForTimeout(250);
  check(`${label} menu closes`, !(await page.locator("dialog.s-menu-sheet[open]").count()));

  // Mood face opens the check-in with the choice made (or the already-today state).
  await page.locator(".s-mood-link").nth(3).tap();
  await page.waitForURL("**/notes/check-in?mood=good");
  await page.waitForLoadState("networkidle"); await page.waitForTimeout(600);
  const preset = await page.locator('input[name="mood"][value="good"]').count() ? await page.locator('input[name="mood"][value="good"]').isChecked({ timeout: 2000 }) : "already";
  check(`${label} mood face → check-in`, preset === true || preset === "already", String(preset));

  // Write and save a note with taps only.
  await page.goto(base + "/notes/write", { waitUntil: "networkidle" });
  await page.getByLabel(/Judul/).fill(`Catatan uji ${label} (sintetis)`);
  await page.getByLabel(/^Catatanmu/).fill("Mencoba menulis dari ponsel. (contoh sintetis)");
  await page.locator(".s-check", { hasText: "Simpan catatan ini" }).tap();
  await page.getByRole("button", { name: "Simpan catatan" }).tap();
  await page.waitForURL((u) => /\/notes\/[^/]+$/.test(u.pathname) && !u.pathname.endsWith("/write"), { timeout: 8000 }).catch(() => {});
  check(`${label} write + save note`, /\/notes\/(?!write)[^/]+$/.test(page.url()), page.url());

  // Open an unsaved room and send a message from the composer.
  await page.goto(base + "/talk/new", { waitUntil: "networkidle" });
  if (await page.getByText("Buka percakapan", { exact: true }).count()) await page.getByText("Buka percakapan", { exact: true }).tap(); else { await page.locator(".s-choice", { hasText: "Jangan simpan" }).tap(); await page.getByRole("button", { name: /Buka ruang cerita/ }).tap(); }
  const box = page.locator("#composer textarea");
  await box.waitFor({ timeout: 8000 });
  await box.fill("Halo, aku cuma mau menulis sebentar. (contoh sintetis)");
  const inView = await page.locator("#composer").evaluate((el) => el.getBoundingClientRect().bottom <= window.innerHeight + 1);
  check(`${label} composer within viewport`, inView);
  await page.locator("#composer").getByRole("button", { name: "Kirim" }).first().tap();
  await page.waitForTimeout(1500);
  check(`${label} message sent`, (await page.locator(".s-turn--user").count()) > 0);
  await page.screenshot({ path: `${out}${label}-room.png` });

  // Display: switch to dark and back.
  await page.goto(base + "/settings/display", { waitUntil: "networkidle" });
  await page.locator(".s-choice", { hasText: "Gelap" }).tap();
  check(`${label} dark scheme applies`, (await page.evaluate(() => document.documentElement.dataset.scheme)) === "dark");
  await page.goto(base + "/home", { waitUntil: "networkidle" });
  await page.screenshot({ path: `${out}${label}-home-dark.png` });
  await page.goto(base + "/settings/display", { waitUntil: "networkidle" });
  await page.locator(".s-choice", { hasText: "Ikuti sistem" }).first().tap();

  // Help is one tap from the header.
  await page.goto(base + "/notes", { waitUntil: "networkidle" });
  await page.locator(".s-header").getByRole("link", { name: /bantuan/i }).tap();
  await page.waitForURL("**/help");
  check(`${label} header help tap`, page.url().endsWith("/help"));

  check(`${label} no console/page errors`, errs.length === 0, errs.slice(0, 3).join(" | "));
  await ctx.close();

  // Signed-out onboarding first step.
  const anon = await browser.newContext({ ...device, locale: "id-ID" });
  const ap = await anon.newPage();
  await ap.goto(base + "/start/age", { waitUntil: "networkidle" });
  await ap.locator(".s-choice", { hasText: "18 tahun ke atas" }).tap();
  await ap.getByRole("button", { name: "Lanjut" }).tap();
  await ap.waitForTimeout(500);
  check(`${label} onboarding step advances`, !ap.url().endsWith("/start/age"), ap.url());
  await anon.close();
}
await browser.close();
const failed = results.filter((r) => !r.ok);
console.log(`\n${results.length - failed.length}/${results.length} passed`);
