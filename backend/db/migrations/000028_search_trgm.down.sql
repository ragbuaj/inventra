-- 000028_search_trgm.down.sql
-- Extension is left installed (cheap, may be used by other objects).
DROP INDEX IF EXISTS asset.assets_name_trgm_idx;
DROP INDEX IF EXISTS asset.assets_tag_trgm_idx;
DROP INDEX IF EXISTS asset.assets_serial_trgm_idx;
DROP INDEX IF EXISTS masterdata.employees_name_trgm_idx;
DROP INDEX IF EXISTS masterdata.employees_code_trgm_idx;
DROP INDEX IF EXISTS masterdata.offices_name_trgm_idx;
DROP INDEX IF EXISTS masterdata.offices_code_trgm_idx;
DROP INDEX IF EXISTS identity.users_name_trgm_idx;
DROP INDEX IF EXISTS approval.requests_reason_trgm_idx;
