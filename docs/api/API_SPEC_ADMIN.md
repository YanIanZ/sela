# Sela | Operator API v1

**Status:** implemented locally (2026-10-30, branch `feat/admin-redis-foundation`); reading review added locally 2026-11-01 on `feat/reading-operator`; accessibility approval gate (`sela-reading-a11y-v1`, migration `1301_reading_accessibility`) added locally 2026-11-02; not verified live. The operator UI covers the dashboard, staff, planned care and reading review; research review queues are not part of this slice.

The operator console is a **separate trust domain**. It shares no cookie, session, table or authorization decision with the anonymous `/api/v3` product. Staff are provisioned out of band (`sela admin create`); there is no signup or invitation route in the API.

## 1. Conventions

- Base path: `/api/admin/v1`.
- Cookie: `__Host-sela_admin` (development over plain HTTP: `sela_admin`). `HttpOnly`, `SameSite=Strict`, `Secure` when `SELA_SECURE_COOKIES=true`. It never authorizes `/api/v3`.
- Every mutation requires `X-Sela-Request: 1` and a same-origin `Origin`, as `/api/v3` does; failures are `403 cross_origin`.
- JSON bodies, `Content-Type: application/json`, at most 16 KiB; otherwise `415 unsupported_media` / `413 too_large`.
- Errors use the shared envelope `{"error":{"code","message","field?"}}`. Codes added by this surface: `admin_unavailable` 503, `admin_invalid_credentials` 401, `admin_throttled` 429, `admin_not_signed_in` 401, `admin_forbidden` 403, `admin_last_owner` 409, `directory_forbidden` 403, `directory_conflict` 409, `cache_disabled` 503, `confirmation_required` 400, `reading_content_changed` 409, `reading_not_approved` 409, `accessibility_checklist_incomplete` 400, `accessibility_checks_changed` 409, `accessibility_checks_failed` 422, `accessibility_override_required` 422.
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
| `GET /reading` | `research/review` or `safety-content/review` | Every embedded reading item (38) with its full detail/preview, `contentHash`, embedded review record, the automatic `accessibility` report (§2.1), current or most recent operator `approval` (with its stored `accessibility` record, absent for approvals made before migration 1301), `hashMatches` and `published`. A top-level `accessibility` object carries `checksVersion`, `manualVersion`, the `manualChecklist` (`[{id, label:{id,en}}]`) and the override reason bounds `{min:20, max:300}`. Reviewer names and notes are editorial metadata; no user data. |
| `GET /reading/audit` | `research/review` or `safety-content/review` | Newest 50 reading review events: actor username when available, `reading.approve`, `reading.revoke` or `reading.a11y_override`, slug, old/new state and reason. For `reading.a11y_override` the old state is the comma-separated list of overridden check ids, the new state `overridden` and the reason the reviewer's override reason. Content-free. |
| `POST /reading/{slug}/approve` | `research/review` or `safety-content/review` | Body `{reviewerName, reviewedOn, note, verifierStatus:"supported", contentHash?, confirm:true, accessibility:{checksVersion, manualVersion, manualChecks:{<id>:true,…}, overrideReason}}`. `confirm` must be literally true (`400 confirmation_required`); the verifier must be `supported`; the reviewer name is 1–160 characters and the date is a real `YYYY-MM-DD` not in the future. `contentHash` is optional but, when present, must equal the current embedded item hash (`409 reading_content_changed`); the stored hash is always the current one. The accessibility gate (§2.1) then runs on the report the server computes from the same embedded bytes. Upserts the single current approval with its accessibility record and records the operator id/time plus `reading.approve` (and `reading.a11y_override` when overridden) in the same transaction. Never auto-approves. |
| `POST /reading/{slug}/revoke` | `research/review` or `safety-content/review` | Body `{reason?}`. Ends publication at once; the public routes stop serving the slug. Idempotent for an already-revoked approval. A slug with no approval row answers `409 reading_not_approved`. Records `reading.revoke`. |
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

### 2.1 Reading accessibility gate (`sela-reading-a11y-v1`)

Every reading item carries `accessibility: {version, result, checks[], metrics}` in `GET /reading`, computed server-side, deterministically, from the exact embedded item (`internal/reading/accessibility.go`). `result` is `pass`, `needs_override` or `blocked`. Each check is `{id, status: pass|fail|not_applicable, blocking, findings:[{code, path?, lang?, value?}]}`; `path` names the content field the way `tests/content/reading_validator.py` does.

| Check id | Blocking | Rule |
| --- | --- | --- |
| `language_parity` | yes | Every user-facing text exists in Indonesian and English; step lists have the same length. |
| `heading_structure` | yes | Title at most 15 words; a guide has sections; each section heading is non-empty, at most 12 words, unique within the item, not a repeat of the title; no empty section (WCAG 1.3.1, 2.4.6). |
| `list_structure` | yes | Exercises and safety plans have steps; at most 10; no typed numbers or bullets (the UI renders an ordered list). |
| `link_text` | yes | No bare URLs, no generic link phrases ("klik di sini", "click here", "selengkapnya", "read more"), markdown/HTML links have descriptive text; each cited source exists and has a real title (WCAG 2.4.4). Attribution and licence notices are not scanned for bare URLs because the CC BY-NC-SA licence requires the URI verbatim. |
| `image_alt` | yes | Any markdown or HTML image has non-empty alt text (WCAG 1.1.1). `not_applicable` when there are no images (the v1 schema has none). |
| `sensory_instructions` | yes | No sentence that relies only on colour, shape or position ("tombol hijau di kanan", "berwarna merah", "the round button"); a control named by its quoted label passes (WCAG 1.3.3, 1.4.1). |
| `sentence_length` | no | No sentence over 20 words (CONTENT_STYLE 3.1 rule 1). |
| `readability` | no | Average at most 15 words per sentence per language; English Flesch–Kincaid grade at most 8 (CONTENT_STYLE 3.2). Indonesian has no validated formula, so only sentence length is measured. Metrics match `sela-plain-language-v1`. |
| `block_length` | no | No paragraph or other text block over 80 words or 5 sentences. |

Approval protocol, in order (after the existing confirm/verifier/hash/date checks):

1. `accessibility` missing → `400 accessibility_checklist_incomplete`, field `accessibility`. `checksVersion` or `manualVersion` not the server's current version → `409 accessibility_checks_changed`, field `accessibility.checksVersion` / `accessibility.manualVersion` (the reviewer saw other rules; reload).
2. Any failing **blocking** check → `422 accessibility_checks_failed`, field `accessibility.checks.<first failing id>`. There is **no override** for blocking checks; the content file must change (which also changes the hash).
3. Every manual checklist item must be `true`, and no unknown ids → `400 accessibility_checklist_incomplete`, field `accessibility.manualChecks.<id>`. `sela-reading-a11y-manual-v1`: `read_aloud_plain_language`, `meaning_without_visuals`, `names_and_order`.
4. When only overridable checks fail, `overrideReason` (trimmed) must be 20–300 characters → otherwise `422 accessibility_override_required`, field `accessibility.overrideReason`. A reason sent when nothing failed → `400 invalid_input`, field `accessibility.overrideReason`, so the record never claims an exception that did not happen.

The stored approval returns `accessibility: {checksVersion, result: pass|overridden, checks, manualVersion, manualConfirmed, overrideReason?, overridden?}`. Publication (`/api/v3/reading`) is unchanged: it still binds the content hash; an approval recorded before migration 1301 keeps publishing and is shown in the console as "not recorded" (re-approval needed to record checks).

Permissions are re-checked on the server for every route; hiding a button is never the control. `staff/manage` covers the read-only staff list and revocation; role editing and two-person approval are not implemented. `directory/*` covers the planned-care demo (creator ≠ reviewer for real entries); reading review is open to either `research/review` (research editor) or `safety-content/review` (safety reviewer), because the library includes mental-health support content. `sandbox/write` is held by owner and ops and only changes labelled synthetic rows.

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

Migration `0600_reading_reviews` (range 600–699):

- `reading_reviews` — one row per slug: `reviewer_name`, `reviewed_on`, `note`, `verifier_status` (checked to `supported`), `content_hash` (checked 64 hex characters), `approved_by` operator uuid, `approved_at`, `revoked_at`, `version`. The hash is the SHA-256 of the exact raw JSON item object in the embedded content set, so any text or formatting change invalidates the approval.
- `reading_review_audit` — append-only `reading.approve`/`reading.revoke` events with the actor uuid and the same shape as `care_audit`/`admin_audit`. `admin_audit` keeps a closed action check, so reading events live here and are read by `GET /reading/audit`; the two tables are not silently merged.

Migration `1301_reading_accessibility` (operator console range; requires 0600) adds to `reading_reviews`: `a11y_checks_version`, `a11y_result` (`pass`|`overridden`), `a11y_checks` (jsonb array, the report at approval time), `a11y_manual_version`, `a11y_manual` (jsonb array of confirmed ids) and `a11y_override_reason` (20–300 characters). The constraint `reading_reviews_a11y_complete` requires all of them together or none (legacy rows), and an override reason exactly when the result is `overridden`. `reading_review_audit` accepts the new action `reading.a11y_override`. The down migration drops the columns and the override events; approvals stay.

`approved_by` and the audit actor are opaque uuids without a foreign key to `admin_users`: the reading range (0600) is applied before the operator range (0910), so a foreign key would make one migration range depend on another. `REVOKE ALL ... FROM PUBLIC` applies to both reading tables. They never reference or copy consumer records.

## 5a. Care network, partner invitations and the console home (D-076, D-079)

Professional care network routes (`/network/*`, `/partner/*`) are specified in [API_SPEC_CARE_NETWORK](API_SPEC_CARE_NETWORK.md). Added by D-079:

| Method and path | Permission | Notes |
| --- | --- | --- |
| `GET /tasks` | any internal operator | `{tasks:{network?:{pendingOrgs,pendingLicenses,openInvites}, moderation?, directoryPending?}}`, each key only for permissions held; partners `403` |
| `POST /network/invites` | `network/manage` | `{username, role: partner_admin\|practitioner\|partner_staff, careOrgId, displayName?, professionalType?, licenseNo?}` → `201 {invite, activationUrl}`; the URL carries a one-time token in its `#fragment` and is returned once; `409 username_taken` |
| `GET /network/orgs/{id}/invites` | `network/manage` or `network/verify` | invitations with `state` open, accepted, revoked or expired; never the token |
| `POST /network/invites/{id}/revoke` | `network/manage` | ends an open invitation |
| `POST /activation` | public (token in body, login throttle) | `{token}` → `{activation:{username, role, orgName, displayName, secret, otpauthUrl}}`; the TOTP secret is created on first use and sealed in the invitation; `410 invite_invalid` for an unknown, used, revoked or expired token |
| `POST /activation/complete` | public (token in body, login throttle) | `{token, password, totp}` → account created with the enrolled secret, invitation accepted and the first session opened in one transaction; sets the operator cookie; `400 password_weak`, `401 admin_invalid_credentials` (field `totp`), `410`; ten failed attempts end the invitation |
| `GET /partner/members` | partner roles | the organisation's accounts; licence numbers only for `partner_admin` |

Invitations last 72 hours; only the SHA-256 of the token is stored (migration `1300_partner_invites`); the inviting operator never sees the password or the TOTP secret. Audit actions `network.invite_create`, `network.invite_revoke`, `network.invite_accept`. Internal staff accounts stay out of band (D-061).

## 6. Deliberately absent

Role editing, two-person approval for staff grants, real partner onboarding or booking, research/safety review queues, provider budget editing, WebAuthn/passkeys and IP allowlisting. The UI at `/operator` consumes the routes above (dashboard, staff, planned care and reading review); these remaining workflows are target design in [Admin Console](../vision/ADMIN_CONSOLE.md) and must not be inferred from the presence of a role name or of the reading reviewer permission.
