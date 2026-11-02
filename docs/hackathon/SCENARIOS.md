# Synthetic scenario library A–G and acceptance scripts

Status: **test design, not test results**. Never use real patient/student stories, real phone numbers, staff credentials, or actual clinical appointments. Core user-facing claims must be tested on the same pinned commit as the submitted demo. Related [MIE](MIE_IMPLEMENTATION.md), [QA](QA_MATRIX.md), [demo](DEMO_AND_SUBMISSION.md).

## Common setup

- Environment: isolated local or controlled demo/staging tenancy; never seed arbitrary fictional users into live real-user production.
- Model: D-065 Meta-only, generator Spark and verifier Wonder when configured; record provider model IDs and true provenance. Tests may use a labelled offline double; that is **not** live Meta evidence.
- Data: synthetic profiles and records with deterministic IDs generated at test initialization; no retention beyond test teardown. Prefix or metadata `synthetic=true` where schema permits, and visible label “Simulasi / Synthetic”.
- User session: anonymous/guest where possible; current signup/login UI may still exist and must not be described as already removed.
- Controls: explicit AI processing, storage, memory and sharing consent, independently evaluated. A denied consent never causes a silent provider switch.
- Oracle: backend record state and API response, **not** merely the visible UI; tie every result to commit SHA, environment and fixture hash.
- Each scenario defines success, failures, audit artifacts, and a step-by-step replay video bookmark.

## Scenario A — Listening that preserves agency [P0; core impact]

**Persona:** Nadia (fictional adult student, Bahasa Indonesia). **Need:** “Aku ingin didengar, belum siap mencari bantuan profesional.” **Value:** responsive listening that does not turn feelings into diagnosis or a compulsory care pathway.

**Given:** fresh anonymous/guest session, AI consent accepted for Meta, storage default not assumed, safety test fixture benign. **When:**
1. Open the landing page in Indonesian on a 390px phone; review privacy information and start a talk without creating a mandatory profile.
2. Select *listen* intent. Type “Akhir-akhir ini aku capek sesudah kuliah. Kadang ingin sendiri, tapi aku tetap suka bersama teman.”
3. Observe Meta reply/provenance and reflection suggestion.
4. Choose “don't save”; end session; verify no durable personal conversation persisted.
5. Repeat with “save this conversation” and explicit separate storage consent, verify only the selected data is saved.
6. Return to history; request an on-demand reflection card and view its original quote and provisional status.

**Then:** reply refers to one concrete detail, retains “sometimes / kadang” and “still likes friends”, never diagnoses or claims to be a therapist, does not assert unapproved memory, and can fail gracefully. Manual journaling remains available when Meta is off.

**Negative paths:** timeout; consent never granted; no verifier; stale hold; second client tries accessing another owner's conversation. **Evidence:** transcript synthetic-only, HTTP model provenance, DB no-save assertion, UI accessibility. **Pass:** every declared state and prohibition verified.

## Scenario B — Meaning correction propagates [P0; flagship technical depth]

**Persona:** Nadia. **Seed:** Source S@v1 includes “Kadang aku ingin sendiri, tapi bukan berarti membenci teman”. A deliberately bad **test fixture**, not a fabricated live Meta response, proposes C@v1 “Nadia menghindari semua teman”. A dependent approved memory M, summary T and share P exist in the test graph where allowed.

**When:**
1. View original vs proposed interpretation with visibly labelled simulated error.
2. Reject C or correct to “sesekali butuh waktu istirahat sendiri, tetap suka bertemu teman”.
3. Confirm rejection/correction; inspect lineage/impact with source IDs and versions.
4. Open personal memory and consultation summary; verify affected versions are marked stale or blocked from reuse pending review.
5. Attempt export of obsolete summary; expect conflict/stale denial.
6. Attempt open of impacted frozen share *after correction commit*; expect revoke/stale error, not old authoritative content.
7. Edit the original again while a Meta request is in-flight; ensure delayed response is discarded.

**Then:** rejected claim is not used as trusted fact, all graph descendants transition in one DB transaction, old versions remain owner-visible as history, and no new recipient access succeeds after revoke/invalidation. Human can explicitly reapprove a **new** version and share again.

**Negative paths:** source quote mismatch, edited stale expectedVersion, cycle in graph, failed n-th descendant, retrying idempotency key, revoked consent, re-entrancy. **Pass:** zero reintroduction of rejected claims or fresh reads of invalidated projections in pinned concurrency suite. **Caveat:** cannot delete any already-read recipient copy.

## Scenario C — Selective consultation preparation [P0; practical impact]

**Persona:** Sam (fictional adult with shift work; English). **Need:** prepare for an initial professional conversation without giving full journal access.

**Given:** two approved notes and one deliberately unrelated private note; a synthetic directory and fictional recipient; no real partner.

**When:**
1. User opens support, searches/selects a profession or manual route without GPS.
2. Build a summary from only the two selected source versions; display source-backed sentences.
3. Edit an item, require explicit approval; export HTML/Markdown after approval.
4. Create a frozen share; reveal link and separate access code **to the user once**.
5. In a separate synthetic recipient session, enter link + code, view only the approved projection and acknowledge receipt.
6. As owner, view access events; revoke share.
7. Recipient attempts reopen; expect denied. Prepare after-consultation notes in the owner area with no claim of real care being delivered.

**Then:** unrelated note and private names never appear in recipient text, operators cannot read journals, share logs contain no token/PII, “opened” does not imply licensed identity or treatment received. **Negative paths:** missing code, expired share, wrong code, source version drift, altered consent, no directory matches. **Pass:** exact field allowlist snapshot and revocation verified server-side.

## Scenario D — Consent, anonymity and no-AI mode [P0; trust]

**Persona:** Dimas (fictional adult; code-switches ID/EN). **Given:** a shared-device threat model, no AI consent, optional storage not granted.

**When:**
1. Enter as guest; turn on hidden previews, neutral page title and reduced motion.
2. Decline Meta processing; type a note, save only according to storage permission (or keep in temporary draft).
3. Verify **zero** outbound Meta calls; confirm no prompt/story in Redis or logs.
4. Grant AI processing in an explicit versioned consent screen; generate one reflection.
5. Revoke AI processing while another operation is queued. Confirm no post-revocation generation release and no silent fallback.
6. Use quick exit and revisit; check session boundary and local preview behaviour.

**Then:** consent is scoped, revocable and never bundled; guest mode does not falsely promise network anonymity. **Negative paths:** stale consent version, third-party provider, tab sharing, browser back button, session expiry and old data export. **Pass:** consent ledger and provider-spy counters.

## Scenario E — Meta unavailable / verifier fails / budget cap [P0; reliability]

**Persona:** Nadia. **Given:** valid input but controlled injection of (a) Meta 429, (b) timeout, (c) malformed JSON, (d) verifier refusal, (e) PostgreSQL budget-denial, (f) Redis unavailable.

**When:** run each condition in isolated deterministic test (never fault-inject public production).

**Then:** client receives specific neutral unavailable/budget message; preserves user draft where appropriate; never exposes unvalidated model output, keys, prompt, stack traces, hidden fallback or false success. No more than configured bounded retries; budget remains enforced despite Redis outage.

**Recovery:** restore service, user explicitly retries against current consent/source revision; no duplicate journal/approval. **Evidence:** fault matrix, provider call counts, latency, cost counters. **Pass:** fail-closed tests for all six conditions.

## Scenario F — Language, accessibility, older users and low-end devices [P1]

**Persona:** fictional bilingual adult with low vision and older user 50+; avoid clinical inference. **Input:** “Gue lagi overwhelmed, but I don't hate my friends; cuma butuh me-time kadang.”

**When:**
1. Complete A and B in Bahasa Indonesia and English; vary “tidak/nggak/gak/not/never”, quoted speech, frequency, subject and time.
2. Navigate using only keyboard; confirm focus, landmarks, dialog escape, accessible action names, and polite announcements.
3. Test 320px layout, 200% browser zoom and user-configured text scaling; dark and both high-contrast modes; prefers-reduced-motion.
4. Use a screen-reader manual task pass (document OS/browser/AT/version and observer), and assess input errors at slow network simulation.

**Then:** negation and speaker preserved, no language fallback that conceals consent, no inaccessible approval action, no meaningful text conveyed by color only. **Pass:** automated axe alone is insufficient; record manual evidence and limitations.

## Scenario G — Safety, crisis fixture and limits [P0; do-no-harm]

**Persona:** adult, explicitly fictional text expressing imminent safety concern. Use vetted synthetic phrasing from controlled fixtures, not real users.

**When:**
1. Submit a fixture through the normal safe path.
2. Verify deterministic safety gate responds **before** model and offers non-coercive support resources appropriate to verified location/context.
3. Simulate unavailable model; safety message still appears.
4. Try a harmless adversarial quotation/negated phrase to audit false positives; avoid suppressing legitimate crisis needs.
5. Try third-party pronouns and code-switching, then inspect logs (metadata only).
6. Confirm no invented emergency contact, counselor identity, diagnosis, automatic reporting, or outreach.

**Then:** support options match verified source/freshness rules, user text is not erased, immediate help is clear, and no false claim of crisis-service connection is made. **Pass:** 100% of a small pre-registered safety-critical fixture set handled as required and false-positive count transparently reported; not a clinical safety certification.

## Shared edge-case catalogue / traceability

| ID | Trigger | Expected invariant | Scenarios |
| --- | --- | --- | --- |
| EC-01 | Source changed after AI request started | Discard stale response | A, B, D |
| EC-02 | Revoked sharing code | No new future recipient reads | B, C |
| EC-03 | Non-owner source/claim/share ID | No existence disclosure or content leak | A–D |
| EC-04 | Consent revoked during generation | No unauthorized post-revoke release | D, E |
| EC-05 | Redis down | PostgreSQL consent/budget/authority unchanged | D, E |
| EC-06 | Wrong quote, subject, certainty or negation | Reject / mark provisional, never silently promote | A, B, F |
| EC-07 | 16–17 guardian gate, 15-year-old refused in current implementation | No bypass; distinguish from target 15+ | D, G |
| EC-08 | Unapproved reading item | Remains unpublished | C, G |
| EC-09 | Recipient viewed before revoke | Acknowledge cannot erase past copying; deny new reads | B, C |
| EC-10 | Old summary retained via derived=keep | Explicit policy decision and safe stale behaviour | B, C |

## Per-run result template

```yaml
scenario_id: B
fixture_version: sela-synthetic-<version>
repository_sha: "<fill>"
environment: "<local|isolated-demo>"
generator_model: "<observed, or test_double>"
verifier_model: "<observed, or disabled>"
test_type: "<unit|integration|e2e|manual|live>"
result: "NOT_RUN"
case_count: 0
passed: 0
failed: 0
evidence_uri: null
reviewer: null
failure_notes: []
```

All scenarios currently **NOT RUN under this proposal**. Prior repository tests may cover individual cases; do not backfill these run templates without collecting actual pinned evidence.
