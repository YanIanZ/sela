-- sela:kind schema
-- sela:requires 0001,0200,0400,1100
--
-- Meaning Integrity Engine 1.0 (D-078). A plain-id lineage graph of what was
-- derived from what (source record version -> memory claim -> summary ->
-- share -> care request), the stale marks an invalidation writes, and one
-- content-free event per invalidation. A correction, a rejected claim, an
-- edited or deleted source marks every current descendant stale and revokes
-- every active share built on it in the same transaction, so no recipient,
-- partner or export reads it as current afterwards. Ids and versions only:
-- no text, quote, hash or title is stored here (MIE-I08).

CREATE TABLE mie_edges (
  user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  parent_kind    text NOT NULL CHECK (parent_kind IN ('record','memory','summary','share')),
  parent_id      uuid NOT NULL,
  parent_version integer NOT NULL CHECK (parent_version > 0),
  child_kind     text NOT NULL CHECK (child_kind IN ('memory','summary','share','care_request')),
  child_id       uuid NOT NULL,
  child_version  integer NOT NULL CHECK (child_version > 0),
  created_at     timestamptz NOT NULL,
  PRIMARY KEY (user_id, parent_kind, parent_id, parent_version, child_kind, child_id, child_version),
  CHECK (parent_id <> child_id)
);
CREATE INDEX mie_edges_parent ON mie_edges (user_id, parent_id, parent_version);
CREATE INDEX mie_edges_child ON mie_edges (user_id, child_id, child_version);

CREATE TABLE mie_events (
  id             uuid PRIMARY KEY,
  user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  origin_kind    text NOT NULL CHECK (origin_kind IN ('record','memory','summary')),
  origin_id      uuid NOT NULL,
  origin_version integer NOT NULL CHECK (origin_version > 0),
  reason         text NOT NULL CHECK (reason IN ('source_edited','source_deleted','claim_corrected','claim_rejected','claim_deleted')),
  memories       integer NOT NULL DEFAULT 0 CHECK (memories >= 0),
  summaries      integer NOT NULL DEFAULT 0 CHECK (summaries >= 0),
  shares         integer NOT NULL DEFAULT 0 CHECK (shares >= 0),
  care_requests  integer NOT NULL DEFAULT 0 CHECK (care_requests >= 0),
  at             timestamptz NOT NULL
);
CREATE INDEX mie_events_owner ON mie_events (user_id, at DESC);

-- One stale mark per record version. A newer version is not stale until an
-- invalidation reaches it; history stays readable to the owner.
CREATE TABLE mie_stale (
  record_id   uuid NOT NULL,
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  version     integer NOT NULL CHECK (version > 0),
  kind        text NOT NULL CHECK (kind IN ('memory','summary')),
  event_id    uuid NOT NULL REFERENCES mie_events(id) ON DELETE CASCADE,
  reason      text NOT NULL,
  at          timestamptz NOT NULL,
  PRIMARY KEY (record_id, version),
  FOREIGN KEY (record_id, user_id) REFERENCES personal_records(id, user_id) ON DELETE CASCADE
);
CREATE INDEX mie_stale_owner ON mie_stale (user_id);

ALTER TABLE shares DROP CONSTRAINT shares_end_reason_check;
ALTER TABLE shares ADD CONSTRAINT shares_end_reason_check CHECK (end_reason IS NULL OR end_reason IN
  ('owner_revoked','expired','source_deleted','account_deleted','meaning_invalidated'));

REVOKE ALL ON mie_edges, mie_events, mie_stale FROM PUBLIC;
