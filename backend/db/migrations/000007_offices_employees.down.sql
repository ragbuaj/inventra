-- Drop the deferred FKs on identity.users first, then the phase-3 office/employee tables.
ALTER TABLE identity.users DROP CONSTRAINT IF EXISTS fk_users_office;
ALTER TABLE identity.users DROP CONSTRAINT IF EXISTS fk_users_employee;

DROP TABLE IF EXISTS masterdata.employees;
DROP TABLE IF EXISTS masterdata.rooms;
DROP TABLE IF EXISTS masterdata.floors;
DROP TABLE IF EXISTS masterdata.offices;
