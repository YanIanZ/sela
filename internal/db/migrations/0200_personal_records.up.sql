-- sela:kind schema
CREATE TABLE personal_records (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK(kind IN ('note','checkin','conversation')),
 local_day date NULL,
 version integer NOT NULL CHECK(version > 0),
 content_ct bytea NOT NULL,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 UNIQUE(id,user_id),
 CHECK ((kind='checkin')=(local_day IS NOT NULL))
);
CREATE UNIQUE INDEX personal_checkin_day ON personal_records(user_id,local_day) WHERE kind='checkin';
CREATE INDEX personal_records_owner ON personal_records(user_id,updated_at DESC);
CREATE TABLE personal_record_versions (
 record_id uuid NOT NULL,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 version integer NOT NULL CHECK(version > 0),
 content_ct bytea NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY(record_id,version),
 FOREIGN KEY(record_id,user_id) REFERENCES personal_records(id,user_id) ON DELETE CASCADE
);
REVOKE ALL ON personal_records,personal_record_versions FROM PUBLIC;
