# Meta integration

Status: **adapter and Modal serving app implemented; live deployment not verified** ([B-1](BLOCKERS.md)). Authority: [SPEC](../SPEC.md) I.meta, C4, V7, V9, V10, V15, V22, V25. Decision record: [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md).

## What runs where

- **Model:** `meta-models/Muse-Glimmer-30B` (Meta, Apache-2.0, open weights), revision `cec94fcfd6512f2cfe708138c32533b346ea4f69`, served by vLLM 0.32.0 on a Modal GPU container (`modal/sela_llm.py`) as `sela-glimmer-cec94fc`.
- **Adapter:** `internal/llm` — OpenAI-compatible `POST /v1/chat/completions` with streaming and `response_format: json_schema`; headers `Modal-Key`/`Modal-Secret` (Modal proxy auth, checked before any container starts) and `Authorization: Bearer` (vLLM API key). Credentials come only from the Go process environment and never reach the browser or logs.
- **Request parameters** (verified against the pinned chat template and vLLM 0.32.0 source, [model notes](../modal/MODEL_NOTES.md) §4): no sampling override (the model's generation config applies; Meta advises against greedy decoding), `chat_template_kwargs.reasoning_strength: "low"`, `stream_options.include_usage`, `max_tokens` with headroom for hidden reasoning (3072 for the ledger, 1536 for opening lines). Reasoning (`delta.reasoning`) is discarded; empty content is a failure. JSON-schema enforcement may not be active under the reasoning parser, so Go validation is the only guarantee.
- **Uses (all bounded, all re-validated by Go):** Story Ledger extraction with exact quotes and qualifiers; one clarification-field pick (the question text comes from Sela's templates); Opening Practice lines grounded in the approved brief; wording in the user's interface language (Indonesian or English) while quotes stay in the user's words.

## Failure behaviour

| Situation | What the user sees | Provenance |
| --- | --- | --- |
| `SELA_LLM_MODE=disabled` or AI consent declined | AI button explains it is unavailable; manual path | `manual` |
| Model cold | "Model is starting" with an estimate. Modal answers 503 at once while no container is ready; the operation retries every 10 s for at most twice the cold-start estimate and never past the operation timeout, only before the first token (`journey.generate`, `TestModalColdStartRetries`) | — |
| Timeout, 5xx, 429 after bounded retries, auth failure | Localized error, retry button, manual path; nothing committed | — |
| Wrong served model name | Result discarded (`ai_model_mismatch`) | — |
| Malformed, empty or ungrounded output | Discarded or reduced to validated claims; counts shown | — |
| Consent withdrawn / story changed / session deleted / crisis during the call | Operation canceled or result discarded | — |
| Test double (dev/CI only) | Banner "TEST DOUBLE — not Meta" on every item | `test_double` |

[EvidenceGuard](CLAUDE_VERIFIER.md) may later review Meta-derived, source-bound claims with Claude after a separate user consent (C12, not wired yet). Claude never drafts in place of an unavailable Meta call; Meta remains the demonstrable **primary** generation provider. An **opt-in DeepSeek** fallback is allowed for eligible Meta availability failures with independently granted DeepSeek consent, default OFF in hackathon mode, no silent switch. See [fallback](DEEPSEEK_FALLBACK.md).

The v2 runtime has **no fallback to any other provider** yet: the DeepSeek fallback (C13, V19–V21) exists only as the standalone, default-OFF prototype `prototypes/generationfallback` and is not wired in (T5b), so every failure above ends in the unavailable/manual path. Retries happen only before the first token; the operation deadline bounds everything.

## Evidence

`go test ./internal/llm/...` exercises the client against local servers and the test double (streaming, retries, cancellation, timeouts, model mismatch, oversize, empty content, no sampling override, reasoning kwarg, auth headers, redirects refused, no payload leakage). `go run ./cmd/sela-eval -mode test_double` runs a fault battery (malformed JSON, fabricated source, dropped negation, diagnosis, invented number, prompt-injection echo, wrong model, 503/429/401, slow, stall, oversize, truncated stream). `tests/modal` checks the Modal app configuration statically. **No live call has been made**; `live_meta` has never been produced. After deployment, `python3 modal/smoke_test.py` and `go run ./cmd/sela-eval -mode live` record non-sensitive evidence (status, latency, token counts, validation outcome) — never prompts or outputs.

A live demo must show actual Meta provenance; unavailable access must remain visibly unavailable unless a separately consented fallback was transparently invoked. A fallback demo is not Meta inference. The fallback prototype is mock-tested only (`cd prototypes/generationfallback && go test ./...`); no live DeepSeek call has been made. A separately labelled `live_anthropic` review, once it exists, must not be presented as Meta processing or clinical correctness. Verify the final hackathon rules about third-party models ([event notes](META_EVENT_NOTES.md)).

## Not used

No hosted Meta Model API, no tool calling, no image/speech models, no fine-tuning, no inference-time retrieval. Each would need a measured benefit and an ADR. Event eligibility of this setup is unverified ([event notes](META_EVENT_NOTES.md)).
