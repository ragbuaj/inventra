// Scope-level presentation metadata. The backend supplies the authoritative
// scope_modules + scope_levels via /authz/catalog; tone (color) and i18n
// descriptions are a frontend concern.
export const SCOPE_LEVEL_KEYS = ['global', 'office_subtree', 'office', 'own'] as const
export type ScopeLevel = typeof SCOPE_LEVEL_KEYS[number]
export type ScopeTone = 'info' | 'primary' | 'warning' | 'neutral'

export const SCOPE_LEVEL_TONE: Record<ScopeLevel, ScopeTone> = {
  global: 'info',
  office_subtree: 'primary',
  office: 'warning',
  own: 'neutral'
}
