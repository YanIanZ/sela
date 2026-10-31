# Contributing to Sela

Thank you for helping build a calmer, safer place to be heard. Sela handles some of the most sensitive things a person can write, so contributions are held to a high bar for privacy, safety and honesty. Please read this guide and the [Code of Conduct](CODE_OF_CONDUCT.md) before you start.

## Before you start

- **Read the source of truth.** [SPEC.md](SPEC.md) holds requirements, invariants and task status; [FORMAT.md](FORMAT.md) defines its IDs and status words. The [documentation index](docs/README.md) and the [current plan](docs/roadmap/SAFE_FLOW_AND_TODO.md) explain where things are going.
- **Open an issue first** for anything larger than a small fix, so scope and safety questions are settled before code is written.
- **Synthetic data only.** Never put a real story, health detail, name, contact, credential or professional's identity in code, tests, fixtures, screenshots, issues, PRs or recordings. Production (https://sela.vertalt.my.id) is the only place real people's data may exist.

## Ground rules

| Rule | Why |
| --- | --- |
| Safety runs first: the crisis router screens text before storage, publishing or any model call | A model must never be the first reader of a crisis |
| Separate consents for AI processing, storage, memory, condition map and sharing; disabling AI keeps a useful manual path | People decide what happens to their words |
| Every AI output is labelled with its provider and passes the guards (and the verifier when flagged) | No silent model, no unlabelled output |
| No diagnosis, treatment, triage claim or clinical-efficacy claim, in code or copy | Sela is not a clinical service |
| Demo professionals and partners are fictional and labelled as such | No invented availability or real hand-off |
| Owner isolation: the owner comes only from the session; another person's resource answers `404` | No cross-user exposure |
| Never log stories, quotes, model payloads, tokens, access codes, keys, usernames, user ids or IP addresses | Logs are not a data store |
| Indonesian and English stay at parity in every visible string, including errors | Bilingual is a requirement, not a translation pass |

Never describe a proposed endpoint, provider or deployment as implemented, and never present a test-double run as Meta output. Status claims need a test, an evaluation result or recorded evidence.

## Development setup

Requirements: Go 1.26, Node.js 22, PostgreSQL 16, Python 3.10+ (standard library only). Redis is optional. See the [README](README.md#getting-started) and the [runbook](docs/operations/RUNBOOK.md).

## Checks

Run what your change touches; CI runs the full set on every pull request.

```sh
gofmt -l cmd internal                       # must print nothing
go vet ./... && go test -race ./...
SELA_REQUIRE_DB_TESTS=1 SELA_TEST_DATABASE_URL='postgres://…/sela_test?sslmode=disable' go test -race -count=1 ./...
(cd web && npm ci && npm run typecheck && npm test && npm run build)
python3 -m unittest discover -s tests/research -p 'test_*.py'
go run ./cmd/sela-eval -mode offline
(cd e2e && npm ci) && E2E_BASE_URL=http://127.0.0.1:8080 node e2e/v3-frontend.e2e.mjs
```

Changes to the safety router (`internal/safety`), the AI guards (`internal/ai/guard`) or moderation must add test cases on **both** sides: phrases that must be caught and everyday phrases that must stay ordinary.

## Pull requests

1. Branch from `main`; keep each PR focused on one change.
2. Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat(talk): …`, `fix(safety): …`, `docs: …`).
3. Fill in the PR template: behaviour, checks run, limitations, and any consent, privacy, safety, migration or deployment effect.
4. Record a decision that changes behaviour in the [decision log](docs/decisions/DECISION_LOG.md); update the spec first when a requirement changes.
5. Update docs and the [document inventory](docs/status/DOCUMENT_INVENTORY.md) when you add, move or remove documentation.
6. A maintainer review is required; nothing merges automatically and production deploys are separate from merges.

## Credits and licence

Sela is © 2026 Sela — YanIanZ, released under the [MIT License](LICENSE). It is an individual continuation that replaces the original Dengar.in; code forked from Dengar.in keeps the Dengar.in Team notice, and its origin is recorded in the [provenance record](docs/product/SELA_PROVENANCE.md). By contributing you agree that your contribution is licensed under the same terms. Third-party data and content keep their own terms.

## Getting help

Use [GitHub Issues](https://github.com/YanIanZ/sela/issues) for reproducible bugs and feature proposals. Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md). GitHub is not an emergency channel: if you or someone else is in danger, contact local emergency services.
