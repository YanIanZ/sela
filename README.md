# Sela

**A little space to feel heard.**

*Ruang sejenak untuk didengar.*

[![Sela v2](https://github.com/YanIanZ/sela/actions/workflows/v2.yml/badge.svg)](https://github.com/YanIanZ/sela/actions/workflows/v2.yml)
[![v1 quality gate](https://github.com/YanIanZ/sela/actions/workflows/quality.yml/badge.svg)](https://github.com/YanIanZ/sela/actions/workflows/quality.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Sela is becoming a bilingual (Indonesian/English) listening and continuity app for people aged 15 to 50 and over (owner brief v3, 2026-10-26). A person can talk without filling in a form, keep personal notes under their control, see their own history, correct what Sela remembers, and — only if they want to — prepare a "Ringkasan untuk konsultasi" (summary for a consultation) to share with someone they invite. Sela does not diagnose, treat, triage or replace a professional, and it never contacts anyone on the user's behalf.

**That is the plan, not the current code.** What runs today is the **v2 demo**: synthetic adults, fictional professionals and simulated care. It books nothing real, contacts no one, and gives no diagnosis, prescription, triage or therapy. Crisis language stops the ordinary journey before any AI runs and shows verified Indonesian help services.

## Status (26 October 2026)

[SPEC.md](SPEC.md) is the source of truth. Status words: planned → in progress → implemented → verified locally → verified live → ready for controlled pilot → public release, or blocked externally; [SPEC §G](SPEC.md#g-goal) defines them. Detailed status: [implementation status](docs/IMPLEMENTATION_STATUS.md) · [requirements traceability](docs/REQUIREMENTS_TRACEABILITY.md) · [backlog](docs/BACKLOG.md) · [blockers](docs/BLOCKERS.md).

### Sela v3 (branch `feat/sela-v3-pilot`, first stacked on PR #6, which is now merged into main)

| Area | Status |
| --- | --- |
| Spec realignment, decisions D-026 to D-052, blockers, backlog, [Dengar.in feature matrix](docs/DENGARIN_FEATURE_MATRIX.md), ADR-0002, ADR-0003, API v3 foundation contract (P1) | In progress: documents written on the branch, under review |
| Foundation: PostgreSQL storage, per-user encryption, accounts, guest mode, recovery codes, consents, export, deletion (`/api/v3`, P2) | In progress: code being written, uncommitted and not yet verified |
| Design direction, information architecture, screens, content style, accessibility criteria (P2) | In progress: documents written ([design](docs/design/DESIGN_DIRECTION.md)), uncommitted, PM review not recorded; no v3 UI code |
| Conversation room, journal, check-in, memory, timeline, home (P3) | Planned |
| Consultation summary, sharing with purpose, invited recipient, follow-up (P4) | Planned |
| AI routing: Meta on Modal primary, Claude Sonnet 5.5 verifier, DeepSeek fallback (P5) | Planned; live use **blocked externally** (B-1, B-8, B-9) |
| Ages 15–17 | In progress (age band and guardian consent record backend, uncommitted); the rest planned; production use **blocked externally** until a legal review (B-10) |
| Controlled pilot with real users | **Not ready**: needs live providers, the legal review (B-10), hosting and region (B-3, B-15), re-verified help lines (B-5) and owner testing |

### Sela v2 (PR #6, branch `feat/v2-go-meta-runtime`; the demo configuration)

| Area | Status |
| --- | --- |
| v2 runtime: Go 1.24 API + React/TypeScript/Vite frontend in one binary | Implemented, tested locally (Go race tests, Vitest, Playwright journeys) |
| Full journey in ID and EN, crisis routing, consents, retention/deletion, Story Ledger with Meaning Lock, Next Step, Opening Practice, handoff with token + access code, recipient view, follow-up, revoke | Implemented, tested locally |
| Safety lexicon | Ruleset `-4` committed (1329 test vectors); ruleset `-5` in progress |
| AI: Meta **Muse Glimmer 30B** (open weights, Apache-2.0) on **Modal** via vLLM | Adapter and Modal app implemented and tested offline; **not deployed** — no Modal credentials or GPU budget yet ([B-1](docs/BLOCKERS.md)). Local runs use AI off or a clearly labelled **test double**, never presented as Meta |
| Optional verifier: Claude Sonnet 5.5 **EvidenceGuard** (owner decision C12) | Standalone Go prototype in `prototypes/evidenceguard` (PR #5); not wired into the v2 runtime; would need its own consent (V16–V18) ([contract](docs/CLAUDE_VERIFIER.md)) |
| Optional generation fallback: DeepSeek-V4.1-Flash-1017 `deepseek-flash` (owner decision C13) | Standalone Go prototype in `prototypes/generationfallback`, mock-tested only; not wired into the v2 runtime, which has no DeepSeek route; OFF by default in hackathon mode; would need its own consent and eligibility guards (V19–V21) ([contract](docs/DEEPSEEK_FALLBACK.md)) |
| Research corpus (100 publications): reproducible pipeline, verified dates, licences, traceable extraction, reviewer search | Implemented, tested locally; human appraisal not done ([B-4](docs/BLOCKERS.md)) |
| Evaluation with pre-registered thresholds | Offline and test-double suites run; live-model metrics blocked ([evaluation](docs/EVALUATION.md)) |
| Public deployment | Not configured ([B-3](docs/BLOCKERS.md)) |
| Inherited v1 (Next.js) | Still runnable; retired only after acceptance (T9) |

## Run v2 locally

These commands run the v2 demo and stay valid until the v3 foundation lands; that change adds PostgreSQL for `/api/v3` and moves the Go toolchain ([ADR-0002](docs/adr/ADR-0002-persistence-accounts-and-encryption.md) §1), and this section will be updated with it.

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

[Documentation index](docs/README.md) · [product](docs/PRODUCT.md) · [safety](docs/SAFETY.md) · [backlog](docs/BACKLOG.md) · [design](docs/design/DESIGN_DIRECTION.md) · [architecture](docs/ARCHITECTURE.md) · [API v2](docs/API_SPEC.md) · [API v3](docs/API_SPEC_V3.md) · [AI policy](docs/AI_POLICY.md) · [Meta integration](docs/META_INTEGRATION.md) · [opt-in DeepSeek fallback](docs/DEEPSEEK_FALLBACK.md) · [Claude verifier](docs/CLAUDE_VERIFIER.md) · [ADR-0001: model choice](docs/adr/ADR-0001-meta-open-weight-model-on-modal.md) · [ADR-0002: persistence and accounts](docs/adr/ADR-0002-persistence-accounts-and-encryption.md) · [ADR-0003: AI routing](docs/adr/ADR-0003-ai-routing-verifier-fallback.md) · [research corpus pipeline](docs/research/CORPUS_PIPELINE.md) · [decision log](docs/DECISION_LOG.md).

## Contributing and security

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md). Use synthetic examples only; never post private stories, credentials or keys in public issues. Requirement changes start in `SPEC.md`.

## Provenance and license

Maintained by [I Wayan Bagus Arya Darma Wiguna](https://github.com/YanIanZ). Sela continues **Dengar.in**, built by the team **Kata Justin Nama Timnya Gokil Udah** (Telkom University Surabaya). Dengar.in began in [iqbalscript/Denger.in-Web-Project](https://github.com/iqbalscript/Denger.in-Web-Project), the origin repository by Iqbal Dovandra, who wrote the initial web app and the crisis engine; it continued in [zlxrnn/dengar.in-2.0](https://github.com/zlxrnn/dengar.in-2.0), and Sela v1 in this repository started from the same tree as that continuation. Git history credits Iqbal Dovandra, I Wayan Bagus Arya Darma Wiguna (who wrote the Care Passport and its opening-lines pipeline within Dengar.in), "Shine" (Bryan Christian Shine Mokolomban, a match inferred from the team's ownership table) and Immanuel Justin; Radit and Alvin held recorded roles without commits. Sela v2 ported and extended the crisis lexicon, the source-grounded Care Passport and the opening lines, and reused the verified help entries; v3 plans to migrate more Dengar.in behaviour with credit recorded per feature. Original authorship and [MIT notices](LICENSE) are retained; third-party material keeps its own terms (for example 22 WHO-adapted self-help cards under CC BY-NC-SA 3.0 IGO). See [provenance](docs/SELA_PROVENANCE.md) and the [feature matrix](docs/DENGARIN_FEATURE_MATRIX.md). Historical AMD work is archived and is not part of the runtime. Hackathon eligibility is not claimed, and Claude or DeepSeek work is never presented as Meta's ([event notes](docs/META_EVENT_NOTES.md)).
