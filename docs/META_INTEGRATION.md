# Meta integration

Status: **adapter and Modal serving app implemented; live deployment not verified** ([B-1](BLOCKERS.md)). The v3 role of Meta as primary generator inside a routed system (DeepSeek fallback, Claude verifier; [ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md), PM decision D-030) is **in progress**: committed on branch `feat/sela-v3-ai` (`44845d3` and `63a5c98`, then `02051ae`, which merges `feat/sela-v3-pilot` at `59dba14`), not merged; nothing verified, and nothing of it is wired into the delivery branch; live use **blocked externally** (B-1, B-8, B-9). Authority: [SPEC](../SPEC.md) I.meta, I.ai, I.fallback, C4, C13, C18, V7, V15, V19–V21, V22, V25, V35 (supersedes V9 and V10), V41 and V42; the owner's [DeepSeek fallback contract](DEEPSEEK_FALLBACK.md). Decision record: [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md).

## Role in v3 (PM decision D-030; in progress on branch `feat/sela-v3-ai`, not merged)

- **Primary generator for every generative task:**
  - listening replies
  - reflection cards ("Sela menangkap…")
  - memory proposals
  - consultation-summary drafts
  - opening lines
- **Never a verifier.** The pinned model stays Muse Glimmer 30B unless the owner decides otherwise (B-7).
- **Only one provider may replace it: DeepSeek**, and only when Meta is *unavailable* before any content arrived:
  - cold start beyond the task budget;
  - 5xx;
  - timeout;
  - 429 (rate limited) after the bounded retries.
  
  The user must also have granted the separate DeepSeek consent (ADR-0003 RR-4). Without that consent, the honest no-AI path follows. ADR-0003 §4 also counted Sela's own admission queue being full and an authentication failure on Sela's side; under the owner's contract, which wins here, those are not upstream availability errors, so they end in the no-AI path with an operator alert (ADR-0003 §17).
- **The owner's fallback contract holds as well** (C13, V19–V21, [DeepSeek fallback](DEEPSEEK_FALLBACK.md)): an opt-in DeepSeek fallback for eligible Meta availability failures, with independently granted DeepSeek consent, OFF by default (in hackathon mode too), never a silent switch, labelled `live_deepseek_fallback`. The owner's prototype `prototypes/generationfallback` is the reference implementation; the production integration (`internal/ai`, P5) must satisfy C13 and V19–V21 as well as ADR-0003.
- **What never leads to DeepSeek:**
  - a model decline or refusal;
  - a raised risk flag;
  - a Meaning Lock or output-guard rejection;
  - malformed, empty or oversize output;
  - a served-model mismatch;
  - any failure after the first content delta (RR-5).
- **Claude does not draft** in place of an unavailable Meta call. It verifies Meta's (or DeepSeek's) output before release where ADR-0003 RR-8 requires it.
- **Processing location.** Requests go to Sela's own vLLM server in Sela's Modal account, in the US (Modal Servers default us-east, observed 2026-10-26). That is a cross-border transfer from Indonesia (D-049).
  - It needs the `ai_processing` consent, with a plain-language disclosure of provider, location and purpose (ADR-0003 §8).
  - Data is minimised as in ADR-0003 §9: only the needed segments, contact details and private names redacted, no account identifiers.

## What runs where (implemented)

- **Model:** `meta-models/Muse-Glimmer-30B` (Meta, Apache-2.0, open weights), revision `cec94fcfd6512f2cfe708138c32533b346ea4f69`, served by vLLM 0.32.0 on a Modal GPU container (`modal/sela_llm.py`) as `sela-glimmer-cec94fc`. The pin was still valid on 2026-10-26 (HF `sha` unchanged, lastModified 2026-10-22).
- **Adapter:** `internal/llm`, an OpenAI-compatible `POST /v1/chat/completions` client with streaming and `response_format: json_schema`.
  - Headers: `Modal-Key`/`Modal-Secret` (Modal proxy auth, checked before any container starts) and `Authorization: Bearer` (vLLM API key).
  - Credentials come only from the Go process environment and never reach the browser or logs.
- **Request parameters** (verified against the pinned chat template and the vLLM 0.32.0 source, [model notes](../modal/MODEL_NOTES.md) §4):
  - No sampling override: the model's generation config applies, and Meta advises against greedy decoding.
  - `chat_template_kwargs.reasoning_strength: "low"` and `stream_options.include_usage`.
  - `max_tokens` with headroom for hidden reasoning: 3072 for the ledger, 1536 for opening lines.
  - Reasoning (`delta.reasoning`) is discarded, and empty content is a failure.
  - JSON-schema enforcement may not be active under the reasoning parser, so Go validation is the only guarantee.
- **Uses (all bounded, all re-validated by Go):**
  - v2: Story Ledger extraction with exact quotes and qualifiers; one clarification-field pick (the question text comes from Sela's templates); Opening Practice lines grounded in the approved brief.
  - All wording is in the user's interface language (Indonesian or English), while quotes stay in the user's words.
  - v3 adds the listening reply, reflection, memory-proposal and summary tasks (ADR-0003 §3; planned). Each generator schema carries a `risk` flag (`none | elevated | crisis`) that can only raise the route, and a `declined` flag (planned).

## Failure behaviour

The first three columns are implemented (v2). The last column is the planned v3 routing.

| Situation | What the user sees (v2) | Provenance | v3 routing (ADR-0003 §4) |
| --- | --- | --- | --- |
| `SELA_LLM_MODE=disabled` or AI consent declined | The AI button explains it is unavailable; manual path | `manual` | Operator off-switch or no consent: no fallback, manual path |
| Model cold | "Model is starting" with an estimate. Modal answers 503 at once while no container is ready. The operation retries every 10 s for at most twice the cold-start estimate, never past the operation timeout, and only before the first token (`journey.generate`, `TestModalColdStartRetries`) | — | Within the task budget: wait, as today. Beyond it: DeepSeek if RR-4 holds, else no-AI. The listening reply waits at most 20 s (engineering default) |
| Timeout, 5xx, 429 after bounded retries, auth failure | Localized error, retry button, manual path; nothing committed | — | Before any content: fallback-eligible (RR-4), except an auth failure on Sela's side, which the owner contract does not count as an availability error (ADR-0003 §17). After content: no fallback |
| Wrong served model name | Result discarded (`ai_model_mismatch`) | — | Integrity failure: abstain, operator alert, no fallback |
| Malformed, empty or ungrounded output | Discarded or reduced to validated claims; counts shown | — | Content failure: never a fallback. One same-provider regeneration is allowed for listening replies, reflections and opening lines (RR-5) |
| Model declines, or raises the risk flag | — (no such signal in v2) | — | Safe stop: support shown; never a fallback (RR-5, RR-7) |
| Consent withdrawn / story changed / session deleted / crisis during the call | Operation canceled or result discarded | — | Same, for every provider used (RR-13) |
| Test double (development and CI only) | Banner "TEST DOUBLE — not Meta" on every item | `test_double` | Production refuses it (implemented). D-046 governs the demo: the generator and the verifier may each be a live provider on synthetic data or a labelled test double, and no demo output is shown as live unless a live provider produced it. ADR-0002 §12 ("`demo` refuses `SELA_LLM_MODE=test_double`") and the `internal/config` guard (development and test only; committed with the foundation, `0ff3341`) are narrower, tracked in [conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 52 for the backend foundation agent. Until then the stricter guard runs |

Retries happen only before the first token, and the operation deadline bounds everything (implemented).

## Minors and usage policy

The Muse Glimmer Usage Policy says the model "is not intended for individuals under the age of 18". The model card makes deployers of systems that under-18s may use responsible for assessing and mitigating the risks and for complying with the law (observed 2026-10-26). This does not prohibit Sela's 15–17 band under the Apache-2.0 licence, but it goes against Meta's stated intent. The v2 "adult attestation" response is therefore replaced:

- Under D-031, the `15_17` and `unknown` bands are enabled only outside production until a documented risk assessment and the legal review (B-10) are done.
- Age-appropriate copy and opt-in memory apply to those bands.

The policy also prohibits unlicensed medical practice and inferring health information without a legal right. Meaning Lock, the output guards and the verifier (ADR-0003 §7) are Sela's controls for both. It also forbids representing third-party outputs as Meta's, so every output carries its provider label.

## Evidence

- `go test ./internal/llm/...` exercises the client against local servers and the test double: streaming, retries, cancellation, timeouts, model mismatch, oversize output, empty content, no sampling override, the reasoning kwarg, auth headers, refused redirects, no payload leakage.
- `go run ./cmd/sela-eval -mode test_double` runs a fault battery: malformed JSON, fabricated source, dropped negation, diagnosis, invented number, prompt-injection echo, wrong model, 503/429/401, slow, stall, oversize, truncated stream.
- `tests/modal` checks the Modal app configuration statically.
- **No live call has been made**, and `live_meta` has never been produced.
- After deployment, `python3 modal/smoke_test.py` and `go run ./cmd/sela-eval -mode live` record non-sensitive evidence (status, latency, token counts, validation outcome), never prompts or outputs.
- No routing code exists in the runtime yet. The routing invariants (no fallback after a content failure, no self-verification, no call without consent) are pre-registered as ADR-0003 §12 metrics to be tested with labelled test doubles.
- The owner's fallback prototype is mock-tested only (`cd prototypes/generationfallback && go test ./...`); no live DeepSeek call has been made.

A live demo must show actual Meta provenance; unavailable access must remain visibly unavailable unless a separately consented fallback was transparently invoked. A fallback demo is not Meta inference. Separately labelled `live_anthropic` reviews and `live_deepseek_fallback` drafts, once they exist, must never be presented as Meta processing or as clinical correctness (D-050). Verify the hackathon rules about third-party models ([event notes](META_EVENT_NOTES.md); B-12).

## Options recorded since ADR-0001 (observed 2026-10-26; not evaluated, not adopted)

- **Modal Dedicated Endpoint for Glimmer** (`modal endpoint create --model meta-models/Muse-Glimmer-30B`): a Modal-tuned stack billed per second, as an alternative to the hand-written `modal/sela_llm.py`.
- **Modal Shared Endpoint for Muse Spark Open:** $1.25 per MTok prompt, $0.15 cached, $4.25 completion; routes through us-west; plan credits do not apply.
  - It removes the multi-GPU budget blocker for Spark Open, but it is Modal-managed serving, not self-hosting.
  - The gated licence text has not been read. The owner decides under B-7.

## Not used

- No hosted Meta Model API, no tool calling, no image or speech models, no fine-tuning, no inference-time retrieval. Each would need a measured benefit and an ADR.
- The event eligibility of this setup is unverified ([event notes](META_EVENT_NOTES.md)).
- Claude and DeepSeek calls are never counted as Meta tooling (D-050).

## History: superseded statements

- "There is **no fallback to any other provider**." This was the v2 policy (SPEC V9; D-001's "no fallback" clause). **Superseded by D-026 and D-030 (SPEC V35):** DeepSeek may replace Meta under ADR-0003 RR-4. It still describes the current code, which has no fallback until P5 lands. Before v3, main amended it with the owner's opt-in fallback (C13, V19–V21; [D-025](DECISION_LOG.md#d-025)): "The v2 runtime has **no fallback to any other provider** yet: the DeepSeek fallback (C13, V19–V21) exists only as the standalone, default-OFF prototype `prototypes/generationfallback` and is not wired in (T5b), so every failure above ends in the unavailable/manual path." That is still true of the v2 runtime.
- "EvidenceGuard **may later** review Meta-derived, source-bound claims with Claude after a separate user consent (C12, not wired yet)." **Superseded by D-030 (SPEC C18, V36):** verification is required before release for consultation-summary wording, memory proposals and flagged text, and the outputs abstain without a permitted verifier. The separate consent and "not wired yet" remain true.
