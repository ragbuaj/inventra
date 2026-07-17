import { test, expect, request } from '@playwright/test'
import type { APIRequestContext } from '@playwright/test'
import { login, EMAIL, PASSWORD, clickRowAction } from './helpers'

// Admin-initiated password reset from User Management: an admin clicks "Reset
// Password" on a user row, confirms, and the backend emails that user a reset
// link (POST /users/:id/reset-password). This drives the real UI + backend and
// asserts the email actually lands in Mailpit addressed to the target user.

const API_BASE = `${(process.env.NUXT_PUBLIC_API_BASE || 'http://localhost:8080/api/v1').replace(/\/+$/, '')}/`
const MAILPIT = 'http://localhost:8025'
const RUN = `${Date.now()}`

function authHeader(token: string) {
  return { Authorization: `Bearer ${token}` }
}

async function apiJson<T>(res: Awaited<ReturnType<APIRequestContext['post']>>): Promise<T> {
  if (!res.ok()) throw new Error(`API ${res.url()} failed: ${res.status()} ${await res.text()}`)
  return await res.json() as T
}

test.describe('Admin-initiated password reset', () => {
  let api: APIRequestContext
  let adminToken: string
  let targetId: string
  const targetName = `E2E Reset Target ${RUN}`
  const targetEmail = `e2e.reset.target.${RUN}@inventra.local`

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    const loginRes = await apiJson<{ access_token: string }>(
      await api.post('auth/login', { data: { email: EMAIL, password: PASSWORD } }))
    adminToken = loginRes.access_token

    // A throwaway user WITH a password (email login) so it is eligible for a reset.
    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')
    if (!superadmin) throw new Error('Superadmin role not found in GET /authz/roles')

    targetId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: targetName, email: targetEmail, password: `Target${RUN}!`, role_id: superadmin.id }
    }))).id
  })

  test.afterAll(async () => {
    // Failure-safe cleanup: soft-delete the throwaway user regardless of outcome.
    try {
      if (targetId) await api.delete(`users/${targetId}`, { headers: authHeader(adminToken) })
    } catch { /* teardown must never throw */ }
    await api.dispose()
  })

  test('reset password row action emails a reset link to the target user', async ({ page, request: req }) => {
    // Purge the mailbox so the search below only sees this test's email.
    await req.delete(`${MAILPIT}/api/v1/messages`)

    await login(page)
    await page.goto('/settings/users')

    // Filter to the throwaway user, then act on its row.
    await page.getByPlaceholder('Cari nama atau email…').fill(targetName)
    const row = page.locator('tr').filter({ hasText: targetName })
    await expect(row).toBeVisible()

    await clickRowAction(page, row, 'Reset Password')

    // Confirm dialog — confirmLabel is "Kirim tautan reset".
    await page.getByRole('button', { name: 'Kirim tautan reset', exact: true }).click()

    // Success toast names the target address. Exact match targets the toast
    // title only — the email also appears in the row cell and the (closing)
    // confirm-dialog description, so a substring match is ambiguous.
    await expect(page.getByText(`Tautan reset password dikirim ke ${targetEmail}`, { exact: true }))
      .toBeVisible({ timeout: 10_000 })

    // The reset email must actually reach Mailpit, addressed to the target user,
    // carrying a /reset-password?token=... link.
    await expect.poll(async () => {
      const res = await req.get(`${MAILPIT}/api/v1/search?query=${encodeURIComponent('to:' + targetEmail)}`)
      return (await res.json() as { messages_count: number }).messages_count
    }, { timeout: 10_000 }).toBeGreaterThan(0)

    const searchRes = await req.get(`${MAILPIT}/api/v1/search?query=${encodeURIComponent('to:' + targetEmail)}`)
    const { messages } = await searchRes.json() as { messages: Array<{ ID: string }> }
    const msgRes = await req.get(`${MAILPIT}/api/v1/message/${messages[0]!.ID}`)
    const msg = await msgRes.json() as { Text?: string, HTML?: string }
    const body = msg.Text || msg.HTML || ''
    expect(body).toMatch(/\/reset-password\?token=[A-Za-z0-9_-]+/)
  })

  test('a Google-only account cannot be reset (422 -> warning toast, no email)', async ({ page, request: req }) => {
    // Provision a Google-only user (no password) directly, so the reset is rejected.
    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')!
    const googleName = `E2E Reset Google ${RUN}`
    const googleEmail = `e2e.reset.google.${RUN}@inventra.local`
    const googleId = (await apiJson<{ id: string }>(await api.post('users', {
      headers: authHeader(adminToken),
      // No password -> Google-only account (no password login to reset).
      data: { name: googleName, email: googleEmail, role_id: superadmin.id }
    }))).id

    try {
      await req.delete(`${MAILPIT}/api/v1/messages`)
      await login(page)
      await page.goto('/settings/users')
      await page.getByPlaceholder('Cari nama atau email…').fill(googleName)
      const row = page.locator('tr').filter({ hasText: googleName })
      await expect(row).toBeVisible()

      await clickRowAction(page, row, 'Reset Password')
      await page.getByRole('button', { name: 'Kirim tautan reset', exact: true }).click()

      // Warning toast (Google-only), and no email sent. Exact match targets the
      // toast title (avoids the hidden aria-live announcer's prefixed copy).
      await expect(page.getByText('Akun ini login lewat Google — tidak ada password untuk direset.', { exact: true }))
        .toBeVisible({ timeout: 10_000 })
      const res = await req.get(`${MAILPIT}/api/v1/search?query=${encodeURIComponent('to:' + googleEmail)}`)
      expect((await res.json() as { messages_count: number }).messages_count).toBe(0)
    } finally {
      await api.delete(`users/${googleId}`, { headers: authHeader(adminToken) }).catch(() => {})
    }
  })
})
