DROP TYPE IF EXISTS audit_action;
DROP TYPE IF EXISTS import_status;
DROP TYPE IF EXISTS attachment_kind;
DROP TYPE IF EXISTS request_status;
DROP TYPE IF EXISTS request_type;
DROP TYPE IF EXISTS maintenance_status;
DROP TYPE IF EXISTS maintenance_type;
DROP TYPE IF EXISTS assignment_status;
DROP TYPE IF EXISTS depreciation_method;
DROP TYPE IF EXISTS asset_status;
DROP TYPE IF EXISTS scope_level;
DROP TYPE IF EXISTS user_status;

DROP FUNCTION IF EXISTS set_updated_at();

DROP EXTENSION IF EXISTS citext;
