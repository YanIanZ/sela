# Security policy

## Runtimes

The maintained branch is `main`. Sela v2 (Go API + static frontend, `cmd/` and `internal/`) is a synthetic demonstration; the inherited Next.js v1 stays runnable until the v2 cutover. Neither runtime has an external security audit. Historical deployments and archived competition snapshots are not supported releases.

## Report privately

Use [GitHub private vulnerability reporting](https://github.com/YanIanZ/sela/security/advisories/new). Include affected commit/paths, reproduction with synthetic data, expected behaviour and impact. Do not include real stories, patient or professional information, credentials or keys. Do not disclose exploitable issues in public tickets. No response timeline is guaranteed.

## v2 controls (tested in this repository)

- Sessions: random identifiers in `HttpOnly`, `SameSite=Strict`, path-scoped cookies (`Secure` in production); idle and absolute expiry; deletion purges the session, its shares and recipient principals. State is memory-only and bounded.
- Mutations: JSON only, size-limited, strict decoding (unknown fields rejected), same-origin guard, per-address rate limits, optimistic version checks, idempotency keys for handoffs.
- Sharing: a 256-bit token in the URL fragment plus a separate access code; the token alone never grants access; five wrong codes lock the share; recipient principals are scoped to their own cookie path; revocation, expiry and deletion end access with a uniform error.
- Headers: strict CSP (`'self'` only, no inline script), `frame-ancestors 'none'`, `nosniff`, no referrer; static files cannot escape the web directory.
- Logging: request metadata only (route pattern, status, duration, bytes); no stories, quotes, model output, tokens, codes or keys. Tests plant canary strings and check the logs.
- AI: crisis routing runs before any model call; only `modal` (Meta on Modal) or a non-production test double; credentials only from the server environment; redirects are never followed with credentials; every model output passes Meaning Lock before it is shown or stored.

See [SPEC](SPEC.md), [safety](docs/SAFETY.md), [AI policy](docs/AI_POLICY.md) and [API](docs/API_SPEC.md). For v1 (encrypted recovery, forum moderation, providers, database), consult the [historical threat model](docs/archive/v1/THREAT_MODEL.md). Old reports do not certify either runtime.

The demo accepts no actual patient-care transaction. GitHub reporting is not an emergency channel; immediate danger requires local emergency assistance.
