# Sela | System architecture, explained simply

**Status:** target architecture, 29 October 2026. No implication of deployment, readiness or completed migration.  
**Style:** diagram-first, Go modular monolith, privacy by construction.

## 1. Explain it without technical jargon

Think of Sela as **five cooperating parts**:

1. **The room:** a text-only, accessible website where a person can write.
2. **The gatekeeper:** Go checks whether the user consented, whether the content needs a safety response, and whether a model is allowed to act.
3. **The assistants:** Meta drafts an interpretation; Claude checks sensitive source fidelity; DeepSeek can substitute only during a narrowly defined Meta outage.
4. **The notebook:** encrypted, selectively saved user records plus a **separate** library of reviewed research.
5. **The bridge:** helps a person prepare their own summary and manually find support; hospital collaboration is simulated until authorized partners exist.
6. **The control tower:** a separate *authenticated* admin workspace that manages public catalogs, source reviews and costs without access to anonymous journals. Redis accelerates repeat public reads and admission, while PostgreSQL holds the authoritative records.

**Only the user decides what to keep, correct or share.**

## 2. Context diagram

~~~mermaid
flowchart LR
    U["User — anonymous browser"] --> W["Sela website"]
    W <--> G["Sela Go API"]
    G <--> DB[("PostgreSQL 16<br/>encrypted personal records")]
    G --> RI[("Curated research<br/>public sources only")]
    G --> META["Meta AI primary<br/>Spark 1.3 target / access pending"]
    G -. "Required independent review" .-> CL["Claude Sonnet 5.5"]
    G -. "Outage + explicit consent only" .-> DS["DeepSeek optional fallback"]
    G --> DIR["Verified public directory<br/>manual discovery"]
    G -. "Synthetic / not a real referral" .-> SIM["Partner simulation sandbox"]
    U -->|"Manual export / user choice"| EXT["Chosen professional or facility"]
~~~

An arrow to a provider represents an **intended trust boundary**, not proof of an active connection. There is no external patient transfer unless explicitly authorized and implemented.

## 3. Container diagram (what runs)

~~~mermaid
flowchart TB
    subgraph Browser["Client — React, TypeScript, Vite"]
      UX["Conversation / journal / care finder"]
      CTRL["Consent / preview / correction UI"]
    end
    subgraph Go["Go application — modular monolith"]
      HTTP["HTTP + session middleware"]
      SAF["Deterministic safety + consent"]
      MEAN["Meaning Integrity Engine"]
      AI["Provider orchestrator + structured schemas"]
      LIB["Research retrieval"]
      CARE["Directory + capsule + partner adapters"]
      VAULT["Owner scoping + crypto/encrypted storage"]
    end
    UX --> HTTP
    CTRL --> HTTP
    HTTP --> SAF
    SAF --> MEAN
    MEAN --> AI
    MEAN --> LIB
    HTTP --> CARE
    MEAN --> VAULT
    CARE --> VAULT
    VAULT --> PG[("PostgreSQL 16")]
    AI --> M["Meta provider"]
    AI -.-> C["Claude verifier"]
    AI -.-> D["DeepSeek opt-in fallback"]
~~~

Go owns HTTP request cancellation, session authorization, deterministic safety, record versioning, memory invalidation, release decisions and audit. A generative model can only produce **untrusted proposals**.

## 4. Request path: one reflection

~~~mermaid
sequenceDiagram
    participant U as User
    participant B as React browser
    participant G as Go API
    participant S as Safety/Consent
    participant M as Meta
    participant V as Meaning validator
    participant C as Claude (if required)
    participant P as PostgreSQL
    U->>B: Write a story
    B->>G: Send text and storage preference
    G->>S: Age/policy, consent, risk check
    alt Crisis, insufficient consent or blocked
      S-->>B: Verified help / manual path, no model call
    else Allowed
      G->>M: Minimized text, structured task
      M-->>G: Proposed reflection + source references
      G->>V: Validate quotes, negation, certainty, prior corrections
      opt Mandatory source-fidelity verification
        G->>C: Minimal approved verification payload
        C-->>G: Verification verdict
      end
      alt Valid proposal
        G-->>B: Unconfirmed reflection
        U->>B: Approve/correct/reject
        B->>G: Explicit decision + source version
        G->>P: Persist only approved content when storage consent is on
      else Invalid, unavailable, stale
        G-->>B: Honest unavailable / user-owned manual path
      end
    end
~~~

**Never silently save a rejected proposal.** A provider error is not consent to switch vendors.

## 5. Data domains — do not mix

| Domain | Contains | Access / storage |
| --- | --- | --- |
| Personal records | Original user text, journal/check-ins, approved interpretations, revisions | Private and encrypted, owner- and version-scoped |
| Meaning graph | Claim IDs, source anchors, review state, correction and invalidation edges | Bound to personal record and revocable; never general training data |
| Public research | Bibliography, approved licensed excerpts, evidence grades and citations | Separate immutable or versioned public index; never mixed with personal stories |
| Care directory | Verified public facility/professional metadata, source and verification date | Separate directory store; may be browsed without GPS |
| Synthetic partner sandbox | Fictional organizations, practitioner identities, handoff states | Strictly namespaced fixtures, unmistakably synthetic |
| Operations | Timing/error codes, request correlation without content | No story, prompt, token, name or sensitive claim text |

## 6. Suggested Go package boundaries

| Proposed package | Responsibility |
| --- | --- |
| internal/httpapi | Existing API, middleware, transport and resource budgets |
| internal/account | Existing guest lifecycle and auth store: **migrate**, do not rip out owner-scoping |
| internal/vault + internal/db | Existing encryption, migrations and transactional stores |
| internal/safety + internal/ledger | Existing deterministic gates and Meaning Lock — must remain authoritative |
| internal/meaning/claims | Typed claim model: user quote vs AI proposal vs evidence vs unknown |
| internal/meaning/lineage | Source/correction/dependency graph and version locks |
| internal/meaning/invalidation | Atomically mark derived records stale after edits/deletions |
| internal/ai | Existing orchestrator; Meta primary, Claude verifier, DeepSeek outage fallback |
| internal/research | Existing BM25 metadata index; extend with curated legally usable passages |
| internal/care/directory | Source-verified facilities, practitioners, services and manual filtering |
| internal/care/capsule | Source-selected consultation summary and safe export |
| internal/care/partners | Separate synthetic adapter now; future signed external connector only after approval |

These are **logical targets**, not claims that the new folders exist today. Avoid microservice splits until measured operational needs justify them.

## 7. Trust boundaries and threats

- **Browser -> Go:** enforce CSRF/origin policy, cookie flags, rate limiting and payload limits. No provider API keys in frontend.
- **Go -> AI provider:** per-provider consent and data minimization; redact unnecessary identifiers; enforce regional/legal policy and response budgets. Provider sees only the smallest permitted input.
- **Go -> research:** public index is untrusted content. Treat retrieved snippets as data, never instructions; prevent prompt injection and citation laundering.
- **Anonymous ownership:** use cryptographically random server-recognized subject; never derive identity from IP/browser fingerprint; ensure other sessions cannot enumerate records.
- **Sharing:** user selects exact records, recipient and expiry; approval bound to a content hash/version; revoke future access; note downloaded copies cannot be recalled.
- **Partner sandbox:** physically separate synthetic tenant/namespace, never connected to a real clinic or booking gateway.
- **Incident:** do not claim encryption means end-to-end; servers may decrypt authorized text for provider calls. Backups and deletion have documented windows and test coverage.

## 8. Critical state transitions

~~~mermaid
stateDiagram-v2
    [*] --> Proposed
    Proposed --> Approved: explicit user approval
    Proposed --> Corrected: user edits
    Proposed --> Rejected: user rejects
    Corrected --> Approved: user confirms revision
    Approved --> Stale: source corrected/deleted or consent revoked
    Stale --> Proposed: rebuild from current authorized sources
    Rejected --> [*]
    Approved --> Deleted: user deletes
    Deleted --> [*]
~~~

Operations must use optimistic version checks or database transactions; stale results from slow AI calls must not resurrect deleted or corrected data.

## 9. Admin and Redis planes — deliberately separate

~~~mermaid
flowchart TB
    U["Anonymous end-user browser"] --> API["Go public API"]
    STAFF["Staff member with MFA"] --> CONSOLE["Operator UI /operator"]
    CONSOLE --> ADMIN["Go /api/admin/v1 — staff identity and role enforcement"]
    API --> AUTH["Guest-scoped personal access"]
    ADMIN --> DIR["Directory / research / verified help operations"]
    ADMIN --> METRICS["AI budget, Redis health and audit views"]
    DIR --> PG[("PostgreSQL — durable and audited")]
    AUTH --> PG
    API --> PUBCACHE[("Redis public cache")]
    API --> BUDGET["Go budget & admission gateway"]
    BUDGET --> PG
    BUDGET --> CTR[("Redis short-lived counters")]
    ADMIN -. "Never implicit read access" .-> AUTH
~~~

- Staff sessions are provisioned out of band, MFA-protected, isolated from consumer cookies and authorized by RBAC + organization-scoped ABAC. They do not include a private-journal viewer.
- Directory editors prepare revisions, independent reviewers verify source/credentials, and PostgreSQL audit records the publication; public Redis caches are invalidated after commit.
- **Redis cache** holds public revisioned metadata. **Redis control** handles bounded tokens/rates/in-flight slots. Separate no-eviction control capacity is preferred over mixing with evictable caches.
- Go checks consent and deterministic safety **before** model requests, including after any cache outcome. Durable budget authority remains PostgreSQL; when controls cannot guarantee capacity, AI abstains and manual pathways continue.
- Planned-care operator workflows are not real bookings. Only licensed/authorized partner integrations can eventually receive user-approved capsules.

[Admin Console](ADMIN_CONSOLE.md) · [Care Operations](ADMIN_CARE_OPERATIONS.md) · [Redis engineering](REDIS_AI_COST_ENGINEERING.md)

## 10. What is already here

Current packages include internal/httpapi, internal/safety, internal/ledger, internal/llm, internal/ai, internal/db, internal/vault, internal/account and internal/research. Current main contains an account/guest foundation and Meta Glimmer/Modal path; the new user-facing account removal, meaning graph and real directory are **target changes**. Cross-check with [implementation status](../status/IMPLEMENTATION_STATUS.md) before any status update.
