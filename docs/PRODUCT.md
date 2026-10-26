# Sela v2 product

Status: target design, not delivered behavior. Authority: [SPEC](../SPEC.md) §G, C1–C9, I.web, V1–V8, V13.

Sela bridges the gap between finding words for distress and reaching professional support. The hackathon demonstrates that journey using fictional people and simulated care. Optional anonymity supports privacy; it is not the primary product positioning.

## Personas and scope

One fictional adult experiencing work stress is the primary case. An English-language adult case and a crisis-interruption case exercise the same rules. Professional profiles represent fictional psychologists/psychiatrists or other explicitly categorized services; the demo does not validate real credentials or make clinical suitability recommendations.

The first version is a responsive browser demo. It has a user perspective and a fictional professional perspective within the same isolated demo session. A role switch is visibly a demonstration control. No professional account registration or real patient access exists in this scope.

## Complete journey

| Stage | User action | Required result |
| --- | --- | --- |
| Start | Select seeded adult scenario and ID/EN | Visible synthetic-data notice; optional synthetic identity choice |
| Intake | Review seeded story; separately consent to AI | Descriptive, source-backed concerns or manual seeded-note path |
| Discovery | Set service/language/location/remote/cost/slot preferences | Deterministic fixture matches; honest empty state |
| Brief | Select/remove source-backed facts/questions; review draft | Versioned brief; no unsupported findings |
| Approval | Explicitly approve current version | Approval bound to version; any edit invalidates it |
| Handoff | Select fictional recipient/slot and shared profile fields | Preview and separate share consent; simulated request receipt |
| Professional view | Open demo inbox; acknowledge then complete scripted consultation | Only approved shared snapshot; clear simulated statuses |
| Follow-up | View fixture-authored plan; submit seeded check-in | Recorded synthetic progress and optional return-visit preview |
| Revoke/reset | Revoke shared access or reset session | Prior shared view is inaccessible; other sessions unaffected |

A crisis fixture interrupts the normal flow before AI or ordinary handoff. It does not proceed to booking, consultation completion or automated follow-up. Help information must identify its geography and verification status.

## Feature decisions

Required: text-first care journey, Meta text integration, manual path, fictional professional view, seeded follow-up, Indonesian and English, clear unavailable/revoked/expired states. Optional, separately consented Anthropic Sonnet 5.5 source-fidelity review is proposed for the current Go migration; never a substitute for Meta generation or human approval ([contract](CLAUDE_VERIFIER.md)).

Deferred: transcription of a synthetic sample with a verified Meta capability. No live microphone capture in the first demo.

Excluded from v2 MVP: forum, gamification, generic chatbot, medical diagnosis/prescribing, payments, real scheduling, real professional onboarding, electronic health-record integrations and model training. V1 features are retired only after the replacement acceptance checks pass.

All stages and error states have complete ID/EN copy. Language selection does not determine the person's country, provider availability, or emergency-service coverage. UI hierarchy, layouts and exact wording remain implementation/design choices; this doc is not a finished visual design.
