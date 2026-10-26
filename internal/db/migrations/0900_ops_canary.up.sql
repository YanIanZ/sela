-- sela:kind schema
-- sela:requires 0001
--
-- Operator canary (ADR-0002 §11 restore drill): one synthetic record sealed
-- under a data key that has a user_data_keys row but no users row, so
-- "sela ops canary check" proves the KEK -> DEK -> field path after a
-- restore or a KEK rotation. Holds no user data and no user_id column.

CREATE TABLE ops_canary (
  singleton  boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  key_id     uuid NOT NULL,
  sealed     bytea NOT NULL,
  digest     bytea NOT NULL CHECK (octet_length(digest) = 32),
  created_at timestamptz NOT NULL
);

REVOKE ALL ON ops_canary FROM PUBLIC;
