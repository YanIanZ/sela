# Implementation status

Updated: 2026-10-26. v3 work happens on branch `feat/sela-v3-pilot`, stacked on PR #6 (`feat/v2-go-meta-runtime`, D-033). Items and acceptance criteria: [backlog](BACKLOG.md). Requirement mapping: [requirements traceability](REQUIREMENTS_TRACEABILITY.md). Order and release ladder: [roadmap](ROADMAP.md). External blockers: [blockers](BLOCKERS.md) (B-1 to B-15). Decisions: [decision log](DECISION_LOG.md) (for v3, D-025 to D-049 (PM) and D-050, D-051 (engineering defaults)). Status words: [SPEC §G](../SPEC.md#g-goal).

## 1. Where v3 stands

**The planning documents, the design documents and the backend foundation are in progress. Nothing in v3 is implemented, verified locally or verified live. Nothing is deployed, and no live call to any AI provider has ever been made.** The v2 evidence in §4 stays valid for v2 scope only.

Status words are defined once in [SPEC §G](../SPEC.md#g-goal) and used here without restating them; the gate for each level, including **ready for controlled pilot** and **public release**, is in [roadmap §2](ROADMAP.md#2-release-ladder).

What exists for v3 today, all uncommitted in the working tree of `feat/sela-v3-pilot`; apart from the documentation link check below, nothing has a recorded run:

- Documents, new: [Dengar.in feature matrix](DENGARIN_FEATURE_MATRIX.md), [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md), [ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md), [API v3 foundation](API_SPEC_V3.md) and the [backlog](BACKLOG.md). Modified: [SPEC.md](../SPEC.md) (the v3 amendments), the [decision log](DECISION_LOG.md) (D-025 to D-049 and D-050, D-051), [decisions](DECISIONS.md), [blockers](BLOCKERS.md) (B-8 to B-15), the [document inventory](DOCUMENT_INVENTORY.md), the [conflict matrix](CONFLICT_MATRIX.md), [product](PRODUCT.md), [safety](SAFETY.md), the [runbook](RUNBOOK.md), [architecture](ARCHITECTURE.md), the AI policy, Claude verifier, Meta integration and provenance documents, `README.md` and the [documentation index](README.md).
- Documentation link check (P1 exit criterion 3; backlog S-273). Command: `python3 -I linkcheck.py . $(git ls-files -co --exclude-standard '*.md')`, where `linkcheck.py` is a session script kept outside the repository (no committed checker or CI step exists yet). It checks every Markdown file Git tracks or would add (137 files, `docs/archive/` included): inline links and reference-style link definitions to local files, and their heading anchors as GitHub-style slugs; external URLs are not fetched. Run 2026-10-26 on the uncommitted working tree, after that day's backlog, status, traceability and roadmap changes. Result: 0 missing files and 0 missing anchors. This is not yet evidence for a commit: it must be re-run on the P1 commit.
- Code: `internal/db` (migration runner, `dbtest`, migrations 0001, 0002 and 0900), `internal/vault`, `internal/account`, `internal/httpapi/v3_*.go`, `internal/config` changes, `internal/fixtures/v3/policies.json` and `policies_v3.go`, `cmd/sela/{cli,database}.go`, `scripts/ops/{backup,restore,restore-drill}.sh`, the PostgreSQL 16 service with migration (up, status, down-last, up) and restore-drill steps in `.github/workflows/v2.yml`, and `go.mod` at Go 1.26.0 with pgx v5.10.0, `golang.org/x/crypto` v0.57.0 and `golang.org/x/text` v0.42.0. Test files exist beside the code; no run of them is recorded here.
- Design (S-190): [design direction](design/DESIGN_DIRECTION.md), [information architecture](design/INFORMATION_ARCHITECTURE.md), [screens](design/SCREENS.md), [content style](design/CONTENT_STYLE.md), [accessibility criteria](design/ACCESSIBILITY_CRITERIA.md), `docs/design/tokens.css` and `docs/design/tools/` (`contrast.py`, `sync_docs.py`). No contrast run and no PM review against brief §4 is recorded; none of it is implemented in `web/`.
- Safety: the ruleset -5 fix loop runs on `feat/v2-go-meta-runtime` (`internal/safety/**`) and has not been merged forward.

### Active work and next steps

This is the repository checkpoint for resuming work (brief §14). It names roles, not people. Each workstream edits only its own files.

| Workstream | Owner role | Files owned | Next step | Risk |
| --- | --- | --- | --- | --- |
| P1 spec and planning documents (S-273) | PM | `SPEC.md`, the decision log, decisions, blockers, conflict matrix, document inventory, product, safety, feature matrix, provenance, AI policy, Claude verifier, Meta integration, ADR-0003, both READMEs; the backlog, this file, traceability and the roadmap | Settle the open alignment points (the SPEC T19 topic note, two PR-template sections); commit P1 and re-run the link and anchor check on the commit (§1) | Documents drift from code while the foundation is uncommitted; every status claim must cite evidence |
| P2 backend foundation (S-002 to S-005, S-171 to S-177, S-211, S-260 to S-262; S-274 planned) | Go backend engineer | ADR-0002, `docs/API_SPEC_V3.md`, `docs/RUNBOOK.md`, `docs/ARCHITECTURE.md`; `internal/db`, `internal/vault`, `internal/account`, `internal/httpapi/v3_*.go`, `internal/config`, `internal/fixtures/v3/` and `policies_v3.go`, `cmd/sela/`, `scripts/ops/`, `go.mod`, `go.sum`, `.github/workflows/v2.yml` | Run F-01 to F-46 locally with `SELA_REQUIRE_DB_TESTS=1` and in CI, and record the results here; commit; open the stacked draft PR (D-033) | The Go 1.26 move also changes the Dockerfile, runbook and contributor docs ([conflict matrix](CONFLICT_MATRIX.md) #48); the hosted parts wait on B-3 and B-15 |
| Design (S-190) | UX designer (design lead) | `docs/design/` | Record the contrast run and the PM review against brief §4; align the "don't save" copy with D-050 ([conflict matrix](CONFLICT_MATRIX.md) row 56) and settle the other design open points ([design direction §10](design/DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners): share expiry, the 15 vs 16–17 age question) | A cosmetic redesign accepted as a structural one; the open points block P3 and P4 copy |
| Safety ruleset -5 (S-150, SPEC T22) | AI engineer (safety) | `internal/safety/**` on `feat/v2-go-meta-runtime` (PR #6), not on this branch | Finish the fix loop and its independent re-verification, commit on PR #6, then merge forward (D-033) | The open gaps from the ruleset -4 re-verification (§4.3); false positives that interrupt ordinary talk |
| P2 frontend (S-001, S-006, S-152, S-153, S-170, S-191, S-193, S-194, S-195 for P2 screens) | Frontend engineer; UX designer | `web/**`, `e2e/**` | Not started. Starts when the foundation API and the design documents are stable: design system, app shell, sign-in, guest start, onboarding, settings and the home skeleton (SPEC T15) | The v2 pages still exist; they stay only in development, test and demo (ADR-0002 §12) |
| Owner actions | Owner | — | Before 2026-11-04 21:00 ET: confirm the submission build and ask the organizers (B-12; [roadmap §3](ROADMAP.md#3-dated-checkpoints)); the GPU budget and Modal credentials (B-1). Before any pilot: B-3, B-5, B-8, B-10, B-15, and B-9 only if the pilot enables DeepSeek. With the Dengar.in team: B-13 | "Verified live" and the controlled pilot cannot be reached without them |

## 2. v3 status per milestone

Milestone order and exit criteria: [backlog §4](BACKLOG.md#4-milestones).

### P1 Audit and spec alignment

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| Dengar.in feature matrix (keep, improve, merge, migrate, retire, paused per feature) | S-273 | in progress | `docs/DENGARIN_FEATURE_MATRIX.md`, uncommitted |
| ADR-0002 persistence, accounts, encryption (accepted for implementation) | S-273 | in progress | uncommitted |
| ADR-0003 AI routing, verifier, fallback (accepted as design) | S-273 | in progress | uncommitted |
| API v3 foundation contract | S-273 | in progress | `docs/API_SPEC_V3.md`, uncommitted |
| Backlog, implementation status, traceability, roadmap | S-273 | in progress | this change, uncommitted |
| AI policy, Claude verifier, Meta integration, provenance brought to v3 | S-273 | in progress | modified in the working tree |
| SPEC amendments (C3, C7, C8, C9, C12, V1, V2, V9, V10, V16, V21, I.data, I.meta); D-025 to D-049 (PM) and D-050, D-051 (engineering defaults) in the decision log; B-8 to B-15 in the blockers; document inventory and index | S-273 | in progress | `SPEC.md`, `docs/DECISION_LOG.md`, `docs/DECISIONS.md`, `docs/BLOCKERS.md`, `docs/DOCUMENT_INVENTORY.md` modified, uncommitted; link and anchor check run 2026-10-26 on the uncommitted tree, 0 broken over 137 files (§1); not yet run on a commit |

### P2 Foundation

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| PostgreSQL 16, migration runner, CI database service, Go 1.26 toolchain | S-260 | in progress | `internal/db`, `cmd/sela`, `.github/workflows/v2.yml`, `go.mod`; ADR-0002 F-01 to F-09 and F-46 not yet recorded as passing |
| Environment separation and startup guards | S-261 | in progress | `internal/config`; F-10, F-11 not yet recorded |
| Per-user envelope encryption, key rotation, crypto-shredding | S-173 | in progress | `internal/vault`; F-12 to F-15 not yet recorded |
| Guest start, accounts, recovery, sessions, age band, guardian consent record | S-002, S-003, S-004, S-005, S-172, S-211 | in progress (backend) | `internal/account`, `internal/httpapi/v3_*.go`; F-16 to F-37 not yet recorded |
| Consents, audit, export, account deletion | S-171, S-174, S-175, S-176 | in progress (backend) | same packages; F-35, F-38 to F-40 not yet recorded |
| v3 policy texts | S-177 | in progress | `internal/fixtures/v3/policies.json`; legal review blocked externally (B-10) |
| Backup, restore and drill scripts (local) | S-262 | in progress | `scripts/ops/`; F-44 not yet recorded; hosted part blocked externally (B-3, B-15) |
| Operator access audit (D-051, SPEC V30) | S-274 | planned; hosted access log blocked externally (B-3) | —; ADR-0002 §13 and the runbook do not describe the record yet ([conflict matrix](CONFLICT_MATRIX.md) row 57) |
| Safety ruleset -5 merged forward | S-150 | in progress | fix loop on PR #6 branch |
| Design direction, information architecture, screens, content style, accessibility criteria | S-190 | in progress | `docs/design/DESIGN_DIRECTION.md`, `INFORMATION_ARCHITECTURE.md`, `SCREENS.md`, `CONTENT_STYLE.md`, `ACCESSIBILITY_CRITERIA.md`, `tokens.css`, `tools/`, uncommitted; contrast run and PM review not yet recorded |
| Welcome, onboarding consents, storage choice, help page, Quick Exit, design system and app shell, bilingual base, WCAG 2.2 AA gate, browser review of the P2 screens | S-001, S-006, S-152, S-153, S-170, S-191, S-193, S-194, S-195 | planned | — |

### P3 Listening, notes and timeline

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| Conversation room, reply rules, streaming accessibility, reflection cards, no-AI path, intent picker | S-007, S-020 to S-024 | planned | — |
| Journal, prompts, reflect with Sela, check-in | S-050, S-051, S-052, S-060 | planned | — |
| Timeline, search, memory, home | S-040, S-070, S-071, S-080 | planned | — |
| Crisis screening on v3 routes, inline crisis support | S-150, S-151 | planned | — |
| Routing core with the Meta path, output guards, data minimisation, Meta-only demo profile | S-220, S-221, S-223, S-226, S-227 | planned; live Meta blocked externally (B-1); organizers' answer pending (B-12) | — |
| P3 parts of spanning items: Meta and `personal_memory` consent purposes, per-record deletion with derivatives, export sections, browser review of the P3 screens | S-171, S-174, S-175, S-195 | planned | — |

### P4 Consultation and sharing

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| Summary builder, print and download | S-090, S-092 | planned | — |
| Share preview and consent, lifecycle and statuses, opening lines, checklists, private names | S-100 to S-104 | planned | — |
| Recipient view, acknowledgement | S-120, S-121 | planned | — |
| After-consultation reflection, own next steps | S-130, S-131 | planned | — |
| P4 parts of spanning items: summary and share deletion and export sections, recipient-open audit events, browser review of the P4 screens | S-174, S-175, S-176, S-195 | planned | — |

### P5 AI integration

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| Routing complete with three labelled test doubles | S-220 | planned | — |
| Claude Sonnet 5.5 verifier (from `prototypes/evidenceguard`, which has 6 mocked tests and is not wired) | S-222 | planned; live blocked externally (B-8) | — |
| DeepSeek fallback generator | S-224 | planned; live blocked externally (B-9; PRC processing needs B-10) | — |
| AI summary wording, memory suggestions | S-081, S-091 | planned | — |
| Evaluation EV-0 to EV-7 | S-228 | planned; live runs blocked externally (B-1, B-8, B-9) | — |
| P5 parts of spanning items: Anthropic and DeepSeek consent purposes, browser review of the P5 screens | S-171, S-195 | planned | — |

### P6 Research and evidence

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| Corpus hygiene, unchecked dates, schema fields | S-240, S-241, S-242 | planned | — |
| v3 gap searches, evidence map, claim-support checks | S-243, S-244, S-245 | planned | — |
| Human appraisal | S-246 | blocked externally (B-4) | — |
| Reading library and "latihan kecil" | S-140, S-141 | planned | — |

### P7 Accessibility, security and operations

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| Manual accessibility review, display settings, slow connections | S-192, S-195, S-196 | planned | — |
| Opt-in voice dictation and read-aloud | S-025, S-026 | planned | — |
| Security review before the pilot, observability | S-265, S-266 | planned | — |
| Hosted deployment, rollback and restore drill; the database host's access log for operator sessions | S-262, S-263, S-274 | blocked externally (B-3, B-15) | — |
| Meta model on Modal; live verification of Meta as the generator | S-264, S-221 (live) | blocked externally (B-1) | — |
| Help-line re-verification | S-154 | blocked externally (B-5) | — |
| Legal readiness (DPIA, DPO, processors, cross-border basis, PSE registration, breach runbook) | S-178 | blocked externally (B-10, B-15) | — |
| Minors: experience, guardian consent screens, evaluation, legal and provider gate | S-210, S-211 (UI), S-212, S-213 | planned; production gate blocked externally (B-10) | — |
| Legacy Dengar.in deployments | S-267 | blocked externally (B-13) | — |

### P8 Final integration and pilot readiness

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| Final browser review and screenshots | S-195 | planned | — |
| `OWNER_TESTER.md` | S-270 | planned | — |
| Pilot readiness report and pilot plan | S-271 | planned | — |

## 3. Not done in v3 (and why)

- **No v3 user-facing feature exists.** There is no conversation room, journal, standalone check-in, cross-session timeline, memory view, multi-entry consultation summary, purpose-bound sharing, v3 recipient view, after-consultation reflection, reading library or home. They are planned for P3 to P6.
- **The foundation is unverified.** The P2 code is uncommitted and none of the ADR-0002 acceptance criteria F-01 to F-46 is recorded as passing.
- **No v3 frontend code exists.** The design documents are written but uncommitted and not yet reviewed; the running frontend is v2, which still shows demo copy (synthetic data, adults only, memory only).
- **The spec realignment is uncommitted.** The SPEC.md amendments, decisions D-025 to D-049 (PM) and D-050, D-051 (engineering defaults) and blockers B-8 to B-15 are in the working tree, uncommitted; the link and anchor check passed on that tree (§1) but has not been run on a commit.
- **No multi-provider AI.** There is no router, no wired verifier and no DeepSeek code in Go. No live call to Meta, Anthropic or DeepSeek has been made (B-1, B-8, B-9); the generator choice is still open (B-7).
- **No WCAG 2.2 evidence.** The v2 e2e scan uses WCAG 2.0/2.1 tags, fails only on serious or critical findings, checks 390 px instead of 320 px, and every page has the same title (S-194).
- **Ages 15–17 are not built** and their production use is blocked by the legal review (B-10).
- **Legal preconditions for real users are open:** DPIA, DPO, processor agreements, cross-border basis, PSE registration and breach runbook (B-10); hosting and region (B-3, B-15); email for recovery does not exist (B-11, optional).
- **Help lines need owner decisions:** a read-only check on 2026-10-26 found differences from the fixtures (Healing119 hours and WhatsApp, LISA cost, Yayasan Pulih contacts) (B-5).
- **Research does not cover the v3 core:** no direct records on listening, journaling, consultation preparation or accessibility; no adolescent consent or confidentiality evidence; no human appraisal (B-4).
- **Outside the repository:** two legacy Dengar.in deployments are live and may process real people's text (B-13); the hackathon organizers' position on non-Meta providers is unknown (B-12).

## 4. v2 evidence that remains valid (v2 scope)

This section describes branch `feat/v2-go-meta-runtime` (PR #6): a synthetic, adults-only, memory-only, Meta-only demo. v3 supersedes those limits as product scope (D-025), but the tests below remain the baseline that v3 must keep passing or replace explicitly. In v2 wording, "tested locally" means the automated suites in this repository pass on that branch (CI runs the same commands); it corresponds to "verified locally" for v2 scope. Nothing below is deployed.

### 4.1 Summary (v2 milestones)

| Milestone | Spec | State | Evidence |
| --- | --- | --- | --- |
| M0 Baseline, conflict matrix, decisions, spec amendments | T1, T2 | Done; live Meta access blocked | [conflict matrix](CONFLICT_MATRIX.md), [decision log](DECISION_LOG.md), [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md) |
| M1 Foundation: Go module, API contract, frontend shell, i18n, CI | T3 | Tested locally | Go race tests, Vitest, `v2.yml` |
| M2 Core flow: onboarding, consents, story room, Story Ledger, approval | T4, T6 | Tested locally | journey + HTTP tests, e2e |
| M3 Continuity: Next Step, Opening Practice, handoff, receipt, acknowledgement, follow-up | T7 | Tested locally | journey + HTTP tests, e2e |
| M4 Meta on Modal: serving app, auth, streaming, failure handling | T5, T11 | Implemented and tested offline; **deployment blocked** ([B-1](BLOCKERS.md)) | `internal/llm` tests, `tests/modal`, fault battery |
| M5 Research pipeline | T10 | Tested locally; human appraisal blocked ([B-4](BLOCKERS.md)) | `tests/research`, `corpus_pipeline.py verify`, eval retrieval |
| M6 Hardening: safety, privacy, authorization, adversarial, accessibility | T8 | Tested locally; adversarial reviews done and fixed | review findings below |
| M7 Demo and delivery: e2e, runbook, docs, PR | T8, T12 | e2e and docs done; live demo evidence blocked | [runbook](RUNBOOK.md), PR |
| Cutover and v1 removal | T9 | Not started (waits for live acceptance) | [roadmap](ROADMAP.md) |

### 4.2 Latest results (v2)

| Check | Result |
| --- | --- |
| `gofmt -l cmd internal`, `go vet ./...` | clean |
| CI on PR #6 at `e9711af` (GitHub check runs, read 2026-10-26) | Go vet, format and race tests; web typecheck, unit tests and build; research and Modal offline tests; e2e journeys with the test double; container health check: **succeeded**. The CodeQL summary check **failed**, matching the 10 open alerts below |
| Local e2e report (`e2e/results/e2e-report.json`, gitignored, run 2026-10-26 12:05 UTC) | 14 of 14 checks passed with the test double; 16 axe scans with 0 violations under the WCAG 2.0/2.1 A/AA tags only (WCAG 2.2 is not checked) |
| Local Go runs on `go1.24.7` at `e9711af` (2026-10-26) | `cmd/fakellm`, `internal/config`, `internal/fixtures`, `internal/ledger`, `internal/llm/...`, `internal/research` and `prototypes/evidenceguard` passed; `internal/safety` was not run because it was being edited |
| Research pipeline at `e9711af` (research audit, 2026-10-26) | `tests/research` 74 tests OK; `corpus_pipeline.py verify` 0 violations and 3 warnings (rows citing the `needs_check` records R065 and R100); an offline rebuild from the local cache was byte-identical to every committed output |
| `sela-eval -mode offline` / `-mode test_double` | no result recorded in this document |

### 4.3 Adversarial reviews (v2)

Each package was implemented, then reviewed by an independent agent that tried to break it, then fixed. Reviewers worked on copies and reported evidence.

| Area | Review verdict | Main findings | Outcome |
| --- | --- | --- | --- |
| Story Ledger / Meaning Lock | Request changes | wording negation checked as a whole-text flag; quotes/brackets ended negation scope; opening lines could reverse a negated item; informal negators (gabisa, dont); wording could add new content; medication/diagnosis lexicon gaps; clinician-attribution exception abusable; quadratic validation on long wording | Fixed with a regression test per finding (`59ebef8`, `85e8923`); validator `sela-meaning-lock-2026.10.26-2` |
| Modal serving app | Approve with fixes | smoke test forwarded credentials on redirects; budget did not bound wall time; malformed chunk crash; snapshot repair could not re-download; report could include generated text | Fixed (`7a808fd`, `c6f4e4f`); 72 offline tests; nothing deployed |
| Research pipeline | Changes requested | country/population labels taken from negated register notes; claim gates flagged required disclaimers; default freeze date conflicts with committed snapshot; verify ignored stale inputs | Fixed (`5bdb395`); pipeline 1.1.0, rules v2, gates v2 |
| LLM client and test double | Request changes | server text could reach error messages via `finish_reason`; health fallback matched model paths by substring; test-double negators drifted from the ledger | Fixed (`9762f66`, `40921d1`); drift tests pin the test double to the ledger |
| Safety lexicon (four review rounds) | Request changes | round 1: own quoted words attributed to others, inner-self death wishes, variant-index bug, non-linear frames, look-alike letters; round 2: accident/idiom exceptions too broad, lyrics/report frames, leetspeak with 1 and \|, slang and spacing gaps, worst-case timing | Round 1 fixed (`ee01781`, ruleset `-2`, 420 vectors); round 2 first committed as `d187977` (ruleset `-3`), whose re-verification found 95 crisis inputs it routed lower and quadratic loops; two more fix and independent re-verification rounds fixed those (ruleset `-4`, 1329 vectors, linear time, 23 ms worst case at 12,000 characters; against `-3` only contrived space-joined variants route lower). The last re-verification still lists four major gaps, three of which v1 misses too (hope/wish phrasings with words between the verb and the death word, intensifiers in the same position, present or future overdose plans) and one in the new accident-marker gate (emoticons and variant spellings), plus minor adoption, method-lexicon and false-positive items; **a further fix round is running** (ruleset `-5`, backlog S-150). |
| Journey, store and HTTP transport | Request changes (33 findings) | crisis gate ordering, stale AI commits, op-runtime purge, recipient principal caps, per-session and per-client limits, retention-aware eviction, revoke, strict decoding and routing, limiter eviction, SSE shutdown | Fixed and re-verified by an independent re-run of the probes (`0b57dba`, `ef53548`, `b8bc4ce`) |
| Frontend | Request changes | opening-line provenance, stale share previews, recipient token handling, session refresh, stream reconnect, crisis routing, catalog fallback, focus handling | Fixed (`a483448`, `ebca156`); 14 e2e journeys |
| CodeQL (GitHub code scanning) | 10 alerts on PR #6 | static file path, clear-text and log-entry logging; research and eval paths from operator configuration | Static files through `os.Root` and named-field logging (`a33607a`); the rest have operator-controlled sources and are left for the owner to dismiss or keep (threads open on the PR); v3 triage is backlog S-266 |

### 4.4 Not done in v2 (and why)

- **Live Meta inference** on Modal: no Modal credentials or GPU budget in this environment ([B-1](BLOCKERS.md)). No `live_meta` output has ever been produced; local runs use AI off or the labelled test double.
- **Live metrics**: bilingual wording quality, important-fact coverage, warm latency, cold start and cost per session are pre-registered but unmeasured.
- **Public deployment** of the Go app ([B-3](BLOCKERS.md)); the container image is built and smoke-tested in CI.
- **Human research appraisal** ([B-4](BLOCKERS.md)); every record is `reviewer_status: not_reviewed`.
- **Cutover (T9)**: v1 stays runnable until v2 is accepted with live evidence. Under v3 the removal waits for v3 acceptance (backlog S-272).

### 4.5 Cost estimate (v2 assumptions)

Modal H100 list price $0.001097/s (≈ $3.95/h), observed 2026-10-26. One demo session ≈ one cold start (minutes, unmeasured) + three AI operations + the 5-minute scale-down window. With an assumed 4-minute cold start and 30 s per operation that is ≈ 10.5 GPU-minutes ≈ $0.70 per isolated session; consecutive sessions within the scale-down window share the warm container. Weights download runs on CPU. Details and ceilings: [modal/README.md](../modal/README.md#cost) and [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md). These are estimates, not measurements. v3 per-operation and per-session estimates for Meta, Claude and DeepSeek, also unmeasured, are in [ADR-0003 §11](adr/ADR-0003-ai-routing-verifier-fallback.md#11-cost-engineering-estimates-from-observed-list-prices-not-measurements).
