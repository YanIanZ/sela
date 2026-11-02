// CHAPTER 08 — FINAL REVEAL (02:45–03:00)
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { C, FONT } from "../brand/tokens";
import { Dip, EASE, Eyebrow, Fragments, Ground, Kinetic, mix, prog, PulseRing, SproutMark } from "../motion";
import T from "../timeline.json";

const q = T.cues.ch08;

export function Ch08() {
  const frame = useCurrentFrame();
  const converge = prog(frame, q.converge, 90, EASE.inOut);
  const grow = prog(frame, q.logo, 40, EASE.out);
  const taglineOut = prog(frame, q.wordmark - 24, 20, EASE.in);
  const lockup = prog(frame, q.wordmark, 36, EASE.out);
  // the mark travels from centre stage to the lock-up position
  const markY = mix(-150, -120, lockup);
  const markX = mix(0, -235, lockup);
  const markSize = mix(200, 150, lockup);
  return (
    <Ground glow={interpolate(frame, [0, 90, q.logo, q.wordmark, 450], [0.4, 0.1, 0.6, 0.7, 0.55], { extrapolateRight: "clamp" })} grid={interpolate(frame, [0, 80], [0.2, 0], { extrapolateRight: "clamp" })}>
      {/* fragments from every chapter converge to a point */}
      <Fragments at={-120} count={26} converge={converge} opacity={1 - prog(frame, 70, 30)} seed="end" speed={0.5} />
      <PulseRing at={q.logo - 4} max={1300} dur={60} />

      <AbsoluteFill style={{ alignItems: "center", justifyContent: "center" }}>
        <div style={{ position: "absolute", transform: `translate(${markX}px, ${markY}px)`, opacity: grow }}>
          <SproutMark size={markSize} grow={grow} ring={prog(frame, q.logo + 10, 30)} />
        </div>
      </AbsoluteFill>

      {/* YOUR STORY. YOUR MEANING. YOUR NEXT STEP. */}
      <AbsoluteFill style={{ alignItems: "center", justifyContent: "center", paddingTop: 230, gap: 6, opacity: 1 - taglineOut }}>
        <Kinetic text="YOUR STORY." at={q.story} size={96} />
        <Kinetic text="YOUR MEANING." at={q.meaning} size={96} />
        <Kinetic text="YOUR *NEXT STEP.*" at={q.next} size={96} />
      </AbsoluteFill>

      {/* Lock-up */}
      <AbsoluteFill style={{ alignItems: "center", justifyContent: "center" }}>
        <div style={{ transform: `translate(${mix(40, 85, lockup)}px, -120px)`, fontFamily: FONT.display, fontWeight: 700, fontSize: 190, letterSpacing: "-0.04em", color: C.ink, opacity: lockup, filter: `blur(${(1 - lockup) * 10}px)` }}>Sela</div>
      </AbsoluteFill>
      <AbsoluteFill style={{ alignItems: "center", justifyContent: "center", paddingTop: 170 }}>
        <Kinetic text="BEYOND BEING *HEARD.*" at={q.beyond} size={74} mode="track" dur={28} tracking={0.04} weight={700} />
      </AbsoluteFill>
      <AbsoluteFill style={{ alignItems: "center", justifyContent: "flex-end", paddingBottom: 110, gap: 14, opacity: prog(frame, q.built, 24) }}>
        <Eyebrow color={C.sage}>Built with Meta AI · Build for Your World</Eyebrow>
        <div style={{ fontFamily: FONT.mono, fontSize: 17, letterSpacing: "0.08em", color: C.dim }}>sela.vertalt.my.id · synthetic demo data · simulated partners · not a medical or crisis service</div>
      </AbsoluteFill>
      <Dip inAt={0} len={8} />
    </Ground>
  );
}
