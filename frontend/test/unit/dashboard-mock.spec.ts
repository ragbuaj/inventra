import { describe, it, expect } from 'vitest'
import { dashboardData, scopeOrder } from '~/mock/dashboard'

// Task 11 fully rewrote useDashboard onto the real /dashboard/summary API —
// `summary(scope, period, locale)` no longer exists (now `summary(query)`,
// backed by useApiClient, which also requires a Nuxt instance this `node`-env
// file doesn't provide). This describe block tested the OLD mock-backed
// behavior end to end; its replacement coverage is
// test/nuxt/use-dashboard.spec.ts. Re-enabled/rewritten in a later task if the
// mock-driven view model in pages/index.vue is retired outright, or removed
// once Task 12 finishes rewiring the dashboard page.
describe.skip('useDashboard.summary', () => {
  it('returns the dataset matching the requested scope', async () => {
    const data = await summary('kanwil', '0', 'id')
    expect(data.scope).toBe('kanwil')
    expect(data.total).toBe(dashboardData.kanwil.total)
  })

  it('falls back to jaksel for an unknown scope', async () => {
    // @ts-expect-error — exercising the runtime fallback for a bad scope value
    const data = await summary('does-not-exist', '0', 'id')
    expect(data.scope).toBe('jaksel')
  })

  it('resolves localized record text for the id locale', async () => {
    const data = await summary('jaksel', '0', 'id')
    expect(data.name).toBe('Kantor Cabang Jakarta Selatan')
    expect(data.maint[0].due).toBe('Besok')
    expect(data.appr[0].title).toBe('Peminjaman Proyektor Epson EB-X51')
  })

  it('resolves localized record text for the en locale', async () => {
    const data = await summary('jaksel', '0', 'en')
    expect(data.name).toBe('Jakarta Selatan Branch')
    expect(data.maint[0].due).toBe('Tomorrow')
    expect(data.appr[0].title).toBe('Loan: Epson EB-X51 Projector')
  })

  it('keeps non-localized data (counts, money, category labels) intact', async () => {
    const data = await summary('jaksel', '0', 'en')
    expect(data.status).toEqual([58, 22, 9, 4, 3])
    expect(data.perolehan).toBe('Rp 3,82 M')
    expect(data.kategori[0]).toEqual(['Elektronik', 41])
  })

  it('period argument does not change the returned figures', async () => {
    const a = await summary('pusat', '0', 'id')
    const b = await summary('pusat', '3', 'id')
    expect(b.total).toBe(a.total)
  })
})

describe('dashboard fixtures', () => {
  it('every scope has 5 status counts and 3 maintenance + 3 approval rows', () => {
    for (const scope of scopeOrder) {
      const d = dashboardData[scope]
      expect(d.status).toHaveLength(5)
      expect(d.maint).toHaveLength(3)
      expect(d.appr).toHaveLength(3)
    }
  })

  it('status counts sum to the headline total for every scope', () => {
    for (const scope of scopeOrder) {
      const d = dashboardData[scope]
      const sum = d.status.reduce((a, b) => a + b, 0)
      expect(sum).toBe(d.total)
    }
  })
})
