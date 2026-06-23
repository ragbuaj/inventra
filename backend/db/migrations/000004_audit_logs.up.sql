-- Audit trail covering every table (cross-cutting; built early). See docs/DATABASE.md §4.5.
-- Append-only: only created_at (no updated_at / deleted_at). Lives in the `audit` schema.
CREATE TABLE audit.audit_logs (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id    uuid REFERENCES identity.users (id),
  entity_type text NOT NULL,
  entity_id   uuid NOT NULL,
  action      shared.audit_action NOT NULL,
  changes     jsonb,
  ip          text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_entity ON audit.audit_logs (entity_type, entity_id);
CREATE INDEX idx_audit_actor ON audit.audit_logs (actor_id);
CREATE INDEX idx_audit_created_at ON audit.audit_logs (created_at);
