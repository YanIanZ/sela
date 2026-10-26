# Sela

**A little space to feel heard.**

*Ruang sejenak untuk didengar.*

[![Sela v2](https://github.com/YanIanZ/sela/actions/workflows/v2.yml/badge.svg)](https://github.com/YanIanZ/sela/actions/workflows/v2.yml)
[![v1 quality gate](https://github.com/YanIanZ/sela/actions/workflows/quality.yml/badge.svg)](https://github.com/YanIanZ/sela/actions/workflows/quality.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Sela is a bilingual (Indonesian/English) bridge toward professional mental-health support, not an AI therapist. A person tells their story, reviews a source-grounded note in which every item quotes their own words, chooses a practical next step, rehearses how to start the conversation, shares only what they approve with a (fictional) professional, and sees honestly what happened next.

The demo uses **synthetic adults, fictional professionals and simulated care**. It books nothing real, contacts no one, and gives no diagnosis, prescription, triage or therapy. Crisis language stops the ordinary journey before any AI runs and shows verified Indonesian help services.

## Status (26 October 2026)

[SPEC.md](SPEC.md) is the source of truth. Detailed status: [implementation status](docs/IMPLEMENTATION_STATUS.md) · [requirements traceability](docs/REQUIREMENTS_TRACEABILITY.md) · [blockers](docs/BLOCKERS.md).

| Area | Status |
| --- | --- |
| v2 runtime: Go 1.24 API + React/TypeScript/Vite frontend in one binary | Implemented, tested locally (Go race tests, Vitest, Playwright journeys) |
| Full journey in ID and EN, crisis routing, consents, retention/deletion, Story Ledger with Meaning Lock, Next Step, Opening Practice, handoff with token + access code, recipient view, follow-up, revoke | Implemented, tested locally |
| AI: Meta **Muse Glimmer 30B** (open weights, Apache-2.0) on **Modal** via vLLM | Adapter and Modal app implemented and tested offline; **not deployed** — no Modal credentials or GPU budget yet ([B-1](docs/BLOCKERS.md)). Local runs use AI off or a clearly labelled **test double**, never presented as Meta |
| Research corpus (100 publications): reproducible pipeline, verified dates, licences, traceable extraction, reviewer search | Implemented, tested locally; human appraisal not done ([B-4](docs/BLOCKERS.md)) |
| Evaluation with pre-registered thresholds | Offline and test-double suites run; live-model metrics blocked ([evaluation](docs/EVALUATION.md)) |
| Public deployment | Not configured ([B-3](docs/BLOCKERS.md)) |
| Inherited v1 (Next.js) | Still runnable; retired only after v2 acceptance (T9) |

## Run v2 locally

Go 1.24 and Node.js 22.

```sh
git clone https://github.com/YanIanZ/sela.git && cd sela
(cd web && npm ci && npm run build)
SELA_ENV=development SELA_SECURE_COOKIES=false go run ./cmd/sela   # AI off; open http://localhost:8080
```

With the labelled test double (development only; the UI shows "TEST DOUBLE — not Meta"):

```sh
go run ./cmd/fakellm -addr 127.0.0.1:8090 &
SELA_ENV=development SELA_SECURE_COOKIES=false SELA_LLM_MODE=test_double SELA_ALLOW_TEST_DOUBLE=true \
  SELA_LLM_BASE_URL=http://127.0.0.1:8090 go run ./cmd/sela
```

Checks: `go vet ./... && go test -race ./...`, `cd web && npm run typecheck && npm test`, `node e2e/journey.e2e.mjs` (after building `build/sela` and `build/fakellm`), `go run ./cmd/sela-eval -mode offline`. Configuration, the Modal deployment, cost controls, rollback and a three-minute demo script are in the [runbook](docs/RUNBOOK.md) and [modal/README.md](modal/README.md).

## Documentation

[Documentation index](docs/README.md) · [product](docs/PRODUCT.md) · [architecture](docs/ARCHITECTURE.md) · [API](docs/API_SPEC.md) · [safety](docs/SAFETY.md) · [AI policy](docs/AI_POLICY.md) · [Meta integration](docs/META_INTEGRATION.md) · [ADR-0001: model choice](docs/adr/ADR-0001-meta-open-weight-model-on-modal.md) · [research corpus pipeline](docs/research/CORPUS_PIPELINE.md) · [decision log](docs/DECISION_LOG.md).

## Contributing and security

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md). Use synthetic examples only; never post private stories, credentials or keys in public issues. Requirement changes start in `SPEC.md`.

## Provenance and license

Maintained by [I Wayan Bagus Arya Darma Wiguna](https://github.com/YanIanZ). Sela builds on the Dengar.in project by **Kata Justin Nama Timnya Gokil Udah** (Telkom University Surabaya): its bilingual support, source-grounded Care Passport, opening-lines pipeline and crisis lexicon were ported into v2 and extended. Original authorship and [MIT notices](LICENSE) are retained. See [provenance](docs/SELA_PROVENANCE.md). Historical AMD work is archived and is not part of the v2 runtime. Hackathon eligibility of the Meta setup is not claimed ([event notes](docs/META_EVENT_NOTES.md)).
