-- sela:kind schema
-- sela:requires 0001
--
-- Restore reconcile ends restored sessions (ADR-0002 §11, review round 2).
-- A content dump holds the sessions that were live when it was taken; a
-- session revoked after that (logout, logout-all, revoke, the revoke step
-- of a password change or recovery) would be live again after a restore.
-- `sela restore reconcile` therefore revokes every live session of the
-- restored database with the new reason 'restored', and everyone signs in
-- again. Only the CHECK of sessions.revoke_reason changes.

ALTER TABLE sessions DROP CONSTRAINT sessions_revoke_reason_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_revoke_reason_check
  CHECK (revoke_reason IN ('logout','logout_all','revoked_by_user','rotated','password_changed','recovery','upgraded','restored'));
