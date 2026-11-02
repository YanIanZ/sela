// Offline renderer for the SELA film (no Remotion install needed).
// Bundles src/ with "remotion" aliased to src/engine/remotion-lite.tsx, draws
// every frame in headless Chrome and pipes JPEG frames into FFmpeg.
//
//   node scripts/render.mjs still <comp> <frame> [frame...]   -> out/stills/
//   node scripts/render.mjs scene <chapterId>                 -> .cache/scenes/<id>.mp4
//   node scripts/render.mjs master                            -> out/SELA-Beyond-Being-Heard-master-1080p.mp4
//   node scripts/render.mjs teaser                            -> out/SELA-Beyond-Being-Heard-teaser-30s.mp4
//
// Env: SELA_NODE_PATH (node_modules with esbuild/react/react-dom/playwright-core
// when not installed locally), SELA_CHROME (browser path), SELA_WORKERS (default 2).
import { execFileSync, spawn } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, renameSync, rmSync, writeFileSync } from "node:fs";
import { dirname, extname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const CACHE = join(ROOT, ".cache");
const OUT = join(ROOT, "out");
const TL = JSON.parse(readFileSync(join(ROOT, "src/timeline.json"), "utf8"));
const nodePaths = [join(ROOT, "node_modules"), process.env.SELA_NODE_PATH].filter(Boolean);
const require = createRequire(import.meta.url);
const req = (m) => {
  for (const p of nodePaths) {
    try {
      return require(join(p, m));
    } catch {}
  }
  return require(m);
};
const CHROME = process.env.SELA_CHROME || ["/usr/bin/google-chrome", "/usr/bin/chromium"].find((p) => existsSync(p));
const ORIGIN = "http://film.test";

function esbuildBin() {
  for (const p of nodePaths) {
    for (const c of ["@esbuild/linux-x64/bin/esbuild", "@esbuild/darwin-arm64/bin/esbuild", "@esbuild/darwin-x64/bin/esbuild", "esbuild/bin/esbuild"]) if (existsSync(join(p, c))) return join(p, c);
  }
  return "esbuild";
}

export function bundle() {
  const dir = join(CACHE, "render");
  mkdirSync(dir, { recursive: true });
  execFileSync(esbuildBin(), [join(ROOT, "src/render-entry.tsx"), "--bundle", `--outfile=${join(dir, "bundle.js")}`, "--jsx=automatic", "--format=esm", "--loader:.json=json", `--alias:remotion=${join(ROOT, "src/engine/remotion-lite.tsx")}`, "--define:process.env.NODE_ENV=\"production\"", "--minify", "--log-level=warning"], { stdio: "inherit", env: { ...process.env, NODE_PATH: nodePaths.join(":") } });
  writeFileSync(join(dir, "index.html"), `<!doctype html><html><head><meta charset="utf-8"><style>html,body{margin:0;background:#050706;overflow:hidden}</style></head><body><div id="root"></div><script type="module" src="/bundle.js"></script></body></html>`);
}

const TYPES = { ".html": "text/html", ".js": "text/javascript", ".json": "application/json", ".jpg": "image/jpeg", ".png": "image/png", ".svg": "image/svg+xml", ".woff2": "font/woff2" };
async function openPage(browser) {
  const page = await browser.newPage({ viewport: { width: 1920, height: 1080 }, deviceScaleFactor: 1 });
  page.on("pageerror", (e) => console.error("PAGEERROR", e.message));
  await page.route("**/*", (route) => {
    const u = new URL(route.request().url());
    if (u.origin !== ORIGIN) return route.abort();
    const p = decodeURIComponent(u.pathname);
    const candidates = [join(CACHE, "render", p === "/" ? "index.html" : p), join(ROOT, "public", p)];
    const f = candidates.find((c) => existsSync(c) && !c.endsWith("/"));
    if (!f) return route.fulfill({ status: 404, body: "" });
    return route.fulfill({ status: 200, contentType: TYPES[extname(f)] ?? "application/octet-stream", body: readFileSync(f) });
  });
  await page.goto(ORIGIN + "/");
  await page.waitForFunction(() => window.__sela !== undefined);
  await page.evaluate(() => window.__sela.ready);
  return page;
}

async function launch() {
  const { chromium } = req("playwright-core");
  return chromium.launch({ executablePath: CHROME, args: ["--force-color-profile=srgb", "--disable-lcd-text", "--font-render-hinting=none"] });
}

async function still(comp, frames) {
  bundle();
  const browser = await launch();
  const page = await openPage(browser);
  const dir = join(OUT, "stills");
  mkdirSync(dir, { recursive: true });
  for (const f of frames) {
    await page.evaluate(([c, n]) => window.__sela.draw(c, n), [comp, Number(f)]);
    const file = join(dir, `${comp}-${String(f).padStart(5, "0")}.jpg`);
    await page.screenshot({ path: file, type: "jpeg", quality: 90 });
    console.log(file);
  }
  await browser.close();
}

/** Render frames [from, to) of comp into an MP4 (video only). */
async function renderRange(page, comp, from, to, file) {
  mkdirSync(dirname(file), { recursive: true });
  const tmp = file.replace(/\.mp4$/, ".partial.mp4");
  const ff = spawn("ffmpeg", ["-y", "-loglevel", "error", "-f", "image2pipe", "-framerate", "30", "-c:v", "mjpeg", "-i", "-", "-c:v", "libx264", "-preset", "veryfast", "-crf", "16", "-pix_fmt", "yuv420p", "-r", "30", "-movflags", "+faststart", tmp], { stdio: ["pipe", "inherit", "inherit"] });
  const done = new Promise((res, rej) => ff.on("close", (code) => (code === 0 ? res() : rej(new Error("ffmpeg exit " + code)))));
  const t0 = Date.now();
  for (let f = from; f < to; f++) {
    await page.evaluate(([c, n]) => window.__sela.draw(c, n), [comp, f]);
    const buf = await page.screenshot({ type: "jpeg", quality: 94 });
    if (!ff.stdin.write(buf)) await new Promise((r) => ff.stdin.once("drain", r));
    if ((f - from) % 150 === 0) process.stdout.write(`  ${comp} ${f}/${to} ${((Date.now() - t0) / 1000).toFixed(0)}s\n`);
  }
  ff.stdin.end();
  await done;
  renameSync(tmp, file);
}

async function scenes(ids) {
  bundle();
  const browser = await launch();
  const workers = Number(process.env.SELA_WORKERS ?? 2);
  const queue = [...ids];
  await Promise.all(
    Array.from({ length: Math.min(workers, queue.length) }, async () => {
      const page = await openPage(browser);
      while (queue.length) {
        const id = queue.shift();
        const ch = TL.chapters.find((c) => c.id === id);
        // Each chapter renders from the Master composition, so overlaps and
        // in-scene transitions are drawn exactly as on the master timeline.
        await renderRange(page, "Master", ch.from, ch.from + ch.duration, join(CACHE, "scenes", `${id}.mp4`));
        console.log("scene done", id);
      }
    }),
  );
  await browser.close();
}

function concatAndMux(files, audio, out) {
  mkdirSync(OUT, { recursive: true });
  const list = join(CACHE, "concat.txt");
  writeFileSync(list, files.map((f) => `file '${f}'`).join("\n"));
  const silent = join(CACHE, "video-only.mp4");
  execFileSync("ffmpeg", ["-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", list, "-c", "copy", silent], { stdio: "inherit" });
  if (audio && existsSync(audio)) {
    execFileSync("ffmpeg", ["-y", "-loglevel", "error", "-i", silent, "-i", audio, "-map", "0:v", "-map", "1:a", "-c:v", "copy", "-c:a", "aac", "-b:a", "256k", "-shortest", "-movflags", "+faststart", out], { stdio: "inherit" });
  } else {
    renameSync(silent, out);
  }
  console.log("wrote", out);
}

const [cmd, ...args] = process.argv.slice(2);
if (cmd === "still") await still(args[0], args.slice(1));
else if (cmd === "scene") await scenes(args);
else if (cmd === "master") {
  const ids = TL.chapters.map((c) => c.id);
  const missing = ids.filter((id) => !existsSync(join(CACHE, "scenes", `${id}.mp4`)) || process.env.SELA_FORCE);
  if (missing.length) await scenes(missing);
  concatAndMux(ids.map((id) => join(CACHE, "scenes", `${id}.mp4`)), join(ROOT, "public/audio/score-master.wav"), join(OUT, "SELA-Beyond-Being-Heard-master-1080p.mp4"));
} else if (cmd === "teaser") {
  bundle();
  const browser = await launch();
  const page = await openPage(browser);
  const total = TL.teaser.reduce((n, s) => n + s.to - s.from, 0);
  const file = join(CACHE, "scenes", "teaser.mp4");
  await renderRange(page, "Teaser", 0, total, file);
  await browser.close();
  concatAndMux([file], join(ROOT, "public/audio/score-teaser.wav"), join(OUT, "SELA-Beyond-Being-Heard-teaser-30s.mp4"));
} else {
  console.log("usage: render.mjs still|scene|master|teaser");
}
