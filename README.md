# Sela

**A little space to feel heard.**

*Ruang sejenak untuk didengar.*

[![Sela v2](https://github.com/YanIanZ/sela/actions/workflows/v2.yml/badge.svg)](https://github.com/YanIanZ/sela/actions/workflows/v2.yml)
[![v1 quality gate](https://github.com/YanIanZ/sela/actions/workflows/quality.yml/badge.svg)](https://github.com/YanIanZ/sela/actions/workflows/quality.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Sela is becoming a bilingual (Indonesian/English) listening and continuity app for people aged 15 to 50 and over (owner brief v3, 2026-10-26). A person can talk without filling in a form, keep personal notes under their control, see their own history, correct what Sela remembers, and — only if they want to — prepare a "Ringkasan untuk konsultasi" (summary for a consultation) to share with someone they invite. Sela does not diagnose, treat, triage or replace a professional, and it never contacts anyone on the user's behalf.

**That is the plan, and the v3 frontend now exists for the parts whose services exist.** The v3 backend foundation (accounts, guest mode, encrypted storage, export, deletion) is built and verified locally, and the v3 frontend on `feat/v3-experience` implements the 28 screen groups to the extent their `/api/v3` routes exist. Memory, consultation summary/sharing/recipient/follow-up and reading are honest blocked states, not working mocks. Nothing is verified live, and the product is not pilot-ready. The **v2 demo** remains at `/demo` (fictional professionals and simulated care); it books nothing real, contacts no one, and gives no diagnosis, prescription, triage or therapy. Crisis language still stops the ordinary journey before any AI runs and shows verified Indonesian help services.

## Current local development increment (28 October 2026)

The `feat/v3-experience` branch (stacked on the local `feat/v3-core-journey` work) delivers the full v3 frontend for the existing `/api/v3` routes: the design system and app shell, Introduction/About, onboarding, sign-in/recovery/guest upgrade, Home, Talk and the conversation room, notes, check-ins, timeline and patterns, Support, Immediate help, Privacy & data, Display & accessibility, Account and Policies. Personal records use PostgreSQL and per-user encryption. Memory, summaries, preparation, sharing, the v3 recipient page and reading are truthful unavailable states with their blockers named; the v2 synthetic recipient stays at `/r`.

Evidence: [frontend validation](docs/V3_FRONTEND_VALIDATION.md) and the [owner acceptance guide](OWNER_TESTER.md). Still open: the missing v3 feature APIs, reading editorial review, manual screen-reader/usability review, and all release gates. No live, clinical or pilot claim is made.

For the configured local Docker environment, run `bash scripts/run-local-v3.sh` and open http://localhost:8080. It reads only the ignored local `.env.docker-testing` and `.env.pg-testing` files; it never prints their contents. The default v3 UI needs the database; a no-database v2-only run can still use `/demo`.

## Status (28 October 2026)

[SPEC.md](SPEC.md) is the source of truth. Status words: planned → in progress → implemented → verified locally → verified live → ready for controlled pilot → public release, or blocked externally; [SPEC §G](SPEC.md#g-goal) defines them. Detailed status: [implementation status](docs/IMPLEMENTATION_STATUS.md) · [requirements traceability](docs/REQUIREMENTS_TRACEABILITY.md) · [backlog](docs/BACKLOG.md) · [blockers](docs/BLOCKERS.md).

### Sela v3 (local delivery branch `feat/v3-experience`)

The branch stacks the local `feat/v3-core-journey` work on merged main `e62506a` (PRs #9–#13; safety ruleset `-5` is a WIP snapshot in draft PR #8 to main, excluded). Status words are those of [SPEC §G](SPEC.md#g-goal).

| Area | Status |
| --- | --- |
| Spec realignment, decisions D-026 to D-056, blockers, backlog, [Dengar.in feature matrix](docs/DENGARIN_FEATURE_MATRIX.md), ADR-0002, ADR-0003, API v3 foundation contract (P1) | In progress: documents written and committed, under review |
| Foundation: PostgreSQL storage, per-user encryption, accounts, guest mode, recovery codes, consents, audit, export, deletion, backup and restore (`/api/v3`, P2) | **Verified locally, CI passed** (commit `0ff3341`, in draft PR #9 against main; [foundation status note](SPEC.md#foundation-status-note-2026-10-26)): every local suite passes on the commit — formatting, vet and the race tests against PostgreSQL 16, the migration round trip, the restore drill, and the web, e2e, Python and `sela-eval` suites — and `.github/workflows/v2.yml` run 40405475962 passed all five jobs on PR #9 (it has no `sela-eval` step; [implementation status](docs/IMPLEMENTATION_STATUS.md#1-where-v3-stands)); the restore announcement in the app and on a status page (backlog S-262) is planned; the hosted database, secret manager and backups are **blocked externally** (B-3, B-15) |
| Design direction, information architecture, screens, content style, accessibility criteria (P2) | Documents committed; **implemented and locally checked** for the screens whose APIs exist ([frontend validation](docs/V3_FRONTEND_VALIDATION.md)); PM/browser review and manual accessibility audit remain open |
| Frontend shell and screens (P2–P4) | **Implemented and verified locally** on `feat/v3-experience`: 28 screen groups accounted for; real APIs for accounts, records, listening, privacy, export/deletion and help. Automated and Chromium evidence: typecheck, 58 tests, 18 axe scans, 320px/desktop, 44/44 browser checks. Not verified live; manual screen-reader/usability review not done |
| Conversation room, journal, check-in, timeline and home (P3) | **Implemented and verified locally** for save/don't-save, listening, note versions, check-ins with energy/tags, timeline and patterns. Corrections, reflection cards, memory proposals, hold-timeout warning and derived-item deletion remain open |
| Memory, consultation summary, sharing, recipient and reading (P3–P6) | **Blocked surfaces**: honest unavailable states, no fake data; the APIs/editorial review are missing |
| Consultation summary, sharing with purpose, invited recipient, follow-up (P4) | Planned |
| AI routing: Meta on Modal primary, Claude Sonnet 5.5 verifier, DeepSeek fallback (P5) | In progress: committed on branch `feat/sela-v3-ai` and open as draft PR #10 against `feat/sela-v3-pilot`, not merged; nothing verified; live use **blocked externally** (B-1, B-8, B-9) |
| Research and reading content (P6) | In progress: committed on branch `feat/sela-v3-research` and open as draft PR #11 against main, not merged; human appraisal **blocked externally** (B-4) |
| Operations and compliance documents (P7) | In progress on a separate branch, not merged; deployment **blocked externally** (B-3, B-15) |
| Ages 15–17 | Onboarding, confidentiality, guest-only start and guardian consent are implemented and locally checked; 15 and under-15 are blocked. The server still has one `15_17` band, and production use stays **blocked externally** until legal review (B-10) |
| Controlled pilot with real users | **Not ready**: needs live providers, the legal review (B-10), hosting and region (B-3, B-15), re-verified help lines (B-5), owner testing and the restore announcement that the privacy text promises (S-262 criterion 3, S-263) |

### Sela v2 (PR #6, merged into main as `7fc6678`; the demo configuration)

| Area | Status |
| --- | --- |
| v2 runtime: Go API + React/TypeScript/Vite frontend in one binary (built with Go 1.24 until the v3 foundation moved the module to Go 1.26, D-054) | Implemented, tested locally (Go race tests, Vitest, Playwright journeys) |
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

These commands run the v2 demo. The `/api/v3` foundation also needs PostgreSQL 16; without `SELA_DATABASE_URL`, its routes answer `503 store_unavailable` in development ([runbook](docs/RUNBOOK.md) §1 and §7).

Go 1.26.10 (the `toolchain` line of `go.mod`; with `GOTOOLCHAIN=auto` the go command fetches it; [D-054](docs/DECISION_LOG.md#d-054)) and Node.js 22.

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

Checks: `go vet ./... && go test -race ./...` (the PostgreSQL tests run when `SELA_TEST_DATABASE_URL` is set; see the [runbook](docs/RUNBOOK.md) §2), `cd web && npm run typecheck && npm test`, `node e2e/journey.e2e.mjs` (after building `build/sela` and `build/fakellm`), `go run ./cmd/sela-eval -mode offline`. Configuration, the Modal deployment, cost controls, rollback and a three-minute demo script are in the [runbook](docs/RUNBOOK.md) and [modal/README.md](modal/README.md).

## Documentation

[Documentation index](docs/README.md) · [product](docs/PRODUCT.md) · [safety](docs/SAFETY.md) · [backlog](docs/BACKLOG.md) · [design](docs/design/DESIGN_DIRECTION.md) · [architecture](docs/ARCHITECTURE.md) · [API v2](docs/API_SPEC.md) · [API v3](docs/API_SPEC_V3.md) · [AI policy](docs/AI_POLICY.md) · [Meta integration](docs/META_INTEGRATION.md) · [opt-in DeepSeek fallback](docs/DEEPSEEK_FALLBACK.md) · [Claude verifier](docs/CLAUDE_VERIFIER.md) · [ADR-0001: model choice](docs/adr/ADR-0001-meta-open-weight-model-on-modal.md) · [ADR-0002: persistence and accounts](docs/adr/ADR-0002-persistence-accounts-and-encryption.md) · [ADR-0003: AI routing](docs/adr/ADR-0003-ai-routing-verifier-fallback.md) · [research corpus pipeline](docs/research/CORPUS_PIPELINE.md) · [decision log](docs/DECISION_LOG.md).

## Contributing and security

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md). Use synthetic examples only; never post private stories, credentials or keys in public issues. Requirement changes start in `SPEC.md`.

## Provenance and license

Maintained by [I Wayan Bagus Arya Darma Wiguna](https://github.com/YanIanZ). Sela continues **Dengar.in**, built by the team **Kata Justin Nama Timnya Gokil Udah** (Telkom University Surabaya). Dengar.in began in [iqbalscript/Denger.in-Web-Project](https://github.com/iqbalscript/Denger.in-Web-Project), the origin repository by Iqbal Dovandra, who wrote the initial web app and the crisis engine; it continued in [zlxrnn/dengar.in-2.0](https://github.com/zlxrnn/dengar.in-2.0), and Sela v1 in this repository started from the same tree as that continuation. Git history credits Iqbal Dovandra, I Wayan Bagus Arya Darma Wiguna (who wrote the Care Passport and its opening-lines pipeline within Dengar.in), "Shine" (Bryan Christian Shine Mokolomban, a match inferred from the team's ownership table) and Immanuel Justin; Radit and Alvin held recorded roles without commits. Sela v2 ported and extended the crisis lexicon, the source-grounded Care Passport and the opening lines, and reused the verified help entries; v3 plans to migrate more Dengar.in behaviour with credit recorded per feature. Original authorship and [MIT notices](LICENSE) are retained; third-party material keeps its own terms (for example 22 WHO-adapted self-help cards under CC BY-NC-SA 3.0 IGO). See [provenance](docs/SELA_PROVENANCE.md) and the [feature matrix](docs/DENGARIN_FEATURE_MATRIX.md). Historical AMD work is archived and is not part of the runtime. Hackathon eligibility is not claimed, and Claude or DeepSeek work is never presented as Meta's ([event notes](docs/META_EVENT_NOTES.md)).
