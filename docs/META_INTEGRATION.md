# Meta integration

Status: planned; no v2 adapter or live call is implemented. Authority: [SPEC](../SPEC.md) I.meta, C4/C8, V7/V9/V10/V15.

## Official references

Checked 26 October 2026: [Meta cookbook](https://dev.meta.ai/docs/cookbook) documents chat completions, streaming, tool calling and structured outputs. These are candidate mechanisms, not verified Sela integrations. The [official event page](https://cerebralvalley.ai/e/meta-ai-developer-hackathon) lists the event's model/tool ecosystem. Confirm actual account access and current model documentation before choosing an identifier.

## MVP integration

Use one server-side Meta text adapter for synthetic intake/consultation drafts. The Go application owns consent, crisis checks, matching, validation and transitions. Model output never grants application authority. Structured output is validated again locally; provider schema compliance alone is insufficient.

Do not invent a base URL, model ID, token format or SDK requirement. T2 verifies the account-supported endpoint/authentication/model and records non-sensitive probe evidence. Proposed configuration names are `META_API_KEY`, `META_MODEL` and optional verified endpoint configuration; they are design names only, not current `.env.example` variables. Provider credentials never enter the browser.

Bound payload, output, operation duration, retries and concurrent calls. Retry only documented transient errors within the operation budget; do not retry denied consent or authentication failures. Propagate cancellation and discard responses after reset/version change. Expose stable app error codes rather than raw provider responses.

## Evidence and optional scope

Record model ID, request/operation ID, latency and validation outcome, excluding story/prompt/output text. Keep mock contract tests separate from live synthetic probes. A live demo must show actual Meta provenance; unavailable access must remain visibly unavailable.

Synthetic-sample transcription is optional after the text journey works and access plus ID/EN quality are checked. No speech synthesis, image generation, segmentation or fine-tuning is required for the care-journey MVP. Add a capability only when it serves the journey and is documented/tested; using every advertised model is not a requirement.

Event eligibility and model-use terms still require separate verification; see [event notes](META_EVENT_NOTES.md).
