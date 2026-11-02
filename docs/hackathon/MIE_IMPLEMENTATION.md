# Meaning Integrity Engine 1.0 — implementation specification

Status: **target architecture only**, not an implemented feature. Supersedes no existing normative decision. Owner authority for current runtime: [SPEC](../../SPEC.md), [D-065](../decisions/DECISION_LOG.md#d-065), [MIE concept](../vision/MEANING_INTEGRITY_ENGINE.md). Work items H-003–H-009 in [delivery](DELIVERY_PLAN.md).

## 1. Problem and boundary

An AI reflection can misstate a user's subject, polarity, time, frequency, uncertainty or intent, then contaminate downstream memory and consultation material. The user must be the authority for **what the user intended**; a Meta verifier is only an additional signal, never ground truth. Preserve every verbatim original version, treat interpretations as proposals, and prevent a declined/retracted interpretation from silently resurfacing as fact.

Existing building blocks at inspected main: `internal/ai/router.go`, `guard`, `metaverifier`, `internal/account/continuity.go`, `internal/account/records.go`, `internal/account/shares.go`, `internal/httpapi/v3_memory_proposals.go`, `v3_summary_export.go`, `web/src/v3/screens/{Talk,Continuity,Notes,Shares}.tsx`. Quote-backed summary items, memory proposals, share access history, and `GET /api/v3/derived?source=...` already exist. **Do not mistake these for a complete MIE dependency graph.**

## 2. Invariants (cannot be weakened)

| ID | Invariant | Enforced where |
| --- | --- | --- |
| MIE-I01 | Original user utterance is immutable as history; edits create a new version. | PostgreSQL and service |
| MIE-I02 | Each derived claim binds existing owner-scoped source ID, version, exact quote or span, and allowed purpose. | Source validator |
| MIE-I03 | No unreviewed AI interpretation becomes an accepted user fact or shareable assertion. | Domain transitions/read projection |
| MIE-I04 | Decline and correction invalidate authority of prior interpretation and every descendant dependent on it. | Transactional propagation |
| MIE-I05 | No stale/retracted/not-approved descendant is exported or read by recipients as current. | Final read gate and snapshot revocation semantics |
| MIE-I06 | Deny unknown, cross-owner, expired/withdrawn consent, mismatched ETag/version or unexpected provider result. | API/service/model |
| MIE-I07 | In-flight AI result cannot resurrect deleted or superseded claims after retry, race, restore or consent revocation. | Transaction conditional write |
| MIE-I08 | Cached/public/admin data never contain user claim bodies, tokens, raw prompts or source-derived identifying hashes. | Cache, ops, logs |
| MIE-I09 | Emergency/safety gate is deterministic and runs before model request; no fabricated diagnosis, therapist identity or emergency handoff. | HTTP + AI router |
| MIE-I10 | Unknown causality is represented as unknown; uncertainty must not silently increase to certainty. | Model schema + verifier + user review |
| MIE-I11 | Every externally shared projection is a **frozen, consent-scoped version**; changing personal records cannot silently replace what an existing recipient previously saw. | Sharing policy |
| MIE-I12 | Source records, claims and derivative projections are encrypted at rest using the established per-user envelope-key scheme. | Vault/account |

Important policy choice: on correction, **revoke future access to impacted frozen shares by default** (or mark unusable pending explicit re-approval). Past disclosure cannot be erased from copies already viewed. The UI must state this. Existing `derived=keep` deletion permits retained frozen shares; the implementation must reconcile that behaviour deliberately via a new reviewed product decision before claiming I05.

## 3. Proposed domain model

Definitions:
- **SourceRecord**: encrypted user-original note/turn/check-in, with immutable version, owner, written time, consent scope and content hash for internal comparison only.
- **Claim**: proposed structured interpretation or an explicit user-authored correction, with source references, review status, belief/certainty fields, provider provenance and expiry/invalidated timestamp.
- **ClaimEdge**: dependency from source or claim at pinned version to derived claim/summary/share projection.
- **ReviewDecision**: append-only accepting/correcting/rejecting event with expectedVersion, actor owner and reason code (no sensitive text in audit).
- **DerivedArtifact**: personal memory, reflection card, summary, export snapshot or share; references claim IDs and source versions.
- **InvalidationEvent**: monotonically revisioned event that marks descendants stale/retracted with reason and origin claim.

States: `proposed → approved | corrected | rejected | discarded`; `approved → stale → reapproved(new version) | withdrawn`. There are no implicit approvals. `corrected` creates a new user-controlled version, does not mutate history. `stale` means cannot be used as trusted context, exported as approved, or presented as current.

Proposed claim JSON response (names/types to be finalized against API_SPEC_V3):

```json
{
  "claimId": "opaque-random-id",
  "ownerVersion": 7,
  "type": "ai_interpretation",
  "text": "You sometimes need quiet time after class.",
  "source": [{"recordId": "record-opaque", "version": 3, "quote": "Kadang aku...", "offsetUnit": "unicode_scalar", "start": 0, "end": 12}],
  "semantics": {"subject": "self", "polarity": "positive", "timeframe": "after_class", "frequency": "sometimes", "certainty": "tentative"},
  "review": {"status": "proposed", "approvedByUser": false},
  "model": {"generator": "muse-spark-1.3", "verifier": "muse-wonder-1.0", "provenance": "live_meta"},
  "stale": false,
  "version": 1
}
```

Example is **illustrative**, not an existing endpoint payload. No owner user identifier goes to models.

## 4. Proposed database migration (SQL sketch, not executable without review)

Use a new migration number after checking current ranges and branch state. Do not collide with 0200/0300/0400/0500/0600/0700/0800/0900/0910. Prefer 1000 series only after repository-specific migration review.

```sql
-- Conceptual schema; adjust owner key / vault columns to actual account schema.
CREATE TABLE mie_claims (
 id uuid PRIMARY KEY,
 owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 source_version bigint NOT NULL,
 version bigint NOT NULL DEFAULT 1 CHECK(version > 0),
 status text NOT NULL CHECK (status IN ('proposed','approved','corrected','rejected','discarded','stale','withdrawn')),
 kind text NOT NULL,
 sealed_body bytea NOT NULL,
 created_at timestamptz NOT NULL,
 reviewed_at timestamptz,
 invalidated_at timestamptz
);
CREATE INDEX mie_claims_owner_status ON mie_claims(owner_id,status);
CREATE TABLE mie_edges (
 owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 parent_kind text NOT NULL,
 parent_id uuid NOT NULL,
 parent_version bigint NOT NULL,
 child_kind text NOT NULL,
 child_id uuid NOT NULL,
 child_version bigint NOT NULL,
 PRIMARY KEY (owner_id,parent_kind,parent_id,parent_version,child_kind,child_id,child_version)
);
CREATE INDEX mie_edges_parent ON mie_edges(owner_id,parent_kind,parent_id,parent_version);
CREATE TABLE mie_review_events (
 id uuid PRIMARY KEY,
 owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 claim_id uuid NOT NULL REFERENCES mie_claims(id) ON DELETE CASCADE,
 prior_version bigint NOT NULL,
 next_version bigint NOT NULL,
 action text NOT NULL,
 at timestamptz NOT NULL
);
```

DB specifics require reconciliation: actual users PK type, established sealed-record helpers, whether relations should store opaque logical refs rather than FK, and how to protect `mie_edges` from cycles. Never store user story text in audit event tables. Add data migration to link *only* clearly traceable existing approved items; mark uncertain legacy derived items “needs review”, not fabricated valid lineage.

## 5. Write transaction and descendant propagation

**Preconditions**: authenticated owner/guest principal, current purpose consent, current source versions, idempotency key, expected version/ETag, valid transition, no cross-owner input, no pending deletion. Refuse if any fail.

Transaction algorithm:
1. Begin transaction; lock owner-scoped graph revision or serialize on owner via PostgreSQL advisory transaction lock. Do not rely on Redis for authority.
2. Re-read source and consent versions from PostgreSQL under transaction and compare expected versions; fail `409 stale_version` on mismatch.
3. Append review event, create corrected claim version or set original to rejected; preserve original body.
4. Traverse outgoing edges by breadth-first/recursive CTE with visited set, cycle guard, depth/size limit and conservative fail-closed if limit reached.
5. Mark every affected derivative stale in the same transaction; record a reason code and source revision, revoke pending/active impacted share capabilities for future reads, invalidate downstream approved snapshot authority.
6. Advance graph revision and commit. Emit only content-free counter/trace metadata; do not commit if a dependent update fails.
7. Invalidate any safe local *metadata* caches. PostgreSQL remains read authority; Redis loss never creates a permission bypass.
8. Publish a client hint after commit; the UI fetches authoritative state. Never trust the hint alone.

Concurrent requests:
- If a share is read while invalidation commits, enforce consistent snapshot ordering: read checks current graph revision and active share status in a transaction; a read that started before revoke may have already disclosed a snapshot, which must be described as an inherent limit. No new read after commit may pass.
- Retries with same idempotency key replay the same version/result, not duplicate branches.
- Delayed generator/verifier result must re-check source versions, active consent and graph revision **just before release/storage**; otherwise discard, not overwrite or stream unvalidated tokens.
- Backup restore may revive stale grants unless the existing restore/deletion journal gate is completed; block claim of full deletion durability pending that work.

## 6. Model request/release pipeline

1. Sanitize and limit the user-selected source (server-side private-name minimization; no context from unconsented history).
2. Deterministic crisis gate before model calls or persistence according to current safety contract.
3. Meta Spark extracts tentative structured claims; constraints: no diagnosis, only user-span grounded statements, preserve subject/negation/time/frequency/uncertainty, show unknown.
4. Strict parser rejects malformed JSON, truncation, unwanted tool output and extra unsupported fields; confirm model ID from response.
5. Go validates exact source spans (Unicode scalar vs UTF-8 byte boundary conversion must be explicit), provenance and purpose allowlist; no unapproved research citations.
6. Meta Wonder (distinct from Spark) verifies source fidelity where required, under current D-065 consent. Fail closed on verifier timeout/refusal/mismatch; provider agreement does not grant approval.
7. Run final safety/grounding gate on the entire emitted response before client release; label as `live_meta` or honest `unavailable`/`manual`.
8. Store as *proposed*; user reviews before any memory or professional projection.

No Claude, DeepSeek or third-party fallback in this plan. Preserve safe no-AI journaling experience.

## 7. Proposed API contract (not implemented)

| Route | Request | Success | Failure |
| --- | --- | --- | --- |
| `GET /api/v3/mie/claims?sourceId=...` | owner, optional status | scoped paginated view, ETag/version | 401/403/404 |
| `POST /api/v3/mie/claims/propose` | source refs, consent version | proposed claims + provenance | 403/409/422/503 |
| `POST /api/v3/mie/claims/{id}/review` | action approve/correct/reject, expectedVersion, idempotency | current claim + impacted counts | 403/404/409/422 |
| `GET /api/v3/mie/claims/{id}/lineage` | owner | source/descendant metadata, no cross-owner data | 404 |
| `GET /api/v3/mie/impact?source=...` | owner | impacted claim/memory/summary/share counts | 404 |
| `POST /api/v3/mie/claims/{id}/reapprove` | revised user text + source versions + consent | new approved version | 409/422 |
| Existing share/export APIs | unchanged routes, stronger read gates | only current approved projection | 403/409/410 |
 
Prefer extending current `GET /api/v3/derived` and review endpoints over duplicating semantics if implementation review finds a cleaner contract. Preserve consistency with existing status/error conventions.

## 8. UI specification

**Talk/Reflection:** source quote alongside proposed meaning, badge “Unconfirmed / Belum dikonfirmasi”, clear Approve / Correct / Reject / Don't save; private names and sensitive source remain behind hidden preview setting. On generation failure, never erase draft. No auto-accept.

**Meaning Diff:** side-by-side view of original and derived text with localized label for subject, negation, time, certainty, and unknown. Screen-reader accessible list of differences instead of color-only highlighting; never imply detector is infallible.

**Lineage/Impact:** “Why did Sela say this?” reveals source and provenance, plus counts of affected memories, summaries and shares; no operator read of personal text.

**After correction:** banner “This changed X saved reflections, Y summaries and Z shares; review before use”. Old sources visible to owner as historical with unambiguous status; recipient access blocked if stale.

**Share/Export:** explicit preflight of exact frozen projection and version, approve button disabled if stale; after revoke, no new access. Source title/id cannot leak private names to unauthorized recipient.

**Accessibility:** labels in Indonesian/English, 44px targets, 320px no horizontal scroll, keyboard focus after dialog, role/status announcements, no forced animation, high contrast and text scaling. Use established design tokens; avoid a new visual system for this feature.

## 9. Edge cases and abuse tests

- Original: “Kadang aku ingin sendiri, tapi tidak membenci teman.” Generated “selalu mengisolasi diri” → reject unsupported frequency and negation.
- Indirect speech: “Dia bilang aku terlihat sedih” → never treat third-party claim as self-diagnosis.
- Code-switching and slang “gak”, “nggak”, “not”, sarcasm, double negation, 15–17 guarded flow.
- User corrects after old share opened; no undo of earlier recipient access, but reject **future** reads and explain limitation.
- Delete source with `derived=keep` vs `delete`; reconcile current behaviour with new invariant; no silent bypass.
- Concurrent source edit, model completion, review, export, share open, revoke; no resurrection.
- Redis unavailable, PostgreSQL serialization conflict, partial external model error, HTTP retry, browser network disconnect; fail closed without losing user original.
- Same receiver opens one code repeatedly; no duplicate acknowledgement event or additional permissions.
- Unknown/deleted/non-owner source IDs indistinguishable in error response.

## 10. Test plan and rollout

Backend: table-driven state machine tests, Postgres migration up/down, -race, concurrent review/share tests, reauth/consent changes, cross-owner isolation, failpoint after n-th descendant, stale-version conflicts.

AI: strict schema / no-source / mismatched provider / truncation / refused verification / corrupted Unicode span / adversarial prompt injection. Distinguish offline test-double from live Meta.

Frontend: accessible keyboard review, 320px, SR announcements, ID/EN, hidden preview, stale modal, denied share/export, no-AI draft retention.

Rollout: feature flag default **OFF** in production until data migration rehearsal, regression suite, resource caps, restore rollback and owner go/no-go. Mixed old/new clients must be blocked from claiming new authoritative semantics. Collect content-free metrics only. Rollback must disable MIE safely and keep existing user data valid; never delete migration-created data without approved backup plan.

## 11. Explicit DoD

MIE 1.0 is **implemented** only when H-003–H-009 pass; **verified locally** only after CI and PostgreSQL E2E with trace evidence; **verified live** only after a consented, safe, non-personal/synthetic hosted rehearsal. The MIE v0.1 document remains a proposal until this evidence exists.
