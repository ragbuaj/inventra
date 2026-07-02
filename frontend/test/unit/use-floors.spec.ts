import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useFloors } from '~/composables/api/useFloors'

beforeEach(() => request.mockReset())

describe('useFloors — floors', () => {
  it('listByOffice GETs /floors scoped by office and returns data', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'f1', office_id: 'o1' }], total: 1, limit: 100, offset: 0 })
    const rows = await useFloors().listByOffice('o1')
    expect(request.mock.calls[0][0]).toBe('/floors?office_id=o1&limit=100')
    expect(rows).toHaveLength(1)
  })

  it('createFloor POSTs office_id + name, including level when set', async () => {
    request.mockResolvedValueOnce({ id: 'f1' })
    await useFloors().createFloor({ office_id: 'o1', name: 'Lantai 1', level: 1 })
    expect(request).toHaveBeenCalledWith('/floors', { method: 'POST', body: { office_id: 'o1', name: 'Lantai 1', level: 1 } })
  })

  it('createFloor omits level when null', async () => {
    request.mockResolvedValueOnce({ id: 'f2' })
    await useFloors().createFloor({ office_id: 'o1', name: 'Lantai X', level: null })
    expect(request).toHaveBeenCalledWith('/floors', { method: 'POST', body: { office_id: 'o1', name: 'Lantai X' } })
  })

  it('updateFloor PUTs /floors/:id and resends the required office_id', async () => {
    request.mockResolvedValueOnce({ id: 'f1' })
    await useFloors().updateFloor('f1', { office_id: 'o1', name: 'Renamed' })
    expect(request).toHaveBeenCalledWith('/floors/f1', { method: 'PUT', body: { office_id: 'o1', name: 'Renamed' } })
  })

  it('removeFloor DELETEs /floors/:id', async () => {
    request.mockResolvedValueOnce(undefined)
    await useFloors().removeFloor('f1')
    expect(request).toHaveBeenCalledWith('/floors/f1', { method: 'DELETE' })
  })
})

describe('useFloors — rooms', () => {
  it('roomsByFloor GETs /rooms scoped by floor and returns data', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'r1', floor_id: 'f1' }], total: 1, limit: 100, offset: 0 })
    const rows = await useFloors().roomsByFloor('f1')
    expect(request.mock.calls[0][0]).toBe('/rooms?floor_id=f1&limit=100')
    expect(rows).toHaveLength(1)
  })

  it('createRoom POSTs floor_id + name, including code when set', async () => {
    request.mockResolvedValueOnce({ id: 'r1' })
    await useFloors().createRoom({ floor_id: 'f1', name: 'Lobi', code: 'L1-LOB' })
    expect(request).toHaveBeenCalledWith('/rooms', { method: 'POST', body: { floor_id: 'f1', name: 'Lobi', code: 'L1-LOB' } })
  })

  it('createRoom omits empty code', async () => {
    request.mockResolvedValueOnce({ id: 'r2' })
    await useFloors().createRoom({ floor_id: 'f1', name: 'Ruang Baru' })
    expect(request).toHaveBeenCalledWith('/rooms', { method: 'POST', body: { floor_id: 'f1', name: 'Ruang Baru' } })
  })

  it('updateRoom PUTs /rooms/:id and resends the required floor_id', async () => {
    request.mockResolvedValueOnce({ id: 'r1' })
    await useFloors().updateRoom('r1', { floor_id: 'f1', name: 'Renamed' })
    expect(request).toHaveBeenCalledWith('/rooms/r1', { method: 'PUT', body: { floor_id: 'f1', name: 'Renamed' } })
  })

  it('removeRoom DELETEs /rooms/:id', async () => {
    request.mockResolvedValueOnce(undefined)
    await useFloors().removeRoom('r1')
    expect(request).toHaveBeenCalledWith('/rooms/r1', { method: 'DELETE' })
  })
})
