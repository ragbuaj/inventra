// Presentation metadata for the authz permission catalog. The API catalog
// supplies the authoritative permission keys + grouping; icons (and i18n
// labels, see locale files) are a frontend concern.
export const GROUP_ICON: Record<string, string> = {
  'Sistem': 'i-lucide-shield',
  'Master Data': 'i-lucide-database',
  'Aset': 'i-lucide-box',
  'Persetujuan': 'i-lucide-git-pull-request',
  'Stock Opname': 'i-lucide-clipboard-check',
  'Cadangan': 'i-lucide-layers'
}

export function iconForGroup(group: string): string {
  return GROUP_ICON[group] ?? 'i-lucide-key'
}
