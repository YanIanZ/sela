# Roadmap

Updated: 2026-10-26. Status words: [SPEC §G](../SPEC.md#g-goal), their single definition. Status authority: the [backlog](BACKLOG.md) (items, priorities, milestone exit criteria), [implementation status](IMPLEMENTATION_STATUS.md) (where each item stands) and, for v2 tasks, [SPEC.md §T](../SPEC.md#t-tasks). This roadmap explains the order of the v3 milestones and the gates between release levels. External blockers: [blockers](BLOCKERS.md) (B-1 to B-15).

## 1. Milestone order (v3)

The owner brief lists eight milestones. Sela v3 keeps all eight and changes the order where dependencies require it: the storage and frontend foundations of brief milestones 2 and 3 come first, because every surface stores owned, encrypted records and runs in the new app shell; the listening experience and the persistent notes then land together. The AI routing core starts with the listening room, and the verifier and fallback follow the features they check. Research runs in parallel from the start.

| Order | Milestone | Brief milestone | Depends on | Why here | Exit (summary; full criteria in [backlog §4](BACKLOG.md#4-milestones)) |
| --- | --- | --- | --- | --- | --- |
| 1 | **P1** Audit and spec alignment | 1 | — | Every later item cites the realigned spec, decisions and ADRs | Spec amended for the superseded v2 limits; D-025 to D-049 (PM) and D-050, D-051 (engineering defaults) and B-8 to B-15 recorded; planning documents committed and link-checked |
| 2 | **P2** Foundation: persistence, accounts and guest mode, encryption, consents, export and deletion; design direction, design system, app shell; help and Quick Exit on every screen | 3 (storage) and 2 (frontend) | P1 | Every later surface writes owned, encrypted records and needs the shell | ADR-0002 F-01 to F-46 pass locally and in CI; welcome → guest → consents → upgrade → export → delete runs in e2e with the WCAG 2.2 gate; every P2 screen passes the browser review with screenshots at 320 CSS px and desktop, in ID and EN (S-195) |
| 3 | **P3** Listening room, journal, check-in, memory, timeline, inline crisis support; routing core with the Meta path | 2 (listening) and 3 (notes, timeline) | P2 | The core experience; consultation summaries are built from these records | The talk, journal, check-in, memory and timeline journey runs in e2e with the test double; crisis entries save with help and zero model calls |
| 4 | **P4** Consultation summary, sharing, recipient, follow-up | 4 | P3 | A summary selects saved entries and memory | Manual summary → share with purpose → recipient opens → owner revokes → recipient denied → after-consultation reflection, in e2e |
| 5 | **P5** AI routing complete: Claude verifier, DeepSeek fallback, AI summary wording, memory suggestions, evaluation | 5 | P3, P4 | The verifier checks P3 and P4 outputs; its live parts are blocked (B-1, B-8, B-9), so placing it after P4 costs no calendar time | Routing invariants hold with three labelled test doubles; EV-0 and thresholds frozen before any live run; live EV-1 to EV-4 when the blockers clear |
| 6 | **P6** Research and evidence; reading library and "latihan kecil" | 6 | P1 (parallel track); reading verification needs the verifier (P5) | Independent data work that informs copy and reading content | Corpus hygiene fixed; v3 gap search frozen; v3 evidence map with a row per feature, brief §9 topic and population, gaps declared; no reading item or exercise published unverified |
| 7 | **P7** Accessibility audit, display settings and opt-in voice (dictation, read-aloud), security review, hosted operations, minors gate | 7 | P3 to P6; owner actions | Needs finished surfaces to audit and hosting to deploy; accessibility and security criteria are already part of every item | Full manual accessibility review; security findings closed or decided; staging and demo deployed with a restore drill; Modal deployed; minors and legal status recorded |
| 8 | **P8** Final integration and pilot readiness | 8 | all | Owner testing and the readiness report close the program | Owner ran the pilot-critical scenarios of `OWNER_TESTER.md`; readiness report places every DoD capability on the release ladder |

```text
P1 ─► P2 ─► P3 ─► P4 ─► P5 ─┐
│                            ├─► P7 ─► P8
└──────► P6 (parallel) ──────┘
Owner actions B-1, B-3, B-5, B-8, B-9, B-10 and B-15 can start now. They gate "verified live"
(each for its provider), the hosted part of P7 and "ready for controlled pilot":
B-1, B-3, B-5, B-8, B-10 and B-15, and B-9 only if the pilot enables DeepSeek.
```

Work that is blocked externally never stops independent work: each blocked item is prepared to the point where only the owner action remains, and the rest of its milestone proceeds.

## 2. Release ladder

The status words are defined in [SPEC §G](../SPEC.md#g-goal); this section lists only the gate for each level and who decides. The first three levels apply to each backlog item; the last two apply to the product as a whole. A level is reached only when every gate below it holds, and the evidence is recorded in [implementation status](IMPLEMENTATION_STATUS.md). A report from an agent is not evidence by itself: the PM checks the results (brief §13), and a self-review is never described as independent approval.

| Level | Gate to reach it (all must hold) | Who decides |
| --- | --- | --- |
| **Implemented** | Every acceptance criterion of the item has a test or a recorded check; the code is committed on the delivery branch with the verified commit identity and described in its PR (problem, outcome, acceptance criteria met, verification evidence, migrations, limitations; brief §12; [traceability §1.3](REQUIREMENTS_TRACEABILITY.md#13-delivery-and-github-brief-12)); no placeholder is shown as a working feature; the affected documents are updated | PM, after reading the diff and the tests |
| **Verified locally** | Go vet, format and race tests including the database tests (`SELA_REQUIRE_DB_TESTS=1`), the web typecheck, unit tests and build, the e2e journeys with the WCAG 2.2 AA gate (backlog S-194), the research and Modal offline tests and `sela-eval` offline and test-double modes all pass locally and in CI; items with a screen pass the browser review with screenshots at 320 CSS px and desktop, in ID and EN (S-195); test doubles are labelled and no live provider is claimed | PM, from the CI results and the review record |
| **Verified live** | Deployed to staging (and the demo where relevant) from the same image as production; Meta on Modal live (B-1), Claude live (B-8), DeepSeek live where enabled (B-9); smoke test passes; the live evaluations EV-1 to EV-4, and EV-5 before the fallback is turned on, meet the pre-registered thresholds; a backup and restore drill and a rollback rehearsal are recorded on the hosted environment; spending stays within the approved budget | PM, with recorded evidence; the budget is the owner's |
| **Ready for controlled pilot** | The PM decision of 2026-10-26: live providers verified (Meta, B-1; Claude, B-8; DeepSeek, B-9, only if the pilot enables it); the legal items of B-10 cleared (DPIA, DPO, processor agreements and the cross-border basis, PSE registration, breach-runbook contact, the health-data localisation question, minors); hosting and region decided (B-3, B-15); help lines re-verified (B-5); owner testing passed (SPEC T21, S-270). By the definition of pilot-critical, every pilot-critical item is at least verified locally and every pilot-critical item that calls a provider is verified live. Production runs with `18_plus` only unless the minors gate (S-212) decided otherwise | Owner |
| **Public release** | Everything for the controlled pilot, plus every public-release gate in the table below, which is the list of [blockers, Gates](BLOCKERS.md#gates) (the authority SPEC §G names) | Owner |

**Public-release gates.** The same list as [blockers, Gates](BLOCKERS.md#gates); a change to it is made there first.

| Gate | Item or blocker | Today (2026-10-26) |
| --- | --- | --- |
| Everything for the controlled pilot | The row above | Not met: no v3 item is implemented |
| The controlled pilot completed under its plan, with its findings addressed | S-271 (internal) | planned; no pilot has run |
| Consented, ethics-reviewed user research across age groups, a required gate (adults 50+ and, if minors are enabled, ages 15–17 represented) | B-6, S-249 | blocked externally (B-6) |
| An independent security assessment and an independent accessibility audit, not self-reviews; S-266 and S-195 are internal reviews and are never presented as independent | Internal; no backlog item yet | planned (specified only here and in blockers, Gates) |
| The minors decision under PP 17/2025 (compliance by 2027-03-27) and the obligations of PP 33/2026 once its official text is verified | B-10, S-212 | blocked externally (B-10); the official text of PP 33/2026 is unverified |
| Readiness for more than one instance, and incident on-call | S-268 (internal) | planned (post-pilot) |
| The legacy Dengar.in deployments resolved | B-13, S-267 | blocked externally (B-13) |
| For any commercial use: the WHO card licence, or replacement cards | B-14 | blocked externally (B-14) |

Rules that apply at every level:

- A regression moves an item back to the highest level whose gates still hold.
- Changing a provider, model, endpoint or hosting region requires live re-verification and new consent under a new policy version (ADR-0003 §8.1).
- Engineering tests and evaluations on synthetic data are never presented as clinical evidence, and agent reviews are never presented as user research (brief §13).

## 3. Dated checkpoints

| Date | What | Effect on the plan |
| --- | --- | --- |
| 2026-11-04 21:00 ET | Meta hackathon submission deadline | **Submission build (proposed; the owner confirms, because submitting is an owner action, B-2, B-12):** the v3 `meta_only` profile (S-227, P3) if it is verified locally by then; otherwise the v2 demo of PR #6, the only build that exists today. Both are Meta-only. v2 has no non-Meta provider mode (`modal`, a labelled test double that production refuses, or `disabled`; traceability BE9 and M13), and the EvidenceGuard prototype is not wired into it. Under `meta_only`, no Anthropic or DeepSeek call is made and the verifier-gated features abstain (ADR-0003 RR-15). Either way, live Meta output needs B-1, and a hosted demo needs B-3; without B-1 the demo runs with AI off or with the labelled test double, which is never presented as Meta inference. The organizers' answer on non-Meta providers is B-12 (D-049) |
| 2026-11-12 | Anthropic Acceptable Use Policy update takes effect | Re-check the verifier's use against the new text (ADR-0003 §2) |
| 2027-01-14 and 2027-01-15 | Re-verify-by dates of the current help-line fixtures | S-154 must be done before then, and before any real user |
| 2027-01-16 | PP 33/2026 reported to take effect (secondary sources; official text unverified) | Part of the legal review (S-178) |
| 2027-03-27 | PP 17/2025 compliance deadline (Pasal 49) | Minors gate (S-212) |
| not before 2027-09-28 | Earliest retirement of `claude-sonnet-5-5` | Plan a model change and re-run EV-2 and EV-3 before it |

## 4. v2 stages (v2 scope)

The table below is the v2 roadmap of branch `feat/v2-go-meta-runtime` (PR #6). Its states are unchanged and remain true for v2.

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

## 5. Cutover and removal of v1

**v2 plan (T9, v2 scope):** remove at cutover the AMD adapters, probes and env requirements; the v1 Next.js API and runtime; the v1 multi-provider fallbacks; forum and gamification; the PostgreSQL, Redis and Supabase infrastructure of v1 from the deployment. Decide exact deletions from dependency evidence. Keep licences, font attribution, Git authorship and archived evidence, and port any v1 test whose protection still matters.

**v3 refinement (backlog S-272, priority post-pilot, milestone later):** the removal waits for v3 acceptance rather than v2's. The v1 provider chain (DeepSeek, Gemini, OpenRouter) is removed with the v1 runtime; v3's own consented fallback follows ADR-0003 instead. v1's database schemas are removed; v3 keeps PostgreSQL with its own schema (D-026). The forum and the voice room stay in Git history and reopen only through backlog items S-142 and S-027 (D-037). Before removal, port what still protects people: the PII redactor patterns (S-104, S-226), the self-help cards (S-140), the destination checklists (S-103) and any crisis vectors not yet in `internal/safety`.

The v2 roadmap's "after the hackathon" list (discovery with users and professionals, one partner and region first, legal review, professional oversight, credential verification, crisis responsibility, consented usability research before any real data) is now part of the gates in §2: the legal review precedes the controlled pilot, credential verification and partnerships come only with S-122, and consented, ethics-reviewed user research is a required public-release gate (B-6, S-249).
