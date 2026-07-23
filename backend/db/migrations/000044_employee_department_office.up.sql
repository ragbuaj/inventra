-- Pegawai + divisi per-kantor (spec 2026-07-23 legacy-parity, Fase 6).
ALTER TABLE masterdata.employees
  ADD COLUMN company_id           uuid REFERENCES masterdata.companies (id),
  ADD COLUMN executor_division_id uuid REFERENCES masterdata.executor_divisions (id);
CREATE INDEX idx_employees_company ON masterdata.employees (company_id);
CREATE INDEX idx_employees_exec_div ON masterdata.employees (executor_division_id);

-- Divisi kantor per-kantor: departemen dapat office_id. NULLABLE dulu (wajib
-- ditegakkan di app layer; DB NOT NULL menyusul setelah data departemen lama
-- disiapkan per-kantor). Keunikan code jadi per-kantor.
ALTER TABLE masterdata.departments ADD COLUMN office_id uuid REFERENCES masterdata.offices (id);
CREATE INDEX idx_departments_office ON masterdata.departments (office_id);
DROP INDEX IF EXISTS masterdata.uq_departments_code;
CREATE UNIQUE INDEX uq_departments_office_code ON masterdata.departments (office_id, code)
  WHERE deleted_at IS NULL AND code IS NOT NULL;
