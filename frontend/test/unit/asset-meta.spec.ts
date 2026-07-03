import { describe, it, expect } from 'vitest'
import { ASSET_STATUSES, ASSET_CLASSES, statusMeta, classMeta } from '~/constants/assetMeta'

describe('assetMeta', () => {
  it('lists all 7 asset statuses', () => {
    expect(ASSET_STATUSES).toHaveLength(7)
    expect(ASSET_STATUSES).toEqual([
      'available', 'assigned', 'under_maintenance', 'in_transfer', 'retired', 'disposed', 'lost'
    ])
  })

  it('has statusMeta entries for every status with a matching labelKey and a valid color', () => {
    const validColors = new Set(['success', 'info', 'warning', 'error', 'neutral'])
    for (const status of ASSET_STATUSES) {
      const meta = statusMeta[status]
      expect(meta).toBeDefined()
      expect(meta.labelKey).toBe(`assets.status.${status}`)
      expect(validColors.has(meta.color)).toBe(true)
    }
  })

  it('has exactly 7 keys in statusMeta, matching ASSET_STATUSES', () => {
    expect(Object.keys(statusMeta).sort()).toEqual([...ASSET_STATUSES].sort())
  })

  it('lists both asset classes', () => {
    expect(ASSET_CLASSES).toHaveLength(2)
    expect(ASSET_CLASSES).toEqual(['tangible', 'intangible'])
  })

  it('has classMeta entries for every class with a matching labelKey', () => {
    for (const cls of ASSET_CLASSES) {
      const meta = classMeta[cls]
      expect(meta).toBeDefined()
      expect(meta.labelKey).toBe(`assets.class.${cls}`)
    }
  })
})
