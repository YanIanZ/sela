# Migration roadmap

Status authority: [SPEC.md §T](../SPEC.md#t-tasks) and [implementation status](IMPLEMENTATION_STATUS.md). This roadmap explains ordering.

| Stage | Spec task | Exit condition | State (2026-10-26) |
| --- | --- | --- | --- |
| Documentation baseline | T1 | Source of truth, active docs, archive map | Done |
| Evidence and access | T2 | v1 baseline recorded; Go pinned; Meta model chosen and pinned; live access verified | Baseline, pins and ADR done; live access blocked (B-1) |
| Runtime foundation | T3 | Go API + static frontend, bounded sessions, fixtures | Implemented and tested |
| Rules before inference | T4 | Crisis, consent, version/approval, share isolation, retention/deletion tests | Implemented and tested |
| Meta text slice | T5 | Validated live output plus honest manual/unavailable paths | Adapter, validation and failure paths tested with the test double; live pending (B-1) |
| Optional verification layer | T5a | Separate Anthropic consent, allowed evidence, version-bound report, honest unavailable/manual review; mock and live evidence distinguished | Standalone prototype on main (PR #5); not integrated into v2 |
| User journey | T6 | ID/EN onboarding, story, ledger, next step, practice, manual path | Implemented; e2e with test double |
| Continuity | T7 | Handoff, recipient view, consultation, follow-up, revocation | Implemented; e2e with test double |
| Acceptance | T8 | Behavioural, accessibility, failure and resource evidence | Offline/test-double evidence recorded; live latency/cost/bilingual quality pending |
| Research | T10 | Reproducible corpus, rights tracking, reviewer search | See implementation status |
| Modal serving | T11 | Deployed dev/staging, smoke test, rollback rehearsed | Code and docs ready; deployment pending (B-1) |
| Evaluation | T12 | Thresholds registered and measured | Registered; offline/test-double measured; live pending |
| Cutover and removal | T9 | v2 accepted including live Meta; v1/AMD runtime retired; tooling and deployment switched | Not started |

## Remove at cutover (T9)

AMD adapters, probes and env requirements; v1 Next.js API/runtime; multi-provider fallbacks; forum/gamification; PostgreSQL/Redis/Supabase infrastructure from the deployment. Decide exact deletions from dependency evidence. Keep licences, font attribution, Git authorship, archived evidence, and port any v1 test whose protection still matters.

## After the hackathon

Discovery with prospective users and professionals; one partner service and region before a broad catalogue; legal review (Indonesian PDP law), professional oversight, credential verification, crisis responsibility and consented usability research before any real data.
