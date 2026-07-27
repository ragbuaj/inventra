<script setup lang="ts">
definePageMeta({ layout: 'info' })

const { t, tm, rt, locale } = useI18n()

interface Section {
  heading: string
  icon: string
  body?: string
  steps: string[]
}

const sections = computed<Section[]>(() => {
  void locale.value
  const raw = tm('guidePage.sections') as unknown as Array<Record<string, unknown>>
  return raw.map(s => ({
    heading: rt(s.heading as string),
    icon: (s.icon as string) || 'i-lucide-circle-dot',
    body: s.body ? rt(s.body as string) : undefined,
    steps: Array.isArray(s.steps) ? (s.steps as string[]).map(step => rt(step)) : []
  }))
})

useSeoMeta({ title: () => `${t('guidePage.title')} - ${t('app.name')}` })
</script>

<template>
  <div class="max-w-3xl mx-auto">
    <PageHeader
      :title="t('guidePage.title')"
      :subtitle="t('guidePage.subtitle')"
    />

    <p class="text-[15px] leading-relaxed text-default mb-8">
      {{ t('guidePage.intro') }}
    </p>

    <div class="space-y-4">
      <UCard
        v-for="(sec, i) in sections"
        :key="i"
      >
        <div class="flex items-start gap-3.5">
          <span class="w-10 h-10 rounded-[10px] bg-primary/10 text-primary flex items-center justify-center flex-none">
            <UIcon
              :name="sec.icon"
              class="size-5"
            />
          </span>
          <div class="min-w-0 flex-1">
            <h2 class="text-base font-semibold tracking-tight mb-1">
              {{ sec.heading }}
            </h2>
            <p
              v-if="sec.body"
              class="text-[14.5px] leading-relaxed text-muted"
            >
              {{ sec.body }}
            </p>
            <ol
              v-if="sec.steps.length"
              class="mt-3 space-y-2"
            >
              <li
                v-for="(step, idx) in sec.steps"
                :key="idx"
                class="flex gap-3 text-[14px] leading-relaxed text-muted"
              >
                <span class="w-[22px] h-[22px] rounded-full bg-muted text-default text-[12px] font-semibold flex items-center justify-center flex-none">
                  {{ idx + 1 }}
                </span>
                <span class="pt-[1px]">{{ step }}</span>
              </li>
            </ol>
          </div>
        </div>
      </UCard>
    </div>
  </div>
</template>
