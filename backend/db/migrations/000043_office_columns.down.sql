DROP INDEX IF EXISTS masterdata.idx_offices_head_employee;
DROP INDEX IF EXISTS masterdata.idx_offices_bldg_class_id;
DROP INDEX IF EXISTS masterdata.idx_offices_class_id;
ALTER TABLE masterdata.offices
  DROP COLUMN contact,
  DROP COLUMN head_employee_id,
  DROP COLUMN description,
  DROP COLUMN office_kind,
  DROP COLUMN building_area,
  DROP COLUMN floor_count,
  DROP COLUMN building_classification_id,
  DROP COLUMN office_class_id,
  DROP COLUMN ownership_status;
