// @vitest-environment nuxt
import { describe, it, expect, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { useAuthStore } from '~/stores/auth'
import { useCan } from '~/composables/useCan'
import { defineComponent, computed } from 'vue'

// Helper: mount a minimal Vue component that exposes useCan result
function CanWrapper(permission: string) {
  return defineComponent({
    setup() {
      const can = useCan()
      const result = computed(() => can(permission))
      return { result }
    },
    template: '<span>{{ result }}</span>'
  })
}

describe('useCan', () => {
  beforeEach(() => {
    // Reset the auth store before each test by calling clear()
    // useAuthStore() here uses the Nuxt app's Pinia instance (the same one
    // that the component under test will use), so mutations are visible.
    useAuthStore().clear()
  })

  it('returns true for a permission the user has', async () => {
    useAuthStore().setSession(
      'tok',
      { id: '1', name: 'Test', email: 'test@e.com', role_id: 'r1', role_name: 'Role' },
      ['asset.read']
    )

    const wrapper = await mountSuspended(CanWrapper('asset.read'))
    expect(wrapper.text()).toBe('true')
  })

  it('returns false for a permission the user does not have', async () => {
    useAuthStore().setSession(
      'tok',
      { id: '1', name: 'Test', email: 'test@e.com', role_id: 'r1', role_name: 'Role' },
      ['asset.read']
    )

    const wrapper = await mountSuspended(CanWrapper('user.manage'))
    expect(wrapper.text()).toBe('false')
  })

  it('returns true for any permission when wildcard * is present', async () => {
    useAuthStore().setSession(
      'tok',
      { id: '1', name: 'Test', email: 'test@e.com', role_id: 'r1', role_name: 'Role' },
      ['*']
    )

    const wrapper = await mountSuspended(CanWrapper('user.manage'))
    expect(wrapper.text()).toBe('true')
  })

  it('returns false when permissions are empty', async () => {
    useAuthStore().setSession(
      'tok',
      { id: '1', name: 'Test', email: 'test@e.com', role_id: 'r1', role_name: 'Role' },
      []
    )

    const wrapper = await mountSuspended(CanWrapper('asset.read'))
    expect(wrapper.text()).toBe('false')
  })
})
