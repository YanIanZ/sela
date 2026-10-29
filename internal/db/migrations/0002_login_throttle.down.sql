-- Reverts 0002_login_throttle: restores the (empty) per-account
-- next_attempt_at column and drops the login throttle.
ALTER TABLE credentials ADD COLUMN next_attempt_at timestamptz NULL;
DROP TABLE IF EXISTS login_throttle;
