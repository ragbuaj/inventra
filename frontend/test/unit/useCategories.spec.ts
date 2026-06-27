import { describe, it, expect } from 'vitest'
import { useCategories } from '~/composables/api/useCategories'

describe('useCategories composable', () => {
  const api = useCategories()

  it('create then list includes the new category', async () => {
    const created = await api.create({
      name: 'Test Kategori',
      code: 'TST',
      parent_id: null,
      default_depreciation_method: 'straight_line',
      default_useful_life_months: 12,
      default_salvage_rate: '0',
      asset_class: 'tangible',
      default_fiscal_group: 'kelompok_1',
      default_fiscal_life_months: 12,
      gl_account_code: '9.9.9.99',
      capitalization_threshold: '500000',
      is_active: true
    })
    expect(created.id).toBeTruthy()
    const page = await api.list({ search: 'Test Kategori' })
    expect(page.data.some(c => c.id === created.id)).toBe(true)
  })

  it('update changes a field and get reflects it', async () => {
    const created = await api.create({
      name: 'Editable',
      code: 'EDT',
      parent_id: null,
      default_depreciation_method: 'straight_line',
      default_useful_life_months: 24,
      default_salvage_rate: null,
      asset_class: 'tangible',
      default_fiscal_group: null,
      default_fiscal_life_months: null,
      gl_account_code: null,
      capitalization_threshold: null,
      is_active: true
    })
    await api.update(created.id, { ...created, name: 'Edited' })
    const got = await api.get(created.id)
    expect(got?.name).toBe('Edited')
  })

  it('update throws for an unknown id', async () => {
    await expect(api.update('no-such-id', {
      name: 'x',
      code: null,
      parent_id: null,
      default_depreciation_method: null,
      default_useful_life_months: null,
      default_salvage_rate: null,
      asset_class: 'tangible',
      default_fiscal_group: null,
      default_fiscal_life_months: null,
      gl_account_code: null,
      capitalization_threshold: null,
      is_active: true
    })).rejects.toThrow('masterdata.categories.errNotFound')
  })

  it('remove then get returns undefined', async () => {
    const created = await api.create({
      name: 'Removable',
      code: 'RMV',
      parent_id: null,
      default_depreciation_method: null,
      default_useful_life_months: null,
      default_salvage_rate: null,
      asset_class: 'tangible',
      default_fiscal_group: null,
      default_fiscal_life_months: null,
      gl_account_code: null,
      capitalization_threshold: null,
      is_active: true
    })
    await api.remove(created.id)
    expect(await api.get(created.id)).toBeUndefined()
  })
})
