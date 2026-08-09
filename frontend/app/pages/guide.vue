<script setup lang="ts">
import type { GuideModule } from '~/types'

definePageMeta({ layout: 'info' })

const { t, locale } = useI18n()
const can = useCan()
const guideApi = useGuide()

const modules = ref<GuideModule[]>([])
const loading = ref(true)
const failed = ref(false)

// Authors get the draft modules too, marked as such, so the public page doubles
// as the preview rather than needing a third layout.
const isAuthor = computed(() => can('guide.manage'))

async function load() {
  loading.value = true
  failed.value = false
  try {
    // This page is reachable without a session. useGuide().list() is the only
    // call made here for exactly that reason: it answers guests as well as
    // signed-in readers, so a visitor never triggers the 401 path that would
    // clear the session and bounce them to /login.
    modules.value = await guideApi.list(isAuthor.value)
  } catch {
    failed.value = true
    modules.value = []
  } finally {
    loading.value = false
  }
}

onMounted(load)

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

    <!-- loading -->
    <div
      v-if="loading"
      class="space-y-4"
    >
      <UCard
        v-for="i in 3"
        :key="i"
      >
        <div class="flex items-start gap-3.5">
          <USkeleton class="size-10 rounded-[10px] flex-none" />
          <div class="min-w-0 flex-1 space-y-2.5">
            <USkeleton class="h-4 w-48" />
            <USkeleton class="h-3 w-full" />
            <USkeleton class="h-3 w-4/5" />
            <USkeleton
              v-if="i === 1"
              class="w-full aspect-video rounded-xl mt-3"
            />
          </div>
        </div>
      </UCard>
    </div>

    <!-- error -->
    <UCard v-else-if="failed">
      <EmptyState
        icon="i-lucide-wifi-off"
        :title="t('guidePage.error.title')"
        :description="t('guidePage.error.description')"
      >
        <template #action>
          <UButton
            icon="i-lucide-rotate-ccw"
            @click="load"
          >
            {{ t('common.retry') }}
          </UButton>
        </template>
      </EmptyState>
    </UCard>

    <!-- empty -->
    <UCard v-else-if="modules.length === 0">
      <EmptyState
        icon="i-lucide-book-open"
        :title="t('guidePage.empty.title')"
        :description="t('guidePage.empty.description')"
      />
    </UCard>

    <!-- data -->
    <div
      v-else
      class="space-y-4"
    >
      <UCard
        v-for="m in modules"
        :key="m.id"
      >
        <div class="flex items-start gap-3.5">
          <span class="w-10 h-10 rounded-[10px] bg-primary/10 text-primary flex items-center justify-center flex-none">
            <UIcon
              :name="m.icon || 'i-lucide-circle-dot'"
              class="size-5"
            />
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap mb-1">
              <h2 class="text-base font-semibold tracking-tight">
                {{ guideText(m.title_id, m.title_en, locale) }}
              </h2>
              <UBadge
                v-if="m.status === 'draft'"
                color="warning"
                variant="subtle"
                size="sm"
              >
                {{ t('guidePage.draft') }}
              </UBadge>
            </div>

            <p
              v-if="guideText(m.body_id, m.body_en, locale)"
              class="text-[14.5px] leading-relaxed text-muted"
            >
              {{ guideText(m.body_id, m.body_en, locale) }}
            </p>

            <ol
              v-if="m.steps.length"
              class="mt-3 space-y-2"
            >
              <li
                v-for="(step, idx) in m.steps"
                :key="idx"
                class="flex gap-3 text-[14px] leading-relaxed text-muted"
              >
                <span class="w-[22px] h-[22px] rounded-full bg-muted text-default text-[12px] font-semibold flex items-center justify-center flex-none">
                  {{ idx + 1 }}
                </span>
                <span class="pt-[1px]">{{ guideText(step.text_id, step.text_en, locale) }}</span>
              </li>
            </ol>

            <!-- attachments -->
            <div
              v-if="m.attachments.length"
              class="mt-[18px] pt-4 border-t border-default"
            >
              <div class="flex items-center gap-1.5 mb-3 text-muted">
                <UIcon
                  name="i-lucide-paperclip"
                  class="size-3.5"
                />
                <span class="text-[11px] font-semibold font-mono tracking-[0.12em] uppercase">
                  {{ t('guidePage.media.label', { n: m.attachments.length }) }}
                </span>
              </div>
              <div class="flex flex-col gap-3.5">
                <GuideMediaCard
                  v-for="att in m.attachments"
                  :key="att.id"
                  :attachment="att"
                />
              </div>
            </div>
          </div>
        </div>
      </UCard>
    </div>
  </div>
</template>
