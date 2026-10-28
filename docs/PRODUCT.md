# Sela product

**2026-10-28 continuity implementation checkpoint (local, uncommitted; not live):** the incumbent v3 frontend now has explicit manual memory (separate off-by-default consent, encrypted owner records/revisions/export/delete; withdrawal hides normal reads), local Monday–Sunday reflection (creation/check-in dates; saved as an ordinary note), and manual consultation summaries from 1–12 pinned current owned note/check-in/conversation versions. Only user conversation turns are copied; wording and goals/questions/bring are editable. Approval is tied to the exact version and invalidated by wording or source changes; approved .txt download rechecks the server. Follow-ups are real editable/deletable owner records and are removed with their parent summary. Home is compact and task-first; Support reuses the incumbent illustration. `/support/prepare` and `/support/after` open the real summary list. Reading searches four official WHO/NHS links with no referrer; Sela exercise cards remain behind human editorial review. Recipient links/token access/sharing, automatic memory extraction/AI injection and AI summary generation remain unavailable. Local PostgreSQL 16 Go race tests and vet exited 0; 64 frontend tests, typecheck and build passed; the final Docker browser run passed 100/100 checks and 39 axe scans with zero browser errors. The 21 captures comprise 18 baseline visual views plus three alias/sharing views. The independent finish verdict is ship for the three scored fixes; it is not exhaustive backend or clinical approval. See [final continuity evidence](V3_CONTINUITY_VALIDATION.md). This checkpoint changes no normative product goal or clinical/editorial gate and claims no live deployment, user research or clinical approval. Historical planning and lineage below remain as dated. See [design system](../web/DESIGN.md) and [surface contract](../web/.impeccable/surfaces/web-src-v3-app-tsx.md).

Status: **Sela v3 is planned.** No v3 surface is implemented yet; the backend foundation behind them (accounts, guest mode, encrypted storage, consents, export, deletion, backup and restore) is verified locally, and its CI run on draft PR #9 passed, with no screen yet ([SPEC](../SPEC.md) T14, [foundation status note](../SPEC.md#foundation-status-note-2026-10-26)). **The v2 demo journey is implemented and verified locally** in `web/` and `/api/v2` ([below](#v2-demo-journey-implemented-verified-locally)); see [implementation status](IMPLEMENTATION_STATUS.md). Authority: [SPEC](../SPEC.md) §G, C1–C26, I.web, V26–V42. Feature lineage from Dengar.in: [feature matrix](DENGARIN_FEATURE_MATRIX.md). Screen names below follow the [information architecture](design/INFORMATION_ARCHITECTURE.md) (design decision; the design lead may rename screens).

## What Sela is

Sela is a space to feel heard (user-confirmed, owner brief v3). People aged 15 to 50 and over, in Indonesian or English, can:

- talk about what is going on and get replies that fit, without filling in a form first;
- keep personal notes ("Catatan pribadiku"), check in on how they feel and see their own history across sessions;
- see and correct what Sela remembers about them;
- when they want to, prepare a "Ringkasan untuk konsultasi" from what they choose, share it with someone they invite, stop that access, and keep reflecting afterwards.

Listening is the core. Seeking a professional is never required (D-035).

## What Sela is not

- Not therapy, diagnosis, treatment, prescribing, triage or a severity score (C23).
- Not telemedicine or a clinical consultation, and never presented as a psychologist, doctor or health facility (C23).
- Not a medical record: summaries are the user's own notes, prepared with AI help where that applies (C22).
- Not a crisis service: it shows verified services and never contacts anyone on the user's behalf (V38, V39).
- Not a directory or booking service for professionals, and no partnerships are claimed (D-042).
- Not a forum or social network in the pilot (paused, D-038), and not a general-purpose assistant.
- Not an engagement app: no streaks, points, badges, guilt messages or push notifications (C20).
- Not anonymous in an absolute sense: guest mode states what the server stores and sees (C17).
- Not trained on users' notes (D-048), and not clinically validated (B-6).

## The journey (v3, planned)

1. **Arrive and start** in a few steps: language, age band, how storage works. Start as a guest or with a username account.
2. **Talk, write or check in.** Choose whether this conversation is saved.
3. **Continue over time.** Saved notes, check-ins and conversations appear in the timeline; Sela proposes memory items that the user confirms, corrects or rejects.
4. **Prepare, if and when the user wants.** Pick entries for a "Ringkasan untuk konsultasi", review each statement against its source, approve, and practise opening lines.
5. **Share what they choose.** Preview the exact page, pick recipient, purpose and duration, consent, and send the link and access code through their own channel.
6. **Follow the real status.** Prepared, shared, opened, acknowledged, consultation in progress, or revoked, expired, ended. None of these means care was given (V30).
7. **Afterwards.** Write their own reflections and check-ins; revoke or let the share expire.
8. **Any time.** "Butuh bantuan sekarang" and Quick exit on every screen; privacy settings, export and deletion always reachable.

The model is four places — **Beranda** (Home), **Cerita** (Talk), **Catatanku** (My notes: notes, check-ins, timeline, memory) and **Dukungan** (Support: summaries, preparation, reading, support services) — plus the **Privasi & data** menu and a separate **recipient page** ([information architecture](design/INFORMATION_ARCHITECTURE.md)).

## Guarantees on every screen

- **Help and exit:** "Butuh bantuan sekarang" / "Need help now" and Quick exit in the same place on every screen (V38); Quick exit leaves without a back entry and never deletes saved data; an unsent draft in the composer is lost, and an unsaved conversation follows V27; its Esc shortcut can be turned off.
- **Language:** Indonesian and English everywhere, including errors and server-made text; code-switched input works (C2, V13).
- **Accessibility:** WCAG 2.2 AA (C26): semantic HTML, keyboard operation, visible focus that is not obscured, labelled fields with announced errors, contrast, text resize to 200 % and reflow at 320 CSS px, targets of at least 24 px, reduced motion, nothing conveyed by colour alone, plain language, streamed text announced once rather than token by token, and support for small devices and slow connections. Detailed criteria: [accessibility criteria](design/ACCESSIBILITY_CRITERIA.md) (design decision, not yet verified).
- **Screen completeness (brief §4):** every screen has a purpose, one primary action, empty, loading, error and success states, a way to recover, keyboard behaviour and a mobile layout, specified per screen in [screens](design/SCREENS.md) (design decision) and checked by a browser and mobile review with screenshots (T21). A passing build is not evidence that the UI works.
- **Honest labels:** every AI output names its provider (V35); every record item shows its provenance class and review status in plain words (V28); the demo shows a synthetic-data banner (V42).
- **Storage respected:** nothing the user writes is stored before their storage choice (V27).

## Surfaces and their guarantees

All surfaces are **planned**. "Inherits" names the Dengar.in or v2 feature the surface continues ([feature matrix](DENGARIN_FEATURE_MATRIX.md)).

### Onboarding — Pengenalan · Mulai / Introduction · Get started

Purpose: understand what Sela is and is not, and start within minutes. Primary action: start as a guest, create an account or sign in. Inherits FM-01 to FM-06.

- A short, plain explanation of what Sela is and is not comes before any input.
- Only language, age band (asked once, with the reason) and the storage explanation are needed to start; intent and topic are optional (V26).
- The guest notice says where data is stored, that it is deleted after 7 days without activity and at the latest after 30 days, that the browser cookie is the only key, and that this is not absolute anonymity (C17). Phrases such as "anonim sepenuhnya" or "completely anonymous" are never used.
- Accounts use a username that is not a real name (email-, phone- and ID-like names refused) and a passphrase of at least 15 characters; paste and password managers work; there is no CAPTCHA or puzzle (WCAG 3.3.8). Ten recovery codes are shown once, with copy, download and an "I have saved them" step.
- Each consent is separate and names provider, location and purpose: terms and privacy (required); AI processing by the Meta model on Modal (US); the Claude verifier (US); the DeepSeek fallback (PRC; default off); personal memory. Declining AI keeps a no-AI path (C18, D-049).
- Ages 15–17 get age-appropriate copy, the limits of confidentiality explained, and a guardian consent record that gives the guardian no access; this band is off in production until B-10 clears it. "Under 15" stores nothing and shows help information (V37).

### Home — Beranda

Purpose: a calm place to pick up where the user left off. Primary action: talk, write a note or check in. Inherits FM-07 without points or levels.

- Shows an unfinished conversation, recent entries, active shares with their real status and next steps the user chose.
- No counts, scores, streaks, points, levels or badges (C20).
- The empty state invites without pressure; deleted items disappear at once.

### Conversation — Cerita, screen "Ruang cerita" / Talk

Purpose: be heard without judgement. Primary action: write (or, if turned on, dictate) and send. Inherits FM-08, FM-12, FM-13 and FM-15; new as a v3 surface (v2 had no conversation).

- No profile is needed. An optional intent (just listen, sort my thoughts, understand patterns, prepare a consultation) only adjusts the reply style and can be changed (V26).
- The save / don't-save choice is asked at the start in plain language and can be changed (V27). An unsaved conversation is held only in server memory while it is open and is discarded at End, after 30 minutes without activity or after 2 hours at most; closing the tab does not keep it (V27, D-051).
- Replies fit the context, ask at most one question and offer rather than push suggestions; deterministic guards block mechanical empathy templates, judgement, diagnosis, a professional persona, exclusivity ("only I understand you") and dependency cues (V26).
- "Sela menangkap…" / "What Sela picked up…" cards are unconfirmed interpretations that the user edits, confirms or removes (V28).
- One action ends the conversation at any time.
- When AI is unavailable or not consented, Sela says so and the room works as a private writing space; no canned empathy line stands in (V26, V35).
- A crisis match keeps the conversation and the user's words, shows verified help inline, and pauses AI replies until the user chooses to continue (V38).
- An off-topic request gets one kind redirect; distress always overrides it.
- Voice dictation is opt-in, with a disclosure that the browser's speech service may send audio to its vendor and an editable transcript before sending; read-aloud uses local speech synthesis and never autoplays (C25). Both land in milestone P7, after the text room (backlog S-025, S-026).

### Journal — Catatan, "Tulis catatan" / Write a note

Purpose: put thoughts somewhere private. Primary action: write and save. Inherits FM-17 and the reflection part of FM-19.

- Stored only with storage on, encrypted per user (C16).
- Editing keeps the original and each correction as versions (V28); deletion asks for confirmation and removes derived data (V34).
- An entry that matches crisis rules is saved (with storage on) and shown with support, never refused (V38).
- Optional reflection prompts; entries can be picked later as dated sources for a summary.

### Check-in — Cek perasaan / Check-in

Purpose: notice how I am doing. Primary action: choose how I feel; every field is optional. Inherits FM-18. The one-tap mood entry on the welcome screen (FM-01, backlog S-008) comes **after the pilot**.

- Any time, never a daily duty: no streak, reminder or guilt message (C20).
- Several taps on one day update one entry; nothing is stored before the storage choice (V27).
- A note that matches crisis rules is saved with support shown (V38). History, edit and delete.

### Timeline — Linimasa

Purpose: see my own continuity. Primary action: open an item. Inherits FM-22 and FM-23.

- Saved conversations, notes, check-ins, summaries, shares and consultation statuses across sessions, grouped by the user's local day.
- No scores, averages shown as grades, or percentages.
- **After the pilot** (backlog S-053): an optional weekly reflection computed only from that week's real records, labelled as rule-based or AI-written (C20).
- Each item opens its record with provenance; deleted items disappear.

### Memory — Yang Sela ingat / What Sela remembers

Purpose: know and control what Sela keeps in mind. Primary action: review a proposed item. New in v3 (Dengar.in had no cross-session memory).

- Off unless the user consents to personal memory; opt-in only for minors (V29, V37).
- AI proposals are verified before they are shown (V36) and stay unconfirmed until the user confirms, corrects or rejects them (V28).
- Every item shows its source; the user can edit or delete any item or turn memory off; deleting a source removes items derived only from it (V29, V34).
- Memory is used in a conversation only when the user has enabled it for that conversation, and never from conversations that were not saved (V27, V41).

### Consultation summary — Ringkasan untuk konsultasi / Summary for a consultation

Purpose: explain my story to a professional in my own words. Primary action: choose entries and memory items, review, approve. Inherits FM-24 to FM-27 and FM-29, and the v2 Story Ledger with Meaning Lock.

- Built only from what the user selects; each statement cites its source with date, provenance class and review status (V28, V30).
- AI wording is checked by the verifier against the user's words before release; without a permitted verifier Sela keeps the user's own words and says the check was unavailable (V36). Meaning Lock checks run first.
- Never adds a diagnosis, medication, trauma or history that the user did not write (V30). Labelled as the user's own notes; never "rekam medis", "diagnosis" or "hasil pemeriksaan" (C22).
- Every edit clears approval; a changed or deleted source marks the summary stale.
- A manual path without AI produces the same structure; "declined" and "unavailable" are told apart.
- A redaction preview shows what identifying detail leaves before AI processing and before sharing.

### Preparation and follow-up — Persiapan · Setelah konsultasi / Preparation · After the consultation

Purpose: know what to say first and what to bring; keep reflecting afterwards. Primary action: draft opening lines; after the consultation, write a reflection. Inherits FM-28 and v2 Opening Practice.

- Destination types with sourced checklists (source and date shown; media and non-Indonesian sources labelled; re-verified before use, B-5), or "my own professional".
- Opening lines come only from approved summary items (AI or template); rehearsal gets "mentioned / not yet mentioned" feedback, is never scored and is not stored.
- No partnership, booking or verified-professional claim (C22, D-042).
- After the consultation the user writes their own notes and check-ins; nothing is presented as a professional's note (`verified_professional_note` is reserved and never assigned).

### Reading and support — Bacaan & latihan kecil · Layanan bantuan / Reading & small exercises · Support services

Purpose: something useful to read or try now; know who to contact. Primary action: open an item, or call or message a service. Inherits FM-19, FM-30 and FM-32 to FM-34.

- Public content, never copied into a user's history; items can be bookmarked by reference (C21).
- WHO-adapted cards show their attribution and adaptation notice (non-commercial use only, B-14); card text never comes from a model; content claims are verified at authoring time with human sign-off before publication (V36, B-4).
- "Latihan kecil" has no day counter or completion pressure (C20); it lands with the reading library in milestone P6 (backlog S-140, S-141).
- The Immediate help page is one action away from every screen, makes no model call and lists only verified services with source, verification date and re-verify-by date; stale entries are flagged; teen services carry an audience tag (V39).
- Research informs content and product decisions; it is not quoted inside empathic replies (brief §9).

### Privacy and data — Privasi & data

Purpose: control my data. Primary action: change a consent, export or delete. Inherits FM-02, FM-38 and FM-39.

- Every consent is shown separately with provider, location, purpose and policy version, and can be changed; withdrawal stops future processing at once and explains that data already sent to a provider cannot be recalled (C18).
- Consent history, active sessions with revoke and "sign out everywhere", and the access audit (V32).
- Export in readable (HTML, Markdown) and structured (JSON) form (V40).
- Delete single items with their derivatives, or the whole account; the delete screen states the backup windows (no usable key after 7 days, ciphertext gone after 30 days), and the privacy text states the restore window: after a total loss of the database, an account deleted in the hour before it may come back with a restore and need deleting again; a restore also ends every guest's access (the guest purge then removes that guest data), which the guest notice states; every restore is announced in the app and on a status page (planned, backlog S-262; V34, D-055).
- A guest can upgrade to an account and keep everything.
- Display and accessibility settings (text size, contrast, spacing, motion) are reachable before sign-in too.

### Recipient — Halaman penerima / Recipient page

Purpose: for the person the user invited, to read what the user chose to share. Primary action: open with the link and the access code, then acknowledge. Continues the v2 recipient view (the Dengar.in review demo, FM-31, is retired).

- A separate page with its own cookie and access to exactly one share; the link alone never grants access and five wrong codes lock the share (V30).
- Shows only the immutable projection, with provenance labels, and says that the recipient's identity is not verified and that these are the user's own notes, not a medical record (C22).
- The recipient can change only the share status; they cannot write into the user's records.
- Revocation, expiry and deletion end access at once with one uniform message; every view is audited (V31, V32).

## Pilot and public release

Readiness is separate from implementation (owner brief §1, §6). The gates are in [SPEC §G](../SPEC.md#g-goal) and [blockers](BLOCKERS.md#gates).

| | Demo | Controlled pilot | Public release |
| --- | --- | --- | --- |
| Data | Synthetic only, labelled, demo environment (V42) | Real users' data in production | Real users' data in production |
| Ages | All bands, synthetic | `18_plus` only until B-10 clears minors; 15-year-olds blocked until the low-risk assessment | As decided after B-10 |
| AI | Live providers on synthetic data, or labelled test doubles (D-046); every output names its source and a test double is never shown as live (V35). D-046 governs; ADR-0002 §12 and the config guard currently allow only live providers or an honest "unavailable", which is narrower, and are being aligned ([conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 52). The DeepSeek fallback is off by default here, as in hackathon mode (C13, V19) | Meta and the Claude verifier verified live; DeepSeek only after EV-5, B-9 and B-10 | As in the pilot |
| Content | WHO cards with notices | Non-commercial use with notices | Commercial use needs B-14 |
| Gates | Every shown feature really works | B-1, B-3, B-5, B-8, B-9 (if DeepSeek is used), B-10, B-15, owner testing (`OWNER_TESTER.md`, T21) and the restore announcement that `terms-privacy-2026-11-01.2` promises, as the in-app notice and the status-page post ([backlog](BACKLOG.md) S-262 criterion 3, S-263; D-056) | The pilot completed with its findings addressed; consented, ethics-reviewed user research (B-6); independent security and accessibility audits; the minors decision and PP 33/2026 obligations (B-10); more than one instance (S-268); the residual risks accepted for the pilot revisited (D-055); B-13 resolved; B-14 for commercial use ([gates](BLOCKERS.md#gates)) |
| Status today | v2 demo implemented and verified locally; v3 demo planned | Not ready | Not ready |

## v2 demo journey (implemented, verified locally)

Superseded as the product by D-026; kept as the demo configuration and as evidence for SPEC T1–T12. Everything in this journey is synthetic or fictional.

| Stage (ID / EN) | What the user does | What the system guarantees |
| --- | --- | --- |
| Mulai / Start | Reads what Sela is and is not; picks language | Synthetic-demo banner; Indonesian and English everywhere |
| Sebelum mulai / Before you start | Reads privacy and boundaries; anonymous or pseudonym; adult attestation | Versioned privacy notice; no real identity collected |
| Izin / Permissions | Decides AI processing and retention separately | Neither implies the other or sharing; changeable in Settings |
| Ceritaku / My story | Chooses a fictional sample (Indonesian, English, code-switched, crisis test) or writes a fictional practice story | Safety gate before anything else; contact details rejected; memory-only |
| Catatanku / My note | AI draft (streamed, cancelable, retryable) or manual selection; reviews each item: own words vs Sela's wording, qualifiers, notes; edits/removes; answers or skips one clarification; approves version *n* | Only validated claims; every edit creates a new version and clears approval |
| Langkah / Next step | Sets language, format, cost, time and one optional barrier | Deterministic fictional matches with reasons, near matches, honest empty state, non-handoff steps first when "not ready" |
| Latihan / Practice | Drafts opening lines (AI or template), rewrites them, rehearses | Lines grounded in the approved note; feedback is "mentioned / not yet", never a score; rehearsal not stored |
| Tinjau berbagi / Review sharing | Picks fields and duration, sees the exact recipient preview, consents explicitly | Version-bound, recipient-bound, field-minimised projection; idempotent request |
| Perspektif penerima / Recipient (fictional) | Second window: link + access code; acknowledges, asks for clarification, declines or completes a scripted consultation | Only the projection; demo authorization, not real identity |
| Perjalananku / My journey | Sees true status, timeline, expiry; revokes; reads the fixture plan; checks in | "Sent" is never "care received"; revoke blocks future access |
| Pengaturan / Settings | Language, identity, permissions, retention, delete everything | Deletion purges session, shares and recipient access immediately |
| Bantuan segera / Immediate help | Verified Indonesian services with sources and verification dates | Crisis route stops the ordinary journey |
| Riset / Research | Reviewer search of the evidence map | No session data; metadata and annotations only |

Feature lineage: source-grounded Care Passport, clarification and opening-lines logic come from the inherited Dengar.in v1 and were ported and extended (Meaning Lock, versioned approval, recipient handoff). The fictional recipient perspective, share tokens + codes, receipts, follow-up and the Go/Meta runtime are new in v2. See [provenance](SELA_PROVENANCE.md).

v2 exclusions (superseded by C20, C23 and C25): forum, gamification, general chatbot, diagnosis/prescribing/triage, therapy roleplay, payments, real scheduling or professionals, live microphone, health-record integration and model training. In v2 the optional, separately consented Anthropic Sonnet 5.5 source-fidelity review (EvidenceGuard, C12) is a standalone prototype, never a substitute for Meta generation or human approval, and not part of the journey ([contract](CLAUDE_VERIFIER.md)).

In v2 the optional, separately consented DeepSeek-V4.1-Flash-1017 availability fallback (C13) is likewise a standalone, default-OFF prototype in `prototypes/generationfallback`, allowed only after an eligible Meta outage and never for unsafe output; it is not part of the journey, which has no DeepSeek route ([contract](DEEPSEEK_FALLBACK.md)). In v3 it becomes the planned fallback route of the AI router (C18, V35, [ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md)), under the same owner contract (C13, V19–V21).
