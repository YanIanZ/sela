# Sela documentation

Start with the [project README](../README.md) for setup, features and current limitations.

## Current identity and event scope

- [Sela provenance](SELA_PROVENANCE.md): maintainer, original project, credits and compatibility decisions.
- [Meta event notes](META_EVENT_NOTES.md): source links, judging criteria and outstanding submission decisions.

## Engineering guides

These guides describe the inherited implementation. References to Dengar.in and `@dengarin/*` may remain where they identify the original architecture, storage or deployment. They do not establish a Sela deployment or a Meta integration.

| Topic | Guides |
| --- | --- |
| Architecture and API | [CareBridge](CAREBRIDGE_ARCHITECTURE.md), [API](API_SPEC.md), [diagrams](CAREBRIDGE_STRUCTURE_MERMAID.md) |
| Safety and privacy | [Safety](SAFETY.md), [AI policy](AI_POLICY.md), [CareBridge policy](CAREBRIDGE_SAFETY_POLICY.md), [crisis classification](CRISIS_CLASSIFICATION.md), [threat model](THREAT_MODEL.md) |
| Data and evaluation | [Dataset](CAREBRIDGE_DATASET.md), [knowledge base](CAREBRIDGE_KNOWLEDGE_BASE.md), [rubric](CAREBRIDGE_EVAL_RUBRIC.md), [test bench](TESTBENCH.md) |
| Voice and accessibility | [Voice room](RUANG_NGOBROL.md), [accessibility](VOICE_ACCESSIBILITY.md), [voice audition](VOICE_AUDITION.md) |
| Storage and operations | [Database migrations](../services/persistence/migrations/README.md), [Supabase migrations](../supabase/migrations/README.md), [recovery](RECOVERY_MIGRATION.md), [rate limiting](M01_RATE_LIMITING.md), [Redis](REDIS.md), [Redis setup](REDIS_SETUP.md), [Redis checks](REDIS_TESTING_CHECKLIST.md), [VPS deployment](../deploy/vps/README.md) |
| Bug history | [Bug index](bugs/README.md) |

## Historical project and competition records

These records are preserved for context. They are not the current Sela roadmap, team assignment, deployment status, or Meta submission copy.

- [Archived long project overview](LEGACY_PROJECT_README.md)
- Original product: [context](PROJECT_CONTEXT.md), [requirements](PRD.md), [design](PRD-DESIGN.md), [UX flows](UX_FLOW.md), [team collaboration](COLLABORATION_RULES.md)
- AMD work: [scope](AMD_01_SCOPE_DAN_SYARAT.md), [setup](AMD_SETUP.md), [development plan](AMD_TRACK2_DEVELOPMENT_PLAN.md), [task record](AMD_TRACK2_TASKS.md)
- Previous submission: [submission copy](SUBMISSION_COPY.md), [demo script](DEMO_SCRIPT.md)
- Historical evaluation: [report](eval/EVAL_REPORT.md), [accessibility audit](eval/A11Y_AUDIT_REPORT_2026-10-16.md), [release gate](RELEASE_GATE_2026-10-16.md), [gap audit](GAP_AUDIT_2026-10-13.md)

Historical files stay at their original paths so existing code comments and references continue to resolve.
