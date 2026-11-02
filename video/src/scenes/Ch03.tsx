// CHAPTER 03 — MEANING INTEGRITY ENGINE (00:40–01:15)
//
// Truth constraints for this sequence:
// - The wrong interpretation is a SCRIPTED TEST FIXTURE, labelled as such
//   before it is shown. It is not a live Meta output.
// - The Meaning Diff codes on screen (frequency: critical, added_content:
//   review) are the real output of internal/ledger.CompareMeaning for these
//   exact strings (video/evidence/meaning-diff/output.txt).
// - The graph mirrors TestMeaningCorrectionPropagates
//   (internal/httpapi/meaning_test.go): one summary marked stale, two share
//   links revoked (the user's link + the care request's hidden share), one
//   care request cancelled, in one transaction. The source stays current.
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { C, FONT } from "../brand/tokens";
import { ChapterMark, Counter, Dip, EASE, envelope, Eyebrow, Float, Gloss, Ground, hexA, Kinetic, mix, prog, PulseRing, Stage, Sweep, Tag, UIShot } from "../motion";
import T from "../timeline.json";

const q = T.cues.ch03;

// ---------------- the dependency graph ----------------
type NodeState = { label: string; color: string; text: string; meta: string };
type GNode = { id: string; x: number; y: number; w: number; kind: string; before: NodeState; after?: NodeState; hitAt?: number; fixture?: boolean };

const NW = 330;
const NODES: GNode[] = [
  { id: "source", x: 150, y: 400, w: NW, kind: "SOURCE · NOTE · v1", before: { label: "CURRENT", color: C.leaf, text: "“Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.”", meta: "Nadia's own words" } },
  {
    id: "claim", x: 560, y: 400, w: NW, kind: "REMEMBERED MEANING", fixture: true,
    before: { label: "v2 · CONFIRMED", color: C.fixture, text: "“Kamu selalu butuh waktu sendiri dan menghindari teman.”", meta: "test fixture · simulated AI error" },
    after: { label: "v3 · YOUR CORRECTION", color: C.leaf, text: "“Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman.”", meta: "Nadia's correction" },
    hitAt: q.correct,
  },
  {
    id: "summary", x: 970, y: 400, w: NW, kind: "CONSULTATION SUMMARY · v1",
    before: { label: "APPROVED", color: C.leaf, text: "Persiapan konsultasi", meta: "cites the remembered meaning" },
    after: { label: "NEEDS REVIEW", color: C.stale, text: "Persiapan konsultasi", meta: "stale · claim_corrected · back to draft" },
    hitAt: q.stale,
  },
  {
    id: "share", x: 1440, y: 210, w: NW, kind: "SHARE LINK",
    before: { label: "ACTIVE", color: C.leaf, text: "Konselor (simulasi)", meta: "link + access code" },
    after: { label: "STOPPED", color: C.stopped, text: "Konselor (simulasi)", meta: "meaning_invalidated · next read denied" },
    hitAt: q.revoked,
  },
  {
    id: "care", x: 1440, y: 620, w: NW, kind: "CARE REQUEST",
    before: { label: "SENT", color: C.leaf, text: "Simulated partner", meta: "reads only via its own hidden share" },
    after: { label: "CANCELLED", color: C.stopped, text: "Simulated partner", meta: "hidden share revoked · content unavailable" },
    hitAt: q.cancelled,
  },
];
const byId = (id: string) => NODES.find((n) => n.id === id)!;
const NH = 230;
const port = (n: GNode, side: "l" | "r") => ({ x: side === "l" ? n.x : n.x + n.w, y: n.y + NH / 2 });

type Edge = { from: string; to: string; waveAt: number; waveDur: number; via?: string };
const EDGES: Edge[] = [
  { from: "source", to: "claim", waveAt: -1, waveDur: 0 },
  { from: "claim", to: "summary", waveAt: q.wave, waveDur: q.stale - q.wave },
  { from: "summary", to: "share", waveAt: q.stale, waveDur: q.revoked - q.stale },
  { from: "summary", to: "care", waveAt: q.stale, waveDur: q.cancelled - q.stale, via: "hidden share" },
];

function edgePath(e: Edge) {
  const a = port(byId(e.from), "r");
  const b = port(byId(e.to), "l");
  const mx = (a.x + b.x) / 2;
  return { d: `M ${a.x} ${a.y} C ${mx} ${a.y}, ${mx} ${b.y}, ${b.x} ${b.y}`, a, b, mx };
}
function bez(t: number, a: { x: number; y: number }, b: { x: number; y: number }, mx: number) {
  const p0 = a, p1 = { x: mx, y: a.y }, p2 = { x: mx, y: b.y }, p3 = b;
  const u = 1 - t;
  return { x: u * u * u * p0.x + 3 * u * u * t * p1.x + 3 * u * t * t * p2.x + t * t * t * p3.x, y: u * u * u * p0.y + 3 * u * u * t * p1.y + 3 * u * t * t * p2.y + t * t * t * p3.y };
}

function Graph({ start }: { start: number }) {
  const frame = useCurrentFrame();
  return (
    <AbsoluteFill>
      <svg width={1920} height={1080} style={{ position: "absolute", inset: 0 }}>
        <defs>
          <filter id="g" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="6" />
          </filter>
        </defs>
        {EDGES.map((e, i) => {
          const { d, a, b, mx } = edgePath(e);
          const draw = prog(frame, q.edges + i * 12, 40, EASE.inOut);
          const len = 1400;
          const cut = e.waveAt > 0 ? prog(frame, e.waveAt, e.waveDur, EASE.inOut) : 0;
          const authority = e.waveAt > 0 && frame >= e.waveAt + e.waveDur;
          const color = authority ? C.stopped : C.leaf;
          // particles flowing along live dependencies
          const particles = !authority && draw >= 1 ? [0, 1, 2].map((k) => ((frame - start) / 60 + k / 3) % 1) : [];
          const wave = e.waveAt > 0 && frame >= e.waveAt && frame < e.waveAt + e.waveDur + 8 ? bez(Math.min(1, cut), a, b, mx) : null;
          const viaPt = e.via ? bez(0.62, a, b, mx) : null;
          return (
            <g key={i}>
              <path d={d} fill="none" stroke={hexA(color, authority ? 0.35 : 0.55)} strokeWidth={2.5} strokeDasharray={authority ? "6 12" : `${len}`} strokeDashoffset={authority ? 0 : len * (1 - draw)} />
              {particles.map((t, k) => {
                const p = bez(t, a, b, mx);
                return <circle key={k} cx={p.x} cy={p.y} r={4} fill={C.leaf} opacity={0.8} />;
              })}
              {wave && (
                <>
                  <circle cx={wave.x} cy={wave.y} r={26} fill={hexA(C.sage, 0.5)} filter="url(#g)" />
                  <circle cx={wave.x} cy={wave.y} r={8} fill={C.ink} />
                </>
              )}
              {viaPt && draw > 0.6 && (
                <g opacity={prog(frame, q.edges + 50, 20)}>
                  <rect x={viaPt.x - 70} y={viaPt.y - 18} width={140} height={36} rx={18} fill={C.ground} stroke={hexA(authority ? C.stopped : C.leaf, 0.6)} />
                  <text x={viaPt.x} y={viaPt.y + 6} textAnchor="middle" fontFamily="DejaVu Sans Mono, monospace" fontSize={15} letterSpacing="1.5" fill={authority ? C.stopped : C.muted}>
                    {authority ? "SHARE REVOKED" : "HIDDEN SHARE"}
                  </text>
                </g>
              )}
            </g>
          );
        })}
      </svg>
      {NODES.map((n, i) => {
        const appear = prog(frame, start + i * 9, 30);
        const flipped = n.after && n.hitAt !== undefined && frame >= n.hitAt;
        const hit = n.hitAt !== undefined ? prog(frame, n.hitAt, 22) : 0;
        const s = flipped ? n.after! : n.before;
        const flash = n.hitAt !== undefined && frame >= n.hitAt ? Math.max(0, 1 - (frame - n.hitAt) / 18) : 0;
        return (
          <div key={n.id} style={{ position: "absolute", left: n.x, top: n.y, width: n.w, height: NH, opacity: appear, transform: `translateY(${(1 - appear) * 40}px) scale(${1 + flash * 0.04})` }}>
            <div style={{ position: "absolute", inset: 0, borderRadius: 22, background: hexA(C.appGround, 0.92), border: `1.5px solid ${hexA(s.color, 0.5 + flash * 0.5)}`, boxShadow: `0 30px 80px rgba(0,0,0,0.5), 0 0 ${30 + flash * 60}px ${hexA(s.color, 0.12 + flash * 0.35)}`, padding: "22px 24px", display: "flex", flexDirection: "column", gap: 12 }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <span style={{ fontFamily: FONT.mono, fontSize: 14, letterSpacing: "0.14em", color: C.muted }}>{n.kind}</span>
              </div>
              <div style={{ fontFamily: FONT.mono, fontSize: 17, fontWeight: 700, letterSpacing: "0.12em", color: s.color, opacity: flipped ? hit : 1 }}>● {s.label}</div>
              <div style={{ fontFamily: FONT.body, fontSize: 21, lineHeight: 1.3, color: C.ink, opacity: flipped && n.id === "claim" ? hit : 1 }}>{s.text}</div>
              <div style={{ marginTop: "auto", fontFamily: FONT.mono, fontSize: 13.5, letterSpacing: "0.06em", color: flipped ? s.color : C.dim }}>{s.meta}</div>
            </div>
            {n.fixture && !flipped && (
              <div style={{ position: "absolute", left: 0, top: -50 }}>
                <Tag color={C.fixture} size={15} solid>
                  Test fixture
                </Tag>
              </div>
            )}
          </div>
        );
      })}
      {/* correction pulse at the claim */}
      <PulseRing at={q.correct} x={byId("claim").x + NW / 2} y={byId("claim").y + NH / 2} max={620} color={C.leaf} dur={44} width={3} />
    </AbsoluteFill>
  );
}

// ---------------- the chapter ----------------
export function Ch03() {
  const frame = useCurrentFrame();
  const swap = prog(frame, q.swap, 22, EASE.inOut);
  const graphOn = envelope(frame, q.graph - 6, q.proof + 6, 16, 18);
  const settled = frame >= q.cancelled + 20;
  return (
    <Ground glow={0.55} gx={frame > q.graph ? 50 : 62} grid={frame > q.graph ? 0.4 : 0.22} tint={frame >= q.wave && frame < q.proof ? C.leaf : C.leafSoft}>
      <ChapterMark n="03" title="MEANING INTEGRITY ENGINE" at={q.diff} out={q.title - 10} />

      {/* ONE WORD. kadang -> selalu. DIFFERENT MEANING. */}
      <AbsoluteFill style={{ opacity: 1 - prog(frame, q.diff - 16, 14) }}>
        <Stage y={-250}>
          <Kinetic text="ONE WORD." at={q.oneWord} size={110} weight={700} color={C.muted} />
        </Stage>
        <AbsoluteFill style={{ alignItems: "center", justifyContent: "center" }}>
          <div style={{ position: "relative", height: 300, width: 1400, overflow: "hidden" }}>
            <div style={{ position: "absolute", inset: 0, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", transform: `translateY(${-swap * 110}%)`, opacity: prog(frame, q.oneWord + 14, 18) }}>
              <div style={{ fontFamily: FONT.display, fontWeight: 800, fontSize: 250, letterSpacing: "-0.04em", color: C.leaf, lineHeight: 1 }}>kadang</div>
              <div style={{ fontFamily: FONT.mono, fontSize: 26, letterSpacing: "0.3em", color: C.muted, marginTop: 8 }}>SOMETIMES</div>
            </div>
            <div style={{ position: "absolute", inset: 0, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", transform: `translateY(${(1 - swap) * 110}%)` }}>
              <div style={{ fontFamily: FONT.display, fontWeight: 800, fontSize: 250, letterSpacing: "-0.04em", color: C.fixture, lineHeight: 1 }}>selalu</div>
              <div style={{ fontFamily: FONT.mono, fontSize: 26, letterSpacing: "0.3em", color: C.muted, marginTop: 8 }}>ALWAYS</div>
            </div>
          </div>
        </AbsoluteFill>
        <Stage y={260}>
          <Kinetic text="DIFFERENT *MEANING.*" at={q.different} size={110} />
        </Stage>
      </AbsoluteFill>

      {/* The fixture, flagged by the deterministic Meaning Diff */}
      <AbsoluteFill style={{ left: 140, top: 250, width: 820, opacity: envelope(frame, q.diff, q.oneCorrection - 4, 14, 14) }}>
        <Tag color={C.fixture} solid>
          Test fixture · simulated AI error
        </Tag>
        <Gloss style={{ marginTop: 18, maxWidth: 700 }}>A scripted wrong interpretation, written for this demo. Not a live Meta output.</Gloss>
        <div style={{ marginTop: 64, opacity: prog(frame, q.flag, 16) }}>
          <Eyebrow color={C.muted}>Meaning Diff · Go · deterministic</Eyebrow>
          <div style={{ display: "flex", alignItems: "baseline", gap: 26, marginTop: 20, fontFamily: FONT.display, fontWeight: 800, fontSize: 92, letterSpacing: "-0.03em" }}>
            <span style={{ color: C.leaf }}>kadang</span>
            <span style={{ color: C.dim, fontSize: 70 }}>→</span>
            <span style={{ color: C.fixture }}>selalu</span>
          </div>
          <div style={{ display: "flex", gap: 14, marginTop: 26 }}>
            <Tag color={C.fixture} size={18}>frequency · critical</Tag>
            <Tag color={C.muted} size={18}>added word · review</Tag>
          </div>
          <Gloss style={{ marginTop: 22 }}>“Sela may have changed your meaning. Check before saving.”</Gloss>
        </div>
      </AbsoluteFill>
      <Float at={q.diff + 4} out={q.oneCorrection - 6} style={{ left: 1110, top: 170 }} from="right">
        <UIShot shot="mie-diff" width={660} sy={140} sh={1259} rings={[{ x: 55, y: 545, w: 1190, h: 260, at: q.diff + 30, color: C.muted }, { x: 60, y: 900, w: 1190, h: 232, at: q.flag, color: C.fixture }]} />
      </Float>

      <Stage>
        <Kinetic text="ONE *CORRECTION.*" at={q.oneCorrection} out={q.impact - 14} size={170} mode="scale" dur={18} />
      </Stage>

      {/* Correction in her own words, with a dry-run impact preview */}
      <AbsoluteFill style={{ left: 140, top: 270, width: 800, opacity: envelope(frame, q.impact, q.every - 4, 14, 14) }}>
        <Eyebrow>Nadia corrects it — in her own words</Eyebrow>
        <div style={{ marginTop: 26, fontFamily: FONT.body, fontWeight: 600, fontSize: 44, lineHeight: 1.22, color: C.ink }}>“Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman.”</div>
        <div style={{ marginTop: 52, opacity: prog(frame, q.impactHit, 16) }}>
          <Eyebrow color={C.stale}>Before saving · preview, nothing written</Eyebrow>
          <Gloss style={{ marginTop: 16, color: C.ink, fontSize: 25 }}>“This will mark 1 summary for review, stop 2 share links, and cancel 1 request to a professional. What others already read cannot be recalled.”</Gloss>
        </div>
      </AbsoluteFill>
      <Float at={q.impact + 4} out={q.every - 6} style={{ left: 1080, top: 200 }} from="bottom">
        <UIShot shot="mie-impact" width={700} sy={520} sh={1120} rings={[{ x: 100, y: 875, w: 1075, h: 370, at: q.impact + 26 }, { x: 100, y: 1296, w: 1075, h: 196, at: q.impactHit, color: C.stale }]} />
      </Float>

      <Stage gap={8}>
        <Kinetic text="EVERY DEPENDENCY" at={q.every} out={q.graph - 12} size={130} />
        <Kinetic text="*MATTERS.*" at={q.every + 10} out={q.graph - 10} size={130} />
      </Stage>

      {/* The graph */}
      <AbsoluteFill style={{ opacity: graphOn, transform: `scale(${mix(0.96, 1, prog(frame, q.graph, 60))})` }}>
        <Graph start={q.graph} />
        <div style={{ position: "absolute", left: 150, top: 150, opacity: prog(frame, q.graph + 20, 20) }}>
          <Eyebrow color={C.muted}>source → meaning → summary → share · care request</Eyebrow>
        </div>
        <div style={{ position: "absolute", left: 150, top: 900, right: 150, display: "flex", alignItems: "flex-end", justifyContent: "space-between", opacity: prog(frame, q.wave, 20) }}>
          <div style={{ display: "flex", gap: 56, fontFamily: FONT.display }}>
            {[
              { n: 1, l: "summary marked for review", c: C.stale, at: q.stale },
              { n: 2, l: "share links stopped", c: C.stopped, at: q.revoked },
              { n: 1, l: "care request cancelled", c: C.stopped, at: q.cancelled },
            ].map((k, i) => (
              <div key={i} style={{ display: "flex", alignItems: "baseline", gap: 14 }}>
                <span style={{ fontSize: 64, fontWeight: 800, color: k.c }}>
                  <Counter value={k.n} at={k.at} dur={10} />
                </span>
                <span style={{ fontFamily: FONT.mono, fontSize: 17, letterSpacing: "0.1em", color: C.muted, textTransform: "uppercase" }}>{k.l}</span>
              </div>
            ))}
          </div>
          <div style={{ textAlign: "right", opacity: settled ? prog(frame, q.cancelled + 20, 20) : 0 }}>
            <Gloss style={{ fontSize: 17 }}>one transaction · one invalidation event</Gloss>
            <Gloss style={{ fontSize: 17, color: C.dim }}>asserted in TestMeaningCorrectionPropagates</Gloss>
          </div>
        </div>
      </AbsoluteFill>

      {/* Proof: the real screens after the correction */}
      <AbsoluteFill style={{ opacity: envelope(frame, q.proof, q.title + 4, 14, 14) }}>
        <div style={{ position: "absolute", left: 140, top: 250 }}>
          <Eyebrow color={C.muted}>After the correction · the real screens</Eyebrow>
        </div>
        <Float at={q.proof} style={{ left: 140, top: 400 }} from="bottom" depth={1.2}>
          <UIShot shot="mie-result" width={520} sy={810} sh={250} label={false} />
          <Gloss style={{ marginTop: 22, fontSize: 19 }}>Memory · “Your correction was carried through.”</Gloss>
        </Float>
        <Float at={q.proof + 10} style={{ left: 720, top: 400 }} from="bottom" depth={0.8}>
          <UIShot shot="mie-stale-summaries" width={540} sx={110} sw={1200} sy={1530} sh={330} label={false} rings={[{ x: 140, y: 1690, w: 560, h: 70, at: q.proof + 30, color: C.stale }]} />
          <Gloss style={{ marginTop: 22, fontSize: 19 }}>Summary · “Needs review again”</Gloss>
        </Float>
        <Float at={q.proof + 20} style={{ left: 1320, top: 400 }} from="bottom" depth={1}>
          <UIShot shot="mie-stopped-shares" width={460} sx={620} sw={1320} sy={790} sh={500} label={false} rings={[{ x: 650, y: 930, w: 1250, h: 90, at: q.proof + 40, color: C.stopped }]} />
          <Gloss style={{ marginTop: 22, fontSize: 19, maxWidth: 460 }}>Link · “Stopped because the meaning was corrected”</Gloss>
        </Float>
      </AbsoluteFill>

      {/* Title */}
      <AbsoluteFill style={{ alignItems: "center", justifyContent: "center", gap: 30 }}>
        <Kinetic text="MEANING INTEGRITY ENGINE." at={q.title} size={104} mode="track" dur={26} />
        <div style={{ opacity: prog(frame, q.title + 26, 18) }}>
          <Eyebrow color={C.muted}>You decide what you meant. Sela carries it through.</Eyebrow>
        </div>
      </AbsoluteFill>

      <Sweep at={q.graph - 8} />
      <Sweep at={q.title - 6} dur={30} />
      <Dip inAt={0} len={12} outAt={1034} />
    </Ground>
  );
}
