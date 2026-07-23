-- Login via NIP atau email (spec 2026-07-23 legacy-parity, Fase 7).
ALTER TABLE identity.users ADD COLUMN username text;
CREATE UNIQUE INDEX uq_users_username ON identity.users (username)
  WHERE deleted_at IS NULL AND username IS NOT NULL;

-- Backfill username dari NIP (code) pegawai tertaut.
UPDATE identity.users u SET username = e.code
FROM masterdata.employees e
WHERE u.employee_id = e.id AND u.username IS NULL AND u.deleted_at IS NULL;
