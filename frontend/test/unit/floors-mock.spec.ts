import { describe, it, expect } from 'vitest'
import { floorStore, roomStore } from '~/mock/floors'
import { useFloors } from '~/composables/api/useFloors'

// Reset stores before each test by re-importing fresh module is not straightforward with
// module-level state, so we test the composable against the shared in-memory store.
// Tests are ordered so they don't depend on prior mutation for correctness of counts.

describe('useFloors — listByOffice', () => {
  const floors = useFloors()

  it('returns only floors belonging to the given office, sorted by lantai', () => {
    const result = floors.listByOffice('o-pusat')
    expect(result.length).toBeGreaterThan(0)
    expect(result.every(f => f.office_id === 'o-pusat')).toBe(true)
    for (let i = 1; i < result.length; i++) {
      expect(result[i].lantai).toBeGreaterThanOrEqual(result[i - 1].lantai)
    }
  })

  it('returns empty array for unknown office', () => {
    expect(floors.listByOffice('no-such-office')).toEqual([])
  })
})

describe('useFloors — createFloor / removeFloor', () => {
  const floors = useFloors()

  it('createFloor inserts a floor with the given fields', () => {
    const before = floors.listByOffice('o-bdg').length
    const f = floors.createFloor('o-bdg', 'Lantai Baru', 99)
    expect(f.office_id).toBe('o-bdg')
    expect(f.nama).toBe('Lantai Baru')
    expect(f.lantai).toBe(99)
    expect(f.id).not.toBe('')
    expect(floors.listByOffice('o-bdg').length).toBe(before + 1)
    // cleanup
    floors.removeFloor(f.id)
  })

  it('removeFloor removes the floor and returns true', () => {
    const f = floors.createFloor('o-jkt', 'Tmp Floor', 88)
    expect(floors.removeFloor(f.id)).toBe(true)
    expect(floorStore.find(f.id)).toBeUndefined()
  })

  it('removeFloor also removes rooms on the floor', () => {
    const f = floors.createFloor('o-jkt', 'Floor With Rooms', 77)
    const r = floors.createRoom(f.id, 'o-jkt', 'Temp Room', 'TMP-R1')
    expect(floors.roomsByFloor(f.id).length).toBe(1)
    floors.removeFloor(f.id)
    expect(floors.roomsByFloor(f.id).length).toBe(0)
    expect(roomStore.find(r.id)).toBeUndefined()
  })
})

describe('useFloors — createRoom / removeRoom', () => {
  const floors = useFloors()

  it('createRoom inserts a room linked to floor and office', () => {
    const r = floors.createRoom('fl-pusat-1', 'o-pusat', 'New Room', 'PST-NEW')
    expect(r.floor_id).toBe('fl-pusat-1')
    expect(r.office_id).toBe('o-pusat')
    expect(r.nama).toBe('New Room')
    expect(r.kode).toBe('PST-NEW')
    // cleanup
    floors.removeRoom(r.id)
  })

  it('removeRoom removes the room and returns true; returns false for missing', () => {
    const r = floors.createRoom('fl-jkt-1', 'o-jkt', 'Tmp', 'TMP')
    expect(floors.removeRoom(r.id)).toBe(true)
    expect(floors.removeRoom(r.id)).toBe(false)
  })

  it('roomsByFloor returns only rooms for the given floor', () => {
    const rooms = floors.roomsByFloor('fl-pusat-1')
    expect(rooms.length).toBeGreaterThan(0)
    expect(rooms.every(r => r.floor_id === 'fl-pusat-1')).toBe(true)
  })
})

describe('useFloors — updateFloor (Bug 4)', () => {
  const floors = useFloors()

  it('updateFloor renames the floor and the new name is returned by getFloor', () => {
    const f = floors.createFloor('o-pusat', 'Lantai Temp', 50)
    const updated = floors.updateFloor(f.id, { nama: 'Lantai Diganti' })
    expect(updated).toBeDefined()
    expect(updated!.nama).toBe('Lantai Diganti')
    const fetched = floors.getFloor(f.id)
    expect(fetched?.nama).toBe('Lantai Diganti')
    // cleanup
    floors.removeFloor(f.id)
  })

  it('updateFloor returns undefined for unknown floor', () => {
    const result = floors.updateFloor('no-such-floor', { nama: 'X' })
    expect(result).toBeUndefined()
  })

  it('updateFloor rename is reflected in listByOffice', () => {
    const f = floors.createFloor('o-jkt', 'Lantai Lama', 60)
    floors.updateFloor(f.id, { nama: 'Lantai Baru Nama' })
    const list = floors.listByOffice('o-jkt')
    const found = list.find(x => x.id === f.id)
    expect(found?.nama).toBe('Lantai Baru Nama')
    // cleanup
    floors.removeFloor(f.id)
  })
})

describe('useFloors — updateRoom (Bug 4)', () => {
  const floors = useFloors()

  it('updateRoom renames the room and is reflected by roomsByFloor', () => {
    const f = floors.createFloor('o-pusat', 'Temp Floor For Room', 55)
    const r = floors.createRoom(f.id, 'o-pusat', 'Ruang Asli', 'TMP-ROOM')
    const updated = floors.updateRoom(r.id, { nama: 'Ruang Direname' })
    expect(updated).toBeDefined()
    expect(updated!.nama).toBe('Ruang Direname')
    const list = floors.roomsByFloor(f.id)
    const found = list.find(x => x.id === r.id)
    expect(found?.nama).toBe('Ruang Direname')
    // cleanup
    floors.removeFloor(f.id)
  })

  it('updateRoom returns undefined for unknown room', () => {
    const result = floors.updateRoom('no-such-room', { nama: 'X' })
    expect(result).toBeUndefined()
  })
})
