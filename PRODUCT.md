# Product

<!-- impeccable:product-schema 1 -->

> **Owner direction update (29 October 2026; not yet implemented):** Sela will be anonymous-first and **consumer login-free**, text-only, without voice/TTS/ASR, avatar or required GPS. Planned differentiator: [Meaning Integrity Engine](docs/vision/MEANING_INTEGRITY_ENGINE.md). Meta remains primary, Claude Sonnet 5.5 verifier, DeepSeek opt-in availability fallback; manual professional/facility discovery joins the real product, while external hospital collaboration is **synthetic-only until real partnerships**. This supersedes conflicting aspirational statements below **for planning**, not claims about the working code. The full scope and migration requirements are [here](docs/vision/README.md) and [ADR-0004](docs/adr/ADR-0004-anonymous-first-meaning-integrity-care-network.md).


## Platform

web

## Users

People aged 15 to 50 and over, in Indonesia and other Indonesian-speaking communities, who want a private place to express what they feel, look back at what they said, and bring chosen context when they seek professional help. English is a full second language. The design works from situations, not age personas: a heavy evening when someone needs to talk now; a shared phone or an unsafe home; low vision or tired eyes; a screen-reader user; a cheap Android phone on slow 3G; preparing for a Puskesmas visit; reflecting after a consultation; being 15–17; code-switching between Indonesian and English.

## Product Purpose

Sela is a calm, honest place to talk, write and check in — not therapy. The core experience: "Aku memiliki ruang untuk bercerita, memahami kembali pengalamanku, dan membawa konteks yang kupilih ketika mencari bantuan" (I have a space to tell my story, understand my experience again, and bring the context I choose when I look for help). Listening is the core function. Preparing for a professional is offered, never required.

Sela continues Dengar.in as a real-use health and wellbeing app. It originated at a hackathon and is moving toward a controlled pilot and public release.

## Positioning

"A little space to feel heard" / "Ruang sejenak untuk didengar." In Indonesian, *sela* is the gap between things — *di sela-sela kesibukan* means "in the gaps of a busy day." The product is that gap: a short, private pause in which a person is listened to. Sela is not a diagnosis, treatment, triage, telemedicine or therapy service.

## Operating Context

A person starts talking without a long profile, gets replies that fit the context, can correct how Sela understood them, and controls what is saved. The four places: Beranda (home), Cerita (talk/listen), Catatanku (my notes), Dukungan (support). "Butuh bantuan sekarang" (need help now) and Quick exit are on every screen.

Key workflows: listening conversations with optional save; personal notes and check-ins; a transparent, correctable memory ("Yang Sela ingat"); a timeline across sessions; consultation summaries built from chosen notes and shared under explicit consent; follow-up notes after a consultation; data export and deletion.

Environments: the user's own phone browser (often a cheap Android on slow 3G, prepaid data); sometimes a shared device. The interface must start fast, leave no trace when exited, and never lose a draft.

## Capabilities and Constraints

- Backend: Go. Frontend: React 19, TypeScript, Vite, plain CSS with custom properties. One Go process serves API and assets. PostgreSQL 16 for persistent storage.
- AI: Meta open-weight model on Modal is the primary generator; Claude is the verifier (never a generator); DeepSeek is an optional, off-by-default availability fallback. Provider keys stay server-side. Every AI output records provider provenance and is labelled in the UI.
- Bilingual: Indonesian and English across the complete journey, including error states. Code-switched input is tested.
- Privacy: anonymous or pseudonymous use (guest mode or username account, no real name required). Per-user envelope encryption with crypto-shredding. Separate consents for AI processing (per provider), storage and sharing. Retention and deletion that actually work. No story content, prompts/outputs, credentials or tokens in operational logs.
- Minors (15–17): self-declared age band; age-appropriate copy; guardian consent gives no account, access, notification or export; memory and pattern features opt-in only; no DeepSeek; no hidden monitoring.
- Content Security Policy: `default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'`. No font CDNs, no inline scripts.
- Sela is not a diagnosis, treatment, triage, telemedicine or therapy service. Synthetic data is for tests, evaluation and the demo only. No clinical-efficacy, pilot-readiness or legal-compliance claim without matching evidence.

## Brand Commitments

- Name: **Sela**. Tagline: "A little space to feel heard" / "Ruang sejenak untuk didengar."
- Visual world: **Taman Napas** ("breathing garden") — a quiet garden metaphor with near-white pages, tinted place sheets, ink pill buttons, drawn SVG ornaments (sprout, sparkles, mood faces). Documented in `web/DESIGN.md`.
- Typography: Lexend (display/body) and Hanken Grotesk (supporting). Self-hosted woff2.
- Five place tints: sage (home), mint (talk), butter (notes), blush (support), quiet (settings). Tints name the place, never actions.
- Four colour schemes: light, dark, contrast-light, contrast-dark.
- Voice: calm, direct, honest. Sentence case throughout. Indonesian is the primary language; English is a complete second language, not a translation artifact.

## Evidence on Hand

- 100-publication research register with a rolling five-year cutoff, title/abstract-screened (not appraised); the evidence map covers the full owner brief §9 topic list. Repository: `scripts/research/corpus_pipeline.py`.
- Verified help resources from official sources, each with verification date and re-verify-by date. Teen entries need the same verification.
- No real user data yet. No clinical-efficacy evidence. No real-world deployment evidence.
- SPEC.md is the single source of truth; decisions D-026 to D-056 in `docs/DECISION_LOG.md`; architecture decisions in `docs/adr/`.

## Operator security and Redis — owner extension, 29 October 2026

Anonymity is an **end-user interface** decision, not permission for unverified operators. Staff must authenticate separately with MFA, least-privilege roles and an immutable audit. Their console manages vetted public care directories, source freshness, research editorial quality and AI budgets—**no anonymous journal viewer**. [Admin Console](docs/vision/ADMIN_CONSOLE.md) and [Planned Care Operations](docs/vision/ADMIN_CARE_OPERATIONS.md).

Go may use Redis for public revisioned directory/research cache, distributed admission and non-sensitive spend counters, with PostgreSQL authoritative for grants, content and durable budget. Never cache raw user stories/prompts or safety decisions; Redis unavailability must not lead to unbounded AI spend. [Redis engineering](docs/vision/REDIS_AI_COST_ENGINEERING.md).

These are **target changes, not deployed features**.

## Product Principles

1. **Listening first.** A person talks and is heard before anything else is offered. Seeking a professional is never required, pushed or nudged.
2. **The user's words stay theirs.** Original text is never overwritten. AI items are unconfirmed until the user confirms. No hidden profile exists outside the transparent memory view.
3. **Safety without paternalism.** The crisis gate never erases what someone wrote; inline help appears without navigating away; support never waits for a model. Quick exit leaves no trace.
4. **Honest about what it is.** Sela labels every AI output by provider, never presents a test double as live, never claims to be therapy, and says "I can't" when it can't.
5. **Privacy as architecture.** Encryption, separate consents, crypto-shredding and content-free logs are structural decisions, not settings. Guest mode and anonymous use are first-class.

## Accessibility & Inclusion

WCAG 2.2 AA (user-confirmed). 55 criteria documented as acceptance criteria in `docs/design/ACCESSIBILITY_CRITERIA.md`. Key commitments: rem-only type scale with text size up to 150% on top of browser zoom; 44px touch targets; icons always labelled; every label that carries meaning at 16px minimum; screen reader: transcript as silent `role="log"` with single announcements; no gesture-only actions, no timed steps; four colour schemes including two high-contrast variants; forced-colors and prefers-reduced-motion support.
