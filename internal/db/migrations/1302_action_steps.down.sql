-- Reverts 1302_action_steps. The older schema cannot represent action
-- steps, so their rows (and, by cascade, their revisions and stale marks)
-- and their lineage edges are removed before the narrower CHECKs return.
DELETE FROM mie_edges WHERE child_kind = 'action_step';
DELETE FROM mie_stale WHERE kind = 'action_step';
DELETE FROM personal_records WHERE kind = 'action_step';
ALTER TABLE mie_events DROP COLUMN actions;

ALTER TABLE mie_stale DROP CONSTRAINT mie_stale_kind_check;
ALTER TABLE mie_stale ADD CONSTRAINT mie_stale_kind_check
  CHECK (kind IN ('memory','summary'));

ALTER TABLE mie_edges DROP CONSTRAINT mie_edges_child_kind_check;
ALTER TABLE mie_edges ADD CONSTRAINT mie_edges_child_kind_check
  CHECK (child_kind IN ('memory','summary','share','care_request'));

ALTER TABLE personal_records DROP CONSTRAINT personal_records_kind_check;
ALTER TABLE personal_records ADD CONSTRAINT personal_records_kind_check
  CHECK (kind IN ('note','checkin','conversation','memory','summary','followup'));
