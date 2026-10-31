# Sela v3 API contract — foundation

Status: **the foundation routes (§4–§6) are implemented and verified locally, and their CI run on draft PR #9 passed** (commit `0ff3341`; ADR-0002 F-01 to F-47, [implementation status](../status/IMPLEMENTATION_STATUS.md#1-where-v3-stands); [SPEC §G foundation status note](../../SPEC.md#foundation-status-note-2026-10-26)); the feature sections (§9) are reserved and not implemented. Owner packages: `internal/httpapi` (routes, cookies, guards), `internal/account` (rules), `internal/vault`, `internal/db`. Design and rationale: [ADR-0002](../adr/ADR-0002-persistence-accounts-and-encryption.md). The v2 contract ([API_SPEC](API_SPEC_V2.md)) stays valid for `/api/v2`; v2 journey routes are served only in development, test and demo (ADR-0002 §12).

This document covers the **foundation** only: authentication, guest mode, recovery, the signed-in user (`/me`), sessions, consents, guardian consent, audit, export and account deletion. **Feature endpoints are added by later milestones in this same document**, in the reserved sections at the end: conversation, journal, check-in, timeline, memory, consultation summary, sharing (owner and recipient), and reading.

## 1. Conventions

- **Base path** `/api/v3`, same origin as the frontend, no CORS. JSON only (`Content-Type: application/json`) for request bodies.
- **Strict decoding** exactly as v2 (`internal/httpapi/strict.go`): unknown keys, keys matching a field only case-insensitively, duplicate keys, nesting deeper than 32 and trailing data → `400 invalid_input` (`field: "body"`). Body limit 16 KiB for every v3 body; larger → `413 too_large`. A wrong method on a real route → `405 method_not_allowed` with `Allow`; any other `/api/...` path → JSON `404 not_found`.
- **Envelopes:** success `{"data": …}`; error `{"error": {"code": "stable_code", "message": "localized text", "field": "optional"}}`. Message language: `X-Sela-Language: id|en`, else `Accept-Language`, else Indonesian. Clients localize by `code`; the server text is a fallback.
- **Mutation guard** on every `POST`, `PUT`, `PATCH`, `DELETE` (including login and registration): `X-Sela-Request: 1`, `Origin` (when sent) equal to the server origin or `SELA_PUBLIC_ORIGIN`, no cross-site `Sec-Fetch-Site`; else `403 cross_origin`. A non-empty non-DELETE body without `application/json` → `415 unsupported_media`.
- **Principal:** the owner is resolved only from the session cookie. No route accepts a user id in the path, query or body; `/me` always means the signed-in user. Owned resources of another user, and unknown ids, both answer `404 not_found`.
- **Times** are RFC 3339 UTC with second precision (`2026-10-26T13:05:00Z`). **Ids** are lowercase UUID strings.
- **Pagination** (lists that grow): `?limit=1..100` (default 50) and `?cursor=<opaque>`; responses carry `nextCursor` (`null` at the end). Order is newest first. A malformed cursor → `400 invalid_input` (`field: "cursor"`); so is a well-formed one that this server could not have produced (a time before 2000-01-01 or more than one day ahead, or an id that is not a version-4 UUID), which never reaches the database.
- **Optimistic concurrency:** `PATCH /me` sends `expectedVersion` = `user.version`; a mismatch → `409 stale_version` with no side effect. Other foundation mutations are independent per resource and need no version.
- **Idempotency:** foundation mutations are idempotent by nature or safe to retry, as stated per route. Responses that carry one-time secrets (recovery codes, session cookies) are **never replayed**, because the server keeps no plaintext copy; `Idempotency-Key` is not used in the foundation. A client that loses such a response recovers as documented on the route (for example, sign in and regenerate the codes).
- **Caching:** every response is `Cache-Control: no-store`.
- **Logs** record request id, route template, status, duration and, for auth routes, a fixed outcome; never bodies, usernames, passwords, codes, tokens, ids or IP addresses.

### Cookies

| Cookie | Attributes | Set by | Cleared by |
| --- | --- | --- | --- |
| `__Host-sela_auth` (`sela_auth` when `SELA_SECURE_COOKIES=false`, plain-HTTP development only) | `HttpOnly; Secure; SameSite=Strict; Path=/`; no `Domain`. Untrusted device: no `Max-Age` (ends with the browser). Trusted device and guest: `Max-Age` = seconds to the absolute expiry | register, login, guest start, guest upgrade, recovery, password change (rotation) | logout, logout-all (unless `keepCurrent`), revoking the current session, account deletion, and any request answered `401 not_signed_in` |

The value is a 256-bit random token (43 base64url characters). The server stores only its SHA-256. The v2 cookies (`sela_session`, `sela_recipient`) are separate and unchanged.

### Session lifetimes (defaults; configurable within bounds, see ADR-0002 §8)

| Session | Idle timeout | Absolute timeout |
| --- | --- | --- |
| Account, `rememberDevice: false` | 2 h | 12 h |
| Account, `rememberDevice: true` | 7 days | 30 days |
| Guest | 7 days (guest inactivity purge) | 30 days from guest creation (guest maximum age) |

A token is rotated (new value, old one revoked) on login, guest upgrade, password change and recovery. Exporting and deleting an account require a password proof within the last 10 minutes (`POST /auth/reauth`); guests are exempt because the cookie is their only credential.

### Rate limits

Client key = IPv4 address or IPv6 /64 (last `X-Forwarded-For` hop only when `SELA_TRUST_PROXY=true`). Username key = HMAC-SHA256 of the normalised username under a server-held secret derived from the KEK (of the trimmed lower-cased input when it is not a valid username), applied whether or not the account exists; only keys of valid usernames are persisted (the login backoff, ADR-0002 §9). Every refusal: `429 rate_limited` with `Retry-After: <seconds>`.

| Route group | Per client key | Per username | Other |
| --- | --- | --- | --- |
| `POST /auth/login` | 20/min, burst 10 | 10/hour, burst 5 | Backoff after 5 consecutive failures of the username (known or not; persisted, so identical for both across restarts; the count starts again after 1 hour without a failure): wait `min(2^(n−5) s, 15 min)`, also for attempts sent at the same time (each is counted before its password is checked); after 100 consecutive failed attempts of an account password sign-in is disabled until recovery, and a right password checked meanwhile is refused (`403 password_login_disabled`) |
| `POST /auth/register`, `POST /auth/guest/upgrade` | 20/hour, burst 5 | — | — |
| `POST /auth/guest` | 30/hour, burst 10 | — | At most `SELA_MAX_GUEST_ACCOUNTS` live guests, also under concurrent starts, else `503 server_busy` |
| `POST /auth/recovery` | 10/hour, burst 3 | 5/hour, burst 3 | — |
| `POST /auth/reauth`, `POST /auth/password`, `POST /auth/recovery-codes` | 10/hour per session | — | A wrong current password counts towards the login backoff |
| `GET /me/export` | 6/hour per user | — | — |
| Other mutations / reads | 180/min / 600/min (v2 buckets) | — | — |

The username and client buckets of these routes are kept apart from the general read and mutation buckets and are never forgotten before they have refilled; when that store is full, a key it has not seen yet gets `429` with the time until a bucket refills.

Password hashing is bounded by a concurrency semaphore; a request that waits longer than 5 s gets `503 server_busy` with `Retry-After: 5`.

## 2. Error codes

General codes are shared with v2 and keep their v2 messages: `invalid_input` 400 · `not_found` 404 · `method_not_allowed` 405 · `stale_version` 409 · `invalid_transition` 409 · `policy_outdated` 409 · `too_large` 413 · `unsupported_media` 415 · `cross_origin` 403 · `rate_limited` 429 · `internal` 500.

New v3 codes (messages to add to `internal/httpapi/errors.go`; every code needs both languages):

| Code | Status | When | Indonesian | English |
| --- | --- | --- | --- | --- |
| `not_signed_in` | 401 | No, unknown, expired or revoked session on a route that needs one; the cookie is cleared | Kamu belum masuk atau sesimu sudah berakhir. Silakan masuk lagi. | You are not signed in, or your session has ended. Please sign in again. |
| `invalid_credentials` | 401 | Login with an unknown username or a wrong password (identical for both) | Nama pengguna atau kata sandi tidak cocok. | The username or password does not match. |
| `recovery_failed` | 401 | Recovery with an unknown username, a wrong code or a used code (identical) | Nama pengguna atau kode pemulihan tidak cocok, atau kode itu sudah pernah dipakai. | The username or recovery code does not match, or that code was already used. |
| `current_password_incorrect` | 403 | Reauth, password change or code regeneration with a wrong current password (session stays valid), or a code regeneration whose password was replaced while it was checked | Kata sandi saat ini tidak cocok. | The current password does not match. |
| `reauth_required` | 403 | Export or deletion without a recent password proof | Untuk melindungi datamu, masukkan kata sandimu sekali lagi. | To protect your data, enter your password once more. |
| `account_required` | 403 | A guest calls a password route | Langkah ini memerlukan akun. Simpan data tamumu ke akun terlebih dahulu. | This step needs an account. Save your guest data to an account first. |
| `password_login_disabled` | 403 | 100 consecutive failed sign-ins; recovery is the way back | Masuk dengan kata sandi dinonaktifkan sementara karena terlalu banyak percobaan. Pakai kode pemulihan untuk membuat kata sandi baru. | Password sign-in is disabled after too many attempts. Use a recovery code to set a new password. |
| `age_not_supported` | 403 | Age band `under_15` or the explicit-15 band `under_16` (`field: "ageBand"`); nothing is stored | Sela saat ini dibuat untuk usia 16 tahun ke atas. Jika kamu butuh bantuan sekarang, lihat informasi bantuan. | Sela is currently made for people aged 16 and over. If you need help now, see the help information. |
| `age_band_not_enabled` | 403 | Band not in `SELA_ALLOWED_AGE_BANDS` (production: only `18_plus` until legal review) | Sela belum tersedia untuk kelompok usia ini. | Sela is not yet available for this age group. |
| `not_guest` | 409 | Guest upgrade from an account session | Akun ini sudah terdaftar. | This account is already registered. |
| `required_consent` | 409 | Withdrawing a consent the account cannot exist without | Persetujuan ini diperlukan untuk memakai Sela. Jika tidak lagi setuju, kamu bisa mengekspor lalu menghapus akunmu. | Sela cannot be used without this consent. If you no longer agree, you can export and then delete your account. |
| `consent_required` | 403 | A feature purpose is off (`personal_memory`, `sharing`), or a `15_17` owner has no active guardian consent on a gated content write (`field: "guardianConsent"`) | Langkah ini memerlukan izinmu terlebih dahulu. | This step needs your permission first. |
| `username_invalid` | 400 | Not 3–32 characters of `a-z 0-9 . _ -` starting with a letter or digit, non-ASCII, reserved, or looks like an email, phone or ID number (`field: "username"`) | Nama pengguna harus 3–32 karakter: huruf a–z, angka, titik, garis bawah, atau tanda hubung. Jangan memakai nama asli, email, atau nomor telepon. | Usernames are 3–32 characters: letters a–z, digits, dot, underscore or hyphen. Do not use your real name, email or phone number. |
| `username_taken` | 409 | Username already registered (`field: "username"`) | Nama pengguna ini sudah dipakai. Pilih yang lain. | This username is taken. Choose another. |
| `password_too_short` | 400 | Fewer than 15 characters after normalisation (`field` names the password field) | Kata sandi minimal 15 karakter. Coba gabungan beberapa kata yang mudah kamu ingat. | Passwords need at least 15 characters. Try a few words you can remember. |
| `password_too_long` | 400 | More than 256 characters or 1024 bytes | Kata sandi maksimal 256 karakter. | Passwords can have at most 256 characters. |
| `password_weak` | 400 | On the blocklist, or equal to/containing the username | Kata sandi ini terlalu umum atau mudah ditebak. Pilih yang lain. | This password is too common or easy to guess. Choose another. |
| `store_unavailable` | 503 | Database not configured (development; `Retry-After: 30`) or unreachable at run time: refused or lost connection, network error or timeout, closed pool, server shutting down or out of connections (`Retry-After: 5`); on every store-backed route, sign-in and session lookup included | Penyimpanan sedang tidak tersedia. Coba lagi sebentar lagi. | Storage is unavailable right now. Try again shortly. |
| `server_busy` | 503 | Hashing slots or guest capacity exhausted (`Retry-After`) | Server sedang sibuk. Coba lagi sebentar lagi. | The server is busy. Try again shortly. |
| `invalid_share` | 401 | Unknown link, wrong access code, ended or expired share (deliberately indistinguishable) | Tautan atau kode tidak cocok, atau sudah tidak berlaku. | The link or code doesn't match, or it is no longer valid. |
| `name_taken` | 409 | A private name already on the owner's list (case-insensitive, `field: "name"`) | Nama ini sudah ada di daftarmu. | This name is already on your list. |
| `name_limit_reached` | 409 | A 21st private name (`field: "name"`); the list holds at most 20 | Daftar nama berisi paling banyak 20 nama. Hapus satu dulu untuk menambah yang lain. | The name list holds at most 20 names. Remove one before adding another. |

**Guardian gate (D-031).** With `SELA_GUARDIAN_CONSENT_REQUIRED=true` (the default), a signed-in `15_17` owner whose guardian consent is missing or withdrawn is refused on content writes before the handler reads the body or changes anything: `403 consent_required`, `field: "guardianConsent"`. The server checks the database on every gated write, never a UI flag. Gated routes: record create/update/delete; memory create/update/delete; consultation-summary create/update/delete/approve and follow-up create; share create/revoke; conversation create/turn/resume/storage; and reflection create/review. Reads, export, account deletion, consents, guardian-consent recording/withdrawal, session actions and recipient routes are never gated. `18_plus` and `unknown` owners are unchanged. An explicit 15 (`under_16`) is refused with `age_not_supported` at registration, guest start and profile update and is never stored.

## 3. Shared objects

**Profile** (`user`):

```json
{
  "id": "6f1c0e8a-3b5d-4c47-9a7e-2f9d1b0c4e11",
  "kind": "account",
  "username": "pelangi.senja",
  "locale": "id",
  "ageBand": "18_plus",
  "ageBandConfirmedAt": "2026-10-26T13:05:00Z",
  "version": 3,
  "createdAt": "2026-10-20T08:00:00Z",
  "lastActiveAt": "2026-10-26T13:05:00Z",
  "guest": null,
  "recovery": { "remaining": 9, "generatedAt": "2026-10-20T08:00:00Z", "acknowledged": true },
  "guardianConsent": null,
  "actionsRequired": [],
  "environment": "production",
  "synthetic": false
}
```

- `kind`: `guest | account`. `username` and `recovery` are `null` for guests.
- `ageBand`: `15_17 | 18_plus | unknown` (self-declared and stored). `under_15` and the explicit-15 band `under_16` are accepted as input only, refused with `age_not_supported`, and never stored; `15_17` remains the stored band for existing 16–17 records.
- `guest` (guests only): `{"inactivePurgeAt", "maxAgePurgeAt", "inactiveTtlHours", "maxAgeDays", "storage": {"location": "server_database", "region": "<configured label or null>", "encrypted": true, "boundTo": "this_browser_cookie"}}`.
- `guardianConsent` (`ageBand: "15_17"` only): `{"required": true, "status": "missing" | "active" | "withdrawn"}`.
- `actionsRequired`: zero or more of `accept_policy:<purpose>` (a required consent is on an older policy version), `acknowledge_recovery_codes`, `confirm_age_band` (a `15_17` band older than one year), `guardian_consent` (required and not active). Privacy actions (consents, export, deletion, sessions, sign-out) are never blocked by these.
- `environment` and `synthetic`: the demo answers `"demo"` and `true`, and the UI shows the synthetic-data banner.

**Session** (`session`):

```json
{
  "id": "0b8f…",
  "current": true,
  "kind": "account",
  "trustedDevice": false,
  "device": { "browser": "firefox", "os": "android" },
  "createdAt": "2026-10-26T13:05:00Z",
  "lastSeenAt": "2026-10-26T13:00:00Z",
  "idleExpiresAt": "2026-10-26T15:05:00Z",
  "absoluteExpiresAt": "2026-10-27T01:05:00Z"
}
```

`lastSeenAt` is rounded down to the hour in lists. `device` values are the closed enums of ADR-0002 §3; the raw User-Agent and IP are never stored or returned.

**Consent** (current decision per purpose):

```json
{ "purpose": "terms_privacy", "required": true, "granted": true, "policyVersion": "terms-privacy-2026-11-01.2",
  "currentPolicyVersion": "terms-privacy-2026-11-01.2", "outdated": false, "decidedAt": "2026-10-26T13:05:00Z", "revision": 1 }
```

A purpose never decided appears with `granted: false`, `policyVersion: null`, `decidedAt: null`, `revision: 0`.

**Consent event:** `{"id", "purpose", "decision": "granted" | "withdrawn", "policyVersion", "source": "onboarding" | "settings" | "upgrade" | "system", "decidedAt"}`.

**Guardian consent record:** `{"id", "relationship": "parent" | "legal_guardian", "method": "guardian_present_attestation", "guardianName": "<string or null>", "policyVersion", "grantedAt", "withdrawnAt": null, "withdrawnBy": null | "user" | "operator"}`.

**Audit event:** `{"id", "at", "type", "actor": "user" | "system" | "recipient", "currentSession": true | false | null, "detail": {…}}`. `detail` holds only allow-listed, content-free keys: `kind, ageBand, from, to, trustedDevice, count, reason, remaining, purpose, policyVersion, relationship, method, format, revoked`.

Audit event types written by the foundation: `account.created`, `account.upgraded`, `auth.login_succeeded`, `auth.login_failed` (coalesced, one per hour with `count`), `auth.reauthenticated`, `auth.logout`, `auth.logout_all`, `session.revoked`, `password.changed`, `password_login.disabled`, `recovery.used`, `recovery.regenerated`, `recovery.acknowledged`, `profile.locale_changed`, `profile.age_band_changed`, `profile.age_band_confirmed`, `consent.granted`, `consent.withdrawn`, `guardian_consent.recorded`, `guardian_consent.withdrawn`, `export.created`. (Deletion leaves no event: the account and its audit trail are gone.)

### Consent purposes

The registry lives in code with a policy text per purpose (served by `GET /policies`); a later milestone adds a purpose by code and policy text, without a migration. The foundation activates:

| Purpose | Required | Applies to | Meaning |
| --- | --- | --- | --- |
| `terms_privacy` | Yes (cannot be withdrawn; `409 required_consent`) | guests and accounts | Service terms and privacy notice |
| `guest_storage` | Yes for guests (moot after upgrade, kept in history) | guests | Where and how long guest data is kept, cookie dependency, no absolute anonymity |

Reserved names for later milestones (not listed by `GET /me/consents` until active): `ai_processing` (Meta model on Modal, P3/P5), `ai_provider_anthropic` (Claude verifier, P5), `ai_provider_deepseek` (DeepSeek fallback, P5), `personal_memory` (now active for explicit manual memory and confirmed reply context; see continuity increment in §6).

## 4. Public routes (no session)

| Method and route | Result |
| --- | --- |
| `GET /api/v2/health` | **Unchanged** (path and fields). |
| `GET /api/v2/ready` | **Unchanged path and existing fields**, plus additive fields `database` (`"ok"`, `"unavailable"` or `"not_configured"`) and `schema` (`"current"`, `"pending"`, `"mismatch"` or `"not_configured"`); `store` keeps describing the v2 journey store (`"memory"` or `"disabled"`). Answers `503` with `status: "not_ready"` only when the environment requires the database and it is unreachable or not current. |
| `GET /api/v3/policies` | Current versioned policy texts: `{"policies": {"terms_privacy": {"version", "title": {"id","en"}, "body": [{"id","en"}, …]}, "guest_storage": …, "guardian_information": …}}`. Read bucket. |
| `GET /api/v3/help-content` | Public reviewed content ported from v1: `{"selfHelpCards": [{"id","topics","title","summary","steps","why","seekHelpWhen","cautions","source","evidenceNote"}], "helpDestinations": [{"id","label","description","speakingTo","vocative","ask","checklist","sources","caveat","verifiedOn"}], "supportOptions": [{"id","category","name","summary","phone","whatsapp","website","hours","cost","limits","source","lastVerified"}]}`. Read bucket. The model may only choose card ids; card text never comes from a model. |
| `GET /api/v3/reading` | Public reading library: `{"items": [{"id","version","kind","topics","audience","title":{"id","en"},"summary":{"id","en"},"readingTimeMinutes":{"id","en"},"sources":[…],"reviewedOn"}]}`. Only items that pass the runtime editorial gate appear: the embedded review is human-approved with all evidence, **or** a current operator approval (`0600_reading_reviews`) binds this exact item's SHA-256 content hash and records `verifier_status = supported`. A changed text makes the approval stale and the item absent. No user data. Read bucket. |
| `GET /api/v3/reading/{slug}` | One published reading item: the list fields plus `steps`, `why`, `seekHelpWhen`, `cautions`, `sections`, `source` (`relation`, `locator`, `sources`), `licence`, `attribution`, `adaptationNotice`, `adaptationNote`, `evidenceNote` and `review` (`status`, `reviewedOn`, `verifier`). Unknown, unpublished, stale-approval, revoked or forged slugs answer `404 not_found`, deliberately indistinguishable. No reviewer name or user data. Read bucket. |
| `GET /api/v3/auth/options` | Rules the UI must show before asking: `{"username": {"minLength": 3, "maxLength": 32}, "password": {"minLength": 15, "maxLength": 256}, "recoveryCodes": {"count": 10, "groups": 4, "groupLength": 4}, "ageBands": {"allowed": ["18_plus"]}, "guest": {"available": true, "inactiveTtlHours": 168, "maxAgeDays": 30}, "sessions": {"idleHours": 2, "absoluteHours": 12, "trustedIdleDays": 7, "trustedAbsoluteDays": 30}, "guardianConsentRequired": true, "environment": "production", "synthetic": false}`. Read bucket. |

Without a configured database (development only), every `/api/v3` route except the public routes in this section answers `503 store_unavailable`; so does every other route, with `Retry-After: 5`, while the configured database cannot be reached.

## 5. Authentication

### `POST /api/v3/auth/register`

```json
{ "username": "pelangi.senja", "password": "tiga kata panjang untuk diingat", "locale": "id",
  "ageBand": "18_plus", "termsPolicyVersion": "terms-privacy-2026-11-01.2", "rememberDevice": false }
```

**201** sets the cookie:

```json
{ "data": { "user": { "…": "Profile, kind account" }, "session": { "…": "Session" },
  "recoveryCodes": ["7K2M-QX9D-4HTR-PZ3B", "… 10 codes in total"] } }
```

Rules: username normalised and validated (ADR-0002 §6); password NFKC-normalised, 15–256 characters, blocklist; age band allowed; `termsPolicyVersion` equal to the current `terms_privacy` version (else `409 policy_outdated`). Creates user, credentials, DEK, 10 recovery codes, the `terms_privacy` consent (`source: onboarding`), a session, and `account.created`, in one transaction. Any session cookie already present is revoked first. The codes are returned **only in this response**; `recovery.acknowledged` stays `false` until `POST /me/recovery-codes/acknowledge`.
Errors: `400 invalid_input | username_invalid | password_too_short | password_too_long | password_weak`, `403 age_not_supported | age_band_not_enabled`, `409 username_taken | policy_outdated`, `429`, `503 server_busy | store_unavailable`.
Retry: if the response was lost, a retry gets `409 username_taken`; the client signs in with the same password and offers to regenerate the codes (`actionsRequired` contains `acknowledge_recovery_codes`).

### `POST /api/v3/auth/login`

`{"username", "password", "rememberDevice": false}` → **200** `{"user", "session"}`, cookie set (rotation: any existing cookie's session is revoked). A successful login resets the failure counter and re-hashes the password if its parameters differ from the current configuration. A password that a password change or a recovery replaced while it was being checked is a wrong password: `401 invalid_credentials`, recorded as a failed attempt, no session and no re-hash. Rarely, the right password gets the same answer once: when another sign-in of the account re-hashed it while it was being checked (after a parameter change; ADR-0002 §9, residual); signing in again succeeds.
Errors: `401 invalid_credentials` (identical for unknown user and wrong password, same work done), `403 password_login_disabled`, `429 rate_limited` (also for backoff), `503 server_busy | store_unavailable`.

### `POST /api/v3/auth/logout`

No body (or `{}`). **200** `{"signedOut": true}`; revokes the current session (`auth.logout`) and clears the cookie. Idempotent: without a session it still answers 200 and clears the cookie.

### `POST /api/v3/auth/logout-all`

`{"keepCurrent": false}` (body optional; default `false`). **200** `{"revoked": 3, "signedOut": true}`. Revokes every live session of the user (`logout_all`), or every other one with `keepCurrent: true` (`signedOut: false`, cookie kept). Writes `auth.logout_all` with `revoked`. Needs a session (`401 not_signed_in`). Idempotent.

### `POST /api/v3/auth/guest`

```json
{ "locale": "id", "ageBand": "15_17", "termsPolicyVersion": "terms-privacy-2026-11-01.2", "guestNoticeVersion": "guest-storage-2026-11-01.2" }
```

**201** `{"user": {…kind guest, guest {…}}, "session"}`, cookie with `Max-Age` to the guest maximum age. Records `terms_privacy` and `guest_storage` consents. The UI must have shown the guest storage notice (location, 7-day inactivity deletion, 30-day maximum, cookie dependency, no absolute anonymity) before this call.
Errors: `400 invalid_input`, `403 age_not_supported | age_band_not_enabled`, `409 policy_outdated`, `429`, `503 server_busy | store_unavailable`.
Retry: a lost response leaves an unused guest that the purge job deletes; nothing to clean up.

### `POST /api/v3/auth/guest/upgrade`

`{"username", "password", "rememberDevice": false}` from a guest session → **200** `{"user": {…kind account}, "session", "recoveryCodes": [10]}`. Same `user.id`, so every owned row stays; credentials and codes created; `guest_expires_at` cleared; session rotated; `account.upgraded`; one transaction. If `terms_privacy` is outdated → `409 policy_outdated` (accept the new version first with `PUT /me/consents/terms_privacy`).
Errors: `401 not_signed_in`, `409 not_guest | username_taken | policy_outdated`, the username and password errors, `429`, `503 server_busy | store_unavailable`.
Retry: a lost response means the account exists and the guest cookie was revoked by rotation; the client signs in and regenerates codes.

### `POST /api/v3/auth/recovery`

```json
{ "username": "pelangi.senja", "recoveryCode": "7k2m qx9d 4htr pz3b", "newPassword": "…", "rememberDevice": false }
```

The new password is validated first (`400 password_*`, no code consumed). On success: the code is marked used (single use), the password replaced, sign-in re-enabled and the failure counter cleared, **every session revoked**, a new session issued (cookie), `recovery.used`. **200** `{"user", "session", "remainingRecoveryCodes": 9}`. With 3 or fewer codes left, `actionsRequired` suggests regeneration.
Errors: `401 recovery_failed` (identical for unknown user, wrong code, used code), `400 password_*`, `429`, `503 server_busy | store_unavailable`.
Retry: a lost response means the password already changed; sign in with the new password.

### `POST /api/v3/auth/reauth`

`{"password"}` (accounts) → **200** `{"reauthenticatedUntil": "2026-10-26T13:15:00Z"}`; `auth.reauthenticated`. Errors: `401 not_signed_in`, `403 account_required | current_password_incorrect | password_login_disabled`, `429`, `503 server_busy | store_unavailable`.

### `POST /api/v3/auth/password`

`{"currentPassword", "newPassword"}` → **200** `{"session", "revokedSessions": 2}`; the current session is rotated (new cookie) and every other session revoked (`password_changed`); `password.changed`. Errors: `401 not_signed_in`, `403 account_required | current_password_incorrect | password_login_disabled`, `400 password_*`, `429`, `503 server_busy | store_unavailable`. Retry: a lost response means the new password is active.

### `POST /api/v3/auth/recovery-codes`

`{"password"}` → **200** `{"recoveryCodes": [10], "generatedAt"}`; every previous code is invalid; `recovery.regenerated`; acknowledgement reset to `false`. A password that a recovery or a password change (from another session) replaced while it was being checked gets `403 current_password_incorrect`, recorded as a failed attempt, and the codes stay as they are; so does, once, the right password when a sign-in of the account re-hashed it meanwhile (ADR-0002 §9, residual; sending it again succeeds). Errors: `401 not_signed_in`, `403 account_required | current_password_incorrect | password_login_disabled`, `429`, `503 server_busy | store_unavailable`. Retry: regenerate again (the earlier batch is already invalid).

## 6. The signed-in user (`/me`)

All routes need a session (`401 not_signed_in` otherwise).

| Method and route | Body | Result and rules |
| --- | --- | --- |
| `GET /api/v3/me` | — | **200** Profile |
| `PATCH /api/v3/me` | `{"expectedVersion", "locale"?: "id"\|"en", "ageBand"?: "under_15"\|"under_16"\|"15_17"\|"18_plus"\|"unknown"}` (at least one field) | **200** Profile (version + 1). Locale changes the UI and future messages only. Sending the current `ageBand` again confirms it (`profile.age_band_confirmed`). `under_15` or `under_16` → `403 age_not_supported`, nothing changes (the UI offers export and deletion). A band outside the allowed list → `403 age_band_not_enabled`. Changes write `profile.locale_changed` / `profile.age_band_changed` with `from`/`to`. Errors also `409 stale_version`, `400 invalid_input` |
| `POST /api/v3/me/recovery-codes/acknowledge` | `{}` | **200** Profile with `recovery.acknowledged: true`; `recovery.acknowledged`. Idempotent. Guests → `403 account_required` |
| `GET /api/v3/me/sessions` | — | **200** `{"sessions": [Session…]}` (live sessions only, current first) |
| `DELETE /api/v3/me/sessions/{id}` | — | **200** `{"revoked": true, "current": false}`; `session.revoked` (`reason: revoked_by_user`). Revoking the current session also clears the cookie (`current: true`). Another user's or an unknown id → `404 not_found`; an own, already ended session → 200 (idempotent) |
| `GET /api/v3/me/consents` | — | **200** `{"consents": [Consent…]}` for every active purpose that applies to this user |
| `PUT /api/v3/me/consents/{purpose}` | `{"granted": true\|false, "policyVersion"}` | **200** `{"consent": Consent, "changed": true\|false}`. Granting needs `policyVersion` = current (else `409 policy_outdated`). Withdrawing is never blocked by a version mismatch, and the version sent with it is not stored: the withdrawal records the version of the user's last decision for that purpose, or the current version when there was none. `policyVersion` must match `^[a-z0-9][a-z0-9._-]{0,63}$` (else `400 invalid_input`, field `policyVersion`). Withdrawing a required purpose → `409 required_consent`. The same decision on the same version → `changed: false`, no new history event. Unknown or inactive purpose → `404 not_found`. Each change appends a consent event and `consent.granted`/`consent.withdrawn`. Withdrawal stops future processing for that purpose; data already processed is handled by the owning feature's rules |
| `GET /api/v3/me/consents/history?purpose=&limit=&cursor=` | — | **200** `{"events": [Consent event…], "nextCursor"}`; `purpose` optional filter |
| `GET /api/v3/me/guardian-consent` | — | **200** `{"required", "status", "active": record\|null, "history": [record…]}`. For a user whose band is not `15_17`: `required: false`, `status: "not_applicable"`, history still returned |
| `PUT /api/v3/me/guardian-consent` | `{"relationship": "parent"\|"legal_guardian", "method": "guardian_present_attestation", "guardianName"?: "≤80 characters", "policyVersion"}` | **200** `{"active": record}`. Only for `ageBand: "15_17"` (else `409 invalid_transition`). `policyVersion` must equal the current `guardian_information` version (`409 policy_outdated`). An active record is withdrawn (`withdrawnBy: user`) and replaced in the same transaction. `guardianName` is optional, encrypted at rest, refused if it looks like contact details (`400 invalid_input`, `field: "guardianName"`). `guardian_consent.recorded`. **The record grants the guardian no access of any kind** |
| `DELETE /api/v3/me/guardian-consent` | — | **200** `{"active": null}`; withdraws the active record; `guardian_consent.withdrawn`. Idempotent |
| `GET /api/v3/me/audit?limit=&cursor=` | — | **200** `{"events": [Audit event…], "nextCursor"}`; own events only |
| `GET /api/v3/me/export?format=json\|html\|md&lang=id\|en` | — | **200** file download (below). Accounts need a recent reauth (`403 reauth_required`). `400 invalid_input` (`field: "format"`), `429`. Writes `export.created` (`format`) |
| `DELETE /api/v3/me` | `{"confirm": true}` | **200** `{"deleted": true, "keysUnrecoverableBy": "<now + 7 days>", "backupsExpireBy": "<now + 30 days>"}`; cookie cleared. Accounts need a recent reauth (`403 reauth_required`); guests do not. A password change or recovery in progress for the account is waited for; it revokes the session, so the deletion then answers `401 not_signed_in` and deletes nothing. `confirm` missing or false → `400 invalid_input` (`field: "confirm"`). Retry after success → `401 not_signed_in`, which the client treats as done |

### Export format

Headers: `Content-Disposition: attachment; filename="sela-export-YYYYMMDD.<json|html|md>"`, `Content-Type: application/json; charset=utf-8` / `text/html; charset=utf-8` / `text/markdown; charset=utf-8`, `Cache-Control: no-store`; the HTML response carries `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; img-src data:; sandbox`. All three formats come from one export model, assembled by the exporter registry (ADR-0002 §10), decrypted for the owner.

```json
{
  "format": "sela-export/1",
  "generatedAt": "2026-10-26T13:20:00Z",
  "environment": "production",
  "synthetic": false,
  "user": { "…": "Profile" },
  "sections": {
    "profile": { "…": "Profile fields" },
    "account": { "username": "pelangi.senja", "passwordChangedAt": "…", "recoveryCodes": { "remaining": 9, "generatedAt": "…" } },
    "consents": [ "Consent…" ],
    "consentHistory": [ "Consent event…" ],
    "guardianConsents": [ "Guardian consent record…" ],
    "privateNames": [ { "id": "…", "name": "Raka", "createdAt": "…" } ],
    "sessions": [ "Session… (live and ended within retention)" ],
    "audit": [ "Audit event…" ]
  },
  "notices": {
    "id": ["Pencabutan akses menghentikan akses berikutnya; salinan yang sudah diunduh atau dilihat orang lain tidak dapat ditarik kembali.", "…"],
    "en": ["Revoking access stops future access; copies already downloaded or viewed by others cannot be recalled.", "…"]
  }
}
```

Password hashes, recovery-code hashes, session token hashes and keys are never exported. Each later feature adds its own section key (for example `journal`, `checkIns`, `memory`, `consultationSummaries`, `shares`) through the registry, in all three formats.

## 6.9 Personal records and listening room (2026-10-27 local implementation)

The default frontend now uses v3. The v2 synthetic journey is retained at `/demo`. Personal-record content and every revision use per-user envelope encryption; one shared `personal_records` table (migration 0200) holds note, check-in and conversation snapshots. This replaces the earlier proposed separate journal/check-in prefixes for this increment. Conversation runtime state is session-scoped memory; saved snapshots participate in the owner export and deletion cascade.

All routes below require the owner cookie and normal same-origin mutation guards. Foreign ids return 404. Request bodies stay under 16 KiB. No user id is accepted from the browser.

| Route | Body / response |
| --- | --- |
| `GET /api/v3/records` | `{records: PersonalRecord[]}` newest first; owned decrypted records |
| `POST /api/v3/records` | `{kind:"note" or "checkin", title?, text, mood?, energy?, tags?, day?, storageGranted:true}`; 201. A note needs text; text max 6000 characters, title max 120. A check-in mood is one of `very_heavy, heavy, okay, good, very_good, mixed`; `energy` is `low, medium, high`; `tags` are up to ten unique values from `sleep, school, work, family, friends, partner, money, health, loneliness, other`. Day is local `YYYY-MM-DD`; at most one check-in per owner/day; duplicate create is 409 stale_version. Fields are optional for a check-in |
| `GET /api/v3/records/{id}` | One owned record |
| `PATCH /api/v3/records/{id}` | Same body plus `expectedVersion`; kind/day immutable; keeps encrypted previous versions |
| `DELETE /api/v3/records/{id}?derived=keep\|delete` | `{expectedVersion}`; removes the record and every revision. `keep` (default) leaves derived summaries in place to become stale on read; `delete` also removes every owned summary that pins this source, with its follow-ups and shares, in the same transaction |
| `GET /api/v3/records/{id}/versions` | `{versions: PersonalRecord[]}` oldest first |
| `GET /api/v3/derived?source={id}` | `{sourceId,summaries:[{id,title,version,approved,shareCount}],totals:{summaries,shares}}`; content-free owner impact of deleting the source. `approved` is the effective read-time state and `shareCount` counts every share row for that summary. `404 not_found` for an unknown, malformed or foreign source |
| `GET /api/v3/conversations` | Active rooms for this owner and this exact session |
| `POST /api/v3/conversations` | `{save:boolean,recordId?:string}`; storage choice mandatory; optionally resume an owned saved conversation; at most 5 live rooms per session and 500 globally |
| `GET /api/v3/conversations/{id}` | `{id,save,version,turns,paused,busy,aiState,recordId?,holdExpiresIn}`; `holdExpiresIn` is seconds until the temporary hold is discarded (30 idle minutes, capped at 2 hours from creation) |
| `POST /api/v3/conversations/{id}/turns` | `{text,expectedVersion,intent?}`; 1–3000 characters, at most 64 turns. Stores the user's words only when saving was chosen. Contextual Meta-primary reply through `internal/ai`; current per-provider consent required. Eight AI operations per owner per 10 minutes, admission-limited; request budget 90 seconds. Response includes only released, guarded output with provider/model provenance; unavailable AI never invents a reply |
| `POST /api/v3/conversations/{id}/turns/stream` | Same body and pipeline as `POST /turns`, returned as `text/event-stream; charset=utf-8`. Frames in order: `event: status` with `{"state":"thinking"}` (or the terminal no-AI state when the router never runs), zero or more `event: chunk` with `{"text":"…"}`, then `event: done` whose data is the same room object as `POST /turns`. Chunks contain only text the router already released and the room already stored; the server does not forward provider tokens. Pre-stream refusals keep the normal JSON error envelope (`invalid_input` 400, `not_found` 404, `stale_version`/`limit_reached` 409); a failure after the stream opens uses `event: error` with a stable `code` (`not_found`, `internal`). This is post-validation chunking, not token streaming |
| `POST /api/v3/conversations/{id}/reflections` | `{expectedVersion}` → room with `reflections` (AT-2 cards; requires `ai_processing`) |
| `PATCH /api/v3/conversations/{id}/reflections/{reflectionId}` | `{expectedVersion, action: confirm\|correct\|reject\|undo, text?}` → room |
| `PATCH /api/v3/conversations/{id}/storage` | `{save:boolean,expectedVersion}`; turning off deletes the saved snapshot and revisions, keeping only the live hold |
| `DELETE /api/v3/conversations/{id}/operation` | Cancel in-flight reply; retain the user's words |
| `POST /api/v3/conversations/{id}/resume` | `{expectedVersion}`; explicit continuation after inline crisis support; does not call a provider |
| `DELETE /api/v3/conversations/{id}` | End; purge the hold and cancel inference. A deliberately saved snapshot remains in owned records |

`storage_required` is 403. Unsaved turns never enter PostgreSQL or exports. Holds expire after 30 inactive minutes or 2 hours total; sign-out purges them. Every user-authored entry is screened; crisis text is kept, inline help shown, and room AI paused. Later turns do not erase the user's choice to continue. Provider consent changes invalidate running results. `ai_processing`, `ai_provider_anthropic` and `ai_provider_deepseek` are independent versioned consent purposes; DeepSeek remains default-off and synthetic-only. When a durable monthly AI budget is configured, admission happens before any provider call; an exhausted or unavailable budget returns `aiState: "budget_limited"` (or `"unavailable"`) and preserves the user's text without a provider call. A listening turn may include at most five confirmed/corrected memory items (600 runes together) as `memory` spans only when `personal_memory` and `ai_processing` are both current; otherwise no memory is sent. These routes and screens are local development work, not pilot or public-release approval.

### Manual continuity increment (2026-10-28)

Migration 0300 adds encrypted `memory`, `summary`, and `followup` record kinds.
No AI generation, automatic extraction, or recipient sharing is enabled by this increment.
The existing cookie principal, mutation guards, ownership, export, revisions, and deletion rules apply.

| Route | Request / response |
| --- | --- |
| `GET /api/v3/memory` | `{enabled,items}`; disabled and empty until current `personal_memory` consent |
| `GET /api/v3/memory/{id}` | One owned memory record with its `reviewStatus`, `provenance` and source refs (including the proposal `quote` and `turnKey`); `404 not_found` without current `personal_memory` consent or for another owner |
| `POST /api/v3/memory` | `{title,text,storageGranted:true,sourceRefs?:[]}` → personal record |
| `POST /api/v3/memory/proposals` | `{conversationId,expectedVersion?}` → `{proposals,aiState}`. Runs AT-3 over the recent user turns of one **saved** conversation record (never a live unsaved hold) and requires current `personal_memory` and `ai_processing` consent. Every released AI wording becomes an owned memory record with `reviewStatus: "unreviewed"`, `provenance: "unconfirmed_interpretation"` and a source ref carrying the user's exact `quote` and the turn key. `aiState` is `released`, `no_input` (no sendable user turn) or `unavailable` (the router withheld or could not release anything); a withheld, dropped or quote-fallback item is never stored. `409 stale_version` when `expectedVersion` is given and the conversation record changed, and `409 invalid_transition` while the saved conversation is crisis-paused |
| `PATCH /api/v3/memory/{id}` | Same, plus `expectedVersion` |
| `PATCH /api/v3/memory/{id}/review` | `{expectedVersion,action:"confirm"\|"correct"\|"reject",text?}` → reviewed memory record. `confirm` keeps the AI wording and sets `confirmed`; `correct` replaces the text in a new revision with `provenance: "user_correction"` and status `corrected`; `reject` sets `rejected`. Owner-scoped and versioned: a stale `expectedVersion` is `409 stale_version`, a foreign or non-memory id is `404 not_found`, and a missing current `personal_memory` consent is `403 consent_required`. Every review action appends a revision; confirm and reject keep the text, a correction replaces it. A rejected item can be confirmed or corrected again |
| `DELETE /api/v3/memory/{id}` | `{expectedVersion}`; also allowed after consent withdrawal |
| `GET /api/v3/shares` | `{shares}` owner metadata, newest first: `openedCount`, `acknowledgedCount`, `lastOpenedAt`, `acknowledged` derived from events; no token, code, hash or content |
| `GET /api/v3/shares/{id}/events` | `{events:[{kind,at}]}` newest first; owner only; `404 not_found` for another owner, an unknown or a malformed id; no token, code, content, recipient identity or IP address |
| `POST /api/v3/shares` | `{summaryId,label,purpose,durationHours}` → `{share,token,code}` once; requires `sharing` |
| `POST /api/v3/shares/{id}/revoke` | `{expectedVersion}` → ended share; every recipient session ends |
| `POST /api/v3/recipient/session` | `{token,code}` → `{share}` and the recipient cookie |
| `GET /api/v3/recipient/share` | frozen projection for the recipient session |
| `POST /api/v3/recipient/acknowledge` | `{acknowledged:true}`; idempotent |
| `GET /api/v3/consultation-summaries` | `{summaries}` |
| `POST /api/v3/consultation-summaries` | `{title,text?,sourceRefs,items?,prepared?,storageGranted:true}` → personal record; at least one of `text` or `items` is required |
| `GET /api/v3/consultation-summaries/{id}` | Personal record with effective `summaryState` and optional `items` |
| `GET /api/v3/consultation-summaries/{id}/export?format=html\|md` | Approved summary as a self-contained HTML or Markdown file; `409 invalid_transition` until approved; includes `items` with their quotes and source labels |
| `PATCH /api/v3/consultation-summaries/{id}` | Creation fields plus `expectedVersion`; `items: null` (or omitted) keeps the stored statements, `items: []` clears them, and any edit resets approval |
| `DELETE /api/v3/consultation-summaries/{id}` | `{expectedVersion}`; summary and attached follow-ups cascade |
| `POST /api/v3/consultation-summaries/{id}/approve` | `{expectedVersion}`; approval binds exact content version |
| `POST /api/v3/consultation-summaries/{id}/followups` | `{title,text?,sections?,storageGranted:true}` → personal follow-up record; at least one of `text` or a non-blank section is required |
| `GET /api/v3/consultation-summaries/{id}/followups` | `{followups}`; each record may carry `sections` |
| `PATCH /api/v3/consultation-summaries/{id}/followups/{followupId}` | Same fields plus `expectedVersion`; the follow-up must belong to this summary; nil `sections` keeps the stored sections, an explicit empty object clears them |

Record deletion always checks `expectedVersion` before changing anything; a mismatch is `409 stale_version` with no side effect, and an unknown or foreign id is `404 not_found`. `derived=keep` is the default and preserves the earlier behaviour: the source disappears, the sealed `sourceRefs` still name it, and `enrichSummaryState` demotes an approved summary to draft on every read, export and share path. Under `keep`, a share built from a surviving summary keeps its frozen projection and is **not** revoked; the UI and this contract say so explicitly. `derived=delete` is explicit and irreversible: every owned summary whose sealed refs include the source, its follow-ups, and its shares, recipient sessions and share events are removed in the same transaction as the source. Another owner's rows are never touched, no new table or migration is involved, and an unknown mode is `400 invalid_input` with `field: "derived"`.

Sharing metadata and history are read-only for the owner. `openedCount`, `acknowledgedCount` and `lastOpenedAt` come from the append-only `share_events`; an idempotent re-acknowledgement adds no second event. These values describe link and access-code use, never the recipient's identity, whether care happened, or the content that was viewed. Revoked and expired shares keep their history readable by their owner.
`sourceRefs` is an array of `{recordId,version}`, maximum 12, minimum one for a summary.
An AT-3 memory proposal adds the optional `quote` and `turnKey` fields, sealed with the rest of the record, so its review card can show the user's exact words without opening the conversation; existing records without them read unchanged.
Sources must be current, owner-owned notes/check-ins/conversations. Derived records cannot be sources.
A source edit or deletion demotes the effective approval on subsequent reads and exports.
`prepared` holds optional `goal`, `questions`, and `bring` text. All such content and source links are encrypted.
A summary's optional `items` array holds at most 36 quote-backed statements. Each item is
`{id,text,quote,sourceRef:{recordId,version},provenance,reviewStatus,position}`: `text` is
1–800 runes of the owner's wording, `quote` is 0–1200 runes of the exact source passage,
and `sourceRef` must name one of the summary's own `sourceRefs` at the same version. A new
item omits `id`; the server assigns one once, and a later edit returns that id to keep it
stable. The server derives `position` from array order and sets `provenance` to
`user_original` or, when a kept item's wording, quote or source changed,
`user_correction`; `reviewStatus` is always `confirmed` because there is no AI drafting
path. An item may not claim AI provenance or an unreviewed state. Items live inside the
same sealed summary content, so they need no table or migration, inherit revisions, export
and deletion, and are part of the version that approval binds. Editing an item resets
approval exactly like a text edit. `text` may be empty when `items` is present, but at least
one of them is required. Older text-only clients keep working: omitting `items` on PATCH
preserves stored statements, and a legacy summary reads and renders as before. The HTML/
Markdown export and the frozen sharing projection include `items` with each quote and the
label of its pinned source; a legacy text-only summary contains no item section.
A follow-up's optional `sections` holds `feeling`, `remember`, `agreed` and `askNext`, each UTF-8
and at most 2000 runes. The four fields live inside the same sealed content as `text`, so they
inherit owner encryption, revisions, export and the summary-deletion cascade; they add no table
or column. `text` remains the free note and is shown when no section is filled; a legacy
text-only follow-up reads with no `sections`. The generic `PATCH /api/v3/records/{id}` now
accepts only notes and check-ins, so a follow-up is edited only through its summary route.
Memory is explicit, defaults off, and is never sent to AI while the
`personal_memory` consent is off or outdated. With that consent on, a reply
turn may include at most five confirmed/corrected memory items (600 runes
together) as `memory` spans, and only when `ai_processing` is also current;
items over the allowance are skipped deterministically. Unreviewed and
rejected items are never reply context. Withdrawal hides
memory from normal record reads, including `GET /api/v3/memory/{id}`;
owner export and deletion remain available.
AT-3 proposals come only from a saved conversation record and are stored as
unreviewed memory items; nothing from an unsaved room is used. AI proposal
wording is released only through the verifier gate (V36), and the source
quote is kept with the item. The generic records PATCH no longer accepts a
`followup`; follow-ups are edited only through their summary route.
Weekly reflection is computed in the browser from original creation dates/check-in local days;
a reflection is saved only after permission as an ordinary encrypted note.


## 6.10 Planned-care directory (2026-10-30 local implementation)

`GET /api/v3/care-directory?q=&kind=&city=` is public and returns only entries with `status = "verified"` whose `reverifyAt` is null or in the future. `kind` is `organization`, `facility` or `practitioner`; `q` searches the name, description and source publisher; `city` is an optional manually typed filter. The API never reads a browser location.

Each entry carries `{id, kind, name, city, description, contactUrl, sourceUrl, sourcePublisher, sourceAccessedAt, status, synthetic, version, verifiedAt, reverifyAt, createdAt, updatedAt}`. There is no user id, appointment, availability, referral or contact action. In development, test and demo, clearly synthetic demo entries are included and marked `synthetic: true`; staging and production return only sourced real entries. A stale or unpublished entry is not returned. The page says that Sela does not contact a provider and does not claim a partnership.

## 6.11 Reading library (2026-10-31; operator approval added 2026-11-01)

`GET /api/v3/reading` and `GET /api/v3/reading/{slug}` are public and read
the embedded content set at `data/content/reading` (the same files
`tests/content/reading_validator.py` checks). The runtime gate is applied on
every request, independently of the authoring validator. An item is published
when **either** the embedded file already carries a complete human approval
(`review.status = human_approved`, `review.publishable = true`, a non-blank
`review.human_editor`, a real `YYYY-MM-DD` `review.reviewed_on` and
`review.verifier.status = supported`) **or** a current row in the operator
table `reading_reviews` (migration `0600_reading_reviews`) exists for the slug.
That row must be unrevoked, record `verifier_status = supported`, and its
`content_hash` must equal the SHA-256 of the exact raw JSON item object in the
content set. The operator approval also supplies the public `reviewedOn` and
`verifier`, and is reported as `review.status = human_approved`; the public
response still omits the reviewer's name, note and operator id.

An approval is bound to the item bytes, not to a slug alone: any text,
version or formatting change to the item changes its hash, so the item is
absent from the list and answers `404 not_found` on detail until a human
re-approves the new text. The public handlers read the approval rows on every
request (no approval cache), so `POST /api/admin/v1/reading/{slug}/revoke` stops
publication immediately; a revoked row also vetoes an item whose embedded file
still claims approval. If the approval store cannot be read, only the
embedded-content gate applies: a database-backed approval is never served on
trust. Forged or incomplete embedded approvals stay absent, as before.

Operators use `GET /api/admin/v1/reading`, `POST /reading/{slug}/approve` and
`POST /reading/{slug}/revoke` (API_SPEC_ADMIN §2). Approval requires an
explicit `confirm: true`, a reviewer name and date, `verifierStatus:
"supported"`, and the previewed `contentHash`; the server refuses a stale hash
with `409 reading_content_changed`. It never auto-approves and does not change
the content files.

## 6.12 Private names (2026-11-01 local implementation)

The owner's "names kept private" list is an explicit privacy control: names are redacted from every provider payload before a call, but the list is not a guarantee against re-identification by other details. It does not rewrite text the owner writes or shares: a shared summary carries the words the owner approved, so a name written there stays visible to the recipient. Names are sealed with the owner's data key (`private_name/<id>/name`); the database stores only ciphertext (migration `0800_private_names`), at most 20 names of 1–120 runes each.

| Route | Body / response |
| --- | --- |
| `GET /api/v3/me/private-names` | `{names: [{id,name,createdAt}…]}` oldest first; own list only |
| `POST /api/v3/me/private-names` | `{name}` → **201** `{id,name,createdAt}`. Edges are trimmed; empty, invalid UTF-8, control characters or more than 120 runes → `400 invalid_input` (`field: "name"`); a case-insensitive duplicate → `409 name_taken` (`field: "name"`); a 21st name → `409 name_limit_reached` (`field: "name"`) |
| `DELETE /api/v3/me/private-names/{id}` | → **200** `{deleted: true}`. Another owner's id, an unknown id and a malformed id all → `404 not_found` |

Every listening reply and reflection request loads the owner's list and passes it to `ai.Input.PrivateNames`; the minimiser replaces each occurrence with `[NAMA-n]` before the payload exists. If the list cannot be read, the request is not blocked: the server logs a content-free warning and continues with the deterministic contact-detail redaction only. The settings preview is computed in the browser from the saved names and is never sent to a provider. The owner's own export includes the decrypted list (`sections.privateNames`) because it is their data; a share or recipient projection never includes it. Deleting the account removes the rows and crypto-shreds the key, so copies of the ciphertext cannot be opened.

## 7. Guest and account differences (foundation)

| Capability | Guest | Account |
| --- | --- | --- |
| Profile, locale, age band, consents, guardian consent, audit, sessions list/revoke, logout, logout-all | Yes | Yes |
| Export, delete | Yes, without reauth | Yes, with reauth |
| Reauth, change password, recovery codes | `403 account_required` | Yes |
| Upgrade | Yes | `409 not_guest` |
| Automatic deletion | 7 days inactive or 30 days after creation | No (until the user deletes) |

## 8. Status codes used

200 success · 201 created (register, guest start) · 400 validation · 401 not signed in or bad credentials/recovery · 403 guard, policy, reauth, age · 404 not found or not owned · 405 wrong method · 409 conflict or state · 413 too large · 415 not JSON · 429 rate limited or backoff · 500 internal · 503 storage unavailable or busy.

## 9. Reserved sections for later milestones

Each milestone adds its routes here, with bodies, errors, limits, exporter section and consent purposes, before implementation. Route prefixes are reserved now:

| Milestone | Prefix | Migration range |
| --- | --- | --- |
| P3 Conversation (listening room) | `/api/v3/conversations` | 0100–0199 |
| P3 Journal and check-in | `/api/v3/journal`, `/api/v3/check-ins` | 0200–0299 |
| P3 Timeline | `/api/v3/timeline` (read model over owned records) | — |
| P3 Memory | `/api/v3/memory` | 0300–0399 |
| P4 Consultation summary and sharing | `/api/v3/consultation-summaries`, `/api/v3/shares` | 0400–0499 |
| P4 Recipient (separate principal, cookie `__Secure-sela_recipient`, `Path=/api/v3/recipient`, one share each) | `/api/v3/recipient` | 0400–0499 |
| P5 AI routing records and provider consents | `/api/v3/ai` | 0500–0599 |
| P6 Reading and support content | `/api/v3/reading` | 0600–0699 |
| Planned care (directory and synthetic sandbox) | `/api/v3/care-directory` | 0700–0799 |
| Privacy controls (names kept private) | `/api/v3/me/private-names` | 0800–0899 |
