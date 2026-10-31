# Delivery plan, work breakdown, release gates and schedule

Status: **proposal**. Owner reported production hosting exists; **do not rebuild hosting**. Target event submission: **2026-11-04 21:00 ET = 2026-11-05 09:00 WIB**, subject to organizer changes. No asynchronous promise or automatic deployment is made by this plan.

## 1. Governance and engineering rules

- Owner approves product scope, production impact, paid model usage, legal/safety decisions and final submission.
- PM controls priorities, scope freeze, traceability, acceptance and rollback notes; no self-certified clinical claims.
- Backend owns Go/Postgres encryption, consent, claim graph, transaction and API.
- AI engineer owns Meta prompts, output validation, benchmark and model provenance.
- Frontend owns meaning-review UX, source diffs, stale cues, keyboard and mobile.
- QA/owner executes validation with synthetic identities; release evidence is tied to SHA.
- Any P0 unsafe privacy behaviour pauses the relevant live feature regardless of competition pressure.
- Keep separate implementation PRs after this docs-only planning PR. No accidental auto-merge; preserve human review.

## 2. Task register

Status legend: **existing baseline**, **planned**, **owner-reported/pending verification**, **blocked external**, **verified with evidence**. All tasks here are *planned* unless stated otherwise.

| ID | Priority | Deliverable | Primary source paths / dependency | Acceptance evidence |
| --- | --- | --- | --- | --- |
| H-001 | P0 | Production deployment fingerprint, env/profile matrix | PRODUCTION_VERIFICATION; owner-reported hosting | Verified commit/image, HTTPS, provider identity, no logs |
| H-002 | P0 | Freeze synthetic demo profile with isolated data | config, fixtures, server | No real users, no demo fixture leakage to ordinary production |
| H-003 | P0 | MIE claim schema and migration | internal/db, account | Ownership, encrypted bodies, FK, status, migration roundtrip |
| H-004 | P0 | Versioned claims and source-span validator | internal/ai, account | Quote anchoring, codepoint offsets, no unsupported source |
| H-005 | P0 | Meta structured extraction + verifier | internal/ai/router.go, metaverifier | Provenance, consent, JSON/schema/error gates |
| H-006 | P0 | Approve/correct/reject domain transactions | account, httpapi | Stale expectedVersion blocked; idempotent accept/reject |
| H-007 | P0 | Transitive invalidation graph | account, db | All impacted descendants stale in same transaction |
| H-008 | P0 | Export/share recipient read-time gate | account/shares, export | Stale or revoked cannot be newly fetched/exported |
| H-009 | P0 | Review + Meaning Diff UI | web/src/v3/screens | Accessible source quote, provisional state, action recovery |
| H-010 | P0 | Synthetic scenario fixtures A–G | tests, eval, e2e | Stable IDs; no personal data; expected outcomes |
| H-011 | P0 | Preregistered baseline + treatment eval | EVALUATION; cmd/sela-eval | Held-out report incl. false positives/negatives and cost |
| H-012 | P0 | End-to-end journey + failure tests | QA_MATRIX; e2e, CI | All main/revoke/stale/consent paths green on pinned SHA |
| H-013 | P0 | Video and submission packet | DEMO_AND_SUBMISSION | Repeatable run, evidence index, accurate credits |
| H-014 | P1 | Manual keyboard/screen reader + 320px review | web, QA | Recorded tasks with known limitations |
| H-015 | P1 | Editorial reading approval | operator/reading | Only qualified human-reviewed content published |
| H-016 | P1 | Docs conflict remediation | older ADR/roadmap/SPEC | D-065 precedence consistent; separate review if normative |
| H-017 | P2 | Directory partner verification, real booking | external partners, compliance | Deferred; not within hackathon |
| H-018 | P2 | True token-level streaming | guarded streaming architecture | Deferred rather than emitting unvalidated tokens |
| H-019 | P2 | Multi-facility actual integrations | legal partners | Deferred; no fabricated provider network |
| H-020 | P2 | Expanded content corpus | human licensing/appraisal | Deferred; no unreviewed literature claims |
| H-021 | P0 | Guardrails: danger lexicon, quick-help mode, anti-jailbreak, typo tolerance, multilingual | internal/safety, internal/ai (D-068, D-070) | **Implemented on main**; vectors and tests pass; evaluation set pending |
| H-022 | P1 | Condition map (opt-in, deterministic, not diagnosis) | internal/account/condition.go, /api/v3/condition-map (D-071) | **Implemented on main**; unit and UI tests pass |
| H-023 | P1 | Simulated professionals for the product demo | /api/v3/care-demo (D-071) | **Implemented on main**; fictional, cannot receive data |
| H-024 | P1 | Moderated anonymous sharing space + operator queue | internal/community, migration 1000 (D-072) | **Implemented on main**; DB tests and live Meta moderation check pass |
| H-025 | P1 | Moderation evaluation set and false-positive review | eval/, roadmap S-036 | planned |
| H-026 | P0 | Expose the release SHA in production health and fix duplicated headers | config/deploy, roadmap S-001/S-002 | planned |

## 3. Dependencies and parallelism

- H-001/H-002 before *any* hosted rehearsal.
- H-021–H-024 are built on main (2026-11-01) and may be shown in the demo only with synthetic data and their labels; they never displace the P0 MIE slice (H-003–H-009).
- H-003 before H-004/H-006; H-006 before H-007; H-007 before H-008; H-003–H-009 before demo A/B/C release.
- H-010 can start in parallel with H-003; H-011 must freeze test split before final prompt tuning.
- H-012 covers completed implementations only; H-013 records the exact commit on which H-012 passes.
- Review old docs after code decision, not by asserting planned features already shipped.

## 4. Dated sprint (WIB)

### 2026-11-01 — Ground truth and production verification
- Confirm actual hostname privately, deployment image digest, app commit, CSP/HSTS, config mode, Meta provenance; record only non-secret results.
- Determine whether production currently serves real users. If yes, **do not seed synthetic data or run invasive fault injection there**; use isolated staging/local preview.
- Record CI current status and risk register. Freeze dataset strategy and baseline/test split.
- No-Go if no safe environment for synthetic demo.

### 2026-11-02 — MIE vertical slice
- Implement only H-003–H-009 for correction→stale→reapprove and recipient guard.
- Pair API tests with UI tests; block partial MIE feature with a feature flag until invariants pass.
- Minimum success: one existing original claim, one correction, one memory/summary descendant, one formerly shared link that is denied after invalidation.

### 2026-11-03 — Evaluation / quality / replay
- Run H-010–H-012; store all runs with SHA, model ID, token cost, failure totals, and thresholds.
- Test negative flows: no consent, model timeout, verifier refusal, revoked share, stale versions, cross-owner read, crisis fixture.
- Run keyboard, screen-reader exploratory, 320px and Indonesian/English; fix P0, document P1.
- Record an internal demo rehearsal. If incomplete, **de-scope** instead of claiming a full build.

### 2026-11-04 — Freeze and submit early
- Freeze implementation branch by 18:00 WIB internal target; one code commit and image digest as source of truth.
- Validate final build, video, source link, rights/provenance and official portal fields. Avoid new features.
- Submit well before **2026-11-05 09:00 WIB**; portal confirmation must be saved by owner.

## 5. Exit gates / stop conditions

**Gate 1 — functional:** all seven scenarios have documented result; A/B/C complete for flagship; no severe cross-owner/consent/stale-share bugs; critical errors block release.

**Gate 2 — model:** live Meta calls are real and labelled; failure is honest; safety is checked before release; no Claude/DeepSeek runtime claims under D-065.

**Gate 3 — security/privacy:** no real stories in fixtures/telemetry/logs; any public demo uses legally appropriate disclosed handling; operator never browses private user records; no production seeding.

**Gate 4 — evidence:** CI green for exact SHA, benchmark reproducible with unknowns disclosed, manual QA documented, backup/rollback has an owner.

**Gate 5 — submission:** working instructions, credits, limitations, synthetic notices, official form completion.

No-Go triggers: reproduced cross-user exposure, token leakage, revoked share still accessible through future reads, silent model fallback, false clinical guidance, privacy consent bypass, dangerous unverified crisis-resource links, production misconfiguration, or unverifiable submission claims. Defer offending features; do not “accept risk” merely to meet the deadline.

## 6. Post-event backlog

Independent security/accessibility assessments, real user research with ethical review, source/licence approval, managed key rotation, backup deletion journal, robust multi-instance operations, and partner/facility verification remain separate product-launch gates. An owner-reported production URL does not close them.

## 7. PR protocol

- This planning PR only adds reference documents, not code/runtime changes. Request review; leave **draft/open and unmerged**.
- Engineering PRs must identify H-IDs, diff/DB migration, rollback, test log, evidence, safety risk, docs changes, and flag status.
- Merge solely after owner review and explicitly authorized approval; production deploy is independent from a documentation PR.
