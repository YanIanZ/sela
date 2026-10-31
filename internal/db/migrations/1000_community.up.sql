-- sela:kind schema
-- sela:requires 0001
--
-- Anonymous peer sharing (D-072, S-030). A post or reply is public content
-- the author chose to publish under a random alias; the author id is kept
-- only so the author can delete it, for rate limits and so that deleting
-- the account removes everything they posted (ON DELETE CASCADE). It is
-- never returned to another person or to an operator. Status:
--   published  visible to everyone
--   held       not shown: moderation was unavailable or unsure
--   hidden     not shown: moderation or reports found it harmful
--   removed    not shown: an operator removed it (kept for the audit)
-- Crisis writing is never stored here: the safety router stops it first.
CREATE TABLE community_posts (
  id          uuid PRIMARY KEY,
  author_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  alias       text NOT NULL CHECK (char_length(alias) BETWEEN 3 AND 40),
  topic       text NOT NULL CHECK (topic IN ('daily', 'work', 'study', 'family', 'relationship', 'loneliness', 'other')),
  body        text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
  status      text NOT NULL CHECK (status IN ('published', 'held', 'hidden', 'removed')),
  labels      text[] NOT NULL DEFAULT '{}',
  reports     integer NOT NULL DEFAULT 0 CHECK (reports >= 0),
  created_at  timestamptz NOT NULL,
  updated_at  timestamptz NOT NULL
);
CREATE INDEX community_posts_feed ON community_posts (status, created_at DESC);
CREATE INDEX community_posts_author ON community_posts (author_id, created_at DESC);

CREATE TABLE community_replies (
  id          uuid PRIMARY KEY,
  post_id     uuid NOT NULL REFERENCES community_posts(id) ON DELETE CASCADE,
  author_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  alias       text NOT NULL CHECK (char_length(alias) BETWEEN 3 AND 40),
  body        text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 1000),
  status      text NOT NULL CHECK (status IN ('published', 'held', 'hidden', 'removed')),
  labels      text[] NOT NULL DEFAULT '{}',
  reports     integer NOT NULL DEFAULT 0 CHECK (reports >= 0),
  created_at  timestamptz NOT NULL,
  updated_at  timestamptz NOT NULL
);
CREATE INDEX community_replies_thread ON community_replies (post_id, created_at);
CREATE INDEX community_replies_author ON community_replies (author_id, created_at DESC);
CREATE INDEX community_replies_queue ON community_replies (status, updated_at DESC);

CREATE TABLE community_reports (
  target_kind text NOT NULL CHECK (target_kind IN ('post', 'reply')),
  target_id   uuid NOT NULL,
  reporter_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reason      text NOT NULL CHECK (reason IN ('bullying', 'harassment', 'hate', 'sexual', 'self_harm', 'personal_info', 'spam', 'other')),
  created_at  timestamptz NOT NULL,
  PRIMARY KEY (target_kind, target_id, reporter_id)
);

REVOKE ALL ON community_posts, community_replies, community_reports FROM PUBLIC;
