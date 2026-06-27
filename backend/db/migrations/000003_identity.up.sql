-- Identity & Authorization tables in the `identity` schema. See docs/DATABASE.md §4.1.
-- Convention: every table carries created_at, updated_at, deleted_at (soft delete);
-- all UNIQUE are partial (WHERE deleted_at IS NULL). Enums & set_updated_at live in `shared`.

-- Roles — configurable by superadmin; is_system marks the built-in roles.
CREATE TABLE identity.roles (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code        text NOT NULL,
  name        text NOT NULL,
  description text,
  is_system   boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE UNIQUE INDEX uq_roles_code ON identity.roles (code) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_roles_set_updated BEFORE UPDATE ON identity.roles
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Per-action RBAC (data-driven; replaces the hardcoded capability matrix).
CREATE TABLE identity.role_permissions (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  role_id        uuid NOT NULL REFERENCES identity.roles (id),
  permission_key text NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);
CREATE UNIQUE INDEX uq_role_permissions ON identity.role_permissions (role_id, permission_key) WHERE deleted_at IS NULL;
CREATE INDEX idx_role_permissions_role ON identity.role_permissions (role_id);
CREATE TRIGGER trg_role_permissions_set_updated BEFORE UPDATE ON identity.role_permissions
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Users (login accounts).
-- NOTE: FKs for employee_id -> masterdata.employees and office_id -> masterdata.offices
-- are added in the masterdata migration (phase 3), once those tables exist.
CREATE TABLE identity.users (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id   uuid,
  office_id     uuid,
  name          text NOT NULL,
  email         citext NOT NULL,
  password_hash text,
  google_id     text,
  avatar_url    text,
  role_id       uuid NOT NULL REFERENCES identity.roles (id),
  status        shared.user_status NOT NULL DEFAULT 'active',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz
);
CREATE UNIQUE INDEX uq_users_email ON identity.users (email) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_users_google_id ON identity.users (google_id) WHERE deleted_at IS NULL AND google_id IS NOT NULL;
CREATE INDEX idx_users_office_id ON identity.users (office_id);
CREATE INDEX idx_users_role_id ON identity.users (role_id);
CREATE INDEX idx_users_employee_id ON identity.users (employee_id);
CREATE TRIGGER trg_users_set_updated BEFORE UPDATE ON identity.users
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Field-level permissions (applies to all entities).
CREATE TABLE identity.field_permissions (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  entity     text NOT NULL,
  field      text NOT NULL,
  role_id    uuid NOT NULL REFERENCES identity.roles (id),
  can_view   boolean NOT NULL DEFAULT true,
  can_edit   boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_field_permissions ON identity.field_permissions (entity, field, role_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_field_permissions_role ON identity.field_permissions (role_id);
CREATE TRIGGER trg_field_permissions_set_updated BEFORE UPDATE ON identity.field_permissions
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Configurable data-scope policies (per role; module '*' = default, else per-module override).
CREATE TABLE identity.data_scope_policies (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  role_id     uuid NOT NULL REFERENCES identity.roles (id),
  module      text NOT NULL DEFAULT '*',
  scope_level shared.scope_level NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE UNIQUE INDEX uq_data_scope ON identity.data_scope_policies (role_id, module) WHERE deleted_at IS NULL;
CREATE INDEX idx_data_scope_role ON identity.data_scope_policies (role_id);
CREATE TRIGGER trg_data_scope_set_updated BEFORE UPDATE ON identity.data_scope_policies
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Global, superadmin-managed configuration (PRD v1.1): capitalization threshold default, etc.
CREATE TABLE identity.app_settings (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  key         text NOT NULL,
  value       text NOT NULL,
  value_type  text,
  description text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE UNIQUE INDEX uq_app_settings_key ON identity.app_settings (key) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_app_settings_set_updated BEFORE UPDATE ON identity.app_settings
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
