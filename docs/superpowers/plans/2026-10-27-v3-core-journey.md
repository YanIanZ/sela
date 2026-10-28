# Sela v3 core journey implementation plan

Goal: replace the default v2 demo with a usable, bilingual v3 guest/account and personal-record journey, while keeping the v2 demo reachable explicitly.
Architecture: retain React/Vite and Go. Reuse v3 session cookies, account ownership, PostgreSQL, envelope encryption and exports. Add owned notes/check-ins and an authenticated listening room behind the existing AI router.
Spec: SPEC.md T15/T16 (in progress), I.web/I.api/I.auth/I.storage/I.export/I.ai; V3/V4/V12/V13/V22/V26–V28/V32–V38/V41/V42.

Constraints: no browser persistence of content; owner only from cookie; no provider call without current consent; preserve originals as revisions; no fictitious care claims; no production readiness claim. Follow docs/design tokens and ID/EN copy. Native implementation in this session, as requested.

Review focus: cross-owner access; save declined; reload/restart; stale versions or revoked consent; database/provider failures preserving typed text.

1. Add encrypted personal-record migration and store, with revision preservation, check-in upsert by local day, storage-choice requirement, export and cascade deletion. Test isolation, encryption at rest, stale update, revisions, export and crisis text preservation against PostgreSQL.
2. Expose authenticated v3 record routes and listening operations, applying existing origin, body/rate limits, safety, cancellation and server-side ownership. Wire consent checker and Meta-primary AI router; bound ephemeral conversations to owner/session and lifetime. Tests before implementation.
3. Build v3 shell, welcome/authentication, home, journal/check-in/timeline, conversation, support and privacy screens from existing design decisions. Keep v2 reachable under /demo. Test guest start, storage choice, error recovery, language and navigation.
4. Run Go race tests, frontend tests/typecheck/build, browser desktop/mobile and ID/EN journeys. Build Docker, migrate local PostgreSQL, run synthetic live AI checks, record exact completed and remaining work in docs.

No task is marked complete until its own acceptance criteria are met. No automatic public deployment or PR merge.
