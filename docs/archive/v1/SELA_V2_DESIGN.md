> ARCHIVED V1 — inherited implementation or superseded proposal, not Sela v2 requirements. [Current source of truth](../../../SPEC.md) · [Documentation index](../../README.md). Snapshot: `d5da323`. Original claims and dates below are historical.

# Sela v2 — proposed care-journey demonstration

Status: draft for user review, 26 October 2026. This document authorizes no implementation or production service.

## Confirmed direction

The user wants a major refactor aligned with the Meta Global AI Developer Hackathon. Sela should support a journey toward professional or medical care, with Indonesian and English experiences. Anonymity is a privacy option rather than the main positioning. All patient, professional, appointment, consultation and follow-up data in the hackathon journey will be synthetic.

The user is considering Go or Swift; neither has been selected. The event and IP notes remain in [Meta event notes](../../architecture/META_EVENT_NOTES.md) and [provenance](../../product/SELA_PROVENANCE.md).

## Product proposal

Sela helps a person explain what is happening, find suitable professional support, prepare for the conversation, and follow a professional-authored plan. The hackathon demonstrates both the person's experience and a fictional professional's receiving view. It provides no actual appointments, consultations, diagnoses, prescriptions, or medical treatment.

Primary demo is browser-based and responsive. Every page that shows synthetic care data has a visible demo label. All professional profiles, availability, requests and care notes are explicitly marked fictional or simulated. No real provider or patient identifiers, external booking endpoints, email, calendar invites or messages are used.

## Main journey

1. **Choose a synthetic story.** Load an adult persona and a seeded situation. Show language choice and explain the demo. Text intake uses seeded scenarios; arbitrary personal stories and live microphone capture are outside this first demo.
2. **Understand the story.** With AI consent, send the seeded text to Meta and ask for structured, descriptive concerns, daily impact and clarification questions. Keep evidence links to the source. No diagnosis or treatment decisions.
3. **Explore professional support.** Display fictional professionals filtered by explicit user preferences for service type, language, location or remote service, cost and availability. Filtering uses fixture data; the model does not invent professionals, credentials or slots.
4. **Prepare a consultation brief.** Create a draft with source-grounded concerns, timeline, impact and questions. The user can edit it and explicitly approve a snapshot; edits invalidate prior approval. AI generation and sharing have separate consent.
5. **Simulate a request.** The user chooses a fictional slot and separately authorizes which synthetic identity fields and approved notes are included. Show a simulated request and receipt, with no implication of an actual appointment.
6. **Inspect the professional view.** A clearly labeled demo role switch opens the fictional professional inbox. It sees only the approved shared snapshot, not the private journal or full conversation. A scripted action simulates acknowledging the request and completing a consultation. This is a demo perspective, not production professional authentication.
7. **Follow up.** Show a predefined, fictional professional-authored plan, a check-in and a return-visit timeline. The AI may restate approved plan text but cannot author medication instructions or change the plan.

Identity remains optional during private reflection. The handoff explicitly previews the selected synthetic profile and information to be shared.

## Demonstration cases

- A fictional adult struggling with work stress completes the full journey.
- A fictional adult uses the same journey in English; the matching rules and data boundaries remain consistent.
- A seeded crisis example bypasses ordinary matching and model calls and shows an immediate-support pathway. Existing verified Indonesia-specific help information can be retained and clearly labeled by geography; no emergency response or referral is simulated as completed.
- Declined AI consent, unavailable model access and missing professional availability have explicit alternate states. Declining AI allows manual notes from the seeded story and the same approval/share flow.

## Proposed architecture — not yet approved

Builder recommendation: Go HTTP API plus a React frontend built with Vite and served as static assets. The Go executable can serve both API and frontend; a Node.js runtime would not be required in production for this proposed design. Swift remains an alternative if the user chooses it. No performance improvement is claimed before measurement.

Use one backend process, session-scoped in-memory synthetic state and deterministic fixtures for this demo. No PostgreSQL or Redis dependency is required for the first demonstration. Use concurrency-safe state, bounded sessions and a reset operation scoped to the current session. Resetting one public visitor's demo must not clear another visitor's state. No migration of real user data or production authentication is part of this scope.

Keep frontend/API contracts independent of the inference provider. Meta is the only live AI provider in the new journey; keys remain server-side. Replace the AMD-specific pipeline, probes, environment keys and active submission instructions in the v2 runtime after behavior has been covered. Retain original license notices and historical contribution attribution.

Before replacing the existing runtime, retain a recoverable Git baseline and migrate one complete vertical slice. Existing legacy routes remain available until the replacement journey passes its acceptance checks; final route removal is part of the approved implementation plan.

## Meta integration proposal

Use Meta Model API for conversational intake and structured consultation drafts. Validate output against the schema and source text before display or sharing. Bound latency, output size, retries and concurrent calls; propagate cancellation. Model identifiers and API credentials must come from verified access and current documentation rather than assumptions.

Text is the first required live integration. Muse Voice Transcribe is an optional later slice using a clearly identified synthetic sample recording, only if access and Indonesian/English quality are verified. No voice synthesis provider is selected.

Label each run's provenance. A missing key or failed API request shows an honest unavailable state and manual seeded-note path. Any prerecorded or fixture-based model demonstration is visibly marked as recorded/simulated and never reported as live Meta inference.

Official references: [Meta cookbook](https://dev.meta.ai/docs/cookbook), [event page](https://cerebralvalley.ai/e/meta-ai-developer-hackathon). These establish product capabilities, not clinical suitability or final event eligibility.

## Scope boundaries

The v2 demo prioritizes intake, approved handoff, fictional professional review and follow-up. Forum, gamification, general-purpose chat, training/fine-tuning, real provider onboarding, payments, real booking, telemedicine and medical records integrations are outside the first new journey. The final feature removal list belongs in the implementation plan; no existing features are deleted by this draft.

## Acceptance evidence

- Complete the main synthetic journey in Indonesian and English with consistent translations.
- Demonstrate the fictional professional receiving only the explicitly shared, approved snapshot.
- Prove no real booking/message/clinical-service request is emitted.
- Test consent refusal, edit-after-approval, revoked sharing, expired sessions, duplicate submission and model timeout/malformed output. Revocation removes app access to the shared snapshot; it cannot retract a file already exported.
- Shared snapshot records its approved version, selected recipient, shared fields and consent timestamp. Repeated submission of the same request does not create duplicate appointments.
- Run the seeded crisis case before inference and ordinary appointment flow.
- Validate matching against fixed professional and slot fixtures.
- Record reproducible startup, idle/active memory, frontend size and request timings before claiming a lighter runtime. Compare equivalent workloads and distinguish model latency from backend work.
- Run tests, bilingual checks and production build; keep a short repeatable demo script with synthetic inputs and explicit live/simulated labels.

## Decision needed before implementation

Confirm or change the proposed Go + static React/Vite web architecture. The care-journey and professional-view details above are design proposals for review, not additional user-approved requirements.
