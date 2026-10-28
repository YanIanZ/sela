-- sela:kind schema
-- sela:requires 0200
--
-- Explicit, user-created continuity items reuse the personal-record
-- envelope (per-owner encryption, revisions, export and deletion): a
-- manual memory item, a consultation summary the person writes, and a
-- follow-up note attached to a summary. Only the kind CHECK changes; no
-- column is added, so sourceRefs, summaryState, prepared and parentId stay
-- inside the sealed content and cannot be read from the database.
ALTER TABLE personal_records DROP CONSTRAINT personal_records_kind_check;
ALTER TABLE personal_records ADD CONSTRAINT personal_records_kind_check
  CHECK (kind IN ('note','checkin','conversation','memory','summary','followup'));
