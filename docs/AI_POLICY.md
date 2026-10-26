# Sela v2 AI policy

Status: **implemented** in `internal/journey` (pipeline, prompts) and `internal/ledger` (validation); live model behaviour unverified ([B-1](BLOCKERS.md)). Authority: [SPEC](../SPEC.md) C4/C13, V3/V4/V7–V10/V12/V15–V22. V1 multi-provider orchestration is [historical](archive/v1/AI_POLICY.md).

## Allowed work

Meta/Muse Spark is the **primary** live draft generator. User-confirmed optional DeepSeek-V4.1-Flash-1017 is a **separately consented availability fallback** only for eligible provider outage. Optional Anthropic Claude Sonnet 5.5 is a **separate source-fidelity reviewer**, never a fallback generator. See [DeepSeek fallback](DEEPSEEK_FALLBACK.md) and [Claude verifier](CLAUDE_VERIFIER.md). See [verifier contract](CLAUDE_VERIFIER.md).

Implementation status: in the v2 runtime the Meta model is the only generator that runs, and it is Muse Glimmer 30B on Modal, not Muse Spark; the Muse Spark naming above awaits the owner's decision (D-024, [B-7](BLOCKERS.md)). The DeepSeek fallback (C13) exists only as the standalone, default-OFF prototype `prototypes/generationfallback`, and the Claude verifier (C12) only as the standalone prototype `prototypes/evidenceguard`; neither is wired into the v2 runtime.

The Meta model (Muse Glimmer 30B on Modal, [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md)) may:

1. Propose Story Ledger claims: each quotes one story sentence verbatim, has a kind, a short wording in the interface language and qualifier flags.
2. Pick at most one clarification field from Sela's fixed list (concern, duration, daily impact, goal, practical preference); the question text is Sela's own.
3. Draft two or three opening lines from the **approved** brief items only.

It may not decide safety routing, consent, approval, sharing, recipients, slots, matching, plans or any transition, and it never sees other sessions, the professional catalogue or the research corpus.

## Pipeline

Session/version checks → deterministic safety gate (on submission) → explicit AI consent → per-session limit and global admission → bounded streamed request with a JSON Schema constraint and fixed prompts (`journey.PromptVersion`) that mark the user's text as data → strict parse → Meaning Lock → recheck story/ledger/consent versions and crisis state → commit as **unreviewed** items with provenance. Cancellation, timeout, withdrawal or staleness discards the result. When the optional verifier (T5a) is wired in, its review runs after Meaning Lock under its own consent, both consents are rechecked before commit, and a failed or unavailable verification is shown as such, never as a pass.

Target pipeline once the optional providers are wired in (T5a, T5b; not in the v2 runtime yet): Validate fixture/session/version → deterministic crisis gate → explicit Meta AI consent → quota/concurrency admission → bounded Meta request → if eligible Meta service failure and separately consented, bounded DeepSeek fallback → schema/evidence validation → if separately opted in, bounded Claude source-fidelity review → recheck session/version/**every relevant provider** consent → store a draft and optional verifier report with separate provenance. Any failed verification is shown as unavailable/needs-review; do not turn it into a fake pass. Cancellation or staleness discards the result and releases admission slots.

Meaning Lock rejects a claim when its segment is unknown, its quote is not in that segment, a governing negator was cut off, another person's statement is presented as the user's own experience, or it asserts a diagnosis or medication/treatment directive. When only the wording is wrong (negation changed for any shared word, a changed subject, time or certainty marker, invented numbers or quantities, content the quote does not contain, unsafe content, too long) the claim keeps the user's own quote as wording and records why. A hedge such as "not sure" or "belum yakin" is certainty, not negation (D-023). Qualifiers always come from deterministic analysis of the source. Opening lines are dropped when they add content words, numbers, identifiers, diagnoses, medical instructions or pressure language, or reverse the negation of any approved item. Template lines ("Draft without AI") carry the provenance of the item wording they reuse. Every item must be reviewed by the user before approval.

## Truthful modes

- `live_meta`: the configured Modal provider returned the pinned served model name and the output passed validation.
- `live_deepseek_fallback`: actual DeepSeek Flash generation following a qualifying Meta outage, explicitly labeled separately from Meta and followed by validation. Not emitted by the v2 runtime, which has no DeepSeek route (T5b open).
- `test_double`: `cmd/fakellm` in development/CI; refused in production; labelled "TEST DOUBLE — not Meta".
- `manual`: the user's own sentences, no model.
- `recorded_demo`: reserved for a labelled recording of a real live run; none exists, none will be fabricated.

No AMD, Gemini or OpenRouter fallback is used by the new journey. DeepSeek fallback exists ONLY if enabled and separately consented for an eligible Meta availability error. Claude is an optional verifier, never a fallback generator. A Meta bad/unsafe output must fail closed without another model. Missing keys, rejection or timeout without a valid fallback route produce a localized unavailable state and manual path. Never relabel a recorded fixture as live Meta output.

Implementation status: the v2 runtime calls only the configured Meta model. The DeepSeek fallback is a standalone, default-OFF prototype (`prototypes/generationfallback`), not wired in, so today a Meta outage always yields the unavailable state and manual path. Chain-of-thought/reasoning tokens are discarded. A schema, a mock test or hackathon affiliation does not establish clinical effectiveness or model quality; see [evaluation](EVALUATION.md).

## Usage policies

The Muse Glimmer Usage Policy (mirrors the Llama 4 AUP) prohibits unlicensed medical practice, inferring health information without rights, facilitating self-harm and presenting outputs as human-written, and says the model is not intended for people under 18. Sela's responses: fictional/synthetic data, adult attestation, crisis gate before inference, AI labels on every AI wording, no medical outputs, and disclosed limitations.

Claude is an optional verifier rather than a fallback, and its outcome records independent provider/mode metadata; no Claude-generated diagnosis, medical advice or permission grant is allowed.

## Optional Claude verifier and data handling

Per-provider consent is explicit: consenting to Meta does **not** automatically authorize sending fixture excerpts to Anthropic. The research registry of 7,580 Europe PMC candidates is bibliographic discovery data, not validated facts or free-to-train copyrighted text. A model agreement is not independent scientific or clinical confirmation. Both providers process synthetic demo content only; retain no prompts, narratives, claims or result text in operational logs. Display the verification provider's name and an honest `model_supported`/`needs_human_review`/`verification_unavailable` outcome. Unavailable verification never silently becomes a verified badge. See [EvidenceGuard](CLAUDE_VERIFIER.md).

## DeepSeek Flash 1017 (availability fallback)

Use `deepseek-flash` against the verified official DeepSeek endpoint after explicit **DeepSeek-specific** consent (independent of Meta/Claude/sharing). Default OFF for competition demo; preserve a runnable Meta-only mode. The model may generate the same source-linked schema, but the deterministic crisis gate, source validator, briefing approval and recipient consent remain unchanged. Only eligible Meta availability errors allow switching. Expose `live_deepseek_fallback` distinctly in UI and sanitized logs, with no implicit authorization granted by either generator. Session version/consent must be rechecked after awaiting both providers. No model gets real patient data in this prototype. See [fallback contract](DEEPSEEK_FALLBACK.md).
