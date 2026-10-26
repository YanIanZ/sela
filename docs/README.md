# Sela documentation

**Start with [SPEC.md](../SPEC.md), the source of truth for Sela v2.** v2 (Go API + React/TypeScript frontend, Meta Muse Glimmer on Modal) is implemented and tested locally; live Meta inference and public deployment are blocked on owner credentials and budget ([blockers](BLOCKERS.md)). The inherited Next.js v1 stays runnable until the cutover.

## Authority and document ownership

1. User decisions govern product scope; record them in `SPEC.md`.
2. `SPEC.md` governs requirements, invariants and task status.
3. Active docs below elaborate those requirements; resolve conflicts in the spec first.
4. Source code and tests establish what runs; status claims cite them ([traceability](REQUIREMENTS_TRACEABILITY.md)).
5. Archived documents preserve old decisions and evidence; none override v2.

External event terms govern event eligibility and third-party rights; the repository's notes do not replace them. Use [FORMAT.md](../FORMAT.md) for IDs and status; never mark implementation complete from a document alone.

## Status and decisions

| Document | Responsibility |
| --- | --- |
| [Implementation status](IMPLEMENTATION_STATUS.md) | What is done, with the latest test, e2e and evaluation results |
| [Requirements traceability](REQUIREMENTS_TRACEABILITY.md) | Every requirement mapped to code, tests, evidence and a status |
| [Blockers](BLOCKERS.md) | External blockers and the owner action that clears each |
| [Conflict matrix](CONFLICT_MATRIX.md) | Sela Relay proposal vs spec vs implementation, with resolutions |
| [Decision log](DECISION_LOG.md) | Engineering and owner decisions D-001 onward |
| [Decision record](DECISIONS.md) | User-confirmed decisions versus revisable engineering defaults |
| [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md) | Meta open-weight model on Modal: options, licence, capability, cost |

## Product and design

| Document | Responsibility |
| --- | --- |
| [Product](PRODUCT.md) | Journey, guarantees per stage, exclusions |
| [Architecture](ARCHITECTURE.md) | Go packages, request path, sessions and sharing |
| [API contract](API_SPEC.md) | `/api/v2` resources, errors, SSE events, limits |
| [Synthetic demo data](DEMO_DATA.md) | Fixtures, practice text, isolation |
| [Safety](SAFETY.md) | Crisis routing, care boundaries, sharing, privacy |
| [AI policy](AI_POLICY.md) | Allowed model work, Meaning Lock, human control, provenance |
| [Meta integration](META_INTEGRATION.md) | Provider boundary, request parameters, failure behaviour |
| [DeepSeek fallback](DEEPSEEK_FALLBACK.md) | Optional transparent generator fallback, explicit opt-in, Go prototype and safety gates |
| [Claude verifier](CLAUDE_VERIFIER.md) | Optional second-pass source-fidelity verification (EvidenceGuard), independent consent, standalone Go prototype and evidence limitations |
| [Modal serving](../modal/README.md) and [model notes](../modal/MODEL_NOTES.md) | Deploy, secrets, cost, smoke test, rollback; verified model/server facts |

## Operating and validating

| Document | Responsibility |
| --- | --- |
| [Runbook](RUNBOOK.md) | Run, configure, deploy, cost controls, rollback, demo script |
| [Development](DEVELOPMENT.md) | v2 and v1 commands and checks |
| [Evaluation](EVALUATION.md) | Pre-registered thresholds, suites, results |
| [Validation and demo](VALIDATION.md) | Acceptance matrix and evidence |
| [Roadmap](ROADMAP.md) | Ordered stages and removal criteria |
| [Meta event notes](META_EVENT_NOTES.md) | Official references and unverified event requirements |
| [Provenance](SELA_PROVENANCE.md) | Credits, inherited work and the new-work boundary |
| [Document inventory](DOCUMENT_INVENTORY.md) | Every repository Markdown document |

## Research

[Sela Relay strategy and 100-publication evidence map](research/sela-relay/README.md) is the non-normative proposal and annotated register (title/abstract screened, not full-text appraisal or clinical validation). The [corpus pipeline](research/CORPUS_PIPELINE.md) rebuilds it reproducibly with verified dates, licence/access status, traceable machine extraction (unreviewed) and a reviewer search index.

## Historical implementation

[Archive index](archive/v1/README.md) contains the pre-refactor documentation, including AMD work, old product requirements, Next.js API contracts, Redis/PostgreSQL operations, evaluation reports and bug history. The [font provenance](../apps/web/src/app/fonts/README.md) remains current for the checked-in font. The [database](../services/persistence/migrations/README.md), [Supabase](../supabase/migrations/README.md) and [VPS](../deploy/vps/README.md) READMEs describe v1 only.
