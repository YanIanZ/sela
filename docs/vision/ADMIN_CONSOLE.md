# Sela | Admin Console & Governance

**Revision:** 2026-10-30 / AC-0.2
**Status:** target technical + product specification. The **staff identity, isolated API, roles, audit, cache controls and the first operator UI slice are implemented in the Go v3 runtime** (2026-10-30, §10). Directory/research workflows, two-person approval, budget editing and the partner workspace remain target design.
**Audience:** owner, operator, engineer, security/privacy reviewer, future verified provider administrator.
**Absolute principle:** anonymous Sela users do not create accounts. **Staff and partner operators must authenticate.** Admin is a separate trust domain, not an exception to anonymous privacy.

## 1. Why the admin exists

Sela needs a safe operational control plane for publishing trustworthy directories, keeping help contacts current, managing research curation, monitoring provider and Redis reliability, controlling AI spending, and managing **planned-support workflows**. None of those jobs should require unrestricted access to people's stories.

There are two independent products:

| Surface | Identity | Access |
| --- | --- | --- |
| Sela Personal Space | Anonymous, randomly scoped session | Conversations, journal, chosen memory, user-owned consultation summary and manual support discovery |
| Sela Operator Console | **Verified staff identity + MFA** | Directory/research operations, configuration within role, aggregates, synthetic partner cases; **no raw personal journal by default** |
| Partner Workspace (future) | Verified organization and professional accounts, contractual onboarding | Only explicit, scoped, time-limited patient-approved disclosures; **unavailable until real partnership** |

The previous Dengar.in Next.js app contains **legacy** `/api/admin/login`, `/api/admin/me`, `/api/admin/logout` and `services/persistence/scripts/seedAdmin.ts`. This is **not** a working Go v3 Admin Console. Do not reuse the legacy signed cookie/session or claim its authentication is production-approved. Build new Go routes and operator identity boundary; migrate or retire the old functionality intentionally.

## 2. Navigation and screens

~~~mermaid
flowchart TD
    A["Admin Console /operator"] --> D["Overview — health & aggregate KPIs"]
    A --> P["Care Operations"]
    P --> PR["Profession taxonomy"]
    P --> OR["Organizations & facilities"]
    P --> PE["Practitioners & affiliations"]
    P --> Q["Directory verification queue"]
    P --> SI["Synthetic partner sandbox"]
    A --> RE["Research editorial review"]
    A --> SA["Safety & official-help source freshness"]
    A --> AI["AI providers, usage & budget"]
    A --> RD["Redis / cache operations"]
    A --> AU["Audit, incidents & access reviews"]
    A --> ST["Staff, roles, MFA & feature flags"]
~~~

### Dashboard

Show: availability/uptime, Go API p95, Meta/Claude/DeepSeek availability and billed token spend, cache health/hit ratio, number of stale directory entries, research review queue, verified help-resource expiry and **synthetic** partner scenario status. No open journal contents, anonymous conversation previews, hidden user health profiles, or unapproved location inference. Only privacy-thresholded aggregate metrics; small groups are suppressed to avoid deanonymization.

### Care Operations

Backoffice for **public provider catalog curation** and **future** authorized care workflow. Operators verify directory records, attach sources, check stale affiliations and mark a facility listing as pending/rejected/verified; they do **not** infer user need, assign diagnoses, make appointments, or send unsolicited messages. For details see [Care Operations](ADMIN_CARE_OPERATIONS.md).

### Research and Safety Content

Editor reviews title/abstract and authorized text, rights, DOI/retraction warnings, population limits and citation anchors. Human/editor approval required for publishing. Safety contact review validates source and recheck date; an expired verified-help record is marked stale and escalated to the appropriate content owner. A reviewer can propose but not unilaterally bypass safety policy.

### AI & Redis Operations

Monitor aggregate cost, usage by task/provider, error/latency and cache health; staged non-clinical configuration changes, canary evaluation and rollbacks. The UI does not reveal provider API keys or input/output payloads. See [Redis and cost engineering](REDIS_AI_COST_ENGINEERING.md).

## 3. Roles and least-privilege policy

| Role | May do | Must not do |
| --- | --- | --- |
| Platform Owner | Configure roles, budgets, provider modes; approve access and release decisions | Circumvent consent/safety gates; view private journals by default |
| Security/Privacy Admin | Review access policies, security events and consent architecture; revoke staff access | Bulk-read personal content, edit clinical advice or silently expand scopes |
| Care Directory Editor | Create/correct draft organizations, facilities and practitioner public metadata | Publish own edits alone; see user journals or cases |
| Care Directory Reviewer | Independently verify sources and approve/reject directory publication | Approve their own submitted edits; invent professional credentials |
| Research Editor/Reviewer | Stage and verify licensed educational evidence | Access private journals or self-certify unreviewed research |
| Safety Content Reviewer | Review public help entries and documented safety copy | Override deterministic crisis routing by changing admin flags |
| Ops/SRE | View deployment/Redis/AI performance, manage safe limits and rollbacks | Read user text, lower safety controls or disable audit logging |
| Partner Administrator (future) | Manage verified organization membership and authorized scoped incoming cases | Cross-tenant access or view users without explicit valid grant |
| Professional (future) | View an exactly scoped approved capsule, if authenticated, and record permitted acknowledgement | Search private stories, create clinical outcomes in Sela without real authenticated clinical integration |

**Separation of duties:** creator != approver for directory credentials, safety contact publication, provider-routing changes and high-impact budget modifications. All scopes are deny-by-default at the API/database layer, not just hidden buttons. Use RBAC **plus** resource/organization-scoped ABAC for tenant and case authorization.

## 4. Staff login and security

- Staff identity only: provision out-of-band, verified invitation, no public signup; **phishing-resistant WebAuthn/passkeys preferred** and enforced MFA for production.
- Separate operator cookie/session/CSRF/origin/audience from anonymous-user cookie; dedicated route `/operator` with server-enforced `/api/admin/v1` permissions. Cookie `__Host-` with Secure/HttpOnly/SameSite, session rotation, idle + absolute expiry; revoke across devices.
- No normal user or existing guest cookie can authorize operator APIs. A staff session cannot impersonate an anonymous user.
- Password fallback, if allowed, must be strong, salted and adaptively hashed with rate limits, recovery policy, no initial default password.
- IP allowlisting/VPN or identity-aware gateway where feasible; do not rely on an IP alone for authentication.
- Protected staff enrollment, dual approval for privileged grants, immutable **PostgreSQL** audit for staff actions; log actor/time/object ID/old-new status/reason without health-story content.
- No "break-glass" private-story viewer in current scope. An emergency response does not grant staff unrestricted access to anonymous journals.
- Strong Content Security Policy, 2FA recovery safeguards, credential rotation and permission review.

## 5. Access to user information — hard boundary

No dashboard feed of anonymous conversations, no search-by-IP, no identity correlation, no admin "inspect all journals", no export of user history. Admin may see non-identifying operational aggregates and **public** directory/catalog records.

A future **real partner** can read **only** the specific Care Context Capsule with its explicit user-approved source versions, recipient/purpose/expiry and authenticated professional tenant, after legal/partner clearance. Admin staff never inherit the partner's disclosure permission.

Revocation stops future server access; already downloaded material cannot be recalled. User deletion and source correction must invalidate eligible shares and materialized summaries. Consent verification runs on every read and state change, not only at initial sharing.

## 6. Proposed API resources (design, not existing endpoints)

| Method / route | Operation | Policy |
| --- | --- | --- |
| POST `/api/admin/v1/sessions` | Authenticate staff | Strong auth, MFA and rate limits |
| DELETE `/api/admin/v1/sessions/current` | Revoke session | Staff-only |
| GET `/api/admin/v1/overview` | Health + privacy-safe aggregates | ops/read |
| GET/POST `/api/admin/v1/directory/organizations` | Browse / draft organization | directory/read, directory/write |
| PATCH `/api/admin/v1/directory/items/{id}` | Draft update | versioned editor scope |
| POST `/api/admin/v1/directory/items/{id}/verify` | Publish/reject with evidence | independent reviewer |
| GET `/api/admin/v1/research/review-queue` | Editorial queue | research/review |
| GET `/api/admin/v1/help/reverification` | Help contact freshness | safety-content/read |
| GET `/api/admin/v1/ai/budgets` | Provider cost/use | ai/finance/read |
| POST `/api/admin/v1/ai/budget-changes` | Propose bounded budget | operations/write + approval |
| GET `/api/admin/v1/cache/health` | Redis metrics only | ops/read |
| POST `/api/admin/v1/cache/invalidate` | Invalidate approved **public** prefixes | ops/write + audit |
| GET `/api/admin/v1/partner-sandbox/requests` | Fictional care requests | sandbox scope only |
| GET `/api/admin/v1/audit` | Scoped administrative audit | security/audit/read |

All mutations require idempotency where relevant, resource `expectedVersion`, server-side authorization, purpose, reason, CSRF protection and an audit event. No raw Redis command console, shell execution or SQL editor should be exposed via the web UI.

## 7. Operations state and source-of-truth

PostgreSQL owns staff identities, role grants, approvals, public directory versions, compliance/audit and any approved care request. Redis **cannot** grant permissions, retain session identity as the only authority, or be the only audit sink. Cache invalidation is derived from committed DB events with revision-stamped keys, and stale data cannot be used to assert verification.

## 8. Acceptance tests

- An anonymous session and an unrelated operator session remain isolated both ways.
- Unauthorized staff and cross-org partner cannot fetch one record by guessing its ID.
- MFA, staff role revocation, idle expiry, concurrent logout and login throttling are checked.
- Directory editor cannot self-approve professional credentials; evidence links required.
- Admin cannot view any personal journal/content from overview, logs, exports or debugging endpoints.
- Staff audit remains in PostgreSQL when Redis is offline; incidents and health reporting remain accessible without leaking content.
- A stale public directory result cannot become "verified" from cache alone.
- Sandbox cases are never shown as real cases; no outbound provider transfer is possible.
- Screen reader, keyboard, 320px reflow and bilingual UI tested.
- A failed provider or Redis connection does not authorize an unsafe state transition.

## 9. Existing work and migration

The legacy Next.js operator routes are **not** a template to blindly expose on the Go API; audit and rebuild privileged identity using the current threat model. Existing `internal/account` protects guest-owned data and must not be removed while cutting out consumer login UI. Track this as a separate **code PR**, not a documentation-only implementation claim.

See also [Care Operations](ADMIN_CARE_OPERATIONS.md), [System Architecture](ARCHITECTURE.md), [Product Requirements](PRODUCT_REQUIREMENTS.md), [Delivery Plan](DELIVERY_AND_ACCEPTANCE.md).

## 10. Implemented Go slice (2026-10-30)

Branch `feat/admin-redis-foundation`; not verified live.

| Area | Implemented | Evidence |
| --- | --- | --- |
| Separate identity | `admin_users` / `admin_sessions` (`0910_admin`), password (argon2id at the account floor) plus TOTP, sealed TOTP secrets under `SELA_ADMIN_MFA_KEY`, replay-protected steps, failed-login throttle | `internal/admin`, `TestAdminCreateAuthenticateAndLogout` |
| Isolation | `__Host-sela_admin` cookie (HttpOnly, Secure in production, SameSite=Strict) is distinct from the anonymous `__Host-sela_auth`; a consumer token cannot authorize `/api/admin/v1` | `TestAdminConsoleSeparateDomainAndPermissions` |
| Roles | Deny-by-default permission map for owner, security admin, directory editor/reviewer, research editor, safety reviewer and ops; server-side checks on every route | `internal/admin/admin.go`, role-scoping HTTP test |
| Audit | Append-only PostgreSQL `admin_audit` for login success/failure, logout, staff create/revoke and cache invalidation; no personal content | `ListAudit`, `admin_audit_created` index |
| Provisioning | `sela admin create --username NAME --role ROLE [--org ORG]` (password on stdin, TOTP secret shown once) and `sela admin revoke`; no self-signup route | `cmd/sela/admin.go` |
| Operations | `GET /api/admin/v1/me`, `/overview`, `/audit`, `/cache/health`; `POST /cache/invalidate` deletes only the revisioned public help key and audits it | `internal/httpapi/admin.go`, admin HTTP tests |
| Operator UI | `/operator`: password + TOTP sign-in, overview, AI spend, cache health and role-gated invalidation, permission-gated audit and staff list, ID/EN, 320 px reflow and dark mode. Mounted outside the consumer shell; cookie-only session | `web/src/operator/`, 8 component tests, `e2e/operator.e2e.mjs` (real-auth and final-build preview runs) |
| Staff | Read-only list (role, organisation, status, last sign-in, live sessions) and revocation with an inline confirmation; the last active owner and the signed-in account cannot be revoked from the UI. No role editing | `internal/admin.ListStaff`, `TestAdminStaffListLiveCountAndLastOwnerGuard`, `TestAdminStaffListAndRevokeRoutes` |
| Privacy | Overview carries aggregates only (staff, live sessions, audit count in 24 h, cache/database/version); AI spend is task/provider/token metadata without user or session ids | `admin.Overview`, `budget.Summary` |

Deliberately absent: role editing, two-person approval, directory and research workflows, AI budget editing, partner/sandbox routes, WebAuthn and IP allowlisting. A positive AI budget cap is enforced (see [Redis and cost engineering](REDIS_AI_COST_ENGINEERING.md) §11), but only the owner can change it through deployment configuration, not through the console. These gaps are **not** claimed as done; they belong to the next D10/D11 increment.
