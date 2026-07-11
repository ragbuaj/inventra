-- 000028_search_trgm.up.sql
-- Trigram indexes so ILIKE '%q%' (global search + existing list search) uses an index.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS assets_name_trgm_idx      ON asset.assets          USING gin (name gin_trgm_ops)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS assets_tag_trgm_idx       ON asset.assets          USING gin (asset_tag gin_trgm_ops)     WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS assets_serial_trgm_idx    ON asset.assets          USING gin (serial_number gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS employees_name_trgm_idx   ON masterdata.employees  USING gin (name gin_trgm_ops)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS employees_code_trgm_idx   ON masterdata.employees  USING gin (code gin_trgm_ops)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS offices_name_trgm_idx     ON masterdata.offices    USING gin (name gin_trgm_ops)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS offices_code_trgm_idx     ON masterdata.offices    USING gin (code gin_trgm_ops)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS users_name_trgm_idx       ON identity.users        USING gin (name gin_trgm_ops)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS requests_reason_trgm_idx  ON approval.requests     USING gin (reason gin_trgm_ops)        WHERE deleted_at IS NULL;
