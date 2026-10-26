# Sela v2 AI policy

Status: **implemented** in `internal/journey` (pipeline, prompts) and `internal/ledger` (validation); live model behaviour unverified ([B-1](BLOCKERS.md)). Authority: [SPEC](../SPEC.md) C4, V3/V4/V7–V10/V12/V15/V16. V1 multi-provider orchestration is [historical](archive/v1/AI_POLICY.md).

## Allowed work

The Meta model (Muse Glimmer 30B on Modal, [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md)) may:

1. Propose Story Ledger claims: each quotes one story sentence verbatim, has a kind, a short wording in the interface language and qualifier flags.
2. Pick at most one clarification field from Sela's fixed list (concern, duration, daily impact, goal, practical preference); the question text is Sela's own.
3. Draft two or three opening lines from the **approved** brief items only.

It may not decide safety routing, consent, approval, sharing, recipients, slots, matching, plans or any transition, and it never sees other sessions, the professional catalogue or the research corpus.

## Pipeline

Session/version checks → deterministic safety gate (on submission) → explicit AI consent → per-session limit and global admission → bounded streamed request with a JSON Schema constraint and fixed prompts (`journey.PromptVersion`) that mark the user's text as data → strict parse → Meaning Lock → recheck story/ledger/consent versions and crisis state → commit as **unreviewed** items with provenance. Cancellation, timeout, withdrawal or staleness discards the result.

Meaning Lock rejects a claim when its segment is unknown, its quote is not in that segment, a governing negator was cut off, another person's statement is presented as the user's own experience, or it asserts a diagnosis or medication/treatment directive. When only the wording is wrong (negation changed for any shared word, a changed subject, time or certainty marker, invented numbers or quantities, content the quote does not contain, unsafe content, too long) the claim keeps the user's own quote as wording and records why. A hedge such as "not sure" or "belum yakin" is certainty, not negation (D-023). Qualifiers always come from deterministic analysis of the source. Opening lines are dropped when they add content words, numbers, identifiers, diagnoses, medical instructions or pressure language, or reverse the negation of any approved item. Template lines ("Draft without AI") carry the provenance of the item wording they reuse. Every item must be reviewed by the user before approval.

## Truthful modes

- `live_meta`: the configured Modal provider returned the pinned served model name and the output passed validation.
- `test_double`: `cmd/fakellm` in development/CI; refused in production; labelled "TEST DOUBLE — not Meta".
- `manual`: the user's own sentences, no model.
- `recorded_demo`: reserved for a labelled recording of a real live run; none exists, none will be fabricated.

No other AI provider exists in v2. Chain-of-thought/reasoning tokens are discarded. A schema, a mock test or hackathon affiliation does not establish clinical effectiveness or model quality; see [evaluation](EVALUATION.md).

## Usage policies

The Muse Glimmer Usage Policy (mirrors the Llama 4 AUP) prohibits unlicensed medical practice, inferring health information without rights, facilitating self-harm and presenting outputs as human-written, and says the model is not intended for people under 18. Sela's responses: fictional/synthetic data, adult attestation, crisis gate before inference, AI labels on every AI wording, no medical outputs, and disclosed limitations.
