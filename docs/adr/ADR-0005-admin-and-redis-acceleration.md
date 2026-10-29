# ADR-0005: Separate Admin Console and Redis acceleration plane

- **Date:** 29 October 2026
- **Status:** Proposed; owner-directed architecture, **not an implemented Go runtime**
- **Related:** [ADR-0004](ADR-0004-anonymous-first-meaning-integrity-care-network.md), [Admin Console](../vision/ADMIN_CONSOLE.md), [Care Operations](../vision/ADMIN_CARE_OPERATIONS.md), [Redis/AI cost](../vision/REDIS_AI_COST_ENGINEERING.md)
- **Supersedes:** Nothing operational without a later code-and-SPEC migration. Legacy Next.js admin and Redis remain legacy facts.

## Context

Sela's new anonymous-first product cannot depend on user accounts, but operating a directory of real professional and facility information needs authenticated, auditable editors and reviewers. The current legacy Next.js admin functions do not provide a verified Go v3 operator console. Go v3 currently relies on in-process rate limiting and PostgreSQL; Compose's Redis is in the legacy profile. New AI tasks require bounded provider costs without caching sensitive journal content.

## Decision

### Operator domain

1. Distinguish **anonymous consumer sessions** from **verified staff/partner identities**. No admin privilege flows from a consumer cookie and no staff privilege grants unrestricted visibility of personal records.
2. New Go `/api/admin/v1` endpoints and `/operator` UI: MFA/passkey-preferred operator identity, server-enforced RBAC + organization-scoped ABAC, strong CSRF/origin checks, immutable PostgreSQL audit and staff revocation.
3. Backoffice responsibilities: directory publication/verification, source freshness, researcher/editor review, safety-help resource quality, non-sensitive AI/budget telemetry, cache operations, sandbox workflows and later verified partner onboarding.
4. Two-person approval for professional credentials and safety-related content; operator cannot self-approve or change safety bypass policy. No private user journal search or arbitrary patient access.
5. Real partner access only after legal authorization, verified organization/professional identity, user-specified content/recipient/purpose/expiry, per-access validation and audit; fictional sandbox records remain isolated and labelled.

### Redis

1. Introduce a Go-accessible **ephemeral** Redis plane for versioned public directory/research caching, multi-instance admission counters and provider cost-control metadata—not the private record store, consent authority, safety answer cache or financial source of truth.
2. **PostgreSQL** remains authoritative for identities, user encrypted records, versioning, content approval, operator audit, directory publication and exact durable spend reservations.
3. No raw user text, personalized model answers, user embeddings, prompts/completions, tokens, precise location or private session identifiers in Redis. Hashing sensitive text does not make a safe shared cache key.
4. Redis failure degrades optional public caches to DB, while uncertain monetary/admission state prevents new AI calls without a safe durable fallback. Immediate help and manual journaling are independent.
5. Prefer separate Redis instances for evictable public cache vs `noeviction` budget/rate-control plane; strictly private network, ACL/TLS where appropriate and short expirations.
6. Optimize AI costs first by avoiding unnecessary calls and trimming *approved* context, then measure task-based model routing and public-cache ROI. Do not claim direct inference savings simply from Redis deployment.

## Considered alternatives

| Alternative | Not selected because |
| --- | --- |
| No staff login because users have none | Publishing verified directories and revoking staff privileges requires strong operator identity |
| Reuse v1 Next.js admin unchanged | Legacy trust model, sessions and UI are not audited as Go v3 production-compatible |
| Let administrators view all anonymous journals | Directly violates privacy and user-controlled consent |
| Make Redis primary user database or sole cost ledger | Volatile/evictable/cache state risks loss of ownership, consent, audit or financial controls |
| Cache per-user AI responses or prompt hashes | Private-data leakage, re-identification and semantic-staleness risk |
| Cache crisis decisions or bypass model verification on hit | Unsafe and incompatible with independent safety and Meaning Integrity gates |
| Put all services into microservices | Complexity without measured necessity; keep modular Go monolith |

## Consequences

- New privileged system increases attack surface: MFA, threat model, role testing, audit and incident procedures are hard prerequisites.
- Directory publication requires sourced reviews and a revision-driven invalidation mechanism.
- Redis adds CPU/RAM/network/ops cost and no guaranteed performance benefit without benchmark.
- Money-sensitive admission requires PostgreSQL transaction + Redis fast control coordination and outage handling.
- Existing user-account removal, planned-care discovery and safety PR #8 must remain separate code changes.
- Do not claim real hospital partnership, booking or production rollout as a result of this ADR.

## Required verification

- Two-way anonymous/staff isolation, operator MFA, denial of cross-tenant access, role revocation, append-only audit.
- Real directory data and credential verification with independent reviewer; cache invalidates on source change.
- Redis outage/eviction/restart and parallel-rate/budget race suite; no uncontrolled AI spend.
- End-to-end planned-care experience without geolocation, with help services independent of Redis.
- No private text in Redis `SCAN`, logs, metrics, backups or LLM output caches.
- Completion requires SPEC/API/backlog reconciliation and implementation PRs with actual test evidence.
