<script setup lang="ts">
definePageMeta({ layout: 'info' })

const { t, tm, rt, locale } = useI18n()

interface Section {
  id: string
  heading: string
  body?: string
  points: string[]
}

// tm() returns the raw locale messages for the current locale; rt() resolves
// each leaf to a plain string. Touching locale.value keeps the list reactive
// when the user switches language.
const sections = computed<Section[]>(() => {
  void locale.value
  const raw = tm('privacyPage.sections') as unknown as Array<Record<string, unknown>>
  return raw.map((s, i) => ({
    id: `privacy-sec-${i + 1}`,
    heading: rt(s.heading as string),
    body: s.body ? rt(s.body as string) : undefined,
    points: Array.isArray(s.points) ? (s.points as string[]).map(p => rt(p)) : []
  }))
})

useSeoMeta({ title: () => `${t('privacyPage.title')} - ${t('app.name')}` })
</script>

<template>
  <div class="max-w-3xl mx-auto">
    <PageHeader
      :title="t('privacyPage.title')"
      :subtitle="t('privacyPage.subtitle')"
    />

    <div class="flex items-center gap-2 mb-6 text-[13px] text-muted">
      <UIcon
        name="i-lucide-calendar-clock"
        class="size-4"
      />
      {{ t('privacyPage.updated') }}
    </div>

    <p class="text-[15px] leading-relaxed text-default mb-8">
      {{ t('privacyPage.intro') }}
    </p>

    <!-- Table of contents -->
    <UCard class="mb-8">
      <div class="text-[12px] font-semibold uppercase tracking-wide text-dimmed mb-3">
        {{ t('privacyPage.tocTitle') }}
      </div>
      <ol class="space-y-1.5">
        <li
          v-for="sec in sections"
          :key="sec.id"
        >
          <a
            :href="`#${sec.id}`"
            class="text-[14px] text-primary hover:underline"
          >{{ sec.heading }}</a>
        </li>
      </ol>
    </UCard>

    <div class="space-y-8">
      <section
        v-for="sec in sections"
        :id="sec.id"
        :key="sec.id"
        class="scroll-mt-[76px]"
      >
        <h2 class="text-lg font-semibold tracking-tight mb-2">
          {{ sec.heading }}
        </h2>
        <p
          v-if="sec.body"
          class="text-[15px] leading-relaxed text-muted"
        >
          {{ sec.body }}
        </p>
        <ul
          v-if="sec.points.length"
          class="mt-3 space-y-2"
        >
          <li
            v-for="(point, idx) in sec.points"
            :key="idx"
            class="flex gap-2.5 text-[14.5px] leading-relaxed text-muted"
          >
            <UIcon
              name="i-lucide-check"
              class="size-4 mt-1 text-primary flex-none"
            />
            <span>{{ point }}</span>
          </li>
        </ul>
      </section>
    </div>
  </div>
</template>
