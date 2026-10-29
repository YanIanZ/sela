# Sela | Product and architecture vision

**Decision date:** 29 October 2026  
**Audience:** product owner, engineers, safety reviewers, designers, researchers, prospective collaborators  
**Status:** owner-directed **target design**, **not** an implementation or launch claim  
**Scope:** Sela anonymous-first, text-only, Meta-powered wellbeing platform

## Start here

1. [Product requirements](PRODUCT_REQUIREMENTS.md) — what users can actually do.
2. [Architecture](ARCHITECTURE.md) — an accessible, diagram-first view of the technology and data flow.
3. [Meaning Integrity Engine](MEANING_INTEGRITY_ENGINE.md) — the proposed technical differentiator.
4. [AI, evidence and safety](AI_EVIDENCE_SAFETY.md) — Meta tasks, verifier, fallback, research and release rules.
5. [Professional Care Network](PROFESSIONAL_CARE_NETWORK.md) — optional manual choice of professionals/facilities without GPS.
6. [Delivery and acceptance](DELIVERY_AND_ACCEPTANCE.md) — migration steps, tests, measurable evidence and launch gates.
7. [Admin Console](ADMIN_CONSOLE.md) — privileged operator roles, dashboard and audit **without access to anonymous stories**.
8. [Admin Care Operations](ADMIN_CARE_OPERATIONS.md) — verify public professionals/facilities and manage synthetic future-partner scenarios.
9. [Redis & AI cost engineering](REDIS_AI_COST_ENGINEERING.md) — acceleration, rate limits, budgets and safe fallbacks.
10. [ADR-0004](../adr/ADR-0004-anonymous-first-meaning-integrity-care-network.md) and [ADR-0005](../adr/ADR-0005-admin-and-redis-acceleration.md) — product/architecture decisions and trade-offs.

## Product in one sentence

Sela is an **anonymous-first space** where users can write freely, receive **source-bound, correctable AI reflections**, control the memory derived from their stories, consult curated research and prepare their own material for a professional of their choice—without diagnosis, compulsory accounts or location permission.

### New decisions versus previous specifications

| Area | 26–28 October approach | 29 October owner direction |
| --- | --- | --- |
| Authentication | Guest **or** username account | Guest-only user journey; internal anonymous session, no public login/signup/recovery UI |
| Media | Text-only after voice removal | Remains text-only; no voice, TTS, ASR or avatar |
| Meta | Pinned Glimmer 30B on Modal | Assess Spark 1.3 primary and optional Wonder for limited complex tasks; preserve existing adapter pending access, policy and benchmark verification |
| Model role | Listening and summary generation | Structured assistant tasks + meaning lineage + evidence-grounded interpretation; not chat-only |
| Memory | Optional, user-approved | Add durable correction rules, uncertainty and derived-item invalidation |
| Professional help | Help links and consultation preparation | Add real, manually searched directory and user-selected profession/facility; no GPS |
| Partners | No real partners | Multi-hospital design; **only partner integration simulated with synthetic data** until authorized |
| Research | Corpus and reviewer metadata | Curated evidence-backed retrieval with claim-level citations and quality review |

## Authority and conflict protocol

The existing [SPEC.md](../../SPEC.md) is the current implemented-system requirements authority; it contains earlier owner directions. This set records **new owner decisions for the next change** and is the **proposal for a coordinated SPEC/backlog/API/decision-log update**. Do not silently claim an existing endpoint meets this vision. For delivery, the owner and implementation reviewer reconcile the conflicts, update normative files and tests, then advance status using evidence.

Existing operational truth: [implementation status](../IMPLEMENTATION_STATUS.md), [blockers](../BLOCKERS.md), [architecture (current/historical)](../ARCHITECTURE.md), [AI routing (current/historical)](../AI_POLICY.md).

**Status vocabulary:** target / existing foundation / implemented / verified locally / verified live / externally blocked / partner-simulated. A document is not evidence of a running feature.

## Operator and performance planes

**Anonymous user** and **authenticated operator** are mutually exclusive trust identities. The operator console manages **public, sourced professional and research catalogs**, operational health and budgets; its users cannot browse private journals. Multi-hospital management is future-contract gated and currently limited to isolated fictional sandbox workflows. **Redis belongs to the Go acceleration/control plane**, never to the private data or safety authority. The Go operator API and public cache now exist ([admin API](../API_SPEC_ADMIN.md)); the legacy Next.js admin and Redis are retired and are not reused. The operator UI and directory/research workflows remain target design.

[Admin requirements](ADMIN_CONSOLE.md) · [Planned help workflow](ADMIN_CARE_OPERATIONS.md) · [Redis/cost constraints](REDIS_AI_COST_ENGINEERING.md)

## Product boundary

- Sela provides wellbeing support, self-understanding, journaling, evidence literacy and consultation preparation.
- Sela is **not** a clinician, diagnosis, treatment, severity score, medical record, triage service, telemedicine provider or emergency responder.
- No automatic contact with professional, guardian or emergency service.
- Synthetic scenarios for product testing are allowed; **normal Sela capabilities must not rely on fake user data**.
- Anonymity is relative to account identifiers. It does not negate network metadata or healthcare providers' identity requirements.
- For real-person or youth deployments, legal, provider policy, privacy and clinical/safety review remain preconditions.

## Concept map

~~~mermaid
flowchart LR
    A["I need a safe place"] --> B["Anonymous text conversation"]
    B --> C["Reflection I can correct"]
    C --> D["Meaning Integrity Engine"]
    D --> E["Approved memory + personal journal"]
    E --> F["Source-linked self-understanding"]
    F --> G["Consultation preparation"]
    G --> H["Manually choose profession / facility"]
    H -. "Future partner agreement" .-> I["Professional collaboration"]
    H --> J["User exports own summary"]
~~~

**Design principle:** help-seeking is optional. There is no location requirement, automated diagnosis or pressure to share.
