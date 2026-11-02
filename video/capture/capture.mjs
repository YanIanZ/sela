// Films the real SELA web UI (built from ../web/src at the production commit)
// against the synthetic Nadia fixture. Output: public/footage/*.jpg plus
// footage.json with the size of every still. Dark scheme (a supported SELA
// setting), Indonesian locale, Asia/Jakarta time.
//
//   SELA_NODE_PATH=/path/to/node_modules node capture/build-web.mjs
//   SELA_NODE_PATH=/path/to/node_modules node capture/capture.mjs [only-name ...]
import { mkdirSync, readFileSync, writeFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import { attach, CHROME, loadPlaywright, ORIGIN, VIDEO } from "./harness.mjs";
import { createRouter, TALK_TURN, CORRECTED, STEP } from "./fixtures.mjs";

const OUT = join(VIDEO, "public/footage");
mkdirSync(OUT, { recursive: true });
const only = process.argv.slice(2);

const settle = (page, ms = 900) => page.waitForTimeout(ms);
const hideChrome = async (page) => {
  // Footage focuses on the work surface; the real header stays in the
  // viewport shots. The footer is never part of a story beat.
  await page.addStyleTag({ content: "footer{display:none!important} *{caret-color:transparent!important}" });
};
// Element shots: pinned bars (top bar, place pill) would overlap the element,
// so they scroll with the page while an element is filmed.
const unpin = (page) =>
  page.evaluate(() => {
    for (const el of document.querySelectorAll("body *")) {
      const pos = getComputedStyle(el).position;
      if (pos === "fixed" || pos === "sticky") el.style.setProperty("position", "static", "important");
    }
  });

const SHOTS = [
  // CH02 — the conversation room
  { name: "talk-compose", path: "/talk/room-1", state: {}, act: async (p) => { await p.getByRole("textbox").last().fill(TALK_TURN); }, viewport: true },
  { name: "talk-reply", path: "/talk/room-1", state: { sent: true }, viewport: true },
  { name: "talk-reflection", path: "/talk/room-1", state: { sent: true, reflection: true }, act: async (p) => { await p.addStyleTag({ content: "#composer{display:none!important}" }); await p.locator("details.s-reflections > summary").click(); await settle(p, 500); }, el: "details.s-reflections", pad: 28 },
  // CH03 — Meaning Integrity Engine on a remembered meaning
  { name: "mie-diff", path: "/notes/memory/mem-f0", state: {}, el: "article.c-paper" },
  { name: "mie-impact", path: "/notes/memory/mem-f0", state: {}, act: async (p) => { await p.getByRole("button", { name: "Perbaiki" }).click(); await settle(p, 400); const t = p.locator("form.c-editor textarea"); await t.fill(CORRECTED); await settle(p, 400); }, el: "article.c-paper" },
  { name: "mie-result", path: "/notes/memory/mem-f0", state: {}, act: async (p) => { await p.getByRole("button", { name: "Perbaiki" }).click(); await settle(p, 300); await p.locator("form.c-editor textarea").fill(CORRECTED); await p.getByRole("button", { name: "Simpan perbaikan" }).click(); await settle(p, 1200); }, el: "main .c-memory", trimTo: "article.c-paper" },
  { name: "mie-stale-summaries", path: "/support/passport", state: { corrected: true, approved: true }, el: "main", marks: { stale: "Persiapan konsultasi (versi awal)", notMedical: "Bukan paspor medis" } },
  { name: "mie-stopped-shares", path: "/support/shares", state: { corrected: true }, el: "main", marks: { stopped: "Dihentikan karena makna dikoreksi" } },
  { name: "mie-cancelled-request", path: "/support/plan", state: { corrected: true }, el: "main", marks: { status: "Status permintaanmu", pick: "Pilih tenaga profesional", how: "Cara kerjanya" } },
  // CH04 — Life Context Timeline
  { name: "timeline", path: "/notes/context", state: {}, viewport: true, scrollTo: "h2, .c-timeline, ol" },
  { name: "timeline-list", path: "/notes/context", state: {}, el: "main", marks: { filters: "Jenis", noAI: "Halaman ini tidak memakai AI", first: "Minggu yang melelahkan", last: "Siapkan ringkasan" } },
  // CH05 — Personal Action Bridge
  { name: "steps-empty", path: "/notes/steps", state: {}, el: "main" },
  { name: "steps-write", path: "/notes/steps", state: {}, act: async (p) => { await p.getByRole("button", { name: "Tulis sendiri" }).click(); await settle(p, 400); await p.locator("form.a-steps-editor textarea").fill(STEP); await settle(p, 300); }, el: "main", marks: { editor: "Langkahku", none: "Tidak ada pengingat" } },
  { name: "steps-saved", path: "/notes/steps", state: { stepSaved: true }, el: "main", marks: { chosen: "Langkah yang kamu pilih", none: "Tidak ada pengingat" } },
  // CH06 — Care Passport (consultation preparation)
  { name: "passport-draft", path: "/support/passport/sum-new", state: {}, el: "main", marks: { notMedical: "Bukan paspor medis", words: "Dengan kata-kataku", ask: "Yang ingin kutanyakan", recipient: "Yang diterima penerima", notSent: "Tidak ikut dikirim", approve: "Periksa dan setujui versi ini" } },
  { name: "passport-approved", path: "/support/passport/sum-new", state: {}, act: async (p) => { await p.getByRole("checkbox", { name: /Aku sudah memeriksa versi 2/ }).check(); await p.getByRole("button", { name: "Setujui versi ini" }).click(); await settle(p, 1200); }, el: "main", marks: { notMedical: "Bukan paspor medis", recipient: "Yang diterima penerima", notSent: "Tidak ikut dikirim", bring: "Bawa dengan caramu" } },
  // CH07 — Support on your terms (simulated partner)
  { name: "care-proposed", path: "/support/plan", state: { approved: true, request: "proposed", oldRequest: false }, el: "main", marks: { pick: "Pilih tenaga profesional", send: "Kirim permintaan", status: "Status permintaanmu" } },
  { name: "care-confirmed", path: "/support/plan", state: { approved: true, request: "confirmed", oldRequest: false }, el: "main", marks: { pick: "Pilih tenaga profesional", status: "Status permintaanmu" } },
  { name: "share-active", path: "/support/shares", state: { approved: true, newShare: "active", corrected: true }, el: "main", marks: { active: "Tautan aktif", make: "Buat tautan baru" } },
  { name: "share-revoked", path: "/support/shares", state: { approved: true, newShare: "revoked", corrected: true }, el: "main", marks: { active: "Tautan aktif", make: "Buat tautan baru" } },
  // Brand / overall
  { name: "home", path: "/home", state: { sent: true }, viewport: true },
];

const { chromium } = loadPlaywright();
const browser = await chromium.launch({ executablePath: CHROME });
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 2, colorScheme: "dark", locale: "id-ID", timezoneId: "Asia/Jakarta", reducedMotion: "reduce" });
const manifestPath = join(OUT, "footage.json");
const manifest = existsSync(manifestPath) ? JSON.parse(readFileSync(manifestPath, "utf8")) : {};

for (const shot of SHOTS) {
  if (only.length && !only.includes(shot.name)) continue;
  const page = await ctx.newPage();
  const log = [];
  page.on("pageerror", (e) => log.push("PAGEERROR " + e.message));
  const { router } = createRouter(shot.state);
  await attach(page, router, log);
  await page.goto(ORIGIN + shot.path);
  await settle(page, 1300);
  await hideChrome(page);
  if (shot.act) await shot.act(page);
  await settle(page, 500);
  const file = join(OUT, shot.name + ".jpg");
  let box;
  if (shot.viewport) {
    await page.screenshot({ path: file, type: "jpeg", quality: 92 });
    box = { width: 1440, height: 900 };
  } else {
    await unpin(page);
    await settle(page, 200);
    const loc = page.locator(shot.el).first();
    if (shot.pad) {
      const b = await loc.boundingBox();
      const sy = await page.evaluate(() => window.scrollY);
      const pad = shot.pad;
      await page.screenshot({ path: file, type: "jpeg", quality: 92, fullPage: true, clip: { x: b.x - pad, y: b.y + sy - pad, width: b.width + 2 * pad, height: b.height + 2 * pad } });
      box = { x: b.x - pad, y: b.y - pad, width: b.width + 2 * pad, height: b.height + 2 * pad };
    } else {
      await loc.screenshot({ path: file, type: "jpeg", quality: 92 });
      box = await loc.boundingBox();
    }
    if (shot.marks) {
      const found = {};
      for (const [k, text] of Object.entries(shot.marks)) {
        const b = await page.getByText(text, { exact: false }).first().boundingBox().catch(() => null);
        if (b) found[k] = { y: Math.round((b.y - box.y) * 2), x: Math.round((b.x - box.x) * 2), h: Math.round(b.height * 2) };
      }
      shot.found = found;
    }
  }
  manifest[shot.name] = { file: `footage/${shot.name}.jpg`, width: Math.round(box.width * 2), height: Math.round(box.height * 2), path: shot.path, state: shot.state, marks: shot.found ?? {} };
  console.log(`${shot.name.padEnd(24)} ${manifest[shot.name].width}x${manifest[shot.name].height} ${log.filter((l) => l.startsWith("PAGE")).join(" ")}`);
  await page.close();
}
writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));
await browser.close();
