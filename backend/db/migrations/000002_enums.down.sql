-- Drop module schemas (CASCADE removes any remaining objects, enums, function).
DROP SCHEMA IF EXISTS audit CASCADE;
DROP SCHEMA IF EXISTS identity CASCADE;
DROP SCHEMA IF EXISTS shared CASCADE;

DROP EXTENSION IF EXISTS citext;
