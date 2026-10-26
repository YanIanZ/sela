# Implementation status

Updated: 2026-10-26 · branch `feat/v2-go-meta-runtime`. Requirement-level detail: [requirements traceability](REQUIREMENTS_TRACEABILITY.md). External blockers: [blockers](BLOCKERS.md). Nothing below is deployed; "tested locally" means the automated suites in this repository pass on this branch (CI runs the same commands).

## Summary

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

## Latest local results

Recorded from runs on this branch; see the PR for CI.

| Check | Result |
| --- | --- |
| `gofmt -l cmd internal`, `go vet ./...` | clean |
| `go test -race -count=1 ./...` | pending final run |
| Web: typecheck, Vitest (i18n parity, hard-coded strings, claim gates, components), build | pending final run |
| Python: `tests/research`, `tests/modal`, corpus `verify` | pending final run |
| e2e (`node e2e/journey.e2e.mjs`, test double) | pending final run |
| `sela-eval -mode offline` / `-mode test_double` | pending final run |

## Adversarial reviews

Each package was implemented, then reviewed by an independent agent that tried to break it, then fixed. Reviewers worked on copies and reported evidence.

| Area | Review verdict | Main findings | Outcome |
| --- | --- | --- | --- |
| Story Ledger / Meaning Lock | Request changes | wording negation checked as a whole-text flag; quotes/brackets ended negation scope; opening lines could reverse a negated item; informal negators (gabisa, dont); wording could add new content; medication/diagnosis lexicon gaps; clinician-attribution exception abusable; quadratic validation on long wording | Fixed with a regression test per finding (`59ebef8`, `85e8923`); validator `sela-meaning-lock-2026.10.26-2` |
| Modal serving app | Approve with fixes | smoke test forwarded credentials on redirects; budget did not bound wall time; malformed chunk crash; snapshot repair could not re-download; report could include generated text | Fixed (`7a808fd`, `c6f4e4f`); 72 offline tests; nothing deployed |
| Research pipeline | Changes requested | country/population labels taken from negated register notes; claim gates flagged required disclaimers; default freeze date conflicts with committed snapshot; verify ignored stale inputs | Fixed (`5bdb395`); pipeline 1.1.0, rules v2, gates v2 |
| LLM client and test double | Request changes | server text could reach error messages via `finish_reason`; health fallback matched model paths by substring; test-double negators drifted from the ledger | Fixed (`9762f66`, `40921d1`); drift tests pin the test double to the ledger |
| Safety lexicon (four review rounds) | Request changes | round 1: own quoted words attributed to others, inner-self death wishes, variant-index bug, non-linear frames, look-alike letters; round 2: accident/idiom exceptions too broad, lyrics/report frames, leetspeak with 1 and \|, slang and spacing gaps, worst-case timing | Round 1 fixed (`ee01781`, ruleset `-2`, 420 vectors); round 2 first committed as `d187977` (ruleset `-3`), whose re-verification found 95 crisis inputs it routed lower and quadratic loops; two more fix and independent re-verification rounds fixed those (ruleset `-4`, 1329 vectors, linear time, 23 ms worst case at 12,000 characters; against `-3` only contrived space-joined variants route lower). The last re-verification still lists four major gaps, three of which v1 misses too (hope/wish phrasings with words between the verb and the death word, intensifiers in the same position, present or future overdose plans) and one in the new accident-marker gate (emoticons and variant spellings), plus minor adoption, method-lexicon and false-positive items; **a further fix round is running**. |
| Journey, store and HTTP transport | Request changes (33 findings) | crisis gate ordering, stale AI commits, op-runtime purge, recipient principal caps, per-session and per-client limits, retention-aware eviction, revoke, strict decoding and routing, limiter eviction, SSE shutdown | Fixed and re-verified by an independent re-run of the probes (`0b57dba`, `ef53548`, `b8bc4ce`) |
| Frontend | Request changes | opening-line provenance, stale share previews, recipient token handling, session refresh, stream reconnect, crisis routing, catalog fallback, focus handling | Fixed (`a483448`, `ebca156`); 14 e2e journeys |
| CodeQL (GitHub code scanning) | 10 alerts on PR #6 | static file path, clear-text and log-entry logging; research and eval paths from operator configuration | Static files through `os.Root` and named-field logging (`a33607a`); the rest have operator-controlled sources and are left for the owner to dismiss or keep (threads open on the PR) |

## Not done (and why)

- **Live Meta inference** on Modal: no Modal credentials or GPU budget in this environment ([B-1](BLOCKERS.md)). No `live_meta` output has ever been produced; local runs use AI off or the labelled test double.
- **Live metrics**: bilingual wording quality, important-fact coverage, warm latency, cold start and cost per session are pre-registered but unmeasured.
- **Public deployment** of the Go app ([B-3](BLOCKERS.md)); the container image is built and smoke-tested in CI.
- **Human research appraisal** ([B-4](BLOCKERS.md)); every record is `reviewer_status: not_reviewed`.
- **Cutover (T9)**: v1 stays runnable until v2 is accepted with live evidence.

## Cost estimate (assumptions)

Modal H100 list price $0.001097/s (≈ $3.95/h), observed 2026-10-26. One demo session ≈ one cold start (minutes, unmeasured) + three AI operations + the 5-minute scale-down window. With an assumed 4-minute cold start and 30 s per operation that is ≈ 10.5 GPU-minutes ≈ $0.70 per isolated session; consecutive sessions within the scale-down window share the warm container. Weights download runs on CPU. Details and ceilings: [modal/README.md](../modal/README.md#cost) and [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md). These are estimates, not measurements.
