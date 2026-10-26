# Requirements traceability

Updated: 2026-10-26. Maps every in-scope requirement of the v2 implementation brief to code, tests and evidence. Normative wording lives in [SPEC.md](../SPEC.md); decisions in the [decision log](DECISION_LOG.md); open external dependencies in [blockers](BLOCKERS.md); latest numbers in [implementation status](IMPLEMENTATION_STATUS.md).

**Status values.** `planned` (not built) · `implemented` (code exists, no automated proof) · `tested locally` (automated tests or evaluation pass in this repository; CI runs the same commands) · `verified in deployment` (observed on a deployed environment) · `blocked externally` (needs an owner action, budget or account; see [blockers](BLOCKERS.md)). Nothing is deployed yet, so no row is `verified in deployment`. "Test double" means `cmd/fakellm`, which is never Meta inference.

Test names are Go tests unless prefixed: `web:` = Vitest in `web/src/test`, `e2e:` = `e2e/journey.e2e.mjs`, `eval:` = `cmd/sela-eval` metric, `py:` = Python unittest.

## A. Onboarding, consent and data

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| A1 | Calm, accessible, mobile-first start | `web/src/pages/Welcome.tsx`, `Start.tsx`, `styles.css` | e2e: public pages axe WCAG 2.1 A/AA = 0 violations, 390 px no horizontal scroll; e2e: keyboard-only start | tested locally |
| A2 | Indonesian and English everywhere | `web/src/i18n/{id,en}.ts`, `internal/httpapi/errors.go` | web: `i18n.test.ts` (key parity, placeholders, error codes), `hardcoded.test.ts`; `TestLocalizedErrorsAndSessionMissing`; e2e ID and EN journeys | tested locally |
| A3 | Anonymous or pseudonym; no real identity | `journey/session.go` (`validIdentity`), `Settings.tsx` | `TestCreateSessionValidation` (journey) | tested locally |
| A4 | Honest data explanation | `fixtures/data/policies.json`, `Privacy.tsx`, `Start.tsx` | Policies versioned and validated at load (`fixtures` tests); e2e start flow | tested locally |
| A5 | Separate consents for AI processing, storage and sharing | `journey/session.go` (`SetConsent`), `handoff.go` (share consent + policy version) | `TestConsentsAreIndependentAndRetentionWorks`, `TestShareRequiresExplicitConsentAndCurrentPolicy`, `TestNoAIConsentManualPathReachesFollowUp` | tested locally |
| A6 | Working retention and deletion | `journey/store.go`, `store/memory.go` (sweeper, idle/absolute expiry), `DeleteSession` | `TestSessionIdleExpiryPurges`, `TestRevokeExpiryAndDeleteEndRecipientAccess`, store tests, e2e settings deletion | tested locally |
| A7 | Synthetic adult demo only (adult attestation, fictional practice text) | `CreateSession` (`AdultAttested`), `SubmitStory` practice bounds, D-003 | `TestCreateSessionValidation`, `TestPracticeTextBoundsAndContactDetails`, e2e contact-detail rejection | tested locally |

## B. Story room

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| B1 | Text input (sample scenarios or fictional practice story) | `journey/story.go`, `Story.tsx` | e2e journeys; `TestPracticeTextBoundsAndContactDetails` | tested locally |
| B2 | Streaming, cancel, retry, timeout, honest errors | `journey/ops.go`, `httpapi/sse.go`, `web/src/lib/operation.ts`, `Ledger.tsx` | `TestFullJourneyOverHTTP` (SSE replay with Last-Event-ID), `TestCancelOperation`, `TestStaleAIResultIsDiscarded`, `TestProviderUnavailableAndDisabled`, `TestModalColdStartRetries`; eval fault battery (timeout, stall, 503/429/401, truncated) | tested locally (test double) |
| B3 | No emotional dependence; no chatbot persona | No chat surface; copy in i18n; prompts (`journey/prompts.go`) | web: `claims.test.ts` (forbidden-claim gates incl. therapy/counselling) | tested locally (see note in status) |
| B4 | Does not impersonate professionals | Recipient view labelled fictional; `fixtures` require "(fiktif)/(fictional)" names | `fixtures` validation tests; e2e recipient view label | tested locally |

## C. Story Ledger (Care Passport)

Lineage: the source-grounded Care Passport and clarification flow come from Dengar.in v1 (`services/validator/src/carePassport.ts`); v2 ports and extends them ([provenance](SELA_PROVENANCE.md)).

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| C1 | Structured, sourced summary with verbatim quotes and offsets | `internal/ledger` (`ValidateDraft`, `SplitSegments`), `journey/ledger_ops.go` | ledger unit + fuzz tests; `TestAIDraftNeedsReviewEditInvalidatesApproval`; eval `ledger.residual_unsupported_rate` | tested locally |
| C2 | Separate user facts, model interpretation and unknowns | `Item.Quote` vs `Item.Wording` + `WordingOrigin`, `Ledger.Unknowns` | `Ledger.tsx` labels; journey tests | tested locally |
| C3 | Clarification question | `ledger.ComputeUnknowns`, `journey.Clarify` | `TestClarificationAnswerAndSkip`, `TestClarificationRequestRequiresNewShare` | tested locally |
| C4 | Correct, delete, approve | `ReviewItem` (confirm/use_quote/edit/remove/set_kind), `Approve` | `TestAIDraftNeedsReviewEditInvalidatesApproval`, e2e ID journey | tested locally |
| C5 | Versioning invalidates approval and stale shares | `changeLedger`, approval hash, `CreateHandoff` ledger-version check | `TestAIDraftNeedsReviewEditInvalidatesApproval`, `TestStaleVersionRejectedWithoutSideEffects` | tested locally |
| C6 | No invented diagnosis, history or events (Meaning Lock) | `ledger/validate.go`, `clinical.go`, `qualifiers.go` | ledger tests; `TestMeaningLockRejectsFabricatedAndUnsafeModelOutput`; eval `meaning_lock.*` and fault battery; adversarial review findings fixed (see status) | tested locally |

## D. Next Step

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| D1 | Deterministic options with simple rationale | `journey/nextstep.go` | `TestNextStepsAreDeterministicAndHonestWhenEmpty` | tested locally |
| D2 | No diagnosis, triage or treatment | No model in Next Step; fixtures; claim gates | web: `claims.test.ts` (triage/diagnosis gates) | tested locally |
| D3 | User decides; honest empty state; non-handoff steps | `NextStep.tsx`, `nextSteps` fixtures | `TestNextStepsAreDeterministicAndHonestWhenEmpty`, e2e | tested locally |

## E. Opening Practice

Lineage: the opening-lines pipeline comes from Dengar.in v1 (`services/validator/src/openingLines.ts`), ported and extended.

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| E1 | Grounded in the approved note | `ledger.ValidateOpening`, `TemplateOpening`, `journey/opening.go` | ledger opening tests; `TestOpeningPracticeTemplateAndCoverage`; e2e ID journey practice step | tested locally |
| E2 | ID/EN and code-switching | ledger opening tests (code-switched cases) | ledger tests | tested locally |
| E3 | Non-manipulative (no pressure language) | `ledger/opening.go` pressure lexicon | ledger opening tests | tested locally |
| E4 | Not therapy; rehearsal not stored, no score | `journey.Practice` (attempt not stored), `Practice.tsx` ("mentioned / not yet") | journey test; e2e | tested locally |

## F. Handoff and continuity

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| F1 | Exact recipient preview | `PreviewHandoff`, `Share.tsx`, `components/Projection.tsx` | `TestFullJourneyOverHTTP` (preview equals stored projection) | tested locally |
| F2 | Explicit, scoped, versioned share consent | `HandoffInput.ShareConsent`, `SharingPolicy`, field selection | `TestShareRequiresExplicitConsentAndCurrentPolicy` | tested locally |
| F3 | Link token with expiry and revocation | 256-bit token in URL fragment, `RevokeShare`, 1/24/72 h | `TestRevokeExpiryAndDeleteEndRecipientAccess`, e2e revoke | tested locally |
| F4 | Recipient authorization; token alone is not access | token + 8-character code, scoped recipient cookie, lockout | `TestTokenAloneIsNotEnoughAndLockout`; eval `authorization.violations = 0` | tested locally |
| F5 | Receipt and acknowledgement | `HandoffView`, `RecipientAct` | e2e ID journey (ack, complete) | tested locally |
| F6 | Follow-up never equates sent with treated | `CareReceived` always false, copy | e2e ("sent does not mean care"); web: `claims.test.ts` | tested locally |
| F7 | Fictional professionals and synthetic data labelled | fixtures validation, demo banner | `fixtures` tests; e2e | tested locally |
| F8 | Idempotent handoff creation | `Idempotency-Key`, payload hash | `TestIdempotentHandoffUnderConcurrency`, HTTP replay test | tested locally |
| F9 | Never contacts real professionals | No outbound integration exists; recipients are per-owner fixtures | Architecture review; `TestSessionsAreIsolated` | tested locally |

## G. Safety

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| G1 | Safety routing before inference | `journey.assessText` before any op; `startOp` crisis gate | `TestCrisisStoryInterruptsBeforeAnyModelCallOrHandoff`; eval `safety.model_calls_on_crisis_path = 0` | tested locally |
| G2 | Deterministic path for crisis and model failure | `internal/safety`, manual ledger path | `TestNoAIConsentManualPathReachesFollowUp`, `TestProviderUnavailableAndDisabled`, e2e crisis | tested locally |
| G3 | Classifier is never the sole safeguard | Lexicon + interruption + human review of every item + no model in routing (D-017) | `internal/safety` tests | tested locally |
| G4 | No diagnosis or safety promises | Meaning Lock clinical checks; copy | ledger clinical tests; web: `claims.test.ts` | tested locally |
| G5 | Verified, maintained local references; no invented numbers | `fixtures/data/help.json` (verified_official only, dates, re-verify by) | `fixtures` tests; re-verification due 2027-01-14 ([B-5](BLOCKERS.md)) | tested locally |
| G6 | Negation, quotes, slang, code-switching, ambiguity tested | `internal/safety/testdata/vectors.json` (420 vectors, ruleset `sela-safety-2026.10.26-2`) | `TestVectors*`, `TestV1Parity`; eval `safety.*` | tested locally |
| G7 | No sensitive content in operational logs | request log fields only; `llm.Observer` metadata; counters instead of safety logs | log canary in `TestFullJourneyOverHTTP` and e2e | tested locally |

## Backend (Go)

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| BE1 | Modules with clear boundaries | `internal/{config,fixtures,journey,ledger,llm,research,safety,store,httpapi}` | [architecture](ARCHITECTURE.md) | implemented |
| BE2 | Documented API | [API spec](API_SPEC.md) | `TestFullJourneyOverHTTP` exercises it | tested locally |
| BE3 | Validation and structured errors | strict JSON decode, `{"error":{code,message,field}}` | `TestSessionCookieAttributesAndStrictJSON`, `TestLocalizedErrorsAndSessionMissing` | tested locally |
| BE4 | Context cancellation, timeouts, limited retry | `journey/ops.go` (`context.WithCancelCause`, op timeout, cold-start retry), `internal/llm` (attempt timeout, retries before first token only) | `TestCancelOperation`, `TestModalColdStartRetries`, `internal/llm` tests | tested locally |
| BE5 | Idempotency | handoff idempotency keys | `TestIdempotentHandoffUnderConcurrency` | tested locally |
| BE6 | Authentication and authorization | session cookie (HttpOnly, SameSite=Strict, Path), recipient principal, same-origin guard | `TestMutationGuardRejectsCrossSite`, `TestSessionCookieAttributesAndStrictJSON`, eval authorization battery | tested locally |
| BE7 | Consent versioning, revocation and deletion | `Consent.PolicyVersion`, `Revision`, withdrawal cancels ops | `TestConsentsAreIndependentAndRetentionWorks`, `TestStaleAIResultIsDiscarded` | tested locally |
| BE8 | Storage abstraction with memory store; no implicit sensitive persistence | `journey.Store`, `store.Memory` (bounded, LRU idle eviction) | store tests, `TestStoreCapacityNeverEvictsActiveSessions`, `TestLiveSessionsArePerClientBounded` (store) and `TestLiveSessionsPerClientAreBounded` (HTTP, 429 `rate_limited`) | tested locally |
| BE9 | Meta/Modal provider abstraction, no silent non-Meta fallback | `llm.Provider`; only `modal`, `test_double` (non-production only), `disabled` | `internal/llm` tests; config tests (`TestTestDoubleIsRefusedInProductionAndWithoutOptIn`) | tested locally |
| BE10 | Sanitised logging | `httpapi/server.go` request log, `cmd/sela` observer | log canary tests | tested locally |
| BE11 | Health and readiness | `/api/v2/health`, `/api/v2/ready` | `TestFullJourneyOverHTTP`; CI container job | tested locally |
| BE12 | Concurrency and race tests | per-session locks, admission | `go test -race ./...`, `TestIdempotentHandoffUnderConcurrency`, store race tests | tested locally |
| BE13 | Static frontend cannot serve files outside its directory | `httpapi/static.go` (`os.Root`, `fs.ValidPath`, SPA fallback) | `TestStaticServingAndSPAFallback`, `TestStaticServingRefusesSymlinkEscape` | tested locally |

## Frontend

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| FE1 | Rebuilt from the journey | `web/src/pages/*` (React 19 + TypeScript + Vite) | e2e journeys | tested locally |
| FE2 | Responsive and keyboard accessible | `styles.css` (44 px targets, focus-visible, reduced motion) | e2e axe on 15 pages, 390 px, keyboard start | tested locally |
| FE3 | Loading, empty, error, expired, revoked and offline states | `session.tsx`, `Layout.tsx` banners, `Recipient.tsx` (access ended), `SessionEnded` | e2e revoke/deletion; web component tests | tested locally |
| FE4 | i18n with no stray strings | i18n dictionaries | web: `i18n.test.ts`, `hardcoded.test.ts` | tested locally |
| FE5 | No placeholders shown as features | Only implemented features are routed; AI footer reflects the real provider mode | e2e; review | tested locally |

## Meta on Modal

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| M1 | ADR with pinned model ID and revision, licence, capability, cost, hackathon fit | [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md), [model notes](../modal/MODEL_NOTES.md) | Sources cited; hackathon tooling eligibility unverified ([B-2](BLOCKERS.md)) | implemented |
| M2 | Reproducible Modal image | `modal/sela_llm.py` (pinned vLLM image digest, pinned client) | py: `tests/modal` (72) | tested locally (build blocked externally, [B-1](BLOCKERS.md)) |
| M3 | Safe model download and cache | `download_weights` with per-file digests and a verified marker | py: `tests/modal` | tested locally (run blocked externally) |
| M4 | Secrets via a secret manager | Modal Secret `sela-llm-api`; Go reads env only; no secrets committed | py: `tests/modal`; config tests | tested locally |
| M5 | Authenticated endpoint | Modal proxy auth + vLLM key | `modal/smoke_test.py` checks; live layered-auth unverified | blocked externally |
| M6 | Streaming | `internal/llm/stream.go` (SSE, `delta.reasoning` ignored) | `internal/llm` tests with the test double | tested locally (test double) |
| M7 | Limits, concurrency, timeout, retry | `max_containers=1`, `max_concurrency=8`, Go admission, attempt timeouts | `internal/llm` tests, `TestModalColdStartRetries` | tested locally |
| M8 | Scale to zero and container cap | `min_containers=0`, `max_containers=1`, `scaledown_window=300` | py: `tests/modal` | tested locally (deployment blocked) |
| M9 | Cold start handling | `journey.generate` retries Modal 503 within budget; UI "warming" state | `TestModalColdStartRetries` | tested locally |
| M10 | dev/staging environments | `modal deploy -e dev|staging`, `.github/workflows/modal-deploy.yml` (manual) | runbook | implemented |
| M11 | Smoke test and rollback | `modal/smoke_test.py`, [runbook](RUNBOOK.md) | py tests against a local fake server; live run blocked | blocked externally |
| M12 | Owner configuration guide | [modal/README.md](../modal/README.md), [runbook](RUNBOOK.md) | — | implemented |
| M13 | No silent fallback to non-Meta providers | provider modes; `live_meta` provenance only when the server reports the pinned served name | `internal/llm` tests (model mismatch), eval fault `wrong_model` | tested locally |
| M14 | Meta used substantively (extraction, wording, opening lines) | `journey/prompts.go`, ledger and opening operations | test double only; live quality pending | blocked externally |

## Research

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| R1 | Reproducible pipeline, steps 1–10 | `scripts/research/corpus_pipeline.py`, `scripts/research/corpus/` | py: `tests/research` (74); offline build byte-identical twice; `verify` 0 violations | tested locally |
| R2 | Rolling cutoff = freeze date − 5 years; snapshots kept | `manifest.json` (`freeze_date`, `cutoff_date`), `snapshots/2026-10-26/` | py tests (snapshot conflict refuses overwrite) | tested locally |
| R3 | Minimal schema, null when unknown, no guessed results | `records.jsonl` | `verify` schema/invariant checks | tested locally |
| R4 | Licence and access status; no paywall bypass | `rights.py`; only open-access full text from Europe PMC | `records.jsonl` fields; py tests | tested locally |
| R5 | No copyrighted full text in Git | full text only in the gitignored cache; snippets ≤240 chars only for CC BY/BY-SA/CC0 | `.gitignore`; `verify` | tested locally |
| R6 | Traceable extraction (value, snippet, locator, status) | `extract.py` | py tests; 694 evidence items, all `machine_extracted_unreviewed` | tested locally |
| R7 | Reviewer status recorded honestly | `reviewer_status = not_reviewed` for all records | `verify` | tested locally |
| R8 | No automatic fine-tuning | D-016; no training code | — | tested locally (by absence; documented) |
| R9 | Reviewer corpus search without conversation data | `internal/research` (BM25), `/api/v2/research/*`, `Research.tsx` | `internal/research` tests; `TestResearchUnavailableWithoutIndexAndPublicEndpoints`; eval `retrieval.*` | tested locally |
| R10 | Documents are data, not instructions; citation validity | Research data never reaches a model; eval citation check over sela-relay docs | eval `retrieval.citation_validity = 1.0` | tested locally |
| R11 | Human appraisal of evidence | — | Not done; needs a qualified reviewer ([B-4](BLOCKERS.md)) | blocked externally |

## Evaluation

Thresholds pre-registered in `eval/thresholds.json` (commit e23e0e0) before any final run.

| ID | Metric family | Evidence | Status |
| --- | --- | --- | --- |
| EV1 | Extraction accuracy, unsupported statements, grounding | eval `ledger.*`, `meaning_lock.*` (offline, test double) | tested locally; live coverage blocked ([B-1](BLOCKERS.md)) |
| EV2 | Correction/approval | journey tests | tested locally |
| EV3 | Cross-session leakage | eval `authorization.violations`, `TestSessionsAreIsolated` | tested locally |
| EV4 | Sharing scope, expiry, revocation | eval authorization battery | tested locally |
| EV5 | ID/EN consistency | i18n parity; live bilingual wording blocked | partly blocked externally |
| EV6 | Safety routing and model failure | eval `safety.*`, fault battery | tested locally |
| EV7 | Retrieval relevance and citation validity | eval `retrieval.*` | tested locally |
| EV8 | Prompt injection | fault `prompt_injection_echo`; prompts treat input as data | tested locally (test double) |
| EV9 | Latency, cold start, cost | eval live mode only | blocked externally |
| EV10 | Accessibility | e2e axe | tested locally |
| EV11 | Full scenario frontend → Go → Meta on Modal → approval → handoff → acknowledgement | e2e with test double; live run blocked | blocked externally (test double passes) |

## GitHub workflow

| ID | Requirement | Evidence | Status |
| --- | --- | --- | --- |
| GH1 | Verified YanIanZ identity, unsigned, no agent co-author lines | D-021; `git log` of the PR branch | implemented |
| GH2 | Feature branch from latest main, small commits | `feat/v2-go-meta-runtime` | implemented |
| GH3 | Checks before push; PR with problem, changes, validation, limitations | PR description; CI `v2.yml` | implemented |
| GH4 | No force-push, no history rewrite, no merge without authorization | branch history | implemented |
