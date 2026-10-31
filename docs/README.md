# Sela documentation

Start with the [product vision](vision/README.md) for where Sela is going, and [SPEC.md](../SPEC.md) for the requirements the code must meet. Everything else is grouped by purpose:

| Folder | What lives there |
| --- | --- |
| [roadmap/](roadmap/SAFE_FLOW_AND_TODO.md) | The current plan: safer flow, weaknesses found and TODO |
| [vision/](vision/README.md) | Product direction, requirements, Meaning Integrity Engine, care network, admin console, Redis and AI cost |
| [product/](product/PRODUCT.md) | Product description, roadmap, backlog, demo data, Dengar.in lineage and provenance |
| [architecture/](architecture/ARCHITECTURE.md) | System architecture, AI policy, safety, Meta integration, verifier and fallback notes |
| [api/](api/API_SPEC_V3.md) | HTTP API contracts (v3 user API, admin API, v2 demo API, consultation and sharing) |
| [design/](design/DESIGN_DIRECTION.md) | Design direction, information architecture, screens, content style, accessibility criteria |
| [operations/](operations/DEVELOPMENT.md) | Local development and the operational runbook |
| [validation/](validation/VALIDATION.md) | Test and evaluation strategy, validation reports and the owner acceptance guide |
| [status/](status/IMPLEMENTATION_STATUS.md) | Implementation status, blockers, requirements traceability, conflict matrix, document inventory |
| [decisions/](decisions/DECISION_LOG.md) · [adr/](adr/) | Decision log, decision record and architecture decision records |
| [research/](research/CORPUS_PIPELINE.md) · [content/](content/READING_LIBRARY.md) | Research corpus pipeline, evidence map and the reading library |
| [evidence/](evidence/) · [audits/](audits/) | Screenshots and audit reports that back status claims |
| [archive/](archive/v1/README.md) | Retired v1 material and superseded plans, kept for history only |

## Detailed index

> **NEW OWNER DIRECTION — 29 Oct 2026:** Start with [Sela product & architecture vision](vision/README.md), [new PRD](vision/PRODUCT_REQUIREMENTS.md), [diagram-first architecture](vision/ARCHITECTURE.md), [Admin Console](vision/ADMIN_CONSOLE.md), [Planned Care Operations](vision/ADMIN_CARE_OPERATIONS.md) and [Redis & AI cost engineering](vision/REDIS_AI_COST_ENGINEERING.md). This is a target design that must be reconciled into SPEC.md, API contracts, backlog and implementations. The historical account-capable v3 scope described below remains valid for explaining current code **until migration is verified**. No fake live-AI or real hospital-partnership claims.

**Start with [SPEC.md](../SPEC.md), the source of truth.** Sela v3 — a real-use listening and continuity app in Indonesian and English for ages 15 to 50+ (owner brief v3, D-026) — is **planned**: its backend foundation is verified locally, and its CI run on draft PR #9 passed (commit `0ff3341` on branch `feat/sela-v3-pilot`, open as draft PR #9 against main; [foundation status note](../SPEC.md#foundation-status-note-2026-10-26)), and its design, frontend shell, AI provider layer, research, listening backend and operations are in progress ([implementation status](status/IMPLEMENTATION_STATUS.md#1-where-v3-stands)). Sela v2 (Go API + React/TypeScript frontend, Meta Muse Glimmer on Modal, synthetic demo) is implemented and verified locally (PR #6, merged into main as `7fc6678` on 2026-10-26); live Meta inference and public deployment are blocked on owner credentials, budget and hosting ([blockers](status/BLOCKERS.md)). The inherited Next.js v1 stays runnable until the cutover.

### Authority and document ownership

1. User decisions govern product scope (owner brief v3 of 2026-10-26 is the current one); record them in `SPEC.md`.
2. `SPEC.md` governs requirements, invariants and task status.
3. Active docs below elaborate those requirements; resolve conflicts in the spec first. PM decisions (D-027 to D-050 and D-054 onward) are recorded in the decision log and revisable by the owner.
4. Source code and tests establish what runs; status claims cite them ([traceability](status/REQUIREMENTS_TRACEABILITY.md)).
5. Archived documents preserve old decisions and evidence; none override the spec.

External terms (laws, provider terms, event rules) govern legality, eligibility and third-party rights; the repository's notes do not replace them. Use [FORMAT.md](../FORMAT.md) for IDs and status; never mark implementation complete from a document alone. Status words: planned → in progress → implemented → verified locally → verified live → ready for controlled pilot → public release, or blocked externally; [SPEC §G](../SPEC.md#g-goal) is their single definition.

### Status, plan and decisions

| Document | Responsibility |
| --- | --- |
| [Implementation status](status/IMPLEMENTATION_STATUS.md) | What is done, with the latest test, e2e and evaluation results |
| [Requirements traceability](status/REQUIREMENTS_TRACEABILITY.md) | Every requirement mapped to code, tests, evidence and a status |
| [Backlog (v3)](product/BACKLOG.md) | Work items by journey stage with problem, outcome, priority, scope, acceptance criteria, owner role, status and evidence; milestones P1–P8 |
| [Roadmap](product/ROADMAP.md) | v3 milestone order and release ladder with the checks per level; the v2 stages and the v1 removal criteria |
| [Blockers](status/BLOCKERS.md) | External blockers, split into implementation, launch and hackathon blockers, with the owner action that clears each |
| [Conflict matrix](status/CONFLICT_MATRIX.md) | Sela Relay proposal vs spec vs implementation (v2), and owner brief v3 vs the v2 spec and code, with resolutions |
| [Decision log](decisions/DECISION_LOG.md) | Owner, PM and engineering decisions D-001 onward; superseded entries point to their replacements |
| [Decision record](decisions/DECISIONS.md) | User-confirmed decisions versus PM decisions and engineering defaults |
| [Dengar.in feature matrix](product/DENGARIN_FEATURE_MATRIX.md) | Every inherited Dengar.in feature and content asset: condition, user problem, keep/improve/merge/migrate/pause/retire decision, acceptance criteria, evidence |
| [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md) | Meta open-weight model on Modal: options, licence, capability, cost |
| [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) | v3 persistence (PostgreSQL), accounts, guest mode, per-user encryption, export, deletion, backups, environments |
| [ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md) | v3 AI routing: Meta generator, Claude verifier, DeepSeek fallback; consent per provider, evaluation plan |

### Product and design

| Document | Responsibility |
| --- | --- |
| [Product](product/PRODUCT.md) | v3 journey, guarantees per surface, what Sela is not, pilot vs public release; the v2 demo journey |
| [Safety](architecture/SAFETY.md) | Crisis handling, second safeguards, verified help, minors, care boundaries, sharing, privacy |
| [Design direction](design/DESIGN_DIRECTION.md) | v3 visual language, principles, tokens and components (design lead); token sheet [tokens.css](design/tokens.css) |
| [Information architecture](design/INFORMATION_ARCHITECTURE.md) | v3 places, names (ID/EN), navigation and route map |
| [Screens](design/SCREENS.md) | Per-screen purpose, primary action, states, recovery, keyboard and mobile behaviour |
| [Content style](design/CONTENT_STYLE.md) | Voice, plain language and terms in Indonesian and English |
| [Accessibility criteria](design/ACCESSIBILITY_CRITERIA.md) | WCAG 2.2 AA acceptance criteria and how they are checked |
| [Architecture](architecture/ARCHITECTURE.md) | Go packages, request path, sessions and sharing |
| [API contract v2](api/API_SPEC_V2.md) | `/api/v2` resources, errors, SSE events, limits |
| [API contract v3](api/API_SPEC_V3.md) | `/api/v3` foundation (auth, guest, `/me`, consents, audit, export, deletion); reserved sections for later milestones |
| [Operator API v1](api/API_SPEC_ADMIN.md) | Separate staff identity, MFA, roles, audit and public-cache controls; implemented locally, UI pending |
| [Synthetic demo data](product/DEMO_DATA.md) | Fixtures, practice text, isolation |
| [AI policy](architecture/AI_POLICY.md) | Allowed model work, Meaning Lock, human control, provenance, v3 routing |
| [Meta integration](architecture/META_INTEGRATION.md) | Provider boundary, request parameters, failure behaviour |
| [DeepSeek fallback](architecture/DEEPSEEK_FALLBACK.md) | Optional transparent generator fallback, explicit opt-in, Go prototype and safety gates (owner contract; v3 routing follows it, ADR-0003 §17) |
| [Claude verifier](architecture/CLAUDE_VERIFIER.md) | Verifier role and contract, the standalone EvidenceGuard prototype and its evidence limits |
| EvidenceGuard prototype (removed after commit e1bf7fc) and generation fallback prototype (removed after commit e1bf7fc) | The owner's standalone Go prototypes (verifier; DeepSeek fallback), mock-tested and not wired into any runtime |
| [Modal serving](../modal/README.md) and [model notes](../modal/MODEL_NOTES.md) | Deploy, secrets, cost, smoke test, rollback; verified model/server facts |

### Operating and validating

| Document | Responsibility |
| --- | --- |
| [Runbook](operations/RUNBOOK.md) | Run, configure, deploy, cost controls, rollback, demo script |
| [Development](operations/DEVELOPMENT.md) | Go runtime (v2 demo and v3 foundation) and v1 commands and checks |
| [Evaluation](validation/EVALUATION.md) | v2 scope: pre-registered thresholds, suites, results. The v3 routing evaluations EV-0 to EV-7 and their thresholds are in [ADR-0003 §12](adr/ADR-0003-ai-routing-verifier-fallback.md#12-evaluation-plan-pre-registered-here-2026-10-26-to-be-added-to-evalthresholdsjson-as-a-new-version-before-the-first-verifier-run) |
| [Validation and demo](validation/VALIDATION.md) | v2 scope: acceptance matrix and evidence. v3 acceptance gates: [roadmap §2](product/ROADMAP.md#2-release-ladder) and the milestone exit criteria in [backlog §4](product/BACKLOG.md#4-milestones) |
| [Meta event notes](architecture/META_EVENT_NOTES.md) | Official references and unverified event requirements |
| [Provenance](product/SELA_PROVENANCE.md) | Credits, inherited work and the new-work boundary for v2 and v3 |
| [Document inventory](status/DOCUMENT_INVENTORY.md) | Every repository Markdown document |
| [Specification format](../FORMAT.md) and [contributing](../CONTRIBUTING.md) | ID, status and statement rules for the spec; how to contribute and which checks to run |

`docs/validation/OWNER_ACCEPTANCE_GUIDE.md` (prerequisites, test steps, expected results, scenario priorities and where to record results) is planned for milestone P8 ([SPEC](../SPEC.md) T21).

### Research

[Sela Relay strategy and 100-publication evidence map](research/sela-relay/README.md) is the non-normative proposal and annotated register (with the [research collection of 2026-10-26](research/sela-relay/RESEARCH_COLLECTION_2026-10-26.md) and the [research expansion pipeline](research/sela-relay/RESEARCH_EXPANSION_PIPELINE.md); the data files are described in [data/research](../data/research/README.md)) (title/abstract screened, not full-text appraisal or clinical validation). The [corpus pipeline](research/CORPUS_PIPELINE.md) rebuilds it reproducibly with verified dates, licence/access status, traceable machine extraction (unreviewed) and a reviewer search index. For v3, brief §9 asks for evidence on listening, journaling, help-seeking, consultation preparation, continuity, adolescents, adults, older adults, the Indonesian context, accessibility, safety and privacy. The known gaps are recorded in D-048 (a shorter list; brief §9 governs), and the gap searches and evidence map are in progress on branch `feat/sela-v3-research`, not merged; nothing verified (backlog S-243, S-244; [SPEC](../SPEC.md) T19).

### Historical implementation

[Archive index](archive/v1/README.md) contains the pre-refactor documentation, including AMD work, old product requirements, Next.js API contracts, Redis/PostgreSQL operations, evaluation reports and bug history. The v1 code paths and deployment READMEs it links to were removed on 2026-10-29 (S-272); the archive remains as provenance.
