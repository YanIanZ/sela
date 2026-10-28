// Palette review captures from the dev-only design preview. Synthetic data only.
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";

const base = process.env.BASE ?? "http://127.0.0.1:5179";
const out = process.env.OUT ?? new URL(".", import.meta.url).pathname;
mkdirSync(out, { recursive: true });
const palettes = (process.env.PALETTES ?? "embun,samudra,teduh,pasir,lipat").split(",");
const schemes = (process.env.SCHEMES ?? "light,dark").split(",");
const browser = await chromium.launch();
for (const palette of palettes) for (const theme of schemes) {
  for (const [label, vp] of [["desktop", { width: 1440, height: 1000 }], ["mobile", { width: 390, height: 844 }]]) {
    const ctx = await browser.newContext({ viewport: vp, deviceScaleFactor: 1 });
    await ctx.addInitScript(([p, t]) => localStorage.setItem("sela.prefs", JSON.stringify({ palette: p, theme: t })), [palette, theme]);
    const page = await ctx.newPage();
    await page.goto(`${base}/dashboard-preview.html`, { waitUntil: "networkidle" });
    await page.screenshot({ path: `${out}${palette}-${theme}-${label}-portal.png`, fullPage: label === "desktop" ? false : false });
    const home = page.locator('a[href="/home"], a[href="/start/age"]').first();
    if (await home.count()) {
      await home.click();
      await page.waitForTimeout(300);
      await page.screenshot({ path: `${out}${palette}-${theme}-${label}-home.png` });
    }
    await ctx.close();
  }
}
await browser.close();
console.log("done");
