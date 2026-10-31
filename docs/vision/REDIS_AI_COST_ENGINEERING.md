# Sela | Redis, Performance & AI Cost Engineering

**Version:** CACHE-0.2 · 30 October 2026
**Status:** the **Go client, revisioned public cache and durable PostgreSQL spend gate are implemented** (2026-10-30, §11). There is still no production Redis cluster, no measured production saving, no live provider billing and no Redis-backed distributed rate limiter; the local Compose Redis is disposable and non-persistent.

## 1. Why Redis — and why not everywhere

Redis is an optional **ephemeral acceleration and coordination plane**. It improves repeated **public** reads, multi-instance rate limiting, request coalescing for safe static operations, and fast token/cost-budget checks. It is **not** the source of truth for personal content or clinical/safety decisions.

~~~mermaid
flowchart LR
    B["React browser"] --> G["Go API"]
    G --> SAF["Go: consent + safety; always before AI/cache"]
    G --> R["Redis: short-lived public cache and counters"]
    R -. "Cache miss / unavailable" .-> PG[("PostgreSQL 16 — authoritative")]
    G --> PG
    SAF --> AD["Go AI router: bounded admission"]
    AD --> R
    AD --> ME["Meta primary"]
    AD -. "Verified selected tasks" .-> CL["Claude verifier"]
    AD -. "Eligible outage + consent" .-> DE["DeepSeek optional fallback"]
    G --> IDX["Local curated public evidence index"]
~~~

A cache hit is **not** a safety result, AI consent grant, proof of a provider's identity, patient authorization, or permission to send a model request.

## 2. Storage rules — strict matrix

| Data | Redis? | Source of truth | Rationale |
| --- | --- | --- | --- |
| Versioned **public** provider/facility metadata | **Yes**, bounded TTL/revision | PostgreSQL public directory | Fast anonymous directory browsing |
| Reviewed, licensed **public** research metadata/allowed snippets | **Yes** if rights permit; TTL/rev | Curated research index | Fast evidence discovery, not patient inference |
| Public UI configuration and immutable prompt **templates** (no user text) | **Yes**, versioned | Deployed code/approved config | Reduce repeated reads; do not confuse with provider token savings |
| Per-provider availability/circuit health | **Yes**, short TTL | Live health checks / internal observation | Route outages; never override content refusals |
| Rate limit counters / concurrency slots | **Yes**, atomic Lua/token bucket | Redis fast state + Go hard bounds | Distributed admission; fail safely during outage |
| AI spend reservation metadata (counts, no prompts) | **Yes**, short TTL | PostgreSQL durable cost ledger/provider billing | Preauthorize budgets, not actual billing truth |
| Raw user story, journal, transcript or consultation summary | **NO** | Encrypted PostgreSQL / bounded explicitly unsaved in-process hold | Sensitive health information |
| User embeddings, semantic cache, derived diagnosis or private memory | **NO** | Approved private record only if needed | Prevent cross-user leakage/profiling |
| Provider prompt/completion, Claude verdict with private text | **NO** | Never a shared cache; only purpose-authorized stored record if explicitly opted in | Prevent accidental replay, purpose misuse and unsafe de-identification |
| Raw client IP, email, phone, session token, consent grant, admin credentials | **NO** | Appropriate protected identity/audit store | Private/secret data |
| Crisis/risk result or release authorization | **NO** | Recalculate deterministic safety and versioned rules every relevant request | Never bypass safety |
| Real care request, appointment, delivery or audit event | **NO** | PostgreSQL transactional data | Durable, legally consequential operations |
| Synthetic partner event counters | Limited metadata only, TTL, isolated | Sandbox PostgreSQL / committed fixture | Must not appear as real medical activity |

**No content-derived hashes as shared AI cache keys**: even SHA-256 of a short sensitive statement can permit guesses/dictionary attacks, and hashing does not make sensitive inputs anonymous. No global cross-user cache for personalized answers. Never cache meaning approval or consent decisions.

## 3. Cost efficiency without unsafe shortcuts

**Preferred order of optimization:**

1. **Avoid unnecessary model calls:** Go answers navigation/config, strict safety blocks and consent-denied requests without any AI.
2. **Trim context safely:** include only user-approved, relevant, version-bound source spans within a task token budget; omit unrelated journal history.
3. **Structured Meta tasks:** one bounded operation can return source anchors and a tentative reflection; validate once with Go rather than repeated uncontrolled model loops.
4. **Use provider tiers by task:** Spark for eligible general text tasks; Wonder only when measurable quality improvement offsets added cost and policy/access allow it. Current Glimmer/Modal remains until an explicit verified migration.
5. **Verify selectively but faithfully:** Claude on every mandatory verifier task, not every simple generic navigation response. Never omit verification to save money where release policy requires it.
6. **Cache public evidence/catalog/templates:** versioned, non-sensitive only, with safe invalidation; *this avoids DB/retrieval work*, not magically a provider token bill.
7. **Request deduplication:** singleflight identical concurrent **public, non-user-specific** computations; never coalesce different anonymous users' private requests.
8. **Use provider-supported prompt caching only when verified** for static, non-sensitive prefixes, with provider-specific cost/retention consent review; never assume automatic savings.
9. **Bound retries:** retry transport availability failures only before first output and within request deadline; no fallback after safety refusal/output validation failure.
10. **Measure:** compare actual token use, accepted task cost and useful output quality against baseline; report no invented savings.

Avoid freeform agent loops, repeated cross-model verification chains and token-dense full-history replay.

## 4. Atomic budget and concurrency design

Redis counters can provide **fast approximate/atomic admission**, while PostgreSQL persists the authoritative budget/event ledger. Redis alone is not a durable spending ledger; it may restart, evict, or lose connection. Real vendor bills remain the final financial source for reconciliation.

Proposed flow:

~~~mermaid
sequenceDiagram
    participant U as Anonymous user
    participant G as Go API
    participant S as Safety/Consent
    participant R as Redis
    participant P as PostgreSQL budget ledger
    participant M as Meta
    U->>G: Request reflection
    G->>S: Validate risk, consent, source version
    alt disallowed
      S-->>U: Safe manual or help path
    else allowed
      G->>P: Reserve bounded task budget (durable/idempotent)
      G->>R: Atomic rate/concurrency admission (Lua)
      alt admitted
        G->>M: Bounded request with deadline
        M-->>G: Output + usage metadata
        G->>P: Reconcile charge, release unused reservation
        G->>S: Validate output, freshness, verification requirements
        S-->>U: Approved unconfirmed proposal or honest failure
      else over capacity or Redis unreliable
        G->>P: Release reservation
        G-->>U: Busy / manual path, no uncontrolled AI calls
      end
    end
~~~

- A reservation is keyed by **idempotent operation ID**, not prompt text; count expected maximum output and model/provider budget.
- Reconciliation records provider, model revision, task, estimated vs actual tokens, cost, outcome and non-sensitive correlation ID; **never prompt, original text or anonymous ID in exposed dashboards**.
- Global ceiling **cannot** rely only on Redis. PostgreSQL transaction enforces strict monetary caps; fail closed for AI if durable authority is unavailable.
- Redis Lua script handles time windows/admission atomically. Reserve a bounded in-flight slot; release in `defer`/on cancellation, with TTL watchdog for crashes. Lease expiration and double-release must be idempotent and covered by tests.
- For multiple Go instances use distributed counters; for one instance local in-process admission can be supported, but **must** still obey a durable budget gate. If Redis outage makes globally safe limits impossible, do not start expensive AI calls.
- Priority fairness: crisis/help path bypasses AI budgets and is always reachable; anonymous AI requests use bounded fairness, never privilege a guessed health profile.

## 5. Public cache invalidation

Use *revision-stamped cache keys* so stale entries do not become authoritative after a directory or research change. Example **public-only** keys:

~~~text
sela:prod:public:directory:v4:facility:public-id-123
sela:prod:public:research:v9:paper:doi-id-safe
sela:prod:public:help:v2:verified-source:public-id
sela:prod:control:ai:capacity:meta:spark
sela:prod:control:ai:budget:2026-10-29
~~~

These are **illustrative names**; never place real identity, user-provided story, raw free-text search or auth/session tokens in key names.

Use **transactional outbox** from PostgreSQL on directory update to publish invalidation after commit; the API must compare DB-backed revision/validation state for claims like "verified", "available" or "current". Public help phone numbers may have cache for UI, but live help must also have a safe embedded/DB fallback on Redis failure.

**Default TTL ranges to benchmark** (not clinical guarantees): immutable public config 5–60 min; public directory listings 1–10 min with versioned invalidation; research public metadata 15–60 min; provider circuit health 5–30 sec; rate-limit counters according to window. Key TTL never replaces source freshness policy.

## 6. Redis infrastructure and security

Use a **private network**, Redis ACL-authenticated client with least privilege, TLS where traffic crosses an untrusted boundary, strict command/key-scoped ACL, no public port, monitoring of connection count, memory, evictions and latency.

**Separate logical/physical workloads:** for real deployments, keep **safety/financial admission counters** away from evictable public-cache memory (separate Redis instances preferred; different ACL/key scopes are not sufficient protection from memory eviction). Suggested policies:
- Redis **cache**: bounded `maxmemory`, an appropriate eviction policy such as `allkeys-lru`, all entries TTL, no sensitive content, loss tolerated.
- Redis **control**: `noeviction` and bounded counters with expirations, dedicated capacity; failure -> AI admission denied or durable safe fallback.
- PostgreSQL remains the durable ledger. Redis persistence is optional for cache-only instance and must never masquerade as a backup.

**Client:** new `internal/cache` and `internal/ai/admission` interfaces in Go with a compatible, pinned Go Redis client. No dependency on the legacy `apps/web/src/lib/redis.ts`; reuse concepts only after audited porting.

**Failure:** time-bounded connection attempts and circuit breaker; avoid every request waiting on a Redis outage. Cache lookup fails open to DB (public data), but budget/authorization safety fails closed when a safe fallback is unavailable. Use health/readiness endpoints that distinguish "optional degraded cache" from "budget admission unavailable".

## 7. APIs, operation and observability

Suggested operator-readable metrics (no user text):
- `sela_cache_hit_ratio{domain="public_directory"}`, `cache_miss`, `cache_p95_ms`, `redis_evictions`, `redis_memory_bytes`.
- `ai_request_admitted_total{task,provider}`, `ai_request_rejected_total{reason}`, `ai_inflight`, `ai_tokens_input_total`, `ai_tokens_output_total`, `ai_cost_usd_total`, `ai_cost_reserved_usd`.
- `ai_first_token_p95_ms`, `ai_total_p95_ms`, `ai_verifier_abstained_total`, `ai_fallback_total{reason}`.
- `manual_path_total`, `external_model_requests_on_crisis_path` (must remain zero in test corpus).

**No high-cardinality IPs, session identifiers, individual prompt traces or model reasoning in metric labels**. Dashboards aggregate and suppress small groups.

## 8. Benchmark and ROI methodology

A/B compare identical synthetic workload and fixed Go hardware:
- baseline with Redis disabled versus public cache enabled;
- 1 vs N Go instances; cold and warm caches;
- 90% public directory reads vs changing source records; research hit/miss; Redis outage/restart;
- AI prompts **identical** and provider/model revision pinned, compare input/output tokens, success, quality, cost; isolate DB cache savings from inference savings.

**Cost arithmetic:** total request cost = Meta generation + mandatory Claude checks + optional separately consented DeepSeek fallback + infrastructure costs. Redis has a cost; report net ROI including its RAM/hosting and management, not just decreased request latency.

Set performance thresholds only after baseline and validate realistic tail latency. Do not claim Redis reduces model tokens merely by caching database results.

## 9. Mandatory failure tests

1. Redis cache unavailable -> directory and official help continue from authoritative sources, no privacy leak.
2. Redis control unavailable -> *no uncontrolled new AI calls*; manual journey remains usable.
3. Redis control counters at maxmemory -> noeviction means deny new admission, not reset a limiter.
4. 100 concurrent duplicate AI requests -> reserved budget remains bounded; same private task is **not** served to another user.
5. Directory record unpublished while cache warm -> never shown as verified; new version invalidates old keys.
6. User withdraws provider consent during request -> output discarded, no new model calls.
7. Personal text appears in journal -> no `GET`, `SCAN`, keys, logs or metrics include it.
8. Safety block -> zero provider calls, even on a public cache hit/miss.
9. Redis restart mid-operation -> ledger reconciles reservations with observed provider usage and stale leases.
10. DeepSeek fallback cannot be enabled from Redis configuration or on a safety/refusal error.
11. Disable Redis feature flag -> correct degraded behavior; no public service hard dependency.
12. No accidental inclusion of journal snippets in Redis or shared cache after future source retrieval changes.

## 10. Source references

- [Redis rate limiting](https://redis.io/docs/latest/develop/use-cases/rate-limiter/) — atomic admission via Lua.
- [Redis client-side caching](https://redis.io/docs/latest/develop/clients/client-side-caching/) — tradeoffs of tracking and invalidation; not mandatory for initial Sela Go design.
- [Redis key expiration](https://redis.io/docs/latest/commands/expire/), [Redis memory management](https://redis.io/docs/latest/operate/oss_and_stack/management/optimization/memory-optimization/).
- [Historical Sela Redis design](../archive/v1/REDIS.md) and [legacy Go architecture](../architecture/ARCHITECTURE.md) for migration context.

## 11. Implemented Go slice (2026-10-30)

Branch `feat/admin-redis-foundation`; local and CI evidence only, no live billing.

- **Go client** (`internal/cache`): `redis://`/`rediss://`, bounded dial/read/write timeout, no retries, keys namespaced `sela:<env>:public:<kind>:<revision>:<id>`. `GetJSON` treats a miss, an outage or a malformed value as "fall back"; `SetJSON` clamps the TTL to the client maximum (10 minutes in the server wiring); errors never fail the request.
- **Public cache**: `GET /api/v3/help-content` reads and writes the revisioned embedded catalogue (`fixtures.Version`). It holds no user text, token, consent grant or safety result. Operator invalidation deletes only that key and appends an audit event.
- **Durable budget** (`internal/budget`, migration `0500_ai_spend`): PostgreSQL `ai_spend` stores task, provider, token counts, reserved/actual USD and an open/settled state, never a user id, session id, prompt or reply. `Admit` takes a transaction-scoped advisory lock, sums the UTC month's committed spend (`actual`, or the reservation while open), and inserts the reservation only when it fits. `Settle` records the estimate once; a repeated settlement is a no-op. A failed operation settles at its reservation.
- **Wiring**: the listening handler admits before `ai.Router.Run` and settles from `Provenance` afterwards. Any admission error fails closed; `budget.ErrExceeded` returns `aiState: "budget_limited"` and makes **zero provider calls** (`TestBudgetDeniesBeforeAnyProviderCall`). A positive cap without a database refuses server start.
- **Operator visibility**: `/api/admin/v1/overview` adds `aiSpend` (limit, reserve, month start, committed, actual, open reservations, operations) and `aiSpendByTask`. Aggregates only.
- **Failure properties asserted**: admission is serialized under concurrency (12 workers, at most the cap), a second gate instance on the same database sees the same spend (an outage/restart cannot reset it), and Redis is never consulted for admission. `TestRedisRoundTrip` covers set/get/delete/TTL against a disposable Redis and skips without `SELA_TEST_REDIS_URL`.

A reservation left open by a crash stays committed at its reserve (never released implicitly), so a crash can overcount the month but can never undercount or reset it; a future maintenance command may settle stale reservations after a stated interval. Counter risk: a client that disconnects after a provider call but before settlement is also settled at the reserve by the handler's failure path.

Not implemented: Redis-backed distributed rate limiting or request coalescing, provider-billing reconciliation, measured warm/cold savings, production Redis (HA, TLS, eviction policy, monitoring), budget-change approval workflow and a spend alert. The cost estimates use the existing `ai-prices-2026-10-26` table; Meta GPU time is not attributed. D12 is therefore **implemented in part**, not verified live.
