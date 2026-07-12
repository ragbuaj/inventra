import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { ReferenceRow } from '~/types'

const listOffices = vi.fn()
const getOffice = vi.fn()
vi.mock('~/composables/api/useOffices', () => ({ useOffices: () => ({ list: listOffices, get: getOffice }) }))

const listEmployees = vi.fn()
const getEmployee = vi.fn()
vi.mock('~/composables/api/useEmployees', () => ({ useEmployees: () => ({ list: listEmployees, get: getEmployee }) }))

const listReference = vi.fn()
vi.mock('~/composables/api/useReference', () => ({ useReference: () => ({ list: listReference }) }))

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useOfficePicker, useEmployeePicker, useReferencePicker } from '~/composables/usePickerSource'

beforeEach(() => {
  listOffices.mockReset()
  getOffice.mockReset()
  listEmployees.mockReset()
  getEmployee.mockReset()
  listReference.mockReset()
  request.mockReset()
})

describe('useOfficePicker', () => {
  it('searchFn maps offices to picker items (label=name, sublabel=code)', async () => {
    listOffices.mockResolvedValueOnce({ data: [{ id: 'o1', name: 'Pusat', code: 'KP-001' }], total: 1, limit: 20, offset: 0 })
    const { searchFn } = useOfficePicker()
    const items = await searchFn('pus')
    expect(listOffices).toHaveBeenCalledWith({ search: 'pus', limit: 20 })
    expect(items).toEqual([{ id: 'o1', label: 'Pusat', sublabel: 'KP-001' }])
  })

  it('resolveFn maps a single office by id', async () => {
    getOffice.mockResolvedValueOnce({ id: 'o1', name: 'Pusat', code: 'KP-001' })
    const { resolveFn } = useOfficePicker()
    expect(await resolveFn('o1')).toEqual({ id: 'o1', label: 'Pusat', sublabel: 'KP-001' })
    expect(getOffice).toHaveBeenCalledWith('o1')
  })

  it('resolveFn resolves to null when the office fetch fails (e.g. 404) instead of rejecting', async () => {
    getOffice.mockRejectedValueOnce(new Error('not found'))
    const { resolveFn } = useOfficePicker()
    await expect(resolveFn('missing')).resolves.toBeNull()
  })
})

describe('useEmployeePicker', () => {
  it('searchFn maps employees to picker items (label=name, sublabel=code)', async () => {
    listEmployees.mockResolvedValueOnce({ data: [{ id: 'e1', name: 'Andi', code: '199001' }], total: 1, limit: 20, offset: 0 })
    const { searchFn } = useEmployeePicker()
    const items = await searchFn('andi')
    expect(listEmployees).toHaveBeenCalledWith({ search: 'andi', limit: 20 })
    expect(items).toEqual([{ id: 'e1', label: 'Andi', sublabel: '199001' }])
  })

  it('resolveFn maps a single employee by id', async () => {
    getEmployee.mockResolvedValueOnce({ id: 'e1', name: 'Andi', code: '199001' })
    const { resolveFn } = useEmployeePicker()
    expect(await resolveFn('e1')).toEqual({ id: 'e1', label: 'Andi', sublabel: '199001' })
    expect(getEmployee).toHaveBeenCalledWith('e1')
  })

  it('resolveFn resolves to null when the employee fetch fails instead of rejecting', async () => {
    getEmployee.mockRejectedValueOnce(new Error('not found'))
    const { resolveFn } = useEmployeePicker()
    await expect(resolveFn('missing')).resolves.toBeNull()
  })
})

describe('useReferencePicker', () => {
  it('searchFn maps reference rows to picker items (label=name, no sublabel when code absent)', async () => {
    listReference.mockResolvedValueOnce({ data: [{ id: 'b1', name: 'Dell' } as ReferenceRow], total: 1, limit: 20, offset: 0 })
    const { searchFn } = useReferencePicker('brands')
    const items = await searchFn('dell')
    expect(listReference).toHaveBeenCalledWith('brands', { search: 'dell', limit: 20 })
    expect(items).toEqual([{ id: 'b1', label: 'Dell', sublabel: undefined }])
  })

  it('searchFn includes sublabel when the row has a code', async () => {
    listReference.mockResolvedValueOnce({ data: [{ id: 'd1', name: 'Jakarta', code: '31' } as ReferenceRow], total: 1, limit: 20, offset: 0 })
    const { searchFn } = useReferencePicker('cities')
    const items = await searchFn('jak')
    expect(items).toEqual([{ id: 'd1', label: 'Jakarta', sublabel: '31' }])
  })

  it('resolveFn GETs /<resource>/:id directly (useReference exposes no per-id getter) and maps the row', async () => {
    request.mockResolvedValueOnce({ id: 'b1', name: 'Dell' })
    const { resolveFn } = useReferencePicker('brands')
    expect(await resolveFn('b1')).toEqual({ id: 'b1', label: 'Dell', sublabel: undefined })
    expect(request).toHaveBeenCalledWith('/brands/b1')
  })

  it('resolveFn resolves to null when the fetch fails instead of rejecting', async () => {
    request.mockRejectedValueOnce(new Error('not found'))
    const { resolveFn } = useReferencePicker('brands')
    await expect(resolveFn('missing')).resolves.toBeNull()
  })
})
