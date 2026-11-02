// Development helper: open paths against the fixture router, screenshot them
// and print every API call, so missing fixture shapes show up quickly.
// usage: node capture/probe.mjs out-dir /path1 /path2 ... [--state '{"corrected":true}']
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { attach, CHROME, loadPlaywright, ORIGIN } from "./harness.mjs";
import { createRouter } from "./fixtures.mjs";

const args = process.argv.slice(2);
const si = args.indexOf("--state");
const initial = si >= 0 ? JSON.parse(args.splice(si, 2)[1]) : {};
const [out, ...paths] = args;
mkdirSync(out, { recursive: true });
const { chromium } = loadPlaywright();
const browser = await chromium.launch({ executablePath: CHROME });
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1, colorScheme: "dark", locale: "id-ID", timezoneId: "Asia/Jakarta" });
for (const p of paths) {
  const page = await ctx.newPage();
  const log = [];
  page.on("pageerror", (e) => log.push("PAGEERROR " + e.message));
  page.on("console", (m) => m.type() === "error" && log.push("CONSOLE " + m.text().slice(0, 200)));
  await attach(page, createRouter(initial).router, log);
  await page.goto(ORIGIN + p);
  await page.waitForTimeout(1500);
  const name = p.replace(/[^a-z0-9]+/gi, "_") || "root";
  await page.screenshot({ path: join(out, name + ".jpg"), type: "jpeg", quality: 80, fullPage: true });
  console.log(`== ${p}\n  ${log.join("\n  ")}`);
  await page.close();
}
await browser.close();
