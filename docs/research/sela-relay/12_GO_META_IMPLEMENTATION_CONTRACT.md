# Go + Meta: implementation contract and migration strategy

**Status: PROPOSED technical design, NOT IMPLEMENTED.** [SPEC.md](../../../SPEC.md) C1–C11 and V1–V15 control actual requirements. Implement changes in a subsequent code PR with measured evidence and owner review.

## Target stack

- **Backend:** one Go HTTP service using net/http, context cancellation, typed domain state, in-memory isolated demo store and explicit application-layer authorization. Pin toolchain and dependencies after verification; do not pretend that current Go files exist.
- **Frontend:** new static React + TypeScript + Vite application, locale dictionaries for ID/EN, accessible mobile-first layouts and API-generated state. Do not port old Next.js server routes.
- **AI:** Meta Model API accessed from a Go server-side adapter after Meta AI consent, with an optional separately consented Claude Sonnet 5.5 **source-fidelity verifier** (see [current contract](../../CLAUDE_VERIFIER.md)); Claude is not a generative fallback. Official cookbook currently documents schema-constrained structured output and function/tool calling; verify the account's allowed model and endpoint with a real, non-sensitive probe before fixing runtime configuration.
- **Data:** versioned *synthetic* scenario, evidence, fictional-professional, slot, receipt, consultation and follow-up fixtures. No real patient database or live clinician marketplace.

Suggested layout, subject to implementation:
~~~text
cmd/sela/                  entrypoint; static assets and HTTP server
internal/domain/           state types, consent and version invariants
internal/application/      use cases and deterministic transitions
internal/httpapi/          JSON handlers, middleware and stable errors
internal/meta/             Meta request/response adapter, budgets
internal/validation/       source-backed claim and translation checks
internal/store/            isolated bounded session snapshots
internal/fixtures/         versioned synthetic catalog
web/                       React + TypeScript + Vite, ID/EN dictionaries
e2e/                       synthetic journey and accessibility checks
~~~

Avoid premature microservices, Redis, PostgreSQL, queues and message brokers for an isolated ten-day demo. Keep interfaces adaptable for a later real service, without implementing medical production infrastructure.

## Meaning Lock: Meta may propose; Go decides

1. Deterministic gate runs **before any** inference or handoff transition. The crisis fixture goes straight to an interruption route; it does not call Meta or create a consultation receipt. Tests of the gate are not a validated clinical crisis classifier.
2. Consent is separate from approval and from sharing. The request contains only whitelisted fixture/evidence IDs and minimum synthetic text.
3. Meta generates structured candidate claims with *evidence IDs, exact excerpt links, proposed paraphrase and optional clarification*. Prompts cannot create new source IDs, providers or slots.
4. Go validates JSON shape, evidence membership, excerpt span, participant, negation, time, uncertainty, ID/EN rendering and restricted content. Deterministic validation catches known classes; difficult semantic cases require user inspection and human review.
5. Unsupported candidates are rejected, not quietly corrected through another autonomous agent. A manual source-backed path remains available.
6. User edits create a new brief version and revoke former approval. Only the exact approved version can be shared with an explicitly selected fictional recipient and fields.
7. All model results carry a verifiable provenance enum: **live_meta**, **recorded_demo** or **manual**. Recorded output never masquerades as live.

### Structured response sketch (illustrative, not a final Meta schema)

~~~json
{
  "fixture_version": "adult-id-v1",
  "claims": [
    {
      "evidence_id": "e1",
      "type": "reported_experience",
      "language": "id",
      "proposed_text": "Saya merasa kewalahan ketika kuliah.",
      "uncertain": false
    }
  ],
  "clarification_candidate": null
}
~~~

Only backend fixture lookup decides whether **e1** exists and what it actually says. A valid JSON response is not evidence of factual validity. Any tool call is read-only and allowlisted (e.g., get synthetic source spans or the fictional catalog). Tools must not share, approve, book or complete care autonomously.

## Safety and privacy invariants

- No actual medical decision, diagnosis, risk score, psychiatric triage, prescription or live emergency handling by the model.
- Unknown or out-of-fixture free text is rejected in the demo, preventing accidental acceptance of real private histories.
- Cookie-based opaque sessions and server-enforced same-origin mutations; independent per-session memory, bounded TTL and store capacity.
- Share projection is immutable, recipient-specific and field-minimized. Role switch is **demonstration only**, not real authorization. No cross-session projection is addressable.
- Reset, expiry and revoke invalidate all read access and discard in-flight stale model results, including on race with a share operation. They cannot retract already-seen information.
- Strict idempotency for simulated handoff; optimistic version checks and atomic transitions for all mutations.
- Provider API key only in server environment. Metadata-only logs exclude full narrative, selected fields, model prompt/output and credential values.
- Model timeouts, cancellation, concurrency limits, rate limits and response-size bounds must be configured and tested before live deployment. Make no specific resource or clinical quality claims without measurements.
- For Meta API settings, distinguish Standard from any mode where submitted content may be used to improve models; synthetic inputs are required regardless of selected mode. Check current provider data-use terms.

## State machine and contract gates

Current planned endpoints are listed in [API_SPEC.md](../../API_SPEC.md). Do not bypass that versioned contract. Adding proposed clarification/decline/expire statuses requires explicit acceptance and concurrent edits to SPEC, API documentation, fixture schemas and tests.

**Assertions to implement as software tests:**
- Decline AI → no Meta request, manual synthetic journey succeeds.
- Crisis fixture → zero Meta calls, zero simulated handoff and zero follow-up.
- Edited approved brief → approval invalidated; prior version fails to share with HTTP 409.
- Requested handoff repeated with same idempotency key/payload → one receipt only.
- Same key/different payload or stale version → conflict without side effect.
- Revoke share and switch to professional view → former projection denied.
- Reset/expiry with in-flight response → no resurrected draft or grant.
- Two demo browser sessions → no access to each other's data.
- Language switch → same fixture facts, quote provenance and version discipline.
- Meta access outage or unsupported structured output → explicit unavailable or manual state, never unannounced third-party fallback.

## Actual Meta integration: engineering evidence

Official docs: https://dev.meta.ai/docs/overview , https://dev.meta.ai/docs/cookbook/structured-output , https://dev.meta.ai/docs/cookbook/tool-function-calling .

The documented example base URL and model names are *candidates*, not proof of API entitlement. Log non-sensitive verification metadata separately: date, account-supported model identifier, endpoint/protocol, result code, request latency, validation status and rate-limit behavior. Do not commit tokens, raw prompts or patient information.

Implement exactly one purposeful live path first (source-linked draft). A second Meta operation, such as bilingual semantic-diff suggestion, is justified only if it improves held-out fidelity; it cannot approve its own claims. A multi-agent spectacle that makes decisions without guardrails is not an evaluation advantage. The optional Sonnet reviewer never grants sharing permission and must not be mistaken for clinical truth.

## Rewrite order and rollback

0. Keep the existing v1 reference and license/provenance; capture runnable baseline. The active code is still Next.js/TypeScript with AMD-era routes, scripts and providers.
1. Stand up bounded Go sessions, synthetic fixtures, static frontend skeleton and predictable ID/EN state rendering.
2. Implement deterministic consent, crisis, brief-version, snapshot, handoff, revoke and isolation rules **before** adding inference.
3. Add real Meta adapter only after source-backed validation exists. Verify live call on synthetic fixture and preserve honest unavailable/manual states.
4. Implement one complete user + fictional professional vertical slice. Then add two negative routes (revoke and crisis) and remaining language parity.
5. Capture repeatable tests, CI evidence, accessibility checks and live-demo recordings before retiring v1 runtime or AMD dependencies. Do not delete historical authored contributions or tests without preserving their behavioral guarantees.
6. Commit a separate migration PR: Go/runtime, frontend and CI/deploy changes atomically, with working rollback instructions. No speculative performance claim.

**Current status:** this document does not claim a Go build, deployed v2 UI, live Meta response or measured efficiency.