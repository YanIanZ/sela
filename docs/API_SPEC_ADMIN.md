# Sela | Operator API v1

**Status:** implemented locally (2026-10-30, branch `feat/admin-redis-foundation`); not verified live. The operator UI and directory/research workflows are not part of this slice.

The operator console is a **separate trust domain**. It shares no cookie, session, table or authorization decision with the anonymous `/api/v3` product. Staff are provisioned out of band (`sela admin create`); there is no signup or invitation route in the API.

## 1. Conventions

- Base path: `/api/admin/v1`.
- Cookie: `__Host-sela_admin` (development over plain HTTP: `sela_admin`). `HttpOnly`, `SameSite=Strict`, `Secure` when `SELA_SECURE_COOKIES=true`. It never authorizes `/api/v3`.
- Every mutation requires `X-Sela-Request: 1` and a same-origin `Origin`, as `/api/v3` does; failures are `403 cross_origin`.
- JSON bodies, `Content-Type: application/json`, at most 16 KiB; otherwise `415 unsupported_media` / `413 too_large`.
- Errors use the shared envelope `{"error":{"code","message","field?"}}`. Codes added by this surface: `admin_unavailable` 503, `admin_invalid_credentials` 401, `admin_throttled` 429, `admin_not_signed_in` 401, `admin_forbidden` 403, `cache_disabled` 503.
- No response contains user text, a user id, a session token, a provider key or a Redis key dump.

## 2. Routes

| Method / route | Permission | Behaviour |
| --- | --- | --- |
| `POST /sessions` | none (rate limited) | `{"username","password","totp"}` → `{"admin":{...}}` and the operator cookie. Wrong password/TOTP and unknown usernames are the same `401 admin_invalid_credentials`; five failures start a one-minute backoff (`429 admin_throttled`). A TOTP step is accepted once (replay refused). |
| `DELETE /sessions/current` | signed in | Revokes the current session and clears the cookie. |
| `GET /me` | signed in | `{"admin":{"username","role","orgId","permissions":[...]}}`. |
| `GET /overview` | `ops/read` | Privacy-safe aggregates: `activeStaff`, `liveSessions`, `auditLast24h`, `cacheEnabled`, `databaseReady`, `version`, `generatedAt`; plus `aiSpend` and `aiSpendByTask` when a monthly AI budget is configured. |
| `GET /audit?limit=N` | `security/audit/read` | Newest first, at most 200 (`100` default). Actor username, action, object kind/id, old/new status, reason, time. |
| `GET /cache/health` | `ops/read` | `{"cache":{"enabled","ok","latencyMs?","error?"}}`. Never returns keys or values. |
| `POST /cache/invalidate` | `ops/cache/write` | Deletes only the revisioned public help-content key and appends an audit event. `503 cache_disabled` when Redis is not configured. |

## 3. Roles

| Role | Permissions |
| --- | --- |
| `owner` | all |
| `security_admin` | `ops/read`, `security/audit/read`, `staff/manage` |
| `directory_editor` | `ops/read`, `directory/write` |
| `directory_reviewer` | `ops/read`, `directory/verify` |
| `research_editor` | `ops/read`, `research/review` |
| `safety_reviewer` | `ops/read`, `safety-content/review` |
| `ops` | `ops/read`, `ops/cache/write` |

Permissions are re-checked on the server for every route; hiding a button is never the control. `directory/*`, `research/*`, `safety-content/*` and `staff/manage` have no route in this slice yet, so those permissions currently grant nothing beyond the listed routes.

## 4. Provisioning

```sh
printf '%s\n' "$OPERATOR_PASSWORD" | sela admin create --username ops.one --role ops [--org ORG]
sela admin revoke --username ops.one --reason "left the organisation"
```

`create` prints the TOTP secret and an `otpauth://` URI exactly once. `revoke` sets the account to `revoked`, ends every live session and is idempotent. Both commands audit the action. `SELA_ADMIN_MFA_KEY` (base64 of 32 bytes) is required; unset disables the whole surface with `503 admin_unavailable`.

## 5. Storage

Migration `0910_admin`:

- `admin_users` — username, argon2id hash and parameters, sealed TOTP secret, role, organisation label, status, failed count/backoff, timestamps. No foreign key to `users`.
- `admin_sessions` — token hash only, idle and absolute expiry, revocation reason.
- `admin_audit` — append-only operator actions; no story, prompt, IP or device data.

`REVOKE ALL ... FROM PUBLIC` applies to all three. The tables never reference or copy consumer records.

## 6. Deliberately absent

Staff list and role editing, two-person approval, directory/care operations, research/safety review queues, provider budget editing, partner sandbox, WebAuthn/passkeys and IP allowlisting. The first UI slice at `/operator` consumes the routes above; these remaining workflows are target design in [Admin Console](vision/ADMIN_CONSOLE.md) and must not be inferred from the presence of a role name.
