import type { ComputedRef, Ref } from 'vue'
import type { ReferenceDescriptor } from '~/composables/api/referenceResources'
import type { ReferenceRow } from '~/types'

// USelect forbids an empty-string item value; a disabled "no floor" sentinel keeps
// the dropdown from being a silent empty popover when the office has no floors yet.
const NO_FLOOR = '__nofloor__'

/**
 * Department-only field wiring for the generic reference screen: a floor picker
 * filtered to the selected office, plus office/floor id->name resolution for the
 * table columns. Extracted from `pages/master/reference.vue` so the reference
 * engine stays generic and department specifics live in one place.
 *
 * Pass the screen's reactive `form`, the `rows` ref, and the active `descriptor`.
 */
export function useDepartmentFields(opts: {
  descriptor: ComputedRef<ReferenceDescriptor>
  form: Record<string, unknown>
  rows: Ref<ReferenceRow[]>
}) {
  const { descriptor, form, rows } = opts
  const { t } = useI18n()
  const floorsApi = useFloors()
  const officesApi = useOffices()

  const hasOfficeField = computed(() => descriptor.value.fields.some(f => f.type === 'office'))
  const hasFloorField = computed(() => descriptor.value.fields.some(f => f.type === 'floor'))

  // Floor options for the form, filtered to the currently selected office's floors.
  const floorFormOptions = ref<{ label: string, value: string }[]>([])
  async function loadFloorFormOptions(officeId: string) {
    if (!officeId) {
      floorFormOptions.value = []
      return
    }
    try {
      const fs = await floorsApi.listByOffice(officeId)
      floorFormOptions.value = fs.map(f => ({ label: f.name, value: f.id }))
    } catch {
      floorFormOptions.value = []
    }
  }
  const floorFormItems = computed(() => {
    if (!form.office_id) return []
    if (floorFormOptions.value.length === 0) {
      return [{ label: t('masterdata.reference.noFloor'), value: NO_FLOOR, disabled: true }]
    }
    return floorFormOptions.value
  })

  // Office change resets the dependent floor and reloads its options.
  async function onOfficeFieldChange(fieldKey: string, val: string | null) {
    form[fieldKey] = val ?? ''
    if (hasFloorField.value) {
      form.floor_id = ''
      await loadFloorFormOptions(String(val ?? ''))
    }
  }

  // Map the disabled "no floor" sentinel back to empty so it can never reach the
  // backend as a bogus floor id.
  function onFloorFieldChange(fieldKey: string, val: string | null) {
    form[fieldKey] = (val === NO_FLOOR ? '' : (val ?? ''))
  }

  // id -> name maps for the office/floor table columns, loaded non-fatally (a
  // failure just leaves the cell showing a dash). Cached so a search keystroke or
  // page change does not refetch: offices are fetched once, and each office's
  // floors are fetched at most once.
  const officeNames = ref<Record<string, string>>({})
  const floorNames = ref<Record<string, string>>({})
  let officesLoaded = false
  const floorOfficesLoaded = new Set<string>()

  function currentRowOfficeIds(): string[] {
    return [...new Set(rows.value.map(r => (r as Record<string, unknown>).office_id).filter(Boolean) as string[])]
  }

  async function loadDeptNameMaps() {
    if (!hasOfficeField.value) return
    if (!officesLoaded) {
      try {
        const offices = await officesApi.tree()
        officeNames.value = Object.fromEntries(offices.map(o => [o.id, o.name]))
        officesLoaded = true
      } catch { /* leave unresolved; retry on the next refresh */ }
    }
    if (!hasFloorField.value) return
    const missing = currentRowOfficeIds().filter(id => !floorOfficesLoaded.has(id))
    if (missing.length === 0) return
    try {
      const entries = await Promise.all(missing.map(async (id) => {
        const list = await floorsApi.listByOffice(id)
        floorOfficesLoaded.add(id)
        return list.map(f => [f.id, f.name] as const)
      }))
      floorNames.value = { ...floorNames.value, ...Object.fromEntries(entries.flat()) }
    } catch { /* leave unresolved; retry on the next refresh */ }
  }

  return {
    hasOfficeField,
    hasFloorField,
    floorFormItems,
    loadFloorFormOptions,
    onOfficeFieldChange,
    onFloorFieldChange,
    officeNames,
    floorNames,
    loadDeptNameMaps
  }
}
