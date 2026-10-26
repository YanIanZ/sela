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

async function startSession(page, { english = false, ai = true, scanConsent = false } = {}) {
  lastPage = page;
  await page.goto(`${BASE}/`);
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

async function chooseAndShare(page, { english = false, quotes = true } = {}) {
  await page.locator('#pref-language').selectOption(english ? 'en' : 'id');
  await page.getByLabel('Format', { exact: true }).selectOption('online');
  await page.getByRole('button', { name: english ? 'Show options' : 'Tampilkan pilihan' }).click();
  const choose = page.getByRole('button', { name: english ? /^Choose / : /^Pilih / }).first();
  await choose.waitFor();
  await choose.click();
  await page.waitForURL('**/share**');
  if (quotes) await page.getByLabel(english ? 'My original words next to each item' : 'Kata-kataku asli di samping tiap butir').check();
  await page.getByText(english ? 'Preview for the recipient' : 'Pratinjau untuk penerima').waitFor();
  await page.getByLabel(english ? /^I allow Sela to share/ : /^Saya mengizinkan Sela/).check();
  await page.getByRole('button', { name: english ? 'Share this note' : 'Bagikan catatan ini' }).click();
  await page.getByText(english ? 'Simulated request sent' : 'Permintaan simulasi terkirim').waitFor();
  const link = await page.getByLabel(english ? 'Link for the recipient' : 'Tautan untuk penerima').inputValue();
  const code = await page.getByLabel(english ? 'Access code' : 'Kode akses').inputValue();
  return { link, code };
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
  try {
    await waitFor(`http://127.0.0.1:${LLM_PORT}/health`);
    await waitFor(`${BASE}/api/v2/ready`);
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
      const { link, code } = await chooseAndShare(page);
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
      await r.goto(link);
      await r.locator('#access-code').fill(code);
      await r.getByRole('button', { name: 'Buka catatan' }).click();
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
    sela.kill('SIGTERM');
    fake.kill('SIGTERM');
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
