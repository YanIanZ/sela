-- sela:kind schema
-- sela:requires 0001,0400,0910
--
-- Professional care network (D-076). Partner organisations (clinics,
-- practices, hospitals, puskesmas) get org-scoped operator accounts for
-- their administrators, practitioners and staff, and a person can send one
-- structured care request that carries an approved consultation summary to
-- one organisation through a share (0400). The organisation answers with
-- proposed times and its own contact instructions; the person confirms.
-- There is no free-text chat and Sela never passes on the person's contact
-- details. Organisations are synthetic (simulated) unless the deployment
-- acknowledges real partners (SELA_CARE_NETWORK_REAL_ACK). Nothing here
-- stores user content in clear text: the summary stays sealed in the share.

CREATE TABLE care_orgs (
  id               uuid PRIMARY KEY,
  kind             text NOT NULL CHECK (kind IN ('clinic','hospital','puskesmas','practice','organization')),
  name             text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 2 AND 160),
  city             text NOT NULL DEFAULT '' CHECK (char_length(city) <= 120),
  description      text NOT NULL DEFAULT '' CHECK (char_length(description) <= 1000),
  modes            text[] NOT NULL DEFAULT '{}' CHECK (modes <@ ARRAY['online','in_person']::text[]),
  stages           text[] NOT NULL DEFAULT '{}' CHECK (stages <@ ARRAY['everyday','attention','support','urgent']::text[]),
  synthetic        boolean NOT NULL,
  status           text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','verified','suspended')),
  accepts_requests boolean NOT NULL DEFAULT true,
  created_by       uuid NULL REFERENCES admin_users(id) ON DELETE SET NULL,
  verified_by      uuid NULL REFERENCES admin_users(id) ON DELETE SET NULL,
  created_at       timestamptz NOT NULL,
  verified_at      timestamptz NULL,
  updated_at       timestamptz NOT NULL,
  CHECK (status <> 'verified' OR verified_at IS NOT NULL)
);
CREATE INDEX care_orgs_public ON care_orgs (status, accepts_requests, name);

-- Partner roles are always bound to one organisation; internal roles never.
ALTER TABLE admin_users DROP CONSTRAINT admin_users_role_check;
ALTER TABLE admin_users ADD CONSTRAINT admin_users_role_check CHECK (role IN (
  'owner','security_admin','directory_editor','directory_reviewer','research_editor','safety_reviewer','ops',
  'partner_admin','practitioner','partner_staff'));
ALTER TABLE admin_users
  ADD COLUMN care_org_id        uuid NULL REFERENCES care_orgs(id) ON DELETE RESTRICT,
  ADD COLUMN display_name       text NULL CHECK (display_name IS NULL OR char_length(btrim(display_name)) BETWEEN 2 AND 120),
  ADD COLUMN professional_type  text NULL CHECK (professional_type IS NULL OR professional_type IN
    ('psychologist','psychiatrist','doctor','counsellor','nurse','social_worker')),
  ADD COLUMN license_no         text NULL CHECK (license_no IS NULL OR char_length(btrim(license_no)) BETWEEN 3 AND 64),
  ADD COLUMN license_verified_by uuid NULL REFERENCES admin_users(id) ON DELETE SET NULL,
  ADD COLUMN license_verified_at timestamptz NULL,
  ADD CONSTRAINT admin_users_partner_org CHECK ((role IN ('partner_admin','practitioner','partner_staff')) = (care_org_id IS NOT NULL)),
  ADD CONSTRAINT admin_users_practitioner_fields CHECK (role <> 'practitioner' OR (professional_type IS NOT NULL AND display_name IS NOT NULL)),
  ADD CONSTRAINT admin_users_license_verified CHECK ((license_verified_at IS NULL) OR (license_no IS NOT NULL));
CREATE INDEX admin_users_care_org ON admin_users (care_org_id) WHERE care_org_id IS NOT NULL;

CREATE TABLE care_requests (
  id              uuid PRIMARY KEY,
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  org_id          uuid NOT NULL REFERENCES care_orgs(id) ON DELETE RESTRICT,
  practitioner_id uuid NULL REFERENCES admin_users(id) ON DELETE SET NULL,
  share_id        uuid NOT NULL UNIQUE REFERENCES shares(id) ON DELETE CASCADE,
  reference       text NOT NULL UNIQUE CHECK (reference ~ '^SR-[A-Z0-9]{6}$'),
  mode            text NOT NULL CHECK (mode IN ('online','in_person','either')),
  state           text NOT NULL CHECK (state IN ('sent','proposed','declined','confirmed','completed','cancelled','expired')),
  proposal        jsonb NULL,
  confirmed_slot  timestamptz NULL,
  decline_reason  text NULL CHECK (decline_reason IS NULL OR decline_reason IN ('no_capacity','not_a_fit','refer_elsewhere','other')),
  synthetic       boolean NOT NULL,
  version         integer NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at      timestamptz NOT NULL,
  updated_at      timestamptz NOT NULL,
  responded_at    timestamptz NULL,
  CHECK ((state = 'confirmed' OR state = 'completed') = (confirmed_slot IS NOT NULL) OR state IN ('cancelled','expired')),
  CHECK ((state = 'declined') = (decline_reason IS NOT NULL))
);
CREATE INDEX care_requests_owner ON care_requests (user_id, updated_at DESC);
CREATE INDEX care_requests_org ON care_requests (org_id, state, updated_at DESC);

CREATE TABLE care_request_events (
  id         uuid PRIMARY KEY,
  request_id uuid NOT NULL REFERENCES care_requests(id) ON DELETE CASCADE,
  kind       text NOT NULL CHECK (kind IN ('sent','opened','proposed','declined','confirmed','completed','cancelled','expired')),
  actor      text NOT NULL CHECK (actor IN ('user','partner','system')),
  at         timestamptz NOT NULL
);
CREATE INDEX care_request_events_request ON care_request_events (request_id, at);

-- The audit vocabulary gains the community moderation actions (written by
-- D-072's queue but missing from 0910's list, so they were refused) and the
-- care network actions.
ALTER TABLE admin_audit DROP CONSTRAINT admin_audit_action_check;
ALTER TABLE admin_audit ADD CONSTRAINT admin_audit_action_check CHECK (action IN (
  'admin.create','admin.login','admin.login_failed','admin.logout','admin.cache_invalidate','admin.revoke',
  'community.restore','community.remove',
  'network.org_create','network.org_verify','network.org_suspend','network.license_verify',
  'care_request.open','care_request.propose','care_request.decline','care_request.complete'));

REVOKE ALL ON care_orgs, care_requests, care_request_events FROM PUBLIC;
