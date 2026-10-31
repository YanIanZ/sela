# Sela decisions: origin of each

Updated: 2026-10-26 (owner brief v3). Authority: [SPEC](../../SPEC.md). This record explains where each decision comes from; it does not override the spec. Full entries: [decision log](DECISION_LOG.md).

Three origins:

- **User-confirmed:** stated by the owner (owner brief v3 of 2026-10-26, or earlier owner direction). Changing it needs new owner direction.
- **PM decision:** taken by the delivery lead under the authority of owner brief v3 (D-027 to D-050 and D-054 onward; D-026 records the owner brief itself; D-051 to D-053 are engineering). Recorded as decided; the owner may revise it.
- **Engineering default:** chosen during implementation with a recorded reason; may change with rationale and consistent spec and doc updates, never silently widening product scope.

A decision does not imply implementation, verification, clinical validation, legal compliance, real partnerships or event acceptance; only the status column says what is built and verified. The status column uses the words of [SPEC §G](../../SPEC.md#g-goal) (planned, in progress, implemented, verified locally, verified live, blocked externally) or "not applicable" for a decision that needs no build work; details follow a semicolon. "Verified locally" for v3 means the foundation commit `0ff3341` with its recorded runs ([implementation status §1](../status/IMPLEMENTATION_STATUS.md#1-where-v3-stands)); its CI run passed (`.github/workflows/v2.yml` run 40405475962 on draft PR #9; [SPEC §G foundation status note](../../SPEC.md#foundation-status-note-2026-10-26)); "uncommitted" means the work exists only in a working tree, and "separate branch" means a branch that is not merged into `feat/sela-v3-pilot` yet.

## User-confirmed

| Decision | Source | Status / implication |
| --- | --- | --- |
| Product name Sela; tagline "A little space to feel heard" / "Ruang sejenak untuk didengar" | Owner | Not applicable (naming); the tagline describes listening, not an anonymity promise |
| Real-use health and wellbeing app for ages 15 to 50+; listening is the core; synthetic data only for tests, evaluation and demo; replaces the synthetic-only, adults-only, memory-only and Meta-only limits | Owner brief v3 §1 (D-026) | Planned; the v3 backend foundation is verified locally (SPEC T14, `0ff3341`) |
| Indonesian and English across the whole product | Owner | Implemented and verified locally in v2 (C2); planned for every v3 surface |
| Backend Go; Python allowed for Modal, inference and research | Owner; brief §10 | Implemented (C1): the v2 Go runtime, the Python Modal app and research pipeline; the v3 foundation is Go too (Go 1.26, D-054) |
| AI roles: Meta open-weight on Modal is primary, Claude Sonnet 5.5 is the runtime verifier through an application API, DeepSeek-V4.1-Flash-1017 is the fallback; no fallback past a safety refusal; processing only with consent | Brief §8 | Planned (C18); routing details are a PM decision (D-030); live use blocked externally (B-1, B-8, B-9) |
| Meta open-weight model self-hosted on Modal | Owner (D-001) | Implemented (C4): adapter and serving app, tested offline; deployment blocked externally (B-1); generator model blocked externally (B-7, owner decision) |
| Claude Sonnet 5.5 as second-pass verifier, never a fallback generator | Owner, 26 October 2026 (C12) | Planned for v3 (V36, SPEC T18); the v2 standalone prototype is implemented (`prototypes/evidenceguard`, 6 tests with mocked HTTP) and not wired in; "sole live generator" superseded by D-030 |
| DeepSeek-V4.1-Flash-1017 (`deepseek-flash`) as an opt-in availability fallback only; Meta remains primary; its own explicit consent, OFF by default in hackathon mode, fail-closed, truthful provider attribution | Owner, 26 October 2026 (C13; [contract](../architecture/DEEPSEEK_FALLBACK.md)) | Planned for v3 (C18, V19–V21, V35, SPEC T18; the v3 design follows the owner contract, and ADR-0003 §17 lists where it differs); the owner's standalone prototype is implemented (`prototypes/generationfallback`, 9 tests with mocked providers) and not wired in; live use blocked externally (B-9) |
| Separate consent for AI processing, storage and sharing; no real name needed; retention and deletion that work | Owner (C14) | Implemented and verified locally in v2 (V4, V23); in v3 the guest mode, username accounts, separate versioned consents and deletion are verified locally in the backend foundation (D-029, `0ff3341`); per-provider AI consent is planned (D-030) |
| Guest mode states storage location and duration; no promise of absolute anonymity | Brief §10 | In progress (C17): guest start and the guest notice text are verified locally in the backend foundation (`0ff3341`, F-30, F-31, F-33); the notice screen is not done (backlog S-002) |
| Listening quality: start without a long profile, contextual replies, no forced solutions, correctable understanding, storage and memory choice, easy ending; optional intent picker never a gate | Brief §5 | Planned (V26, V27); details in D-035 |
| No exploitative engagement: no guilt notifications, no punishing streaks | Brief §5 | Planned (C20); pilot rule in D-037; v2 has no gamification |
| Accessibility target WCAG 2.2 AA; no single assumption about older users | Brief §6 | Planned (C26); v2 checks WCAG 2.1 AA tags only ([conflict matrix](../status/CONFLICT_MATRIX.md#v3-owner-brief-v3-vs-the-v2-spec-and-code) row 40) |
| Ages 15–17: age-appropriate, consent and provider terms verified, no automatic guardian access, no default sharing with guardians, confidentiality limits explained, no hidden monitoring | Brief §6 | Planned (C19); production use blocked externally (B-10); gating in D-031 |
| Honest terms "Catatan pribadiku" and "Ringkasan untuk konsultasi"; never an official medical record | Brief §7 | Planned (C22); D-039 |
| Separate the user's statements, AI summaries, corrections, unconfirmed interpretations, uploads and verified professional notes, each with provenance, time, version and review status | Brief §7 | Planned (V28); D-040 |
| Timeline, correctable memory, storage control, readable and structured export, sharing preview, consent by content, recipient and purpose, expiry and revocation, access audit, isolation, deletion with derivatives, backup policy | Brief §7 | Planned (V27–V34, V40); the foundation's export, deletion, isolation, audit, backup and restore backend is verified locally (SPEC T14, `0ff3341`); a restore window remains, accepted for a controlled pilot (D-055) |
| Handoff statuses prepared, shared, opened, acknowledged, consultation in progress; a sent message is not care; no partnership, booking or verified-professional claims | Brief §7 | Planned (V30, C22); D-041, D-042 |
| No diagnosis, treatment or autonomous triage; safety routing not dependent on one classifier; no automatic contact; verified local support only | Brief §11 | Implemented and verified locally in v2 (deterministic crisis gate, no diagnosis or triage, verified adult help entries); v3 handling planned (C23, V38, V39) |
| Demo uses synthetic data, but its features really work; demo separated from real use | Brief §11 | In progress (C24, V42; D-046); the environment guards in `internal/config` are verified locally (`0ff3341`); D-046 governs; ADR-0002 §12 and the config guard are narrower and are being aligned ([conflict matrix](../status/CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 52) |
| Do not hide Claude or DeepSeek, or claim their work as Meta's | Brief §11 | Planned (C18, V35); D-050 |
| Research: separate journal, corpus, evaluation and training data; five-year cutoff; screening is not full-text review; no fine-tuning or training on user notes by default | Brief §9; owner (C15) | Verified locally for the v2 corpus pipeline (D-048); v3 gap research in progress on branch `feat/sela-v3-research`, not merged; nothing verified (S-243, S-244; SPEC T19); appraisal blocked externally (B-4) |
| Do not remove Dengar.in features only to ease the refactor; keep contributor attribution | Brief §2 | Planned (C25, D-038); every feature has a recorded decision in the [feature matrix](../product/DENGARIN_FEATURE_MATRIX.md); credits in [provenance](../product/SELA_PROVENANCE.md) |
| Visual direction is the design lead's choice, judged on clarity, consistency, readability, task completion, privacy control, accessibility and implementation quality | Brief §4 | In progress (D-032); design documents committed (`4cadbfd`), PM review not yet recorded; the app shell is in progress, uncommitted |
| Commit identity: the verified YanIanZ name and email, unsigned, no agent attribution | Owner (D-021) | Not applicable (process rule); applies to every commit |
| Legacy documentation archive | Owner (documentation refactor) | Implemented (SPEC T1); historical content and credits preserved; active requirements are in SPEC |

## PM decisions (v3, revisable by the owner)

| Decision | Entry | Status / implication |
| --- | --- | --- |
| PostgreSQL 16 (pgx v5) system of record; own migrator; memory store only in development, demo and test | [D-027](DECISION_LOG.md#d-027) | Verified locally: `internal/db`, migrations and the migration round trip (`0ff3341`; [ADR-0002](../adr/ADR-0002-persistence-accounts-and-encryption.md) F-01 to F-09); the hosted database is blocked externally (B-3, B-15) |
| Per-user envelope encryption with crypto-shredding | [D-028](DECISION_LOG.md#d-028) | Verified locally: `internal/vault` and crypto-shredding (`0ff3341`, F-12 to F-15); the secret manager is blocked externally (B-3) |
| Username and password (argon2id), 10 recovery codes, server-side sessions, guest mode with a 7-day inactivity purge; no email | [D-029](DECISION_LOG.md#d-029) | Backend verified locally (`internal/account`, `0ff3341`, F-16 to F-33); the screens are in progress; email blocked externally (B-11) |
| AI routing, verifier and fallback rules | [D-030](DECISION_LOG.md#d-030) | In progress on a separate branch, not merged ([ADR-0003](../adr/ADR-0003-ai-routing-verifier-fallback.md)); live use blocked externally (B-1, B-8, B-9) |
| Ages 15–17 built but disabled in production until legal review; 15-year-olds blocked | [D-031](DECISION_LOG.md#d-031) | The age band and the guardian consent record backend are verified locally (backlog S-005, S-211; `0ff3341`); the rest planned; production use blocked externally (B-10) |
| React 19 + TypeScript + Vite kept; IA and visual design redone | [D-032](DECISION_LOG.md#d-032) | In progress; design documents committed in `docs/design/` (`4cadbfd`); the app shell is in progress, uncommitted |
| Content separation (public, personal, community, demo, research) | [D-033](DECISION_LOG.md#d-033) | Planned |
| Stacked branch `feat/sela-v3-pilot` on PR #6 | [D-034](DECISION_LOG.md#d-034) | Not applicable (branch choice); the branch holds the v3 work. PR #6 is now merged into main (`7fc6678`); the branch is open as draft PR #9 against main, and ruleset `-5` takes a new PR against main, draft PR #8, a WIP snapshot (D-034 status) |
| Conversation room, intent picker, reply rules, storage choice per conversation | [D-035](DECISION_LOG.md#d-035) | Planned |
| Inline crisis handling without erasure; AI pause; second safeguards | [D-036](DECISION_LOG.md#d-036) | In progress; ruleset `-4` implemented (`8018372`), `-5` a WIP snapshot committed on `fix/safety-ruleset-5` (`f619677`, draft PR #8 to main), re-verification pending (SPEC T22); the inline handling is planned |
| No streaks, points, badges or push notifications in the pilot | [D-037](DECISION_LOG.md#d-037) | Planned |
| Dengar.in features retired, paused or merged, each with a reason | [D-038](DECISION_LOG.md#d-038) | Planned; each feature's decision and backlog item are in the [feature matrix](../product/DENGARIN_FEATURE_MATRIX.md) |
| Honest terms | [D-039](DECISION_LOG.md#d-039) | Planned |
| Provenance classes and review statuses | [D-040](DECISION_LOG.md#d-040) | Planned |
| Handoff statuses | [D-041](DECISION_LOG.md#d-041) | Planned |
| User-invited, unverified recipients; no directory or booking | [D-042](DECISION_LOG.md#d-042) | Planned |
| Uploads deferred | [D-043](DECISION_LOG.md#d-043) | Not applicable in the pilot; uploads come after the pilot (backlog S-182); the provenance class is reserved |
| Verified help only, with an enforced re-verify-by date | [D-044](DECISION_LOG.md#d-044) | Planned; re-verification blocked externally (B-5) |
| WHO cards for a non-commercial pilot only | [D-045](DECISION_LOG.md#d-045) | Planned for the non-commercial pilot; blocked externally (B-14) for commercial use |
| Environments and production refusals | [D-046](DECISION_LOG.md#d-046) | In progress; the environment guards in `internal/config` are verified locally (`0ff3341`, ADR-0002 §12, F-10, F-11); D-046 governs; ADR-0002 §12 and the config guard are narrower and are being aligned ([conflict matrix](../status/CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 52) |
| Legacy Dengar.in deployments recorded as a risk | [D-047](DECISION_LOG.md#d-047) | Blocked externally (B-13); an owner and team action |
| Research pipeline kept; v3 evidence gaps; no fine-tuning | [D-048](DECISION_LOG.md#d-048) | Verified locally (the v2 corpus pipeline); v3 gap research in progress on branch `feat/sela-v3-research`, not merged; nothing verified (S-243, S-244; SPEC T19); appraisal blocked externally (B-4) |
| Cross-border disclosure per provider; legal review before a pilot | [D-049](DECISION_LOG.md#d-049) | Planned; legal review blocked externally (B-10) |
| Meta-only demo path and provider labels for the hackathon | [D-050](DECISION_LOG.md#d-050) | In progress (the `meta_only` profile, on a separate branch, not merged); blocked externally (B-12) for the organizers' answer |
| Go toolchain 1.26 (`go 1.26.0`, `toolchain go1.26.10`; image `golang:1.26.10-alpine`): pgx v5.10.0 needs Go 1.25+, x/crypto v0.57.0 needs Go 1.26, Go 1.24 gets no security fixes | [D-054](DECISION_LOG.md#d-054) | Verified locally with the foundation (`0ff3341`) |
| The foundation's residual risks accepted for a controlled pilot (username timing after an argon2id parameter change; restore revival window until a deletion journal exists), revisited before a public release | [D-055](DECISION_LOG.md#d-055) | Not applicable (risk acceptance); the privacy text states the restore window (`terms-privacy-2026-11-01.2`); the review is a public-release gate ([blockers](../status/BLOCKERS.md#gates)) |
| PM confirmations of the integration pass: the restore sentence of `terms-privacy-2026-11-01.2` with its second clause (every restore announced in the app and on the status page, both a gate for the controlled pilot; until they exist, no restore of a pilot database without a direct notice through the app to every affected user); the guest case of a restore and `guest-storage-2026-11-01.2`; `golang.org/x/text` v0.42.0; the PR #9 acceptance-criteria list (F-01 to F-47, backend; the web parts of F-17 and F-33 with the frontend PR); ADR-0002 §11 on guests | [D-056](DECISION_LOG.md#d-056) | Not applicable (confirmations); `golang.org/x/text` v0.42.0 is in the foundation commit `0ff3341`; the two policy versions come with the integration pass, committed on `feat/sela-v3-pilot` (draft PR #9); the in-app restore notice and the status page do not exist yet: S-262 criterion 3 is planned and S-263 is blocked externally (B-3, B-15) |
| P4 consultation and sharing defaults: the recommended defaults of contract §13 (CS-OP-1 to CS-OP-28); AT-4 drafts from the person's picked quotes only (`mode: "sources"` off by default until B-10 and the policy texts); the legal points are interim until B-10 | [D-057](DECISION_LOG.md#d-057) | Planned; the contract is committed on `feat/sela-v3-sharing`, and the P4 build follows it |

## Engineering defaults (v3)

| Decision | Entry | Status / implication |
| --- | --- | --- |
| An unsaved conversation is held only in bounded server memory and purged at End, at session end, after 30 minutes idle or 2 hours, or on restart | [D-051](DECISION_LOG.md#d-051) | In progress (V27; backlog S-020, backend on a separate branch, not merged) |
| Operator access to personal data or keys is audited, content-free; no tool shows an operator a user's content | [D-052](DECISION_LOG.md#d-052) | Planned (V32) |
| Merge of main `7fc6678` (owner's DeepSeek fallback contract, PR #6): v3 IDs renumbered so that main's published IDs are never reused; v3 AI documents aligned with the owner contract; differences listed in ADR-0003 §17 | [D-053](DECISION_LOG.md#d-053) | Not applicable (documentation and IDs); nothing is implemented by it |

## Engineering defaults (v2)

These describe the implemented v2 runtime. Where v3 replaced them, the v2 code still behaves this way until the replacement lands.

| Decision | Status / implication |
| --- | --- |
| Single static-web deployment: React/TypeScript + Vite, Go serves API and assets | Implemented (v2); kept in v3 (D-032) |
| In-memory bounded state per visitor (D-005, D-014) | Implemented (v2); superseded for real use by D-027 to D-029; kept for development, demo and test |
| Text-first, adult fixtures and fictional practice text (D-003) | Implemented (v2); superseded by D-026; kept as the v2 demo |
| Clinical handoff simulation: per-owner fictional recipient, link token + access code, separate sharing consent, scripted consultation and plan, demo labels (D-006, D-007, D-008) | Implemented (v2); the token + code model carries into v3; states superseded by D-041; fictional recipients are demo-only (D-042) |
| Help resources from v1 adult `verified_official` entries (D-018) | Implemented (v2); superseded by D-044 |
| Provenance values `live_meta`, `recorded_demo`, `manual`, `test_double` (D-019) | Implemented (v2); main's V10 adds the owner's `live_deepseek_fallback` (C13), which the v2 runtime does not emit (T5b open); extended by D-030 and D-040, and v3 uses the owner's `live_deepseek_fallback` for the fallback draft (D-053); v3 also uses `test_double` as a verification value for labelled test-double verifiers (SPEC V35, planned; [conflict matrix](../status/CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 58) |
| Deterministic safety lexicon, no ML classifier (D-017) | Implemented (v2); kept; D-036 adds second safeguards |
| Story Ledger and Meaning Lock, deterministic Next Step, Opening Practice (D-010 to D-012) | Implemented (v2); Meaning Lock and Opening Practice carry into v3; Next Step matching is demo-only (D-042) |
| No fine-tuning; no retrieval at inference (D-016) | Not applicable (nothing to build); kept by D-048 |
| Cost guards on Modal and per-session limits (D-022) | Implemented (v2); extended with per-provider ceilings (ADR-0003 §11) |
