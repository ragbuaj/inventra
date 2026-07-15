// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'

// Unmounts every wrapper after each test — required so a FormModal's
// teleported (document.body) content from one test doesn't leak into the
// next (see master-reference.spec.ts, which hit the same issue).
enableAutoUnmount(afterEach)

// useAccount's getProfile/updateProfile/requestEmailChange now hit the real
// backend via useApiClient — stub the HTTP client so account.vue's mount
// doesn't try to reach :8080 (per the wiring-composable-breaks-consumer-tests
// memory). requestMock is path-aware so different endpoints can be asserted
// on and made to fail independently.
interface ProfileApiResponse {
  id: string
  name: string
  email: string
  phone: string | null
  role_id: string
  role_name: string | null
  office_id: string | null
  office_name: string | null
  employee_id: string | null
  employee_name: string | null
  status: string
  avatar_url: string | null
  google_linked: boolean
  joined_at: string
}

const defaultProfileResponse: ProfileApiResponse = {
  id: 'u1',
  name: 'Andi Saputra',
  email: 'andi@inventra.local',
  phone: '0812-3456-7890',
  role_id: 'r1',
  role_name: 'Asset Manager',
  office_id: null,
  office_name: null,
  employee_id: null,
  employee_name: null,
  status: 'active',
  avatar_url: null,
  google_linked: false,
  joined_at: '2024-03-12T00:00:00Z'
}

let profileResponse: ProfileApiResponse = { ...defaultProfileResponse }
let emailChangeImpl: (opts?: Record<string, unknown>) => Promise<unknown> = () => Promise.resolve({ status: 'ok' })

const requestMock = vi.fn((path: string, opts?: Record<string, unknown>) => {
  if (path === '/auth/email/change-request') return emailChangeImpl(opts)
  if (path === '/auth/profile' && opts?.method === 'PUT') {
    const body = opts.body as { name: string, phone: string }
    profileResponse = { ...profileResponse, name: body.name, phone: body.phone }
    return Promise.resolve(profileResponse)
  }
  return Promise.resolve(profileResponse)
})
vi.mock('~/composables/useApiClient', () => ({
  useApiClient: () => ({ request: requestMock, requestBlob: vi.fn(), refreshToken: vi.fn() })
}))

// eslint-disable-next-line import/first
import Akun from '~/pages/account.vue'
// eslint-disable-next-line import/first
import { useAuthStore } from '~/stores/auth'

function user() {
  useAuthStore().setSession('t', { id: '1', name: 'Andi Saputra', email: 'andi@inventra.local', role_id: 'r', role_name: 'Asset Manager', office_id: null }, ['*'])
}

async function mountLoaded() {
  const w = await mountSuspended(Akun, { route: '/account' })
  await new Promise(r => setTimeout(r, 500))
  await flushPromises()
  return w
}

// FormModal wraps UModal, which teleports its content to document.body — it
// lives outside `wrapper`'s own DOM subtree, so it must be found via
// document.body rather than wrapper.find() (see master-reference.spec.ts).
function bodyEl(testid: string): HTMLElement {
  const el = document.body.querySelector(`[data-testid="${testid}"]`)
  expect(el, `expected [data-testid="${testid}"] in document.body`).toBeTruthy()
  return el as HTMLElement
}

function bodyElExists(testid: string): boolean {
  return !!document.body.querySelector(`[data-testid="${testid}"]`)
}

function setNativeValue(el: HTMLElement, value: string) {
  const input = el as HTMLInputElement
  input.value = value
  input.dispatchEvent(new Event('input'))
}

function findBodyButtonByText(text: string): HTMLElement {
  const btn = Array.from(document.body.querySelectorAll('button')).find(b => b.textContent?.trim() === text)
  expect(btn, `expected a <button> with text "${text}" in document.body`).toBeTruthy()
  return btn as HTMLElement
}

function clickEl(el: HTMLElement) {
  el.dispatchEvent(new MouseEvent('click', { bubbles: true }))
}

describe('Account page — Profil tab', () => {
  beforeEach(() => {
    useAuthStore().clear()
    user()
    profileResponse = { ...defaultProfileResponse }
    emailChangeImpl = () => Promise.resolve({ status: 'ok' })
    requestMock.mockClear()
  })

  it('renders the profile header and personal data', async () => {
    const w = await mountLoaded()
    expect(w.text()).toContain('Andi Saputra')
    expect(w.text()).toContain('Asset Manager')
    expect(w.text()).toContain('Data Diri')
  })

  it('shows the required error when saving with an empty name', async () => {
    const w = await mountLoaded()
    await w.find('[data-testid="profile-edit"]').trigger('click')
    await flushPromises()
    const nameInput = w.find('[data-testid="profile-nama"]')
    await nameInput.setValue('')
    const saveBtn = w.find('[data-testid="profile-save"]')
    await saveBtn.trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Wajib diisi')
  })

  it('shows "—" for kantor/pegawai when the API returns empty strings (no fake data)', async () => {
    const w = await mountLoaded()
    expect(w.text()).toContain('—')
    // employee_id is null in the default mock, so pegawai must not show a name.
    expect(w.text()).not.toContain('Kantor Pusat')
  })

  it('renders the enriched kantor/pegawai names resolved by the API', async () => {
    profileResponse = { ...defaultProfileResponse, office_id: 'o1', office_name: 'Cabang Jakarta Selatan', employee_id: 'e1', employee_name: 'Andi Saputra' }
    const w = await mountLoaded()
    expect(w.text()).toContain('Cabang Jakarta Selatan')
    expect(w.text()).toContain('Pegawai Tertaut')
    // the employee name renders in the "Pegawai Tertaut" row, not just the header.
    expect(w.text()).toContain('Andi Saputra')
  })

  describe('view/edit state', () => {
    it('starts read-only: Edit button shown, Simpan/Batal hidden, inputs disabled', async () => {
      const w = await mountLoaded()
      expect(w.find('[data-testid="profile-edit"]').exists()).toBe(true)
      expect(w.find('[data-testid="profile-save"]').exists()).toBe(false)
      expect(w.find('[data-testid="profile-cancel"]').exists()).toBe(false)
      expect(w.find('[data-testid="profile-nama"]').attributes('disabled')).toBeDefined()
    })

    it('clicking Edit enables the name input and swaps to Simpan/Batal', async () => {
      const w = await mountLoaded()
      await w.find('[data-testid="profile-edit"]').trigger('click')
      await flushPromises()
      expect(w.find('[data-testid="profile-nama"]').attributes('disabled')).toBeUndefined()
      expect(w.find('[data-testid="profile-save"]').exists()).toBe(true)
      expect(w.find('[data-testid="profile-cancel"]').exists()).toBe(true)
      expect(w.find('[data-testid="profile-edit"]').exists()).toBe(false)
    })

    it('Batal reverts a changed value and returns to read-only without saving', async () => {
      const w = await mountLoaded()
      await w.find('[data-testid="profile-edit"]').trigger('click')
      await flushPromises()
      await w.find('[data-testid="profile-nama"]').setValue('Nama Berubah')
      await w.find('[data-testid="profile-cancel"]').trigger('click')
      await flushPromises()
      expect((w.find('[data-testid="profile-nama"]').element as HTMLInputElement).value).toBe('Andi Saputra')
      expect(w.find('[data-testid="profile-edit"]').exists()).toBe(true)
      expect(requestMock).not.toHaveBeenCalledWith('/auth/profile', expect.objectContaining({ method: 'PUT' }))
    })

    it('Simpan calls updateProfile with the entered values and returns to read-only', async () => {
      const w = await mountLoaded()
      await w.find('[data-testid="profile-edit"]').trigger('click')
      await flushPromises()
      await w.find('[data-testid="profile-nama"]').setValue('Andi Baru')
      await w.find('[data-testid="profile-save"]').trigger('click')
      await flushPromises()
      expect(requestMock).toHaveBeenCalledWith('/auth/profile', expect.objectContaining({
        method: 'PUT',
        body: { name: 'Andi Baru', phone: '0812-3456-7890' }
      }))
      expect(w.find('[data-testid="profile-edit"]').exists()).toBe(true)
      expect(w.find('[data-testid="profile-save"]').exists()).toBe(false)
    })
  })

  describe('telepon disabled when no linked employee', () => {
    it('is disabled with the hint when hasEmployee is false, even while editing', async () => {
      const w = await mountLoaded()
      await w.find('[data-testid="profile-edit"]').trigger('click')
      await flushPromises()
      expect(w.find('[data-testid="profile-telepon"]').attributes('disabled')).toBeDefined()
      expect(w.find('[data-testid="profile-telepon-hint"]').exists()).toBe(true)
    })

    it('is enabled while editing and the hint is hidden when hasEmployee is true', async () => {
      profileResponse = { ...defaultProfileResponse, employee_id: 'e1' }
      const w = await mountLoaded()
      await w.find('[data-testid="profile-edit"]').trigger('click')
      await flushPromises()
      expect(w.find('[data-testid="profile-telepon"]').attributes('disabled')).toBeUndefined()
      expect(w.find('[data-testid="profile-telepon-hint"]').exists()).toBe(false)
    })

    it('stays disabled (read-only) when hasEmployee is true but not editing', async () => {
      profileResponse = { ...defaultProfileResponse, employee_id: 'e1' }
      const w = await mountLoaded()
      expect(w.find('[data-testid="profile-telepon"]').attributes('disabled')).toBeDefined()
    })
  })

  describe('"Ubah Email" modal', () => {
    it('is hidden for Google-linked accounts', async () => {
      profileResponse = { ...defaultProfileResponse, google_linked: true }
      const w = await mountLoaded()
      expect(w.find('[data-testid="profile-change-email"]').exists()).toBe(false)
      expect(w.text()).toContain('dikelola oleh akun Google')
    })

    it('opens the modal with new-email + current-password fields for email accounts', async () => {
      const w = await mountLoaded()
      expect(w.find('[data-testid="profile-change-email"]').exists()).toBe(true)
      await w.find('[data-testid="profile-change-email"]').trigger('click')
      await flushPromises()
      expect(bodyElExists('change-email-input')).toBe(true)
      expect(bodyElExists('change-email-password')).toBe(true)
    })

    it('submit calls requestEmailChange with the entered new email + password, then shows the sent state', async () => {
      const w = await mountLoaded()
      await w.find('[data-testid="profile-change-email"]').trigger('click')
      await flushPromises()

      setNativeValue(bodyEl('change-email-input'), 'baru@inventra.local')
      setNativeValue(bodyEl('change-email-password'), 'secret123')
      await flushPromises()

      clickEl(findBodyButtonByText('Simpan'))
      await flushPromises()

      expect(requestMock).toHaveBeenCalledWith('/auth/email/change-request', expect.objectContaining({
        method: 'POST',
        body: { new_email: 'baru@inventra.local', current_password: 'secret123' }
      }))
      expect(bodyElExists('change-email-sent')).toBe(true)
      expect(document.body.textContent).toContain('baru@inventra.local')
      // form fields are gone once we've switched to the sent state
      expect(bodyElExists('change-email-input')).toBe(false)
    })

    it('a rejected submit (400 wrong password) shows the error text and keeps the modal open', async () => {
      emailChangeImpl = () => Promise.reject({ statusCode: 400, data: { error: 'password salah' } })
      const w = await mountLoaded()
      await w.find('[data-testid="profile-change-email"]').trigger('click')
      await flushPromises()

      setNativeValue(bodyEl('change-email-input'), 'baru@inventra.local')
      setNativeValue(bodyEl('change-email-password'), 'wrongpass')
      await flushPromises()

      clickEl(findBodyButtonByText('Simpan'))
      await flushPromises()

      expect(bodyElExists('change-email-error')).toBe(true)
      expect(bodyEl('change-email-error').textContent).toContain('password salah')
      // still in the form state, not the sent state, and the modal is still open
      expect(bodyElExists('change-email-sent')).toBe(false)
      expect(bodyElExists('change-email-input')).toBe(true)
    })

    it('validates the new email format before calling the API', async () => {
      const w = await mountLoaded()
      await w.find('[data-testid="profile-change-email"]').trigger('click')
      await flushPromises()

      setNativeValue(bodyEl('change-email-input'), 'not-an-email')
      setNativeValue(bodyEl('change-email-password'), 'secret123')
      await flushPromises()
      requestMock.mockClear()

      clickEl(findBodyButtonByText('Simpan'))
      await flushPromises()

      expect(requestMock).not.toHaveBeenCalledWith('/auth/email/change-request', expect.anything())
      expect(document.body.textContent).toContain('Format email tidak valid')
    })

    it('resend re-calls requestEmailChange and restarts the cooldown', async () => {
      const w = await mountLoaded()
      await w.find('[data-testid="profile-change-email"]').trigger('click')
      await flushPromises()
      setNativeValue(bodyEl('change-email-input'), 'baru@inventra.local')
      setNativeValue(bodyEl('change-email-password'), 'secret123')
      await flushPromises()
      clickEl(findBodyButtonByText('Simpan'))
      await flushPromises()

      expect(bodyElExists('change-email-sent')).toBe(true)
      // Cooldown just started — resend button is disabled with a countdown.
      const resendBtn = bodyEl('change-email-resend') as HTMLButtonElement
      expect(resendBtn.disabled).toBe(true)

      requestMock.mockClear()
      clickEl(resendBtn)
      await flushPromises()
      expect(requestMock).not.toHaveBeenCalledWith('/auth/email/change-request', expect.anything())
    })
  })
})
