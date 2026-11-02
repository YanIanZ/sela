# Demo narrative, script and submission readiness

Status: **proposal, not submitted**. Competition: Meta Global AI Developer Hackathon 2026; Indie Developer / Build for Your World. Submission due **2026-11-04 21:00 ET** (2026-11-05 09:00 WIB), organizer may change timeline. Sources: https://cerebralvalley.ai/e/meta-ai-developer-hackathon and https://dev.meta.ai/events/global-hackathon .

This playbook assumes **existing production hosting per owner report** but a production run is **not verified**; use [production verification](PRODUCTION_VERIFICATION.md) for honest provenance. Never use real user stories or pretend simulated professional services are real.

## 1. Positioning

**Title:** Sela — Listening AI that respects what you actually meant.

**Short pitch:** Sela is a bilingual, privacy-first place to talk. Unlike a generic companion that silently turns AI guesses into personal “facts”, Sela lets people trace, correct and control their meaning across memory and the consultation summaries they choose to share.

**Problem:** A person can say “sometimes I need to be alone” and have AI interpret it as “socially isolated”. In a wellbeing context, that shift of subject, certainty or frequency matters, especially if the interpretation later travels to a care conversation.

**Solution:** Meta-driven reflections are source-linked proposals; Go enforces consent, source/version checks, deterministic safety and dependency invalidation; the person decides what becomes lasting memory and what is shared. This is an engineering and agency claim, *not* medical efficacy.

**Novelty boundary:** Source quotes, edit buttons and wellbeing chats have precedents. The proposed distinguishing *combination* is end-to-end meaning revision propagation into memory and selective care preparation, validated by a reproducible baseline/ablation. Do not say “world's first”, patented, medically validated or connected to hospitals.

## 2. Main three-minute shot list (editorial target; verify actual video rules)

| Time | Visual / user action | Voice-over point | Evidence |
| --- | --- | --- | --- |
| 0:00–0:15 | Quiet landing page, fictional Nadia chooses language | “People need somewhere to be heard, without losing control of their story.” | Screenshot on pinned SHA |
| 0:15–0:35 | Nadia types an ordinary synthetic worry | “Sela responds with Meta; it does not diagnose.” | True provider provenance; latency |
| 0:35–0:55 | AI proposes an interpretation with exact quote | “A reflection is an interpretation, never a fact without review.” | Source link/version |
| 0:55–1:20 | Clearly injected fixture shows wrong interpretation (“sometimes”→“always”); user rejects | “Sela makes AI mistakes reversible and visible.” | Test fixture label, rejection event |
| 1:20–1:45 | Lineage impact marks memory/summary stale and blocks old export | “Correction propagates to downstream uses.” | Postgres graph / API assertion |
| 1:45–2:10 | User approves revised summary, chooses exact fields and opens fictional recipient | “Only the approved version can be shared.” | Frozen projection / link + separate code |
| 2:10–2:25 | User revokes and recipient retries | “No new reads after revocation, though past copies cannot be recalled.” | 403/410 or exact documented denial |
| 2:25–2:45 | Baseline B0 vs full MIE comparison with n, error bars and failure | “This is how we measure the difference, including the remaining errors.” | Actual preregistered results only |
| 2:45–3:00 | Architecture single slide and product closing line | “AI should not own your meaning. You should.” | Repo, source, build SHA |

A full run may exceed 3 minutes; film clean reproducible segments rather than editing together inconsistent builds. Keep text legible, 16:9 if portal accepts, subtitles ID/EN, simple movement, no fabricated counters. Avoid showing real tokens, email, domain admin pages or still-valid share links. If demo has not achieved MIE invariant, de-scope and rewrite script rather than editing video to imply it has.

## 3. Backup demonstrations

### Backup 1 — Meta service unavailable
Show a **real** unavailable state or deterministic fault in **isolated test environment**, not production. The handwritten journal/draft remains usable. Label test fault injection; don't call it live provider outage unless it actually occurred.

### Backup 2 — Reviewer cannot load the site
Provide a current working video, reproducible repo commands, screenshots and a clearly labelled synthetic seeded walkthrough. The backup does not replace a real hosted proof if the event requires one.

### Backup 3 — Verifier rejects
Show abstention/review needed, original story unchanged, no “safe because verified” marketing. Explain Wonder is a different Meta model, not clinical expert and not ground truth.

### Backup 4 — Screen reader / no AI
Show one keyboard-only task and language switching, then off/consent-denied route. Demonstrate ethical accessibility work without pretending one device test covers all ages.

### Backup 5 — Recipient can't complete interaction
Show the user exporting the approved summary locally and explanation of the simulated recipient flow. Never pretend healthcare professional replied.

## 4. Submission materials

| Material | Planned source / requirements | Status |
| --- | --- | --- |
| Public repository | YanIanZ/sela and exact code/tag used | URL known; final SHA TBD |
| Live demo URL | Owner confirms deployed production; use safe synthetic mode | OWNER-REPORTED, NOT VERIFIED |
| Video | 3-minute target + subtitles, no personal data | NOT RECORDED |
| Project description | Pitch, problem, novelty boundary, real-world intended impact | DRAFT IN THIS DOC |
| Architecture diagram | Go privacy/consent gates, Meta Spark, Meta Wonder, PostgreSQL, browser | PLANNED |
| Engineering evidence | CI for exact SHA, e2e, security/revoke/race, WCAG results | NOT COLLECTED FOR PROPOSED MIE |
| Model evidence | Observed Meta API/Modal mode, model IDs, failure labels, budget | NOT COLLECTED FOR FINAL SHA |
| Baseline benchmark | B0/B1/B2/T metrics + confidence intervals + limitations | NOT RUN |
| Credits / license | Sela lineage and contributors to Dengar.in; third-party terms | DOCUMENTED, REVIEW NEEDED |
| Safety limitations | Not therapy/diagnosis/emergency, synthetic recipients, no clinical claim | DRAFT |
| Eligibility check | Official portal, accepted participant, video/rights/format rules | OWNER MUST VERIFY |
| Submission receipt | Portal confirmation and timestamp | NOT SUBMITTED |

## 5. Authorship and rights

Sela continues the team-originated Dengar.in project. The individual maintainer's ownership of new contributions does **not** imply sole ownership of all team-authored original work. Keep original credit, relevant license notices and accurate preexisting/new feature boundaries. Check third-party data/WHO license conditions separately. See [SELA_PROVENANCE](../product/SELA_PROVENANCE.md), [LICENSE](../../LICENSE), and full repository history. Never represent a solo hackathon entry as proof that inherited code is exclusively solo authored.

Prepare a short source-history paragraph for submission:
> Sela evolved from the collaboratively developed Dengar.in foundation. This entry focuses on the new Go-based, Meta-powered listening, consent, provenance, and user-governed meaning/care workflows. Inherited components are credited in the project repository.

Adjust the sentence if the submitted code does not actually implement all named additions.

## 6. Exact live-vs-synthetic labels

| State | UI label | Rules |
| --- | --- | --- |
| Genuine Meta response in authorized environment | `Live Meta • <observed model>` | Must have real server provenance and valid policy consent |
| Local/offline test double | `TEST DOUBLE — simulated model` | Never count as live Meta |
| Recorded legitimate prior run | `Recorded run • date/build/model` | No suggestion of present live inference |
| Scripted wrong-interpretation fixture | `Simulated AI error for testing` | Do not manufacture a live failure |
| Fictional recipient/professional | `Simulasi • fictional participant` | No actual booking, hospital linkage or advice |
| Manual/no-AI mode | `No AI used` | Do not display invented verification claims |

No use of external consumer API without explicit authorizing environment, privacy policy and terms. A verifier “pass” must not become “clinically approved”.

## 7. Reviewer walkthrough / runbook

1. Open landing; confirm build marker and synthetic disclaimer.
2. In synthetic guest, consent to Meta and start harmless conversation.
3. Inspect initial source-bound reflection and provider ID.
4. Trigger **labelled** wrong-claim fixture and correct/reject it.
5. Open source→memory→summary impact and demonstrate safe stale gating.
6. Approve revised summary and inspect exactly shared fields.
7. Open fictional recipient context, acknowledge, revoke and retry.
8. Open EVALUATION report with n, failures and measurement procedures.
9. Show no-AI and safety fixture, without suggesting emergency contact happened.
10. Close with links to code, test evidence and known limitations.

Keep a clean browser context and known fixture seed; use reset only in isolated demo DB; never show owner production admin/real data.

## 8. Final copy blocks for adaptation (not final unverified claims)

**English / short:** “Sela is a privacy-first, bilingual listening space powered by Meta models. It lets people review the AI's interpretation of their words, preserve the original source, and control what becomes memory or a professional-care summary. Our flagship work focuses on making corrections propagate safely across those surfaces.”

**Indonesian / short:** “Sela adalah ruang bercerita bilingual yang mengutamakan privasi. Interpretasi AI bukan fakta otomatis: pengguna dapat melihat sumbernya, mengoreksi, menentukan memori yang disimpan, dan memilih ringkasan yang akan dibagikan. Fokus inovasi kami adalah konsistensi makna saat suatu interpretasi dikoreksi.”

Don't replace “focuses on” with “proven” until MIE implementation/evaluation meets gates.

## 9. Final Go/No-Go sign-off

```yaml
submission:
  event: Meta Global AI Developer Hackathon 2026
  track: Indie Developer
  theme: Build for Your World
  cutoff_wib: "2026-11-05 09:00"
  owner_portal_requirements_confirmed: false
  repository_sha: null
  demo_release_sha: null
  demo_privacy_review: NOT_DONE
  live_meta_verified_on_release: false
  m_ie_graph_verified: false
  ci_on_submission_sha: NOT_RUN
  heldout_benchmark_reviewed: false
  seven_scenarios_status: NOT_RUN
  attribution_reviewed: false
  owner_approval: false
  submission_receipt: null
```

Submit only truthful capabilities; the project can still be submitted with reduced scope if issues are clearly disclosed and core safety gates hold. This document or its PR **does not submit** to the hackathon.
