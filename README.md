# Sela

**A little space to feel heard.**

*Ruang sejenak untuk didengar.*

[![Quality gate](https://github.com/YanIanZ/sela/actions/workflows/quality.yml/badge.svg)](https://github.com/YanIanZ/sela/actions/workflows/quality.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Sela is being redesigned as a bilingual companion for the journey toward professional mental-health support: **tell your story → understand your needs → explore professionals → prepare a consultation brief → share with consent → follow up**.

The Meta hackathon demonstration uses **synthetic people, fictional professionals and simulated care**. It does not book real appointments, deliver medical treatment, diagnose or prescribe. Anonymity is an optional privacy choice within the journey.

## Source of truth and status

**[SPEC.md](SPEC.md) is the source of truth.** Start there, then use the [documentation index](docs/README.md).

| Area | Status |
| --- | --- |
| Backend decision | **Go**, confirmed |
| Target frontend | Static React/TypeScript + Vite; engineering default |
| AI direction | Meta/Muse Spark primary; optional DeepSeek Flash 1017 fallback and Claude Sonnet 5.5 verifier prototypes (not product-integrated) |
| New professional-care demo | Designed, not implemented |
| Current runnable application | Inherited Next.js/TypeScript v1 |
| V2 deployment | Not configured |

The current CI badge tests v1, not the proposed Go implementation. Docs and task status distinguish target behavior from code that already exists.

## Target demonstration

- Indonesian and English user and fictional-professional perspectives.
- Seeded intake, source-grounded consultation notes and explicit AI consent.
- Deterministic discovery of fictional professionals and appointment slots.
- Version-bound review, approval and separate consent to share selected fields.
- Simulated handoff and consultation followed by a fixture-authored plan.
- Clear crisis interruption, unavailable-model, manual, revoked and expired states.

Read the [product](docs/PRODUCT.md), [architecture](docs/ARCHITECTURE.md), [planned API](docs/API_SPEC.md), [Meta integration](docs/META_INTEGRATION.md), [opt-in DeepSeek fallback](docs/DEEPSEEK_FALLBACK.md), [optional Claude verifier](docs/CLAUDE_VERIFIER.md) and [migration roadmap](docs/ROADMAP.md).

## Run the existing v1 application

These commands run the old runtime, not Sela v2. Use Node.js 22.18+ and npm 10+.

```sh
git clone https://github.com/YanIanZ/sela.git
cd sela
npm ci
cp .env.example .env.local
npm run dev
```

Open [localhost:3000](http://localhost:3000). Current environment templates and provider settings are v1-only. No Go command is available until the new runtime is implemented. See [development](docs/DEVELOPMENT.md) for current checks and [archive](docs/archive/v1/README.md) for inherited setup/operations.

## Contributing and security

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md). Use synthetic examples only; never post private stories, credentials or recovery keys in public issues. Requirement changes start in `SPEC.md`.

## Provenance and license

Maintained individually by [I Wayan Bagus Arya Darma Wiguna](https://github.com/YanIanZ). Sela builds on the Dengar.in project by **Kata Justin Nama Timnya Gokil Udah** (Telkom University Surabaya). Original authorship and [MIT notices](LICENSE) are retained. See [provenance](docs/SELA_PROVENANCE.md).

Historical AMD work is archived and does not describe the new Meta implementation. The [Meta event notes](docs/META_EVENT_NOTES.md) record official sources and unresolved eligibility/submission details.
