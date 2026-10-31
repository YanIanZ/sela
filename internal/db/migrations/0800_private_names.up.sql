-- sela:kind schema
-- sela:requires 0001
--
-- The owner's "names kept private" list (privacy controls). Only the
-- ciphertext of each name is stored, sealed with the owner's data key under
-- the ref private_name/<id>/name, so a database dump, a backup or a restored
-- copy never contains a readable name. The minimum ciphertext length rejects
-- a plaintext row by accident: a sealed value is plaintext + 29 bytes of
-- nonce and tag, so the shortest allowed name (1 byte) yields 30 bytes while
-- the longest allowed name (120 runes, at most 480 UTF-8 bytes) yields 509.
-- Deleting the owner removes the rows and the destroyed key makes old
-- copies unreadable.
CREATE TABLE private_names (
  id         uuid PRIMARY KEY,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name_ct    bytea NOT NULL CHECK (octet_length(name_ct) BETWEEN 30 AND 509),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);
CREATE INDEX private_names_owner ON private_names (user_id, created_at, id);

REVOKE ALL ON private_names FROM PUBLIC;
