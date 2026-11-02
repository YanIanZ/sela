// Resolves the demo edit (src/demo/plan.mjs + recorded clips + narration cues)
// and writes:
//   .cache/demo-plan.json      segments, absolute click frames, cues (for score/dub)
//   subtitles/demo.en.srt      English subtitles (also burned into the video)
//   subtitles/demo.id.srt      Indonesian dubbing script with timings
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { buildPlan, resolveCues, FPS } from "../src/demo/plan.mjs";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const demo = JSON.parse(readFileSync(join(ROOT, "public/demo/demo.json"), "utf8"));
const vo = JSON.parse(readFileSync(join(ROOT, "src/demo/vo.json"), "utf8"));
const plan = buildPlan(demo);
const cues = resolveCues(plan, vo.cues);

const clicks = plan.flatMap((s) => (s.clip ? s.clip.clicks.map((c) => s.from + c) : []));
const out = {
  fps: FPS,
  segments: plan.map(({ id, type, from, dur, title, chapter }) => ({ id, type, from, dur, title, chapter })),
  clicks,
  cues: cues.map(({ start, end, id, en }) => ({ start, end, id, en })),
};
mkdirSync(join(ROOT, ".cache"), { recursive: true });
writeFileSync(join(ROOT, ".cache/demo-plan.json"), JSON.stringify(out, null, 2));

const ts = (f) => {
  const ms = Math.round((f / FPS) * 1000);
  const h = Math.floor(ms / 3600000), m = Math.floor((ms % 3600000) / 60000), s = Math.floor((ms % 60000) / 1000);
  return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")},${String(ms % 1000).padStart(3, "0")}`;
};
mkdirSync(join(ROOT, "subtitles"), { recursive: true });
for (const lang of ["en", "id"]) {
  const srt = cues.map((c, i) => `${i + 1}\n${ts(c.start)} --> ${ts(c.end)}\n${c[lang]}\n`).join("\n");
  writeFileSync(join(ROOT, `subtitles/demo.${lang}.srt`), srt);
}
let issues = 0;
cues.forEach((c, i) => {
  const cps = c.en.length / ((c.end - c.start) / FPS);
  if (cps > 17) (issues++, console.log(`fast EN cue ${i + 1}: ${cps.toFixed(1)} cps`));
  if (i > 0 && c.start < cues[i - 1].end) (issues++, console.log(`overlap at cue ${i + 1}`));
});
console.log(`${cues.length} cues, ${clicks.length} clicks, ${issues} issues`);
