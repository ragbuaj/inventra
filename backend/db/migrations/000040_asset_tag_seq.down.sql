-- Kembalikan tabel counter per-(kantor,kategori,tahun).
CREATE TABLE asset.asset_tag_counters (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  office_id   uuid NOT NULL REFERENCES masterdata.offices (id),
  category_id uuid NOT NULL REFERENCES masterdata.categories (id),
  year        int NOT NULL,
  last_seq    int NOT NULL DEFAULT 0,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_asset_tag_counters ON asset.asset_tag_counters (office_id, category_id, year);
CREATE INDEX idx_atc_office ON asset.asset_tag_counters (office_id);
CREATE INDEX idx_atc_category ON asset.asset_tag_counters (category_id);
CREATE TRIGGER trg_asset_tag_counters_set_updated BEFORE UPDATE ON asset.asset_tag_counters
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

DROP INDEX IF EXISTS asset.idx_assets_office_tagseq;
ALTER TABLE asset.assets DROP COLUMN tag_seq;
-- CATATAN: string asset_tag TIDAK dikembalikan ke format lama (re-tag tak reversibel).
