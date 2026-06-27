-- Master data — reference & geography tables in the `masterdata` schema.
-- See docs/DATABASE.md §4.2. Convention: created_at/updated_at/deleted_at + soft delete;
-- UNIQUE are partial (WHERE deleted_at IS NULL [AND <col> IS NOT NULL] for nullable codes).

CREATE SCHEMA IF NOT EXISTS masterdata;

-- Geography ------------------------------------------------------------------
CREATE TABLE masterdata.provinces (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  code       text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_provinces_code ON masterdata.provinces (code) WHERE deleted_at IS NULL AND code IS NOT NULL;
CREATE TRIGGER trg_provinces_set_updated BEFORE UPDATE ON masterdata.provinces
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.cities (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  province_id uuid NOT NULL REFERENCES masterdata.provinces (id),
  name        text NOT NULL,
  code        text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE UNIQUE INDEX uq_cities_code ON masterdata.cities (code) WHERE deleted_at IS NULL AND code IS NOT NULL;
CREATE INDEX idx_cities_province_id ON masterdata.cities (province_id);
CREATE TRIGGER trg_cities_set_updated BEFORE UPDATE ON masterdata.cities
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Organisational reference ---------------------------------------------------
CREATE TABLE masterdata.office_types (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_office_types_name ON masterdata.office_types (name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_office_types_set_updated BEFORE UPDATE ON masterdata.office_types
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.departments (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  code       text,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_departments_code ON masterdata.departments (code) WHERE deleted_at IS NULL AND code IS NOT NULL;
CREATE TRIGGER trg_departments_set_updated BEFORE UPDATE ON masterdata.departments
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.positions (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_positions_name ON masterdata.positions (name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_positions_set_updated BEFORE UPDATE ON masterdata.positions
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.vendors (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name         text NOT NULL,
  contact_name text,
  phone        text,
  email        text,
  address      text,
  is_active    boolean NOT NULL DEFAULT true,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz
);
CREATE INDEX idx_vendors_name ON masterdata.vendors (name);
CREATE TRIGGER trg_vendors_set_updated BEFORE UPDATE ON masterdata.vendors
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Asset reference ------------------------------------------------------------
CREATE TABLE masterdata.brands (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_brands_name ON masterdata.brands (name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_brands_set_updated BEFORE UPDATE ON masterdata.brands
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.models (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id   uuid NOT NULL REFERENCES masterdata.brands (id),
  name       text NOT NULL,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_models_brand_name ON masterdata.models (brand_id, name) WHERE deleted_at IS NULL;
CREATE INDEX idx_models_brand_id ON masterdata.models (brand_id);
CREATE TRIGGER trg_models_set_updated BEFORE UPDATE ON masterdata.models
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.categories (
  id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name                        text NOT NULL,
  code                        text,
  parent_id                   uuid REFERENCES masterdata.categories (id),
  default_depreciation_method shared.depreciation_method,
  default_useful_life_months  int,
  default_salvage_rate        numeric(5,4),
  -- Bank fixed-asset (PRD v1.1): accounting/tax defaults.
  asset_class                 shared.asset_class NOT NULL DEFAULT 'tangible',
  default_fiscal_group        shared.fiscal_asset_group,
  default_fiscal_life_months  int,
  gl_account_code             text,
  capitalization_threshold    numeric(18,2),
  is_active                   boolean NOT NULL DEFAULT true,
  created_at                  timestamptz NOT NULL DEFAULT now(),
  updated_at                  timestamptz NOT NULL DEFAULT now(),
  deleted_at                  timestamptz
);
CREATE UNIQUE INDEX uq_categories_code ON masterdata.categories (code) WHERE deleted_at IS NULL AND code IS NOT NULL;
CREATE INDEX idx_categories_parent_id ON masterdata.categories (parent_id);
CREATE TRIGGER trg_categories_set_updated BEFORE UPDATE ON masterdata.categories
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.maintenance_categories (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_maintenance_categories_name ON masterdata.maintenance_categories (name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_maintenance_categories_set_updated BEFORE UPDATE ON masterdata.maintenance_categories
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.problem_categories (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_problem_categories_name ON masterdata.problem_categories (name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_problem_categories_set_updated BEFORE UPDATE ON masterdata.problem_categories
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.units (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  symbol     text,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_units_name ON masterdata.units (name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_units_set_updated BEFORE UPDATE ON masterdata.units
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
