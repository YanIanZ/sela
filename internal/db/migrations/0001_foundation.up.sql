-- sela:kind schema
--
-- Foundation schema (ADR-0002 §3): environment marker, users, credentials,
-- recovery codes, sessions, wrapped data keys, consents and their history,
-- guardian consent records and the audit trail. Conventions: ids are uuid
-- generated in Go, times are timestamptz written from the Go clock, enums
-- are text with CHECK, every table holding a user's data cascades from
-- users (single exception: user_data_keys), PUBLIC is revoked everywhere,
-- no extensions, encrypted columns are bytea named *_ct.

CREATE TABLE instance_meta (
  singleton   boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  environment text NOT NULL CHECK (environment IN ('development','test','demo','staging','production')),
  created_at  timestamptz NOT NULL
);

CREATE TABLE users (
  id                    uuid PRIMARY KEY,
  kind                  text NOT NULL CHECK (kind IN ('guest','account')),
  age_band              text NOT NULL CHECK (age_band IN ('15_17','18_plus','unknown')),
  age_band_source       text NOT NULL CHECK (age_band_source IN ('self_declared')),
  age_band_confirmed_at timestamptz NOT NULL,
  locale                text NOT NULL CHECK (locale IN ('id','en')),
  version               integer NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at            timestamptz NOT NULL,
  last_active_at        timestamptz NOT NULL,
  upgraded_at           timestamptz NULL,
  guest_expires_at      timestamptz NULL,
  CHECK ((kind = 'guest') = (guest_expires_at IS NOT NULL))
);
CREATE INDEX users_guest_inactive ON users (last_active_at) WHERE kind = 'guest';
CREATE INDEX users_guest_expiry   ON users (guest_expires_at) WHERE kind = 'guest';

CREATE TABLE credentials (
  user_id                     uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  username                    text NOT NULL UNIQUE CHECK (username ~ '^[a-z0-9][a-z0-9._-]{2,31}$'),
  password_hash               bytea NOT NULL CHECK (octet_length(password_hash) = 32),
  password_salt               bytea NOT NULL CHECK (octet_length(password_salt) = 16),
  argon2_version              smallint NOT NULL CHECK (argon2_version = 19),
  argon2_memory_kib           integer NOT NULL CHECK (argon2_memory_kib >= 19456),
  argon2_iterations           integer NOT NULL CHECK (argon2_iterations >= 2),
  argon2_parallelism          smallint NOT NULL CHECK (argon2_parallelism BETWEEN 1 AND 16),
  password_changed_at         timestamptz NOT NULL,
  failed_count                integer NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
  last_failed_at              timestamptz NULL,
  next_attempt_at             timestamptz NULL,
  password_login_disabled_at  timestamptz NULL,
  recovery_generated_at       timestamptz NOT NULL,
  recovery_acknowledged_at    timestamptz NULL,
  created_at                  timestamptz NOT NULL
);

CREATE TABLE recovery_codes (
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash  bytea NOT NULL CHECK (octet_length(code_hash) = 32),
  created_at timestamptz NOT NULL,
  used_at    timestamptz NULL,
  PRIMARY KEY (user_id, code_hash)
);

CREATE TABLE sessions (
  id                   uuid PRIMARY KEY,
  user_id              uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash           bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
  kind                 text NOT NULL CHECK (kind IN ('guest','account')),
  trusted_device       boolean NOT NULL,
  device_browser       text NOT NULL CHECK (device_browser IN ('chrome','edge','firefox','safari','samsung','opera','other')),
  device_os            text NOT NULL CHECK (device_os IN ('android','ios','windows','macos','linux','chromeos','other')),
  created_at           timestamptz NOT NULL,
  last_seen_at         timestamptz NOT NULL,
  idle_timeout_seconds integer NOT NULL CHECK (idle_timeout_seconds > 0),
  absolute_expires_at  timestamptz NOT NULL,
  authenticated_at     timestamptz NOT NULL,
  rotated_from         uuid NULL,
  revoked_at           timestamptz NULL,
  revoke_reason        text NULL CHECK (revoke_reason IN ('logout','logout_all','revoked_by_user','rotated','password_changed','recovery','upgraded')),
  CHECK ((revoked_at IS NULL) = (revoke_reason IS NULL))
);
CREATE INDEX sessions_user_live ON sessions (user_id) WHERE revoked_at IS NULL;
CREATE INDEX sessions_cleanup   ON sessions (absolute_expires_at);

-- No FK on purpose: the row outlives the user as a destruction tombstone.
CREATE TABLE user_data_keys (
  user_id      uuid PRIMARY KEY,
  wrapped_dek  bytea NULL,
  kek_id       text NULL,
  dek_version  smallint NOT NULL DEFAULT 1 CHECK (dek_version > 0),
  created_at   timestamptz NOT NULL,
  rewrapped_at timestamptz NULL,
  destroyed_at timestamptz NULL,
  CHECK ((destroyed_at IS NULL) = (wrapped_dek IS NOT NULL AND kek_id IS NOT NULL))
);
CREATE INDEX user_data_keys_by_kek ON user_data_keys (kek_id) WHERE destroyed_at IS NULL;

CREATE TABLE consents (
  user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purpose        text NOT NULL CHECK (purpose ~ '^[a-z][a-z0-9_]{1,47}$'),
  granted        boolean NOT NULL,
  policy_version text NOT NULL CHECK (char_length(policy_version) BETWEEN 1 AND 64),
  decided_at     timestamptz NOT NULL,
  revision       integer NOT NULL CHECK (revision > 0),
  PRIMARY KEY (user_id, purpose)
);

CREATE TABLE consent_events (
  id             uuid PRIMARY KEY,
  user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purpose        text NOT NULL,
  decision       text NOT NULL CHECK (decision IN ('granted','withdrawn')),
  policy_version text NOT NULL,
  source         text NOT NULL CHECK (source IN ('onboarding','settings','upgrade','system')),
  decided_at     timestamptz NOT NULL
);
CREATE INDEX consent_events_user ON consent_events (user_id, decided_at DESC, id DESC);

-- Ages 15-17. Consent is not access: no guardian principal exists.
CREATE TABLE guardian_consents (
  id               uuid PRIMARY KEY,
  user_id          uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  relationship     text NOT NULL CHECK (relationship IN ('parent','legal_guardian')),
  method           text NOT NULL CHECK (method IN ('guardian_present_attestation')),
  guardian_name_ct bytea NULL,
  policy_version   text NOT NULL,
  granted_at       timestamptz NOT NULL,
  withdrawn_at     timestamptz NULL,
  withdrawn_by     text NULL CHECK (withdrawn_by IN ('user','operator')),
  CHECK ((withdrawn_at IS NULL) = (withdrawn_by IS NULL))
);
CREATE UNIQUE INDEX guardian_consents_one_active ON guardian_consents (user_id) WHERE withdrawn_at IS NULL;

-- Append-only, no content.
CREATE TABLE audit_events (
  id         uuid PRIMARY KEY,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  at         timestamptz NOT NULL,
  type       text NOT NULL CHECK (type ~ '^[a-z_]+\.[a-z_]+$'),
  actor      text NOT NULL CHECK (actor IN ('user','system','recipient')),
  session_id uuid NULL,
  detail     jsonb NOT NULL DEFAULT '{}'::jsonb
             CHECK (jsonb_typeof(detail) = 'object' AND octet_length(detail::text) <= 1024)
);
CREATE INDEX audit_events_user ON audit_events (user_id, at DESC, id DESC);

-- Append-only for the application: UPDATE is rejected; DELETE happens only
-- through the users cascade and the retention job.
CREATE FUNCTION sela_reject_update() RETURNS trigger LANGUAGE plpgsql AS
$$ BEGIN RAISE EXCEPTION 'append-only table %', TG_TABLE_NAME; END $$;
CREATE TRIGGER audit_events_append_only   BEFORE UPDATE ON audit_events   FOR EACH ROW EXECUTE FUNCTION sela_reject_update();
CREATE TRIGGER consent_events_append_only BEFORE UPDATE ON consent_events FOR EACH ROW EXECUTE FUNCTION sela_reject_update();

REVOKE ALL ON instance_meta, users, credentials, recovery_codes, sessions, user_data_keys,
  consents, consent_events, guardian_consents, audit_events FROM PUBLIC;
