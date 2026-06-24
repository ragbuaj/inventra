import type { Floor, Room } from '~/types'
import { floorStore, roomStore } from '~/mock/floors'
import { generateId } from '~/mock/helpers'

export function useFloors() {
  function listByOffice(officeId: string): Floor[] {
    return floorStore.all().filter(f => f.office_id === officeId)
      .sort((a, b) => a.lantai - b.lantai)
  }

  function getFloor(floorId: string): Floor | undefined {
    return floorStore.find(floorId)
  }

  function createFloor(officeId: string, nama: string, lantai: number): Floor {
    const floor: Floor = {
      id: generateId(),
      office_id: officeId,
      nama,
      lantai,
      created_at: new Date().toISOString().slice(0, 10)
    }
    return floorStore.insert(floor)
  }

  function removeFloor(floorId: string): boolean {
    // Also remove all rooms on this floor
    const rooms = roomsByFloor(floorId)
    for (const r of rooms) {
      roomStore.remove(r.id)
    }
    return floorStore.remove(floorId)
  }

  function roomsByFloor(floorId: string): Room[] {
    return roomStore.all().filter(r => r.floor_id === floorId)
  }

  function createRoom(floorId: string, officeId: string, nama: string, kode: string): Room {
    const room: Room = {
      id: generateId(),
      floor_id: floorId,
      office_id: officeId,
      nama,
      kode,
      created_at: new Date().toISOString().slice(0, 10)
    }
    return roomStore.insert(room)
  }

  function updateFloor(floorId: string, patch: Partial<Pick<Floor, 'nama'>>): Floor | undefined {
    return floorStore.patch(floorId, patch)
  }

  function updateRoom(roomId: string, patch: Partial<Pick<Room, 'nama'>>): Room | undefined {
    return roomStore.patch(roomId, patch)
  }

  function removeRoom(roomId: string): boolean {
    return roomStore.remove(roomId)
  }

  return {
    listByOffice,
    getFloor,
    createFloor,
    updateFloor,
    removeFloor,
    roomsByFloor,
    createRoom,
    updateRoom,
    removeRoom
  }
}
