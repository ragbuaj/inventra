import { describe, it, expect } from 'vitest'
import { googleMapsUrl } from '~/utils/googleMapsUrl'

describe('googleMapsUrl', () => {
  it('builds a maps search URL from lat/lng', () => {
    expect(googleMapsUrl(-6.1754, 106.8272)).toBe('https://www.google.com/maps/search/?api=1&query=-6.1754%2C106.8272')
  })
})
