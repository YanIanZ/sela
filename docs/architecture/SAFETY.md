# Sela safety and privacy

Status: the **v3 safety design is planned**, apart from the privacy and account controls of the v3 backend foundation, which are **verified locally, and their CI run on draft PR #9 passed** (commit `0ff3341`; [SPEC §G foundation status note](../../SPEC.md#foundation-status-note-2026-10-26); marked below) (PM decision D-036, with D-030, D-031, D-044; [SPEC](../../SPEC.md) V3, V26, V30–V42, C19, C23). The **v2 behaviour below marked "implemented" is implemented and tested as software behaviour** (`internal/safety`, `internal/journey`, `internal/httpapi`); it is not a clinical validation or safety certification. Historical v1 controls are [archived](../archive/v1/SAFETY.md); the v1 defects v3 must not repeat are listed in the [feature matrix §4](../product/DENGARIN_FEATURE_MATRIX.md#4-v1-defects-that-v3-must-not-carry-over).

## Care boundary

Sela supports wellbeing and preparing for help (C23). It does not diagnose, prescribe, triage, score severity, give treatment advice, act as a therapist or provide consultations, and it never presents itself or its AI as a licensed professional or a health facility. Summaries are the user's own notes ("Catatan pribadiku", "Ringkasan untuk konsultasi"), never a medical record (C22). Recipients are people the user invites; their identity is not verified and the recipient page says so (D-042). A sent, opened or acknowledged share is not care received (V30). **Sela never contacts anyone automatically** — not a helpline, a guardian or a recipient (V38).

*v2 (implemented):* Sela prepares and simulates a handoff to **fictional** professionals. Fictional follow-up plans are fixture text attributed to a fictional professional; the model cannot write or change them. Every care surface carries a synthetic/simulated label in Indonesian and English, and receipts state that a sent or acknowledged request is not care received.

## Deterministic screening (implemented; kept in v3)

`internal/safety` assesses **every user-authored text before any other check, before it is stored and before it is sent to a model**: the story (as a whole and per sentence, in any mode), clarification answers, wording edits, opening lines, rehearsal attempts and pseudonyms. Length limits, contact-detail checks and approval requirements never pre-empt the crisis route. It is a lexicon ported from the Dengar.in crisis engine with the same rule ids, extended for clause-level explicit denial ("aku tidak mau bunuh diri" alone does not count, but "… tapi aku ingin mengakhiri hidupku" does), quotations and third-party framing (routed to *elevated* support information rather than ignored), slang, leetspeak and code-switching, and documented idiom exceptions. The exceptions are narrow: an accident or medication-adherence marker must sit next to the injury or dose and is ignored when the sentence also carries a risk, method, farewell or intent word (and never covers wrists, veins or the neck), and a lyrics or news frame covers only what it introduces, so a speaker who adopts the act is still crisis. Ambiguity takes the safer path. Each assessment records the ruleset version. Its limitations are listed in the package documentation and the test vectors.

- **Ruleset `sela-safety-2026.10.26-4`** (`8018372`) remains the historical baseline, checked by 1329 test vectors. Its last independent re-verification listed four major gaps: hope/wish phrasings with words between the verb and the death word, intensifiers in that position, present or future overdose plans, and emoticons or variant spellings in the accident-marker gate.
- **Ruleset `sela-safety-2026.10.26-5` is merged** (`99b60fd`, PR #8, 2026-10-29) and is the active lexicon. The corpus is **2728 vectors**; `go test -race -count=1 ./internal/safety ./internal/journey ./internal/httpapi` passes with database tests required. An independent subagent review returned **READY FOR UN-DRAFT** with no P0/P1 findings after fixing A (`kembali lagi` is a positive return), B (a later no-return wins over `tidak akan lama`/`sebentar lagi`), C (512-byte post-exemption lookahead with real 119/120/121/240-byte boundary tests), the denial skip-equivalence key, the `pernah`/`bisa`/`mungkin`/`ndak`/`kagak`/`tak kan` and `bangun` variants, and the relationship-forever precision cases. The `-4` to `-5` differential over the 1329 shared vectors is **0 lower, 0 higher**.
- **Known P2 precision limits:** a distant third-party no-return phrase inside the 512-byte window can still over-route to crisis, and some plural/adjectival relationship phrases can over-route. These err toward safety and are documented rather than blocking the merge. External 91/30/10/4 corpora are not in the repository and were not re-run; the committed vectors cover every reproduced class.
- No ML classifier is used (D-017); the model never decides routing. In v3 the lexicon is no longer the only safeguard (below).
- In v3 the same screening covers every new text surface: conversation turns, journal entries, check-in notes, memory corrections, summary edits and opening lines (V38).

## Crisis handling in v3 (planned, D-036)

What happens on a crisis match:

- **Nothing the user wrote is erased, and Sela does not navigate away.** Verified help appears inline, next to the user's words, with a link to the Immediate help page.
- **AI generation pauses for that conversation** until the user chooses to continue after seeing help. No provider is called on the crisis path (ADR-0003 RR-1), and crisis support never waits for a model.
- **The text is kept only if the user's storage choice allows it.** A conversation the user chose not to save stays unsaved.
- **Journal and check-in entries that match are saved** (with storage on) and shown with support. They are never refused, because refusing pushes people to soften what they write (v1 defect DX-02).
- **Elevated** matches (for example quoted or third-party framing) keep the flow going and show support information.
- Privacy actions — consents, export, deletion, revocation, sign-out — always stay available.

How this differs from earlier versions: v1 navigated to `/crisis` and lost the conversation (DX-03) and refused heavy journal entries (DX-02); v2 interrupts the whole session and discards the triggering text and the draft (below). v3 keeps the person in their conversation.

*v2 (implemented):* **crisis** → the session is interrupted: the triggering text and the draft are discarded, running AI operations are canceled and their event logs reduced to a content-free terminal event, and no further story, model call, approval, share, recipient transition, check-in or plan display is possible. The UI goes to Immediate help and every session page shows the interrupted state; revoking access and deleting data stay available. The help page lists verified Indonesian services with region, sources and verification dates and says that language does not determine jurisdiction. "Delete this session and start again" is offered. **elevated** → the journey continues and support information is shown. Routing outcomes are counted (content-free counters), never logged per request; the counters are operator-only (`SELA_METRICS_TOKEN`), so a visitor cannot watch them change in real time.

## Second safeguards (planned)

Safety routing does not rest on one classifier (owner brief §11):

1. The deterministic lexicon on every user text (above).
2. An always-visible **"Butuh bantuan sekarang" / "Need help now"** control on every screen.
3. **Quick exit** on every screen: one action to a neutral site without a back entry; it never deletes saved data (an unsent draft in the composer is lost, and an unsaved conversation follows V27); its Esc shortcut can be turned off (WCAG 2.1.4).
4. A **structured risk flag** (`none | elevated | crisis`) in every generator response that can only raise the route, never lower it (ADR-0003 RR-7).
5. The verifier's **`safety_signal`** issue, which withholds the output and shows inline help (ADR-0003 CK-9, VC-8).
6. **User review** of anything shared: nothing leaves Sela without the exact preview and the user's consent (V30).

Deterministic **hard guards** reject outright, whatever a verifier says: a diagnosis addressed to the user, a medication or treatment directive, a professional persona, exclusivity ("cuma aku yang ngerti kamu"), dependency cues, guilt or pressure (ADR-0003 RR-8, V26). A fallback provider is never used to get around a refusal or safety block (V35).

## Verified help resources

- *v2 (implemented, D-018):* only v1 entries marked `verified_official` for adults (Healing119, LISA Helpline, Yayasan Pulih), with their sources and access dates (2026-10-14/15), region "Indonesia" and a re-verify-by date (2027-01-14/15). No new numbers were added.
- *v3 (planned, D-044):* only entries verified from official sources, each with source, verification date and a re-verify-by date **that the server enforces**; a stale entry is flagged, never shown as current. One catalogue serves every surface. Teen-specific services need the same verification; the unverified KPAI WhatsApp from v1 is not shown (DX-06). Nothing is invented (V39).
- A read-only check of the cited pages on 2026-10-26 found changes (Healing119 hours and channels, LISA's cost wording, Yayasan Pulih's contact page and channel labels). They are listed with the owner actions in [B-5](../status/BLOCKERS.md); every channel must be re-verified by a person before any pilot.

## Minors (planned, D-031)

- Age band self-declared once; "under 15" stores nothing and shows help information; `unknown` gets the protective defaults of `15_17` (V37).
- Ages 15–17 are built but **off in production** until the legal review (B-10) confirms consent and verification (UU PDP Pasal 25; PP 17/2025 Pasal 9, 19, 21, 22) and the provider terms. 15-year-olds stay blocked until a low-risk assessment under PP 17/2025 Pasal 21(1)(b).
- Age-appropriate copy, with the limits of confidentiality explained before use.
- A guardian consent record gives the guardian **no** account, access, notification, report or export; nothing the young person writes is shared with a guardian by default; there is **no hidden monitoring**.
- Memory and pattern features are opt-in only (PP 17/2025 Pasal 19 profiling rules).
- No DeepSeek for minors, as generator or verifier (ADR-0003 §8.3).

## AI boundaries (planned, D-030)

- Meta generates (Meta Model API in development/demo, open-weight on Modal in production); Claude Sonnet 5.5 verifies; DeepSeek replaces the generator only on availability failures, with its own consent and only for `18_plus` (V35). The owner's fallback contract applies as well: the fallback is off by default, including in hackathon mode, never follows unsafe or invalid Meta output, a crisis, missing consent or a cancellation, and its result is rechecked before it is kept (C13, V19–V21, [DeepSeek fallback](DEEPSEEK_FALLBACK.md); ADR-0003 §17).
- AI summary wording, AI memory proposals, flagged health claims and every listening reply written by the DeepSeek fallback (ADR-0003 RR-8, §17 O-6) are verified before release; without a permitted verifier, Sela abstains and keeps the user's words, and DeepSeek is not called for a listening reply (V36; ADR-0003 RR-4 (i)).
- Providers receive only the segments a task needs, with contact details and names marked private replaced by placeholders, and no account identifiers (V41).
- No chain-of-thought is requested or stored; agreement between models is not clinical evidence.
- Every AI output names its provider; test-double output is never presented as live (V35).

## Consent, approval and sharing

*v2 (implemented):* four independent, versioned, server-enforced decisions: AI processing, storage (retention), brief approval (bound to one ledger version and content hash) and sharing (per recipient, selected fields, duration, current policy version). Any ledger change clears approval. A share can never outlive the session. The recipient needs the link (token in the URL fragment, removed from the address bar on load) **and** a separate access code; five wrong codes lock the share; one uniform error hides why access failed. Revocation, expiry, supersession and deletion end access immediately; the UI says already-viewed information cannot be recalled.

*v3 (planned):* AI consent splits per provider (Meta through Model API or Modal, Anthropic, DeepSeek), each naming provider, location and purpose before first use (C18, D-049). Sharing consent covers content, recipient **and purpose** (V30). Handoff statuses are `prepared → shared → opened → acknowledged → consultation_in_progress`, plus `revoked`, `expired`, `ended`. Every recipient view is audited, not only the first (V32). Revocation stops future access only; copies already downloaded or viewed cannot be recalled, and the UI and export say so (V31).

## Privacy

*v2 (implemented):*

- **What is processed:** story text the user selects or writes, clarification answers and edits; with AI consent, the sentences are sent to the Meta model on Modal (configured not to log requests; Modal processes data as infrastructure provider). The hosting platform processes IP addresses and timing.
- **What is stored:** only in server memory under the retention choice; restart erases everything. Practice text with obvious contact details (emails, phone-like numbers, 16-digit IDs, URLs) is rejected.
- **Identity:** anonymous by default or a pseudonym; not technical anonymity.
- **Logs:** route template, status, duration and request id; model call metadata (status, latency, token counts). Never story text, prompts, model output, tokens, access codes or credentials — verified by tests that search logs for canary strings.
- **Browser:** HttpOnly SameSite=Strict cookies, strict CSP (`default-src 'self'`, no inline scripts), `Referrer-Policy: no-referrer`, `frame-ancestors 'none'`. Only the interface language is kept in `localStorage`.

*v3 ([ADR-0002](../adr/ADR-0002-persistence-accounts-and-encryption.md), [ADR-0003](../adr/ADR-0003-ai-routing-verifier-fallback.md); planned unless marked verified locally, which means the backend foundation `0ff3341` with no screen yet):*

- **Where data is stored** (verified locally; hosting blocked externally, B-3, B-15): PostgreSQL on Sela's server, in a region the owner chooses (B-15); personal content encrypted per user; the server can decrypt it to show it to the user and run the features they choose.
- **Who processes it:** the hosting provider; with consent, Modal (US) for the Meta model, Anthropic (US) for verification, DeepSeek (PRC, or Modal us-west under B-9) for the fallback. Each provider's retention is stated before consent (ADR-0003 §8.1).
- **Identity** (backend verified locally): a guest (the browser cookie is the only key) or a username account; no real name, email or phone number; never absolute anonymity (C17).
- **Unsaved conversations:** held only in server memory while open, never in the database, backups or logs, and purged at End, at session end, after 30 minutes without activity or 2 hours at most, or on a server restart (V27, D-051).
- **Deletion and backups** (verified locally, apart from the restore announcement, which is planned; hosted backups blocked externally, B-3, B-15): deleting the account removes every owned row at once and destroys the data key; no backup holds a usable key after 7 days and the ciphertext ages out after 30 days. A restore honours every deletion recorded in the newest key dump; an account deleted after it (at most one hour before a total loss of the live database) can come back with the restore until a deletion journal exists (needs B-3). A restore also ends every session, so guests lose access to their guest data, which the guest purge then removes. The privacy text and the guest notice say so; the announcement of every restore in the app and on a status page, which the privacy text promises, is **planned** (backlog S-262 criterion 3). The risk is accepted for a controlled pilot and revisited before a public release (V34, D-055).
- **Operator access:** operator commands that reach personal data or keys, and operator changes to a user's record, write a content-free operator audit record; a change to one user's record also appears in that user's audit; no tool shows an operator a user's content (V32, D-052).
- **Logs** (verified locally for the foundation routes, F-41): additionally never usernames, passwords, recovery codes, user ids, guardian names, keys, database URLs or IP addresses (ADR-0002 §13).
- **Browser:** no secret or personal content at rest in browser storage; only non-sensitive preferences (v1 defect DX-04).

## Abuse and cost bounds

*v2 (implemented):* per-address token buckets (IPv6 by /64, least-recently-used eviction, never failing closed) for session creation, AI starts, recipient sign-in and other requests; at most 50 live sessions per address by default; per-session caps on ledger items, story size and handoffs; at most 3 recipient principals per handoff; per-session AI operation limit; global admission semaphore; Modal proxy authentication before any GPU starts; at most one GPU container; scale to zero after five minutes.

*v3 (verified locally with the backend foundation, `0ff3341`):* per-username login limits with exponential backoff and a lock after 100 consecutive failures until recovery; uniform failures that do not reveal whether a username exists at login (a small timing difference remains for a while after the argon2id parameters change; rate limits apply; accepted for a controlled pilot, D-055); a bounded password-hashing semaphore; a cap on live guests (ADR-0002 §9). *Planned:* per-user AI limits and daily spend ceilings per provider (ADR-0003 §11).

## Open points

- Whether a crisis match changes what the user may share afterwards is specified with the P4 sharing contract; an automatic request or share is never created (V3).
- How the API tells 15-year-olds apart from 16–17-year-olds while only `15_17` exists ([conflict matrix](../status/CONFLICT_MATRIX.md#v3-owner-brief-v3-vs-the-v2-spec-and-code)).

Tests demonstrate this software behaviour on synthetic data only. Private security reports: [SECURITY.md](../../SECURITY.md).
