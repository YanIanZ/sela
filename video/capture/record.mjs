// Records the product demo: real interactions in the real SELA web UI
// (built from ../web/src) against the synthetic Nadia fixture.
//
// Frame-exact screen recording: every frame is a screenshot taken after the
// action it shows (typing a character, moving the cursor, scrolling a step,
// a click). Held frames are hard links. CSS motion is reduced so captures
// never catch a half-finished transition. A drawn cursor and click ripple are
// injected into the page because headless Chrome has no visible pointer.
//
// Output: public/demo/<clip>/f00000.jpg … and public/demo/demo.json with frame
// counts, marks (named events, e.g. "send") and clicks per clip.
//
//   SELA_NODE_PATH=… node capture/record.mjs [clip ...]
import { linkSync, mkdirSync, readFileSync, rmSync, writeFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import { attach, CHROME, loadPlaywright, ORIGIN, VIDEO } from "./harness.mjs";
import { createRouter, TALK_TURN, CORRECTED, STEP } from "./fixtures.mjs";

const W = 1280, H = 720, DSF = 1.2; // 1536×864 frames
const OUT = join(VIDEO, "public/demo");
mkdirSync(OUT, { recursive: true });

const CURSOR_JS = `
(() => {
  const mk = () => {
    if (document.getElementById('__cur')) return;
    const c = document.createElement('div');
    c.id = '__cur';
    c.style.cssText = 'position:fixed;left:0;top:0;width:30px;height:30px;z-index:2147483647;pointer-events:none;transform:translate(-200px,-200px);';
    c.innerHTML = '<svg width="30" height="30" viewBox="0 0 24 24"><path d="M4 2l16 9.2-7 1.6 3.9 7.4-3 1.5-3.9-7.4L4 19z" fill="#fff" stroke="#111" stroke-width="1.4" stroke-linejoin="round"/></svg>';
    const r = document.createElement('div');
    r.id = '__rip';
    r.style.cssText = 'position:fixed;left:0;top:0;width:44px;height:44px;margin:-22px 0 0 -22px;border-radius:50%;border:3px solid #A9D5A0;background:rgba(169,213,160,.25);z-index:2147483646;pointer-events:none;opacity:0;';
    document.documentElement.append(r, c);
  };
  window.__cursor = (x, y) => { mk(); document.getElementById('__cur').style.transform = 'translate(' + (x - 4) + 'px,' + (y - 2) + 'px)'; };
  window.__ripple = (x, y, p) => { mk(); const r = document.getElementById('__rip'); r.style.left = x + 'px'; r.style.top = y + 'px'; r.style.opacity = String(Math.max(0, 1 - p)); r.style.transform = 'scale(' + (0.5 + p * 1.2) + ')'; };
  document.addEventListener('DOMContentLoaded', mk);
})();
`;

const ease = (t) => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2);

class Rec {
  constructor(page, name) {
    this.page = page;
    this.name = name;
    this.dir = join(OUT, name);
    rmSync(this.dir, { recursive: true, force: true });
    mkdirSync(this.dir, { recursive: true });
    this.n = 0;
    this.last = null;
    this.marks = {};
    this.clicks = [];
    this.focus = [];
    this.cur = { x: W * 0.62, y: H * 0.7 };
  }
  file(i) {
    return join(this.dir, `f${String(i).padStart(5, "0")}.jpg`);
  }
  async shot(hold = 1) {
    const f = this.file(this.n);
    await this.page.screenshot({ path: f, type: "jpeg", quality: 90 });
    this.last = f;
    this.n++;
    for (let i = 1; i < hold; i++) this.dup();
  }
  dup() {
    linkSync(this.last, this.file(this.n));
    this.n++;
  }
  hold(frames) {
    for (let i = 0; i < frames; i++) this.dup();
  }
  mark(name) {
    this.marks[name] = this.n;
  }
  /** Ask the film to push in on `loc` for the next `frames` frames. */
  async zoom(loc, frames, name = "") {
    const b = await loc.boundingBox();
    if (!b) return;
    this.focus.push({ name, from: this.n, to: this.n + frames, x: Math.round(b.x * DSF), y: Math.round(b.y * DSF), w: Math.round(b.width * DSF), h: Math.round(b.height * DSF) });
  }
  async setCursor(x, y) {
    this.cur = { x, y };
    await this.page.evaluate(([a, b]) => window.__cursor(a, b), [x, y]);
  }
  async move(x, y, frames = 18) {
    const { x: x0, y: y0 } = this.cur;
    for (let i = 1; i <= frames; i++) {
      const t = ease(i / frames);
      // a slight arc reads as a hand, not a robot
      const arc = Math.sin(t * Math.PI) * Math.min(40, Math.hypot(x - x0, y - y0) * 0.08);
      await this.setCursor(x0 + (x - x0) * t, y0 + (y - y0) * t - arc);
      await this.shot();
    }
  }
  async center(loc) {
    await loc.scrollIntoViewIfNeeded();
    const b = await loc.boundingBox();
    return { x: b.x + b.width / 2, y: b.y + b.height / 2 };
  }
  async click(loc, { frames = 18, after = 12 } = {}) {
    const c = await this.center(loc);
    await this.move(c.x, c.y, frames);
    this.clicks.push(this.n);
    for (let i = 0; i < 4; i++) {
      await this.page.evaluate(([a, b, p]) => window.__ripple(a, b, p), [c.x, c.y, i / 6]);
      await this.shot();
    }
    await loc.click();
    for (let i = 4; i <= 7; i++) {
      await this.page.evaluate(([a, b, p]) => window.__ripple(a, b, p), [c.x, c.y, i / 7]);
      await this.page.waitForTimeout(60);
      await this.shot();
    }
    await this.page.evaluate(() => window.__ripple(0, 0, 1));
    await this.page.waitForTimeout(250);
    await this.shot(after);
  }
  async type(text, perChar = 2) {
    for (const ch of text) {
      await this.page.keyboard.type(ch);
      await this.shot(perChar);
    }
  }
  async scrollTo(y, frames = 40) {
    const y0 = await this.page.evaluate(() => window.scrollY);
    for (let i = 1; i <= frames; i++) {
      const t = ease(i / frames);
      await this.page.evaluate((v) => window.scrollTo(0, v), y0 + (y - y0) * t);
      await this.shot();
    }
  }
  async scrollToEl(loc, offset = 120, frames = 40) {
    const y = await loc.evaluate((el, off) => el.getBoundingClientRect().top + window.scrollY - off, offset);
    await this.scrollTo(Math.max(0, y), frames);
  }
  meta() {
    return { frames: this.n, marks: this.marks, clicks: this.clicks, focus: this.focus, width: Math.round(W * DSF), height: Math.round(H * DSF) };
  }
}

// ------------------------------------------------------------------ clips
const CLIPS = {
  // 1. Talk: type, send, see the labelled test reply, ask what Sela noticed, agree.
  talk: {
    path: "/talk/room-1",
    state: {},
    async run(r, p) {
      await r.shot(40);
      const box = p.locator("#composer-text");
      r.mark("compose");
      await r.click(box, { frames: 20, after: 6 });
      r.mark("typing");
      await r.type(TALK_TURN, 2);
      await r.shot(18);
      r.mark("send");
      const kirim = p.getByRole("button", { name: "Kirim" });
      const c = await r.center(kirim);
      await r.move(c.x, c.y, 14);
      r.clicks.push(r.n);
      const done = kirim.click();
      await p.waitForTimeout(250);
      await r.shot(30); // "Sela is writing…" while the reply is prepared
      await done;
      await p.getByText("balasan uji", { exact: false }).first().waitFor();
      await p.waitForTimeout(300);
      r.mark("reply");
      await r.zoom(p.locator(".s-turn, article").filter({ hasText: "balasan uji" }).last(), 90, "reply");
      await r.shot(90);
      const summary = p.locator("details.s-reflections > summary");
      await r.click(summary, { after: 40 });
      r.mark("notice");
      const notice = p.getByRole("button", { name: "Tangkap dari percakapan ini" });
      await r.click(notice, { after: 6 });
      await p.getByText("Rumusan Sela").first().waitFor();
      await p.waitForTimeout(250);
      await r.scrollToEl(p.locator("details.s-reflections"), 90, 24);
      r.mark("reflection");
      await r.zoom(p.locator("details.s-reflections article").first(), 150, "reflection");
      await r.shot(150);
      await r.click(p.getByRole("button", { name: "Setuju" }), { after: 10 });
      r.mark("agreed");
      await r.shot(60);
    },
  },
  // 2. Meaning Integrity Engine: remembered meaning (test fixture), edit, preview impact, save.
  memory: {
    path: "/notes/memory",
    state: {},
    async run(r, p) {
      await r.shot(60);
      await r.click(p.getByRole("link", { name: /Tentang waktu sendiri/ }).first(), { frames: 22, after: 10 });
      await p.getByText("Rumusan Sela").first().waitFor();
      r.mark("detail");
      await r.scrollToEl(p.locator("article.c-paper .c-meaning"), 70, 36);
      r.mark("diff");
      await r.zoom(p.locator("article.c-paper .c-meaning"), 270, "diff");
      await r.shot(270);
      await r.click(p.getByRole("button", { name: "Perbaiki" }), { after: 8 });
      const ta = p.locator("form.c-editor textarea");
      await r.scrollToEl(ta, 150, 24);
      r.mark("edit");
      await r.click(ta, { frames: 14, after: 4 });
      await p.keyboard.press("ControlOrMeta+A");
      await p.keyboard.press("Backspace");
      await r.shot(8);
      r.mark("typing");
      await r.type(CORRECTED, 1);
      await r.shot(14);
      await r.scrollToEl(p.locator(".c-meaning-impact"), 360, 24);
      r.mark("impact");
      await r.zoom(p.locator(".c-meaning-impact"), 165, "impact");
      await r.shot(165);
      r.mark("save");
      await r.click(p.getByRole("button", { name: "Simpan perbaikan" }), { after: 6 });
      await p.getByText("Koreksimu sudah diteruskan").waitFor();
      await r.scrollToEl(p.getByText("Koreksimu sudah diteruskan"), 220, 30);
      r.mark("result");
      await r.zoom(p.getByText("Koreksimu sudah diteruskan").locator("xpath=ancestor::section[contains(@class,'s-notice')][1]"), 130, "result");
      await r.shot(130);
    },
  },
  // 3. What the correction reached, on the real downstream screens.
  "reach-summary": {
    path: "/support/passport",
    state: { corrected: true, approved: true },
    async run(r, p) {
      await r.shot(16);
      await r.scrollToEl(p.getByText("Persiapan konsultasi (versi awal)"), 300, 30);
      r.mark("stale");
      await r.setCursor(560, 460);
      await r.move(520, 430, 10);
      await r.shot(80);
    },
  },
  "reach-share": {
    path: "/support/shares",
    state: { corrected: true },
    async run(r, p) {
      await r.shot(10);
      await r.scrollToEl(p.getByText("Dihentikan karena makna dikoreksi"), 300, 24);
      r.mark("stopped");
      await r.shot(72);
    },
  },
  "reach-care": {
    path: "/support/plan",
    state: { corrected: true },
    async run(r, p) {
      await r.shot(10);
      await r.scrollToEl(p.getByText("Status permintaanmu"), 120, 36);
      r.mark("cancelled");
      await r.zoom(p.getByText("Status permintaanmu").locator("xpath=ancestor::section[1]"), 170, "cancelled");
      await r.shot(170);
    },
  },
  // 4. Life Context Timeline: scroll the saved story, bring two records to a summary.
  timeline: {
    path: "/notes/context",
    state: {},
    async run(r, p) {
      await r.shot(110);
      r.mark("filters");
      await r.click(p.getByRole("button", { name: /30 hari terakhir/ }).first(), { frames: 20, after: 20 }).catch(() => {});
      await r.scrollToEl(p.getByText("Minggu yang melelahkan").first(), 160, 40);
      r.mark("entries");
      await r.shot(120);
      const boxes = p.getByText("Bawa ke ringkasan");
      await r.click(boxes.nth(1), { frames: 18, after: 10 });
      await r.scrollToEl(boxes.nth(2), 300, 30);
      await r.click(boxes.nth(2), { frames: 16, after: 10 });
      await r.scrollToEl(p.getByText("Siapkan ringkasan").first(), 260, 34);
      r.mark("prepare");
      await r.shot(80);
    },
  },
  // 5. Personal Action Bridge: write one step, save it privately.
  steps: {
    path: "/notes/steps",
    state: {},
    async run(r, p) {
      await r.shot(90);
      await r.click(p.getByRole("button", { name: "Tulis sendiri" }), { frames: 20, after: 8 });
      const ta = p.locator("form.a-steps-editor textarea");
      await r.click(ta, { frames: 14, after: 4 });
      r.mark("typing");
      await r.type(STEP, 2);
      await r.shot(24);
      r.mark("save");
      await r.click(p.getByRole("button", { name: "Simpan langkah" }), { after: 10 });
      await p.getByText("Langkah yang kamu pilih").waitFor();
      await r.scrollToEl(p.getByText("Langkah yang kamu pilih"), 200, 26);
      r.mark("saved");
      await r.shot(120);
    },
  },
  // 6. Care Passport: read the sheet and the recipient preview, approve the exact version.
  passport: {
    path: "/support/passport/sum-new",
    state: {},
    async run(r, p) {
      await r.shot(24);
      await r.scrollToEl(p.getByText("Bukan paspor medis"), 120, 34);
      r.mark("notMedical");
      await r.zoom(p.getByText("Bukan paspor medis").locator("xpath=ancestor::section[contains(@class,'s-notice')][1]"), 150, "notMedical");
      await r.shot(150);
      await r.scrollToEl(p.getByText("Dengan kata-kataku").first(), 100, 44);
      r.mark("quotes");
      await r.shot(120);
      await r.scrollToEl(p.getByText("Yang diterima penerima").first(), 90, 50);
      r.mark("recipient");
      await r.shot(120);
      await r.scrollToEl(p.getByText("Periksa dan setujui versi ini"), 260, 44);
      r.mark("approve");
      await r.click(p.getByText(/Aku sudah memeriksa versi 2/), { frames: 20, after: 10 });
      await r.click(p.getByRole("button", { name: "Setujui versi ini" }), { frames: 14, after: 6 });
      await p.waitForTimeout(500);
      await r.scrollToEl(p.getByText("Bawa dengan caramu"), 300, 36);
      r.mark("approved");
      await r.shot(110);
    },
  },
  // 7. Support on your terms: a simulated partner proposes a time; Nadia picks one.
  care: {
    path: "/support/plan",
    state: { approved: true, request: "proposed", oldRequest: false },
    async run(r, p) {
      await r.shot(100);
      await r.scrollToEl(p.getByText("Pilih tenaga profesional"), 100, 40);
      r.mark("partner");
      await r.zoom(p.locator("li").filter({ has: p.locator(".c-plan-org-head") }).first(), 150, "partner");
      await r.shot(150);
      await r.scrollToEl(p.getByText("Status permintaanmu"), 100, 40);
      r.mark("proposed");
      await r.shot(90);
      await r.click(p.getByRole("button", { name: /Pilih 5 Nov 2026/ }), { frames: 22, after: 8 });
      await p.waitForTimeout(400);
      await r.scrollToEl(p.getByText("Status permintaanmu"), 100, 16);
      r.mark("confirmed");
      await r.shot(100);
    },
  },
  // 8. Revoking a share link.
  revoke: {
    path: "/support/shares",
    state: { approved: true, newShare: "active", corrected: true },
    async run(r, p) {
      await r.shot(40);
      await r.scrollToEl(p.getByText("Tautan aktif"), 120, 26);
      r.mark("active");
      await r.shot(80);
      await r.click(p.getByRole("button", { name: "Cabut akses" }).first(), { frames: 20, after: 8 });
      await p.waitForTimeout(400);
      r.mark("revoked");
      await r.zoom(p.locator("article.s-consent").first(), 130, "revoked");
      await r.shot(130);
    },
  },
};

// ------------------------------------------------------------------ run
const only = process.argv.slice(2);
const manifestPath = join(OUT, "demo.json");
const manifest = existsSync(manifestPath) ? JSON.parse(readFileSync(manifestPath, "utf8")) : {};
const { chromium } = loadPlaywright();
const browser = await chromium.launch({ executablePath: CHROME });
for (const [name, clip] of Object.entries(CLIPS)) {
  if (only.length && !only.includes(name)) continue;
  const ctx = await browser.newContext({ viewport: { width: W, height: H }, deviceScaleFactor: DSF, colorScheme: "dark", locale: "id-ID", timezoneId: "Asia/Jakarta", reducedMotion: "reduce" });
  await ctx.addInitScript(CURSOR_JS);
  const page = await ctx.newPage();
  const log = [];
  page.on("pageerror", (e) => log.push("PAGEERROR " + e.message));
  const { router } = createRouter(clip.state);
  await attach(page, router, log);
  await page.goto(ORIGIN + clip.path);
  await page.waitForTimeout(1400);
  await page.addStyleTag({ content: "*{caret-color:#A9D5A0!important} ::-webkit-scrollbar{display:none}" });
  const r = new Rec(page, name);
  await r.setCursor(r.cur.x, r.cur.y);
  try {
    await clip.run(r, page);
  } catch (e) {
    console.error(`clip ${name} failed at frame ${r.n}:`, e.message.split("\n")[0]);
    await page.screenshot({ path: join(OUT, `${name}-FAILED.jpg`) });
  }
  manifest[name] = { ...r.meta(), path: clip.path, state: clip.state };
  console.log(`${name.padEnd(14)} ${String(r.n).padStart(5)} frames ${(r.n / 30).toFixed(1)}s marks=${JSON.stringify(r.marks)} ${log.filter((l) => l.startsWith("PAGE")).join(" ")}`);
  await ctx.close();
}
writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));
await browser.close();
