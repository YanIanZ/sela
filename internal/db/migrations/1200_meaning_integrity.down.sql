-- Reverts 1200_meaning_integrity. Shares ended by an invalidation keep
-- their revoked state and are relabelled as owner revocations, which is the
-- closest 0400 reason; the lineage graph and stale marks are dropped.
UPDATE shares SET end_reason = 'owner_revoked' WHERE end_reason = 'meaning_invalidated';
ALTER TABLE shares DROP CONSTRAINT shares_end_reason_check;
ALTER TABLE shares ADD CONSTRAINT shares_end_reason_check CHECK (end_reason IS NULL OR end_reason IN
  ('owner_revoked','expired','source_deleted','account_deleted'));
DROP TABLE IF EXISTS mie_stale;
DROP TABLE IF EXISTS mie_events;
DROP TABLE IF EXISTS mie_edges;
