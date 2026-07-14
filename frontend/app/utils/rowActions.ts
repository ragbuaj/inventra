import type { DropdownMenuItem } from '@nuxt/ui'
import type { RowAction } from '~/types'

// Map row actions to grouped menu items; a `separator` flag starts a new group
// so a divider is drawn before it (e.g. destructive actions). Shared by both
// the row dropdown (RowActionsMenu) and any right-click context menu.
export function buildActionGroups(items: RowAction[]): DropdownMenuItem[][] {
  const groups: DropdownMenuItem[][] = []
  let current: DropdownMenuItem[] = []
  for (const a of items) {
    if (a.separator && current.length) {
      groups.push(current)
      current = []
    }
    current.push({
      label: a.label,
      icon: a.icon,
      color: a.color,
      disabled: a.disabled,
      onSelect: () => a.onSelect?.()
    })
  }
  if (current.length) groups.push(current)
  return groups
}
