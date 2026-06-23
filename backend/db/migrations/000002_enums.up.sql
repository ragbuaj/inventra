-- Extensions, shared trigger function, and enum types.
-- See docs/DATABASE.md §1 (konvensi) and §2 (tipe enum).

CREATE EXTENSION IF NOT EXISTS citext;

-- Shared trigger function: keeps updated_at current on every UPDATE.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Enum types. NOTE: role is NOT an enum (it is the configurable `roles` table).
CREATE TYPE user_status         AS ENUM ('active', 'inactive', 'suspended');
CREATE TYPE scope_level         AS ENUM ('global', 'office_subtree', 'office', 'own');
CREATE TYPE asset_status        AS ENUM ('available', 'assigned', 'under_maintenance', 'retired', 'lost');
CREATE TYPE depreciation_method AS ENUM ('straight_line', 'declining_balance');
CREATE TYPE assignment_status   AS ENUM ('active', 'returned');
CREATE TYPE maintenance_type    AS ENUM ('preventive', 'corrective');
CREATE TYPE maintenance_status  AS ENUM ('scheduled', 'in_progress', 'completed', 'cancelled');
CREATE TYPE request_type        AS ENUM ('asset_create', 'asset_delete', 'assignment', 'maintenance', 'valuation_exclusion');
CREATE TYPE request_status      AS ENUM ('pending', 'approved', 'rejected', 'cancelled');
CREATE TYPE attachment_kind     AS ENUM ('photo', 'document');
CREATE TYPE import_status       AS ENUM ('pending', 'processing', 'completed', 'failed');
CREATE TYPE audit_action        AS ENUM ('create', 'update', 'delete');
