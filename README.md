<div align="center">

# Sela

**A little space to feel heard.** · *Ruang sejenak untuk didengar.*

A private, bilingual place to talk, write and check in — and to bring only the context you choose when you look for professional help.

**[Open Sela → sela.vertalt.my.id](https://sela.vertalt.my.id)**

[![CI](https://github.com/YanIanZ/sela/actions/workflows/v2.yml/badge.svg)](https://github.com/YanIanZ/sela/actions/workflows/v2.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![React 19](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)
![PostgreSQL 16](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)

<img src="docs/assets/screenshots/home-desktop.png" alt="Sela home screen: a greeting, a way into a conversation, shortcuts to write a note or check in, and six mood faces" width="860">

</div>

> **Sela is not a therapist, doctor, diagnosis, triage or crisis service.** It never contacts anyone on a person's behalf. If you are in danger, contact local emergency services.

## Why Sela

Most wellbeing apps decide what to remember about you. Sela is built to **respect what it must not assume**:

- **Your words stay yours.** Every AI reflection quotes what you actually wrote, is labelled as an interpretation, and can be confirmed, corrected or rejected.
- **You decide what is kept.** Each conversation is saved or not saved by your choice; storage, AI processing and sharing are separate consents.
- **Honest AI.** Every reply shows which model wrote it and whether it was checked. When AI is unavailable or not allowed, Sela says so and still works as a writing space.
- **Help is always one tap away.** "Need help now" and Quick exit are on every screen. A deterministic safety router — multilingual, typo-tolerant and jailbreak-aware — runs before any AI, and words that read as danger open quick-help mode at once.

## Features

| | |
| --- | --- |
| **Talk** | A calm conversation room with streamed replies in two modes — *talk it through* (options, honest views, feedback) or *just listen* (one small step) — save / don't save per conversation, and "What Sela noticed" reflection cards you can review |
| **My notes** | Free writing, mood check-ins with drawn mood faces, a timeline, and a weekly look-back without automatic conclusions |
| **What Sela remembers** | Memory you add, edit or confirm yourself, behind its own consent. Sela can suggest items from a saved conversation when you ask; nothing is remembered until you confirm or correct it |
| **Support** | Build a consultation summary from chosen notes, approve the exact version, export it (HTML / Markdown) or share it through a revocable link with an access code; record follow-ups afterwards |
| **Planned care** | An opt-in **condition map** that reads your saved check-ins, notes, reflections and conversations and shows one of four stages — everyday to needs help now — with its reasons (not a diagnosis), the help that fits, a public directory and simulated professionals for the product demo |
| **Sharing space** | Share stories anonymously with other adults and reply with support; every post and reply passes the safety router and Meta moderation before it appears, bullying is hidden at once, and reports go to an operator |
| **Help & reading** | Verified help services with source and verification dates, quick-help mode with one-tap call buttons, and curated reading from official publishers |
| **Your device, your rules** | Indonesian and English, light / dark / two high-contrast schemes, text up to 150 %, reduced motion, hidden previews and neutral tab titles for shared phones |
| **Operator console** | A separate, strongly authenticated staff area for source verification, catalogue quality, community moderation, AI budgets and audit — staff cannot read personal journals |

<p align="center">
  <img src="docs/assets/screenshots/talk-desktop.png" alt="Conversation room on a laptop: the person's words on the right, Sela's labelled replies on the left, and a writing box pinned to the bottom" width="640">
  &nbsp;
  <img src="docs/assets/screenshots/talk-phone.png" alt="The same conversation room on a phone" width="200">
</p>

## How it works

```mermaid
flowchart LR
    U["Browser<br/>React · TypeScript · Vite"] --> API["Go service<br/>one binary: API + web assets"]
    API --> S["Consent, safety and<br/>provenance gates"]
    S --> AI["Meta generator<br/>+ separate Meta verifier"]
    API --> PG[("PostgreSQL 16<br/>per-user envelope encryption")]
    API -.-> R[("Redis — optional<br/>public cache and rate limits")]
    O["Operator console"] --> API
```

- **Go owns every boundary**: identity, consent, the crisis check, provenance, storage and what is finally released to the screen. Models propose; people approve.
- **Privacy by construction**: personal records are encrypted per user and can be crypto-shredded; nothing personal is written to logs or to Redis.
- **No fallback that bypasses consent**: if the Meta model is unavailable, Sela shows an honest no-AI state rather than routing elsewhere.

More: [architecture](docs/architecture/ARCHITECTURE.md) · [AI policy](docs/architecture/AI_POLICY.md) · [safety](docs/architecture/SAFETY.md) · [Meaning Integrity Engine](docs/vision/MEANING_INTEGRITY_ENGINE.md)

## Getting started

**Requirements:** Go 1.26, Node.js 22, PostgreSQL 16 (Docker works too). Redis is optional.

```sh
git clone https://github.com/YanIanZ/sela.git && cd sela
(cd web && npm ci && npm run build)        # build the web app into web/dist
go build -o build/sela ./cmd/sela

# Point Sela at a local PostgreSQL 16 database and create a development key
export SELA_ENV=development SELA_SECURE_COOKIES=false SELA_DB_AUTO_MIGRATE=true
export SELA_DATABASE_URL='postgres://sela:sela-dev-only@127.0.0.1:5432/sela_dev?sslmode=disable'
eval "$(./build/sela keys generate-kek --env development | grep '^SELA_')"
export SELA_DATA_KEK SELA_DATA_KEK_ID

./build/sela                               # app and API on http://localhost:8080
```

For hot reload, keep the server running and start `cd web && npm run dev` (Vite on :5173 proxies `/api`).

**With Docker.** The Compose stack runs the app, PostgreSQL and Redis. It reads two local, git-ignored files — `.env.docker-testing` (app settings, start from [`.env.example`](.env.example)) and `.env.pg-testing` (PostgreSQL credentials) — then:

```sh
docker compose up --build
```

AI replies are off until a model endpoint is configured; Sela remains fully usable as a private writing space. See the [development guide](docs/operations/DEVELOPMENT.md) and the [runbook](docs/operations/RUNBOOK.md) for configuration, AI providers, migrations and backups.

### Checks

```sh
go vet ./... && go test -race ./...
(cd web && npm run typecheck && npm test && npm run build)
go run ./cmd/sela-eval -mode offline
```

## Project layout

```text
cmd/         sela (server and CLI), sela-eval (evaluation), fakellm (labelled test double)
internal/    Go packages: HTTP API, accounts and consent, encryption, safety, AI routing, reading, sharing
web/         React 19 + TypeScript frontend and its design system (web/DESIGN.md)
data/        Reading library content and the research corpus and register
modal/       Meta open-weight model serving on Modal
deploy/      Container image
e2e/ eval/ tests/   Browser checks, model evaluation, content and research tests
scripts/     Operations (backup and restore drills) and the research corpus pipeline
docs/        Product, architecture, API, design, operations and validation docs
SPEC.md      Requirements, invariants and task status — the source of truth
```

## Status

Sela runs in **production at [sela.vertalt.my.id](https://sela.vertalt.my.id)** and is in active development toward a controlled pilot. Features run end to end against PostgreSQL with live Meta inference and are covered by Go, frontend and browser tests. Sela makes no clinical-efficacy claim, and professionals shown in planned care are fictional demo data. Current plan and open work: [safer flow and TODO](docs/roadmap/SAFE_FLOW_AND_TODO.md) · [implementation status](docs/status/IMPLEMENTATION_STATUS.md) · [blockers](docs/status/BLOCKERS.md).

## Documentation

| Start here | |
| --- | --- |
| [Product vision](docs/vision/README.md) | Where Sela is going and why |
| [Product](docs/product/PRODUCT.md) · [Design system](web/DESIGN.md) | What Sela does and how it looks |
| [API](docs/api/API_SPEC_V3.md) | HTTP contracts |
| [Safer flow and TODO](docs/roadmap/SAFE_FLOW_AND_TODO.md) | The current plan, weaknesses found and what is next |
| [Docs index](docs/README.md) | Everything else, grouped by purpose |

## Contributing and security

Contributions are welcome — read [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md) first. Use synthetic data only; never put real personal stories in issues, tests or fixtures. Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## Credits and license

**© 2026 Sela — [YanIanZ](https://github.com/YanIanZ).** Released under the [MIT License](LICENSE).

**Forked from Dengar.in.** Sela is an individual continuation that replaces the original Dengar.in, started by a Telkom University Surabaya team. Code carried over from Dengar.in keeps the Dengar.in Team copyright notice, and its contributors' authorship is recorded in the [provenance record](docs/product/SELA_PROVENANCE.md). Third-party data and content keep their own terms.

<p align="center"><b>Built for people to be heard — not to be classified.</b></p>
