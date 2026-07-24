-- Divisi kantor (departemen) mendapat kolom lantai: departemen menempati satu
-- lantai pada kantornya. NULLABLE di DB (departemen legacy / global NULL-office
-- boleh tanpa lantai); wajib + difilter per kantor ditegakkan di UI master data.
-- Integritas "lantai harus milik kantor departemen" ditegakkan di service layer.
ALTER TABLE masterdata.departments
  ADD COLUMN floor_id uuid REFERENCES masterdata.floors (id);
CREATE INDEX idx_departments_floor ON masterdata.departments (floor_id);
