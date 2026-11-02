# SELA — Beyond Being Heard (product film)

A 3-minute kinetic-typography product film and a 30-second teaser for SELA's *Build for Your World* entry. This folder is a separate production workspace: **nothing here is imported by, built into or deployed with the SELA app.**

| | |
| --- | --- |
| Master | 1920×1080 · 30 fps · 180 s (`out/SELA-Beyond-Being-Heard-master-1080p.mp4`, music + typography, no narration) |
| Teaser | 1920×1080 · 30 fps · 30 s (`out/SELA-Beyond-Being-Heard-teaser-30s.mp4`) |
| Footage | Real SELA UI built from `web/src` at `dfdffd0` (= production), synthetic "Nadia" data |
| Truth audit | [`docs/QA_REPORT.md`](docs/QA_REPORT.md) — scene-by-scene evidence, labels, missing assets |
| Narration | [`docs/VO_SCRIPT_ID.md`](docs/VO_SCRIPT_ID.md), [`subtitles/`](subtitles/) (ID + EN SRT) |
| Sound | [`docs/SOUND_DESIGN.md`](docs/SOUND_DESIGN.md) — original, code-synthesised score |
| **Product demo** | 1920×1080 · 30 fps · 180 s (`out/SELA-Product-Demo-3min-1080p-EN-subs.mp4`): ~85 % real SELA UI in motion, English subtitles burned in; Indonesian dubbing via [`docs/DUB_SCRIPT_ID.md`](docs/DUB_SCRIPT_ID.md) + `scripts/dub.py` |

## Layout

```text
src/
  timeline.json        single source of truth: chapter frames, cue frames, teaser edit
  brand/tokens.ts      SELA dark-scheme colours, Lexend / Hanken Grotesk, safe area
  motion/index.tsx     motion system: Kinetic (rise/track/scale/fragment), UIShot, Float,
                       Tag, Gloss, PulseRing, Sweep, Dip, Fragments, SproutMark, ChapterMark
  scenes/Ch01–Ch08     one file per chapter, pure functions of the frame
  Master.tsx Teaser.tsx Root.tsx index.ts   Remotion compositions (Master, Teaser, ch01…ch08)
  engine/remotion-lite.tsx   API-compatible Remotion subset (see below)
capture/               films the real web UI against the synthetic fixture (Playwright)
scripts/render.mjs     offline renderer: Chrome → JPEG frames → FFmpeg, one file per chapter
scripts/score.py       original score, placed from timeline.json, loudness-normalised
evidence/meaning-diff  real CompareMeaning output for the strings shown in chapter 03
public/footage         captured stills + footage.json (sizes and crop marks)
```

## Remotion

Scenes import from `"remotion"` and use only `useCurrentFrame`, `useVideoConfig`, `interpolate`, `spring`, `Easing`, `Sequence`, `AbsoluteFill`, `Img`, `staticFile`, `random`, `Composition`, `registerRoot`.

- **With Remotion installed** (`npm i`): `npm run studio` to preview, `npm run render:remotion` to render. Remove the `paths` entry in `tsconfig.json` so types come from the real package.
- **Without it** (how these files were rendered — the build sandbox had no npm registry): `scripts/render.mjs` bundles with esbuild and aliases `remotion` to `src/engine/remotion-lite.tsx`.

## Rebuild everything

```sh
cd video
npm i                                   # or set SELA_NODE_PATH to a node_modules with esbuild, react, react-dom, playwright-core
npm run capture:build && npm run capture  # real SELA UI → public/footage (synthetic data, no network)
npm run score                           # public/audio/score-*.wav (Python 3 + NumPy + SciPy)
npm run subs                            # subtitles/*.srt from docs/vo_cues.tsv
npm run still -- Master 1750 2000       # check frames before a full render
npm run render:master                   # .cache/scenes/ch01–08.mp4 → concat → mux score
npm run render:teaser
```

### Product demo

```sh
npm run capture:build
node capture/record.mjs            # frame-exact screen recording → public/demo/<clip>/ + demo.json
node scripts/demo-plan.mjs         # resolve edit + cues → .cache/demo-plan.json, subtitles/demo.*.srt
python3 scripts/score_demo.py      # music bed + UI sounds → public/audio/score-demo.wav
node scripts/render.mjs demo       # → out/SELA-Product-Demo-3min-1080p-EN-subs.mp4
python3 scripts/dub.py --tts edge  # or --lines DIR with recorded takes → ...-ID-dub-EN-subs.mp4
```

The edit lives in `src/demo/plan.mjs` (segment order, chapter titles, truth tags) and `src/demo/vo.json` (narration anchored to recorded marks such as `send` or `impact`). Re-recording a clip moves its marks, and the subtitles, sounds and dubbing follow automatically.

To change one chapter, edit its scene, delete `.cache/scenes/<id>.mp4` and run `render:master` again: only that chapter re-renders.
