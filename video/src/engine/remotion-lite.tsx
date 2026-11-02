/*
 * remotion-lite — a small, API-compatible subset of Remotion used to render
 * this film where the `remotion` package cannot be installed (offline build
 * sandbox). Scenes import from "remotion"; the bundler aliases that import
 * here. With `npm i` (remotion@4 in package.json) the same scenes run on real
 * Remotion unchanged: delete the alias in scripts/render.mjs or use the
 * Remotion CLI (`npm run studio`).
 *
 * Implemented: useCurrentFrame, useVideoConfig, interpolate, spring, Easing,
 * Sequence, AbsoluteFill, Img, staticFile, random, Composition, registerRoot.
 */
import React, { createContext, useContext, type CSSProperties, type ReactNode } from "react";

type VideoConfig = { fps: number; width: number; height: number; durationInFrames: number; id: string };

const FrameContext = createContext<number>(0);
const ConfigContext = createContext<VideoConfig>({ fps: 30, width: 1920, height: 1080, durationInFrames: 1, id: "root" });

export function useCurrentFrame(): number {
  return useContext(FrameContext);
}
export function useVideoConfig(): VideoConfig {
  return useContext(ConfigContext);
}

/** Root provider used by the renderer page. */
export function FrameRoot({ frame, config, children }: { frame: number; config: VideoConfig; children: ReactNode }) {
  return (
    <ConfigContext.Provider value={config}>
      <FrameContext.Provider value={frame}>{children}</FrameContext.Provider>
    </ConfigContext.Provider>
  );
}

// ---------- interpolate ----------
type Extrapolate = "clamp" | "extend" | "identity" | "wrap";
type InterpolateOptions = { easing?: (t: number) => number; extrapolateLeft?: Extrapolate; extrapolateRight?: Extrapolate };

export function interpolate(input: number, inputRange: readonly number[], outputRange: readonly number[], options: InterpolateOptions = {}): number {
  if (inputRange.length !== outputRange.length) throw new Error("interpolate: ranges differ in length");
  const easing = options.easing ?? ((t: number) => t);
  const left = options.extrapolateLeft ?? "extend";
  const right = options.extrapolateRight ?? "extend";
  let i = 1;
  while (i < inputRange.length - 1 && input > inputRange[i]) i++;
  const [a, b] = [inputRange[i - 1], inputRange[i]];
  const [c, d] = [outputRange[i - 1], outputRange[i]];
  let x = input;
  if (x < a) {
    if (left === "clamp") return c;
    if (left === "identity") return x;
  }
  if (x > b) {
    if (right === "clamp") return d;
    if (right === "identity") return x;
  }
  let t = b === a ? 0 : (x - a) / (b - a);
  const inside = t >= 0 && t <= 1;
  if (inside) t = easing(t);
  return c + (d - c) * t;
}

// ---------- Easing (same names/semantics as Remotion) ----------
function bezier(x1: number, y1: number, x2: number, y2: number) {
  const cx = 3 * x1, bx = 3 * (x2 - x1) - cx, ax = 1 - cx - bx;
  const cy = 3 * y1, by = 3 * (y2 - y1) - cy, ay = 1 - cy - by;
  const sx = (t: number) => ((ax * t + bx) * t + cx) * t;
  const sy = (t: number) => ((ay * t + by) * t + cy) * t;
  const dx = (t: number) => (3 * ax * t + 2 * bx) * t + cx;
  return (x: number) => {
    if (x <= 0) return 0;
    if (x >= 1) return 1;
    let t = x;
    for (let k = 0; k < 8; k++) {
      const e = sx(t) - x;
      const de = dx(t);
      if (Math.abs(e) < 1e-6 || de === 0) break;
      t -= e / de;
    }
    t = Math.min(1, Math.max(0, t));
    return sy(t);
  };
}
export const Easing = {
  linear: (t: number) => t,
  quad: (t: number) => t * t,
  cubic: (t: number) => t * t * t,
  poly: (n: number) => (t: number) => Math.pow(t, n),
  sin: (t: number) => 1 - Math.cos((t * Math.PI) / 2),
  circle: (t: number) => 1 - Math.sqrt(1 - t * t),
  exp: (t: number) => (t === 0 ? 0 : Math.pow(2, 10 * (t - 1))),
  back: (s = 1.70158) => (t: number) => t * t * ((s + 1) * t - s),
  bezier,
  in: (e: (t: number) => number) => e,
  out: (e: (t: number) => number) => (t: number) => 1 - e(1 - t),
  inOut: (e: (t: number) => number) => (t: number) => (t < 0.5 ? e(t * 2) / 2 : 1 - e((1 - t) * 2) / 2),
};

// ---------- spring (damped harmonic oscillator, Remotion's defaults) ----------
type SpringConfig = { damping: number; mass: number; stiffness: number; overshootClamping: boolean };
export function spring({ frame, fps, config = {}, from = 0, to = 1, delay = 0, durationInFrames }: { frame: number; fps: number; config?: Partial<SpringConfig>; from?: number; to?: number; delay?: number; durationInFrames?: number }): number {
  const { damping = 10, mass = 1, stiffness = 100, overshootClamping = false } = config;
  let f = frame - delay;
  if (f <= 0) return from;
  if (durationInFrames) f = f * (naturalDuration({ damping, mass, stiffness }, fps) / durationInFrames);
  const t = f / fps;
  const w0 = Math.sqrt(stiffness / mass);
  const zeta = damping / (2 * Math.sqrt(stiffness * mass));
  let x: number; // displacement from target, starting at 1
  if (zeta < 1) {
    const wd = w0 * Math.sqrt(1 - zeta * zeta);
    x = Math.exp(-zeta * w0 * t) * (Math.cos(wd * t) + ((zeta * w0) / wd) * Math.sin(wd * t));
  } else {
    x = Math.exp(-w0 * t) * (1 + w0 * t);
  }
  let p = 1 - x;
  if (overshootClamping) p = Math.min(p, 1);
  return from + (to - from) * p;
}
function naturalDuration(c: { damping: number; mass: number; stiffness: number }, fps: number) {
  for (let f = 1; f < fps * 20; f++) {
    const v = spring({ frame: f, fps, config: c });
    if (Math.abs(1 - v) < 0.005 && Math.abs(1 - spring({ frame: f + 1, fps, config: c })) < 0.005) return f;
  }
  return fps * 2;
}

// ---------- random (deterministic, seeded) ----------
export function random(seed: string | number | null): number {
  let h = 2166136261;
  const s = String(seed);
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  h += h << 13; h ^= h >>> 7; h += h << 3; h ^= h >>> 17; h += h << 5;
  return ((h >>> 0) % 1_000_000) / 1_000_000;
}

// ---------- layout primitives ----------
export function AbsoluteFill({ style, children, className }: { style?: CSSProperties; children?: ReactNode; className?: string }) {
  return (
    <div className={className} style={{ position: "absolute", inset: 0, display: "flex", flexDirection: "column", ...style }}>
      {children}
    </div>
  );
}

export function Sequence({ from = 0, durationInFrames = Infinity, children, layout = "absolute-fill", name }: { from?: number; durationInFrames?: number; children: ReactNode; layout?: "absolute-fill" | "none"; name?: string }) {
  const frame = useCurrentFrame();
  const local = frame - from;
  if (local < 0 || local >= durationInFrames) return null;
  const inner = <FrameContext.Provider value={local}>{children}</FrameContext.Provider>;
  return layout === "none" ? inner : <AbsoluteFill className={name ? `seq-${name}` : undefined}>{inner}</AbsoluteFill>;
}

export function Img(props: React.ImgHTMLAttributes<HTMLImageElement>) {
  return <img {...props} decoding="sync" />;
}

let base = "/";
export function setStaticBase(b: string) {
  base = b.endsWith("/") ? b : b + "/";
}
export function staticFile(path: string): string {
  return base + path.replace(/^\//, "");
}

// ---------- compositions ----------
export type CompositionProps = { id: string; component: React.ComponentType<any>; durationInFrames: number; fps: number; width: number; height: number; defaultProps?: Record<string, unknown> };
const registry: CompositionProps[] = [];
export function Composition(props: CompositionProps) {
  if (!registry.find((c) => c.id === props.id)) registry.push(props);
  return null;
}
export function getCompositions() {
  return registry;
}
let rootComponent: React.ComponentType | null = null;
export function registerRoot(c: React.ComponentType) {
  rootComponent = c;
}
export function getRoot() {
  return rootComponent;
}
