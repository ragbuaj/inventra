-- Login via NIP atau email (spec 2026-07-23 legacy-parity, Fase 7).
ALTER TABLE identity.users ADD COLUMN username text;

-- Sebuah username tak boleh menyerupai email: menjaga agar ruang username tidak
-- pernah menabrak ruang email pada GetUserByLogin (email = $1 OR username = $1).
ALTER TABLE identity.users
  ADD CONSTRAINT chk_users_username_not_email
  CHECK (username IS NULL OR username !~ '@');

CREATE UNIQUE INDEX uq_users_username ON identity.users (username)
  WHERE deleted_at IS NULL AND username IS NOT NULL;

-- Backfill username dari NIP (code) pegawai tertaut. Hanya isi bila:
--   1. pegawai belum terhapus (e.deleted_at IS NULL), dan
--   2. tidak ada user non-deleted LAIN yang menunjuk pegawai yang sama
--      (users.employee_id TIDAK unik) — mencegah dua user berbagi NIP yang sama
--      dan melanggar uq_users_username sehingga migrasi abort di produksi.
-- User yang terlewat backfill tetap login via email; username dapat diisi kemudian.
UPDATE identity.users u SET username = e.code
FROM masterdata.employees e
WHERE u.employee_id = e.id
  AND u.username IS NULL
  AND u.deleted_at IS NULL
  AND e.deleted_at IS NULL
  AND e.code !~ '@'
  AND NOT EXISTS (
    SELECT 1 FROM identity.users u2
    WHERE u2.employee_id = u.employee_id
      AND u2.id <> u.id
      AND u2.deleted_at IS NULL
  );
