// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import RowActionsMenu from '~/components/RowActionsMenu.vue'

describe('RowActionsMenu', () => {
  it('renders a kebab trigger with an aria-label when items are provided', async () => {
    const wrapper = await mountSuspended(RowActionsMenu, {
      props: {
        items: [
          { label: 'Edit', icon: 'i-lucide-pencil', onSelect: () => {} },
          { label: 'Delete', color: 'error', separator: true, onSelect: () => {} }
        ]
      }
    })
    const trigger = wrapper.find('button[aria-haspopup="menu"]')
    expect(trigger.exists()).toBe(true)
    expect(trigger.text().trim()).toBe('')
    expect(trigger.attributes('aria-label')).toBe('Aksi')
  })

  it('opens to show both labels and invokes onSelect for the clicked item', async () => {
    let edited = 0
    let deleted = 0
    const wrapper = await mountSuspended(RowActionsMenu, {
      props: {
        items: [
          { label: 'Edit', icon: 'i-lucide-pencil', onSelect: () => { edited++ } },
          { label: 'Delete', color: 'error', separator: true, onSelect: () => { deleted++ } }
        ]
      }
    })
    await wrapper.find('button[aria-haspopup="menu"]').trigger('click')
    await new Promise(r => setTimeout(r, 0))
    const items = Array.from(document.querySelectorAll('[role="menuitem"]')) as HTMLElement[]
    const labels = items.map(i => i.textContent?.trim())
    expect(labels).toContain('Edit')
    expect(labels).toContain('Delete')

    items.find(i => i.textContent?.includes('Edit'))?.click()
    expect(edited).toBe(1)
    expect(deleted).toBe(0)
  })

  it('renders nothing when items is empty', async () => {
    const wrapper = await mountSuspended(RowActionsMenu, {
      props: { items: [] }
    })
    expect(wrapper.find('button[aria-haspopup="menu"]').exists()).toBe(false)
    expect(wrapper.find('button').exists()).toBe(false)
  })
})
