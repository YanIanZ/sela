// The demo edit: segment order, chapter titles and truth tags, resolved
// against the recorded clips (public/demo/demo.json). Plain JS so both the
// composition and the Node scripts (subtitles, score, dubbing) use one plan.
export const FPS = 30;
export const TOTAL = 5400;
export const INTRO = 240;
export const GRAPH = 180;

export const SEGMENTS = [
  { id: "intro", type: "intro" },
  { id: "talk", type: "clip", chapter: "01", title: "BEYOND LISTENING", tags: [{ from: "send", text: "Test double · not live Meta output", tone: "fixture" }] },
  { id: "memory", type: "clip", chapter: "02", title: "MEANING INTEGRITY ENGINE", tags: [{ from: "detail", to: "save", text: "Test fixture · simulated AI error", tone: "fixture", solid: true }] },
  { id: "graph", type: "graph", chapter: "02", tags: [{ from: 0, text: "Illustration · asserted in TestMeaningCorrectionPropagates", tone: "muted" }] },
  { id: "reach-summary", type: "clip", chapter: "02", tags: [{ from: 0, text: "After the correction · summary", tone: "stale" }] },
  { id: "reach-share", type: "clip", chapter: "02", tags: [{ from: 0, text: "After the correction · share link", tone: "stopped" }] },
  { id: "reach-care", type: "clip", chapter: "02", tags: [{ from: 0, text: "After the correction · simulated partner", tone: "sim" }] },
  { id: "timeline", type: "clip", chapter: "03", title: "CONTEXT OVER TIME", tags: [{ from: 0, text: "No AI on this page", tone: "leaf" }] },
  { id: "steps", type: "clip", chapter: "04", title: "YOUR NEXT STEP", tags: [{ from: 0, text: "Written by Nadia · optional", tone: "leaf" }] },
  { id: "passport", type: "clip", chapter: "05", title: "CARE PASSPORT", tags: [{ from: 0, text: "Personal preparation · not a medical record", tone: "leaf" }] },
  { id: "care", type: "clip", chapter: "06", title: "SUPPORT, ON YOUR TERMS", tags: [{ from: 0, text: "Simulated professional", tone: "sim", solid: true }] },
  { id: "revoke", type: "clip", chapter: "06", tags: [{ from: 0, text: "Revocable access", tone: "leaf" }] },
  { id: "outro", type: "outro" },
];

/** Lay segments on the timeline; the outro absorbs the remainder. */
export function buildPlan(demo) {
  let t = 0;
  const out = [];
  const fixed = SEGMENTS.reduce((n, s) => n + (s.type === "clip" ? demo[s.id].frames : s.type === "intro" ? INTRO : s.type === "graph" ? GRAPH : 0), 0);
  for (const s of SEGMENTS) {
    const dur = s.type === "clip" ? demo[s.id].frames : s.type === "intro" ? INTRO : s.type === "graph" ? GRAPH : TOTAL - fixed;
    const marks = s.type === "clip" ? demo[s.id].marks : {};
    out.push({ ...s, from: t, dur, marks, clip: s.type === "clip" ? demo[s.id] : null });
    t += dur;
  }
  return out;
}

/** Resolve narration cues to absolute frames. */
export function resolveCues(plan, cues) {
  return cues.map((c, i) => {
    const seg = plan.find((s) => s.id === c.seg);
    if (!seg) throw new Error("cue " + i + ": no segment " + c.seg);
    const base = typeof c.mark === "number" ? c.mark : seg.marks[c.mark];
    if (base === undefined) throw new Error("cue " + i + ": no mark " + c.mark + " in " + c.seg);
    const start = seg.from + base + Math.round(c.off * FPS);
    return { ...c, start, end: start + Math.round(c.dur * FPS) };
  });
}
