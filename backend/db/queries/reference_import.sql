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
