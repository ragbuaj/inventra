export type ReferenceKey
  = 'office-types' | 'departments' | 'positions' | 'units'
    | 'maintenance-categories' | 'problem-categories' | 'brands'
    | 'vendors' | 'provinces' | 'cities' | 'models'

export interface ReferenceField {
  key: string
  labelKey: string
  placeholder?: string
}

export interface ReferenceDescriptor {
  key: ReferenceKey
  labelKey: string
  fields: ReferenceField[]
}

const nameField: ReferenceField = { key: 'name', labelKey: 'masterdata.reference.fields.name' }
const codeField: ReferenceField = { key: 'code', labelKey: 'masterdata.reference.fields.code' }

export const referenceResources: ReferenceDescriptor[] = [
  { key: 'office-types', labelKey: 'masterdata.reference.resources.office-types', fields: [nameField] },
  { key: 'departments', labelKey: 'masterdata.reference.resources.departments', fields: [nameField] },
  { key: 'positions', labelKey: 'masterdata.reference.resources.positions', fields: [nameField] },
  { key: 'units', labelKey: 'masterdata.reference.resources.units', fields: [nameField, { key: 'symbol', labelKey: 'masterdata.reference.fields.symbol' }] },
  { key: 'maintenance-categories', labelKey: 'masterdata.reference.resources.maintenance-categories', fields: [nameField] },
  { key: 'problem-categories', labelKey: 'masterdata.reference.resources.problem-categories', fields: [nameField] },
  { key: 'brands', labelKey: 'masterdata.reference.resources.brands', fields: [nameField] },
  { key: 'vendors', labelKey: 'masterdata.reference.resources.vendors', fields: [nameField, { key: 'email', labelKey: 'masterdata.reference.fields.email' }, { key: 'phone', labelKey: 'masterdata.reference.fields.phone' }] },
  { key: 'provinces', labelKey: 'masterdata.reference.resources.provinces', fields: [nameField, codeField] },
  { key: 'cities', labelKey: 'masterdata.reference.resources.cities', fields: [nameField, codeField] },
  { key: 'models', labelKey: 'masterdata.reference.resources.models', fields: [nameField] }
]
