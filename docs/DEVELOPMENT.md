# Development and runtime status

Authority: [SPEC](../SPEC.md) C5/C10/C11, I.ops, V15.

## One runtime

Sela runs as a single Go process serving the React 19 + TypeScript + Vite 7
frontend in `web/`, with PostgreSQL 16 for `/api/v3` and an optional Meta model
on Modal. The inherited Next.js app (`apps/web`), its `packages/*` and
`services/*` runtime, and the v1 CI workflow were retired on 2026-10-29; their
useful content (self-help cards, help-destination checklists and support
options) is now embedded under `internal/fixtures/data/`, and the v1 crisis test
snapshot is under `internal/safety/testdata/v1/`.

| | Runtime |
| --- | --- |
| Code | `cmd/`, `internal/` (Go 1.26.10, D-054), `web/` (React 19 + TypeScript + Vite 7), `modal/` (Modal + vLLM), `e2e/`, `eval/`, `scripts/ops/`, `scripts/research/` |
| Data | v2 demo (`/api/v2`): in-memory sessions, embedded synthetic fixtures. v3 (`/api/v3`): PostgreSQL 16 with per-user encryption (ADR-0002) |
| AI | Meta Muse Glimmer on Modal (not yet deployed) or a labelled test double |
| CI | `.github/workflows/v2.yml` |

## Commands

See the [runbook](RUNBOOK.md) for configuration. From the repository root:

```sh
(cd web && npm ci && npm run build)
go run ./cmd/sela                                   # AI disabled; open http://localhost:8080
# with the labelled test double:
go run ./cmd/fakellm -addr 127.0.0.1:8090 &
SELA_ENV=development SELA_SECURE_COOKIES=false SELA_LLM_MODE=test_double SELA_ALLOW_TEST_DOUBLE=true \
  SELA_LLM_BASE_URL=http://127.0.0.1:8090 go run ./cmd/sela
```

`/api/v3` needs PostgreSQL 16 (`SELA_DATABASE_URL`, a development KEK from
`sela keys generate-kek --env development`; [runbook](RUNBOOK.md) §1 and §7);
without it, its routes answer `503 store_unavailable` in development.

Checks: `gofmt -l cmd internal`, `go vet ./...`, `go test -race ./...` (with
`SELA_TEST_DATABASE_URL` set and `SELA_REQUIRE_DB_TESTS=1` the PostgreSQL tests
are required, as in CI), `SELA_DRILL_DATABASE_URL="$SELA_TEST_DATABASE_URL"
scripts/ops/restore-drill.sh` (the URL in the environment, not
`--database-url`; [runbook §8](RUNBOOK.md#8-backup-and-restore)),
`cd web && npm run typecheck && npm test && npm run build`,
`python3 -m unittest discover -s tests/research -p 'test_*.py'`,
`python3 -m unittest discover -s tests/modal -p 'test_*.py'`,
`python3 scripts/research/corpus_pipeline.py verify`,
`E2E_BASE_URL=http://127.0.0.1:8080 node e2e/v3-frontend.e2e.mjs` (app and
database running), `go run ./cmd/sela-eval -mode offline|test_double`.

Frontend dependencies are pinned exactly in `web/package.json`. Go uses the
standard library plus the approved modules pgx v5.10.0, `golang.org/x/crypto`
v0.57.0 and `golang.org/x/text` v0.42.0 (Unicode NFKC normalization of
passwords); all three are pinned in `go.mod` and `go.sum`. The frontend has no
router or UI framework dependency beyond React.

## Documentation work

Update `SPEC.md` first when a requirement changes, then the affected docs, the
[decision log](DECISION_LOG.md) and the [inventory](DOCUMENT_INVENTORY.md).
Status claims must point at tests, evaluation results or recorded evidence
([validation](VALIDATION.md)).
