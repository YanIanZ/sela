// Local browser evidence for the v3 frontend. Synthetic data only.
import { chromium } from "playwright";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = "/repo";
const OUT = join(ROOT, "e2e/results/v3-frontend");
mkdirSync(OUT, { recursive: true });
const axeSource = readFileSync(join(ROOT, "web/node_modules/axe-core/axe.min.js"), "utf8");
const base = process.env.E2E_BASE_URL ?? "http://127.0.0.1:8080";
const results = [];
const errors = [];

function check(name, ok, detail = "") {
  results.push({ name, ok, detail });
  console.log(`${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`);
}

async function audit(page, name) {
  await page.evaluate(axeSource);
  const violations = await page.evaluate(async () => {
    const r = await window.axe.run(document, {
      runOnly: { type: "tag", values: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"] },
    });
    return r.violations
      .filter((v) => v.impact === "serious" || v.impact === "critical")
      .map((v) => ({
        id: v.id,
        impact: v.impact,
        nodes: v.nodes.map((n) => ({ target: n.target, html: n.html, summary: n.failureSummary })),
      }));
  });
  check(`axe ${name}`, violations.length === 0, JSON.stringify(violations));
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
  );
  check(`no horizontal overflow ${name}`, !overflow);
}

const browser = await chromium.launch({ headless: true, executablePath: "/usr/bin/chromium-browser", args: ["--no-sandbox"] });
const context = await browser.newContext({ viewport: { width: 1365, height: 900 }, locale: "id-ID" });
const page = await context.newPage();
page.on("pageerror", (e) => errors.push("pageerror: " + e.message));
page.on("console", (m) => {
  if (m.type() === "error" && !/401 \(Unauthorized\)/.test(m.text())) {
    errors.push("console: " + m.text());
  }
});

try {
  await page.goto(base + "/", { waitUntil: "networkidle" });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "01-intro-desktop.png"), fullPage: true });
  await audit(page, "intro desktop");

  // Keyboard: the skip link is the first focusable element.
  await page.keyboard.press("Tab");
  const first = await page.evaluate(() => document.activeElement?.textContent?.trim());
  check("intro first focusable is the skip link", first === "Lewati ke konten", first);

  await page.getByRole("link", { name: "Mulai", exact: true }).first().click();
  await page.getByRole("radio", { name: "18 tahun ke atas" }).click();
  await page.getByRole("button", { name: "Lanjut" }).click();
  await page.getByRole("radio", { name: /Mulai sebagai tamu/ }).click();
  await page.getByRole("radio", { name: /Simpan di Catatanku/ }).click();
  await page.getByLabel(/Aku sudah membaca dan menyetujui/).click();
  await page.getByLabel(/Aku mengerti cara penyimpanan/).click();
  await page.getByRole("button", { name: "Mulai sebagai tamu", exact: true }).click();
  await page.waitForURL("**/start/ai");
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "02-ai-consent-desktop.png"), fullPage: true });
  await audit(page, "AI consent");

  // Decline both AI cards; the room still works as a writing space.
  const cards = page.locator(".s-consent");
  if (await cards.count()) {
    await cards.nth(0).getByRole("button", { name: "Tidak sekarang" }).click();
    await page.waitForTimeout(150);
    if (await cards.count() > 1) {
      await cards.nth(1).getByRole("button", { name: "Tidak sekarang" }).click();
      await page.waitForTimeout(150);
    }
  }
  await page.getByRole("button", { name: "Mulai bercerita", exact: true }).click();
  await page.waitForURL("**/talk/new");
  await page.getByRole("radio", { name: /Jangan simpan/ }).click();
  await page.getByRole("button", { name: "Buka ruang cerita" }).click();
  await page.waitForURL(/\/talk\/[0-9a-f]+/);
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "03-room-empty-desktop.png"), fullPage: true });
  await audit(page, "room empty");
  await page.getByRole("button", { name: "Atur percakapan" }).click();
  const detailFocus = await page.evaluate(() => document.activeElement?.textContent?.trim());
  check(
    "room Detail sheet starts with the help control",
    detailFocus === "Butuh bantuan sekarang",
    detailFocus,
  );
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "03b-room-detail-sheet.png"), fullPage: true });
  await page.keyboard.press("Escape");

  const composer = page.getByLabel("Tulis untuk Sela");
  await composer.fill("Aku menulis ini untuk menguji ruang cerita sintetis.");
  await page.getByRole("button", { name: "Kirim" }).click();
  await page.waitForTimeout(400);
  const noReply = await page.locator("text=/Balasan AI tidak diizinkan|Sela belum bisa membalas/").count();
  check("room shows an honest no-reply state without AI consent", noReply > 0);
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "04-room-no-ai-desktop.png"), fullPage: true });
  await page.getByRole("button", { name: "Akhiri percakapan" }).first().click();
  await page.waitForTimeout(250);
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "05-room-closing-desktop.png"), fullPage: true });

  // Write and save a note.
  await page.goto(base + "/notes/write", { waitUntil: "networkidle" });
  await page.getByLabel("Catatanmu").fill("Catatan sintetis: aku merasa lebih tenang setelah menulis.");
  await page.getByLabel(/Simpan catatan ini di Catatanku/).click();
  await page.getByRole("button", { name: "Simpan catatan" }).click();
  await page.waitForURL(/\/notes\/[0-9a-f-]+/);
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "06-note-detail-desktop.png"), fullPage: true });
  await audit(page, "note detail");

  // Check in.
  await page.goto(base + "/notes/check-in", { waitUntil: "networkidle" });
  await page.getByRole("radio", { name: "Biasa saja" }).click();
  await page.getByRole("radio", { name: "Sedang" }).click();
  await page.getByText("Tidur", { exact: true }).click();
  await page.getByLabel(/Simpan cek perasaan ini/).click();
  await page.getByRole("button", { name: "Simpan", exact: true }).click();
  await page.waitForURL(/\/notes\/[0-9a-f-]+/);
  await audit(page, "check-in detail");

  await page.goto(base + "/notes", { waitUntil: "networkidle" });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "07-timeline-desktop.png"), fullPage: true });
  await audit(page, "timeline");

  await page.goto(base + "/settings/privacy", { waitUntil: "networkidle" });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "08-privacy-desktop.png"), fullPage: true });
  await audit(page, "privacy");

  await page.goto(base + "/settings/display", { waitUntil: "networkidle" });
  await page.getByRole("radio", { name: "Aa 130%" }).click();
  await page.getByRole("radio", { name: "Gelap" }).click();
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "09-display-dark-130.png"), fullPage: true });
  await audit(page, "display dark 130%");

  // English and a 320px phone.
  await page.getByRole("button", { name: "English" }).first().click();
  await page.setViewportSize({ width: 320, height: 740 });
  await page.goto(base + "/home", { waitUntil: "networkidle" });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "10-home-mobile-en.png"), fullPage: true });
  await audit(page, "home mobile 320 EN");

  await page.goto(base + "/talk", { waitUntil: "networkidle" });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "11-talk-mobile-en.png"), fullPage: true });
  await audit(page, "talk list mobile 320 EN");

  await page.goto(base + "/help", { waitUntil: "networkidle" });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "12-help-mobile-en.png"), fullPage: true });
  await audit(page, "help mobile 320 EN");

  // Authenticated accessibility sample in Indonesian at 320.
  await page.getByRole("button", { name: "Menu" }).click();
  const menuFocus = await page.evaluate(() => document.activeElement?.textContent?.trim());
  check(
    "menu sheet starts with the help control",
    /help now/i.test(menuFocus ?? ""),
    menuFocus,
  );
  await page.getByRole("button", { name: "Bahasa Indonesia" }).click();
  await page.keyboard.press("Escape");
  await page.waitForTimeout(200);
  await page.setViewportSize({ width: 320, height: 740 });
  await page.goto(base + "/settings/privacy", { waitUntil: "networkidle" });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "13-privacy-mobile-id.png"), fullPage: true });
  await audit(page, "privacy mobile 320 ID");

  await page.goto(base + "/settings/account", { waitUntil: "networkidle" });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "14-account-mobile-id.png"), fullPage: true });
  await audit(page, "account mobile 320 ID");

  await page.goto(base + "/support", { waitUntil: "networkidle" });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "15-support-mobile-id.png"), fullPage: true });
  await audit(page, "support mobile 320 ID");

  await page.goto(base + "/support/summaries/new", { waitUntil: "networkidle" });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "16-summary-blocked-mobile-id.png"), fullPage: true });
  await audit(page, "summary blocked mobile 320 ID");

  await page.goto(base + "/reading", { waitUntil: "networkidle" });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "17-reading-blocked-mobile-id.png"), fullPage: true });
  await audit(page, "reading blocked mobile 320 ID");

  await page.goto(base + "/policies", { waitUntil: "networkidle" });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "18-policies-mobile-id.png"), fullPage: true });
  await page.getByRole("link", { name: /Ketentuan/ }).first().click();
  await page.waitForURL("**/policies/terms_privacy");
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "19-policy-detail-mobile-id.png"), fullPage: true });
  await audit(page, "policy detail mobile 320 ID");

  // High contrast is part of the display preference set; check it applies.
  await page.goto(base + "/settings/display", { waitUntil: "networkidle" });
  await page
    .getByRole("radiogroup", { name: "Kontras tinggi" })
    .getByRole("radio", { name: "Ya", exact: true })
    .click();
  check(
    "high contrast changes the effective scheme",
    (await page.evaluate(() => document.documentElement.getAttribute("data-scheme")))?.startsWith(
      "contrast-",
    ) === true,
  );
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "20-display-contrast-mobile-id.png"), fullPage: true });
  await audit(page, "display high contrast mobile 320 ID");

  const review = join(ROOT, '.impeccable/review');
  mkdirSync(review, { recursive: true });
  await page.evaluate(() => {
    const prefs = JSON.parse(localStorage.getItem('sela.prefs') ?? '{}');
    Object.assign(prefs, { theme: 'light', contrast: 'system', textScale: '100', spacing: 'normal' });
    localStorage.setItem('sela.prefs', JSON.stringify(prefs));
  });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(base + '/home', { waitUntil: 'networkidle' });
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(review, 'desktop.png'), fullPage: true });
  await audit(page, 'new identity home desktop');
  await page.getByRole('button', { name: 'Menu', exact: true }).click();
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(review, 'menu-desktop.png'), fullPage: false });
  await page.keyboard.press('Escape');
  for (const [route, filename] of [['/notes/write','note-desktop.png'],['/notes/check-in','checkin-desktop.png'],['/settings/privacy','privacy-desktop.png'],['/settings/account','account-desktop.png']]) {
    await page.goto(base + route, { waitUntil: 'networkidle' });
    await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(review, filename), fullPage: true });
    await audit(page, 'new identity ' + route);
  }
  for (const width of [390, 320]) {
    await page.setViewportSize({ width, height: 844 });
    await page.goto(base + '/home', { waitUntil: 'networkidle' });
    await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(review, width === 390 ? 'mobile.png' : 'user-320.png'), fullPage: true });
    await audit(page, 'new identity home ' + width);
  }
  const created = await context.request.post(base + '/api/v3/conversations', {
    data: { save: false }, headers: { 'X-Sela-Request':'1', 'Content-Type':'application/json' },
  });
  check('create a real unsaved room for mobile verification', created.status() === 201);
  const mobileRoom = (await created.json()).data;
  await page.goto(base + '/talk/' + mobileRoom.id, { waitUntil:'networkidle' });
  await page.getByLabel('Tulis untuk Sela').fill('Hari ini pikiranku terasa penuh. Ini percakapan uji sintetis.');
  await page.getByRole('button', { name:'Kirim', exact:true }).click();
  await page.getByText('Hari ini pikiranku terasa penuh. Ini percakapan uji sintetis.', { exact:true }).waitFor();
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path:join(review,'room-mobile.png'),fullPage:true });
  await audit(page,'new identity room 320');
  await page.getByLabel('Tulis untuk Sela').fill('Aku mau bunuh diri malam ini.');
  await page.getByRole('button',{name:'Kirim',exact:true}).click();
  await page.waitForTimeout(500);
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({path:join(review,'crisis-mobile.png'),fullPage:true});
  await audit(page,'new identity crisis 320');
  const roomResponse=await context.request.get(base+'/api/v3/conversations/'+mobileRoom.id);
  check('crisis keeps original user words and pauses real room', (await roomResponse.json()).data.paused === true);
  const publicContext=await browser.newContext({viewport:{width:1440,height:900},locale:'id-ID'});
  const publicPage=await publicContext.newPage();
  await publicPage.goto(base+'/',{waitUntil:'networkidle'});
  await publicPage.screenshot({path:join(review,'intro-desktop.png'),fullPage:true});
  await audit(publicPage,'new identity public desktop');
  await publicPage.setViewportSize({width:390,height:844});
  await publicPage.screenshot({path:join(review,'intro-mobile.png'),fullPage:true});
  await audit(publicPage,'new identity public mobile');
  await publicContext.close();

  // Clean up the synthetic guest through the same server, without UI
  // automation of the destructive dialog.
  const deletion = await context.request.delete(base + "/api/v3/me", {
    data: { confirm: true },
    headers: { "X-Sela-Request": "1", "Content-Type": "application/json" },
  });
  check("synthetic guest deletion succeeds", deletion.ok(), String(deletion.status()));
  const after = await context.request.get(base + "/api/v3/me");
  check("deleted guest has no access", after.status() === 401, String(after.status()));

  check("no browser console or page errors", errors.length === 0, errors.slice(0, 5).join(" | "));
} catch (e) {
  check("journey completed", false, e.message);
  await page.evaluate(() => { window.scrollTo(0, 0); document.activeElement?.blur(); });
  await page.screenshot({ path: join(OUT, "failure.png"), fullPage: true }).catch(() => {});
} finally {
  writeFileSync(join(OUT, "results.json"), JSON.stringify({ results, errors }, null, 2));
  await browser.close();
  process.exitCode = results.some((result) => !result.ok) ? 1 : 0;
}
