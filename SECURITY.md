# Security policy

Sela stores and processes deeply personal writing. We treat every vulnerability that could expose it, mislead a person in distress or bypass a consent as serious.

## Supported versions

| Version | Where | Supported |
| --- | --- | --- |
| `main` (Sela v3) | Production: https://sela.vertalt.my.id | Yes |
| Older branches, the retired v2 demo and archived Dengar.in deployments | — | No |

The legacy Dengar.in deployments are not Sela and are not covered by this policy.

## Reporting a vulnerability

Report privately through [GitHub private vulnerability reporting](https://github.com/YanIanZ/sela/security/advisories/new). Please include:

- the affected commit, URL or path, and the impact;
- reproduction steps using **synthetic data only**;
- what you expected and what happened.

Never include real stories, health information, credentials, tokens, keys or another person's data. Do not open public issues, pull requests or posts about an exploitable problem until it is fixed. We aim to acknowledge reports within 3 working days and to agree a disclosure date with you; we credit reporters who want it.

**This is not an emergency channel.** If you or someone else is in danger, contact local emergency services or a crisis line.

### Safe harbour

Good-faith research that respects this policy is welcome. Stay within your own synthetic accounts, do not access or keep other people's data, do not degrade service (no load or denial-of-service testing on production), do not use social engineering, and stop and report as soon as you reach someone else's data.

### In scope

- Cross-user or cross-session access to records, conversations, shares, posts or moderation data
- Consent bypass (AI processing, storage, memory, condition map, sharing, community)
- Safety bypass: crisis text reaching a model, being stored contrary to the person's choice, or being published in the community
- Prompt injection or jailbreak that makes Sela output a diagnosis, medication advice, a professional persona, pressure, contact details or hidden instructions
- Moderation bypass that publishes bullying, harassment, doxxing or self-harm encouragement
- Share-link or access-code weaknesses, session fixation, CSRF, XSS, CSP bypass
- Secrets, personal content or identifiers in logs, errors or responses
- Operator console access control and audit gaps

### Out of scope

Reports that need a compromised device or browser, missing best-practice headers with no exploit, rate limits on public static pages, and findings on the legacy Dengar.in deployments.

## How Sela protects data

- **Sessions**: server-side sessions in a `__Host-` `HttpOnly`, `Secure`, `SameSite=Strict` cookie; rotation on privilege change; idle and absolute expiry.
- **Encryption**: personal content is encrypted per person (AES-256-GCM data key wrapped by a key-encryption key held outside the database, AAD bound to owner, record and field); deleting an account destroys its key.
- **Isolation**: the owner comes only from the session; every store method is owner-bound; another person's resource answers `404`; servers and operator commands refuse a database or key from another environment.
- **Mutations**: JSON only, size-limited, strict decoding, same-origin guard, rate limits, optimistic versions, idempotency keys.
- **Headers**: strict CSP (`'self'` only, no inline script), HSTS, `frame-ancestors 'none'`, `nosniff`, no referrer.
- **Safety and AI**: a deterministic crisis and danger router runs before storage, publishing and any model call; jailbreak and injection prefaces are neutralised; every model output passes hard guards, and a separate Meta verifier checks flagged output; provider keys stay on the server.
- **Community**: adults only; every post and reply passes the safety router and Meta moderation before anyone sees it; reports auto-hide; operators see posts only through the moderation queue and never a person's private space.
- **Operators**: separate identity with TOTP, role checks and an append-only audit; no access to private journals or conversations.
- **Logs**: metadata only; canary tests check that content never reaches them.

Design details and residual risks: [ADR-0002](docs/adr/ADR-0002-persistence-accounts-and-encryption.md), [safety](docs/architecture/SAFETY.md), [AI policy](docs/architecture/AI_POLICY.md) and the [current plan](docs/roadmap/SAFE_FLOW_AND_TODO.md). Sela has had internal security reviews but **no independent external audit yet**.
