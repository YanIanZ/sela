# Decision log

Chronological, append-only record of Sela decisions: v2 implementation decisions D-001 to D-024, the v3 decisions D-025 to D-049 of 2026-10-26 and the v3 engineering defaults from D-050. Authority: [SPEC.md](../SPEC.md); the [decision summary](DECISIONS.md) lists origins. Each entry states origin, the decision, why, and what it changes. Origins: **owner direction** (user-confirmed; changing it needs new owner direction), **PM decision** (taken by the delivery lead under the authority of owner brief v3; recorded as decided and revisable by the owner) and **engineering** (revisable with rationale). Conflicts that led to entries are in the [conflict matrix](CONFLICT_MATRIX.md).

Entries are never rewritten. When a later decision replaces an earlier one, the earlier entry keeps its text and gets a *Status* line that points to its replacement. Superseded decisions still describe the v2 code until that code changes.

## D-001

*Status (2026-10-26): the clause "V9 is unchanged: Meta is the only AI provider; no fallback provider exists" is superseded by [D-025](#d-025) and [D-029](#d-029) (Meta primary, Claude verifier, DeepSeek fallback for availability failures only). The rest stands: Meta open-weight on Modal remains the primary generator.*

2026-10-26 · Owner direction · **AI runs on a Meta open-weight model self-hosted on Modal.com**, replacing the earlier hosted "Meta Model API" plan. V9 is unchanged: Meta is the only AI provider; no fallback provider exists. Model and serving details: [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md).

## D-002

2026-10-26 · Engineering · **Hackathon tooling eligibility is recorded as unverified.** The Official Rules name "Glimmer" and the event page says "run Meta open weights models", but "Meta AI tooling" has no formal definition. No document or UI claims eligibility or a judging advantage.

## D-003

*Status (2026-10-26): superseded by [D-025](#d-025) (real use, ages 15 to 50+), [D-034](#d-034) (conversation room), [D-030](#d-030) (age bands) and [D-026](#d-026) (persistence). Practice text remains a v2 demo behaviour of `/api/v2`.*

2026-10-26 · Owner direction (story room with text input) within the kept synthetic boundary · **Practice text mode.** Besides seeded scenarios (still the default demo path), the user may type a *fictional practice story* (20–3000 characters). Boundaries: adult attestation, explicit "do not enter real names, contacts or health history" notice, deterministic crisis gate before storage or inference, rejection of obvious contact details (email, phone-like runs of ≥9 digits, 16-digit ID numbers, URLs), memory-only storage under the retention policy, no content in logs. This amends SPEC V2; it does not make Sela a real intake service.

## D-004

2026-10-26 · Owner direction · **Four independent decisions:** AI processing consent, storage (retention) consent, brief approval and sharing consent. Each is versioned against a policy text version, server-enforced, and none implies another. Withdrawing AI consent cancels running operations and discards their results.

## D-005

*Status (2026-10-26): superseded by [D-026](#d-026), [D-027](#d-027) and [D-028](#d-028) (persistent encrypted storage, guest purge after 7 days of inactivity or 30 days, backup policy in ADR-0002 §11). It still describes the v2 memory store.*

2026-10-26 · Engineering · **Retention.** Without storage consent: deleted after 30 minutes idle and 2 hours at most. With storage consent: kept in memory up to 24 hours. "Delete everything" purges the session, its shares and recipient access immediately. A restart erases all state and the UI says so. Enforced by store expiry checks on every access plus a one-minute sweeper.

## D-006

2026-10-26 · Engineering · **Identity.** Anonymous by default; optional pseudonym (2–24 letters, no digits/@). The privacy page states that anonymity is not technical anonymity (hosting sees IP/timing; Modal processes AI requests). No real identity is collected.

## D-007

2026-10-26 · Owner direction (token + recipient authorization) · **Two-part recipient access.** A share creates a 256-bit capability token (delivered as a URL fragment, never sent to the server in a URL) and a separate 8-character access code. The recipient principal (its own HttpOnly cookie) is created only when both match, is bound to exactly one handoff of one owner session and expires with the share. Five wrong codes lock the share. Fictional recipients are per owner session, so there is no cross-visitor inbox. All failures return one uniform `access_ended` error. This replaces the same-session "role switch" of the earlier spec.

## D-008

*Status (2026-10-26): superseded by [D-040](#d-040) (prepared → shared → opened → acknowledged → consultation_in_progress, plus revoked, expired, ended). The rule that "sent" is never "care received" is kept.*

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

*Status (2026-10-26): superseded by [D-026](#d-026) (PostgreSQL 16 is the system of record; the memory store is allowed only in development, demo and test).*

2026-10-26 · Engineering (keeps SPEC C7) · **Storage:** `journey.Store` interface with one bounded in-memory implementation (`internal/store`). No database, no disk persistence of session data.

## D-015

2026-10-26 · Owner direction · **Research corpus pipeline** (Python, standard library) rebuilds the 100-record register with DOI normalisation, de-duplication, multi-source date verification, a rolling five-year cutoff (freeze date minus five years), licence/access status, screening and conservative evidence fields (null when unknown). Discrepancies with earlier documents are reported, not hidden. See [corpus pipeline](research/CORPUS_PIPELINE.md).

## D-016

2026-10-26 · Engineering · **No fine-tuning.** None of the prerequisites holds: no measured baseline gap, no licensed task dataset, no train/validation/test split of real-distribution data, and synthetic fixtures are too few. Retrieval at inference is also not used: product answers never need literature, and evidence is mapped to features statically. A reviewer-only corpus search is provided.

## D-017

2026-10-26 · Engineering · **No ML safety classifier in v2.** The deterministic lexicon (ported from Dengar.in, extended for clause-level negation, quotes, third-party framing, slang and code-switching) is the gate. Llama Guard 4 was considered: gated, extra GPU cost and no verified Indonesian performance. A classifier would never be the only safeguard.

## D-018

*Status (2026-10-26): superseded by [D-043](#d-043) (verified entries only, with a re-verify-by date the server enforces; teen resources need the same verification).*

2026-10-26 · Engineering · **Help resources:** only v1 entries marked `verified_official` for adults (Healing119, LISA Helpline, Yayasan Pulih) with their sources and access dates (2026-10-14/15), region "Indonesia", and a re-verification date. No new numbers were added.

## D-019

*Status (2026-10-26): superseded by [D-029](#d-029) and [D-039](#d-039). The four draft values stay; `live_deepseek` and new verification values are added (ADR-0003 §10), and record items also carry a provenance class.*

2026-10-26 · Engineering · **Provenance values** `live_meta`, `recorded_demo`, `manual`, and new `test_double`. `live_meta` requires the configured Modal provider and a server-reported model equal to the pinned served name. `test_double` (cmd/fakellm) is allowed only outside production and is shown with a banner. No recorded results exist yet; none will be fabricated.

## D-020

*Status (2026-10-26): amended for v3 under [D-025](#d-025): the T9 removal waits for v3 acceptance instead of v2's (backlog S-272, [roadmap §5](ROADMAP.md#5-cutover-and-removal-of-v1)); v1 stays runnable and tested until then.*

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

2026-10-26 · Owner direction (owner brief v3, user-confirmed) · **Owner brief v3 supersedes the v2 limits synthetic-only, adults-only, memory-only and Meta-only.** Sela is a real-use health and wellbeing app for ages 15 to 50+ in Indonesian and English, and listening is its core. Synthetic data is for tests, evaluation and the demo only. The safety controls stay. Status reports separate implemented, verified locally, verified live, ready for controlled pilot and public-release blockers (SPEC §G). Spec changes: C3, C7, C8 and C9 superseded and C12 in part; V1, V2, V9, V10, V16 and V21 superseded with replacements; I.data, I.meta, I.api and I.web revised; I.auth, I.ai, I.storage and I.export added; new constraints C15 to C25 and invariants V24 to V40. Superseded decisions: the "no fallback" clause of D-001, D-003, D-005, D-008, D-014, D-018 and D-019.

## D-026

2026-10-26 · PM decision · **PostgreSQL 16 (pgx v5) is the system of record**, with Sela's own embedded, checksummed migrator, reserved migration ranges and data-migration gates (backup, dry run, ownership validation, rollback). The memory store is allowed only in development, demo and test. Why: durable, user-controlled notes across sessions, transactional deletion and restore drills; SQLite and memory-only were rejected ([ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md)). Changes C7 → C15, I.storage. Supersedes D-014.

## D-027

2026-10-26 · PM decision · **Per-user envelope encryption.** Each user's content is sealed with AES-256-GCM under a per-user data key, wrapped by a key-encryption key from the secret manager, with additional data that binds each ciphertext to its user, record and field. Account deletion destroys the data key (crypto-shredding), so content in older backups becomes unreadable once the 7-day key backups expire ([ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) §5, §11). Changes C15, V32.

## D-028

2026-10-26 · PM decision · **Accounts and guest mode.** Username and password (argon2id) with 10 single-use recovery codes; server-side sessions; guest mode stored server-side with a stated location and duration (default deleted after 7 days of inactivity, at the latest 30 days after creation), never promised as absolute anonymity. No email until a provider exists ([B-11](BLOCKERS.md)). Implements owner brief §10 (authentication and recovery; guest mode explains location and duration). Changes C16, I.auth, V25; supersedes D-005 together with D-026 and D-027.

## D-029

2026-10-26 · PM decision implementing owner brief §8 (the provider roles are user-confirmed) · **AI routing policy** ([ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md)). The Meta open-weight model on Modal (pinned Muse Glimmer 30B unless the owner decides B-7) is the primary generator for every generative task: listening replies, "Sela menangkap…" reflections, memory proposals, summary drafts and opening lines. DeepSeek (`deepseek-flash`, serving `DeepSeek-V4.1-Flash-1017`; the served version is recorded) replaces the generator only when Meta is unavailable (cold start beyond budget, 5xx, timeout, capacity, an authentication failure on Sela's side) and only with the separate DeepSeek consent, which explains processing in the PRC; never after a refusal, a safety block, a Meaning Lock rejection or any content-based failure; without consent the honest no-AI path remains. Claude Sonnet 5.5 (`claude-sonnet-5-5`, Messages API, structured output) verifies before release: AI consultation-summary wording, AI memory proposals and generated text flagged by the deterministic health-claim detector; reading content is verified at authoring time. Verdicts are supported / unsupported / contradicted / insufficient_evidence with a short reason and the source span checked; no chain-of-thought is requested or stored; Claude needs its own consent (processing in the US). DeepSeek may be the alternative verifier only when Claude is unavailable, Meta was the generator (a model never verifies its own output), the user consented and a default-off flag is enabled after evaluation; its verdicts are labelled. With no permitted verifier, verifier-gated outputs abstain (the user's words are kept, the manual path offered, and the UI says so); ordinary listening replies pass deterministic guards and are released without the verifier unless flagged. Crisis support is deterministic and never waits for a model. Data is minimised (needed segments only, contact details and private names redacted, no account identifiers). Every output records provider, model id, served version, route, verdict and latency. Fallback never gets around a safety refusal. Withdrawing a provider's consent stops its use at once. Changes C12 (in part) → C17, I.ai, V9 → V33, V10 → V33, V16 → V34 and V39. Supersedes the D-001 clause and, with D-039, D-019.

## D-030

2026-10-26 · PM decision implementing owner brief §6 · **Ages 15–17 are implemented but gated.** Age band at onboarding, age-appropriate copy, confidentiality limits explained, a guardian consent record that grants the guardian no access, teen help resources, memory and pattern features opt-in only for minors (PP 17/2025 profiling rules), no hidden monitoring. Disabled in production by configuration until a legal review confirms consent and verification under UU PDP Pasal 25, PP 17/2025 (Pasal 9, 21, 22) and the provider terms (Anthropic minors guidance; Meta Muse Glimmer "not intended for individuals under the age of 18", with a deployer risk duty; DeepSeek guardian-consent wording). 15-year-olds stay blocked until the product is assessed as low risk under PP 17/2025 Pasal 21(1)(b). Changes C8 → C18, V35; blocker B-10.

## D-031

2026-10-26 · PM decision · **Frontend stays React 19 + TypeScript + Vite**, the tested and accessible v2 baseline; the information architecture, interaction design and visual design are redone by the design lead (owner brief §4: the design agent chooses the visual direction). Changes C6 (note), I.web.

## D-032

2026-10-26 · PM decision implementing owner brief §2 · **Content separation.** Public content (reading library, help directory), personal data (owned by one user, linked by ownership and consent), community content (none in the pilot), demo data (synthetic, demo environment only, labelled) and the research corpus (a separate store, never linked to any user's history). Another person's story or a public article is never copied into a user's history; a reading item can be bookmarked by reference. Changes C20, I.data.

## D-033

2026-10-26 · PM decision · **Delivery branch** `feat/sela-v3-pilot`, a stacked draft PR on top of PR #6 (`feat/v2-go-meta-runtime`). Safety ruleset `-5` lands on PR #6 first and is merged forward (SPEC T22).

## D-034

2026-10-26 · PM decision implementing owner brief §5 · **Listening is the core.** The conversation room ("Ruang cerita" / "Talk") lets a person start talking without a profile. An optional intent picker (just listen / sort my thoughts / understand patterns / prepare a consultation) adjusts the reply style and is never a gate. Replies are contextual, with no mechanical empathy templates, no judgement, no diagnosis, no professional persona, no exclusive relationship and no dependency cues; at most one question per reply; suggestions are offered, never pushed; one action ends the conversation. Each conversation has a storage choice (save / don't save; asked at the start in plain language and changeable). The user can correct or reject Sela's understanding ("Sela menangkap…" cards are editable, confirmable and removable). Seeking a professional is never required. Changes C9 → C24, V24, V25.

## D-035

2026-10-26 · PM decision implementing owner brief §11 · **Crisis handling in v3.** Every user text is screened deterministically before storage and before any model call (the existing lexicon; ruleset `-5` pending). A crisis match never erases what the user wrote and never hard-redirects away from the conversation: it shows inline verified help (and the Immediate help page), pauses AI generation for that conversation until the user chooses to continue after seeing help, and keeps the text only if the storage choice allows it. Journal and check-in entries that match are saved (if storage is on) with support shown, not refused. Second safeguards beyond the lexicon: an always-visible "Butuh bantuan sekarang" control, Quick Exit, a structured risk flag from the generator that can only raise (never lower) the route, the verifier's check, and user review of anything shared. Sela never contacts anyone automatically. Why: v1 navigated away and lost the conversation and refused heavy journal entries (feature matrix DX-02, DX-03); v2 interrupts the whole session. Changes V3 (note), V36.

## D-036

2026-10-26 · PM decision implementing owner brief §5 · **No engagement pressure.** No streaks, points, levels, badges, guilt notifications or push notifications in the pilot. Optional neutral reminders only if the user turns them on (not in pilot scope). Continuity is shown as a calm history, never as a score. Changes C19.

## D-037

2026-10-26 · PM decision implementing owner brief §2 (no silent cuts) · **Dengar.in features not migrated to the pilot, with reasons.** Retired: the severity-scored assessment and "professional pathway" labels (triage, forbidden by brief §11; the context questions merge into the optional intent picker); gamification (engagement exploitation); the 14-day intervention path (a stub, and prescriptive); the OpenRouter "Second Brain" debiaser (a free-tier third party may train on the data; the verifier covers bias and distortion); the AMD provider (removed earlier); the AI response cache; Supabase and forum persistence (replaced by the v3 schema). Paused, kept in v1 code and reopened as backlog items with prerequisites: the anonymous forum "Ruang Cerita" and its moderation (needs moderation operations, author deletion, a minors policy and labelled seed content) and the Gemini voice room (provider outside the v3 plan; autoplay). Voice dictation migrates as an opt-in, with a disclosure that the browser's speech service may send audio to its vendor, an editable transcript before sending and no autoplay; read-aloud through the browser's local speech synthesis is opt-in. The E2EE cloud backup merges into encrypted server persistence plus export (its recovery-phrase idea informs the recovery codes). The daily micro-missions merge into Reading & support as optional "latihan kecil" without day counters. Details per feature: [Dengar.in feature matrix](DENGARIN_FEATURE_MATRIX.md). Changes C9 → C24.

## D-038

2026-10-26 · PM decision implementing owner brief §7 · **Honest terms.** "Catatan pribadiku" (My personal notes) and "Ringkasan untuk konsultasi" (Summary for a consultation). Never "rekam medis", "diagnosis", "hasil pemeriksaan" or "assessment result". Summaries say they are the user's own notes, prepared with AI help where applicable. Changes C21.

## D-039

2026-10-26 · PM decision implementing owner brief §7 · **Provenance classes.** Every important record item carries a class — `user_original`, `ai_summary`, `user_correction`, `unconfirmed_interpretation`, `uploaded_document` (reserved; uploads not in the pilot), `verified_professional_note` (reserved; no verified professionals exist) — plus created and updated time, version and review status (`unreviewed`, `confirmed`, `corrected`, `rejected`). Changes V26, I.data; with D-029 supersedes D-019.

## D-040

2026-10-26 · PM decision implementing owner brief §7 · **Handoff statuses** `prepared → shared → opened → acknowledged → consultation_in_progress`, plus `revoked`, `expired` and `ended`. A sent or opened share is not care; no status implies that treatment was given. Changes V28. Supersedes D-008.

## D-041

2026-10-26 · PM decision implementing owner brief §7 · **Recipients in the pilot** are people the user invites themselves (their own professional), through a link plus a separate access code; the recipient's identity is not verified and is labelled so. No directory of verified professionals, no booking and no partnership claims. The v2 fixture professionals stay for the demo configuration only. Changes C21, V8 (note), V28.

## D-042

2026-10-26 · PM decision · **Uploaded documents are deferred** (not in the pilot): validation, malware scanning and storage need their own design. The `uploaded_document` provenance class is reserved. Changes C24, V26.

## D-043

2026-10-26 · PM decision implementing owner brief §11 · **Help resources.** Only entries verified from official sources are shown, each with source, verification date and a re-verify-by date that the server enforces (stale entries are flagged, not silently shown). Teen-specific resources need the same verification; the unverified KPAI WhatsApp contact from v1 is not shown. Why: v2 checks only the date format (`internal/fixtures/fixtures.go`), and v1 showed an unverified contact to teens (feature matrix DX-06). Changes V37; blocker B-5. Supersedes D-018.

## D-044

2026-10-26 · PM decision · **WHO self-help cards.** The 22 WHO-adapted cards (CC BY-NC-SA 3.0 IGO) may be used in a non-commercial pilot with the required attribution and adaptation notice; any commercial launch is blocked until they are licensed or replaced ([B-14](BLOCKERS.md)).

## D-045

2026-10-26 · PM decision implementing owner brief §11 · **Environments** development, demo, staging and production (plus test). Production refuses the memory store, test doubles, demo fixtures and missing secrets. The demo uses synthetic data with every feature really working (test doubles labelled, or live providers on synthetic data). Changes C23, V40 ([ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) §12).

## D-046

2026-10-26 · PM decision (risk record) · **Legacy Dengar.in deployments.** Two legacy deployments are live outside this repository (`dengar-in.vertalt.my.id` and `dengar-in.vercel.app`; health checks returned 200 on 2026-10-26) and may process real users' text with DeepSeek, Gemini and a free-tier OpenRouter model. Decommissioning or a notice is an owner/team action ([B-13](BLOCKERS.md)).

## D-047

2026-10-26 · PM decision implementing owner brief §9 · **Research and evidence.** The 100-record corpus pipeline is kept (verified reproducible by the discovery research audit of 2026-10-26: the 74 offline `tests/research` tests pass, `corpus_pipeline.py verify` reports 0 violations, and an offline rebuild from the local cache is byte-identical to the committed outputs). It is title/abstract-screened, not appraised. v3 needs new evidence for listening, journaling, consultation preparation, accessibility, adolescents' consent and confidentiality and adults 50+ (the register has no direct record on listening, journaling, consultation preparation or accessibility); the gaps are recorded and researched without paywall bypass. No fine-tuning is approved; user notes are never used for training. Changes C14 (note); SPEC T19.

## D-048

2026-10-26 · PM decision · **Cross-border processing disclosure.** All three AI providers are cross-border processors (Modal and Anthropic in the US, DeepSeek in the PRC). The app discloses each provider, location and purpose in plain language before consent; the legal review (DPIA, processor agreements or explicit consent under UU PDP Pasal 56) is a pilot blocker ([B-10](BLOCKERS.md)). Changes C17.

## D-049

2026-10-26 · PM decision implementing owner brief §11 · **Hackathon.** The Meta hackathon rules neither allow nor forbid Claude or DeepSeek. Keep a Meta-only demo path, label every output's provider, never count Claude or DeepSeek as Meta tooling, and ask the organizers (owner action, [B-12](BLOCKERS.md)) before the submission deadline (2026-11-04 21:00 ET). Changes C12 (note), C17, V33.

## D-050

2026-10-26 · Engineering (spec default under D-034; settles [design direction §10](design/DESIGN_DIRECTION.md#10-open-points-for-the-pm-and-other-owners), "don't save" conversations) · **An unsaved conversation lives only in bounded server memory.** While a "don't save" conversation is open, its turns are held in server memory for that owner and conversation only, bounded in count and size, never in the database, backups, logs or browser storage, and used only as context for that conversation's replies (ADR-0003 AT-1). They are purged at End, at sign-out or session end, after 30 minutes without activity or 2 hours in total (whichever comes first), and on a server restart; within that window the same session can reopen the conversation after a reload. Why: the reply needs the earlier turns, but "don't save" must leave nothing behind when the person walks away (tab closed, crash, idle), and memory-only holding keeps the text out of backups, so no backup deletion path is needed. The 30-minute and 2-hour bounds reuse the v2 retention without storage consent (D-005). The hold is per instance; the pilot runs one instance, and backlog S-268 revisits it before scaling out. "Discarded" in the UI covers only what Sela holds; provider retention is disclosed before consent (V32). Changes V25.

## D-051

2026-10-26 · Engineering (spec default; discovery gap analysis of 2026-10-26: operator access must be logged) · **Operator access is audited.** Every operator command that can reach personal data or keys (`sela keys`, such as a rewrap; `sela restore` and its reconcile; the restore drill's canary decryption; data migrations) and every operator change to a user's record (such as withdrawing a guardian consent, ADR-0002 `withdrawn_by = 'operator'`) writes an append-only, content-free operator audit record: command, environment, operator identity, time, outcome and counts; never content, user ids, keys or IP addresses. An operator change to one user's record also appears in that user's own audit (V30). Direct database sessions in staging and production are recorded by the database host's access log, which depends on the hosting choice (B-3). No route or tool shows an operator a user's decrypted content. Why: V30 covered only owner-visible events, so key operations and support access would otherwise leave no trace, and Sela holds specific personal data (UU PDP Pasal 4(2)(a)). Changes V30.
