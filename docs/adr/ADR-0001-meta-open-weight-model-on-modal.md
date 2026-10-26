# ADR-0001 — Meta open-weight model for Sela v2, served on Modal

Status: **accepted for implementation; live serving not yet verified** (no Modal credentials or GPU budget in this environment, see [blockers](../BLOCKERS.md)). Date: 2026-10-26. Authority: [SPEC](../../SPEC.md) C4, I.meta, V7, V9, V10, V15; owner direction of 2026-10-26.

## Context

The owner directed that Sela's AI run on a **Meta open-weight model served on Modal.com**, chosen for licence, capability and hackathon terms, with no assumption that every Meta model has the same licence, that open weights are unconditional, that self-hosting counts as hackathon tooling, or that the account already has model/GPU access.

Sela uses the model for four bounded, evaluable tasks only: Story Ledger extraction (structured JSON with exact quotes), one clarification-field pick, Opening Practice drafting from an approved brief, and bilingual wording (Indonesian/English, including code-switched input). Every output is re-validated by Go (Meaning Lock). The model never decides safety routing, consent, approval, sharing, matching or plans.

All facts below were checked on 2026-10-26 against primary sources by the external-verification workflow of this branch (Hugging Face API, model cards and licences, Meta developer docs, Modal docs and pricing, vLLM registry, the event page and its embedded Official Rules). "Observed" means read from the source; "inferred" means derived.

## Options

| Option | Licence / access | Indonesian | Serving footprint | Hackathon fit | Verdict |
| --- | --- | --- | --- | --- | --- |
| **`meta-models/Muse-Glimmer-30B`** (Aug 2026) | Apache-2.0; ungated (observed). Separate Usage Policy (not a licence condition, treated as binding guidance) | Not enumerated on the card ("more than 100 languages"); the Muse Glimmer paper (arXiv 2610.24628, Table 10) lists Indonesian in its "strong support" tier, defined by a translation proxy (observed by the critic pass). **Unverified for Sela's tasks** | Dense ~29.8B params, BF16 ~59.6 GB; Meta docs budget ~72 GB VRAM → one H100/A100-80GB (tight KV cache) or H200. vLLM support since v0.28.0 (observed in registry) | Official Rules name "Glimmer"; event page says "run Meta open weights models"; Modal offers credits "to use Meta's open-weights models" (observed) | **Chosen** |
| `meta-llama/Llama-4-Scout-17B-16E-Instruct` | Llama 4 Community Licence ("Built with Llama", AUP, 700M MAU, EU multimodal clause); gated, manual approval | Officially supported (card lists 12 languages incl. Indonesian) | 109B MoE, BF16 ~217 GB → ≥4× H100 (≈$16/h); single-H100 INT4 path unverified; FP8 only from a third party under another licence | Llama is never named in the event materials (observed) | Rejected: cost, gating, weaker event alignment |
| `meta-llama/Llama-3.3-70B-Instruct` | Llama 3.3 Community Licence; gated | **Not supported**; card "strongly discourages" unsupported languages | BF16 ~141 GB → 2× H100 | Not named | Rejected |
| `meta-llama/Llama-3.1-8B-Instruct` | Llama 3.1 Community Licence; gated | **Not supported** | ~16 GB, one L4/A10/L40S | Not named | Rejected for Indonesian quality risk; kept as a documented low-cost fallback *only* if a future ADR proves Glimmer unworkable |
| `meta-models/Muse-Spark-Open-816B-A42B` | Muse Model Licence; gated | Not enumerated | ≥700 GB VRAM (card) | Named family | Rejected: beyond budget |
| Hosted Meta Model API (`api.meta.ai`, `muse-spark-1.3`) | Meta API terms; $150 hackathon credits (optional) | Not evaluated | No GPU to operate | Listed | Out of scope by owner direction (Modal self-hosting). Not implemented; never used as a silent fallback |

## Decision

- **Model:** `meta-models/Muse-Glimmer-30B`, pinned revision **`cec94fcfd6512f2cfe708138c32533b346ea4f69`** (HF API `sha`, observed 2026-10-26; the repository changed on 2026-10-22, so the pin matters). Weights BF16, no quantization.
- **Server:** vLLM OpenAI-compatible server, **`vllm==0.32.0`** pinned (latest on PyPI, 2026-10-18; Glimmer needs ≥0.28.0). Flags follow Meta's deployment guidance: `--reasoning-parser muse_glimmer --generation-config auto`, `--served-model-name sela-glimmer-cec94fc` (the name encodes the short revision), bounded `--max-model-len` (16384 initially), `--max-num-seqs` 8, request logging disabled. Reasoning tokens are separated by the parser and **discarded** by Sela; only final content is validated and stored.
- **Requests:** OpenAI `response_format: json_schema` plus local validation (Meaning Lock is the guarantee: with the `muse_glimmer` reasoning parser, vLLM 0.32 may never activate the grammar for a plain answer, see [model notes](../../modal/MODEL_NOTES.md) §4d); no sampling override, so the model's generation config applies (temperature 1.0, top-p 0.95, top-k 64; Meta advises against greedy decoding, which is not reproducible either); an optional `seed`; `chat_template_kwargs.reasoning_strength = "low"` (the template ignores vLLM's `reasoning_effort`). Reasoning arrives as `delta.reasoning` and is discarded; empty content is a failure whatever the finish reason. Recorded 2026-10-26 after checking the pinned chat template and vLLM 0.32.0 source.
- **Platform:** Modal `@app.server` (modal client pinned to **1.6.2**), `gpu="H100"` (Modal may place it on an H200 at no extra cost), `min_containers=0` (scale to zero), `max_containers=1` (hard cost ceiling), `scaledown_window` 5 min, proxy auth **on** (Modal-Key/Modal-Secret) plus a vLLM API key, weights and compile caches on Modal Volumes, secrets via `modal.Secret`. Separate `dev`/`staging` Modal environments.
- **Application:** Go adapter (`internal/llm`) is OpenAI-compatible and model-agnostic; the pinned model ID, revision and served name are configuration and are checked against every streamed chunk's `model` field (and `GET /v1/models`, whose `root` contains the full revision, for the health check; vLLM's `/health` carries no identity) before any output is labelled `live_meta`.

## Licence and policy obligations

- Apache-2.0: keep the licence and notices when redistributing weights (Sela does not redistribute weights; it downloads them inside the owner's Modal account).
- Muse Glimmer Usage Policy (observed, mirrors the Llama 4 AUP): no unlicensed practice of medicine/health; no inferring health information about individuals without rights; nothing that facilitates self-harm; do not represent outputs as human-generated; disclose known dangers; **not intended for people under 18**. Sela's response: synthetic or explicitly fictional practice text only, adult attestation, deterministic crisis routing before inference, every AI wording labelled as AI interpretation, and the privacy page discloses limits. No real-user deployment without a separate legal review (Indonesian PDP law not assessed).

## Capability, cost and risk

- **Indonesian/English/code-switching:** evidence is indirect (paper tier). Sela runs its own bilingual and code-switched evaluation with pre-registered thresholds ([evaluation](../EVALUATION.md)); failure would trigger a new ADR, not a silent model swap.
- **Memory:** ~72 GB VRAM budget on an 80 GB GPU leaves little KV cache; Sela's prompts are short (<4k tokens) and output is bounded, so `--max-model-len` 16384 with 8 concurrent sequences is the starting point, to be measured.
- **Latency:** unmeasured. Reasoning models emit hidden reasoning tokens; low effort and bounded `max_tokens` are required. Cold start (container + ~60 GB weight load from Volume) is expected to take minutes; Modal answers 503 at once while no container is ready, so the operation retries every 10 s within twice the cold-start estimate and the UI shows a warming state.
- **Cost (Modal list prices, observed 2026-10-26):** H100 $0.001097/s ≈ $3.95/h; H200 ≈ $4.54/h; A100-80GB ≈ $2.50/h. With `max_containers=1` the burn rate is bounded by one GPU while warm. Illustrative demo session: 4 min cold start + 10 min use + 5 min scale-down ≈ 19 GPU-minutes ≈ **$1.25 on H100**. Starter plan includes $30/month credits and needs a payment method for GPUs. These are estimates, not measurements.
- **Structured output:** constrained decoding guarantees shape, not truth; Meaning Lock remains mandatory.

## Hackathon tooling — status: unverified

Observed: the Official Rules list "Muse Spark, Glimmer, SAM 3, and any other Meta AI products identified on the Event page"; the event page says "run Meta open weights models"; the Credit Terms allow building "with Meta's open-weight models without a Meta Model API account" and state that credits do not affect judging. There is no formal definition of "Meta AI tooling" and judging is qualitative. Sela therefore **claims nothing about eligibility or scoring**; it documents what it uses and how.

## Consequences

- Live inference needs owner actions: Modal account with payment method and an agreed GPU budget, proxy-auth token, secrets, deployment and a smoke test. Until then the app runs with AI **disabled** (manual path) or with the labelled **test double** in CI/dev; neither is presented as Meta inference.
- Rollback: Modal `app rollback` is Team-plan only (observed); on Starter, redeploy the previous Git commit (`modal deploy` from that commit). The Go app can switch AI off instantly with `SELA_LLM_MODE=disabled`.
- Revisit this ADR if the revision changes, vLLM drops support, the bilingual evaluation fails its thresholds, or the owner prefers the hosted Model API.

## Sources

Hugging Face API (`/api/models/meta-models/Muse-Glimmer-30B`, `/api/organizations/meta-models/overview`, Llama repositories); model cards, LICENSE and USAGE_POLICY files; https://dev.meta.ai/docs/open-weight-models, https://dev.meta.ai/docs/muse-glimmer and its deploy/prompting pages; https://arxiv.org/abs/2610.24628; vLLM registry at tags v0.27.1/v0.28.0/v0.32.0 and https://docs.vllm.ai/en/stable/features/structured_outputs/; https://pypi.org/pypi/vllm/json; https://pypi.org/pypi/modal/json; https://modal.com/docs/guide/servers, /guide/webhook-proxy-auth, /guide/environments, /guide/gpu, /pricing; https://cerebralvalley.ai/e/meta-ai-developer-hackathon and its waiver/Official Rules payload; https://dev.meta.ai/events/global-hackathon.
