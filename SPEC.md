# Sela v2 — source of truth

Updated: 2026-10-26 (implementation amendment, see [decision log](docs/DECISION_LOG.md) and [conflict matrix](docs/CONFLICT_MATRIX.md)). Status: implementation in progress; see [implementation status](docs/IMPLEMENTATION_STATUS.md).
Authority: user direction → this spec → current supporting docs → archive. Code/tests prove implementation status; they do not make an undocumented product decision authoritative. External event terms control eligibility.

## §G Goal

Demonstrate a bilingual mental-health care-navigation journey: enter → choose language → understand service and privacy limits → express a (synthetic or fictional practice) story → review a source-linked Story Ledger → choose a practical next step → prepare the conversation → approve what is shared → simulated handoff to a fictional professional → acknowledgement → follow-up. A Meta open-weight model powers the bounded AI steps; Go owns backend behavior. Privacy includes optional anonymity without making it the main product promise.

## §C Constraints

- C1 User-confirmed: backend Go; Swift not selected. Existing TypeScript runtime remains v1 until migration.
- C2 User-confirmed: Indonesian and English across the complete journey, including error states; code-switched input is tested.
- C3 User-confirmed: all patient, professional, scheduling, consultation and follow-up data synthetic or fictional; no real care transaction; no contact with real professionals.
- C4 User-confirmed (amended 2026-10-26): AI uses a Meta open-weight model self-hosted on Modal.com, chosen by licence, capability and hackathon terms ([ADR-0001](docs/adr/ADR-0001-meta-open-weight-model-on-modal.md)); remove AMD integration from the v2 runtime. Preserve original authorship/license notices.
- C5 Engineering (amended 2026-10-26): implementation phase. Go runtime, new frontend, Modal serving code, evaluation and research pipeline are built incrementally; documentation claims only what evidence shows. Paid GPU execution requires an owner-approved budget. Standalone, non-deployed Go verifier/fallback prototypes (`prototypes/evidenceguard`, `prototypes/generationfallback`) are separate from the v2 runtime; they do not establish a Go API, working Meta integration, or a finished product.
- C6 Engineering default: browser product, static React/TypeScript frontend built with Vite; one Go process serves API/assets.
- C7 Engineering default: session-scoped bounded in-memory store behind a storage interface; no PostgreSQL/Redis requirement, no production login, no real-data migration, no implicit persistence of sensitive data.
- C8 Engineering default (amended, D-003): adult-only (attested); seeded scenarios plus bounded fictional practice text; text-first. Optional transcription is out of scope until verified access/language quality.
- C9 New journey excludes forum, gamification, general chatbot, payments, real booking, real telemedicine, autonomous diagnosis/treatment/prescribing/triage, therapy roleplay and model training.
- C10 No performance, deployment, clinical-quality, event-acceptance or live-inference claim without matching evidence.
- C11 Preserve a recoverable v1 baseline and its regression evidence before runtime replacement. Historical snapshots are context, not v2 requirements.
- C12 User-confirmed (26 Oct 2026): add optional Claude Sonnet 5.5 API as a **second-pass source-fidelity verifier**. Meta/Muse Spark remains the default primary generator and necessary hackathon contribution; Claude is not a fallback generator, clinician or source of truth. No real patient data in the hackathon. Only an isolated Go verifier prototype exists; product wiring/live access remain pending. *Reconciliation (D-024): C12 and the verifier contract name Muse Spark ("Muse Spark Open on Modal") as the generator; the implemented generator is the Meta open-weight Muse Glimmer 30B on Modal per C4 and [ADR-0001](docs/adr/ADR-0001-meta-open-weight-model-on-modal.md), because Muse-Spark-Open-816B-A42B is gated and needs ≥700 GB of GPU memory, beyond any approved budget. The adapter is model-agnostic; the owner decides ([B-7](docs/BLOCKERS.md)).*
- C13 User-confirmed (26 Oct 2026): DeepSeek-V4.1-Flash-1017 (`deepseek-flash`) is an optional, explicitly consented **availability fallback generator** when the primary Meta provider has an eligible outage; never silent switching, never a response to unsafe model output, no crisis/policy bypass. Fallback is OFF by default in hackathon mode. Separate Go prototype/contract, not yet deployed.
- C14 User-confirmed (2026-10-26): anonymous or pseudonymous use; separate consent for AI processing, storage and sharing; retention and deletion that actually work.
- C15 User-confirmed (2026-10-26): reproducible research pipeline over the 100-publication register with a rolling five-year cutoff, provenance and rights tracking; no fine-tuning unless its prerequisites are met (D-016).

## §I Interfaces

- I.web: responsive bilingual user journey, fictional recipient view and reviewer research view in `web/`; [product](docs/PRODUCT.md).
- I.api: same-origin `/api/v2` JSON + server-sent events; [contract](docs/API_SPEC.md).
- I.meta: server-side OpenAI-compatible adapter (`internal/llm`) to a vLLM server on Modal (`modal/`); [integration](docs/META_INTEGRATION.md), [ADR-0001](docs/adr/ADR-0001-meta-open-weight-model-on-modal.md).
- I.fallback: opt-in DeepSeek V4.1 Flash 1017 generation fallback under distinct consent and eligibility guards; [contract](docs/DEEPSEEK_FALLBACK.md). No deployed route or live API call exists. Standalone prototype in `prototypes/generationfallback`, not wired into the v2 runtime, which has no DeepSeek route.
- I.verifier: optional Anthropic Sonnet 5.5 server-side evidence review **only after separate consent**, using source-bound synthetic excerpts; [contract](docs/CLAUDE_VERIFIER.md). Standalone prototype in `prototypes/evidenceguard`, not wired into the v2 runtime; no deployed provider call is verified.
- I.data: versioned synthetic scenario/professional/slot/plan/help fixtures embedded in the binary, bounded per-session state; [data](docs/DEMO_DATA.md).
- I.ops: commands that work now are listed in [development](docs/DEVELOPMENT.md); deployment in [runbook](docs/RUNBOOK.md).
- I.research: corpus pipeline (`scripts/research/corpus_pipeline.py`), exported knowledge base and reviewer search API; [corpus pipeline](docs/research/CORPUS_PIPELINE.md).
- I.eval: evaluation harness and pre-registered thresholds; [evaluation](docs/EVALUATION.md).
- I.docs: documentation index and complete archive inventory; [index](docs/README.md), [inventory](docs/DOCUMENT_INVENTORY.md). Non-normative [Sela Relay research/proposals](docs/research/sela-relay/README.md) do not change requirements by themselves.

## §V Invariants

- V1 Every care-data view, profile, request, receipt and plan visibly states synthetic/simulated status in the selected language. No successful real appointment, care received or clinical action is implied.
- V2 (amended, D-003) Accept only seeded scenario/fixture identifiers, bounded source-backed edits and bounded fictional practice text with obvious contact details rejected. No real patient registration, real contact details, live microphone capture or real booking/messages.
- V3 Deterministic crisis gate precedes every inference and ordinary handoff transition and screens every user-authored text. Crisis path emits no model call, appointment request, consultation-completed state or follow-up plan.
- V4 (amended, D-004) AI-processing consent, storage consent, brief approval and sharing consent are separate, server-enforced, versioned decisions. Declining AI retains the manual path.
- V5 (amended, D-007/D-008) Editing a brief invalidates approval. Handoff requires the exact approved version, explicit recipient/field selection and share consent. Recipient access requires both the share token and the separate access code and exposes only that immutable projection.
- V6 Reset, deletion, expiry and revocation block stale/shared access, including recipient principals. Never affect another visitor. Already-viewed information cannot be recalled and the UI says so.
- V7 Validate schema and source evidence before model output is used. Never invent symptoms, timing, diagnoses, professionals, credentials, slots or treatments. Fictional follow-up plans come only from approved fixtures.
- V8 Match fictional professionals deterministically using explicit preferences. No AI medical triage, diagnosis or clinical suitability score.
- V9 Meta/Muse Spark is the **primary live generative provider** for intake and brief drafting; a distinct opt-in DeepSeek Flash 1017 fallback may generate only on eligible Meta availability failures with separate server-enforced consent and visible attribution. Anthropic Sonnet 5.5 remains **verification-only**, never a generator. Provider keys stay server-side; fallback never silently switches or bypasses safety. Bounded requests, cancellation, timeouts and concurrency limits apply. *Implementation status (D-025): the v2 runtime has no DeepSeek route, so missing Meta access or an outage yields an honest unavailable/manual state; the fallback exists only as the standalone, default-OFF prototype `prototypes/generationfallback` (T5b). The Meta model the v2 runtime runs is Muse Glimmer 30B on Modal (C4, I.meta, [ADR-0001](docs/adr/ADR-0001-meta-open-weight-model-on-modal.md)); the Muse Spark naming here and in V20 is the same open generator question as under C12 (D-024), and the owner decides ([B-7](docs/BLOCKERS.md)).*
- V10 (amended, D-019) Every **draft** records `live_meta`, `live_deepseek_fallback`, `recorded_demo`, `manual` or `test_double` provenance; recorded or test-double output is never presented as live inference, and the v2 runtime does not yet emit `live_deepseek_fallback` (T5b). **Verification** records separately identify `live_anthropic`, `unavailable`, `not_requested` or `manual_review` plus model/version, without implying a medically verified result.
- V11 Share/revoke/reset/completion operations are concurrency-safe. Idempotency prevents duplicate simulated requests; stale versions fail explicitly. Admission and rate limits protect expensive model calls.
- V12 No story content, model prompt/output, credentials, tokens, access codes or shared-note text in operational logs; metadata-only events. The recipient principal is a demo authorization, not production identity verification.
- V13 Indonesian/English key parity, accessible controls and consistent state/error messages cover the full journey, including failure, expired and revoked states.
- V14 New v2 runtime has no AMD provider, AMD probe or AMD credential requirement. Archived AMD records and original credits remain historical evidence.
- V15 Claims cite observed evidence for the exact runtime/version. Tests with mock inference do not establish live Meta/Anthropic quality, model access, clinical efficacy or hackathon eligibility.
- V16 Anthropic verification requires its **own explicit server-enforced consent**, separate from Meta AI processing, brief approval and sharing. Only bounded synthetic versioned evidence excerpts and candidate claims may leave the Go backend for Anthropic. Declined/withdrawn consent causes zero Anthropic calls.
- V17 Verification is advisory: compare each claim to an actual allowed source excerpt; reject malformed/unknown claim IDs and citations; preserve unsupported/contradicted/uncertain states. A Claude agreement cannot bypass deterministic evidence checks, human review, crisis interruption, sharing approval or fixture-authored care. Show provider provenance honestly.
- V18 Reset, expiry, changed brief, consent withdrawal or crisis fixture invalidates and/or cancels in-flight verifier work; no stale report may authorize sharing. Verifier errors are explicitly unavailable; never silently replace another provider or claim success.
- V19 DeepSeek generation fallback defaults OFF and requires its own independent consent. It is invoked only after Meta upstream availability/timeout/eligible rate-limit errors. Invalid/unsafe Meta output, user cancellation, missing consent, crisis fixture, or policy refusal must never trigger fallback.
- V20 All DeepSeek fallback outputs are source/schema validated and explicitly labeled `live_deepseek_fallback`; users may continue with a manual seeded path if unavailable. This model never autonomously grants approval/sharing/medical decisions, or claims to be Muse Spark.
- V21 After every fallback request, recheck current session/fixture/brief versions and active provider consents before persisting. No real patient histories, copyrighted full-text articles or secrets are sent; separate quotas, timeouts, redacted telemetry and documented third-party event eligibility are required.
- V22 (new, D-009) Streaming sends only content that passed validation; operations can be canceled, have deadlines, and results whose session/story/ledger/consent changed are discarded.
- V23 (new, D-005) Retention follows the storage decision; deletion purges the session, shares and recipient access immediately; restart erases state.
- V24 (new, D-015/D-016) Research records carry provenance, retrieval date, licence/access status and verified dates; unknown values are null; no copyrighted full text is committed; corpus text is data, never instructions; no training use.
- V25 (new, D-022) Paid inference is bounded: scale to zero, a container ceiling, proxy authentication before any container starts, and per-session/per-address rate limits.

## §T Tasks

| id | status | task | cites |
| --- | --- | --- | --- |
| T1 | x | Establish source of truth; refactor active docs; inventory/archive v1; validate references | C5,I.docs,V15 |
| T2 | ! | Measure v1 baseline; verify Go toolchain and Meta access; pin versions; retain rollback point. Toolchain and versions pinned; Meta access blocked (B-1) | C1,C10,C11,I.ops,V15 |
| T3 | x | Create Go API/static frontend skeleton, bounded sessions and synthetic fixtures | C6,C7,I.api,I.data,V1,V2,V6,V11 |
| T4 | x | Implement deterministic crisis/consent/approval/share rules with behavioral tests | I.api,V3,V4,V5,V6,V11,V12,V23 |
| T5 | ! | Integrate Meta text inference, schema/evidence validation, provenance and failure path. Implemented and tested against the labelled test double; live inference blocked (B-1) | I.meta,V7,V9,V10,V15,V22 |
| T5b | . | Integrate opt-in DeepSeek Flash 1017 fallback with dedicated provider consent, Meta-outage classification, provenance, session revocation and fail-closed tests. Standalone Go router/API prototype only | C13,I.fallback,V3,V4,V7,V9,V10,V19–V21 |
| T5a | . | Integrate optional Claude verifier with distinct consent, version safety, evidence reference validation and mock/live evidence. Standalone Go adapter prototype exists but no product integration | C12,I.verifier,V3,V4,V7,V9,V10,V16–V18 |
| T6 | x | Deliver bilingual onboarding/story/ledger/next-step/opening/manual path | I.web,V1,V2,V4,V7,V8,V13 |
| T7 | x | Deliver simulated handoff, recipient view, consultation and follow-up | I.web,I.api,V1,V5,V6,V7,V11,V12 |
| T8 | ! | Verify complete journeys, isolation, failure states, accessibility and resource measurements. Journeys, isolation, failure states and accessibility verified locally; live latency/cost measurement blocked (B-1) | C10,I.ops,I.eval,V1,V3,V6,V9,V13,V15 |
| T9 | . | Remove superseded v1/AMD runtime features after acceptance; update CI/env/deployment and demo evidence | C4,C9,C11,V14,V15 |
| T10 | x | Reproducible research corpus pipeline, rights tracking and reviewer search | C15,I.research,V24 |
| T11 | ! | Modal serving app, authentication, scale-to-zero, smoke test and rollback; live verification. Implemented and tested offline; deployment blocked (B-1) | C4,C5,I.meta,V9,V25 |
| T12 | ! | Evaluation harness with pre-registered thresholds (grounding, safety, bilingual, isolation, latency/cost). Offline and test-double suites pass; live-model metrics blocked (B-1) | C10,I.eval,V7,V13,V15 |

T1 evidence: all documentation classified in the inventory; source-reference relocation and Markdown/link/spec checks recorded in Git commit validation. T3, T4, T6, T7 and T10 evidence: the Go, web, Python and browser (e2e) suites on branch `feat/v2-go-meta-runtime`, run locally and in CI (`v2.yml`). `x` means tested locally, not deployed. Blocked tasks name their blocker in [blockers](docs/BLOCKERS.md). Status of T2–T12 and their evidence: [implementation status](docs/IMPLEMENTATION_STATUS.md) and [requirements traceability](docs/REQUIREMENTS_TRACEABILITY.md). T5a and T5b stay open: their standalone prototypes (`prototypes/evidenceguard`, `prototypes/generationfallback`) are mock-tested only and not wired into the v2 runtime. Branch IDs were renumbered when main's DeepSeek fallback (`f4ccc4b`) was merged (C13–C14 → C14–C15, V20–V23 → V22–V25); the cites above use the new IDs ([D-025](docs/DECISION_LOG.md#d-025)).

## §B Bug backpropagation

| id | date | cause | fix |
| --- | --- | --- | --- |
