-- Schemas, extensions, shared trigger function, and enum types.
-- Schema-per-module layout (see docs/DATABASE.md §1.2). Shared vocabulary
-- (enums + set_updated_at) lives in `shared`; each module owns its own schema.

CREATE SCHEMA IF NOT EXISTS shared;
CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS audit;

-- Extensions live in public (standard location); gen_random_uuid() resolves via search_path.
CREATE EXTENSION IF NOT EXISTS citext;

-- Shared trigger function: keeps updated_at current on every UPDATE.
CREATE OR REPLACE FUNCTION shared.set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Enum types (shared vocabulary). NOTE: role is NOT an enum (configurable identity.roles).
CREATE TYPE shared.user_status         AS ENUM ('active', 'inactive', 'suspended');
CREATE TYPE shared.scope_level         AS ENUM ('global', 'office_subtree', 'office', 'own');
CREATE TYPE shared.asset_status        AS ENUM ('available', 'assigned', 'under_maintenance', 'in_transfer', 'retired', 'disposed', 'lost');
CREATE TYPE shared.depreciation_method AS ENUM ('straight_line', 'declining_balance');
CREATE TYPE shared.assignment_status   AS ENUM ('active', 'returned');
CREATE TYPE shared.maintenance_type    AS ENUM ('preventive', 'corrective');
CREATE TYPE shared.maintenance_status  AS ENUM ('scheduled', 'in_progress', 'completed', 'cancelled');
CREATE TYPE shared.request_type        AS ENUM ('asset_create', 'asset_disposal', 'asset_transfer', 'assignment', 'maintenance', 'valuation_exclusion');
CREATE TYPE shared.request_status      AS ENUM ('pending', 'approved', 'rejected', 'cancelled');
CREATE TYPE shared.attachment_kind     AS ENUM ('photo', 'document');
CREATE TYPE shared.import_status       AS ENUM ('pending', 'processing', 'completed', 'failed');
CREATE TYPE shared.audit_action        AS ENUM ('create', 'update', 'delete');

-- Bank fixed-asset (PRD v1.1) enums.
CREATE TYPE shared.asset_class          AS ENUM ('tangible', 'intangible');
CREATE TYPE shared.depreciation_basis   AS ENUM ('commercial', 'fiscal');
CREATE TYPE shared.fiscal_asset_group   AS ENUM ('kelompok_1', 'kelompok_2', 'kelompok_3', 'kelompok_4', 'bangunan_permanen', 'bangunan_non_permanen', 'non_susut');
CREATE TYPE shared.transfer_status      AS ENUM ('pending', 'approved', 'in_transit', 'received', 'rejected', 'cancelled');
CREATE TYPE shared.opname_session_status AS ENUM ('open', 'counting', 'reconciling', 'closed');
CREATE TYPE shared.opname_item_result   AS ENUM ('pending', 'found', 'not_found', 'damaged', 'misplaced');
CREATE TYPE shared.disposal_method      AS ENUM ('sale', 'auction', 'donation', 'write_off');
CREATE TYPE shared.approver_level       AS ENUM ('office', 'office_subtree', 'wilayah', 'pusat');
CREATE TYPE shared.asset_document_type  AS ENUM ('bast_acquisition', 'bast_transfer', 'bast_disposal', 'invoice', 'contract', 'other');
