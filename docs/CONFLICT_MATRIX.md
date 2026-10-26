# Conflict matrix — Sela Relay proposal vs SPEC vs implementation vs owner direction

Created: 2026-10-26 (Milestone 0). Authority: [SPEC.md](../SPEC.md). This matrix records every material difference found during the baseline audit between:

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
