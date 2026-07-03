import type { AssetAttachment, Paginated } from '~/types'

/** Asset attachments (photos/documents), wired to /api/v1/assets/:id/attachments. */
export function useAssetAttachments() {
  const { request, requestBlob } = useApiClient()

  async function list(assetId: string): Promise<Paginated<AssetAttachment>> {
    return request<Paginated<AssetAttachment>>(`/assets/${assetId}/attachments`)
  }

  async function upload(assetId: string, file: File): Promise<AssetAttachment> {
    const formData = new FormData()
    formData.append('file', file)
    return request<AssetAttachment>(`/assets/${assetId}/attachments`, {
      method: 'POST',
      body: formData
    })
  }

  async function remove(assetId: string, attachmentId: string): Promise<void> {
    await request(`/assets/${assetId}/attachments/${attachmentId}`, { method: 'DELETE' })
  }

  async function thumbnailBlob(assetId: string, attachmentId: string): Promise<Blob> {
    return requestBlob(`/assets/${assetId}/attachments/${attachmentId}/thumbnail`)
  }

  async function contentBlob(assetId: string, attachmentId: string): Promise<Blob> {
    return requestBlob(`/assets/${assetId}/attachments/${attachmentId}/content`)
  }

  return { list, upload, remove, thumbnailBlob, contentBlob }
}
