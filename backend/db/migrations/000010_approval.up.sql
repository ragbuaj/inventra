-- Approval / maker-checker — `approval` schema. See docs/DATABASE.md §4.5 and PRD §3.6.

CREATE SCHEMA IF NOT EXISTS approval;

CREATE TABLE approval.requests (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  type            shared.request_type NOT NULL,
  office_id       uuid REFERENCES masterdata.offices (id),
  -- Bank fixed-asset (PRD v1.1): value-tiered approval routing.
  amount          numeric(18,2),
  current_step    int NOT NULL DEFAULT 1,
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

-- Value-tiered approval limits (PRD v1.1 §2.4) — configurable bands per request type.
CREATE TABLE approval.approval_thresholds (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  request_type   shared.request_type NOT NULL,
  amount_from    numeric(18,2) NOT NULL DEFAULT 0,
  amount_to      numeric(18,2),
  required_level shared.approver_level NOT NULL,
  step_order     int NOT NULL DEFAULT 1,
  is_active      boolean NOT NULL DEFAULT true,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);
CREATE UNIQUE INDEX uq_apprthr_type_from_step ON approval.approval_thresholds (request_type, amount_from, step_order) WHERE deleted_at IS NULL;
CREATE INDEX idx_apprthr_type ON approval.approval_thresholds (request_type);
CREATE TRIGGER trg_apprthr_set_updated BEFORE UPDATE ON approval.approval_thresholds
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Per-step audit trail of a request's multi-level approval chain (PRD v1.1 §3.6).
CREATE TABLE approval.request_approvals (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id     uuid NOT NULL REFERENCES approval.requests (id) ON DELETE CASCADE,
  step_order     int NOT NULL,
  required_level shared.approver_level NOT NULL,
  approver_id    uuid REFERENCES identity.users (id),
  decision       shared.request_status NOT NULL DEFAULT 'pending',
  note           text,
  decided_at     timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);
CREATE UNIQUE INDEX uq_reqappr_request_step ON approval.request_approvals (request_id, step_order) WHERE deleted_at IS NULL;
CREATE INDEX idx_reqappr_request ON approval.request_approvals (request_id);
CREATE INDEX idx_reqappr_approver ON approval.request_approvals (approver_id);
CREATE TRIGGER trg_reqappr_set_updated BEFORE UPDATE ON approval.request_approvals
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
