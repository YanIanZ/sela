# Development and runtime status

Authority: [SPEC](../SPEC.md) C5/C10/C11, I.ops, V15.

## What runs now — v1

The repository still contains Next.js/TypeScript code and its inherited runtime. The docs refactor does not create a Go module, Vite app, new API or Meta adapter. Existing commands run from the repo root with Node.js 22.18+ and npm 10+; `.nvmrc` selects 22.

```sh
npm ci
cp .env.example .env.local
npm run dev
```

Open localhost:3000. Current credentials and legacy provider settings are described in the checked-in template; they are not v2 requirements. No key is needed for deterministic/manual paths. Do not claim that a v1 route demonstrates the new complete care journey.

Current checks:

```sh
npm run lint
npm run lint:services
npm run typecheck
npm test
npm run build
```

Full tests need `redis-server` or Docker for the isolated v1 Redis runtime. Live probes are opt-in and separate from mock tests. Existing production Redis checks remain in CI until the v1 runtime is retired.

## Target Go runtime — not yet runnable

T2/T3 will verify and pin a Go toolchain, initialize a module, establish build/test commands and add a static React frontend. Until then, do not advertise `go run`, `go test`, a v2 dev server or deployment commands as working project commands.

Keep v1 runnable while one v2 vertical slice is built. Go serves static assets and API in the target; Node is a frontend build dependency. A session-scoped in-memory store removes the demo's database/Redis requirement after cutover, not before it.

## Documentation work

Update `SPEC.md` first when changing a requirement, then the affected active docs. Archived docs describe snapshots and are never silently promoted to current requirements. Preserve relative links and source comments when moving files. `docs/DOCUMENT_INVENTORY.md` must cover every tracked Markdown file and legacy asset.

Verify links, anchors, task/invariant references, document status, formatting, and unchanged runtime behavior when fixtures or source-reference paths move. No new implementation tests are needed for prose-only edits. See [contributing](../CONTRIBUTING.md).
