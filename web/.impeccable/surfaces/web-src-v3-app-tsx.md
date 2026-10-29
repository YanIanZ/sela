---
version: 1
slug: "web-src-v3-app-tsx"
primary_target: "web/src/v3/App.tsx"
related_targets: ["web/src/v3/shell.tsx","web/src/v3/screens/Home.tsx","web/src/v3/styles.css"]
---

# Sela replacement interface — Taman Napas (wellness)

Mode: Operate (public portal is its own Read-mode brief, inheriting this world). Scope: whole web/src/v3 app. Owner request 2026-10-29 (/impeccable): replace the whole UI/UX system and CSS with a wellness world pinned by an owner-supplied reference image (sage-green mobile wellness app: big rounded tinted header sheet, bold rounded sans greeting, mood faces, tinted activity cards with circular arrow buttons, black pill bottom navigation with a sage active dot). Owner answers: whole v3; no mascot; illustrations as crisp SVG geometry only (no raster AI available). Supersedes Sampul Buku. Preserve owner authorization, consent, records, crisis help, Quick exit, ID/EN, display preferences (scheme, contrast, text size, spacing, motion) and every supported API; unsupported APIs stay honest blocked states. All copy kept.

## Direction contract

THESIS: Sela is a quiet garden you step into: every place opens on a soft tinted sheet with rounded lower corners that holds the greeting and the way in, and the work below sits on a clean near-white page. Refuses both the cream-serif editorial default and the clinical white-card dashboard.

OWN-WORLD: Sage ground sheets (#D4E5C3) with blush (#F4DDD3), mint (#CDE8DC) and butter (#F6E7B4) as place tints; near-white page #FBFBF7; ink #1C1B18 for text and the primary black pill buttons; deep leaf green #2F6A43 for links, focus and selection. Lexend (self-hosted) for headings, buttons and nav; Hanken Grotesk for body. Pill controls, 20–32px card radii, white circular icon buttons, 4-point sparkle stars and leaf geometry as the only ornament. Flat, one soft lift shadow for floating layers.

STORY: Arrive, read a warm greeting in large bold type, pick talk / note / check-in from the sheet, glance at mood faces, then read saved words in rounded lists; Help and Quick exit always top-right.

FIRST VIEWPORT: Top bar: Sela leaf mark + wordmark left, places as a black pill segmented nav (sage active segment) centre, Help (blush pill) + Quick exit (outline pill) + Menu right. Then the place sheet: greeting 3–4.5rem bold Lexend left, primary black pill "Mulai bercerita" with chevrons, two tinted activity cards (Tulis catatan, Cek perasaan) with circular arrow buttons on the right. Below: mood faces row linking into check-in, then continue + recent records left, more spaces right. Phones: header shares the sheet tint, floating black pill bottom nav.

FORM: Owner-pinned reference (wellness mobile UI image), translated to web Operate shell. The roll was skipped because a user-pinned direction beats the roll (`concept-seed --check-seed none` exits 0: "none means the roll was skipped") — seed: none (unavailable). Code-led.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Constraints carried forward
- No kicker above headings; no scores, streaks or progress. Mood faces always carry their text label; colour never alone.
- Help/danger/success/warning semantics identical across schemes; text 4.5:1, controls/focus 3:1 in every scheme; high contrast keeps its own values and drops tints.
- Focused writing/listening rooms keep a protected writing area; 320px and 130–150% text must reflow without horizontal scroll.
- Dev-only dashboard preview stays synthetic, labelled and API-free.
