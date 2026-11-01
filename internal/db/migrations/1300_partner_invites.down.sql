-- Reverts 1300_partner_invites. Accounts created from invitations stay;
-- open invitations are dropped and their audit rows removed with the
-- vocabulary.
ALTER TABLE admin_audit DROP CONSTRAINT admin_audit_action_check;
DELETE FROM admin_audit WHERE action IN ('network.invite_create','network.invite_revoke','network.invite_accept');
ALTER TABLE admin_audit ADD CONSTRAINT admin_audit_action_check CHECK (action IN (
  'admin.create','admin.login','admin.login_failed','admin.logout','admin.cache_invalidate','admin.revoke',
  'community.restore','community.remove',
  'network.org_create','network.org_verify','network.org_suspend','network.license_verify',
  'care_request.open','care_request.propose','care_request.decline','care_request.complete'));
DROP TABLE IF EXISTS admin_invites;
