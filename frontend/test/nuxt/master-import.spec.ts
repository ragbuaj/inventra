// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'

const uploadImport = vi.fn()
const getJob = vi.fn()
const getRows = vi.fn()
const listJobs = vi.fn()
const confirmJob = vi.fn()
const cancelJob = vi.fn()
const getTemplate = vi.fn()
const getErrorReport = vi.fn()

vi.mock('~/composables/api/useImports', () => ({
  useImports: () => ({
    uploadImport, getJob, getRows, listJobs, confirmJob, cancelJob, getTemplate, getErrorReport
  })
}))

// eslint-disable-next-line import/first
import MasterImportPage from '~/pages/master/import.vue'

function login(permissions: string[]) {
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'User', email: 'u@test.com', role_id: 'r1', role_name: 'Role', office_id: null },
    permissions
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  listJobs.mockResolvedValue({ data: [], total: 0, limit: 1, offset: 0 })
})

describe('Master-data import page — target resolution', () => {
  it('renders an invalid-target state when ?target= is missing', async () => {
    login(['*'])
    const wrapper = await mountSuspended(MasterImportPage, { route: '/master/import' })
    await flushPromises()

    expect(wrapper.text()).toContain('Target import tidak valid')
    expect(wrapper.findComponent({ name: 'ImportWizard' }).exists()).toBe(false)
  })

  it('renders an invalid-target state for an unrecognized target', async () => {
    login(['*'])
    const wrapper = await mountSuspended(MasterImportPage, { route: '/master/import?target=vendor' })
    await flushPromises()

    expect(wrapper.text()).toContain('Target import tidak valid')
    expect(wrapper.findComponent({ name: 'ImportWizard' }).exists()).toBe(false)
  })

  it('mounts ImportWizard with target=employee and the employee-manage permission', async () => {
    login(['*'])
    const wrapper = await mountSuspended(MasterImportPage, { route: '/master/import?target=employee' })
    await flushPromises()

    expect(listJobs).toHaveBeenCalledWith('employee', { limit: 1 })
    const wizard = wrapper.findComponent({ name: 'ImportWizard' })
    expect(wizard.exists()).toBe(true)
    expect(wizard.props('target')).toBe('employee')
    expect(wizard.props('permission')).toBe('masterdata.employee.manage')
  })

  it('mounts ImportWizard with target=office and the office-manage permission', async () => {
    login(['*'])
    const wrapper = await mountSuspended(MasterImportPage, { route: '/master/import?target=office' })
    await flushPromises()

    const wizard = wrapper.findComponent({ name: 'ImportWizard' })
    expect(wizard.props('target')).toBe('office')
    expect(wizard.props('permission')).toBe('masterdata.office.manage')
  })

  it('mounts ImportWizard for reference:provinces with the global-manage permission', async () => {
    login(['*'])
    const wrapper = await mountSuspended(MasterImportPage, { route: '/master/import?target=reference:provinces' })
    await flushPromises()

    const wizard = wrapper.findComponent({ name: 'ImportWizard' })
    expect(wizard.props('target')).toBe('reference:provinces')
    expect(wizard.props('permission')).toBe('masterdata.global.manage')
  })

  it('mounts ImportWizard for reference:cities with the global-manage permission', async () => {
    login(['*'])
    const wrapper = await mountSuspended(MasterImportPage, { route: '/master/import?target=reference:cities' })
    await flushPromises()

    const wizard = wrapper.findComponent({ name: 'ImportWizard' })
    expect(wizard.props('target')).toBe('reference:cities')
    expect(wizard.props('permission')).toBe('masterdata.global.manage')
  })

  it('mounts ImportWizard for reference:brands with the global-manage permission and Brand label', async () => {
    login(['*'])
    const wrapper = await mountSuspended(MasterImportPage, { route: '/master/import?target=reference:brands' })
    await flushPromises()

    const wizard = wrapper.findComponent({ name: 'ImportWizard' })
    expect(wizard.exists()).toBe(true)
    expect(wizard.props('target')).toBe('reference:brands')
    expect(wizard.props('permission')).toBe('masterdata.global.manage')
    expect(wrapper.text()).toContain('Brand')
  })

  it('mounts ImportWizard for reference:models with the global-manage permission and Model label', async () => {
    login(['*'])
    const wrapper = await mountSuspended(MasterImportPage, { route: '/master/import?target=reference:models' })
    await flushPromises()

    const wizard = wrapper.findComponent({ name: 'ImportWizard' })
    expect(wizard.exists()).toBe(true)
    expect(wizard.props('target')).toBe('reference:models')
    expect(wizard.props('permission')).toBe('masterdata.global.manage')
    expect(wrapper.text()).toContain('Model')
  })

  it('mounts ImportWizard for reference:units with the global-manage permission and Satuan label', async () => {
    login(['*'])
    const wrapper = await mountSuspended(MasterImportPage, { route: '/master/import?target=reference:units' })
    await flushPromises()

    const wizard = wrapper.findComponent({ name: 'ImportWizard' })
    expect(wizard.exists()).toBe(true)
    expect(wizard.props('target')).toBe('reference:units')
    expect(wizard.props('permission')).toBe('masterdata.global.manage')
    expect(wrapper.text()).toContain('Satuan')
  })
})

describe('Master-data import page — authorization', () => {
  it('shows a not-authorized state when the caller lacks the target permission', async () => {
    login(['masterdata.office.manage']) // office perm only — not employee
    const wrapper = await mountSuspended(MasterImportPage, { route: '/master/import?target=employee' })
    await flushPromises()

    expect(wrapper.text()).toContain('Anda tidak memiliki izin untuk mengimpor data ini.')
    expect(wrapper.findComponent({ name: 'ImportWizard' }).exists()).toBe(false)
  })

  it('renders the wizard when the caller holds the exact matching permission', async () => {
    login(['masterdata.employee.manage'])
    const wrapper = await mountSuspended(MasterImportPage, { route: '/master/import?target=employee' })
    await flushPromises()

    expect(wrapper.findComponent({ name: 'ImportWizard' }).exists()).toBe(true)
  })
})
