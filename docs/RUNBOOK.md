# Sela v2 runbook

Authority: [SPEC](../SPEC.md) I.ops, C5, V9, V25. Commands below were run in this repository on 2026-10-26 unless marked **not run** (they need credentials or a budget that this environment does not have; see [blockers](BLOCKERS.md)).

## 1. Run locally from a clean checkout

Requirements: Go 1.24.7+ and Node.js 22+ (npm 10+). No database, Redis, Docker or API key is needed.

```sh
git clone https://github.com/YanIanZ/sela.git && cd sela
(cd web && npm ci && npm run build)          # builds web/dist
go build -o build/sela ./cmd/sela
go build -o build/fakellm ./cmd/fakellm      # TEST DOUBLE, never Meta

# A) AI disabled (manual path only) — the honest default without Modal:
SELA_ENV=development SELA_SECURE_COOKIES=false ./build/sela
# open http://localhost:8080

# B) Development with the labelled test double for AI steps:
./build/fakellm -addr 127.0.0.1:8090 &
SELA_ENV=development SELA_SECURE_COOKIES=false \
SELA_LLM_MODE=test_double SELA_ALLOW_TEST_DOUBLE=true SELA_LLM_BASE_URL=http://127.0.0.1:8090 \
./build/sela
```

In mode B every AI result is labelled **TEST DOUBLE — bukan Meta / not Meta**. The test double is refused when `SELA_ENV=production`.

Frontend development with hot reload: run the Go server on :8080, then `cd web && npm run dev` (Vite on :5173 proxies `/api`).

## 2. Checks

```sh
gofmt -l cmd internal                      # must print nothing
go vet ./...
go test -race -count=1 ./...
(cd web && npm run typecheck && npm test && npm run build)
python3 -m unittest discover -s tests/research -p 'test_*.py'
python3 -m unittest discover -s tests/modal -p 'test_*.py'
python3 scripts/research/corpus_pipeline.py verify
node e2e/journey.e2e.mjs                   # needs build/ binaries, web/dist and Playwright + Chromium
go run ./cmd/sela-eval -mode offline       # deterministic evaluation vs eval/thresholds.json
go run ./cmd/sela-eval -mode test_double   # + pipeline and fault-injection runs (TEST DOUBLE)
```

The v1 checks (`npm test`, `npm run lint`, …) still apply to the inherited Next.js app; see [development](DEVELOPMENT.md).

## 3. Configuration (environment)

| Variable | Default | Meaning |
| --- | --- | --- |
| `SELA_ENV` | `production` | `production`, `development` or `test`. Production forbids the test double and defaults to Secure cookies |
| `SELA_ADDR` | `:8080` | Listen address |
| `SELA_WEB_DIR` | `web/dist` | Built frontend |
| `SELA_PUBLIC_ORIGIN` | (request host) | Exact origin accepted for mutations, e.g. `https://sela.example` |
| `SELA_SECURE_COOKIES` | `true` in production | Set `false` only for plain-HTTP local runs |
| `SELA_TRUST_PROXY` | `false` | Use the last `X-Forwarded-For` element (across all header lines) for rate limiting; only behind a proxy that appends it |
| `SELA_MAX_SESSIONS` | `500` | Memory bound. When full, an ordinary session idle for 10 minutes is evicted first; a session with storage consent or an active share only after 30 idle minutes; otherwise new visitors get 503 `capacity` |
| `SELA_MAX_SESSIONS_PER_CLIENT` | `50` | Live sessions per client address (IPv6 /64); more → 429. Raise it if many demo visitors share one NAT |
| `SELA_METRICS_TOKEN` | — | Optional operator bearer token (≥16 characters, no spaces) for `GET /api/v2/metrics`; unset means the route does not exist |
| `SELA_RESEARCH_KB` | `data/research/corpus/knowledge_base.jsonl` | Reviewer corpus index |
| `SELA_LLM_MODE` | `disabled` | `disabled`, `modal` or `test_double` |
| `SELA_LLM_BASE_URL` | — | Modal server URL (https) or test double URL |
| `SELA_LLM_API_KEY` | — | vLLM API key (secret) |
| `SELA_MODAL_KEY`, `SELA_MODAL_SECRET` | — | Modal proxy-auth token pair (secret) |
| `SELA_LLM_SERVED_MODEL` | — | `sela-glimmer-cec94fc` |
| `SELA_LLM_MODEL_ID` | — | `meta-models/Muse-Glimmer-30B` |
| `SELA_LLM_MODEL_REVISION` | — | `cec94fcfd6512f2cfe708138c32533b346ea4f69` |
| `SELA_LLM_CONCURRENCY` | `2` | Concurrent model calls (admission) |
| `SELA_LLM_OP_TIMEOUT` | `6m` | Whole AI operation budget, including a cold start (maximum 15m; warm the model with the smoke test before a demo) |
| `SELA_LLM_WARM_WINDOW` | `5m` | Assumed warm time after a live call (= Modal scale-down) |
| `SELA_LLM_COLD_START_ESTIMATE` | `4m` | Shown to users while the model starts |
| `SELA_MAX_AI_OPS_PER_SESSION` | `8` | Per-session AI operations per 10 minutes |

Secrets come only from the environment of the host or a secret manager; never from files in the repository. The binary never logs them.

## 4. Container image

```sh
docker build -f deploy/v2/Dockerfile --build-arg VERSION=$(git rev-parse --short HEAD) -t sela:v2 .
docker run --rm -p 8080:8080 -e SELA_SECURE_COOKIES=false sela:v2
```

Deploy as **one instance** (sessions live in process memory; several instances would split them). Put HTTPS in front, set `SELA_PUBLIC_ORIGIN`, keep `SELA_SECURE_COOKIES=true`, and set `SELA_TRUST_PROXY=true` only behind a proxy that overwrites `X-Forwarded-For`. A restart erases all sessions, which the UI explains. On SIGTERM/SIGINT the server ends open event streams with a final content-free event and exits cleanly; a second signal forces exit. **Not run:** no hosting account was available (B-3).

## 5. Meta model on Modal

Follow [modal/README.md](../modal/README.md): budget approval, `modal token new`, environments (`dev`, `staging`), secrets, `modal deploy -e dev modal/sela_llm.py`, proxy token, smoke test, then set the `SELA_LLM_*` variables above and restart the Go app. Verify with `go run ./cmd/sela-eval -mode live`. **Not run** (B-1).

Cost controls (D-022): scale to zero after 5 minutes, at most one GPU container, proxy auth rejects anonymous requests before a container starts, and Sela's own per-session and per-address limits. To stop all spend immediately: `modal app stop sela-llm -e <env>` and set `SELA_LLM_MODE=disabled`.

Rollback: Go app — redeploy the previous image tag. Model server — on Modal Team plans `modal app rollback sela-llm -e <env>`; on Starter, check out the previous commit and `modal deploy` again. Switching AI off never affects the manual path.

## 6. Demo script (about 3 minutes)

Everything shown is synthetic; say so at the start. Use a fresh browser profile, plus a second profile/window for the fictional recipient.

1. **Start (0:00–0:20).** Welcome page: what Sela is and is not. *Mulai pelan-pelan* → boundaries and privacy (memory only, Modal processing, no real services) → anonymous → adult attestation.
2. **Permissions (0:20–0:35).** Allow AI processing; keep session-only retention. Point out they are separate and neither means sharing.
3. **Story (0:35–0:45).** Choose *Nadia* (Indonesian). The story contains a negation ("Aku tidak susah tidur"), a friend's opinion, uncertainty and a worry.
4. **Story Ledger (0:45–1:25).** *Bantu susun catatan dengan AI*: show the warming/running status, validated previews, provenance badge (`live_meta` only when the Modal endpoint is configured; otherwise say that this recording uses the test double or the manual path). Open an item: *Kata-katamu* vs *Rumusan Sela*; qualifier chips (negation, other person, past, uncertain); a "wording reverted" note if any. Confirm each item; approve version *n*. Edit one item to show approval is cleared and the version increments; approve again.
5. **Next Step (1:25–1:45).** Preferences: Indonesian, online, free or low cost, weekday evening, barrier *cost*. Show reasons (✓/✗), near matches and non-handoff steps. Choose a fictional slot.
6. **Opening Practice (optional, 1:45–2:00).** Draft lines, then type a practice attempt and show "mentioned / not yet mentioned" (no score).
7. **Share (2:00–2:20).** Select fields; the preview is exactly what the recipient will see; explicit consent sentence names version, recipient and duration; share. Receipt: link + separate access code; "sent ≠ care received".
8. **Recipient (2:20–2:40).** Second window: open the link, try a wrong code (rejected), enter the code, see only the shared fields, acknowledge, complete the scripted consultation with the fixture plan.
9. **Follow-up and revoke (2:40–3:00).** Owner *Perjalananku*: completed (simulated), fixture plan, check-in. Revoke; the recipient's next action fails with "access not available".

Supplementary clip (30 s): the crisis fixture stops the journey before any AI call and shows verified Indonesian help resources with sources; the no-AI path reaches a handoff; with `SELA_LLM_MODE=disabled` the AI button explains it is unavailable.

Record the commit, fixture version, provider mode and model revision with any recording. Never present a test-double or recorded run as live Meta inference.
