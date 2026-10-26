# Sela LLM on Modal: owner runbook

> **Status: nothing has been deployed from this repository yet.** No Modal app, Secret, Volume or proxy token exists because of this code, and no live inference has been observed. Every latency, VRAM and cost figure below is an estimate from documentation and list prices, not a measurement. Until the [smoke test](#9-run-the-smoke-test) passes against your deployment, run the Go app with `SELA_LLM_MODE=disabled` (manual path) or the labelled test double. Never present either as Meta inference.

This directory serves Meta's open-weight **Muse Glimmer 30B** (`meta-models/Muse-Glimmer-30B` @ `cec94fcfd6512f2cfe708138c32533b346ea4f69`, Apache-2.0) with **vLLM 0.32.0** on **one Modal H100**. It exposes an OpenAI-compatible API for `internal/llm`. The decision is in [ADR-0001](../docs/adr/ADR-0001-meta-open-weight-model-on-modal.md); verified model and serving facts, with sources, are in [MODEL_NOTES.md](MODEL_NOTES.md).

| File | Purpose |
| --- | --- |
| [sela_llm.py](sela_llm.py) | Modal app `sela-llm`: server `serve` (GPU, vLLM) and function `download_weights` (CPU, pinned weights into a Volume) |
| [smoke_test.py](smoke_test.py) | Stdlib-only end-to-end check of a deployment; prints a JSON report |
| [requirements.txt](requirements.txt) | Pinned Modal client (`modal==1.6.2`) for your machine or CI |
| [MODEL_NOTES.md](MODEL_NOTES.md) | Request parameters (reasoning strength, sampling, streaming fields), risks, sources |
| [../tests/modal/](../tests/modal/test_sela_llm_config.py) | Offline tests (no Modal, no network) |
| [../.github/workflows/modal-deploy.yml](../.github/workflows/modal-deploy.yml) | Optional manual-dispatch deploy (and smoke test) |

```
Go app (internal/llm) ──HTTPS──▶ Modal proxy ──▶ vLLM (port 8000) on 1× H100 ──▶ /root/.cache/huggingface (Volume sela-hf-cache, read-only)
  Modal-Key / Modal-Secret        401 without a proxy token        Authorization: Bearer <VLLM_API_KEY>       /root/.cache/vllm (Volume sela-vllm-cache)
  Authorization: Bearer <key>     503 while no container is ready  request logging off, no prompt logging
```

## 1. Prerequisites and budget decision

- [ ] A Modal account and workspace, with a **payment method**: GPUs need one, even on Starter with its $30/month included credits (https://modal.com/pricing).
- [ ] **An agreed GPU budget.** Read [Cost](#cost) and set a Workspace budget, plus optional per-Environment budgets, in the dashboard under Usage & Billing (https://modal.com/docs/guide/budgets). A budget close to the included credits ($30 on Starter) is a sensible first cap.
- [ ] Python 3.10–3.15 locally, and a virtualenv: `python3 -m venv .venv-modal && . .venv-modal/bin/activate && pip install -r modal/requirements.txt`.
- [ ] You accept the Muse Glimmer Usage Policy as summarised in ADR-0001: synthetic or explicitly fictional practice text only, and not intended for people under 18.

## 2. Modal token (CLI login)

```bash
modal token new                 # opens a browser; stores the token in ~/.modal.toml
modal token info                # confirms which workspace you are using
```

On a machine without a browser, run `modal token set` *without* `--token-id`/`--token-secret` so it prompts for them and they stay out of shell history. In CI, use repository secrets `MODAL_TOKEN_ID` and `MODAL_TOKEN_SECRET` (see [CI](#ci-optional)).

## 3. Environments (dev, staging)

Each environment has its own Secrets, Volumes and app instance. Nothing in the code names a workspace or an environment; you choose one with `-e` on every command.

```bash
modal environment create dev
modal environment create staging
modal environment list
```

Each environment gets a *web suffix* that appears in its URLs; one environment may have none (https://modal.com/docs/guide/environments). The commands below use `-e dev`. Repeat them with `-e staging`.

## 4. Secrets (names and keys only; values never go in the repo)

| Modal Secret (per environment) | Key | Required | Used by |
| --- | --- | --- | --- |
| `sela-llm-api` | `VLLM_API_KEY` (≥ 32 random characters) | yes; deploy fails without it, and the server refuses to start with a shorter key | vLLM API key; the same value becomes `SELA_LLM_API_KEY` in the Go app |
| any name, e.g. `sela-hf-token` | `HF_TOKEN` | no (the model is ungated) | `download_weights`, only when you export `SELA_HF_SECRET_NAME=<that name>` before `modal run` (this path is untested) |

Create `sela-llm-api` in the dashboard (Secrets → Custom, in the right environment), or from a file that never touches the repo or your shell history:

```bash
umask 077; tmp="$(mktemp)"
python3 -c 'import json, secrets; print(json.dumps({"VLLM_API_KEY": secrets.token_urlsafe(48)}))' > "$tmp"
modal secret create -e dev sela-llm-api --from-json "$tmp"
# Copy the value from "$tmp" into your password manager (it is SELA_LLM_API_KEY), then:
rm -f "$tmp"
modal secret list -e dev
```

Use a different key per environment so a leaked staging key does not unlock dev.

## 5. Download the pinned weights (CPU, once per environment)

```bash
modal run -e dev modal/sela_llm.py::download_weights
```

This downloads about 59.6 GB at the pinned revision into Volume `sela-hf-cache`, which it creates if it is missing. It verifies all 14 files (SHA-256 for LFS files, git blob SHA-1 for the rest) and writes a "verified" marker. It runs on CPU (4–8 cores, 8–32 GiB); expect tens of minutes. Running it again is cheap, because it only re-checks sizes. To re-hash everything, run `modal run -e dev modal/sela_llm.py::download_weights --deep-verify`. Check the result with `modal volume ls -e dev sela-hf-cache sela-verified`.

The GPU server never downloads anything. It runs with `HF_HUB_OFFLINE=1`, mounts the weights read-only, and **refuses to start** if the marker or any file is missing. That makes a missing step cost seconds of GPU time, not a 60 GB download billed at GPU rates.

`modal run` registers an ephemeral copy of the whole app, so the Secret `sela-llm-api` must already exist (step 4). The ephemeral server is never called and starts no GPU.

## 6. Deploy

```bash
modal deploy -e dev --strategy recreate modal/sela_llm.py
modal deploy -e staging --strategy recreate modal/sela_llm.py
```

`--strategy recreate` is deliberate. With `max_containers=1`, Modal cannot start a replacement container next to the old one during a rolling deploy (https://modal.com/docs/guide/servers). Expect a short outage: in-flight requests may be interrupted (the server sets a 2-minute `exit_grace_period` for draining), and the next request triggers a cold start. Deploy when no demo is running. Deploying costs nothing until a request arrives (`min_containers=0`). The first deploy builds the image (pulls `vllm/vllm-openai:v0.32.0`, about 10 GB) on CPU.

What the deployment does:

- Requests without a Modal proxy token get **401** from Modal's proxy and start no container.
- While no container is ready, valid requests get **503** immediately; Modal does not queue them.
- The first valid request starts one H100 container. A cold start loads the image, reads the weights from the Volume, compiles (cached in `sela-vllm-cache` after the first boot) and captures CUDA graphs. Expect minutes; the hard limit is 20 minutes. The first boot is slower than later ones.
- At most 8 concurrent requests run (vLLM `--max-num-seqs 8`); the 9th gets **429**.
- After 5 idle minutes the container stops and the cost returns to zero.

## 7. Get the URL

The URL is printed by `modal deploy`, shown on the app's dashboard page and by `modal app info sela-llm -e dev`, and returned by:

```bash
python -c 'import modal; print(modal.Server.from_name("sela-llm", "serve", environment_name="dev").get_url())'
```

Copy it exactly; do not construct it. By Modal's examples the shape should be `https://<workspace>[-<env-suffix>]--sela-llm-serve.us-east.modal.direct`, but this is inferred, not observed. Servers use `.modal.direct`, not the `.modal.run` domain of Web Functions.

## 8. Create a proxy auth token

```bash
modal workspace proxy-tokens create --name sela-dev     # prints the token id and secret ONCE
modal workspace proxy-tokens list
```

Store both values in your secret manager immediately. The token id (prefix `wk-`) is `SELA_MODAL_KEY` and the secret (prefix `ws-`) is `SELA_MODAL_SECRET`. On RBAC-enabled workspaces, scope each token to one environment with `modal workspace proxy-tokens allow <token-id> dev`; tokens outside the app's environment get 403 (https://modal.com/docs/guide/webhook-proxy-auth). Use one token per environment and per consumer, so you can revoke one with `modal workspace proxy-tokens delete`.

### Configure the Go app

| Variable | Value |
| --- | --- |
| `SELA_LLM_MODE` | `modal` |
| `SELA_LLM_BASE_URL` | the URL from step 7 (https, no trailing slash, no path) |
| `SELA_LLM_API_KEY` | the `VLLM_API_KEY` value of that environment's `sela-llm-api` Secret |
| `SELA_MODAL_KEY` | proxy token id |
| `SELA_MODAL_SECRET` | proxy token secret |
| `SELA_LLM_SERVED_MODEL` | `sela-glimmer-cec94fc` |
| `SELA_LLM_MODEL_ID` | `meta-models/Muse-Glimmer-30B` |
| `SELA_LLM_MODEL_REVISION` | `cec94fcfd6512f2cfe708138c32533b346ea4f69` |

Also keep `SELA_LLM_CONCURRENCY` at 8 or below (Modal's `max_concurrency`). Keep `SELA_LLM_OP_TIMEOUT` above the cold-start time if requests may hit a cold server. `SELA_LLM_WARM_WINDOW` (default 5m) matches the 5-minute scale-down. Request parameters the adapter must send are listed in [MODEL_NOTES.md §4](MODEL_NOTES.md#4-request-parameters-sela-must-use): `chat_template_kwargs.reasoning_strength` (not `reasoning_effort`), no `temperature: 0`, reasoning arriving in `delta.reasoning`, empty content treated as a failure, and model identity read from `/v1/models` (not `/health`).

## 9. Run the smoke test

```bash
export SELA_LLM_BASE_URL='https://…modal.direct'      # from step 7
read -rs SELA_LLM_API_KEY && export SELA_LLM_API_KEY  # paste, Enter (not echoed)
read -rs SELA_MODAL_KEY && export SELA_MODAL_KEY
read -rs SELA_MODAL_SECRET && export SELA_MODAL_SECRET
python3 modal/smoke_test.py --cold-start-budget 1200 > smoke-report.json; echo "exit $?"
```

**This starts a GPU container** (about $0.80–$1.00 including the idle scale-down; see [Cost](#cost)). The report on stdout contains no secrets and no generated text; add `--show-output` to see the JSON content the model produced. Progress goes to stderr. The checks:

| Check | Pass condition |
| --- | --- |
| `unauthenticated_rejected` | `GET /health` without credentials gets 401/403. vLLM never guards `/health`, so only Modal's proxy can reject it. If this is 200 or 503, the endpoint is public: stop the app |
| `cold_start` | `/v1/models` reaches 200 within the budget, retrying 503/429 with capped exponential backoff. Reports `cold_start_seconds` |
| `models` | `sela-glimmer-cec94fc` is listed (`root_contains_revision` should be `true`) |
| `vllm_key_enforced` | proxy token plus a wrong vLLM key gets 401. Together with `cold_start`, this proves both auth layers work through Modal's proxy (an open question, see MODEL_NOTES §4e) |
| `chat_stream` | one streamed completion on a synthetic Indonesian sentence returns schema-valid JSON. Reports `ttfb_seconds`, `total_seconds`, `completion_tokens`, `reasoning_present`/`reasoning_fields`, `finish_reason` and `quality_signals` |

The smoke test never follows redirects, so the proxy token and vLLM key only ever go to the host in `SELA_LLM_BASE_URL`: any 3xx fails the check that received it with `redirect not followed (HTTP 3xx to <scheme>://<host>)`. If you see that, the base URL is wrong (for example `http://` or an old URL); copy it again from step 7.

Useful options: `--skip-chat` (auth and warm-up only), `--reasoning-strength medium`, `--max-tokens 4096`, and `--extra-body '{"include_reasoning": false}'` for the experiments in MODEL_NOTES §4d. Exit codes: 0 pass, 1 a check failed, 2 configuration error. Record the report (it is safe to share) before labelling any output `live_meta`.

## 10. Logs

```bash
modal app logs sela-llm -e dev            # last 100 entries
modal app logs sela-llm -e dev -f         # follow
modal app logs sela-llm -e dev --since 2h
```

The logs show the vLLM command line (it contains no secrets; the API key reaches vLLM only through the `VLLM_API_KEY` environment variable), startup progress, the KV-cache and maximum-concurrency lines, and vLLM's periodic aggregate throughput stats. **Prompts and outputs are not logged:** vLLM request logging is off (`--no-enable-log-requests`), the uvicorn access log is off, the log level is INFO, and `smoke_test.py` prints no content by default. What Modal's platform itself records about proxied requests was not assessed. The privacy page must say that Modal processes AI requests.

## 11. Stop: guaranteed zero compute cost

```bash
modal app stop sela-llm -e dev            # asks for confirmation; -y to skip
modal app list -e dev                     # the app should show as stopped
```

`modal app stop` permanently stops the app and terminates its containers. To bring it back, deploy again (step 6). Even without stopping, no GPU is billed after 5 idle minutes, provided nothing keeps calling the API. An instant off-switch that does not touch Modal is `SELA_LLM_MODE=disabled` in the Go app.

Storage is separate from compute: the Volumes keep about 56 GiB of weights plus the compile cache. Modal's pricing page lists Volumes at $0.09/GiB/month and also says "Volumes include up to 1 TiB free". For zero storage as well, run `modal volume delete -e dev sela-hf-cache` and `modal volume delete -e dev sela-vllm-cache` (you then need to repeat step 5). Check spend with `modal environment billing summary` or on the dashboard.

## 12. Rollback

- **Team or Enterprise plan:** `modal app rollback sela-llm -e staging` (previous version) or `modal app rollback sela-llm -e staging v3`, optionally with `--strategy recreate`. `modal app history sela-llm -e staging` lists versions. Rollbacks are not available on Starter (https://modal.com/docs/guide/managing-deployments).
- **Starter plan:** redeploy the previous commit: `git checkout <previous-good-commit> -- modal/ && modal deploy -e staging --strategy recreate modal/sela_llm.py`, then restore your working tree.
- In both cases, re-run the smoke test. If the model revision changed, the Go app's `SELA_LLM_SERVED_MODEL` and `SELA_LLM_MODEL_REVISION` must change with it; otherwise Go reports `model_mismatch`, which is the intended behaviour.

## Cost

List prices observed on https://modal.com/pricing on **2026-10-26** (re-check before deciding the budget): H100 $0.001097/s, CPU $0.0000131/core/s, memory $0.00000222/GiB/s. `gpu="H100"` may run on an H200 "at no extra cost" (https://modal.com/docs/guide/gpu). Billing uses the higher of requested and used CPU and memory (https://modal.com/docs/guide/resources). The app requests 4 cores and 16 GiB. It limits CPU at 8 cores (a soft limit: Modal throttles above it) and memory at 128 GiB (a hard limit: the container is OOM-killed above it). Those limits bound the hourly cost.

| Item (one container, `max_containers=1`) | Per hour |
| --- | --- |
| H100 | $3.95 |
| CPU 4 cores requested / 8 cores cap | $0.19 / $0.38 |
| Memory 16 GiB requested / 128 GiB cap | $0.13 / $1.02 |
| **Warm container, at requests** | **$4.27** |
| **Hard ceiling per hour (GPU + CPU cap + memory cap)** | **$5.35** |
| Zero requests for more than 5 minutes, or app stopped | $0 compute |

| Scenario (estimates, not measurements) | GPU only | Ceiling |
| --- | --- | --- |
| Smoke test: ~5 min cold start + ~1 min checks + 5 min idle | $0.72 | $0.98 |
| Demo session: 5 min cold start + 10 min use + 5 min idle | $1.32 | $1.78 |
| Failed cold start that hangs until the 20-min `startup_timeout` | $1.32 | $1.78 |
| **Something keeps calling the API all day (e.g. a health poller)** | $94.78 | **$128.39 / 24 h** |
| `download_weights` (CPU only, ~30 min, ceiling $0.63/h) | n/a | ≈ $0.13–$0.32 |
| Volumes (~56 GiB) | n/a | $0–$5.04/month (list price, but "up to 1 TiB free") |

Because of `max_containers=1`, the burn rate can never exceed one GPU, whatever the traffic. The real risk is time: anything that sends authenticated requests at least every 5 minutes keeps the GPU warm indefinitely. The Go app computes AI status without contacting Modal (see the decision log), and nothing should poll `/health` on a timer. The Workspace budget from step 1 is the backstop.

## Pre-flight checklist (before every deploy)

- [ ] Payment method on file, and Workspace/Environment budget set (step 1).
- [ ] Logged in to the intended workspace (`modal token info`), and targeting the intended environment (`-e dev` or `-e staging`).
- [ ] Secret `sela-llm-api` exists in that environment with key `VLLM_API_KEY` of at least 32 characters (`modal secret list -e <env>`).
- [ ] `download_weights` succeeded in that environment (marker present in `sela-hf-cache/sela-verified/`).
- [ ] Offline tests pass: `python3 -m unittest discover -s tests/modal -p 'test_*.py' -v`.
- [ ] `git diff` shows no change to `MODEL_REVISION`, `SERVED_MODEL_NAME` or `VLLM_VERSION` that the Go configuration has not also received.
- [ ] Proxy token for this environment exists and is stored in a secret manager (step 8).
- [ ] You know how to stop it: `modal app stop sela-llm -e <env>`.
- [ ] After deploying: run the smoke test, keep the report, then confirm that the container scales to zero (dashboard, or `modal app logs` stops showing activity after about 5 minutes).

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| Deploy fails: Secret not found / missing key | Step 4 not done in this environment |
| Image build fails at the `python --version` step | The vLLM image changed its Python layout; see the setup command in `sela_llm.py` |
| 503 for the whole budget | Container failing to start. Read `modal app logs`. "pinned weights are missing or unverified" means run step 5. "VLLM_API_KEY … refusing to start" means fix the Secret. "vLLM exited with code …" means a flag or out-of-memory problem |
| CUDA out of memory at startup | Lower `MAX_MODEL_LEN` or `GPU_MEMORY_UTILIZATION`, or use `--enforce-eager` (Meta's tight-memory variant). Change these in code, then redeploy |
| 401 with body `modal-http: …` | Missing or wrong proxy token |
| 403 | RBAC: the token is not allowed in this environment |
| 401 with JSON `{"error": "Unauthorized"}` | vLLM key mismatch, or Modal's proxy did not forward `Authorization` (MODEL_NOTES §4e) |
| 429 | More than 8 concurrent requests |
| `chat_stream` has `schema_valid: false` | Possibly no grammar enforcement under the reasoning parser (MODEL_NOTES §4d); the Go-side validation must reject such output |
| Empty content, `finish_reason` `length` or `stop` | Reasoning used up `max_tokens`; raise `--max-tokens` or keep `reasoning_strength` at `low` |

## CI (optional)

[.github/workflows/modal-deploy.yml](../.github/workflows/modal-deploy.yml) runs only on **manual dispatch** (Actions → "Modal deploy (manual)"), never on push or pull requests. Inputs: `environment` (`dev`/`staging`) and `run_smoke` (default `false`). It installs `modal==1.6.2`, runs the offline tests, runs `download_weights` (CPU, idempotent), deploys with `--strategy recreate`, and optionally runs the smoke test against the URL it resolves from Modal for that environment.

Repository secrets: `MODAL_TOKEN_ID` and `MODAL_TOKEN_SECRET` (create a dedicated Modal API token for CI). For the smoke test, also `SELA_LLM_API_KEY`, `SELA_MODAL_KEY` and `SELA_MODAL_SECRET`, plus optionally `SELA_LLM_BASE_URL`; when set, it must match the chosen environment's URL or the job fails. Repository secrets are shared by both environments, so if dev and staging use different vLLM keys, run the smoke test locally instead. Values reach steps only through `env:` and are never echoed.
