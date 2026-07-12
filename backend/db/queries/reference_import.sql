-- Dedicated queries for the reference-target bulk importer (provinces, cities).
-- These exist alongside the generic reference engine (internal/masterdata/reference/engine.go)
-- because the engine operates on a *pgxpool.Pool directly and cannot join the
-- import worker's sqlc transaction (target.Execute receives a tx-bound
-- *sqlc.Queries). See internal/masterdata/reference/importer.go.

-- name: CreateProvince :one
INSERT INTO masterdata.provinces (name, code) VALUES ($1, $2)
RETURNING *;

-- name: GetProvinceByCode :one
-- Fresh, side-effect-free existence check used by the reference importer's
-- Execute anti-poisoning pre-check for provinces (mirrors GetEmployeeByCode /
-- GetOfficeByCode).
SELECT * FROM masterdata.provinces WHERE code = $1 AND deleted_at IS NULL LIMIT 1;

-- name: ListProvincesLookup :many
-- Flat id/name/code lookup for the cities importer's "provinsi" column
-- (matched by name OR code, case-insensitive).
SELECT id, name, code FROM masterdata.provinces WHERE deleted_at IS NULL;

-- name: CreateCity :one
INSERT INTO masterdata.cities (province_id, name, code) VALUES ($1, $2, $3)
RETURNING *;

-- name: GetCityByCode :one
-- Fresh, side-effect-free existence check used by the reference importer's
-- Execute anti-poisoning pre-check for cities (cities.code IS uniquely
-- constrained — uq_cities_code — so this pre-check is required, mirroring
-- GetProvinceByCode).
SELECT * FROM masterdata.cities WHERE code = $1 AND deleted_at IS NULL LIMIT 1;

-- name: ListCityCodes :many
-- Existing (non-deleted) city codes for the cities importer's validate-time
-- dupKode check (mirrors ListProvincesLookup's existingCodes use for
-- provinces) — cities.code IS uniquely constrained (uq_cities_code), so a
-- match here is authoritative, not just an in-file check.
SELECT code FROM masterdata.cities WHERE code IS NOT NULL AND deleted_at IS NULL;

-- name: ListDepartmentsLookup :many
-- id/name/code lookup for the employee importer's optional "departemen" column
-- (matched by name OR code, case-insensitive).
SELECT id, name, code FROM masterdata.departments WHERE deleted_at IS NULL;

-- name: ListPositionsLookup :many
-- id/name lookup for the employee importer's optional "jabatan" column
-- (matched by name, case-insensitive). positions has no code column.
SELECT id, name FROM masterdata.positions WHERE deleted_at IS NULL;

-- name: ListBrandsLookup :many
-- id/name lookup: brand-name dedup (brands importer) AND "merek" resolution
-- (models importer). brands.name IS uniquely constrained (uq_brands_name).
SELECT id, name FROM masterdata.brands WHERE deleted_at IS NULL;

-- name: GetBrandByName :one
-- Side-effect-free existence check for the brands importer's Execute
-- anti-poisoning pre-check (uq_brands_name; matched case-insensitively).
SELECT * FROM masterdata.brands WHERE lower(name) = lower($1) AND deleted_at IS NULL LIMIT 1;

-- name: CreateBrand :one
INSERT INTO masterdata.brands (name) VALUES ($1) RETURNING *;

-- name: ListUnitNames :many
-- Existing (non-deleted) unit names for the units importer's dupNama check
-- (uq_units_name).
SELECT name FROM masterdata.units WHERE deleted_at IS NULL;

-- name: GetUnitByName :one
SELECT * FROM masterdata.units WHERE lower(name) = lower($1) AND deleted_at IS NULL LIMIT 1;

-- name: CreateUnit :one
INSERT INTO masterdata.units (name, symbol) VALUES ($1, $2) RETURNING *;

-- name: ListModelsLookup :many
-- (brand_id, name) pairs for the models importer's composite dupNama check
-- (uq_models_brand_name).
SELECT brand_id, name FROM masterdata.models WHERE deleted_at IS NULL;

-- name: GetModelByBrandAndName :one
SELECT * FROM masterdata.models
WHERE brand_id = $1 AND lower(name) = lower($2) AND deleted_at IS NULL LIMIT 1;

-- name: CreateModel :one
INSERT INTO masterdata.models (brand_id, name) VALUES ($1, $2) RETURNING *;
