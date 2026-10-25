# Synthetic demo data

Status: target data contract; new fixtures are not implemented. Authority: [SPEC](../SPEC.md) I.data, C3/C7/C8, V1/V2/V6/V7/V8/V11.

Use deterministic, versioned fictional records. Never repurpose a real journal entry, patient story, provider identity, appointment, audio recording or clinical document. No real email, phone number, address, professional license or booking URL is required. Existing v1 evaluation fixtures remain historical until explicitly reviewed and adopted.

## Fixture families

| Record | Required contents |
| --- | --- |
| Scenario | Stable ID/version, adult persona, source evidence spans, ID/EN narrative, allowed questions and edits, scripted check-ins |
| Professional | Clearly fictional name, service category, language tags, synthetic location/remote status, demo cost, allowed slot IDs |
| Slot | Stable ID, professional ID, fixed demo timestamp, availability flag |
| Plan | Stable ID/version, linked scenario/professional, explicit fictional-professional attribution, predefined follow-up steps |
| Recorded AI result | Source fixture hash/version, labeled recorded provenance, original synthetic input reference; never default live evidence |
| Crisis case | Explicit synthetic hazard input and expected deterministic interruption; no ordinary handoff or plan |

Use a fixed demo clock for scenarios and available slots so recordings are repeatable. Display the demo date separately from real system time. Session expiry and operational timeout use the real clock. Language changes select translations of the same IDs rather than mutating the clinical story.

## Editable data and isolation

The MVP exposes seeded story and source-backed selection/edit controls, not open real-patient intake. API rejects unknown IDs, arbitrary personal fields, cross-scenario evidence, unrelated plans/slots and unsupported model claims. AI consent sends only the selected synthetic narrative/evidence needed for that operation.

Session state contains selected IDs, versions, bounded generated output, consent/approval events, shared snapshots, simulated handoff status and seeded follow-up choices. Exact limits are implementation constants documented with T3 evidence; do not leave memory unbounded.

Public visitors never share user state. A fictional professional view is a projection inside the same visitor's session. Reset/expiry purges that session's data and revokes shares; an in-flight model result is discarded if its generation/version is stale. No cross-session directory of patient snapshots exists.

Optional synthetic audio must have documented generation/source rights and language transcripts. No live microphone recording, real consultation transcription or automatic media generation is part of the MVP.
