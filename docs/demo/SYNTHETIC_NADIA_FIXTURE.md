# Synthetic Nadia storyline — reproducible fixture specification

Status: **fictional scripted content for a future isolated demo; not an inserted production record**. All names, events and care partners below are made up. Never send any real user's story to a fixture file, the submission video, or a public bug report. No medical diagnosis or medical-record claim.

## 1. Scenario purpose

Demonstrate a meaningful continuous journey across **three dates** without fabricated months of history:
1. Nadia chose to save two original notes and one conversation.
2. She reviews an interpretation and corrects a wrong *simulated* statement.
3. She sees a source-linked timeline and chooses an optional action.
4. She prepares a shareable **personal consultation preparation sheet**.
5. She shares to a fictional care organisation, receives a proposed time and revokes the request or ends access.

This is a fictional user journey, not a case study, user research or evidence of clinical benefit. **All times below are fixtures**, not claimed real user activity.

## 2. Personas and permissions

| Entity | Identity | Access limits |
| --- | --- | --- |
| Nadia | Fictional adult, 20+, guest/demo user | Owns records, approves statements and choices |
| “Klinik Cerita Demo” | **Fictional** partner organisation for sandbox | May receive only selected, current, approved summary via care request |
| “Konselor Demo B” | **Fictional** role in partner workspace | Cannot browse private journal, only scope-bound material |
| Sela operator | Internal admin in synthetic-only environment | May manage partner catalogue but cannot read Nadia's private notes |
| Recipient link session | Simulated reader of one frozen approved share | Link+access code, cannot use after revoke |

No actual mental-health professional credentials, contact links, medical IDs or emergency call actions should be present.

## 3. Three original sources

Use these as **separate owner-saved records**, with stable generated IDs and server-assigned versions. Times must be normalized to UTC in API (local captions in WIB).

**Record N1 — 2026-10-27, saved note**:
> “Sepulang kuliah aku sering capek. Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.”

**Record N2 — 2026-10-30, saved check-in/note**:
> “Hari ini aku agak kewalahan dengan tugas. Aku belum tahu bagian mana yang paling membuatku lelah.”

**Record N3 — 2026-11-01, saved conversation user turn**:
> “Aku ingin tahu bagaimana menjelaskan minggu yang melelahkan ini. Aku juga ingin menyiapkan beberapa pertanyaan kalau suatu saat ngobrol dengan konselor.”

All text is synthetic. These records **do not establish a diagnosis, a cause, a risk score or a real clinical need**. If using existing check-in schema with structured fields, adapt to its actual allowed values without adding unsupported free-text metadata.

## 4. Interpretation fixture and correction

**F0 — SCRIPTED WRONG INTERPRETATION, NOT LIVE META OUTPUT**:
> “Nadia selalu menghindari teman-temannya dan tidak suka bergaul.”

Mark in UI/video “Test fixture: simulated incorrect interpretation” **before** showing it.

**Expected Go Meaning Diff:** frequency shift “kadang”→“selalu”, negation/meaning drift about liking friends, unsupported global behavior. Exact codes depend on current deterministic Meaning Diff; do not forge detection results if code disagrees.

**Nadia's user-corrected wording**:
> “Kadang aku perlu waktu tenang setelah kuliah, tetapi aku tetap menikmati waktu bersama teman. Aku belum tahu penyebab rasa lelahku.”

MIE approval is Nadia's explicit action after reviewing original text, not Wonder/Meta's autonomous decision.

**Expected effect (if applicable graph set up)**: memory depending on rejected claim becomes invalid; approved summary referencing it becomes stale; old share blocks future access; open fictional care request is cancelled; new summary requires source-based editing and approval.

## 5. Optional step Nadia chooses

Manual step (recommended safe P0):
> “Aku akan menulis dua pertanyaan yang ingin aku sampaikan saat berbicara dengan orang yang kupercaya atau konselor.”

State `chosen`. If Action Bridge is not implemented, put this in **existing** consultation summary `prepared.questions` and say it is user-authored preparation, not “Sela Action Bridge launched”.

Optional first question:
> “Bagaimana aku bisa menjelaskan apa yang kurasakan tanpa harus langsung menyimpulkan penyebabnya?”

Optional second question:
> “Apa informasi dari catatan pribadiku yang paling berguna untuk kubawa saat konsultasi?”

No deadline or pressure. If Nadia declines this step, demo still works.

## 6. Care Passport candidate

**Title:** “Hal yang ingin kuceritakan” / “What I want to discuss”.

**Purpose:** “Aku ingin menyiapkan kalimat dan pertanyaan untuk membantu menjelaskan pengalaman pribadiku.”

**Included N1 (quote unchanged):** “Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.”

**Included N2 (quote unchanged):** “Hari ini aku agak kewalahan dengan tugas. Aku belum tahu bagian mana yang paling membuatku lelah.”

**Excluded N3?** If trying to demonstrate selective share, **intentionally exclude** one privately saved record (e.g., N3) from the recipient projection. Ensure no excerpt from excluded record appears in title/items/export/AI prompt for the share.

**Approved item (Nadia's words):**
> “Sesekali aku perlu waktu sendiri setelah kuliah, tetapi aku masih senang bersama teman. Belakangan aku juga merasa kewalahan dengan tugas, dan aku belum tahu penyebabnya.”

Make source refs reference only actual records/versions. Do not attach multi-source paraphrase as an **exact quote**—use individually unchanged source quotes and a separate user-authored summarized sentence that cites both appropriately.

**Prepared questions:** two from `5. **Consent:** sharing is separate and off unless Nadia explicitly permits it. **Recipient:** fictional sandbox. **Status:** approved only after exact source and wording review.

## 7. Controlled sequence A/B

### Sequence A: core working MIE demonstration
1. Seed N1/N2/N3 and a wrong F0 claim as labelled fixture in isolated environment using existing safe test harness.
2. Generate an approved summary referencing F0 and create a synthetic share/partner care request if tests permit.
3. Correct/reject F0. Inspect `/api/v3/meaning/impact` and `/meaning/events`.
4. Confirm old snapshot is denied after invalidation and open care request cancelled.
5. Create a revised, source-true approved summary; share only N1/N2 and revoke on command.
6. Record backend HTTP status and content-free trace ID.

### Sequence B: expanded proposal (only once implementation exists)
1. Open `/notes/context` (new proposed view) and show dates N1/N2/N3.
2. User optionally proposes a change summary, reviews its exact quotes and declines unsupported causality.
3. Create a private action; mark `chosen`; optionally mark `tried` without implying success.
4. Open `/support/passport` preset, select N1/N2, approve and share to simulated recipient.
5. Revise N1 again; confirm action needs_review, passport stale and recipient denied.
6. If any new feature is absent, **stop and switch to existing fallback**, not a static HTML mock advertised as functioning.

## 8. Evidence and privacy pack

- Fixture hash and repo SHA, plus test-mode/provider mode.
- Screenshots of original, fixture-labelled bad claim, correction impact, stale summary, new approved snapshot, share/revoke, partner synthetic badge.
- One API assertion per critical claim (owner scoped; no raw story from live data).
- Recording made only in a clean demo browser with no real passwords, QR secrets, TOTP, active links or share/access codes.
- Teardown removes/invalidates demo sessions and links; retain only intentionally published synthetic artifacts.
- Credits: inherited Dengar.in collaborative provenance and new Sela contributor roles accurately acknowledged.
