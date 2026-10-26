# Contributing to Sela

Start with [SPEC.md](SPEC.md), then the [documentation index](docs/README.md). New work goes to v2 (`cmd/`, `internal/`, `web/`, `modal/`, `e2e/`, `eval/`, `scripts/research/`); v1 (Next.js) stays runnable until the cutover (T9). Never describe a proposed endpoint, provider or deployment as implemented, and never present a test-double run as Meta inference.

## Work boundaries

Update the spec before changing requirements; use [FORMAT.md](FORMAT.md) for stable IDs and task status. Record engineering decisions in the [decision log](docs/DECISION_LOG.md) and keep [requirements traceability](docs/REQUIREMENTS_TRACEABILITY.md) and [implementation status](docs/IMPLEMENTATION_STATUS.md) truthful: a status needs a test, an evaluation result or recorded evidence. Preserve original contributor and licence notices.

Keep changes focused. Describe the resulting behaviour, limitations and checks in each PR. Use synthetic data in fixtures, screenshots, issues and recordings. No real professional or patient data, medical claims or real care transactions belong in this demo. Do not lower an evaluation threshold to make a run pass.

## v2 checks

Go 1.24, Node.js 22, Python 3.10+ (standard library only). From the repository root:

```sh
gofmt -l cmd internal                 # must print nothing
go vet ./... && go test -race ./...
(cd web && npm ci && npm run typecheck && npm test && npm run build)
python3 -m unittest discover -s tests/research -p 'test_*.py'
python3 -m unittest discover -s tests/modal -p 'test_*.py'
python3 scripts/research/corpus_pipeline.py verify
go build -o build/sela ./cmd/sela && go build -o build/fakellm ./cmd/fakellm
(cd e2e && npm ci) && node e2e/journey.e2e.mjs
go run ./cmd/sela-eval -mode offline   # and -mode test_double
```

CI runs the same commands (`.github/workflows/v2.yml`), plus a container build. Live Modal deployment is manual (`.github/workflows/modal-deploy.yml`) and costs money; see [modal/README.md](modal/README.md).

## v1 checks

Node.js 22.18+ and npm 10+: `npm ci`, `npm run lint`, `npm run lint:services`, `npm run typecheck`, `npm test`, `npm run build`. Full v1 tests need `redis-server` or Docker. See [development](docs/DEVELOPMENT.md).

## Review focus

Preserve ID/EN parity, crisis interruption before inference, independent AI/storage/sharing consents, exact-version approval and shares, Meaning Lock grounding, session isolation, recipient authorization (token alone is never access) and truthful AI provenance. Tests changing those behaviours should exercise failure, race, expiry and revocation cases, not merely mirror the implementation. Never log stories, quotes, model payloads, tokens, access codes or keys.

Docs changes need link/anchor checks; update the [document inventory](docs/DOCUMENT_INVENTORY.md) when adding, moving or removing documentation. Check [validation](docs/VALIDATION.md) before claiming acceptance.

Use [GitHub Issues](https://github.com/YanIanZ/sela/issues) for reproducible bugs. Report sensitive vulnerabilities privately through [SECURITY.md](SECURITY.md).
