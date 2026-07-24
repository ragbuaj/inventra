DROP INDEX IF EXISTS identity.uq_users_username;
ALTER TABLE identity.users DROP COLUMN username;
