-- Office hierarchy (Pusat -> Wilayah -> Cabang -> Outlet), physical locations,
-- and employees. See docs/DATABASE.md §4.3. Also wires the deferred FKs on identity.users.

CREATE TABLE masterdata.offices (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  parent_id      uuid REFERENCES masterdata.offices (id),
  office_type_id uuid NOT NULL REFERENCES masterdata.office_types (id),
  province_id    uuid REFERENCES masterdata.provinces (id),
  city_id        uuid REFERENCES masterdata.cities (id),
  name           text NOT NULL,
  code           text NOT NULL,
  cost_center_code text,
  address        text,
  is_active      boolean NOT NULL DEFAULT true,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);
CREATE UNIQUE INDEX uq_offices_code ON masterdata.offices (code) WHERE deleted_at IS NULL;
CREATE INDEX idx_offices_parent_id ON masterdata.offices (parent_id);
CREATE INDEX idx_offices_type_id ON masterdata.offices (office_type_id);
CREATE INDEX idx_offices_province_id ON masterdata.offices (province_id);
CREATE INDEX idx_offices_city_id ON masterdata.offices (city_id);
CREATE TRIGGER trg_offices_set_updated BEFORE UPDATE ON masterdata.offices
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.floors (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  office_id  uuid NOT NULL REFERENCES masterdata.offices (id),
  name       text NOT NULL,
  level      int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_floors_office_name ON masterdata.floors (office_id, name) WHERE deleted_at IS NULL;
CREATE INDEX idx_floors_office_id ON masterdata.floors (office_id);
CREATE TRIGGER trg_floors_set_updated BEFORE UPDATE ON masterdata.floors
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.rooms (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  floor_id   uuid NOT NULL REFERENCES masterdata.floors (id),
  name       text NOT NULL,
  code       text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE UNIQUE INDEX uq_rooms_floor_name ON masterdata.rooms (floor_id, name) WHERE deleted_at IS NULL;
CREATE INDEX idx_rooms_floor_id ON masterdata.rooms (floor_id);
CREATE TRIGGER trg_rooms_set_updated BEFORE UPDATE ON masterdata.rooms
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE TABLE masterdata.employees (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code          text NOT NULL,
  name          text NOT NULL,
  email         text,
  avatar_key    text,
  department_id uuid REFERENCES masterdata.departments (id),
  position_id   uuid REFERENCES masterdata.positions (id),
  office_id     uuid NOT NULL REFERENCES masterdata.offices (id),
  status        shared.user_status NOT NULL DEFAULT 'active',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz
);
CREATE UNIQUE INDEX uq_employees_code ON masterdata.employees (code) WHERE deleted_at IS NULL;
CREATE INDEX idx_employees_office_id ON masterdata.employees (office_id);
CREATE INDEX idx_employees_department_id ON masterdata.employees (department_id);
CREATE INDEX idx_employees_position_id ON masterdata.employees (position_id);
CREATE TRIGGER trg_employees_set_updated BEFORE UPDATE ON masterdata.employees
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

-- Wire the deferred FKs from identity.users (created in phase 2 without these targets).
ALTER TABLE identity.users
  ADD CONSTRAINT fk_users_employee FOREIGN KEY (employee_id) REFERENCES masterdata.employees (id);
ALTER TABLE identity.users
  ADD CONSTRAINT fk_users_office FOREIGN KEY (office_id) REFERENCES masterdata.offices (id);
