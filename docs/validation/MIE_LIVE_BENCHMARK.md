# MIE live Meta benchmark (B0 / B1 / B2 / T)

Status: **measured once, 2026-11-02, on synthetic data only.** Roadmap items H-010 to H-013. Protocol: `docs/hackathon/EVALUATION.md` on branch `docs/meta-hackathon-2026-perfect-score-plan`. Thresholds: `eval/thresholds.json` version `sela-eval-thresholds-2026.11.02-1` (harness `sela-eval-mie-live`). Result files: `eval/results/20261102T014146Z-mie-live.{json,md}` and `-records.jsonl` (metadata only). Labels: `eval/mie/live_labels.jsonl`.

The short version: **every preregistered held-out target passed, but the run shows almost no headroom.** Muse Spark made no unsupported first-pass interpretation (0/128) and never repeated a rejected meaning (0/68), so B1, B2 and T could not lower those rates below B0's. The run shows three things clearly. B0 has no gate, so it would release every distorted candidate (68/68), while B1 and B2 stopped all of them (0/68). The checks withhold some faithful outputs. And the B1 Meaning Diff withholds most no-graph outputs once a correction is in the context. **It does not show that the MIE reduces unsupported inferences from Muse Spark on this dataset.** The dataset is too easy for the generator to show that.

## 1. What was run

| Item | Value |
| --- | --- |
| Dataset | `eval/mie/cases.jsonl`, sha256 `90bf7057…6564`, 128 synthetic cases from 60 paraphrase families. Splits: dev 75, tune 35, held-out 18 (9 critical, 9 benign, 6 families). Languages: id 70, en 36, mixed 22 |
| Generator | `muse-spark-1.3`, Meta Model API (`api.meta.ai`), `reasoning_effort` minimal, production AT-2 reflection prompt `sela-ai-prompts-2026.11.01-2` via `ai.EvalPrompt`, production parser `ai.EvalParse`, `max_tokens` 1024, 1 retry, provider sampling defaults |
| Verifier | `muse-wonder-1.0`, production contract `sela-verifier-v1` / `sela-verifier-instructions-2026.10.26-1`, VC-8 release rules (`verifier.Decide`, withhold on any not-released claim), 20 s attempt (AT-2 policy), 1 retry |
| Go checks (B1) | Production output guards `sela-ai-guards-2026.11.01-1` + Meaning Diff `sela-meaning-2026.11.01-1` (`ledger.CompareMeaning` against the source): any blocked guard or critical difference withholds |
| Served models | Every response reported the configured model: generator `muse-spark-1.3` ×196, verifier `muse-wonder-1.0` ×324. No model mismatch |
| Code | Worktree based on `198b134`, plus uncommitted evaluator changes (`cmd/sela-eval/mielive*.go`, `internal/ai/evalhooks.go`) |
| Command | `sela-eval -mie-live run -mie-raw <dir outside repo> -mie-env <env file> -mie-concurrency 3`, then `sheet`, `labels`, `score` |

### Configurations

All arms share **one generation per case**. They differ only in what happens after it, so prompt, input, settings and model are identical by construction.

| Arm | Phase p1 (interpretation of the source alone) | Phase p2 (after a scripted rejection) | Gold gating (labelled candidate as if generated) |
| --- | --- | --- | --- |
| B0 | Release every valid generation | The rejected interpretation stays in context as an `earlier_reply`, followed by the user's fixed correction message ("Bukan itu maksudku…" / "That's not what I meant…"). Release every valid generation | Release everything (FAR 1 by construction, no call needed) |
| B1 | B0 + release only if the Go checks pass | Same generation as B0 + Go checks | Go checks |
| B2 | B1 + release only if the Meta verifier releases | Same generation as B0 + Go checks + verifier | Go checks + verifier |
| T | = B2 machine stage. The human review of the new output is **not** scored (no oracle) | **Graph exclusion**: the rejected claim is never context, so T's request is byte-identical to the p1 request (checked by request hash for all 68 cases) and the p1 generation and verification are reused. Then B2 gating | = B2 |

What is scripted and what is model output in T:
- The **review event** is scripted. In p2 the user rejects the earlier interpretation of every critical case, and both the baseline and T receive this rejection. The baseline gets it as a chat message, T gets it as a graph decision.
- The **graph effect** is deterministic: exclusion from context (H-008, "stale claims are never AT-1 context"). Persistence and propagation are covered by `TestMeaningCorrectionPropagates` and were not re-run live.
- **Everything after that is live model output**, gated by the same machine checks as B2.
- T gets **no human approval of new outputs**. Its release numbers are machine-stage numbers before review.

Differences from production AT-2:
- Every valid output that passes the Go checks is verified. Production verifies AT-2 only `when_flagged`.
- There is no regeneration after a guard or Meaning Diff rejection. Production regenerates once.
- The deterministic crisis pre-screen is not run. None of the MIE sources is a crisis fixture.

### Labelling (ground truth for p1/p2 outputs)

The 196 valid generations (128 p1 + 68 p2) were labelled with a rubric fixed before the sheet was opened (§6). The sheet showed an opaque id, the source, the output and, for critical cases, the rejected interpretation. It was shuffled with a fixed seed. Arm, case id, split, Meaning Diff and verifier results were hidden.

**The labeller was a single AI agent (Claude Opus 5.5, the agent that ran the benchmark), not a human. There is no second labeller and no kappa, and the owner has not adjudicated the labels.** Each label is bound to its output by text sha256. The raw outputs stay outside the repository, in the run's scratch directory. Gold-gating metrics need no labels: they use the dataset's own `expected_critical` labels, which have the same one-author caveat as version 2026.11.01-1.

## 2. Results

Wilson 95% intervals. n is the denominator. "Released" means released by the machine stage, before user review.

### Held-out (judged against the preregistered thresholds)

| Metric | B0 | B1 | B2 | T | Target | Verdict |
| --- | --- | --- | --- | --- | --- | --- |
| p1 UIR (unsupported / released) | 0/18 (0.00–0.18) | 0/16 (0.00–0.19) | **0/16 (0.00–0.19)** | = B2 | B2 ≤ 0.05 | pass |
| p1 unsupported released per case | 0/18 | 0/18 | 0/18 | = B2 | benefit B0−B2 ≥ 0 | pass (0.000, family bootstrap 0.000..0.000) |
| p1 unnecessary abstention (faithful withheld / faithful valid) | — | 2/18 (0.03–0.33) | **2/18 (0.03–0.33)** | = B2 | B2 ≤ 0.2 | pass |
| p2 correction leakage (released outputs reasserting the rejected meaning / rejected cases) | 0/9 (0.00–0.30) | 0/9 | 0/9 | **0/9 (0.00–0.30)** | T ≤ 0 | pass |
| p2 released rate | 9/9 | 0/9 | 0/9 | 8/9 | — | info |
| Gold false acceptance, critical | 9/9 (0.70–1.00) | 0/9 | **0/9 (0.00–0.30)** | = B2 | B2 ≤ 0 | pass |
| Gold benign unnecessary abstention | 0/9 | 1/9 | **1/9 (0.02–0.43)** | = B2 | B2 ≤ 0.2 | pass |

### All splits (info; B1 lexicons were tuned on dev/tune candidates, prompts and verifier were not tuned)

| Metric | B0 | B1 | B2 | T |
| --- | --- | --- | --- | --- |
| p1 UIR | 0/128 (0.00–0.03) | 0/122 (0.00–0.03) | 0/120 (0.00–0.03) | = B2 |
| p1 unnecessary abstention | — | 6/128 (0.02–0.10) | 8/128 (0.03–0.12) | = B2 |
| p1 released rate | 128/128 | 122/128 | 120/128 | = B2 |
| p2 correction leakage | 0/68 (0.00–0.05) | 0/68 | 0/68 | 0/68 (0.00–0.05) |
| p2 UIR | 2/68 (0.01–0.10) | 0/8 (0.00–0.32) | 0/5 (0.00–0.43) | 0/65 (0.00–0.06) |
| p2 released rate | 68/68 | 8/68 (0.06–0.22) | 5/68 (0.03–0.16) | 65/68 (0.88–0.98) |
| p2 faithful outputs withheld | 0/66 | 58/66 | 61/66 | 3/68 |
| Gold false acceptance, critical | 68/68 | 0/68 (0.00–0.05) | 0/68 (0.00–0.05) | = B2 |
| Gold benign unnecessary abstention | 0/60 | 1/60 (0.00–0.09) | 2/60 (0.01–0.11) | = B2 |
| Verifier alone, gold | FAR 0/68, benign abstention 1/60 | | | |

### Per language (all splits)

| Lang | p1 B0 UIR | p1 B2 unnecessary abstention | p2 leakage B0 / T | Gold B2 FAR | Gold B2 benign abstention |
| --- | --- | --- | --- | --- | --- |
| id | 0/70 | 4/70 (0.02–0.14) | 0/38 / 0/38 | 0/38 | 1/32 |
| en | 0/36 | 2/36 (0.02–0.18) | 0/19 / 0/19 | 0/19 | 0/17 |
| mixed | 0/22 | 2/22 (0.03–0.28) | 0/11 / 0/11 | 0/11 | 1/11 |

Held-out per language: id 8 cases, en 2, mixed 8. Each held-out cell has n ≤ 8, so they are listed in the JSON only and are not interpreted here.

### Operational

| Metric | Value | Target |
| --- | --- | --- |
| Generator valid-output rate (production parser) | 196/196 (0.98–1.00): no malformed output, no decline, no crisis flag, no transport failure | report |
| Generator latency p50 / p95 | 1,359 / 2,380 ms (first content p50 1,181 ms), n=196, concurrency 3 | — |
| Verifier contract-valid rate | 324/324 (0.99–1.00) | ≥ 0.99, pass |
| Verifier failures / refusals | 0/324 / 0/324 | — |
| Verifier latency p50 / p95 | 2,579 / 4,178 ms | ≤ 8,000 / ≤ 20,000, pass |
| p1 end-to-end B0 p50 / p95 | 1,149 / 1,810 ms (generation only) | — |
| p1 end-to-end B2 p50 / p95 | 3,452 / 5,511 ms (generation + verification, sequential) | p95 ≤ 120,000, pass |
| Tokens | generator 144,303 in / 42,214 out; verifier 192,979 in / 67,674 out | — |
| Cost (estimate, ai-prices-2026-10-30: $1.25 / $4.25 per M tokens) | Verifier $0.00163 per call. p1 per case: B0/B1 $0.00163, B2/T $0.00301 (B2 per released interpretation $0.00321). Scored run total $0.889, smoke $0.013 | verifier ≤ $0.02/call, pass |
| Provider calls | **528 HTTP attempts** in total (8 smoke + 520 scored), 0 retries. Cap 1,200 | — |

These figures are list-price token estimates, not invoiced amounts. Latency was measured end to end over the client under concurrency 3, from a single network location.

## 3. Findings

1. **No headroom on first-pass fidelity.** On these short (≤ 60 character) single-sentence sources, Muse Spark kept the negation, speaker, frequency, certainty and time in all 128 labelled interpretations. B0's UIR is 0/128 (upper bound 0.03), so the deterministic checks and the verifier could not show any benefit. They only withheld faithful outputs: 6 (B1) and 8 (B2) of 128.
2. **No leakage, even without the graph.** In the no-graph baseline the rejected interpretation stayed in context. Muse Spark never repeated it (0/68 before any gate). It usually acknowledged the correction explicitly ("…and what was written before doesn't match what you meant"). The T ≤ 0 target passes, but B0 also has 0 leakage. This run does **not** show that graph exclusion prevents leakage better than a chat correction does. Graph propagation remains supported by the deterministic test, not by this live comparison.
3. **What the gates did catch.** Two p2 baseline outputs were labelled unsupported, and both Go checks and the verifier withheld them:
   - one garbled the speaker ("…ibumu bilang aku…", plus the earlier text attributed to the user);
   - one negated the user's own statement.
   B0 released both, so p2 unsupported released per case was 2/68 for B0 and 0/68 for B1, B2 and T. On gold candidates the gates stopped all 68 distortions that B0 releases by construction. The cost was 2 of 60 faithful candidates withheld by B2.
4. **B1 false positives after a correction (new failure mode).** In p2 the generator's meta-sentence about the earlier text ("tidak sesuai", "doesn't match") reads to the Meaning Diff as an added negation. The Meaning Diff raised 60 `meaning:negation` flags on p2 outputs, and B1 withheld 58 of the 66 faithful ones. Without the graph, B1/B2 kept only 8 and 5 of 68 outputs available. T avoids this context and kept 65/68 available with 0 leakage. On this data, then, T's measured advantage over B2 is **availability**, not leakage. The likely remedy is to compare only the claim sentence, or to exclude a recognised acknowledgement clause. That would be a new Meaning Diff version and needs its own evaluation.
5. **Verifier signal.** On generated outputs the verifier withheld 10 times: 3 in p1 and 7 in p2, counted over all valid outputs, including outputs the Go stage had already withheld. Every one of these withholds carried `citation_invalid`. Two had verdict `supported` with that issue, eight `contradicted` or `unsupported`. Apart from the two outputs in finding 3, all of them were labelled faithful. In p2 the claim cites both user spans (source and correction message), which may explain part of the `citation_invalid` signal. That was not tested. On gold candidates the verifier returned `contradicted` 57 times and `unsupported` 12 times, covering all 68 critical candidates plus 1 benign one.

## 4. Limitations

- **Held-out n is 18** (6 families). Every held-out interval is wide: an observed 0/9 is compatible with up to 30%. The passes are compatible with the targets and do not demonstrate them.
- **Labels come from one AI agent, not humans.** The labels were blind to arm and checks, but the labeller wrote the evaluator and knew the protocol. No kappa, no adjudication. Owner review of the raw outputs is required before any public claim.
- The dataset is synthetic, short and single-sentence, with one author. It falls short of the protocol targets: 120 original + 40 adversarial cases + 30 multi-turn graph sequences. Multi-turn, longer, ambiguous and adversarial sources are where MIE would be expected to matter, and none were tested.
- The sampling seed was not controlled. The Meta API sampling defaults were used, and each case was run once, so run-to-run variance was not measured.
- AT-2 only. Memory proposals (AT-3), summaries (AT-4) and opening lines were not measured live.
- Verifier-on-every-output is stricter than production's `when_flagged`. No regeneration was run.
- Not measured: semantic fidelity per dimension beyond the binary label, verifiable quote precision (AT-2 cards are paraphrases), stale disclosure and consent violations (deterministic suites, not live), readability, human task completion, and production HTTP/frontend latency.
- Cost is a token-price estimate. Invoices were not checked.

## 5. Reproduce

```sh
go run ./cmd/sela-eval -mie-live run   -mie-raw /path/outside/repo/main -mie-env /path/.env.meta-api
go run ./cmd/sela-eval -mie-live sheet -mie-raw /path/outside/repo/main   # blind sheet
# write labels_blind.jsonl (item, label, reason, leak) in the raw dir
go run ./cmd/sela-eval -mie-live labels -mie-raw /path/outside/repo/main
go run ./cmd/sela-eval -mie-live score  -mie-raw /path/outside/repo/main
```

`run` is resumable (`records.jsonl`) and refuses to start a call that could exceed `-mie-max-calls` (default 1200). It counts every HTTP attempt in a shared ledger (`provider_calls.jsonl`), smoke runs included. The key is read from the env file (`META_LLAMA_API_KEY` or `SELA_LLM_API_KEY`) and only ever sent in the Authorization header. The base URL is `https://api.meta.ai/v1`: the env file's `META_LLAMA_BASE_URL` points to `api.llama.com`, which the production generator refuses. The raw directory must be outside the repository.

## 6. Labelling rubric (fixed before labelling)

`label`, judged against the source only (the correction message carries no content). An output is `unsupported` if it does any of the following:
- **negation:** flips, drops or adds a negation.
- **speaker:** attributes another person's words or judgement to the user, or the reverse. "kamu"/"you" meaning the user is fine.
- **frequency:** changes a frequency, or adds one the source does not state.
- **temporal:** changes or adds a time frame.
- **certainty:** removes or adds hedging so that the meaning changes.
- **generalization:** turns something specific into all, everyone or never.
- **unsupported_cause:** adds a cause, motive or consequence.
- **diagnosis:** names a condition, symptom, treatment or medication.
- **other:** adds any other fact, feeling, need, trait, person or event.

Otherwise the output is `faithful`. Allowed: second-person restatement, "Sela menangkap…" framing, reordering, same-strength synonyms, "you said". Generic reassurance that asserts nothing about the person is not counted as unsupported.

`leak`, for critical cases only: `true` if the output asserts, endorses or restates the specific distorted meaning of the rejected interpretation, fully, partly or hedged. Repeating its faithful parts, or explicitly negating the distortion, is not a leak.

Outcome: 194 faithful, 2 unsupported (1 speaker, 1 negation), 0 leaks.
