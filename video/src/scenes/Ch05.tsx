// CHAPTER 05 — PERSONAL ACTION BRIDGE (01:40–02:00)
// "Satu langkah, pilihanku" (/notes/steps, D-081): a step the person writes
// or picks, saved privately. No reminders, streaks or scores; the Meta
// suggestion is optional and never stored (TestV3ActionSuggestionIsProposalOnly).
// Here Nadia writes her own step, so no AI wording appears.
import { AbsoluteFill, useCurrentFrame } from "remotion";
import { C, FONT } from "../brand/tokens";
import { ChapterMark, Dip, EASE, envelope, Eyebrow, Float, Gloss, Ground, Kinetic, prog, Stage, Sweep, Tag, UIShot } from "../motion";
import T from "../timeline.json";

const q = T.cues.ch05;

export function Ch05() {
  const frame = useCurrentFrame();
  const saved = prog(frame, q.saved, 20, EASE.inOut);
  return (
    <Ground glow={0.55} gx={64} grid={0.22}>
      <ChapterMark n="05" title="PERSONAL ACTION BRIDGE" at={q.write} out={584} />
      <Stage gap={8}>
        <Kinetic text="UNDERSTANDING" at={q.understanding} out={q.next - 18} size={160} weight={700} color={C.muted} />
        <Kinetic text="IS A *BEGINNING.*" at={q.beginning} out={q.next - 14} size={160} />
      </Stage>
      <Stage>
        <Kinetic text="YOUR *NEXT STEP.*" at={q.next} out={q.write - 12} size={190} mode="scale" dur={18} />
      </Stage>

      <Float at={q.write} out={586} style={{ left: 820, top: 180 }} from="right">
        <div style={{ position: "relative", width: 960, height: 560 }}>
          <div style={{ position: "absolute", inset: 0, opacity: 1 - saved }}>
            <UIShot shot="steps-write" width={960} sx={60} sw={2480} sy={720} sh={1440} rings={[{ x: 110, y: 1190, w: 2330, h: 180, at: q.write + 30 }]} />
          </div>
          <div style={{ position: "absolute", inset: 0, opacity: saved }}>
            <UIShot shot="steps-saved" width={960} sx={60} sw={2480} sy={720} sh={1000} label={false} rings={[{ x: 80, y: 735, w: 2300, h: 70, at: q.saved + 40, color: C.leaf }]} />
          </div>
        </div>
      </Float>

      <AbsoluteFill style={{ left: 140, top: 290, width: 620, opacity: envelope(frame, q.write + 8, 586, 16, 14) }}>
        <Eyebrow>Optional · written by Nadia</Eyebrow>
        <div style={{ marginTop: 26, fontFamily: FONT.body, fontWeight: 600, fontSize: 52, lineHeight: 1.18, color: C.ink }}>“Tuliskan dua pertanyaan untuk konsultasi.”</div>
        <Gloss style={{ marginTop: 14 }}>“Write down two questions for the consultation.”</Gloss>
        <div style={{ marginTop: 54, opacity: prog(frame, q.saved + 24, 18) }}>
          <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
            <Tag>No reminders</Tag>
            <Tag>No streaks</Tag>
            <Tag>Not treatment</Tag>
          </div>
          <Gloss style={{ marginTop: 18 }}>“No reminders or day counts. This step is not treatment.”</Gloss>
        </div>
      </AbsoluteFill>
      <AbsoluteFill style={{ top: 860, alignItems: "center", opacity: envelope(frame, q.optional, 586, 10, 14) }}>
        <Kinetic text="*OPTIONAL.* ALWAYS YOURS TO SKIP." at={q.optional} size={54} weight={700} tracking={0.01} />
      </AbsoluteFill>
      <Sweep at={q.write - 14} />
      <Dip inAt={0} len={10} outAt={586} />
    </Ground>
  );
}
