// End-to-end journeys against the real Go binary, the built frontend and the
// cmd/fakellm TEST DOUBLE (never Meta inference). Run: node e2e/journey.e2e.mjs
// Requires: `go build` outputs in build/, web/dist built, Playwright + Chromium.
import { spawn } from 'node:child_process';
import { createRequire } from 'node:module';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const { chromium } = require('playwright');
const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..');
const AXE = readFileSync(require.resolve('axe-core/axe.min.js', { paths: [join(ROOT, 'web')] }), 'utf8');
const PORT = Number(process.env.E2E_PORT ?? 18099);
const LLM_PORT = Number(process.env.E2E_LLM_PORT ?? 18090);
const BASE = `http://127.0.0.1:${PORT}`;
// A second server whose test double never answers (fakellm -fault stall),
// for the checks that need an AI step to stay running.
const STALL_PORT = PORT + 1;
const STALL_LLM_PORT = LLM_PORT + 1;
const STALL_BASE = `http://127.0.0.1:${STALL_PORT}`;
const CANARY = `CANARY${Math.random().toString(36).slice(2, 10)}`;
const results = [];
let serverLog = '';

function startProcess(cmd, args, env) {
  const p = spawn(cmd, args, { env: { ...process.env, ...env }, stdio: ['ignore', 'pipe', 'pipe'] });
  p.stdout.on('data', (d) => (serverLog += d.toString()));
  p.stderr.on('data', (d) => (serverLog += d.toString()));
  return p;
}

async function waitFor(url, ms = 20000) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    try {
      const r = await fetch(url);
      if (r.ok) return;
    } catch {}
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error(`timeout waiting for ${url}`);
}

async function check(name, fn) {
  // E2E_ONLY=<substring> runs only matching checks (local debugging).
  if (process.env.E2E_ONLY && !name.includes(process.env.E2E_ONLY)) return;
  const start = Date.now();
  try {
    await fn();
    results.push({ name, ok: true, ms: Date.now() - start });
    console.log(`ok   ${name} (${Date.now() - start} ms)`);
  } catch (err) {
    let where = '';
    try {
      if (lastPage && !lastPage.isClosed()) where = await lastPage.evaluate(() => `${location.pathname} | alerts: ${[...document.querySelectorAll('[role=alert],[role=status]')].map((e) => e.textContent).join(' / ')} | ${document.body.innerText.slice(0, 300).replace(/\s+/g, ' ')}`);
    } catch {}
    results.push({ name, ok: false, error: String(err?.stack ?? err), where });
    console.log(`FAIL ${name}\n  ${String(err?.message ?? err)}\n  at: ${where}`);
  }
}

function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

async function axe(page, label) {
  // Injected through the DevTools protocol: an inline <script> would be (and
  // should be) blocked by the app's CSP.
  await page.evaluate(AXE);
  const res = await page.evaluate(async () => {
    // eslint-disable-next-line no-undef
    const r = await axe.run(document, { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'] } });
    return r.violations.map((v) => ({ id: v.id, impact: v.impact, nodes: v.nodes.length, help: v.help, targets: v.nodes.slice(0, 4).map((n) => `${n.target.join(' ')} :: ${(n.failureSummary || '').split('\n').slice(1, 2).join('')}`) }));
  });
  const serious = res.filter((v) => v.impact === 'serious' || v.impact === 'critical');
  a11yReport.push({ page: label, violations: res });
  assert(serious.length === 0, `${label}: axe serious/critical violations: ${JSON.stringify(serious)}`);
}
const a11yReport = [];

async function noHorizontalScroll(page, label) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  assert(overflow <= 1, `${label}: horizontal overflow ${overflow}px at mobile width`);
}

let lastPage = null;

async function startSession(page, { english = false, ai = true, scanConsent = false, base = BASE } = {}) {
  lastPage = page;
  await page.goto(`${base}/`);
  if (english) await page.getByRole('button', { name: 'English' }).click();
  await page.getByRole('link', { name: english ? 'Take a first step' : 'Mulai pelan-pelan' }).click();
  await page.getByLabel(english ? /I am 18 or older/ : /Saya berusia 18 tahun/).check();
  await page.getByRole('button', { name: english ? 'I understand, continue' : 'Saya mengerti, lanjutkan' }).click();
  await page.waitForURL('**/consent');
  if (scanConsent) await axe(page, 'consent');
  await page.getByRole('button', { name: ai ? (english ? 'Allow AI to help draft' : 'Izinkan AI membantu menyusun draf') : english ? 'No AI (manual path)' : 'Tanpa AI (jalur manual)' }).click();
  await page.getByText(ai ? (english ? 'Allowed' : 'Diizinkan') : english ? 'Not allowed' : 'Tidak diizinkan', { exact: true }).waitFor();
  await page.getByRole('button', { name: english ? 'Continue to the story' : 'Lanjut ke cerita' }).click();
  await page.waitForURL('**/story');
}

async function useScenario(page, title) {
  const card = page.locator('article.scenario', { has: page.getByRole('heading', { name: title }) });
  await card.getByRole('button').click();
}

async function confirmAll(page, label) {
  for (let i = 0; i < 30; i++) {
    const btn = page.getByRole('button', { name: label, exact: true });
    if ((await btn.count()) === 0) return;
    await btn.first().click();
    await page.waitForTimeout(80);
  }
}

const TEST_DOUBLE = { id: 'TEST DOUBLE — bukan Meta, hanya untuk pengujian', en: 'TEST DOUBLE — not Meta, for testing only' };
const OPENING_DRAFT = {
  id: 'Saran kalimat pembuka yang disetujui pengguna (bukan tulisan pengguna sendiri)',
  en: 'Suggested opening lines the user approved (not written by the user)',
};
const OPENING_OWN = { id: 'Kalimat pembuka yang ditulis pengguna sendiri', en: "The user's own opening lines" };
const OPENING_TEMPLATE = {
  id: 'Kalimat pembuka yang disusun Sela dari rumusan catatan yang disetujui (bukan tulisan pengguna sendiri)',
  en: 'Opening lines Sela built from the wording of the approved note (not written by the user)',
};

// assertOpeningDraft checks that test-double opening lines in a projection
// carry their own TEST DOUBLE badge and are never titled the user's own.
async function assertOpeningDraft(scope, lang, label) {
  await scope.getByRole('heading', { name: OPENING_DRAFT[lang] }).waitFor();
  await scope.locator('.opening-provenance').getByText(TEST_DOUBLE[lang], { exact: true }).waitFor();
  assert((await scope.getByRole('heading', { name: OPENING_OWN[lang] }).count()) === 0, `${label}: AI-drafted opening lines titled as the user's own`);
}

async function chooseAndShare(page, { english = false, quotes = true, opening = false, option = 0, setPrefs = true } = {}) {
  if (setPrefs) {
    await page.locator('#pref-language').selectOption(english ? 'en' : 'id');
    await page.getByLabel('Format', { exact: true }).selectOption('online');
  }
  await page.getByRole('button', { name: english ? 'Show options' : 'Tampilkan pilihan' }).click();
  const choose = page.locator('article.option').nth(option).getByRole('button', { name: english ? /^Choose / : /^Pilih / }).first();
  await choose.waitFor();
  await choose.click();
  await page.waitForURL('**/share**');
  if (quotes) await page.getByLabel(english ? 'My original words next to each item' : 'Kata-kataku asli di samping tiap butir').check();
  await page.getByText(english ? 'Preview for the recipient' : 'Pratinjau untuk penerima').waitFor();
  if (opening) {
    await page.getByLabel(english ? 'Opening lines' : 'Kalimat pembuka', { exact: true }).check();
    await assertOpeningDraft(page.locator('.projection'), english ? 'en' : 'id', 'share preview');
  }
  await page.getByLabel(english ? /^I allow Sela to share/ : /^Saya mengizinkan Sela/).check();
  await page.getByRole('button', { name: english ? 'Share this note' : 'Bagikan catatan ini' }).click();
  await page.getByText(english ? 'Simulated request sent' : 'Permintaan simulasi terkirim').waitFor();
  const link = await page.getByLabel(english ? 'Link for the recipient' : 'Tautan untuk penerima').inputValue();
  const code = await page.getByLabel(english ? 'Access code' : 'Kode akses').inputValue();
  return { link, code };
}

// openNote submits the access code on the recipient page. Sign-ins share a
// small per-address rate limit (several checks sign in from 127.0.0.1), so a
// rate-limited attempt waits for the bucket to refill and tries again.
async function openNote(r, code, button) {
  await r.locator('#access-code').fill(code);
  for (let i = 0; i < 4; i++) {
    await r.getByRole('button', { name: button }).click();
    const limited = r.getByRole('alert').filter({ hasText: /Too many requests|Terlalu banyak permintaan/ });
    await Promise.race([r.locator('#req-title').waitFor(), limited.waitFor()]);
    if ((await r.locator('#req-title').count()) > 0) return r.locator('#req-title').innerText();
    await r.waitForTimeout(7000);
  }
  throw new Error('recipient sign-in stayed rate-limited');
}

async function main() {
  const fake = startProcess(join(ROOT, 'build', 'fakellm'), ['-addr', `127.0.0.1:${LLM_PORT}`], {});
  const sela = startProcess(join(ROOT, 'build', 'sela'), [], {
    SELA_ENV: 'test',
    SELA_ADDR: `127.0.0.1:${PORT}`,
    SELA_WEB_DIR: join(ROOT, 'web', 'dist'),
    SELA_SECURE_COOKIES: 'false',
    SELA_LLM_MODE: 'test_double',
    SELA_ALLOW_TEST_DOUBLE: 'true',
    SELA_LLM_BASE_URL: `http://127.0.0.1:${LLM_PORT}`,
    SELA_RESEARCH_KB: join(ROOT, 'data', 'research', 'corpus', 'knowledge_base.jsonl'),
  });
  const stallFake = startProcess(join(ROOT, 'build', 'fakellm'), ['-addr', `127.0.0.1:${STALL_LLM_PORT}`, '-fault', 'stall'], {});
  const stallSela = startProcess(join(ROOT, 'build', 'sela'), [], {
    SELA_ENV: 'test',
    SELA_ADDR: `127.0.0.1:${STALL_PORT}`,
    SELA_WEB_DIR: join(ROOT, 'web', 'dist'),
    SELA_SECURE_COOKIES: 'false',
    SELA_LLM_MODE: 'test_double',
    SELA_ALLOW_TEST_DOUBLE: 'true',
    SELA_LLM_BASE_URL: `http://127.0.0.1:${STALL_LLM_PORT}`,
  });
  try {
    await waitFor(`http://127.0.0.1:${LLM_PORT}/health`);
    await waitFor(`${BASE}/api/v2/ready`);
    await waitFor(`http://127.0.0.1:${STALL_LLM_PORT}/health`);
    await waitFor(`${STALL_BASE}/api/v2/ready`);
    const browser = await chromium.launch();

    await check('ID journey: AI draft (test double) → review → approve → share → recipient ack → complete → follow-up → revoke', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID' });
      const page = await ctx.newPage();
      await startSession(page, { scanConsent: true });
      await axe(page, 'story');
      await useScenario(page, 'Nadia: kewalahan dengan skripsi dan kerja paruh waktu');
      await page.waitForURL('**/ledger');
      await page.getByRole('button', { name: 'Bantu susun catatan dengan AI' }).click();
      await page.getByText(/Draf siap:/).waitFor({ timeout: 15000 });
      assert((await page.getByText('TEST DOUBLE — bukan Meta, hanya untuk pengujian').count()) > 0, 'test-double provenance not labelled');
      await page.getByText('Kata-katamu').first().waitFor();
      await axe(page, 'ledger');
      await confirmAll(page, 'Sesuai');
      await page.getByRole('button', { name: /Setujui catatan versi/ }).click();
      await page.getByText(/Disetujui: versi/).waitFor();
      await page.getByRole('button', { name: 'Lanjut ke langkah praktis' }).click();
      await page.waitForURL('**/next');
      await axe(page, 'next-step');
      // Opening Practice: AI draft grounded in the approved note, then a rehearsal.
      await page.goto(`${BASE}/practice`);
      await page.getByRole('button', { name: 'Susun dengan AI' }).click();
      await page.getByText(/Berdasarkan:/).first().waitFor({ timeout: 15000 });
      await axe(page, 'practice');
      await page.getByLabel('Pembukaanku (tidak disimpan)').fill('Akhir-akhir ini aku merasa kewalahan dengan skripsi dan kerja paruh waktu.');
      await page.getByRole('button', { name: 'Lihat bagian yang sudah tersebut' }).click();
      const mentioned = page.locator('h3', { hasText: 'Sudah kamu sebut' }).locator('xpath=following-sibling::ul[1]/li');
      await mentioned.first().waitFor();
      await page.goto(`${BASE}/next`);
      const { link, code } = await chooseAndShare(page, { opening: true });
      await axe(page, 'share-receipt');
      assert(link.includes('/r#t=') && /^[A-Z0-9]{4}-[A-Z0-9]{4}$/.test(code), `bad link/code: ${link} ${code}`);

      const rctx = await browser.newContext({ locale: 'id-ID' });
      const r = await rctx.newPage();
      await r.goto(link);
      await r.getByRole('heading', { name: 'Masukkan kode akses' }).waitFor();
      assert(!r.url().includes('#t='), 'token left in the address bar');
      await axe(r, 'recipient-code');
      await r.locator('#access-code').fill('AAAA-AAAA');
      await r.getByRole('button', { name: 'Buka catatan' }).click();
      await r.getByRole('alert').waitFor();
      await r.locator('#access-code').fill(code);
      await r.getByRole('button', { name: 'Buka catatan' }).click();
      await r.getByText('Perspektif penerima (fiktif)').waitFor();
      await r.getByText(/Catatan yang disetujui \(versi/).waitFor();
      await assertOpeningDraft(r.locator('.projection'), 'id', 'recipient view');
      await axe(r, 'recipient-view');
      await r.getByRole('button', { name: 'Konfirmasi diterima' }).click();
      await r.getByLabel('Rencana tindak lanjut (fixture)').selectOption('plan-psychologist-followup');
      await r.getByRole('button', { name: 'Selesaikan konsultasi simulasi' }).click();
      await r.getByText(/Status: Konsultasi simulasi selesai/).waitFor();

      await page.goto(`${BASE}/journey`);
      await page.getByText('Konsultasi simulasi selesai').first().waitFor();
      await page.getByText('Rencana tindak lanjut (fiktif)').waitFor();
      await page.getByText('Terkirim atau diterima belum berarti kamu sudah mendapat perawatan. Semua ini simulasi.').waitFor();
      await axe(page, 'journey');
      await page.getByRole('button', { name: 'Aku sudah menyepakati sesi berikutnya (simulasi)' }).click();
      await page.getByText(/Tercatat/).waitFor();
      await page.getByRole('button', { name: 'Cabut akses' }).click();
      await page.getByRole('button', { name: 'Cabut akses' }).click();
      await page.getByText('Akses dicabut').first().waitFor();
      const status = await r.evaluate(async () => (await fetch('/api/v2/recipient/handoff')).status);
      assert(status === 403, `recipient still has access after revoke: ${status}`);
      await ctx.close();
      await rctx.close();
    });

    await check('EN journey: no AI consent → manual note → share → owner revokes before ack → recipient action denied', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID' });
      const page = await ctx.newPage();
      await startSession(page, { english: true, ai: false });
      await useScenario(page, 'Sam: rotating shifts and broken sleep');
      await page.waitForURL('**/ledger');
      await page.getByText('You have not allowed AI processing.', { exact: false }).waitFor();
      await page.getByText('Build it manually from your sentences').click();
      for (let i = 0; i < 3; i++) {
        await page.getByRole('button', { name: 'Add', exact: true }).first().click();
        await page.waitForTimeout(120);
      }
      await page.getByText('Manual, no AI').first().waitFor();
      await page.getByRole('button', { name: /Approve note version/ }).click();
      await page.getByText(/Approved: version/).waitFor();
      await page.getByRole('button', { name: 'Continue to a practical step' }).click();
      await page.waitForURL('**/next');
      const { link, code } = await chooseAndShare(page, { english: true, quotes: false });
      const rctx = await browser.newContext({ locale: 'id-ID' });
      const r = await rctx.newPage();
      // The fixture catalog is down for the recipient at first: the view
      // says so and offers a retry instead of a blank page.
      let catalogDown = true;
      await r.route('**/api/v2/catalog', (route) =>
        catalogDown ? route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":{"code":"capacity"}}' }) : route.continue(),
      );
      await r.goto(link);
      await r.locator('#access-code').fill(code);
      await r.getByRole('button', { name: 'Buka catatan' }).click();
      await r.getByRole('heading', { name: 'Sela belum bisa memuat isi demo' }).waitFor();
      await r.getByRole('alert').filter({ hasText: 'Demo sedang penuh' }).waitFor();
      catalogDown = false;
      await r.getByRole('button', { name: 'Coba lagi' }).click();
      await r.locator('#req-title').waitFor();
      await r.getByText('Perspektif penerima (fiktif)').waitFor();
      assert((await r.getByText('Kata-kata asli').count()) === 0, 'quotes leaked although not selected');
      await page.goto(`${BASE}/journey`);
      await page.getByRole('button', { name: 'Revoke access' }).click();
      await page.getByRole('button', { name: 'Revoke access' }).click();
      await page.getByText('Access revoked').first().waitFor();
      await r.getByRole('button', { name: 'Konfirmasi diterima' }).click();
      await r.getByText('Akses ke catatan ini tidak tersedia.', { exact: false }).waitFor();
      await ctx.close();
      await rctx.close();
    });

    await check('Crisis fixture interrupts before any model call or handoff', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID' });
      const page = await ctx.newPage();
      await startSession(page);
      await useScenario(page, 'Uji jalur bantuan segera (fixture krisis)');
      await page.waitForURL('**/help');
      await page.getByText(/Alur biasa Sela dihentikan untuk sesi ini/).waitFor();
      await page.getByText('Healing119 (Kementerian Kesehatan RI)').waitFor();
      await axe(page, 'help-crisis');
      const session = await page.evaluate(async () => (await (await fetch('/api/v2/session')).json()).data);
      assert(session.safety.interrupted === true && session.ops.length === 0 && !session.story, `crisis state wrong: ${JSON.stringify(session.safety)}`);
      await page.goto(`${BASE}/ledger`);
      await page.getByRole('alert').filter({ hasText: 'Alur biasa dihentikan' }).waitFor();
      await page.getByRole('heading', { name: 'Alur biasa dihentikan untuk sesi ini' }).waitFor();
      assert((await page.getByRole('button', { name: 'Bantu susun catatan dengan AI' }).count()) === 0, 'journey controls offered on an interrupted session');
      await ctx.close();
    });

    await check('Crisis language in a ledger wording edit routes to immediate help and stops the journey', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID' });
      const page = await ctx.newPage();
      await startSession(page, { english: true });
      await useScenario(page, 'Sam: rotating shifts and broken sleep');
      await page.waitForURL('**/ledger');
      await page.getByText('Build it manually from your sentences').click();
      await page.getByRole('button', { name: 'Add', exact: true }).first().click();
      await page.getByText('In the note').first().waitFor();
      await page.getByRole('button', { name: 'Change wording' }).first().click();
      await page.getByLabel('Your wording for this item').fill('aku mau bunuh diri');
      await page.getByRole('button', { name: 'Save', exact: true }).click();
      await page.waitForURL('**/help');
      await page.getByText(/ordinary journey has stopped for this session/).first().waitFor();
      const session = await page.evaluate(async () => (await (await fetch('/api/v2/session')).json()).data);
      assert(session.safety.interrupted === true && !session.ledger, `crisis edit did not interrupt: ${JSON.stringify(session.safety)}`);
      // Every journey page now shows the interrupted state, never its controls.
      await page.getByRole('navigation').getByRole('link', { name: 'Settings' }).waitFor();
      assert((await page.getByRole('navigation').getByRole('link', { name: 'My story' }).count()) === 0, 'story link still offered after a crisis');
      await page.goto(`${BASE}/story`);
      await page.getByRole('heading', { name: 'The ordinary journey has stopped for this session' }).waitFor();
      assert((await page.getByRole('button', { name: 'Use this story' }).count()) === 0, 'a new story can be chosen after a crisis');
      await ctx.close();
    });

    await check('Recipient: a second link opened in the same tab asks for its own code', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID' });
      const page = await ctx.newPage();
      await startSession(page, { english: true, ai: false });
      await useScenario(page, 'Sam: rotating shifts and broken sleep');
      await page.waitForURL('**/ledger');
      await page.getByText('Build it manually from your sentences').click();
      await page.getByRole('button', { name: 'Add', exact: true }).first().click();
      await page.getByText('In the note').first().waitFor();
      await page.getByRole('button', { name: /Approve note version/ }).click();
      await page.getByText(/Approved: version/).waitFor();
      await page.getByRole('button', { name: 'Continue to a practical step' }).click();
      await page.waitForURL('**/next');
      const first = await chooseAndShare(page, { english: true, quotes: false });
      await page.goto(`${BASE}/next`);
      const second = await chooseAndShare(page, { english: true, quotes: false, option: 1, setPrefs: false });
      // The demo's own path: the recipient perspective in the owner's browser.
      const r = await ctx.newPage();
      const posted = [];
      const ownerWrites = [];
      r.on('request', (q) => {
        if (q.url().endsWith('/recipient/sessions')) posted.push(JSON.parse(q.postData()).token);
        if (q.url().endsWith('/api/v2/session')) ownerWrites.push(q.method());
      });
      lastPage = r;
      await r.goto(first.link);
      const title1 = await openNote(r, first.code, 'Open the note');
      // Paste the second link into the same tab.
      await r.goto(second.link);
      await r.getByRole('heading', { name: 'Enter the access code' }).waitFor();
      assert(!r.url().includes('#t='), 'second token left in the address bar');
      assert((await r.locator('#req-title').count()) === 0, 'the first request is still shown for the second link');
      const title2 = await openNote(r, second.code, 'Open the note');
      assert(title1 !== title2, `second link shows the first request: ${title2}`);
      const t1 = first.link.split('#t=')[1];
      const t2 = second.link.split('#t=')[1];
      const order = posted.map((t) => (t === t1 ? '1' : t === t2 ? '2' : '?')).join('');
      assert(/^1+2+$/.test(order), `the second sign-in did not use the second link token: ${order}`);
      // Outside the owner session: no owner expiry, and a local-only language switch.
      assert(!(await r.locator('footer').innerText()).includes('Session ends'), 'owner session expiry shown to the recipient');
      await r.getByRole('button', { name: 'Indonesia' }).click();
      await r.getByRole('heading', { name: 'Perspektif penerima (fiktif)' }).waitFor();
      assert(ownerWrites.length === 0, `recipient perspective called the owner session: ${ownerWrites.join(',')}`);
      const owner = await page.evaluate(async () => (await (await fetch('/api/v2/session')).json()).data);
      assert(owner.language === 'en', `recipient language switch changed the owner session: ${owner.language}`);
      await ctx.close();
    });

    await check('Practice: an AI opening draft is resumed after navigation and can be stopped', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID' });
      const page = await ctx.newPage();
      await startSession(page, { english: true, base: STALL_BASE });
      await useScenario(page, 'Sam: rotating shifts and broken sleep');
      await page.waitForURL('**/ledger');
      await page.getByText('Build it manually from your sentences').click();
      await page.getByRole('button', { name: 'Add', exact: true }).first().click();
      await page.getByText('In the note').first().waitFor();
      await page.getByRole('button', { name: /Approve note version/ }).click();
      await page.getByText(/Approved: version/).waitFor();
      await page.goto(`${STALL_BASE}/practice`);
      await page.getByRole('button', { name: 'Draft with AI' }).click();
      const stop = page.locator('.op-panel').getByRole('button', { name: 'Stop' });
      await stop.waitFor();
      // Leave and come back: the running draft is followed again, not lost.
      await page.getByRole('navigation').getByRole('link', { name: 'Next step' }).click();
      await page.waitForURL('**/next');
      await page.getByRole('link', { name: 'Practise an opening line first' }).click();
      await page.waitForURL('**/practice');
      await stop.waitFor();
      await stop.click();
      await page.getByText('The AI step was stopped. Nothing was saved.').first().waitFor();
      assert((await stop.count()) === 0, 'Stop still shown after stopping');
      const session = await page.evaluate(async () => (await (await fetch('/api/v2/session')).json()).data);
      const ops = session.ops.filter((o) => o.kind === 'opening_draft').map((o) => o.state);
      assert(ops.length === 1 && ops[0] === 'canceled' && !session.opening, `opening draft not canceled: ${JSON.stringify(ops)}`);
      // A new draft can start at once: nothing is left blocking it.
      await page.getByRole('button', { name: 'Draft with AI' }).click();
      await stop.waitFor();
      await stop.click();
      await page.getByText('The AI step was stopped. Nothing was saved.').first().waitFor();
      await ctx.close();
    });

    await check('Ledger: an AI step whose event stream is refused stays followable and stoppable', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID' });
      const page = await ctx.newPage();
      // Every events request is refused (e.g. a 429 from a proxy) while the step runs.
      await page.route('**/api/v2/operations/*/events', (route) => route.fulfill({ status: 429, contentType: 'application/json', body: '{"error":{"code":"rate_limited"}}' }));
      await startSession(page, { english: true, base: STALL_BASE });
      await useScenario(page, 'Sam: rotating shifts and broken sleep');
      await page.waitForURL('**/ledger');
      await page.getByRole('button', { name: 'Help draft my note with AI' }).click();
      const panel = page.locator('.op-panel');
      await panel.getByText('Reconnecting to the AI step…').waitFor();
      assert((await page.getByRole('alert').filter({ hasText: 'AI model is unavailable' }).count()) === 0, 'a refused stream was reported as the model being unavailable');
      assert((await page.getByRole('button', { name: /^(Try again|Help draft my note with AI)$/ }).count()) === 0, 'a second AI step offered while one still runs');
      await panel.getByRole('button', { name: 'Stop' }).click();
      await page.getByText('The AI step was stopped. Nothing was saved.').first().waitFor();
      const session = await page.evaluate(async () => (await (await fetch('/api/v2/session')).json()).data);
      const ops = session.ops.filter((o) => o.kind === 'ledger_draft').map((o) => o.state);
      assert(ops.length === 1 && ops[0] === 'canceled', `ledger draft not canceled: ${JSON.stringify(ops)}`);
      await ctx.close();
    });

    await check('Share: a no-AI opening over AI-drafted wording keeps that wording labelled; a failed preview can be retried', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID' });
      const page = await ctx.newPage();
      await startSession(page, { english: true });
      await useScenario(page, 'Sam: rotating shifts and broken sleep');
      await page.waitForURL('**/ledger');
      await page.getByRole('button', { name: 'Help draft my note with AI' }).click();
      await page.getByText(/Draft ready:/).waitFor({ timeout: 15000 });
      await confirmAll(page, 'Looks right');
      await page.getByRole('button', { name: /Approve note version/ }).click();
      await page.getByText(/Approved: version/).waitFor();
      await page.goto(`${BASE}/practice`);
      await page.getByRole('button', { name: 'Draft without AI' }).click();
      const lines = page.getByRole('region', { name: 'Opening lines' });
      await lines.locator('ol.lines').waitFor();
      // Sela's template strings together the test double's wording.
      await lines.getByText(TEST_DOUBLE.en, { exact: true }).waitFor();
      assert((await lines.getByText('Manual, no AI', { exact: true }).count()) === 0, 'Practice: template over AI wording badged "Manual, no AI"');
      await page.goto(`${BASE}/next`);
      await page.locator('#pref-language').selectOption('en');
      await page.getByLabel('Format', { exact: true }).selectOption('online');
      await page.getByRole('button', { name: 'Show options' }).click();
      const choose = page.locator('article.option').first().getByRole('button', { name: /^Choose / }).first();
      await choose.waitFor();
      await choose.click();
      await page.waitForURL('**/share**');
      await page.locator('.projection').waitFor();
      // The preview for the new selection fails once (connection reset).
      let failNext = true;
      await page.route('**/api/v2/handoffs/preview', (route) => {
        if (!failNext) return route.continue();
        failNext = false;
        return route.abort('connectionreset');
      });
      await page.getByLabel('Opening lines', { exact: true }).check();
      await page.getByRole('alert').filter({ hasText: 'Cannot reach the server' }).waitFor();
      assert(!(await page.getByLabel(/^I allow Sela to share/).isEnabled()), 'consent offered without a preview of the current selection');
      await page.getByRole('button', { name: 'Try again' }).click();
      const projection = page.locator('.projection');
      await projection.getByRole('heading', { name: OPENING_TEMPLATE.en }).waitFor();
      await projection.locator('.opening-provenance').getByText(TEST_DOUBLE.en, { exact: true }).waitFor();
      assert((await projection.getByRole('heading', { name: OPENING_OWN.en }).count()) === 0, "share preview: template opening titled as the user's own");
      assert((await projection.getByText('Manual, no AI', { exact: true }).count()) === 0, 'share preview: AI wording badged "Manual, no AI"');
      await page.getByLabel(/^I allow Sela to share/).check();
      assert(await page.getByRole('button', { name: 'Share this note' }).isEnabled(), 'sharing still blocked after the preview arrived');
      await ctx.close();
    });

    await check('A session deleted and started again in another tab replaces the old tab view', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID' });
      const a = await ctx.newPage();
      await startSession(a, { english: true, ai: false });
      await useScenario(a, 'Sam: rotating shifts and broken sleep');
      await a.waitForURL('**/ledger');
      await a.getByText('Build it manually from your sentences').click();
      for (let i = 0; i < 3; i++) {
        await a.getByRole('button', { name: 'Add', exact: true }).first().click();
        await a.waitForTimeout(120);
      }
      await a.getByText('In the note').first().waitFor();
      const b = await ctx.newPage();
      lastPage = b;
      await b.goto(`${BASE}/settings`);
      await b.getByRole('button', { name: 'Delete all data now' }).click();
      await b.getByRole('button', { name: 'Delete all data now' }).click();
      await b.getByText('All session data has been deleted.').waitFor();
      await b.getByRole('button', { name: 'Start a new session' }).click();
      await b.waitForURL('**/start');
      await b.getByLabel(/I am 18 or older|Saya berusia 18 tahun/).check();
      await b.getByRole('button', { name: /I understand, continue|Saya mengerti, lanjutkan/ }).click();
      await b.waitForURL('**/consent');
      // Back in tab A: returning to the tab reloads the view, which is now
      // the new session (a lower version), never the deleted one.
      lastPage = a;
      await a.bringToFront();
      await a.evaluate(() => window.dispatchEvent(new Event('focus')));
      await a.getByText(/There is no story yet|Belum ada cerita/).waitFor();
      assert((await a.locator('.ledger-item').count()) === 0, "the deleted session's note is still shown");
      await ctx.close();
    });

    await check('Practice text with contact details is rejected; canary text never reaches logs', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID' });
      const page = await ctx.newPage();
      await startSession(page);
      await page.getByLabel('Ceritamu (latihan)').fill(`Tokoh ${CANARY} merasa lelah. Hubungi dia di 0812-3456-7890.`);
      await page.getByRole('button', { name: 'Simpan cerita ini' }).click();
      await page.getByText(/tampak memuat kontak/).waitFor();
      await page.getByLabel('Ceritamu (latihan)').fill(`Tokoh ${CANARY} merasa lelah dengan pekerjaannya dan ingin bicara dengan konselor.`);
      await page.getByRole('button', { name: 'Simpan cerita ini' }).click();
      await page.waitForURL('**/ledger');
      await page.getByRole('button', { name: 'Bantu susun catatan dengan AI' }).click();
      await page.getByText(/Draf siap:/).waitFor({ timeout: 15000 });
      await ctx.close();
    });

    await check('Settings: delete everything ends the session', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID' });
      const page = await ctx.newPage();
      await startSession(page);
      await page.goto(`${BASE}/settings`);
      await axe(page, 'settings');
      await page.getByRole('button', { name: 'Hapus semua data sekarang' }).click();
      await page.getByRole('button', { name: 'Hapus semua data sekarang' }).click();
      await page.getByText('Semua data sesi sudah dihapus.').waitFor();
      const status = await page.evaluate(async () => (await fetch('/api/v2/session')).status);
      assert(status === 401, `session still exists: ${status}`);
      await ctx.close();
    });

    await check('Public pages are accessible and fit a 390px viewport', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID', viewport: { width: 390, height: 844 } });
      const page = await ctx.newPage();
      for (const path of ['/', '/start', '/privacy', '/help', '/research']) {
        await page.goto(`${BASE}${path}`);
        await page.locator('main h1').first().waitFor();
        await axe(page, `public${path}`);
        await noHorizontalScroll(page, path);
      }
      await startSession(page);
      await noHorizontalScroll(page, '/story (mobile)');
      await ctx.close();
    });

    await check('Keyboard-only start: skip link, checkbox and submit', async () => {
      const ctx = await browser.newContext({ locale: 'id-ID' });
      const page = await ctx.newPage();
      await page.goto(`${BASE}/start`);
      await page.keyboard.press('Tab');
      const skip = await page.evaluate(() => document.activeElement?.textContent ?? '');
      assert(skip.includes('Lewati ke isi utama'), `first tab stop is not the skip link: ${skip}`);
      await page.getByLabel(/Saya berusia 18 tahun/).focus();
      await page.keyboard.press('Space');
      await page.getByRole('button', { name: 'Saya mengerti, lanjutkan' }).focus();
      await page.keyboard.press('Enter');
      await page.waitForURL('**/consent');
      await ctx.close();
    });

    await browser.close();
  } finally {
    for (const p of [sela, fake, stallSela, stallFake]) p.kill('SIGTERM');
    await new Promise((r) => setTimeout(r, 300));
  }
  await check('Operational logs contain no story canary, tokens or codes', async () => {
    assert(!serverLog.includes(CANARY), 'canary story text found in server logs');
    assert(!/"(token|code|text|story|wording|quote)":/.test(serverLog), 'content-like fields found in logs');
  });
  const outDir = join(ROOT, 'e2e', 'results');
  mkdirSync(outDir, { recursive: true });
  writeFileSync(join(outDir, 'e2e-report.json'), JSON.stringify({ ranAt: new Date().toISOString(), results, a11y: a11yReport }, null, 2));
  const failed = results.filter((r) => !r.ok);
  console.log(`\n${results.length - failed.length}/${results.length} passed`);
  if (failed.length) process.exit(1);
}

main().catch((err) => {
  console.error(err);
  console.error(serverLog.slice(-4000));
  process.exit(1);
});
