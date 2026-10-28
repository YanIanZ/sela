// Requires an already running v3 server with a database. Synthetic data only.
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
const require = createRequire(import.meta.url);
const { chromium } = require("playwright");
const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const RESULTS = process.env.E2E_RESULTS ?? join(ROOT, "e2e/results/v3-core");

import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
mkdirSync(RESULTS, { recursive: true });
const base = process.env.E2E_BASE_URL ?? "http://127.0.0.1:8080";
if(process.env.E2E_LIVE_AI!=="1")throw new Error("Set E2E_LIVE_AI=1 to run the synthetic live-provider journey.");
const browser = await chromium.launch({
  ...(process.env.CHROMIUM_PATH
    ? { executablePath: process.env.CHROMIUM_PATH }
    : {}),
  headless: true,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});
const results = [];
let currentPage;
async function audit(page, name) {
  await page.evaluate(
    readFileSync(
      require.resolve("axe-core/axe.min.js", { paths: [join(ROOT, "web")] }),
      "utf8",
    ),
  );
  const violations = await page.evaluate(async () => {
    const x = await axe.run(document, {
      runOnly: {
        type: "tag",
        values: ["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"],
      },
    });
    return x.violations
      .filter((v) => ["serious", "critical"].includes(v.impact))
      .map((v) => ({
        id: v.id,
        impact: v.impact,
        nodes: v.nodes.map((n) => n.target),
      }));
  });
  if (violations.length)
    throw new Error(name + ": " + JSON.stringify(violations));
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth > window.innerWidth + 1,
  );
  if (overflow) throw new Error(name + ": horizontal overflow");
  results.push({ page: name, a11y: "pass", overflow: false });
}
try {
  const context = await browser.newContext({
    viewport: { width: 1365, height: 900 },
  });
  await context.addInitScript(
    () =>
      !localStorage.getItem("sela.lang") &&
      localStorage.setItem("sela.lang", "id"),
  );
  const p = await context.newPage();
  currentPage = p;
  p.on("pageerror", (e) => console.log("BROWSER ERROR", e.message));
  await p.goto(base + "/");
  await p.getByRole("button", { name: "Mulai tanpa akun" }).waitFor();
  await audit(p, "welcome desktop ID");
  await p.screenshot({
    path: join(RESULTS, "welcome-desktop.png"),
    fullPage: true,
  });
  await p.getByLabel("Kelompok usia").selectOption("18_plus");
  await p.getByLabel(/Saya memahami penyimpanan/).check();
  await p.getByRole("button", { name: "Mulai tanpa akun" }).click();
  await p.waitForURL("**/home");
  await audit(p, "home desktop ID");
  await p.screenshot({
    path: join(RESULTS, "home-desktop.png"),
    fullPage: true,
  });
  await p.goto(base + "/notes");
  await p.getByRole("button", { name: "Tulis catatan baru" }).click();
  await p.getByLabel("Judul (opsional)").fill("Synthetic browser note");
  await p
    .getByLabel("Tulisanmu")
    .fill("Synthetic browser test: saya sedang mengatur waktu belajar.");
  await p.getByLabel(/Simpan tulisan ini/).check();
  await p.getByRole("button", { name: "Simpan catatan", exact: true }).click();
  await p.getByRole("status").filter({ hasText: "Tersimpan" }).waitFor();
  await p.reload();
  await p.getByLabel("Tulisanmu").waitFor();
  if (
    (await p.getByLabel("Tulisanmu").inputValue()) !==
    "Synthetic browser test: saya sedang mengatur waktu belajar."
  )
    throw new Error("note lost after reload");
  await audit(p, "note edit desktop ID");
  results.push({ flow: "note create and reload", passed: true });
  await p.goto(base + "/checkin");
  await p.getByRole("radio", { name: "Cukup baik", exact: true }).check();
  await p.getByLabel(/Simpan tulisan ini/).check();
  await p.getByRole("button", { name: "Simpan catatan", exact: true }).click();
  await p.getByRole("status").filter({ hasText: "Tersimpan" }).waitFor();
  await p.reload();
  await p.getByRole("radio", { name: "Cukup baik", exact: true }).waitFor();
  if (
    !(await p
      .getByRole("radio", { name: "Cukup baik", exact: true })
      .isChecked())
  )
    throw new Error("checkin lost");
  results.push({ flow: "checkin reload", passed: true });
  await p.goto(base + "/talk");
  await p.getByLabel(/Jangan simpan/).check();
  await p.getByRole("button", { name: "Buka ruang cerita" }).click();
  await p
    .getByText("Pilihan AI · selalu dalam kendalimu", { exact: true })
    .click();
  const meta = p
    .locator(".v3-ai-permission section")
    .filter({
      has: p.getByRole("heading", { name: "Meta di Modal", exact: true }),
    });
  if (process.env.E2E_LIVE_AI !== "1")
    throw new Error(
      "Run this synthetic live-provider journey only with E2E_LIVE_AI=1, or use unit/test-double suites.",
    );
  await meta.getByRole("button", { name: "Izinkan penyedia ini" }).click();
  await meta.getByText("Diizinkan", { exact: true }).waitFor();
  await p
    .getByLabel("Apa yang ingin kamu ceritakan?")
    .fill(
      "Pengujian sintetis: tugas kuliah menumpuk dan saya ingin mengurai prioritas.",
    );
  await p.getByRole("button", { name: "Kirim cerita" }).click();
  await p.locator(".v3-sela-words").waitFor({ timeout: 120000 });
  await audit(p, "live conversation desktop ID");
  await p.screenshot({
    path: join(RESULTS, "talk-desktop.png"),
    fullPage: true,
  });
  results.push({ flow: "live Meta listening reply", passed: true });
  await p.getByRole("button", { name: "Akhiri percakapan" }).click();
  await p.waitForURL("**/home");
  await p.goto(base + "/privacy");
  await audit(p, "privacy desktop ID");
  const downloadPromise = p.waitForEvent("download");
  await p.getByRole("button", { name: "Unduh JSON", exact: true }).click();
  await downloadPromise;
  results.push({ flow: "owner export", passed: true });
  await p.setViewportSize({ width: 320, height: 740 });
  await p.goto(base + "/home");
  await audit(p, "home mobile ID");
  await p.screenshot({
    path: join(RESULTS, "home-mobile.png"),
    fullPage: true,
  });
  await p.getByRole("button", { name: "Switch to English" }).click();
  await p
    .getByRole("heading", { name: "There is room for you today." })
    .waitFor();
  await audit(p, "home mobile EN");
  await p.goto(base + "/notes");
  await audit(p, "notes mobile EN");
  await p.screenshot({
    path: join(RESULTS, "notes-mobile-en.png"),
    fullPage: true,
  });
  await p.goto(base + "/support");
  await audit(p, "support mobile EN");
  await p.goto(base + "/privacy");
  await p
    .getByRole("button", { name: "Delete data and account", exact: true })
    .click();
  await p
    .getByRole("button", { name: "Yes, delete all my data", exact: true })
    .click();
  await p.getByRole("button", { name: "Start without an account" }).waitFor();
  results.push({ flow: "owner deletion", passed: true });
  await audit(p, "welcome mobile EN");
  await p.screenshot({
    path: join(RESULTS, "welcome-mobile-en.png"),
    fullPage: true,
  });
  await context.close();
  console.log(JSON.stringify(results, null, 2));
  writeFileSync(join(RESULTS, "report.json"), JSON.stringify(results, null, 2));
} catch (e) {
  console.error(e.stack);
  if (currentPage) {
    console.error("FAILED URL", currentPage.url());
    await currentPage.screenshot({
      path: "/results/failure.png",
      fullPage: true,
    });
  }
  process.exitCode = 1;
} finally {
  await browser.close();
}
