# Sela identity and provenance

Current requirements: [SPEC.md](../SPEC.md). Maintainer: **I Wayan Bagus Arya Darma Wiguna** ([YanIanZ](https://github.com/YanIanZ)).

## Current direction

Sela v2 is a Go-backed, Indonesian/English care-journey demonstration aligned with the Meta hackathon. Professional profiles, handoffs, consultation and follow-up are synthetic. Optional anonymity supports privacy; it is not the primary positioning. The v2 runtime is implemented and tested locally; the live Meta model on Modal is not deployed yet ([B-1](BLOCKERS.md)).

Tagline: **A little space to feel heard.** / **Ruang sejenak untuk didengar.** Brand/domain availability has not been established.

## Inherited work

Original project: [Dengar.in](https://github.com/zlxrnn/dengar.in-2.0), built by **Kata Justin Nama Timnya Gokil Udah** (Telkom University Surabaya). Its bilingual support, CareBridge, safety tests, forum, storage and AMD work predate this v2 refactor. Preserve original MIT notices and authorship; no rewriting another contributor's work as sole authorship.

The maintainer's own historical Git name/email aliases were normalized, with other authors preserved. A pre-normalization Git bundle was retained locally. The recoverable repository baseline immediately before the documentation refactor is `d5da323`.

All pre-v2 documents and supporting docs assets are indexed in the [archive](archive/v1/README.md) and [document inventory](DOCUMENT_INVENTORY.md). They retain original claims and dates as historical content. Removing AMD from the v2 runtime does not erase those records or original contributor credit.

Internal `@dengarin/*` names, keys, cookies, schemas, scripts and old deployments still belong to the running v1 code. New naming and removal happen during the verified v2 cutover (T9). Old deployment URLs are not a Sela v2 deployment.

## New-work boundary

| Capability in v2 | Origin |
| --- | --- |
| Crisis lexicon (rule ids preserved) | **Ported** from Dengar.in `services/crisis-engine`; extended with clause-level denial, quotation and third-party framing, slang, code-switching and idioms; v1 regression tests carried over as vectors |
| Source-grounded Care Passport and clarification | **Ported** from Dengar.in `services/validator/src/carePassport.ts`; extended as the Story Ledger with Meaning Lock (negation, subject, time and certainty checks), versioned approval and user review of every item |
| Opening-lines pipeline | **Ported** from Dengar.in `services/validator/src/openingLines.ts`; extended with pressure-language, number and identifier checks and a rehearsal view |
| Bilingual ID/EN support | **Inherited** concept; v2 dictionaries are new |
| Help resources | **Inherited** entries marked `verified_official` in v1, unchanged |
| Go API, in-memory session store, consents and retention, Next Step, handoff with token + access code, fictional recipient view, receipts, follow-up, revocation | **New** in v2 |
| React/TypeScript frontend | **New** in v2 |
| Meta Muse Glimmer on Modal (adapter, serving app, smoke test) and the labelled test double | **New** in v2 (not yet deployed) |
| Research corpus pipeline and reviewer search | **New** in v2, built on the Sela Relay register |
| Evaluation harness with pre-registered thresholds | **New** in v2 |

Do not count inherited features or documentation-only changes as a newly completed Meta integration. See [event notes](META_EVENT_NOTES.md) for unresolved reuse and eligibility questions.
