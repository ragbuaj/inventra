import { test, expect } from '@playwright/test'
import { login } from './helpers'

test('opens the command palette and searches', async ({ page }) => {
  await login(page)
  await page.getByRole('button', { name: /Cari aset, pegawai/ }).click()
  const input = page.getByPlaceholder(/Cari aset, pegawai/)
  await expect(input).toBeVisible()
  await input.fill('latitude')
  await expect(page.getByText('Aset', { exact: true })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(input).toBeHidden()
})

test('toggles the palette with the keyboard shortcut', async ({ page }) => {
  await login(page)
  // The Ctrl+K listener is attached in CommandPalette's onMounted; on slow CI a
  // keypress fired right after login can land before the listener exists and is
  // silently lost. Wait for the app shell, then press-and-check until it opens.
  await expect(page.getByRole('button', { name: /Cari aset, pegawai/ })).toBeVisible()
  await expect(async () => {
    await page.keyboard.press('ControlOrMeta+k')
    await expect(page.getByPlaceholder(/Cari aset, pegawai/)).toBeVisible({ timeout: 2000 })
  }).toPass({ timeout: 15000 })
})
