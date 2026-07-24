-- Kolom kantor (spec 2026-07-23 legacy-parity, Fase 5):
--   ownership_status           — status kepemilikan (enum shared.office_ownership)
--   office_class_id            — kelas kantor (FK masterdata.office_classes)
--   building_classification_id — klasifikasi gedung (FK masterdata.building_classifications)
--   floor_count                — jumlah lantai (memicu saran klasifikasi di UI)
--   building_area              — luas bangunan (m2)
--   office_kind                — konvensional/syariah (enum, default konvensional)
--   description                — deskripsi
--   head_employee_id           — kepala kantor (FK masterdata.employees)
--   contact                    — kontak
ALTER TABLE masterdata.offices
  ADD COLUMN ownership_status           shared.office_ownership,
  ADD COLUMN office_class_id            uuid REFERENCES masterdata.office_classes (id),
  ADD COLUMN building_classification_id uuid REFERENCES masterdata.building_classifications (id),
  ADD COLUMN floor_count                int,
  ADD COLUMN building_area              numeric(12,2),
  ADD COLUMN office_kind                shared.office_kind NOT NULL DEFAULT 'konvensional',
  ADD COLUMN description                text,
  ADD COLUMN head_employee_id           uuid REFERENCES masterdata.employees (id),
  ADD COLUMN contact                    text;
CREATE INDEX idx_offices_class_id ON masterdata.offices (office_class_id);
CREATE INDEX idx_offices_bldg_class_id ON masterdata.offices (building_classification_id);
CREATE INDEX idx_offices_head_employee ON masterdata.offices (head_employee_id);
