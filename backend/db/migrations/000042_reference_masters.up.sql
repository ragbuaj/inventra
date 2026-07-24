-- Master baru (spec 2026-07-23 legacy-parity, Fase 4):
--   office_classes          — kelas kantor (datar → generic reference engine)
--   executor_divisions      — divisi pelaksana (datar) + seed 5 nilai
--   companies               — perusahaan pegawai (datar)
--   building_classifications — klasifikasi gedung (numerik min/max lantai;
--                              engine diberi typeInt agar tetap deklaratif)

CREATE TABLE masterdata.office_classes (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_office_classes_name ON masterdata.office_classes (name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_office_classes_set_updated BEFORE UPDATE ON masterdata.office_classes
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.executor_divisions (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_executor_divisions_name ON masterdata.executor_divisions (name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_executor_divisions_set_updated BEFORE UPDATE ON masterdata.executor_divisions
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
INSERT INTO masterdata.executor_divisions (name) VALUES
  ('Engineering'), ('Security'), ('Housekeeping'), ('Parkir'), ('Operator');

CREATE TABLE masterdata.companies (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_companies_name ON masterdata.companies (name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_companies_set_updated BEFORE UPDATE ON masterdata.companies
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- max_floors NULL = tak terbatas (opsi "25+").
CREATE TABLE masterdata.building_classifications (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  min_floors int NOT NULL,
  max_floors int,
  is_active  boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT chk_bldg_floor_range CHECK (max_floors IS NULL OR max_floors >= min_floors)
);
CREATE UNIQUE INDEX uq_building_classifications_name ON masterdata.building_classifications (name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_building_classifications_set_updated BEFORE UPDATE ON masterdata.building_classifications
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
