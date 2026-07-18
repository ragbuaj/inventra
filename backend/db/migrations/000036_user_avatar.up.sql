-- The column has always held nothing (no code ever wrote it). It now stores a
-- MinIO object key rather than a URL — this codebase proxies object bytes and
-- never presigns — so the name is corrected to match masterdata.employees.avatar_key.
ALTER TABLE identity.users RENAME COLUMN avatar_url TO avatar_key;
