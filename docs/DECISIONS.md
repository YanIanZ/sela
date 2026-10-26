# Sela decisions: origin of each

Updated: 2026-10-26 (owner brief v3). Authority: [SPEC](../SPEC.md). This record explains where each decision comes from; it does not override the spec. Full entries: [decision log](DECISION_LOG.md).

Three origins:

- **User-confirmed:** stated by the owner (owner brief v3 of 2026-10-26, or earlier owner direction). Changing it needs new owner direction.
- **PM decision:** taken by the delivery lead under the authority of owner brief v3 (D-025 onward). Recorded as decided; the owner may revise it.
- **Engineering default:** chosen during implementation with a recorded reason; may change with rationale and consistent spec and doc updates, never silently widening product scope.

No row implies implementation, verification, clinical validation, legal compliance, real partnerships or event acceptance. The status column uses the words of [SPEC §G](../SPEC.md#g-goal) (planned, in progress, implemented, verified locally, verified live, blocked externally) or "not applicable" for a decision that needs no build work; details follow a semicolon. "Uncommitted" means the code exists only in the working tree of `feat/sela-v3-pilot`.

## User-confirmed

| Decision | Source | Status / implication |
| --- | --- | --- |
| Product name Sela; tagline "A little space to feel heard" / "Ruang sejenak untuk didengar" | Owner | Not applicable (naming); the tagline describes listening, not an anonymity promise |
| Real-use health and wellbeing app for ages 15 to 50+; listening is the core; synthetic data only for tests, evaluation and demo; replaces the synthetic-only, adults-only, memory-only and Meta-only limits | Owner brief v3 §1 (D-025) | Planned; the v3 foundation is in progress (SPEC T14) |
| Indonesian and English across the whole product | Owner | Implemented and verified locally in v2 (C2); planned for every v3 surface |
| Backend Go; Python allowed for Modal, inference and research | Owner; brief §10 | Implemented (C1): the v2 Go runtime, the Python Modal app and research pipeline |
| AI roles: Meta open-weight on Modal is primary, Claude Sonnet 5.5 is the runtime verifier through an application API, DeepSeek-V4.1-Flash-1017 is the fallback; no fallback past a safety refusal; processing only with consent | Brief §8 | Planned (C17); routing details are a PM decision (D-029); live use blocked externally (B-1, B-8, B-9) |
| Meta open-weight model self-hosted on Modal | Owner (D-001) | Implemented (C4): adapter and serving app, tested offline; deployment blocked externally (B-1); generator model blocked externally (B-7, owner decision) |
| Claude Sonnet 5.5 as second-pass verifier, never a fallback generator | Owner, 26 October 2026 (C12) | Planned for v3 (V34, SPEC T18); the v2 standalone prototype is implemented (`prototypes/evidenceguard`, 6 tests with mocked HTTP) and not wired in; "sole live generator" superseded by D-029 |
| Separate consent for AI processing, storage and sharing; no real name needed; retention and deletion that work | Owner (C13) | Implemented and verified locally in v2 (V4, V21); v3 form planned: guest mode or username account (D-028), per-provider consent (D-029) |
| Guest mode states storage location and duration; no promise of absolute anonymity | Brief §10 | In progress (C16); guest start is in the uncommitted foundation code (backlog S-002) |
| Listening quality: start without a long profile, contextual replies, no forced solutions, correctable understanding, storage and memory choice, easy ending; optional intent picker never a gate | Brief §5 | Planned (V24, V25); details in D-034 |
| No exploitative engagement: no guilt notifications, no punishing streaks | Brief §5 | Planned (C19); pilot rule in D-036; v2 has no gamification |
| Accessibility target WCAG 2.2 AA; no single assumption about older users | Brief §6 | Planned (C25); v2 checks WCAG 2.1 AA tags only ([conflict matrix](CONFLICT_MATRIX.md#v3-owner-brief-v3-vs-the-v2-spec-and-code) row 40) |
| Ages 15–17: age-appropriate, consent and provider terms verified, no automatic guardian access, no default sharing with guardians, confidentiality limits explained, no hidden monitoring | Brief §6 | Planned (C18); production use blocked externally (B-10); gating in D-030 |
| Honest terms "Catatan pribadiku" and "Ringkasan untuk konsultasi"; never an official medical record | Brief §7 | Planned (C21); D-038 |
| Separate the user's statements, AI summaries, corrections, unconfirmed interpretations, uploads and verified professional notes, each with provenance, time, version and review status | Brief §7 | Planned (V26); D-039 |
| Timeline, correctable memory, storage control, readable and structured export, sharing preview, consent by content, recipient and purpose, expiry and revocation, access audit, isolation, deletion with derivatives, backup policy | Brief §7 | Planned (V25–V32, V38); export, deletion, isolation and the audit backend are in progress (uncommitted, SPEC T14) |
| Handoff statuses prepared, shared, opened, acknowledged, consultation in progress; a sent message is not care; no partnership, booking or verified-professional claims | Brief §7 | Planned (V28, C21); D-040, D-041 |
| No diagnosis, treatment or autonomous triage; safety routing not dependent on one classifier; no automatic contact; verified local support only | Brief §11 | Implemented and verified locally in v2 (deterministic crisis gate, no diagnosis or triage, verified adult help entries); v3 handling planned (C22, V36, V37) |
| Demo uses synthetic data, but its features really work; demo separated from real use | Brief §11 | In progress (C23, V40; D-045); environment guards in uncommitted `internal/config`; D-045 governs; ADR-0002 §12 and the config guard are narrower and are being aligned ([conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 52) |
| Do not hide Claude or DeepSeek, or claim their work as Meta's | Brief §11 | Planned (C17, V33); D-049 |
| Research: separate journal, corpus, evaluation and training data; five-year cutoff; screening is not full-text review; no fine-tuning or training on user notes by default | Brief §9; owner (C14) | Verified locally for the v2 corpus pipeline (D-047); v3 gap research planned (SPEC T19); appraisal blocked externally (B-4) |
| Do not remove Dengar.in features only to ease the refactor; keep contributor attribution | Brief §2 | Planned (C24, D-037); every feature has a recorded decision in the [feature matrix](DENGARIN_FEATURE_MATRIX.md); credits in [provenance](SELA_PROVENANCE.md) |
| Visual direction is the design lead's choice, judged on clarity, consistency, readability, task completion, privacy control, accessibility and implementation quality | Brief §4 | In progress (D-031); design documents written on the branch, uncommitted |
| Commit identity: the verified YanIanZ name and email, unsigned, no agent attribution | Owner (D-021) | Not applicable (process rule); applies to every commit |
| Legacy documentation archive | Owner (documentation refactor) | Implemented (SPEC T1); historical content and credits preserved; active requirements are in SPEC |

## PM decisions (v3, revisable by the owner)

| Decision | Entry | Status / implication |
| --- | --- | --- |
| PostgreSQL 16 (pgx v5) system of record; own migrator; memory store only in development, demo and test | [D-026](DECISION_LOG.md#d-026) | In progress; `internal/db` and migrations uncommitted ([ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md)) |
| Per-user envelope encryption with crypto-shredding | [D-027](DECISION_LOG.md#d-027) | In progress; `internal/vault` uncommitted |
| Username and password (argon2id), 10 recovery codes, server-side sessions, guest mode with a 7-day inactivity purge; no email | [D-028](DECISION_LOG.md#d-028) | In progress; `internal/account` uncommitted; email blocked externally (B-11) |
| AI routing, verifier and fallback rules | [D-029](DECISION_LOG.md#d-029) | Planned ([ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md)); live use blocked externally (B-1, B-8, B-9) |
| Ages 15–17 built but disabled in production until legal review; 15-year-olds blocked | [D-030](DECISION_LOG.md#d-030) | In progress for the guardian consent record backend (backlog S-211, uncommitted); the rest planned; production use blocked externally (B-10) |
| React 19 + TypeScript + Vite kept; IA and visual design redone | [D-031](DECISION_LOG.md#d-031) | In progress; design documents written in `docs/design/`, uncommitted; no v3 UI code yet |
| Content separation (public, personal, community, demo, research) | [D-032](DECISION_LOG.md#d-032) | Planned |
| Stacked branch `feat/sela-v3-pilot` on PR #6 | [D-033](DECISION_LOG.md#d-033) | Not applicable (branch choice); the branch holds the v3 work |
| Conversation room, intent picker, reply rules, storage choice per conversation | [D-034](DECISION_LOG.md#d-034) | Planned |
| Inline crisis handling without erasure; AI pause; second safeguards | [D-035](DECISION_LOG.md#d-035) | In progress; ruleset `-4` implemented (`8018372`), `-5` uncommitted (SPEC T22); the inline handling is planned |
| No streaks, points, badges or push notifications in the pilot | [D-036](DECISION_LOG.md#d-036) | Planned |
| Dengar.in features retired, paused or merged, each with a reason | [D-037](DECISION_LOG.md#d-037) | Planned; each feature's decision and backlog item are in the [feature matrix](DENGARIN_FEATURE_MATRIX.md) |
| Honest terms | [D-038](DECISION_LOG.md#d-038) | Planned |
| Provenance classes and review statuses | [D-039](DECISION_LOG.md#d-039) | Planned |
| Handoff statuses | [D-040](DECISION_LOG.md#d-040) | Planned |
| User-invited, unverified recipients; no directory or booking | [D-041](DECISION_LOG.md#d-041) | Planned |
| Uploads deferred | [D-042](DECISION_LOG.md#d-042) | Not applicable in the pilot; uploads come after the pilot (backlog S-182); the provenance class is reserved |
| Verified help only, with an enforced re-verify-by date | [D-043](DECISION_LOG.md#d-043) | Planned; re-verification blocked externally (B-5) |
| WHO cards for a non-commercial pilot only | [D-044](DECISION_LOG.md#d-044) | Planned for the non-commercial pilot; blocked externally (B-14) for commercial use |
| Environments and production refusals | [D-045](DECISION_LOG.md#d-045) | In progress; environment guards in uncommitted `internal/config` (ADR-0002 §12); D-045 governs; ADR-0002 §12 and the config guard are narrower and are being aligned ([conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 52) |
| Legacy Dengar.in deployments recorded as a risk | [D-046](DECISION_LOG.md#d-046) | Blocked externally (B-13); an owner and team action |
| Research pipeline kept; v3 evidence gaps; no fine-tuning | [D-047](DECISION_LOG.md#d-047) | Verified locally (the v2 corpus pipeline); v3 gap research planned; appraisal blocked externally (B-4) |
| Cross-border disclosure per provider; legal review before a pilot | [D-048](DECISION_LOG.md#d-048) | Planned; legal review blocked externally (B-10) |
| Meta-only demo path and provider labels for the hackathon | [D-049](DECISION_LOG.md#d-049) | Planned; blocked externally (B-12) for the organizers' answer |

## Engineering defaults (v3)

| Decision | Entry | Status / implication |
| --- | --- | --- |
| An unsaved conversation is held only in bounded server memory and purged at End, at session end, after 30 minutes idle or 2 hours, or on restart | [D-050](DECISION_LOG.md#d-050) | Planned (V25; backlog S-020) |
| Operator access to personal data or keys is audited, content-free; no tool shows an operator a user's content | [D-051](DECISION_LOG.md#d-051) | Planned (V30) |

## Engineering defaults (v2)

These describe the implemented v2 runtime. Where v3 replaced them, the v2 code still behaves this way until the replacement lands.

| Decision | Status / implication |
| --- | --- |
| Single static-web deployment: React/TypeScript + Vite, Go serves API and assets | Implemented (v2); kept in v3 (D-031) |
| In-memory bounded state per visitor (D-005, D-014) | Implemented (v2); superseded for real use by D-026 to D-028; kept for development, demo and test |
| Text-first, adult fixtures and fictional practice text (D-003) | Implemented (v2); superseded by D-025; kept as the v2 demo |
| Clinical handoff simulation: per-owner fictional recipient, link token + access code, separate sharing consent, scripted consultation and plan, demo labels (D-006, D-007, D-008) | Implemented (v2); the token + code model carries into v3; states superseded by D-040; fictional recipients are demo-only (D-041) |
| Help resources from v1 adult `verified_official` entries (D-018) | Implemented (v2); superseded by D-043 |
| Provenance values `live_meta`, `recorded_demo`, `manual`, `test_double` (D-019) | Implemented (v2); extended by D-029 and D-039; v3 also uses `test_double` as a verification value for labelled test-double verifiers (SPEC V33, planned; [conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 58) |
| Deterministic safety lexicon, no ML classifier (D-017) | Implemented (v2); kept; D-035 adds second safeguards |
| Story Ledger and Meaning Lock, deterministic Next Step, Opening Practice (D-010 to D-012) | Implemented (v2); Meaning Lock and Opening Practice carry into v3; Next Step matching is demo-only (D-041) |
| No fine-tuning; no retrieval at inference (D-016) | Not applicable (nothing to build); kept by D-047 |
| Cost guards on Modal and per-session limits (D-022) | Implemented (v2); extended with per-provider ceilings (ADR-0003 §11) |
