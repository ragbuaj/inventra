// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import CategoryFormSlideover from '~/components/category/CategoryFormSlideover.vue'

type Vm = {
  form: Record<string, unknown>
  isIntangible: boolean
  isBuilding: boolean
  onSubmit: () => void
}

async function mountOpen() {
  const wrapper = await mountSuspended(CategoryFormSlideover, {
    props: { open: true, category: null, parentOptions: [{ value: 'c-it', label: 'Perangkat IT' }] }
  })
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('CategoryFormSlideover', () => {
  it('renders the four numbered sections', async () => {
    const _wrapper = await mountOpen()
    const html = document.body.innerHTML
    expect(html).toContain('Umum')
    expect(html).toContain('Penyusutan Komersial')
    expect(html).toContain('Pajak / Fiskal')
    expect(html).toContain('Akuntansi')
  })

  it('switches the depreciation section to Amortisasi/PSAK 19 when class is intangible', async () => {
    const wrapper = await mountOpen()
    const vm = wrapper.vm as unknown as Vm
    vm.form.asset_class = 'intangible'
    await wrapper.vm.$nextTick()
    const html = document.body.innerHTML
    expect(html).toContain('Amortisasi Komersial')
    expect(html).toContain('PSAK 19')
    expect(html).not.toContain('Bangunan Permanen')
  })

  it('locks method to Garis Lurus when a building fiscal group is selected', async () => {
    const wrapper = await mountOpen()
    const vm = wrapper.vm as unknown as Vm
    vm.form.default_fiscal_group = 'bangunan_permanen'
    await wrapper.vm.$nextTick()
    expect(vm.isBuilding).toBe(true)
    expect(vm.form.default_depreciation_method).toBe('straight_line')
    expect(document.body.innerHTML).toContain('wajib memakai Garis Lurus')
  })

  it('blocks submit and flags errors when name and code are empty', async () => {
    const wrapper = await mountOpen()
    const vm = wrapper.vm as unknown as Vm
    vm.onSubmit()
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('submit')).toBeFalsy()
    expect(document.body.innerHTML).toContain('Wajib diisi')
  })

  it('lets an edited child category clear its parent back to none', async () => {
    const wrapper = await mountSuspended(CategoryFormSlideover, {
      props: {
        open: true,
        category: {
          id: 'c-laptop', name: 'Komputer & Laptop', code: 'ELK', parent_id: 'c-it',
          default_depreciation_method: 'straight_line', default_useful_life_months: 48,
          default_salvage_rate: '0', asset_class: 'tangible', default_fiscal_group: 'kelompok_1',
          default_fiscal_life_months: 48, gl_account_code: '1.2.3.01', capitalization_threshold: '1000000',
          is_active: true, created_at: '2026-01-06'
        },
        parentOptions: [{ value: 'c-it', label: 'Perangkat IT' }]
      }
    })
    await wrapper.vm.$nextTick()
    const vm = wrapper.vm as unknown as { form: Record<string, unknown>, onSubmit: () => void }
    expect(vm.form.parent_id).toBe('c-it')
    vm.form.parent_id = '__none__'
    vm.onSubmit()
    await wrapper.vm.$nextTick()
    const payload = wrapper.emitted('submit')?.[0]?.[0] as Record<string, unknown>
    expect(payload.parent_id).toBeNull()
  })

  it('emits submit with a snake_case CategoryInput payload', async () => {
    const wrapper = await mountOpen()
    const vm = wrapper.vm as unknown as Vm
    Object.assign(vm.form, {
      name: 'Genset',
      code: 'GEN',
      asset_class: 'tangible',
      default_depreciation_method: 'declining_balance',
      default_useful_life_months: '96',
      default_salvage_rate: '10',
      default_fiscal_group: 'kelompok_2',
      default_fiscal_life_months: '96',
      gl_account_code: '1.2.7.00',
      capitalization_threshold: '10.000.000',
      parent_id: 'c-it',
      is_active: true
    })
    vm.onSubmit()
    await wrapper.vm.$nextTick()
    const payload = wrapper.emitted('submit')?.[0]?.[0]
    expect(payload).toEqual({
      name: 'Genset',
      code: 'GEN',
      parent_id: 'c-it',
      default_depreciation_method: 'declining_balance',
      default_useful_life_months: 96,
      default_salvage_rate: '10',
      asset_class: 'tangible',
      default_fiscal_group: 'kelompok_2',
      default_fiscal_life_months: 96,
      gl_account_code: '1.2.7.00',
      capitalization_threshold: '10000000',
      is_active: true
    })
  })
})
