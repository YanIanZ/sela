# Sela | Operator API v1

**Status:** implemented locally (2026-10-30, branch `feat/admin-redis-foundation`); not verified live. The operator UI and directory/research workflows are not part of this slice.

The operator console is a **separate trust domain**. It shares no cookie, session, table or authorization decision with the anonymous `/api/v3` product. Staff are provisioned out of band (`sela admin create`); there is no signup or invitation route in the API.

## 1. Conventions

- Base path: `/api/admin/v1`.
- Cookie: `__Host-sela_admin` (development over plain HTTP: `sela_admin`). `HttpOnly`, `SameSite=Strict`, `Secure` when `SELA_SECURE_COOKIES=true`. It never authorizes `/api/v3`.
- Every mutation requires `X-Sela-Request: 1` and a same-origin `Origin`, as `/api/v3` does; failures are `403 cross_origin`.
- JSON bodies, `Content-Type: application/json`, at most 16 KiB; otherwise `415 unsupported_media` / `413 too_large`.
- Errors use the shared envelope `{"error":{"code","message","field?"}}`. Codes added by this surface: `admin_unavailable` 503, `admin_invalid_credentials` 401, `admin_throttled` 429, `admin_not_signed_in` 401, `admin_forbidden` 403, `admin_last_owner` 409, `directory_forbidden` 403, `directory_conflict` 409, `cache_disabled` 503.
- No response contains user text, a user id, a session token, a provider key or a Redis key dump.

## 2. Routes

| Method / route | Permission | Behaviour |
| --- | --- | --- |
| `POST /sessions` | none (rate limited) | `{"username","password","totp"}` → `{"admin":{...}}` and the operator cookie. Wrong password/TOTP and unknown usernames are the same `401 admin_invalid_credentials`; five failures start a one-minute backoff (`429 admin_throttled`). A TOTP step is accepted once (replay refused). |
| `DELETE /sessions/current` | signed in | Revokes the current session and clears the cookie. |
| `GET /me` | signed in | `{"admin":{"username","role","orgId","permissions":[...]}}`. |
| `GET /overview` | `ops/read` | Privacy-safe aggregates: `activeStaff`, `liveSessions`, `auditLast24h`, `cacheEnabled`, `databaseReady`, `version`, `generatedAt`; plus `aiSpend` and `aiSpendByTask` when a monthly AI budget is configured. |
| `GET /audit?limit=N` | `security/audit/read` | Newest first, at most 200 (`100` default). Actor username, action, object kind/id, old/new status, reason, time. |
| `GET /directory` | `directory/write` or `directory/verify` | Editorial list: every public directory entry with status, source, version and synthetic flag. |
| `POST /directory` | `directory/write` | Create a draft (`kind`, `name`, `city`, `description`, `contactUrl`, `sourceUrl`, `sourcePublisher`, `sourceAccessedAt`). `synthetic` is forced false; only the seed command creates labelled demo rows. |
| `PATCH /directory/{id}` | `directory/write` | Edit a draft/rejected entry with `expectedVersion`. |
| `POST /directory/{id}/submit` | `directory/write` | Draft/rejected → pending. |
| `POST /directory/{id}/verify` | `directory/verify` | Pending → verified; optional `reverifyAt` (YYYY-MM-DD, default +3 months). A non-synthetic entry needs a reviewer other than its creator (`403 directory_forbidden`). |
| `POST /directory/{id}/reject` | `directory/verify` | Pending → rejected with a reason. |
| `POST /directory/{id}/unpublish` | `directory/verify` | Verified/stale → unpublished. |
| `GET /directory/audit` | `directory/write` or `directory/verify` | Recent directory/sandbox events, content-free. |
| `GET /sandbox/requests` | `sandbox/write` | Fictional planned-support requests only. |
| `POST /sandbox/requests` | `sandbox/write` | Create a labelled simulation request. |
| `POST /sandbox/requests/{id}/transition` | `sandbox/write` | `prepared → pending → received → acknowledged → closed`, or revoked from any non-terminal state. Never calls a partner, sends email or books anything. |
| `GET /staff` | `staff/manage` | Staff identities only: username, role, organisation label, status, created/last-login times and live session count. Never a password hash, TOTP secret, token or user record. |
| `POST /staff/{username}/revoke` | `staff/manage` | `{"reason"}` (optional, max 200) sets the account to `revoked`, ends every live session, writes `admin.revoke` with the signed-in actor and returns `{username,status}`. Idempotent for an already-revoked account. The **last active owner cannot be revoked** (`409 admin_last_owner`). |
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

Permissions are re-checked on the server for every route; hiding a button is never the control. `staff/manage` covers the read-only staff list and revocation; role editing and two-person approval are not implemented. `directory/*` covers the planned-care demo (creator ≠ reviewer for real entries); `research/*` and `safety-content/*` still have no route. `sandbox/write` is held by owner and ops and only changes labelled synthetic rows.

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

Migration `0700_care_directory`: `care_directory_entries` (sourced public metadata and editorial state), `care_sandbox_requests` (fictional, always `synthetic = true`) and `care_audit`. No table references a consumer user; `created_by`/`verified_by` are opaque staff uuid values. The public route returns only `verified` entries whose `reverifyAt` is current; synthetic rows appear only in development/test/demo and are labelled.

## 6. Deliberately absent

Role editing, two-person approval for staff grants, real partner onboarding or booking, research/safety review queues, provider budget editing, WebAuthn/passkeys and IP allowlisting. The first UI slice at `/operator` consumes the routes above, including the staff list and guarded revocation; these remaining workflows are target design in [Admin Console](../vision/ADMIN_CONSOLE.md) and must not be inferred from the presence of a role name.
