# Sela | Product requirements (PRD)

**Version:** vision-2026.10.29  
**Owner decision:** anonymous-first full-product, text-only, user-controlled meaning, evidence-informed, optional professional care  
**Document state:** proposed requirements pending synchronization with SPEC.md and implementation  
**Not a launch statement.**

## 1. Problem and opportunity

People can struggle to express experiences accurately, remember what they meant over time, or present a concise summary at an appointment. Generic AI may overinterpret short statements, convert guesses into remembered facts, and conceal where its conclusions came from. Sela aims to reduce that friction **without replacing professional care**.

## 2. Users and inclusion

- Adults (18+) who prefer to start without providing identifying account information.
- Indonesian-primary and English users, including code-switching and accessibility needs.
- People seeking reflection alone; people preparing to seek help; people continuing their own journaling after an appointment.
- Low-end phones, slow connections and shared-device situations.
- **Under-18 access is not authorized by this PRD**: age eligibility requires model/provider restrictions, safeguards, legal review and an owner-approved production policy. Do not infer permission from historical 15–17 designs.

## 3. Product promises

1. **Start without signup or GPS**; describe storage choices before saving.
2. **Respect the original:** never overwrite the user's own words; show the source of every AI interpretation.
3. **Do not infer a diagnosis:** leave unknowns unknown; distinguish user experience, AI hypothesis and published evidence.
4. **Make AI correctable:** a correction changes what future features may use, not merely the current chat reply.
5. **Put the user in control:** opt-in memory, explicit sharing, revoke future access, export and delete.
6. **Help is optional:** quick help independent of AI, and manual selection of planned professional support.
7. **Be honest:** no fake directory entries, availability, appointment, live provider or clinical validation claim.

## 4. User journey and feature requirements

| ID | Feature | User action / desired behavior | Acceptance criterion |
| --- | --- | --- | --- |
| SV-01 | Anonymous entry | Choose temporary or private anonymous space; never required to register | No mandatory name, email, password, phone or GPS field; consent record and random session identity |
| SV-02 | Text listening | Talk, choose whether to save, stop/exit instantly | AI off/outage still permits writing and help; no voice/avatars |
| SV-03 | Journal / check-in | Write, edit, timestamp, view timeline, delete | Encrypted authorized content; original + revisions; full ownership checks |
| SV-04 | Reflection | Meta suggests a traceable, tentative interpretation | Source-span anchors; no diagnosis; clear uncertainty and one-click reject/correct |
| SV-05 | Meaning integrity | Approve or correct an interpretation | Corrected source invalidates all derived memory/summary proposals before reuse |
| SV-06 | Optional memory | Review suggestions, decide if remembered, remove | Off by default; approved-only retrieval; deletion propagates to derived items |
| SV-07 | Research | Browse curated studies and official guidance | Every informational claim includes source ID, URL, quality/limitations; no personal notes added to public corpus |
| SV-08 | Consultation summary | Select journal passages, edit and export a personal summary | Only chosen and current sources; provenance, versioning, user approval and no diagnosis |
| SV-09 | Care discovery | Pick a professional type/facility and manually enter any city/name | No geolocation API or required city; verified directory source, last-checked status; no invented schedules |
| SV-10 | Care context capsule | Select what to disclose and preview payload | Private-by-default, version-pinned, revocable future sharing; no automatic send |
| SV-11 | Partner simulation | Explore how multi-facility requests would work | Synthetic identities/facilities visibly labelled; no actual request or appointment implied |
| SV-12 | Safety and quick help | View help without waiting for any AI service | Crisis processing halts ordinary AI flow; never auto-contacts or routes around a refusal |
| SV-13 | Privacy controls | Withdraw provider consent, delete/export, end session | No provider request after withdrawal; key/retention explanations accurate |
| SV-14 | Accessibility | ID/EN, screen readers, keyboard, reduced motion, 320px reflow | WCAG 2.2 AA automated + manual checks; tasks work in both languages |
| SV-15 | Separate operator console | Staff-curated verified directory/research/help and operational overview | MFA-protected, role/tenant authorization and content-free audit; zero implicit journal access |
| SV-16 | Planned care administration | Source-backed organization/professional verification and future-partner sandbox | Independent approval, public source freshness; fictional requests isolated from real services |
| SV-17 | Redis acceleration | Cache vetted public directory/research metadata and bounded distributed rate counters | No private content, short TTL/revision, graceful cache outage fallback |
| SV-18 | AI spending control | Bound Meta/Claude/DeepSeek calls using per-task budgets and admission | PostgreSQL authoritative reservations; Redis fast counters; cannot exceed a cap after Redis failure |

## 5. Interaction contracts

### Temporary session
No persistent story; memory/notes unavailable unless explicitly switched to a private space. Exit discards session-scoped unsaved content according to documented expiry; Quick Exit must explain the distinction between hiding the screen and deleting saved data.

### Private anonymous space
Cryptographically random session subject with server-side owner scoping and encrypted saved records; a separately protected optional recovery capability may be explored. Anonymous subject IDs are **not** passed to AI providers. No fingerprint-based recovery or covert tracking.

### Understanding and memory
The AI may ask permission to store a source-bound interpretation. The user may approve, amend or reject. Rejection may be respected as a temporary conversational boundary; **do not covertly persist sensitive rejected text**. Deterministic non-diagnostic policy applies even without any memory.

### Planned care
Search and compare **real, publicly sourced, verified** organizations, professionals and services; location may be typed as a filter but is never inferred. Export personal consultation notes independently of any facility partnership.

### Partner collaboration
All workflows that claim a facility accepted a request, a clinician acknowledged it, or a slot was booked remain **synthetic simulations** pending written partnership, approved integration, privacy/legal review and verified recipient identity.

## 6. Nonfunctional requirements

- **Go-first:** modular monolith, bounded goroutines and contexts, server-side secrets, predictable memory/CPU. Benchmark rather than promise lower RSS.
- **Data:** PostgreSQL 16 with encryption/deletion guarantees; no user story in logs, telemetry, fixtures or public research data.
- **Performance:** track p95 API latency excluding external model wait, model latency separately, RSS, concurrency, cost per successful task; quantitative thresholds defined before final evaluation.
- **Security:** session fixation prevention, CSRF/same-origin checks, per-session and per-IP rate limits with privacy-conscious retention, input/output limits, audit without content.
- **Resilience:** fail closed for consent/safety/verification; degraded manual path for outages.
- **Accessibility:** meaningful keyboard focus, labels, 200% zoom, 320px width and slow-connection behavior.
- **Evidence:** benchmark, source validation, adverse-case suite and reproducible owner testing; no fabricated user outcomes.

## 7. Exclusions and prohibited claims

No voice/TTS/ASR, avatar, required user account, background GPS/IP geolocating, diagnosis, clinical scoring, therapy, automatic referrals, unsolicited outreach, fake testimonials, fake doctor profiles or false claims of established partners. No model output silently becomes a medical record or professional opinion.

## 8. Success measures

**Product:** task completion for start → reflect → correct → export; source fidelity; successful deletion; user comprehension of data controls; time to create a useful personal consultation summary; accessibility task completion.

**AI:** source-anchor precision, unsupported-inference rate, model identity/provenance, guardrail bypasses, abstention correctness, crisis-routing regressions.

**Infrastructure:** RSS, Go API p95, outbound AI p95, per-provider cost and error rate under a fixed load profile. Record samples and environment alongside metrics.

All quantitative impact results are **pending actual measurement**. A small, consented synthetic-scenario usability study can establish usability, **not clinical efficacy**.

## 9. Admin and infrastructure requirements (2026-10-29 extension)

The anonymous application and the **operator domain** must be built as separate identities. Staff authenticate with MFA and can edit/review public provider directories and curated research using least-privilege role grants; **no operator gains direct access to anonymous user stories**. Professional and hospital entries must be based on sourced, independently verified information. Synthetic collaboration workflows exist only in a visibly isolated partner sandbox.

Redis is an accelerator for public data and a best-effort distributed admission plane, **not** storage for personal journals or AI prompt/completion data. Durable PostgreSQL transactions own cost reservations and business decisions. If Redis control fails and no safe global budget admission exists, AI must abstain; journaling and urgent help continue.

See [Admin Console](ADMIN_CONSOLE.md), [Planned Care Operations](ADMIN_CARE_OPERATIONS.md), [Redis/AI Cost Engineering](REDIS_AI_COST_ENGINEERING.md), [ADR-0005](../adr/ADR-0005-admin-and-redis-acceleration.md). Implementation remains planned.
