-- sela:kind schema
-- sela:requires 0001
--
-- Operator sign-off for the embedded reading library (range 600-699).
-- One current approval per slug. content_hash is the SHA-256 of the exact
-- embedded item object, so any text or formatting change invalidates the
-- approval until a human re-approves it. revoked_at is kept so a revocation
-- can veto an item whose embedded review claims approval.
--
-- approved_by/actor_id are opaque staff uuids without a foreign key: the
-- operator range (09xx) is applied after this range, so a reference here
-- would make the reading range depend on the ops range. Audit follows the
-- care_audit/admin_audit conventions and stores no user content.

CREATE TABLE reading_reviews (
  slug            text PRIMARY KEY CHECK (char_length(slug) BETWEEN 1 AND 120),
  reviewer_name   text NOT NULL CHECK (char_length(btrim(reviewer_name)) BETWEEN 1 AND 160),
  reviewed_on     date NOT NULL,
  note            text NOT NULL DEFAULT '' CHECK (char_length(note) <= 1000),
  verifier_status text NOT NULL CHECK (verifier_status = 'supported'),
  content_hash    text NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
  approved_by     uuid NOT NULL,
  approved_at     timestamptz NOT NULL,
  revoked_at      timestamptz NULL,
  version         integer NOT NULL DEFAULT 1 CHECK (version > 0)
);

CREATE TABLE reading_review_audit (
  id          uuid PRIMARY KEY,
  actor_id    uuid NULL,
  action      text NOT NULL CHECK (action IN ('reading.approve','reading.revoke')),
  object_kind text NOT NULL CHECK (object_kind = 'reading_item'),
  object_id   text NOT NULL,
  old_status  text NULL,
  new_status  text NULL,
  reason      text NULL CHECK (reason IS NULL OR char_length(reason) <= 300),
  created_at  timestamptz NOT NULL
);
CREATE INDEX reading_review_audit_created ON reading_review_audit (created_at DESC);

REVOKE ALL ON reading_reviews, reading_review_audit FROM PUBLIC;
