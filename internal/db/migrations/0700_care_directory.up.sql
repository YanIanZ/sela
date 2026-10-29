-- sela:kind schema
-- sela:requires 0001
--
-- Public, sourced planned-care directory and the isolated synthetic partner
-- sandbox (D11). No table references a consumer user or stores private
-- stories; staff identities are opaque uuid values without a foreign key so
-- this product range does not depend on the ops range.
CREATE TABLE care_directory_entries (
  id                 uuid PRIMARY KEY,
  kind               text NOT NULL CHECK (kind IN ('organization','facility','practitioner')),
  name               text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 160),
  city               text NOT NULL DEFAULT '' CHECK (char_length(city) <= 120),
  description        text NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
  contact_url        text NOT NULL DEFAULT '' CHECK (char_length(contact_url) <= 2048),
  source_url         text NOT NULL CHECK (char_length(source_url) BETWEEN 1 AND 2048),
  source_publisher   text NOT NULL CHECK (char_length(source_publisher) BETWEEN 1 AND 160),
  source_accessed_at date NOT NULL,
  status             text NOT NULL DEFAULT 'draft'
                     CHECK (status IN ('draft','pending','verified','rejected','stale','unpublished')),
  synthetic          boolean NOT NULL DEFAULT false,
  created_by         uuid NOT NULL,
  verified_by        uuid NULL,
  verified_at        timestamptz NULL,
  reverify_at        date NULL,
  version            integer NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at         timestamptz NOT NULL,
  updated_at         timestamptz NOT NULL
);
CREATE INDEX care_directory_status ON care_directory_entries (status, kind);
CREATE INDEX care_directory_public ON care_directory_entries (kind, city) WHERE status = 'verified';

CREATE TABLE care_sandbox_requests (
  id           uuid PRIMARY KEY,
  organization text NOT NULL CHECK (char_length(organization) BETWEEN 1 AND 160),
  professional text NOT NULL DEFAULT '' CHECK (char_length(professional) <= 160),
  service      text NOT NULL DEFAULT '' CHECK (char_length(service) <= 200),
  status       text NOT NULL DEFAULT 'prepared'
               CHECK (status IN ('prepared','pending','received','acknowledged','revoked','closed')),
  note         text NOT NULL DEFAULT '' CHECK (char_length(note) <= 1000),
  synthetic    boolean NOT NULL DEFAULT true CHECK (synthetic),
  created_by   uuid NOT NULL,
  version      integer NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at   timestamptz NOT NULL,
  updated_at   timestamptz NOT NULL
);
CREATE INDEX care_sandbox_created ON care_sandbox_requests (created_at DESC);

CREATE TABLE care_audit (
  id          uuid PRIMARY KEY,
  actor_id    uuid NULL,
  action      text NOT NULL CHECK (action IN ('directory.create','directory.update','directory.submit',
                                             'directory.verify','directory.reject','directory.unpublish',
                                             'sandbox.create','sandbox.transition')),
  object_kind text NOT NULL,
  object_id   text NOT NULL,
  old_status  text NULL,
  new_status  text NULL,
  reason      text NULL CHECK (reason IS NULL OR char_length(reason) <= 300),
  created_at  timestamptz NOT NULL
);
CREATE INDEX care_audit_created ON care_audit (created_at DESC);

REVOKE ALL ON care_directory_entries, care_sandbox_requests, care_audit FROM PUBLIC;
