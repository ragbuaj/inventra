-- Vendor master data (masterdata.vendors). Vendors are managed via the generic
-- reference engine; this file holds only bespoke queries needed by other
-- modules.

-- name: ListVendorsLookup :many
-- Flat vendor lookup (id, name) for the asset importer. Vendors are not
-- office-scoped, so the full non-deleted set is returned.
SELECT id, name FROM masterdata.vendors
WHERE deleted_at IS NULL
ORDER BY name;
