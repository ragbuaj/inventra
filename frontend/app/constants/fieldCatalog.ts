// Frontend catalog of the (entity, field) pairs the backend actually field-masks
// via authz FilterView. Field keys are the real serialization keys from the
// backend's response maps (assetToMap / userToMap) — rules on any other key would
// have no effect. Entity-agnostic: the screen renders whatever is listed here, so
// adding an entity later is a constant edit + a one-line FilterView call in that
// entity's handler.
export interface CellRule { view: boolean, edit: boolean }
export interface CatalogEntity { entity: string, fields: string[] }

export const FIELD_CATALOG: CatalogEntity[] = [
  {
    entity: 'assets',
    fields: [
      'name', 'category_id', 'office_id', 'serial_number', 'purchase_date',
      'purchase_cost', 'book_value', 'accumulated_depreciation', 'salvage_value', 'impairment_loss',
      'depreciation_method', 'po_number', 'funding_source', 'warranty_expiry', 'status', 'notes'
    ]
  },
  {
    entity: 'users',
    fields: ['name', 'email', 'role_id', 'office_id', 'employee_id', 'status']
  }
]
