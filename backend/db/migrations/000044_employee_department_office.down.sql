DROP INDEX IF EXISTS masterdata.uq_departments_office_code;
CREATE UNIQUE INDEX uq_departments_code ON masterdata.departments (code) WHERE deleted_at IS NULL AND code IS NOT NULL;
DROP INDEX IF EXISTS masterdata.idx_departments_office;
ALTER TABLE masterdata.departments DROP COLUMN office_id;

DROP INDEX IF EXISTS masterdata.idx_employees_exec_div;
DROP INDEX IF EXISTS masterdata.idx_employees_company;
ALTER TABLE masterdata.employees
  DROP COLUMN executor_division_id,
  DROP COLUMN company_id;
