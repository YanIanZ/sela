/*
 * SELA — 3-minute product demo. ~85% real SELA UI in motion (recorded by
 * capture/record.mjs from web/src at dfdffd0 against the synthetic Nadia
 * fixture), with short kinetic chapter titles, an MIE graph insert, burned-in
 * English subtitles and truth labels. Indonesian dubbing is mixed afterwards
 * (scripts/dub.py) on the same cue sheet (src/demo/vo.json).
 */
import { AbsoluteFill, Img, Sequence, staticFile, useCurrentFrame } from "remotion";
import { C, FONT } from "../brand/tokens";
import { Fonts } from "../Fonts";
import { Dip, EASE, envelope, Eyebrow, Ground, hexA, Kinetic, prog, PulseRing, SproutMark, Stage, Sweep, Tag } from "../motion";
import { Ch03 } from "../scenes/Ch03";
import { Ch08 } from "../scenes/Ch08";
import demo from "../../public/demo/demo.json";
import vo from "./vo.json";
import { buildPlan, resolveCues, TOTAL } from "./plan.mjs";

type Focus = { name: string; from: number; to: number; x: number; y: number; w: number; h: number };
type ClipMeta = { frames: number; marks: Record<string, number>; clicks: number[]; focus: Focus[]; width: number; height: number };
type Seg = ReturnType<typeof buildPlan>[number] & { clip: ClipMeta | null; tags?: Array<{ from: string | number; to?: string; text: string; tone: string; solid?: boolean }> };

const PLAN = buildPlan(demo as Record<string, ClipMeta>) as Seg[];
const CUES = resolveCues(PLAN, vo.cues);
export const DEMO_FRAMES = TOTAL;

const UI = { x: 192, y: 30, w: 1536, h: 864 };
const TONE: Record<string, string> = { fixture: C.fixture, stale: C.stale, stopped: C.stopped, sim: C.sim, leaf: C.leaf, muted: C.muted };
const pad5 = (n: number) => String(n).padStart(5, "0");

// ---------------------------------------------------------------- clip
function DemoClip({ seg }: { seg: Seg }) {
  const frame = useCurrentFrame();
  const clip = seg.clip!;
  const f = Math.max(0, Math.min(clip.frames - 1, frame));
  // push-in on the recorded focus box
  let z = 1, ox = UI.w / 2, oy = UI.h / 2;
  for (const fc of clip.focus) {
    if (frame >= fc.from - 2 && frame < fc.to) {
      z = 1 + 0.13 * Math.min(prog(frame, fc.from, 22, EASE.inOut), 1 - prog(frame, fc.to - 18, 18, EASE.inOut));
      ox = Math.min(UI.w * 0.82, Math.max(UI.w * 0.18, fc.x + fc.w / 2));
      oy = Math.min(UI.h * 0.82, Math.max(UI.h * 0.18, fc.y + fc.h / 2));
    }
  }
  const fadeIn = prog(frame, 0, 8, EASE.soft);
  const fadeOut = 1 - prog(frame, clip.frames - 7, 7, EASE.inOut);
  const titleDim = seg.title ? 1 - prog(frame, 34, 22, EASE.inOut) : 0;
  return (
    <AbsoluteFill>
      <div style={{ position: "absolute", left: UI.x, top: UI.y, width: UI.w, height: UI.h, borderRadius: 18, overflow: "hidden", background: C.appGround, boxShadow: `0 40px 120px rgba(0,0,0,0.6), 0 0 0 1.5px ${hexA(C.sage, 0.14)}, 0 0 90px ${hexA(C.leafSoft, 0.07)}`, opacity: fadeIn * fadeOut }}>
        <div style={{ position: "absolute", inset: 0, transform: `scale(${z})`, transformOrigin: `${ox}px ${oy}px` }}>
          <Img src={staticFile(`demo/${seg.id}/f${pad5(f)}.jpg`)} style={{ width: UI.w, height: UI.h, display: "block" }} />
        </div>
        {titleDim > 0 && <div style={{ position: "absolute", inset: 0, background: `rgba(5,7,6,${0.78 * titleDim})` }} />}
      </div>
      {seg.title && (
        <AbsoluteFill style={{ left: UI.x, top: UI.y, width: UI.w, height: UI.h, alignItems: "center", justifyContent: "center", gap: 18 }}>
          <div style={{ opacity: envelope(frame, 2, 52, 10, 14) }}>
            <Eyebrow>{seg.chapter}</Eyebrow>
          </div>
          <Kinetic text={seg.title} at={4} out={38} size={92} dur={18} />
        </AbsoluteFill>
      )}
    </AbsoluteFill>
  );
}

// ---------------------------------------------------------------- intro / graph / outro
function DemoIntro() {
  const frame = useCurrentFrame();
  return (
    <Ground glow={0.6} grid={0.2}>
      <PulseRing at={4} max={1200} dur={60} />
      <Stage gap={6}>
        <Kinetic text="EVERY STORY" at={8} out={100} size={170} mode="scale" dur={16} />
        <Kinetic text="*MATTERS.*" at={28} out={104} size={170} />
      </Stage>
      <Stage gap={6}>
        <Kinetic text="BUT WHO" at={112} out={186} size={150} weight={700} color={C.muted} />
        <Kinetic text="UNDERSTANDS*?*" at={130} out={190} size={180} mode="fragment" />
      </Stage>
      <AbsoluteFill style={{ alignItems: "center", justifyContent: "center", gap: 26, opacity: envelope(frame, 192, 244, 10, 10) }}>
        <SproutMark size={120} grow={prog(frame, 192, 24)} ring={prog(frame, 196, 20)} />
        <Eyebrow color={C.sage}>Sela · product demo · synthetic data</Eyebrow>
      </AbsoluteFill>
      <Sweep at={128} />
    </Ground>
  );
}

function GraphInsert() {
  // The Meaning Integrity Engine graph from the master film (chapter 03),
  // from just before the correction until the counters settle.
  return (
    <Sequence from={-640}>
      <Ch03 hideMark />
    </Sequence>
  );
}

// Start the end card as the sprout finishes growing, so the lock-up and the
// "Built with Meta AI" line both get enough time inside the remaining frames.
export const OUTRO_OFFSET = 130;
function Outro() {
  return (
    <Sequence from={-OUTRO_OFFSET}>
      <Ch08 />
    </Sequence>
  );
}

// ---------------------------------------------------------------- overlays
function BottomBar() {
  const frame = useCurrentFrame();
  const seg = PLAN.find((s) => frame >= s.from && frame < s.from + s.dur)!;
  if (!seg || seg.type === "intro" || seg.type === "outro" || seg.type === "graph") return null;
  const local = frame - seg.from;
  const titleOf = (id?: string) => PLAN.find((s) => s.chapter === id && s.title)?.title ?? "";
  const tags = (seg.tags ?? []).filter((t) => {
    const a = typeof t.from === "number" ? t.from : seg.marks[t.from] ?? 0;
    const b = t.to === undefined ? Infinity : seg.marks[t.to] ?? Infinity;
    return local >= a && local < b;
  });
  const fade = Math.min(prog(local, 4, 12), 1 - prog(local, seg.dur - 6, 6));
  return (
    <div style={{ position: "absolute", left: UI.x, top: UI.y + UI.h + 14, width: UI.w, display: "flex", justifyContent: "space-between", alignItems: "center", opacity: fade }}>
      <div style={{ display: "flex", alignItems: "center", gap: 14, fontFamily: FONT.mono, fontSize: 17, letterSpacing: "0.18em", color: C.dim }}>
        <span style={{ color: C.leaf }}>{seg.chapter}</span>
        <span>{titleOf(seg.chapter)}</span>
      </div>
      <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
        {tags.map((t) => (
          <Tag key={t.text} color={TONE[t.tone] ?? C.leaf} size={14} solid={t.solid}>
            {t.text}
          </Tag>
        ))}
        {seg.type === "clip" && (
          <Tag color={C.dim} size={14}>
            SELA UI · dfdffd0 · synthetic data
          </Tag>
        )}
      </div>
    </div>
  );
}

function Subtitles() {
  const frame = useCurrentFrame();
  const cue = CUES.find((c) => frame >= c.start && frame < c.end);
  if (!cue) return null;
  const a = Math.min(prog(frame, cue.start, 5), 1 - prog(frame, cue.end - 5, 5));
  const seg = PLAN.find((s) => frame >= s.from && frame < s.from + s.dur);
  // keep clear of the graph counters and the end-card lock-up
  const top = seg?.type === "graph" ? 1000 : seg?.type === "outro" ? 70 : 958;
  return (
    <div style={{ position: "absolute", left: 0, right: 0, top, display: "flex", justifyContent: "center", opacity: a }}>
      <div style={{ maxWidth: 1500, textAlign: "center", fontFamily: FONT.body, fontWeight: 600, fontSize: 38, lineHeight: 1.25, color: C.ink, textShadow: "0 2px 12px rgba(0,0,0,0.9)", padding: "4px 18px", borderRadius: 10, background: "rgba(5,7,6,0.55)" }}>{cue.en}</div>
    </div>
  );
}

// ---------------------------------------------------------------- film
export function DemoFilm() {
  return (
    <AbsoluteFill style={{ background: C.void }}>
      <Fonts />
      <Ground glow={0.35} grid={0.12} />
      {PLAN.map((s) => (
        <Sequence key={s.id} from={s.from} durationInFrames={s.dur} name={s.id}>
          {s.type === "clip" ? <DemoClip seg={s} /> : s.type === "intro" ? <DemoIntro /> : s.type === "graph" ? <GraphInsert /> : <Outro />}
        </Sequence>
      ))}
      <BottomBar />
      <Subtitles />
      <Dip outAt={TOTAL - 14} len={14} />
    </AbsoluteFill>
  );
}

export { PLAN, CUES };
