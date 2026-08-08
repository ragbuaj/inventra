import { describe, it, expect } from 'vitest'
import {
  guideText,
  guideNeedsTranslation,
  youtubeEmbedUrl,
  youtubeThumbnailUrl,
  youtubeVideoId,
  guideSlug,
  formatFileSize,
  GUIDE_ICON_CHOICES,
  GUIDE_PDF_MAX_BYTES,
  GUIDE_MAX_ATTACHMENTS
} from '~/utils/guideText'

describe('guideText', () => {
  it('returns the Indonesian text for the id locale', () => {
    expect(guideText('Masuk dan Keamanan Akun', 'Sign In and Account Security', 'id'))
      .toBe('Masuk dan Keamanan Akun')
  })

  it('returns the English text for the en locale', () => {
    expect(guideText('Masuk dan Keamanan Akun', 'Sign In and Account Security', 'en'))
      .toBe('Sign In and Account Security')
  })

  it('falls back to Indonesian when English is missing', () => {
    expect(guideText('Katalog Aset', null, 'en')).toBe('Katalog Aset')
    expect(guideText('Katalog Aset', undefined, 'en')).toBe('Katalog Aset')
  })

  // An editor who cleared the field meant "not translated", not "show nothing".
  it('treats blank and whitespace-only English as untranslated', () => {
    expect(guideText('Katalog Aset', '', 'en')).toBe('Katalog Aset')
    expect(guideText('Katalog Aset', '   ', 'en')).toBe('Katalog Aset')
    expect(guideText('Katalog Aset', '\n\t', 'en')).toBe('Katalog Aset')
  })

  it('handles regional english locales', () => {
    expect(guideText('Katalog', 'Catalog', 'en-US')).toBe('Catalog')
    expect(guideText('Katalog', 'Catalog', 'en-GB')).toBe('Catalog')
  })

  it('never returns English for a non-english locale, even when present', () => {
    expect(guideText('Katalog', 'Catalog', 'id-ID')).toBe('Katalog')
  })

  it('trims surrounding whitespace from whichever value it picks', () => {
    expect(guideText('  Katalog  ', null, 'id')).toBe('Katalog')
    expect(guideText('Katalog', '  Catalog  ', 'en')).toBe('Catalog')
  })

  it('degrades to an empty string when both are missing', () => {
    expect(guideText(null, null, 'id')).toBe('')
    expect(guideText(undefined, undefined, 'en')).toBe('')
    expect(guideText('', '', 'en')).toBe('')
  })
})

describe('guideNeedsTranslation', () => {
  it('is false when every field has a value', () => {
    expect(guideNeedsTranslation(['Title', 'Body'])).toBe(false)
  })

  it('is true when any field is missing, empty, or blank', () => {
    expect(guideNeedsTranslation(['Title', null])).toBe(true)
    expect(guideNeedsTranslation(['Title', ''])).toBe(true)
    expect(guideNeedsTranslation(['Title', '   '])).toBe(true)
    expect(guideNeedsTranslation([undefined])).toBe(true)
  })

  it('is false for an empty field list — nothing to translate', () => {
    expect(guideNeedsTranslation([])).toBe(false)
  })
})

describe('youtubeEmbedUrl', () => {
  it('builds a youtube-nocookie embed url', () => {
    expect(youtubeEmbedUrl('dQw4w9WgXcQ'))
      .toBe('https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?rel=0')
  })

  it('never uses the tracking youtube.com host', () => {
    expect(youtubeEmbedUrl('dQw4w9WgXcQ')).not.toMatch(/\/\/(www\.)?youtube\.com/)
  })

  // The API only ever returns ids it validated, but the URL is still built by
  // encoding rather than concatenation so a malformed value cannot break out of
  // the src attribute.
  it('encodes anything that is not a plain id', () => {
    expect(youtubeEmbedUrl('a/b?c=d')).toBe('https://www.youtube-nocookie.com/embed/a%2Fb%3Fc%3Dd?rel=0')
    expect(youtubeEmbedUrl('"><script>')).not.toContain('<script>')
  })

  it('builds a matching thumbnail url', () => {
    expect(youtubeThumbnailUrl('dQw4w9WgXcQ')).toContain('dQw4w9WgXcQ')
  })
})

describe('youtubeVideoId', () => {
  it('accepts the three shapes the backend accepts', () => {
    expect(youtubeVideoId('https://www.youtube.com/watch?v=dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ')
    expect(youtubeVideoId('https://youtu.be/dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ')
    expect(youtubeVideoId('https://www.youtube.com/shorts/dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ')
  })

  it('accepts the bare, m., and http variants of the host', () => {
    expect(youtubeVideoId('https://youtube.com/watch?v=dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ')
    expect(youtubeVideoId('https://m.youtube.com/watch?v=dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ')
    expect(youtubeVideoId('http://www.youtube.com/watch?v=dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ')
  })

  it('drops extra parameters instead of rejecting the link', () => {
    expect(youtubeVideoId('https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s&list=PL123'))
      .toBe('dQw4w9WgXcQ')
    expect(youtubeVideoId('https://youtu.be/dQw4w9WgXcQ?si=abcdef')).toBe('dQw4w9WgXcQ')
  })

  it('tolerates surrounding whitespace and a trailing dot on the host', () => {
    expect(youtubeVideoId('  https://youtu.be/dQw4w9WgXcQ  ')).toBe('dQw4w9WgXcQ')
    expect(youtubeVideoId('https://www.youtube.com./watch?v=dQw4w9WgXcQ')).toBe('dQw4w9WgXcQ')
  })

  // The host is matched exactly. Suffix matching would accept the first of
  // these, substring matching the second, and userinfo the third.
  it('rejects look-alike hosts', () => {
    expect(youtubeVideoId('https://youtube.com.evil.example.com/watch?v=dQw4w9WgXcQ')).toBeNull()
    expect(youtubeVideoId('https://evil.example.com/youtube.com/watch?v=dQw4w9WgXcQ')).toBeNull()
    expect(youtubeVideoId('https://youtube.com@evil.example.com/watch?v=dQw4w9WgXcQ')).toBeNull()
    expect(youtubeVideoId('https://notyoutube.com/watch?v=dQw4w9WgXcQ')).toBeNull()
  })

  it('rejects non-http(s) schemes', () => {
    expect(youtubeVideoId('javascript:alert(1)')).toBeNull()
    expect(youtubeVideoId('data:text/html,<script>alert(1)</script>')).toBeNull()
    expect(youtubeVideoId('ftp://youtube.com/watch?v=dQw4w9WgXcQ')).toBeNull()
  })

  it('rejects a YouTube host with no usable video id', () => {
    expect(youtubeVideoId('https://www.youtube.com/')).toBeNull()
    expect(youtubeVideoId('https://www.youtube.com/watch')).toBeNull()
    expect(youtubeVideoId('https://www.youtube.com/watch?v=short')).toBeNull()
    expect(youtubeVideoId('https://www.youtube.com/watch?v=waytoolongforanid')).toBeNull()
    expect(youtubeVideoId('https://www.youtube.com/watch?v=has spaces')).toBeNull()
    expect(youtubeVideoId('https://www.youtube.com/channel/UCabcdefghij')).toBeNull()
    expect(youtubeVideoId('https://youtu.be/')).toBeNull()
  })

  it('rejects blank input and anything that is not a URL', () => {
    expect(youtubeVideoId('')).toBeNull()
    expect(youtubeVideoId('   ')).toBeNull()
    expect(youtubeVideoId('dQw4w9WgXcQ')).toBeNull()
    expect(youtubeVideoId('www.youtube.com/watch?v=dQw4w9WgXcQ')).toBeNull()
  })

  it('accepts an id that uses the full URL-safe alphabet', () => {
    expect(youtubeVideoId('https://youtu.be/a_B-c1D2e3F')).toBe('a_B-c1D2e3F')
  })
})

describe('guideSlug', () => {
  it('kebab-cases an Indonesian title', () => {
    expect(guideSlug('Masuk dan Keamanan Akun')).toBe('masuk-dan-keamanan-akun')
  })

  it('collapses punctuation and repeated separators', () => {
    expect(guideSlug('Pemeliharaan (Maintenance)')).toBe('pemeliharaan-maintenance')
    expect(guideSlug('Mutasi, Stock Opname, dan Penghapusan'))
      .toBe('mutasi-stock-opname-dan-penghapusan')
  })

  it('never leaves a leading or trailing separator', () => {
    expect(guideSlug('  Dashboard!  ')).toBe('dashboard')
    expect(guideSlug('---Laporan---')).toBe('laporan')
  })

  it('caps the slug at the column length without a dangling dash', () => {
    const slug = guideSlug('a'.repeat(130))
    expect(slug.length).toBeLessThanOrEqual(120)
    expect(slug.endsWith('-')).toBe(false)
    // The 120-char cut lands exactly on a separator here; it must not survive.
    expect(guideSlug(`${'a'.repeat(119)} b`)).toBe('a'.repeat(119))
  })

  it('degrades to an empty slug rather than throwing on unusable titles', () => {
    expect(guideSlug('')).toBe('')
    expect(guideSlug('???')).toBe('')
  })
})

describe('guide constants', () => {
  it('caps PDFs at 10 MB — the WAF rejects bodies over roughly 12.5 MB', () => {
    expect(GUIDE_PDF_MAX_BYTES).toBe(10 * 1024 * 1024)
    expect(formatFileSize(GUIDE_PDF_MAX_BYTES, 'id')).toBe('10 MB')
  })

  it('mirrors the backend attachment cap', () => {
    expect(GUIDE_MAX_ATTACHMENTS).toBe(10)
  })

  it('offers a closed icon list of valid lucide names with no duplicates', () => {
    expect(GUIDE_ICON_CHOICES.length).toBeGreaterThanOrEqual(9)
    expect(new Set(GUIDE_ICON_CHOICES).size).toBe(GUIDE_ICON_CHOICES.length)
    for (const icon of GUIDE_ICON_CHOICES) expect(icon).toMatch(/^i-lucide-[a-z0-9-]+$/)
  })
})

describe('formatFileSize', () => {
  it('renders megabytes with one decimal', () => {
    expect(formatFileSize(1_400_000, 'id')).toMatch(/1,3 MB|1.3 MB/)
  })

  it('renders kilobytes below one megabyte', () => {
    expect(formatFileSize(120_000, 'id')).toMatch(/KB$/)
  })

  it('returns an empty string for missing or nonsensical sizes', () => {
    expect(formatFileSize(undefined, 'id')).toBe('')
    expect(formatFileSize(0, 'id')).toBe('')
    expect(formatFileSize(-5, 'id')).toBe('')
  })
})
