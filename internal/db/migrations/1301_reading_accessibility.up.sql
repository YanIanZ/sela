-- sela:kind schema
-- sela:requires 0600
--
-- Accessibility as an approval requirement for reading items (operator
-- console range). Every new approval stores the automatic check report the
-- server computed from the exact content (rule set version, result and the
-- per-check outcome), the manual checklist version and the item ids the
-- reviewer confirmed, and the reviewer's reason when overridable checks
-- (readability only) failed. Blocking checks can never be overridden.
--
-- The columns are NULL for approvals recorded before this migration; the
-- application writes all of them for every new approval and the checks
-- below keep a recorded set internally consistent. Nothing here approves,
-- re-approves or rewrites an existing row.

ALTER TABLE reading_reviews
  ADD COLUMN a11y_checks_version  text  NULL CHECK (a11y_checks_version IS NULL OR char_length(a11y_checks_version) BETWEEN 1 AND 64),
  ADD COLUMN a11y_result          text  NULL CHECK (a11y_result IS NULL OR a11y_result IN ('pass','overridden')),
  ADD COLUMN a11y_checks          jsonb NULL CHECK (a11y_checks IS NULL OR jsonb_typeof(a11y_checks) = 'array'),
  ADD COLUMN a11y_manual_version  text  NULL CHECK (a11y_manual_version IS NULL OR char_length(a11y_manual_version) BETWEEN 1 AND 64),
  ADD COLUMN a11y_manual          jsonb NULL CHECK (a11y_manual IS NULL OR (jsonb_typeof(a11y_manual) = 'array' AND jsonb_array_length(a11y_manual) > 0)),
  ADD COLUMN a11y_override_reason text  NULL CHECK (a11y_override_reason IS NULL OR char_length(btrim(a11y_override_reason)) BETWEEN 20 AND 300);

-- All accessibility columns are recorded together or not at all, and an
-- override reason exists exactly when the result is "overridden".
ALTER TABLE reading_reviews ADD CONSTRAINT reading_reviews_a11y_complete CHECK (
  (a11y_checks_version IS NULL AND a11y_result IS NULL AND a11y_checks IS NULL
     AND a11y_manual_version IS NULL AND a11y_manual IS NULL AND a11y_override_reason IS NULL)
  OR
  (a11y_checks_version IS NOT NULL AND a11y_result IS NOT NULL AND a11y_checks IS NOT NULL
     AND a11y_manual_version IS NOT NULL AND a11y_manual IS NOT NULL
     AND ((a11y_result = 'overridden') = (a11y_override_reason IS NOT NULL)))
);

-- An override is its own audit event, so it can be found without reading
-- every approval.
ALTER TABLE reading_review_audit DROP CONSTRAINT reading_review_audit_action_check;
ALTER TABLE reading_review_audit ADD CONSTRAINT reading_review_audit_action_check
  CHECK (action IN ('reading.approve','reading.revoke','reading.a11y_override'));
