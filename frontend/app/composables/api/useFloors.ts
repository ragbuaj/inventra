import type { Floor, Paginated, Room } from '~/types'

export interface FloorInput {
  office_id: string
  name: string
  level?: number | null
}

export interface RoomInput {
  floor_id: string
  name: string
  code?: string | null
}

/**
 * Floors + rooms, wired to /api/v1/floors and /api/v1/rooms (server-enforced
 * `offices` data-scope; rooms scope transitively through their floor's office).
 * Floors are listed per office (`?office_id=`), rooms per floor (`?floor_id=`).
 * Note: floor/room updates must resend office_id/floor_id (required by the API).
 */
export function useFloors() {
  const { request } = useApiClient()

  async function listByOffice(officeId: string): Promise<Floor[]> {
    const res = await request<Paginated<Floor>>(`/floors?office_id=${officeId}&limit=100`)
    return res.data
  }

  async function roomsByFloor(floorId: string): Promise<Room[]> {
    const res = await request<Paginated<Room>>(`/rooms?floor_id=${floorId}&limit=100`)
    return res.data
  }

  async function createFloor(input: FloorInput): Promise<Floor> {
    const body: Record<string, unknown> = { office_id: input.office_id, name: input.name }
    if (input.level != null) body.level = input.level
    return request<Floor>('/floors', { method: 'POST', body })
  }

  async function updateFloor(id: string, input: FloorInput): Promise<Floor> {
    const body: Record<string, unknown> = { office_id: input.office_id, name: input.name }
    if (input.level != null) body.level = input.level
    return request<Floor>(`/floors/${id}`, { method: 'PUT', body })
  }

  async function removeFloor(id: string): Promise<void> {
    await request(`/floors/${id}`, { method: 'DELETE' })
  }

  async function createRoom(input: RoomInput): Promise<Room> {
    const body: Record<string, unknown> = { floor_id: input.floor_id, name: input.name }
    if (input.code) body.code = input.code
    return request<Room>('/rooms', { method: 'POST', body })
  }

  async function updateRoom(id: string, input: RoomInput): Promise<Room> {
    const body: Record<string, unknown> = { floor_id: input.floor_id, name: input.name }
    if (input.code) body.code = input.code
    return request<Room>(`/rooms/${id}`, { method: 'PUT', body })
  }

  async function removeRoom(id: string): Promise<void> {
    await request(`/rooms/${id}`, { method: 'DELETE' })
  }

  return { listByOffice, roomsByFloor, createFloor, updateFloor, removeFloor, createRoom, updateRoom, removeRoom }
}
