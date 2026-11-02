// Builds the real SELA web app (../web/src, unchanged) into video/.cache/web
// so it can be filmed against synthetic fixtures. This never touches web/dist
// or the deployed application.
//
// Uses esbuild + react/react-dom. Resolution order: video/node_modules, then
// SELA_NODE_PATH (a directory that holds node_modules with esbuild/react).
import { execFileSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const video = resolve(here, "..");
const web = resolve(video, "../web");
const out = join(video, ".cache/web");

const nodePaths = [join(video, "node_modules"), process.env.SELA_NODE_PATH].filter(Boolean);
function findEsbuild() {
  for (const p of nodePaths) {
    const native = join(p, "@esbuild/linux-x64/bin/esbuild");
    if (existsSync(native)) return native;
    const js = join(p, "esbuild/bin/esbuild");
    if (existsSync(js)) return js;
  }
  return "esbuild";
}

rmSync(out, { recursive: true, force: true });
mkdirSync(join(out, "assets"), { recursive: true });
cpSync(join(web, "public"), out, { recursive: true });

execFileSync(
  findEsbuild(),
  [
    join(web, "src/main.tsx"),
    "--bundle",
    `--outdir=${join(out, "assets")}`,
    "--jsx=automatic",
    "--format=esm",
    "--splitting",
    "--minify",
    "--external:/fonts/*",
    "--define:process.env.NODE_ENV=\"production\"",
    "--log-level=warning",
  ],
  { stdio: "inherit", env: { ...process.env, NODE_PATH: nodePaths.join(":") } },
);

// esbuild does not inject CSS for lazy chunks the way Vite does, so link every
// stylesheet up front (main first, then chunks) to match the production look.
const css = readdirSync(join(out, "assets"))
  .filter((f) => f.endsWith(".css"))
  .sort((a, b) => (a.startsWith("main") ? -1 : b.startsWith("main") ? 1 : a.localeCompare(b)));

writeFileSync(
  join(out, "index.html"),
  `<!doctype html>
<html lang="id">
<head>
<meta charset="UTF-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<meta name="color-scheme" content="light dark" />
<link rel="icon" href="/icon.svg" type="image/svg+xml" />
${css.map((f) => `<link rel="stylesheet" href="/assets/${f}" />`).join("\n")}
<script src="/boot.js"></script>
<title>Sela</title>
</head>
<body><div id="root"></div><script type="module" src="/assets/main.js"></script></body>
</html>
`,
);
console.log(`built ${out} (${css.length} stylesheets)`);
