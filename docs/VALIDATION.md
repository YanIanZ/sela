# Sela v2 acceptance and demo evidence

Status: planned checks, not passing results. Authority: [SPEC](../SPEC.md) §V and T2–T9. Historical v1 test reports do not establish v2 acceptance.

## Acceptance matrix

| Coverage | Required observation | Spec |
| --- | --- | --- |
| Complete journey, ID/EN | Seeded story through fictional consultation and follow-up; complete localized states | V1,V2,V13 |
| Crisis fixture | No inference or ordinary handoff; geography-labeled support screen | V3 |
| Consent refusal/withdrawal | Manual synthetic path; canceled/stale AI result cannot commit | V4,V9 |
| Edit after approval | Version changes, approval clears, stale handoff rejected | V5,V11 |
| Shared projection | Only selected identity fields and approved facts reach the fictional inbox | V5,V12 |
| Revoked/reset/expired session | All stale professional reads/actions denied; other visitors unaffected | V6,V11 |
| Model failures | Malformed/ungrounded output rejected, timeout bounded, manual path available | V7,V9,V10 |
| Discovery/empty availability | Exact fixture matches or honest empty state; no invented slot | V8 |
| Repeat/racing requests | One idempotent simulated receipt; stale/changed payload conflicts | V11 |
| Follow-up | Only allowed fixture plan after scripted completion; no model treatment changes | V7 |
| Data boundaries | Unknown fixture IDs and arbitrary personal payloads rejected; metadata-only logs | V2,V12 |
| Runtime cutover | Go/static bundle works without AMD, Node server, Redis or PostgreSQL dependencies | C6,C7,V14 |
| Resource claims | Equivalent v1/v2 workload, environment and raw measurement recorded | V15 |

## Repeatable demo

1. Start a fresh session with the work-stress adult fixture and visible synthetic label.
2. Choose ID/EN, inspect the seeded story and explicitly consent to Meta processing.
3. Show a validated draft and its live/recorded/manual provenance.
4. Filter fictional professionals and choose a fictional slot.
5. Edit source-backed selections, approve the current brief, preview fields and separately share.
6. Switch to the labeled professional perspective and show only the shared snapshot.
7. Simulate acknowledgement and consultation completion; display a fixture-authored plan.
8. Return to the user view, make a seeded follow-up check-in, then revoke/reset and prove the old view is inaccessible.
9. Run the separate crisis fixture and show that inference/booking are bypassed.
10. Demonstrate unavailable-model and no-AI-consent paths without pretending to call Meta.

Live API access is a separate prerequisite. If using a recording, say so throughout. Record the commit, fixture version, language, model/mode, checks and any limitations with the demo. Never use a real patient story or professional contact to make it more convincing.

## Baseline measurements

Pin environment, build mode, fixture and load. Measure process startup, idle RSS, active RSS, static asset sizes and non-model API latency. Report model latency separately. Compare equivalent functionality and session/load counts; a smaller partial rewrite does not prove a faster full product. No numeric performance target is invented in this documentation phase.

## Documentation baseline verified — 26 October 2026

This evidence covers the documentation refactor and existing v1 reference compatibility only; it does not complete any v2 runtime task.

- Inventory covers all 93 repository Markdown files; all 70 archived documents/assets exist and archived Markdown carries a superseded-status header.
- All 630 relative Markdown file links and 29 checked local section anchors resolve. No missing explicit `docs/` paths remain outside the archive.
- Spec task status/citation checks pass; all cited constraint/interface/invariant IDs exist.
- The 34 changed runtime/test/SQL files differ only by old-document-path to archive-path substitutions.
- `git diff --check`, `npm run test:migrations`, `npm run test:infra`, `npm run lint`, `npm run lint:services` and `npm run typecheck` pass.
- No new Go build, new Meta call, v2 behavioral test or Sela deployment is claimed by this evidence.

## Optional Claude Sonnet 5.5 verifier acceptance (T5a, NOT YET PRODUCT-IMPLEMENTED)

Mock-network Go adapter tests are in [prototypes/evidenceguard](../prototypes/evidenceguard/README.md). Beyond adapter tests, the future Go runtime must prove: Anthropic consent declined -> zero outbound calls; crisis fixture -> no Meta or Claude; malformed IDs/output rejected; mismatch means needs human review; timeout/missing key/429 -> explicit unavailable; changed brief or consent revoked mid-flight -> stale report dropped; professional sees only separately approved handoff snapshot; both ID and EN and negation edge cases retain source provenance. Live Sonnet 5.5 request, real usage/latency, and end-to-end session safety remain unverified until T5a/T8 evidence is recorded. A model-supported label is NOT clinical safety validation.

## Opt-in DeepSeek Flash 1017 fallback acceptance (T5b; PRODUCT INTEGRATION PENDING)

Go prototype mock tests in [prototypes/generationfallback](../prototypes/generationfallback/README.md) validate Meta-first generation, separate fallback consent, explicit provider attribution, nonfallback on bad/unsafe output, provider request format and fail-closed responses. The future integrated Go service must additionally verify real session-state atomic versioning, cancellation vs revoke races, per-provider secrets/quotas, language fidelity and crisis interruption before any call, end-to-end happy and failure paths, and a real live Meta + optional DeepSeek trace with strictly synthetic fixtures. A mocked fallback test does not prove a deployed integration, clinical validation or competition eligibility.
