-- sela:kind schema
-- sela:requires 0001
--
-- Separate operator identity and audit domain (ADR-0005, Admin Console).
-- Staff are provisioned out of band, never through the anonymous user
-- journey. No table here references users or holds personal content.

CREATE TABLE admin_users (
  id                  uuid PRIMARY KEY,
  username            text NOT NULL UNIQUE CHECK (username ~ '^[a-z0-9][a-z0-9._-]{2,31}$'),
  password_hash       bytea NOT NULL CHECK (octet_length(password_hash) = 32),
  password_salt       bytea NOT NULL CHECK (octet_length(password_salt) = 16),
  argon2_version      smallint NOT NULL CHECK (argon2_version = 19),
  argon2_memory_kib   integer NOT NULL CHECK (argon2_memory_kib >= 19456),
  argon2_iterations   integer NOT NULL CHECK (argon2_iterations >= 2),
  argon2_parallelism  smallint NOT NULL CHECK (argon2_parallelism BETWEEN 1 AND 16),
  mfa_secret_ct       bytea NOT NULL CHECK (octet_length(mfa_secret_ct) > 0),
  last_totp_step      bigint NOT NULL DEFAULT 0 CHECK (last_totp_step >= 0),
  role                text NOT NULL CHECK (role IN ('owner','security_admin','directory_editor','directory_reviewer','research_editor','safety_reviewer','ops')),
  org_id              text NULL,
  status              text NOT NULL DEFAULT 'active' CHECK (status IN ('active','revoked')),
  failed_count        integer NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
  next_attempt_at     timestamptz NULL,
  created_at          timestamptz NOT NULL,
  last_login_at       timestamptz NULL
);

CREATE TABLE admin_sessions (
  id                   uuid PRIMARY KEY,
  admin_id             uuid NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  token_hash           bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
  created_at           timestamptz NOT NULL,
  last_seen_at         timestamptz NOT NULL,
  idle_timeout_seconds integer NOT NULL CHECK (idle_timeout_seconds > 0),
  absolute_expires_at  timestamptz NOT NULL,
  revoked_at           timestamptz NULL,
  revoke_reason        text NULL CHECK (revoke_reason IN ('logout','revoked','expired')),
  CHECK ((revoked_at IS NULL) = (revoke_reason IS NULL))
);
CREATE INDEX admin_sessions_live ON admin_sessions (admin_id) WHERE revoked_at IS NULL;

CREATE TABLE admin_audit (
  id          uuid PRIMARY KEY,
  actor_id    uuid NULL REFERENCES admin_users(id) ON DELETE SET NULL,
  action      text NOT NULL CHECK (action IN ('admin.create','admin.login','admin.login_failed','admin.logout','admin.cache_invalidate','admin.revoke')),
  object_kind text NULL,
  object_id   text NULL,
  old_status  text NULL,
  new_status  text NULL,
  reason      text NULL,
  created_at  timestamptz NOT NULL
);
CREATE INDEX admin_audit_created ON admin_audit (created_at DESC);

REVOKE ALL ON admin_users, admin_sessions, admin_audit FROM PUBLIC;
