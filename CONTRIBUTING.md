# Contributing to Sela

Start with [SPEC.md](SPEC.md), then the [documentation index](docs/README.md). Go is the selected v2 backend; code is still v1 Next.js/TypeScript. Never describe a proposed endpoint, provider or deployment as implemented.

## Work boundaries

Update the spec before changing requirements; use [FORMAT.md](FORMAT.md) for stable IDs and task status. Supporting docs must cite the relevant invariants and agree with the spec. Archived v1 docs are evidence/reference, not current directives. Preserve original contributor and license notices.

Keep changes focused. Describe the resulting behavior, limitations and checks in each PR. Use synthetic data in fixtures, screenshots, issues and recordings. No real professional/patient registration, medical claims or real care transaction belongs in this demo.

## Commands that work now

Use Node.js 22.18+ and npm 10+; `nvm install && nvm use` selects the CI major version.

```sh
npm ci
cp .env.example .env.local
npm run dev
```

Current checks:

```sh
npm run lint
npm run lint:services
npm run typecheck
npm test
npm run build
```

Full v1 tests need `redis-server` or Docker. Go module/version/build/test commands will be documented when T2/T3 create them. See [development](docs/DEVELOPMENT.md).

## Review focus

Preserve ID/EN parity, crisis interruption, independent consent/approval/share decisions, exact-version snapshots, session isolation and truthful AI provenance. Tests changing those behaviors should exercise failure/race/revocation cases, not merely mirror implementation. Never publish keys, raw stories or model payloads in logs.

Docs-only changes need link/anchor checks and source-reference validation; runtime fixture moves also need affected tests. Update [document inventory](docs/DOCUMENT_INVENTORY.md) when adding/moving/removing documentation. Check [validation](docs/VALIDATION.md) before claiming v2 acceptance.

Use [GitHub Issues](https://github.com/YanIanZ/sela/issues) for reproducible bugs. Report sensitive vulnerabilities privately through [SECURITY.md](SECURITY.md).
