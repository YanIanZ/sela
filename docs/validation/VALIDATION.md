# Sela acceptance and demo evidence

**v3 frontend (2026-10-28):** local automated and browser evidence is in
[`V3_FRONTEND_VALIDATION.md`](V3_FRONTEND_VALIDATION.md). It is not v2 evidence,
not a live run and not a manual accessibility audit. The v2 matrix below stays
valid for v2 scope only.

Authority: [SPEC](../../SPEC.md) §V, T2–T12. Historical v1 reports do not establish v2 acceptance. Each row names the automated evidence; "live" rows stay pending until a real Modal deployment exists ([B-1](../status/BLOCKERS.md)).

## Acceptance matrix

| Coverage | Required observation | Evidence | Spec |
| --- | --- | --- | --- |
| Complete journey, ID and EN | Story → ledger → next step → share → recipient acknowledgement → scripted consultation → follow-up → revoke, in both languages | `e2e/journey.e2e.mjs` (ID with AI test double; EN manual), `internal/httpapi` `TestFullJourneyOverHTTP` | V1, V2, V13 |
| Crisis fixture | No inference, no handoff/transition/plan; verified, region-labelled help | `TestCrisisStoryInterruptsBeforeAnyModelCallOrHandoff`, `TestCrisisAfterHandoffBlocksRecipientTransitions`, e2e crisis case, eval `safety.model_calls_on_crisis_path` | V3 |
| Safety lexicon | Negation, quotation, slang, code-switching, ambiguity, idioms | `internal/safety` vectors and tests; eval `safety.*` | V3 |
| Consent refusal/withdrawal | Manual path to handoff; canceled or stale AI results never commit | `TestNoAIConsentManualPathReachesFollowUp`, `TestStaleAIResultIsDiscarded`, `TestCancelOperation` | V4, V9, V22 |
| Retention and deletion | Idle/absolute expiry by storage consent; delete purges session, shares, recipients | `TestConsentsAreIndependentAndRetentionWorks`, `TestSessionIdleExpiryPurges`, `TestRevokeExpiryAndDeleteEndRecipientAccess`, store tests, e2e settings case | V6, V23 |
| Edit after approval | Version changes, approval clears, stale share rejected | `TestAIDraftNeedsReviewEditInvalidatesApproval` | V5, V11 |
| Shared projection | Only selected fields; preview equals stored projection | `TestNoAIConsentManualPathReachesFollowUp`, `TestFullJourneyOverHTTP`, e2e EN case | V5, V12 |
| Recipient authorization | Token alone insufficient; wrong code; lockout; uniform errors; separate cookie scope | `TestTokenAloneIsNotEnoughAndLockout`, `TestFullJourneyOverHTTP`, eval `authorization.violations` | V5, V12 |
| Revoked/expired/deleted access | Recipient reads and actions denied; other visitors unaffected | `TestRevokeExpiryAndDeleteEndRecipientAccess`, `TestSessionsAreIsolated`, eval | V6, V11 |
| Model failures | Malformed/ungrounded output rejected; unavailable/timeout honest; manual path | `TestMeaningLockRejectsFabricatedAndUnsafeModelOutput`, `TestMalformedModelOutputFailsHonestly`, `TestProviderUnavailableAndDisabled`, `internal/llm` tests, eval fault battery | V7, V9, V10 |
| Next Step | Deterministic matches, reasons, honest empty state | `TestNextStepsAreDeterministicAndHonestWhenEmpty` | V8 |
| Idempotency and races | One receipt for concurrent duplicates; changed payload conflicts; race detector clean | `TestIdempotentHandoffUnderConcurrency`, `go test -race ./...` | V11 |
| Follow-up | Fixture plan only after scripted completion; once per option | `TestNoAIConsentManualPathReachesFollowUp` | V7 |
| Data boundaries and logs | Unknown fields/IDs rejected; contact details rejected; no story, token or code in logs | `TestSessionCookieAttributesAndStrictJSON`, `TestPracticeTextBoundsAndContactDetails`, log canary checks in `TestFullJourneyOverHTTP` and e2e | V2, V12 |
| Transport security | Same-origin guard, cookie attributes, CSP and headers, rate limits, static path safety | `internal/httpapi` tests | V11, V12, V25 |
| Accessibility and i18n | axe WCAG 2.1 A/AA (no serious/critical), keyboard start, 390 px layout; key parity; no hard-coded strings | e2e, `web/src/test/*.test.ts` | V13 |
| Research corpus | Reproducible build, provenance, licence/access, verified dates, nulls, citation validity | `tests/research/test_corpus_pipeline.py`, `corpus_pipeline.py verify`, eval `retrieval.*` | V24 |
| Live Meta | Real Modal call with pinned model; bilingual wording; latency; cold start; cost | `modal/smoke_test.py`, `go run ./cmd/sela-eval -mode live` — **pending (B-1)** | V9, V10, V15 |
| Runtime cutover | Go/static bundle without AMD, Node server, Redis or PostgreSQL | v2 binary and image have none of these; cutover itself pending (T9) | C6, C7, V14 |

## Repeatable demo

The script is in the [runbook](../operations/RUNBOOK.md#6-demo-script-about-3-minutes). Record commit, fixture version, provider mode and model revision with any recording; never present a test-double run as live Meta inference.

## Documentation baseline verified — 26 October 2026 (T1, before v2 implementation)

Inventory covered all 93 Markdown files; 630 relative links and 29 anchors resolved; spec task/citation checks passed; `git diff --check`, `npm run test:migrations`, `npm run test:infra`, `npm run lint`, `npm run lint:services` and `npm run typecheck` passed. That evidence covered documentation only.

## Optional Claude Sonnet 5.5 verifier acceptance (T5a, NOT YET PRODUCT-IMPLEMENTED)

Mock-network Go adapter tests are in prototypes/evidenceguard (removed after commit e1bf7fc). Beyond adapter tests, the future Go runtime must prove: Anthropic consent declined -> zero outbound calls; crisis fixture -> no Meta or Claude; malformed IDs/output rejected; mismatch means needs human review; timeout/missing key/429 -> explicit unavailable; changed brief or consent revoked mid-flight -> stale report dropped; professional sees only separately approved handoff snapshot; both ID and EN and negation edge cases retain source provenance. Live Sonnet 5.5 request, real usage/latency, and end-to-end session safety remain unverified until T5a/T8 evidence is recorded. A model-supported label is NOT clinical safety validation.

## Opt-in DeepSeek Flash 1017 fallback acceptance (T5b; PRODUCT INTEGRATION PENDING)

Go prototype mock tests in prototypes/generationfallback (removed after commit e1bf7fc) validate Meta-first generation, separate fallback consent, explicit provider attribution, nonfallback on bad/unsafe output, provider request format and fail-closed responses. The future integrated Go service must additionally verify real session-state atomic versioning, cancellation vs revoke races, per-provider secrets/quotas, language fidelity and crisis interruption before any call, end-to-end happy and failure paths, and a real live Meta + optional DeepSeek trace with strictly synthetic fixtures. A mocked fallback test does not prove a deployed integration, clinical validation or competition eligibility.
