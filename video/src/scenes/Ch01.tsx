// CHAPTER 01 — EVERY STORY MATTERS (00:00–00:20)
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { C } from "../brand/tokens";
import { Dip, Dot, EASE, Fragments, Ground, Kinetic, prog, PulseRing, Stage, Sweep } from "../motion";
import T from "../timeline.json";

const q = T.cues.ch01;

export function Ch01() {
  const frame = useCurrentFrame();
  // Glow wakes with the pulse, dims in the silence, returns with the question.
  const glow = interpolate(frame, [0, q.pulse1, q.every, q.silenceFrom, q.silenceTo, q.understands, 600], [0, 0.2, 0.7, 0.15, 0.15, 0.85, 0.3], { extrapolateRight: "clamp" });
  const closeP = prog(frame, q.close, 50, EASE.inOut);
  return (
    <Ground glow={glow} grid={interpolate(frame, [q.every, q.every + 60], [0, 0.25], { extrapolateLeft: "clamp", extrapolateRight: "clamp" })}>
      {/* Fragments of fictional written experience, drifting at depth */}
      <Fragments at={q.matters + 10} count={18} opacity={interpolate(frame, [q.silenceFrom, q.silenceTo, q.understands, q.close], [0.34, 0.16, 0.42, 0.1], { extrapolateLeft: "clamp", extrapolateRight: "clamp" })} converge={interpolate(frame, [q.butWho, q.understands + 30], [0, 0.18], { extrapolateLeft: "clamp", extrapolateRight: "clamp" })} />

      {/* A faint pulse in complete darkness */}
      {frame < q.every + 6 && <Dot at={q.pulse1 - 18} />}
      <PulseRing at={q.pulse1} max={700} />
      <PulseRing at={q.pulse2} max={1100} />
      <PulseRing at={q.every} max={1800} dur={50} width={1.5} />

      <AbsoluteFill style={{ transform: `scaleY(${1 - closeP * 0.92}) translateY(${closeP * 0}px)`, opacity: 1 - prog(frame, q.close + 30, 20) }}>
        <Stage gap={6}>
          <Kinetic text="EVERY STORY" at={q.every} out={q.silenceFrom - 16} size={190} mode="scale" exit="track" dur={18} stagger={3} />
          <Kinetic text="*MATTERS.*" at={q.matters} out={q.silenceFrom - 10} size={190} exit="track" dur={20} />
        </Stage>
        <Stage gap={6}>
          <Kinetic text="BUT WHO" at={q.butWho} size={170} weight={700} color={C.muted} dur={22} />
          <Kinetic text="UNDERSTANDS*?*" at={q.understands} size={210} mode="fragment" />
        </Stage>
      </AbsoluteFill>

      {/* The line the question collapses into */}
      {closeP > 0 && (
        <AbsoluteFill style={{ alignItems: "center", justifyContent: "center" }}>
          <div style={{ width: `${closeP * 90}%`, height: 2, background: `linear-gradient(90deg, transparent, ${C.leaf}, transparent)`, opacity: 1 - prog(frame, 585, 15), boxShadow: `0 0 30px ${C.leafSoft}` }} />
        </AbsoluteFill>
      )}
      <Sweep at={q.understands - 2} dur={30} />
      <Dip inAt={0} len={1} />
    </Ground>
  );
}
