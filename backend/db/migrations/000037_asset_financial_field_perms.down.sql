-- Kembalikan accumulated_depreciation menjadi masked untuk Manager (kondisi 000016).
UPDATE identity.field_permissions
SET can_view = false
WHERE entity = 'assets'
  AND field = 'accumulated_depreciation'
  AND deleted_at IS NULL
  AND role_id IN (SELECT id FROM identity.roles WHERE name = 'Manager' AND deleted_at IS NULL);
