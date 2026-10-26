# Development and runtime status

Authority: [SPEC](../SPEC.md) C5/C10/C11, I.ops, V15.

## Two runtimes in one repository

| | v2 (target) | v1 (inherited) |
| --- | --- | --- |
| Code | `cmd/`, `internal/` (Go 1.24), `web/` (React 19 + TypeScript + Vite 7), `modal/` (Modal + vLLM), `e2e/`, `eval/` | `apps/web` (Next.js), `packages/*`, `services/*` |
| Data | In-memory sessions, embedded synthetic fixtures | PostgreSQL/Supabase, Redis |
| AI | Meta Muse Glimmer on Modal (not yet deployed) or labelled test double | v1 providers incl. AMD (historical) |
| CI | `.github/workflows/v2.yml` | `.github/workflows/quality.yml` |

Both are kept runnable until v2 acceptance (T9). New work goes to v2.

## v2 commands that work now

See the [runbook](RUNBOOK.md) for configuration. From the repository root:

```sh
(cd web && npm ci && npm run build)
go run ./cmd/sela                                   # AI disabled; open http://localhost:8080
# with the labelled test double:
go run ./cmd/fakellm -addr 127.0.0.1:8090 &
SELA_ENV=development SELA_SECURE_COOKIES=false SELA_LLM_MODE=test_double SELA_ALLOW_TEST_DOUBLE=true \
  SELA_LLM_BASE_URL=http://127.0.0.1:8090 go run ./cmd/sela
```

Checks: `gofmt -l cmd internal`, `go vet ./...`, `go test -race ./...`, `cd web && npm run typecheck && npm test && npm run build`, `python3 -m unittest discover -s tests/research -p 'test_*.py'`, `python3 -m unittest discover -s tests/modal -p 'test_*.py'`, `python3 scripts/research/corpus_pipeline.py verify`, `node e2e/journey.e2e.mjs`, `go run ./cmd/sela-eval -mode offline|test_double`.

Frontend dependencies are pinned exactly in `web/package.json`; Go uses the standard library only (no `go.sum`). The frontend has no router or UI framework dependency beyond React.

## v1 commands (unchanged)

Node.js 22.18+ and npm 10+:

```sh
npm ci
cp .env.example .env.local
npm run dev        # http://localhost:3000
npm run lint && npm run lint:services && npm run typecheck && npm test && npm run build
```

Full v1 tests need `redis-server` or Docker. Baseline on 2026-10-26 before v2 work: `npm test` passed (926 node:test cases plus script suites), lint and typecheck passed, research Python tests passed (11).

## Documentation work

Update `SPEC.md` first when a requirement changes, then the affected docs, the [decision log](DECISION_LOG.md) and the [inventory](DOCUMENT_INVENTORY.md). Status claims must point at tests, evaluation results or recorded evidence ([validation](VALIDATION.md)).
