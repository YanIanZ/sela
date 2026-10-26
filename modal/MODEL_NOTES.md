# Muse Glimmer on vLLM on Modal: verified facts

Checked on **2026-10-26** against the primary sources linked in each section (§4a re-checked the same day: Meta's deploy page, vLLM v0.32.0 `vllm/renderers/hf.py` and `chat_completion/protocol.py`, and the pinned `chat_template.jinja`). "Observed" means read from the source on that date. "Inferred" means derived from observed facts, usually by reading source code. **UNVERIFIED** means the claim could not be confirmed without a live deployment, which has not happened. Nothing has been deployed from this repository yet, so no runtime behaviour below has been observed.

Implements [ADR-0001](../docs/adr/ADR-0001-meta-open-weight-model-on-modal.md). The app is in [sela_llm.py](sela_llm.py) and the runbook in [README.md](README.md).

## 1. Pinned model

| Fact | Value | Source |
| --- | --- | --- |
| Repository | `meta-models/Muse-Glimmer-30B` | https://huggingface.co/meta-models/Muse-Glimmer-30B |
| Revision (HF API `sha`) | `cec94fcfd6512f2cfe708138c32533b346ea4f69` (repo `lastModified` 2026-10-22) | https://huggingface.co/api/models/meta-models/Muse-Glimmer-30B and `/revision/<sha>` |
| Licence / gating | `apache-2.0` (cardData), `gated: false`, `private: false` | same |
| Architecture | `MuseGlimmerForConditionalGeneration`, `model_type: muse_glimmer`, BF16 | `config.json` at the revision |
| Text config | 52 layers, 2 KV heads, head_dim 128, sliding window 2048, `max_position_embeddings` 131072 | `config.json` at the revision |
| Files | 14 files, 59,581,830,875 bytes (59.58 GB). Shards: `model-00001-of-00002.safetensors` (49.95 GB, sha256 `8eef6153…`), `model-00002-of-00002.safetensors` (9.60 GB, sha256 `b58cc214…`) | https://huggingface.co/api/models/meta-models/Muse-Glimmer-30B/tree/cec94fcfd6512f2cfe708138c32533b346ea4f69?recursive=true |
| Served name | `sela-glimmer-cec94fc` (encodes the short revision; the Go app checks it) | this repo |

`sela_llm.py` embeds the size and digest of all 14 files (SHA-256 for LFS files, git blob SHA-1 for the others). I checked the git blob SHA-1 scheme against four downloaded files. The CPU job `download_weights` verifies every digest before it writes a "verified" marker. Each GPU cold start re-checks presence and sizes only (cheap), and refuses to start without the marker.

Usage notes from the model card (observed): the model is not intended for use by people under 18, it is trained on more than 100 languages, and Indonesian is not listed separately. Sela's obligations are in ADR-0001.

## 2. Serving stack

| Fact | Value | Source |
| --- | --- | --- |
| vLLM version | `0.32.0`, latest on PyPI, released 2026-10-18. Glimmer support first appeared in 0.28.0 (2026-08-26) | https://pypi.org/pypi/vllm/json, https://recipes.vllm.ai/meta-models/Muse-Glimmer-30B ("vLLM 0.28.0 +", "Image: vllm/vllm-openai:v0.28.0") |
| Image | `vllm/vllm-openai:v0.32.0`. Pushed 2026-10-18. Index digest `sha256:befccf0e7e4089aa23926bb6f887a85c0d46c2a99e6de5c5ffdf5cb7957c5b34`; linux/amd64 digest `sha256:6ee133356d49ed24cee03dd0cda3d3980279a11c107ce3e752c89e76540f41c9`; ~10.3 GB | https://hub.docker.com/v2/repositories/vllm/vllm-openai/tags/v0.32.0 |
| Meta's tag | `vllm/vllm-openai:muse-glimmer` exists (pushed 2026-08-11, before vLLM 0.28.0 was released). Which vLLM version it contains is **UNVERIFIED**. Not used here | https://hub.docker.com/v2/repositories/vllm/vllm-openai/tags/muse-glimmer, https://dev.meta.ai/docs/muse-glimmer/deploy |
| Image CUDA / Python | CUDA 13.0.3 base, Python 3.12 (`python3` plus pip via get-pip; no unversioned `python`), `ENTRYPOINT ["vllm", "serve"]` | https://github.com/vllm-project/vllm/blob/v0.32.0/docker/Dockerfile |
| Modal host driver | 610.57.04, CUDA Driver API 13.3 (compatible with 13.0 images) | https://modal.com/docs/guide/cuda |
| Model code in 0.32.0 | `MuseGlimmerForConditionalGeneration` registered; parsers `muse_glimmer` (reasoning, tool) registered | `vllm/model_executor/models/registry.py`, `vllm/reasoning/__init__.py`, `vllm/tool_parsers/__init__.py` at tag v0.32.0 |

**Why the official image rather than pip install on a CUDA base.** Meta's deploy guide says "Docker is the supported path". The vLLM recipe, which Meta names as "the source of truth for the image tag, flags, and hardware requirements", specifies an image. The image also ships the CUDA 13.0 JIT toolchain pieces vLLM uses at runtime. Modal supports registry images that have `python` and `pip` on `PATH` and an entrypoint that execs its arguments (https://modal.com/docs/guide/existing-images, and the `Image.from_registry` docstring in modal 1.6.2). The app therefore symlinks `python` to `python3` in a setup step that fails the build if either is missing, and clears the entrypoint with `.entrypoint([])`. Modal's own vLLM example uses the other pattern (`nvidia/cuda` plus `uv_pip_install("vllm==…")`, https://modal.com/docs/examples/vllm_inference). Either pattern would work. The image keeps Sela on the exact artifact Meta and vLLM document. **UNVERIFIED** until the first build: whether Modal's builder accepts this image without further changes.

## 3. Server flags (vLLM 0.32.0)

All flag names were checked in the v0.32.0 source: `vllm/engine/arg_utils.py` (dataclass fields become `--kebab-case`, booleans become `BooleanOptionalAction` with a `--no-` form) and `vllm/entrypoints/launchers/cli_args.py`.

| Flag | Why |
| --- | --- |
| `<local snapshot path>` | Serves the verified files, offline (`HF_HUB_OFFLINE=1`). `/v1/models` then reports this path as `root`, which contains the full revision |
| `--served-model-name sela-glimmer-cec94fc` | Name the API answers to, and the name the Go app checks |
| `--generation-config auto` | Meta: picks up the checkpoint's sampling defaults and stop tokens (it is also vLLM's default) |
| `--reasoning-parser muse_glimmer` | Meta: routes the `to=self` thinking channel to `reasoning` instead of `content`. The parser also forces `skip_special_tokens=False` (recipe, and `adjust_request` in `vllm/reasoning/muse_glimmer_reasoning_parser.py`) |
| `--enable-auto-tool-choice --tool-call-parser muse_glimmer` | The recipe runs both parsers together because they key off the same channel framing. Sela sends no tools |
| `--default-chat-template-kwargs '{"reasoning_strength": "low"}'` | Server-wide default. Request-level `chat_template_kwargs` take precedence (FrontendArgs docstring) |
| `--max-model-len 16384`, `--max-num-seqs 8` | Bounded KV cache. Upper bound at full load is about 6.5 GiB (inferred: 52 KiB per token × 8 × 16384) |
| `--gpu-memory-utilization 0.90` | Meta's "tighter memory" command uses 0.9. Meta budgets 72 GB of VRAM for BF16 on an 80 GB H100 |
| `--language-model-only` | Sets every multimodal limit to 0. vLLM then skips the vision tower, which is marked with `_mark_tower_model` in `muse_glimmer.py` (frees VRAM), and rejects image input |
| `--no-enable-log-requests` | Request logging is off by default in 0.32. `--enable-log-requests` is a `BooleanOptionalAction` with default `False`, and the old `--disable-log-requests` is gone (not in `arg_utils.py` at v0.32.0). `--enable-log-outputs` requires request logging and stays off |
| `--disable-uvicorn-access-log`, `--disable-fastapi-docs` | No per-request access lines and no `/docs` |
| no `--api-key` | vLLM 0.32 uses `VLLM_API_KEY` from the environment when `--api-key` is absent (`vllm/entrypoints/serve/middleware/register.py`). This keeps the key out of argv and out of the logged command. The server refuses to start if the key is shorter than 32 characters |
| env `VLLM_NO_USAGE_STATS=1`, `VLLM_DO_NOT_TRACK=1`, `HF_HUB_DISABLE_TELEMETRY=1`, `VLLM_LOGGING_LEVEL=INFO` | Telemetry off. Log level is never DEBUG, where vLLM's request logger would print prompt inputs if it were ever enabled |

**UNVERIFIED until the first boot:** the actual KV-cache size and "maximum concurrency" lines in the startup log, the VRAM headroom, and the cold-start duration (both with and without the compile cache).

## 4. Request parameters Sela must use

### (a) Reasoning effort: `chat_template_kwargs.reasoning_strength`

- **Name: `reasoning_strength`. Values: `low`, `medium`, `high`, `xhigh`. The template default is `high`.** The chat template at the pinned revision reads `{%- set rs = reasoning_strength if reasoning_strength is defined and reasoning_strength else 'high' -%}` and renders `Reasoning strength: <rs>.` into the system turn ([chat_template.jinja](https://huggingface.co/meta-models/Muse-Glimmer-30B/raw/cec94fcfd6512f2cfe708138c32533b346ea4f69/chat_template.jinja)). The four levels come from the model card ("Best Practices") and from https://dev.meta.ai/docs/muse-glimmer/prompting.
- Send it per request as `"chat_template_kwargs": {"reasoning_strength": "low"}`. Evidence for vLLM 0.32 (from source, not observed at runtime): `build_chat_params` merges the request's `chat_template_kwargs` into the template kwargs (`ChatCompletionRequest`, `vllm/entrypoints/openai/chat_completion/protocol.py` at v0.32.0), and `resolve_chat_template_kwargs` in `vllm/renderers/hf.py` then keeps only the keys the template declares: the variables returned by `jinja2.meta.find_undeclared_variables` for the template, plus `apply_chat_template`'s own parameters. Running `find_undeclared_variables` (jinja2 3.1.6) on the pinned `chat_template.jinja` (revision `cec94fcfd6512f2cfe708138c32533b346ea4f69`, git blob `7507f3c9…`, matching `EXPECTED_FILES`) lists `reasoning_strength`, which the template defaults to `'high'`; so the kwarg reaches the template. Meta's deploy guide shows the same JSON, but in its **llama.cpp** section ("Control reasoning length"); its vLLM section does not mention `chat_template_kwargs` or `reasoning_strength`.
- **Do not use `reasoning_effort`.** In vLLM 0.32, `build_chat_params` forwards `reasoning_effort` as a template kwarg (plus `enable_thinking` when it is set). The pinned template declares neither variable, so `resolve_chat_template_kwargs` drops them (unless `apply_chat_template` takes them as parameters), and the template never reads them in any case: the request succeeds and the setting has no effect (inferred from source). Meta's callout "Use reasoning_strength, not reasoning_effort" is also in its llama.cpp section, where it explains that llama.cpp does not implement the OpenAI spelling; it is not a statement about vLLM.
- Alternatives: a `Reasoning strength: <value>` line in the system prompt (model card, recipe). The template rewrites "Reasoning effort" to "Reasoning strength" inside system text, and it skips the kwarg-driven line when the system prompt already contains "reasoning strength". **Sela's system prompts must therefore not contain the phrase "reasoning strength" or "reasoning effort" by accident.**
- Reasoning cannot be turned off. The template has no "off" level (an empty or missing value falls back to `high`), and Meta's deploy guide says "Reasoning can't be turned off here" in its llama.cpp section. `low` is the lowest of the four levels the model card lists, and the level Meta suggests for spending fewer thinking tokens.

### (b) Sampling

- `generation_config.json` at the revision: `do_sample: true`, `temperature: 1.0`, `top_p: 0.95`, `top_k: 64`, `eos_token_id: [200001, 200008]`, `max_length: 131072`.
- With `--generation-config auto`, vLLM applies these as defaults whenever a request leaves them out (Meta deploy guide, "Sampling").
- **Do not run greedy.** Meta: "Don't run this model greedy. Sending `"temperature": 0` overrides the published settings". If you set sampling per request, send all three values. The recipe adds that greedy is not reproducible either: "identical requests at temperature: 0 with a fixed seed returned 70, 80, and 86 completion tokens across runs".
- **Conflict to resolve in Go:** the `Request.Temperature` comment in `internal/llm/llm.go` says "Temperature 0 makes repeated synthetic runs comparable". For this model that is against Meta's guidance and does not make runs reproducible. Recommendation: omit `temperature`, `top_p` and `top_k`, send a `seed`, and compare runs statistically.
- Stop tokens: leave them to the generation config. Never stop on `<|eom|>` (200007) (Meta deploy guide).

### (c) Reasoning in streaming deltas: `delta.reasoning`

- vLLM 0.32.0 `DeltaMessage` has `role`, `content`, `reasoning`, `tool_calls`, and `ChatMessage` has `reasoning` (`vllm/entrypoints/generate/base/protocol.py`, `chat_completion/protocol.py`). There is no `reasoning_content` output field in 0.32. On *input* messages, `reasoning_content` is accepted as a deprecated alias and renamed to `reasoning`.
- Meta deploy guide: the reasoning parser routes thinking "to `message.reasoning`", and to "`delta.reasoning` when streaming". SGLang and llama.cpp use `reasoning_content`; vLLM does not.
- With the reasoning parser on, a stream carries reasoning deltas first and then content deltas. The parser holds back partial channel markers, so no framing tokens leak (from reading the parser source). Sela discards reasoning; `smoke_test.py` counts reasoning deltas and accepts either field name, but never prints reasoning.
- **Truncation:** reasoning tokens count against `max_tokens`. A request that runs out mid-thought returns empty content with `finish_reason: "length"` (Meta deploy guide, llama.cpp section), and the recipe says a tight budget can also produce empty content with `finish_reason: "stop"`. **The Go client must treat empty content as a failure, whatever `finish_reason` says**, and leave headroom (the smoke test uses 2048 with `low`).
- Usage: send `"stream_options": {"include_usage": true}` to get `prompt_tokens` and `completion_tokens` in the final chunk.

### (d) Structured output (`response_format: json_schema`): enforcement UNVERIFIED and possibly absent

- vLLM 0.32 accepts `{"type": "json_schema", "json_schema": {"name", "schema", "strict"}}` (`ResponseFormat` and `JsonSchemaResponseFormat` in `generate/base/protocol.py`).
- **Risk (inferred from source, not observed):** with a reasoning parser configured, vLLM applies the grammar only after the parser's `is_reasoning_end()` returns true (`vllm/v1/structured_output/__init__.py`, `_get_constraint_bounds`). `MuseGlimmerReasoningParser.is_reasoning_end` returns true only once a *tool-call* channel containing ATEM markup appears. Its docstring says "A `to=user` answer is NOT a reason to leave the reasoning phase". For a plain `to=user` JSON answer, the grammar may therefore never activate, and the content may be unconstrained even though the request asks for a schema.
- Consequence: Sela must not claim constrained decoding for this model until the smoke test, and an evaluation over many runs, shows schema-valid output. The Go Meaning Lock validation is the real guarantee (it is mandatory anyway). The smoke test reports `json_parse_ok` and `schema_valid`.
- Options to evaluate if JSON validity is poor. All are **UNVERIFIED**:
  1. `"include_reasoning": false`. Reading the 0.32 source, this marks reasoning as already ended, so the grammar applies from the first generated token (`ChatCompletionRequest.resolve_reasoning_ended`, which `chat_completion/serving.py` passes as `reasoning_ended`). That would bypass the model's channel framing and may break the parser's content extraction. Test it with `smoke_test.py --extra-body '{"include_reasoning": false}'`.
  2. Named tool calling with the schema as tool parameters.
  3. Stricter prompting plus retries.

### (e) Authentication: two layers

- Modal proxy (server default, `unauthenticated=False`). A request needs `Modal-Key` and `Modal-Secret` headers, or a single `Authorization: Bearer <token-id>.<token-secret>`. A missing token gets 401, and the request does not count toward autoscaling (so it cannot start a GPU). A token outside the environment's scope on RBAC workspaces gets 403 (https://modal.com/docs/guide/servers, https://modal.com/docs/guide/webhook-proxy-auth).
- vLLM API key. `AuthenticationMiddleware` checks only `Authorization: Bearer <key>`, and only for paths starting with `/v1`, `/v2`, `/inference` or `/cohere`. `/health`, `/metrics` and `/invocations` are not covered by the vLLM key; on Modal they are protected by the proxy layer alone (`vllm/entrypoints/serve/middleware/authenticate.py`, plus the warning in the `--api-key` docstring).
- **UNVERIFIED (key compatibility question):** the Go client sends `Modal-Key`/`Modal-Secret` *and* `Authorization: Bearer <vLLM key>`. Modal's docs do not say how the proxy treats an `Authorization` header that is not a proxy token when the Modal headers are also present, or whether the header is forwarded to the container. `smoke_test.py` tests exactly this combination. If `cold_start` ends with a 401 from vLLM (`{"error": "Unauthorized"}`), or with a 401 from the proxy despite correct Modal headers, the layered scheme does not work as designed. The fallback then needs an ADR-level decision, for example proxy auth only with no vLLM key, or the vLLM key in a different header through a custom `--middleware`.

### (f) Health and identity

- vLLM's `/health` returns an empty 200, or 503 when the engine is dead (`vllm/entrypoints/serve/instrumentator/health.py` at v0.32.0). It carries no model information. **Model identity must come from `GET /v1/models`** (`data[].id` is the served name, and `data[].root` is the snapshot path containing the full revision) and from the `model` field of each chunk. ADR-0001 mentions checking `/health`; that is not enough on its own.
- Any authenticated request, `/health` included, starts a GPU container when none is running.

### (g) Modal request behaviour

- No container ready: Modal answers **503** immediately (requests do not queue) and starts a container. Over `max_concurrency` (8): **429**. Request timeouts cannot be configured in Modal; the client must set them. There are no built-in retries (https://modal.com/docs/guide/servers).
- URL: Modal prints it at deploy time and `modal.Server.from_name("sela-llm", "serve", environment_name=...).get_url()` returns it. Modal's vLLM example shows the shape `https://<workspace>--<app>-<server>.us-east.modal.direct`. The expected shape for this app, `https://<workspace>[-<env-suffix>]--sela-llm-serve.us-east.modal.direct`, is inferred, so **copy the real URL; do not construct it.** The example URL in the Go `Config` comment (`*.modal.run`) is the Web Function domain, not the Servers domain.

## 5. Platform facts used for limits and cost

- `@app.server` in modal 1.6.2: `inspect.signature(modal.App.server)` matches https://modal.com/docs/sdk/py/latest/App. Relevant defaults: `startup_timeout=30`, `port=8000`, `unauthenticated=False`, `routing_region="us-east"`, `exit_grace_period=0`. `max_concurrency` must be ≥ `target_concurrency`.
- `max_containers=1`: Modal recommends leaving `target_concurrency` unset for singletons because `max_containers=1` "will prevent Modal from bringing up a replacement to gracefully shift traffic during a rolling redeployment" (servers guide). Sela accepts this for a hard cost ceiling and deploys with `--strategy recreate`.
- GPU: `gpu="H100"` may be placed on an H200, and "this automatic upgrade does not change the cost" (https://modal.com/docs/guide/gpu).
- Prices (https://modal.com/pricing, observed 2026-10-26): H100 $0.001097/s, H200 $0.001261/s, A100-80GB $0.000694/s, CPU $0.0000131/core/s, memory $0.00000222/GiB/s, Volumes $0.09/GiB/month ("Volumes include up to 1 TiB free"), egress $0.04/GiB. Starter: $0/month with $30/month credits. Team: $250/month with $100/month credits. Rollbacks are Team/Enterprise only (https://modal.com/docs/guide/managing-deployments). The cost table is in [README.md](README.md#cost).

## 6. Open items (all UNVERIFIED)

1. Image build on Modal with the `python` symlink and the empty entrypoint.
2. VRAM fit at `--gpu-memory-utilization 0.90`, `--max-model-len 16384`, `--max-num-seqs 8`, `--language-model-only`. Cold-start time.
3. Layered auth (§4e) through Modal's proxy.
4. Whether `response_format: json_schema` is enforced with the `muse_glimmer` reasoning parser (§4d).
5. Latency and tokens per second at `reasoning_strength: low` for Sela's prompts. Indonesian and code-switched quality (Sela's own evaluation, docs/EVALUATION.md).
6. The optional HF token Secret path (`SELA_HF_SECRET_NAME`) has not been exercised; the model is ungated, so it is not needed.
