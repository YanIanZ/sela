# Implementation status

**Current local checkpoint — 2026-10-28:** the owner requested a replacement visual identity and Dengar.in continuity features. The current UI authority is [web/DESIGN.md](../web/DESIGN.md), with the task-first Home and cobalt/open-S identity. Manual memory, weekly reflection, source-bound manual summaries, preparation and follow-ups are now connected locally; recipient sharing and editorial clinical cards remain unavailable. See [continuity validation](V3_CONTINUITY_VALIDATION.md). Earlier dated implementation/visual prose below is historical and does not override that checkpoint.

## Local v3 frontend delivery — 2026-10-28

Current workspace: `feat/v3-experience`, stacked on the local
`feat/v3-core-journey` work (merged main `e62506a`, PRs #9–#13; #8 excluded).
**Status: implemented and verified locally for the screens whose APIs exist;
nothing is verified live and no pilot or public-release claim is made.**

**Refinement pass, 2026-10-28:** a rendered-screen review at 1365, 390 and
320 px fixed the leaked v2 stylesheet, a text-fading route animation, the
pre-commit scroll reset, the phone first viewport, and the 320 px conversation
room. The display face is now a system serif. Evidence and the deliberate
deviations are in [the validation record](V3_FRONTEND_VALIDATION.md#refinement-pass-2026-10-28)
and [DESIGN_DIRECTION §11](design/DESIGN_DIRECTION.md#11-revision-log). No API,
consent, storage or safety contract changed.

The default frontend now implements the 28 screen groups in
[`docs/design/SCREENS.md`](design/SCREENS.md) to the extent their APIs exist:
the app shell and design system, Introduction/About, onboarding (18+ guest and
account, 16–17 guest with guardian consent, 15/under-15 blocks), sign-in,
recovery, guest upgrade, Home, Talk list and room, notes/check-ins, timeline and
patterns, Support hub, Immediate help, Privacy & data, Display & accessibility,
Account and Policies. Memory, consultation summary/preparation/sharing/
recipient/follow-up and reading are honest blocked states and name their missing
API or editorial gate. The v2 synthetic demo stays at `/demo` and `/r`; the
legacy `apps/web/` app is untouched.

Real integrations: accounts, guest, recovery, reauth, password, recovery codes,
consents, guardian consent, sessions, audit, export, account/guest deletion,
personal records and revisions, conversation list/room/turns/stop/storage/resume
and the bundled verified help directory. The only backend change is additive to
the encrypted check-in payload: `energy`, `tags` and mood `mixed`, covered by
`TestV27CheckInEnergyAndTags`.

Evidence: [`V3_FRONTEND_VALIDATION.md`](V3_FRONTEND_VALIDATION.md). Frontend
typecheck, 58 unit/component/axe tests and the production build pass; every Go
package passes with `SELA_REQUIRE_DB_TESTS=1`; a Chromium journey with 18 axe
scans (WCAG 2.0/2.1/2.2 A/AA) and 320px/desktop checks passed 44/44 and deleted
its synthetic guest. CI on PR #14, head `0c7cbd0`, passed every job: `v2.yml`
run 40622842591 and Quality gate run 40622842602. Screenshots are in
[`docs/evidence/v3-frontend/`](evidence/v3-frontend/). Manual
screen-reader/usability testing has **not** occurred.

Still open: memory, summary/sharing/recipient/follow-up, reading editorial
review, conversation corrections/reflections/streaming/hold warning/derived-item
deletion, earlier policy versions, the 15-year-old server sub-band, legal review
(B-10), hosting/region (B-3, B-15), live provider evaluation (B-1, B-8, B-9) and
the restore announcement gate. Items keep their backlog status; this frontend
delivery does not make them verified live.

## Historical local v3 core increment — 2026-10-27

Current workspace: `feat/v3-core-journey`, based on merged main `e62506a` (PRs #9–#13; #8 remains excluded). Work is **in progress**, with local test evidence; it is not a pilot/public-release claim. The default frontend now uses `/api/v3`: guest/account entry and recovery, home, Meta-primary listening with independent provider consent, encrypted saved conversations, journal with revisions, one check-in per local day, timeline, support, privacy, export and deletion. `/demo` retains the v2 synthetic journey.

No personal content is saved without its surface's explicit storage choice. Unsaved rooms are session-scoped holds, purged at End/sign-out, 30 inactive minutes or 2 hours total. A saved crisis pause survives reopen; deleting its record cancels the associated room. The user original is preserved with versions; owned records and revisions participate in export and account deletion.

Evidence and exact commands: [local v3 validation](V3_CORE_VALIDATION.md). Still open: memory and interpretation review, consultation summaries/sharing/follow-up v3, reading-library UI and authoring verification, conversation streaming, full settings/minor/guardian surfaces, hosted operations and pilot gates. The implementation below is a historical checkpoint from 2026-10-26; references there to unmerged PRs and missing provider credentials are stale, not current merge/access status. Local credentials are not evidence of deployment, model-revision pinning or a completed verifier evaluation.


Updated: 2026-10-26. v3 work happens on branch `feat/sela-v3-pilot`, which was stacked on PR #6 (`feat/v2-go-meta-runtime`, D-034). PR #6 was merged into main on 2026-10-26 (`7fc6678`), and this branch merges it (D-053). The branch is pushed and open as **draft PR #9** against main: `0ff3341`, `4cadbfd` and `59dba14`, and on top of them the documentation integration pass that this file describes as "the integration pass", committed on `feat/sela-v3-pilot` and pushed to draft PR #9 (the CI run recorded in §1 tested `59dba14`; no CI run on the integration pass's commit is recorded here). The AI provider layer is draft PR #10 against `feat/sela-v3-pilot`, not merged; the research and reading work is draft PR #11 against main, not merged; ruleset `-5` takes a new PR against main, draft PR #8, a WIP snapshot (D-034 status, [conflict matrix](CONFLICT_MATRIX.md) row 63). Items and acceptance criteria: [backlog](BACKLOG.md). Requirement mapping: [requirements traceability](REQUIREMENTS_TRACEABILITY.md). Order and release ladder: [roadmap](ROADMAP.md). External blockers: [blockers](BLOCKERS.md) (B-1 to B-15). Decisions: [decision log](DECISION_LOG.md) (for v3: owner direction D-026; PM decisions D-027 to D-050 and D-054 to D-056; engineering defaults D-051 and D-052; merge record D-053). Status words: [SPEC §G](../SPEC.md#g-goal).

## 1. Where v3 stands

**The v3 backend foundation is verified locally, and its CI run on draft PR #9 passed** (commit `0ff3341`, ADR-0002 criteria F-01 to F-47; how the status word applies to it: [SPEC §G foundation status note](../SPEC.md#foundation-status-note-2026-10-26), which the PM confirms or withdraws now that the CI run is recorded; its implemented-gate point is closed, confirmed by the PM, [D-056](DECISION_LOG.md#d-056)). **At that 2026-10-27 checkpoint, everything else in v3 was planned or in progress, no v3 screen existed, nothing was verified live, nothing was deployed, and no live call to any AI provider had been made.** The 2026-10-28 frontend section above supersedes the screen statement; live/deployment status is unchanged. The v2 evidence in §4 stays valid for v2 scope only.

Status words are defined once in [SPEC §G](../SPEC.md#g-goal) and used here without restating them; the gate for each level, including **ready for controlled pilot** and **public release**, is in [roadmap §2](ROADMAP.md#2-release-ladder).

### Verified locally, CI passed on draft PR #9: the backend foundation (`0ff3341`)

- **What:** commit `0ff3341` "v3 foundation: PostgreSQL persistence, encryption, accounts, guest mode and /api/v3" on `feat/sela-v3-pilot` (87 files; pushed with the documentation commits `4cadbfd` and `59dba14` to `origin/feat/sela-v3-pilot` on 2026-10-26 at 21:33 UTC and described in draft PR #9). Code: `internal/db` (migration runner, `dbtest`, migrations 0001, 0002, 0003 and 0900), `internal/vault`, `internal/account`, `internal/httpapi/v3_*.go`, `internal/config`, `internal/fixtures/v3/policies.json` and `policies_v3.go`, `cmd/sela/{cli,database}.go`, `scripts/ops/{backup,restore,restore-drill,pgauth}.sh`, the PostgreSQL 16 service, the migration round trip, the restore drill and the container job in `.github/workflows/v2.yml`, `deploy/v2/Dockerfile` on `golang:1.26.10-alpine`, and `go.mod` at `go 1.26.0` with `toolchain go1.26.10` (D-054), pgx v5.10.0, `golang.org/x/crypto` v0.57.0 and `golang.org/x/text` v0.42.0 (approved, confirmed by the PM, [D-056](DECISION_LOG.md#d-056); ADR-0002 §1). Documents: [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md), [API v3 foundation](API_SPEC_V3.md), the [runbook](RUNBOOK.md) and [architecture](ARCHITECTURE.md).
- **Recorded run** (2026-10-26, at `0ff3341`, local PostgreSQL 16, synthetic data only; checked by the PM):
  - `gofmt -l cmd internal` prints nothing and `go vet ./...` is clean;
  - `go test -race ./...` against PostgreSQL 16 passes in every package (`cmd/fakellm`, `cmd/sela`, `internal/account`, `internal/config`, `internal/db`, `internal/db/dbtest`, `internal/fixtures`, `internal/httpapi`, `internal/journey`, `internal/ledger`, `internal/llm`, `internal/llm/fakellm`, `internal/research`, `internal/safety`, `internal/store`, `internal/vault`);
  - the migration round trip (up, status, down-last for every migration, up) and `scripts/ops/restore-drill.sh` pass (the drill rows of [runbook §8](RUNBOOK.md#8-backup-and-restore) record the runs after review rounds 1, 2 and 3);
  - the container smoke steps, run with the binary: the production default refuses to start without a database, a KEK and an https origin, and a development start against PostgreSQL migrates and serves health, readiness and the public `/api/v3` routes.
- **Recorded run of every suite in the roadmap §2 gate** (2026-10-26, 21:04 to 21:12 UTC, on a `git archive 0ff3341` export outside the repository; Go 1.26.10, PostgreSQL 16.15, Node 22.22.0, Python 3.13.17; synthetic data and labelled test doubles only; run by the documentation integrator, an agent, with the logs kept outside the repository; not yet checked by the PM). The foundation changes `cmd/sela`, the binary the e2e journeys start, and `cmd/sela-eval/model.go`, so these suites belong to its evidence:
  - `gofmt -l cmd internal` prints nothing; `go vet ./...` is clean; the direct module requirements are exactly the list in `v2.yml` (pgx v5.10.0, `golang.org/x/crypto` v0.57.0, `golang.org/x/text` v0.42.0);
  - `SELA_REQUIRE_DB_TESTS=1 SELA_TEST_DATABASE_URL=… go test -race -count=1 ./...`: `ok` in all 16 packages that have tests (`cmd/sela-eval` has none);
  - the migration round trip of `v2.yml` on a scratch database (status, dry-run, up, status, down-last for 0900, 0003, 0002 and 0001, status, up, status): exit 0;
  - `SELA_DRILL_DATABASE_URL="$SELA_TEST_DATABASE_URL" scripts/ops/restore-drill.sh --sela build/sela`: "restore drill ok: time to restore 1s"; reconcile deleted 2 users through the deletion path, destroyed 1 key without a user and revoked 1 session; canary ok (row in the [runbook §8](RUNBOOK.md#8-backup-and-restore) drill table);
  - `cd web && npm ci && npm run typecheck && npm test && npm run build`: typecheck and build pass. The first Vitest run, made while the race tests were running in parallel, failed one timing-bound test (`src/test/operation.test.tsx`, "keeps the step stoppable instead of reporting the model unavailable", a 1-second `findByRole` wait); one recorded re-run passed 47 of 47 (both logs are kept); the CI web job on PR #9 passed as well (below). The failure is recorded here so that a flaky test is not hidden (the web owner may widen that wait);
  - `node e2e/journey.e2e.mjs` (preinstalled Playwright 1.63.0 with Chromium from `/opt/pw-browsers`, the fake LLM test double, ports moved with `E2E_PORT` and `E2E_LLM_PORT`): 14 of 14 journeys pass, including the axe and 390 px checks of the public pages;
  - `python3 -m unittest discover -s tests/research -p 'test_*.py'`: 74 tests OK; the same for `tests/modal`: 72 tests OK; `python3 scripts/research/corpus_pipeline.py verify`: 0 violations and the 3 known warnings (evidence-map rows that cite the non-included records R065 and R100);
  - `go run ./cmd/sela-eval -mode offline`: exit 0, 11 pass, 10 not measured (live Meta inference or later milestones), none failing; `-mode test_double`: exit 0, 14 pass, 7 not measured, none failing.
- **Recorded CI run** (2026-10-26; read through the GitHub REST API, not yet checked by the PM): draft PR #9 (`feat/sela-v3-pilot` → main, head `59dba14`, opened 21:34:18 UTC). `.github/workflows/v2.yml` run **40405475962** (event `pull_request`, so it tested the merge ref `514207b` of `59dba14` with main `7fc6678`; its tree is that of `59dba14`, which already merges main) completed with **success** at 21:39:11 UTC, all five jobs passing:
  - "Go vet, format and race tests (PostgreSQL 16)": format, vet, the approved-module check, the migration round trip, `go test -race -count=1 ./...` with `SELA_REQUIRE_DB_TESTS=1`, and the restore drill;
  - "Frontend typecheck, unit tests and build"; "Research pipeline and Modal config tests (offline)";
  - "Container image builds and serves health": the first image build of `deploy/v2/Dockerfile` on Go 1.26.10, with the production-refusal and health smoke steps;
  - "End-to-end journeys (test double, never Meta)".
  The Quality gate run **40405475912** (v1 checks) passed too. In the root module's Go code (the code `v2.yml` tests) `59dba14` differs from `0ff3341` only in three comments (`internal/config/v3_config_test.go`, `internal/journey/story.go`, `internal/llm/llm.go`); the rest of the difference is documentation, the design tooling and, from main, the owner's `prototypes/generationfallback` with its workflow. The run is therefore evidence for the foundation commit. `v2.yml` has no `sela-eval` step, so the evaluation modes are recorded only in the local run above ([roadmap §2](ROADMAP.md#2-release-ladder)). PR #9's description also records a validation of `59dba14` in a clean checkout (Go race tests with PostgreSQL, the two prototypes, web 47 tests and build, e2e 14 of 14). Until 2026-10-27, PR #10 (`feat/sela-v3-ai` → `feat/sela-v3-pilot`) got no `v2.yml` run, because the workflow ran only for pull requests to main; `v2.yml` now also runs for pull requests to `feat/sela-v3-pilot`, so the stacked v3 PRs get the same checks.
- **Review:** three rounds of security review by agent reviewers independent of the implementer (internal; not the independent security assessment that public release requires) covered authentication and sessions, and data, cryptography, migrations and backups; their findings were fixed, and each finding and its change is in the ADR-0002 amendments. The residual risks the ADR states (a small username timing difference for a while after the argon2id parameters change; the restore revival window until a deletion journal exists) are accepted for a controlled pilot and revisited before a public release ([D-055](DECISION_LOG.md#d-055)).
- **Not done yet:** the hosted parts (managed database, secret manager, backup storage, hosted restore drill), which wait on B-3 and B-15; `sela-eval` in CI (`v2.yml` has no step for it; adding one is a change for the workflow's owner, the Go backend engineer). The foundation has no screen, so no browser review applies to it; every screen that uses it needs one (S-195).
- **What it covers in the backlog:** S-173 and S-260 are verified locally (CI passed on PR #9). The backend parts of S-002 to S-005, S-171, S-172, S-174 to S-177, S-211 and S-261, and criterion 1 of S-262 (F-44: the local backup, restore and drill), are verified locally; those items stay in progress for their frontend, feature, demo or hosted parts. S-262 stays in progress because its criterion 3, the restore announcement in the app and on a status page that `terms-privacy-2026-11-01.2` promises, is planned, and its criterion 2 (the hosted drill) is blocked externally. The operator access audit record (S-274) is not part of it and is planned.
- **Since the foundation commit** (the integration pass, committed on `feat/sela-v3-pilot`, draft PR #9; no CI run on it is recorded yet): the `terms_privacy` text states the restore window and is a new version, `terms-privacy-2026-11-01.2` (D-055; the sentence, including its second clause that every restore is announced in the app and on the status page, confirmed by the PM, [D-056](DECISION_LOG.md#d-056)), with the test `TestTermsPrivacyStatesBackupWindows`; the `guest_storage` text states that a restore ends every guest's access and is a new version, `guest-storage-2026-11-01.2`, checked by `TestGuestStoragePolicyText` (written in the integration pass from `internal/account/lifecycle.go`, confirmed by the PM, [D-056](DECISION_LOG.md#d-056); ADR-0002 §11, [runbook §8](RUNBOOK.md#8-backup-and-restore)). `go test ./internal/fixtures/...` passes, and with PostgreSQL 16 so do `./internal/httpapi/...` and `./internal/account/...` (2026-10-26).

### In progress (nothing about it can be claimed yet)

| Work | Where | Backlog items |
| --- | --- | --- |
| Planning documents: spec, decisions, blockers, backlog, status, traceability, roadmap, feature matrix, ADR-0003, AI documents | Committed with the planning documents (`4cadbfd`) and the merge of main (`59dba14`, D-053), both on `origin/feat/sela-v3-pilot` in draft PR #9; the integration pass that records D-054 to D-056 is committed on `feat/sela-v3-pilot` (draft PR #9) | S-273 |
| Design documents: [design direction](design/DESIGN_DIRECTION.md), [information architecture](design/INFORMATION_ARCHITECTURE.md), [screens](design/SCREENS.md), [content style](design/CONTENT_STYLE.md), [accessibility criteria](design/ACCESSIBILITY_CRITERIA.md), `docs/design/tokens.css` and `docs/design/tools/` (`contrast.py`, `sync_docs.py`) | Committed with the planning documents (`4cadbfd`, in draft PR #9); the integration pass's consistency fixes to them are committed with it (draft PR #9); no contrast run and no PM review against brief §4 recorded; nothing of it is implemented in `web/` | S-190 |
| Frontend shell: design system, app shell with help and Quick Exit, ID/EN namespaces, sign-in and settings screens, `e2e/v3-*.e2e.mjs` | Uncommitted in the working tree of `feat/sela-v3-pilot` | S-153, S-191, S-193 (and the UI parts of the P2 items) |
| AI provider layer: `internal/ai` (router, rules, generator, Anthropic verifier, DeepSeek, guards, minimisation, provenance), `eval/verifier`, a new `eval/thresholds.json` version | Committed on branch `feat/sela-v3-ai` (`44845d3` and `63a5c98`, then `02051ae`, which merges `feat/sela-v3-pilot` at `59dba14`) and open as draft PR #10 against `feat/sela-v3-pilot`, not merged (`v2.yml` runs for it from 2026-10-27, when the workflow started to cover pull requests to `feat/sela-v3-pilot`); nothing verified | S-220 to S-224, S-226 to S-228 |
| Research and reading content: v3 gap search, v3 evidence map, claim checks, corpus fields, the reading library | Committed on branch `feat/sela-v3-research` (`268a101` "research: v3 gap search, evidence map v3 and pipeline 1.2.0" and `64aa3f6` "content: reading library v1", rebased on main `7fc6678` and renumbered to the repository IDs while rebasing) and open as draft PR #11 against main, not merged. Local checks passed on 2026-10-26: `sh scripts/research/precommit.sh` (202 tests OK, `corpus_pipeline.py verify` 0 violations and 3 warnings, leak check 0 leaks), `corpus_pipeline.py reproduce` byte-identical for both frozen snapshots (`2026-10-26` and `2026-10-26_pipeline-1.2.0`), and 79 tests OK in `tests/content`. CI on PR #11 passed on head `64aa3f6` (2026-10-26: the research corpus workflow on Python 3.10 and 3.12, `v2.yml` and `quality.yml`; no workflow runs `tests/content` yet). The work is not on the delivery branch, so its items stay in progress | S-140, S-240, S-242 to S-245 |
| P3 listening backend: conversations with the unsaved hold, journal and check-in, memory, timeline (migrations 0100, 0200, 0300; contract draft `docs/api/V3_LISTENING_AND_NOTES.md`) | Branch `feat/sela-v3-listening`, uncommitted, not merged | S-020, S-050, S-060, S-070, S-080 (backend) |
| Operations and compliance: `deploy/v3`, deployment and observability documents, `docs/compliance/` drafts | Branch `feat/sela-v3-ops`, uncommitted, not merged | S-265; preparation for S-263 and S-178, which stay blocked externally |
| Safety ruleset `-5` fix loop (`internal/safety/**`) | A/B/C fix commit `39e21cf` (on `278f335`, merged main `d9a5baa`) on `fix/safety-ruleset-5` as draft PR #8 to main ("WIP — DO NOT MERGE"); 2707 vectors. The latest independent review's P1 findings A/B/C are fixed (positive "kembali lagi", "tidak akan lama" followed by a no-return, 512-byte post-exemption lookahead with 119/120/121/240-byte boundary tests). `go test -race -count=1 ./internal/safety ./internal/journey ./internal/httpapi` passes with database tests required; the -4/-5 differential over the 1329 shared vectors has zero lower and zero higher routing. Independent re-verification is still pending (D-034 status) | S-150 |

- **Documentation link check** (P1 exit criterion 3; backlog S-273). Command: `python3 -I linkcheck.py . $(git ls-files -co --exclude-standard '*.md')`, where `linkcheck.py` is a session script kept outside the repository (no committed checker or CI step exists yet). It checks every Markdown file Git tracks or would add (139 files, `docs/archive/` included): inline links and reference-style link definitions to local files, and their heading anchors as GitHub-style slugs; external URLs are not fetched. Run 2026-10-26 on the tree of this documentation pass, again after its status updates for PR #9, and again after the D-056 follow-up corrections (same result each time). Result: 0 missing anchors and one reported missing file, a known false positive: the CSS selector `:not(main):focus` inside a code block of [design direction](design/DESIGN_DIRECTION.md), which the script reads as a link. It must be re-run on the commit of the delivery branch.

### Active work and next steps

This is the repository checkpoint for resuming work (brief §14). It names roles, not people. Each workstream edits only its own files.

| Workstream | Owner role | Files owned | Next step | Risk |
| --- | --- | --- | --- | --- |
| P1 spec and planning documents (S-273) | PM | `SPEC.md`, the decision log, decisions, blockers, conflict matrix, document inventory, product, safety, feature matrix, provenance, AI policy, Claude verifier, Meta integration, ADR-0003, both READMEs; the backlog, this file, traceability and the roadmap | Re-run the link and anchor check on the commit of the integration pass (§1); confirm or withdraw the [SPEC §G foundation status note](../SPEC.md#foundation-status-note-2026-10-26) now that the CI run is recorded (§1); confirm the milestones given to S-262 criterion 3 (the in-app notice in P2, the status-page post in P7); settle the open alignment points ([conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) rows 49, 50, 52, 53, 56, 57, 59, 60, 61; two PR-template sections, traceability DL-3). Done: the integration pass is committed on `feat/sela-v3-pilot` (draft PR #9); `golang.org/x/text`, the restore sentence of `terms-privacy-2026-11-01.2` as written (including its second clause), the `guest_storage` text `guest-storage-2026-11-01.2`, the guest case of ADR-0002 §11 and the acceptance-criteria list in the description of PR #9 are confirmed by the PM ([D-056](DECISION_LOG.md#d-056); row 64 closed) | Documents drift from code while several branches are unmerged; every status claim must cite evidence |
| P2 backend foundation (S-002 to S-005, S-171 to S-177, S-211, S-260 to S-262; S-274 planned) | Go backend engineer | ADR-0002, `docs/API_SPEC_V3.md`, `docs/RUNBOOK.md`, `docs/ARCHITECTURE.md`; `internal/db`, `internal/vault`, `internal/account`, `internal/httpapi/v3_*.go`, `internal/config`, `internal/fixtures/v3/` and `policies_v3.go`, `cmd/sela/`, `scripts/ops/`, `go.mod`, `go.sum`, `.github/workflows/v2.yml`, `deploy/v2/Dockerfile` | Done: the branch is validated in a clean checkout, pushed, and its CI run on draft PR #9 is recorded (§1). Next: refine, if needed, the wording of the guest case that the integration pass added to ADR-0002 §11 and runbook §8 (its meaning is confirmed by the PM, [D-056](DECISION_LOG.md#d-056)); the operator audit record (S-274, row 57) and the demo test-double alignment (row 52); a `sela-eval` step in `v2.yml` if the PM wants the roadmap gate to cover it in CI | The hosted parts wait on B-3 and B-15; the deletion journal that closes the restore window needs the backup storage (D-055) |
| Design (S-190) | UX designer (design lead) | `docs/design/` | Record the contrast run and the PM review against brief §4; review the backup-model copy that the integration pass wrote for [conflict matrix](CONFLICT_MATRIX.md) row 62 and widen the DeepSeek consent card to every purpose it serves ([content style §7.1](design/CONTENT_STYLE.md#71-consent)); specify the state and copy of the in-app restore notice (backlog S-262 criterion 3, P2); align the "don't save" copy with D-051 ([conflict matrix](CONFLICT_MATRIX.md) row 56) and settle the other design open points ([design direction §10](design/DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners): share expiry, the 15 vs 16–17 age question) | A cosmetic redesign accepted as a structural one; the open points block P3 and P4 copy |
| Safety ruleset -5 (S-150, SPEC T22) | AI engineer (safety) | `internal/safety/**` on branch `fix/safety-ruleset-5` (draft PR #8 to main; the fix loop continues in its working tree), not on this branch | The PM's route is a new PR against main (D-034 status, [conflict matrix](CONFLICT_MATRIX.md) row 63): finish the fix loop and its independent re-verification, and commit the result on `fix/safety-ruleset-5` for PR #8, whose A/B/C fix commit is `39e21cf` (2707 vectors). Once PR #8 is merged, `-5` reaches this branch with the next merge of main | The open gaps from the ruleset -4 re-verification (§4.3); false positives that interrupt ordinary talk |
| P2 frontend (S-001, S-006, S-152, S-153, S-170, S-191, S-193, S-194, S-195 for P2 screens) | Frontend engineer; UX designer | `web/**`, `e2e/**` | In progress, uncommitted: design system, app shell, sign-in, settings. Next: guest start, onboarding and the home skeleton (SPEC T15), the WCAG 2.2 gate (S-194) and the browser review with screenshots (S-195); then commit | The v2 pages still exist; they stay only in development, test and demo (ADR-0002 §12) |
| AI provider layer, research, P3 listening backend, operations | AI engineer; research engineer; Go backend engineer | Their own branches (§1 table) | Finish, verify and merge into `feat/sela-v3-pilot` in the PM's order (the AI provider layer through draft PR #10; the research and reading work through draft PR #11 to main and the next merge of main); each merge updates the backlog, this file and traceability | Merge conflicts in shared packages (`internal/account`, `internal/httpapi`, `cmd/sela`, `internal/llm/fakellm`) |
| Owner actions | Owner | — | Before 2026-11-04 21:00 ET: confirm the submission build and ask the organizers (B-12; [roadmap §3](ROADMAP.md#3-dated-checkpoints)); the GPU budget and Modal credentials (B-1). Before any pilot: B-1, B-3, B-5, B-8, B-10, B-15, and B-9 only if the pilot enables DeepSeek. With the Dengar.in team: B-13 | "Verified live" and the controlled pilot cannot be reached without them |

## 2. v3 status per milestone

Milestone order and exit criteria: [backlog §4](BACKLOG.md#4-milestones).

### P1 Audit and spec alignment

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| Dengar.in feature matrix (keep, improve, merge, migrate, retire, paused per feature) | S-273 | in progress | `docs/DENGARIN_FEATURE_MATRIX.md`, committed with the planning documents (`4cadbfd`) |
| ADR-0002 persistence, accounts, encryption | S-273 | in progress (the ADR); the design it describes is verified locally (P2) | committed with the foundation (`0ff3341`); the D-054, D-055 and D-056 consistency fixes come with the integration pass |
| ADR-0003 AI routing, verifier, fallback (accepted as design) | S-273 | in progress | committed (`4cadbfd`, aligned with the owner's fallback contract in `59dba14`, D-053) |
| API v3 foundation contract | S-273 | in progress (the contract); the routes it specifies are verified locally (P2) | `docs/API_SPEC_V3.md`, committed with the foundation (`0ff3341`) |
| Backlog, implementation status, traceability, roadmap | S-273 | in progress | committed (`4cadbfd`, `59dba14`) and updated by the integration pass |
| AI policy, Claude verifier, Meta integration, provenance brought to v3 | S-273 | in progress | committed (`4cadbfd`, `59dba14`) |
| SPEC amendments (C3, C7, C8, C9, C12, V1, V2, V9, V10, V16, V23, I.data, I.meta); D-026 to D-056 in the decision log (owner direction D-026; PM decisions D-027 to D-050 and D-054 to D-056; engineering defaults D-051 and D-052; merge record D-053); B-8 to B-15 in the blockers; document inventory and index | S-273 | in progress | `SPEC.md`, `docs/DECISION_LOG.md`, `docs/DECISIONS.md`, `docs/BLOCKERS.md`, `docs/DOCUMENT_INVENTORY.md`; link and anchor check run 2026-10-26 on the tree of the integration pass, 0 broken over 139 files apart from one known false positive (§1); to be re-run on the commit of the delivery branch |

### P2 Foundation

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| PostgreSQL 16, migration runner, CI database service, Go 1.26 toolchain (D-054) | S-260 | verified locally (CI passed on draft PR #9) | `internal/db`, `cmd/sela`, `.github/workflows/v2.yml`, `go.mod`; F-01 to F-09 and F-46 pass (foundation commit `0ff3341`, recorded local and CI runs in §1) |
| Environment separation and startup guards | S-261 | in progress (guards verified locally) | `internal/config`; F-10 and F-11 pass (foundation commit `0ff3341`, recorded run in §1); the demo still refuses the test double ([conflict matrix](CONFLICT_MATRIX.md) row 52), and the demo criteria (S-261 criteria 2 to 4) need the demo build |
| Per-user envelope encryption, key rotation, crypto-shredding | S-173 | verified locally (CI passed on draft PR #9) | `internal/vault`; F-12 to F-15 pass (foundation commit `0ff3341`, recorded local and CI runs in §1) |
| Guest start, accounts, recovery, sessions, age band, guardian consent record | S-002, S-003, S-004, S-005, S-172, S-211 | in progress (backend verified locally) | `internal/account`, `internal/httpapi/v3_*.go`; F-16 to F-34, F-36 and F-37 pass (foundation commit `0ff3341`, recorded run in §1); the screens are not done |
| Consents, audit, export, account deletion | S-171, S-174, S-175, S-176 | in progress (backend verified locally) | same packages; F-35 and F-38 to F-43, F-45 and F-47 pass (foundation commit `0ff3341`, recorded run in §1); the screens and the feature parts are not done |
| v3 policy texts | S-177 | in progress (foundation texts verified locally) | `internal/fixtures/v3/policies.json` (`terms_privacy` now `terms-privacy-2026-11-01.2`, D-055; `guest_storage` now `guest-storage-2026-11-01.2`; the `terms_privacy` restore sentence as written and the `guest_storage` text confirmed by the PM, [D-056](DECISION_LOG.md#d-056); both versions come with the integration pass, committed on `feat/sela-v3-pilot`, draft PR #9); F-33 and `TestTermsPrivacyStatesBackupWindows` pass; the Pasal 21 element test and the per-provider texts are not done; legal review blocked externally (B-10) |
| Backup, restore and drill scripts (local); the in-app restore notice | S-262 | in progress: criterion 1 verified locally (CI passed on draft PR #9); criterion 3, the restore announcement that `terms-privacy-2026-11-01.2` promises, planned (the in-app notice in P2, with its state and copy from S-190; the status-page post in P7, with S-263); criterion 2, the hosted drill, blocked externally (B-3, B-15) | `scripts/ops/`; F-44 passes (foundation commit `0ff3341`, recorded local and CI runs in §1); the restore revival window is accepted for a controlled pilot (D-055); the notice has no design state, code or e2e test yet |
| Operator access audit (D-052, SPEC V32) | S-274 | planned; hosted access log blocked externally (B-3) | —; not part of the foundation commit; ADR-0002 §13 and the runbook do not describe the record yet ([conflict matrix](CONFLICT_MATRIX.md) row 57) |
| Safety ruleset -5 | S-150 | in progress | A/B/C fix commit `39e21cf` on `fix/safety-ruleset-5` (draft PR #8 to main), 2707 vectors; independent review findings A/B/C fixed, re-verification pending; PR #6 is merged into main with `-4` |
| Design direction, information architecture, screens, content style, accessibility criteria | S-190 | in progress | `docs/design/DESIGN_DIRECTION.md`, `INFORMATION_ARCHITECTURE.md`, `SCREENS.md`, `CONTENT_STYLE.md`, `ACCESSIBILITY_CRITERIA.md`, `tokens.css`, `tools/`, committed with the planning documents (`4cadbfd`); contrast run and PM review not yet recorded |
| Design system and app shell with help and Quick Exit, bilingual base | S-153, S-191, S-193 | in progress; implemented and locally checked 2026-10-28 | `web/src/v3/` and [V3_FRONTEND_VALIDATION.md](V3_FRONTEND_VALIDATION.md); owner/browser and screen-reader review pending |
| Welcome, onboarding consents, storage choice, help page, WCAG 2.2 AA gate, browser review of the P2 screens | S-001, S-006, S-152, S-170, S-194, S-195 | in progress; screens implemented and locally checked 2026-10-28; the automated WCAG 2.2 CI gate is not yet wired | `web/src/v3/`; [V3_FRONTEND_VALIDATION.md](V3_FRONTEND_VALIDATION.md); owner/browser and manual review pending |

### P3 Listening, notes and timeline

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| Conversation room, reply rules, streaming accessibility, reflection cards, no-AI path, intent picker | S-007, S-020 to S-024 | S-020 in progress (backend, branch `feat/sela-v3-listening`, not merged); the rest planned | — |
| Journal, prompts, reflect with Sela, check-in | S-050, S-051, S-052, S-060 | S-050 and S-060 in progress (backend, branch `feat/sela-v3-listening`, not merged); the rest planned | — |
| Timeline, search, memory, home | S-040, S-070, S-071, S-080 | S-070 and S-080 in progress (backend, branch `feat/sela-v3-listening`, not merged); the rest planned | — |
| Crisis screening on v3 routes, inline crisis support | S-150, S-151 | planned | — |
| Routing core with the Meta path, output guards, data minimisation, Meta-only demo profile | S-220, S-221, S-223, S-226, S-227 | in progress (branch `feat/sela-v3-ai`, not merged); live Meta blocked externally (B-1); organizers' answer pending (B-12) | — |
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
| Routing complete with three labelled test doubles | S-220 | in progress (branch `feat/sela-v3-ai`, not merged) | — |
| Claude Sonnet 5.5 verifier (from `prototypes/evidenceguard`, which has 6 mocked tests and is not wired) | S-222 | in progress (branch `feat/sela-v3-ai`, not merged); live blocked externally (B-8) | — |
| DeepSeek fallback generator under the owner's contract (C13, V19–V21; reference implementation `prototypes/generationfallback`, 9 mocked tests, not wired; the v2 task T5b is carried into T18) | S-224 | in progress (branch `feat/sela-v3-ai`, not merged); live blocked externally (B-9; PRC processing needs B-10) | — |
| AI summary wording, memory suggestions | S-081, S-091 | planned | — |
| Evaluation EV-0 to EV-7 | S-228 | in progress (branch `feat/sela-v3-ai`, not merged); live runs blocked externally (B-1, B-8, B-9) | — |
| P5 parts of spanning items: Anthropic and DeepSeek consent purposes, browser review of the P5 screens | S-171, S-195 | planned | — |

### P6 Research and evidence

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| Corpus hygiene, unchecked dates, schema fields | S-240, S-241, S-242 | S-240 and S-242 in progress (branch `feat/sela-v3-research`, draft PR #11 to main, not merged); S-241 planned | S-240: `268a101` pins LF for the hashed outputs and register documents (`data/research/.gitattributes`, `docs/research/.gitattributes`) and adds dated superseded notes to docs 06 and 10; the CI path filter and the `Research.tsx` flags are not done |
| v3 gap searches, evidence map, claim-support checks | S-243, S-244, S-245 | in progress (branch `feat/sela-v3-research`, draft PR #11 to main, not merged) | — |
| Human appraisal | S-246 | blocked externally (B-4) | — |
| Reading library and "latihan kecil" | S-140, S-141 | S-140 in progress (branch `feat/sela-v3-research`, draft PR #11 to main, not merged); S-141 planned | — |

### P7 Accessibility, security and operations

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| Manual accessibility review, display settings, slow connections | S-192, S-195, S-196 | planned | — |
| Voice dictation, read-aloud and voice room | S-025, S-026, S-027 | retired (D-060) | — |
| Security review before the pilot, observability | S-265, S-266 | S-265 in progress (documents, branch `feat/sela-v3-ops`, not merged); S-266 planned | — |
| Hosted deployment, rollback and restore drill; the status page (S-263), on which the restore post of S-262 criterion 3 (planned) depends; the database host's access log for operator sessions | S-262, S-263, S-274 | blocked externally (B-3, B-15) | deployment files in progress on branch `feat/sela-v3-ops`, not merged |
| Meta model on Modal; live verification of Meta as the generator | S-264, S-221 (live) | blocked externally (B-1) | — |
| Help-line re-verification | S-154 | blocked externally (B-5) | — |
| Legal readiness (DPIA, DPO, processors, cross-border basis, PSE registration, breach runbook) | S-178 | blocked externally (B-10, B-15) | drafts in progress in `docs/compliance/` on branch `feat/sela-v3-ops`, not merged |
| Minors: experience, guardian consent screens, evaluation, legal and provider gate | S-210, S-211 (UI), S-212, S-213 | planned; production gate blocked externally (B-10) | — |
| Legacy Dengar.in deployments | S-267 | blocked externally (B-13) | — |

### P8 Final integration and pilot readiness

| Work | Items | Status | Evidence |
| --- | --- | --- | --- |
| Final browser review and screenshots | S-195 | planned | — |
| `OWNER_TESTER.md` | S-270 | planned | — |
| Pilot readiness report and pilot plan | S-271 | planned | — |

## 3. Not done in v3 (and why)

- **No v3 user-facing feature exists.** There is no conversation room, journal, standalone check-in, cross-session timeline, memory view, multi-entry consultation summary, purpose-bound sharing, v3 recipient view, after-consultation reflection, reading library or home. They are planned for P3 to P6; the backend of the P3 surfaces is in progress on a separate branch, not merged.
- **The foundation is verified locally, not live.** F-01 to F-47 pass in the recorded local run and in the CI run on draft PR #9 (§1); the hosted database, secret manager and backup storage, and the hosted restore drill are not done (B-3, B-15), the restore announcement of S-262 criterion 3 is planned, and the operator audit record (S-274) is planned.
- **No v3 frontend code is committed.** The design documents are committed but not yet reviewed against brief §4; the app shell is in progress, uncommitted; the running frontend is v2, which still shows demo copy (synthetic data, adults only, memory only).
- **The spec realignment is on the delivery branch.** The SPEC.md amendments, decisions D-026 to D-053 and blockers B-8 to B-15 are committed with the planning documents (`4cadbfd`) and the merge of main (`59dba14`), both on the remote delivery branch `origin/feat/sela-v3-pilot` in draft PR #9. The integration pass, which adds D-054 to D-056 and these status updates, is committed on `feat/sela-v3-pilot` (draft PR #9); the link and anchor check passed on its tree (§1) and is re-run on that commit.
- **No multi-provider AI.** The v2 runtime and the committed v3 code have no AI router, no wired verifier and no DeepSeek code (the AI provider layer is in progress on a separate branch, not merged) (the v1 TypeScript orchestrator has its own DeepSeek provider, wired only into v1 and not migrated); the owner's DeepSeek fallback (C13, V19–V21) exists only as the standalone, default-OFF prototype `prototypes/generationfallback` (mock-tested, not wired; T5b open). No live call to Meta, Anthropic or DeepSeek has been made (B-1, B-8, B-9); the generator choice is still open (B-7).
- **No WCAG 2.2 evidence.** The v2 e2e scan uses WCAG 2.0/2.1 tags, fails only on serious or critical findings, checks 390 px instead of 320 px, and every page has the same title (S-194).
- **Ages 15–17 are not built** beyond the foundation's age band and guardian consent record (backend verified locally, S-005, S-211), and their production use is blocked by the legal review (B-10).
- **Legal preconditions for real users are open:** DPIA, DPO, processor agreements, cross-border basis, PSE registration and breach runbook (B-10); hosting and region (B-3, B-15); email for recovery does not exist (B-11, optional).
- **Help lines need owner decisions:** a read-only check on 2026-10-26 found differences from the fixtures (Healing119 hours and WhatsApp, LISA cost, Yayasan Pulih contacts) (B-5).
- **Research does not cover the v3 core:** no direct records on listening, journaling, consultation preparation or accessibility; no adolescent consent or confidentiality evidence; no human appraisal (B-4).
- **Outside the repository:** two legacy Dengar.in deployments are live and may process real people's text (B-13); the hackathon organizers' position on non-Meta providers is unknown (B-12).

## 4. v2 evidence that remains valid (v2 scope)

This section describes branch `feat/v2-go-meta-runtime` (PR #6, merged into main as `7fc6678` on 2026-10-26): a synthetic, adults-only, memory-only, Meta-only demo. v3 supersedes those limits as product scope (D-026), but the tests below remain the baseline that v3 must keep passing or replace explicitly. In v2 wording, "tested locally" means the automated suites in this repository pass on that branch (CI runs the same commands except `sela-eval`); it corresponds to "verified locally" for v2 scope. Nothing below is deployed.

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
| CI on main after the PR #6 merge (`7fc6678`, `v2.yml` run 40375830529, 2026-10-26 18:13 to 18:27 UTC) | **Failed** in one job, "End-to-end journeys (test double, never Meta)": 13 of 14 journeys passed, and "Keyboard-only start: skip link, checkbox and submit" failed with "first tab stop is not the skip link". The other four jobs and the Quality gate (run 40375830956) passed. The same journey passed locally on `0ff3341` (§1) and in the later `v2.yml` runs on PR #8 (40404566325, `f619677`) and PR #9 (40405475962, `59dba14`), both built on `7fc6678`, so it is probably a flaky test; no re-run on main is recorded. For the web and e2e owner (frontend engineer) |
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
| Safety lexicon (four review rounds) | Request changes | round 1: own quoted words attributed to others, inner-self death wishes, variant-index bug, non-linear frames, look-alike letters; round 2: accident/idiom exceptions too broad, lyrics/report frames, leetspeak with 1 and \|, slang and spacing gaps, worst-case timing | Round 1 fixed (`ee01781`, ruleset `-2`, 420 vectors); round 2 first committed as `d187977` (ruleset `-3`), whose re-verification found 95 crisis inputs it routed lower and quadratic loops; two more fix and independent re-verification rounds fixed those (ruleset `-4`, 1329 vectors, linear time, 23 ms worst case at 12,000 characters; against `-3` only contrived space-joined variants route lower). The last re-verification still lists four major gaps, three of which v1 misses too (hope/wish phrasings with words between the verb and the death word, intensifiers in the same position, present or future overdose plans) and one in the new accident-marker gate (emoticons and variant spellings), plus minor adoption, method-lexicon and false-positive items; **ruleset `-5` is in progress** on draft PR #8 (fix commit `39e21cf`, 2707 vectors; the latest P1 review findings A/B/C are fixed, independent re-verification pending; backlog S-150). |
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
