-- Password self-service: track when a user's password last changed so refresh
-- tokens issued before that instant can be rejected (logout-everywhere on change).
ALTER TABLE identity.users
    ADD COLUMN password_changed_at timestamptz;
