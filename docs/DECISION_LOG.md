# Decision log

Chronological, append-only record of v2 implementation decisions. Authority: [SPEC.md](../SPEC.md); the [decision summary](DECISIONS.md) lists origins. Each entry states origin (owner direction = user-confirmed; engineering = revisable with rationale), the decision, why, and what it changes. Conflicts that led to entries are in the [conflict matrix](CONFLICT_MATRIX.md).

## D-001

2026-10-26 · Owner direction · **AI runs on a Meta open-weight model self-hosted on Modal.com**, replacing the earlier hosted "Meta Model API" plan. V9 is unchanged: Meta is the only AI provider; no fallback provider exists. Model and serving details: [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md).

## D-002

2026-10-26 · Engineering · **Hackathon tooling eligibility is recorded as unverified.** The Official Rules name "Glimmer" and the event page says "run Meta open weights models", but "Meta AI tooling" has no formal definition. No document or UI claims eligibility or a judging advantage.

## D-003

2026-10-26 · Owner direction (story room with text input) within the kept synthetic boundary · **Practice text mode.** Besides seeded scenarios (still the default demo path), the user may type a *fictional practice story* (20–3000 characters). Boundaries: adult attestation, explicit "do not enter real names, contacts or health history" notice, deterministic crisis gate before storage or inference, rejection of obvious contact details (email, phone-like runs of ≥9 digits, 16-digit ID numbers, URLs), memory-only storage under the retention policy, no content in logs. This amends SPEC V2; it does not make Sela a real intake service.

## D-004

2026-10-26 · Owner direction · **Four independent decisions:** AI processing consent, storage (retention) consent, brief approval and sharing consent. Each is versioned against a policy text version, server-enforced, and none implies another. Withdrawing AI consent cancels running operations and discards their results.

## D-005

2026-10-26 · Engineering · **Retention.** Without storage consent: deleted after 30 minutes idle and 2 hours at most. With storage consent: kept in memory up to 24 hours. "Delete everything" purges the session, its shares and recipient access immediately. A restart erases all state and the UI says so. Enforced by store expiry checks on every access plus a one-minute sweeper.

## D-006

2026-10-26 · Engineering · **Identity.** Anonymous by default; optional pseudonym (2–24 letters, no digits/@). The privacy page states that anonymity is not technical anonymity (hosting sees IP/timing; Modal processes AI requests). No real identity is collected.

## D-007

2026-10-26 · Owner direction (token + recipient authorization) · **Two-part recipient access.** A share creates a 256-bit capability token (delivered as a URL fragment, never sent to the server in a URL) and a separate 8-character access code. The recipient principal (its own HttpOnly cookie) is created only when both match, is bound to exactly one handoff of one owner session and expires with the share. Five wrong codes lock the share. Fictional recipients are per owner session, so there is no cross-visitor inbox. All failures return one uniform `access_ended` error. This replaces the same-session "role switch" of the earlier spec.

## D-008

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

2026-10-26 · Engineering (keeps SPEC C7) · **Storage:** `journey.Store` interface with one bounded in-memory implementation (`internal/store`). No database, no disk persistence of session data.

## D-015

2026-10-26 · Owner direction · **Research corpus pipeline** (Python, standard library) rebuilds the 100-record register with DOI normalisation, de-duplication, multi-source date verification, a rolling five-year cutoff (freeze date minus five years), licence/access status, screening and conservative evidence fields (null when unknown). Discrepancies with earlier documents are reported, not hidden. See [corpus pipeline](research/CORPUS_PIPELINE.md).

## D-016

2026-10-26 · Engineering · **No fine-tuning.** None of the prerequisites holds: no measured baseline gap, no licensed task dataset, no train/validation/test split of real-distribution data, and synthetic fixtures are too few. Retrieval at inference is also not used: product answers never need literature, and evidence is mapped to features statically. A reviewer-only corpus search is provided.

## D-017

2026-10-26 · Engineering · **No ML safety classifier in v2.** The deterministic lexicon (ported from Dengar.in, extended for clause-level negation, quotes, third-party framing, slang and code-switching) is the gate. Llama Guard 4 was considered: gated, extra GPU cost and no verified Indonesian performance. A classifier would never be the only safeguard.

## D-018

2026-10-26 · Engineering · **Help resources:** only v1 entries marked `verified_official` for adults (Healing119, LISA Helpline, Yayasan Pulih) with their sources and access dates (2026-10-14/15), region "Indonesia", and a re-verification date. No new numbers were added.

## D-019

2026-10-26 · Engineering · **Provenance values** `live_meta`, `recorded_demo`, `manual`, and new `test_double`. `live_meta` requires the configured Modal provider and a server-reported model equal to the pinned served name. `test_double` (cmd/fakellm) is allowed only outside production and is shown with a banner. No recorded results exist yet; none will be fabricated.

## D-020

2026-10-26 · Engineering · **v1 stays runnable and tested.** T9 removal waits for v2 acceptance including live Meta evidence. The v2 runtime contains no AMD code.

## D-021

2026-10-26 · Owner direction · **Git identity.** Commits use the name and email that GitHub links to the YanIanZ account (verified from existing commits' author association), are unsigned, and carry no agent co-author lines. Dengar.in credits remain in [provenance](SELA_PROVENANCE.md).

## D-022

2026-10-26 · Engineering · **Cost guards.** Modal `max_containers=1`, scale to zero after 5 minutes, proxy auth before any container starts; Go per-session limit (8 AI operations per 10 minutes) and per-address request limits. AI status is computed without contacting Modal (a health request can start a GPU).

## D-023

2026-10-26 · Engineering · **Meaning of `negated`, and one gold-label correction.** `negated` is the polarity of the quoted clause's main predicate: true when a negator (tidak, nggak, gak, bukan, belum, no, not, never, -n't, can't) applies to it. A hedge such as "not sure", "nggak yakin" or "tidak tahu apakah" is certainty (`uncertain`), not negation. Under this definition the synthetic gold fact "I wake up after a few hours and can't get back to sleep" was mislabelled `negated: false`; it is corrected to `true` in fixtures `sela-fixtures-2026.10.26-2`. The correction was made during development, before any final evaluation run, and the pre-registered threshold (`meaning_lock.gold_qualifier_negation_accuracy >= 1.0`) is unchanged. The remaining disagreement ("I'm not sure if it's the rotating schedule…" analysed as negated) is a ledger defect to fix, not a label to change.
