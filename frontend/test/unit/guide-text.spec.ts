import { describe, it, expect } from 'vitest'
import {
  guideText,
  guideNeedsTranslation,
  youtubeEmbedUrl,
  youtubeThumbnailUrl,
  formatFileSize
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
