import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.describe('Operasional cluster (mock-backed)', () => {
  test('Penugasan — tabs switch to history', async ({ page }) => {
    await login(page)
    await page.goto('/assignment')
    await expect(page.getByRole('heading', { name: 'Penugasan Aset' })).toBeVisible()

    await page.getByRole('button', { name: 'Riwayat' }).click()
    await expect(page.getByText('Televisi Samsung 55" Crystal')).toBeVisible()
  })

  test('Maintenance — due banner and schedule render', async ({ page }) => {
    await login(page)
    await page.goto('/maintenance')
    await expect(page.getByRole('heading', { name: 'Maintenance' })).toBeVisible()
    await expect(page.getByText('Maintenance jatuh tempo')).toBeVisible()
    await expect(page.getByText('Switch Cisco Catalyst 1000')).toBeVisible()
  })

  test('Approval — inbox lists requests and shows detail', async ({ page }) => {
    await login(page)
    await page.goto('/approval')
    await expect(page.getByRole('heading', { name: 'Pengajuan & Approval' })).toBeVisible()
    await expect(page.getByText('Registrasi 12 Laptop Asus ExpertBook B1').first()).toBeVisible()
    // The first request is selected by default → detail sections visible.
    await expect(page.getByText('Data Diajukan')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Setujui', exact: true })).toBeVisible()
  })

  test('Laporan — applying the asset report shows KPIs and totals', async ({ page }) => {
    await login(page)
    await page.goto('/reports')
    await expect(page.getByRole('heading', { name: 'Laporan' })).toBeVisible()
    await page.getByRole('button', { name: 'Terapkan' }).click()
    await expect(page.getByText('Total Aset')).toBeVisible()
    await expect(page.getByText('Nilai Buku per Kategori')).toBeVisible()
  })
})
