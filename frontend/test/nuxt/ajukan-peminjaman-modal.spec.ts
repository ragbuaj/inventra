// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { flushPromises, enableAutoUnmount } from '@vue/test-utils'
import type { AvailableAsset } from '~/composables/api/useAssignment'

// useToast's real toast portal isn't mounted in these component tests (no
// UApp wrapper), so success/error toast text never lands in the DOM. Mock it
// and assert on the call args instead (mirrors assets-form.spec.ts / stock-opname.spec.ts).
const { toastAddMock } = vi.hoisted(() => ({ toastAddMock: vi.fn() }))
mockNuxtImport('useToast', () => () => ({ add: toastAddMock }))

const AVAILABLE_ASSETS: AvailableAsset[] = [
  { id: 'as1', asset_tag: 'JKT01-ELK-2026-00001', name: 'Laptop Dell Latitude 5440' },
  { id: 'as2', asset_tag: 'JKT01-ELK-2026-00002', name: 'Proyektor Epson EB-X51' }
]

const borrowMock = vi.fn()
const availableMock = vi.fn()

vi.mock('~/composables/api/useAssignment', () => ({
  useAssignment: () => ({
    list: vi.fn(),
    available: availableMock,
    checkout: vi.fn(),
    checkin: vi.fn(),
    borrow: borrowMock,
    myRequests: vi.fn(),
    cancel: vi.fn()
  })
}))

// eslint-disable-next-line import/first
import AjukanPeminjamanModal from '~/components/assignment/AjukanPeminjamanModal.vue'

enableAutoUnmount(afterEach)

// UModal teleports its #body/#footer content to document.body (Reka UI Dialog
// Portal). The teleported content only actually appears in the DOM once the
// enter transition/Presence machinery settles — a bare flushPromises() isn't
// enough; a real wait (~400ms) is required. Same pattern as disposals.spec.ts
// / settings-users.spec.ts (document.body queries for teleported content).
async function mountAndWait(props: Record<string, unknown>) {
  const wrapper = await mountSuspended(AjukanPeminjamanModal, { props })
  await flushPromises()
  await new Promise(resolve => setTimeout(resolve, 400))
  await wrapper.vm.$nextTick()
  await flushPromises()
  return wrapper
}

function bodyEl(testid: string): HTMLElement {
  const el = document.body.querySelector(`[data-testid="${testid}"]`)
  expect(el, `expected [data-testid="${testid}"] in document.body`).toBeTruthy()
  return el as HTMLElement
}

function bodyElExists(testid: string): boolean {
  return !!document.body.querySelector(`[data-testid="${testid}"]`)
}

function setInputValue(el: HTMLElement, value: string) {
  const input = el as HTMLInputElement | HTMLTextAreaElement
  input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

beforeEach(() => {
  borrowMock.mockReset()
  availableMock.mockReset()
  availableMock.mockResolvedValue({ data: AVAILABLE_ASSETS })
  toastAddMock.mockReset()
})

describe('AjukanPeminjamanModal — locked asset (from Detail Aset)', () => {
  const LOCKED_ASSET = {
    id: 'as1',
    name: 'Proyektor Epson EB-X51',
    asset_tag: 'JKT01-ELK-2026-00002',
    category: 'Elektronik',
    office: 'Cabang Jakarta Selatan',
    location: 'Lantai 2 · Ruang Rapat'
  }

  it('renders the locked read-only asset block with name + tag, and no asset picker', async () => {
    await mountAndWait({ open: true, asset: LOCKED_ASSET })

    const locked = bodyEl('peminjaman-modal-locked-asset')
    expect(locked.textContent).toContain('Proyektor Epson EB-X51')
    expect(locked.textContent).toContain('JKT01-ELK-2026-00002')
    expect(bodyElExists('peminjaman-modal-asset-picker')).toBe(false)
    // Locked mode never needs the available-assets picker fetch.
    expect(availableMock).not.toHaveBeenCalled()
  })

  it('shows category / office / location from the asset prop', async () => {
    await mountAndWait({ open: true, asset: LOCKED_ASSET })
    const locked = bodyEl('peminjaman-modal-locked-asset')
    expect(locked.textContent).toContain('Elektronik')
    expect(locked.textContent).toContain('Cabang Jakarta Selatan')
    expect(locked.textContent).toContain('Lantai 2 · Ruang Rapat')
  })

  it('blocks submit and shows an inline error when Alasan is empty', async () => {
    await mountAndWait({ open: true, asset: LOCKED_ASSET })

    bodyEl('peminjaman-modal-submit').click()
    await flushPromises()

    expect(borrowMock).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('Alasan wajib diisi.')
  })

  it('fills Alasan + clicks Kirim → calls borrow with the locked asset id + typed notes, emits submitted', async () => {
    const wrapper = await mountAndWait({ open: true, asset: LOCKED_ASSET })

    setInputValue(bodyEl('peminjaman-modal-notes'), 'Presentasi ke nasabah prioritas')
    await flushPromises()
    bodyEl('peminjaman-modal-submit').click()
    await flushPromises()

    expect(borrowMock).toHaveBeenCalledWith({
      asset_id: 'as1',
      due_date: null,
      notes: 'Presentasi ke nasabah prioritas'
    })
    expect(wrapper.emitted('submitted')).toBeTruthy()
    expect(wrapper.emitted('update:open')).toBeTruthy()
    expect(wrapper.emitted('update:open')![0]).toEqual([false])
    expect(toastAddMock).toHaveBeenCalledWith(expect.objectContaining({ title: 'Pengajuan peminjaman terkirim' }))
  })

  it('includes the typed due date when provided', async () => {
    await mountAndWait({ open: true, asset: LOCKED_ASSET })

    setInputValue(bodyEl('peminjaman-modal-due-date'), '2026-07-15')
    setInputValue(bodyEl('peminjaman-modal-notes'), 'Kerja lapangan')
    await flushPromises()
    bodyEl('peminjaman-modal-submit').click()
    await flushPromises()

    expect(borrowMock).toHaveBeenCalledWith({
      asset_id: 'as1',
      due_date: '2026-07-15',
      notes: 'Kerja lapangan'
    })
  })

  it('shows the green info banner text', async () => {
    await mountAndWait({ open: true, asset: LOCKED_ASSET })
    expect(document.body.textContent).toContain('Peminjaman akan dikirim ke Manager untuk disetujui. Aset baru berpindah ke Anda setelah disetujui.')
  })
})

describe('AjukanPeminjamanModal — asset=null (page usage)', () => {
  it('renders the asset picker fed by useAssignment().available()', async () => {
    await mountAndWait({ open: true, asset: null })

    expect(availableMock).toHaveBeenCalled()
    expect(bodyElExists('peminjaman-modal-locked-asset')).toBe(false)
    expect(bodyElExists('peminjaman-modal-asset-picker')).toBe(true)
  })

  it('does not call available() when the modal is closed', async () => {
    availableMock.mockClear()
    await mountAndWait({ open: false, asset: null })
    expect(availableMock).not.toHaveBeenCalled()
  })

  it('blocks submit when no asset is picked, even with Alasan filled', async () => {
    await mountAndWait({ open: true, asset: null })
    setInputValue(bodyEl('peminjaman-modal-notes'), 'Butuh untuk rapat')
    await flushPromises()
    bodyEl('peminjaman-modal-submit').click()
    await flushPromises()

    expect(borrowMock).not.toHaveBeenCalled()
  })
})
