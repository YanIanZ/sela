# Sela v2 — source of truth

Updated: 2026-10-26. Status: requirements established; implementation pending.
Authority: user direction → this spec → current supporting docs → archive. Code/tests prove implementation status; they do not make an undocumented product decision authoritative. External event terms control eligibility.

## §G Goal

Demonstrate a bilingual mental-health care journey: synthetic story → needs clarification → professional discovery → approved brief → simulated handoff/consultation → follow-up. Meta powers the AI steps; Go owns backend behavior. Privacy includes optional anonymity without making it the main product promise.

## §C Constraints

- C1 User-confirmed: backend Go; Swift not selected. Existing TypeScript runtime remains v1 until migration.
- C2 User-confirmed: Indonesian and English across the complete journey.
- C3 User-confirmed: all patient, professional, scheduling, consultation and follow-up data synthetic; no real care transaction.
- C4 User-confirmed: new AI functionality aligned with Meta and its hackathon; remove AMD integration from the v2 runtime. Preserve original authorship/license notices.
- C5 Existing Sela v2 application runtime remains unimplemented. Research/docs and a standalone non-deployed Go verifier prototype may exist; they do not establish a Go API, working Meta integration or finished product.
- C6 Engineering default: browser product, static React/TypeScript frontend built with Vite; one Go process serves API/assets. Change through this spec if implementation evidence warrants.
- C7 Engineering default: session-scoped in-memory demo store; no PostgreSQL/Redis requirement, no production login, no real-data migration.
- C8 Engineering default: adult seeded scenarios; text-first MVP. Optional Meta transcription of synthetic samples comes after verified access/language quality.
- C9 New journey excludes forum, gamification, general chatbot, payments, real booking, real telemedicine, autonomous diagnosis/treatment/prescribing, and model training.
- C10 No performance, deployment, clinical-quality, event-acceptance or live-inference claim without matching evidence.
- C11 Preserve a recoverable v1 baseline and its regression evidence before runtime replacement. Historical snapshots are context, not v2 requirements.
- C12 User-confirmed (26 Oct 2026): add optional Claude Sonnet 5.5 API as a **second-pass source-fidelity verifier**. Meta/Muse Spark remains the sole live *generator* and necessary hackathon contribution; Claude is not a fallback generator, clinician or source of truth. No real patient data in the hackathon. The requirement is documented and an isolated Go prototype is added, but product wiring and live access remain pending.

## §I Interfaces

- I.web: responsive bilingual user journey and clearly labeled fictional professional perspective; [product](docs/PRODUCT.md).
- I.api: planned same-origin `/api/v2` session, fixture, brief, handoff and follow-up endpoints; [contract](docs/API_SPEC.md). Not present in the current runtime.
- I.verifier: optional Anthropic Sonnet 5.5 server-side evidence review **only after separate consent**, using source-bound synthetic excerpts; [contract](docs/CLAUDE_VERIFIER.md). No deployed provider call is verified.
- I.meta: server-side Meta Model API adapter for seeded intake and brief drafting; [integration](docs/META_INTEGRATION.md). Exact endpoint/model/access verified before implementation.
- I.data: versioned synthetic scenario/professional/slot/plan fixtures, bounded per-session state; [data](docs/DEMO_DATA.md).
- I.ops: existing v1 npm commands work now; Go/build/deployment commands documented only when implemented; [development](docs/DEVELOPMENT.md).
- I.docs: documentation index and complete archive inventory; [index](docs/README.md), [inventory](docs/DOCUMENT_INVENTORY.md). Non-normative [Sela Relay research/proposals](docs/research/sela-relay/README.md) do not change requirements or implementation task status.

## §V Invariants

- V1 Every care-data view, profile, request, receipt and plan visibly states synthetic/simulated status in the selected language. No successful real appointment or clinical action is implied.
- V2 Accept only seeded scenario/fixture identifiers and bounded source-backed edits. No real patient registration, arbitrary personal stories, live microphone capture, real contact details or real booking/messages in the MVP.
- V3 Deterministic crisis gate precedes every inference and ordinary handoff transition. Crisis path emits no model call, appointment request, consultation-completed state or follow-up plan.
- V4 AI-processing consent, brief approval and sharing consent are separate, server-enforced, versioned decisions. Declining AI retains the manual synthetic path.
- V5 Editing a brief invalidates approval. Handoff requires the exact approved version and explicit recipient/field selection. Professional views expose only that shared snapshot.
- V6 Reset, expiry and revocation block stale/shared access for that session, including after role switching. Never affect another visitor. Exported files cannot be remotely retracted; MVP has no export requirement.
- V7 Validate schema and source evidence before model output is used. Never invent symptoms, timing, diagnoses, professionals, credentials, slots or treatments. Fictional follow-up plans come only from approved fixtures.
- V8 Match fictional professionals deterministically using explicit preferences. No AI medical triage, diagnosis or clinical suitability score.
- V9 Meta is the only live **generative** provider for intake and brief drafting. Optional Anthropic Sonnet 5.5 performs **verification-only** review and may not replace Meta inference, approve notes or generate treatments. Both keys stay server-side. Provider outages use honest unavailable/manual states; never silent fallback.
- V10 Every **draft** records `live_meta`, `recorded_demo` or `manual` provenance; **verification** records separately identify `live_anthropic`, `unavailable`, `not_requested` or `manual_review` plus model/version, without implying a medically verified result.
- V11 Share/revoke/reset/completion operations are concurrency-safe. Idempotency prevents duplicate simulated requests; stale versions fail explicitly. Admission rate limits also protect expensive model calls.
- V12 No patient/story content, model prompt/output, credentials or shared-note text in operational logs; metadata-only events. Professional role switching is explicitly a demo view, not a production authorization claim.
- V13 Indonesian/English key parity, accessible controls and consistent state/error messages cover the full journey, including failure and revocation states.
- V14 New v2 runtime has no AMD provider, AMD probe or AMD credential requirement. Archived AMD records and original credits remain historical evidence.
- V15 Claims cite observed evidence for the exact runtime/version. Tests with mock inference do not establish live Meta/Anthropic quality, model access, clinical efficacy or hackathon eligibility.
- V16 Anthropic verification requires its **own explicit server-enforced consent**, separate from Meta AI processing, brief approval and sharing. Only bounded synthetic versioned evidence excerpts and candidate claims may leave the Go backend for Anthropic. Declined/withdrawn consent causes zero Anthropic calls.
- V17 Verification is advisory: compare each claim to an actual allowed source excerpt; reject malformed/unknown claim IDs and citations; preserve unsupported/contradicted/uncertain states. A Claude agreement cannot bypass deterministic evidence checks, human review, crisis interruption, sharing approval or fixture-authored care. Show provider provenance honestly.
- V18 Reset, expiry, changed brief, consent withdrawal or crisis fixture invalidates and/or cancels in-flight verifier work; no stale report may authorize sharing. Verifier errors are explicitly unavailable; never silently replace another provider or claim success.

## §T Tasks

| id | status | task | cites |
| --- | --- | --- | --- |
| T1 | x | Establish source of truth; refactor active docs; inventory/archive v1; validate references | C5,I.docs,V15 |
| T2 | . | Measure v1 baseline; verify Go toolchain and Meta access; pin versions; retain rollback point | C1,C10,C11,I.ops,V15 |
| T3 | . | Create Go API/static frontend skeleton, bounded sessions and synthetic fixtures | C6,C7,I.api,I.data,V1,V2,V6,V11 |
| T4 | . | Implement deterministic crisis/consent/approval/share rules with behavioral tests | I.api,V3,V4,V5,V6,V11,V12 |
| T5 | . | Integrate Meta text inference, schema/evidence validation, provenance and failure path | I.meta,V7,V9,V10,V15 |
| T5a | . | Integrate optional Claude verifier with distinct consent, version safety, evidence reference validation and mock/live evidence. Standalone Go adapter prototype exists but no product integration | C12,I.verifier,V3,V4,V7,V9,V10,V16–V18 |
| T6 | . | Deliver bilingual intake/discovery/brief/manual path | I.web,V1,V2,V4,V7,V8,V13 |
| T7 | . | Deliver simulated handoff, professional perspective, consultation and follow-up | I.web,I.api,V1,V5,V6,V7,V11,V12 |
| T8 | . | Verify complete journeys, isolation, failure states, accessibility and resource measurements | C10,I.ops,V1,V3,V6,V9,V13,V15 |
| T9 | . | Remove superseded v1/AMD runtime features after acceptance; update CI/env/deployment and demo evidence | C4,C9,C11,V14,V15 |

T1 evidence: all documentation classified in the inventory; source-reference relocation and Markdown/link/spec checks recorded in Git commit validation. Runtime tasks remain pending; no Go commands are advertised as implemented.

## §B Bug backpropagation

| id | date | cause | fix |
| --- | --- | --- | --- |
