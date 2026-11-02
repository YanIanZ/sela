// CHAPTER 07 — SUPPORT, ON YOUR TERMS (02:25–02:45)
// Care network (D-076): a structured request from an approved summary to a
// simulated partner, which proposes a time; the person confirms or cancels.
// Cancelling revokes the hidden share (TestCareNetworkEndToEnd). Partners in
// this film are fictional and always labelled SIMULATED PROFESSIONAL.
// The partner-side Professional Workspace (/operator) was not filmed: it
// needs a staff sign-in that the offline capture does not fake.
import { AbsoluteFill, useCurrentFrame } from "remotion";
import { C, FONT } from "../brand/tokens";
import { ChapterMark, Dip, EASE, envelope, Eyebrow, Float, Gloss, Ground, hexA, Kinetic, mix, prog, SproutMark, Stage, Sweep, Tag, UIShot } from "../motion";
import T from "../timeline.json";

const q = T.cues.ch07;
const A = { x: 520, y: 520 };
const B = { x: 1400, y: 520 };

function Packet({ at, dur, from, to, label, color = C.leaf }: { at: number; dur: number; from: typeof A; to: typeof A; label: string; color?: string }) {
  const frame = useCurrentFrame();
  const p = prog(frame, at, dur, EASE.inOut);
  if (frame < at || frame > at + dur + 30) return null;
  const x = mix(from.x, to.x, p);
  const fade = 1 - prog(frame, at + dur, 24);
  return (
    <div style={{ position: "absolute", left: x, top: from.y, transform: "translate(-50%, -50%)", opacity: fade }}>
      <div style={{ width: 16, height: 16, borderRadius: 99, background: color, boxShadow: `0 0 30px ${color}`, margin: "0 auto" }} />
      <div style={{ marginTop: 18, whiteSpace: "nowrap", fontFamily: FONT.mono, fontSize: 18, letterSpacing: "0.1em", color, textAlign: "center" }}>{label}</div>
    </div>
  );
}

export function Ch07() {
  const frame = useCurrentFrame();
  const link = prog(frame, q.connect, 30, EASE.inOut);
  const cut = prog(frame, q.revoke, 16, EASE.out);
  const diagOn = envelope(frame, q.connect - 6, q.ui + 6, 14, 16);
  const confirmed = frame >= q.confirm && frame < q.revoke;
  return (
    <Ground glow={0.5} gx={50} grid={0.24}>
      <ChapterMark n="07" title="SUPPORT, ON YOUR TERMS" at={q.connect} out={584} />
      <Stage>
        <Kinetic text="WHEN YOU'RE *READY.*" at={q.ready} out={q.support - 18} size={150} mode="track" exit="fade" dur={26} />
      </Stage>
      <Stage gap={8}>
        <Kinetic text="SUPPORT" at={q.support} out={q.connect - 14} size={170} weight={700} />
        <Kinetic text="*ON YOUR TERMS.*" at={q.terms} out={q.connect - 12} size={170} mode="scale" dur={18} />
      </Stage>

      {/* A controlled connection */}
      <AbsoluteFill style={{ opacity: diagOn }}>
        {/* the line, in two halves so revocation can open a gap */}
        {[0, 1].map((h) => {
          const mid = (A.x + B.x) / 2;
          const x0 = h === 0 ? A.x + 90 : mid + cut * 60;
          const x1 = h === 0 ? mid - cut * 60 : B.x - 90;
          const len = (x1 - x0) * Math.min(1, link * 2 - h);
          const col = cut > 0 ? C.stopped : confirmed ? C.leaf : hexA(C.leaf, 0.6);
          return <div key={h} style={{ position: "absolute", left: x0, top: A.y - 1.5, width: Math.max(0, len), height: 3, background: col, boxShadow: confirmed ? `0 0 24px ${C.leafSoft}` : "none", opacity: cut > 0 ? 0.7 : 1 }} />;
        })}
        {/* Nadia */}
        <div style={{ position: "absolute", left: A.x - 80, top: A.y - 80, width: 160, height: 160, borderRadius: 99, border: `2px solid ${hexA(C.leaf, 0.5)}`, display: "flex", alignItems: "center", justifyContent: "center", background: hexA(C.appGround, 0.95), opacity: prog(frame, q.connect - 6, 20) }}>
          <SproutMark size={86} ring={0} />
        </div>
        <div style={{ position: "absolute", left: A.x - 200, top: A.y + 104, width: 400, textAlign: "center", opacity: prog(frame, q.connect, 20) }}>
          <div style={{ fontFamily: FONT.display, fontWeight: 600, fontSize: 30, color: C.ink }}>Nadia</div>
          <Gloss style={{ fontSize: 17 }}>fictional adult · owns the summary</Gloss>
        </div>
        {/* Simulated partner */}
        <div style={{ position: "absolute", left: B.x - 80, top: B.y - 80, width: 160, height: 160, borderRadius: 30, border: `2px dashed ${hexA(C.sim, 0.7)}`, display: "flex", alignItems: "center", justifyContent: "center", background: hexA(C.appGround, 0.95), fontFamily: FONT.mono, fontSize: 18, letterSpacing: "0.1em", color: C.sim, opacity: prog(frame, q.connect + 8, 20) }}>
          DEMO
        </div>
        <div style={{ position: "absolute", left: B.x - 240, top: B.y - 160, width: 480, display: "flex", justifyContent: "center", opacity: prog(frame, q.connect + 10, 20) }}>
          <Tag color={C.sim} solid size={18}>
            Simulated professional
          </Tag>
        </div>
        <div style={{ position: "absolute", left: B.x - 220, top: B.y + 104, width: 440, textAlign: "center", opacity: prog(frame, q.connect + 8, 20) }}>
          <div style={{ fontFamily: FONT.display, fontWeight: 600, fontSize: 30, color: C.ink }}>Klinik Cerita Demo</div>
          <Gloss style={{ fontSize: 17 }}>fictional partner · not a real clinic</Gloss>
        </div>
        <Packet at={q.send} dur={40} from={A} to={B} label="APPROVED SUMMARY v2" />
        <Packet at={q.propose} dur={34} from={B} to={A} label="PROPOSED TIME · SIMULATED" color={C.sim} />
        {confirmed && (
          <div style={{ position: "absolute", left: 0, right: 0, top: A.y - 70, textAlign: "center", fontFamily: FONT.mono, fontSize: 19, letterSpacing: "0.14em", color: C.leaf, opacity: prog(frame, q.confirm, 12) }}>CONFIRMED BY NADIA</div>
        )}
        {cut > 0 && (
          <div style={{ position: "absolute", left: 0, right: 0, top: A.y - 84, display: "flex", flexDirection: "column", alignItems: "center", gap: 10, opacity: cut }}>
            <Tag color={C.stopped}>Request cancelled · access ended</Tag>
            <Gloss style={{ fontSize: 18 }}>“The service can no longer read your summary.”</Gloss>
          </div>
        )}
        <div style={{ position: "absolute", left: 0, right: 0, top: 860, textAlign: "center", opacity: prog(frame, q.revoke + 10, 16) }}>
          <Gloss style={{ color: C.ink }}>Copies already viewed cannot be recalled. SELA says so before you share.</Gloss>
        </div>
        <div style={{ position: "absolute", left: 0, right: 0, top: 940, textAlign: "center", opacity: prog(frame, q.connect + 30, 20) }}>
          <Gloss style={{ fontSize: 18, color: C.dim }}>Demo only · no real appointment, partnership or care delivered</Gloss>
        </div>
      </AbsoluteFill>

      {/* The real screens */}
      <Float at={q.ui} out={586} style={{ left: 140, top: 230 }} from="left">
        <UIShot shot="care-proposed" width={820} sx={90} sw={1930} sy={3560} sh={1060} rings={[{ x: 180, y: 3700, w: 760, h: 80, at: q.ui + 30, color: C.sim }]} />
        <Gloss style={{ marginTop: 20, fontSize: 19 }}>Status permintaanmu · “Times proposed” · Simulasi</Gloss>
      </Float>
      <Float at={q.revokedUi} out={586} style={{ left: 1080, top: 330 }} from="right">
        <UIShot shot="share-revoked" width={700} sx={700} sw={1340} sy={760} sh={520} label={false} rings={[{ x: 730, y: 930, w: 1250, h: 80, at: q.revokedUi + 24, color: C.stopped }]} />
        <Gloss style={{ marginTop: 20, fontSize: 19 }}>Bagikan ringkasan · “Revoked by you”</Gloss>
      </Float>
      <Sweep at={q.connect - 12} />
      <Dip inAt={0} len={10} outAt={586} />
    </Ground>
  );
}
