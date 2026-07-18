-- Role read-only untuk akses analitik/inspeksi database PRODUKSI (mis. dari
-- MCP postgres di Claude Code). Role ini HANYA boleh SELECT — tidak ada
-- INSERT/UPDATE/DELETE/DDL.
--
-- Jalankan sekali di VPS:
--   docker exec -i inventra-postgres psql -U inventra -d inventra \
--     -v ro_password="'PASSWORD_KUAT_DI_SINI'" < ops/db/mcp_readonly_role.sql
--
-- Bangkitkan password: openssl rand -hex 32

\set ON_ERROR_STOP on

-- 1) Role login tanpa privilese bawaan apa pun.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'inventra_ro') THEN
    CREATE ROLE inventra_ro LOGIN;
  END IF;
END
$$;

ALTER ROLE inventra_ro WITH PASSWORD :ro_password NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION;

-- Batasi beban query eksploratif agar tidak mengganggu trafik aplikasi.
ALTER ROLE inventra_ro SET statement_timeout = '30s';
ALTER ROLE inventra_ro SET idle_in_transaction_session_timeout = '60s';
ALTER ROLE inventra_ro SET default_transaction_read_only = on;

-- 2) Cabut hak menulis di database & schema publik.
REVOKE CREATE ON SCHEMA public FROM inventra_ro;
REVOKE ALL ON DATABASE inventra FROM inventra_ro;
GRANT CONNECT ON DATABASE inventra TO inventra_ro;

-- 3) Beri SELECT pada seluruh schema modul (lihat docs/DATABASE.md).
DO $$
DECLARE
  s text;
  schemas text[] := ARRAY[
    'public', 'shared', 'identity', 'masterdata', 'audit',
    'asset', 'approval', 'assignment', 'maintenance', 'depreciation',
    'disposal', 'transfer', 'stockopname', 'import', 'notification'
  ];
BEGIN
  FOREACH s IN ARRAY schemas LOOP
    IF EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = s) THEN
      EXECUTE format('GRANT USAGE ON SCHEMA %I TO inventra_ro', s);
      EXECUTE format('GRANT SELECT ON ALL TABLES IN SCHEMA %I TO inventra_ro', s);
      EXECUTE format('GRANT SELECT ON ALL SEQUENCES IN SCHEMA %I TO inventra_ro', s);
      -- Tabel baru dari migrasi berikutnya ikut terbaca otomatis.
      EXECUTE format(
        'ALTER DEFAULT PRIVILEGES FOR ROLE inventra IN SCHEMA %I GRANT SELECT ON TABLES TO inventra_ro', s);
    END IF;
  END LOOP;
END
$$;

-- 4) Verifikasi: harus mengembalikan 0 baris (tidak ada hak tulis).
SELECT table_schema, table_name, privilege_type
FROM information_schema.table_privileges
WHERE grantee = 'inventra_ro'
  AND privilege_type <> 'SELECT';
