-- Reverts 0003_session_restored. Sessions ended by a restore reconcile are
-- already revoked and cannot be signed in with; their rows are deleted
-- (as the retention job would after 30 days) so the earlier CHECK holds.
DELETE FROM sessions WHERE revoke_reason = 'restored';
ALTER TABLE sessions DROP CONSTRAINT sessions_revoke_reason_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_revoke_reason_check
  CHECK (revoke_reason IN ('logout','logout_all','revoked_by_user','rotated','password_changed','recovery','upgraded'));
