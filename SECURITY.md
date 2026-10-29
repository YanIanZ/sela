# Security policy

## Runtimes

The maintained branch is `main`. Sela v2 (Go API + static frontend, `cmd/` and `internal/`) is a synthetic demonstration. Sela v3, built for real use, is in development on `feat/sela-v3-pilot`: its backend foundation (`/api/v3`) is verified locally, and its CI run on draft PR #9 passed ([foundation status note](SPEC.md#foundation-status-note-2026-10-26)); it is not deployed. The inherited Next.js v1 stays runnable until the cutover. No runtime has an external security audit; the v3 foundation had three rounds of security review by agent reviewers independent of the implementer (internal; not the independent security assessment that public release requires). Historical deployments and archived competition snapshots are not supported releases.

## Report privately

Use [GitHub private vulnerability reporting](https://github.com/YanIanZ/sela/security/advisories/new). Include affected commit/paths, reproduction with synthetic data, expected behaviour and impact. Do not include real stories, patient or professional information, credentials or keys. Do not disclose exploitable issues in public tickets. No response timeline is guaranteed.

## v2 controls (tested in this repository)

- Sessions: random identifiers in `HttpOnly`, `SameSite=Strict`, path-scoped cookies (`Secure` in production); idle and absolute expiry; deletion purges the session, its shares and recipient principals. State is memory-only and bounded.
- Mutations: JSON only, size-limited, strict decoding (unknown fields rejected), same-origin guard, per-address rate limits, optimistic version checks, idempotency keys for handoffs.
- Sharing: a 256-bit token in the URL fragment plus a separate access code; the token alone never grants access; five wrong codes lock the share; recipient principals are scoped to their own cookie path; revocation, expiry and deletion end access with a uniform error.
- Headers: strict CSP (`'self'` only, no inline script), `frame-ancestors 'none'`, `nosniff`, no referrer; static files cannot escape the web directory.
- Logging: request metadata only (route pattern, status, duration, bytes); no stories, quotes, model output, tokens, codes or keys. Tests plant canary strings and check the logs.
- AI: crisis routing runs before any model call; only `modal` (Meta on Modal) or a non-production test double; credentials only from the server environment; redirects are never followed with credentials; every model output passes Meaning Lock before it is shown or stored.

## v3 foundation controls (verified locally, CI passed on draft PR #9; not deployed)

- Accounts: argon2id passwords with paste and password managers allowed, ten single-use recovery codes, uniform failures for unknown usernames, persisted per-username backoff and rate limits; server-side sessions in a `__Host-` cookie, rotated on sign-in, upgrade, password change and recovery.
- Data: PostgreSQL 16; personal content encrypted per user (AES-256-GCM data key wrapped by a key-encryption key from the secret manager, AAD bound to user, record and field); account deletion destroys the data key; backups split content and keys, and a restore reconciles deletions (one residual window after a total database loss, stated in the privacy text and accepted for a controlled pilot, D-055).
- Isolation: the owner comes only from the session cookie, every store method takes the owner, and another user's resource answers `404`; the server and every operator command refuse a database or key from another environment.
- Logs: metadata only; never content, usernames, passwords, recovery codes, tokens, user ids, keys, database URLs or IP addresses.

Details and residual risks: [ADR-0002](docs/adr/ADR-0002-persistence-accounts-and-encryption.md). See [SPEC](SPEC.md), [safety](docs/SAFETY.md), [AI policy](docs/AI_POLICY.md) and [API](docs/API_SPEC.md). For v1 (encrypted recovery, forum moderation, providers, database), consult the [historical threat model](docs/archive/v1/THREAT_MODEL.md). Old reports do not certify either runtime.

The demo accepts no actual patient-care transaction. GitHub reporting is not an emergency channel; immediate danger requires local emergency assistance.
