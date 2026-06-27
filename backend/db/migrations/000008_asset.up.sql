-- Asset core — `asset` schema. See docs/DATABASE.md §4.4 and §4.7.
-- Convention: created_at/updated_at/deleted_at + soft delete; UNIQUE partial.

CREATE SCHEMA IF NOT EXISTS asset;

CREATE TABLE asset.assets (
  id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_tag                   text NOT NULL,
  name                        text NOT NULL,
  category_id                 uuid NOT NULL REFERENCES masterdata.categories (id),
  brand_id                    uuid REFERENCES masterdata.brands (id),
  model_id                    uuid REFERENCES masterdata.models (id),
  -- room_id is nullable: intangible assets (PRD v1.1) have no physical room.
  room_id                     uuid REFERENCES masterdata.rooms (id),
  office_id                   uuid NOT NULL REFERENCES masterdata.offices (id),
  unit_id                     uuid REFERENCES masterdata.units (id),
  status                      shared.asset_status NOT NULL DEFAULT 'available',
  serial_number               text,
  purchase_date               date,
  purchase_cost               numeric(18,2),
  vendor_id                   uuid REFERENCES masterdata.vendors (id),
  po_number                   text,
  funding_source              text,
  warranty_expiry             date,
  specifications              jsonb NOT NULL DEFAULT '{}',
  -- Bank fixed-asset (PRD v1.1): class, capitalization, dual-basis depreciation, impairment, BAST.
  asset_class                 shared.asset_class NOT NULL DEFAULT 'tangible',
  capitalized                 boolean NOT NULL DEFAULT true,
  depreciation_method         shared.depreciation_method,
  useful_life_months          int,
  salvage_value               numeric(18,2),
  fiscal_group                shared.fiscal_asset_group,
  fiscal_life_months          int,
  accumulated_depreciation    numeric(18,2) NOT NULL DEFAULT 0,
  book_value                  numeric(18,2),
  impairment_loss             numeric(18,2),
  acquisition_bast_no         text,
  current_holder_employee_id  uuid REFERENCES masterdata.employees (id),
  excluded_from_valuation     boolean NOT NULL DEFAULT false,
  valuation_exclusion_reason  text,
  created_by_id               uuid REFERENCES identity.users (id),
  notes                       text,
  created_at                  timestamptz NOT NULL DEFAULT now(),
  updated_at                  timestamptz NOT NULL DEFAULT now(),
  deleted_at                  timestamptz,
  -- Tangible assets must have a physical location; intangible assets must not require one.
  CONSTRAINT chk_assets_tangible_room CHECK (asset_class = 'intangible' OR room_id IS NOT NULL)
);
CREATE UNIQUE INDEX uq_assets_asset_tag ON asset.assets (asset_tag) WHERE deleted_at IS NULL;
CREATE INDEX idx_assets_office_id ON asset.assets (office_id);
CREATE INDEX idx_assets_status ON asset.assets (status);
CREATE INDEX idx_assets_category_id ON asset.assets (category_id);
CREATE INDEX idx_assets_room_id ON asset.assets (room_id);
CREATE INDEX idx_assets_brand_id ON asset.assets (brand_id);
CREATE INDEX idx_assets_model_id ON asset.assets (model_id);
CREATE INDEX idx_assets_vendor_id ON asset.assets (vendor_id);
CREATE INDEX idx_assets_unit_id ON asset.assets (unit_id);
CREATE INDEX idx_assets_holder ON asset.assets (current_holder_employee_id);
CREATE INDEX idx_assets_created_by ON asset.assets (created_by_id);
CREATE INDEX idx_assets_class ON asset.assets (asset_class);
CREATE TRIGGER trg_assets_set_updated BEFORE UPDATE ON asset.assets
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE asset.asset_attachments (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id          uuid NOT NULL REFERENCES asset.assets (id),
  kind              shared.attachment_kind NOT NULL,
  object_key        text NOT NULL,
  thumbnail_key     text,
  original_filename text NOT NULL,
  size_bytes        bigint NOT NULL,
  mime_type         text NOT NULL,
  created_by_id     uuid REFERENCES identity.users (id),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz
);
CREATE INDEX idx_attachments_asset_id ON asset.asset_attachments (asset_id);
CREATE INDEX idx_attachments_created_by ON asset.asset_attachments (created_by_id);
CREATE TRIGGER trg_asset_attachments_set_updated BEFORE UPDATE ON asset.asset_attachments
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- asset_tag sequence counter (helper; exempt from soft delete). See §4.7.
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
