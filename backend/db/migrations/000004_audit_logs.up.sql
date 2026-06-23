-- Audit trail covering every table (cross-cutting; built early). See docs/DATABASE.md §4.5.
-- Append-only: only created_at (no updated_at / deleted_at).
CREATE TABLE audit_logs (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id    uuid REFERENCES users (id),
  entity_type text NOT NULL,
  entity_id   uuid NOT NULL,
  action      audit_action NOT NULL,
  changes     jsonb,
  ip          text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_entity ON audit_logs (entity_type, entity_id);
CREATE INDEX idx_audit_actor ON audit_logs (actor_id);
CREATE INDEX idx_audit_created_at ON audit_logs (created_at);
