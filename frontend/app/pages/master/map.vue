<script setup lang="ts">
import type { MapOffice, OfficeTier } from '~/types'
import { tierMeta, TIER_ORDER } from '~/constants/officeMapMeta'
import { googleMapsUrl } from '~/utils/googleMapsUrl'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const { list } = useOfficeMap()

// --- State ---
const loading = ref(true)
const loadFailed = ref(false)
const offices = ref<MapOffice[]>([])
const q = ref('')
const fTier = ref<'all' | OfficeTier>('all')
const fProv = ref<string>('all')
const selId = ref<string | null>(null)
const mapRef = ref<{ resetView: () => void, zoomIn: () => void, zoomOut: () => void } | null>(null)

async function reload() {
  loading.value = true
  loadFailed.value = false
  try {
    offices.value = await list()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}
onMounted(reload)

// --- Derived ---
const provinces = computed(() => Array.from(new Set(offices.value.map(o => o.province_name).filter((p): p is string => !!p))).sort())

const filtered = computed(() => {
  const sq = q.value.trim().toLowerCase()
  return offices.value.filter((o) => {
    if (sq && !o.name.toLowerCase().includes(sq) && !o.code.toLowerCase().includes(sq)) return false
    if (fTier.value !== 'all' && o.tier !== fTier.value) return false
    if (fProv.value !== 'all' && o.province_name !== fProv.value) return false
    return true
  })
})

const mapped = computed(() => filtered.value.filter(o => o.latitude != null && o.longitude != null))

const summaryText = computed(() => {
  const cities = new Set(filtered.value.map(o => o.city_name).filter(Boolean)).size
  const provs = new Set(filtered.value.map(o => o.province_name).filter(Boolean)).size
  return t('map.summary', { o: filtered.value.length, k: cities, p: provs })
})

const selected = computed(() => filtered.value.find(o => o.id === selId.value) ?? null)

const tierItems = computed(() => [
  { value: 'all', label: t('map.jenisAll') },
  ...TIER_ORDER.map(j => ({ value: j, label: t(tierMeta[j].labelKey) }))
])

const provItems = computed(() => [
  { value: 'all', label: t('map.provAll') },
  ...provinces.value.map(p => ({ value: p, label: p }))
])

// Reset selection when filters change
watch(fTier, () => {
  selId.value = null
})
watch(fProv, () => {
  selId.value = null
})

// --- Actions ---
function selectOffice(id: string) {
  selId.value = id
}

function closeDetail() {
  selId.value = null
}

function resetView() {
  selId.value = null
  mapRef.value?.resetView()
}
</script>

<template>
  <div class="flex flex-col h-full min-h-0">
    <!-- Page header -->
    <div class="mb-3">
      <h1 class="text-2xl font-bold tracking-tight mb-1">
        {{ $t('map.title') }}
      </h1>
      <div class="inline-flex items-center gap-1.5 text-xs text-dimmed">
        <UIcon
          name="i-lucide-info"
          class="size-3.5 flex-none"
        />
        <span>{{ $t('map.usageNote') }}</span>
      </div>
    </div>

    <!-- Two-column layout -->
    <div class="flex gap-4 flex-1 min-h-0">
      <!-- LEFT: Office list panel (w-[312px]) -->
      <div class="w-[312px] flex-none flex flex-col bg-default border border-default rounded-[14px] shadow-sm overflow-hidden">
        <!-- Filters -->
        <div class="flex-none px-3 pt-3 pb-2.5 border-b border-default">
          <!-- Search -->
          <UInput
            v-model="q"
            :placeholder="$t('map.searchPlaceholder')"
            icon="i-lucide-search"
            size="sm"
            class="mb-2 w-full"
          />
          <!-- Tier + Provinsi selects -->
          <div class="flex gap-2">
            <USelect
              v-model="fTier"
              :items="tierItems"
              size="sm"
              class="flex-1"
            />
            <USelect
              v-model="fProv"
              :items="provItems"
              size="sm"
              class="flex-1"
            />
          </div>
        </div>

        <!-- Office list -->
        <div class="flex-1 overflow-y-auto p-2">
          <!-- Loading skeleton -->
          <template v-if="loading">
            <div
              v-for="n in 5"
              :key="n"
              class="flex items-center gap-2.5 px-2.5 py-2.5 mb-1"
            >
              <div class="size-[30px] rounded-lg bg-muted animate-pulse flex-none" />
              <div class="flex-1 flex flex-col gap-1.5">
                <div class="h-2.5 w-2/3 rounded bg-muted animate-pulse" />
                <div class="h-2 w-2/5 rounded bg-muted animate-pulse" />
              </div>
            </div>
          </template>

          <!-- Error state -->
          <div
            v-else-if="loadFailed"
            class="px-4 py-10 text-center"
          >
            <p class="text-[13.5px] font-semibold mb-2">
              {{ $t('map.loadError') }}
            </p>
            <UButton
              color="neutral"
              variant="subtle"
              size="sm"
              data-testid="map-retry"
              @click="reload"
            >
              {{ $t('map.retry') }}
            </UButton>
          </div>

          <!-- Populated rows -->
          <template v-else>
            <button
              v-for="office in filtered"
              :key="office.id"
              data-testid="office-row"
              class="flex items-start gap-2.5 w-full px-2.5 py-2.5 mb-1 rounded-[10px] border text-left cursor-pointer transition-colors hover:border-primary"
              :class="selId === office.id
                ? 'bg-primary/10 border-primary'
                : 'bg-default border-default'"
              @click="selectOffice(office.id)"
            >
              <!-- Pin icon -->
              <span
                class="size-[30px] rounded-lg flex items-center justify-center flex-none mt-0.5"
                :class="[tierMeta[office.tier].softBg, tierMeta[office.tier].softText]"
              >
                <UIcon
                  name="i-lucide-map-pin"
                  class="size-4"
                />
              </span>
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-1.5">
                  <span
                    class="text-[13px] font-semibold truncate"
                    :class="selId === office.id ? 'text-primary' : ''"
                  >
                    {{ office.name }}
                  </span>
                </div>
                <div class="flex items-center gap-1.5 mt-0.5 flex-wrap">
                  <span
                    class="px-1.5 py-px text-[10px] font-semibold rounded-full"
                    :class="[tierMeta[office.tier].softBg, tierMeta[office.tier].softText]"
                  >
                    {{ $t(tierMeta[office.tier].labelKey) }}
                  </span>
                  <span class="font-mono text-[11px] text-dimmed">{{ office.code }}</span>
                </div>
                <div class="text-[11.5px] text-muted mt-0.5 truncate">
                  {{ office.city_name }}{{ office.province_name ? ', ' + office.province_name : '' }}
                </div>
              </div>
            </button>

            <!-- Empty state -->
            <div
              v-if="filtered.length === 0"
              class="px-4 py-10 text-center"
            >
              <div class="size-[42px] mx-auto mb-3 rounded-[11px] bg-muted text-dimmed flex items-center justify-center">
                <UIcon
                  name="i-lucide-search"
                  class="size-5"
                />
              </div>
              <p class="text-[13.5px] font-semibold mb-1">
                {{ $t('map.emptyListTitle') }}
              </p>
              <p class="text-xs text-muted">
                {{ $t('map.emptyListSub') }}
              </p>
            </div>
          </template>
        </div>
      </div>

      <!-- RIGHT: Map panel -->
      <div class="flex-1 flex flex-col min-w-0 bg-default border border-default rounded-[14px] shadow-sm overflow-hidden">
        <!-- Header: summary strip + legend -->
        <div class="flex-none flex items-center justify-between gap-3 flex-wrap px-4 py-3 border-b border-default">
          <span class="inline-flex items-center gap-2 text-[13px] font-semibold">
            <UIcon
              name="i-lucide-map"
              class="size-4 text-primary flex-none"
            />
            {{ summaryText }}
          </span>
          <div class="flex items-center gap-3.5 flex-wrap">
            <div class="flex items-center gap-3">
              <span
                v-for="j in TIER_ORDER"
                :key="j"
                class="inline-flex items-center gap-1.5 text-[11.5px] font-medium text-muted"
              >
                <span
                  class="size-2 rounded-full"
                  :class="tierMeta[j].softBg"
                  :style="{ background: `var(${tierMeta[j].pinVar})` }"
                />
                {{ $t(tierMeta[j].labelKey) }}
              </span>
            </div>
          </div>
        </div>

        <!-- Map area -->
        <div class="flex-1 relative overflow-hidden bg-info/5">
          <!-- Loading shimmer -->
          <div
            v-if="loading"
            class="absolute inset-0 bg-muted animate-pulse"
          />

          <!-- Map + controls (loaded) -->
          <template v-else>
            <!-- Leaflet map -->
            <ClientOnly>
              <OfficeMap
                ref="mapRef"
                :offices="mapped"
                :selected-id="selId"
                @select="(id) => selId = id"
              />
            </ClientOnly>

            <!-- Empty map overlay -->
            <div
              v-if="mapped.length === 0"
              class="absolute inset-0 flex flex-col items-center justify-center gap-2.5"
              style="background: color-mix(in srgb, var(--ui-bg) 55%, transparent)"
            >
              <div class="size-[50px] rounded-[13px] bg-default text-dimmed flex items-center justify-center shadow-sm">
                <UIcon
                  name="i-lucide-map-pin-off"
                  class="size-6"
                />
              </div>
              <p class="text-[14.5px] font-semibold">
                {{ $t('map.emptyMapTitle') }}
              </p>
              <p class="text-[12.5px] text-muted">
                {{ $t('map.emptyMapSub') }}
              </p>
            </div>

            <!-- Zoom controls (top-right) -->
            <div class="absolute top-3.5 right-3.5 flex flex-col gap-px bg-default border border-default rounded-[10px] shadow-sm overflow-hidden">
              <button
                class="flex items-center justify-center size-[34px] text-muted hover:bg-muted hover:text-default transition-colors"
                :title="$t('common.add')"
                @click="mapRef?.zoomIn()"
              >
                <UIcon
                  name="i-lucide-plus"
                  class="size-4"
                />
              </button>
              <div class="h-px bg-border" />
              <button
                class="flex items-center justify-center size-[34px] text-muted hover:bg-muted hover:text-default transition-colors"
                :title="$t('common.reset')"
                @click="mapRef?.zoomOut()"
              >
                <UIcon
                  name="i-lucide-minus"
                  class="size-4"
                />
              </button>
            </div>

            <!-- Reset View button (bottom-right) -->
            <button
              class="absolute bottom-3.5 right-3.5 inline-flex items-center gap-1.5 px-3 py-[7px] text-[12.5px] font-medium text-default bg-default border border-default rounded-[9px] shadow-sm hover:bg-muted transition-colors"
              :title="$t('map.resetTip')"
              @click="resetView"
            >
              <UIcon
                name="i-lucide-rotate-ccw"
                class="size-3.5"
              />
              {{ $t('map.resetLabel') }}
            </button>

            <!-- Detail card (bottom-left) -->
            <Transition name="slide-up">
              <div
                v-if="selected"
                data-testid="office-detail-card"
                class="absolute left-4 bottom-4 w-[312px] max-w-[calc(100%-32px)] bg-elevated border border-default rounded-[14px] shadow-xl overflow-hidden"
              >
                <!-- Header row: icon + name/tier/code + close -->
                <div class="flex items-start gap-3 px-4 pt-4 pb-3">
                  <span
                    class="size-9 rounded-[9px] flex items-center justify-center flex-none"
                    :class="[tierMeta[selected.tier].softBg, tierMeta[selected.tier].softText]"
                  >
                    <UIcon
                      :name="tierMeta[selected.tier].icon"
                      class="size-[17px]"
                    />
                  </span>
                  <div class="flex-1 min-w-0">
                    <div class="text-[15px] font-bold leading-snug">
                      {{ selected.name }}
                    </div>
                    <div class="flex items-center gap-2 mt-1">
                      <span
                        class="px-2 py-px text-[10.5px] font-semibold rounded-full"
                        :class="[tierMeta[selected.tier].softBg, tierMeta[selected.tier].softText]"
                      >
                        {{ $t(tierMeta[selected.tier].labelKey) }}
                      </span>
                      <span class="font-mono text-[11px] text-dimmed">{{ selected.code }}</span>
                    </div>
                  </div>
                  <button
                    class="p-1.5 text-dimmed hover:bg-muted hover:text-default rounded-md transition-colors flex-none"
                    @click="closeDetail"
                  >
                    <UIcon
                      name="i-lucide-x"
                      class="size-4"
                    />
                  </button>
                </div>

                <!-- Body: address + city/prov + asset count -->
                <div class="px-4 pb-3 flex flex-col gap-2.5">
                  <div class="flex items-start gap-2">
                    <UIcon
                      name="i-lucide-map-pin"
                      class="size-3.5 text-dimmed flex-none mt-0.5"
                    />
                    <div class="text-[12.5px] leading-relaxed text-muted">
                      {{ selected.address }}
                    </div>
                  </div>
                  <div class="flex items-center gap-2">
                    <UIcon
                      name="i-lucide-building-2"
                      class="size-3.5 text-dimmed flex-none"
                    />
                    <div class="text-[12.5px] font-medium">
                      {{ selected.city_name }}{{ selected.province_name ? ', ' + selected.province_name : '' }}
                    </div>
                  </div>
                  <div class="flex items-center gap-2 px-3 py-2.5 rounded-[9px] bg-muted">
                    <UIcon
                      name="i-lucide-package"
                      class="size-[15px] text-primary flex-none"
                    />
                    <span class="text-[13px] font-semibold">{{ selected.asset_count }}</span>
                    <span class="text-[12px] text-muted">{{ $t('map.registeredAssets') }}</span>
                  </div>
                </div>

                <!-- Action buttons -->
                <div class="flex gap-2 px-4 pb-4 pt-3 border-t border-default">
                  <NuxtLink
                    to="/master/offices"
                    class="flex-1 inline-flex items-center justify-center gap-1.5 px-2 py-2 text-[12.5px] font-semibold text-primary-foreground bg-primary border border-primary rounded-[9px] hover:bg-primary/90 transition-colors"
                  >
                    <UIcon
                      name="i-lucide-building"
                      class="size-3.5"
                    />
                    {{ $t('map.viewOffice') }}
                  </NuxtLink>
                  <template v-if="selected.latitude != null && selected.longitude != null">
                    <a
                      :href="googleMapsUrl(selected.latitude, selected.longitude)"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="flex-1 inline-flex items-center justify-center gap-1.5 px-2 py-2 text-[12.5px] font-medium text-default bg-default border border-strong rounded-[9px] hover:bg-muted transition-colors"
                    >
                      <UIcon
                        name="i-lucide-map-pin"
                        class="size-3.5"
                      />
                      {{ $t('map.openMaps') }}
                    </a>
                  </template>
                </div>
              </div>
            </Transition>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.slide-up-enter-active,
.slide-up-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.slide-up-enter-from,
.slide-up-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
