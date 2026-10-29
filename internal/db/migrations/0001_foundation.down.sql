-- Reverts 0001_foundation: drops every foundation object in reverse order.
DROP TRIGGER IF EXISTS consent_events_append_only ON consent_events;
DROP TRIGGER IF EXISTS audit_events_append_only ON audit_events;
DROP FUNCTION IF EXISTS sela_reject_update();
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS guardian_consents;
DROP TABLE IF EXISTS consent_events;
DROP TABLE IF EXISTS consents;
DROP TABLE IF EXISTS user_data_keys;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS recovery_codes;
DROP TABLE IF EXISTS credentials;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS instance_meta;
