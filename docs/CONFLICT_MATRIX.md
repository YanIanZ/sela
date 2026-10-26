# Conflict matrix — Sela Relay proposal vs SPEC vs implementation vs owner direction

Created: 2026-10-26 (Milestone 0); v3 section added 2026-10-26 ([below](#v3-owner-brief-v3-vs-the-v2-spec-and-code)). Authority: [SPEC.md](../SPEC.md). This matrix records every material difference found during the baseline audit between:

- **SPEC** — `SPEC.md` at `89ac3cf` (before the v2 amendment in this branch).
- **Relay** — the non-normative [Sela Relay package](research/sela-relay/README.md), first published in `27f6394`.
- **Owner** — the owner's implementation brief of 2026-10-26 (the prompt that started this branch). User direction outranks the spec; the spec is amended *before* dependent code.
- **Code** — what actually ran at `89ac3cf`: the inherited Next.js/TypeScript v1 only. No Go code, Vite app, `/api/v2` route or Meta adapter existed.

Each row ends with a resolution and the decision that records it in the [decision log](DECISION_LOG.md). "Kept" means the stricter existing boundary stays.

| # | Topic | SPEC (89ac3cf) | Relay proposal | Owner direction | Code (89ac3cf) | Resolution |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | AI provider | Hosted "Meta Model API" adapter (I.meta); model/endpoint unverified | Hosted `api.meta.ai` examples, explicitly unverified | Meta **open-weight** model self-hosted on **Modal.com**; licence, revision pin, GPU, cost and hackathon fit must be verified | v1 multi-provider (AMD default, Gemini, DeepSeek, OpenRouter) | Adopt self-hosted Meta open-weight model on Modal; V9 "Meta-only" kept; no other provider in v2. [D-001], [ADR-0001] |
| 2 | Hackathon tooling eligibility | "External event terms control eligibility"; unverified | Mentions judging weights; unverified | Do not assume self-hosting counts as Meta tooling | n/a | Recorded as **unverified** with sources in ADR-0001 and [blockers](BLOCKERS.md). No eligibility claim. [D-002] |
| 3 | Story input | V2: seeded fixtures only; **no arbitrary personal stories** | Same; "free text input requires SPEC amendment" | "Ruang bercerita — input teks"; express feelings; keep synthetic-adult boundary | v1 Care Passport accepts free text | Amend V2: keep seeded scenarios as default demo; add bounded **practice text** that the UI labels as fictional/practice-only, with adult attestation, crisis gate, size limits, memory-only storage and no content logs. Not a real-patient intake. [D-003] |
| 4 | Consent model | V4: AI consent, brief approval, sharing consent | Same + share grant per recipient/version/field | Separate consent for **AI processing, storage, sharing**; working retention and deletion | v1: client-side consent + AI consent | Add **storage/retention consent** as a fourth independent, versioned decision; retention choices enforced by the store janitor; delete-now purges. [D-004], [D-005] |
| 5 | Identity | Optional anonymity "not main promise"; synthetic identity choice | Anonymous/pseudonymous optional | Anonymous or pseudonymous mode per threat model; honest explanation of what is still processed | v1 anonymous local storage + E2EE recovery | Anonymous default; optional pseudonym (bounded, warned). Privacy page lists what is processed and by whom (app memory, Modal GPU worker, transport metadata). No E2EE claim. [D-006] |
| 6 | Professional access | Fictional professional view is a projection **inside the same visitor session**; role switch is a demo control | Recipient-bound receipt; role switch demo only | Link/token with expiry and revocation; **recipient authorization — token alone must not grant access** | v1 has no handoff | Share grant issues a capability token (URL fragment) **and** a separate recipient access code; a recipient principal (own cookie) is scoped to that one grant; fictional recipients are instantiated per owner session so no cross-visitor inbox exists. [D-007] |
| 7 | Handoff states | `simulated_requested → acknowledged → completed`; share `active → revoked` | Adds clarification-requested, declined, expired | Receipt, acknowledgement; follow-up must not equate "sent" with "care received" | none | Adopt Relay states: `requested`, `acknowledged`, `clarification_requested`, `declined`, `completed`; share `active`, `revoked`, `expired`. UI never says care was received. [D-008] |
| 8 | Streaming / cancel | Cancellation and timeouts required (V9) | Deadline + cancellation | Streaming, cancel, retry, timeout and error states | v1 non-streaming | Server-sent events for AI operations; only **validated** items are streamed; cancel endpoint; retry = new operation; bounded deadlines; cold-start state. [D-009] |
| 9 | Brief / ledger semantics | Source-backed brief; editing invalidates approval | Story Ledger + Meaning Lock (negation, subject, time, uncertainty) | Separate user facts, model interpretation, unknowns; clarify ambiguity; correct/delete/approve; versioning | v1 Care Passport: quote grounding, unknowns, clarification, approval (inherited work) | Port v1 grounding behaviour to Go and add Meaning Lock checks (negation window, subject, time, uncertainty, diagnosis/medication/percentage). Each item shows user quote vs Sela wording vs status. Inherited, not new invention. [D-010] |
| 10 | Next Step | Deterministic matching (V8) | One practical barrier, deterministic options, plain rationale | Practical options from preferences and barriers with simple rationale; user decides | v1 support options config | Deterministic catalogue filter + templated rationale; "not ready" path without handoff; no model involvement. [D-011] |
| 11 | Opening Practice | Not in SPEC | P1, source-bound, no roleplay therapist | Required; ID/EN and code-switching tested; non-manipulative | v1 opening-lines pipeline (inherited) | Port: model draft constrained to approved items (vocabulary validator) or deterministic template; user practice attempt gets a deterministic coverage check, never a score. Inherited feature adapted. [D-012] |
| 12 | Frontend | React/TS + Vite static, Go serves assets (C6) | Same | New frontend built from the journey | v1 Next.js | Keep C6. New `web/` app; v1 UI untouched until cut-over. [D-013] |
| 13 | Storage | In-memory session store (C7) | Same; PostgreSQL only for production later | Storage abstraction; memory store for demo; no implicit sensitive persistence | v1 PostgreSQL/Redis/Supabase | `store.Store` interface, only a bounded memory implementation; restart erases state and the UI says so. [D-014] |
| 14 | Research corpus | Non-normative register | 100 title/abstract-screened; full-text pending | Reproducible pipeline, rolling 5-year cutoff, rights tracking, null for unknowns, retrieval only if needed | Python collectors/screening for 7,580 metadata candidates | New reproducible corpus pipeline over the 100-record register; licence/access recorded; reviewer search in Go; no full text in Git; **no fine-tuning** (criteria unmet). [D-015], [D-016] |
| 15 | Register integrity claim | n/a | Doc 10 states first-publication years 2023: 2; 2024: 3; 2025: 86; 2026: 9 | Do not claim the old corpus meets the age window without re-checking | `research_100.json` fields give 2023: 2; 2024: 3; 2025: 94; 2026: 1 | Discrepancy recorded; the pipeline recomputes from verified sources and reports both. [D-015] |
| 16 | Safety classifier | Deterministic gate before inference (V3) | Keyword gate is not clinically sensitive | Gate before inference; classifier never the only safeguard; test negation, quotes, slang, code-switching, ambiguity | v1 deterministic ID/EN lexicon (inherited) | Port v1 lexicon to Go with clause-level negation/quote/third-party handling; no ML classifier in v2 (Llama Guard deferred, see D-017). [D-017] |
| 17 | Help contacts | Geography, source, verification date required | Do not print unverified numbers | Verified, maintained references; never invent numbers | v1 contacts with sources, accessed 2026-10-14/15 | Reuse only v1 entries marked `verified_official` for adults, with their sources and access dates; status "re-verification due". [D-018] |
| 18 | AI provenance values | `live_meta`, `recorded_demo`, `manual` | Same | Test doubles labelled as test-only; never called live | v1 labels provider names | Add `test_double` provenance (CI/dev only; UI banner). `live_meta` requires the configured Modal endpoint and a matching pinned model ID. [D-019] |
| 19 | Fine-tuning | Excluded (C9) | Excluded | Only after measurable goal, licensed data, baseline gap, splits, bilingual eval | v1 Gemma training draft archived | Not used; rationale documented in [research method](research/CORPUS_PIPELINE.md). [D-016] |
| 20 | AMD / v1 runtime removal | T9 after acceptance | Remove AMD after acceptance | Migrate gradually; keep needed behaviour until replacement passes | v1 still default | v1 kept and still tested; v2 has no AMD code (V14). Removal stays pending until acceptance including live Meta evidence. [D-020] |
| 21 | Commit identity / attribution | n/a | Preserve Dengar.in credit | Use verified YanIanZ identity; no agent attribution; no signed/Verified claim | History authored by YanIanZ + Dengar.in team | Commits use the YanIanZ name/email that GitHub links to the account; unsigned; Dengar.in credits preserved. [D-021] |

[D-001]: DECISION_LOG.md#d-001
[D-002]: DECISION_LOG.md#d-002
[D-003]: DECISION_LOG.md#d-003
[D-004]: DECISION_LOG.md#d-004
[D-005]: DECISION_LOG.md#d-005
[D-006]: DECISION_LOG.md#d-006
[D-007]: DECISION_LOG.md#d-007
[D-008]: DECISION_LOG.md#d-008
[D-009]: DECISION_LOG.md#d-009
[D-010]: DECISION_LOG.md#d-010
[D-011]: DECISION_LOG.md#d-011
[D-012]: DECISION_LOG.md#d-012
[D-013]: DECISION_LOG.md#d-013
[D-014]: DECISION_LOG.md#d-014
[D-015]: DECISION_LOG.md#d-015
[D-016]: DECISION_LOG.md#d-016
[D-017]: DECISION_LOG.md#d-017
[D-018]: DECISION_LOG.md#d-018
[D-019]: DECISION_LOG.md#d-019
[D-020]: DECISION_LOG.md#d-020
[D-021]: DECISION_LOG.md#d-021
[ADR-0001]: adr/ADR-0001-meta-open-weight-model-on-modal.md

## Boundaries explicitly kept

- C3 synthetic people, fictional professionals, simulated care. Practice text does not turn the demo into a real service; the UI says so on every care surface.
- Adult-only scenarios and adult attestation (C8). No minors' pathway is offered.
- No diagnosis, triage, prescription, therapy roleplay, real booking, payment, forum or gamification (C9).
- No AI fallback to a non-Meta provider (V9); outage means an honest unavailable state plus the manual path.
- No content, prompt or model output in operational logs (V12).
- Provenance and credits for Dengar.in and its team ([provenance](SELA_PROVENANCE.md)).

## After merging main (PR #5, EvidenceGuard)

| Topic | main (owner, PR #5) | This branch | Resolution |
| --- | --- | --- | --- |
| Second AI provider | Optional Claude Sonnet 5.5 verifier, separate consent, verification only (C12, V16–V18, T5a) | Meta only (V9) | Kept as main defines it; standalone prototype, not wired into v2 (D-024) |
| Generator model | Muse Spark / "Muse Spark Open on Modal" | Muse Glimmer 30B on Modal (ADR-0001) | Glimmer kept for budget and access; owner decides (B-7) |
| Spec IDs | C12, I.verifier, V16–V18 | C12–C13, V16–V19 (unpublished) | Branch IDs renumbered to C13–C14, V20–V23 |

## v3: owner brief v3 vs the v2 spec and code

Added 2026-10-26. Compared:

- **Brief** — owner brief v3 of 2026-10-26 (Indonesian; it governs and supersedes the synthetic-only, adults-only, memory-only and Meta-only limits).
- **v2 spec** — `SPEC.md` at `e9711af`, before the v3 realignment.
- **v2 code** — branch `feat/v2-go-meta-runtime` at `e9711af` (PR #6), as read by the discovery gap analysis of 2026-10-26.

Each resolution names the PM decision ([decision log](DECISION_LOG.md)) and the spec IDs that now hold it. Every v3 resolution is **planned**: the v2 code still behaves as the "v2 code" column says until the replacing milestone lands.

| # | Topic | v2 spec | v2 code | Brief | Resolution |
| --- | --- | --- | --- | --- | --- |
| 22 | Product scope | Synthetic or fictional data only (C3); synthetic labels on every care view (V1) | `Synthetic: true` hard-coded in views, projections and recipient views; `/health` reports `mode: "synthetic_demo"`; fixture professionals must be named "fiktif"/"fictional" | Real use; synthetic data only for testing, evaluation and demo (§1, §11) | C3 and V1 superseded → C23, V40, V28. The v2 journey becomes the demo configuration. [D-025], [D-045] |
| 23 | Ages | Adults only, attested (C8) | `adultAttested` enforced (`internal/journey/session.go`); projection `age_range: adult_18_plus` | Users from 15 to 50+; age-appropriate experience; verify consent and provider terms (§6) | C18, V35: 15–17 built, off in production until B-10; 15-year-olds blocked until the low-risk assessment. [D-030] |
| 24 | Storage | Bounded in-memory store; restart erases state (C7, V21) | `internal/store` is the only store; `/ready` reports `persistent: false` | Persistent personal notes, timeline, export, deletion of derivatives, a clear backup policy (§7, §10) | C15, V25, V32: PostgreSQL 16, per-user envelope encryption, crypto-shredding, stated backup windows. [D-026], [D-027] |
| 25 | AI providers | Meta is the only live generator; no fallback (C12, V9; D-001) | `llm.Mode` allows only `modal`, `test_double`, `disabled` | Meta primary, Claude Sonnet 5.5 verifier, DeepSeek-V4.1-Flash-1017 fallback; no fallback past a safety refusal (§8) | C17, V33, V34: availability-only fallback with its own consent; Claude verifies, never generates. [D-029], [ADR-0003] |
| 26 | Verifier input and verdicts | Only synthetic excerpts may go to Anthropic (V16) | `prototypes/evidenceguard` rejects `Synthetic=false`; verdicts lack `unsupported`; not wired | Verify real stories, four structured verdicts, no chain-of-thought (§8) | V34, V39: minimised real-user data with consent; supported / unsupported / contradicted / insufficient_evidence. [D-029] |
| 27 | Listening | "General chatbot" excluded (C9) | No conversation surface; one story textarea | Listening is the core function; start without a long profile (§1, §5) | C9 superseded; V24 and the conversation room. [D-034] |
| 28 | Crisis handling | Gate before inference (V3) | `Safety.Interrupted` ends the whole session and discards the story and draft. v1: hard redirect to `/crisis` (DX-03) and refused journal saves (DX-02) | Do not turn every conversation into intake; urgent support never waits for models; not a single classifier (§5, §8, §11) | V3 kept; V36: inline help, no erasure, AI paused per conversation, entries saved with support, second safeguards. [D-035] |
| 29 | Identity | Anonymous or pseudonymous session (C13; D-006) | Cookie session only; no login or recovery | Authentication and recovery; guest mode states location and duration; no absolute anonymity (§10) | C16: username and password, recovery codes, guest mode with stated purge windows. [D-028] |
| 30 | Handoff states | `requested → acknowledged → completed`, plus clarification and declined (D-008) | `HandoffStatus` with a scripted "simulated consultation completed" | Prepared, shared, opened, acknowledged, consultation in progress; a sent message is not care (§7) | V28: new statuses plus revoked, expired, ended; no status implies care. [D-040] |
| 31 | Recipients | Deterministic matching of fictional professionals (V8) | Recipients come only from fixtures | No partnership, booking or verified-professional claims unless they really exist (§7) | User-invited, unverified, labelled recipients; matching stays demo-only. [D-041] |
| 32 | Sharing consent | Content (fields) and recipient (V5) | No `purpose` field anywhere | Consent by content, recipient and purpose (§7) | V28 adds purpose. [D-040] |
| 33 | Access audit | Timeline of handoff events | `opened` recorded on the first sign-in only | Access audit (§7) | V30: every recipient view and every privacy-relevant action is audited |
| 34 | Export | None | No route or UI | Readable and structured export (§7) | V38, I.export: HTML, Markdown and JSON `sela-export/1` |
| 35 | Provenance | Quote vs Sela wording in the Story Ledger | `WordingOrigin` per item | Separate the user's statement, AI summary, correction, unconfirmed interpretation, upload and verified professional note, each with time, version and review status (§7) | V26: six classes (two reserved) with review status. [D-039] |
| 36 | Uploads | None | None | Uploaded documents are a separate class; validate uploads if they exist (§7, §10) | Deferred from the pilot; class reserved. [D-042] |
| 37 | Help resources | Adult v1 entries only (D-018) | 3 entries; `internal/fixtures/fixtures.go` checks only the date format of `reverifyBy` | Verify local support sources; never invent numbers (§11) | V37: server-enforced re-verify-by, teen resources verified, KPAI WhatsApp not shown; re-verification is B-5. [D-043] |
| 38 | Engagement | Gamification excluded (C9) | v1 still awards points, levels and badges | Reject guilt notifications and punishing streaks (§5) | C19: none in the pilot; gamification retired. [D-036], [D-037] |
| 39 | Terms | "Catatanku / My note", "brief" | v2 copy | "Catatan pribadiku", "Ringkasan untuk konsultasi"; never an official medical record (§7) | C21. [D-038] |
| 40 | Accessibility target | Accessible controls (V13) | axe runs only `wcag2a, wcag2aa, wcag21a, wcag21aa` and fails only on serious or critical findings (`e2e/journey.e2e.mjs`); reflow checked at 390 px | WCAG 2.2 AA verified against official sources (§6) | C25; WCAG 2.2 tags, 320 px reflow and manual checks of the six new criteria in P7 (T20) |
| 41 | Dengar.in features | Forum, gamification and others excluded by C9 | Still in v1 code | Do not remove features only to ease the refactor; keep attribution (§2) | Each feature kept, improved, merged, migrated, paused or retired with a reason. [D-037], [feature matrix](DENGARIN_FEATURE_MATRIX.md) |
| 42 | Content kinds | Not distinguished | Research corpus separate from sessions | Separate public content, personal data, community content, demo data and research corpus (§2) | C20. [D-032] |
| 43 | Research coverage | Register built for the v2 Relay features | 0 direct records on listening, journaling, consultation preparation or accessibility; 2 on adults 50+ | Map evidence for listening, journaling, help-seeking, consultation preparation, continuity, adolescents, older adults, Indonesia, accessibility, safety and privacy (§9) | Gaps recorded and researched without paywall bypass; no fine-tuning. [D-047], SPEC T19 |
| 44 | Minors and provider terms | Not applicable (adults only) | Not applicable | 15+ users; verify provider terms (§6) | Muse Glimmer is "not intended for individuals under the age of 18" (deployer risk duty); no DeepSeek for minors; production gated. [D-030], ADR-0003 §8.3, B-10 |
| 45 | Hackathon | Eligibility unverified (D-002); Meta only | — | Verify the rules on providers; never hide Claude or DeepSeek or claim their work as Meta's (§11) | Meta-only demo path, provider labels, organizer question. [D-049], B-12 |
| 46 | Legacy deployments | Not covered | Two Dengar.in deployments live outside this repository | Real use | Recorded as a risk; owner/team action. [D-046], B-13 |

### Documentation and contract follow-ups (open)

These are conflicts between documents, or between a document and uncommitted work, found while writing the v3 spec. Each names who resolves it.

| # | Conflict | Resolution |
| --- | --- | --- |
| 47 | [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) §6 and its consequences call the email-provider blocker "proposed B-8"; in [blockers](BLOCKERS.md) B-8 is the Anthropic key and the email provider is B-11 (PM decisions of 2026-10-26). ADR-0003 §13 already notes this | Blocker numbers follow the PM decisions; ADR-0002 is updated by its owner (backend foundation) |
| 48 | ADR-0002 §1 leaves the Go toolchain ruling pending; the uncommitted foundation work in the worktree moves `go.mod` to `go 1.26.0` with `toolchain go1.26.10` (option a), while `README.md`, `CONTRIBUTING.md` and `deploy/v2/Dockerfile` still say Go 1.24 | Run instructions stay unchanged until the foundation lands; the foundation change updates the Dockerfile, runbook and contributor docs together |
| 49 | API_SPEC_V3 has only the age band `15_17`, but D-030 blocks 15-year-olds while allowing 16–17 where enabled; the design lead proposes asking "15" and "16–17" separately ([design direction §10](design/DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners)) | Open; decided with the minors work (SPEC T20) before any environment enables `15_17`. Production allows only `18_plus` meanwhile, so nothing is exposed |
| 50 | The v2 AI policy text (`internal/fixtures/data/policies.json`) calls the Meta model "Llama"; whether Muse Glimmer is Llama-derived is unverified | v3 policy texts name the model as served (D-048); the v2 text is corrected with the v3 policies |
| 51 | [FORMAT.md](../FORMAT.md) and [CONTRIBUTING.md](../CONTRIBUTING.md) still describe "Sela v2" | Their rules apply to v3 unchanged; the wording is updated by their owners |
| 52 | **Demo AI.** [D-045](DECISION_LOG.md#d-045) lets the demo use labelled test doubles or live providers on synthetic data. [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) §12 says the demo "never" uses the test double (else the honest unavailable state), and the uncommitted `internal/config/config.go` refuses `SELA_LLM_MODE=test_double` outside development and test. That is narrower than D-045 and, without configured providers, leaves the demo's AI features unavailable, against brief §11 ("fiturnya harus benar-benar bekerja") | SPEC C23, V40 and [product](PRODUCT.md) follow D-045; D-045 governs and is not reopened. Resolved by the **backend foundation agent**, who owns ADR-0002 §12 and the config guard, by allowing a labelled test double in the demo (generator and verifier; SPEC V33 now has a `test_double` verification value, row 58). Until then the stricter guard applies, and no demo output is ever presented as live |
| 53 | **Shared rate limiting.** ADR-0002 §9 and its consequences place the shared limiter in P7; the [backlog](BACKLOG.md) puts it in S-268 (later, after the pilot), because the pilot runs one instance | SPEC T20 follows the backlog: S-268 after the pilot, and before any scale-out. The ADR-0002 wording is updated by the backend foundation agent |
| 54 | **Status words.** The [backlog](BACKLOG.md) §1, [implementation status](IMPLEMENTATION_STATUS.md) §1 and [roadmap](ROADMAP.md) §2 each restate the status definitions, with wording that differed from SPEC §G | [SPEC §G](../SPEC.md#g-goal) is the single definition (committed code for "implemented"; browser review for v3 items with a screen; deployed environment on synthetic data for "verified live"). The backlog owner replaces the restatements with a link; the roadmap keeps only the checks per level |
| 55 | **B-9 as a pilot gate.** The roadmap §1 diagram lists B-9 among the owner actions that gate "ready for controlled pilot" without a condition; the [blockers gates](BLOCKERS.md#gates), [product](PRODUCT.md) and SPEC T21 need B-9 only if the pilot enables DeepSeek | B-9 gates the pilot only if DeepSeek is enabled. The backlog owner updates the roadmap diagram |
| 56 | **"Don't save" hold time.** [Design direction §10](design/DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners) left open how long an unsaved conversation stays on the server; the copy says Sela "discards it when the conversation ends" | Settled by [D-050](DECISION_LOG.md#d-050) and SPEC V25: server memory only, purged at End, at session end, after 30 minutes idle or 2 hours, or on restart. The design lead aligns the copy (abandoned conversations are discarded too; "discarded" covers only what Sela holds); the P3 conversation contract (API v3 §9, backend) specifies it |
| 57 | **Operator access audit.** SPEC V30 now requires a content-free operator audit ([D-051](DECISION_LOG.md#d-051)); ADR-0002 §13 and the runbook do not yet describe where that record is written or who reviews it | Resolved by the backend foundation agent (ADR-0002, runbook); the backlog owner attaches it to an item (for example S-265 or S-266) |
| 58 | **Verification provenance for a test-double verifier.** [ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md#101-verification-status-and-provenance-engineering-default) §10.1 lets a labelled test-double verifier stand in for Claude or DeepSeek in development, test (EV-7) and the demo (D-045, V40, C23), but SPEC V33 listed no verification provenance value for its reports; ADR-0003 §10.1 flags this as a SPEC gap for the PM and proposes `test_double` | SPEC V33 adds the verification value `test_double`: labelled "TEST DOUBLE — not Claude" or "— not DeepSeek", refused in production, never stored or shown as a check by Claude or DeepSeek. This is a SPEC amendment under D-045 and reopens no decision. The ADR-0003 owner replaces the §10.1 note ("once SPEC adds one") with a reference to V33 and adds the value to the list in §10; the [AI policy](AI_POLICY.md) owner does the same in its provenance list |

[D-025]: DECISION_LOG.md#d-025
[D-026]: DECISION_LOG.md#d-026
[D-027]: DECISION_LOG.md#d-027
[D-028]: DECISION_LOG.md#d-028
[D-029]: DECISION_LOG.md#d-029
[D-030]: DECISION_LOG.md#d-030
[D-032]: DECISION_LOG.md#d-032
[D-034]: DECISION_LOG.md#d-034
[D-035]: DECISION_LOG.md#d-035
[D-036]: DECISION_LOG.md#d-036
[D-037]: DECISION_LOG.md#d-037
[D-038]: DECISION_LOG.md#d-038
[D-039]: DECISION_LOG.md#d-039
[D-040]: DECISION_LOG.md#d-040
[D-041]: DECISION_LOG.md#d-041
[D-042]: DECISION_LOG.md#d-042
[D-043]: DECISION_LOG.md#d-043
[D-045]: DECISION_LOG.md#d-045
[D-046]: DECISION_LOG.md#d-046
[D-047]: DECISION_LOG.md#d-047
[D-049]: DECISION_LOG.md#d-049
[ADR-0003]: adr/ADR-0003-ai-routing-verifier-fallback.md

### Boundaries kept from v2

The "Boundaries explicitly kept" list above described v2. In v3: C3 (synthetic only), C8 (adults only) and the "no fallback" rule (V9) are superseded as listed in rows 22–25. Still kept: no diagnosis, triage, prescription, therapy roleplay, real booking or payment (C22, C24); no forum or gamification in the pilot (C19, C24); no content, prompt or model output in logs (V12); and the provenance and credits for Dengar.in and its team ([provenance](SELA_PROVENANCE.md)).
