-- Approval / maker-checker — `approval` schema. See docs/DATABASE.md §4.5 and PRD §3.6.

CREATE SCHEMA IF NOT EXISTS approval;

CREATE TABLE approval.requests (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  type            shared.request_type NOT NULL,
  office_id       uuid REFERENCES masterdata.offices (id),
  target_entity   text,
  target_id       uuid,
  payload         jsonb NOT NULL DEFAULT '{}',
  reason          text,
  status          shared.request_status NOT NULL DEFAULT 'pending',
  requested_by_id uuid NOT NULL REFERENCES identity.users (id),
  decided_by_id   uuid REFERENCES identity.users (id),
  decision_note   text,
  decided_at      timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,
  -- Segregation of duty: a maker cannot approve their own request.
  CONSTRAINT chk_requests_sod CHECK (decided_by_id IS NULL OR decided_by_id <> requested_by_id)
);
CREATE INDEX idx_requests_status_type ON approval.requests (status, type);
CREATE INDEX idx_requests_office_id ON approval.requests (office_id);
CREATE INDEX idx_requests_requester ON approval.requests (requested_by_id);
CREATE INDEX idx_requests_decided_by ON approval.requests (decided_by_id);
CREATE INDEX idx_requests_target ON approval.requests (target_entity, target_id);
CREATE TRIGGER trg_requests_set_updated BEFORE UPDATE ON approval.requests
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
