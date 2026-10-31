# Sela | Delivery, migration and acceptance plan

**Baseline:** main at 3278bc7 (29 October 2026) for this documentation PR.  
**Status:** roadmap, not a claim that any new feature was implemented.  
**Release model:** docs approval -> code and tests -> local verification -> live-provider verification -> independent safety/privacy review -> controlled pilot -> public release.

## 1. Delivery principles

1. **Build the entire Sela product, not a fake demo.** Synthetic data belongs to tests and the partner-collaboration sandbox, not the primary product journey.
2. **Do not break existing foundation.** Guest owner scoping, encryption, audit, deletion and Meaning Lock are valuable; change the user-facing account path via migration.
3. **One change per bounded PR.** Keep safety ruleset PR #8 separate; do not merge it on green CI alone.
4. **Status requires evidence.** "Designed", "implemented", "verified locally", "verified live" and "ready for real users" are not synonyms.
5. **Prioritize working vertical slices.** A real end-to-end path matters more than 30 placeholder pages.

## 2. Work packages and dependencies

| Order | Work package | Suggested scope | Completion evidence |
| --- | --- | --- | --- |
| D0 | Reconcile source-of-truth docs | Update SPEC, API v3, backlog, decision log, age policy, security copy and traceability from this approved direction | No contradictory normative claims |
| D1 | Guest-only access | Remove account UI; migrate account-owned records safely; maintain anonymous session auth and consent | Fresh and upgraded guests can access only their own data; no data loss |
| D2 | Meaning graph | Typed claim/source refs, correction events, transactional invalidation, review UI | Reject/edit leads to stale downstream results; no unauthorized reuse |
| D3 | Meta structured tasks | Spark 1.3 vs current Glimmer adapter decision; real provider smoke test and structured outputs | Real model ID/provenance, timeouts, schema and consent verified |
| D4 | Claude + DeepSeek integration | Conditional verifier, explicit fallback, provider isolation and costs | Refusal, outage, revocation and no-AI tests succeed |
| D5 | Curated research | Licensed source ingestion, anchored evidence retrieval, human appraisal | Claim-citation precision evaluation; no unsupported medical claims |
| D6 | Care finder | Profession/facility manual search, sourced directory, no location permission | Real directory searches and stale-data handling tested |
| D7 | Care Context Capsule | Source-approved summary, version pin, edit, secure export | Stale/export, redaction and permission tests succeed |
| D8 | Synthetic hospital sandbox | Multiple fictional organizations/facilities/practitioners and idempotent status states | No external medical calls; consistently labelled as simulated |
| D9 | Quality, evaluation and operating readiness | Go benchmarks, accessible journeys, observability, data retention, threat review | Reproducible reports and independent sign-off evidence |
| D10 | Admin identity and governance | Staff-only MFA, roles, isolated API, immutable audit, no private journal viewer | Anonymous/staff and cross-tenant isolation; role revoke; two-person approval |
| D11 | Planned-care backoffice | Directory revision and independent source verification, professional affiliation checks, isolated synthetic sandbox | Real public directory workflow works; synthetic-only requests never contact real providers |
| D12 | Redis + AI budget admission | Go client, public revisioned cache, separated control counters, durable PostgreSQL ledger and cost visibility | Warm/cold benchmark; controlled spend under concurrency; Redis outage never bypasses consent or safety |

**These are dependencies, not deadline promises.** External credentials, human review, authorization and actual hospital partnerships cannot be solved by code alone.

## 3. Cross-document migration checklist

- [ ] SPEC.md: define new anonymous-only owner model and release gates; remove account requirement or mark legacy.
- [ ] PRODUCT.md and docs/product/PRODUCT.md: reconcile previous user-account and "not a directory" wording; retain original history.
- [ ] docs/api/API_SPEC_V3.md: define session/memory/care APIs and migration; do not remove security middleware.
- [ ] docs/architecture/AI_POLICY.md / Meta docs / ADR-0003: record selected model after **verified availability and policy check**; keep provider separation.
- [ ] docs/architecture/SAFETY.md: document user age, care scope and source-based help; do not infer clinical approval.
- [ ] docs/product/BACKLOG.md, docs/product/ROADMAP.md, docs/status/REQUIREMENTS_TRACEABILITY.md, docs/status/IMPLEMENTATION_STATUS.md: add feature IDs, owners, tests, evidence and latest statuses.
- [ ] web: remove login/recovery/signup navigation safely; update anonymous privacy copy and quick exit.
- [ ] internal/account: preserve guest principal enforcement, migration, session rotation, archival and deletion.
- [ ] E2E/tests: run old journeys plus new invariant suite; verify all claimed capabilities.
- [ ] Docs inventory: add this vision set as active direction, tag old scope as historical where needed.

## 4. Required automated suites

**Anonymous access**: no signup, persistence mode boundaries, 2-session isolation, token theft/fixation prevention, idle/absolute expiry, data migration, export/delete, backup-restore behavior.

**Meaning integrity**: subject/negation/time/uncertainty transformations; revision and invalidation; retry/race; stale result discard; JSON schema and quote boundaries; source deletion and revoked consent.

**AI routing**: actual Meta inference smoke test; provider identity and budget; gated Claude verification; DeepSeek availability-only fallback; no fallbacks for content/safety refusal; realistic cancellation; consent per provider.

**Research**: source and license provenance, retracted/off-topic papers, cited-span correctness, prompt injection in retrieved text, clinical language and abstention tests.

**Care**: no GPS/location invocation, typed city optional, zero results, stale source status, profession/facility relationships, capsule export vs stale versions, synthetic-partner isolation.

**Safety and accessibility**: independent adversarial corpus, code-switch, crisis path zero model calls, aid directory fresh sources, WCAG 2.2 AA keyboard and manual screen-reader testing.

## 5. Go performance report template

| Metric | Unit | Boundaries |
| --- | --- | --- |
| API idle / peak RSS | MiB | report binary, OS, instance size, CGO and DB configuration |
| p50 / p95 / p99 Go route latency | ms | distinguish model-free and model-inclusive endpoints |
| Meta inference p50 / p95 and cold start | ms | model ID, provider region, request size, concurrency |
| Throughput / error rate | req/s, % | sustained realistic load with timeout budgets |
| DB pool / goroutines | count | leaks, queue depth, graceful shutdown |
| Per-task AI spend | USD | Meta/Claude/DeepSeek counted separately |

Use the same test hardware and workload before claiming lower resource use than an alternative stack.

## 6. Research and real-user impact evidence

Measure usability with adult volunteers using synthetic scenarios, with consent and no collection of real health stories in the unapproved prototype. Report sample size, task success, time to consent/correction/export, ability to explain where content goes, and reviewer disagreement. No clinical efficacy or treatment-outcome claim.

Do not invent impact percentages or testimonials. Preserve failed test cases and negative results.

## 7. Launch gates (hard blocks)

- Model licenses, allowed-use, residency, age eligibility and sensitive-health data processing reviewed for each provider.
- Independent safety review; no unresolved P0/P1 crisis bypass or source-leak regression, including pending PR #8 review.
- Hosted PostgreSQL, KEK/secrets, incident response, monitoring, encryption/deletion/restore tested.
- Verified local official-help information and manual screen-reader accessibility review.
- Human review of research-derived advice and care directory credential/sourcing claims.
- Explicit release owner approval and, for partner live transfer, real contracts and authenticated organization/clinician identities.

## 8. Submission narrative versus truth

**Hackathon demonstration:** Sela's ordinary functioning features should run against their genuine implementation. Only the **partner side** of future hospital collaboration uses clearly synthetic actors and statuses. If Meta/Claude/DeepSeek are not live at submission, label the limitation; never replay fixtures as live inference. Separate code quality and product usability evidence from claims of medical benefit.

Meta event judging details belong to [META_EVENT_NOTES](../architecture/META_EVENT_NOTES.md) and may change; check the organizer's official terms before submission.

## 9. Administrative and Redis extension — release gating

**Administrator MVP is a product governance requirement, not a consumer login feature.** Begin D10 early enough to support D6 directory publication. D11 can start with verifiable public directory editorial operations even while actual hospital partnerships remain unavailable. D12 must be evaluated for net cost/latency rather than accepted based on architecture alone.

**Priority acceptance suite:**
- MFA for every privileged staff user, role and organization scoping, separate cookies, CSRF and append-only PostgreSQL audit.
- No staff route able to retrieve anonymous stories/meaning claims without a separately authorized future transfer.
- Independent provider-source publication, stale listing invalidation, and user-facing searching without GPS.
- Only synthetic partner workflows in a sandbox, with no production webhook/booking credentials.
- Redis down: public directory/help continue; AI admission uses safe durable gate or declines new paid calls.
- Redis evicted/restarted: budgets do not reset to infinite and private content cannot appear in cache.
- A/B measure Go p95, database query reductions, cost per accepted AI task, **and** Redis infrastructure spend.
- PR #8 safety engine findings remain a separate release blocker; operator toggles never waive that gate.

[Admin Console](ADMIN_CONSOLE.md) · [Care Operations](ADMIN_CARE_OPERATIONS.md) · [Redis and AI cost engineering](REDIS_AI_COST_ENGINEERING.md).
