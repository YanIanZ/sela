-- Reverts 1100_care_network. Care requests end with their shares left in
-- place (the person can still revoke them); partner accounts are removed
-- because the 0910 role list cannot hold them.
ALTER TABLE admin_audit DROP CONSTRAINT admin_audit_action_check;
DELETE FROM admin_audit WHERE action NOT IN
  ('admin.create','admin.login','admin.login_failed','admin.logout','admin.cache_invalidate','admin.revoke');
ALTER TABLE admin_audit ADD CONSTRAINT admin_audit_action_check CHECK (action IN
  ('admin.create','admin.login','admin.login_failed','admin.logout','admin.cache_invalidate','admin.revoke'));

DROP TABLE IF EXISTS care_request_events;
DROP TABLE IF EXISTS care_requests;

DELETE FROM admin_users WHERE role IN ('partner_admin','practitioner','partner_staff');
ALTER TABLE admin_users
  DROP CONSTRAINT IF EXISTS admin_users_license_verified,
  DROP CONSTRAINT IF EXISTS admin_users_practitioner_fields,
  DROP CONSTRAINT IF EXISTS admin_users_partner_org,
  DROP COLUMN IF EXISTS license_verified_at,
  DROP COLUMN IF EXISTS license_verified_by,
  DROP COLUMN IF EXISTS license_no,
  DROP COLUMN IF EXISTS professional_type,
  DROP COLUMN IF EXISTS display_name,
  DROP COLUMN IF EXISTS care_org_id;
ALTER TABLE admin_users DROP CONSTRAINT admin_users_role_check;
ALTER TABLE admin_users ADD CONSTRAINT admin_users_role_check CHECK (role IN
  ('owner','security_admin','directory_editor','directory_reviewer','research_editor','safety_reviewer','ops'));

DROP TABLE IF EXISTS care_orgs;
