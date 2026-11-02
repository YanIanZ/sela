-- sela:kind schema
-- sela:requires 0300,1200
--
-- Personal Action Bridge (BW-3, "Satu langkah, pilihanku"). An action step
-- is one more sealed personal-record kind: the same per-owner vault
-- envelope, append-only revisions, export and account/guest deletion apply,
-- and its text, status, due date, origin and source pins stay inside the
-- sealed content. Only the kind CHECKs change, plus one content-free counter.
--
-- MIE: an action that the owner pinned to a source record version is a leaf
-- child in mie_edges (record -> action_step). Editing or deleting that
-- source writes a stale mark on the action's current version, which the
-- store reads as "needs_review"; the action text is never deleted by an
-- invalidation. An action is never a parent, so the graph walk stays inside
-- its existing 500-node / 6-level bounds.
ALTER TABLE personal_records DROP CONSTRAINT personal_records_kind_check;
ALTER TABLE personal_records ADD CONSTRAINT personal_records_kind_check
  CHECK (kind IN ('note','checkin','conversation','memory','summary','followup','action_step'));

ALTER TABLE mie_edges DROP CONSTRAINT mie_edges_child_kind_check;
ALTER TABLE mie_edges ADD CONSTRAINT mie_edges_child_kind_check
  CHECK (child_kind IN ('memory','summary','share','care_request','action_step'));

ALTER TABLE mie_stale DROP CONSTRAINT mie_stale_kind_check;
ALTER TABLE mie_stale ADD CONSTRAINT mie_stale_kind_check
  CHECK (kind IN ('memory','summary','action_step'));

ALTER TABLE mie_events ADD COLUMN actions integer NOT NULL DEFAULT 0 CHECK (actions >= 0);
