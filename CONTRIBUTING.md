# Contributing to Sela

Start with [SPEC.md](SPEC.md), then the [documentation index](docs/README.md). Sela is built in the Go and React runtime (`cmd/`, `internal/`, `web/`, `modal/`, `e2e/`, `eval/`, `scripts/ops/`, `scripts/research/`); the inherited Next.js app was retired on 2026-10-29. Work items are in the [backlog](docs/product/BACKLOG.md). Never describe a proposed endpoint, provider or deployment as implemented, and never present a test-double run as Meta, Claude or DeepSeek output.

## Work boundaries

Update the spec before changing requirements; use [FORMAT.md](FORMAT.md) for stable IDs and task status, and the status words of [SPEC §G](SPEC.md#g-goal) for every status claim. Record engineering decisions in the [decision log](docs/decisions/DECISION_LOG.md) and keep [requirements traceability](docs/status/REQUIREMENTS_TRACEABILITY.md) and [implementation status](docs/status/IMPLEMENTATION_STATUS.md) truthful: a status needs a test, an evaluation result or recorded evidence. Preserve original contributor and licence notices.

Keep changes focused. Describe the resulting behaviour, limitations and checks in each PR. Use synthetic data in fixtures, screenshots, issues and recordings: v3 supports real use, but real people's data exists only in production (SPEC C24, V42), never in the repository, tests, the demo or staging. No real professional or patient data, medical claims or real care transactions belong in contributions. Do not lower an evaluation threshold to make a run pass.

## Checks (v2 demo and v3)

Go 1.26.10 (the `toolchain` line of `go.mod`, [D-054](docs/decisions/DECISION_LOG.md#d-054)), Node.js 22, Python 3.10+ (standard library only), and PostgreSQL 16 for the v3 database tests. From the repository root:

```sh
gofmt -l cmd internal                 # must print nothing
go vet ./... && go test -race ./...
# v3 database tests, required as in CI (local example URL from the runbook §2):
export SELA_TEST_DATABASE_URL='postgres://sela:sela-dev-only@127.0.0.1:5432/sela_test?sslmode=disable'
SELA_REQUIRE_DB_TESTS=1 go test -race -count=1 ./...
SELA_DRILL_DATABASE_URL="$SELA_TEST_DATABASE_URL" scripts/ops/restore-drill.sh   # the URL goes in the environment, not --database-url (runbook §8)
(cd web && npm ci && npm run typecheck && npm test && npm run build)
python3 -m unittest discover -s tests/research -p 'test_*.py'
python3 -m unittest discover -s tests/modal -p 'test_*.py'
python3 scripts/research/corpus_pipeline.py verify
go build -o build/sela ./cmd/sela && go build -o build/fakellm ./cmd/fakellm
(cd e2e && npm ci) && E2E_BASE_URL=http://127.0.0.1:8080 node e2e/v3-frontend.e2e.mjs   # app running, e.g. docker compose up
go run ./cmd/sela-eval -mode offline   # and -mode test_double
```

CI (`.github/workflows/v2.yml`, for pull requests to main and pushes to main) runs these commands except `sela-eval`, with a PostgreSQL 16 service and the migration round trip, plus a container build; run `sela-eval` locally. Live Modal deployment is manual (`.github/workflows/modal-deploy.yml`) and costs money; see [modal/README.md](modal/README.md).

## Review focus

Preserve ID/EN parity, crisis screening before storage and inference, independent AI/storage/sharing consents (per provider in v3), exact-version approval and shares, Meaning Lock grounding, session and owner isolation (v3: the owner only from the session cookie, every store method bound to the owner, another user's resource answers `404`), per-user encryption with AAD, migrations in their reserved range with the data-migration gates (ADR-0002 §4), recipient authorization (token alone is never access) and truthful AI provenance. Tests changing those behaviours should exercise failure, race, expiry and revocation cases, not merely mirror the implementation. Never log stories, quotes, model payloads, tokens, access codes, keys, usernames, user ids or IP addresses (SPEC V12).

Docs changes need link/anchor checks; update the [document inventory](docs/status/DOCUMENT_INVENTORY.md) when adding, moving or removing documentation. Check [validation](docs/validation/VALIDATION.md) before claiming acceptance.

Use [GitHub Issues](https://github.com/YanIanZ/sela/issues) for reproducible bugs. Report sensitive vulnerabilities privately through [SECURITY.md](SECURITY.md).
