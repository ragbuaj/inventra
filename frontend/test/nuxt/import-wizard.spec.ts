// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import type { ImportJob, ImportRow } from '~/composables/api/useImports'
import { useAuthStore } from '~/stores/auth'

// ---------------------------------------------------------------------------
// Stub the useImports composable with controllable fakes (same idiom as
// approval.spec.ts). Every wizard state is driven by the job the fakes return.
// ---------------------------------------------------------------------------
const uploadImport = vi.fn()
const getJob = vi.fn()
const getRows = vi.fn()
const listJobs = vi.fn()
const confirmJob = vi.fn()
const cancelJob = vi.fn()
const getTemplate = vi.fn()
const getErrorReport = vi.fn()

vi.mock('~/composables/api/useImports', async (importOriginal) => {
  const actual = await importOriginal<typeof import('~/composables/api/useImports')>()
  return {
    ...actual,
    useImports: () => ({
      uploadImport, getJob, getRows, listJobs, confirmJob, cancelJob, getTemplate, getErrorReport
    })
  }
})

// eslint-disable-next-line import/first
import ImportWizard from '~/components/import/ImportWizard.vue'

const job = (over: Partial<ImportJob> = {}): ImportJob => ({
  id: 'job-1',
  target: 'asset',
  format: 'csv',
  filename: 'assets.csv',
  status: 'validated',
  total_rows: 3,
  success_rows: 2,
  failed_rows: 1,
  created_at: '2026-07-01T00:00:00Z',
  ...over
})

const rowsPage = (data: ImportRow[]) => ({ data, total: data.length, limit: 20, offset: 0 })

// Backend/OpenAPI ImportRow shape: target-column values are flat siblings of
// id/row_no/valid/errors (additionalProperties), NOT nested under a `data`
// key — see useImports.ts's ImportRow doc comment.
const okRow: ImportRow = { id: 'r-1', row_no: 1, valid: true, asset_tag: 'A-1', nama: 'Kursi', kategori: 'Furnitur', errors: [] }
const badRow: ImportRow = {
  id: 'r-2',
  row_no: 2,
  valid: false,
  asset_tag: 'A-2',
  nama: 'Meja',
  kategori: 'Elektronikk',
  errors: [{ column: 'kategori', error_key: 'kat' }]
}

beforeEach(() => {
  vi.clearAllMocks()
  useAuthStore().setSession(
    'tok',
    { id: '1', name: 'Admin', email: 'a@b.com', role_id: 'r1', role_name: 'Superadmin', office_id: null },
    ['*']
  )
  // Default: no resumable job → wizard starts at step 1.
  listJobs.mockResolvedValue({ data: [], total: 0, limit: 1, offset: 0 })
  getRows.mockResolvedValue(rowsPage([okRow, badRow]))
})

async function mount(props: { target: string, permission: string } = { target: 'asset', permission: 'asset.manage' }) {
  const w = await mountSuspended(ImportWizard, { props })
  await flushPromises()
  return w
}

describe('ImportWizard — step 1 (upload)', () => {
  it('renders the file input, template button, and asset columns', async () => {
    const w = await mount()
    expect(listJobs).toHaveBeenCalledWith('asset', { limit: 1 })

    const input = w.find('[data-testid="import-file-input"]')
    expect(input.exists()).toBe(true)
    expect(input.attributes('accept')).toBe('.csv,.xlsx')

    const text = w.text()
    // Template download button (resolved i18n, not the raw key).
    expect(text).toContain('Unduh Template')
    // Asset column badges are hardcoded for the asset target.
    expect(text).toContain('asset_tag')
    expect(text).toContain('kategori')

    w.unmount()
  })

  it('uploads on validate click and shows a spinner while polling a pending job', async () => {
    uploadImport.mockResolvedValue(job({ status: 'pending' }))
    getJob.mockResolvedValue(job({ status: 'pending' }))
    const w = await mount()

    // Selecting a valid file enables the validate button.
    const file = new File(['a,b\n1,2'], 'assets.csv', { type: 'text/csv' })
    const input = w.find('[data-testid="import-file-input"]').element as HTMLInputElement
    Object.defineProperty(input, 'files', { value: [file], configurable: true })
    await w.find('[data-testid="import-file-input"]').trigger('change')
    await flushPromises()
    expect(w.text()).toContain('assets.csv')

    const validate = w.findAll('button').find(b => b.text().includes('Validasi Berkas'))
    await validate!.trigger('click')
    await flushPromises()

    expect(uploadImport).toHaveBeenCalledWith('asset', file)
    // pending job → processing card visible.
    expect(w.text()).toContain('Memvalidasi berkas…')
    w.unmount()
  })
})

describe('ImportWizard — file validation', () => {
  it('rejects an oversize file without selecting it', async () => {
    const w = await mount()
    const big = new File([new Uint8Array(1)], 'big.csv', { type: 'text/csv' })
    Object.defineProperty(big, 'size', { value: 11 * 1024 * 1024 })
    const input = w.find('[data-testid="import-file-input"]').element as HTMLInputElement
    Object.defineProperty(input, 'files', { value: [big], configurable: true })
    await w.find('[data-testid="import-file-input"]').trigger('change')
    await flushPromises()

    // File not accepted → the drop-zone prompt is still shown, name absent.
    expect(w.text()).not.toContain('big.csv')
    expect(w.text()).toContain('Klik untuk pilih berkas')
    w.unmount()
  })

  it('rejects a wrong extension', async () => {
    const w = await mount()
    const pdf = new File(['x'], 'doc.pdf', { type: 'application/pdf' })
    const input = w.find('[data-testid="import-file-input"]').element as HTMLInputElement
    Object.defineProperty(input, 'files', { value: [pdf], configurable: true })
    await w.find('[data-testid="import-file-input"]').trigger('change')
    await flushPromises()
    expect(w.text()).not.toContain('doc.pdf')
    w.unmount()
  })
})

describe('ImportWizard — step 2 (validate)', () => {
  it('resumes to the row table with an error-highlighted cell (resolved i18n)', async () => {
    listJobs.mockResolvedValue({ data: [job({ status: 'validated' })], total: 1, limit: 1, offset: 0 })
    // Resume fetches the enriched single-job view before deciding the phase.
    getJob.mockResolvedValue(job({ status: 'validated' }))
    const w = await mount()

    expect(getJob).toHaveBeenCalledWith('job-1')
    expect(getRows).toHaveBeenCalledWith('job-1', { onlyErrors: false, limit: 20, offset: 0 })
    const text = w.text()
    expect(text).toContain('Total baris')
    expect(text).toContain('2 Valid')
    expect(text).toContain('1 Error')
    expect(text).toContain('Meja') // a rendered data cell
    // The cell's error_key 'kat' resolves via i18n in the note column.
    expect(text).toContain('Kategori tidak ditemukan')

    // The errored cell carries the error tint class.
    const errored = w.findAll('td').filter(td => td.classes().includes('bg-error/10'))
    expect(errored.length).toBeGreaterThan(0)
    w.unmount()
  })

  it('re-fetches rows with onlyErrors:true when the toggle is checked', async () => {
    listJobs.mockResolvedValue({ data: [job({ status: 'validated' })], total: 1, limit: 1, offset: 0 })
    getJob.mockResolvedValue(job({ status: 'validated' }))
    const w = await mount()
    getRows.mockClear()
    getRows.mockResolvedValue(rowsPage([badRow]))

    // UCheckbox renders a button[role="checkbox"] — click it to toggle v-model.
    const checkbox = w.find('button[role="checkbox"]')
    await checkbox.trigger('click')
    await flushPromises()

    expect(getRows).toHaveBeenCalledWith('job-1', { onlyErrors: true, limit: 20, offset: 0 })
    w.unmount()
  })

  it('shows a retry affordance when getRows fails', async () => {
    listJobs.mockResolvedValue({ data: [job({ status: 'validated' })], total: 1, limit: 1, offset: 0 })
    getJob.mockResolvedValue(job({ status: 'validated' }))
    getRows.mockRejectedValueOnce(new Error('boom'))
    const w = await mount()

    expect(w.text()).toContain('Gagal memuat baris')
    const retry = w.findAll('button').find(b => b.text().includes('Coba lagi'))
    expect(retry).toBeDefined()

    // Retry succeeds → table renders.
    getRows.mockResolvedValue(rowsPage([okRow, badRow]))
    await retry!.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Meja')
    w.unmount()
  })

  it('shows the empty-rows message when a validated job has no rows', async () => {
    listJobs.mockResolvedValue({
      data: [job({ status: 'validated', total_rows: 0, success_rows: 0, failed_rows: 0 })],
      total: 1, limit: 1, offset: 0
    })
    getJob.mockResolvedValue(job({ status: 'validated', total_rows: 0, success_rows: 0, failed_rows: 0 }))
    getRows.mockResolvedValue(rowsPage([]))
    const w = await mount()

    expect(w.text()).toContain('Tidak ada baris untuk ditampilkan.')
    // Neither the row table nor the loading/error placeholders render.
    expect(w.find('table').exists()).toBe(false)
    w.unmount()
  })

  it('confirms the job and begins polling', async () => {
    listJobs.mockResolvedValue({ data: [job({ status: 'validated' })], total: 1, limit: 1, offset: 0 })
    confirmJob.mockResolvedValue(job({ status: 'executing' }))
    // Resume's enrich fetch must return the same (validated) status so the
    // wizard still lands on step 2 — only confirmJob (not getJob) drives the
    // later "executing" transition asserted below.
    getJob.mockResolvedValue(job({ status: 'validated' }))
    const w = await mount()

    const create = w.findAll('button').find(b => b.text().includes('Buat Aset Valid'))
    expect(create).toBeDefined()
    await create!.trigger('click')
    await flushPromises()

    expect(confirmJob).toHaveBeenCalledWith('job-1')
    expect(w.text()).toContain('Membuat aset…')
    w.unmount()
  })
})

describe('ImportWizard — step 3 (result)', () => {
  it('asset awaiting_approval shows the approval-pending card, not the tiles', async () => {
    listJobs.mockResolvedValue({
      data: [job({ status: 'awaiting_approval', request_id: 'REQ-42' })],
      total: 1, limit: 1, offset: 0
    })
    getJob.mockResolvedValue(job({ status: 'awaiting_approval', request_id: 'REQ-42' }))
    const w = await mount()

    expect(getJob).toHaveBeenCalledWith('job-1')
    const text = w.text()
    expect(text).toContain('Diajukan untuk persetujuan')
    expect(text).toContain('REQ-42')
    expect(text).toContain('Menunggu keputusan approver')
    // The created/failed result tiles must NOT appear.
    expect(text).not.toContain('Aset dibuat')
    expect(text).not.toContain('Baris gagal')
    w.unmount()
  })

  it('rejected approval shows the rejected card with an error-report download (enriched view, not the stale list row)', async () => {
    // list() never enriches approval_status (only the single-job GET does),
    // so a resumed rejected batch arrives from listJobs looking merely
    // "awaiting" — the un-enriched row alone would render the waiting card.
    listJobs.mockResolvedValue({
      data: [job({ status: 'awaiting_approval', request_id: 'REQ-9', failed_rows: 1 })],
      total: 1, limit: 1, offset: 0
    })
    // The enriched getJob response is what actually carries approval_status.
    getJob.mockResolvedValue(job({ status: 'awaiting_approval', request_id: 'REQ-9', approval_status: 'rejected', failed_rows: 1 }))
    const w = await mount()

    expect(getJob).toHaveBeenCalledWith('job-1')
    expect(w.text()).toContain('Pengajuan ditolak')
    // Never rendered the un-enriched "awaiting" state on the way to rejected.
    expect(w.text()).not.toContain('Menunggu keputusan approver')
    const dl = w.findAll('button').find(b => b.text().includes('Unduh Baris Gagal'))
    expect(dl).toBeDefined()

    getErrorReport.mockResolvedValue(new Blob(['x']))
    vi.stubGlobal('URL', { ...URL, createObjectURL: () => 'blob:x', revokeObjectURL: () => {} })
    await dl!.trigger('click')
    await flushPromises()
    expect(getErrorReport).toHaveBeenCalledWith('job-1')
    vi.unstubAllGlobals()
    w.unmount()
  })

  // A terminal job is NOT auto-resumed (resume only covers active jobs), so the
  // failed state is reached through the upload → poll path.
  it('failed job shows the error card with the translated error_key', async () => {
    uploadImport.mockResolvedValue(job({ status: 'pending' }))
    getJob.mockResolvedValue(job({ status: 'failed', error_key: 'badHeader' }))
    const w = await mount()

    const file = new File(['a,b\n1,2'], 'assets.csv', { type: 'text/csv' })
    const input = w.find('[data-testid="import-file-input"]').element as HTMLInputElement
    Object.defineProperty(input, 'files', { value: [file], configurable: true })
    await w.find('[data-testid="import-file-input"]').trigger('change')
    await flushPromises()

    // Enable fake timers only now so the poll's setTimeout is controllable.
    vi.useFakeTimers()
    await w.findAll('button').find(b => b.text().includes('Validasi Berkas'))!.trigger('click')
    await flushPromises()

    // Advance the 1.5s poll → getJob resolves to a failed job.
    await vi.advanceTimersByTimeAsync(1600)
    await flushPromises()

    expect(w.text()).toContain('Import gagal')
    expect(w.text()).toContain('Header kolom tidak sesuai template')
    vi.useRealTimers()
    w.unmount()
  })

  // Completed is terminal too — reached by confirming a validated job.
  it('master-data completed job shows created/failed tiles + error-report button', async () => {
    listJobs.mockResolvedValue({ data: [job({ target: 'office', status: 'validated' })], total: 1, limit: 1, offset: 0 })
    getJob.mockResolvedValue(job({ target: 'office', status: 'validated' }))
    confirmJob.mockResolvedValue(job({ target: 'office', status: 'completed', success_rows: 5, failed_rows: 2 }))
    const w = await mount({ target: 'office', permission: 'masterdata.office.manage' })

    await w.findAll('button').find(b => b.text().includes('Buat'))!.trigger('click')
    await flushPromises()

    const text = w.text()
    expect(text).toContain('Import selesai diproses')
    expect(text).toContain('Aset dibuat') // createdLabel (shared key)
    expect(text).toContain('5')
    expect(text).toContain('2')

    const dl = w.findAll('button').find(b => b.text().includes('Unduh Baris Gagal'))
    expect(dl).toBeDefined()
    w.unmount()
  })

  it('completed job with no failures hides the error-report button', async () => {
    listJobs.mockResolvedValue({ data: [job({ target: 'office', status: 'validated' })], total: 1, limit: 1, offset: 0 })
    getJob.mockResolvedValue(job({ target: 'office', status: 'validated' }))
    confirmJob.mockResolvedValue(job({ target: 'office', status: 'completed', success_rows: 5, failed_rows: 0 }))
    const w = await mount({ target: 'office', permission: 'masterdata.office.manage' })

    await w.findAll('button').find(b => b.text().includes('Buat'))!.trigger('click')
    await flushPromises()

    expect(w.text()).toContain('Import selesai diproses')
    const dl = w.findAll('button').find(b => b.text().includes('Unduh Baris Gagal'))
    expect(dl).toBeUndefined()
    w.unmount()
  })
})

describe('ImportWizard — polling lifecycle', () => {
  it('does not schedule another poll if getJob resolves after unmount (no leaked polling loop)', async () => {
    uploadImport.mockResolvedValue(job({ status: 'pending' }))
    const w = await mount()

    const file = new File(['a,b\n1,2'], 'assets.csv', { type: 'text/csv' })
    const input = w.find('[data-testid="import-file-input"]').element as HTMLInputElement
    Object.defineProperty(input, 'files', { value: [file], configurable: true })
    await w.find('[data-testid="import-file-input"]').trigger('change')
    await flushPromises()

    vi.useFakeTimers()
    await w.findAll('button').find(b => b.text().includes('Validasi Berkas'))!.trigger('click')
    await flushPromises()

    // Arm getJob to hang so a request can be "in flight" when we unmount.
    let resolveGetJob!: (j: ImportJob) => void
    getJob.mockImplementation(() => new Promise<ImportJob>((resolve) => {
      resolveGetJob = resolve
    }))

    // Advance past the poll interval — the scheduled poll fires and getJob is
    // now in flight (its promise is intentionally left unresolved).
    await vi.advanceTimersByTimeAsync(1600)
    expect(getJob).toHaveBeenCalledTimes(1)

    // Unmount while that request is still pending.
    w.unmount()

    // The in-flight request resolves after teardown — it must not schedule
    // another timer or touch reactive state on the destroyed instance.
    resolveGetJob(job({ status: 'pending' }))
    await flushPromises()

    // Advance well past another poll interval: no further getJob calls means
    // no timer was armed after unmount (the leak this test guards against).
    await vi.advanceTimersByTimeAsync(5000)
    expect(getJob).toHaveBeenCalledTimes(1)

    vi.useRealTimers()
  })
})

describe('ImportWizard — template download', () => {
  it('downloads the CSV template for the target', async () => {
    getTemplate.mockResolvedValue(new Blob(['tag,name'], { type: 'text/csv' }))
    // jsdom lacks createObjectURL — stub it so the download path runs.
    const createURL = vi.fn(() => 'blob:x')
    const revokeURL = vi.fn()
    vi.stubGlobal('URL', { ...URL, createObjectURL: createURL, revokeObjectURL: revokeURL })

    const w = await mount()
    const btn = w.findAll('button').find(b => b.text().includes('Unduh Template'))
    await btn!.trigger('click')
    await flushPromises()

    expect(getTemplate).toHaveBeenCalledWith('asset', 'csv')
    expect(createURL).toHaveBeenCalled()
    vi.unstubAllGlobals()
    w.unmount()
  })
})
