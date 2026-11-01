# Documentation inventory

Updated: 1 November 2026 (28 previously unlisted documents classified: the vision set, ADR-0004/0005, v3 validation, research, audit and supporting READMEs); earlier: 26 October 2026 (v3 planning documents added; classifications updated after the v3 foundation commit `0ff3341`). Authority: [SPEC.md](../../SPEC.md). Every repository Markdown document is classified below. Supporting v1 SQL/JSON assets are in the relocation map. Node dependencies and generated build output are excluded. "v3 and v2" marks a current document that now describes the v3 plan as well as the implemented v2; a v3 planning document describes planned work only and marks nothing implemented.

**Reorganisation — 1 November 2026:** the flat `docs/` root was grouped by purpose into `product/`, `architecture/`, `api/`, `operations/`, `status/`, `decisions/` and `validation/` (see the [docs index](../README.md)); every link and code reference was rewritten, and no document was deleted. `OWNER_TESTER.md` moved to [validation/OWNER_ACCEPTANCE_GUIDE.md](../validation/OWNER_ACCEPTANCE_GUIDE.md), `docs/API_SPEC.md` became [api/API_SPEC_V2.md](../api/API_SPEC_V2.md), the 27 October plan moved to [archive/plans/](../archive/plans/2026-10-27-v3-core-journey.md), and the v1 `proto/carebridge/dataset.proto` moved to [archive/v1/carebridge/](../archive/v1/carebridge/dataset.proto). The superseded EvidenceGuard and DeepSeek generation-fallback prototypes (`prototypes/`, D-065) and their CI workflows were removed; they remain in git history up to `e1bf7fc`.

**Added — 1 November 2026:** [roadmap/SAFE_FLOW_AND_TODO.md](../roadmap/SAFE_FLOW_AND_TODO.md) (plan, weaknesses, TODO), `CODE_OF_CONDUCT.md`; `CONTRIBUTING.md` and `SECURITY.md` rewritten (D-074, first recorded as a duplicate D-069).

## Current files

| Document | Classification |
| --- | --- |
| [.github/PULL_REQUEST_TEMPLATE.md](../../.github/PULL_REQUEST_TEMPLATE.md) | Current project / v2 supporting documentation |
| [CONTRIBUTING.md](../../CONTRIBUTING.md) | Current project / v3 and v2 supporting documentation |
| [FORMAT.md](../../FORMAT.md) | Specification maintenance rules (v3 and v2) |
| [README.md](../../README.md) | Current project / v3 and v2 supporting documentation |
| [SECURITY.md](../../SECURITY.md) | Current project / v3 and v2 supporting documentation |
| [SPEC.md](../../SPEC.md) | Normative source of truth (v3; keeps the v2 requirements and tasks as history) |
| [data/research/README.md](../../data/research/README.md) | Supporting research data documentation |
| [docs/architecture/AI_POLICY.md](../architecture/AI_POLICY.md) | Current project / v3 and v2 supporting documentation |
| [docs/api/API_SPEC_V2.md](../api/API_SPEC_V2.md) | Current project / v2 supporting documentation |
| [docs/api/API_SPEC_V3.md](../api/API_SPEC_V3.md) | Current project / v3 API contract (foundation implemented and verified locally, CI passed on draft PR #9; feature sections reserved) |
| [docs/api/API_SPEC_ADMIN.md](../api/API_SPEC_ADMIN.md) | Current project / separate operator API v1 (implemented locally 2026-10-30; UI and directory workflows pending) |
| [docs/architecture/ARCHITECTURE.md](../architecture/ARCHITECTURE.md) | Current project / v3 and v2 supporting documentation |
| [docs/product/BACKLOG.md](../product/BACKLOG.md) | Current project / v3 planning document (backlog and milestones) |
| [docs/status/BLOCKERS.md](BLOCKERS.md) | Current project / v3 and v2 supporting documentation |
| [docs/architecture/CLAUDE_VERIFIER.md](../architecture/CLAUDE_VERIFIER.md) | Current project / verifier contract (owner decision C12; v3 role per D-030) |
| [docs/status/CONFLICT_MATRIX.md](CONFLICT_MATRIX.md) | Current project / v3 and v2 supporting documentation |
| [docs/decisions/DECISIONS.md](../decisions/DECISIONS.md) | Current project / v3 and v2 supporting documentation |
| [docs/decisions/DECISION_LOG.md](../decisions/DECISION_LOG.md) | Current project / v3 and v2 supporting documentation |
| [docs/architecture/DEEPSEEK_FALLBACK.md](../architecture/DEEPSEEK_FALLBACK.md) | Current project / optional generation fallback contract (owner decision C13; the v3 fallback route follows it, ADR-0003) |
| [docs/product/DEMO_DATA.md](../product/DEMO_DATA.md) | Current project / v2 supporting documentation |
| [docs/product/DENGARIN_FEATURE_MATRIX.md](../product/DENGARIN_FEATURE_MATRIX.md) | Current project / v3 planning document (Dengar.in feature and content inventory with decisions) |
| [docs/operations/DEVELOPMENT.md](../operations/DEVELOPMENT.md) | Current project / v3 and v2 supporting documentation |
| [docs/status/DOCUMENT_INVENTORY.md](DOCUMENT_INVENTORY.md) | Current project / v3 and v2 supporting documentation |
| [docs/validation/EVALUATION.md](../validation/EVALUATION.md) | Current project / v2 supporting documentation |
| [docs/status/IMPLEMENTATION_STATUS.md](IMPLEMENTATION_STATUS.md) | Current project / v3 and v2 supporting documentation |
| [docs/architecture/META_EVENT_NOTES.md](../architecture/META_EVENT_NOTES.md) | Current project / v2 supporting documentation |
| [docs/architecture/META_INTEGRATION.md](../architecture/META_INTEGRATION.md) | Current project / v3 and v2 supporting documentation |
| [docs/product/PRODUCT.md](../product/PRODUCT.md) | Current project / v3 and v2 supporting documentation |
| [docs/README.md](../README.md) | Current project / v3 and v2 supporting documentation |
| [docs/status/REQUIREMENTS_TRACEABILITY.md](REQUIREMENTS_TRACEABILITY.md) | Current project / v3 and v2 supporting documentation |
| [docs/product/ROADMAP.md](../product/ROADMAP.md) | Current project / v3 and v2 supporting documentation |
| [docs/operations/RUNBOOK.md](../operations/RUNBOOK.md) | Current project / v3 and v2 supporting documentation |
| [docs/architecture/SAFETY.md](../architecture/SAFETY.md) | Current project / v3 and v2 supporting documentation |
| [docs/product/SELA_PROVENANCE.md](../product/SELA_PROVENANCE.md) | Current project / v3 and v2 supporting documentation |
| [docs/validation/VALIDATION.md](../validation/VALIDATION.md) | Current project / v2 supporting documentation |
| [docs/adr/ADR-0001-meta-open-weight-model-on-modal.md](../adr/ADR-0001-meta-open-weight-model-on-modal.md) | Current project / v2 architecture decision record |
| [docs/adr/ADR-0002-persistence-accounts-and-encryption.md](../adr/ADR-0002-persistence-accounts-and-encryption.md) | Current project / v3 architecture decision record (accepted; the foundation is implemented and verified locally, CI passed on draft PR #9) |
| [docs/adr/ADR-0003-ai-routing-verifier-fallback.md](../adr/ADR-0003-ai-routing-verifier-fallback.md) | Current project / v3 architecture decision record (accepted as design; implementation in progress on branch `feat/sela-v3-ai`, not merged; nothing verified; live use blocked externally, B-1, B-8, B-9) |
| [docs/archive/v1/AI_POLICY.md](../archive/v1/AI_POLICY.md) | Archived v1 / superseded draft |
| [docs/archive/v1/AMD_01_SCOPE_DAN_SYARAT.md](../archive/v1/AMD_01_SCOPE_DAN_SYARAT.md) | Archived v1 / superseded draft |
| [docs/archive/v1/AMD_SETUP.md](../archive/v1/AMD_SETUP.md) | Archived v1 / superseded draft |
| [docs/archive/v1/AMD_TRACK2_DEVELOPMENT_PLAN.md](../archive/v1/AMD_TRACK2_DEVELOPMENT_PLAN.md) | Archived v1 / superseded draft |
| [docs/archive/v1/AMD_TRACK2_TASKS.md](../archive/v1/AMD_TRACK2_TASKS.md) | Archived v1 / superseded draft |
| [docs/archive/v1/API_SPEC.md](../archive/v1/API_SPEC.md) | Archived v1 / superseded draft |
| [docs/archive/v1/CAREBRIDGE_ARCHITECTURE.md](../archive/v1/CAREBRIDGE_ARCHITECTURE.md) | Archived v1 / superseded draft |
| [docs/archive/v1/CAREBRIDGE_DATASET.md](../archive/v1/CAREBRIDGE_DATASET.md) | Archived v1 / superseded draft |
| [docs/archive/v1/CAREBRIDGE_EVAL_RUBRIC.md](../archive/v1/CAREBRIDGE_EVAL_RUBRIC.md) | Archived v1 / superseded draft |
| [docs/archive/v1/CAREBRIDGE_KNOWLEDGE_BASE.md](../archive/v1/CAREBRIDGE_KNOWLEDGE_BASE.md) | Archived v1 / superseded draft |
| [docs/archive/v1/CAREBRIDGE_SAFETY_POLICY.md](../archive/v1/CAREBRIDGE_SAFETY_POLICY.md) | Archived v1 / superseded draft |
| [docs/archive/v1/CAREBRIDGE_STRUCTURE_MERMAID.md](../archive/v1/CAREBRIDGE_STRUCTURE_MERMAID.md) | Archived v1 / superseded draft |
| [docs/archive/v1/CB-035_GOOGLE_DEEPMIND_GEMMA_AMD.md](../archive/v1/CB-035_GOOGLE_DEEPMIND_GEMMA_AMD.md) | Archived v1 / superseded draft |
| [docs/archive/v1/CB-035_GO_NO_GO.md](../archive/v1/CB-035_GO_NO_GO.md) | Archived v1 / superseded draft |
| [docs/archive/v1/COLLABORATION_RULES.md](../archive/v1/COLLABORATION_RULES.md) | Archived v1 / superseded draft |
| [docs/archive/v1/CRISIS_CLASSIFICATION.md](../archive/v1/CRISIS_CLASSIFICATION.md) | Archived v1 / superseded draft |
| [docs/archive/v1/DEMO_SCRIPT.md](../archive/v1/DEMO_SCRIPT.md) | Archived v1 / superseded draft |
| [docs/archive/v1/GAP_AUDIT_2026-10-13.md](../archive/v1/GAP_AUDIT_2026-10-13.md) | Archived v1 / superseded draft |
| [docs/archive/v1/GEMMA_TRAINING_DRAFT.md](../archive/v1/GEMMA_TRAINING_DRAFT.md) | Archived v1 / superseded draft |
| [docs/archive/v1/HOSTED_FAULT_QA.md](../archive/v1/HOSTED_FAULT_QA.md) | Archived v1 / superseded draft |
| [docs/archive/v1/LEGACY_PROJECT_README.md](../archive/v1/LEGACY_PROJECT_README.md) | Archived v1 / superseded draft |
| [docs/archive/v1/M01_RATE_LIMITING.md](../archive/v1/M01_RATE_LIMITING.md) | Archived v1 / superseded draft |
| [docs/archive/v1/PRD-DESIGN.md](../archive/v1/PRD-DESIGN.md) | Archived v1 / superseded draft |
| [docs/archive/v1/PRD.md](../archive/v1/PRD.md) | Archived v1 / superseded draft |
| [docs/archive/v1/PROJECT_CONTEXT.md](../archive/v1/PROJECT_CONTEXT.md) | Archived v1 / superseded draft |
| [docs/archive/v1/README.md](../archive/v1/README.md) | Archive navigation |
| [docs/archive/v1/RECOVERY_MIGRATION.md](../archive/v1/RECOVERY_MIGRATION.md) | Archived v1 / superseded draft |
| [docs/archive/v1/REDIS.md](../archive/v1/REDIS.md) | Archived v1 / superseded draft |
| [docs/archive/v1/REDIS_SETUP.md](../archive/v1/REDIS_SETUP.md) | Archived v1 / superseded draft |
| [docs/archive/v1/REDIS_TESTING_CHECKLIST.md](../archive/v1/REDIS_TESTING_CHECKLIST.md) | Archived v1 / superseded draft |
| [docs/archive/v1/RELEASE_GATE_2026-10-16.md](../archive/v1/RELEASE_GATE_2026-10-16.md) | Archived v1 / superseded draft |
| [docs/archive/v1/RUANG_NGOBROL.md](../archive/v1/RUANG_NGOBROL.md) | Archived v1 / superseded draft |
| [docs/archive/v1/SAFETY.md](../archive/v1/SAFETY.md) | Archived v1 / superseded draft |
| [docs/archive/v1/SCRIPTWRITE_CHANGELOG.md](../archive/v1/SCRIPTWRITE_CHANGELOG.md) | Archived v1 / superseded draft |
| [docs/archive/v1/SCRUM-47_LIVE_EVAL_2026-10-16.md](../archive/v1/SCRUM-47_LIVE_EVAL_2026-10-16.md) | Archived v1 / superseded draft |
| [docs/archive/v1/SELA_V2_DESIGN.md](../archive/v1/SELA_V2_DESIGN.md) | Archived v1 / superseded draft |
| [docs/archive/v1/SUBMISSION_COPY.md](../archive/v1/SUBMISSION_COPY.md) | Archived v1 / superseded draft |
| [docs/archive/v1/Scriptwrite.md](../archive/v1/Scriptwrite.md) | Archived v1 / superseded draft |
| [docs/archive/v1/TESTBENCH.md](../archive/v1/TESTBENCH.md) | Archived v1 / superseded draft |
| [docs/archive/v1/THREAT_MODEL.md](../archive/v1/THREAT_MODEL.md) | Archived v1 / superseded draft |
| [docs/archive/v1/UX_FLOW.md](../archive/v1/UX_FLOW.md) | Archived v1 / superseded draft |
| [docs/archive/v1/VOICE_ACCESSIBILITY.md](../archive/v1/VOICE_ACCESSIBILITY.md) | Archived v1 / superseded draft |
| [docs/archive/v1/VOICE_AUDITION.md](../archive/v1/VOICE_AUDITION.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/BUG_2026-10-16_FIRST_VISIT_MODAL_RACE.md](../archive/v1/bugs/BUG_2026-10-16_FIRST_VISIT_MODAL_RACE.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/BUG_2026-10-16_MANUAL_REVIEW_INVALIDATION.md](../archive/v1/bugs/BUG_2026-10-16_MANUAL_REVIEW_INVALIDATION.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/BUG_2026-10-16_REDIS_PRODUCTION_WARMUP.md](../archive/v1/bugs/BUG_2026-10-16_REDIS_PRODUCTION_WARMUP.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/BUG_2026-10-16_SERVER_CONSENT_FALLBACK.md](../archive/v1/bugs/BUG_2026-10-16_SERVER_CONSENT_FALLBACK.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/README.md](../archive/v1/bugs/README.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261014-01-accessibility-init-script-ssr-leak.md](../archive/v1/bugs/fixed-bug-20261014-01-accessibility-init-script-ssr-leak.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261015-01-missing-protobufjs-dependency.md](../archive/v1/bugs/fixed-bug-20261015-01-missing-protobufjs-dependency.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-01-public-route-consent-test.md](../archive/v1/bugs/fixed-bug-20261016-01-public-route-consent-test.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-02-approximate-duration-extraction.md](../archive/v1/bugs/fixed-bug-20261016-02-approximate-duration-extraction.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-03-first-build-rollout-budget.md](../archive/v1/bugs/fixed-bug-20261016-03-first-build-rollout-budget.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-04-font-build-network.md](../archive/v1/bugs/fixed-bug-20261016-04-font-build-network.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-05-local-storage-mnemonic-plaintext.md](../archive/v1/bugs/fixed-bug-20261016-05-local-storage-mnemonic-plaintext.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-06-sync-backup-id-in-url-query.md](../archive/v1/bugs/fixed-bug-20261016-06-sync-backup-id-in-url-query.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-07-node-typeless-package-json-warnings.md](../archive/v1/bugs/fixed-bug-20261016-07-node-typeless-package-json-warnings.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-08-care-passport-export-markdown-injection.md](../archive/v1/bugs/fixed-bug-20261016-08-care-passport-export-markdown-injection.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-09-mobile-dock-active-highlight-confusion.md](../archive/v1/bugs/fixed-bug-20261016-09-mobile-dock-active-highlight-confusion.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-10-privacy-popup-body-scroll-leak.md](../archive/v1/bugs/fixed-bug-20261016-10-privacy-popup-body-scroll-leak.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-11-care-passport-a11y-focus-and-names.md](../archive/v1/bugs/fixed-bug-20261016-11-care-passport-a11y-focus-and-names.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-12-cache-handler-redis-build-id-namespace.md](../archive/v1/bugs/fixed-bug-20261016-12-cache-handler-redis-build-id-namespace.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-13-playwright-standalone-test-runner.md](../archive/v1/bugs/fixed-bug-20261016-13-playwright-standalone-test-runner.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-14-next-metadata-route-path-escaping.md](../archive/v1/bugs/fixed-bug-20261016-14-next-metadata-route-path-escaping.md) | Archived v1 / superseded draft |
| [docs/archive/v1/bugs/fixed-bug-20261016-15-test-runners-environment-fallbacks.md](../archive/v1/bugs/fixed-bug-20261016-15-test-runners-environment-fallbacks.md) | Archived v1 / superseded draft |
| [docs/archive/v1/eval/A11Y_AUDIT_REPORT_2026-10-16.md](../archive/v1/eval/A11Y_AUDIT_REPORT_2026-10-16.md) | Archived v1 / superseded draft |
| [docs/archive/v1/eval/EVAL_REPORT.md](../archive/v1/eval/EVAL_REPORT.md) | Archived v1 / superseded draft |
| [docs/archive/v1/research/RISET_FITUR_DAN_GAP_2026-10-13.md](../archive/v1/research/RISET_FITUR_DAN_GAP_2026-10-13.md) | Archived v1 / superseded draft |
| [docs/design/ACCESSIBILITY_CRITERIA.md](../design/ACCESSIBILITY_CRITERIA.md) | Current project / v3 design document (design lead; design decision, not implemented) |
| [docs/design/CONTENT_STYLE.md](../design/CONTENT_STYLE.md) | Current project / v3 design document (design lead; design decision, not implemented) |
| [docs/design/DESIGN_DIRECTION.md](../design/DESIGN_DIRECTION.md) | Current project / v3 design document (design lead; design decision, not implemented) |
| [docs/design/INFORMATION_ARCHITECTURE.md](../design/INFORMATION_ARCHITECTURE.md) | Current project / v3 design document (design lead; design decision, not implemented) |
| [docs/design/SCREENS.md](../design/SCREENS.md) | Current project / v3 design document (design lead; design decision, not implemented) |
| [docs/research/CORPUS_PIPELINE.md](../research/CORPUS_PIPELINE.md) | Current project / v2 research pipeline documentation |
| [modal/MODEL_NOTES.md](../../modal/MODEL_NOTES.md) | Current project / v2 verified model and serving facts |
| [modal/README.md](../../modal/README.md) | Current project / v2 Modal serving runbook |
| prototypes/evidenceguard/README.md (removed after commit e1bf7fc) | Standalone verifier prototype documentation (not wired into v2) |
| prototypes/generationfallback/README.md (removed after commit e1bf7fc) | Standalone DeepSeek fallback prototype documentation (not wired into v2) |
| [CODE_OF_CONDUCT.md](../../CODE_OF_CONDUCT.md) | Current project / community conduct policy |
| [web/DESIGN.md](../../web/DESIGN.md) | Current project / v3 frontend design system |
| [eval/verifier/README.md](../../eval/verifier/README.md) | Supporting evaluation inputs (EV-0 seed set; inputs only, no evaluation run; provider parts superseded by D-065) |
| [data/research/register_inputs/frozen/README.md](../../data/research/register_inputs/frozen/README.md) | Supporting research data documentation |
| [data/content/reading/LICENSE.md](../../data/content/reading/LICENSE.md) | Licence notice for the reading content |
| [internal/safety/testdata/v1/README.md](../../internal/safety/testdata/v1/README.md) | Supporting safety test data (v1 crisis snapshot) |
| [docs/adr/ADR-0004-anonymous-first-meaning-integrity-care-network.md](../adr/ADR-0004-anonymous-first-meaning-integrity-care-network.md) | Current project / ADR (anonymous-first, meaning integrity, care discovery; largely target design) |
| [docs/adr/ADR-0005-admin-and-redis-acceleration.md](../adr/ADR-0005-admin-and-redis-acceleration.md) | Current project / ADR (operator console and Redis; implemented locally, T23 and T24) |
| [docs/api/V3_CONSULTATION_AND_SHARING.md](../api/V3_CONSULTATION_AND_SHARING.md) | Current project / v3 P4 API contract (its header still says planned; summaries and sharing are implemented, see implementation status) |
| [docs/audits/REPOSITORY_ALIGNMENT_2026-10-29.md](../audits/REPOSITORY_ALIGNMENT_2026-10-29.md) | Historical audit (its voice and dictation work was removed by D-060) |
| [docs/content/READING_LIBRARY.md](../content/READING_LIBRARY.md) | Current project / reading content set v1 (0 of 38 items approved) |
| [docs/research/EVIDENCE_MAP_V3.md](../research/EVIDENCE_MAP_V3.md) | Research / machine-assembled, unreviewed evidence map |
| [docs/research/V3_GAP_SEARCH.md](../research/V3_GAP_SEARCH.md) | Research / frozen gap search runs with machine screening |
| [docs/validation/V3_CORE_VALIDATION.md](../validation/V3_CORE_VALIDATION.md) | Historical validation checkpoint (2026-10-27; frontend since replaced) |
| [docs/validation/V3_CONTINUITY_VALIDATION.md](../validation/V3_CONTINUITY_VALIDATION.md) | Local validation evidence (continuity increment) |
| [docs/validation/V3_DASHBOARD_VALIDATION.md](../validation/V3_DASHBOARD_VALIDATION.md) | Local validation evidence (dashboard and workspace) |
| [docs/validation/V3_FRONTEND_VALIDATION.md](../validation/V3_FRONTEND_VALIDATION.md) | Local validation evidence (v3 frontend delivery) |
| [docs/validation/V3_INFO_PORTAL_VALIDATION.md](../validation/V3_INFO_PORTAL_VALIDATION.md) | Local validation evidence (public information portal) |
| [docs/vision/README.md](../vision/README.md) | Owner-directed target design (vision set); not implementation evidence — see the document's own status line |
| [docs/vision/ARCHITECTURE.md](../vision/ARCHITECTURE.md) | Owner-directed target design (vision set); not implementation evidence — see the document's own status line |
| [docs/vision/ADMIN_CARE_OPERATIONS.md](../vision/ADMIN_CARE_OPERATIONS.md) | Owner-directed target design (vision set); not implementation evidence — see the document's own status line |
| [docs/vision/ADMIN_CONSOLE.md](../vision/ADMIN_CONSOLE.md) | Owner-directed target design (vision set); not implementation evidence — see the document's own status line |
| [docs/vision/AI_EVIDENCE_SAFETY.md](../vision/AI_EVIDENCE_SAFETY.md) | Owner-directed target design (vision set); not implementation evidence — see the document's own status line |
| [docs/vision/DELIVERY_AND_ACCEPTANCE.md](../vision/DELIVERY_AND_ACCEPTANCE.md) | Owner-directed target design (vision set); not implementation evidence — see the document's own status line |
| [docs/vision/MEANING_INTEGRITY_ENGINE.md](../vision/MEANING_INTEGRITY_ENGINE.md) | Owner-directed target design (vision set); not implementation evidence — see the document's own status line |
| [docs/vision/PRODUCT_REQUIREMENTS.md](../vision/PRODUCT_REQUIREMENTS.md) | Owner-directed target design (vision set); not implementation evidence — see the document's own status line |
| [docs/vision/PROFESSIONAL_CARE_NETWORK.md](../vision/PROFESSIONAL_CARE_NETWORK.md) | Owner-directed target design (vision set); not implementation evidence — see the document's own status line |
| [docs/vision/REDIS_AI_COST_ENGINEERING.md](../vision/REDIS_AI_COST_ENGINEERING.md) | Owner-directed target design (vision set); not implementation evidence — see the document's own status line |


## Research and proposal package

These additions are non-normative, title/abstract-screened research and design proposals. They do not override SPEC.md or mark implementation complete.

| Document / asset | Classification |
| --- | --- |
| [docs/research/sela-relay/01_REPO_AUDIT.md](../research/sela-relay/01_REPO_AUDIT.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/02_PRODUCT_STRATEGY.md](../research/sela-relay/02_PRODUCT_STRATEGY.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/03_JOURNEY_AND_UX.md](../research/sela-relay/03_JOURNEY_AND_UX.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/04_GO_REFACTOR.md](../research/sela-relay/04_GO_REFACTOR.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/05_META_AND_JUDGING.md](../research/sela-relay/05_META_AND_JUDGING.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/06_RESEARCH_METHOD.md](../research/sela-relay/06_RESEARCH_METHOD.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/07_EVIDENCE_TO_FEATURE.md](../research/sela-relay/07_EVIDENCE_TO_FEATURE.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/08_ROADMAP_AND_VALIDATION.md](../research/sela-relay/08_ROADMAP_AND_VALIDATION.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/09_RESEARCH_REGISTER.md](../research/sela-relay/09_RESEARCH_REGISTER.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/10_EVIDENCE_APPRAISAL_AND_CLAIMS.md](../research/sela-relay/10_EVIDENCE_APPRAISAL_AND_CLAIMS.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/11_PRODUCT_VERTICAL_SLICE.md](../research/sela-relay/11_PRODUCT_VERTICAL_SLICE.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/12_GO_META_IMPLEMENTATION_CONTRACT.md](../research/sela-relay/12_GO_META_IMPLEMENTATION_CONTRACT.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/13_JUDGING_AND_DEMO_PACKAGE.md](../research/sela-relay/13_JUDGING_AND_DEMO_PACKAGE.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/14_METADATA_SCREENING_PROTOCOL.md](../research/sela-relay/14_METADATA_SCREENING_PROTOCOL.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/RESEARCH_COLLECTION_2026-10-26.md](../research/sela-relay/RESEARCH_COLLECTION_2026-10-26.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/RESEARCH_EXPANSION_PIPELINE.md](../research/sela-relay/RESEARCH_EXPANSION_PIPELINE.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/README.md](../research/sela-relay/README.md) | Non-normative research / proposal documentation |
| [docs/research/sela-relay/research_100.bib](../research/sela-relay/research_100.bib) | Supporting research metadata / citation export |
| [docs/research/sela-relay/research_100.json](../research/sela-relay/research_100.json) | Supporting research metadata / citation export |
| [docs/research/sela-relay/search_queries.json](../research/sela-relay/search_queries.json) | Supporting research metadata / citation export |

## Complete v1 relocation map

Every pre-refactor document/asset under `docs/` other than the maintained index, provenance and event notes was moved. No historical artifact was silently deleted.

| Previous location | New location |
| --- | --- |
| `docs/architecture/AI_POLICY.md` | [docs/archive/v1/AI_POLICY.md](../archive/v1/AI_POLICY.md) |
| `docs/AMD_01_SCOPE_DAN_SYARAT.md` | [docs/archive/v1/AMD_01_SCOPE_DAN_SYARAT.md](../archive/v1/AMD_01_SCOPE_DAN_SYARAT.md) |
| `docs/AMD_SETUP.md` | [docs/archive/v1/AMD_SETUP.md](../archive/v1/AMD_SETUP.md) |
| `docs/AMD_TRACK2_DEVELOPMENT_PLAN.md` | [docs/archive/v1/AMD_TRACK2_DEVELOPMENT_PLAN.md](../archive/v1/AMD_TRACK2_DEVELOPMENT_PLAN.md) |
| `docs/AMD_TRACK2_TASKS.md` | [docs/archive/v1/AMD_TRACK2_TASKS.md](../archive/v1/AMD_TRACK2_TASKS.md) |
| `docs/api/API_SPEC_V2.md` | [docs/archive/v1/API_SPEC.md](../archive/v1/API_SPEC.md) |
| `docs/CAREBRIDGE_ARCHITECTURE.md` | [docs/archive/v1/CAREBRIDGE_ARCHITECTURE.md](../archive/v1/CAREBRIDGE_ARCHITECTURE.md) |
| `docs/CAREBRIDGE_DATASET.md` | [docs/archive/v1/CAREBRIDGE_DATASET.md](../archive/v1/CAREBRIDGE_DATASET.md) |
| `docs/CAREBRIDGE_EVAL_RUBRIC.md` | [docs/archive/v1/CAREBRIDGE_EVAL_RUBRIC.md](../archive/v1/CAREBRIDGE_EVAL_RUBRIC.md) |
| `docs/CAREBRIDGE_KNOWLEDGE_BASE.md` | [docs/archive/v1/CAREBRIDGE_KNOWLEDGE_BASE.md](../archive/v1/CAREBRIDGE_KNOWLEDGE_BASE.md) |
| `docs/CAREBRIDGE_SAFETY_POLICY.md` | [docs/archive/v1/CAREBRIDGE_SAFETY_POLICY.md](../archive/v1/CAREBRIDGE_SAFETY_POLICY.md) |
| `docs/CAREBRIDGE_STRUCTURE_MERMAID.md` | [docs/archive/v1/CAREBRIDGE_STRUCTURE_MERMAID.md](../archive/v1/CAREBRIDGE_STRUCTURE_MERMAID.md) |
| `docs/CB-035_GOOGLE_DEEPMIND_GEMMA_AMD.md` | [docs/archive/v1/CB-035_GOOGLE_DEEPMIND_GEMMA_AMD.md](../archive/v1/CB-035_GOOGLE_DEEPMIND_GEMMA_AMD.md) |
| `docs/CB-035_GO_NO_GO.md` | [docs/archive/v1/CB-035_GO_NO_GO.md](../archive/v1/CB-035_GO_NO_GO.md) |
| `docs/COLLABORATION_RULES.md` | [docs/archive/v1/COLLABORATION_RULES.md](../archive/v1/COLLABORATION_RULES.md) |
| `docs/CRISIS_CLASSIFICATION.md` | [docs/archive/v1/CRISIS_CLASSIFICATION.md](../archive/v1/CRISIS_CLASSIFICATION.md) |
| `docs/DEMO_SCRIPT.md` | [docs/archive/v1/DEMO_SCRIPT.md](../archive/v1/DEMO_SCRIPT.md) |
| `docs/GAP_AUDIT_2026-10-13.md` | [docs/archive/v1/GAP_AUDIT_2026-10-13.md](../archive/v1/GAP_AUDIT_2026-10-13.md) |
| `docs/GEMMA_TRAINING_DRAFT.md` | [docs/archive/v1/GEMMA_TRAINING_DRAFT.md](../archive/v1/GEMMA_TRAINING_DRAFT.md) |
| `docs/HOSTED_FAULT_QA.md` | [docs/archive/v1/HOSTED_FAULT_QA.md](../archive/v1/HOSTED_FAULT_QA.md) |
| `docs/LEGACY_PROJECT_README.md` | [docs/archive/v1/LEGACY_PROJECT_README.md](../archive/v1/LEGACY_PROJECT_README.md) |
| `docs/M01_RATE_LIMITING.md` | [docs/archive/v1/M01_RATE_LIMITING.md](../archive/v1/M01_RATE_LIMITING.md) |
| `docs/PRD-DESIGN.md` | [docs/archive/v1/PRD-DESIGN.md](../archive/v1/PRD-DESIGN.md) |
| `docs/PRD.md` | [docs/archive/v1/PRD.md](../archive/v1/PRD.md) |
| `docs/PROJECT_CONTEXT.md` | [docs/archive/v1/PROJECT_CONTEXT.md](../archive/v1/PROJECT_CONTEXT.md) |
| `docs/RECOVERY_MIGRATION.md` | [docs/archive/v1/RECOVERY_MIGRATION.md](../archive/v1/RECOVERY_MIGRATION.md) |
| `docs/REDIS.md` | [docs/archive/v1/REDIS.md](../archive/v1/REDIS.md) |
| `docs/REDIS_SETUP.md` | [docs/archive/v1/REDIS_SETUP.md](../archive/v1/REDIS_SETUP.md) |
| `docs/REDIS_TESTING_CHECKLIST.md` | [docs/archive/v1/REDIS_TESTING_CHECKLIST.md](../archive/v1/REDIS_TESTING_CHECKLIST.md) |
| `docs/RELEASE_GATE_2026-10-16.md` | [docs/archive/v1/RELEASE_GATE_2026-10-16.md](../archive/v1/RELEASE_GATE_2026-10-16.md) |
| `docs/RUANG_NGOBROL.md` | [docs/archive/v1/RUANG_NGOBROL.md](../archive/v1/RUANG_NGOBROL.md) |
| `docs/architecture/SAFETY.md` | [docs/archive/v1/SAFETY.md](../archive/v1/SAFETY.md) |
| `docs/SCRIPTWRITE_CHANGELOG.md` | [docs/archive/v1/SCRIPTWRITE_CHANGELOG.md](../archive/v1/SCRIPTWRITE_CHANGELOG.md) |
| `docs/SCRUM-47_LIVE_EVAL_2026-10-16.md` | [docs/archive/v1/SCRUM-47_LIVE_EVAL_2026-10-16.md](../archive/v1/SCRUM-47_LIVE_EVAL_2026-10-16.md) |
| `docs/SELA_V2_DESIGN.md` | [docs/archive/v1/SELA_V2_DESIGN.md](../archive/v1/SELA_V2_DESIGN.md) |
| `docs/SUBMISSION_COPY.md` | [docs/archive/v1/SUBMISSION_COPY.md](../archive/v1/SUBMISSION_COPY.md) |
| `docs/Scriptwrite.md` | [docs/archive/v1/Scriptwrite.md](../archive/v1/Scriptwrite.md) |
| `docs/TESTBENCH.md` | [docs/archive/v1/TESTBENCH.md](../archive/v1/TESTBENCH.md) |
| `docs/THREAT_MODEL.md` | [docs/archive/v1/THREAT_MODEL.md](../archive/v1/THREAT_MODEL.md) |
| `docs/UX_FLOW.md` | [docs/archive/v1/UX_FLOW.md](../archive/v1/UX_FLOW.md) |
| `docs/VOICE_ACCESSIBILITY.md` | [docs/archive/v1/VOICE_ACCESSIBILITY.md](../archive/v1/VOICE_ACCESSIBILITY.md) |
| `docs/VOICE_AUDITION.md` | [docs/archive/v1/VOICE_AUDITION.md](../archive/v1/VOICE_AUDITION.md) |
| `docs/bugs/BUG_2026-10-16_FIRST_VISIT_MODAL_RACE.md` | [docs/archive/v1/bugs/BUG_2026-10-16_FIRST_VISIT_MODAL_RACE.md](../archive/v1/bugs/BUG_2026-10-16_FIRST_VISIT_MODAL_RACE.md) |
| `docs/bugs/BUG_2026-10-16_MANUAL_REVIEW_INVALIDATION.md` | [docs/archive/v1/bugs/BUG_2026-10-16_MANUAL_REVIEW_INVALIDATION.md](../archive/v1/bugs/BUG_2026-10-16_MANUAL_REVIEW_INVALIDATION.md) |
| `docs/bugs/BUG_2026-10-16_REDIS_PRODUCTION_WARMUP.md` | [docs/archive/v1/bugs/BUG_2026-10-16_REDIS_PRODUCTION_WARMUP.md](../archive/v1/bugs/BUG_2026-10-16_REDIS_PRODUCTION_WARMUP.md) |
| `docs/bugs/BUG_2026-10-16_SERVER_CONSENT_FALLBACK.md` | [docs/archive/v1/bugs/BUG_2026-10-16_SERVER_CONSENT_FALLBACK.md](../archive/v1/bugs/BUG_2026-10-16_SERVER_CONSENT_FALLBACK.md) |
| `docs/bugs/README.md` | [docs/archive/v1/bugs/README.md](../archive/v1/bugs/README.md) |
| `docs/bugs/fixed-bug-20261014-01-accessibility-init-script-ssr-leak.md` | [docs/archive/v1/bugs/fixed-bug-20261014-01-accessibility-init-script-ssr-leak.md](../archive/v1/bugs/fixed-bug-20261014-01-accessibility-init-script-ssr-leak.md) |
| `docs/bugs/fixed-bug-20261015-01-missing-protobufjs-dependency.md` | [docs/archive/v1/bugs/fixed-bug-20261015-01-missing-protobufjs-dependency.md](../archive/v1/bugs/fixed-bug-20261015-01-missing-protobufjs-dependency.md) |
| `docs/bugs/fixed-bug-20261016-01-public-route-consent-test.md` | [docs/archive/v1/bugs/fixed-bug-20261016-01-public-route-consent-test.md](../archive/v1/bugs/fixed-bug-20261016-01-public-route-consent-test.md) |
| `docs/bugs/fixed-bug-20261016-02-approximate-duration-extraction.md` | [docs/archive/v1/bugs/fixed-bug-20261016-02-approximate-duration-extraction.md](../archive/v1/bugs/fixed-bug-20261016-02-approximate-duration-extraction.md) |
| `docs/bugs/fixed-bug-20261016-03-first-build-rollout-budget.md` | [docs/archive/v1/bugs/fixed-bug-20261016-03-first-build-rollout-budget.md](../archive/v1/bugs/fixed-bug-20261016-03-first-build-rollout-budget.md) |
| `docs/bugs/fixed-bug-20261016-04-font-build-network.md` | [docs/archive/v1/bugs/fixed-bug-20261016-04-font-build-network.md](../archive/v1/bugs/fixed-bug-20261016-04-font-build-network.md) |
| `docs/bugs/fixed-bug-20261016-05-local-storage-mnemonic-plaintext.md` | [docs/archive/v1/bugs/fixed-bug-20261016-05-local-storage-mnemonic-plaintext.md](../archive/v1/bugs/fixed-bug-20261016-05-local-storage-mnemonic-plaintext.md) |
| `docs/bugs/fixed-bug-20261016-06-sync-backup-id-in-url-query.md` | [docs/archive/v1/bugs/fixed-bug-20261016-06-sync-backup-id-in-url-query.md](../archive/v1/bugs/fixed-bug-20261016-06-sync-backup-id-in-url-query.md) |
| `docs/bugs/fixed-bug-20261016-07-node-typeless-package-json-warnings.md` | [docs/archive/v1/bugs/fixed-bug-20261016-07-node-typeless-package-json-warnings.md](../archive/v1/bugs/fixed-bug-20261016-07-node-typeless-package-json-warnings.md) |
| `docs/bugs/fixed-bug-20261016-08-care-passport-export-markdown-injection.md` | [docs/archive/v1/bugs/fixed-bug-20261016-08-care-passport-export-markdown-injection.md](../archive/v1/bugs/fixed-bug-20261016-08-care-passport-export-markdown-injection.md) |
| `docs/bugs/fixed-bug-20261016-09-mobile-dock-active-highlight-confusion.md` | [docs/archive/v1/bugs/fixed-bug-20261016-09-mobile-dock-active-highlight-confusion.md](../archive/v1/bugs/fixed-bug-20261016-09-mobile-dock-active-highlight-confusion.md) |
| `docs/bugs/fixed-bug-20261016-10-privacy-popup-body-scroll-leak.md` | [docs/archive/v1/bugs/fixed-bug-20261016-10-privacy-popup-body-scroll-leak.md](../archive/v1/bugs/fixed-bug-20261016-10-privacy-popup-body-scroll-leak.md) |
| `docs/bugs/fixed-bug-20261016-11-care-passport-a11y-focus-and-names.md` | [docs/archive/v1/bugs/fixed-bug-20261016-11-care-passport-a11y-focus-and-names.md](../archive/v1/bugs/fixed-bug-20261016-11-care-passport-a11y-focus-and-names.md) |
| `docs/bugs/fixed-bug-20261016-12-cache-handler-redis-build-id-namespace.md` | [docs/archive/v1/bugs/fixed-bug-20261016-12-cache-handler-redis-build-id-namespace.md](../archive/v1/bugs/fixed-bug-20261016-12-cache-handler-redis-build-id-namespace.md) |
| `docs/bugs/fixed-bug-20261016-13-playwright-standalone-test-runner.md` | [docs/archive/v1/bugs/fixed-bug-20261016-13-playwright-standalone-test-runner.md](../archive/v1/bugs/fixed-bug-20261016-13-playwright-standalone-test-runner.md) |
| `docs/bugs/fixed-bug-20261016-14-next-metadata-route-path-escaping.md` | [docs/archive/v1/bugs/fixed-bug-20261016-14-next-metadata-route-path-escaping.md](../archive/v1/bugs/fixed-bug-20261016-14-next-metadata-route-path-escaping.md) |
| `docs/bugs/fixed-bug-20261016-15-test-runners-environment-fallbacks.md` | [docs/archive/v1/bugs/fixed-bug-20261016-15-test-runners-environment-fallbacks.md](../archive/v1/bugs/fixed-bug-20261016-15-test-runners-environment-fallbacks.md) |
| `docs/drafts/forum-thread-replies.sql` | [docs/archive/v1/drafts/forum-thread-replies.sql](../archive/v1/drafts/forum-thread-replies.sql) |
| `docs/eval/A11Y_AUDIT_REPORT_2026-10-16.md` | [docs/archive/v1/eval/A11Y_AUDIT_REPORT_2026-10-16.md](../archive/v1/eval/A11Y_AUDIT_REPORT_2026-10-16.md) |
| `docs/eval/EVAL_REPORT.md` | [docs/archive/v1/eval/EVAL_REPORT.md](../archive/v1/eval/EVAL_REPORT.md) |
| `docs/eval/mock_results.json` | [docs/archive/v1/eval/mock_results.json](../archive/v1/eval/mock_results.json) |
| `docs/forum-replies-migration.sql` | [docs/archive/v1/forum-replies-migration.sql](../archive/v1/forum-replies-migration.sql) |
| `docs/research/RISET_FITUR_DAN_GAP_2026-10-13.md` | [docs/archive/v1/research/RISET_FITUR_DAN_GAP_2026-10-13.md](../archive/v1/research/RISET_FITUR_DAN_GAP_2026-10-13.md) |

## Reference and scope notes

Source comments, script messages and fixture-read paths that referenced moved material now point to `docs/archive/v1/`. This relocation does not change application rules. Runtime migration is pending.

`README.md`, `CONTRIBUTING.md`, `SECURITY.md`, current docs and attached v1 READMEs were rewritten or status-scoped. Exact pre-edit revisions remain available at `d5da323` in Git. Archive headers override old words such as “active”, “done”, or “deployed” inside historical records.

Supporting design assets that are not Markdown live next to the design documents: [docs/design/tokens.css](../design/tokens.css) (token reference sheet) and the scripts in `docs/design/tools/` (`contrast.py` computes the WCAG contrast pairs; `sync_docs.py`). They are owned by the design lead and are not implemented in `web/` yet.

Test fixtures under `tests/research/fixtures/corpus/repo/` are trimmed synthetic copies used only by the corpus pipeline tests; they are not documentation. Add a classification here when adding/removing/moving a Markdown file. Update the archive index and reference paths when relocating historical assets.
