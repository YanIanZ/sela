/*
 * SELA film tokens. Derived from web/DESIGN.md (dark scheme): ground #121713,
 * surface #1A211B, pale text #EEF2E8, leaf #A9D5A0, leaf-soft #8FC17A, pale
 * sage #DCEBCB. The film goes one step deeper than the app ground for
 * contrast, and keeps SELA's green family as the only accent — no teal/cyan,
 * because SELA's identity is leaf green.
 */
export const C = {
  void: "#050706",
  ground: "#0A0E0B",
  appGround: "#121713",
  surface: "#1A211B",
  ink: "#EEF2E8",
  muted: "#A3AD9C",
  dim: "#5E6959",
  hair: "rgba(220,235,203,0.10)",
  leaf: "#A9D5A0",
  leafSoft: "#8FC17A",
  leafDeep: "#2F6A43",
  sage: "#DCEBCB",
  // Semantic labels (dark-scheme variants of SELA's warning/demo/test roles)
  fixture: "#F2C46B", // TEST FIXTURE — simulated AI error
  stale: "#F2C46B", // needs review
  stopped: "#F0937F", // revoked / cancelled
  sim: "#C2B2F2", // SIMULATED PROFESSIONAL (SELA "demo" purple)
} as const;

export const FONT = {
  display: "Lexend, 'Hanken Grotesk', system-ui, sans-serif",
  body: "'Hanken Grotesk', system-ui, sans-serif",
  mono: "'JetBrains Mono', 'DejaVu Sans Mono', ui-monospace, monospace",
} as const;

export const SAFE = { x: 140, y: 96 }; // title-safe margins on 1920×1080

export const FPS = 30;
export const W = 1920;
export const H = 1080;
