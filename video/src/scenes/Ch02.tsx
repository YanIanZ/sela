// CHAPTER 02 — BEYOND LISTENING (00:20–00:40)
// Real SELA conversation room. The reply shown is SELA's own labelled test
// reply ("Sela · balasan uji (bukan AI)"): no Meta runtime was captured for
// this film, so no Meta provenance is claimed on screen.
import { AbsoluteFill, useCurrentFrame } from "remotion";
import { C, FONT } from "../brand/tokens";
import { ChapterMark, Dip, EASE, envelope, Eyebrow, Float, Gloss, Ground, Kinetic, prog, Stage, Sweep, Tag, UIShot } from "../motion";
import T from "../timeline.json";

const q = T.cues.ch02;

export function Ch02() {
  const frame = useCurrentFrame();
  const uiPhase = frame >= q.ui - 10;
  const replyMix = prog(frame, q.reply, 20, EASE.inOut);
  return (
    <Ground glow={uiPhase ? 0.55 : 0.4} gx={uiPhase ? 66 : 50} grid={0.22}>
      <ChapterMark n="02" title="BEYOND LISTENING" at={q.ui} out={q.out} />
      {/* Typography */}
      <Stage>
        <Kinetic text="AI CAN *LISTEN.*" at={q.listen} out={q.butCanIt - 18} size={168} mode="track" exit="fade" dur={28} />
      </Stage>
      <Stage gap={8}>
        <Kinetic text="BUT CAN IT" at={q.butCanIt} out={q.ui - 26} size={150} weight={700} color={C.muted} />
        <Kinetic text="*UNDERSTAND?*" at={q.understand} out={q.ui - 20} size={200} mode="scale" dur={18} />
      </Stage>

      {/* Product: the conversation room */}
      <Float at={q.ui} out={q.reflection - 6} style={{ left: 980, top: 150 }} from="right">
        <div style={{ position: "relative", width: 780, height: 798 }}>
          <div style={{ position: "absolute", inset: 0, opacity: 1 - replyMix }}>
            <UIShot shot="talk-compose" width={780} sx={560} sw={1760} rings={[{ x: 640, y: 1480, w: 1600, h: 120, at: q.ui + 40 }]} />
          </div>
          <div style={{ position: "absolute", inset: 0, opacity: replyMix }}>
            <UIShot shot="talk-reply" width={780} sx={560} sw={1760} label={false} rings={[{ x: 1629, y: 415, w: 563, h: 240, at: q.reply + 14 }, { x: 700, y: 718, w: 470, h: 46, at: q.reply + 44, color: C.fixture }]} />
          </div>
        </div>
      </Float>

      <AbsoluteFill style={{ left: 140, top: 300, width: 760, opacity: envelope(frame, q.ui + 10, q.reflection - 4, 20, 14) }}>
        <Eyebrow>Fictional adult · synthetic data</Eyebrow>
        <div style={{ marginTop: 28, fontFamily: FONT.body, fontSize: 64, lineHeight: 1.15, color: C.ink, fontWeight: 600, letterSpacing: "-0.01em", opacity: prog(frame, q.ui + 20, 24), transform: `translateY(${(1 - prog(frame, q.ui + 20, 30)) * 30}px)` }}>“Kadang aku butuh waktu sendiri.”</div>
        <Gloss style={{ marginTop: 14, opacity: prog(frame, q.ui + 34, 20) }}>“Sometimes I need time alone.”</Gloss>
        <div style={{ marginTop: 56, opacity: prog(frame, q.reply + 40, 20) }}>
          <Tag color={C.fixture}>Test double · not live Meta output</Tag>
          <Gloss style={{ marginTop: 18, maxWidth: 640 }}>The reply is SELA's labelled test program — “Sela · balasan uji (bukan AI)”. Nothing becomes a memory automatically.</Gloss>
        </div>
      </AbsoluteFill>

      {/* "What Sela noticed": an interpretation, reviewable */}
      <Float at={q.reflection} out={q.out} style={{ left: 840, top: 330 }} from="bottom">
        <UIShot shot="talk-reflection" width={940} rings={[{ x: 80, y: 252, w: 1250, h: 52, at: q.reflection + 30 }, { x: 108, y: 636, w: 600, h: 116, at: q.reflection + 66 }]} />
      </Float>
      <AbsoluteFill style={{ left: 140, top: 330, width: 680, opacity: envelope(frame, q.reflection + 8, q.out + 10, 16, 14) }}>
        <Kinetic text="AN INTERPRETATION." at={q.reflection + 6} size={58} align="left" tracking={-0.02} />
        <div style={{ height: 14 }} />
        <Kinetic text="*NOT A FACT.*" at={q.reflection + 22} size={58} align="left" tracking={-0.02} />
        <Gloss style={{ marginTop: 34, opacity: prog(frame, q.reflection + 36, 18) }}>“This is an AI interpretation, not a fact. The quote below is your own wording; confirm, correct or reject it.”</Gloss>
        <div style={{ display: "flex", gap: 14, marginTop: 34, opacity: prog(frame, q.reflection + 70, 18) }}>
          <Tag>Confirm</Tag>
          <Tag>Correct</Tag>
          <Tag>Reject</Tag>
        </div>
      </AbsoluteFill>

      <Sweep at={q.ui - 18} dur={30} />
      <Dip inAt={0} len={10} outAt={q.out + 18} />
    </Ground>
  );
}
