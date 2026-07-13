import { defineStore } from 'pinia'

export const useInboxStore = defineStore('inbox', {
  state: () => ({ pendingCount: 0 }),
  actions: {
    async refresh() {
      const can = useCan()
      if (!can('request.decide')) {
        this.pendingCount = 0
        return
      }
      try {
        this.pendingCount = await useApproval().inboxCount()
      } catch {
        // keep last known count
      }
    }
  }
})
