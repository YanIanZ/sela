# Sela

**A little space to feel heard.**  
*Ruang sejenak untuk didengar.*

[![Sela v2](https://github.com/YanIanZ/sela/actions/workflows/v2.yml/badge.svg)](https://github.com/YanIanZ/sela/actions/workflows/v2.yml)
[![Quality gate](https://github.com/YanIanZ/sela/actions/workflows/quality.yml/badge.svg)](https://github.com/YanIanZ/sela/actions/workflows/quality.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Sela is an anonymous-first, text-only wellbeing platform being built in Go.** It helps people describe their experiences, reflect without being assigned a diagnosis, keep only the personal context they approve, find evidence-informed reading, and prepare their own notes before seeking professional help.

**Sela bukan terapis, dokter, layanan diagnosis, sistem triase, atau pengganti layanan krisis.** Sela tidak menghubungi profesional atau rumah sakit atas nama pengguna tanpa tindakan dan persetujuan eksplisit.

> **Status — 29 October 2026:** This README presents the **new owner-approved product direction**, not a claim of completed implementation. The previous v3 specification and code still contain optional username accounts and a Glimmer/Modal AI path. Live Meta/Claude/DeepSeek inference, anonymous-only cutover, Meaning Integrity Engine, and real partner integrations are **not verified as operational**. The work is tracked in [the vision documentation](docs/vision/README.md) and [implementation status](docs/IMPLEMENTATION_STATUS.md). No public clinical or pilot-readiness claim is made.

## What makes Sela different?

Sela is designed around a **Meaning Integrity Engine**: the AI must distinguish the user's exact words from its own interpretation and from scientific evidence. A user can correct or reject an interpretation; downstream memory and consultation summaries derived from it then become invalid until reconciled. Sela must also preserve what it **does not know**.

**Our design thesis:** *Most AI assistants decide what to remember about you. Sela is designed to respect what it must not assume.*

This is a proposed, testable engineering distinction—not a claim of a world-first invention or validated clinical outcome. [Read the mechanism](docs/vision/MEANING_INTEGRITY_ENGINE.md).

## Product capabilities

| Area | Intended experience | Current evidence / boundary |
| --- | --- | --- |
| Anonymous-first access | Start without registration, password, phone, email or GPS; select ephemeral or private storage | Guest sessions and encrypted storage already exist; removing account UI/APIs is not yet implemented |
| Listening and journal | Text conversation, journal, check-ins, timeline, user-controlled storage | Local v3 work exists; live provider integration unverified |
| Meaning Integrity | Quote-backed reflections, user corrections, uncertainty, provenance and stale-result invalidation | Existing Meaning Lock is a foundation; cross-feature integrity is **planned** |
| Knowledge library | Curated research and official guidance, with visible sources and limits | Research metadata/index exists; a validated live evidence-to-answer pipeline is **planned** |
| Meta intelligence | Meta-first structured reflection, extraction, source-grounded synthesis and preparation | Glimmer/Modal adapter exists; Spark 1.3 / Wonder access and live calls **not verified** |
| Verification / fallback | Claude Sonnet 5.5 independent verifier; DeepSeek optional availability fallback | Adapter/design work exists; live provider behaviour unverified and consent-gated |
| Professional care discovery | Choose type of professional, facility or service manually; optional typed city, **never mandatory location permission** | Verified real directory and source validation **planned** |
| Consultation preparation | Editable, source-linked user-owned summary; export and review | Local manual continuity work exists; automated integrity and live release gates **planned** |
| Hospital collaboration | Multi-provider organization/facility/practitioner architecture | **Synthetic partner handoff only** until actual partners, credentials and legal agreements exist |

**Full-product principle:** Features under Sela's control must function for real; **synthetic data is restricted to evaluation and unpartnered professional/hospital collaboration demonstrations**. Synthetic patients, facilities and appointments must never appear as real or bookable.

**Out of scope:** voice/TTS/ASR, AI avatars, mandatory user accounts, GPS-based discovery, automated diagnoses, automated contact with clinicians, invented availability, engagement streaks and hidden profiling.

## Understand the architecture in one minute

~~~mermaid
flowchart TD
    P["Anonymous user — text-only browser"] --> FE["React + TypeScript + Vite"]
    FE --> API["Go modular-monolith API"]
    API --> A["Anonymous session + consent + encrypted storage"]
    API --> G["Deterministic safety & Meaning Integrity gates"]
    G --> M["Meta AI — primary structured tasks"]
    M --> V["Go validation + Claude verification when required"]
    M -. "Availability outage + separate opt-in" .-> D["DeepSeek fallback"]
    API --> R["Curated research index — no personal stories"]
    API --> C["Care directory — manual search, no GPS"]
    C -. "Partner integration not live" .-> S["Synthetic multi-facility handoff"]
    V --> FE
~~~

**Rule:** Go owns identity boundaries, consent, safety decisions, provenance, persistence and final output release. Models propose; users approve. No model can grant its own permission to store or share.

[Plain-language architecture](docs/vision/ARCHITECTURE.md) · [AI, evidence and safety](docs/vision/AI_EVIDENCE_SAFETY.md) · [Care network](docs/vision/PROFESSIONAL_CARE_NETWORK.md)

## Operator Console and the planned-care control plane

**Anonymous for users does not mean anonymous for staff.** Sela's future `/operator` area is a separate, strongly authenticated staff-only interface for source verification, professional/facility catalog quality, research editorial review, official-help freshness, AI/provider budgets, cache health and immutable operational audit. Operators **cannot browse anonymous journals**. Partner accounts are distinct from user sessions; no actual partner exchange is claimed. [Admin Console](docs/vision/ADMIN_CONSOLE.md) · [Care Operations](docs/vision/ADMIN_CARE_OPERATIONS.md).

**Redis is planned for the Go runtime**, rather than reused unmodified from the inherited Next.js application. Cache only **public versioned directory/research assets**, manage short-lived rate/admission counters, and coordinate budget reservations with an authoritative PostgreSQL ledger. **No user stories, private AI responses, prompts or consent grants in Redis**. Redis outages must never bypass crisis handling or allow unbounded paid AI requests. [Redis and AI cost engineering](docs/vision/REDIS_AI_COST_ENGINEERING.md).

~~~mermaid
flowchart LR
    A["Anonymous Sela browser"] --> G["Go application"]
    O["Authenticated staff operator"] --> OC["Separate Go Admin Console"]
    OC --> G
    G --> PG[("PostgreSQL — durable authority")]
    G --> R[("Redis — public cache + short-lived controls")]
    G --> M["Meta / Claude / DeepSeek (gated)"]
    OC --> D["Directory + research review"]
    D --> PG
~~~

## Why Go?

Sela's backend uses a **Go modular monolith** to keep the runtime understandable and operable. One process can serve same-origin JSON APIs and built frontend assets; PostgreSQL 16 stores explicitly approved, encrypted personal records. External AI inference remains separately accounted for.

We **do not claim Go consumes less memory than another stack without a fair benchmark**. The [delivery and evaluation plan](docs/vision/DELIVERY_AND_ACCEPTANCE.md) specifies idle/peak RSS, p50/p95 latency, cold start, throughput and costs, with comparable test conditions.

## Local development

Requirements: **Go 1.26.10** (see go.mod), **Node.js 22**, and **PostgreSQL 16** for v3 routes.

~~~sh
git clone https://github.com/YanIanZ/sela.git
cd sela
(cd web && npm ci && npm run build)
SELA_ENV=development SELA_SECURE_COOKIES=false go run ./cmd/sela
~~~

Open http://localhost:8080. The no-database development journey can respond with a database-unavailable state for v3; set up the database before exercising persistent features.

For the repository's configured Docker environment:

~~~sh
docker compose up --build
~~~

Optional local runbook (requires ignored local environment files): bash scripts/run-local-v3.sh. Do **not** point a development deployment at real personal stories.

Validation:

~~~sh
go vet ./...
go test -race ./...
(cd web && npm run typecheck && npm test && npm run build)
go run ./cmd/sela-eval -mode offline
~~~

Database-specific Go tests require the test database settings in [the runbook](docs/RUNBOOK.md). CI green is **not** clinical validation or proof of live model access. [Development](docs/DEVELOPMENT.md) · [Operational runbook](docs/RUNBOOK.md) · [Owner testing](OWNER_TESTER.md).

## Documentation — where to begin

| Read this | For |
| --- | --- |
| **[New product vision](docs/vision/README.md)** | Owner-approved 29 October direction; implementation vs target |
| [Product requirements](docs/vision/PRODUCT_REQUIREMENTS.md) | Scope, workflows, priorities and acceptance criteria |
| [Architecture](docs/vision/ARCHITECTURE.md) | System diagrams, modules, data flows and threat boundaries |
| [Meaning Integrity Engine](docs/vision/MEANING_INTEGRITY_ENGINE.md) | Original technical differentiator and invalidation contract |
| [AI, research and safety](docs/vision/AI_EVIDENCE_SAFETY.md) | Meta, Claude, DeepSeek, research grounding and fail-closed design |
| [Professional Care Network](docs/vision/PROFESSIONAL_CARE_NETWORK.md) | GPS-free discovery, multiple facilities, synthetic-only partner simulation |
| **[Admin Console](docs/vision/ADMIN_CONSOLE.md)** | Separate staff identity, RBAC/ABAC, catalog governance, audit and AI operations |
| [Admin Care Operations](docs/vision/ADMIN_CARE_OPERATIONS.md) | Planned support workflow, source verification, fictional partner sandbox |
| **[Redis & AI cost engineering](docs/vision/REDIS_AI_COST_ENGINEERING.md)** | Public caching, rate admission, durable budgets and graceful degradation |
| [Delivery and acceptance](docs/vision/DELIVERY_AND_ACCEPTANCE.md) | Work packages, evaluation, gate criteria and honest status |
| [Complete documentation index](docs/README.md) | Existing spec, API contracts, ADRs and historical evidence |

**Migration note:** The 26–28 October v3 documents describe a prior account-capable design. The newer direction is explicitly documented as a change proposal; code and [SPEC.md](SPEC.md) must be reconciled before the new design can be reported as implemented. [Decision record](docs/adr/ADR-0004-anonymous-first-meaning-integrity-care-network.md).

## Privacy and safety

Sela does **not** promise absolute anonymity. Servers and external providers may process IP/network metadata, and a professional provider may need to verify identity for a real consultation. No browser fingerprinting or covert identity graph is permitted. No prompts, journal contents or access tokens belong in application logs. Separate consent is required for each provider; disabling AI must leave a useful manual path.

Sela's crisis gate is deterministic, precedes generative AI and must never be bypassed by fallback routing. Help information needs independent source verification. Sensitive real-world deployment and under-18 model access remain blocked pending applicable policy, legal and safety review.

[Security](SECURITY.md) · [Safety](docs/SAFETY.md) · [Current blockers](docs/BLOCKERS.md)

## Origins, ownership and license

Sela continues the work of **Dengar.in**. The initial project and contributions retain their authorship and notices. The current Sela repository is maintained by [YanIanZ](https://github.com/YanIanZ). See [the full provenance and contributor record](docs/SELA_PROVENANCE.md) and [Dengar.in feature matrix](docs/DENGARIN_FEATURE_MATRIX.md). Sela source is under [MIT](LICENSE); third-party datasets and content retain their own terms.

**Built for people to be heard — not to be classified.**
