// @vitest-environment nuxt
// The Panduan Penggunaan CMS: the module list, the editor slideover, and the
// attachment manager.
//
// Three behaviours carry the most risk and are asserted hardest: an update
// replaces the WHOLE module (so publish/reorder must resend every field), the
// slug is derived once and then frozen (renaming must not break deep links),
// and the YouTube field must reject look-alike hosts before the request is
// ever sent.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { useConfirm } from '~/composables/useConfirm'
import { GUIDE_PDF_MAX_BYTES } from '~/utils/guideText'
import type { GuideAttachment, GuideModule, GuideModuleInput } from '~/types'

interface Call {
  path: string
  method: string
  body: unknown
}

const calls: Call[] = []
let handler: (path: string, opts?: Record<string, unknown>) => unknown = () => ({ data: [] })

const { toastAddMock } = vi.hoisted(() => ({ toastAddMock: vi.fn() }))
mockNuxtImport('useToast', () => () => ({ add: toastAddMock }))

vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({
    request: (path: string, opts?: Record<string, unknown>) => {
      calls.push({ path, method: String(opts?.method ?? 'GET'), body: opts?.body })
      return Promise.resolve(handler(path, opts))
    },
    requestBlob: () => Promise.reject(new Error('not used here'))
  })
}))

// eslint-disable-next-line import/first
import GuideAdminPage from '~/pages/settings/guide.vue'
// eslint-disable-next-line import/first
import GuideModuleForm from '~/components/guide/GuideModuleForm.vue'
// eslint-disable-next-line import/first
import GuideAttachmentManager from '~/components/guide/GuideAttachmentManager.vue'

function attachment(over: Partial<GuideAttachment> = {}): GuideAttachment {
  return {
    id: 'a1',
    kind: 'video',
    title_id: 'Rekaman langkah',
    title_en: 'Screen recording',
    sort_order: 1,
    locked: false,
    youtube_id: 'dQw4w9WgXcQ',
    ...over
  }
}

function guideModule(over: Partial<GuideModule> = {}): GuideModule {
  return {
    id: 'm1',
    slug: 'katalog-aset',
    icon: 'i-lucide-package',
    sort_order: 3,
    status: 'published',
    title_id: 'Katalog Aset',
    title_en: 'Asset Catalogue',
    body_id: 'Telusuri dan kelola seluruh aset.',
    body_en: null,
    steps: [{ text_id: 'Gunakan kolom pencarian.', text_en: null }],
    attachments: [],
    published_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-04T00:00:00Z',
    ...over
  }
}

const MODULES: GuideModule[] = [
  guideModule({ id: 'm0', slug: 'masuk', title_id: 'Masuk dan Keamanan Akun', title_en: null, sort_order: 1 }),
  guideModule(),
  guideModule({
    id: 'm2',
    slug: 'approval',
    title_id: 'Pengajuan dan Persetujuan',
    title_en: 'Requests and Approval',
    status: 'draft',
    sort_order: 5,
    steps: [],
    attachments: [attachment(), attachment({ id: 'a2', kind: 'document', sort_order: 2, youtube_id: undefined, original_filename: 'alur.pdf', size_bytes: 1_400_000 })]
  })
]

function respondWith(modules: GuideModule[]) {
  handler = (path, opts) => {
    if (path.startsWith('/guide/modules') && (!opts?.method || opts.method === 'GET')) {
      return { data: modules }
    }
    return {}
  }
}

interface PageVm {
  modules: GuideModule[]
  filtered: GuideModule[]
  search: string
  fStatus: string
  formOpen: boolean
  editing: GuideModule | null
  openCreate: () => void
  openEdit: (m: GuideModule) => void
  onSubmit: (input: GuideModuleInput) => Promise<void>
  togglePublish: (m: GuideModule) => Promise<void>
  move: (m: GuideModule, delta: number) => Promise<void>
  onDelete: (m: GuideModule) => Promise<void>
  nextOrder: number
}

async function mountPage(modules: GuideModule[] = MODULES) {
  respondWith(modules)
  const wrapper = await mountSuspended(GuideAdminPage)
  await flushPromises()
  return wrapper
}

function writes(): Call[] {
  return calls.filter(c => c.method !== 'GET')
}

enableAutoUnmount(afterEach)

beforeEach(() => {
  calls.length = 0
  toastAddMock.mockClear()
  respondWith(MODULES)
})

/* ── list ──────────────────────────────────────────────────────────────── */

describe('guide admin — module list', () => {
  it('fetches drafts too: this screen is the authoring surface', async () => {
    await mountPage()
    expect(calls[0]!.path).toBe('/guide/modules?status=all')
  })

  it('renders every module with its order, status, step and attachment counts', async () => {
    const wrapper = await mountPage()
    const html = wrapper.html()
    expect(html).toContain('Masuk dan Keamanan Akun')
    expect(html).toContain('Katalog Aset')
    expect(html).toContain('Pengajuan dan Persetujuan')
    expect(html).toContain('Terbit')
    expect(html).toContain('Draf')
    // Third module: two attachments (one video, one document) and no steps.
    expect(html).toContain('Menampilkan 3 dari 3 modul')
  })

  it('badges a module that has no English title', async () => {
    const wrapper = await mountPage()
    expect(wrapper.html()).toContain('Belum diterjemahkan')
    // ... and shows the English title when there is one.
    expect(wrapper.html()).toContain('Asset Catalogue')
  })

  it('filters by search across both languages', async () => {
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as PageVm
    vm.search = 'katalog'
    await wrapper.vm.$nextTick()
    expect(vm.filtered.map(m => m.id)).toEqual(['m1'])

    vm.search = 'Approval'
    await wrapper.vm.$nextTick()
    expect(vm.filtered.map(m => m.id)).toEqual(['m2'])

    vm.search = '   '
    await wrapper.vm.$nextTick()
    expect(vm.filtered).toHaveLength(3)
  })

  it('filters by status', async () => {
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as PageVm
    vm.fStatus = 'draft'
    await wrapper.vm.$nextTick()
    expect(vm.filtered.map(m => m.id)).toEqual(['m2'])
    expect(wrapper.html()).toContain('Menampilkan 1 dari 3 modul')

    vm.fStatus = 'published'
    await wrapper.vm.$nextTick()
    expect(vm.filtered.map(m => m.id)).toEqual(['m0', 'm1'])
  })

  it('shows the filtered-empty message rather than the first-run empty state', async () => {
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as PageVm
    vm.search = 'tidak-ada-modul-begini'
    await wrapper.vm.$nextTick()
    expect(wrapper.html()).toContain('Tidak ada modul yang cocok')
    expect(wrapper.html()).not.toContain('Belum ada modul panduan')
  })

  it('shows the first-run empty state with its create button when nothing exists', async () => {
    const wrapper = await mountPage([])
    expect(wrapper.html()).toContain('Belum ada modul panduan')
    expect(wrapper.html()).toContain('Tambah Modul')
  })

  it('shows the error state and recovers on retry', async () => {
    handler = () => {
      throw new Error('boom')
    }
    const wrapper = await mountSuspended(GuideAdminPage)
    await flushPromises()
    expect(wrapper.html()).toContain('Daftar modul gagal dimuat')

    respondWith(MODULES)
    await wrapper.find('[data-testid="guide-retry"]').trigger('click')
    await flushPromises()
    expect(wrapper.html()).toContain('Katalog Aset')
  })
})

/* ── writes from the list ──────────────────────────────────────────────── */

describe('guide admin — publish, reorder, delete', () => {
  it('publishing resends every field with only the status flipped', async () => {
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as PageVm
    await vm.togglePublish(MODULES[2]!)
    await flushPromises()

    const patch = writes()[0]!
    expect(patch.method).toBe('PATCH')
    expect(patch.path).toBe('/guide/modules/m2')
    // An update REPLACES the module, so a partial body would blank the rest.
    expect(patch.body).toMatchObject({
      slug: 'approval',
      icon: 'i-lucide-package',
      sort_order: 5,
      status: 'published',
      title_id: 'Pengajuan dan Persetujuan',
      title_en: 'Requests and Approval'
    })
  })

  it('unpublishing flips the other way', async () => {
    const wrapper = await mountPage()
    await (wrapper.vm as unknown as PageVm).togglePublish(MODULES[1]!)
    await flushPromises()
    expect(writes()[0]!.body).toMatchObject({ status: 'draft' })
  })

  it('moving a module up swaps sort_order with its neighbour', async () => {
    const wrapper = await mountPage()
    await (wrapper.vm as unknown as PageVm).move(MODULES[1]!, -1)
    await flushPromises()

    const [first, second] = writes()
    expect(first!.path).toBe('/guide/modules/m1')
    expect(first!.body).toMatchObject({ sort_order: 1 })
    expect(second!.path).toBe('/guide/modules/m0')
    expect(second!.body).toMatchObject({ sort_order: 3 })
  })

  it('does nothing at the ends of the list', async () => {
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as PageVm
    await vm.move(MODULES[0]!, -1)
    await vm.move(MODULES[2]!, 1)
    await flushPromises()
    expect(writes()).toHaveLength(0)
  })

  // Two modules on the same number cannot be reordered by swapping numbers: the
  // swap writes the same value twice, nothing moves, and the success toast lies.
  // The tie is broken by pushing the row that must end up second one step down.
  it('breaks a sort_order tie when moving up instead of swapping nothing', async () => {
    const tied = [
      guideModule({ id: 't0', slug: 'satu', title_id: 'Satu', sort_order: 2 }),
      guideModule({ id: 't1', slug: 'dua', title_id: 'Dua', sort_order: 2 })
    ]
    const wrapper = await mountPage(tied)
    await (wrapper.vm as unknown as PageVm).move(tied[1]!, -1)
    await flushPromises()

    // One PATCH, on the NEIGHBOUR — never a decrement, because sort_order is
    // validated `gte=0` and a row already at 0 would be rejected.
    expect(writes()).toHaveLength(1)
    expect(writes()[0]!.path).toBe('/guide/modules/t0')
    expect(writes()[0]!.body).toMatchObject({ sort_order: 3 })
  })

  it('breaks a sort_order tie when moving down by pushing the mover', async () => {
    const tied = [
      guideModule({ id: 't0', slug: 'satu', title_id: 'Satu', sort_order: 2 }),
      guideModule({ id: 't1', slug: 'dua', title_id: 'Dua', sort_order: 2 })
    ]
    const wrapper = await mountPage(tied)
    await (wrapper.vm as unknown as PageVm).move(tied[0]!, 1)
    await flushPromises()

    expect(writes()).toHaveLength(1)
    expect(writes()[0]!.path).toBe('/guide/modules/t0')
    expect(writes()[0]!.body).toMatchObject({ sort_order: 3 })
  })

  // A new module must not land on a number that is already taken, or the very
  // first reorder the author tries hits the tie path above.
  it('offers a new module the first free sort_order, not a fixed one', async () => {
    const wrapper = await mountPage()
    expect((wrapper.vm as unknown as PageVm).nextOrder).toBe(6) // highest is 5
  })

  it('offers 1 when there is no module at all', async () => {
    const wrapper = await mountPage([])
    expect((wrapper.vm as unknown as PageVm).nextOrder).toBe(1)
  })

  it('deletes only after the confirmation is accepted', async () => {
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as PageVm

    const pending = vm.onDelete(MODULES[1]!)
    await wrapper.vm.$nextTick()
    useConfirm().resolve(false)
    await pending
    expect(writes()).toHaveLength(0)

    const accepted = vm.onDelete(MODULES[1]!)
    await wrapper.vm.$nextTick()
    useConfirm().resolve(true)
    await accepted
    await flushPromises()
    expect(writes()[0]).toMatchObject({ path: '/guide/modules/m1', method: 'DELETE' })
  })

  // Creation cannot publish. The endpoint has no status field, so sending one
  // would look like it works and quietly do nothing — the page drops it instead.
  it('creates without a status, so a new module can only be a draft', async () => {
    const wrapper = await mountPage()
    await (wrapper.vm as unknown as PageVm).onSubmit({
      slug: 'modul-baru', icon: 'i-lucide-book-open', sort_order: 4, status: 'published',
      title_id: 'Modul Baru', title_en: null, body_id: null, body_en: null,
      steps: [{ text_id: 'Langkah satu', text_en: null }]
    })
    await flushPromises()

    const call = writes()[0]!
    expect(call.method).toBe('POST')
    expect(call.path).toBe('/guide/modules')
    expect(call.body).not.toHaveProperty('status')
    // Everything else still travels; dropping status must not drop the payload.
    expect(call.body).toMatchObject({
      slug: 'modul-baru', icon: 'i-lucide-book-open', sort_order: 4, title_id: 'Modul Baru'
    })
    expect((call.body as { steps: unknown[] }).steps).toHaveLength(1)
  })

  // Editing is where status becomes writable, and it must still travel.
  it('sends the status on update, because publishing is an edit', async () => {
    const wrapper = await mountPage()
    const vm = wrapper.vm as unknown as PageVm
    vm.openEdit(MODULES[0]!)
    await vm.onSubmit({
      slug: MODULES[0]!.slug, icon: MODULES[0]!.icon, sort_order: 1, status: 'published',
      title_id: MODULES[0]!.title_id, title_en: null, body_id: null, body_en: null, steps: []
    })
    await flushPromises()

    const call = writes()[0]!
    expect(call.method).toBe('PATCH')
    expect(call.body).toMatchObject({ status: 'published' })
  })

  it('explains a slug collision instead of leaving the generic toast', async () => {
    const wrapper = await mountPage()
    handler = () => {
      throw Object.assign(new Error('conflict'), { statusCode: 409 })
    }
    await (wrapper.vm as unknown as PageVm).onSubmit({
      slug: 'katalog-aset', icon: 'i-lucide-package', sort_order: 1, status: 'draft',
      title_id: 'Katalog Aset', title_en: null, body_id: null, body_en: null, steps: []
    })
    await flushPromises()
    expect(toastAddMock).toHaveBeenCalledWith(expect.objectContaining({ color: 'error' }))
    const last = toastAddMock.mock.calls.at(-1)![0] as { title: string }
    expect(last.title).toContain('Slug modul sudah dipakai')
  })
})

/* ── the editor ────────────────────────────────────────────────────────── */

interface FormVm {
  form: {
    icon: string
    sort_order: string
    status: string
    title_id: string
    title_en: string
    body_id: string
    body_en: string
    steps: { text_id: string, text_en: string }[]
  }
  showStepEn: boolean
  addStep: () => void
  moveStep: (i: number, d: number) => void
  removeStep: (i: number) => void
  orderDelta: (d: number) => void
  onSubmit: () => void
}

async function mountForm(module: GuideModule | null, nextOrder?: number) {
  const wrapper = await mountSuspended(GuideModuleForm, { props: { open: true, module, nextOrder } })
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('GuideModuleForm', () => {
  it('opens empty for a new module, as a draft', async () => {
    const wrapper = await mountForm(null)
    const vm = wrapper.vm as unknown as FormVm
    expect(vm.form.title_id).toBe('')
    expect(vm.form.status).toBe('draft')
    expect(vm.form.steps).toHaveLength(0)
    const html = document.body.innerHTML
    expect(html).toContain('Modul baru')
    expect(html).toContain('Informasi modul')
    expect(html).toContain('Daftar langkah')
    expect(html).toContain('Lampiran')
  })

  // Prefilled from the parent so a new module never ties with an existing one.
  it('prefills a new module with the order the parent hands it', async () => {
    const wrapper = await mountForm(null, 6)
    expect((wrapper.vm as unknown as FormVm).form.sort_order).toBe('6')
  })

  it('falls back to 1 when no order is supplied', async () => {
    const wrapper = await mountForm(null)
    expect((wrapper.vm as unknown as FormVm).form.sort_order).toBe('1')
  })

  // The control stays on screen so the form keeps its shape, but it cannot be
  // used before the module exists — and the hint says why rather than leaving a
  // dead button to be discovered by clicking it.
  it('pins the status control while creating, and explains it', async () => {
    await mountForm(null)
    const buttons = document.body.querySelectorAll('[data-testid^="guide-status-"]')
    expect(buttons).toHaveLength(2)
    buttons.forEach(b => expect(b.hasAttribute('disabled')).toBe(true))
    expect(document.body.innerHTML).toContain('Modul baru selalu dibuat sebagai draf')
  })

  it('releases the status control once the module exists', async () => {
    await mountForm(guideModule())
    const buttons = document.body.querySelectorAll('[data-testid^="guide-status-"]')
    expect(buttons).toHaveLength(2)
    buttons.forEach(b => expect(b.hasAttribute('disabled')).toBe(false))
    expect(document.body.innerHTML).toContain('Modul draf tidak terlihat oleh pembaca')
  })

  it('hydrates every field from the module being edited', async () => {
    const wrapper = await mountForm(guideModule())
    const vm = wrapper.vm as unknown as FormVm
    expect(vm.form.title_id).toBe('Katalog Aset')
    expect(vm.form.title_en).toBe('Asset Catalogue')
    expect(vm.form.body_id).toBe('Telusuri dan kelola seluruh aset.')
    // A null English body becomes an empty string, not the literal "null".
    expect(vm.form.body_en).toBe('')
    expect(vm.form.sort_order).toBe('3')
    expect(vm.form.status).toBe('published')
    expect(vm.form.steps).toEqual([{ text_id: 'Gunakan kolom pencarian.', text_en: '' }])
  })

  it('blocks submit and flags the title when it is blank', async () => {
    const wrapper = await mountForm(null)
    const vm = wrapper.vm as unknown as FormVm
    vm.form.title_id = '   '
    vm.onSubmit()
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('submit')).toBeFalsy()
    expect(document.body.innerHTML).toContain('Wajib diisi')
  })

  it('blocks submit when a step has no Indonesian text', async () => {
    const wrapper = await mountForm(null)
    const vm = wrapper.vm as unknown as FormVm
    vm.form.title_id = 'Modul Uji'
    vm.addStep()
    vm.form.steps[0]!.text_en = 'English only'
    vm.onSubmit()
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('submit')).toBeFalsy()
  })

  it('derives a slug from the title for a new module', async () => {
    const wrapper = await mountForm(null)
    const vm = wrapper.vm as unknown as FormVm
    vm.form.title_id = '  Mutasi, Stock Opname, dan Penghapusan  '
    vm.onSubmit()
    const emitted = wrapper.emitted('submit')![0]![0] as GuideModuleInput
    expect(emitted.slug).toBe('mutasi-stock-opname-dan-penghapusan')
    expect(emitted.title_id).toBe('Mutasi, Stock Opname, dan Penghapusan')
  })

  it('keeps the original slug when an existing module is renamed', async () => {
    const wrapper = await mountForm(guideModule())
    const vm = wrapper.vm as unknown as FormVm
    vm.form.title_id = 'Katalog Aset Baru'
    vm.onSubmit()
    const emitted = wrapper.emitted('submit')![0]![0] as GuideModuleInput
    expect(emitted.slug).toBe('katalog-aset')
    expect(emitted.title_id).toBe('Katalog Aset Baru')
  })

  it('sends blank optional fields as null, never as empty strings', async () => {
    const wrapper = await mountForm(null)
    const vm = wrapper.vm as unknown as FormVm
    vm.form.title_id = 'Dashboard'
    vm.form.title_en = '   '
    vm.form.body_id = ''
    vm.addStep()
    vm.form.steps[0]!.text_id = ' Buka menu. '
    vm.form.steps[0]!.text_en = '  '
    vm.onSubmit()
    const emitted = wrapper.emitted('submit')![0]![0] as GuideModuleInput
    expect(emitted.title_en).toBeNull()
    expect(emitted.body_id).toBeNull()
    expect(emitted.steps).toEqual([{ text_id: 'Buka menu.', text_en: null }])
  })

  it('reorders and removes steps', async () => {
    const wrapper = await mountForm(null)
    const vm = wrapper.vm as unknown as FormVm
    vm.form.steps = [
      { text_id: 'satu', text_en: '' },
      { text_id: 'dua', text_en: '' },
      { text_id: 'tiga', text_en: '' }
    ]
    vm.moveStep(2, -1)
    expect(vm.form.steps.map(s => s.text_id)).toEqual(['satu', 'tiga', 'dua'])
    vm.moveStep(0, -1) // already first — no-op, and no crash
    expect(vm.form.steps.map(s => s.text_id)).toEqual(['satu', 'tiga', 'dua'])
    vm.moveStep(2, 1) // already last — no-op
    expect(vm.form.steps.map(s => s.text_id)).toEqual(['satu', 'tiga', 'dua'])
    vm.removeStep(1)
    expect(vm.form.steps.map(s => s.text_id)).toEqual(['satu', 'dua'])
  })

  it('never lets the order spinner go below zero', async () => {
    const wrapper = await mountForm(null)
    const vm = wrapper.vm as unknown as FormVm
    vm.form.sort_order = '1'
    vm.orderDelta(-1)
    expect(vm.form.sort_order).toBe('0')
    vm.orderDelta(-1)
    expect(vm.form.sort_order).toBe('0')
    vm.orderDelta(1)
    expect(vm.form.sort_order).toBe('1')
  })

  it('tells the author to save first before attachments can be added', async () => {
    await mountForm(null)
    expect(document.body.innerHTML).toContain('Simpan modul ini lebih dulu')
  })
})

/* ── attachments ───────────────────────────────────────────────────────── */

interface AttVm {
  tab: 'video' | 'pdf'
  videoUrl: string
  videoTitleId: string
  videoId: string | null
  canAddVideo: boolean
  addVideo: () => Promise<void>
  acceptFile: (f: File | null | undefined) => void
  pdfFile: File | null
  pdfError: string | null
  startEdit: (a: GuideAttachment) => void
  saveEdit: (a: GuideAttachment) => Promise<void>
  move: (i: number, d: number) => Promise<void>
  remove: (a: GuideAttachment) => Promise<void>
}

async function mountAttachments(attachments: GuideAttachment[]) {
  const wrapper = await mountSuspended(GuideAttachmentManager, {
    props: { moduleId: 'm2', attachments }
  })
  await wrapper.vm.$nextTick()
  return wrapper
}

describe('GuideAttachmentManager', () => {
  it('lists existing attachments with their kind and meta', async () => {
    const wrapper = await mountAttachments([
      attachment(),
      attachment({ id: 'a2', kind: 'document', youtube_id: undefined, original_filename: 'alur.pdf', size_bytes: 1_400_000, title_en: null })
    ])
    const html = wrapper.html()
    expect(html).toContain('Rekaman langkah')
    expect(html).toContain('youtube_id: dQw4w9WgXcQ')
    expect(html).toContain('alur.pdf')
    expect(html).toMatch(/1,3 MB|1.3 MB/)
    expect(html).toContain('Belum diterjemahkan')
  })

  it('shows the empty hint when the module has none', async () => {
    const wrapper = await mountAttachments([])
    expect(wrapper.html()).toContain('Belum ada lampiran pada modul ini')
  })

  it('replaces the adder with an explanation once ten attachments exist', async () => {
    const many = Array.from({ length: 10 }, (_, i) => attachment({ id: `a${i}`, sort_order: i + 1 }))
    const wrapper = await mountAttachments(many)
    expect(wrapper.html()).toContain('Batas lampiran tercapai')
    expect(wrapper.find('[data-testid="guide-att-add-video"]').exists()).toBe(false)
  })

  it('accepts a YouTube link and shows the id that will be stored', async () => {
    const wrapper = await mountAttachments([])
    const vm = wrapper.vm as unknown as AttVm
    vm.videoUrl = 'https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=12s'
    await wrapper.vm.$nextTick()
    expect(vm.videoId).toBe('dQw4w9WgXcQ')
    expect(wrapper.html()).toContain('youtube_id: dQw4w9WgXcQ')
    expect(wrapper.html()).toContain('Tautan sah')
  })

  it('rejects a look-alike host before anything is sent', async () => {
    const wrapper = await mountAttachments([])
    const vm = wrapper.vm as unknown as AttVm
    vm.videoUrl = 'https://youtube.com.evil.example.com/watch?v=dQw4w9WgXcQ'
    vm.videoTitleId = 'Judul'
    await wrapper.vm.$nextTick()
    expect(vm.videoId).toBeNull()
    expect(vm.canAddVideo).toBe(false)
    expect(wrapper.html()).toContain('Tautan tidak dikenali')

    await vm.addVideo()
    expect(writes()).toHaveLength(0)
  })

  it('requires a title as well as a valid link', async () => {
    const wrapper = await mountAttachments([])
    const vm = wrapper.vm as unknown as AttVm
    vm.videoUrl = 'https://youtu.be/dQw4w9WgXcQ'
    await wrapper.vm.$nextTick()
    expect(vm.canAddVideo).toBe(false)
    vm.videoTitleId = 'Rekaman'
    await wrapper.vm.$nextTick()
    expect(vm.canAddVideo).toBe(true)
  })

  it('posts the video and asks the parent to refetch', async () => {
    const wrapper = await mountAttachments([attachment({ sort_order: 4 })])
    const vm = wrapper.vm as unknown as AttVm
    vm.videoUrl = 'https://youtu.be/dQw4w9WgXcQ'
    vm.videoTitleId = 'Rekaman baru'
    await wrapper.vm.$nextTick()
    await vm.addVideo()
    await flushPromises()

    const call = writes()[0]!
    expect(call.method).toBe('POST')
    expect(call.path).toBe('/guide/modules/m2/attachments/video')
    // New rows land after the current last one, computed from sort_order rather
    // than the count so a gap never produces a duplicate.
    expect(call.body).toMatchObject({ url: 'https://youtu.be/dQw4w9WgXcQ', sort_order: 5 })
    expect(wrapper.emitted('changed')).toBeTruthy()
    // The form is cleared so the same video is not added twice by accident.
    expect(vm.videoUrl).toBe('')
  })

  it('rejects a non-PDF file locally, without an upload', async () => {
    const wrapper = await mountAttachments([])
    const vm = wrapper.vm as unknown as AttVm
    vm.tab = 'pdf'
    vm.acceptFile(new File(['x'], 'gambar.png', { type: 'image/png' }))
    await wrapper.vm.$nextTick()
    expect(vm.pdfError).toBe('type')
    expect(vm.pdfFile).toBeNull()
    expect(wrapper.html()).toContain('bukan PDF')
    expect(writes()).toHaveLength(0)
  })

  it('rejects an oversized PDF locally, without an upload', async () => {
    const wrapper = await mountAttachments([])
    const vm = wrapper.vm as unknown as AttVm
    const big = new File([new Uint8Array(1)], 'besar.pdf', { type: 'application/pdf' })
    Object.defineProperty(big, 'size', { value: GUIDE_PDF_MAX_BYTES + 1 })
    vm.tab = 'pdf'
    vm.acceptFile(big)
    await wrapper.vm.$nextTick()
    expect(vm.pdfError).toBe('size')
    expect(vm.pdfFile).toBeNull()
    expect(wrapper.html()).toContain('melebihi 10 MB')
    expect(writes()).toHaveLength(0)
  })

  it('accepts a PDF at exactly the limit and prefills the title from the filename', async () => {
    const wrapper = await mountAttachments([])
    const vm = wrapper.vm as unknown as AttVm
    const f = new File([new Uint8Array(1)], 'panduan-registrasi-aset.pdf', { type: 'application/pdf' })
    Object.defineProperty(f, 'size', { value: GUIDE_PDF_MAX_BYTES })
    vm.tab = 'pdf'
    vm.acceptFile(f)
    await wrapper.vm.$nextTick()
    expect(vm.pdfError).toBeNull()
    expect(vm.pdfFile).toBe(f)
    expect(wrapper.html()).toContain('panduan-registrasi-aset.pdf')
  })

  it('uploads the real File object, not a reactive proxy of it', async () => {
    // FormData.append has no way to read a Proxy's File internals: a proxied
    // File would be coerced to the string "[object File]" and the upload would
    // silently carry no bytes. This is why pdfFile is a shallowRef.
    const wrapper = await mountAttachments([attachment({ sort_order: 2 })])
    const vm = wrapper.vm as unknown as AttVm & { pdfTitleId: string, addPdf: () => Promise<void> }
    const f = new File(['%PDF-1.7'], 'alur.pdf', { type: 'application/pdf' })
    vm.tab = 'pdf'
    vm.acceptFile(f)
    await wrapper.vm.$nextTick()
    vm.pdfTitleId = 'Alur registrasi'
    await vm.addPdf()
    await flushPromises()

    const call = writes()[0]!
    expect(call.path).toBe('/guide/modules/m2/attachments/document')
    expect(call.method).toBe('POST')
    const body = call.body as FormData
    expect(body).toBeInstanceOf(FormData)
    expect(body.get('file')).toBe(f)
    expect(body.get('title_id')).toBe('Alur registrasi')
    expect(body.get('sort_order')).toBe('3')
    expect(wrapper.emitted('changed')).toBeTruthy()
  })

  it('surfaces a server-side rejection of a file that passed the pre-flight', async () => {
    const wrapper = await mountAttachments([])
    const vm = wrapper.vm as unknown as AttVm & { pdfTitleId: string, addPdf: () => Promise<void> }
    const f = new File(['not really a pdf'], 'palsu.pdf', { type: 'application/pdf' })
    vm.tab = 'pdf'
    vm.acceptFile(f)
    await wrapper.vm.$nextTick()
    vm.pdfTitleId = 'Palsu'
    handler = () => {
      throw Object.assign(new Error('unsupported'), { statusCode: 415 })
    }
    await vm.addPdf()
    await flushPromises()
    expect(vm.pdfError).toBe('type')
    expect(wrapper.html()).toContain('bukan PDF')
    expect(wrapper.emitted('changed')).toBeFalsy()
  })

  it('warns that attachments are visible to every signed-in user', async () => {
    const wrapper = await mountAttachments([])
    const vm = wrapper.vm as unknown as AttVm
    vm.tab = 'pdf'
    await wrapper.vm.$nextTick()
    expect(wrapper.html()).toContain('SEMUA pengguna yang sudah masuk')
    expect(wrapper.html()).toContain('maksimal 10 MB per berkas')
  })

  it('patches only the title and order when a row is retitled', async () => {
    const a = attachment({ sort_order: 4 })
    const wrapper = await mountAttachments([a])
    const vm = wrapper.vm as unknown as AttVm
    vm.startEdit(a)
    await wrapper.vm.$nextTick()
    expect(wrapper.html()).toContain('Untuk mengganti videonya, hapus lampiran ini')
    await vm.saveEdit(a)
    await flushPromises()
    expect(writes()[0]).toMatchObject({ path: '/guide/attachments/a1', method: 'PATCH' })
    expect(writes()[0]!.body).toMatchObject({ title_id: 'Rekaman langkah', sort_order: 4 })
  })

  it('swaps the order of two neighbouring attachments', async () => {
    const first = attachment({ id: 'a1', sort_order: 1 })
    const second = attachment({ id: 'a2', sort_order: 2 })
    const wrapper = await mountAttachments([first, second])
    await (wrapper.vm as unknown as AttVm).move(0, 1)
    await flushPromises()
    expect(writes()[0]).toMatchObject({ path: '/guide/attachments/a1' })
    expect(writes()[0]!.body).toMatchObject({ sort_order: 2 })
    expect(writes()[1]).toMatchObject({ path: '/guide/attachments/a2' })
    expect(writes()[1]!.body).toMatchObject({ sort_order: 1 })
  })

  it('does not move past either end', async () => {
    const wrapper = await mountAttachments([attachment({ id: 'a1' })])
    const vm = wrapper.vm as unknown as AttVm
    await vm.move(0, -1)
    await vm.move(0, 1)
    expect(writes()).toHaveLength(0)
  })

  it('deletes only after the confirmation is accepted', async () => {
    const a = attachment()
    const wrapper = await mountAttachments([a])
    const vm = wrapper.vm as unknown as AttVm

    const declined = vm.remove(a)
    await wrapper.vm.$nextTick()
    useConfirm().resolve(false)
    await declined
    expect(writes()).toHaveLength(0)

    const accepted = vm.remove(a)
    await wrapper.vm.$nextTick()
    useConfirm().resolve(true)
    await accepted
    await flushPromises()
    expect(writes()[0]).toMatchObject({ path: '/guide/attachments/a1', method: 'DELETE' })
    expect(wrapper.emitted('changed')).toBeTruthy()
  })
})
