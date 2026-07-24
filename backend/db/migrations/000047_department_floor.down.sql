DROP INDEX IF EXISTS masterdata.idx_departments_floor;
ALTER TABLE masterdata.departments DROP COLUMN IF EXISTS floor_id;
