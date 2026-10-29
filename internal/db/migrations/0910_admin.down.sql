-- Reverts 0910_admin: no personal data exists in these tables.
DROP TABLE IF EXISTS admin_audit;
DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS admin_users;
