# Evaluation protocol

Authority: [SPEC](../../SPEC.md) I.eval, C10, V7, V13, V15. Thresholds: [`eval/thresholds.json`](../../eval/thresholds.json), **registered 2026-10-26 before any final run** (commit `e23e0e0`). Results are written to `eval/results/` by `cmd/sela-eval`; the summary below is updated only from those files.

## What is and is not evaluated

Engineering properties of the software on **synthetic** fixtures: safety routing, Meaning Lock grounding, authorization, retrieval, bilingual wording, latency/cost and accessibility. **Not evaluated, and not claimed:** clinical validity, therapeutic benefit, reduction of symptoms, suicide prevention, safety certification, real-user usability or hackathon eligibility. Test-double runs measure the pipeline, never Meta model quality.

## Suites

| Suite | How | Mode |
| --- | --- | --- |
| Safety lexicon | Every vector in `internal/safety/testdata/vectors.json` (v1 regression corpus plus new negation, quotation, slang, code-switching, third-party, ambiguous and idiom cases): crisis recall, elevated recall (elevated or crisis), idiom false-positive rate; exact agreement by tag reported | all |
| Crisis path | Crisis fixture with AI consent: count model calls (must be 0) | all |
| Gold qualifiers | For each gold fact in `internal/fixtures/data/scenarios.json`, deterministic `ledger.Analyze` vs annotated negation/subject/time/certainty | all |
| Authorization battery | Token without code, wrong token, cross-session revoke/read/check-in, out-of-order and out-of-scope recipient actions, access after expiry, revocation and owner deletion, cross-visitor effects | all |
| Retrieval | 15 feature-level reviewer queries (`eval/retrieval_queries.json`): expected record in top 5; every R-id cited by the Relay docs and evidence map must exist | all |
| Ledger pipeline | Each seeded scenario in Indonesian and English UI: residual unsupported items (quote not verbatim in its segment), forbidden-pattern hits in committed wording, raw rejected rate; coverage and language match (live only) | test_double, live |
| Fault injection | Test double faults: malformed JSON, fabricated source, dropped negation, diagnosis, invented number, prompt-injection echo, wrong model, HTTP 503, truncated and oversize streams. Each must end in an honest error or in rejected/reverted claims with no forbidden content | test_double |
| Latency and cost | Warm p50/p95 of ledger drafts, first-request (cold) latency, cost estimate at H100 list price including scale-down | live |
| MIE live ablation | `sela-eval -mie-live` (`run`, `sheet`, `labels`, `score`) on `eval/mie/cases.jsonl`: configs B0/B1/B2/T of the Meaning Integrity protocol with Muse Spark (generator) and Muse Wonder (verifier); first-pass unsupported-interpretation rate, correction leakage after a scripted rejection, gold-candidate false acceptance, latency, tokens and cost. Thresholds version `sela-eval-thresholds-2026.11.02-1` (harness `sela-eval-mie-live`, registered 2026-11-02 before any live call of the benchmark; adds metrics, changes none). Results and limits: [MIE_LIVE_BENCHMARK](MIE_LIVE_BENCHMARK.md) | live |
| Accessibility | axe-core WCAG 2.1 A/AA on audited pages in `e2e/journey.e2e.mjs` (serious/critical must be 0) | e2e |
| i18n | Key parity, placeholders, no hard-coded strings (`web/src/test/*.test.ts`) | unit |
| Community moderation (S-036) | 222 synthetic posts and replies (`eval/moderation/cases.jsonl`, 10 strata, id/en/mixed, dev/tune/heldout by family) through the D-072 publishing pipeline: benign false-positive rate, harmful false-negative rate and crisis routing recall with 95% Wilson intervals, per split, stratum and language. Thresholds version `sela-eval-thresholds-2026.11.02-2`, registered before held-out was scored. Review: [MODERATION_FALSE_POSITIVE_REVIEW](MODERATION_FALSE_POSITIVE_REVIEW.md) | rules: offline, `-mode moderation`; full pipeline: `-mode moderation_live` (live Meta, `-max-calls` cap) |

Gold annotations, scenarios and forbidden patterns are fictional and were written for this demo; they are a small development set (three seeded scenarios, 24 facts). They are not a held-out benchmark and are too small for statistical claims; results are descriptive.

## Results

Populated from `eval/results/*.json` — see [implementation status](../status/IMPLEMENTATION_STATUS.md) for the latest run and its commit.
