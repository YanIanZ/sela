# Sela v3 backlog

Updated: 2026-10-26 · branch `feat/sela-v3-pilot`. Status: **planning document.** The backend foundation is **verified locally, and its CI run on draft PR #9 passed** (commit `0ff3341`; [SPEC §G foundation status note](../SPEC.md#foundation-status-note-2026-10-26)): S-173 and S-260 are verified locally, and the other foundation items have their backend part verified locally (for S-262, criterion 1). The branch is open as draft PR #9 against main. The design and planning documents are in progress; ruleset `-5` is a WIP snapshot in draft PR #8 to main; the frontend (implemented and locally checked on `feat/v3-experience`, 2026-10-28, with blocked surfaces named), the AI provider layer (draft PR #10, stacked on `feat/sela-v3-pilot`), research and reading content (draft PR #11 to main), the P3 listening backend and operations (each on a separate branch, not merged) are in progress. Nothing in v3 is verified live.
**2026-10-28 frontend delivery note.** `feat/v3-experience` implements the v3
frontend screens backed by existing `/api/v3` routes and records the 28 screen
groups, API integrations and blocked states in
[`V3_FRONTEND_VALIDATION.md`](V3_FRONTEND_VALIDATION.md). This note does not
change any item's status: an item stays `in progress` (or planned) until its own
evidence and, for screens, the browser review and release gates are recorded.

Authority: owner brief v3 (2026-10-26) → [SPEC.md](../SPEC.md) → the v3 decisions in the decision log: [D-026](DECISION_LOG.md#d-026) (owner direction), [D-027 to D-050](DECISION_LOG.md#d-027), [D-054](DECISION_LOG.md#d-054), [D-055](DECISION_LOG.md#d-055) and [D-056](DECISION_LOG.md#d-056) (PM decisions of 2026-10-26), [D-051](DECISION_LOG.md#d-051) and [D-052](DECISION_LOG.md#d-052) (engineering defaults) and [D-053](DECISION_LOG.md#d-053) (merge record) → this backlog. The backlog orders and specifies work; it does not change requirements by itself. Related: [Dengar.in feature matrix](DENGARIN_FEATURE_MATRIX.md) (what is kept, merged, retired or paused, and why), [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) (persistence, accounts, encryption), [ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md) (AI routing), [API v3](API_SPEC_V3.md), [implementation status](IMPLEMENTATION_STATUS.md), [requirements traceability](REQUIREMENTS_TRACEABILITY.md), [roadmap](ROADMAP.md), [blockers](BLOCKERS.md).

## 1. How to read this

- **IDs.** `S-NNN`, grouped in blocks by journey stage (S-001 onboarding, S-020 conversation, … S-260 operations). IDs are stable: a retired item keeps its ID and says so; numbers are never reused.
- **Priority.** **pilot-critical**: the controlled pilot cannot start without it (a Definition-of-Done capability of brief §15, a safety control, or a legal or security precondition for holding real health data). **pilot-important**: the pilot can start without it, but it should land during or soon after. **post-pilot**: after the pilot, or paused with stated prerequisites. Every item gives its reason. Totals are in §5. These words replace the earlier labels P0, P1 and P2, which shared their prefix with the milestones; P1 to P8 and `later` always name milestones.
- **Status.** The status words (planned, in progress, implemented, verified locally, verified live, blocked externally, and above them ready for controlled pilot and public release) are defined once in [SPEC §G](../SPEC.md#g-goal) and are not restated here. The checks each level needs are in the [roadmap](ROADMAP.md#2-release-ladder).
- **Evidence** stays `—` until work exists. An item in progress names its files and where they are: committed, uncommitted in the working tree of `feat/sela-v3-pilot`, or on a separate branch that is not merged yet; work in progress is never evidence. An item or part that is verified locally names its recorded run in [implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands).
- **Owner role** names the responsible role, not a person: Go backend engineer, frontend engineer, UX designer, AI engineer, research engineer, security/accessibility reviewer, PM, or the owner (YanIanZ) for external actions.
- **Cross-references.** `FM-`, `CA-`, `DX-`: [feature matrix](DENGARIN_FEATURE_MATRIX.md). `F-`: ADR-0002 acceptance criteria. `AT-`, `RR-`, `VC-`, `CK-`, `DM-`, `EV-`: ADR-0003. `D-`: [decisions](DECISION_LOG.md). `B-`: [blockers](BLOCKERS.md) B-1 to B-15.
- **Port rule.** Dengar.in behaviour is ported from the in-repo v1 code (the same tree as Dengar.in 2.0), never from the original web project, which lacks later fixes (DX-16). Credits stay in [provenance](SELA_PROVENANCE.md).
- **Terms.** "Catatan pribadiku" (My personal notes) and "Ringkasan untuk konsultasi" (Summary for a consultation), never "rekam medis", "diagnosis", "hasil pemeriksaan" or "assessment result" (D-039). Provenance classes and review statuses follow D-040; handoff statuses follow D-041.

## 2. Shared acceptance criteria

Every user-facing item must also meet BASE-1 to BASE-9, BASE-11 and BASE-12; every item that stores user data must meet BASE-10. Items write "BASE" for all that apply. WCAG references are WCAG 2.2 (W3C Recommendation, 12 December 2024). BASE-3 to BASE-7 and BASE-12 summarise the [accessibility criteria](design/ACCESSIBILITY_CRITERIA.md), which give the per-criterion checks, the states to scan and the manual procedures.

| ID | Criterion | Check |
| --- | --- | --- |
| BASE-1 | **Screen contract** (brief §4): the screen's purpose, one primary action, empty, loading, error and success states, a recovery path from each error, keyboard behaviour and the mobile layout are written in the design screen inventory and implemented | Browser review with screenshots at 320 CSS px and desktop, in ID and EN, attached to the PR (S-195) |
| BASE-2 | **ID/EN parity**: every string, error and server-made text exists in both languages; `<html lang>` follows the UI language; text in the other language carries `lang` (3.1.1, 3.1.2) | `web/src/test/i18n.test.ts` and `hardcoded.test.ts` extended to v3; server message-table test (the F-42 pattern) |
| BASE-3 | **Automated WCAG 2.2 AA**: zero axe violations of any impact with tags `wcag2a, wcag2aa, wcag21a, wcag21aa, wcag22aa`, scanned also in dynamic states (error, open dialog, streaming), both themes, at 320 CSS px | e2e axe gate (S-194) |
| BASE-4 | **Keyboard and focus**: every function works by keyboard with no trap; focus is visible and never fully hidden by the sticky header, banners or the on-screen keyboard; a route change sets a unique `document.title` and moves focus to the main heading (2.1.1, 2.1.2, 2.4.2, 2.4.3, 2.4.7, 2.4.11) | e2e keyboard script; manual check (S-195) |
| BASE-5 | **Resize, reflow, spacing, targets**: nothing is lost at 200 % text zoom or at 320 CSS px width; a text-spacing override (line height 1.5, paragraph 2 em, letter 0.12 em, word 0.16 em) clips nothing; pointer targets are at least 24×24 CSS px and Sela's primary controls 44×44 (1.4.4, 1.4.10, 1.4.12, 2.5.8) | e2e overflow, spacing-injection and bounding-box checks (S-194) |
| BASE-6 | **Perception and feedback**: no information by colour alone; text contrast ≥ 4.5:1 (≥ 3:1 for large text) and component contrast ≥ 3:1 in both themes; visible labels; errors in text next to the field with a suggested fix; status messages announced through `role="status"` without moving focus; custom controls expose name, role and state (1.4.1, 1.4.3, 1.4.11, 3.3.1, 3.3.2, 3.3.3, 4.1.2, 4.1.3) | axe; `docs/design/tools/contrast.py` on the tokens; manual screen-reader check |
| BASE-7 | **Motion and sound**: no autoplay audio; nothing flashes more than three times a second; no animation under `prefers-reduced-motion: reduce` (1.4.2, 2.2.2, 2.3.1) | Code review; e2e with reduced motion emulated |
| BASE-8 | **Honest, plain language**: the claim gates (`data/research/corpus/eval_fixtures/claim_gates.json`, extended for v3) find no forbidden term in the copy; no streak, points, level, badge, guilt or pressure copy; the pilot build asks for no notification or push permission (D-037, D-039; SPEC C20; PP 17/2025 Pasal 17) | `tests/research` claim-gate scan over the i18n files; a code scan finds no Notification or Push API use in `web/` |
| BASE-9 | **Help always reachable**: "Butuh bantuan sekarang / Need help now" (the fixed wording of [content style](design/CONTENT_STYLE.md)) and Quick Exit appear in the same relative order on every screen (3.2.6; S-153; SPEC V38) | e2e checks every route |
| BASE-10 | **Ownership and privacy**: every owned record is encrypted with AAD (S-173), cascades from `users` (F-08), is claimed by an exporter (F-39) and answers another user with `404` (F-34); every important record item carries exactly one provenance class and a review status, and the reserved classes `uploaded_document` and `verified_professional_note` are refused (SPEC V28; D-040); logs contain no content (the F-41 log canary) | Go tests per feature; a database constraint test for the reserved classes |
| BASE-11 | **Slow connections and small devices**: usable on a throttled connection with a visible loading state and no lost input; a draft survives a failed request and session expiry within the tab. Drafts are kept in memory only, never in browser storage, and Quick Exit discards them ([design direction §6.5](design/DESIGN_DIRECTION.md#65-conversation-transcript-messages-composer); [safety](SAFETY.md), v3 browser rule; feature matrix DX-04; SPEC C16) (S-196) | e2e with network throttling, a forced failed request and a forced session expiry; after each journey an e2e scan of `localStorage`, `sessionStorage`, IndexedDB and Cache Storage finds no marker content |
| BASE-12 | **Semantic structure** (brief §6, "Semantic HTML"): landmarks (one `header`, a labelled main `nav`, one `main`); one `h1` per view and no skipped heading levels; lists, tables, definition lists and form groups (`fieldset`/`legend`) marked up as such; native elements before ARIA, with ARIA only where no element exists (1.3.1, 4.1.2; [accessibility criteria §4](design/ACCESSIBILITY_CRITERIA.md#4-the-briefs-own-list-6-mapped)) | axe structure rules and the best-practice gates `heading-order`, `page-has-heading-one`, `landmark-one-main`, `landmark-unique`, `region` ([accessibility criteria §6.1](design/ACCESSIBILITY_CRITERIA.md#61-axe-core)); the heading and landmark list read in a screen reader (S-195) |

## 3. Backlog by journey stage

### 3.1 Onboarding and first conversation (S-001 to S-019)

#### S-001 Welcome: what Sela is and is not
- **Problem:** A first-time visitor cannot tell whether Sela is a chatbot, a clinic or a test, or what happens to what they write. v2's copy frames Sela as "a first step towards professional support", not as a listening space.
- **Outcome:** On one screen the visitor learns that Sela listens and helps them reflect and prepare, that it is not a doctor or psychologist and gives no diagnosis, and can start.
- **Priority:** pilot-critical. The brief requires an introduction (§4), and this screen carries the care boundary before any input (§11; UU 17/2023 Pasal 312).
- **Scope:** Public landing page in ID and EN; primary action "Mulai bercerita / Start talking"; links to sign in, privacy and help.
- **Non-goals:** Marketing pages, testimonials, usage counts, invented social proof (DX-07).
- **Depends on:** S-190, S-191, S-153.
- **Acceptance criteria:**
  1. The page says, in ID and EN, that Sela listens and helps prepare help-seeking, is not a professional and does not diagnose; the claim gates pass.
  2. One primary action leads to the start flow (S-002 or S-003).
  3. Nothing is stored and no session is created until the visitor acts (DX-01); a test asserts zero rows after a page load.
  4. BASE.
- **Risks:** Warm copy such as "selalu ada untukmu" invites dependency (D-035).
- **Owner:** UX designer; frontend engineer. **Milestone:** P2. **Status:** planned. **Evidence:** —

#### S-002 Start as a guest with an honest storage notice
- **Problem:** People want to try Sela without a name or email, but need to know where their words are kept and for how long.
- **Outcome:** A guest starts in one step after a short notice that names the location, deletion after 7 days without activity and at most 30 days after creation, the dependence on the browser cookie, and that anonymity is not absolute.
- **Priority:** pilot-critical. Guest mode is the no-profile entry (brief §5) and must state location and duration (brief §10; D-029).
- **Scope:** `POST /api/v3/auth/guest` and the guest lifecycle (ADR-0002 §7); guest notice in ID and EN; purge dates in the profile; a warning from 7 days before the maximum age; the first-visit "do not share identifying details" note folded into the notice (FM-06); the purge job.
- **Non-goals:** Absolute anonymity; device-only storage; email.
- **Depends on:** S-005, S-171, S-173, S-177, S-260.
- **Acceptance criteria:**
  1. ADR-0002 F-30, F-31 and F-33 pass.
  2. The notice shows both deletion dates and the hosting-region label once B-15 is decided, and contains none of "anonim sepenuhnya", "tidak bisa dilacak", "completely anonymous", "untraceable".
  3. The notice explains before start that losing the cookie (private window, cleared data, another device) means the notes cannot be opened again, and that a restore of Sela's database from a backup ends guest access in the same way (`guest-storage-2026-11-01.2`, written in the integration pass, confirmed by the PM, [D-056](DECISION_LOG.md#d-056); ADR-0002 §11).
  4. The notice is announced once, focus moves to its heading, and Esc or Back never loses the visitor's place (2.4.3).
  5. BASE.
- **Risks:** Shared devices; the hosting region is unknown until B-15.
- **Owner:** Go backend engineer (API, purge); UX designer and frontend engineer (notice). **Milestone:** P2. **Status:** in progress (backend verified locally; the notice and the other frontend parts are not done). **Evidence:** backend: ADR-0002 F-30, F-31 and F-33 pass (`internal/account`, `internal/httpapi`, `internal/fixtures`), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); criteria 2 to 5 need the frontend.

#### S-003 Create an account, sign in and recover without email
- **Problem:** People who want to keep notes across devices need an account, Sela has no email provider, and a lost password must not silently mean lost access.
- **Outcome:** Username and passphrase accounts with ten single-use recovery codes, accessible sign-in and secure sessions.
- **Priority:** pilot-critical. Persistent notes, the timeline and export need durable identity (DoD); brief §10 requires authentication, recovery and session security.
- **Scope:** Register, login, logout, logout-all, recovery, reauthentication, password change and code regeneration (API_SPEC_V3 §5; ADR-0002 §6, §8, §9); argon2id; uniform failures; rate limits and backoff; the `__Host-sela_auth` cookie, timeouts and rotation; the forms.
- **Non-goals:** Email or SMS recovery (S-179), social login, passkeys (S-180), CAPTCHA.
- **Depends on:** S-005, S-171, S-173, S-177, S-260.
- **Acceptance criteria:**
  1. ADR-0002 F-16 to F-29 pass.
  2. Paste and password managers work in the username, password and recovery-code fields; there is no CAPTCHA, puzzle or "type characters 3 and 5"; recovery codes can be copied and downloaded; fields use `autocomplete` `username`, `current-password`, `new-password` and `one-time-code` (3.3.8, 1.3.5).
  3. The password rule (15 or more characters, a passphrase suggested) is visible before typing; errors suggest a fix (3.3.2, 3.3.3).
  4. At least 20 seconds before idle expiry the app warns and offers "Tetap masuk / Stay signed in" in one action, and an unsent draft survives expiry (2.2.1).
  5. The copy says plainly that losing both the password and every recovery code means the account cannot be recovered.
  6. BASE.
- **Risks:** Lost credentials are unrecoverable by design; school and campus networks share one address and can hit rate limits (ADR-0002 §9).
- **Owner:** Go backend engineer; frontend engineer; security/accessibility reviewer. **Milestone:** P2. **Status:** in progress (backend verified locally; the forms are not done). **Evidence:** backend: F-16 to F-29 pass (`internal/account`, `internal/httpapi`), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); criteria 2 to 6 need the frontend.

#### S-004 Upgrade a guest to an account without losing anything
- **Problem:** A guest who wants to keep notes beyond 30 days must not have to start over.
- **Outcome:** Upgrading keeps every note and setting under the same user.
- **Priority:** pilot-important. Useful during the pilot; starting as a guest or an account works without it.
- **Scope:** `POST /api/v3/auth/guest/upgrade` (ADR-0002 §7); recovery codes shown once; the UI explains what changes (no automatic deletion).
- **Non-goals:** Merging two existing accounts.
- **Depends on:** S-002, S-003.
- **Acceptance criteria:**
  1. ADR-0002 F-32 passes (per-table counts equal before and after).
  2. The language and age band already given are not asked again (3.3.7).
  3. BASE.
- **Risks:** The user may not save the recovery codes; the acknowledgement step keeps offering them.
- **Owner:** Go backend engineer; frontend engineer. **Milestone:** P2. **Status:** in progress (backend verified locally). **Evidence:** backend: F-32 passes (`internal/httpapi`), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); criteria 2 and 3 need the frontend.

#### S-005 Age band at the start
- **Problem:** Wording, help and protections must fit the person's age, but asking for a birth date collects more than needed.
- **Outcome:** One self-declared question (15–17, 18 or older, prefer not to say, under 15) with its reason. Under 15 sees age-appropriate help and nothing is stored. Production accepts 18+ only until the legal review clears minors (D-031).
- **Priority:** pilot-critical. Needed to apply the production age gate, the protective defaults for `unknown`, and the per-provider age rules (ADR-0003 §8.3).
- **Scope:** ADR-0002 §3 age-band notes; `SELA_ALLOWED_AGE_BANDS`; a yearly re-confirmation for 15–17; correction in settings.
- **Non-goals:** Age verification (S-212); storing a birth date.
- **Depends on:** S-260.
- **Acceptance criteria:**
  1. ADR-0002 F-36 passes; "under 15" leaves zero rows (test).
  2. A band outside the allowed configuration is refused with a plain-language explanation and help links.
  3. No setting, text size or flow is chosen from the age band alone; people aged 50+ get the same defaults as everyone else (brief §6).
  4. BASE.
- **Risks:** Self-declaration is not verification (PP 17/2025 Pasal 22); people may answer untruthfully. The four answers cannot tell 15 from 16–17, which D-031 needs before any minors band is enabled; S-212 criterion 3 closes this ([conflict matrix](CONFLICT_MATRIX.md) #49).
- **Owner:** Go backend engineer; UX designer. **Milestone:** P2. **Status:** in progress (backend verified locally). **Evidence:** backend: F-36 passes (`internal/httpapi`, `internal/account`), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); criteria 2 to 4 need the frontend.

#### S-006 Choose consents during onboarding
- **Problem:** People need to know which company processes their words, where and why, before they write, and must be able to say no and still use Sela.
- **Outcome:** Separate, plain-language choices for storage, Sela's AI (Meta model on Modal, US), the Claude check (Anthropic, US) and the DeepSeek backup (PRC, default off). Declining AI keeps a private writing space.
- **Priority:** pilot-critical. UU PDP Pasal 20–24 require separate, recorded consent per purpose; D-030, D-049. DX-09 (a false claim about who processes text) must not recur.
- **Scope:** The onboarding consent step on the consent API (S-171), with the ADR-0003 §8.1 disclosures; a "decide later" path; consent is never requested during an outage.
- **Non-goals:** Bundled consent; pre-selected boxes; an unequal "No" button.
- **Depends on:** S-171, S-177, S-190.
- **Acceptance criteria:**
  1. Each purpose has its own control, none pre-selected, and "Tidak / No" is as prominent as "Ya / Yes".
  2. Each disclosure names provider, location, purpose, data sent, provider retention, the effect of withdrawal and the user's rights, in ID and EN, and matches the policy version served by `GET /api/v3/policies`.
  3. With every AI consent declined the user still reaches the conversation room as a private writing space (S-024); an e2e test covers it.
  4. No content is written before the storage decision; a test asserts zero content rows after onboarding without storage consent (DX-01).
  5. Every choice can be reviewed and changed later in settings (3.3.4); language and age band are not asked again (3.3.7).
  6. BASE.
- **Risks:** Consent fatigue; long legal text harms understanding. Use a short layered notice with the full text one step away.
- **Owner:** UX designer; frontend engineer; PM (copy). **Milestone:** P2. **Status:** planned. **Evidence:** —

#### S-007 Optional conversation intent
- **Problem:** Some people want to be listened to, others want to sort their thoughts, see patterns or prepare a consultation; one style does not fit all.
- **Outcome:** An optional picker (just listen / sort my thoughts / understand patterns / prepare a consultation) adjusts the reply style and is never a gate (D-035).
- **Priority:** pilot-important. Brief §5 asks for it "if it helps" and forbids making it mandatory; listening works without it.
- **Scope:** Picker at the start of a conversation and changeable during it; the chosen intent sent to AT-1 (ADR-0003 §3.1); "prepare a consultation" offers a link to S-090. The context questions of the retired Dengar.in assessment merge here (FM-05).
- **Non-goals:** Any score, severity band or "professional pathway" label (FM-05 retired).
- **Depends on:** S-020, S-221.
- **Acceptance criteria:**
  1. The first message can be sent without touching the picker (e2e).
  2. The intent can be changed or cleared at any time and is stored with the conversation, encrypted.
  3. "Prepare a consultation" never forces the summary flow.
  4. BASE.
- **Risks:** It may read like an intake form; the design keeps it to one optional row.
- **Owner:** UX designer; AI engineer; frontend engineer. **Milestone:** P3. **Status:** planned. **Evidence:** —

#### S-008 One-tap mood entry from the welcome screen
- **Problem:** Some people want to note how they feel without writing (FM-01).
- **Outcome:** A one-tap mood that becomes a check-in after the storage choice.
- **Priority:** post-pilot. The check-in (S-060) covers the need; the Dengar.in version wrote data before consent (DX-01).
- **Scope:** Mood tap on the welcome or home screen.
- **Non-goals:** Points or rewards.
- **Depends on:** S-060, S-170.
- **Acceptance criteria:**
  1. Nothing is stored before the storage choice (test).
  2. Repeat taps on one day update one entry.
  3. BASE.
- **Risks:** Accidental taps.
- **Owner:** Frontend engineer. **Milestone:** later. **Status:** planned. **Evidence:** —

### 3.2 Listening and the conversation room (S-020 to S-039)

#### S-020 Conversation room: talk, choose whether to keep it, end it
- **Problem:** The core of Sela, telling what is on your mind and being heard, does not exist in v2 (no conversation surface; C9 excluded a chatbot).
- **Outcome:** A person writes, gets a contextual reply, decides whether this conversation is saved, and ends it with one action. Saved conversations can be listed, resumed and deleted.
- **Priority:** pilot-critical. Listening is the core function (brief §1, §5; D-035) and DoD capability 1.
- **Scope:** Conversation and message records (migration range 0100–0199, encrypted, exporter), `/api/v3/conversations` specified in API_SPEC_V3 §9 before implementation, composer and message log ("Ruang cerita / Talk"), the per-conversation storage choice (save or don't save, asked in plain language before the first message is stored, changeable), the bounded in-memory hold for "don't save" turns (D-051; SPEC V27), "Selesai / End", list, resume and delete.
- **Non-goals:** Group chat, voice room (S-027, retired), attachments, therapy role-play, a companion persona.
- **Depends on:** S-002 or S-003, S-150, S-151, S-170, S-171, S-191, S-220, S-221, S-223.
- **Acceptance criteria:**
  1. A new visitor reaches the composer with no profile field other than the age band and the required terms, in at most four screens from the welcome page (e2e counts them).
  2. The storage question appears before the first message is stored. A "don't save" conversation never has a message row (DB test during and after the conversation). Its turns are held only in server memory, bounded in count and size, per owner and conversation, and never reach the database, backups, logs or browser storage. They are purged at End, at sign-out or session end, after 30 minutes without activity, after 2 hours in total and on a server restart (fake-clock and restart tests), and within that window the same session can reopen the conversation after a reload (e2e). It is never used for memory, later context, the timeline or export (DM-4) (D-051; SPEC V27).
  3. "Selesai / End" is one action from the composer, asks nothing, and says what happens to the text under the current storage choice.
  4. Every message is screened (S-150) before storage and before any model call; crisis handling follows S-151.
  5. Deleting a conversation deletes its messages, its reflection cards, the memory items proposed from it and their AI provenance records, and marks summaries that quote it stale (S-090).
  6. Each reply is labelled with its generator, for example "Ditulis dengan bantuan AI (Meta Muse Glimmer)" (ADR-0003 §10).
  7. BASE.
- **Risks:** The room drifts into an intake form; a cold start (B-1) makes the first reply slow, so writing must stay possible while waiting.
- **Owner:** Go backend engineer; frontend engineer; UX designer. **Milestone:** P3. **Status:** in progress (backend, on branch `feat/sela-v3-listening`, uncommitted, not merged). **Evidence:** in progress — `internal/listen`, migration 0100 and the contract draft `docs/api/V3_LISTENING_AND_NOTES.md` on that branch; nothing verified.

#### S-021 Replies that listen: contextual, no judgement, no diagnosis, no dependency
- **Problem:** Generic chatbots repeat empathy templates, push solutions, judge, diagnose, pretend to be professionals or invite dependence.
- **Outcome:** Replies respond to what the person said, ask at most one question, offer suggestions without pushing them, and never diagnose, claim professional status, build an exclusive relationship or encourage dependence.
- **Priority:** pilot-critical. Brief §5 lists these as hard rules; D-035.
- **Scope:** The versioned listening prompt (the Dengar.in reply rules merged, CA-19), the AT-1 data rules (ADR-0003 §3.1), the guards and flags of S-223, a template-repetition check, a kind off-topic redirect that distress always overrides (FM-12), and a synthetic conversation set (ID, EN, code-switched; adult, 50+ and, outside production, 15–17 personas).
- **Non-goals:** Efficacy claims; persona customisation.
- **Depends on:** S-221, S-223. The relevance metric is measured live under S-228 (P5).
- **Acceptance criteria:**
  1. A reply with a diagnosis addressed to the user, a medication or treatment directive, a professional persona, exclusivity ("cuma aku yang ngerti kamu") or a dependency cue is never released (bilingual unit tests).
  2. A reply has at most one question (deterministic check); a violation gets the single same-provider regeneration (RR-5) or the reply is withheld.
  3. A reply whose first sentence matches a listed empathy formula, or repeats the first sentence of an earlier reply in the same conversation, is regenerated once or withheld.
  4. An off-topic request gets one bilingual redirect; distress language always overrides it (tests from FM-12 cases).
  5. A contextual-relevance metric on the synthetic set is pre-registered in `eval/thresholds.json` before the first live run (S-228) and reported as engineering evidence on synthetic data only.
  6. No reply presents seeking a professional as required or urges it when the person has not asked about help; Sela's static help card (S-151) and help control (S-153) are unaffected (SPEC V26). Bilingual guard tests, plus a count on the synthetic set reported with S-228.
- **Risks:** Deterministic checks can flag legitimate text (the user's own quoted words); live quality is unknown until B-1.
- **Owner:** AI engineer; UX designer (voice and tone). **Milestone:** P3. **Status:** planned. **Evidence:** —

#### S-022 Streaming that works with screen readers, and Stop
- **Problem:** Streamed text that re-announces itself token by token is unusable with a screen reader; v2's live panels would do exactly that if reused for chat.
- **Outcome:** Replies are announced once, focus stays in the composer, and the person can stop a reply at any time.
- **Priority:** pilot-critical. Brief §6 names screen-reader-friendly streaming; WCAG 4.1.3 and 2.2.2.
- **Scope:** The twelve streaming criteria of the 2026-10-26 compliance research (§1.4), applied to the conversation room and every streamed AI view.
- **Non-goals:** Token-level live announcements.
- **Depends on:** S-020, S-191.
- **Acceptance criteria:**
  1. The transcript is a `role="log"`; the in-progress reply is not live and is committed to the log as one node when complete, so a three-sentence reply yields one announcement or none, never fragments.
  2. A visually hidden `role="status"` announces "Sela sedang menulis…" at the start and "Balasan selesai" at the end, at most two announcements per reply.
  3. A setting "Bacakan balasan otomatis / Announce replies to screen readers" sets only the transcript's `aria-live` (`polite` when on, `off` when off). It is on by default ([accessibility criteria](design/ACCESSIBILITY_CRITERIA.md) ST-3) and stored on this device with the other display preferences ([design direction §5.9](design/DESIGN_DIRECTION.md#59-display-preferences); S-192). It never produces audio; the app has no speech synthesis (D-060). When it is off, "Baca balasan terakhir / Read last reply" moves focus to the last Sela message.
  4. Focus stays in the composer during and after streaming; a visible, keyboard-reachable "Hentikan balasan / Stop reply" sits next to it; stopped text is kept and marked "Dihentikan / Stopped".
  5. Auto-scroll happens only while the reader is at the bottom; otherwise a "Pesan baru / New message" button appears and is announced once.
  6. Under reduced motion there is no caret or typing animation; text may appear sentence by sentence.
  7. Failures (interrupted, timed out, slow) are announced in text with Retry; crisis help never waits on the stream.
  8. Each message has an author label and time in text, AI text is labelled as AI in text, and a reply in the other language carries `lang`.
  9. Manual check passes with NVDA + Firefox, JAWS + Chrome, VoiceOver + Safari (iOS) and TalkBack + Chrome (Android) (S-195).
- **Risks:** Screen readers differ (`aria-busy` is optional for them), so manual testing cannot be skipped.
- **Owner:** Frontend engineer; security/accessibility reviewer. **Milestone:** P3. **Status:** planned. **Evidence:** —

#### S-023 "Sela menangkap…" cards the person can confirm, correct or remove
- **Problem:** When an AI misunderstands, the person cannot see or fix what it assumed.
- **Outcome:** Sela's understanding is shown as short cards that the person confirms, corrects or removes; only confirmed or corrected cards are used later.
- **Priority:** pilot-critical. Brief §5 ("bisa mengoreksi pemahaman AI"); D-035, D-040.
- **Scope:** AT-2 reflection cards in the conversation; provenance `unconfirmed_interpretation` until reviewed; a correction stored as `user_correction` with the earlier version kept.
- **Non-goals:** Cards that state facts about the person's health.
- **Depends on:** S-020, S-221, S-223.
- **Acceptance criteria:**
  1. Each card says in text that it is Sela's interpretation, not a fact (1.4.1), and points to the passage it is about.
  2. Confirm, correct and remove are one action each and keyboard operable; review status becomes `confirmed`, `corrected` or `rejected`.
  3. Unconfirmed or removed cards never feed memory or summaries (test).
  4. A card whose quote is not in its segment, or that flips negation, subject, time or certainty, is not shown (Meaning Lock checks reused).
  5. Flagged cards are verified or withheld (RR-8, RR-11); no card appears when AI is off or a crisis hold is active.
  6. BASE.
- **Risks:** Too many cards feel like being analysed; at most one card per exchange (UX default, revisable).
- **Owner:** AI engineer; frontend engineer; UX designer. **Milestone:** P3. **Status:** planned. **Evidence:** —

#### S-024 When AI is off or unavailable
- **Problem:** A person who declines AI, or meets an outage, must not lose their words or get a fake reply.
- **Outcome:** Declined AI gives a labelled private writing space; an outage says honestly that Sela cannot reply now, with retry, keep writing and end.
- **Priority:** pilot-critical. Brief §8 (fail safely); D-030 (honest no-AI path); the v2 manual-path principle (V4).
- **Scope:** Conversation states for `ai_processing` declined, provider disabled, kill switch, outage and abstention.
- **Non-goals:** Substitute template empathy lines (D-035).
- **Depends on:** S-020, S-220.
- **Acceptance criteria:**
  1. With AI declined the room says so and stores messages per the storage choice; no provider call is made (test-double call count 0).
  2. With the provider disabled or failing, the message is kept per the storage choice and Retry, keep writing and End are offered; the state is announced (4.1.3).
  3. No canned reply is shown in place of a model reply.
  4. BASE.
- **Risks:** Frequent outages before B-1 make the room feel broken; the copy must not blame the person.
- **Owner:** Frontend engineer; Go backend engineer. **Milestone:** P3. **Status:** planned. **Evidence:** —

#### S-025 Voice dictation (retired)
- **Status:** retired 2026-10-29 (D-060). The v3 app is text-only: no ASR route, `asr_meta` consent, microphone control or provider audio transfer exists. The 2026-10-29 implementation was removed.

#### S-026 Read replies aloud (retired)
- **Status:** retired 2026-10-29 (D-060). No display preference, per-reply control or browser speech synthesis remains.

#### S-027 Voice room (retired)
- **Status:** retired 2026-10-29 (D-060). The Amazon Nova Sonic room, its consent and its realtime route were removed; the text talk room is the only conversation surface.

### 3.3 Personal home (S-040 to S-049)

#### S-040 Personal home
- **Problem:** A returning person has no single place to continue (v2 has no home).
- **Outcome:** A calm home that shows recent conversations and entries, what Sela remembers, active shares and the next actions the person chose.
- **Priority:** pilot-important. Navigation reaches every surface without it, and no DoD capability depends on it, but it makes continuity visible.
- **Scope:** Home after sign-in or guest start; empty state that leads to "Mulai bercerita"; links to the timeline, memory and shares.
- **Non-goals:** Scores, streaks, points, levels, badges, "you have not written for N days" (D-037; FM-07, FM-21 retired).
- **Depends on:** S-020, S-050, S-070, S-080, S-191. Active shares appear once S-101 lands (P4).
- **Acceptance criteria:**
  1. Every block has empty, loading and error states; the empty home has one primary action.
  2. A copy test finds no streak, score or absence message.
  3. The timeline is reachable from the home and from the navigation (2.4.5).
  4. BASE.
- **Risks:** A dashboard feel; keep it to a few blocks.
- **Owner:** UX designer; frontend engineer. **Milestone:** P3. **Status:** planned. **Evidence:** —

#### S-041 Optional neutral reminders
- **Problem:** Some people want a gentle reminder to check in.
- **Outcome:** Reminders only if the person turns them on, with neutral wording.
- **Priority:** post-pilot. Not in the pilot (D-037).
- **Scope:** A reminder time and channel to be decided (no push notifications in the pilot).
- **Non-goals:** Guilt messages, streaks, default-on reminders.
- **Depends on:** S-060.
- **Acceptance criteria:**
  1. Off by default; one action turns it off.
  2. A copy test finds no guilt or pressure wording.
- **Risks:** Notifications on a shared phone reveal that Sela is used.
- **Owner:** UX designer. **Milestone:** later. **Status:** planned. **Evidence:** —

### 3.4 Journal and reflection (S-050 to S-059)

#### S-050 Notes (journal, "Tulis catatan") inside Catatan pribadiku
- **Problem:** People need a private place to write that keeps their notes across sessions. The design calls a journal entry a "Catatan" (write: "Tulis catatan") and the place that holds every kept record "Catatan pribadiku" ([information architecture](design/INFORMATION_ARCHITECTURE.md); D-039). Dengar.in kept the journal in plaintext browser storage (DX-04) and refused to save heavy entries (DX-02).
- **Outcome:** Journal entries that are created, edited and deleted, kept server-side and encrypted, with every edit kept as a version.
- **Priority:** pilot-critical. DoD capabilities 3 and 9 (see and correct personal context; continue reflecting after a consultation) rest on saved entries.
- **Scope:** Journal records (migration range 0200–0299), `/api/v3/journal` specified first; the original text (`user_original`) and each correction (`user_correction`) as versions; exporter section.
- **Non-goals:** Word counts, streaks, sharing from the journal directly (sharing goes through S-090).
- **Depends on:** S-002 or S-003, S-150, S-151, S-170, S-173, S-260.
- **Acceptance criteria:**
  1. Entries are stored only when storage is on.
  2. An entry that matches crisis rules is saved (when storage is on) and support is shown; a test covers it (DX-02).
  3. Editing keeps the earlier version; the entry shows its created and updated times.
  4. Deleting an entry deletes the reflections and memory items derived from it and marks summaries that quote it stale.
  5. BASE.
- **Risks:** People may treat the journal as a medical record; the copy says these are their own notes (D-039).
- **Owner:** Go backend engineer; frontend engineer. **Milestone:** P3. **Status:** in progress (backend, on branch `feat/sela-v3-listening`, uncommitted, not merged). **Evidence:** in progress — `internal/notes` and migration 0200 on that branch; nothing verified.

#### S-051 Optional reflection prompts
- **Problem:** A blank page is hard to start.
- **Outcome:** Optional prompts in ID and EN that never block writing.
- **Priority:** pilot-important. Helpful, not required for the DoD.
- **Scope:** A small prompt set, sourced (S-244) or labelled team-written and reviewed; no AI needed.
- **Non-goals:** Daily prompt duties or counters.
- **Depends on:** S-050.
- **Acceptance criteria:**
  1. An entry can be saved without a prompt.
  2. Each prompt shows its source or the team-written label.
  3. BASE.
- **Risks:** Prompts that steer people toward heavy topics; review them with the safety owner.
- **Owner:** UX designer; research engineer. **Milestone:** P3. **Status:** planned. **Evidence:** —

#### S-052 Reflect on an entry with Sela
- **Problem:** People want help understanding what they wrote.
- **Outcome:** On request, Sela shows "Sela menangkap…" cards for a journal entry (AT-2), with the same review controls as S-023.
- **Priority:** pilot-important. The journal works without AI.
- **Scope:** AT-2 over journal segments only.
- **Non-goals:** Automatic analysis of every entry.
- **Depends on:** S-023, S-050.
- **Acceptance criteria:**
  1. Runs only when the person asks and `ai_processing` is granted.
  2. S-023 criteria 1 to 5 hold for journal cards.
- **Risks:** Over-interpretation; the verifier and guards apply.
- **Owner:** AI engineer; frontend engineer. **Milestone:** P3. **Status:** planned. **Evidence:** —

#### S-053 Weekly reflection
- **Problem:** People want to look back on a week (FM-23).
- **Outcome:** An optional weekly view computed from that week's real records.
- **Priority:** post-pilot. The timeline covers looking back; Dengar.in's version invented numbers (DX-11).
- **Scope:** A rule-based summary, labelled as such.
- **Non-goals:** Consistency percentages, scores.
- **Depends on:** S-070.
- **Acceptance criteria:**
  1. Built only from that week's records (test with records outside the week).
  2. ID and EN; no percentage.
- **Risks:** Reads as an assessment.
- **Owner:** Frontend engineer. **Milestone:** later. **Status:** planned. **Evidence:** —

### 3.5 Check-in (S-060 to S-069)

#### S-060 Check in any time
- **Problem:** People want to note how they are doing quickly; Dengar.in tied it to a daily routine and points, and v2 only offers check-ins after a simulated consultation.
- **Outcome:** A standalone check-in (mood words, energy, optional tags, optional note) with history, edit and delete.
- **Priority:** pilot-important. Part of the brief's journey (§4), but the journal covers reflection and no DoD capability depends on it.
- **Scope:** Check-in records (migration range 0200–0299); history in the timeline.
- **Non-goals:** Daily duty, streaks, reminders, scores.
- **Depends on:** S-002 or S-003, S-150, S-151, S-170, S-260.
- **Acceptance criteria:**
  1. Every field is optional and a check-in can be saved with one choice.
  2. Mood options are text plus icon, never colour alone (1.4.1); any scale has buttons or steppers, no drag-only slider (2.5.7).
  3. A crisis-matching note is saved (when storage is on) with support shown.
  4. Edit and delete work; deleted check-ins leave the timeline at once.
  5. BASE.
- **Risks:** Mood charts read as a diagnosis; show history as a list first, any chart with a text equivalent (1.1.1).
- **Owner:** Frontend engineer; Go backend engineer. **Milestone:** P3. **Status:** in progress (backend, on branch `feat/sela-v3-listening`, uncommitted, not merged). **Evidence:** in progress — migration 0200 (journal and check-in) and `internal/notes` on that branch; nothing verified.

### 3.6 Timeline (S-070 to S-079)

#### S-070 Timeline across sessions
- **Problem:** People cannot see their own history across conversations, entries and consultations; v2 has only a per-handoff timeline.
- **Outcome:** One timeline of saved conversations, journal entries, check-ins, summaries, shares and their status changes, and after-consultation reflections, grouped by the person's local day.
- **Priority:** pilot-critical. DoD capability 4; brief §7 requires a timeline across sessions.
- **Scope:** `/api/v3/timeline`, a read model over the person's own records; each item opens its record with provenance class, time and version. Check-ins, summaries, shares and after-consultation reflections join the timeline as their items land (P3, P4).
- **Non-goals:** Scores; server-side search over content (ADR-0002 §5).
- **Depends on:** S-020, S-050.
- **Acceptance criteria:**
  1. Items appear grouped by local day with the date in text; the timeline is a list (1.3.1).
  2. A deleted record disappears from the timeline at once (test).
  3. Only the owner's records appear (cross-user test).
  4. Paginated; keyboard navigable; reachable from the home and the navigation (2.4.5).
  5. BASE.
- **Risks:** Long histories on slow connections; paginate (BASE-11).
- **Owner:** Go backend engineer; frontend engineer. **Milestone:** P3. **Status:** in progress (backend, on branch `feat/sela-v3-listening`, uncommitted, not merged). **Evidence:** in progress — `internal/timeline` on that branch; nothing verified.

#### S-071 Find things in my timeline
- **Problem:** Long histories are hard to scan.
- **Outcome:** Filters by type and date, and text search that runs on the person's own decrypted records.
- **Priority:** pilot-important. Useful once histories grow; not needed on day one.
- **Scope:** Client-side or per-user search after decryption.
- **Non-goals:** Server-side content indexes.
- **Depends on:** S-070.
- **Acceptance criteria:**
  1. Filters and search change no data and are announced as status ("12 hasil").
  2. BASE.
- **Risks:** Performance on low-end phones.
- **Owner:** Frontend engineer. **Milestone:** P3. **Status:** planned. **Evidence:** —

### 3.7 Memory: "what Sela remembers" (S-080 to S-089)

#### S-080 See and correct what Sela remembers
- **Problem:** AI memory is usually invisible and cannot be fixed; Dengar.in had no cross-session memory and v2 forgets everything with each new story.
- **Outcome:** A plain list of what Sela remembers, each item with its source, time, version and review status, which the person can add to, confirm, correct or forget, and a per-conversation switch to use memory or not.
- **Priority:** pilot-critical. Brief §7 requires transparent, correctable memory; DoD capability 3.
- **Scope:** Memory items (migration range 0300–0399), `/api/v3/memory`, the `personal_memory` consent, manual items (typed or taken from a confirmed card), up to 5 confirmed items sent to AT-1 when enabled (ADR-0003 §3.1).
- **Non-goals:** Hidden profiles; inferred diagnoses; memory from unsaved conversations.
- **Depends on:** S-020, S-023, S-050, S-171, S-260.
- **Acceptance criteria:**
  1. Every item shows its source (a link to the record), provenance class, created and updated time, version and review status (D-040).
  2. Correcting keeps the previous version; forgetting deletes the item and the next reply request no longer contains it (test on the captured request).
  3. Without `personal_memory` nothing is stored as memory or sent; withdrawal stops use at once.
  4. Items from "don't save" conversations never appear (DM-4).
  5. For ages 15–17 memory is opt-in only (S-210).
  6. BASE.
- **Risks:** People feel watched; the screen explains what memory is for and that it is off unless chosen.
- **Owner:** Go backend engineer; frontend engineer; UX designer. **Milestone:** P3. **Status:** in progress (backend, on branch `feat/sela-v3-listening`, uncommitted, not merged). **Evidence:** in progress — `internal/memory` and migration 0300 on that branch; nothing verified.

#### S-081 Memory suggestions from Sela
- **Problem:** Adding memory by hand is effort.
- **Outcome:** Sela proposes memory items from saved entries the person allowed (AT-3); nothing is kept until the person confirms.
- **Priority:** pilot-important. Manual memory (S-080) meets the DoD; proposals need the verifier (S-222), whose live use is blocked (B-8).
- **Scope:** AT-3 with mandatory verification; proposals stay `unreviewed` until the person acts.
- **Non-goals:** Automatic memory without review.
- **Depends on:** S-080, S-222, S-226.
- **Acceptance criteria:**
  1. Every proposal passed the verifier, or none is shown (RR-11; test with the verifier unavailable).
  2. A proposal cites the passage it came from.
  3. Minors get proposals only after opting in.
  4. BASE.
- **Risks:** Profiling concerns for minors (PP 17/2025 Pasal 19).
- **Owner:** AI engineer; frontend engineer. **Milestone:** P5. **Status:** planned. **Evidence:** —

### 3.8 Consultation summary (S-090 to S-099)

#### S-090 Build a "Ringkasan untuk konsultasi"
- **Problem:** Before seeing a professional, people struggle to say what matters, and AI summaries can invent diagnoses or history.
- **Outcome:** The person picks entries, messages, check-ins and memory items; the summary shows each statement with its source quote, date, provenance class and review status; the person approves a version. Works fully without AI.
- **Priority:** pilot-critical. DoD capability 5; brief §7.
- **Scope:** Summary records (migration range 0400–0499); a dated-evidence picker (FM-25); manual wording; Meaning Lock on any AI wording; approval bound to version and hash and invalidated by any edit (v2 rules reused); a stale marker when a source changes or is deleted.
- **Non-goals:** Diagnosis, medication lists, inferred trauma or history; uploads (S-182); the words "rekam medis", "diagnosis", "hasil pemeriksaan".
- **Depends on:** S-020, S-050, S-080, S-260.
- **Acceptance criteria:**
  1. The summary contains only selected items; a test inserts unselected records and checks they are absent from the summary and its share projection.
  2. Each statement shows its source quote, date and provenance class in text.
  3. With AI off, a complete summary can be built and approved (e2e).
  4. Any edit clears approval and a share needs the approved version (the v2 `TestAIDraftNeedsReviewEditInvalidatesApproval` behaviour ported).
  5. The header says it is the person's own notes, prepared with AI help where used; the claim gates pass.
  6. A changed or deleted source marks the summary stale, and its shares end or need re-approval (test).
  7. Items already chosen are never typed again in later steps (3.3.7).
  8. BASE.
- **Risks:** Professionals may still read it as clinical; the recipient view repeats the label (S-120).
- **Owner:** Go backend engineer; frontend engineer; UX designer. **Milestone:** P4. **Status:** planned. **Evidence:** —

#### S-091 AI wording for summary items, checked before release
- **Problem:** Turning raw notes into clear sentences is hard; unchecked AI wording can distort them.
- **Outcome:** Sela suggests wording for each selected item (AT-4); only wording the verifier finds supported is shown, otherwise the person's own quote stays.
- **Priority:** pilot-important. The manual summary meets the DoD; AI wording is an aid whose release depends on the verifier (live use blocked, B-8).
- **Scope:** AT-4; VC-8 release rules; per-verdict explanations; "Detail pemeriksaan / Check details" labelled as an automated check by the named provider (VC-9); abstain notice.
- **Non-goals:** Releasing unverified wording in any environment (ADR-0003 §15).
- **Depends on:** S-090, S-222, S-226.
- **Acceptance criteria:**
  1. With the verifier unavailable or not consented, no AI wording is released and the notice says why (test).
  2. `unsupported`, `contradicted` and `insufficient_evidence` fall back to the quote with Sela's fixed explanation (test per verdict).
  3. The verifier's reason is shown only under "Detail pemeriksaan" and never as advice.
  4. BASE.
- **Risks:** A high false-rejection rate makes the feature useless (threshold `verifier.false_rejection_rate`).
- **Owner:** AI engineer; frontend engineer. **Milestone:** P5. **Status:** planned. **Evidence:** —

#### S-092 Print or download one summary
- **Problem:** Many consultations (for example at a Puskesmas) happen without any link-sharing; people bring paper or a phone screen.
- **Outcome:** A printable and downloadable readable copy of an approved summary.
- **Priority:** pilot-important. Sharing (S-100, S-101) and the full export (S-175) cover the DoD.
- **Scope:** HTML and print stylesheet, Markdown; user text escaped (the Dengar.in export-injection fix, FM-29); an audit event.
- **Non-goals:** PDF generation on the server.
- **Depends on:** S-090, S-176.
- **Acceptance criteria:**
  1. User text with Markdown or HTML syntax is rendered as text (injection test).
  2. The copy carries the "own notes" label and the date, and says a downloaded copy cannot be recalled.
  3. BASE.
- **Risks:** Printed copies left behind; the copy suggests care.
- **Owner:** Frontend engineer. **Milestone:** P4. **Status:** planned. **Evidence:** —

### 3.9 Preparation and sharing (S-100 to S-119)

#### S-100 Preview and consent before sharing
- **Problem:** People must see exactly what a professional will see and decide who gets it and why.
- **Outcome:** A preview that equals what the recipient will see, and a consent that names the content, the recipient and the purpose.
- **Priority:** pilot-critical. DoD capability 6; brief §7 (preview; consent by content, recipient and purpose).
- **Scope:** Choose items or fields, type a recipient label (the person's own professional; identity not verified, D-042), pick a purpose from a fixed list and a duration; explicit consent with the policy version; v2 handoff code reused with a purpose added.
- **Non-goals:** A professional directory, booking, partnership claims (D-042).
- **Depends on:** S-090, S-171.
- **Acceptance criteria:**
  1. The preview equals the stored projection exactly (the v2 `TestFullJourneyOverHTTP` assertion ported).
  2. The consent record names item ids and version, recipient label, purpose and expiry; a share missing any of them is refused.
  3. The recipient label shows "identitas belum diverifikasi / identity not verified".
  4. A final review step offers Back and Confirm (3.3.4); nothing is typed twice (3.3.7).
  5. BASE.
- **Risks:** People share more than needed; default to the minimum fields.
- **Owner:** Go backend engineer; frontend engineer; UX designer. **Milestone:** P4. **Status:** planned. **Evidence:** —

#### S-101 Share link, access code, expiry, revocation and status
- **Problem:** Once shared, people need to stop further access and know what happened to the share, without mistaking a sent note for care received.
- **Outcome:** A link plus a separate access code, an expiry, one-action revocation, and statuses prepared → shared → opened → acknowledged → consultation in progress, plus revoked, expired and ended (D-041).
- **Priority:** pilot-critical. DoD capability 7; brief §7 (expiry, revocation, access audit, handoff statuses).
- **Scope:** Share records and recipient principals (migration range 0400–0499); token and 8-character code; lockout; durations; revocation; the status machine; an audit event for every recipient open (S-176).
- **Non-goals:** Recalling copies already viewed or downloaded.
- **Depends on:** S-100, S-120, S-176.
- **Acceptance criteria:**
  1. The token alone never grants access (v2 `TestTokenAloneIsNotEnoughAndLockout` ported).
  2. Revocation, expiry, account deletion and source deletion end recipient access at once, including open recipient sessions (tests, including a race).
  3. The UI and the export say in ID and EN that revocation stops further access and cannot recall copies already viewed or downloaded.
  4. Each status is shown as text (1.4.1); no status says or implies that treatment was given (claim gates).
  5. Every recipient open writes an audit event the owner can see.
  6. The token and code are shown to the owner once or stored encrypted; a database scan finds neither in plaintext.
  7. Share creation is idempotent under concurrency (v2 `TestIdempotentHandoffUnderConcurrency` ported).
  8. BASE.
- **Risks:** The access code must travel by a separate channel the person chooses; the copy explains why.
- **Owner:** Go backend engineer; frontend engineer. **Milestone:** P4. **Status:** planned. **Evidence:** —

#### S-102 Opening lines for a consultation
- **Problem:** Knowing what to say first is hard.
- **Outcome:** A few opening lines grounded in the approved summary (AT-5), with labelled template lines when AI is off.
- **Priority:** pilot-important. Helps preparation; not a DoD capability.
- **Scope:** v2 Opening Practice reused (pressure, number and identifier checks; rehearsal not stored and not scored).
- **Non-goals:** Scripts that pressure the professional; scoring the person.
- **Depends on:** S-090, S-221, S-223.
- **Acceptance criteria:**
  1. Lines use only approved summary items (v2 `TestValidateOpening` cases ported).
  2. The template path works with AI off.
  3. BASE.
- **Risks:** Lines that sound unnatural in Indonesian; review with fluent speakers.
- **Owner:** AI engineer; frontend engineer. **Milestone:** P4. **Status:** planned. **Evidence:** —

#### S-103 What to bring and what to expect, per destination
- **Problem:** People do not know what a Puskesmas, a psychologist, Healing119 or a campus service will ask or offer.
- **Outcome:** Short checklists per destination type, plus "my own professional", each with its source and date.
- **Priority:** pilot-important. Preparation aid; its sources need re-verification first (B-5).
- **Scope:** The Dengar.in checklists (CA-03) re-verified; media and non-Indonesian sources labelled.
- **Non-goals:** Partnership, booking or verified-professional claims.
- **Depends on:** S-152.
- **Acceptance criteria:**
  1. Each checklist shows its source and verification date; stale entries are flagged (the S-152 rule); every checklist is re-verified with the help lines (S-154) before real users see it.
  2. Claim gates pass.
  3. BASE.
- **Risks:** Service details change.
- **Owner:** Research engineer; UX designer. **Milestone:** P4. **Status:** planned. **Evidence:** —

#### S-104 Mark names as private and preview what is removed
- **Problem:** People mention other people and identifying details they do not want to leave Sela.
- **Outcome:** A segment-level preview of what is removed before AI processing and before sharing; names the person marks private are replaced.
- **Priority:** pilot-important. Contact details are always removed from provider requests by S-226 (pilot-critical); this adds the person's own choices and the preview.
- **Scope:** The Go port of the Dengar.in PII redactor (FM-26) shared with S-226; placeholders such as `[NAMA-1]` mapped back only in the owner's view.
- **Non-goals:** Promising perfect de-identification.
- **Depends on:** S-090, S-226.
- **Acceptance criteria:**
  1. Phone numbers, NIK, email and card numbers are always removed from shares and provider requests (tests).
  2. A name marked private appears in no provider request and, if chosen, in no share projection (test on captured requests).
  3. BASE.
- **Risks:** Missed names give false confidence; the copy says detection is imperfect.
- **Owner:** Go backend engineer; frontend engineer. **Milestone:** P4. **Status:** planned. **Evidence:** —

### 3.10 Recipient experience (S-120 to S-129)

#### S-120 What the recipient sees
- **Problem:** A professional the person invites needs a clear, trustworthy view without an account, and must not mistake it for a medical record.
- **Outcome:** After entering the access code, the recipient sees the immutable shared summary, its purpose and expiry, and clear labels.
- **Priority:** pilot-critical. Sharing (DoD 6) needs a recipient side.
- **Scope:** `/api/v3/recipient` with a separate principal and cookie (`__Secure-sela_recipient`, path `/api/v3/recipient`; ADR-0002 §10); code entry; uniform "access ended" state.
- **Non-goals:** Recipient accounts, messaging, uploads by the recipient.
- **Depends on:** S-101.
- **Acceptance criteria:**
  1. Labels say: identity not verified; the person's own notes, prepared with AI help where used; not a medical record or examination result.
  2. A recipient principal cannot call owner routes and an owner session cannot call recipient routes (tests).
  3. Expired, revoked and deleted shares give the same "access ended" view.
  4. Works on a phone without an account; no third-party scripts or trackers.
  5. BASE.
- **Risks:** Forwarded links; the separate code limits this.
- **Owner:** Go backend engineer; frontend engineer. **Milestone:** P4. **Status:** planned. **Evidence:** —

#### S-121 Acknowledgement and consultation status
- **Problem:** The person wants to know the summary arrived, without implying care.
- **Outcome:** The recipient can acknowledge; the person marks when a consultation is in progress or ended.
- **Priority:** pilot-important. Opened and shared statuses already give control; this adds continuity.
- **Scope:** Recipient "acknowledge" action; owner "consultation in progress / ended" action; audit events.
- **Non-goals:** A "care completed" status; recipient-written clinical notes.
- **Depends on:** S-101, S-120.
- **Acceptance criteria:**
  1. Acknowledgement is one action and is audited.
  2. No status text implies treatment (claim gates).
  3. BASE.
- **Risks:** Recipients who never acknowledge; the person sees "opened" instead.
- **Owner:** Go backend engineer; frontend engineer. **Milestone:** P4. **Status:** planned. **Evidence:** —

#### S-122 Verified professionals, booking and professional notes (later)
- **Problem:** People may want a verified professional or a note written by one.
- **Outcome:** Only after the prerequisites exist.
- **Priority:** post-pilot. No verified professionals, partnerships or booking exist, and claiming them is forbidden (brief §7; D-042).
- **Scope (prerequisites):** a credential-verification process; a health-facility (Fasyankes) partner and PSE registration (UU 17/2023 Pasal 172); legal review; consent design for professional-written notes; activating the reserved `verified_professional_note` provenance class (D-040).
- **Non-goals:** Any of this in the pilot.
- **Depends on:** S-178.
- **Acceptance criteria:** Defined when opened.
- **Risks:** Telemedicine regulation (PP 28/2024 Pasal 558).
- **Owner:** PM; owner. **Milestone:** later. **Status:** planned. **Evidence:** —

### 3.11 Follow-up after a consultation (S-130 to S-139)

#### S-130 Reflect after a consultation
- **Problem:** After seeing a professional, people forget what was said and lose the thread; v2's follow-up is fixture text after a simulated consultation.
- **Outcome:** An "after the consultation" entry, linked to the summary or share if there was one, that keeps what the person wants to remember and continues in the journal.
- **Priority:** pilot-critical. DoD capability 9.
- **Scope:** A journal entry type with optional prompts (what was said, what I want to remember, questions for next time), linked to a summary or share, or standing alone.
- **Non-goals:** Prescribed plans; recording what the professional decided as fact; implying care was received.
- **Depends on:** S-050, S-070; links to S-101 when used.
- **Acceptance criteria:**
  1. It can be created from a share, from the timeline or from the journal, with or without a share.
  2. Entries are `user_original`; nothing presents them as professional notes.
  3. A copy test finds no wording that implies treatment was given.
  4. BASE.
- **Risks:** People record advice incorrectly; the label says these are their own notes.
- **Owner:** UX designer; frontend engineer; Go backend engineer. **Milestone:** P4. **Status:** planned. **Evidence:** —

#### S-131 My own next steps
- **Problem:** People want to keep track of what they decided to do.
- **Outcome:** A short, person-written list of next steps, optionally linked to reading items.
- **Priority:** pilot-important. Supports continuity; not a DoD capability.
- **Scope:** User-authored steps; fixture plans stay in the demo only (v2 V7).
- **Non-goals:** Day counters, completion pressure, prescribed paths (FM-20 retired).
- **Depends on:** S-130.
- **Acceptance criteria:**
  1. Steps are written by the person; production serves no fixture plan (config test).
  2. BASE.
- **Risks:** Becomes a to-do app; keep it minimal.
- **Owner:** Frontend engineer. **Milestone:** P4. **Status:** planned. **Evidence:** —

### 3.12 Reading and support (S-140 to S-149)

#### S-140 Reading library
- **Problem:** People want something useful to read or try, from trustworthy sources.
- **Outcome:** A browsable ID/EN library built from the 30 self-help cards (FM-34), with sources and licence notices.
- **Priority:** pilot-important. Support content helps, but the pilot's core is listening and continuity; urgent needs are covered by S-152 (pilot-critical).
- **Scope:** `/api/v3/reading` (migration range 0600–0699); the 22 WHO cards with attribution and adaptation notice (CC BY-NC-SA 3.0 IGO), Kemenkes and CDC attribution, the safety-plan structure without reproduced text; the malformed WHO URL corrected; bookmarks by reference (D-033).
- **Non-goals:** Commercial use (blocked until licensed or replaced, B-14, D-045); copying an article into a person's history; academic citations inside empathic replies (brief §9).
- **Depends on:** S-191, S-245.
- **Acceptance criteria:**
  1. Every WHO card shows the attribution and adaptation notice; a test checks every card has licence fields.
  2. Model output can reference cards only by ID; card text never comes from a model (test).
  3. Each item is verified at authoring time (S-245) or stays unpublished.
  4. BASE.
- **Risks:** The non-commercial licence limits later business models.
- **Owner:** Research engineer; UX designer; frontend engineer. **Milestone:** P6. **Status:** in progress (content, committed on branch `feat/sela-v3-research` as `64aa3f6`, draft PR #11 to main, not merged). **Evidence:** in progress — `data/content/reading`, `docs/content/READING_LIBRARY.md` and `tests/content` on that branch; its 79 content tests passed locally on 2026-10-26 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); nothing verified while the work is not on the delivery branch.

#### S-141 Small exercises ("latihan kecil")
- **Problem:** Some people want a small, safe step to try (FM-19).
- **Outcome:** Optional exercises inside Reading & support without day counters.
- **Priority:** pilot-important. D-038 merges the Dengar.in daily micro-missions into Reading & support as optional "latihan kecil", and SPEC T19 places them in P6 with the reading library. The pilot can start without them; the Dengar.in templates have no cited source (CA-05), so each exercise is sourced or reviewed before it is shown.
- **Scope:** Exercises that are sourced or reviewed and labelled team-written, shown in Reading & support next to the library (S-140).
- **Non-goals:** "Day N of 14", streaks, completion pressure (D-037).
- **Depends on:** S-140, S-245.
- **Acceptance criteria:**
  1. Each exercise has a source or the team-written label, and is verified at authoring like a reading item (S-245) or stays unpublished.
  2. A reflection is saved to the notes only when storage is on.
  3. A copy test finds no day counter, streak or completion-pressure wording (claim gates).
  4. BASE.
- **Risks:** Exercises presented as treatment.
- **Owner:** Research engineer; UX designer. **Milestone:** P6. **Status:** planned. **Evidence:** —

#### S-142 Community forum "Ruang Cerita" (paused)
- **Problem:** Feeling less alone by reading others' experiences (FM-36, FM-37).
- **Outcome:** Reopened only when its prerequisites are met.
- **Priority:** post-pilot, paused (D-038): no moderation operations, authors cannot delete (DX-05), invented seed stories shown as real (DX-07), "100% anonim" copy (DX-10).
- **Scope (prerequisites to reopen):** moderation operations and an operator UI with an audit trail; author deletion; a minors policy; examples labelled as examples; no link to any personal history (D-033); a crisis gate on every post; a name distinct from the private conversation room "Ruang cerita" (D-035), so a public community space is never confused with personal content (C21).
- **Non-goals:** Porting the Dengar.in forum as it is.
- **Depends on:** S-150, S-178.
- **Acceptance criteria:** Defined when reopened; at minimum each prerequisite above with a test.
- **Risks:** Harmful content and moderation load.
- **Owner:** PM (reopen decision). **Milestone:** later. **Status:** planned (paused). **Evidence:** —

### 3.13 Immediate help and safety (S-150 to S-169)

#### S-150 Screen every text for crisis language before storage and before AI
- **Problem:** Urgent risk must never be missed or wait for a model.
- **Outcome:** A deterministic screen runs on every text a person writes before it is stored and before any model call, in ID, EN and code-switched language.
- **Priority:** pilot-critical. Brief §11; D-036; RR-1.
- **Scope:** The v2 `internal/safety` lexicon (ported from Dengar.in, rule IDs kept) at ruleset -5, merged forward from main once its PR is merged (draft PR #8; D-034 status); applied to every v3 write route; new vectors for multi-turn context, teen slang (ID/EN), older-adult phrasing, quotes and lyrics, negation and code-switching, with each vector tagged by age register (discovery research audit of 2026-10-26, recommendation R-10).
- **Non-goals:** A model-only classifier; contacting anyone automatically.
- **Depends on:** —
- **Acceptance criteria:**
  1. A route-table test proves every v3 route that accepts person-written text screens it before storage and before any provider call.
  2. `safety.model_calls_on_crisis_path = 0` holds for all three providers (EV-7).
  3. The v1 vectors stay as regression tests (`TestV1Parity`, `TestVectorsCoverV1`); the open gaps recorded for ruleset -4 are closed or listed as known gaps with tests.
  4. Worst-case screening time stays linear in input length (benchmark).
  5. Each stored assessment records the ruleset version that produced it (DB test; SPEC V38).
- **Risks:** False negatives on new phrasing; false positives that interrupt ordinary talk. Second safeguards are in S-151 and S-222.
- **Owner:** AI engineer (safety); security/accessibility reviewer. **Milestone:** P2 (ruleset -5) and P3 (v3 routes). **Status:** in progress. **Evidence:** in progress — ruleset -5 A/B/C fix commit `223dc65` (on `278f335`, merged main `d9a5baa`) on `fix/safety-ruleset-5`, draft PR #8 to main ("WIP — DO NOT MERGE"), 2728 vectors. The latest independent review's P1 findings A/B/C are fixed (positive "kembali lagi", "tidak akan lama" followed by a no-return, and the 512-byte post-exemption lookahead with 119/120/121/240-byte boundary tests); `go test -race -count=1 ./internal/safety ./internal/journey ./internal/httpapi` passes with database tests required; the -4/-5 differential over the 1329 shared vectors has zero lower and zero higher routing. Independent re-verification is still pending, so the PR stays draft; v2 evidence for ruleset -4 (1,329 vectors) is v2 scope.

#### S-151 Crisis support that keeps the person's words
- **Problem:** Dengar.in redirected away from the chat on a crisis match and lost the conversation (DX-03), and refused to save heavy journal entries (DX-02); v2 deletes the story.
- **Outcome:** On a crisis match Sela shows verified help inline, keeps the text if storage allows, pauses AI for that conversation until the person chooses to continue, and never contacts anyone.
- **Priority:** pilot-critical. Brief §11; D-036.
- **Scope:** Inline help card in the conversation, journal and check-in; crisis hold per conversation; continue-after-help action; the generator risk flag that can only raise the route (RR-7).
- **Non-goals:** Automatic calls or messages; erasing the person's text; hard redirects.
- **Depends on:** S-150, S-152, S-153.
- **Acceptance criteria:**
  1. A crisis-matching message is kept per the storage choice, the page does not navigate, and inline help appears (e2e).
  2. While the hold is active zero provider calls are made for that conversation; after "Lanjutkan / Continue" generation resumes (test).
  3. Crisis-matching journal and check-in entries save (when storage is on) with support shown.
  4. A model risk flag of `crisis` shows help and withholds that output; `elevated` shows support beside it; a flag never lowers the route (tests).
  5. The help card text is static and announced once; it never waits on a model stream.
  6. BASE.
- **Risks:** Repeated holds frustrate people who talk about past events; quotes and negation are tested in S-150.
- **Owner:** Go backend engineer; frontend engineer; AI engineer. **Milestone:** P3. **Status:** planned. **Evidence:** —

#### S-152 Immediate help page and verified help directory
- **Problem:** People need correct contacts fast; Dengar.in showed an unverified KPAI contact as fact (DX-06) and kept two drifting catalogues (DX-14).
- **Outcome:** One help page, reachable in one action from every screen, listing only services verified from official sources, with source, verification date and a re-verify-by date the server enforces.
- **Priority:** pilot-critical. Brief §11 (verify local support sources; never invent numbers); D-044.
- **Scope:** One catalogue with audience tags (teen, adult, all); limits, cost and hours as the source states them; Puskesmas/FKTP; tap-to-call links named by service; the Stanley & Brown safety-plan structure with attribution (FM-30).
- **Non-goals:** Unverified entries; the KPAI WhatsApp number; any model call on this page.
- **Depends on:** S-191.
- **Acceptance criteria:**
  1. The page makes no model call (test).
  2. Past its re-verify-by date an entry is never shown as current: the server flags it (fake-clock test); the current v2 check of the date format only (`internal/fixtures/fixtures.go`) is replaced.
  3. Facts the source does not state are labelled unconfirmed (for example LISA "free").
  4. Link text names the service (2.4.4).
  5. Every surface that shows help (this page, the inline crisis card of S-151, the under-15 screen of S-005) reads the one catalogue; a bundled offline copy such as `web/src/data/help.fallback.json` is generated from it, and a test fails if the two differ (DX-14; SPEC V39).
  6. BASE.
- **Risks:** Services change; S-154 re-verifies before real users.
- **Owner:** Go backend engineer; frontend engineer; PM (content). **Milestone:** P2. **Status:** planned. **Evidence:** —

#### S-153 "Butuh bantuan sekarang / Need help now" and Quick Exit on every screen
- **Problem:** Help must be one action away, and someone may need to leave the screen fast.
- **Outcome:** A help control labelled "Butuh bantuan sekarang / Need help now" (the fixed wording of [content style](design/CONTENT_STYLE.md), never shortened) and a Quick Exit on every screen, in the same place.
- **Priority:** pilot-critical. D-036 (second safeguards); FM-35; WCAG 3.2.6.
- **Scope:** App-shell controls; Quick Exit to a neutral site via `location.replace`; an optional Esc shortcut.
- **Non-goals:** Deleting data on exit unless the person asks.
- **Depends on:** S-191.
- **Acceptance criteria:**
  1. Both controls appear in the same relative order on every route (e2e).
  2. Quick Exit leaves in one action with no back entry; the Esc shortcut is ignored inside inputs and dialogs and can be turned off (2.1.4).
  3. Both stay visible at 320 CSS px and never hide the focused element (2.4.11).
  4. BASE.
- **Risks:** Browser history on shared devices still shows Sela; the help text says how to clear it.
- **Owner:** Frontend engineer; UX designer. **Milestone:** P2. **Status:** in progress; implemented on `feat/v3-experience` (2026-10-28), owner/browser acceptance pending. **Evidence:** `web/src/v3/shell.tsx`, `prefs.tsx` and the browser checks in [V3_FRONTEND_VALIDATION.md](V3_FRONTEND_VALIDATION.md); Quick Exit is rendered and Esc arming is implemented, but a real leave-the-page test is still a manual owner step.

#### S-154 Re-verify every help line before real users
- **Problem:** Help-line details checked on 2026-10-14/15 have changed: Healing119's site now says 24 hours and lists WhatsApp while the Kemenkes article of 2025-08-07 says morning to night; LISA does not state "free"; Yayasan Pulih contacts moved to `?page_id=201` with new chat lines.
- **Outcome:** Every entry re-checked against its official source, with owner decisions on the conflicting wording.
- **Priority:** pilot-critical. Wrong numbers in a crisis are a safety failure (D-044).
- **Scope:** Owner decisions on Healing119 hours and WhatsApp, LISA cost, Yayasan Pulih source URL, channel labels and "tarif standar"; teen resources verified the same way; `lastVerified` and `reverifyBy` updated.
- **Non-goals:** Calling services from this environment.
- **Depends on:** S-152.
- **Acceptance criteria:**
  1. Each decision is recorded in the decision log with its source and date.
  2. Every shown entry has a re-verify-by date after the pilot's end.
- **Risks:** Services answer differently by phone than on their sites.
- **Owner:** Owner; PM. **Milestone:** P7. **Status:** blocked externally (B-5). **Evidence:** —

### 3.14 Privacy and data (S-170 to S-189)

#### S-170 Choose how Sela keeps my notes
- **Problem:** People need one clear place to choose and change how their notes are kept, and what each choice means.
- **Outcome:** Guest or account, a default (ask each time, save, don't save) and the per-conversation choice, each explained in plain language with location and duration.
- **Priority:** pilot-critical. DoD capability 2; brief §7 (storage control).
- **Scope:** Storage settings; journal and check-in need storage on; switching a conversation to "don't save" deletes its stored messages after confirmation.
- **Non-goals:** Device-only storage of personal content (DX-04).
- **Depends on:** S-002, S-003, S-171.
- **Acceptance criteria:**
  1. No content row exists for a choice that does not store (tests for conversation, journal and check-in).
  2. The durations in the policy text equal the configured durations (test).
  3. Changing the choice is reviewed and confirmed (3.3.4).
  4. BASE.
- **Risks:** Too many options; keep the default question to one sentence.
- **Owner:** UX designer; Go backend engineer; frontend engineer. **Milestone:** P2. **Status:** planned. **Evidence:** —

#### S-171 Consent records per purpose and per provider
- **Problem:** Consent must be separate per purpose and provider, provable, and withdrawable at once.
- **Outcome:** Versioned, server-enforced consent records for storage, sharing, `ai_processing` (Meta on Modal), `ai_provider_anthropic`, `ai_provider_deepseek` and `personal_memory`, with history.
- **Priority:** pilot-critical. UU PDP Pasal 20–24, 9 (withdrawal); D-030, D-049.
- **Scope:** `PUT /api/v3/me/consents/{purpose}` and history (API_SPEC_V3 §3, §6); purposes activated by their milestones with policy texts; withdrawal cancels in-flight work (RR-13).
- **Non-goals:** Bundled consents; consent asked during an outage.
- **Depends on:** S-260.
- **Acceptance criteria:**
  1. ADR-0002 F-35 passes.
  2. Withdrawal stops future calls at once and cancels in-flight operations, whose results are discarded (test with withdrawal mid-operation).
  3. A policy change returns `policy_outdated` and asks again.
  4. The person sees their consent history in plain language.
  5. BASE.
- **Risks:** Withdrawal cannot recall data already sent to a provider; the screen says so (ADR-0003 §8.2).
- **Owner:** Go backend engineer; frontend engineer. **Milestone:** P2 (foundation), P3 and P5 (provider purposes). **Status:** in progress (foundation backend verified locally; the provider purposes come with P3 and P5). **Evidence:** foundation: F-35 passes (`internal/account`, `internal/httpapi`, `internal/db`), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); criterion 2 needs the P3 and P5 operations, criteria 4 and 5 the frontend.

#### S-172 See and end my sessions
- **Problem:** People on shared devices need to see where they are signed in and sign out remotely.
- **Outcome:** A session list with device type, created and last-seen times, and one-action revoke and sign-out-everywhere.
- **Priority:** pilot-important. Session security itself is part of S-003 (pilot-critical); this is the management view.
- **Scope:** API_SPEC_V3 §6 session routes; a settings screen.
- **Non-goals:** IP addresses or raw user agents (never stored).
- **Depends on:** S-003.
- **Acceptance criteria:**
  1. ADR-0002 F-28 passes.
  2. Revoking another device is confirmed and its effect stated.
  3. BASE.
- **Risks:** Device labels are coarse; that is intended (minimisation).
- **Owner:** Frontend engineer; Go backend engineer. **Milestone:** P2. **Status:** in progress (backend verified locally). **Evidence:** backend: F-28 passes (`internal/httpapi`), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); criteria 2 and 3 need the frontend.

#### S-173 Encryption at rest and crypto-shredding
- **Problem:** A stolen database or backup must not expose people's notes, and deletion must reach backups.
- **Outcome:** Per-user envelope encryption (AES-256-GCM data key wrapped by a key-encryption key from the secret manager), AAD binding every ciphertext to its place, and key destruction on deletion (D-028).
- **Priority:** pilot-critical. Brief §10 (encryption in transit and at rest); UU PDP specific data.
- **Scope:** `internal/vault`; key rotation (`sela keys rewrap`); TLS `verify-full` to the database in staging and production; every feature migration lists its plaintext columns with a reason.
- **Non-goals:** Client-side end-to-end encryption (FM-38 merged into this item).
- **Depends on:** S-260.
- **Acceptance criteria:**
  1. ADR-0002 F-12 to F-15 pass.
  2. Each feature migration's plaintext-column list is reviewed in its PR; a scan finds no marker plaintext in any column (F-15 extended to every feature).
- **Risks:** A lost key-encryption key makes all content unreadable; custody is an owner duty (runbook).
- **Owner:** Go backend engineer; security/accessibility reviewer. **Milestone:** P2. **Status:** verified locally (foundation; criterion 2 is repeated by every feature migration). **Evidence:** F-12 to F-15 pass (`internal/vault`, `internal/account`), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)).

#### S-174 Delete my account, my guest data, or one record and what came from it
- **Problem:** Deletion that leaves readable copies (DX-05) breaks trust and the law.
- **Outcome:** Deleting an account or guest removes every owned row, derived summaries, shares and recipient access in one transaction and destroys the key; deleting one record removes what was derived from it.
- **Priority:** pilot-critical. DoD capability 8; brief §7 (deletion with derivatives; clear backup policy).
- **Scope:** `DELETE /api/v3/me` (ADR-0002 §10); the deletion rule every feature implements: a deleted record takes its reflection cards, memory items and AI outputs with it, marks summaries that quote it stale and ends their shares; the UI states what backups still hold.
- **Non-goals:** Recalling copies already downloaded or viewed.
- **Depends on:** S-173, S-260.
- **Acceptance criteria:**
  1. ADR-0002 F-08, F-14 and F-40 pass.
  2. The confirm step lists what will be deleted and requires reauthentication for accounts (3.3.4).
  3. The UI says: gone from the live database now; no backup holds a usable key after 7 days; ciphertext leaves content backups within 30 days (ADR-0002 §11).
  4. Each feature has a test that deleting its records removes their derivatives.
  5. BASE.
- **Risks:** People delete by mistake; the confirm step says it cannot be undone.
- **Owner:** Go backend engineer; frontend engineer. **Milestone:** P2 (account) and P3/P4 (per-record rules). **Status:** in progress (account and guest deletion backend verified locally; the per-record rules come with P3 and P4). **Evidence:** backend: F-08, F-14 and F-40 pass (`internal/db`, `internal/account`, `internal/httpapi`), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); criteria 2 and 3 need the frontend, criterion 4 the feature records.

#### S-175 Export my data
- **Problem:** People need their own copy, readable and machine-readable (UU PDP Pasal 7, 13).
- **Outcome:** A readable (HTML, Markdown) and structured (JSON `sela-export/1`) export of everything the person owns, with provenance and dates.
- **Priority:** pilot-critical. DoD capability 8; brief §7.
- **Scope:** The exporter registry (ADR-0002 §10); each feature registers its section; reauthentication for accounts; an audit event per export.
- **Non-goals:** Exporting hashes, keys or tokens; importing (S-181).
- **Depends on:** S-173, S-176, S-260.
- **Acceptance criteria:**
  1. ADR-0002 F-39 passes, with every feature section registered.
  2. User text is escaped in HTML and Markdown (injection test).
  3. The export says in ID and EN that a downloaded copy cannot be recalled by Sela.
  4. BASE.
- **Risks:** Exports left on shared devices.
- **Owner:** Go backend engineer; frontend engineer. **Milestone:** P2 (foundation sections), P3/P4 (feature sections). **Status:** in progress (foundation sections verified locally; the feature sections come with P3 and P4). **Evidence:** foundation: F-39 passes, HTML escaping of user text is tested (`internal/httpapi/v3_me_test.go`) and the export carries the ID/EN sentence that downloaded copies cannot be recalled (`internal/account/export.go`), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); criterion 4 needs the export screen.

#### S-176 Access audit I can read
- **Problem:** People cannot see who opened their shared summary or what happened to their account.
- **Outcome:** A plain-language, content-free list of sign-ins, consent changes, exports, shares created, opened and revoked, recipient opens and deletions.
- **Priority:** pilot-critical. Brief §7 requires an access audit.
- **Scope:** `audit_events` (append-only; ADR-0002 §3); recipient-open events added in P4; a settings screen.
- **Non-goals:** Content in audit details; operator analytics. The operator access audit is S-274; operator changes to a person's record also appear in this list through it.
- **Depends on:** S-260.
- **Acceptance criteria:**
  1. ADR-0002 F-38 passes; recipient opens produce events (P4 test).
  2. The list is paginated, in plain language, and included in the export.
  3. BASE.
- **Risks:** Event wording that alarms; review copy with the UX designer.
- **Owner:** Go backend engineer; frontend engineer. **Milestone:** P2 (foundation), P4 (recipient events). **Status:** in progress (foundation events verified locally; recipient events come with P4). **Evidence:** foundation: F-38 passes (`internal/httpapi`), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); the list screen needs the frontend.

#### S-177 Privacy policy, terms and per-provider disclosures
- **Problem:** v2's texts say "there is no database" and cover a demo; real use needs complete, honest texts.
- **Outcome:** Versioned ID/EN privacy policy, terms, guest notice, guardian information and one text per provider, containing everything UU PDP Pasal 21 requires.
- **Priority:** pilot-critical. UU PDP Pasal 21–24; brief §7 (clear backup policy), §10 (guest storage).
- **Scope:** Policy texts served by `GET /api/v3/policies`; purpose, data types, retention, processing period, rights, provider, location, backup windows; forbidden phrases.
- **Non-goals:** Legal sign-off (S-178).
- **Depends on:** S-171.
- **Acceptance criteria:**
  1. A test checks each text has every Pasal 21 element and no forbidden phrase (F-33 pattern).
  2. Backup and retention statements equal ADR-0002 §11 and the configuration. While `terms-privacy-2026-11-01.2` is current, the key-dump interval (an operator cron, [runbook §8](RUNBOOK.md#8-backup-and-restore)) stays at one hour or shorter, because the text states a one-hour restore window; a longer interval needs a new policy version first.
  3. BASE.
- **Risks:** Texts drift from code; the equality tests above guard it. The key-dump interval is operator configuration that no test sees, so criterion 2 relies on the runbook rule.
- **Owner:** PM; UX designer; Go backend engineer. **Milestone:** P2. **Status:** in progress (foundation texts implemented and verified locally; the `terms_privacy` text is now `terms-privacy-2026-11-01.2`, with the restore window of D-055, and the `guest_storage` text `guest-storage-2026-11-01.2`, with the guest case of a restore; both versions come with the integration pass, committed on `feat/sela-v3-pilot` (draft PR #9), and the PM confirms the restore sentence of the privacy text as written, including its second clause, and that `guest_storage` text, [D-056](DECISION_LOG.md#d-056)). **Evidence:** `internal/fixtures/v3/policies.json` and `internal/fixtures/policies_v3.go` in the foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); `TestGuestStoragePolicyText` (F-33, which now also checks the restore sentence) and `TestTermsPrivacyStatesBackupWindows` (added with D-055) pass; criterion 1 (a Pasal 21 element test for every text) and the per-provider texts are not done; legal review blocked externally (B-10).

#### S-178 Legal readiness for a real-user pilot
- **Problem:** Sela will process health and possibly children's data across borders; Indonesian law sets duties before users arrive.
- **Outcome:** A DPIA, a DPO, processor agreements, a cross-border basis per provider, PSE registration, a breach runbook and a hosting-region decision, each done or decided by the owner.
- **Priority:** pilot-critical. UU PDP Pasal 34 (DPIA), 46 (breach notice within 3×24 hours), 51 (processors), 53 (DPO), 56 (transfer); PP 71/2019 Pasal 6 and Permenkominfo 5/2020 (PSE registration before use); UU 17/2023 Pasal 346–348 (health-data localisation question); D-049.
- **Scope:** The PM drafts the DPIA, the processing record, the processor list (Modal, Anthropic, DeepSeek, host), the breach runbook with an in-app notice channel (no email exists) and the registration checklist; the owner obtains legal review and registrations.
- **Non-goals:** Legal advice from the team.
- **Depends on:** S-177.
- **Acceptance criteria:**
  1. Each item has a document or a recorded owner decision.
  2. The pilot gate in the [roadmap](ROADMAP.md#2-release-ladder) checks every item.
- **Risks:** PP 33/2026 (reported in force from 2027-01-16; official text unverified) may add duties.
- **Owner:** PM (drafts); owner (review, registration). **Milestone:** P7. **Status:** blocked externally (B-10, B-15). **Evidence:** drafts in progress on branch `feat/sela-v3-ops`, uncommitted, not merged (`docs/compliance/`); nothing verified.

#### S-179 Email verification and recovery (later)
- **Problem:** Some people prefer email recovery.
- **Outcome:** Optional email, with its own consent and ADR.
- **Priority:** post-pilot. Recovery codes cover recovery; no email provider exists.
- **Scope:** Email provider, consent, verification flow.
- **Non-goals:** Making email mandatory.
- **Depends on:** S-003.
- **Acceptance criteria:** Defined with the ADR.
- **Risks:** Email identifies people.
- **Owner:** Go backend engineer; owner (provider). **Milestone:** later. **Status:** blocked externally (B-11). **Evidence:** —

#### S-180 Passkeys (later)
- **Problem:** Passwords are hard for some people.
- **Outcome:** Passkeys as an additional sign-in method.
- **Priority:** post-pilot. Deferred by ADR-0002 §6 (new dependency; shared-device recovery risk); the password path already meets 3.3.8.
- **Scope:** WebAuthn as an extra method.
- **Non-goals:** Replacing recovery codes.
- **Depends on:** S-003.
- **Acceptance criteria:** Defined with a later ADR.
- **Risks:** Device-bound credentials on shared phones.
- **Owner:** Go backend engineer. **Milestone:** later. **Status:** planned. **Evidence:** —

#### S-181 Import a Dengar.in device export (decision needed)
- **Problem:** Dengar.in users kept notes only in their browser (CA-09).
- **Outcome:** An owner decision on whether to support an import.
- **Priority:** post-pilot. Not decided; nothing exists on a server to migrate.
- **Scope:** Decision first; any import needs backup, dry run, ownership validation and rollback (brief §10).
- **Non-goals:** Importing forum or other people's content.
- **Depends on:** S-175.
- **Acceptance criteria:** Defined after the decision.
- **Risks:** Malformed or injected import files.
- **Owner:** Owner; PM. **Milestone:** later. **Status:** planned. **Evidence:** —

#### S-182 Upload documents (later)
- **Problem:** People may want to attach a letter or prior report.
- **Outcome:** Uploads only after their own design.
- **Priority:** post-pilot. Deferred (D-043): validation, malware scanning and storage need design; the `uploaded_document` provenance class is reserved.
- **Scope (prerequisites):** file-type and size validation, malware scanning, encrypted storage, export and deletion, an ADR.
- **Non-goals:** Uploads in the pilot.
- **Depends on:** S-173, S-175.
- **Acceptance criteria:** Defined with the ADR.
- **Risks:** Malware; third-party data in documents.
- **Owner:** Go backend engineer; security/accessibility reviewer. **Milestone:** later. **Status:** planned. **Evidence:** —

### 3.15 Accessibility and display settings (S-190 to S-209)

#### S-190 Design direction, information architecture, screens and content style
- **Problem:** v2's interface was built for a demo flow; v3 needs a full redesign of structure and interaction, not cosmetics (brief §4).
- **Outcome:** A chosen visual direction with reasons, an information architecture covering every journey surface, a screen inventory with each screen's contract, a content style (merging Dengar.in's "Bahasa Lazim" guide) and design tokens.
- **Priority:** pilot-critical. Every user-facing item depends on it.
- **Scope:** `docs/design/` (direction, information architecture, screens, content style, accessibility criteria, `tokens.css`); alternatives explored internally; choices for ages 15 to 50+ without stereotypes.
- **Non-goals:** Asking the owner to pick a style (brief §4).
- **Depends on:** —
- **Acceptance criteria:**
  1. Every surface in brief §4 has a screen entry with purpose, primary action, all states, recovery, keyboard and mobile behaviour.
  2. Token contrast passes `docs/design/tools/contrast.py` (4.5:1 text, 3:1 components) in both themes.
  3. The PM's review against brief §4 (clarity, consistency, readability, task completion, privacy control, accessibility) is recorded.
- **Risks:** A cosmetic redesign accepted as a redesign; the PM review checks structure and interaction.
- **Owner:** UX designer (design lead); PM (review). **Milestone:** P2. **Status:** in progress. **Evidence:** in progress — `docs/design/DESIGN_DIRECTION.md`, `INFORMATION_ARCHITECTURE.md`, `SCREENS.md`, `CONTENT_STYLE.md`, `ACCESSIBILITY_CRITERIA.md`, `tokens.css` and `tools/` (`contrast.py`, `sync_docs.py`), committed with the planning documents (`4cadbfd`); no contrast run and no PM review (criterion 3) recorded yet.

#### S-191 Design system and app shell
- **Problem:** Features need consistent components, navigation and states.
- **Outcome:** A React 19 + TypeScript + Vite component set with every state, and an app shell with navigation, skip link, page titles, help and Quick Exit (D-032).
- **Priority:** pilot-critical. Every screen is built on it.
- **Scope:** Components, layout, navigation, focus management, light and dark themes, a JavaScript size budget (set here, enforced by S-196).
- **Non-goals:** A new framework.
- **Depends on:** S-190.
- **Acceptance criteria:**
  1. Navigation is in the same order on every view (3.2.3); the same function has the same name everywhere (3.2.4).
  2. A sticky header never hides focus (`scroll-padding-top`; 2.4.11).
  3. Every route sets a unique title in the active language (2.4.2).
  4. BASE.
- **Risks:** Rebuilding while v2 pages still exist; the v2 routes stay only in development, test and demo (ADR-0002 §12).
- **Owner:** Frontend engineer; UX designer. **Milestone:** P2. **Status:** in progress; implemented on `feat/v3-experience` (2026-10-28), owner/browser acceptance pending. **Evidence:** `web/src/v3/shell.tsx`, `ui.tsx`, `styles.css` and the browser checks in [V3_FRONTEND_VALIDATION.md](V3_FRONTEND_VALIDATION.md); manual screen-reader review remains open.

#### S-192 Display settings ("Akses cepat")
- **Problem:** People read differently; some need bigger text, more contrast or no motion (FM-40).
- **Outcome:** Settings for text size, theme, contrast, reading spacing, reduced motion and announcing replies to screen readers (S-022), applied before the first paint and kept. No voice settings remain (D-060).
- **Priority:** pilot-important. WCAG conformance (pilot-critical) comes from the default design; this panel serves preferences beyond it.
- **Scope:** "Tampilan & aksesibilitas / Display & accessibility" ([design direction §5.9](design/DESIGN_DIRECTION.md#59-display-preferences)). Preferences are stored on this device in `localStorage` behind `try/catch`; they are non-sensitive, as the v3 browser rule in [safety](SAFETY.md) requires (DX-04). If storage is blocked, a choice lasts until the tab closes. Syncing preferences to the account is planned and needs an API field (design direction §10, item 5).
- **Non-goals:** Defaults chosen by age band; personal content in browser storage.
- **Depends on:** S-191.
- **Acceptance criteria:**
  1. Settings apply before first paint and survive a reload when browser storage is available; with storage blocked the page still works.
  2. Text up to 200 % reflows without loss (1.4.4, 1.4.10).
  3. BASE.
- **Risks:** Settings that conflict with operating-system preferences; the system setting is the default.
- **Owner:** Frontend engineer. **Milestone:** P7. **Status:** planned. **Evidence:** —

#### S-193 Indonesian and English everywhere
- **Problem:** Some Dengar.in server texts were Indonesian only (FM-41); people code-switch.
- **Outcome:** The whole journey, including errors, policies, exports and AI replies, works in ID and EN, and mixed-language input is handled.
- **Priority:** pilot-critical. DoD capability 10.
- **Scope:** i18n dictionaries; server message tables; reply language follows the person; `lang` on parts; code-switched tests in safety, guards and Meaning Lock.
- **Non-goals:** Other languages.
- **Depends on:** S-191.
- **Acceptance criteria:**
  1. i18n parity and hard-coded string tests pass for every v3 page; every v3 error code has both messages (F-42 pattern).
  2. A reply's language matches the person's language or their message's language and is marked with `lang` (3.1.2).
  3. Copy is reviewed by a fluent speaker of each language on the team.
- **Risks:** Live bilingual quality is unmeasured until B-1.
- **Owner:** Frontend engineer; UX designer; AI engineer. **Milestone:** P2 onward. **Status:** in progress; implemented on `feat/v3-experience` (2026-10-28), fluent-speaker copy review pending. **Evidence:** v3 screens use the `w(id, en)` pair at the call site; ID/EN browser paths and the hard-coded-string test pass; a fluent-speaker review is still required by criterion 3.

#### S-194 Automated WCAG 2.2 AA gate in CI
- **Problem:** The e2e scan uses WCAG 2.0/2.1 tags only, fails only on serious findings, checks 390 px instead of 320 px, and every page has the same title.
- **Outcome:** CI fails on any automated WCAG 2.2 AA finding and on reflow, spacing, title and target-size checks.
- **Priority:** pilot-critical. DoD capability 11; brief §6.
- **Scope:** `e2e/` axe tags plus `wcag22aa`, any impact; scans after dynamic states and in both themes; 320 px overflow check; text-spacing injection; unique title per route; target bounding boxes; the accessibility metric in `eval/thresholds.json` moved to WCAG 2.2 AA as a new version.
- **Non-goals:** Treating automated checks as conformance (most criteria are manual; S-195).
- **Depends on:** S-191.
- **Acceptance criteria:**
  1. CI fails when any of the checks above fails (a seeded violation proves it).
  2. Every v3 route is scanned.
- **Risks:** axe has one WCAG 2.2 rule (`target-size`); the manual review is mandatory.
- **Owner:** Frontend engineer; security/accessibility reviewer. **Milestone:** P2. **Status:** planned. **Evidence:** —

#### S-195 Manual accessibility review and browser screenshots
- **Problem:** A passing build is not proof that the interface works for people (brief §4, §13).
- **Outcome:** A recorded manual review of the main journeys with screen readers, keyboard and zoom, and screenshots of each journey.
- **Priority:** pilot-critical. Brief §4 (browser review and screenshots), §6; DoD capability 11.
- **Scope:** NVDA + Firefox, JAWS + Chrome, VoiceOver + Safari (iOS), TalkBack + Chrome (Android); keyboard only; 200 % and 400 % zoom; the six WCAG 2.2 additions (2.4.11, 2.5.7, 2.5.8, 3.2.6, 3.3.7, 3.3.8); journeys: onboarding, conversation with streaming, journal, check-in, timeline, memory, summary, share, recipient, help, export and deletion.
- **Non-goals:** Calling the review user research; it is an engineering review (brief §13). Presenting it as the independent accessibility audit that public release needs ([roadmap §2](ROADMAP.md#2-release-ladder)).
- **Depends on:** the journey items of each milestone.
- **Acceptance criteria:**
  1. Each journey has screenshots at 320 px and desktop, in ID and EN, attached to the milestone PR.
  2. Every finding is fixed or recorded with severity; no open finding blocks a task.
  3. The report states who reviewed, with which tools, and that it is not user research.
- **Risks:** The team lacks some assistive technologies; record what could not be tested.
- **Owner:** Security/accessibility reviewer; UX designer; PM. **Milestone:** each of P2 to P6 (that milestone's screens); final in P7 and P8. **Status:** planned. **Evidence:** —

#### S-196 Slow connections and small devices
- **Problem:** Many people use low-end phones and slow mobile data.
- **Outcome:** Sela stays usable on a throttled connection and a small screen.
- **Priority:** pilot-important. BASE-11 covers each screen; this adds the budget and the network tests.
- **Scope:** A JavaScript size budget enforced in CI; throttled-network e2e; 320 px and 360×640 viewports.
- **Non-goals:** An offline-first app.
- **Depends on:** S-191.
- **Acceptance criteria:**
  1. CI fails when the initial bundle exceeds the budget set in S-191.
  2. On the throttled profile no input is lost and every wait shows a loading state.
- **Risks:** AI waits dominate on slow links; drafts must survive failures within the tab (BASE-11).
- **Owner:** Frontend engineer. **Milestone:** P7. **Status:** planned. **Evidence:** —

### 3.16 Minors, ages 15 to 17 (S-210 to S-219)

#### S-210 An experience that fits ages 15 to 17
- **Problem:** Young people need wording, help and protections that fit their age, and must know the limits of confidentiality; Dengar.in refused them in one place and admitted them elsewhere without protections (DX-13).
- **Outcome:** One minors policy across every surface, built and tested, and disabled in production until the legal review clears it (D-031).
- **Priority:** pilot-important. The brief requires it to be built, but production use is blocked by law (B-10), so the pilot starts with adults.
- **Scope:** Age-appropriate copy; a confidentiality explanation (what Sela keeps, that no guardian sees anything, when Sela shows help, that Sela contacts no one); verified teen help resources; memory and pattern features opt-in only (PP 17/2025 Pasal 19); high-privacy defaults (Pasal 7); audience class `young_person` sent to providers; no DeepSeek.
- **Non-goals:** Guardian access, guardian reports, hidden monitoring.
- **Depends on:** S-005, S-080, S-152, S-211.
- **Acceptance criteria:**
  1. `routing.deepseek_calls_minor_bands = 0` (EV-7).
  2. Production configuration refuses `15_17` (F-10).
  3. No guardian view, export, notification or report exists (route-table test).
  4. Memory and pattern features are off until the young person turns them on.
  5. The age band `unknown` gets the protective defaults of `15_17` wherever it is allowed (test; SPEC V37).
  6. BASE.
- **Risks:** 15-year-olds need a low-risk profile from the Minister (PP 17/2025 Pasal 21(1)(b)); they stay blocked.
- **Owner:** UX designer; frontend engineer; Go backend engineer. **Milestone:** P7. **Status:** planned. **Evidence:** —

#### S-211 Guardian consent record (consent is not access)
- **Problem:** Indonesian law requires a parent's or guardian's consent for children's data (UU PDP Pasal 25), but access for the guardian would harm the young person's trust.
- **Outcome:** A recorded guardian consent that grants the guardian nothing.
- **Priority:** pilot-important. Needed with S-210; production use is blocked (B-10).
- **Scope:** ADR-0002 §3 guardian consent (`guardian_present_attestation`); `SELA_GUARDIAN_CONSENT_REQUIRED`.
- **Non-goals:** Guardian accounts, contact details, notifications.
- **Depends on:** S-005, S-260.
- **Acceptance criteria:**
  1. ADR-0002 F-37 passes.
  2. The UI explains that consent is not access; withdrawal works.
  3. BASE.
- **Risks:** The attestation method may not satisfy the law (S-212).
- **Owner:** Go backend engineer; UX designer. **Milestone:** P2 (backend), P7 (UI). **Status:** in progress (backend verified locally; the screens are P7). **Evidence:** backend: F-37 passes (`internal/httpapi`), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)).

#### S-212 Legal and provider gate for minors
- **Problem:** Whether and how Sela may serve 15–17-year-olds depends on rules and provider terms the team cannot settle alone.
- **Outcome:** Recorded decisions that open or keep closed each age band in production.
- **Priority:** pilot-important. Gates S-210 in production; compliance deadline 2027-03-27 (PP 17/2025 Pasal 49).
- **Scope:** Guardian consent method (UU PDP Pasal 25; PP 17/2025 Pasal 9, including the parent-notification path for 17-year-olds); age verification level (Pasal 22); risk self-assessment (Permenkomdigi 9/2026); profiling (Pasal 19); DPO (Pasal 7(1)(j)); provider terms (Meta Muse Glimmer "not intended for under 18" with the deployer's risk duty; Anthropic's minors guidance; DeepSeek excluded).
- **Non-goals:** Enabling minors before the decisions.
- **Depends on:** S-178, S-005.
- **Acceptance criteria:**
  1. Each question has a recorded decision with its source.
  2. `SELA_ALLOWED_AGE_BANDS` in production matches the decisions.
  3. Before production enables any minors band, the age question tells 15 apart from 16–17 and a 15-year-old is refused with help information (D-031; [design direction §10](design/DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners), item 4). Until then `15_17` stays disabled in production ([conflict matrix](CONFLICT_MATRIX.md) #49).
- **Risks:** Rules change (PP 33/2026).
- **Owner:** Owner; PM. **Milestone:** P7. **Status:** blocked externally (B-10). **Evidence:** —

#### S-213 Evaluation and evidence for young people
- **Problem:** All v2 evaluation personas are adults, and adult evidence must not be applied to teenagers.
- **Outcome:** Fictional 15–17 personas, teen-slang safety vectors and verifier cases, and no adult evidence cited for teenagers.
- **Priority:** pilot-important. Required before minors are enabled anywhere real.
- **Scope:** Personas with gold facts; teen-slang vectors (ID/EN); `age_inappropriate` verifier cases inside EV-0.
- **Non-goals:** Real minors' data.
- **Depends on:** S-150, S-228, S-244.
- **Acceptance criteria:**
  1. The fixtures contain 15–17 personas and the safety vectors carry an age-register tag.
  2. The evidence map labels every row adult or adolescent explicitly (S-244).
- **Risks:** Team-written teen slang is not authentic; recorded as a limitation.
- **Owner:** AI engineer; research engineer. **Milestone:** P7. **Status:** planned. **Evidence:** —

### 3.17 AI routing, verifier and fallback (S-220 to S-239)

#### S-220 Routing core with labelled test doubles
- **Problem:** Three providers with different roles need one predictable policy for when each is used and when Sela abstains.
- **Outcome:** The ADR-0003 routing policy as a pure, table-tested Go state machine, with cost guards, kill switches and provenance records.
- **Priority:** pilot-critical. Brief §8; D-030. Every AI feature calls it.
- **Scope:** Failure classes and rules RR-1 to RR-16; preflight before every call; commit re-check; circuit breaker; profiles and kill switches; cost guards (per-user daily limits, `SELA_VERIFIER_DAILY_USD`, `SELA_DEEPSEEK_DAILY_USD`, Modal admission and container ceiling); labelled test doubles for all three providers; provenance records (migration range 0500–0599); `/api/v3/ai` specified in API_SPEC_V3 §9 first.
- **Non-goals:** Live provider calls (S-221, S-222, S-224); any cache of replies to personal disclosures (DX-15, FM-44 retired).
- **Depends on:** S-171, S-260.
- **Acceptance criteria:**
  1. EV-7 thresholds hold at zero: `routing.fallback_after_content_failure`, `routing.self_verification`, `routing.provider_calls_without_consent` (including withdrawal mid-operation), `routing.deepseek_calls_minor_bands`, `routing.verifier_gated_released_without_check`, `routing.deepseek_listening_calls_without_claude_route` (ADR-0003 RR-4 (i)).
  2. Sela's own rate limit never triggers a fallback (test).
  3. Every output stores the ADR-0003 §10 fields (DB test).
  4. Production refuses test doubles (F-10).
  5. Provider outage, timeout, malformed output and database failure each have a test (the Dengar.in fault-injection scenarios, FM-49).
- **Risks:** The state machine grows untestable; keep it pure and table-driven.
- **Owner:** AI engineer; Go backend engineer. **Milestone:** P3 (Meta-only path), P5 (complete). **Status:** in progress (committed on branch `feat/sela-v3-ai` (`44845d3` and `63a5c98`, then `02051ae`, which merges `feat/sela-v3-pilot` at `59dba14`), not merged; nothing verified). **Evidence:** in progress — `internal/ai` (router, rules, provenance) on that branch; nothing verified.

#### S-221 Meta open-weight model as the primary generator
- **Problem:** The owner requires a Meta open-weight model on Modal for every generative task.
- **Outcome:** Listening replies, reflections, memory proposals, summary wording and opening lines are generated by the pinned Meta model through the router, with honest provenance.
- **Priority:** pilot-critical. Brief §8; D-030. A live provider is a pilot gate.
- **Scope:** `internal/llm` Modal mode through S-220; the served-model pin checked on every chunk (existing); UI provider labels; latency and token records.
- **Non-goals:** Changing the model (owner decision B-7).
- **Depends on:** S-220. Live verification needs the Modal deployment (S-264, P7).
- **Acceptance criteria:**
  1. `live_meta` provenance appears only for output from the real provider whose served name matches the pin (existing tests kept, extended to v3).
  2. Every AI output in the UI names its generator.
  3. Live: smoke test, EV-1 and EV-4 recorded on staging.
- **Risks:** Cold start and GPU cost (ADR-0001); generator choice pending (B-7).
- **Owner:** AI engineer. **Milestone:** P3 (test double), P7 (live). **Status:** in progress (the generator through the router, committed on branch `feat/sela-v3-ai` (`44845d3` and `63a5c98`, then `02051ae`, which merges `feat/sela-v3-pilot` at `59dba14`), not merged; nothing verified); live use blocked externally (B-1). **Evidence:** in progress — `internal/ai/generator.go` on that branch; nothing verified.

#### S-222 Claude Sonnet 5.5 verifier
- **Problem:** AI summaries and health statements can distort what the person said or add unsupported claims.
- **Outcome:** A server-side check that returns supported, unsupported, contradicted or insufficient_evidence per claim against the cited source spans, fails closed, and never stores chain-of-thought.
- **Priority:** pilot-critical. Brief §8 requires verification before health claims and summary wording reach the person; without it every flagged listening reply is withheld.
- **Scope:** VC-1 to VC-9 and CK-1 to CK-9 (ADR-0003 §6–§7), grown from `prototypes/evidenceguard` (missing `unsupported` verdict and issues added, case-insensitive enums, refusal and truncation reasons); `ai_provider_anthropic` consent; `inference_geo: "us"`; workspace-scoped key.
- **Non-goals:** Claude as a generator or fallback (RR-6); presenting agreement between models as clinical evidence.
- **Depends on:** S-171, S-220, S-226.
- **Acceptance criteria:**
  1. VC-6 validation rejects the whole report for every malformed case (unknown, missing or duplicate claim ids, span ids outside the request, `supported` without one of its own spans, reason over 300 characters, wrong model, wrong stop reason).
  2. A refusal or truncation becomes `verification_unavailable` with its reason, and no alternative verifier is tried (RR-10).
  3. No prompt asks for reasoning, and no stored column holds it (prompt test and storage scan).
  4. VC-8 release rules have one test per verdict and issue.
  5. Live: EV-2 and EV-3 recorded on staging.
- **Risks:** Refusals on mental-health text (threshold `verifier.refusal_rate`); Anthropic's usage policy changes on 2026-11-12.
- **Owner:** AI engineer. **Milestone:** P5. **Status:** in progress (committed on branch `feat/sela-v3-ai` (`44845d3` and `63a5c98`, then `02051ae`, which merges `feat/sela-v3-pilot` at `59dba14`), not merged; nothing verified); live use blocked externally (B-8). **Evidence:** in progress — `internal/ai/anthropic`, `internal/ai/verifier` and `eval/verifier` on that branch; nothing verified.

#### S-223 Deterministic output guards and health-claim detector
- **Problem:** No model may be trusted to avoid diagnosis, medication advice, a professional persona or manipulation.
- **Outcome:** Bilingual hard guards that reject outright, and flags that send output to the verifier, applied identically to every generator (RR-7, RR-8).
- **Priority:** pilot-critical. Brief §5 and §8; the listening room (P3) needs it before any reply is released.
- **Scope:** Hard guards: diagnosis addressed to the person, medication or treatment directive (`internal/ledger/clinical.go`), professional persona, exclusivity, dependency, guilt or pressure. Flags: condition or symptom interpretation, research or statistics, efficacy, general health information (claim-gate patterns extended). Schema validation of every model output and the action allow-list from Dengar.in (FM-09): actions may open only real v3 targets (a journal prompt, a reading item, the summary builder, help); `suggest_forum` and `adjust_path` are removed.
- **Non-goals:** Replacing the verifier.
- **Depends on:** —
- **Acceptance criteria:**
  1. Each hard guard and flag has positive and negative tests in ID, EN and code-switched text, including quotation of the person's own words.
  2. The same guard set runs on Meta and DeepSeek output (test).
  3. Output with an unknown action, an unknown target or an invalid schema is rejected before display (test).
- **Risks:** False positives withhold good replies; measure the rate on the synthetic set.
- **Owner:** AI engineer. **Milestone:** P3. **Status:** in progress (committed on branch `feat/sela-v3-ai` (`44845d3` and `63a5c98`, then `02051ae`, which merges `feat/sela-v3-pilot` at `59dba14`), not merged; nothing verified). **Evidence:** in progress — `internal/ai/guard` on that branch; nothing verified.

#### S-224 DeepSeek as fallback generator
- **Problem:** When the Meta model is unavailable, people get no reply.
- **Outcome:** DeepSeek replaces the generator only on an availability failure, only with its own consent, only for adults, and never after a refusal or safety block.
- **Priority:** pilot-important. It improves availability; the honest no-AI path (S-024) is safe without it; it stays off until EV-5 passes and B-9 and the PRC-processing legal question (B-10) are cleared.
- **Scope:** RR-4 under the owner's contract (SPEC C13, V19–V21, [DeepSeek fallback](DEEPSEEK_FALLBACK.md); ADR-0003 §17), with the owner's `prototypes/generationfallback` as the reference implementation and its cases ported; `internal/llm` DeepSeek mode (`deepseek-flash`, `json_object`, thinking disabled, `reasoning_content` dropped); the response `model` field recorded; `SELA_AI_FALLBACK` off by default in every environment, hackathon mode included.
- **Non-goals:** DeepSeek as primary; fallback after any content failure, or after Sela's own capacity limit or a credential failure on Sela's side (owner V19).
- **Depends on:** S-171, S-220, S-221.
- **Acceptance criteria:**
  1. `routing.fallback_after_content_failure = 0` and `routing.deepseek_calls_minor_bands = 0`.
  2. Without consent a Meta outage ends in the no-AI path (test).
  3. Every DeepSeek output is labelled "cadangan / backup" with its provider, stores the provenance `live_deepseek_fallback`, and says that Meta was unavailable and DeepSeek Flash 1017 wrote it (owner contract).
  4. Live: EV-5 passes before the flag is turned on.
  5. A DeepSeek listening reply is released only after a Claude check of it passes. If that check, permitted at preflight, then fails or becomes unavailable, the reply is withheld and DeepSeek never checks it (ADR-0003 RR-8, RR-9, RR-11). The Claude check is ADR-0003's means of meeting V20 for this brief-v3 task, not owner direction (§17 O-6, O-8).
  6. When the Claude route is not permitted for the user or its daily spend ceiling is already reached (no current `ai_provider_anthropic` consent, an age band without Claude, the verifier disabled or the `meta_only` profile, the per-user verifier limit reached, or Sela's verifier daily spend ceiling `SELA_VERIFIER_DAILY_USD` already reached that day), a Meta outage on a listening reply makes zero DeepSeek calls and ends in the no-AI path: `routing.deepseek_listening_calls_without_claude_route = 0`, one test per reason (ADR-0003 RR-2, RR-4 (i)).
- **Risks:** Processing in the PRC; empty `json_object` responses.
- **Owner:** AI engineer. **Milestone:** P5. **Status:** in progress (committed on branch `feat/sela-v3-ai` (`44845d3` and `63a5c98`, then `02051ae`, which merges `feat/sela-v3-pilot` at `59dba14`), not merged; nothing verified); live use blocked externally (B-9). **Evidence:** in progress — `internal/ai/deepseek` on that branch; nothing verified.

#### S-225 DeepSeek as alternative verifier (later)
- **Problem:** A Claude outage makes verifier-gated features abstain.
- **Outcome:** DeepSeek may verify Meta output during a Claude outage, labelled as the alternative verifier.
- **Priority:** post-pilot. Abstaining is safe; the flag stays off until EV-6 passes (D-030).
- **Scope:** RR-9; VC-5.
- **Non-goals:** Verifying DeepSeek's own output; replacing a missing Claude consent.
- **Depends on:** S-222, S-224.
- **Acceptance criteria:**
  1. `routing.self_verification = 0`.
  2. EV-6 passes with the same thresholds as Claude.
- **Risks:** Weaker verification quality.
- **Owner:** AI engineer. **Milestone:** later. **Status:** planned; live use blocked externally (B-9). **Evidence:** —

#### S-226 Send providers as little as possible
- **Problem:** Every provider call is a cross-border transfer of health data.
- **Outcome:** Only the fields each task needs leave Sela, with contact details and private names replaced and no identifiers.
- **Priority:** pilot-critical. Brief §8 ("minimalkan data"); UU PDP Pasal 56; D-030.
- **Scope:** DM-1 to DM-5; the Go port of the Dengar.in PII redactor; per-request span ids; metadata-only logs.
- **Non-goals:** Claiming anonymisation.
- **Depends on:** S-220.
- **Acceptance criteria:**
  1. Requests captured by the test doubles contain no account, user, session or database id, username, IP address or absolute date (test per task).
  2. Phone, NIK, email, card numbers, URLs and handles are replaced by placeholders (tests).
  3. Logs hold provider, task, route, outcome, latency and token counts only (log canary).
- **Risks:** Over-redaction hurts reply quality.
- **Owner:** AI engineer; Go backend engineer. **Milestone:** P3. **Status:** in progress (committed on branch `feat/sela-v3-ai` (`44845d3` and `63a5c98`, then `02051ae`, which merges `feat/sela-v3-pilot` at `59dba14`), not merged; nothing verified). **Evidence:** in progress — `internal/ai/minimize` on that branch; nothing verified.

#### S-227 Meta-only demo profile
- **Problem:** The Meta hackathon submission (deadline 2026-11-04 21:00 ET) must not depend on, or hide, other providers (D-050).
- **Outcome:** `SELA_AI_PROFILE=meta_only` runs every listening feature on Meta alone; verifier-gated features abstain; every output names its provider.
- **Priority:** pilot-important. Needed for the hackathon, not for the pilot.
- **Scope:** The profile flag; the demo environment banner; the organizers' answer on non-Meta providers recorded.
- **Non-goals:** Counting Claude or DeepSeek work as Meta tooling.
- **Depends on:** S-220, S-221.
- **Acceptance criteria:**
  1. Zero Anthropic and DeepSeek calls under the profile (test).
  2. AT-3 and AT-4 abstain with an explanation.
- **Risks:** Live Meta needs B-1 before the deadline.
- **Owner:** AI engineer; PM. **Milestone:** P3. **Status:** in progress (committed on branch `feat/sela-v3-ai` (`44845d3` and `63a5c98`, then `02051ae`, which merges `feat/sela-v3-pilot` at `59dba14`), not merged; nothing verified); the organizers' answer is blocked externally (B-12). **Evidence:** in progress — the `meta_only` profile in `internal/ai` on that branch; nothing verified.

#### S-228 AI evaluation
- **Problem:** Without measured baselines, the verifier's benefit, error rates, latency, cost and fallback quality are guesses.
- **Outcome:** A labelled synthetic set and pre-registered thresholds, run offline, with test doubles and live (ADR-0003 §12).
- **Priority:** pilot-critical for EV-0 to EV-4 and EV-7; EV-5 is pilot-important and EV-6 post-pilot, following S-224 and S-225.
- **Scope:** EV-0 (at least 400 claims across the four verdicts, at least 80 issue cases, at least 40 prompt-injection items, two independent labellers with Cohen's κ, frozen by hash before any verifier run); thresholds added to `eval/thresholds.json` as a new version before the first run; adult, 50+ and 15–17 personas; the Dengar.in CareBridge synthetic set (67 cases, FM-50) migrated as labelled evaluation data.
- **Non-goals:** Clinical validation; lowering a threshold to pass.
- **Depends on:** S-220, S-222.
- **Acceptance criteria:**
  1. EV-0 and the thresholds are committed before the first verifier run (commit order).
  2. Offline and test-double results are recorded; live results only after B-1, B-8 and B-9.
  3. Every report says it measures engineering properties on synthetic text.
- **Risks:** Labellers are not clinicians; recorded as a limitation.
- **Owner:** AI engineer; research engineer. **Milestone:** P5. **Status:** in progress (committed on branch `feat/sela-v3-ai` (`44845d3` and `63a5c98`, then `02051ae`, which merges `feat/sela-v3-pilot` at `59dba14`), not merged; nothing verified); live runs blocked externally (B-1, B-8, B-9). **Evidence:** in progress — `eval/verifier` and a new `eval/thresholds.json` version on that branch; nothing verified.

#### S-229 Fine-tuning proposal gate (later)
- **Problem:** Fine-tuning is sometimes proposed without a measured need.
- **Outcome:** Fine-tuning only with a measured reason, a licensed dataset, a baseline and an evaluation plan, approved by the owner.
- **Priority:** post-pilot. Not approved (D-048); brief §9.
- **Scope:** The gate only.
- **Non-goals:** Training on user notes (never; D-048).
- **Depends on:** S-228.
- **Acceptance criteria:** A proposal document meets every prerequisite before any engineering starts.
- **Risks:** Licence and consent of training data.
- **Owner:** PM; AI engineer. **Milestone:** later. **Status:** planned. **Evidence:** —

### 3.18 Research and evidence (S-240 to S-259)

The 100-record corpus pipeline is kept: it is verified reproducible but title/abstract-screened, not appraised (D-048). The research corpus is a separate store and never linked to any person's history (D-033).

#### S-240 Corpus hygiene fixes
- **Problem:** The 2026-10-26 research audit found screening-manifest hashes that fail after checkout (CRLF vs LF), screening outputs outside PR CI, design flags not shown in the reviewer page, and stale statements in docs 06 and 10.
- **Outcome:** Hashes verify on checkout, CI covers the screening outputs, flags are visible, and stale statements carry dated superseded notes.
- **Priority:** pilot-important. Keeps the verified pipeline trustworthy; not pilot-blocking.
- **Scope:** `scripts/research/screen_candidates.py` line endings; the CI path filter; `web/src/pages/Research.tsx` flags; dated notes in `docs/research/sela-relay/`.
- **Non-goals:** Rewriting history silently.
- **Depends on:** —
- **Acceptance criteria:**
  1. Re-hashing the committed screening files matches the manifest.
  2. `corpus_pipeline.py verify` reports 0 violations; `tests/research` pass.
- **Risks:** None significant.
- **Owner:** Research engineer. **Milestone:** P6. **Status:** in progress. **Evidence:** `268a101` on branch `feat/sela-v3-research` (draft PR #11 to main, not merged): `data/research/.gitattributes` and `docs/research/.gitattributes` pin LF for every hashed output and register document, and docs 06 and 10 carry dated superseded notes; `corpus_pipeline.py verify` reports 0 violations and `tests/research` pass locally and in CI on PR #11. Not done: the CI path filter (the workflow owner must add the paths listed in `docs/research/CORPUS_PIPELINE.md`) and the design flags in `web/src/pages/Research.tsx`.

#### S-241 Resolve unchecked dates and wrong design labels
- **Problem:** R002, R013, R065 and R100 lack a day-precision primary date, and R065 and R100 support feature rows; R032 and R083 are labelled RCT wrongly.
- **Outcome:** Dates resolved from publisher pages or the records dropped from feature rows; design labels corrected by a reviewer.
- **Priority:** pilot-important. Feature claims must not rest on unchecked records.
- **Scope:** Reviewer overrides with name and date.
- **Non-goals:** Paywall bypass.
- **Depends on:** S-242.
- **Acceptance criteria:** `verify` reports no warnings for cited `needs_check` records.
- **Risks:** Publisher pages may be closed.
- **Owner:** Research engineer. **Milestone:** P6. **Status:** planned. **Evidence:** —

#### S-242 Strengthen the corpus schema
- **Problem:** Publication language, age ranges, findings and limitations are missing, and population is matched from titles only. Brief §9 asks to verify each record's design, population, language, results and limitations ("keterbatasan").
- **Outcome:** Fields for publication language, age range with locator, full-text country evidence, a reviewer-only findings field, a reviewer-only limitations field, and reviewer name, date and decision.
- **Priority:** pilot-important. Needed to map v3 evidence honestly.
- **Scope:** `scripts/research/corpus/` deterministic changes; new pipeline version.
- **Non-goals:** Machine-written findings or limitations.
- **Depends on:** —
- **Acceptance criteria:**
  1. The offline rebuild stays byte-reproducible.
  2. Every new field, including `limitations`, is null when unknown; `findings` and `limitations` can be set only through a reviewer override with name and date (a `verify` check rejects any other source).
- **Risks:** None significant.
- **Owner:** Research engineer. **Milestone:** P6. **Status:** in progress (committed on branch `feat/sela-v3-research` as `268a101`, draft PR #11 to main, not merged). **Evidence:** in progress — `scripts/research/corpus/v3fields.py` and the corpus changes on that branch; the local research checks passed on 2026-10-26 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); nothing verified while the work is not on the delivery branch.

#### S-243 Search the gaps v3 needs
- **Problem:** The register has no direct records on listening, journaling, consultation preparation or accessibility, two on adults 50+ (neither digital), and none on adolescent consent or confidentiality. Its help-seeking, safety and privacy records are dominated by students and young adults and include no Indonesian or code-switching crisis-detection evidence (discovery research audit of 2026-10-26, §5).
- **Outcome:** A new frozen search run and a re-screen covering every topic of brief §9.
- **Priority:** pilot-important. Informs product decisions and reading content; not pilot-blocking.
- **Scope:** The v2 families `help_seeking`, `continuity`, `privacy`, `indonesia` and `safety` ([search queries](research/sela-relay/search_queries.json)) carry over and are re-run in the new frozen run under the same freeze-date rule. New v3 families: listening and feeling heard; journaling and expressive writing; consultation preparation; continuity after consultation; help-seeking through digital and AI tools, including ages 15–17 and 50+; safety and crisis support in listening and AI tools, including Indonesian and code-switched text; adolescents 15–17 including consent and confidentiality; adults 50+ and digital mental health; accessibility and disability; Indonesian digital mental health and health-data privacy; Bahasa Indonesia and code-switching in AI. Sources with free APIs; publication language recorded; protocols, corrections and editorials excluded; the false-negative audit of deferred rows run.
- **Non-goals:** Paywall bypass; committing abstracts.
- **Depends on:** S-242.
- **Acceptance criteria:** The run is frozen with its queries and manifest; the screening is reproducible.
- **Risks:** Few Indonesian-language sources in indexed databases.
- **Owner:** Research engineer. **Milestone:** P6. **Status:** in progress (committed on branch `feat/sela-v3-research` as `268a101`, draft PR #11 to main, not merged). **Evidence:** in progress — `scripts/research/v3_gap_search.py`, `data/research/v3_gap_search/` and `docs/research/V3_GAP_SEARCH.md` on that branch; the local research checks passed on 2026-10-26 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); nothing verified while the work is not on the delivery branch.

#### S-244 Evidence map for v3 features
- **Problem:** The evidence map covers only v2 features; 75 of 100 records map to no feature. Brief §9 asks for evidence on listening, journaling, help-seeking, consultation preparation, continuity, adolescents, adults, older adults, the Indonesian context, accessibility, safety and privacy.
- **Outcome:** One row per v3 feature and per brief §9 topic, with supporting records, counter-evidence, population match and claim boundary; gaps declared.
- **Priority:** pilot-important. Research must inform product decisions and education (brief §9).
- **Scope:** `data/research/corpus/evidence_map.json` new version. Feature rows: listening room, journal, check-in, timeline, memory correction, consultation summary, sharing and revocation, minors, accessibility. Topic rows: help-seeking, continuity after consultation, safety and crisis support, privacy and data control, Indonesian context. Population rows: adults (18–49), older adults (50+) and adolescents (15–17). The v2 rows stay as the previous version. SPEC T19 and D-048 name a shorter topic list; brief §9 governs, and D-048 stays as written.
- **Non-goals:** Generalising adult evidence to adolescents; presenting correlation as causation; citations inside empathic replies.
- **Depends on:** S-243.
- **Acceptance criteria:**
  1. Every row labels the population of each cited record (adult 18–49, older adult 50+, adolescent 15–17) explicitly; evidence from one population is never presented as evidence for another (brief §9).
  2. The map has a row for each feature, topic and population listed in the scope; a test fails if one is missing.
  3. Rows without evidence say so as declared gaps, and each row says whether it rests on titles and abstracts or on appraised full text.
- **Risks:** Abstract-level inference mistaken for appraisal; labels say which level each row is.
- **Owner:** Research engineer. **Milestone:** P6. **Status:** in progress (committed on branch `feat/sela-v3-research` as `268a101`, draft PR #11 to main, not merged). **Evidence:** in progress — `data/research/corpus/evidence_map_v3.json`, `scripts/research/v3_evidence.py` and `docs/research/EVIDENCE_MAP_V3.md` on that branch; the local research checks passed on 2026-10-26 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); nothing verified while the work is not on the delivery branch.

#### S-245 Check that sources support the claims, and verify reading content
- **Problem:** Only 6 of the research-doc statements were spot-checked against their abstracts; reading content must be checked before publication.
- **Outcome:** Every remaining statement in docs 07 and 10 checked against its source; every reading item verified at authoring (AT-6) and signed off by a human.
- **Priority:** pilot-important. Needed before reading content is published (S-140).
- **Scope:** Claim-support checks; AT-6 with the verifier on licensed passages only; the Stanford news link replaced by the peer-reviewed R099.
- **Non-goals:** Batch processing of personal data (AT-6 has none).
- **Depends on:** S-222, S-244.
- **Acceptance criteria:**
  1. Each checked statement records source, level checked (abstract or full text) and result.
  2. No reading item is published without a recorded verification.
- **Risks:** Licences that forbid text processing; those items stay unverified and unpublished.
- **Owner:** Research engineer; AI engineer. **Milestone:** P6. **Status:** in progress (committed on branch `feat/sela-v3-research` as `268a101`, draft PR #11 to main, not merged). **Evidence:** in progress — `data/research/corpus/claim_check_v3.json` on that branch; the local research checks passed on 2026-10-26 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); nothing verified while the work is not on the delivery branch.

#### S-246 Human appraisal of the evidence
- **Problem:** No record has had full-text critical appraisal; all are `not_reviewed`.
- **Outcome:** A first batch of 12 records appraised by a qualified reviewer, and dual screening of the 400-item queue.
- **Priority:** pilot-important. Required before any evidence-based claim in product copy.
- **Scope:** R023, R031, R038, R047, R057, R087 and R098 (open full text), R065 and R069 (read on PMC), R099 (arXiv), R027 and R100 (author manuscripts); RoB 2, ROBINS-I, AMSTAR 2 or CASP by design.
- **Non-goals:** Calling machine extraction a review.
- **Depends on:** S-242.
- **Acceptance criteria:** Each appraisal records reviewer, date, tool and result.
- **Risks:** Reviewer availability.
- **Owner:** Research engineer; owner (assign reviewers). **Milestone:** P6. **Status:** blocked externally (B-4). **Evidence:** —

#### S-247 Overlap table for reviews (later)
- **Problem:** 18 reviews may share primary studies, which inflates apparent evidence.
- **Outcome:** A table of included primary studies per review.
- **Priority:** post-pilot. Matters once reviews support claims.
- **Scope:** Open-access reference lists only.
- **Non-goals:** Paywalled sources.
- **Depends on:** S-246.
- **Acceptance criteria:** Each flagged review has its row.
- **Risks:** Time.
- **Owner:** Research engineer. **Milestone:** later. **Status:** planned. **Evidence:** —

#### S-248 Held-out retrieval queries (later)
- **Problem:** The retrieval evaluation is circular (queries and expected ids by the same team).
- **Outcome:** A held-out query set written by someone who does not know the expected ids.
- **Priority:** post-pilot. Retrieval serves the reviewer page, not the pilot.
- **Scope:** `eval/` new query set.
- **Non-goals:** Changing thresholds to pass.
- **Depends on:** S-244.
- **Acceptance criteria:** The set is committed before its first run.
- **Risks:** None significant.
- **Owner:** Research engineer. **Milestone:** later. **Status:** planned. **Evidence:** —

#### S-249 Clinical and user-research validation (later)
- **Problem:** Engineering tests and agent reviews say nothing about benefit or harm to real people (brief §13).
- **Outcome:** A consented, ethics-reviewed study, completed before public release.
- **Priority:** post-pilot. A required public-release gate (B-6; [blockers, Gates](BLOCKERS.md#gates); [roadmap §2](ROADMAP.md#2-release-ladder)), not a controlled-pilot gate; the owner changes this gate only by a recorded decision.
- **Scope:** Study protocol, ethics review, participants across age groups, with adults 50+ and, if minors are enabled, ages 15–17 represented (B-6).
- **Non-goals:** Efficacy claims from synthetic data.
- **Depends on:** S-271.
- **Acceptance criteria:** Defined by the protocol.
- **Risks:** Recruitment, ethics timelines.
- **Owner:** Owner; PM. **Milestone:** later. **Status:** blocked externally (B-6). **Evidence:** —

### 3.19 Operations (S-260 to S-289)

#### S-260 PostgreSQL and migrations
- **Problem:** v2 keeps everything in memory; real notes need durable, migratable storage.
- **Outcome:** PostgreSQL 16 through pgx with an embedded migration runner that supports status, up, dry-run and down-last, with gates for data migrations (D-027).
- **Priority:** pilot-critical. Every persistent feature depends on it; brief §10 (schema migrations; data migrations need backup, dry run, ownership validation and rollback).
- **Scope:** `internal/db` (runner, `dbtest`, migrations 0001 and 0900), `sela migrate` commands, CI PostgreSQL service with `SELA_REQUIRE_DB_TESTS=1`, the Go toolchain move to 1.26 (ADR-0002 §1 option a).
- **Non-goals:** An ORM; SQLite.
- **Depends on:** —
- **Acceptance criteria:**
  1. ADR-0002 F-01 to F-09 and F-46 pass locally and in CI.
- **Risks:** Toolchain change affects the Dockerfile and runbook.
- **Owner:** Go backend engineer. **Milestone:** P2. **Status:** verified locally ([SPEC §G foundation status note](../SPEC.md#foundation-status-note-2026-10-26)): criterion 1 passes locally and in CI (`v2.yml` run 40405475962 on draft PR #9, implementation status §1); if a later CI run fails in these packages, the item counts as implemented until a fix passes. **Evidence:** F-01 to F-09 and F-46 pass (`internal/db`, `internal/db/dbtest`, `cmd/sela`) on Go 1.26.10 (D-054), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)).

#### S-261 Separate environments
- **Problem:** Demo or staging data must never mix with real data, and production must refuse test shortcuts.
- **Outcome:** development, test, demo, staging and production with startup guards; the demo uses synthetic data but every feature really works (D-046).
- **Priority:** pilot-critical. Brief §10, §11.
- **Scope:** `SELA_ENV`; `instance_meta` marker; key-id prefix per environment; production refuses the memory store, test doubles, demo fixtures, insecure cookies and missing secrets. Demo fixtures (the v2 fictional professionals and scenarios, the Dengar.in review-demo cases, FM-31) load only in the demo environment, labelled synthetic (D-042).
- **Non-goals:** A copy of production in staging.
- **Depends on:** S-260.
- **Acceptance criteria:**
  1. ADR-0002 F-10 and F-11 pass. F-10 counts toward this item only once ADR-0002 §12 and the `internal/config` guard are aligned with D-046 ([conflict matrix](CONFLICT_MATRIX.md) row 52): today they refuse the test double in the demo, which is narrower than D-046, and the stricter guard applies until then.
  2. The demo runs accounts, guest mode, persistence, encryption, export and deletion for real, with a "Demo — data sintetis" banner.
  3. In the demo, AI features run on live providers with synthetic data or on labelled test doubles (D-046); test-double output is labelled as such and never shown as live (SPEC V35, V42) (e2e on the demo configuration).
  4. Demo fixtures load only in the demo environment; staging and production refuse to start with them (configuration test; SPEC V42).
- **Risks:** Configuration drift between hosts; the guards fail closed.
- **Owner:** Go backend engineer. **Milestone:** P2. **Status:** in progress (startup guards implemented and verified locally; criterion 1 waits for row 52, criteria 2 to 4 for the demo build). **Evidence:** F-10 and F-11 pass (`internal/config`, `cmd/sela`, `internal/db`), foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); the guard still refuses the test double in the demo ([conflict matrix](CONFLICT_MATRIX.md) row 52).

#### S-262 Backup and restore
- **Problem:** Notes must survive failures, and a restore must honour deletions. One residual window remains: an account deleted after the newest key dump can come back after a total loss of the live database until a deletion journal exists in the backup storage (ADR-0002 §11; accepted for a controlled pilot and revisited before a public release, D-055).
- **Outcome:** Daily content and key dumps with 30-day and 7-day retention, a reconcile step after restore, and a monthly restore drill (ADR-0002 §11).
- **Priority:** pilot-critical. Brief §7 (clear backup policy), §10 (backup/restore).
- **Scope:** `scripts/ops/` backup, restore and drill; `sela restore reconcile`; the canary record; storage in the hosting region.
- **Non-goals:** Point-in-time recovery in the pilot.
- **Depends on:** S-173, S-260.
- **Acceptance criteria:**
  1. ADR-0002 F-44 passes locally.
  2. A drill on the hosted environment is recorded in the runbook with time-to-restore.
  3. After every restore, Sela announces it in ID and EN in two places, as the privacy text `terms-privacy-2026-11-01.2` promises: an in-app notice that every visitor sees on their next visit after the restore, until they dismiss it (also without a session, because the restore ended every session), and a post on Sela's public status page. Both say what [runbook §8](RUNBOOK.md#8-backup-and-restore) lists under "After a restore, tell users", including that an account deleted in the hour before a total loss may have come back and should be deleted again, and that guests have lost access to their guest data, which the guest purge removes on schedule, also when it was deleted in that hour and came back (`guest-storage-2026-11-01.2`). The notice has a state in [screens](design/SCREENS.md) and its copy in [content style](design/CONTENT_STYLE.md) (design lead, S-190; not specified yet); an e2e test on a restored test database shows it in both languages. Milestones: the in-app notice is **P2**, with the app shell and sign-in (S-191, S-003); the status page and its address are part of the hosting setup (S-263), so the status-page post is **P7**.
- **Risks:** Backup storage target undecided (B-3, B-15); the restore revival window above (D-055); the privacy text states it (S-177) and promises that every restore is announced (criterion 3, planned; RUNBOOK §8); without a status page and the in-app notice the privacy text's promise is not kept, so both are a gate for "ready for controlled pilot", and until they exist no restore of a pilot database may happen without a direct notice to every affected user through the app ([D-056](DECISION_LOG.md#d-056); [runbook §8](RUNBOOK.md#8-backup-and-restore)).
- **Owner:** Go backend engineer; frontend engineer and UX designer (the in-app notice); owner (the status page's host). **Milestone:** P2 (criterion 1 and the in-app notice of criterion 3), P7 (criterion 2 and the status-page post of criterion 3). **Status:** in progress: criterion 1 verified locally (CI passed on draft PR #9, [SPEC §G foundation status note](../SPEC.md#foundation-status-note-2026-10-26)); criterion 3 (the in-app restore notice and the status-page post) planned; criterion 2, the hosted drill, blocked externally (B-3, B-15). **Evidence:** F-44 passes: `scripts/ops/restore-drill.sh` and the `cmd/sela` backup and restore tests, foundation commit `0ff3341`, verified locally on 2026-10-26 and in CI on draft PR #9 ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); criterion 2 needs the hosted environment; criterion 3 has no design state, code or status page yet.

#### S-263 Deployment and rollback
- **Problem:** Nothing is deployed; a pilot needs staging, demo and production with a way back.
- **Outcome:** The same container image deployed to staging, demo and production over HTTPS, with secrets from a secret manager, migrations as a separate step and a rehearsed rollback.
- **Priority:** pilot-critical. Brief §10 (deployment, rollback, environment separation); pilot gate (hosting).
- **Scope:** Host choice and region (owner), HSTS, secret manager, a migration step with a backup reference, rollback (previous image plus down-last or restore), smoke test, uptime check, and Sela's public status page with its address (where the restore post of S-262 criterion 3 appears).
- **Non-goals:** Multi-instance (S-268).
- **Depends on:** S-261, S-262.
- **Acceptance criteria:**
  1. Staging and demo deployed; smoke test passes; rollback rehearsed and recorded in the runbook.
  2. Production refuses to start with any guard of S-261 failing.
- **Risks:** Data residency (UU 17/2023 Pasal 346–348 question).
- **Owner:** Go backend engineer; owner (host). **Milestone:** P7. **Status:** blocked externally (B-3, B-15). **Evidence:** deployment files in progress on branch `feat/sela-v3-ops`, uncommitted, not merged (`deploy/v3`, `docs/DEPLOYMENT.md`); nothing verified.

#### S-264 Deploy the Meta model on Modal
- **Problem:** No live Meta output has ever been produced.
- **Outcome:** The pinned model deployed to Modal dev and staging, smoke-tested, scaling to zero, within an approved budget.
- **Priority:** pilot-critical. A live provider is a pilot gate; brief §10 (Modal concurrency limits, scale to zero).
- **Scope:** `modal/sela_llm.py` (ready and tested offline), `modal/smoke_test.py`, the manual deploy workflow, the runbook.
- **Non-goals:** Unbounded GPU spend.
- **Depends on:** —
- **Acceptance criteria:**
  1. `modal deploy -e dev` and the smoke test pass; scale-to-zero and the container ceiling are observed; cold start is measured.
  2. Spend stays within the approved budget.
- **Risks:** Cost; generator decision (B-7).
- **Owner:** AI engineer; owner (credentials, budget). **Milestone:** P7. **Status:** blocked externally (B-1). **Evidence:** —

#### S-265 Observability without content
- **Problem:** Operators need to see failures and costs without seeing anyone's words.
- **Outcome:** Content-free metrics, readiness that reports the database and schema, and alerts.
- **Priority:** pilot-important. The pilot can start on metadata logs and the metrics route; alerts reduce risk.
- **Scope:** Counters for sign-ins by outcome, guests, deletions, exports, AI routes and outcomes, verifier verdicts, latency and cost; F-43 readiness; alerts for spend ceilings, model mismatch, backup and purge failures.
- **Non-goals:** Per-user analytics; content in logs.
- **Depends on:** S-260, S-220.
- **Acceptance criteria:**
  1. ADR-0002 F-41 and F-43 pass; the log canary covers every v3 route.
  2. Each alert fires in a staging test.
- **Risks:** Metrics that identify people at low volume; aggregate only.
- **Owner:** Go backend engineer. **Milestone:** P7. **Status:** in progress (documents, on branch `feat/sela-v3-ops`, uncommitted, not merged). **Evidence:** in progress — `docs/OBSERVABILITY.md` on that branch; nothing verified.

#### S-266 Security review before the pilot
- **Problem:** Real health data raises the stakes; 10 CodeQL alerts were open on PR #6 (merged into main as `7fc6678`; re-check them on main).
- **Outcome:** A threat-model update, the CodeQL alerts triaged, a dependency audit and a review of the authorization and isolation tests, with findings fixed or accepted by the owner.
- **Priority:** pilot-critical. Brief §13 (isolation and authorization evidence); real personal data.
- **Scope:** The v3 routes, cookies, recipient principal, encryption, logs, provider calls and the operator access audit (S-274).
- **Non-goals:** Calling this review independent approval (brief §12) or presenting it as the independent security assessment that public release needs ([roadmap §2](ROADMAP.md#2-release-ladder)); an external penetration test (part of that assessment).
- **Depends on:** S-260 to S-262 and the P3–P5 features.
- **Acceptance criteria:**
  1. Every finding is fixed or has a recorded owner decision.
  2. The report says who reviewed and that it is not an independent audit.
  3. An outbound-destination check shows that the server and the operator scripts contact only the database, the configured AI providers and the configured hosting services (secret manager, backup storage), so nothing contacts a person or another service automatically (SPEC C23).
- **Risks:** Self-review blind spots.
- **Owner:** Security/accessibility reviewer; PM. **Milestone:** P7. **Status:** planned. **Evidence:** —

#### S-267 Legacy Dengar.in deployments and their data
- **Problem:** Two Dengar.in deployments are live outside this repository and may process real people's text with DeepSeek, Gemini and a free OpenRouter model (D-047).
- **Outcome:** A notice or redirect, export and deletion for any real users, and key revocation.
- **Priority:** pilot-important. A risk to people today, but outside the pilot's code.
- **Scope:** Owner and team action; any data move needs backup, dry run, ownership validation and rollback.
- **Non-goals:** Moving legacy data into v3 without those steps.
- **Depends on:** —
- **Acceptance criteria:** The owner's decision and the sunset steps are recorded.
- **Risks:** Unknown data volume.
- **Owner:** Owner; PM. **Milestone:** P7. **Status:** blocked externally (B-13). **Evidence:** —

#### S-268 More than one app instance and incident on-call (later)
- **Problem:** The rate limiter, key cache, AI operation runtimes and the "don't save" hold (D-051) are per instance, and nobody is on call for incidents.
- **Outcome:** Shared limiting and an operation bus so Sela can scale out, and an incident on-call rota with an escalation path.
- **Priority:** post-pilot. The pilot runs one instance (ADR-0002 consequences); readiness for more than one instance and incident on-call are a public-release gate ([blockers, Gates](BLOCKERS.md#gates); [roadmap §2](ROADMAP.md#2-release-ladder)). Incident handling during the pilot is part of the pilot plan (S-271).
- **Scope:** Shared limiter; sticky routing or a shared event bus; the "don't save" hold across instances; an on-call rota, escalation path and incident runbook.
- **Non-goals:** Scaling in the pilot.
- **Depends on:** S-263.
- **Acceptance criteria:** Defined with an ADR.
- **Risks:** Complexity.
- **Owner:** Go backend engineer; PM (on-call rota). **Milestone:** later. **Status:** planned. **Evidence:** —

#### S-269 Row Level Security as a second layer (later)
- **Problem:** Authorization lives in Go only.
- **Outcome:** Database Row Level Security with separate owner and runtime roles (migration range 0900).
- **Priority:** post-pilot. Store methods already require the owner and cross-user tests exist (F-34).
- **Scope:** Roles and policies.
- **Non-goals:** Browser access to the database.
- **Depends on:** S-260.
- **Acceptance criteria:** Defined with the migration.
- **Risks:** Operational complexity.
- **Owner:** Go backend engineer. **Milestone:** later. **Status:** planned. **Evidence:** —

#### S-270 Owner testing guide (`OWNER_TESTER.md`)
- **Problem:** The owner needs to test the product without reading code (brief §13).
- **Outcome:** A guide with prerequisites, steps, expected results, scenario priorities and where to record results and problems.
- **Priority:** pilot-critical. Owner testing is a pilot gate.
- **Scope:** Scenarios for every DoD capability, with the pilot-critical scenarios first; ID and EN; desktop and phone.
- **Non-goals:** Calling owner testing user research.
- **Depends on:** the P3 and P4 journeys.
- **Acceptance criteria:**
  1. Every DoD capability has at least one scenario with expected results.
  2. The guide names where results go (GitHub issues template or a results table).
- **Risks:** Steps that assume developer knowledge; a dry run by someone not on the team.
- **Owner:** PM. **Milestone:** P8. **Status:** planned. **Evidence:** —

#### S-271 Pilot readiness report and pilot plan
- **Problem:** The owner needs one honest statement of what works, what is verified and what blocks the pilot (brief §15).
- **Outcome:** A report with product outcomes, requirements met and not met, quality evidence, commits and PRs, deployments, run guide, AI evaluation results, corpus status, cost assumptions, blockers and remaining work, plus the pilot's operating plan (participants, consent, feedback channel, stop rules, incident handling).
- **Priority:** pilot-critical. Brief §15.
- **Scope:** The report and the plan.
- **Non-goals:** Calling the project done while a main journey is mocked or unverified.
- **Depends on:** every pilot-critical item.
- **Acceptance criteria:**
  1. Each DoD capability has a status on the release ladder with evidence links.
  2. Every open blocker names its owner action.
- **Risks:** Optimism; the PM checks every claim against evidence.
- **Owner:** PM. **Milestone:** P8. **Status:** planned. **Evidence:** —

#### S-272 Retire the v1 runtime
- **Problem:** The Next.js v1 runtime still shipped in the repository (SPEC T9).
- **Outcome:** v1 and AMD runtime removed, with remaining protections ported.
- **Priority:** done 2026-10-29.
- **Scope:** Port the PII redactor (already in `internal/ai/minimize`), self-help cards, help-destination checklists and support options; freeze the v1 crisis tests for `TestVectorsCoverV1`; keep licences, font attribution and Git authorship.
- **Non-goals:** Deleting archived evidence or credits.
- **Depends on:** S-271.
- **Acceptance criteria:** The removal list is decided from dependency evidence and each ported protection has a test.
- **Risks:** Losing a protection only v1 has.
- **Owner:** Go backend engineer; PM. **Milestone:** done. **Status:** done 2026-10-29. **Evidence:** `internal/fixtures/legacy_content.go` + `internal/fixtures/legacy_content_test.go`; `internal/safety/testdata/v1/`; `go test ./...`; `web` typecheck/tests/build; `docker compose config`; CI.

#### S-273 Align the spec and the documentation with v3
- **Problem:** SPEC.md and the supporting docs still describe the v2 limits (synthetic-only, adults-only, memory-only, Meta-only).
- **Outcome:** The spec, decision log, blockers, feature matrix, provenance, ADRs, API contract, backlog, status, traceability and roadmap state the v3 scope explicitly.
- **Priority:** pilot-critical. Brief §1 asks for explicit spec updates; everything else depends on them.
- **Scope:** SPEC amendments to C3, C7, C8, C9, C12 in part, V1, V2, V9, V10, V16, V23, I.data and I.meta, with superseded decisions marked (D-026); D-026 to D-056 (owner direction D-026; PM decisions D-027 to D-050 and D-054 to D-056; engineering defaults D-051 and D-052; merge record D-053) in the decision log; B-8 to B-15 in the blockers; attribution preserved ([provenance](SELA_PROVENANCE.md)); document inventory and index updated.
- **Non-goals:** Reopening PM decisions.
- **Depends on:** —
- **Acceptance criteria:**
  1. Every amended spec ID states its new text and status.
  2. Link and anchor checks pass; the document inventory lists every new document.
- **Risks:** Docs drifting from code; status claims cite evidence only.
- **Owner:** PM. **Milestone:** P1. **Status:** in progress. **Evidence:** in progress — new: `docs/DENGARIN_FEATURE_MATRIX.md`, ADR-0003, this backlog, `docs/design/`; ADR-0002 and `docs/API_SPEC_V3.md` came with the foundation commit `0ff3341`; modified: `SPEC.md` (amendments), `docs/DECISION_LOG.md` (D-026 to D-056), `docs/DECISIONS.md`, `docs/BLOCKERS.md` (B-8 to B-15), `docs/DOCUMENT_INVENTORY.md`, `docs/CONFLICT_MATRIX.md`, `docs/PRODUCT.md`, `docs/SAFETY.md`, provenance, AI policy, Claude verifier, Meta integration, `README.md` and `docs/README.md`. Committed with the planning documents (`4cadbfd`) and the merge of main (`59dba14`, D-053), both on `origin/feat/sela-v3-pilot` in draft PR #9; the integration pass that records D-054 to D-056 is committed on `feat/sela-v3-pilot` (draft PR #9). The link and anchor check passes on the tree of that integration pass ([implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands) has the command and the result); criterion 2 needs it re-run on that commit.

#### S-274 Operator access audit
- **Problem:** The access audit of S-176 covers what the owner sees. Operator commands that can reach personal data or keys, and operator changes to a person's record, would leave no trace (D-052; discovery gap analysis of 2026-10-26: operator access must be logged).
- **Outcome:** Every such command and change writes an append-only, content-free operator audit record, and an operator change to one person's record also appears in that person's own audit (SPEC V32).
- **Priority:** pilot-critical. SPEC V32 (D-052); Sela holds specific personal data (UU PDP Pasal 4(2)(a)), and operators run these commands before and during the pilot.
- **Scope:** An operator audit record (command, environment, operator identity, time, outcome, counts) written by `sela keys` (for example `rewrap`), `sela restore` and `sela restore reconcile` (dry run and real), the restore drill's canary decryption (`sela ops canary check`) and every `sela migrate` run that applies or rolls back a data migration; operator changes to a person's record (such as withdrawing a guardian consent, `withdrawn_by = 'operator'`) mirrored into that person's `audit_events`; where the records are kept and who reviews them, described in ADR-0002 §13 and the runbook by their owner ([conflict matrix](CONFLICT_MATRIX.md) row 57); the database host's access log for direct sessions in staging and production (hosting choice, B-3).
- **Non-goals:** Content, user ids, keys or IP addresses in any operator record; any route or tool that shows an operator decrypted content; per-person analytics (S-265).
- **Depends on:** S-173, S-176, S-260, S-262.
- **Acceptance criteria:**
  1. Each listed command writes one operator audit record per run, with command, environment, operator identity, time, outcome and counts (test per command).
  2. Operator audit records are append-only: the application and the operator commands can add records but not change or delete them (database test).
  3. An operator change to a person's record also writes an event in that person's own audit, shown in S-176 (test).
  4. Operator records and captured logs contain none of the marker content, username, user id, key, token or IP address (log canary, the F-41 pattern).
  5. No route or operator command shows decrypted personal content; the canary check decrypts only the environment's synthetic canary record (route-table and command-list test).
  6. Hosted: the database host's access log is on in staging and production, and the runbook says where it and the operator records are kept and who reviews them (needs B-3).
- **Risks:** Operator identity is only as strong as the host's access control; named operator accounts reduce this (B-3).
- **Owner:** Go backend engineer; security/accessibility reviewer. **Milestone:** P2 (the record and the foundation commands), P7 (hosted access log). **Status:** planned; hosted part blocked externally (B-3). **Evidence:** —

## 4. Milestones

### 4.1 Order and why

The brief lists eight milestones. They are kept, with two changes driven by dependencies: the storage and frontend foundations of brief milestones 2 and 3 move first (P2), because every surface stores encrypted, owned data and runs in the new app shell; the listening experience and the persistent notes then land together (P3), because they write the same records. AI routing (P5) follows the features it checks, but its core starts in P3 because the listening reply calls it. Research (P6) is an independent track from P1 onward. Accessibility and security criteria are part of every item (§2); P7 is the audit, the hosted operations and the minors gate.

| Order | Milestone | Brief milestone | Depends on | Why here |
| --- | --- | --- | --- | --- |
| 1 | **P1** Audit and spec alignment | 1 | — | Every later item cites the realigned spec, decisions and ADRs |
| 2 | **P2** Foundation: persistence, accounts, encryption, consents, export and deletion; design direction, design system, app shell; help on every screen | 3 (storage) and 2 (frontend) | P1 | Conversations, notes, memory and shares are owned, encrypted records; every screen needs the shell, help control and Quick Exit |
| 3 | **P3** Listening room, journal, check-in, memory, timeline, crisis inline; routing core with the Meta path | 2 (listening) and 3 (notes, timeline) | P2 | The core experience; summaries (P4) need these records |
| 4 | **P4** Consultation summary, sharing, recipient, follow-up | 4 | P3 | A summary is built from saved entries and memory |
| 5 | **P5** AI routing complete: Claude verifier, DeepSeek fallback, AI summary wording, memory suggestions, evaluation | 5 | P3, P4 | The verifier checks P3/P4 outputs; its live parts are blocked (B-1, B-8, B-9), so building it after P4 costs no calendar time |
| 6 | **P6** Research and evidence; reading library | 6 | P1 (track runs in parallel); S-245 needs S-222 | Independent data work; reading content needs the evidence map and authoring-time verification |
| 7 | **P7** Accessibility audit, display settings, security, hosted operations, minors | 7 | P3–P6 | Needs the finished surfaces to audit, and hosting to deploy; voice features were retired (D-060) |
| 8 | **P8** Final integration and pilot readiness | 8 | all | Owner testing and the readiness report close the program |

### 4.2 Items and exit criteria

Exit means every listed pilot-critical item is at least `verified locally` (or `blocked externally` with everything not blocked verified locally), every listed pilot-important item is done or explicitly moved, and the criteria below hold. An item that spans milestones is listed in each, with the part that milestone delivers in brackets; the exit applies to that part. Status per milestone is tracked in [implementation status](IMPLEMENTATION_STATUS.md).

**P1 Audit and spec alignment.** Items: S-273.
1. SPEC.md states the amended text and status of C3, C7, C8, C9, C12, V1, V2, V9, V10, V16, V23, I.data and I.meta.
2. D-026 to D-056 (owner direction D-026; PM decisions D-027 to D-050 and D-054 to D-056; engineering defaults D-051 and D-052; merge record D-053) are in the decision log and B-8 to B-15 in the blockers list.
3. The feature matrix, provenance, ADR-0002, ADR-0003, API v3 foundation, this backlog, status, traceability and roadmap are committed; link and anchor checks pass; the document inventory is updated.

**P2 Foundation.** Items: S-001, S-002, S-003, S-004, S-005, S-006, S-150 (ruleset -5 merged forward), S-152, S-153, S-170, S-171 (foundation and the storage and sharing purposes), S-172, S-173, S-174 (account and guest deletion), S-175 (foundation sections), S-176 (foundation events), S-177, S-190, S-191, S-193, S-194, S-195 (P2 screens), S-211 (backend), S-260, S-261, S-262 (local: criterion 1, and the in-app restore notice of criterion 3), S-274 (record and foundation commands).
1. ADR-0002 F-01 to F-47 pass locally with `SELA_REQUIRE_DB_TESTS=1` and in CI.
2. The design documents exist and the PM's review against brief §4 is recorded.
3. An e2e journey in ID and EN runs: welcome → guest start → consents → account upgrade → export → delete, with the S-194 gate passing on every screen.
4. Ruleset -5 is merged forward and its tests pass.
5. Every P2 screen (welcome, guest start, sign-in, account and recovery codes, onboarding consents, storage settings, sessions, export and deletion, the help page, Quick Exit and the app shell) passes the browser review of S-195: screenshots at 320 CSS px and desktop, in ID and EN, attached to the PR (BASE-1; SPEC T15; brief §4).

**P3 Listening, notes and timeline.** Items: S-007, S-020, S-021, S-022, S-023, S-024, S-040, S-050, S-051, S-052, S-060, S-070, S-071, S-080, S-150 (v3 routes), S-151, S-171 (provider purposes for Meta and `personal_memory`), S-174, S-175 (P3 feature sections: per-record deletion with derivatives, export sections), S-195 (P3 screens), S-220 (Meta path), S-221 (test double), S-223, S-226, S-227.
1. e2e with the test double: start without a profile, talk, correct a card, end; save a journal entry and a check-in; a crisis-matching entry saves with inline help and zero provider calls; add, correct and forget a memory item; the timeline shows all of it; delete a conversation and its derivatives disappear.
2. EV-7 routing thresholds for the Meta path hold at zero.
3. S-022 streaming criteria checked manually with at least one screen reader per platform; screenshots attached (S-195).

**P4 Consultation and sharing.** Items: S-090, S-092, S-100, S-101, S-102, S-103, S-104, S-120, S-121, S-130, S-131, S-174, S-175 (P4 feature sections: summaries and shares), S-176 (recipient events), S-195 (P4 screens).
1. e2e: build a manual summary from three entries, approve, preview, share with a purpose, recipient opens with the code, acknowledges, owner revokes, recipient is denied, owner writes an after-consultation reflection.
2. The ported v2 sharing tests (token alone, revocation, expiry, idempotency) and new race tests pass.
3. Screenshots and the manual review are attached (S-195).

**P5 AI integration.** Items: S-081, S-091, S-171 (provider purposes for Anthropic and DeepSeek), S-195 (P5 screens), S-220 (complete), S-222, S-224, S-228.
1. Routing with all three labelled test doubles passes every EV-7 threshold.
2. The VC-6 and VC-8 tests pass; EV-0 and the new threshold version are committed before any live verifier run.
3. With B-1 and B-8 cleared, EV-1 to EV-4 are measured on staging; otherwise the item is recorded `blocked externally` with everything else verified locally.

**P6 Research and evidence.** Items: S-140, S-141, S-195 (P6 screens: reading library and "latihan kecil"), S-240, S-241, S-242, S-243, S-244, S-245, S-246.
1. `tests/research` pass; `verify` has 0 violations and no warnings for cited records.
2. The v3 gap search is frozen; the v3 evidence map has a row per v3 feature, per brief §9 topic and per population, with gaps declared.
3. No reading item or "latihan kecil" is published without recorded verification; B-4 progress is recorded.

**P7 Accessibility, security and operations.** Items: S-154, S-178, S-192, S-195 (full), S-196, S-210, S-211 (UI), S-212, S-213, S-221 (live), S-262 (hosted: criterion 2, and the status-page post of criterion 3), S-263, S-264, S-265, S-266, S-267, S-274 (hosted access log).
1. The full manual accessibility review is complete with no open blocking finding.
2. The security review's findings are fixed or accepted by the owner.
3. Staging and demo are deployed with a recorded restore drill (needs B-3, B-15); Modal is deployed (needs B-1).
4. The minors decision and the legal pack status are recorded.

**P8 Final integration and pilot readiness.** Items: S-195 (final screenshots), S-270, S-271.
1. The owner has run the pilot-critical scenarios of `OWNER_TESTER.md` and recorded the results.
2. The readiness report gives every DoD capability a status on the release ladder, with evidence links.
3. Every remaining blocker names its owner action.

**Later (after the pilot or paused):** S-008, S-041, S-053, S-122, S-142, S-179, S-180, S-181, S-182, S-225, S-229, S-247, S-248, S-249, S-268, S-269.

## 5. Index

| ID | Item | Priority | Milestone | Status |
| --- | --- | --- | --- | --- |
| S-001 | Welcome: what Sela is and is not | pilot-critical | P2 | planned |
| S-002 | Guest start with honest storage notice | pilot-critical | P2 | in progress |
| S-003 | Account, sign-in and recovery without email | pilot-critical | P2 | in progress |
| S-004 | Guest upgrade keeps everything | pilot-important | P2 | in progress |
| S-005 | Age band at the start | pilot-critical | P2 | in progress |
| S-006 | Consents during onboarding | pilot-critical | P2 | planned |
| S-007 | Optional conversation intent | pilot-important | P3 | planned |
| S-008 | One-tap mood entry | post-pilot | later | planned |
| S-020 | Conversation room | pilot-critical | P3 | in progress |
| S-021 | Replies that listen | pilot-critical | P3 | planned |
| S-022 | Screen-reader-friendly streaming and Stop | pilot-critical | P3 | planned |
| S-023 | "Sela menangkap…" cards | pilot-critical | P3 | planned |
| S-024 | AI off or unavailable | pilot-critical | P3 | planned |
| S-025 | Voice dictation | retired | — | retired (D-060) |
| S-026 | Read aloud | retired | — | retired (D-060) |
| S-027 | Voice room | retired | — | retired (D-060) |
| S-040 | Personal home | pilot-important | P3 | planned |
| S-041 | Optional neutral reminders | post-pilot | later | planned |
| S-050 | Notes (journal) inside Catatan pribadiku | pilot-critical | P3 | in progress |
| S-051 | Reflection prompts | pilot-important | P3 | planned |
| S-052 | Reflect on an entry with Sela | pilot-important | P3 | planned |
| S-053 | Weekly reflection | post-pilot | later | planned |
| S-060 | Check in any time | pilot-important | P3 | in progress |
| S-070 | Timeline across sessions | pilot-critical | P3 | in progress |
| S-071 | Find things in my timeline | pilot-important | P3 | planned |
| S-080 | See and correct what Sela remembers | pilot-critical | P3 | in progress |
| S-081 | Memory suggestions from Sela | pilot-important | P5 | planned |
| S-090 | Build a "Ringkasan untuk konsultasi" | pilot-critical | P4 | planned |
| S-091 | AI wording for summary items, verified | pilot-important | P5 | planned |
| S-092 | Print or download one summary | pilot-important | P4 | planned |
| S-100 | Preview and consent before sharing | pilot-critical | P4 | planned |
| S-101 | Share link, code, expiry, revocation, status | pilot-critical | P4 | planned |
| S-102 | Opening lines | pilot-important | P4 | planned |
| S-103 | Destination checklists | pilot-important | P4 | planned |
| S-104 | Private names and redaction preview | pilot-important | P4 | planned |
| S-120 | What the recipient sees | pilot-critical | P4 | planned |
| S-121 | Acknowledgement and consultation status | pilot-important | P4 | planned |
| S-122 | Verified professionals, booking, notes | post-pilot | later | planned |
| S-130 | Reflect after a consultation | pilot-critical | P4 | planned |
| S-131 | My own next steps | pilot-important | P4 | planned |
| S-140 | Reading library | pilot-important | P6 | in progress |
| S-141 | Small exercises ("latihan kecil") | pilot-important | P6 | planned |
| S-142 | Forum (paused) | post-pilot | later | planned (paused) |
| S-150 | Crisis screening on every text | pilot-critical | P2, P3 | in progress |
| S-151 | Crisis support that keeps the words | pilot-critical | P3 | planned |
| S-152 | Immediate help page and verified directory | pilot-critical | P2 | planned |
| S-153 | "Need help now" and Quick Exit everywhere | pilot-critical | P2 | in progress |
| S-154 | Re-verify help lines | pilot-critical | P7 | blocked externally (B-5) |
| S-170 | Choose how Sela keeps my notes | pilot-critical | P2 | planned |
| S-171 | Consent records per purpose and provider | pilot-critical | P2, P3, P5 | in progress |
| S-172 | See and end my sessions | pilot-important | P2 | in progress |
| S-173 | Encryption at rest and crypto-shredding | pilot-critical | P2 | verified locally (CI passed on PR #9) |
| S-174 | Deletion with derivatives | pilot-critical | P2–P4 | in progress |
| S-175 | Export my data | pilot-critical | P2–P4 | in progress |
| S-176 | Access audit | pilot-critical | P2, P4 | in progress |
| S-177 | Policy texts and disclosures | pilot-critical | P2 | in progress |
| S-178 | Legal readiness | pilot-critical | P7 | blocked externally (B-10, B-15) |
| S-179 | Email verification and recovery | post-pilot | later | blocked externally (B-11) |
| S-180 | Passkeys | post-pilot | later | planned |
| S-181 | Dengar.in device import (decision) | post-pilot | later | planned |
| S-182 | Document upload | post-pilot | later | planned |
| S-190 | Design direction, IA, screens, content style | pilot-critical | P2 | in progress |
| S-191 | Design system and app shell | pilot-critical | P2 | in progress |
| S-192 | Display settings | pilot-important | P7 | planned |
| S-193 | Indonesian and English everywhere | pilot-critical | P2 onward | in progress |
| S-194 | Automated WCAG 2.2 AA gate | pilot-critical | P2 | planned |
| S-195 | Manual accessibility review and screenshots | pilot-critical | P2–P8 | planned |
| S-196 | Slow connections and small devices | pilot-important | P7 | planned |
| S-210 | Experience for ages 15–17 | pilot-important | P7 | planned |
| S-211 | Guardian consent record | pilot-important | P2, P7 | in progress |
| S-212 | Legal and provider gate for minors | pilot-important | P7 | blocked externally (B-10) |
| S-213 | Evaluation and evidence for young people | pilot-important | P7 | planned |
| S-220 | Routing core with test doubles | pilot-critical | P3, P5 | in progress |
| S-221 | Meta as primary generator | pilot-critical | P3, P7 | in progress (live blocked, B-1) |
| S-222 | Claude Sonnet 5.5 verifier | pilot-critical | P5 | in progress (live blocked, B-8) |
| S-223 | Output guards and health-claim detector | pilot-critical | P3 | in progress |
| S-224 | DeepSeek fallback generator | pilot-important | P5 | in progress (live blocked, B-9) |
| S-225 | DeepSeek alternative verifier | post-pilot | later | planned |
| S-226 | Data minimisation and redaction | pilot-critical | P3 | in progress |
| S-227 | Meta-only demo profile | pilot-important | P3 | in progress (B-12 open) |
| S-228 | AI evaluation | pilot-critical | P5 | in progress (live blocked, B-1, B-8, B-9) |
| S-229 | Fine-tuning proposal gate | post-pilot | later | planned |
| S-240 | Corpus hygiene | pilot-important | P6 | in progress |
| S-241 | Unchecked dates and design labels | pilot-important | P6 | planned |
| S-242 | Corpus schema fields | pilot-important | P6 | in progress |
| S-243 | v3 gap searches | pilot-important | P6 | in progress |
| S-244 | v3 evidence map | pilot-important | P6 | in progress |
| S-245 | Claim support and reading verification | pilot-important | P6 | in progress |
| S-246 | Human appraisal | pilot-important | P6 | blocked externally (B-4) |
| S-247 | Review overlap table | post-pilot | later | planned |
| S-248 | Held-out retrieval queries | post-pilot | later | planned |
| S-249 | Clinical and user-research validation | post-pilot | later | blocked externally (B-6) |
| S-260 | PostgreSQL and migrations | pilot-critical | P2 | verified locally (CI passed on PR #9) |
| S-261 | Separate environments | pilot-critical | P2 | in progress |
| S-262 | Backup and restore | pilot-critical | P2, P7 | in progress (criterion 1 verified locally; the in-app restore notice, P2, and the status-page post, P7, planned; the hosted drill blocked externally, B-3, B-15) |
| S-263 | Deployment and rollback | pilot-critical | P7 | blocked externally (B-3, B-15) |
| S-264 | Meta model on Modal | pilot-critical | P7 | blocked externally (B-1) |
| S-265 | Observability without content | pilot-important | P7 | in progress |
| S-266 | Security review before the pilot | pilot-critical | P7 | planned |
| S-267 | Legacy Dengar.in deployments | pilot-important | P7 | blocked externally (B-13) |
| S-268 | More than one instance and incident on-call | post-pilot | later | planned |
| S-269 | Row Level Security | post-pilot | later | planned |
| S-270 | `OWNER_TESTER.md` | pilot-critical | P8 | planned |
| S-271 | Pilot readiness report and plan | pilot-critical | P8 | planned |
| S-272 | Retire v1 runtime | done | 2026-10-29 | done; content ported and tests added |
| S-273 | Spec and documentation alignment | pilot-critical | P1 | in progress |
| S-274 | Operator access audit | pilot-critical | P2, P7 | planned (hosted part blocked, B-3) |

**Totals:** 107 items: 52 pilot-critical, 37 pilot-important, 18 post-pilot. The pilot-critical set is large because it holds the eleven DoD capabilities of brief §15 together with the safety controls and the legal and security preconditions for holding real health data; everything that improves the pilot without being required for it is pilot-important, and everything after the pilot or paused is post-pilot. Status today (2026-10-28): 2 verified locally (S-173 and S-260), 39 in progress, 9 blocked externally and 57 planned; the frontend delivery adds local evidence for the screens but does not move an item to verified locally until its acceptance and owner/browser review are complete; S-221, S-222, S-224, S-225, S-227, S-228, S-262 and S-274 also have parts blocked externally. Of the 39 items in progress, 13 have their backend foundation part verified locally (S-002 to S-005, S-171, S-172, S-174 to S-177, S-211, S-261, and criterion 1 of S-262, whose restore notice is planned and whose hosted drill is blocked externally), 23 have work only uncommitted or on separate branches that are not merged (frontend shell, AI provider layer, research and reading content, listening backend, operations), and S-150, S-190 and S-273 are the ruleset `-5` fix loop (a WIP snapshot in draft PR #8), the design documents and the planning documents. Nothing is verified live.

## 6. Feature matrix coverage

Every row of the [Dengar.in feature matrix](DENGARIN_FEATURE_MATRIX.md) lands in a backlog item or in a recorded retirement (D-037, D-038), so no Dengar.in capability is dropped silently.

| Feature matrix row | Decision there | Backlog |
| --- | --- | --- |
| FM-01 Quick mood tap | improve | S-008 |
| FM-02 Consent and disclosure | merge into separate consents | S-006, S-171 |
| FM-03 Anonymous identity and recovery key | migrate into guest mode and recovery codes | S-002, S-003 |
| FM-04 Age band and topic | improve | S-005, S-210 |
| FM-05 Severity assessment and pathway | retire; context questions merge | S-007 |
| FM-06 First-visit privacy warning | merge | S-002 |
| FM-07 Dashboard | migrate without points | S-040 |
| FM-08 Multi-turn chat | migrate (core) | S-020, S-021 |
| FM-09 Action allow-list and output validator | improve | S-223 |
| FM-10 Provider tiers | migrate the pattern | S-220, S-224 |
| FM-11 "Second Brain" debiaser | retire | — (distortion checks in S-222) |
| FM-12 Domain gate | merge | S-021 |
| FM-13 Distress and crisis cards | improve | S-151 |
| FM-14 Deterministic crisis engine | keep (already in v2) | S-150 |
| FM-15 Voice dictation and read-aloud | retire (D-060) | S-025, S-026 (retired) |
| FM-16 Voice room | retire (D-060) | S-027 (retired) |
| FM-17 Journal | migrate | S-050 |
| FM-18 Daily check-in | migrate as standalone | S-060 |
| FM-19 Daily micro-mission | merge into small exercises | S-141 |
| FM-20 14-day path | retire | — |
| FM-21 Gamification | retire | — |
| FM-22 14-day timeline | merge | S-070 |
| FM-23 Weekly report | merge | S-053 |
| FM-24 Care Passport | merge into the consultation summary | S-090, S-091 |
| FM-25 Dated evidence picker | migrate | S-090 |
| FM-26 PII redaction preview | migrate | S-104, S-226 |
| FM-27 Manual no-AI note | merge | S-090 |
| FM-28 Opening lines and destination checklists | merge | S-102, S-103 |
| FM-29 Passport export | migrate, widened | S-092, S-175 |
| FM-30 Immediate-help screen | keep | S-152 |
| FM-31 Review demo | retire the page; cases to demo fixtures | S-261 |
| FM-32 Crisis contacts | migrate | S-152, S-154 |
| FM-33 Help directory and support options | merge | S-152 |
| FM-34 Self-help card library | migrate | S-140 |
| FM-35 Quick Exit | keep | S-153 |
| FM-36, FM-37 Forum and moderation | paused | S-142 |
| FM-38 E2EE cloud backup | merge into server encryption and backups | S-173, S-262 |
| FM-39 Wipe local data | merge into deletion | S-174 |
| FM-40 Accessibility panel | migrate | S-192 |
| FM-41 Bilingual UI and style guide | merge | S-190, S-193 |
| FM-42 Skip link, focus, progress | merge | S-022, S-191 |
| FM-43 Rate limits | merge (already in Go) | S-003, S-220 |
| FM-44 AI response cache | retire | — (S-220 non-goal) |
| FM-45 v1 PostgreSQL and Supabase | retire; replaced | S-260 |
| FM-46 Next.js cache and health routes | retire with v1 (done 2026-10-29) | S-272 |
| FM-47 Live legacy deployments | retire after cutover | S-267 |
| FM-48 AMD adapter | retire | — (v2 V14) |
| FM-49 Fault-injection testbench | merge into failure tests | S-220 |
| FM-50 CareBridge evaluation set | migrate | S-228 |
| FM-51 Docs-only items | retire | — (fine-tuning gate S-229) |

v3 surfaces with no Dengar.in predecessor: memory (S-080), the working recipient view (S-120, from Sela v2), sharing consent by content, recipient and purpose (S-100), the access audit (S-176), the operator access audit (S-274) and the handoff statuses (S-101).
