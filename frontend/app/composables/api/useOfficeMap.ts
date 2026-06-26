import type { MapOffice } from '~/types'
import { fakeLatency } from '~/mock/helpers'
import { mapOffices } from '~/mock/officeMap'

export function useOfficeMap() {
  async function list(): Promise<MapOffice[]> {
    await fakeLatency(500)
    return mapOffices
  }
  return { list }
}
