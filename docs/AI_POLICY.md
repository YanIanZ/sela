# Sela v2 AI policy

Status: target contract. Authority: [SPEC](../SPEC.md) C4/C13, V3/V4/V7–V10/V12/V15–V21. V1 multi-provider orchestration is [historical](archive/v1/AI_POLICY.md).

## Allowed work

Meta/Muse Spark is the **primary** live draft generator. User-confirmed optional DeepSeek-V4.1-Flash-1017 is a **separately consented availability fallback** only for eligible provider outage. Optional Anthropic Claude Sonnet 5.5 is a **separate source-fidelity reviewer**, never a fallback generator. See [DeepSeek fallback](DEEPSEEK_FALLBACK.md) and [Claude verifier](CLAUDE_VERIFIER.md). See [verifier contract](CLAUDE_VERIFIER.md).

Meta may structure seeded intake into descriptive concerns, daily impact, stated timeline and consultation questions. Every retained factual claim must be backed by the selected scenario's evidence. Missing information remains unknown. The model may suggest a clarification from allowed scenario questions; it cannot introduce new patient facts.

The model does not select crisis urgency, invent professionals or slots, decide a diagnosis, issue medication instructions, complete appointments, grant consent, approve notes, choose shared fields or modify a fictional professional's plan. Those transitions are deterministic application behavior under explicit user actions.

## Pipeline

Validate fixture/session/version → deterministic crisis gate → explicit Meta AI consent → quota/concurrency admission → bounded Meta request → if eligible Meta service failure and separately consented, bounded DeepSeek fallback → schema/evidence validation → if separately opted in, bounded Claude source-fidelity review → recheck session/version/**every relevant provider** consent → store a draft and optional verifier report with separate provenance. Any failed verification is shown as unavailable/needs-review; do not turn it into a fake pass. Cancellation or staleness discards the result and releases admission slots.

Treat all provider output as untrusted. Reject malformed JSON, unknown fields, unsupported claims, unsafe instructions, forged fixture IDs and output outside the schema. Do not execute model-supplied URLs or side effects. Function/tool calls, if introduced, need an allowlisted read-only catalog contract and the same server-side checks; they are not required for the first text integration.

## Truthful modes

- `live_meta`: a successful real Meta API request with verified model ID, followed by validation.
- `live_deepseek_fallback`: actual DeepSeek Flash generation following a qualifying Meta outage, explicitly labeled separately from Meta and followed by validation.
- `recorded_demo`: a prerecorded synthetic result explicitly labeled recorded/simulated.
- `manual`: allowed synthetic facts selected without a model call.

No AMD, Gemini or OpenRouter fallback is used by the new journey. DeepSeek fallback exists ONLY if enabled and separately consented for an eligible Meta availability error. Claude is an optional verifier, never a fallback generator. A Meta bad/unsafe output must fail closed without another model. Missing keys, rejection or timeout without a valid fallback route produce a localized unavailable state and manual path. Never relabel a recorded fixture as live Meta output.

A model schema, mock test or hackathon affiliation does not establish clinical effectiveness. Access, endpoint, model capabilities and supported languages must be verified from the current official docs and reproducible calls. See [Meta integration](META_INTEGRATION.md).

## Optional Claude verifier and data handling

Per-provider consent is explicit: consenting to Meta does **not** automatically authorize sending fixture excerpts to Anthropic. The research registry of 7,580 Europe PMC candidates is bibliographic discovery data, not validated facts or free-to-train copyrighted text. A model agreement is not independent scientific or clinical confirmation. Both providers process synthetic demo content only; retain no prompts, narratives, claims or result text in operational logs. Display the verification provider's name and an honest `model_supported`/`needs_human_review`/`verification_unavailable` outcome. Unavailable verification never silently becomes a verified badge. See [EvidenceGuard](CLAUDE_VERIFIER.md).

## DeepSeek Flash 1017 (availability fallback)

Use `deepseek-flash` against the verified official DeepSeek endpoint after explicit **DeepSeek-specific** consent (independent of Meta/Claude/sharing). Default OFF for competition demo; preserve a runnable Meta-only mode. The model may generate the same source-linked schema, but the deterministic crisis gate, source validator, briefing approval and recipient consent remain unchanged. Only eligible Meta availability errors allow switching. Expose `live_deepseek_fallback` distinctly in UI and sanitized logs, with no implicit authorization granted by either generator. Session version/consent must be rechecked after awaiting both providers. No model gets real patient data in this prototype. See [fallback contract](DEEPSEEK_FALLBACK.md).
