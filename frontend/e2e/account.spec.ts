import { test, expect } from '@playwright/test'
import { login } from './helpers'

test('account: open from user menu and change password validation', async ({ page }) => {
  await login(page)
  await page.goto('/akun')
  await expect(page.getByRole('button', { name: 'Keamanan' })).toBeVisible()
  await page.getByRole('button', { name: 'Keamanan' }).click()
  const pw = page.locator('input[type="password"]')
  await pw.nth(0).fill('oldpass')
  await pw.nth(1).fill('Abcdefg1!')
  await pw.nth(2).fill('different')
  await page.getByRole('button', { name: 'Ganti Password' }).last().click()
  await expect(page.getByText('tidak cocok')).toBeVisible()
})

test('account: switch language preference', async ({ page }) => {
  await login(page)
  await page.goto('/akun?tab=pref')
  await page.getByRole('button', { name: 'English' }).click()
  await expect(page.getByText('Appearance')).toBeVisible()
})
