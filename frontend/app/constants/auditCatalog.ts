// The real entity_type values recorded by the backend's audit.Record(...) calls.
// Used to populate the Audit Trail entity-type filter dropdown.
export const AUDIT_ENTITY_TYPES = [
  'assets', 'users', 'roles', 'role_permissions', 'data_scope_policies', 'field_permissions',
  'offices', 'employees', 'categories', 'floors', 'rooms', 'requests',
  'asset_attachments', 'asset_documents'
] as const
