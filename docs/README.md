# Sela documentation

**Start with [SPEC.md](../SPEC.md), the source of truth for Sela v2.** Backend Go is selected. V2 is designed, not implemented; the runnable application is still the inherited Next.js/TypeScript v1.

## Authority and document ownership

1. User decisions govern product scope; record them in `SPEC.md`.
2. `SPEC.md` governs requirements, invariants and task status.
3. Active docs below elaborate those requirements; resolve conflicts in the spec first.
4. Existing source/tests establish what currently runs, not what v2 already implements.
5. Archived documents preserve old decisions and evidence; none override v2.

External event terms govern event eligibility and third-party rights. The repo's notes do not replace those terms. Update the spec and affected docs together when a decision changes. Use [FORMAT.md](../FORMAT.md) for IDs/status; never mark implementation complete from a document alone.

## Active documents

| Document | Responsibility |
| --- | --- |
| [Source of truth](../SPEC.md) | Goal, confirmed constraints, engineering defaults, interfaces, invariants, task status |
| [Product](PRODUCT.md) | User/professional journeys, feature boundaries, languages |
| [Architecture](ARCHITECTURE.md) | Target Go/static-web design, state ownership, migration boundary |
| [API contract](API_SPEC.md) | Planned `/api/v2` resources, states, consent/version rules |
| [Synthetic demo data](DEMO_DATA.md) | Fixtures, identity choices, repeatability, session isolation |
| [Safety](SAFETY.md) | Crisis and care boundaries, sharing, privacy |
| [AI policy](AI_POLICY.md) | Allowed model work, validation, human control, truthful provenance |
| [Meta integration](META_INTEGRATION.md) | Primary generation boundary, access verification, failure behavior |
| [Claude verifier](CLAUDE_VERIFIER.md) | Optional second-pass source-fidelity verification, independent consent, Go prototype and evidence limitations |
| [Development](DEVELOPMENT.md) | Commands that work now and the future Go cutover |
| [Validation and demo](VALIDATION.md) | Acceptance matrix, demonstration script, evidence |
| [Migration roadmap](ROADMAP.md) | Ordered transition tasks and removal criteria |
| [Decision record](DECISIONS.md) | User-confirmed decisions versus revisable engineering defaults |
| [Meta event notes](META_EVENT_NOTES.md) | Official references and unresolved event requirements |
| [Provenance](SELA_PROVENANCE.md) | Credits, historical basis and new-work boundary |
| [Document inventory](DOCUMENT_INVENTORY.md) | Every repository Markdown document and moved legacy asset |

## Research and proposals

[Sela Relay strategy and 100-publication evidence map](research/sela-relay/README.md) contains the Indonesian product refinement, proposed journey and Go migration details, Meta/demo strategy, owner checklist and annotated references with JSON/BibTeX exports. It is non-normative: the register is title/abstract screened, not 100 full-text appraisals or clinical validation. Proposed features remain pending spec adoption and implementation.

## Historical implementation

[Archive index](archive/v1/README.md) contains the complete pre-refactor documentation, including AMD work, old product requirements, Next.js API contracts, Redis/PostgreSQL operations, evaluation reports and bug history. Old file paths in source comments and scripts now point there. License/source-attribution instructions stay attached to their artifacts.

The [font provenance](../apps/web/src/app/fonts/README.md) remains current for the checked-in font. The [database](../services/persistence/migrations/README.md), [Supabase](../supabase/migrations/README.md) and [VPS](../deploy/vps/README.md) READMEs describe v1 only.
