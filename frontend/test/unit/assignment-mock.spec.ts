import { describe, it, expect, beforeEach } from 'vitest'
import { useAssignment } from '~/composables/api/useAssignment'
import { assignmentStore, assignmentSeed, availableSeed, CONDITION_KEYS } from '~/mock/assignment'

const { list, available, checkout, checkin } = useAssignment()

beforeEach(() => assignmentStore.reset())

describe('mock/assignment', () => {
  it('seeds 6 assignments (3 active, 3 returned) and 6 available assets', () => {
    expect(assignmentSeed).toHaveLength(6)
    expect(assignmentSeed.filter(a => a.status === 'active')).toHaveLength(3)
    expect(assignmentSeed.filter(a => a.status === 'returned')).toHaveLength(3)
    expect(availableSeed).toHaveLength(6)
    expect(CONDITION_KEYS).toEqual(['baik', 'ringan', 'berat'])
  })
})

describe('useAssignment', () => {
  it('lists all assignments and the full available pool initially', async () => {
    expect(await list()).toHaveLength(6)
    expect(await available()).toHaveLength(6)
  })

  it('check-out creates an active assignment and removes the asset from the available pool', async () => {
    const target = availableSeed[0]!
    const created = await checkout({ tag: target.tag, nama: target.nama, pemegang: 'Andi Saputra', ini: 'AS', pinjam: '2026-06-24', kondisi: 'baik' })
    expect(created.status).toBe('active')
    expect(created.tag).toBe(target.tag)
    expect(await list()).toHaveLength(7)
    const pool = await available()
    expect(pool).toHaveLength(5)
    expect(pool.some(a => a.tag === target.tag)).toBe(false)
  })

  it('check-in flips status to returned, records the return + condition, and frees the asset', async () => {
    const target = availableSeed[1]!
    const created = await checkout({ tag: target.tag, nama: target.nama, pemegang: 'Rina Putri', ini: 'RP', pinjam: '2026-06-20', kondisi: 'baik' })
    const returned = await checkin(created.id, { kembali: '2026-06-24', kondisi: 'ringan' })
    expect(returned.status).toBe('returned')
    expect(returned.kembali).toBe('2026-06-24')
    expect(returned.kondisi).toBe('ringan')
    // freed back into the available pool
    expect((await available()).some(a => a.tag === target.tag)).toBe(true)
  })

  it('throws the sentinel error checking in a missing assignment', async () => {
    await expect(checkin('nope', { kembali: '2026-06-24', kondisi: 'baik' })).rejects.toThrow('assignment.errNotFound')
  })

  it('reset restores the seed and clears lent assets', async () => {
    await checkout({ tag: availableSeed[0]!.tag, nama: 'x', pemegang: 'p', ini: 'PP', pinjam: '2026-06-24', kondisi: 'baik' })
    assignmentStore.reset()
    expect(await list()).toHaveLength(6)
    expect(await available()).toHaveLength(6)
  })
})
