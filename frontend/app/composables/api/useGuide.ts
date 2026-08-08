import type {
  GuideAttachment,
  GuideAttachmentPatch,
  GuideModule,
  GuideModuleInput,
  GuideVideoInput
} from '~/types'

/**
 * Panduan Penggunaan content, wired to /api/v1/guide.
 *
 * `list()` is the one call in the app that works with or without a session:
 * the guide page is public, and useApiClient only attaches an Authorization
 * header when a token exists, so the same call returns the guest projection for
 * a visitor and the full one for a signed-in reader. Nothing here should ever be
 * called with a session-only expectation from the public page — a 401 makes the
 * client clear the session and redirect to /login, which would eject a reader
 * from a page they are entitled to see.
 */
export function useGuide() {
  const client = useApiClient()

  /** Published modules. `includeDrafts` is honoured only for guide.manage holders. */
  async function list(includeDrafts = false): Promise<GuideModule[]> {
    const res = await client.request<{ data: GuideModule[] }>(
      `/guide/modules${includeDrafts ? '?status=all' : ''}`
    )
    return res.data ?? []
  }

  async function get(id: string): Promise<GuideModule> {
    return client.request<GuideModule>(`/guide/modules/${id}`)
  }

  async function create(input: GuideModuleInput): Promise<GuideModule> {
    return client.request<GuideModule>('/guide/modules', { method: 'POST', body: input })
  }

  async function update(id: string, input: GuideModuleInput): Promise<GuideModule> {
    return client.request<GuideModule>(`/guide/modules/${id}`, { method: 'PATCH', body: input })
  }

  async function remove(id: string): Promise<void> {
    await client.request(`/guide/modules/${id}`, { method: 'DELETE' })
  }

  async function addVideo(moduleId: string, input: GuideVideoInput): Promise<GuideAttachment> {
    return client.request<GuideAttachment>(`/guide/modules/${moduleId}/attachments/video`, {
      method: 'POST',
      body: input
    })
  }

  async function addDocument(
    moduleId: string,
    file: File,
    meta: Omit<GuideVideoInput, 'url'>
  ): Promise<GuideAttachment> {
    const form = new FormData()
    form.append('file', file)
    form.append('title_id', meta.title_id)
    if (meta.title_en) form.append('title_en', meta.title_en)
    form.append('sort_order', String(meta.sort_order))
    return client.request<GuideAttachment>(`/guide/modules/${moduleId}/attachments/document`, {
      method: 'POST',
      body: form
    })
  }

  async function updateAttachment(id: string, input: GuideAttachmentPatch): Promise<GuideAttachment> {
    return client.request<GuideAttachment>(`/guide/attachments/${id}`, { method: 'PATCH', body: input })
  }

  async function removeAttachment(id: string): Promise<void> {
    await client.request(`/guide/attachments/${id}`, { method: 'DELETE' })
  }

  /**
   * The PDF endpoint is authenticated, so its URL cannot be used as a bare
   * <iframe src>. Fetch the bytes and hand back an object URL; the caller owns
   * it and must revokeObjectURL when replacing or unmounting.
   */
  async function attachmentObjectURL(id: string): Promise<string | null> {
    try {
      const blob = await client.requestBlob(`/guide/attachments/${id}/content`, {
        suppressErrorToast: true
      })
      return URL.createObjectURL(blob)
    } catch {
      // A deleted attachment or a transient failure degrades to "no preview"
      // rather than taking the whole page down.
      return null
    }
  }

  return {
    list,
    get,
    create,
    update,
    remove,
    addVideo,
    addDocument,
    updateAttachment,
    removeAttachment,
    attachmentObjectURL
  }
}
