import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
const requestBlob = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request, requestBlob }) }))

// eslint-disable-next-line import/first
import { useAssetAttachments } from '~/composables/api/useAssetAttachments'

const sampleAttachment = {
  id: 'att1',
  asset_id: 'a1',
  kind: 'photo',
  original_filename: 'foto.jpg',
  size_bytes: 1024,
  mime_type: 'image/jpeg',
  has_thumbnail: true,
  created_at: '2026-07-03T00:00:00Z'
}

beforeEach(() => {
  request.mockReset()
  requestBlob.mockReset()
})

describe('useAssetAttachments.list', () => {
  it('GETs /assets/:id/attachments', async () => {
    request.mockResolvedValueOnce({ data: [sampleAttachment], total: 1 })
    const res = await useAssetAttachments().list('a1')
    expect(request).toHaveBeenCalledWith('/assets/a1/attachments')
    expect(res.total).toBe(1)
    expect(res.data).toEqual([sampleAttachment])
  })

  it('propagates errors from request', async () => {
    request.mockRejectedValueOnce(new Error('not found'))
    await expect(useAssetAttachments().list('nope')).rejects.toThrow('not found')
  })
})

describe('useAssetAttachments.upload', () => {
  it('POSTs multipart FormData with field "file" to /assets/:id/attachments', async () => {
    request.mockResolvedValueOnce(sampleAttachment)
    const file = new File(['data'], 'foto.jpg', { type: 'image/jpeg' })
    const res = await useAssetAttachments().upload('a1', file)
    expect(request).toHaveBeenCalledTimes(1)
    const [path, opts] = request.mock.calls[0] as [string, { method: string, body: FormData }]
    expect(path).toBe('/assets/a1/attachments')
    expect(opts.method).toBe('POST')
    expect(opts.body).toBeInstanceOf(FormData)
    expect(opts.body.get('file')).toBe(file)
    expect(opts).not.toHaveProperty('headers')
    expect(res.id).toBe('att1')
  })

  it('propagates errors from request', async () => {
    request.mockRejectedValueOnce(new Error('too large'))
    const file = new File(['data'], 'foto.jpg', { type: 'image/jpeg' })
    await expect(useAssetAttachments().upload('a1', file)).rejects.toThrow('too large')
  })
})

describe('useAssetAttachments.remove', () => {
  it('DELETEs /assets/:id/attachments/:aid', async () => {
    request.mockResolvedValueOnce(undefined)
    await useAssetAttachments().remove('a1', 'att1')
    expect(request).toHaveBeenCalledWith('/assets/a1/attachments/att1', { method: 'DELETE' })
  })

  it('propagates errors from request', async () => {
    request.mockRejectedValueOnce(new Error('forbidden'))
    await expect(useAssetAttachments().remove('a1', 'att1')).rejects.toThrow('forbidden')
  })
})

describe('useAssetAttachments.thumbnailBlob', () => {
  it('calls requestBlob on the thumbnail path and returns the Blob', async () => {
    const blob = new Blob(['thumb'], { type: 'image/jpeg' })
    requestBlob.mockResolvedValueOnce(blob)
    const res = await useAssetAttachments().thumbnailBlob('a1', 'att1')
    expect(requestBlob).toHaveBeenCalledWith('/assets/a1/attachments/att1/thumbnail')
    expect(res).toBe(blob)
  })

  it('propagates errors from requestBlob', async () => {
    requestBlob.mockRejectedValueOnce(new Error('no thumbnail'))
    await expect(useAssetAttachments().thumbnailBlob('a1', 'att1')).rejects.toThrow('no thumbnail')
  })
})

describe('useAssetAttachments.contentBlob', () => {
  it('calls requestBlob on the content path and returns the Blob', async () => {
    const blob = new Blob(['content'], { type: 'application/pdf' })
    requestBlob.mockResolvedValueOnce(blob)
    const res = await useAssetAttachments().contentBlob('a1', 'att1')
    expect(requestBlob).toHaveBeenCalledWith('/assets/a1/attachments/att1/content')
    expect(res).toBe(blob)
  })

  it('propagates errors from requestBlob', async () => {
    requestBlob.mockRejectedValueOnce(new Error('gone'))
    await expect(useAssetAttachments().contentBlob('a1', 'att1')).rejects.toThrow('gone')
  })
})
