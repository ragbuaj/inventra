<script setup lang="ts">
import type { SearchGroup, SearchItem } from '~/types'

const { t } = useI18n()
const can = useCan()
const { isOpen, close, toggle, recent, pushRecent } = useCommandPalette()
const { search } = useGlobalSearch()

const query = ref('')
const loading = ref(false)
const groups = ref<SearchGroup[]>([])
const sel = ref(0)
let seq = 0

const quickActions = computed(() => [
  { key: 'a', labelKey: 'search.qAddAsset', icon: 'i-lucide-plus', to: '/assets/new', perm: 'masterdata.office.manage' },
  { key: 'l', labelKey: 'search.qOpenReports', icon: 'i-lucide-bar-chart-2', to: '/reports', perm: '' },
  { key: 'p', labelKey: 'search.qCreateRequest', icon: 'i-lucide-send', to: '/approval', perm: '' }
].filter(a => !a.perm || can(a.perm)))

const hasQuery = computed(() => query.value.trim().length > 0)
const showInitial = computed(() => isOpen.value && !hasQuery.value)
const showLoading = computed(() => isOpen.value && hasQuery.value && loading.value)
const showResults = computed(() => isOpen.value && hasQuery.value && !loading.value && groups.value.length > 0)
const showEmpty = computed(() => isOpen.value && hasQuery.value && !loading.value && groups.value.length === 0)

// flat list of items for keyboard navigation, in group order
const flat = computed<SearchItem[]>(() => groups.value.flatMap(g => g.items))

watch(query, async (q) => {
  sel.value = 0
  if (!q.trim()) {
    groups.value = []
    loading.value = false
    return
  }
  loading.value = true
  const mine = ++seq
  const res = await search(q)
  if (mine === seq) {
    groups.value = res
    loading.value = false
  }
})

watch(isOpen, (v) => {
  if (!v) {
    query.value = ''
    groups.value = []
    sel.value = 0
  }
})

function go(item: SearchItem) {
  pushRecent(item.title)
  close()
  navigateTo(item.to)
}

function runQuick(to: string) {
  close()
  navigateTo(to)
}

function useRecent(term: string) {
  query.value = term
}

function onKey(e: KeyboardEvent) {
  if (!isOpen.value) return
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
  } else if (e.key === 'ArrowDown') {
    e.preventDefault()
    if (flat.value.length) sel.value = (sel.value + 1) % flat.value.length
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    if (flat.value.length) sel.value = (sel.value - 1 + flat.value.length) % flat.value.length
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const it = flat.value[sel.value]
    if (it) go(it)
  }
}

function onGlobalKey(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault()
    toggle()
  }
}

onMounted(() => window.addEventListener('keydown', onGlobalKey))
onUnmounted(() => window.removeEventListener('keydown', onGlobalKey))

// index helper so the template can compare a group/item against the flat selection
function flatIndex(gi: number, ii: number): number {
  let n = 0
  for (let i = 0; i < gi; i++) n += groups.value[i]!.items.length
  return n + ii
}

function parts(title: string): { pre: string, mark: string, post: string } {
  const q = query.value.trim()
  if (!q) return { pre: title, mark: '', post: '' }
  const i = title.toLowerCase().indexOf(q.toLowerCase())
  if (i < 0) return { pre: title, mark: '', post: '' }
  return { pre: title.slice(0, i), mark: title.slice(i, i + q.length), post: title.slice(i + q.length) }
}

// Icon color map for quick actions
const quickIconBg = ['bg-primary/10', 'bg-info/10', 'bg-neutral/10']
const quickIconFg = ['text-primary', 'text-info', 'text-neutral']

// Group icon / colors
const groupIconMap: Record<string, { bg: string, fg: string }> = {
  aset: { bg: 'bg-primary/10', fg: 'text-primary' },
  pegawai: { bg: 'bg-info/10', fg: 'text-info' },
  kantor: { bg: 'bg-warning/10', fg: 'text-warning' },
  user: { bg: 'bg-neutral/10', fg: 'text-neutral' },
  pengajuan: { bg: 'bg-info/10', fg: 'text-info' }
}

function groupMeta(type: string) {
  return groupIconMap[type] ?? { bg: 'bg-primary/10', fg: 'text-primary' }
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="isOpen"
      class="fixed inset-0 z-60 flex justify-center items-start bg-[var(--ui-overlay-scrim)]"
      style="backdrop-filter: blur(3px); padding: 88px 20px 20px;"
      @click="close()"
    >
      <!-- Panel -->
      <div
        class="w-full max-w-[640px] flex flex-col bg-default border border-default rounded-2xl shadow-xl overflow-hidden"
        style="max-height: calc(100vh - 130px);"
        @click.stop
      >
        <!-- Input row -->
        <div class="flex-none flex items-center gap-3 px-[18px] py-4 border-b border-default">
          <span class="text-dimmed flex flex-none">
            <UIcon
              name="i-lucide-search"
              class="size-5"
            />
          </span>
          <input
            v-model="query"
            :placeholder="t('search.placeholder')"
            autofocus
            class="flex-1 min-w-0 text-[16px] text-default bg-transparent border-none outline-none"
            @keydown="onKey"
          >
          <button
            v-if="hasQuery"
            :title="t('search.clearButton')"
            class="flex p-1 text-dimmed bg-transparent border-none rounded-[6px] cursor-pointer flex-none hover:bg-muted hover:text-default"
            @click="query = ''"
          >
            <UIcon
              name="i-lucide-x"
              class="size-4"
            />
          </button>
          <button
            class="inline-flex items-center px-[9px] py-1 font-mono text-[11px] font-semibold text-muted bg-default border border-default rounded-[6px] cursor-pointer flex-none hover:text-default"
            @click="close()"
          >
            Esc
          </button>
        </div>

        <!-- Body -->
        <div class="flex-1 overflow-y-auto p-2">
          <!-- LOADING: 4 moving-gradient shimmer skeleton rows (matches mockup @keyframes shimmer) -->
          <div
            v-if="showLoading"
            class="px-2 py-1"
          >
            <div class="h-[10px] w-[70px] rounded-[5px] my-2 mx-1.5 mb-3 bg-muted" />
            <div
              v-for="n in 4"
              :key="n"
              class="flex items-center gap-3 px-2 py-[9px]"
            >
              <div class="shimmer size-[34px] rounded-[9px] flex-none" />
              <div class="flex-1 flex flex-col gap-1.5">
                <div class="shimmer h-[11px] w-[55%] rounded-[5px]" />
                <div class="shimmer h-[9px] w-[32%] rounded-[5px]" />
              </div>
            </div>
          </div>

          <!-- INITIAL STATE: Recent + Quick Actions -->
          <div v-if="showInitial">
            <!-- Recent Searches -->
            <div class="px-2.5 py-2.5 pb-1 text-[11px] font-semibold tracking-[.07em] uppercase text-dimmed">
              {{ t('search.recentTitle') }}
            </div>
            <button
              v-for="term in recent"
              :key="term"
              class="flex items-center gap-3 w-full px-2.5 py-[9px] rounded-[10px] bg-transparent border-none cursor-pointer text-left hover:bg-muted transition-colors"
              @click="useRecent(term)"
            >
              <span class="w-[30px] h-[30px] rounded-[8px] bg-muted text-muted flex items-center justify-center flex-none">
                <UIcon
                  name="i-lucide-clock"
                  class="size-[15px]"
                />
              </span>
              <span class="flex-1 text-[14px] font-medium text-default">{{ term }}</span>
              <span class="flex text-dimmed">
                <UIcon
                  name="i-lucide-arrow-up-right"
                  class="size-[15px]"
                />
              </span>
            </button>

            <!-- Quick Actions -->
            <div class="px-2.5 pt-3.5 pb-1 text-[11px] font-semibold tracking-[.07em] uppercase text-dimmed">
              {{ t('search.quickTitle') }}
            </div>
            <button
              v-for="(action, i) in quickActions"
              :key="action.key"
              class="flex items-center gap-3 w-full px-2.5 py-[9px] rounded-[10px] bg-transparent border-none cursor-pointer text-left hover:bg-muted transition-colors"
              @click="runQuick(action.to)"
            >
              <span
                class="w-[30px] h-[30px] rounded-[8px] flex items-center justify-center flex-none"
                :class="[quickIconBg[i % 3], quickIconFg[i % 3]]"
              >
                <UIcon
                  :name="action.icon"
                  class="size-[15px]"
                />
              </span>
              <span class="flex-1 text-[14px] font-medium text-default">{{ t(action.labelKey) }}</span>
              <span class="inline-flex items-center px-[7px] py-[2px] font-mono text-[10.5px] font-semibold text-muted bg-default border border-default rounded-[6px]">
                {{ action.key.toUpperCase() }}
              </span>
            </button>
          </div>

          <!-- RESULTS: grouped -->
          <div v-if="showResults">
            <div
              v-for="(g, gi) in groups"
              :key="g.type"
              class="mb-1"
            >
              <!-- Group header -->
              <div class="flex items-center justify-between px-2.5 py-2.5 pb-1">
                <span class="text-[11px] font-semibold tracking-[.07em] uppercase text-dimmed">
                  {{ t(g.labelKey) }}
                </span>
                <button class="text-[11px] font-semibold text-primary bg-transparent border-none cursor-pointer">
                  {{ t('search.seeAll', { n: g.total }) }}
                </button>
              </div>
              <!-- Result rows -->
              <button
                v-for="(it, ii) in g.items"
                :key="it.to + ii"
                class="flex items-center gap-3 w-full px-2.5 py-[9px] rounded-[10px] border-none cursor-pointer text-left transition-colors"
                :class="flatIndex(gi, ii) === sel ? 'bg-primary/10' : 'bg-transparent hover:bg-muted'"
                :style="flatIndex(gi, ii) === sel ? 'box-shadow: inset 3px 0 0 var(--ui-primary)' : ''"
                @click="go(it)"
                @mouseenter="sel = flatIndex(gi, ii)"
              >
                <!-- Icon chip -->
                <span
                  class="size-[34px] rounded-[9px] flex items-center justify-center flex-none"
                  :class="[groupMeta(g.type).bg, groupMeta(g.type).fg]"
                >
                  <UIcon
                    :name="it.icon"
                    class="size-4"
                  />
                </span>
                <!-- Content -->
                <div class="flex-1 min-w-0">
                  <div class="text-[14px] font-medium text-default whitespace-nowrap overflow-hidden text-ellipsis">
                    <span>{{ parts(it.title).pre }}</span><mark
                      v-if="parts(it.title).mark"
                      class="rounded-[3px] px-px"
                      style="background: var(--ui-color-warning-200); color: var(--ui-color-warning-800);"
                    >{{ parts(it.title).mark }}</mark><span>{{ parts(it.title).post }}</span>
                  </div>
                  <div class="flex items-center gap-[7px] mt-0.5">
                    <span class="font-mono text-[12px] text-dimmed">{{ it.sub }}</span>
                    <StatusBadge
                      v-if="it.status"
                      :status="it.status"
                    />
                  </div>
                </div>
                <!-- Enter chip when selected -->
                <span
                  v-if="flatIndex(gi, ii) === sel"
                  class="inline-flex items-center px-[7px] py-[2px] font-mono text-[10.5px] font-semibold text-primary bg-primary/10 rounded-[6px] flex-none"
                >
                  ↵
                </span>
              </button>
            </div>
          </div>

          <!-- NO RESULTS empty state -->
          <div
            v-if="showEmpty"
            class="px-6 py-[46px] text-center"
          >
            <div class="size-[50px] mx-auto mb-3 rounded-[13px] bg-muted text-dimmed flex items-center justify-center">
              <UIcon
                name="i-lucide-search-x"
                class="size-6"
              />
            </div>
            <div class="text-[15px] font-semibold mb-1">
              {{ t('search.emptyTitle', { q: query.trim() }) }}
            </div>
            <div class="text-[13px] text-muted">
              {{ t('search.emptySub') }}
            </div>
          </div>
        </div>

        <!-- Footer hints -->
        <div class="flex-none flex items-center gap-4 px-4 py-[11px] border-t border-default bg-muted">
          <span class="inline-flex items-center gap-1.5 text-[11.5px] text-muted">
            <span class="inline-flex gap-[2px]">
              <span class="px-[5px] py-[1px] font-mono text-[10px] font-semibold bg-default border border-default rounded-[5px]">↑</span>
              <span class="px-[5px] py-[1px] font-mono text-[10px] font-semibold bg-default border border-default rounded-[5px]">↓</span>
            </span>
            {{ t('search.navHint') }}
          </span>
          <span class="inline-flex items-center gap-1.5 text-[11.5px] text-muted">
            <span class="px-[6px] py-[1px] font-mono text-[10px] font-semibold bg-default border border-default rounded-[5px]">↵</span>
            {{ t('search.openHint') }}
          </span>
          <span class="inline-flex items-center gap-1.5 text-[11.5px] text-muted">
            <span class="px-[6px] py-[1px] font-mono text-[10px] font-semibold bg-default border border-default rounded-[5px]">Esc</span>
            {{ t('search.closeHint') }}
          </span>
          <div class="flex-1" />
          <span class="text-[11px] text-dimmed">{{ t('search.scopeNote') }}</span>
        </div>
      </div>
    </div>
  </Teleport>
</template>
