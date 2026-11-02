/*
 * SELA motion system: reusable, data-driven kinetic typography and
 * compositing primitives. Every component is a pure function of the current
 * frame (Remotion model), so any frame renders identically in isolation.
 */
import React, { type CSSProperties, type ReactNode } from "react";
import { AbsoluteFill, Easing, Img, interpolate, random, staticFile, useCurrentFrame } from "remotion";
import { C, FONT } from "../brand/tokens";
import footage from "../../public/footage/footage.json";

// ---------- easing presets ----------
export const EASE = {
  out: Easing.bezier(0.16, 1, 0.3, 1), // expo-out: snappy arrival, long settle
  inOut: Easing.bezier(0.65, 0, 0.35, 1),
  in: Easing.bezier(0.7, 0, 0.84, 0),
  soft: Easing.bezier(0.33, 1, 0.68, 1),
};

/** progress 0..1 of a span starting at `from` lasting `dur` frames. */
export function prog(frame: number, from: number, dur: number, easing = EASE.out) {
  return interpolate(frame, [from, from + dur], [0, 1], { extrapolateLeft: "clamp", extrapolateRight: "clamp", easing });
}
export const mix = (a: number, b: number, t: number) => a + (b - a) * t;

/** Visibility envelope: fade in at `a`, out at `b` (fade lengths fi/fo). */
export function envelope(frame: number, a: number, b: number, fi = 12, fo = 12) {
  return Math.min(prog(frame, a, fi, EASE.soft), 1 - prog(frame, b - fo, fo, EASE.inOut));
}

// ---------- background ----------
export function Ground({ glow = 0.5, gx = 50, gy = 50, grid = 0.35, tint = C.leafSoft, children }: { glow?: number; gx?: number; gy?: number; grid?: number; tint?: string; children?: ReactNode }) {
  const frame = useCurrentFrame();
  const drift = Math.sin(frame / 90) * 2;
  return (
    <AbsoluteFill style={{ background: C.ground, overflow: "hidden" }}>
      <AbsoluteFill
        style={{
          background: `radial-gradient(60% 55% at ${gx + drift}% ${gy}%, ${hexA(tint, 0.16 * glow)} 0%, ${hexA(tint, 0.05 * glow)} 38%, transparent 70%)`,
        }}
      />
      {grid > 0 && <Grid opacity={grid} />}
      <AbsoluteFill style={{ background: "radial-gradient(120% 90% at 50% 50%, transparent 55%, rgba(0,0,0,0.55) 100%)" }} />
      {children}
    </AbsoluteFill>
  );
}

export function Grid({ opacity = 0.35, size = 96 }: { opacity?: number; size?: number }) {
  const frame = useCurrentFrame();
  const off = (frame * 0.25) % size;
  return (
    <AbsoluteFill
      style={{
        opacity,
        backgroundImage: `linear-gradient(${C.hair} 1px, transparent 1px), linear-gradient(90deg, ${C.hair} 1px, transparent 1px)`,
        backgroundSize: `${size}px ${size}px`,
        backgroundPosition: `${off}px ${off * 0.5}px`,
        maskImage: "radial-gradient(70% 60% at 50% 50%, black 0%, transparent 100%)",
        WebkitMaskImage: "radial-gradient(70% 60% at 50% 50%, black 0%, transparent 100%)",
      }}
    />
  );
}

export function hexA(hex: string, a: number) {
  const n = parseInt(hex.slice(1), 16);
  return `rgba(${(n >> 16) & 255},${(n >> 8) & 255},${n & 255},${Math.max(0, Math.min(1, a))})`;
}

// ---------- kinetic typography ----------
export type KMode = "rise" | "track" | "scale" | "fragment" | "slide";

type KineticProps = {
  text: string;
  at: number; // local frame the line starts entering
  out?: number; // local frame the line starts leaving
  size?: number;
  weight?: number;
  color?: string;
  accent?: string; // colour for *starred* words
  tracking?: number; // em
  stagger?: number;
  mode?: KMode;
  exit?: "rise" | "fade" | "track" | "drop";
  align?: "center" | "left" | "right";
  font?: string;
  style?: CSSProperties;
  dur?: number;
};

/** One line of kinetic type. `*word*` renders in the accent colour. */
export function Kinetic({ text, at, out, size = 150, weight = 800, color = C.ink, accent = C.leaf, tracking = -0.035, stagger = 4, mode = "rise", exit = "rise", align = "center", font = FONT.display, style, dur = 22 }: KineticProps) {
  const frame = useCurrentFrame();
  if (frame < at - 1) return null;
  const words = text.split(" ");
  const outP = out === undefined ? 0 : prog(frame, out, 16, EASE.in);
  if (outP >= 1) return null;
  const justify = align === "center" ? "center" : align === "left" ? "flex-start" : "flex-end";
  let letterIndex = 0;
  return (
    <div style={{ display: "flex", justifyContent: justify, flexWrap: "wrap", gap: `0 ${size * 0.26}px`, fontFamily: font, fontWeight: weight, fontSize: size, lineHeight: 1.0, letterSpacing: `${tracking}em`, color, ...style }}>
      {words.map((raw, wi) => {
        // `*...*` toggles the accent colour, so "UNDERSTANDS*?*" accents only "?".
        const segs: Array<{ t: string; acc: boolean }> = [];
        raw.split("*").forEach((t, i) => t && segs.push({ t, acc: i % 2 === 1 }));
        const word = raw.replace(/\*/g, "");
        const p = prog(frame, at + wi * stagger, dur);
        const wc = color;
        const letters = segs.flatMap((sg) => [...sg.t].map((ch) => ({ ch, acc: sg.acc })));
        if (mode === "fragment") {
          return (
            <span key={wi} style={{ display: "inline-flex", color: wc }}>
              {letters.map(({ ch, acc }, ci) => {
                const k = letterIndex++;
                const lp = prog(frame, at + k * 1.2, 26);
                const dx = (random(`fx${text}${k}`) - 0.5) * 420 * (1 - lp);
                const dy = (random(`fy${text}${k}`) - 0.5) * 260 * (1 - lp);
                const r = (random(`fr${text}${k}`) - 0.5) * 50 * (1 - lp);
                return (
                  <span key={ci} style={{ display: "inline-block", color: acc ? accent : undefined, transform: `translate(${dx}px,${dy}px) rotate(${r}deg)`, opacity: lp * (1 - outP), filter: `blur(${(1 - lp) * 10 + outP * 8}px)` }}>
                    {ch}
                  </span>
                );
              })}
            </span>
          );
        }
        let transform = "";
        let opacity = 1;
        let filter = "none";
        let ls: string | undefined;
        if (mode === "rise") transform = `translateY(${(1 - p) * 108}%) rotate(${(1 - p) * 3}deg)`;
        if (mode === "slide") transform = `translateX(${(1 - p) * -40}%)`, (opacity = p);
        if (mode === "scale") (transform = `scale(${mix(1.35, 1, p)})`), (opacity = p), (filter = `blur(${(1 - p) * 14}px)`);
        if (mode === "track") (ls = `${mix(0.55, tracking, p)}em`), (opacity = p), (filter = `blur(${(1 - p) * 6}px)`);
        if (outP > 0) {
          if (exit === "rise") transform += ` translateY(${-outP * 108}%)`;
          if (exit === "drop") transform += ` translateY(${outP * 108}%)`;
          if (exit === "fade") (opacity *= 1 - outP), (filter = `blur(${outP * 10}px)`);
          if (exit === "track") (ls = `${mix(tracking, 0.5, outP)}em`), (opacity *= 1 - outP);
        }
        const masked = mode === "rise" || mode === "slide" || exit === "rise" || exit === "drop";
        return (
          <span key={wi} style={{ display: "inline-block", overflow: masked ? "hidden" : "visible", padding: `${size * 0.06}px 0 ${size * 0.14}px`, margin: `-${size * 0.06}px 0 -${size * 0.14}px` }}>
            <span style={{ display: "inline-block", transform, opacity, filter, letterSpacing: ls, color: wc, willChange: "transform" }}>
              {segs.map((sg, si) => (
                <span key={si} style={sg.acc ? { color: accent } : undefined}>
                  {sg.t}
                </span>
              ))}
            </span>
          </span>
        );
      })}
    </div>
  );
}

/** Centered stack of Kinetic lines. */
export function Stage({ children, y = 0, gap = 24, align = "center", x = 0 }: { children: ReactNode; y?: number; gap?: number; align?: "center" | "left"; x?: number }) {
  return (
    <AbsoluteFill style={{ alignItems: align === "center" ? "center" : "flex-start", justifyContent: "center", transform: `translate(${x}px, ${y}px)`, gap, padding: align === "left" ? "0 140px" : 0 }}>
      {children}
    </AbsoluteFill>
  );
}

// ---------- labels ----------
export function Tag({ children, color = C.leaf, solid = false, size = 22, style }: { children: ReactNode; color?: string; solid?: boolean; size?: number; style?: CSSProperties }) {
  return (
    <div
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 10,
        fontFamily: FONT.mono,
        fontSize: size,
        fontWeight: 600,
        letterSpacing: "0.12em",
        textTransform: "uppercase",
        color: solid ? C.void : color,
        background: solid ? color : hexA(color, 0.08),
        border: `1.5px solid ${hexA(color, solid ? 1 : 0.55)}`,
        borderRadius: 999,
        padding: `${size * 0.42}px ${size * 0.9}px`,
        whiteSpace: "nowrap",
        ...style,
      }}
    >
      <span style={{ width: size * 0.42, height: size * 0.42, borderRadius: 99, background: solid ? C.void : color }} />
      {children}
    </div>
  );
}

/** Small English gloss under Indonesian UI copy. */
export function Gloss({ children, style }: { children: ReactNode; style?: CSSProperties }) {
  return <div style={{ fontFamily: FONT.mono, fontSize: 22, letterSpacing: "0.02em", color: C.muted, lineHeight: 1.45, ...style }}>{children}</div>;
}

export function Eyebrow({ children, color = C.leaf, style }: { children: ReactNode; color?: string; style?: CSSProperties }) {
  return <div style={{ fontFamily: FONT.mono, fontSize: 22, letterSpacing: "0.24em", textTransform: "uppercase", color, ...style }}>{children}</div>;
}

// ---------- product footage ----------
type Shot = keyof typeof footage;
type Rect = { x: number; y: number; w: number; h: number };

/**
 * UIShot shows a real SELA capture (2× pixels) cropped to source rows
 * [sy, sy+sh], fitted to `width` CSS px. `scroll` pans the crop between two
 * source y values. `rings` draw highlight rectangles in source coordinates.
 */
export function UIShot({ shot, width, sx = 0, sw, sy = 0, sh, scrollTo, scrollFrom = 0, scrollDur = 60, rings = [], label = true, radius = 22, style, dim = 0 }: { shot: Shot; width: number; sx?: number; sw?: number; sy?: number; sh?: number; scrollTo?: number; scrollFrom?: number; scrollDur?: number; rings?: Array<Rect & { at: number; color?: string }>; label?: boolean; radius?: number; style?: CSSProperties; dim?: number }) {
  const frame = useCurrentFrame();
  const meta = footage[shot] as { file: string; width: number; height: number };
  const scale = width / (sw ?? meta.width);
  const cropH = sh ?? meta.height - sy;
  const y = scrollTo === undefined ? sy : mix(sy, scrollTo, prog(frame, scrollFrom, scrollDur, EASE.inOut));
  return (
    <div style={{ position: "relative", width, height: cropH * scale, ...style }}>
      <div
        style={{
          position: "absolute",
          inset: 0,
          overflow: "hidden",
          borderRadius: radius,
          background: C.appGround,
          boxShadow: `0 40px 120px rgba(0,0,0,0.55), 0 0 0 1.5px ${hexA(C.sage, 0.14)}, 0 0 80px ${hexA(C.leafSoft, 0.08)}`,
        }}
      >
        <Img src={staticFile(meta.file)} style={{ position: "absolute", left: -sx * scale, top: -y * scale, width: meta.width * scale, height: meta.height * scale }} />
        {rings.map((r, i) => {
          const p = prog(frame, r.at, 18);
          if (p <= 0) return null;
          const color = r.color ?? C.leaf;
          return (
            <div
              key={i}
              style={{
                position: "absolute",
                left: (r.x - sx) * scale - 10,
                top: (r.y - y) * scale - 10,
                width: r.w * scale + 20,
                height: r.h * scale + 20,
                borderRadius: 16,
                border: `3px solid ${hexA(color, p)}`,
                boxShadow: `0 0 ${40 * p}px ${hexA(color, 0.45 * p)}, inset 0 0 ${30 * p}px ${hexA(color, 0.12 * p)}`,
                transform: `scale(${mix(1.06, 1, p)})`,
              }}
            />
          );
        })}
        {dim > 0 && <div style={{ position: "absolute", inset: 0, background: `rgba(5,7,6,${dim})` }} />}
      </div>
      {label && (
        <div style={{ position: "absolute", left: 0, top: -44, fontFamily: FONT.mono, fontSize: 17, letterSpacing: "0.16em", color: C.dim, textTransform: "uppercase", whiteSpace: "nowrap" }}>
          SELA UI · source dfdffd0 · synthetic data
        </div>
      )}
    </div>
  );
}

/** A floating panel wrapper with 3D entry and gentle parallax drift. */
export function Float({ at, out, children, from = "bottom", depth = 1, style, tilt = 10 }: { at: number; out?: number; children: ReactNode; from?: "bottom" | "right" | "left" | "scale"; depth?: number; style?: CSSProperties; tilt?: number }) {
  const frame = useCurrentFrame();
  const p = prog(frame, at, 34);
  const o = out === undefined ? 0 : prog(frame, out, 20, EASE.in);
  if (p <= 0 || o >= 1) return null;
  const drift = Math.sin((frame - at) / 70) * 6 * depth;
  let t = "";
  if (from === "bottom") t = `translateY(${(1 - p) * 160}px) rotateX(${(1 - p) * tilt}deg)`;
  if (from === "right") t = `translateX(${(1 - p) * 220}px) rotateY(${-(1 - p) * tilt}deg)`;
  if (from === "left") t = `translateX(${-(1 - p) * 220}px) rotateY(${(1 - p) * tilt}deg)`;
  if (from === "scale") t = `scale(${mix(0.9, 1, p)})`;
  t += ` translateY(${drift}px) translateY(${-o * 60}px) scale(${1 - o * 0.04})`;
  return (
    <div style={{ position: "absolute", perspective: 1600, ...style }}>
      <div style={{ transform: t, opacity: p * (1 - o), filter: `blur(${(1 - p) * 8 + o * 10}px)`, transformOrigin: "50% 100%" }}>{children}</div>
    </div>
  );
}

// ---------- graphic primitives ----------
export function PulseRing({ at, x = 960, y = 540, max = 900, color = C.leaf, dur = 70, width = 2 }: { at: number; x?: number; y?: number; max?: number; color?: string; dur?: number; width?: number }) {
  const frame = useCurrentFrame();
  const p = prog(frame, at, dur, EASE.soft);
  if (frame < at || p >= 1) return null;
  const r = mix(8, max, p);
  return <div style={{ position: "absolute", left: x - r / 2, top: y - r / 2, width: r, height: r, borderRadius: "50%", border: `${width}px solid ${hexA(color, (1 - p) * 0.7)}`, boxShadow: `0 0 40px ${hexA(color, (1 - p) * 0.3)}` }} />;
}

export function Dot({ at, x = 960, y = 540, color = C.leaf, size = 14 }: { at: number; x?: number; y?: number; color?: string; size?: number }) {
  const frame = useCurrentFrame();
  const p = prog(frame, at, 30);
  const beat = 1 + 0.25 * Math.max(0, Math.sin(((frame - at) / 27) * Math.PI)) ** 8;
  return <div style={{ position: "absolute", left: x - size / 2, top: y - size / 2, width: size, height: size, borderRadius: "50%", background: color, opacity: p, transform: `scale(${p * beat})`, boxShadow: `0 0 ${30 * p}px ${hexA(color, 0.8)}` }} />;
}

/** Diagonal light sweep used as a transition accent. */
export function Sweep({ at, dur = 26, color = C.sage, angle = 18 }: { at: number; dur?: number; color?: string; angle?: number }) {
  const frame = useCurrentFrame();
  const p = prog(frame, at, dur, EASE.inOut);
  if (frame < at || p >= 1) return null;
  const x = mix(-60, 160, p);
  return <AbsoluteFill style={{ background: `linear-gradient(${90 + angle}deg, transparent ${x - 18}%, ${hexA(color, 0.10)} ${x - 4}%, ${hexA(color, 0.22)} ${x}%, ${hexA(color, 0.10)} ${x + 4}%, transparent ${x + 18}%)`, mixBlendMode: "screen" }} />;
}

/** Fade to/from near-black. */
export function Dip({ inAt, outAt, len = 14 }: { inAt?: number; outAt?: number; len?: number }) {
  const frame = useCurrentFrame();
  let a = 0;
  if (inAt !== undefined) a = Math.max(a, 1 - prog(frame, inAt, len, EASE.soft));
  if (outAt !== undefined) a = Math.max(a, prog(frame, outAt, len, EASE.inOut));
  if (a <= 0) return null;
  return <AbsoluteFill style={{ background: C.void, opacity: a }} />;
}

/** The SELA sprout mark (web/public/icon.svg, dark variant), drawn in. */
export function SproutMark({ size = 220, grow = 1, ring = 1 }: { size?: number; grow?: number; ring?: number }) {
  const l = Math.min(1, grow * 1.6);
  const r = Math.max(0, Math.min(1, grow * 1.6 - 0.45));
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" style={{ overflow: "visible" }}>
      <circle cx="16" cy="16" r={16 * ring} fill="#253425" opacity={ring} />
      <g transform={`translate(16 25) scale(${l}) translate(-16 -25)`}>
        <path fill={C.leafSoft} d="M16 25c0-6.2-3.6-10.4-9.4-11 .2 6.3 3.9 10.5 9.4 11Z" />
      </g>
      <g transform={`translate(16 25) scale(${r}) translate(-16 -25)`}>
        <path fill={C.leaf} d="M16 25c0-7.6 4.1-12.9 10.6-14-.3 7.9-4.4 13.1-10.6 14Z" />
      </g>
    </svg>
  );
}

/** Abstract drifting fragments of (fictional) written experience. */
export const FRAGMENTS = [
  "Kadang aku butuh waktu sendiri.",
  "Sepulang kuliah aku sering capek.",
  "Hari ini aku agak kewalahan.",
  "Aku belum tahu bagian mana yang paling melelahkan.",
  "Aku tetap senang bertemu teman-temanku.",
  "Bagaimana menjelaskan minggu yang melelahkan ini?",
  "Aku ingin menyiapkan beberapa pertanyaan.",
  "Aku belum tahu penyebabnya.",
  "Some days the words are harder than the answer.",
  "Aku ingin bisa menceritakannya pelan-pelan.",
];

export function Fragments({ at, count = 22, converge = 0, opacity = 0.5, speed = 1, seed = "f" }: { at: number; count?: number; converge?: number; opacity?: number; speed?: number; seed?: string }) {
  const frame = useCurrentFrame();
  const t = frame - at;
  if (t < 0) return null;
  return (
    <AbsoluteFill style={{ overflow: "hidden" }}>
      {Array.from({ length: count }).map((_, i) => {
        const r1 = random(`${seed}a${i}`), r2 = random(`${seed}b${i}`), r3 = random(`${seed}c${i}`);
        const depth = 0.35 + r3 * 0.9;
        const text = FRAGMENTS[i % FRAGMENTS.length];
        const dir = i % 2 === 0 ? 1 : -1;
        const baseX = r1 * 2400 - 240;
        let x = baseX + dir * t * (0.6 + depth * 1.4) * speed;
        x = ((x % 2600) + 2600) % 2600 - 340;
        let y = 70 + r2 * 940;
        const fade = prog(frame, at + i * 3, 40, EASE.soft);
        const cx = 960, cy = 540;
        x = mix(x, cx - 180, converge);
        y = mix(y, cy, converge);
        const size = 22 + depth * 30;
        return (
          <div key={i} style={{ position: "absolute", left: x, top: y, fontFamily: FONT.body, fontStyle: "italic", fontSize: size, color: C.sage, whiteSpace: "nowrap", opacity: fade * opacity * (0.25 + depth * 0.55) * (1 - converge * 0.85), filter: `blur(${(1.3 - depth) * 3 + converge * 4}px)`, transform: `scale(${1 - converge * 0.7})` }}>
            {text}
          </div>
        );
      })}
    </AbsoluteFill>
  );
}

export function Counter({ value, at, dur = 24 }: { value: number; at: number; dur?: number }) {
  const frame = useCurrentFrame();
  return <>{Math.round(value * prog(frame, at, dur))}</>;
}

/** Persistent chapter index, top-left, in the title-safe area. */
export function ChapterMark({ n, title, at = 0, out }: { n: string; title: string; at?: number; out?: number }) {
  const frame = useCurrentFrame();
  const p = prog(frame, at, 30);
  const o = out === undefined ? 0 : prog(frame, out, 16);
  return (
    <div style={{ position: "absolute", left: 140, top: 84, display: "flex", alignItems: "center", gap: 18, fontFamily: FONT.mono, fontSize: 19, letterSpacing: "0.22em", color: C.dim, opacity: p * (1 - o) }}>
      <span style={{ color: C.leaf }}>{n}</span>
      <span style={{ width: 60 * p, height: 1.5, background: C.dim }} />
      <span>{title}</span>
    </div>
  );
}
