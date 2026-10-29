# ADR-0004: Anonymous-first, meaning integrity and professional care discovery

- **Date:** 29 October 2026
- **Status:** **Proposed for normative SPEC reconciliation** (owner direction confirmed in conversation; code not yet migrated)
- **Deciders:** Product owner; engineering, privacy/safety and clinical partners must approve their relevant implementation or legal gates
- **Supersedes in future:** Specific account-capable and no-directory parts of ADR-0002 and prior product plans; **does not silently overwrite their implementation/operational history**.

## Context

Previous v3 scope allowed guest and username accounts, focused on listening and preparation, and excluded real professional directory discovery. The new owner direction requires **no consumer login**, no GPS, a full functioning anonymous-first text product, user-governed meaning and an extensible care network. Only future integrations with unaffiliated professional/hospital partners are simulated.

## Decision

1. **Consumer experience:** remove mandatory and optional account login/signup/recovery surfaces from the end-user product after migration; retain secure anonymous sessions and admin/operator authentication.
2. **Data:** maintain encrypted user-owned records under cryptographically random guest subjects; offer clearly explained temporary vs saved mode; define migration/deletion before retiring account endpoints.
3. **Media:** text-only; no voice, TTS, ASR or avatar.
4. **Meaning:** adopt source-linked claim states, user corrections, uncertainty, deterministic invalidation and user approval in a Meaning Integrity Engine.
5. **AI:** Go guardrails and consent first; Meta primary structured generator (Spark 1.3 / Wonder evaluate when available), Claude Sonnet 5.5 independent verifier, DeepSeek availability-only optional fallback.
6. **Research:** two separate corpora: private personal journal and curated licensed public evidence. No real user stories used as training.
7. **Care:** users manually search professions, facilities and public verified provider information; no location permission or hidden IP geolocation. Consultation summaries are user-owned, not medical records.
8. **Partnership:** multi-organization design; external handoff/booking simulated with synthetic entities until formal agreements and actual integration approval.
9. **Claims:** do not represent design as implemented, pretend clinical verification or present synthetic appointments as real.

## Reasoning

- Reduces onboarding barriers and unnecessary identity collection while protecting session authorization.
- Helps protect against AI meaning distortion and unsupported certainty; corrections have system-wide effects rather than changing only a chat message.
- Enables user-directed help-seeking independent of precise location.
- Keeps Sela technically simple (Go modular monolith) while allowing partner-specific adapters later.
- Separates unverified external partnerships from the functionality Sela directly controls.

## Considered alternatives

| Alternative | Reason not selected |
| --- | --- |
| Keep username accounts alongside guest | More identity and recovery UX than the owner wants; preserve safe migration rather than indefinite dual surface |
| Store all conversations in opaque AI memory | Inhibits user correction, audit and deletion of derived interpretations |
| Use LLM-only guardrails | A model cannot be its own authoritative crisis or consent gate |
| Require GPS or infer location from IP | Unnecessary disclosure and friction; manual filters suffice |
| Fake complete hospital integrations | Misleads users and judges about delivered care |
| Split everything into microservices | Adds operating cost and failure boundaries without measured need |
| Reintroduce voice/avatar for accessibility | Owner excluded them; prioritize accessible text and device/browser assistive technology |

## Consequences and risks

- Removing accounts can break access to saved notes unless migration/recovery carefully preserves user ownership.
- Guest-only identity still involves cookies, session expiry, device loss and service-visible network metadata. Do not promise absolute anonymity.
- More sophisticated meaning lineage adds transactional complexity and new privacy-sensitive metadata.
- Provider calls can involve cross-border transfer and sensitive-data restrictions; youth flows may remain blocked.
- Directory metadata requires source verification, update intervals, and no implied facility endorsement.
- Future partners need authenticated professional roles, contractual consent, legal review and scoped integrations.

## Evidence and follow-up

1. Reconcile SPEC.md, PRODUCT.md, docs/PRODUCT.md, API_SPEC_V3.md, BACKLOG.md, DECISION_LOG.md, safety and privacy texts.
2. Ship bounded code PRs with stated acceptance suites and clear status.
3. Record live provider/model IDs and end-to-end test results separately from proposed architecture.
4. Keep PR #8 safety work independently reviewed.
5. Revisit this ADR if provider policies, regulations, partner contracts or safety evidence invalidate assumptions.

[Product requirements](../vision/PRODUCT_REQUIREMENTS.md) · [Architecture](../vision/ARCHITECTURE.md) · [Delivery plan](../vision/DELIVERY_AND_ACCEPTANCE.md)

## Extension — Admin Console and Redis (29 October 2026)

Anonymous consumer sessions remain distinct from verified staff and future partner identities; staff operator MFA, source review and publication workflows require a privileged control plane, but **not** access to private user journals. Redis is an optional Go acceleration/short-lived admission plane for public data and non-sensitive counters; PostgreSQL remains the sole authoritative record and spending ledger. See [ADR-0005](ADR-0005-admin-and-redis-acceleration.md), [Admin Console](../vision/ADMIN_CONSOLE.md), [Care Operations](../vision/ADMIN_CARE_OPERATIONS.md) and [Redis engineering](../vision/REDIS_AI_COST_ENGINEERING.md). These are unimplemented target requirements until aligned with SPEC and code.
