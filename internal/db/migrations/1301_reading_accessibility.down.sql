-- Reverts 1301_reading_accessibility. Approvals stay; their recorded
-- accessibility results and the override audit events are dropped with the
-- vocabulary.
ALTER TABLE reading_review_audit DROP CONSTRAINT reading_review_audit_action_check;
DELETE FROM reading_review_audit WHERE action = 'reading.a11y_override';
ALTER TABLE reading_review_audit ADD CONSTRAINT reading_review_audit_action_check
  CHECK (action IN ('reading.approve','reading.revoke'));
ALTER TABLE reading_reviews DROP CONSTRAINT reading_reviews_a11y_complete;
ALTER TABLE reading_reviews
  DROP COLUMN a11y_override_reason,
  DROP COLUMN a11y_manual,
  DROP COLUMN a11y_manual_version,
  DROP COLUMN a11y_checks,
  DROP COLUMN a11y_result,
  DROP COLUMN a11y_checks_version;
