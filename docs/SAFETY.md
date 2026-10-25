# Sela v2 safety and privacy

Status: required target behavior, not a claim of clinical validation. Authority: [SPEC](../SPEC.md) V1–V8, V11–V13. Historical v1 controls are [archived](archive/v1/SAFETY.md).

## Care boundary

The demo shows access and handoff toward fictional professional care. Sela does not provide clinical diagnosis, prescriptions, treatment decisions, autonomous medical triage or actual consultations. The fictional professional's plan is a predefined demonstration artifact; AI cannot create or modify treatment instructions. All care-data surfaces identify the simulation in ID/EN.

## Crisis interruption

A deterministic crisis rule runs before inference and ordinary handoff transitions. The crisis fixture interrupts the journey and provides a clearly identified immediate-support screen. It makes no model call, marks no referral completed and schedules no visit. Any retained real help contact must carry geography, source and verification date; English does not imply global coverage. Unverified contacts are not presented as verified.

## Consent and disclosure

AI consent authorizes only a bounded synthetic processing operation. Brief approval approves one version. Sharing consent authorizes a selected snapshot, recipient and synthetic identity fields. None of these decisions implies another. The server validates them and rejects stale versions; client checkboxes alone are insufficient.

Edits invalidate approval. Withdrawal/reset/expiry cancels pending inference and invalidates stale responses. Share revocation removes the professional view's access even after a simulated consultation was completed. No old URL, guessed ID or demo role toggle restores it.

The professional view sees only explicitly selected fields and the approved brief; it cannot read private intake/history. Selected profile fields are synthetic. Optional anonymity remains available before handoff, with an explicit preview if a synthetic identity is chosen.

## Operations

Store bounded session data in memory and isolate each visitor. Apply body limits, same-origin mutation checks, opaque session cookies, expiry, admission limits and concurrency-safe updates. Metadata-only logs exclude text, prompts, provider responses, credentials and note content. Error messages do not echo provider payloads. See [API](API_SPEC.md) and [architecture](ARCHITECTURE.md).

Tests demonstrate the specified software behavior, not clinical safety certification. No actual patient or professional data is accepted in this demo. Private security reports follow [SECURITY.md](../SECURITY.md).
