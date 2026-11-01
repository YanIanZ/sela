-- sela:kind schema
-- sela:requires 0910,1100
--
-- Partner invitations (D-079). An operator with network/manage invites a
-- clinic administrator, practitioner or staff member from the console. The
-- server stores only the SHA-256 of a one-time activation token and the
-- account's future id; the invitee opens the activation link, receives a
-- fresh TOTP secret (sealed here until activation) and sets their own
-- password. The inviting operator never learns the password or the secret.
-- Internal staff accounts stay out-of-band (D-061).

CREATE TABLE admin_invites (
  id                uuid PRIMARY KEY,
  admin_id          uuid NOT NULL UNIQUE,
  token_hash        bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
  username          text NOT NULL CHECK (username ~ '^[a-z0-9][a-z0-9._-]{2,31}$'),
  role              text NOT NULL CHECK (role IN ('partner_admin','practitioner','partner_staff')),
  care_org_id       uuid NOT NULL REFERENCES care_orgs(id) ON DELETE CASCADE,
  display_name      text NULL CHECK (display_name IS NULL OR char_length(btrim(display_name)) BETWEEN 2 AND 120),
  professional_type text NULL CHECK (professional_type IS NULL OR professional_type IN
    ('psychologist','psychiatrist','doctor','counsellor','nurse','social_worker')),
  license_no        text NULL CHECK (license_no IS NULL OR char_length(btrim(license_no)) BETWEEN 3 AND 64),
  mfa_secret_ct     bytea NULL,
  attempts          integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  created_by        uuid NULL REFERENCES admin_users(id) ON DELETE SET NULL,
  created_at        timestamptz NOT NULL,
  expires_at        timestamptz NOT NULL,
  accepted_at       timestamptz NULL,
  revoked_at        timestamptz NULL,
  CHECK (expires_at > created_at),
  CHECK (role <> 'practitioner' OR (professional_type IS NOT NULL AND display_name IS NOT NULL))
);
CREATE UNIQUE INDEX admin_invites_open_username ON admin_invites (username) WHERE accepted_at IS NULL AND revoked_at IS NULL;
CREATE INDEX admin_invites_org ON admin_invites (care_org_id, created_at DESC);

ALTER TABLE admin_audit DROP CONSTRAINT admin_audit_action_check;
ALTER TABLE admin_audit ADD CONSTRAINT admin_audit_action_check CHECK (action IN (
  'admin.create','admin.login','admin.login_failed','admin.logout','admin.cache_invalidate','admin.revoke',
  'community.restore','community.remove',
  'network.org_create','network.org_verify','network.org_suspend','network.license_verify',
  'network.invite_create','network.invite_revoke','network.invite_accept',
  'care_request.open','care_request.propose','care_request.decline','care_request.complete'));

REVOKE ALL ON admin_invites FROM PUBLIC;
