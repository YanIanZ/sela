// CHAPTER 04 — CONTEXT OVER TIME (01:15–01:40)
// Life Context Timeline (/notes/context, D-081): a read-only projection of
// saved notes, check-ins and conversations with their current version and
// MIE status. No AI call, no scores (TestV3ContextTimelineOwnerScopeNoAI).
import { AbsoluteFill, useCurrentFrame } from "remotion";
import { C, FONT } from "../brand/tokens";
import { ChapterMark, Dip, EASE, envelope, Eyebrow, Float, Gloss, Ground, hexA, Kinetic, mix, prog, Stage, Sweep, UIShot } from "../motion";
import T from "../timeline.json";

const q = T.cues.ch04;

const MARKS = [
  { at: q.m1, x: 380, date: "SEL · 27 OKT 2026", kind: "CATATAN · NOTE", title: "Sepulang kuliah", text: "“Sepulang kuliah aku sering capek. Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.”", up: true },
  { at: q.m2, x: 960, date: "JUM · 30 OKT 2026", kind: "CEK PERASAAN · CHECK-IN", title: "Cek perasaan", text: "“Hari ini aku agak kewalahan dengan tugas. Aku belum tahu bagian mana yang paling membuatku lelah.”", up: false },
  { at: q.m3, x: 1540, date: "MIN · 1 NOV 2026", kind: "PERCAKAPAN · CONVERSATION", title: "Minggu yang melelahkan", text: "“Aku ingin tahu bagaimana menjelaskan minggu yang melelahkan ini…”", up: true },
];
const AXIS_Y = 600;

export function Ch04() {
  const frame = useCurrentFrame();
  const axis = prog(frame, q.axis, 50, EASE.inOut);
  const axisOn = envelope(frame, q.axis - 4, q.ui + 4, 10, 16);
  const head = mix(200, 1720, prog(frame, q.axis + 30, q.noScores - q.axis - 30, EASE.inOut));
  return (
    <Ground glow={0.5} gx={50} gy={52} grid={0.28}>
      <ChapterMark n="04" title="CONTEXT OVER TIME" at={q.axis} />
      <Stage>
        <Kinetic text="NOT JUST A *MOMENT.*" at={q.moment} out={q.story - 16} size={150} mode="track" exit="fade" dur={28} />
      </Stage>
      <Stage gap={8}>
        <Kinetic text="YOUR STORY" at={q.story} out={q.axis - 14} size={170} weight={700} />
        <Kinetic text="*IN CONTEXT.*" at={q.context} out={q.axis - 12} size={170} mode="scale" dur={18} />
      </Stage>

      {/* Chronological axis of original saved records */}
      <AbsoluteFill style={{ opacity: axisOn }}>
        <div style={{ position: "absolute", left: 140, top: 160 }}>
          <Eyebrow color={C.muted}>Saved by Nadia · original words · source versions</Eyebrow>
        </div>
        <div style={{ position: "absolute", left: 200, top: AXIS_Y, width: 1520 * axis, height: 2, background: `linear-gradient(90deg, ${hexA(C.leaf, 0.1)}, ${C.leaf} 60%, ${hexA(C.leaf, 0.2)})` }} />
        {Array.from({ length: 16 }).map((_, i) => (
          <div key={i} style={{ position: "absolute", left: 200 + i * 101, top: AXIS_Y - 6, width: 1, height: 12, background: C.dim, opacity: prog(frame, q.axis + i * 2, 20) }} />
        ))}
        {/* playhead sweep */}
        <div style={{ position: "absolute", left: head - 160, top: AXIS_Y - 80, width: 320, height: 160, background: `radial-gradient(closest-side, ${hexA(C.sage, 0.22)}, transparent)`, opacity: axis }} />
        {MARKS.map((m, i) => {
          const p = prog(frame, m.at, 26);
          const stem = 90;
          const cardTop = m.up ? AXIS_Y - stem - 230 : AXIS_Y + stem;
          return (
            <div key={i} style={{ opacity: p }}>
              <div style={{ position: "absolute", left: m.x - 9, top: AXIS_Y - 9, width: 18, height: 18, borderRadius: 99, background: C.leaf, boxShadow: `0 0 24px ${C.leafSoft}`, transform: `scale(${p})` }} />
              <div style={{ position: "absolute", left: m.x, top: m.up ? AXIS_Y - stem * p : AXIS_Y, width: 1.5, height: stem * p, background: hexA(C.leaf, 0.6) }} />
              <div style={{ position: "absolute", left: m.x - 130, top: m.up ? AXIS_Y + 26 : AXIS_Y - 50, width: 260, textAlign: "center", fontFamily: FONT.mono, fontSize: 18, letterSpacing: "0.14em", color: C.ink }}>{m.date}</div>
              <div style={{ position: "absolute", left: m.x - 230, top: cardTop + (1 - p) * (m.up ? 30 : -30), width: 460, height: 230, borderRadius: 22, background: hexA(C.appGround, 0.95), border: `1.5px solid ${hexA(C.sage, 0.16)}`, boxShadow: "0 30px 80px rgba(0,0,0,0.5)", padding: "20px 24px", display: "flex", flexDirection: "column", gap: 10 }}>
                <div style={{ display: "flex", justifyContent: "space-between", fontFamily: FONT.mono, fontSize: 14, letterSpacing: "0.12em", color: C.muted }}>
                  <span>{m.kind}</span>
                  <span>v1</span>
                </div>
                <div style={{ fontFamily: FONT.display, fontWeight: 600, fontSize: 24, color: C.ink }}>{m.title}</div>
                <div style={{ fontFamily: FONT.body, fontSize: 19, lineHeight: 1.35, color: C.sage }}>{m.text}</div>
                <div style={{ marginTop: "auto", display: "flex", gap: 18, fontFamily: FONT.mono, fontSize: 14, letterSpacing: "0.1em" }}>
                  <span style={{ color: C.leaf }}>● TERKINI</span>
                  <span style={{ color: C.dim }}>KATA-KATAMU</span>
                </div>
              </div>
            </div>
          );
        })}
        <div style={{ position: "absolute", left: 0, right: 0, top: 975, display: "flex", flexDirection: "column", alignItems: "center", gap: 12 }}>
          <Kinetic text="NO SCORES. NO *DIAGNOSIS.*" at={q.noScores} size={50} weight={700} tracking={0.02} />
        </div>
      </AbsoluteFill>

      {/* The real Life Context Timeline */}
      <Float at={q.ui} style={{ left: 560, top: 150 }} from="bottom">
        <UIShot shot="timeline-list" width={1080} sy={700} sh={1150} scrollTo={1180} scrollFrom={q.ui + 40} scrollDur={90} rings={[{ x: 70, y: 780, w: 2000, h: 66, at: q.ui + 18 }]} />
      </Float>
      <AbsoluteFill style={{ left: 140, top: 940, width: 1640, alignItems: "center", opacity: envelope(frame, q.ui + 12, 760, 16, 10) }}>
        <Gloss style={{ textAlign: "center" }}>“This page doesn't use AI, doesn't score you, and doesn't conclude causes or conditions.” — Perjalanan ceritaku</Gloss>
      </AbsoluteFill>
      <Sweep at={q.axis - 10} />
      <Dip inAt={0} len={10} outAt={736} />
    </Ground>
  );
}
