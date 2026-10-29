-- sela:kind schema
-- sela:requires 0001
--
-- Login throttle (ADR-0002 §9). The per-username backoff lives here, for
-- every username key whether or not an account exists, so a throttled
-- answer and its Retry-After are the same for a known and an unknown
-- username and survive a restart. The key is the keyed rate-limit key of
-- the username: HMAC-SHA256 of its normalised form under a secret derived
-- from the KEK (HKDF, "sela.login-throttle.v1") that only the server holds,
-- so a dictionary cannot reverse it; there is no user id and no username
-- in clear, and input that is not a valid username is never written. A
-- row idle for the quiet period (1 h) counts as expired and the retention
-- job deletes it; content dumps leave its rows out.
-- credentials.next_attempt_at is retired; credentials.failed_count stays
-- only for the 100-failure disable.

CREATE TABLE login_throttle (
  key_hash       bytea PRIMARY KEY CHECK (octet_length(key_hash) = 32),
  failures       integer NOT NULL CHECK (failures > 0),
  last_failed_at timestamptz NOT NULL
);
CREATE INDEX login_throttle_last_failed ON login_throttle (last_failed_at);

ALTER TABLE credentials DROP COLUMN next_attempt_at;

REVOKE ALL ON login_throttle FROM PUBLIC;
