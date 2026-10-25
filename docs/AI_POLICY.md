# Sela v2 AI policy

Status: target contract. Authority: [SPEC](../SPEC.md) C4, V3/V4/V7–V10/V12/V15. V1 multi-provider orchestration is [historical](archive/v1/AI_POLICY.md).

## Allowed work

Meta may structure seeded intake into descriptive concerns, daily impact, stated timeline and consultation questions. Every retained factual claim must be backed by the selected scenario's evidence. Missing information remains unknown. The model may suggest a clarification from allowed scenario questions; it cannot introduce new patient facts.

The model does not select crisis urgency, invent professionals or slots, decide a diagnosis, issue medication instructions, complete appointments, grant consent, approve notes, choose shared fields or modify a fictional professional's plan. Those transitions are deterministic application behavior under explicit user actions.

## Pipeline

Validate fixture/session/version → deterministic crisis gate → explicit AI consent → quota/concurrency admission → bounded Meta request → schema/evidence validation → recheck live session/version/consent → store a draft with provenance. Cancellation or staleness discards the result and releases admission slots.

Treat all provider output as untrusted. Reject malformed JSON, unknown fields, unsupported claims, unsafe instructions, forged fixture IDs and output outside the schema. Do not execute model-supplied URLs or side effects. Function/tool calls, if introduced, need an allowlisted read-only catalog contract and the same server-side checks; they are not required for the first text integration.

## Truthful modes

- `live_meta`: a successful real Meta API request with verified model ID, followed by validation.
- `recorded_demo`: a prerecorded synthetic result explicitly labeled recorded/simulated.
- `manual`: allowed synthetic facts selected without a model call.

No AMD, DeepSeek, Gemini or OpenRouter fallback is used by the new journey. A missing Meta key, access error, timeout or validation failure produces a localized unavailable state and manual path. Never relabel a recorded fixture as live Meta output.

A model schema, mock test or hackathon affiliation does not establish clinical effectiveness. Access, endpoint, model capabilities and supported languages must be verified from the current official docs and reproducible calls. See [Meta integration](META_INTEGRATION.md).
