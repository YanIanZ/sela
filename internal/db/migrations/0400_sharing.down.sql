-- Reverts 0400_sharing. Shares, recipient sessions and events are removed;
-- the approved summaries they projected stay untouched.
DROP TABLE IF EXISTS share_events;
DROP TABLE IF EXISTS share_recipient_sessions;
DROP TABLE IF EXISTS shares;
