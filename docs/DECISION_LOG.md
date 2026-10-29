# Decision log

Chronological, append-only record of Sela decisions: v2 implementation decisions D-001 to D-025 (D-025 is main's merge of the owner's DeepSeek fallback), the v3 decisions D-026 to D-050 of 2026-10-26 (D-026 is owner direction, owner brief v3; D-027 to D-050 are PM decisions), the v3 engineering defaults D-051 and D-052, the merge record D-053, and the later v3 PM decisions from D-054 (D-054 the Go toolchain, D-055 the residual risks accepted for a controlled pilot, D-056 the PM confirmations of the integration pass, D-057 the P4 consultation and sharing defaults). Authority: [SPEC.md](../SPEC.md); the [decision summary](DECISIONS.md) lists origins. Each entry states origin, the decision, why, and what it changes. Origins: **owner direction** (user-confirmed; changing it needs new owner direction), **PM decision** (taken by the delivery lead under the authority of owner brief v3; recorded as decided and revisable by the owner) and **engineering** (revisable with rationale). Conflicts that led to entries are in the [conflict matrix](CONFLICT_MATRIX.md).

Entries are never rewritten. When a later decision replaces an earlier one, the earlier entry keeps its text and gets a *Status* line that points to its replacement. Superseded decisions still describe the v2 code until that code changes.

## D-001

*Status (2026-10-26): the clause "V9 is unchanged: Meta is the only AI provider; no fallback provider exists" was first narrowed to the v2 runtime by main's [D-025](#d-025) (the owner's opt-in DeepSeek fallback, C13) and is superseded by [D-026](#d-026) and [D-030](#d-030) (Meta primary, Claude verifier, DeepSeek fallback for availability failures only). The rest stands: Meta open-weight on Modal remains the primary generator.*

2026-10-26 · Owner direction · **AI runs on a Meta open-weight model self-hosted on Modal.com**, replacing the earlier hosted "Meta Model API" plan. V9 is unchanged: Meta is the only AI provider; no fallback provider exists. Model and serving details: [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md).

## D-002

2026-10-26 · Engineering · **Hackathon tooling eligibility is recorded as unverified.** The Official Rules name "Glimmer" and the event page says "run Meta open weights models", but "Meta AI tooling" has no formal definition. No document or UI claims eligibility or a judging advantage.

## D-003

*Status (2026-10-26): superseded by [D-026](#d-026) (real use, ages 15 to 50+), [D-035](#d-035) (conversation room), [D-031](#d-031) (age bands) and [D-027](#d-027) (persistence). Practice text remains a v2 demo behaviour of `/api/v2`.*

2026-10-26 · Owner direction (story room with text input) within the kept synthetic boundary · **Practice text mode.** Besides seeded scenarios (still the default demo path), the user may type a *fictional practice story* (20–3000 characters). Boundaries: adult attestation, explicit "do not enter real names, contacts or health history" notice, deterministic crisis gate before storage or inference, rejection of obvious contact details (email, phone-like runs of ≥9 digits, 16-digit ID numbers, URLs), memory-only storage under the retention policy, no content in logs. This amends SPEC V2; it does not make Sela a real intake service.

## D-004

2026-10-26 · Owner direction · **Four independent decisions:** AI processing consent, storage (retention) consent, brief approval and sharing consent. Each is versioned against a policy text version, server-enforced, and none implies another. Withdrawing AI consent cancels running operations and discards their results.

## D-005

*Status (2026-10-26): superseded by [D-027](#d-027), [D-028](#d-028) and [D-029](#d-029) (persistent encrypted storage, guest purge after 7 days of inactivity or 30 days, backup policy in ADR-0002 §11). It still describes the v2 memory store.*

2026-10-26 · Engineering · **Retention.** Without storage consent: deleted after 30 minutes idle and 2 hours at most. With storage consent: kept in memory up to 24 hours. "Delete everything" purges the session, its shares and recipient access immediately. A restart erases all state and the UI says so. Enforced by store expiry checks on every access plus a one-minute sweeper.

## D-006

2026-10-26 · Engineering · **Identity.** Anonymous by default; optional pseudonym (2–24 letters, no digits/@). The privacy page states that anonymity is not technical anonymity (hosting sees IP/timing; Modal processes AI requests). No real identity is collected.

## D-007

2026-10-26 · Owner direction (token + recipient authorization) · **Two-part recipient access.** A share creates a 256-bit capability token (delivered as a URL fragment, never sent to the server in a URL) and a separate 8-character access code. The recipient principal (its own HttpOnly cookie) is created only when both match, is bound to exactly one handoff of one owner session and expires with the share. Five wrong codes lock the share. Fictional recipients are per owner session, so there is no cross-visitor inbox. All failures return one uniform `access_ended` error. This replaces the same-session "role switch" of the earlier spec.

## D-008

*Status (2026-10-26): superseded by [D-041](#d-041) (prepared → shared → opened → acknowledged → consultation_in_progress, plus revoked, expired, ended). The rule that "sent" is never "care received" is kept.*

2026-10-26 · Engineering, adopting the Relay proposal · **Handoff states** `requested → acknowledged → completed`, plus `clarification_requested` and `declined`; share states `active`, `revoked`, `expired`, `locked`. A clarification is answered by editing the brief, re-approving and sharing again with new consent; the new share supersedes (revokes) the old one. Receipts expose `careReceived: false` always; the UI never equates "sent" or "completed (simulated)" with care received.

## D-009

2026-10-26 · Engineering · **AI operations are asynchronous with server-sent events.** Events: `status` (queued, warming, running, validating), `item` (claims that already passed Meaning Lock), `progress`, `done`, `error`. Cancellation endpoint, per-operation deadline, per-session rate limit, global admission semaphore; retry is a new operation. Unvalidated model text is never streamed to the browser.

## D-010

2026-10-26 · Engineering, porting inherited Dengar.in behaviour · **Story Ledger and Meaning Lock.** Each item keeps the user's verbatim quote (user fact) separately from Sela's wording (AI interpretation) or the user's edit, with deterministic qualifiers (negation, subject, time, certainty). Quotes that drop a governing negator or present another person's statement as the user's are rejected; wording that changes those qualifiers or adds numbers/diagnoses reverts to the quote. Approval requires every item to be reviewed. Care Passport grounding and clarification were implemented earlier by the Dengar.in team; this is a port and extension, not a new invention.

## D-011

2026-10-26 · Engineering · **Next Step is deterministic.** Explicit preferences (language, format, budget, time) and one optional barrier filter the fictional catalogue; exact matches and one-criterion near matches are shown with machine-readable reasons rendered as plain text; "not ready" puts non-handoff steps first. No model involvement, no ranking by clinical suitability.

## D-012

2026-10-26 · Engineering, porting inherited Dengar.in behaviour · **Opening Practice.** Model draft constrained to approved items and an intent vocabulary (ported opening-lines validator) or a deterministic template; the user may rewrite lines; rehearsal attempts get deterministic coverage feedback ("mentioned / not yet mentioned"), never a score, and are not stored. The opening-lines pipeline existed in v1.

## D-013

2026-10-26 · Engineering (keeps SPEC C6) · **Frontend:** new React + TypeScript + Vite app in `web/`, served as static files by the Go binary. The v1 Next.js app is untouched until cut-over.

## D-014

*Status (2026-10-26): superseded by [D-027](#d-027) (PostgreSQL 16 is the system of record; the memory store is allowed only in development, demo and test).*

2026-10-26 · Engineering (keeps SPEC C7) · **Storage:** `journey.Store` interface with one bounded in-memory implementation (`internal/store`). No database, no disk persistence of session data.

## D-015

2026-10-26 · Owner direction · **Research corpus pipeline** (Python, standard library) rebuilds the 100-record register with DOI normalisation, de-duplication, multi-source date verification, a rolling five-year cutoff (freeze date minus five years), licence/access status, screening and conservative evidence fields (null when unknown). Discrepancies with earlier documents are reported, not hidden. See [corpus pipeline](research/CORPUS_PIPELINE.md).

## D-016

2026-10-26 · Engineering · **No fine-tuning.** None of the prerequisites holds: no measured baseline gap, no licensed task dataset, no train/validation/test split of real-distribution data, and synthetic fixtures are too few. Retrieval at inference is also not used: product answers never need literature, and evidence is mapped to features statically. A reviewer-only corpus search is provided.

## D-017

2026-10-26 · Engineering · **No ML safety classifier in v2.** The deterministic lexicon (ported from Dengar.in, extended for clause-level negation, quotes, third-party framing, slang and code-switching) is the gate. Llama Guard 4 was considered: gated, extra GPU cost and no verified Indonesian performance. A classifier would never be the only safeguard.

## D-018

*Status (2026-10-26): superseded by [D-044](#d-044) (verified entries only, with a re-verify-by date the server enforces; teen resources need the same verification).*

2026-10-26 · Engineering · **Help resources:** only v1 entries marked `verified_official` for adults (Healing119, LISA Helpline, Yayasan Pulih) with their sources and access dates (2026-10-14/15), region "Indonesia", and a re-verification date. No new numbers were added.

## D-019

*Status (2026-10-26): superseded by [D-030](#d-030) and [D-040](#d-040). The four draft values stay; the owner's `live_deepseek_fallback` (SPEC V10, V20) and new verification values are added (ADR-0003 §10), and record items also carry a provenance class.*

2026-10-26 · Engineering · **Provenance values** `live_meta`, `recorded_demo`, `manual`, and new `test_double`. `live_meta` requires the configured Modal provider and a server-reported model equal to the pinned served name. `test_double` (cmd/fakellm) is allowed only outside production and is shown with a banner. No recorded results exist yet; none will be fabricated.

## D-020

*Status (2026-10-26): amended for v3 under [D-026](#d-026): the T9 removal waits for v3 acceptance instead of v2's (backlog S-272, [roadmap §5](ROADMAP.md#5-cutover-and-removal-of-v1)); v1 stays runnable and tested until then.*

2026-10-26 · Engineering · **v1 stays runnable and tested.** T9 removal waits for v2 acceptance including live Meta evidence. The v2 runtime contains no AMD code.

## D-021

2026-10-26 · Owner direction · **Git identity.** Commits use the name and email that GitHub links to the YanIanZ account (verified from existing commits' author association), are unsigned, and carry no agent co-author lines. Dengar.in credits remain in [provenance](SELA_PROVENANCE.md).

## D-022

2026-10-26 · Engineering · **Cost guards.** Modal `max_containers=1`, scale to zero after 5 minutes, proxy auth before any container starts; Go per-session limit (8 AI operations per 10 minutes) and per-address request limits. AI status is computed without contacting Modal (a health request can start a GPU).

## D-023

2026-10-26 · Engineering · **Meaning of `negated`, and one gold-label correction.** `negated` is the polarity of the quoted clause's main predicate: true when a negator (tidak, nggak, gak, bukan, belum, no, not, never, -n't, can't) applies to it. A hedge such as "not sure", "nggak yakin" or "tidak tahu apakah" is certainty (`uncertain`), not negation. Under this definition the synthetic gold fact "I wake up after a few hours and can't get back to sleep" was mislabelled `negated: false`; it is corrected to `true` in fixtures `sela-fixtures-2026.10.26-2`. The correction was made during development, before any final evaluation run, and the pre-registered threshold (`meaning_lock.gold_qualifier_negation_accuracy >= 1.0`) is unchanged. The remaining disagreement ("I'm not sure if it's the rotating schedule…" analysed as negated) is a ledger defect to fix, not a label to change.

## D-024

*Status (2026-10-26): still open ([B-7](BLOCKERS.md)); a Modal Shared Endpoint for Muse Spark Open now exists, see B-7.*

2026-10-26 · Engineering, owner decision pending · **Merge of main's EvidenceGuard verifier (PR #5) with the v2 branch.** main added C12 (optional Claude Sonnet 5.5 source-fidelity verifier), I.verifier, V16–V18 and T5a; this branch's own IDs moved to C13–C14 and V20–V23 so published IDs stay stable. The verifier is kept exactly as the owner defined it: optional, separately consented, verification-only, never a fallback generator; it remains a standalone prototype in `prototypes/evidenceguard`, not wired into the v2 runtime (T5a open). **Conflict:** C12 and `docs/CLAUDE_VERIFIER.md` name Muse Spark ("Muse Spark Open on Modal") as the generator, while ADR-0001 (written from this session's brief: a Meta open-weight model on Modal, no unbudgeted GPU use) chose Muse Glimmer 30B because `meta-models/Muse-Spark-Open-816B-A42B` is gated under the Muse Model Licence and needs ≥700 GB of GPU memory (about eight H100/H200 GPUs, roughly $30–40 per hour on Modal). The Go adapter and Modal app are model-agnostic (served model, revision and GPU count are configuration), so either choice is a serving and budget change, not a rewrite. Until the owner decides ([B-7](BLOCKERS.md)), the implemented generator stays Muse Glimmer 30B and no Spark Open deployment is attempted.

## D-025

2026-10-26 · Engineering (merge), owner decisions kept · **Merge of main's DeepSeek fallback prototype (`f4ccc4b`) with the v2 branch; branch IDs renumbered again.** main added C13 (DeepSeek-V4.1-Flash-1017, `deepseek-flash`, as an optional, explicitly consented availability fallback, OFF by default in hackathon mode), I.fallback, V19–V21, T5b and new wording for C5, C12, V9 and V10. main's IDs are published and win, so this branch's unpublished IDs moved: C13 → C14, C14 → C15, V20 → V22, V21 → V23, V22 → V24, V23 → V25 in the spec (§T cites included), the docs and one code comment; no test or eval file cited them. D-024 and the earlier conflict-matrix note keep the IDs that were current when they were written. The fallback is kept exactly as the owner defined it and exists only as the standalone, mock-tested prototype `prototypes/generationfallback` with its own CI workflow. The v2 runtime still has no DeepSeek route, so D-001's "no fallback provider exists" now holds for the runtime only, and T5b stays open. main's V9, V20, AI policy and DeepSeek fallback contract name Muse Spark as the primary generator; as in D-024, that text is kept, the implemented generator stays Muse Glimmer 30B until the owner decides ([B-7](BLOCKERS.md)), and V9 and the AI policy point there. main's docs/README.md and docs/ROADMAP.md each listed the same DeepSeek row twice; the first was kept. See the [conflict matrix](CONFLICT_MATRIX.md).

## D-026

2026-10-26 · Owner direction (owner brief v3, user-confirmed) · **Owner brief v3 supersedes the v2 limits synthetic-only, adults-only, memory-only and Meta-only.** Sela is a real-use health and wellbeing app for ages 15 to 50+ in Indonesian and English, and listening is its core. Synthetic data is for tests, evaluation and the demo only. The safety controls stay. Status reports separate implemented, verified locally, verified live, ready for controlled pilot and public-release blockers (SPEC §G). Spec changes: C3, C7, C8 and C9 superseded and C12 in part; V1, V2, V9, V10, V16 and V23 superseded with replacements; I.data, I.meta, I.api and I.web revised; I.auth, I.ai, I.storage and I.export added; new constraints C16 to C26 and invariants V26 to V42. Superseded decisions: the "no fallback" clause of D-001, D-003, D-005, D-008, D-014, D-018 and D-019.

## D-027

2026-10-26 · PM decision · **PostgreSQL 16 (pgx v5) is the system of record**, with Sela's own embedded, checksummed migrator, reserved migration ranges and data-migration gates (backup, dry run, ownership validation, rollback). The memory store is allowed only in development, demo and test. Why: durable, user-controlled notes across sessions, transactional deletion and restore drills; SQLite and memory-only were rejected ([ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md)). Changes C7 → C16, I.storage. Supersedes D-014.

## D-028

2026-10-26 · PM decision · **Per-user envelope encryption.** Each user's content is sealed with AES-256-GCM under a per-user data key, wrapped by a key-encryption key from the secret manager, with additional data that binds each ciphertext to its user, record and field. Account deletion destroys the data key (crypto-shredding), so content in older backups becomes unreadable once the 7-day key backups expire ([ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) §5, §11). Changes C16, V34.

## D-029

2026-10-26 · PM decision · **Accounts and guest mode.** Username and password (argon2id) with 10 single-use recovery codes; server-side sessions; guest mode stored server-side with a stated location and duration (default deleted after 7 days of inactivity, at the latest 30 days after creation), never promised as absolute anonymity. No email until a provider exists ([B-11](BLOCKERS.md)). Implements owner brief §10 (authentication and recovery; guest mode explains location and duration). Changes C17, I.auth, V27; supersedes D-005 together with D-027 and D-028.

## D-030

*Status (2026-10-26): narrowed by [D-053](#d-053) to the owner's fallback contract (C13, V19–V21, [DeepSeek fallback](DEEPSEEK_FALLBACK.md)): only upstream Meta availability errors trigger the fallback (not Sela's own capacity limit or an authentication failure on Sela's side); the fallback is OFF by default, in hackathon mode too; the draft provenance value is `live_deepseek_fallback`; a DeepSeek listening reply is always verified by Claude, so DeepSeek is called for a reply only when the Claude route is permitted; Meta is always tried first, so the circuit breaker only stops the cold-start wait. The rest stands. Details and the points where owner brief v3 governs: [ADR-0003 §17](adr/ADR-0003-ai-routing-verifier-fallback.md#17-owner-fallback-contract-differences-and-resolutions).*

2026-10-26 · PM decision implementing owner brief §8 (the provider roles are user-confirmed) · **AI routing policy** ([ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md)). The Meta open-weight model on Modal (pinned Muse Glimmer 30B unless the owner decides B-7) is the primary generator for every generative task: listening replies, "Sela menangkap…" reflections, memory proposals, summary drafts and opening lines. DeepSeek (`deepseek-flash`, serving `DeepSeek-V4.1-Flash-1017`; the served version is recorded) replaces the generator only when Meta is unavailable (cold start beyond budget, 5xx, timeout, capacity, an authentication failure on Sela's side) and only with the separate DeepSeek consent, which explains processing in the PRC; never after a refusal, a safety block, a Meaning Lock rejection or any content-based failure; without consent the honest no-AI path remains. Claude Sonnet 5.5 (`claude-sonnet-5-5`, Messages API, structured output) verifies before release: AI consultation-summary wording, AI memory proposals and generated text flagged by the deterministic health-claim detector; reading content is verified at authoring time. Verdicts are supported / unsupported / contradicted / insufficient_evidence with a short reason and the source span checked; no chain-of-thought is requested or stored; Claude needs its own consent (processing in the US). DeepSeek may be the alternative verifier only when Claude is unavailable, Meta was the generator (a model never verifies its own output), the user consented and a default-off flag is enabled after evaluation; its verdicts are labelled. With no permitted verifier, verifier-gated outputs abstain (the user's words are kept, the manual path offered, and the UI says so); ordinary listening replies pass deterministic guards and are released without the verifier unless flagged. Crisis support is deterministic and never waits for a model. Data is minimised (needed segments only, contact details and private names redacted, no account identifiers). Every output records provider, model id, served version, route, verdict and latency. Fallback never gets around a safety refusal. Withdrawing a provider's consent stops its use at once. Changes C12 (in part) → C18, I.ai, V9 → V35, V10 → V35, V16 → V36 and V41. Supersedes the D-001 clause and, with D-040, D-019.

## D-031

2026-10-26 · PM decision implementing owner brief §6 · **Ages 15–17 are implemented but gated.** Age band at onboarding, age-appropriate copy, confidentiality limits explained, a guardian consent record that grants the guardian no access, teen help resources, memory and pattern features opt-in only for minors (PP 17/2025 profiling rules), no hidden monitoring. Disabled in production by configuration until a legal review confirms consent and verification under UU PDP Pasal 25, PP 17/2025 (Pasal 9, 21, 22) and the provider terms (Anthropic minors guidance; Meta Muse Glimmer "not intended for individuals under the age of 18", with a deployer risk duty; DeepSeek guardian-consent wording). 15-year-olds stay blocked until the product is assessed as low risk under PP 17/2025 Pasal 21(1)(b). Changes C8 → C19, V37; blocker B-10.

## D-032

2026-10-26 · PM decision · **Frontend stays React 19 + TypeScript + Vite**, the tested and accessible v2 baseline; the information architecture, interaction design and visual design are redone by the design lead (owner brief §4: the design agent chooses the visual direction). Changes C6 (note), I.web.

## D-033

2026-10-26 · PM decision implementing owner brief §2 · **Content separation.** Public content (reading library, help directory), personal data (owned by one user, linked by ownership and consent), community content (none in the pilot), demo data (synthetic, demo environment only, labelled) and the research corpus (a separate store, never linked to any user's history). Another person's story or a public article is never copied into a user's history; a reading item can be bookmarked by reference. Changes C21, I.data.

## D-034

*Status (2026-10-26, after [D-053](#d-053)): PR #6 is merged into main (`7fc6678`, "Merge pull request #6"), with safety ruleset `-4` (`8018372`) and without `-5`. This branch merges main, so it is no longer stacked on an open PR, and "`-5` lands on PR #6 first" can no longer happen. The PM picks the new route for `-5` (for example a new PR against main that is then merged forward, or a commit on the v3 branch) and the base of the v3 draft PR; SPEC T22, backlog S-150, traceability DL-4 and the status texts follow ([conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 63). Nothing changes status: `-5` is still uncommitted work in progress.*

*Status (2026-10-26, 21:35 UTC; facts read from GitHub through the REST API): the PM took the route of a new PR against main. A WIP snapshot of ruleset `-5` is committed on `fix/safety-ruleset-5` (`f619677`, based on main `7fc6678`) as draft PR #8 to main ("WIP — DO NOT MERGE", opened 21:27 UTC); its independent re-verification is pending and the fix loop continues in the working tree of that branch; its CI runs passed (`v2.yml` 40404566325, Quality gate 40404566458), which does not verify the ruleset. Once PR #8 is merged, `-5` reaches the v3 branch with the next merge of main (as [D-053](#d-053) merged `7fc6678`). The v3 delivery branch is pushed (`origin/feat/sela-v3-pilot` = `59dba14`) and open as draft PR #9 with base main, not stacked; the AI provider layer is draft PR #10 with base `feat/sela-v3-pilot`. SPEC T22, backlog S-150, traceability DL-4 and the status texts follow ([conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 63). This status line adds to the one above and changes neither it nor the decision below.*

2026-10-26 · PM decision · **Delivery branch** `feat/sela-v3-pilot`, a stacked draft PR on top of PR #6 (`feat/v2-go-meta-runtime`). Safety ruleset `-5` lands on PR #6 first and is merged forward (SPEC T22).

## D-035

2026-10-26 · PM decision implementing owner brief §5 · **Listening is the core.** The conversation room ("Ruang cerita" / "Talk") lets a person start talking without a profile. An optional intent picker (just listen / sort my thoughts / understand patterns / prepare a consultation) adjusts the reply style and is never a gate. Replies are contextual, with no mechanical empathy templates, no judgement, no diagnosis, no professional persona, no exclusive relationship and no dependency cues; at most one question per reply; suggestions are offered, never pushed; one action ends the conversation. Each conversation has a storage choice (save / don't save; asked at the start in plain language and changeable). The user can correct or reject Sela's understanding ("Sela menangkap…" cards are editable, confirmable and removable). Seeking a professional is never required. Changes C9 → C25, V26, V27.

## D-036

2026-10-26 · PM decision implementing owner brief §11 · **Crisis handling in v3.** Every user text is screened deterministically before storage and before any model call (the existing lexicon; ruleset `-5` pending). A crisis match never erases what the user wrote and never hard-redirects away from the conversation: it shows inline verified help (and the Immediate help page), pauses AI generation for that conversation until the user chooses to continue after seeing help, and keeps the text only if the storage choice allows it. Journal and check-in entries that match are saved (if storage is on) with support shown, not refused. Second safeguards beyond the lexicon: an always-visible "Butuh bantuan sekarang" control, Quick Exit, a structured risk flag from the generator that can only raise (never lower) the route, the verifier's check, and user review of anything shared. Sela never contacts anyone automatically. Why: v1 navigated away and lost the conversation and refused heavy journal entries (feature matrix DX-02, DX-03); v2 interrupts the whole session. Changes V3 (note), V38.

## D-037

2026-10-26 · PM decision implementing owner brief §5 · **No engagement pressure.** No streaks, points, levels, badges, guilt notifications or push notifications in the pilot. Optional neutral reminders only if the user turns them on (not in pilot scope). Continuity is shown as a calm history, never as a score. Changes C20.

## D-038

2026-10-26 · PM decision implementing owner brief §2 (no silent cuts) · **Dengar.in features not migrated to the pilot, with reasons.** Retired: the severity-scored assessment and "professional pathway" labels (triage, forbidden by brief §11; the context questions merge into the optional intent picker); gamification (engagement exploitation); the 14-day intervention path (a stub, and prescriptive); the OpenRouter "Second Brain" debiaser (a free-tier third party may train on the data; the verifier covers bias and distortion); the AMD provider (removed earlier); the AI response cache; Supabase and forum persistence (replaced by the v3 schema). Paused, kept in v1 code and reopened as backlog items with prerequisites: the anonymous forum "Ruang Cerita" and its moderation (needs moderation operations, author deletion, a minors policy and labelled seed content) and the Gemini voice room (provider outside the v3 plan; autoplay). Voice dictation migrates as an opt-in, with a disclosure that the browser's speech service may send audio to its vendor, an editable transcript before sending and no autoplay; read-aloud through the browser's local speech synthesis is opt-in. The E2EE cloud backup merges into encrypted server persistence plus export (its recovery-phrase idea informs the recovery codes). The daily micro-missions merge into Reading & support as optional "latihan kecil" without day counters. Details per feature: [Dengar.in feature matrix](DENGARIN_FEATURE_MATRIX.md). Changes C9 → C25.

## D-039

2026-10-26 · PM decision implementing owner brief §7 · **Honest terms.** "Catatan pribadiku" (My personal notes) and "Ringkasan untuk konsultasi" (Summary for a consultation). Never "rekam medis", "diagnosis", "hasil pemeriksaan" or "assessment result". Summaries say they are the user's own notes, prepared with AI help where applicable. Changes C22.

## D-040

2026-10-26 · PM decision implementing owner brief §7 · **Provenance classes.** Every important record item carries a class — `user_original`, `ai_summary`, `user_correction`, `unconfirmed_interpretation`, `uploaded_document` (reserved; uploads not in the pilot), `verified_professional_note` (reserved; no verified professionals exist) — plus created and updated time, version and review status (`unreviewed`, `confirmed`, `corrected`, `rejected`). Changes V28, I.data; with D-030 supersedes D-019.

## D-041

2026-10-26 · PM decision implementing owner brief §7 · **Handoff statuses** `prepared → shared → opened → acknowledged → consultation_in_progress`, plus `revoked`, `expired` and `ended`. A sent or opened share is not care; no status implies that treatment was given. Changes V30. Supersedes D-008.

## D-042

2026-10-26 · PM decision implementing owner brief §7 · **Recipients in the pilot** are people the user invites themselves (their own professional), through a link plus a separate access code; the recipient's identity is not verified and is labelled so. No directory of verified professionals, no booking and no partnership claims. The v2 fixture professionals stay for the demo configuration only. Changes C22, V8 (note), V30.

## D-043

2026-10-26 · PM decision · **Uploaded documents are deferred** (not in the pilot): validation, malware scanning and storage need their own design. The `uploaded_document` provenance class is reserved. Changes C25, V28.

## D-044

2026-10-26 · PM decision implementing owner brief §11 · **Help resources.** Only entries verified from official sources are shown, each with source, verification date and a re-verify-by date that the server enforces (stale entries are flagged, not silently shown). Teen-specific resources need the same verification; the unverified KPAI WhatsApp contact from v1 is not shown. Why: v2 checks only the date format (`internal/fixtures/fixtures.go`), and v1 showed an unverified contact to teens (feature matrix DX-06). Changes V39; blocker B-5. Supersedes D-018.

## D-045

2026-10-26 · PM decision · **WHO self-help cards.** The 22 WHO-adapted cards (CC BY-NC-SA 3.0 IGO) may be used in a non-commercial pilot with the required attribution and adaptation notice; any commercial launch is blocked until they are licensed or replaced ([B-14](BLOCKERS.md)).

## D-046

2026-10-26 · PM decision implementing owner brief §11 · **Environments** development, demo, staging and production (plus test). Production refuses the memory store, test doubles, demo fixtures and missing secrets. The demo uses synthetic data with every feature really working (test doubles labelled, or live providers on synthetic data). Changes C24, V42 ([ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) §12).

## D-047

2026-10-26 · PM decision (risk record) · **Legacy Dengar.in deployments.** Two legacy deployments are live outside this repository (`dengar-in.vertalt.my.id` and `dengar-in.vercel.app`; health checks returned 200 on 2026-10-26) and may process real users' text with DeepSeek, Gemini and a free-tier OpenRouter model. Decommissioning or a notice is an owner/team action ([B-13](BLOCKERS.md)).

## D-048

2026-10-26 · PM decision implementing owner brief §9 · **Research and evidence.** The 100-record corpus pipeline is kept (verified reproducible by the discovery research audit of 2026-10-26: the 74 offline `tests/research` tests pass, `corpus_pipeline.py verify` reports 0 violations, and an offline rebuild from the local cache is byte-identical to the committed outputs). It is title/abstract-screened, not appraised. v3 needs new evidence for listening, journaling, consultation preparation, accessibility, adolescents' consent and confidentiality and adults 50+ (the register has no direct record on listening, journaling, consultation preparation or accessibility); the gaps are recorded and researched without paywall bypass. No fine-tuning is approved; user notes are never used for training. Changes C15 (note); SPEC T19.

## D-049

2026-10-26 · PM decision · **Cross-border processing disclosure.** All three AI providers are cross-border processors (Modal and Anthropic in the US, DeepSeek in the PRC). The app discloses each provider, location and purpose in plain language before consent; the legal review (DPIA, processor agreements or explicit consent under UU PDP Pasal 56) is a pilot blocker ([B-10](BLOCKERS.md)). Changes C18.

## D-050

2026-10-26 · PM decision implementing owner brief §11 · **Hackathon.** The Meta hackathon rules neither allow nor forbid Claude or DeepSeek. Keep a Meta-only demo path, label every output's provider, never count Claude or DeepSeek as Meta tooling, and ask the organizers (owner action, [B-12](BLOCKERS.md)) before the submission deadline (2026-11-04 21:00 ET). Changes C12 (note), C18, V35.

## D-051

2026-10-26 · Engineering (spec default under D-035; settles [design direction §10](design/DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners), "don't save" conversations) · **An unsaved conversation lives only in bounded server memory.** While a "don't save" conversation is open, its turns are held in server memory for that owner and conversation only, bounded in count and size, never in the database, backups, logs or browser storage, and used only as context for that conversation's replies (ADR-0003 AT-1). They are purged at End, at sign-out or session end, after 30 minutes without activity or 2 hours in total (whichever comes first), and on a server restart; within that window the same session can reopen the conversation after a reload. Why: the reply needs the earlier turns, but "don't save" must leave nothing behind when the person walks away (tab closed, crash, idle), and memory-only holding keeps the text out of backups, so no backup deletion path is needed. The 30-minute and 2-hour bounds reuse the v2 retention without storage consent (D-005). The hold is per instance; the pilot runs one instance, and backlog S-268 revisits it before scaling out. "Discarded" in the UI covers only what Sela holds; provider retention is disclosed before consent (V34). Changes V27.

## D-052

2026-10-26 · Engineering (spec default; discovery gap analysis of 2026-10-26: operator access must be logged) · **Operator access is audited.** Every operator command that can reach personal data or keys (`sela keys`, such as a rewrap; `sela restore` and its reconcile; the restore drill's canary decryption; data migrations) and every operator change to a user's record (such as withdrawing a guardian consent, ADR-0002 `withdrawn_by = 'operator'`) writes an append-only, content-free operator audit record: command, environment, operator identity, time, outcome and counts; never content, user ids, keys or IP addresses. An operator change to one user's record also appears in that user's own audit (V32). Direct database sessions in staging and production are recorded by the database host's access log, which depends on the hosting choice (B-3). No route or tool shows an operator a user's decrypted content. Why: V32 covered only owner-visible events, so key operations and support access would otherwise leave no trace, and Sela holds specific personal data (UU PDP Pasal 4(2)(a)). Changes V32.

## D-053

2026-10-26 · Engineering (merge, under the PM's merge rules), owner decisions kept · **Merge of main `7fc6678` into the v3 branch; v3 IDs renumbered so that main's published IDs are never reused; v3 AI documents aligned with the owner's DeepSeek fallback contract.** main (PR #6 merged, including `67d313c`, which merged the owner's DeepSeek fallback prototype `f4ccc4b`) published C13, I.fallback, V19–V21, T5b, the C5, C12, V9 and V10 wording, its renumbering of the v2 branch IDs and [D-025](#d-025). Those win. The v3 documents had been written on the old numbering, so: v3 references to the old v2 branch IDs follow main's mapping (C13 → C14, C14 → C15, V20 → V22, V21 → V23, V22 → V24, V23 → V25); the v3 constraints C15–C25 become C16–C26; the v3 invariants V24–V40 become V26–V42, in the same order; and the v3 decisions D-025–D-051 become D-026–D-052, because main's D-025 is a different decision. The v3 placeholder "V19 Unused" is replaced by main's V19; nothing cited it. Task IDs: main's T5b is added and carried into T18; T13–T22 keep their numbers. The renumbering covers the spec, every document (including `docs/design/`, the backlog, ADR-0002, ADR-0003, traceability, implementation status and roadmap) and one test comment (`internal/config/v3_config_test.go`); no other code or test cited a moved ID. Historical records keep the IDs that were current when they were written (D-024, the PR #5 row of the conflict matrix). The dated table is in the [conflict matrix](CONFLICT_MATRIX.md#renumbering-after-merging-main-7fc6678-2026-10-26).

The v3 AI documents (ADR-0003, the AI policy, the Claude verifier contract, the Meta integration notes) now follow the owner's [DeepSeek fallback contract](DEEPSEEK_FALLBACK.md): the draft provenance value is main's `live_deepseek_fallback` (v3 had called it `live_deepseek`); `live_deepseek_alternative` stays only for the alternative verifier that owner brief v3 §8 asks for; the fallback is OFF by default, in hackathon mode too; the owner's `prototypes/generationfallback` is the reference implementation, and `internal/ai` (P5, being built) is the production integration, which must satisfy C13 and V19–V21 as well as ADR-0003. Where ADR-0003 and the owner contract differ in substance, the owner contract wins unless owner brief v3 says otherwise; every difference and its resolution is listed in [ADR-0003 §17](adr/ADR-0003-ai-routing-verifier-fallback.md#17-owner-fallback-contract-differences-and-resolutions). To meet the owner's V20 for listening replies, which are a brief-v3 task (§17 O-9), ADR-0003 has Claude check every DeepSeek reply (§17 O-6, O-8). That check is the ADR's means and an engineering default under this entry, not owner direction, since the owner contract leaves Claude review to its own consent. So DeepSeek is called for a reply only when the Claude route is permitted (ADR-0003 RR-4 (i), tested by `routing.deepseek_listening_calls_without_claude_route = 0`); the design copy that follows is [conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 62. With PR #6 merged, the D-034 route for safety ruleset `-5` is open for the PM (row 63). The owner's files under `prototypes/` and `docs/DEEPSEEK_FALLBACK.md` are byte-identical to main. Nothing becomes implemented by this merge: T5b, T18 and every v3 task keep their status.

## D-054

2026-10-26 · PM decision (the toolchain ruling that [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) §1 left pending; option a) · **The Go module moves to Go 1.26.** `go.mod` declares `go 1.26.0` with `toolchain go1.26.10`, and the container image builds with `golang:1.26.10-alpine` (`deploy/v2/Dockerfile`); CI follows `go.mod` (`setup-go` with `go-version-file`). Landed with the v3 foundation commit `0ff3341`. Why: the approved pins need it — pgx v5.10.0 declares `go 1.25.0` and `golang.org/x/crypto` v0.57.0 declares `go 1.26.0` in their go.mod files — and Go 1.24 no longer receives security fixes (the newest releases on the Go module proxy were go1.27.3 and go1.26.10 when read on 2026-10-26, and Go supports the two newest major releases; ADR-0002 §1). Option b (stay on Go 1.24.7 with pgx v5.8.0 and x/crypto v0.48.0) was rejected: it would pin a toolchain without security fixes for a service that holds real personal data. Changes: ADR-0002 §1 and F-46 (the toolchain is chosen); `README.md`, `CONTRIBUTING.md`, the [development](DEVELOPMENT.md) and [runbook](RUNBOOK.md) requirements now say Go 1.26.10. The owner's standalone prototypes (`prototypes/evidenceguard`, `prototypes/generationfallback`) keep their own `go.mod` and CI workflows and are unchanged. Closes [conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 48.

## D-055

2026-10-26 · PM decision (risk acceptance) · **The residual risks of the v3 foundation are accepted for a controlled pilot, and must be revisited before a public release.** [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) states two residual risks after three rounds of security review by agent reviewers independent of the implementer (internal; not the independent security assessment that public release requires) of the foundation (`0ff3341`):

1. **Username timing after an argon2id parameter change** (ADR-0002 §6, §9; RUNBOOK §3). An unknown username verifies against a dummy hash that follows the stored parameter distribution, refreshed at start and every 10 minutes. For a while after the argon2id parameters change (until the next refresh, and for an attacker who knows a particular account's history, such as that it has not signed in since the change), and after a KEK rotation while more than one parameter tuple is in use, a failed login can still show a small timing difference between an existing and an unknown username. The per-client and per-username rate limits and the backoff allow only a handful of samples per username (ADR-0002 §9), registration already reveals that a username is taken, and operators rotate the KEK while one parameter tuple is in use where possible.
2. **Restore revival window** (ADR-0002 §11; RUNBOOK §8). If the live database is lost entirely and restored from backups, an account deleted after the newest key dump — at most one key-dump interval before the failure, one hour by default — comes back with the restore, and it cannot be identified afterwards, because deletion leaves no record outside the key table. Reconcile honours every deletion the key dump records, the newest key dump is required, a fresh key dump is taken from the damaged database when it is still reachable, the privacy text states the window (`terms-privacy-2026-11-01.2`), and every restore is announced so that a person who deleted their account in that hour can delete it again. A deletion (tombstone) journal shipped to the backup storage on every deletion would close the window; it waits for the backup-storage decision (B-3, B-15).

Why: both are bounded, stated to users or operators, and outweighed for a controlled pilot by the protections around them; closing the second needs the hosting decision. Revisit before any public release: re-measure the timing difference with the production parameters and decide whether a further change is needed, and build the deletion journal (or another way to honour every deletion at restore) once B-3 decides the backup storage. This review is listed among the public-release gates ([blockers, Gates](BLOCKERS.md#gates)). Changes: SPEC V34 (the restore window replaces "a restore never revives a deleted account"); the `terms_privacy` text, new version `terms-privacy-2026-11-01.2`; ADR-0002 §6, §9 and §11 (residuals accepted for the pilot); the public-release gates in [blockers](BLOCKERS.md#gates) and [roadmap §2](ROADMAP.md#2-release-ladder); [safety](SAFETY.md), [product](PRODUCT.md), the [backlog](BACKLOG.md) (S-262), the [feature matrix](DENGARIN_FEATURE_MATRIX.md) (FM-38) and [traceability](REQUIREMENTS_TRACEABILITY.md) (QG-7, V34).

*Status (2026-10-26, integration pass): guests. Reconcile revokes every session at a restore, and a guest's cookie is its only credential, so after any restore every guest loses access, and a guest deleted in the window cannot "delete it again": the guest purge removes that guest data on schedule (at most the inactivity period after the last activity the backup holds, 7 days by default, or at the maximum age). ADR-0002 §11 and [runbook §8](RUNBOOK.md#8-backup-and-restore) now state this; the guest notice says it in a new version, `guest-storage-2026-11-01.2`, and the restore notice of backlog S-262 criterion 3 includes it. The sentence was written in the integration pass; the PM confirms the guest text `guest-storage-2026-11-01.2` that holds it, and the `terms_privacy` sentence of `terms-privacy-2026-11-01.2` as written, including its second clause (every restore announced in the app and on Sela's status page) ([D-056](#d-056)); the risk acceptance above is unchanged.*

## D-056

2026-10-26 · PM decision (confirmations of points that the integration pass left pending) · **PM confirmations of the integration pass.** The PM confirms five points; each is decided:

1. **The restore sentence of the privacy text.** The `terms_privacy` sentence of `terms-privacy-2026-11-01.2` is confirmed as written, including its second clause, that Sela announces every restore in the app and on Sela's status page ([D-055](#d-055); [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) §11). The in-app notice and the status page do not exist yet (the notice is planned, [backlog](BACKLOG.md) S-262 criterion 3; the status page is part of S-263, which is blocked externally, B-3 and B-15) and are a gate for "ready for controlled pilot" ([roadmap §2](ROADMAP.md#2-release-ladder), [blockers, Gates](BLOCKERS.md#gates)). Until they exist, no restore of a pilot database may happen without a direct notice to every affected user through the app ([runbook §8](RUNBOOK.md#8-backup-and-restore)).
2. **The guest case of a restore.** A restore ends every guest's access, and a guest deleted in the window is removed by the guest purge (the status line of [D-055](#d-055); ADR-0002 §11; [runbook §8](RUNBOOK.md#8-backup-and-restore)). The `guest_storage` text `guest-storage-2026-11-01.2`, which says so, is confirmed ([backlog](BACKLOG.md) S-002 criterion 3, S-177).
3. **`golang.org/x/text` v0.42.0** is approved as a direct dependency, for Unicode NFKC normalisation ([ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) §1 and F-46). The confirmation names passwords and usernames. In the foundation code (`internal/account`) NFKC is applied wherever a password is set or verified (`CheckPassword`, sign-in and re-authentication) and in the password rule's comparison with the username; a username itself is ASCII only and refused otherwise (`NormalizeUsername`), so NFKC does not change it.
4. **The implemented gate of the foundation.** The open point of the [SPEC §G foundation status note](../SPEC.md#foundation-status-note-2026-10-26) is closed: the PM adds the acceptance criteria met to the description of draft PR #9, ADR-0002 F-01 to F-47 for the backend, with the web parts of F-17 (the password and recovery forms) and F-33 (the guest notice) following with the frontend PR. Read from GitHub on 2026-10-26: the description of PR #9 now says "Acceptance criteria met: ADR-0002 F-01 to F-47, backend part" and names both web parts. The [PR template](../.github/PULL_REQUEST_TEMPLATE.md) still has no such section ([traceability](REQUIREMENTS_TRACEABILITY.md#13-delivery-and-github-brief-12) DL-3).
5. **ADR-0002 §11 on guests,** written in the integration pass "for the backend owner to review", is accepted by the PM now; the backend owner may still refine its wording, and the meaning stays as item 2 states it.

Why: the integration pass wrote these points from the code (`internal/account`) and from D-055 and left them open for the PM; these confirmations settle them. Not part of them: the SPEC §G foundation status note as a whole, which the PM still confirms or withdraws, and the milestones given to S-262 criterion 3 (the in-app notice in P2, the status-page post in P7). Nothing becomes implemented, verified live or ready for controlled pilot by this entry: S-262 criterion 3 stays planned; S-263 stays blocked externally (B-3, B-15); S-177 stays in progress (the Pasal 21 element test, the per-provider texts and the legal review, B-10); and the web parts of F-17 and F-33 are not done. Changes: the [SPEC §G foundation status note](../SPEC.md#foundation-status-note-2026-10-26) (implemented gate); the [decision summary](DECISIONS.md); [implementation status](IMPLEMENTATION_STATUS.md) §1 to §3; [traceability](REQUIREMENTS_TRACEABILITY.md) S-177 and DL-3; the [backlog](BACKLOG.md) S-002, S-177 and S-262; [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) §1, §11, its consequences and F-46; the [development](DEVELOPMENT.md) guide; [runbook §8](RUNBOOK.md#8-backup-and-restore); the "ready for controlled pilot" gate in [roadmap §2](ROADMAP.md#2-release-ladder) and [blockers, Gates](BLOCKERS.md#gates); the status line of D-055. Closes [conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 64.

## D-057

2026-10-27 · PM decision (P4 contract open points) · **Consultation summary and sharing defaults.** The PM settles the open points CS-OP-1 to CS-OP-28 of the [P4 contract](api/V3_CONSULTATION_AND_SHARING.md#13-open-points-for-the-pm):

1. **The recommended defaults are adopted** as written in contract §13 for CS-OP-1 to CS-OP-24 and CS-OP-26 to CS-OP-28. In short:
   - share durations of 24 hours, 3, 7 or 14 days with none pre-selected;
   - the link and code shown once, with a sealed copy kept only for a 10-minute retry;
   - reauthentication before an account creates a share;
   - each share is its own consent record under a versioned `sharing` text, with no account-level purpose;
   - only the person sets the consultation statuses;
   - a crisis match never blocks and never triggers a share;
   - rejected AI wording is never shown or stored;
   - all six contact kinds are masked;
   - no device data about a recipient;
   - a restore revokes every active share;
   - deleting a source ends every active share that quotes it;
   - withdrawing guardian consent revokes a minor's shares;
   - the purpose "Lainnya" has no free text.
2. **CS-OP-25 (AT-4 drafting from whole selected sources) takes the narrower alternative for now.** Only `mode: "wording"` is offered: the person picks the quotes, and only those go to a provider. `mode: "sources"` may be built, but it stays behind a configuration switch that is off by default in every environment. It may be switched on only when three things hold: the legal review clears cross-border processing of that input (B-10, [D-049](#d-049)), the `ai_processing` and per-provider texts name it, and the PM records the change.
3. **The points marked legal** (CS-OP-4, CS-OP-13, CS-OP-21, CS-OP-25, CS-OP-27) are adopted as the interim rule and wait for B-10 before real users. AT-4 and AT-5 abstain outside development and test until the combined `ai_processing` text is published (CS-OP-13).

Why: the owner brief asks for sharing with consent per content, recipient and purpose, with expiry, revocation and an access audit. It also says to minimise what goes to providers and to process across providers only with consent. The recommended defaults follow the backlog criteria (S-090 to S-104, S-120, S-121, S-130, S-131), SPEC V30 to V34 and [D-041](#d-041) / [D-042](#d-042). Where a default sends more data across borders (CS-OP-25), the narrower option holds until the legal review. Nothing becomes implemented by this entry. Changes: the [P4 contract](api/V3_CONSULTATION_AND_SHARING.md) §13 marks each point decided; the P4 build follows it.

## D-058

**Superseded by [D-060](#d-060): the voice room and all voice/TTS surfaces were removed on 2026-10-29.**

2026-10-29 · Owner decision · **Experimental Amazon Nova 2.5 Sonic voice room (SPEC C27).** The owner asked for a full voice conversation with Amazon Nova 2.5 Sonic and, after being told the trade-offs, confirmed three points:

1. **Sonic generates the voice reply.** In the voice room only, AWS (not the Meta model on Modal) produces the answer. This narrows C18 and V35 for that room; the text talk room keeps Meta as the primary generator. Every Sonic output carries Amazon's label and is never counted or shown as Meta's (D-050).
2. **Indonesian is offered although Sonic does not officially support it.** AWS's service card lists English, Spanish, German, French, Italian, Portuguese and Hindi only and warns of reduced recognition accuracy and content errors in other languages. A live check on 29 Oct 2026 confirmed misrecognised Indonesian words ("mendangkarkanmu", "kapek"). The room and its consent say so plainly and are labelled experimental; the crisis router runs on every transcript regardless.
3. **IAM credentials, not a Bedrock API key.** `InvokeModelWithBidirectionalStream` refuses Bedrock API keys (verified: 403 "This operation does not support API Keys"; Sonic is not served on `bedrock-mantle`). A dedicated IAM user (`sela-voice`) holds a policy limited to invoking `amazon.nova-2-5-sonic` in us-east-1. Credentials live only in git-ignored environment files.

Safety defaults set by the engineering lead under this decision: `18_plus` only; separate consent `ai_provider_aws_voice`; assistant audio held until its text passes `safety.Assess` and `guard.Check` (health-claim flags withhold too, because no verifier runs on spoken replies); a crisis in a transcript ends the session at once and shows human help; one session per person; nothing stored; `SELA_VOICE_MODE=disabled` by default. Evidence: `internal/ai/sonic` (unit and opt-in live tests), `internal/httpapi/v3_voice*_test.go`, browser runs with a synthetic Indonesian utterance (reply played) and a synthetic crisis utterance (session ended, no assistant audio).


## D-059

**Superseded by [D-060](#d-060): the AWS and gorilla/websocket dependencies were removed with the voice features.**

2026-10-29 · PM decision · **Direct Go dependencies for the voice room and dictation.** Under D-058 (the Sonic voice room) and the Sonic dictation relay (BUG-6 second revision, backlog S-025), four direct requirements join the approved set of ADR-0002 §1: `github.com/aws/aws-sdk-go-v2` v1.47.5 (Bedrock stream core), `github.com/aws/aws-sdk-go-v2/credentials` v1.20.12 (static IAM credentials for the dedicated `sela-voice` user), `github.com/aws/aws-sdk-go-v2/service/bedrockruntime` v1.63.8 (`InvokeModelWithBidirectionalStream` on `amazon.nova-2-5-sonic`) and `github.com/gorilla/websocket` v1.5.3 (the voice and dictation sockets; already in use by the v2 realtime journey). Written reasons: [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) §1, whose F-46 row and the CI allowlist step in `.github/workflows/v2.yml` now list exactly these modules. AWS credentials stay server-side (D-058); no new indirect module is approved by this entry beyond the SDK's own requirements recorded in `go.sum`.

## D-060

2026-10-29 · Owner decision · **Text-only product; remove every voice and TTS surface and focus on Meta AI.** The owner reviewed the previous voice work (PR #15: browser read-aloud, the Amazon Nova 2.5 Sonic voice room, and the Sonic/Meta dictation relay) and decided the product should be text-only. Removed:

1. **Read-aloud TTS.** The `readAloud` display preference, its Settings switch and the per-reply "Bacakan / Read aloud" control. No browser speech synthesis remains in the v3 app.
2. **Amazon Nova Sonic voice room.** The `/talk/voice` screen, `GET /api/v3/voice/realtime`, `internal/ai/sonic`, `cmd/sela/voice.go`, the `ai_provider_aws_voice` consent, `SELA_VOICE_*` configuration and the voice-room tests.
3. **Dictation.** The composer microphone, `web/src/v3/asr.ts`, `GET /api/v3/asr/realtime`, `asr_meta`, `SELA_ASR_*` configuration and the dictation tests.

The direct AWS SDK and `gorilla/websocket` requirements are removed from `go.mod` and the CI allowlist; D-059 is superseded. Meta text generation through `internal/ai` stays the primary and only generative path, with the existing DeepSeek fallback and Claude verifier contract unchanged. No provider receives microphone audio, and no speech is synthesized. The legacy v1 `apps/web` runtime was out of scope for the voice removal; it was retired completely on 2026-10-29 (S-272), with its useful content ported to Go.

Why: the owner wants the Meta AI text experience to be the focus of the submission, and the voice stack added an AWS provider, extra consent purposes, realtime sockets and audio edge cases that are not part of that story. Changes: SPEC C25/C27/V43, `docs/API_SPEC_V3.md`, `docs/RUNBOOK.md`, `docs/META_INTEGRATION.md`, backlog S-025/S-026/S-027, ADR-0002 §1 and the v3 settings/talk UI.

## D-061

2026-10-30 · PM decision · **A separate operator trust domain and a durable, Redis-independent AI spend gate.** The operator console is not an extension of the anonymous user product:

1. **Staff identity is separate.** `admin_users`, `admin_sessions` and an append-only `admin_audit` (migration `0910_admin`) have no foreign key to `users` and no personal content. Staff are provisioned out of band (`sela admin create`), sign in with a password and TOTP, and use `__Host-sela_admin` (SameSite=Strict, HttpOnly); a consumer token cannot authorize `/api/admin/v1`. Roles are deny-by-default and enforced server-side.
2. **Redis is an optional public cache, never an authority.** Only the revisioned help-content catalogue is cached. A miss, outage or malformed value falls back to the embedded source; Redis never holds user text, tokens, consent grants or safety decisions, and cache invalidation appends an audit event.
3. **One new direct dependency.** `github.com/redis/go-redis/v9` v9.23.0 (go.mod `go 1.26.0`) is approved for the optional public cache only; its written reason is in [ADR-0002](docs/adr/ADR-0002-persistence-accounts-and-encryption.md) §1, and the CI direct-module allowlist and F-46 are updated to match.
4. **The AI budget is enforced in PostgreSQL.** `ai_spend` (migration `0500`) holds task/provider/token/cost metadata only. A transaction-scoped advisory lock serializes `Admit`, so concurrent instances cannot spend the same headroom; `Settle` records the measured estimate once and a failed operation settles at its reservation. A Redis outage or restart therefore cannot reset or bypass the cap. Admission failure returns `aiState: "budget_limited"` (or `"unavailable"`) with zero provider calls. A positive cap without a database refuses server start.

Scope: API and CLI only. The operator UI, directory/research workflows, two-person approval, provider-budget editing, WebAuthn, production Redis and live billing reconciliation remain open ([admin console](docs/vision/ADMIN_CONSOLE.md) §10, [Redis](docs/vision/REDIS_AI_COST_ENGINEERING.md) §11, tasks T23/T24).
