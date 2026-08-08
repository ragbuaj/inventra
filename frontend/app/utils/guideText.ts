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

/**
 * Accepted YouTube hosts, matched EXACTLY — the same list the backend enforces
 * (internal/guide/youtube.go). Suffix matching would accept
 * "youtube.com.evil.example.com"; substring matching would accept
 * "https://evil.example.com/youtube.com/watch?v=x".
 */
const YOUTUBE_HOSTS = new Set(['youtube.com', 'www.youtube.com', 'm.youtube.com', 'youtu.be'])
const YOUTUBE_ID_RE = /^[A-Za-z0-9_-]{11}$/

/**
 * Extract a video id from a pasted link, or null when the link is not YouTube.
 *
 * This mirrors the server-side parser so the editor can say "link accepted" (and
 * show which id will be stored) before submitting. It is NOT the security
 * boundary: the server parses the URL again and stores only what it extracts
 * itself. Keep the two in lockstep — a client that accepts more than the server
 * produces a confusing 422 on save.
 */
export function youtubeVideoId(raw: string): string | null {
  const trimmed = (raw ?? '').trim()
  if (!trimmed) return null

  let u: URL
  try {
    u = new URL(trimmed)
  } catch {
    return null
  }

  // http(s) only — this is what closes javascript: and data:.
  if (u.protocol !== 'http:' && u.protocol !== 'https:') return null

  // URL.hostname already strips userinfo, so "https://youtube.com@evil.com/x"
  // is matched as evil.com — which is exactly what must be rejected.
  const host = u.hostname.toLowerCase().replace(/\.$/, '')
  if (!YOUTUBE_HOSTS.has(host)) return null

  let id: string
  if (host === 'youtu.be') id = firstSegment(u.pathname)
  else if (u.pathname === '/watch') id = u.searchParams.get('v') ?? ''
  else if (u.pathname.startsWith('/shorts/')) id = firstSegment(u.pathname.slice('/shorts'.length))
  else return null

  return YOUTUBE_ID_RE.test(id) ? id : null
}

function firstSegment(path: string): string {
  const p = path.replace(/^\//, '')
  const i = p.indexOf('/')
  return i >= 0 ? p.slice(0, i) : p
}

/**
 * Closed icon list for a guide module — the nine icons the seeded modules use
 * plus a few general ones. Deliberately not a free-text field: a typo would
 * render an empty box on a public page, and the reader page has no way to
 * validate an arbitrary icon name.
 */
export const GUIDE_ICON_CHOICES: readonly string[] = [
  'i-lucide-log-in',
  'i-lucide-layout-dashboard',
  'i-lucide-package',
  'i-lucide-hand',
  'i-lucide-check-square',
  'i-lucide-wrench',
  'i-lucide-arrow-right-left',
  'i-lucide-bar-chart-2',
  'i-lucide-database',
  'i-lucide-clipboard-check',
  'i-lucide-trending-down',
  'i-lucide-shield-check',
  'i-lucide-users',
  'i-lucide-key',
  'i-lucide-building-2',
  'i-lucide-book-open',
  'i-lucide-settings',
  'i-lucide-info'
]

/**
 * PDF upload ceiling, mirroring the backend default of GUIDE_PDF_MAX_BYTES.
 *
 * 10 MB, not the 20 MB the spec floated: production traffic passes through
 * Coraza with the recommended config, whose SecRequestBodyLimit rejects bodies
 * over roughly 12.5 MB before they ever reach the backend. Raising this number
 * means raising the WAF limit first. Kept as one constant so the drop-zone text
 * and the pre-flight size check can never disagree with each other.
 */
export const GUIDE_PDF_MAX_BYTES = 10 * 1024 * 1024

/** Attachments a single module may carry — mirrors the backend's own cap. */
export const GUIDE_MAX_ATTACHMENTS = 10

/**
 * Slug derived from the Indonesian title. The slug is the stable key used by the
 * seed migration and by deep links, but it is not worth a field of its own in
 * the editor: authors think in titles. Renaming a module keeps its original slug
 * (see the page), so existing links never break.
 */
export function guideSlug(title: string): string {
  return (title ?? '')
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 120)
    .replace(/-+$/g, '')
}

/** Human-readable file size for the document card ("1,4 MB"). */
export function formatFileSize(bytes: number | undefined, locale: string): string {
  if (!bytes || bytes <= 0) return ''
  const mb = bytes / (1024 * 1024)
  if (mb >= 1) return `${mb.toLocaleString(locale, { maximumFractionDigits: 1 })} MB`
  const kb = bytes / 1024
  return `${kb.toLocaleString(locale, { maximumFractionDigits: 0 })} KB`
}
