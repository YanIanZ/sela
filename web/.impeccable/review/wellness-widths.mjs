// Intermediate widths for the responsive pass. Reuses the saved synthetic guest session.
import { chromium } from "playwright";
const base = process.env.BASE ?? "http://localhost:5179";
const out = new URL("./wellness/w-", import.meta.url).pathname;
const state = "/private/var/folders/1v/m_hbw0l522z4fhqbj91_6h7w0000gn/T/opencode/sela-state.json";
const widths = (process.env.WIDTHS ?? "600,768,1024,1280").split(",").map(Number);
const paths = (process.env.PATHS ?? "/home,/notes,/notes/check-in,/support,/").split(",");
const browser = await chromium.launch();
const errors = [];
for (const width of widths) {
  const ctx = await browser.newContext({ viewport: { width, height: 900 }, storageState: state, locale: "id-ID" });
  const page = await ctx.newPage();
  for (const path of paths) {
    const anon = path === "/";
    const p = anon ? await (await browser.newContext({ viewport: { width, height: 900 }, locale: "id-ID" })).newPage() : page;
    await p.goto(base + path, { waitUntil: "networkidle" });
    await p.waitForTimeout(400);
    const overflow = await p.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1);
    if (overflow) errors.push(`overflow ${width} ${path}`);
    if (width < 1024) await p.addStyleTag({ content: ".s-place-nav{display:none!important}" });
    await p.screenshot({ path: `${out}${width}${path === "/" ? "-portal" : path.replaceAll("/", "-")}.png`, fullPage: true });
    if (anon) await p.context().close();
  }
  await ctx.close();
}
await browser.close();
console.log(errors.length ? errors.join("\n") : "no errors");
