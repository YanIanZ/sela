// Accessibility audit beyond axe (docs/validation/ACCESSIBILITY_AUDIT_2026-11.md).
// Synthetic data only. Probes keyboard order, visible focus, focus after
// route changes and dialogs, landmarks and headings, live regions, form
// labels, 320 px reflow, 200 % text, reduced motion and contrast in light
// and dark. It never approves a reading item.
//
//   E2E_BASE_URL=http://127.0.0.1:8094 \
//   OPERATOR_USERNAME=… OPERATOR_PASSWORD=… OPERATOR_TOTP_SECRET=… \
//   PLAYWRIGHT_BROWSERS_PATH=… node e2e/a11y-audit.e2e.mjs
//
// Without operator credentials only the user app is audited. The script
// never prints the password or the TOTP secret.
import { chromium } from "playwright";
import { createHmac } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const OUT = join(ROOT, "e2e/results/a11y-audit");
mkdirSync(OUT, { recursive: true });
const axeSource = readFileSync(join(ROOT, "web/node_modules/axe-core/axe.min.js"), "utf8");
const base = process.env.E2E_BASE_URL ?? "http://127.0.0.1:8094";
const opUser = process.env.OPERATOR_USERNAME;
const opPass = process.env.OPERATOR_PASSWORD;
const opSecret = process.env.OPERATOR_TOTP_SECRET;

const results = [];
function check(area, name, ok, detail = "") {
  results.push({ area, name, ok, detail });
  console.log(`${ok ? "PASS" : "FAIL"} [${area}] ${name}${detail ? " — " + detail : ""}`);
}

function base32(text) {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = 0;
  let value = 0;
  const out = [];
  for (const char of text.replace(/=+$/, "").replace(/[\s-]/g, "").toUpperCase()) {
    value = (value << 5) | alphabet.indexOf(char);
    bits += 5;
    if (bits >= 8) {
      out.push((value >>> (bits - 8)) & 0xff);
      bits -= 8;
    }
  }
  return Buffer.from(out);
}

function totp(secret, now = Date.now()) {
  const buf = Buffer.alloc(8);
  buf.writeBigUInt64BE(BigInt(Math.floor(now / 1000 / 30)));
  const d = createHmac("sha1", base32(secret)).update(buf).digest();
  const o = d[d.length - 1] & 0x0f;
  return String((d.readUInt32BE(o) & 0x7fffffff) % 1_000_000).padStart(6, "0");
}

async function settle(page) {
  await page.waitForLoadState("networkidle").catch(() => {});
  await page.evaluate(
    () => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))),
  );
}

async function axe(page, area, name, scheme) {
  await page.evaluate(axeSource);
  const res = await page.evaluate(async () =>
    axe.run(document, { runOnly: { type: "tag", values: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"] } }),
  );
  const v = res.violations.filter((x) => ["serious", "critical"].includes(x.impact));
  check(area, `axe ${scheme}: ${name}`, v.length === 0, v.map((x) => `${x.id}(${x.nodes.length})`).join(", "));
}

/** Structure: landmarks, one h1, no skipped heading levels, title. */
async function structure(page, area, name) {
  const s = await page.evaluate(() => {
    const vis = (el) => {
      const r = el.getBoundingClientRect();
      const st = getComputedStyle(el);
      return st.display !== "none" && st.visibility !== "hidden" && (r.width > 0 || r.height > 0 || el.tabIndex === -1);
    };
    const hs = [...document.querySelectorAll("h1,h2,h3,h4,h5,h6")].filter(vis).map((h) => Number(h.tagName[1]));
    const skips = [];
    for (let i = 1; i < hs.length; i++) if (hs[i] > hs[i - 1] + 1) skips.push(`h${hs[i - 1]}→h${hs[i]}`);
    const navs = [...document.querySelectorAll("nav")];
    return {
      title: document.title,
      mains: document.querySelectorAll("main,[role=main]").length,
      h1: hs.filter((n) => n === 1).length,
      skips,
      unlabelledNavs: navs.length > 1 ? navs.filter((n) => !n.getAttribute("aria-label") && !n.getAttribute("aria-labelledby")).length : 0,
      lang: document.documentElement.lang,
      liveRegions: document.querySelectorAll("[role=status],[role=alert],[aria-live]").length,
    };
  });
  check(area, `${name}: one main landmark`, s.mains === 1, String(s.mains));
  check(area, `${name}: exactly one visible h1`, s.h1 === 1, String(s.h1));
  check(area, `${name}: no skipped heading levels`, s.skips.length === 0, s.skips.join(" "));
  check(area, `${name}: navs labelled when more than one`, s.unlabelledNavs === 0, String(s.unlabelledNavs));
  check(area, `${name}: page language set`, /^(id|en)/.test(s.lang), s.lang);
  check(area, `${name}: document title`, s.title.trim().length > 0, s.title);
  return s;
}

/** Tab through the page and check a visible focus indicator on each stop. */
async function keyboard(page, area, name, stops = 30) {
  await page.evaluate(() => {
    document.activeElement?.blur?.();
    window.scrollTo(0, 0);
  });
  await page.mouse.click(1, 1).catch(() => {});
  const seen = [];
  const invisible = [];
  const obscured = [];
  for (let i = 0; i < stops; i++) {
    await page.keyboard.press("Tab");
    const info = await page.evaluate(() => {
      const el = document.activeElement;
      if (!el || el === document.body) return null;
      const st = getComputedStyle(el);
      const outline = st.outlineStyle !== "none" && parseFloat(st.outlineWidth) >= 1;
      const shadow = st.boxShadow && st.boxShadow !== "none";
      const r = el.getBoundingClientRect();
      const cx = Math.min(Math.max(r.left + r.width / 2, 0), innerWidth - 1);
      const cy = Math.min(Math.max(r.top + Math.min(r.height / 2, 10), 0), innerHeight - 1);
      const top = document.elementFromPoint(cx, cy);
      const covered = top && top !== el && !el.contains(top) && !top.contains(el);
      const label = (el.getAttribute("aria-label") || el.textContent || el.getAttribute("name") || el.tagName).trim().replace(/\s+/g, " ").slice(0, 40);
      return { label: `${el.tagName.toLowerCase()}:${label}`, visible: outline || shadow, covered: Boolean(covered), coveredBy: covered ? `${top.tagName.toLowerCase()}.${top.className}`.slice(0, 50) : "" };
    });
    if (!info) continue;
    if (seen.length && seen[0] === info.label && i > 2) break;
    seen.push(info.label);
    if (!info.visible) invisible.push(info.label);
    if (info.covered) obscured.push(`${info.label} under ${info.coveredBy}`);
  }
  check(area, `${name}: keyboard reaches controls`, seen.length > 0, `${seen.length} stops; first: ${seen.slice(0, 3).join(" | ")}`);
  check(area, `${name}: visible focus indicator on every stop`, invisible.length === 0, invisible.slice(0, 5).join(" | "));
  check(area, `${name}: focused element not obscured`, obscured.length === 0, obscured.slice(0, 3).join(" | "));
  return seen;
}

async function reflow(page, area, name, path) {
  await page.setViewportSize({ width: 320, height: 720 });
  await page.goto(base + path);
  await settle(page);
  const over = await page.evaluate(() => {
    const w = document.documentElement.clientWidth;
    const wide = [...document.querySelectorAll("body *")]
      .filter((el) => {
        const r = el.getBoundingClientRect();
        return r.right > w + 1 && getComputedStyle(el).position !== "fixed" && r.width > 0;
      })
      .slice(0, 4)
      .map((el) => `${el.tagName.toLowerCase()}.${String(el.className).slice(0, 30)}`);
    return { scroll: document.documentElement.scrollWidth, w, wide };
  });
  check(area, `${name}: 320 px reflow without horizontal scroll`, over.scroll <= over.w + 1, `${over.scroll}/${over.w} ${over.wide.join(" ")}`);
  await page.screenshot({ path: join(OUT, `${area}-${name.replace(/\W+/g, "-")}-320.png`), fullPage: true });
  // 200 % text: double the root font size at a 640 px layout (the same CSS
  // pixels as 200 % zoom on a 1280 px window).
  await page.setViewportSize({ width: 640, height: 900 });
  await page.evaluate(() => {
    document.documentElement.style.fontSize = "200%";
  });
  await settle(page);
  const zoom = await page.evaluate(() => {
    const w = document.documentElement.clientWidth;
    const clipped = [...document.querySelectorAll("body *")]
      .filter((el) => {
        const st = getComputedStyle(el);
        // Visually hidden text (1 px clip boxes) is meant to be clipped.
        if (el.clientWidth <= 2 || st.clip !== "auto" || st.clipPath !== "none") return false;
        return (st.overflowX === "hidden" || st.overflow === "hidden") && el.scrollWidth > el.clientWidth + 2 && el.textContent.trim().length > 0;
      })
      .slice(0, 4)
      .map((el) => `${el.tagName.toLowerCase()}.${String(el.className).slice(0, 30)}`);
    return { scroll: document.documentElement.scrollWidth, w, clipped };
  });
  check(area, `${name}: 200% text without horizontal scroll`, zoom.scroll <= zoom.w + 1, `${zoom.scroll}/${zoom.w}`);
  check(area, `${name}: 200% text without clipped text boxes`, zoom.clipped.length === 0, zoom.clipped.join(" "));
  await page.evaluate(() => {
    document.documentElement.style.fontSize = "";
  });
  await page.setViewportSize({ width: 1280, height: 900 });
}

async function reducedMotion(page, area, name) {
  const moving = await page.evaluate(() =>
    [...document.querySelectorAll("*")]
      .filter((el) => {
        const st = getComputedStyle(el);
        const anim = st.animationName !== "none" && parseFloat(st.animationDuration) > 0.01 && st.animationIterationCount !== "0";
        const trans = st.transitionDuration.split(",").some((d) => parseFloat(d) > 0.01) && st.transitionProperty !== "none";
        return anim || trans;
      })
      .slice(0, 5)
      .map((el) => `${el.tagName.toLowerCase()}.${String(el.className).slice(0, 30)}`),
  );
  check(area, `${name}: no animation or transition with reduced motion`, moving.length === 0, moving.join(" "));
}

async function auditPage(ctxs, area, name, path, prep) {
  const { light, dark, motion } = ctxs;
  await light.goto(base + path);
  await settle(light);
  if (prep) await prep(light);
  await structure(light, area, name);
  await axe(light, area, name, "light");
  await keyboard(light, area, name);
  await light.screenshot({ path: join(OUT, `${area}-${name.replace(/\W+/g, "-")}-light.png`), fullPage: true });
  await dark.goto(base + path);
  await settle(dark);
  if (prep) await prep(dark);
  await axe(dark, area, name, "dark");
  await dark.screenshot({ path: join(OUT, `${area}-${name.replace(/\W+/g, "-")}-dark.png`), fullPage: true });
  await motion.goto(base + path);
  await settle(motion);
  await reducedMotion(motion, area, name);
  await reflow(light, area, name, path);
}

/** Client-side navigation must move focus to the new page, not leave it on the link or body. */
async function routeFocus(page, area, linkName, expect) {
  const link = page.getByRole("link", { name: linkName, exact: true }).first();
  await link.focus();
  await page.keyboard.press("Enter");
  await settle(page);
  await page.waitForTimeout(200);
  const where = await page.evaluate(() => {
    const el = document.activeElement;
    return el ? `${el.tagName.toLowerCase()}#${el.id}:${(el.textContent || "").trim().slice(0, 30)}` : "none";
  });
  check(area, `route change via "${linkName}" moves focus to the new page`, expect(where), where);
}

const browser = await chromium.launch();
async function contexts(storageState) {
  const mk = (opts) => browser.newContext({ locale: "id-ID", viewport: { width: 1280, height: 900 }, storageState, ...opts });
  const [l, d, m] = await Promise.all([
    mk({ colorScheme: "light" }),
    mk({ colorScheme: "dark" }),
    mk({ reducedMotion: "reduce" }),
  ]);
  return { ctx: [l, d, m], light: await l.newPage(), dark: await d.newPage(), motion: await m.newPage() };
}

try {
  // ---------------------------------------------------------------- user app
  const anon = await contexts();
  const publicPages = [
    ["dashboard", "/"],
    ["help", "/help"],
    ["reading", "/reading"],
    ["about", "/about"],
    ["policies", "/policies"],
    ["signin", "/signin"],
    ["display settings", "/settings/display"],
  ];
  for (const [name, path] of publicPages) await auditPage(anon, "user", name, path);

  // Skip link and route focus on the public dashboard.
  await anon.light.goto(base + "/");
  await settle(anon.light);
  await anon.light.keyboard.press("Tab");
  const first = await anon.light.evaluate(() => document.activeElement?.textContent?.trim());
  check("user", "first Tab stop is the skip link", first === "Lewati ke konten", first);
  await anon.light.keyboard.press("Enter");
  const afterSkip = await anon.light.evaluate(() => document.activeElement?.id);
  check("user", "skip link moves focus to main", afterSkip === "main", afterSkip);
  await anon.light.goto(base + "/help");
  await settle(anon.light);
  await routeFocus(anon.light, "user", "Tentang Sela", (w) => w.startsWith("main#main") || w.startsWith("h1"));

  // Sign-in error association.
  await anon.light.goto(base + "/signin");
  await settle(anon.light);
  const submit = anon.light.getByRole("button", { name: "Masuk", exact: true });
  if (await submit.count()) {
    // Empty fields use native required validation; probe the server error
    // path with synthetic wrong credentials instead.
    await anon.light.getByLabel("Nama pengguna").fill("audit-nobody");
    await anon.light.getByLabel("Kata sandi", { exact: true }).fill("not-a-real-password-123");
    await submit.click();
    await anon.light.waitForTimeout(600);
    const err = await anon.light.evaluate(() => {
      const invalid = [...document.querySelectorAll("[aria-invalid=true]")];
      return {
        invalid: invalid.length,
        described: invalid.filter((el) => (el.getAttribute("aria-describedby") || "").split(" ").some((id) => id && document.getElementById(id)?.textContent.trim())).length,
        focus: document.activeElement?.tagName + "#" + document.activeElement?.id,
      };
    });
    check("user", "sign-in: wrong credentials mark fields invalid", err.invalid > 0, JSON.stringify(err));
    check("user", "sign-in: invalid fields describe their error", err.invalid > 0 && err.described === err.invalid, JSON.stringify(err));
  }

  // Guest onboarding (synthetic) to reach owner pages.
  const p = anon.light;
  await p.goto(base + "/start/age");
  await settle(p);
  await p.getByRole("radio", { name: "18 tahun ke atas" }).click();
  await p.getByRole("button", { name: "Lanjut" }).click();
  await p.getByRole("radio", { name: /Mulai sebagai tamu/ }).click();
  await p.getByRole("radio", { name: /Simpan di Catatanku/ }).click();
  await p.getByLabel(/Aku sudah membaca dan menyetujui/).click();
  await p.getByLabel(/Aku mengerti cara penyimpanan/).click();
  await p.getByRole("button", { name: "Mulai sebagai tamu", exact: true }).click();
  await p.waitForURL("**/start/ai");
  await settle(p);
  await structure(p, "user", "AI consent");
  await axe(p, "user", "AI consent", "light");
  const state = await anon.ctx[0].storageState();
  const guest = await contexts(state);
  const ownerPages = [
    ["home", "/home"],
    ["talk", "/talk"],
    ["notes", "/notes"],
    ["write note", "/notes/write"],
    ["check-in", "/notes/check-in"],
    ["support", "/support"],
    ["privacy", "/settings/privacy"],
    ["account", "/settings/account"],
  ];
  for (const [name, path] of ownerPages) await auditPage(guest, "user", name, path);

  // Menu dialog: focus moves in, Escape closes, focus returns.
  const g = guest.light;
  await g.goto(base + "/home");
  await settle(g);
  const menuBtn = g.getByRole("button", { name: /^Menu/ }).first();
  if (await menuBtn.count()) {
    await menuBtn.focus();
    await g.keyboard.press("Enter");
    await g.waitForTimeout(250);
    const inDialog = await g.evaluate(() => Boolean(document.activeElement?.closest("dialog,[role=dialog]")));
    check("user", "menu: focus moves into the dialog", inDialog);
    await g.keyboard.press("Escape");
    await g.waitForTimeout(250);
    const back = await g.evaluate(() => document.activeElement?.textContent?.trim());
    check("user", "menu: Escape returns focus to the trigger", /^Menu/.test(back ?? ""), back);
  } else {
    check("user", "menu button found at 1280 px", false, "no Menu button");
  }
  await routeFocus(g, "user", "Catatanku", (w) => w.startsWith("main#main") || w.startsWith("h1"));

  // Note form: an unavailable save stays focusable and says why (criteria 4.1.2).
  await g.goto(base + "/notes/write");
  await settle(g);
  const save = g.getByRole("button", { name: "Simpan catatan", exact: true });
  if (await save.count()) {
    const info = await save.evaluate((el) => ({
      disabled: el.disabled,
      aria: el.getAttribute("aria-disabled"),
      reason: (el.getAttribute("aria-describedby") || "").split(" ").map((id) => document.getElementById(id)?.textContent.trim()).filter(Boolean).join(" "),
    }));
    check("user", "write note: unavailable save is focusable with aria-disabled and a reason", !info.disabled && info.aria === "true" && info.reason.length > 0, JSON.stringify(info));
  }
  await guest.ctx[0].request.delete(base + "/api/v3/me", { data: { confirm: true }, headers: { "X-Sela-Request": "1" } }).catch(() => {});
  for (const c of [...anon.ctx, ...guest.ctx]) await c.close();

  // ---------------------------------------------------------------- operator
  if (opUser && opPass && opSecret) {
    const opCtx = await contexts();
    const op = opCtx.light;
    await op.goto(base + "/operator");
    await settle(op);
    await structure(op, "operator", "sign-in");
    await axe(op, "operator", "sign-in", "light");
    await op.getByLabel(/Nama pengguna/).fill(opUser);
    await op.getByLabel(/Kata sandi/).fill(opPass);
    await op.getByLabel(/Kode/).fill(totp(opSecret));
    await op.getByRole("button", { name: /^Masuk/ }).click();
    await op.getByRole("heading", { level: 1, name: "Beranda" }).waitFor();
    const opState = await opCtx.ctx[0].storageState();
    const staff = await contexts(opState);
    for (const [name, path] of [
      ["home", "/operator"],
      ["network", "/operator/network"],
      ["moderation", "/operator/moderation"],
      ["content", "/operator/content"],
      ["reading", "/operator/reading"],
      ["system", "/operator/system"],
    ]) {
      await auditPage(staff, "operator", name, path);
    }
    const s = staff.light;
    await s.goto(base + "/operator");
    await settle(s);
    await routeFocus(s, "operator", "Tinjauan bacaan", (w) => w.startsWith("h1"));

    // Tinjauan bacaan: open an item by keyboard; focus must land on it.
    await s.goto(base + "/operator/reading");
    await settle(s);
    const row = s.getByRole("button", { name: /Berpijak saat badai emosi/ });
    await row.focus();
    await s.keyboard.press("Enter");
    await s.waitForTimeout(250);
    const opened = await s.evaluate(() => `${document.activeElement?.tagName}:${document.activeElement?.textContent?.trim().slice(0, 40)}`);
    check("operator", "reading: opening an item moves focus to its heading", opened.startsWith("H3"), opened);
    const group = s.getByRole("group", { name: /Daftar periksa aksesibilitas manual/ });
    check("operator", "reading: manual checklist is a named group of checkboxes", (await group.getByRole("checkbox").count()) === 3);
    const region = s.getByRole("region", { name: "Pemeriksaan aksesibilitas otomatis" });
    check("operator", "reading: automatic checks are a named region", (await region.count()) === 1);
    check(
      "operator",
      "reading: override reason is a labelled, described textarea",
      (await s.getByRole("textbox", { name: "Alasan pengecualian keterbacaan (wajib)" }).count()) === 1,
    );
    check(
      "operator",
      "reading: approve stays disabled before the checklist (nothing approved)",
      (await s.getByRole("button", { name: "Setujui untuk publikasi" }).getAttribute("aria-disabled")) === "true",
    );
    await structure(s, "operator", "reading (item open)");
    await axe(s, "operator", "reading (item open)", "light");
    await keyboard(s, "operator", "reading (item open)", 60);
    await s.screenshot({ path: join(OUT, "operator-reading-open-light.png"), fullPage: true });
    const sd = staff.dark;
    await sd.goto(base + "/operator/reading");
    await settle(sd);
    await sd.getByRole("button", { name: /Berpijak saat badai emosi/ }).click();
    await sd.waitForTimeout(200);
    await axe(sd, "operator", "reading (item open)", "dark");
    await sd.screenshot({ path: join(OUT, "operator-reading-open-dark.png"), fullPage: true });
    await s.setViewportSize({ width: 320, height: 720 });
    await s.waitForTimeout(150);
    const w320 = await s.evaluate(() => [document.documentElement.scrollWidth, document.documentElement.clientWidth]);
    check("operator", "reading (item open): 320 px reflow", w320[0] <= w320[1] + 1, w320.join("/"));
    await s.screenshot({ path: join(OUT, "operator-reading-open-320.png"), fullPage: true });
    for (const c of [...opCtx.ctx, ...staff.ctx]) await c.close();
  } else {
    console.log("operator credentials not set: operator audit skipped");
  }
} catch (err) {
  check("run", "audit completed", false, String(err?.message ?? err).slice(0, 300));
} finally {
  await browser.close();
}

writeFileSync(join(OUT, "results.json"), JSON.stringify(results, null, 2));
const failed = results.filter((r) => !r.ok);
console.log(`\n${results.length - failed.length}/${results.length} checks passed`);
process.exit(failed.length ? 1 : 0);
