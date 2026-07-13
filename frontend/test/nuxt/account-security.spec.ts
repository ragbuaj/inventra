// @vitest-environment nuxt
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { enableAutoUnmount, flushPromises } from '@vue/test-utils'

// Unmounts every wrapper after each test — required so a FormModal's
// teleported (document.body) content from one test doesn't leak into the
// next (see account-profile.spec.ts / master-reference.spec.ts).
enableAutoUnmount(afterEach)

// useAccount's getProfile/requestPasswordChange now hit the real backend via
// useApiClient — stub the HTTP client so account.vue's mount doesn't try to
// reach :8080 (per the wiring-composable-breaks-consumer-tests memory).
// requestMock is path-aware so the password-change-request endpoint can be
// made to fail independently of the profile fetch.
interface ProfileApiResponse {
  id: string
  name: string
  email: string
  phone: string | null
  role_id: string
  office_id: string | null
  employee_id: string | null
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
  office_id: null,
  employee_id: null,
  status: 'active',
  avatar_url: null,
  google_linked: false,
  joined_at: '2024-03-12T00:00:00Z'
}

let profileResponse: ProfileApiResponse = { ...defaultProfileResponse }
let pwChangeImpl: (opts?: Record<string, unknown>) => Promise<unknown> = () => Promise.resolve({ status: 'ok' })

const requestMock = vi.fn((path: string, opts?: Record<string, unknown>) => {
  if (path === '/auth/password/change-request') return pwChangeImpl(opts)
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
  const w = await mountSuspended(Akun, { props: {} })
  await new Promise(r => setTimeout(r, 500))
  await flushPromises()
  return w
}

// FormModal wraps UModal, which teleports its content to document.body — it
// lives outside `wrapper`'s own DOM subtree, so it must be found via
// document.body rather than wrapper.find().
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

async function openSecurityTab(w: Awaited<ReturnType<typeof mountLoaded>>) {
  const tabBtn = w.findAll('button').find(b => b.text().trim() === 'Keamanan')!
  await tabBtn.trigger('click')
  await flushPromises()
}

describe('Account page — Keamanan tab', () => {
  beforeEach(() => {
    useAuthStore().clear()
    user()
    profileResponse = { ...defaultProfileResponse }
    pwChangeImpl = () => Promise.resolve({ status: 'ok' })
    requestMock.mockClear()
  })

  it('shows a "Ganti Password" button and no inline password inputs', async () => {
    const w = await mountLoaded()
    await openSecurityTab(w)
    expect(w.find('[data-testid="security-change-password"]').exists()).toBe(true)
    // The old inline flow had 3 password inputs (old/new/confirm) directly on
    // the page; the new flow keeps password inputs only inside the modal,
    // which is closed (and not rendered) by default.
    expect(w.findAll('input[type="password"]')).toHaveLength(0)
    expect(w.text()).toContain('Sesi & Perangkat')
  })

  it('clicking the button opens a modal with a single current-password field', async () => {
    const w = await mountLoaded()
    await openSecurityTab(w)
    await w.find('[data-testid="security-change-password"]').trigger('click')
    await flushPromises()
    expect(bodyElExists('change-password-current')).toBe(true)
    expect(document.body.querySelectorAll('[data-testid="change-password-current"]')).toHaveLength(1)
  })

  it('submit calls requestPasswordChange with the entered current password, then shows the sent state', async () => {
    const w = await mountLoaded()
    await openSecurityTab(w)
    await w.find('[data-testid="security-change-password"]').trigger('click')
    await flushPromises()

    setNativeValue(bodyEl('change-password-current'), 'oldpass123')
    await flushPromises()

    clickEl(findBodyButtonByText('Simpan'))
    await flushPromises()

    expect(requestMock).toHaveBeenCalledWith('/auth/password/change-request', expect.objectContaining({
      method: 'POST',
      body: { current_password: 'oldpass123' }
    }))
    expect(bodyElExists('change-password-sent')).toBe(true)
    // form field is gone once we've switched to the sent state
    expect(bodyElExists('change-password-current')).toBe(false)
  })

  it('a rejected submit (400 wrong password) shows the error, keeps the modal open, and does not log out', async () => {
    pwChangeImpl = () => Promise.reject({ statusCode: 400, data: { error: 'password lama salah' } })
    const w = await mountLoaded()
    await openSecurityTab(w)
    await w.find('[data-testid="security-change-password"]').trigger('click')
    await flushPromises()

    setNativeValue(bodyEl('change-password-current'), 'wrongpass')
    await flushPromises()

    clickEl(findBodyButtonByText('Simpan'))
    await flushPromises()

    expect(bodyElExists('change-password-error')).toBe(true)
    expect(bodyEl('change-password-error').textContent).toContain('password lama salah')
    // still in the form state, not the sent state, and the modal is still open
    expect(bodyElExists('change-password-sent')).toBe(false)
    expect(bodyElExists('change-password-current')).toBe(true)
    // no forced logout on a 400 (only useApiClient's 401 path clears the session)
    expect(useAuthStore().isAuthenticated).toBe(true)
  })

  it('resend re-calls requestPasswordChange and restarts the cooldown', async () => {
    const w = await mountLoaded()
    await openSecurityTab(w)
    await w.find('[data-testid="security-change-password"]').trigger('click')
    await flushPromises()
    setNativeValue(bodyEl('change-password-current'), 'oldpass123')
    await flushPromises()
    clickEl(findBodyButtonByText('Simpan'))
    await flushPromises()

    expect(bodyElExists('change-password-sent')).toBe(true)
    // Cooldown just started — resend button is disabled with a countdown.
    const resendBtn = bodyEl('change-password-resend') as HTMLButtonElement
    expect(resendBtn.disabled).toBe(true)

    requestMock.mockClear()
    clickEl(resendBtn)
    await flushPromises()
    expect(requestMock).not.toHaveBeenCalledWith('/auth/password/change-request', expect.anything())
  })

  it('Google-linked accounts see the "manage via Google" card instead of the change-password button', async () => {
    profileResponse = { ...defaultProfileResponse, google_linked: true }
    const w = await mountLoaded()
    await openSecurityTab(w)
    expect(w.find('[data-testid="security-change-password"]').exists()).toBe(false)
    expect(w.text()).toContain('Google')
  })
})
