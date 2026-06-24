import { describe, it, expect } from 'vitest'
import { employeeSeed, employeeStore } from '~/mock/employees'
import { filterBy } from '~/mock/helpers'

describe('employees mock', () => {
  it('seeds more than one employee', () => {
    expect(employeeSeed.length).toBeGreaterThan(1)
  })

  it('every seeded employee has an active or inactive status', () => {
    expect(employeeSeed.every(e => e.status === 'active' || e.status === 'inactive')).toBe(true)
  })

  it('filterBy matches by name and nip', () => {
    const all = employeeStore.all()
    const first = all[0]
    expect(filterBy(all, { search: first.nama }, ['nama', 'nip', 'email'])).toContainEqual(first)
  })
})
