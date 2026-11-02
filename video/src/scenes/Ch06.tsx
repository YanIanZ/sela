// CHAPTER 06 — CARE PASSPORT (02:00–02:25)
// /support/passport (D-081) is a view of the consultation summary: chosen
// source quotes pinned to their versions, approval of the exact version, and a
// recipient preview built by the same projectSummary used for shares and care
// requests (TestV3CarePassport). SELA itself labels it "Bukan paspor medis".
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { C, FONT } from "../brand/tokens";
import { ChapterMark, Dip, EASE, envelope, Eyebrow, Float, Gloss, Ground, hexA, Kinetic, mix, prog, Stage, Sweep, Tag, UIShot } from "../motion";
import T from "../timeline.json";

const q = T.cues.ch06;

const SOURCES = [
  { id: "n1", kind: "CATATAN · v1 · 27 OKT", quote: "Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.", pick: q.pick1 },
  { id: "n2", kind: "CEK PERASAAN · v1 · 30 OKT", quote: "Hari ini aku agak kewalahan dengan tugas. Aku belum tahu bagian mana yang paling membuatku lelah.", pick: q.pick2 },
  { id: "n3", kind: "PERCAKAPAN · v1 · 1 NOV", quote: "Aku ingin tahu bagaimana menjelaskan minggu yang melelahkan ini…", pick: -1 },
];
const DOC = { x: 1010, y: 150, w: 770, h: 800 };

export function Ch06() {
  const frame = useCurrentFrame();
  const asm = envelope(frame, q.cards - 4, q.ui + 6, 14, 18);
  return (
    <Ground glow={0.55} gx={50} grid={0.24}>
      <ChapterMark n="06" title="CARE PASSPORT · CONSULTATION PREPARATION" at={q.cards} out={734} />
      <Stage gap={4} x={-60}>
        {[
          { w: "WORDS.", at: q.words },
          { w: "CHOICE.", at: q.choice },
          { w: "APPROVAL.", at: q.approval },
        ].map((l) => (
          <div key={l.w} style={{ display: "flex", gap: 40, alignItems: "baseline" }}>
            <Kinetic text="YOUR" at={l.at} out={q.cards - 22} size={150} weight={600} color={C.muted} />
            <Kinetic text={`*${l.w}*`} at={l.at + 4} out={q.cards - 20} size={150} />
          </div>
        ))}
      </Stage>

      {/* Source cards assemble into a personal preparation sheet */}
      <AbsoluteFill style={{ opacity: asm }}>
        <div style={{ position: "absolute", left: 140, top: 160 }}>
          <Eyebrow color={C.muted}>Choose sources · exact quotes · pinned versions</Eyebrow>
        </div>
        {/* the sheet */}
        <div style={{ position: "absolute", left: DOC.x, top: DOC.y, width: DOC.w, height: DOC.h, borderRadius: 26, background: hexA(C.appGround, 0.96), border: `1.5px solid ${hexA(C.sage, 0.18)}`, boxShadow: "0 40px 120px rgba(0,0,0,0.6)", padding: "40px 46px", opacity: prog(frame, q.cards + 20, 30), transform: `translateY(${(1 - prog(frame, q.cards + 20, 40)) * 40}px)` }}>
          <div style={{ fontFamily: FONT.mono, fontSize: 15, letterSpacing: "0.16em", color: C.muted }}>RINGKASAN UNTUK DIBAWA · v2</div>
          <div style={{ fontFamily: FONT.display, fontWeight: 700, fontSize: 40, color: C.ink, marginTop: 14 }}>Hal yang ingin kuceritakan</div>
          <div style={{ fontFamily: FONT.mono, fontSize: 18, color: C.dim, marginTop: 6 }}>What I want to discuss</div>
          <div style={{ fontFamily: FONT.display, fontWeight: 600, fontSize: 22, color: C.sage, marginTop: 30 }}>Dengan kata-kataku</div>
          <div style={{ height: 300 }} />
          <div style={{ fontFamily: FONT.display, fontWeight: 600, fontSize: 22, color: C.sage, opacity: prog(frame, q.assemble + 30, 20) }}>Yang ingin kutanyakan</div>
          {["Bagaimana aku bisa menjelaskan apa yang kurasakan tanpa harus langsung menyimpulkan penyebabnya?", "Apa informasi dari catatan pribadiku yang paling berguna untuk kubawa saat konsultasi?"].map((t, i) => (
            <div key={i} style={{ fontFamily: FONT.body, fontSize: 19, lineHeight: 1.35, color: C.ink, marginTop: 10, opacity: prog(frame, q.assemble + 40 + i * 10, 18) }}>
              {i + 1}. {t}
            </div>
          ))}
          <div style={{ position: "absolute", left: 46, right: 46, bottom: 34, display: "flex", justifyContent: "space-between", alignItems: "center", opacity: prog(frame, q.assemble + 60, 20) }}>
            <span style={{ fontFamily: FONT.mono, fontSize: 14, letterSpacing: "0.12em", color: C.muted }}>PERSONAL PREPARATION SHEET · NOT A MEDICAL RECORD</span>
          </div>
        </div>
        {/* cards */}
        {SOURCES.map((s, i) => {
          const appear = prog(frame, q.cards + i * 10, 26);
          const picked = s.pick > 0 ? prog(frame, s.pick, 14) : 0;
          const fly = s.pick > 0 ? prog(frame, q.assemble + i * 8, 40, EASE.inOut) : 0;
          const x0 = 140, y0 = 240 + i * 230;
          const x1 = DOC.x + 46, y1 = DOC.y + 200 + i * 150;
          const x = mix(x0, x1, fly), y = mix(y0, y1, fly);
          const w = mix(600, DOC.w - 92, fly);
          const left = s.pick < 0 && frame > q.assemble;
          return (
            <div key={s.id} style={{ position: "absolute", left: x, top: y, width: w, opacity: appear * (left ? mix(1, 0.45, prog(frame, q.assemble, 20)) : 1), transform: `translateY(${(1 - appear) * 30}px)` }}>
              <div style={{ borderRadius: 18, background: fly > 0.95 ? "transparent" : hexA(C.surface, 0.95), border: `1.5px solid ${fly > 0.95 ? "transparent" : hexA(picked ? C.leaf : C.sage, picked ? 0.6 : 0.15)}`, padding: fly > 0.95 ? "0 0 0 20px" : "18px 22px", borderLeft: fly > 0.5 ? `3px solid ${C.leafSoft}` : undefined, boxShadow: fly > 0.95 ? "none" : "0 20px 60px rgba(0,0,0,0.45)" }}>
                {fly < 0.6 && (
                  <div style={{ display: "flex", alignItems: "center", gap: 12, fontFamily: FONT.mono, fontSize: 14, letterSpacing: "0.12em", color: C.muted, marginBottom: 10 }}>
                    <span style={{ width: 22, height: 22, borderRadius: 6, border: `2px solid ${picked ? C.leaf : C.dim}`, background: picked ? C.leaf : "transparent", color: C.void, fontSize: 16, display: "inline-flex", alignItems: "center", justifyContent: "center", fontWeight: 900 }}>{picked > 0.5 ? "✓" : ""}</span>
                    {s.kind}
                  </div>
                )}
                <div style={{ fontFamily: FONT.body, fontSize: mix(22, 19, fly), lineHeight: 1.35, color: C.ink }}>“{s.quote}”</div>
                {fly > 0.9 && <div style={{ fontFamily: FONT.mono, fontSize: 13, color: C.dim, marginTop: 6, letterSpacing: "0.08em" }}>{i === 0 ? "Sepulang kuliah · v1" : "Cek perasaan · v1"}</div>}
              </div>
              {left && (
                <div style={{ marginTop: 14, opacity: prog(frame, q.assemble + 10, 16) }}>
                  <Tag color={C.muted} size={16}>Not included · stays private</Tag>
                </div>
              )}
            </div>
          );
        })}
        <div style={{ position: "absolute", left: DOC.x + 46, top: DOC.y + DOC.h - 140, opacity: prog(frame, q.approve, 14), transform: `scale(${mix(1.3, 1, prog(frame, q.approve, 18))})`, transformOrigin: "left center" }}>
          <Tag color={C.leaf} solid>
            Approved · exact version v2
          </Tag>
        </div>
        <div style={{ position: "absolute", left: 140, top: 940, opacity: prog(frame, q.approve + 14, 16) }}>
          <Gloss>Only an approved, current version can be shared.</Gloss>
        </div>
      </AbsoluteFill>

      {/* The real screen */}
      <Float at={q.ui} out={736} style={{ left: 900, top: 130 }} from="right">
        <UIShot shot="passport-approved" width={860} sx={680} sw={1360} sy={1180} sh={1100} scrollTo={4150} scrollFrom={q.ui + 70} scrollDur={110} rings={[{ x: 720, y: 1270, w: 1280, h: 330, at: q.ui + 26, color: C.leaf }]} />
      </Float>
      <AbsoluteFill style={{ left: 140, top: 300, width: 660, opacity: envelope(frame, q.ui + 10, 736, 16, 14) }}>
        <Eyebrow>Ringkasan untuk dibawa</Eyebrow>
        <div style={{ marginTop: 22, fontFamily: FONT.display, fontWeight: 700, fontSize: 60, lineHeight: 1.08, color: C.ink }}>
          “Bukan paspor <span style={{ color: C.leaf }}>medis.</span>”
        </div>
        <Gloss style={{ marginTop: 16 }}>“Not a medical passport.” SELA says so on the screen: not an official health record, diagnosis or professional document.</Gloss>
        <div style={{ marginTop: 50, opacity: prog(frame, q.ui + 100, 18) }}>
          <Eyebrow color={C.muted}>Yang diterima penerima</Eyebrow>
          <Gloss style={{ marginTop: 14, color: C.ink }}>The recipient preview is exactly what is shared — built by the same server projection as the share link.</Gloss>
        </div>
      </AbsoluteFill>
      <Sweep at={q.cards - 12} />
      <Dip inAt={0} len={10} outAt={736} />
    </Ground>
  );
}
