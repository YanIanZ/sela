-- Reverts 0300_continuity_kinds. The old schema cannot represent memory,
-- summary or follow-up records, so their rows (and, by cascade, their
-- versions) are removed before the narrower CHECK is restored.
DELETE FROM personal_records WHERE kind IN ('memory','summary','followup');
ALTER TABLE personal_records DROP CONSTRAINT personal_records_kind_check;
ALTER TABLE personal_records ADD CONSTRAINT personal_records_kind_check
  CHECK (kind IN ('note','checkin','conversation'));
