-- sela:kind schema
-- sela:requires 0001,0200
--
-- P4 sharing: one immutable projection of an approved consultation summary,
-- a link token stored only as SHA-256, an access code stored only as a
-- token-keyed hash, recipient sessions, and append-only events. The
-- projection is sealed with the owner's data key, so deleting the owner or
-- the summary ends every future access (ADR-0002, V30/V31).
CREATE TABLE shares (
  id            uuid PRIMARY KEY,
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  summary_id    uuid NOT NULL,
  summary_version integer NOT NULL CHECK (summary_version > 0),
  label         text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80),
  purpose       text NOT NULL CHECK (purpose IN ('consultation','second_opinion','family','other')),
  projection_ct bytea NOT NULL,
  token_hash    bytea NOT NULL UNIQUE,
  code_hash     bytea NOT NULL,
  state         text NOT NULL DEFAULT 'active' CHECK (state IN ('active','revoked','expired')),
  version       integer NOT NULL DEFAULT 1 CHECK (version > 0),
  attempts      integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  expires_at    timestamptz NOT NULL,
  created_at    timestamptz NOT NULL,
  updated_at    timestamptz NOT NULL,
  revoked_at    timestamptz NULL,
  end_reason    text NULL CHECK (end_reason IS NULL OR end_reason IN ('owner_revoked','expired','source_deleted','account_deleted')),
  UNIQUE (id, user_id),
  FOREIGN KEY (summary_id, user_id) REFERENCES personal_records(id, user_id) ON DELETE CASCADE,
  CHECK ((state = 'revoked') = (revoked_at IS NOT NULL)),
  CHECK (expires_at > created_at)
);
CREATE INDEX shares_owner ON shares (user_id, updated_at DESC);
CREATE INDEX shares_summary ON shares (summary_id);

CREATE TABLE share_recipient_sessions (
  id            uuid PRIMARY KEY,
  share_id      uuid NOT NULL REFERENCES shares(id) ON DELETE CASCADE,
  token_hash    bytea NOT NULL UNIQUE,
  created_at    timestamptz NOT NULL,
  expires_at    timestamptz NOT NULL,
  acknowledged_at timestamptz NULL,
  CHECK (expires_at > created_at)
);
CREATE INDEX share_sessions_share ON share_recipient_sessions (share_id);

CREATE TABLE share_events (
  id       uuid PRIMARY KEY,
  share_id uuid NOT NULL REFERENCES shares(id) ON DELETE CASCADE,
  kind     text NOT NULL CHECK (kind IN ('created','opened','acknowledged','revoked')),
  at       timestamptz NOT NULL
);
CREATE INDEX share_events_share ON share_events (share_id, at);

REVOKE ALL ON shares, share_recipient_sessions, share_events FROM PUBLIC;
