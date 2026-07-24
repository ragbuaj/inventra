export type ReferenceKey
  = 'office-types' | 'departments' | 'positions' | 'units'
    | 'maintenance-categories' | 'problem-categories' | 'brands'
    | 'vendors' | 'provinces' | 'cities' | 'models'
    | 'office-classes' | 'executor-divisions' | 'companies' | 'building-classifications'

export type ReferenceFieldType = 'text' | 'fk' | 'select' | 'number' | 'office' | 'floor'

export interface ReferenceFieldOption {
  value: string
  labelKey: string
}

export interface ReferenceField {
  key: string
  labelKey: string
  placeholder?: string
  type?: ReferenceFieldType // default 'text'
  fkResource?: ReferenceKey // for type:'fk' — source resource for options + name resolution
  options?: ReferenceFieldOption[] // for type:'select' — static options
  required?: boolean
}

export interface ReferenceDescriptor {
  key: ReferenceKey
  labelKey: string
  hasActive: boolean // false for provinces & cities (no is_active column)
  fields: ReferenceField[]
}

const nameField: ReferenceField = { key: 'name', labelKey: 'masterdata.reference.fields.name' }
const codeField: ReferenceField = { key: 'code', labelKey: 'masterdata.reference.fields.code' }

export const referenceResources: ReferenceDescriptor[] = [
  { key: 'office-types', labelKey: 'masterdata.reference.resources.office-types', hasActive: true, fields: [
    nameField,
    { key: 'tier', labelKey: 'masterdata.reference.fields.tier', type: 'select', options: [
      { value: 'pusat', labelKey: 'map.tier.pusat' },
      { value: 'wilayah', labelKey: 'map.tier.wilayah' },
      { value: 'office', labelKey: 'map.tier.office' }
    ] }
  ] },
  { key: 'departments', labelKey: 'masterdata.reference.resources.departments', hasActive: true, fields: [
    nameField,
    codeField,
    { key: 'office_id', labelKey: 'masterdata.reference.fields.office', type: 'office', required: true },
    // Floor is required and its options are filtered to the selected office's floors.
    { key: 'floor_id', labelKey: 'masterdata.reference.fields.floor', type: 'floor', required: true }
  ] },
  { key: 'positions', labelKey: 'masterdata.reference.resources.positions', hasActive: true, fields: [nameField] },
  { key: 'units', labelKey: 'masterdata.reference.resources.units', hasActive: true, fields: [nameField, { key: 'symbol', labelKey: 'masterdata.reference.fields.symbol' }] },
  { key: 'maintenance-categories', labelKey: 'masterdata.reference.resources.maintenance-categories', hasActive: true, fields: [nameField] },
  { key: 'problem-categories', labelKey: 'masterdata.reference.resources.problem-categories', hasActive: true, fields: [nameField] },
  { key: 'brands', labelKey: 'masterdata.reference.resources.brands', hasActive: true, fields: [nameField] },
  { key: 'vendors', labelKey: 'masterdata.reference.resources.vendors', hasActive: true, fields: [
    nameField,
    { key: 'contact_name', labelKey: 'masterdata.reference.fields.contact_name' },
    { key: 'phone', labelKey: 'masterdata.reference.fields.phone' },
    { key: 'email', labelKey: 'masterdata.reference.fields.email' },
    { key: 'address', labelKey: 'masterdata.reference.fields.address' }
  ] },
  { key: 'provinces', labelKey: 'masterdata.reference.resources.provinces', hasActive: false, fields: [nameField, codeField] },
  { key: 'cities', labelKey: 'masterdata.reference.resources.cities', hasActive: false, fields: [
    { key: 'province_id', labelKey: 'masterdata.reference.fields.province', type: 'fk', fkResource: 'provinces', required: true },
    nameField,
    codeField
  ] },
  { key: 'models', labelKey: 'masterdata.reference.resources.models', hasActive: true, fields: [
    { key: 'brand_id', labelKey: 'masterdata.reference.fields.brand', type: 'fk', fkResource: 'brands', required: true },
    nameField
  ] },
  // Legacy-parity Fase 4 masters.
  { key: 'office-classes', labelKey: 'masterdata.reference.resources.office-classes', hasActive: true, fields: [nameField] },
  { key: 'executor-divisions', labelKey: 'masterdata.reference.resources.executor-divisions', hasActive: true, fields: [nameField] },
  { key: 'companies', labelKey: 'masterdata.reference.resources.companies', hasActive: true, fields: [nameField] },
  { key: 'building-classifications', labelKey: 'masterdata.reference.resources.building-classifications', hasActive: true, fields: [
    nameField,
    { key: 'min_floors', labelKey: 'masterdata.reference.fields.min_floors', type: 'number', required: true },
    { key: 'max_floors', labelKey: 'masterdata.reference.fields.max_floors', type: 'number' }
  ] }
]
