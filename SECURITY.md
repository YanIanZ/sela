# Security policy

## Current and planned runtime

The maintained branch is `main`. It currently runs the inherited Next.js/TypeScript v1; the Go-based Sela v2 is planned and has no security-validation claim. Historical deployments and archived competition snapshots are not supported releases.

## Report privately

Use [GitHub private vulnerability reporting](https://github.com/YanIanZ/sela/security/advisories/new). Include affected commit/paths, reproduction with synthetic data, expected behavior and impact. Do not include real stories, patient/professional information, journal entries, credentials or recovery keys. Do not disclose exploitable issues in public tickets. No response timeline is guaranteed.

## Relevant boundaries

For v2: consent/approval/share separation, versioned snapshots, crisis interruption, fixture validation, session isolation/expiry, idempotency, provider admission, secret handling and model-output validation. See [SPEC](SPEC.md), [safety](docs/SAFETY.md) and [AI policy](docs/AI_POLICY.md).

For v1: encrypted recovery, forum moderation, existing provider/cache/database behavior and deployment configuration remain relevant until retirement; consult the [historical threat model](docs/archive/v1/THREAT_MODEL.md). Old reports do not certify either runtime.

The new journey is a synthetic demonstration and accepts no actual patient-care transaction. GitHub reporting is not an emergency channel; immediate danger requires local emergency assistance.
