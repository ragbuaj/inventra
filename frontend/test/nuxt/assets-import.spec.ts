// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import { useAuthStore } from '~/stores/auth'

// The asset import page is a thin wrapper around ImportWizard (fully covered
// by test/nuxt/import-wizard.spec.ts) — this test only verifies the wiring:
// the wizard mounts with the right target/permission and the page heading
// renders. Stub useImports so mounting never hits the real backend.
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
import ImportPage from '~/pages/assets/import.vue'

beforeEach(() => {
  vi.clearAllMocks()
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'admin@test.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
  listJobs.mockResolvedValue({ data: [], total: 0, limit: 1, offset: 0 })
})

describe('Asset import page', () => {
  it('renders the page heading and mounts ImportWizard with the asset target', async () => {
    const wrapper = await mountSuspended(ImportPage)
    await flushPromises()

    expect(wrapper.text()).toContain('Import Massal Aset')
    // Resuming call proves the wizard mounted with target="asset".
    expect(listJobs).toHaveBeenCalledWith('asset', { limit: 1 })

    const wizard = wrapper.findComponent({ name: 'ImportWizard' })
    expect(wizard.exists()).toBe(true)
    expect(wizard.props('target')).toBe('asset')
    expect(wizard.props('permission')).toBe('asset.manage')
  })

  it('renders the upload drop-zone once resumed', async () => {
    const wrapper = await mountSuspended(ImportPage)
    await flushPromises()

    expect(wrapper.text()).toContain('Klik untuk pilih berkas')
    expect(wrapper.find('[data-testid="import-file-input"]').exists()).toBe(true)
  })
})
