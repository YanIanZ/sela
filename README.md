# Sela

**A little space to feel heard.**

*Ruang sejenak untuk didengar.*

[![Quality gate](https://github.com/YanIanZ/sela/actions/workflows/quality.yml/badge.svg)](https://github.com/YanIanZ/sela/actions/workflows/quality.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Sela is an anonymous wellbeing companion in **Indonesian and English**. It offers a space to talk, reflect on everyday feelings, and prepare notes for a first mental health consultation.

An individual continuation by [I Wayan Bagus Arya Darma Wiguna](https://github.com/YanIanZ), built on the Dengar.in team project. Original contributor credits and the MIT notice are retained; see [provenance](docs/SELA_PROVENANCE.md).

## What you can do

- **Talk or write:** text chat and a voice room with explicit microphone consent.
- **Reflect privately:** daily check-ins, a local journal, micro-missions, and weekly reflection.
- **Prepare for a consultation:** CareBridge helps turn your own words into a Care Passport you review before exporting. AI processing has separate consent and a redaction preview.
- **Read and share:** an anonymous story forum with moderation.
- **Choose your language:** Indonesian and English interface, chat, and voice support through the language selector.
- **Find human support:** an Indonesia-focused help directory and deterministic crisis detection before model calls.

Sela is a self-help prototype, not a diagnostic, treatment, or emergency service. English support does not imply worldwide emergency-contact coverage.

## Project status

The bilingual experience and CareBridge are inherited functionality. Sela branding is implemented. **A Meta model integration and a Sela production deployment have not been configured.**

The target event is the Meta Global AI Developer Hackathon, with the maintainer intending to participate solo in the Indie Developer track. [Event notes](docs/META_EVENT_NOTES.md) separate checked sources from outstanding requirements. Historical AMD plans and reports remain available through the [documentation index](docs/README.md).

## Run locally

Use **Node.js 22.18+** and **npm 10+**; CI uses Node.js 22. Commands run from the repository root.

```sh
git clone https://github.com/YanIanZ/sela.git
cd sela
npm ci
cp .env.example .env.local
npm run dev
```

Open [localhost:3000](http://localhost:3000). Use the language selector to switch between Indonesian and English.

The checked-in [environment template](.env.example) contains empty credentials and setup notes:

| Configuration | Purpose | When omitted |
| --- | --- | --- |
| `DEEPSEEK_API_KEY`, `GEMINI_API_KEY`, `OPENROUTER_API_KEY` | Existing chat providers | Deterministic fallback; live model replies require a configured provider |
| `AMD_BASE_URL`, `AMD_MODEL`, optional `AMD_API_KEY` | Existing CareBridge inference adapter | AI unavailable state with a manual writing path |
| `DATABASE_URL` | PostgreSQL persistence | In-memory server storage for local development |
| `REDIS_URL` | Shared cache and rate limits | Local fallbacks in development; production requirements apply |
| `ADMIN_SESSION_SECRET` | Moderator authentication | Moderator login unavailable |

In-memory server data is temporary. Never put secrets in `NEXT_PUBLIC_*` variables or commit local environment files. For persistent storage and deployment, read the [database migrations](services/persistence/migrations/README.md), [Redis guide](docs/REDIS.md), and [deployment guide](deploy/vps/README.md). Deployment examples retain legacy identifiers and need review before use for Sela.

## Development checks

```sh
npm run lint
npm run lint:services
npm run typecheck
npm test
npm run build
```

The full test suite needs `redis-server` locally or Docker for its isolated Redis runtime. Provider tests use mocks; passing them does not establish live model quality or availability. See [contributing](CONTRIBUTING.md) for focused checks and [test bench](docs/TESTBENCH.md) for optional live evaluations.

## Repository map

| Directory | Contents |
| --- | --- |
| `apps/web/` | Next.js application, API routes, bilingual copy, browser storage and speech |
| `packages/` | Shared types, configuration, prompts |
| `services/` | Crisis engine, model orchestration, validation, authentication and persistence |
| `tests/` | Unit, integration, security and accessibility checks |
| `scripts/` | Evaluation, dataset, provider-probe and test-bench tools |
| `docs/` | Technical guides, provenance, event notes and historical records |
| `deploy/`, `docker/`, `supabase/` | Infrastructure examples and database tooling |

Internal `@dengarin/*` package names, storage keys and database migrations remain compatible with the original project.

## Documentation and participation

- [Documentation index](docs/README.md)
- [Architecture](docs/CAREBRIDGE_ARCHITECTURE.md) · [API](docs/API_SPEC.md)
- [Safety](docs/SAFETY.md) · [AI policy](docs/AI_POLICY.md) · [Threat model](docs/THREAT_MODEL.md)
- [Contribution guide](CONTRIBUTING.md) · [Security reporting](SECURITY.md)
- [Project provenance](docs/SELA_PROVENANCE.md) · [Meta event notes](docs/META_EVENT_NOTES.md)

## License and credits

[MIT](LICENSE). Sela is maintained by **I Wayan Bagus Arya Darma Wiguna**. The original Dengar.in project was built by **Kata Justin Nama Timnya Gokil Udah** (Telkom University Surabaya); original authorship remains in Git history and the license notice.
