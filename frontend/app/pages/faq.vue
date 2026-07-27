<script setup lang="ts">
import type { AccordionItem } from '@nuxt/ui'

definePageMeta({ layout: 'info' })

const { t, tm, rt, locale } = useI18n()

interface FaqItem {
  category: string
  q: string
  a: string
}

const allItems = computed<FaqItem[]>(() => {
  void locale.value
  const raw = tm('faqPage.items') as unknown as Array<Record<string, unknown>>
  return raw.map(it => ({
    category: rt(it.category as string),
    q: rt(it.q as string),
    a: rt(it.a as string)
  }))
})

const query = ref('')

const filtered = computed<FaqItem[]>(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return allItems.value
  return allItems.value.filter(it =>
    it.q.toLowerCase().includes(q)
    || it.a.toLowerCase().includes(q)
    || it.category.toLowerCase().includes(q)
  )
})

// Group the filtered items by category, preserving first-seen order, and map
// each group to Nuxt UI accordion items.
interface FaqGroup {
  category: string
  items: AccordionItem[]
}

const groups = computed<FaqGroup[]>(() => {
  const order: string[] = []
  const byCat = new Map<string, AccordionItem[]>()
  filtered.value.forEach((it, i) => {
    if (!byCat.has(it.category)) {
      byCat.set(it.category, [])
      order.push(it.category)
    }
    byCat.get(it.category)!.push({
      label: it.q,
      content: it.a,
      value: `faq-${i}`
    })
  })
  return order.map(category => ({ category, items: byCat.get(category)! }))
})

useSeoMeta({ title: () => `${t('faqPage.title')} - ${t('app.name')}` })
</script>

<template>
  <div class="max-w-3xl mx-auto">
    <PageHeader
      :title="t('faqPage.title')"
      :subtitle="t('faqPage.subtitle')"
    />

    <UInput
      v-model="query"
      icon="i-lucide-search"
      :placeholder="t('faqPage.searchPlaceholder')"
      size="lg"
      class="w-full mb-6"
    />

    <EmptyState
      v-if="!groups.length"
      icon="i-lucide-search-x"
      :title="t('faqPage.empty')"
    />

    <div
      v-else
      class="space-y-7"
    >
      <section
        v-for="group in groups"
        :key="group.category"
      >
        <h2 class="text-[12px] font-semibold uppercase tracking-wide text-dimmed mb-2">
          {{ group.category }}
        </h2>
        <UAccordion
          :items="group.items"
          type="multiple"
        />
      </section>
    </div>
  </div>
</template>
