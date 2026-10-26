# Sela identity and provenance

Current requirements: [SPEC.md](../SPEC.md). Maintainer: **I Wayan Bagus Arya Darma Wiguna** ([YanIanZ](https://github.com/YanIanZ)). Updated: 2026-10-26.

Statement labels are the [SPEC.md](../SPEC.md) header labels, which follow [FORMAT.md](../FORMAT.md), plus **inferred**: **observed** (read from Git history or files, read-only, on 2026-10-26, with the source named: SPEC's *externally sourced* label, under the name [ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md) uses), **inferred** (likely but unconfirmed), **user-confirmed** and **PM decision** ([D-026 to D-050](DECISION_LOG.md#d-026) of 2026-10-26). "Verified" is not a label here: it appears only in the [SPEC §G](../SPEC.md#g-goal) status words (verified locally, verified live) and in quoted data values such as `verified_official`. Feature-level reuse decisions are in the [Dengar.in feature matrix](DENGARIN_FEATURE_MATRIX.md).

## Current direction

**Sela v3** (user-confirmed, owner brief v3 of 2026-10-26; D-026): a real-use health and wellbeing app in Indonesian and English for ages 15 to 50+. Listening is the core; synthetic data is for tests, evaluation and demo only; the safety controls stay. Status: **planned** on branch `feat/sela-v3-pilot`; the backend foundation (persistence, accounts, guest mode, encryption, export, deletion) is **in progress** there, uncommitted and unverified ([SPEC](../SPEC.md) T14). No v3 capability is implemented or verified yet.

**Sela v2** (`feat/v2-go-meta-runtime`, PR #6) is a Go-backed, Indonesian/English care-journey demonstration aligned with the Meta hackathon. Its professional profiles, handoffs, consultation and follow-up are synthetic. The v2 runtime is implemented and tested locally; the live Meta model on Modal is not deployed yet ([B-1](BLOCKERS.md)). In v2, optional anonymity supports privacy and is not the primary positioning; in v3, guest mode states where and for how long data is stored and never promises absolute anonymity (D-029).

Tagline: **A little space to feel heard.** / **Ruang sejenak untuk didengar.** Brand/domain availability has not been established.

## Lineage

Sela continues **Dengar.in**, built by the team **Kata Justin Nama Timnya Gokil Udah** (Telkom University Surabaya). Observed sources: the "Team" section of `README.md` in both upstream repositories ("Built by Kata Justin Nama Timnya Gokil Udah (Telkom University Surabaya)"), the footer credit in `apps/web/src/i18n/common.ts`, `LICENSE` ("Copyright (c) 2026 Dengar.in Team") and `package.json` (`author: "Dengar.in Team"`). The individual contributors are listed below.

| Stage | Repository and commit | Relation (observed) |
| --- | --- | --- |
| 1. Origin | [`iqbalscript/Denger.in-Web-Project`](https://github.com/iqbalscript/Denger.in-Web-Project) @ `76e760e` (2026-10-14) | The **origin repository** of Dengar.in, owned by Iqbal Dovandra. 86 commits; root commit `385600d` "Initial commit" (2026-09-17, Iqbal Dovandra); tag `production-baseline-2026-09-21` → `1985be9` |
| 2. Hackathon continuation | [`zlxrnn/dengar.in-2.0`](https://github.com/zlxrnn/dengar.in-2.0) @ `dbcc0ba` (merge of PR #18, 2026-10-16) | Same root commit. `76e760e` is an ancestor of `dbcc0ba`; stage 2 adds 76 commits (2026-10-14 to 16, PRs #1 to #18) and deletes no file. 162 commits in total |
| 3. Sela v1 (in this repository) | `apps/`, `services/`, `packages/`, `supabase/`, `proto/`, `docs/archive/v1/` | Sela commit `1a4f91f` has the same tree as `dbcc0ba` (`bfce7c8…`). After it, `b2b9227` (rebrand as Sela) and `c51807c` (Sela v2 source of truth; docs moved to the archive) changed only brand strings, doc pointers, metadata, `CODEOWNERS`, `LICENSE`, `package.json` and `README.md`. No v1 feature was removed or changed in behaviour |
| 4. Sela v2 | `internal/`, `web/`, `cmd/`, `modal/`, `e2e/`, `eval/`, `scripts/research/` | New work, see the boundary below |
| 5. Sela v3 | `feat/sela-v3-pilot`, first stacked on PR #6 (D-034), which is now merged into main (`7fc6678`) | Planned; backend foundation in progress (uncommitted) |

The maintainer's own historical Git name/email aliases were normalized, with other authors preserved. A pre-normalization Git bundle was retained locally. The recoverable repository baseline immediately before the documentation refactor is `d5da323`. Because of that normalization, only the first six stage 1 commits (all by Iqbal Dovandra, `385600d` to `7241474`, 2026-09-17/18) keep their hashes in Sela's history; the other 80 stage 1 commits and all stage 2 commits carry new hashes here (for example `76e760e` is `02f04e1`), so content identity was checked by tree hash, not commit hash. Commit hashes cited in this document are upstream (stage 1 or 2) hashes. No further rewriting of history is allowed (owner brief v3 §12).

**How this was observed (2026-10-26, read-only):** full-history clones of both upstream repositories and a blobless clone of `YanIanZ/sela` (234 commits on all branches); `git shortlog`, `git log --follow --diff-filter=A` per file for who first created each file, `git merge-base --is-ancestor 76e760e dbcc0ba`, and `git rev-parse` of both trees. Ownership of record comes from `docs/archive/v1/COLLABORATION_RULES.md` §2 and the stage 2 `.github/CODEOWNERS`. Commit counts below are per repository.

## Contributors

"Built" means Git authorship. "Owner of record" means the review areas assigned on 2026-10-15/16 in `COLLABORATION_RULES.md` and `CODEOWNERS`; it is **not** authorship. For example, the core wellbeing pages assigned to YanIanZ and the crisis engine assigned to Justin were first written by Iqbal Dovandra. Credit both.

| Person | Identity (how known) | Commits (stage 1 / 2 / Sela) | Built (observed in Git) | Owner of record |
| --- | --- | --- | --- | --- |
| **Iqbal Dovandra** ([@iqbalscript](https://github.com/iqbalscript)) | Git author; GitHub owner of the origin repository; `CODEOWNERS` default owner `*` | 37 / 38 / 38 | The initial web app (`d27e4bf`, 2026-09-17): landing, consent, onboarding, assessment, dashboard, chat page, journal, check-in, mission, report, forum, recovery, crisis and resources pages, navbar and footer, shared config and device storage, the crisis engine, the action validator and `PRD.md`. Then: Quick Exit (`7241474`); the Gemini tier, the "Second Brain" debiaser and the domain gate in the orchestrator (`9ec0c5c`); E2EE backup and recovery, the content moderator and secure sync (`b6e4c06`, `a3af8f8`); gamification and the Perjalanan page (`60da941`); browser speech and the Ruang Ngobrol voice room with Gemini TTS (`e7bbd1e`, `4473b28`, `cf581e6`); forum threads and replies (`e65f9de`, `51aee63`); landing components and the privacy popup (`9e02838`); the app icon (`f92e174`); `PRD-DESIGN.md` (`63d8a83`); production hardening (`1985be9`); and the CI quality gate, added in his merge of PR #11 (stage 2, 2026-10-16) | Release Owner (SCRUM-12): build, Docker, Redis, health, CI, scripts |
| **I Wayan Bagus Arya Darma Wiguna** ([@YanIanZ](https://github.com/YanIanZ)) | Git author under the names "YanIanZ" and the full name; maintainer of Sela; `LICENSE` Sela-modifications line; his commits use a Telkom University student address | 49 / 112 / 184 (Sela count includes all v2 work) | Stage 1: the backend skeleton (`d0ef84d`: chat, forum, sync and weekly-report API routes, the orchestrator, prompts, DeepSeek and OpenRouter providers, persistence interfaces); PostgreSQL persistence and admin auth (`fe19946`); Redis and rate limits; the 14-day wellbeing timeline (`c4c9206`); chat progress; the copy rewrite and the "Bahasa Lazim" style guide; forum fixes; and every commit from `ff707c7` (2026-10-14): Care Passport/CareBridge (pipeline, grounding, PII redaction, opening lines, help-destination checklists, AMD adapter), the accessible redesign and the "Akses cepat" panel, ID/EN, the voice button, crisis fixes. Stage 2 (63 of its 76 commits): the self-help card library (`4ca1359`), the CareBridge dataset (`7312751`), the test bench (`39a5290`), VPS deployment (`193901b`), the bundled font with its licence (`ee8f26e`), the release gate (`ed0edb5`) and fixes such as export escaping and approval reset on edit; he merged PRs #1 to #18. Sela: the rebrand, v2 and the documentation | SCRUM-5 CareBridge, SCRUM-9 Core Wellbeing pages, SCRUM-24 release gate, test bench, dataset |
| **Bryan Christian Shine Mokolomban** | Git author name "Shine". The full name is in the `COLLABORATION_RULES.md` ownership table; matching it to "Shine" is **inferred**. Shine's own commit `9b52bc5` maps the Shine area to `@zlxrnn` in `CODEOWNERS`, so `@zlxrnn` (owner of stage 2) is probably Shine's account (**inferred**) | 0 / 10 / 10 | Stage 2, 2026-10-14 to 16: the CareBridge evaluation runner, metrics and AMD probe script (`15e3832`); the review-demo and support-options pages with ID/EN text (`21c6e4c`); the accessibility audit report, its test and UI fixes (`909d6e8`); privacy-warning text and popup fixes (`b7d4b0e`, `11f4795`); mobile navigation (`d5c6714`); the threat model, security bug reports and abuse tests (`99935ea`); `CODEOWNERS` (`9b52bc5`) | SCRUM-6 AMD inference: providers, orchestrator, debiaser, fallback, guardrails |
| **Immanuel Justin** (`@immanueljustinn`) | Git author `immanueljustinn`; "Justin" in `COLLABORATION_RULES.md`; "Immanuel" in `CODEOWNERS`; the team name contains "Justin" | 0 / 2 / 2 | Stage 2, 2026-10-15: PII detection and redaction tests (`4720216`); the CareBridge crisis-handling policy, a crisis rule for "bunuh diri semuanya" and the Yayasan Pulih contact re-verification (`281d948`) | SCRUM-7 Safety, SCRUM-8 Privacy, SCRUM-10 Community; approver for safety changes |
| **Radit** | `COLLABORATION_RULES.md` only. `CODEOWNERS` lists "Syafa" (`@Syafa-rdt`) for the same area; that they are one person is **inferred** | 0 / 0 / 0 | No commits | SCRUM-11 voice, accessibility, i18n |
| **Alvin** | `COLLABORATION_RULES.md` and a `CODEOWNERS` placeholder; GitHub handle unknown | 0 / 0 / 0 | No commits | SCRUM-27/28/29 documentation |

People with no commits are credited for their recorded roles; the history cannot show their work. `docs/archive/v1/AMD_SETUP.md` mentions a team research kit, so team work also happened outside Git.

## Licence and notices

- `LICENSE` (MIT) keeps "Copyright (c) 2026 Dengar.in Team" and adds "Copyright (c) 2026 I Wayan Bagus Arya Darma Wiguna (Sela modifications)". MIT requires the copyright and permission notice in all copies or substantial portions: keep `LICENSE` with every distribution, and when v3 ports Dengar.in code or data (crisis rules, validator rules, help entries, checklists, self-help cards, copy), record the origin in this document or in the file.
- Preserve original MIT notices and authorship; no rewriting another contributor's work as sole authorship. Keep Git authors; do not squash, rebase or re-author others' commits.
- Third-party material inside Dengar.in assets keeps its own terms, which MIT does not replace: 22 WHO self-help cards under CC BY-NC-SA 3.0 IGO (attribution and adaptation notice; non-commercial; D-045, B-14); the Schibsted Grotesk font under SIL OFL 1.1; `lucide-react` under ISC; Kemenkes and CDC material as recorded per card. Details: [feature matrix §3](DENGARIN_FEATURE_MATRIX.md#3-content-assets).

## Archive, internal names and old deployments

All pre-v2 documents and supporting docs assets are indexed in the [archive](archive/v1/README.md) and [document inventory](DOCUMENT_INVENTORY.md). They retain original claims and dates as historical content. Removing AMD from the v2 runtime does not erase those records or original contributor credit.

Internal `@dengarin/*` names, keys, cookies, schemas, scripts and old deployments still belong to the running v1 code. New naming and removal happen at the cutover, after acceptance (T9). Old deployment URLs are not a Sela deployment. Two legacy Dengar.in deployments are live outside this repository (health checks returned 200 on 2026-10-26): `dengar-in.vertalt.my.id`, which auto-deploys `zlxrnn/dengar.in-2.0` main, and `dengar-in.vercel.app`, which serves a build later than stage 1 (exact commit not established). They may process real users' text with third-party AI; decommissioning or a notice is an owner/team action (D-047; blocker [B-13](BLOCKERS.md)).

## New-work boundary

| Capability in v2 | Origin |
| --- | --- |
| Crisis lexicon (rule ids preserved) | **Ported** from Dengar.in `services/crisis-engine` (first written by Iqbal Dovandra; 28 rules in stage 2); extended with clause-level denial, quotation and third-party framing, slang, code-switching and idioms; v1 regression tests carried over as vectors |
| Source-grounded Care Passport and clarification | **Ported** from Dengar.in `services/validator/src/carePassport.ts`; extended as the Story Ledger with Meaning Lock (negation, subject, time and certainty checks), versioned approval and user review of every item |
| Opening-lines pipeline | **Ported** from Dengar.in `services/validator/src/openingLines.ts`; extended with pressure-language, number and identifier checks and a rehearsal view |
| Bilingual ID/EN support | **Inherited** concept; v2 dictionaries are new |
| Help resources | **Inherited** entries marked `verified_official` in v1 (Healing119, LISA, Yayasan Pulih), unchanged |
| Go API, in-memory session store, consents and retention, Next Step, handoff with token + access code, fictional recipient view, receipts, follow-up, revocation | **New** in v2 |
| React/TypeScript frontend | **New** in v2 |
| Meta Muse Glimmer on Modal (adapter, serving app, smoke test) and the labelled test double | **New** in v2 (not yet deployed) |
| Research corpus pipeline and reviewer search | **New** in v2, built on the Sela Relay register |
| Evaluation harness with pre-registered thresholds | **New** in v2 |

| Capability in v3 | Origin | Status |
| --- | --- | --- |
| Persistence (PostgreSQL), accounts, guest mode, recovery codes, per-user envelope encryption, export, deletion with crypto-shredding | **New** in v3 ([ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md)); v1 SQL practices reused; the v1 recovery-phrase idea informs the recovery codes | in progress (backend, uncommitted; S-002 to S-005, S-171 to S-177, S-260 to S-262 in the [backlog](BACKLOG.md)) |
| Conversation room, journal, check-in, cross-session timeline, home | **Migrated** from Dengar.in behaviour (FM-07, FM-08, FM-17, FM-18, FM-22); rebuilt in Go/React | planned |
| Memory with correction, provenance classes, handoff statuses, sharing by content, recipient and purpose, access audit | **New** in v3, extending v2 sharing and the Story Ledger | planned; the access-audit foundation is in progress (backend, uncommitted; S-176) |
| AI routing (Meta primary, Claude verifier, DeepSeek fallback) | **New** in v3 (D-030); keeps the v1 idea of bounded tiers with an honest outage | planned; live use blocked externally (B-1, B-8, B-9) |
| Minors (15–17) flow | **New** in v3 (D-031), replacing the v1 age band routing | planned; the age band and guardian consent record are in progress (backend, uncommitted; S-005, S-211); production use blocked externally (B-10) |

Do not count inherited features or documentation-only changes as a newly completed Meta integration, and never count Claude or DeepSeek work as Meta tooling (D-050). See [event notes](META_EVENT_NOTES.md) for unresolved reuse and eligibility questions.

## What v3 reuses from each source

None of the reuse below is implemented or verified yet. Most of it is **planned**; the parts that feed the backend foundation (SQL practices, the recovery-code idea) are **in progress** with it. Completion evidence is tracked per row in the [feature matrix](DENGARIN_FEATURE_MATRIX.md).

| Source | Reused in v3 | Not reused |
| --- | --- | --- |
| Stage 1, `iqbalscript/Denger.in-Web-Project` | The product idea and page set, and the first versions of the crisis engine, validators, Quick Exit, journal, check-in and voice flows, all **through** their stage 2 descendants. Nothing is ported directly from `76e760e`, because stage 1 lacks later fixes (recovery-phrase purge, backup id out of the URL, export escaping, approval reset on edit, passive-ideation crisis rules, the SSR-safe accessibility script; matrix DX-16) | Direct code ports from `76e760e` |
| Stage 2 and Sela v1 (`zlxrnn/dengar.in-2.0` @ `dbcc0ba` = in-repo v1) | Listening reply rules and the high-distress pathway, made inline (FM-08, FM-13); the crisis lexicon and its v1 vectors, already in `internal/safety` (FM-14); journal, check-in, timeline and home behaviour (FM-07, FM-17, FM-18, FM-22); dated evidence, PII redaction preview, escaped export and the manual path (FM-25 to FM-27, FM-29); destination checklists and opening lines (FM-28); the self-help card library with its notices (FM-34); re-verified help entries and support-option limits (FM-32, FM-33); Quick Exit and the "Akses cepat" panel (FM-35, FM-40); opt-in voice dictation (FM-15); the per-destination consent disclosure (FM-02); the alias generator and the "Bahasa Lazim" style guide; rate-limit budgets and fault scenarios; the CareBridge evaluation set (FM-50); SQL practices (`REVOKE … FROM PUBLIC`, `CHECK`, cascades) per ADR-0002 | Retired: severity assessment, gamification, the 14-day path, the OpenRouter debiaser, AMD, the AI response cache, Supabase/forum persistence and the Next.js runtime (D-037, D-038). Paused: the forum and moderation, the Gemini voice room (D-038) |
| Sela v2 (`feat/v2-go-meta-runtime`) | The Go HTTP layer (strict JSON, mutation guard, limits, metadata-only logs), independent versioned consents, version-bound approval, Meaning Lock, share token plus access code, revocation and expiry, the crisis gate, the operation and streaming runtime, ID/EN dictionaries, the Meta adapter and Modal app, the labelled test double, the evaluation harness and the research corpus pipeline; v2 fixtures for the demo configuration only (D-042) | The memory-only store outside development, demo and test (D-027); the adults-only and synthetic-only limits (D-026) |
