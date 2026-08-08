/**
 * Locale resolution for guide content.
 *
 * Guide text lives in the database, not in the i18n catalogue, because editors
 * author it without a release. The API therefore sends BOTH languages on every
 * row and the client picks — which is what lets switching locale re-render
 * instantly instead of refetching.
 *
 * Indonesian is mandatory; English is optional and falls back to Indonesian.
 * A blank or whitespace-only English value counts as absent: an editor who
 * cleared the field meant "not translated", not "show nothing".
 */
export function guideText(
  id: string | null | undefined,
  en: string | null | undefined,
  locale: string
): string {
  if (locale.startsWith('en')) {
    const trimmed = (en ?? '').trim()
    if (trimmed !== '') return trimmed
  }
  return (id ?? '').trim()
}

/** True when a module still lacks its English version — surfaced as a badge in the admin list. */
export function guideNeedsTranslation(fields: Array<string | null | undefined>): boolean {
  return fields.some(f => (f ?? '').trim() === '')
}

/**
 * The privacy-preserving embed URL for a YouTube video.
 *
 * Built here from the validated 11-character id the API returns — never from a
 * URL supplied by an editor. `youtube-nocookie.com` keeps YouTube from setting
 * tracking cookies until the reader actually plays the video.
 */
export function youtubeEmbedUrl(videoId: string): string {
  return `https://www.youtube-nocookie.com/embed/${encodeURIComponent(videoId)}?rel=0`
}

/** Thumbnail for the lazy-loaded player facade; same origin rules as the embed. */
export function youtubeThumbnailUrl(videoId: string): string {
  return `https://i.ytimg.com/vi/${encodeURIComponent(videoId)}/hqdefault.jpg`
}

/** Human-readable file size for the document card ("1,4 MB"). */
export function formatFileSize(bytes: number | undefined, locale: string): string {
  if (!bytes || bytes <= 0) return ''
  const mb = bytes / (1024 * 1024)
  if (mb >= 1) return `${mb.toLocaleString(locale, { maximumFractionDigits: 1 })} MB`
  const kb = bytes / 1024
  return `${kb.toLocaleString(locale, { maximumFractionDigits: 0 })} KB`
}
