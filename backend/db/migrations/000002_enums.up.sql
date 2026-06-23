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
CREATE TYPE shared.asset_status        AS ENUM ('available', 'assigned', 'under_maintenance', 'retired', 'lost');
CREATE TYPE shared.depreciation_method AS ENUM ('straight_line', 'declining_balance');
CREATE TYPE shared.assignment_status   AS ENUM ('active', 'returned');
CREATE TYPE shared.maintenance_type    AS ENUM ('preventive', 'corrective');
CREATE TYPE shared.maintenance_status  AS ENUM ('scheduled', 'in_progress', 'completed', 'cancelled');
CREATE TYPE shared.request_type        AS ENUM ('asset_create', 'asset_delete', 'assignment', 'maintenance', 'valuation_exclusion');
CREATE TYPE shared.request_status      AS ENUM ('pending', 'approved', 'rejected', 'cancelled');
CREATE TYPE shared.attachment_kind     AS ENUM ('photo', 'document');
CREATE TYPE shared.import_status       AS ENUM ('pending', 'processing', 'completed', 'failed');
CREATE TYPE shared.audit_action        AS ENUM ('create', 'update', 'delete');
