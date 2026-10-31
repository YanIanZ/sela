# Judging-oriented demo, experiment and submission plan

**Status: PROPOSED runbook.** Local plan as of 26 October 2026; organizer terms, selection, model access and current submission requirements must be verified independently. [SPEC.md](../../../SPEC.md) remains authoritative.

## Rubric-to-evidence map

The supplied hackathon project rubric weighs **Impact 30%, Use of Meta AI Tooling 25%, Technical Depth & Execution 20%, Demo 25%**. No category is won by documentation alone.

| Judge dimension | Make demonstrable | Exact proof artifact |
| --- | --- | --- |
| Impact (30%) | One real friction: turning an unclear first-consultation story into a user-approved, shared and acknowledged preparation note | 1-minute problem statement, plausible fictional persona, task completion, consent comprehension and honest transfer limits |
| Meta tooling (25%) | Meta proposes a structured source-linked draft, optional strictly read-only grounded tool retrieval and bilingual wording comparison | Sanitized live API trace, model + prompt/fixture versions, JSON validation record, model-off/manual baseline |
| Technical (20%) | Go state machine and authorization hold under stale edits, duplicate requests, concurrent reset/revoke and expired sessions | Go test report, API contract examples, negative-path recording, resource baseline separated from model latency |
| Demo (25%) | Full path visibly reaches fictional professional and scripted follow-up; failure path is coherent | Working deployed slice, single-take main video, shorter crisis/revoke video, reproducible local run instructions |

Do not pad the project with models or services that do not improve the end-to-end task.

## Core pitch

**Indonesia:** “Banyak orang ingin berkonsultasi tetapi kesulitan mencari kata-kata, memilih langkah praktis, dan memastikan catatannya hanya dibagikan kepada orang yang mereka pilih. Sela mengubah cerita menjadi catatan yang dapat diperiksa sumbernya, meminta izin secara terpisah, lalu memperlihatkan status serah terima hingga tindak lanjut. Demo ini menggunakan data dan profesional fiktif; Sela bukan terapis atau layanan klinis.”

**English:** “For many people, reaching professional mental-health support is harder than starting a conversation. Sela turns a story into a source-linked, user-approved brief, lets people choose exactly what to share, and shows the true status of a simulated professional handoff. It is a care-navigation demo, not a therapist or a real booking service.”

## Proposed 3-minute live/video story

This is an internal recommendation; **do not describe three minutes as an organizer-mandated video length**.

| Time | Visual and interaction | The reason it matters |
| --- | --- | --- |
| 0:00–0:20 | Indonesian synthetic first-consultation case; show cost/time uncertainty and fiction label | Problem/impact clear at once |
| 0:20–0:48 | User grants AI consent; genuine Meta structured draft arrives with live_meta provenance | Meta is doing substantive work |
| 0:48–1:13 | Select one claim; show its source. Correct a negation or translation; approval changes version | Meaning Lock engineering value |
| 1:13–1:32 | Filter fictional professionals using explicit preferences; choose one | Practical navigation without invented medical match |
| 1:32–1:52 | Review exact share snapshot, selected fields and recipient; separately approve and consent | Permission isn't an invisible checkbox |
| 1:52–2:13 | Receipt says requested, NOT yet accepted; fictional professional sees only selected fields then acknowledges | Closed-loop handoff |
| 2:13–2:33 | Scripted simulated consultation completes, fixture-authored next step appears | Journey continuity, not chat-only |
| 2:33–2:50 | Revoke; professional reopens and gets access denied | A deliberate failure path |
| 2:50–3:00 | One slide: measured synthetic fixtures, ID/EN parity, state tests and limitations | Trustworthy technical finish |

A 20–40-second supplemental recording can show no-AI manual flow and a crisis fixture that interrupts **before** Meta inference. A separate English path must be executable, not just translated labels.

## Evaluation protocol: compare against meaningful baselines

Use seeded, versioned held-out **synthetic** scenarios, with English, Indonesian and limited code-switching cases. Baselines: A, manual structured form; B, Meta summary without Story Ledger; C, Sela Relay with source-backed validators and approval/handoff. Keep fixtures and task instructions comparable and report denominators. B vs C is a multi-component comparison; do additional ablations to attribute improvements.

| Metric | Operational definition | Evidence |
| --- | --- | --- |
| Unsupported claim rate | Unsupported factual claims / total factual claims; report before and after validation | Human-scored claim sheet and fixture source IDs |
| Source coverage | Preserved important fixture facts / expected important facts | Gold synthetic annotation, including negation/time/uncertainty |
| Correction burden | Count meaningful corrections and review time to approved brief | Instrumented scripted trial |
| Consent comprehension | Correct answers about recipient, fields, current version and revocation limits / all questions | 4-question user task |
| Journey success | Users finishing the fictional task unaided / users starting | Small descriptive usability study; report dropouts |
| Isolation and stale-write failures | Unauthorized successful operations in explicitly tested cases | Repeatable Go/API negative tests |
| Model contribution | Live Meta present vs unavailable/manual; latency/cost separately logged | Provider metadata without story contents |

Do not claim clinical efficacy, statistical significance, improved treatment adherence or safety certification from hackathon data. No patient histories or real mental-health risk judgments belong in user tests. Report known failures and uncertainty as part of the work.

## Failure-mode rehearsal

- Meta unavailable: clear status, user can continue manually; no hidden AMD/Gemini substitution.
- Empty professional result: transparent fictional catalog limitation; change explicit filters.
- Stale approval: block sharing and require re-review.
- Clarification/refusal: show status and ask for a new explicit consent if recipient or shared snapshot changes. This branch is a stretch proposal until adopted into SPEC/API.
- Revoke or reset: stale professional view must fail; explain that previously read/copied text cannot be erased.
- Crisis fixture: immediately interrupt with geographically qualified help information; never simulate a successful care transaction.
- Accessibility/language: complete the same flow by keyboard with Indonesian and English copy.

## Calendar and freeze plan (WIB)

Official public event calendar: hackathon starts **26 October 2026**; submission is due **4 November 2026 at 21:00 US Eastern**, which corresponds to **5 November 2026 at 09:00 WIB** due to the US DST change by that date. Verify the portal shortly before submission and plan an earlier internal freeze.

| Date (WIB) | Solo-builder gate |
| --- | --- |
| Oct 26 | Finalize scope, attributable new-work plan, v1 baseline, Go + Meta access probe |
| Oct 27 | Go sessions/fixtures/state skeleton + new frontend shell |
| Oct 28 | Deterministic crisis, approval, consent, share and revoke tests |
| Oct 29 | Meta structured draft + evidence validation on synthetic cases |
| Oct 30 | ID/EN Story Ledger, Meaning Lock, options and share UI |
| Oct 31 | Fictional professional view and acknowledged receipt |
| Nov 1 | Scripted completion, follow-up, manual/error branches |
| Nov 2 | Held-out bilingual checks, security/isolation, accessibility |
| Nov 3 | Stabilize, run repeatable evidence, record video and backup |
| Nov 4 | Submit in advance of official cutoff; preserve a working commit/tag |
| Nov 5 by 09:00 WIB | Absolute public deadline, subject to organizer changes |

**De-scope rule:** if P0 fails to reach an acknowledged simulated handoff by Oct 31, drop optional audio, extra personas, complex dashboards and all P2 items. If live Meta remains unavailable, do not misrepresent recorded output; show the true manual/recorded capability and disclose limitations.

## Authorship, reuse and submission evidence

Sela inherits team-authored Dengar.in code under its stated license. Maintain original author credit, notices and evidence; verify any additional hackathon-specific representations about the scope of preexisting work and rights. A solo maintainer does **not** automatically own all earlier collaborative work. Record new Go/Meta functionality committed within the hackathon period, link relevant PRs and identify reused elements honestly.

Submission packet should contain a clear repo link, running demo instructions, explicit simulated-care banner, full journey video, brief architecture/Meta explanation, sanitized test/evaluation results, bilingual screenshots, research map and limitations. Organizer's actual mandatory format, video constraints and acceptance remain to be confirmed. See [event notes](../../architecture/META_EVENT_NOTES.md) and [provenance](../../product/SELA_PROVENANCE.md).