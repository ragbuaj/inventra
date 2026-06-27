-- Bank fixed-asset (PRD v1.1) operational tables: inter-office transfer (mutasi),
-- stock opname (physical inventory), disposal, and BAST/documents. These reference
-- asset.assets + approval.requests (created earlier) and each other, so they live in
-- their own migration after those exist. See docs/DATABASE.md §4.5b and docs/ERD.md.

-- Mutasi aset antar-kantor (PRD §3.8) -----------------------------------------
CREATE SCHEMA IF NOT EXISTS transfer;

CREATE TABLE transfer.asset_transfers (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id        uuid NOT NULL REFERENCES asset.assets (id),
  from_office_id  uuid NOT NULL REFERENCES masterdata.offices (id),
  to_office_id    uuid NOT NULL REFERENCES masterdata.offices (id),
  to_room_id      uuid REFERENCES masterdata.rooms (id),
  status          shared.transfer_status NOT NULL DEFAULT 'pending',
  reason          text,
  requested_by_id uuid NOT NULL REFERENCES identity.users (id),
  approved_by_id  uuid REFERENCES identity.users (id),
  shipped_date    date,
  received_date   date,
  received_by_id  uuid REFERENCES identity.users (id),
  bast_no         text,
  request_id      uuid REFERENCES approval.requests (id),
  notes           text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);
CREATE INDEX idx_transfer_asset ON transfer.asset_transfers (asset_id);
CREATE INDEX idx_transfer_from ON transfer.asset_transfers (from_office_id);
CREATE INDEX idx_transfer_to ON transfer.asset_transfers (to_office_id);
CREATE INDEX idx_transfer_status ON transfer.asset_transfers (status);
CREATE TRIGGER trg_asset_transfers_set_updated BEFORE UPDATE ON transfer.asset_transfers
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Penghapusan/pelepasan aset (PRD §3.6/§5) ------------------------------------
CREATE SCHEMA IF NOT EXISTS disposal;

CREATE TABLE disposal.disposals (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id               uuid NOT NULL REFERENCES asset.assets (id),
  method                 shared.disposal_method NOT NULL,
  disposal_date          date NOT NULL,
  proceeds               numeric(18,2),
  book_value_at_disposal numeric(18,2),
  gain_loss              numeric(18,2),
  bast_no                text,
  approved_by_id         uuid REFERENCES identity.users (id),
  request_id             uuid REFERENCES approval.requests (id),
  created_by_id          uuid REFERENCES identity.users (id),
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  deleted_at             timestamptz
);
CREATE UNIQUE INDEX uq_disposals_asset ON disposal.disposals (asset_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_disposal_date ON disposal.disposals (disposal_date);
CREATE TRIGGER trg_disposals_set_updated BEFORE UPDATE ON disposal.disposals
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Stock opname / inventarisasi fisik (PRD §3.9) -------------------------------
CREATE SCHEMA IF NOT EXISTS stockopname;

CREATE TABLE stockopname.stock_opname_sessions (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  office_id     uuid NOT NULL REFERENCES masterdata.offices (id),
  name          text,
  period        date NOT NULL,
  status        shared.opname_session_status NOT NULL DEFAULT 'open',
  started_by_id uuid NOT NULL REFERENCES identity.users (id),
  started_at    timestamptz NOT NULL DEFAULT now(),
  closed_by_id  uuid REFERENCES identity.users (id),
  closed_at     timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz
);
CREATE INDEX idx_opname_office ON stockopname.stock_opname_sessions (office_id);
CREATE INDEX idx_opname_status ON stockopname.stock_opname_sessions (status);
CREATE TRIGGER trg_opname_sessions_set_updated BEFORE UPDATE ON stockopname.stock_opname_sessions
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE stockopname.stock_opname_items (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id    uuid NOT NULL REFERENCES stockopname.stock_opname_sessions (id) ON DELETE CASCADE,
  asset_id      uuid NOT NULL REFERENCES asset.assets (id),
  expected      boolean NOT NULL DEFAULT true,
  result        shared.opname_item_result NOT NULL DEFAULT 'pending',
  counted_by_id uuid REFERENCES identity.users (id),
  counted_at    timestamptz,
  note          text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz
);
CREATE UNIQUE INDEX uq_opnitem_session_asset ON stockopname.stock_opname_items (session_id, asset_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_opnitem_session ON stockopname.stock_opname_items (session_id);
CREATE INDEX idx_opnitem_result ON stockopname.stock_opname_items (result);
CREATE TRIGGER trg_opname_items_set_updated BEFORE UPDATE ON stockopname.stock_opname_items
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- BAST & dokumen resmi (PRD §3.10) — references requests/transfers/disposals ---
CREATE TABLE asset.asset_documents (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id            uuid NOT NULL REFERENCES asset.assets (id) ON DELETE CASCADE,
  doc_type            shared.asset_document_type NOT NULL,
  doc_no              text,
  doc_date            date,
  counterparty        text,
  object_key          text,
  related_request_id  uuid REFERENCES approval.requests (id),
  related_transfer_id uuid REFERENCES transfer.asset_transfers (id),
  related_disposal_id uuid REFERENCES disposal.disposals (id),
  created_by_id       uuid REFERENCES identity.users (id),
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz
);
CREATE INDEX idx_assetdoc_asset ON asset.asset_documents (asset_id);
CREATE INDEX idx_assetdoc_type ON asset.asset_documents (doc_type);
CREATE TRIGGER trg_asset_documents_set_updated BEFORE UPDATE ON asset.asset_documents
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
