import { describe, it, expect, beforeEach } from 'vitest'
import { useMaintenance } from '~/composables/api/useMaintenance'
import {
  maintenanceStore, scheduleSeed, recordSeed, loc, dayDiff, dueLevel, MAINT_TODAY
} from '~/mock/maintenance'

const { schedule, records, reports, addRecord, addReport } = useMaintenance()

beforeEach(() => maintenanceStore.reset())

describe('mock/maintenance — seeds', () => {
  it('seeds 5 schedule items and 6 records', () => {
    expect(scheduleSeed).toHaveLength(5)
    expect(recordSeed).toHaveLength(6)
  })
})

describe('loc()', () => {
  it('resolves a localized value by language and passes plain strings through', () => {
    expect(loc({ id: 'Inspeksi', en: 'Inspection' }, 'en')).toBe('Inspection')
    expect(loc({ id: 'Inspeksi', en: 'Inspection' }, 'id')).toBe('Inspeksi')
    expect(loc('Auto2000', 'en')).toBe('Auto2000')
  })
})

describe('dayDiff() / dueLevel() — relative to fixed today 2026-06-24', () => {
  it('classifies overdue, today, soon and later correctly', () => {
    expect(MAINT_TODAY).toBe('2026-06-24')
    expect(dayDiff('2026-06-20')).toBe(-4)
    expect(dueLevel(dayDiff('2026-06-20'))).toBe('overdue')
    expect(dayDiff('2026-06-24')).toBe(0)
    expect(dueLevel(0)).toBe('today')
    expect(dayDiff('2026-06-27')).toBe(3)
    expect(dueLevel(3)).toBe('soon')
    expect(dayDiff('2026-07-18')).toBe(24)
    expect(dueLevel(24)).toBe('later')
  })

  it('only two scheduled items fall within the 3-day due window', () => {
    const within = scheduleSeed.filter(s => dayDiff(s.due) <= 3)
    expect(within).toHaveLength(2) // Avanza (overdue) + nothing else ≤3 besides... only Avanza & the 27th is +3
    expect(within.map(s => s.due).sort()).toEqual(['2026-06-20', '2026-06-27'])
  })
})

describe('useMaintenance', () => {
  it('lists schedule, records and (empty) reports', async () => {
    expect(await schedule()).toHaveLength(5)
    expect(await records()).toHaveLength(6)
    expect(await reports()).toHaveLength(0)
  })

  it('addRecord prepends a maintenance note', async () => {
    await addRecord({ tag: 'NEW-1', nama: 'Test Asset', tipe: 'preventive', kategori: 'Inspeksi', tanggal: '2026-06-24', status: 'scheduled', biaya: 500000, vendor: 'Auto2000' })
    const all = await records()
    expect(all).toHaveLength(7)
    expect(all[0]!.tag).toBe('NEW-1')
  })

  it('addReport queues a damage report, and reset clears reports but keeps seed records', async () => {
    await addReport({ tag: 'JKT01-ELK-2026-00005', nama: 'Monitor LG 27UL550', problemKey: 'display', desc: 'flicker', date: MAINT_TODAY })
    expect(await reports()).toHaveLength(1)
    maintenanceStore.reset()
    expect(await reports()).toHaveLength(0)
    expect(await records()).toHaveLength(6)
  })
})
