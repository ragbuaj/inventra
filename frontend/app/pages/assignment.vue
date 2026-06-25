<script setup lang="ts">
import type { Assignment, AssetCondition } from '~/mock/assignment'
import { useAssignment } from '~/composables/api/useAssignment'
import { recipientSeed, CONDITION_KEYS } from '~/mock/assignment'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
const ALL = '__all__'
const CONDITION_TEXT: Record<AssetCondition, string> = {
  baik: 'text-success',
  ringan: 'text-warning',
  berat: 'text-error'
}

const { t } = useI18n()
const api = useAssignment()

const tab = ref<'checkout' | 'checkin' | 'history'>('checkout')
const assignments = ref<Assignment[]>([])
const availableAssets = ref<{ label: string, value: string, nama: string }[]>([])
const loading = ref(true)

// check-out form
const coTag = ref('')
const coPegawai = ref('')
const coTgl = ref('')
const coKondisi = ref<AssetCondition>('baik')
const coCatatan = ref('')
const coMsg = ref<{ text: string, type: 'ok' | 'error' } | null>(null)

// check-in form
const ciId = ref('')
const ciTgl = ref('')
const ciKondisi = ref<AssetCondition>('baik')
const ciMaint = ref(false)
const ciMsg = ref<{ text: string, type: 'ok' | 'error' } | null>(null)

// history filters
const hq = ref('')
const hStatus = ref(ALL)

let coTimer: ReturnType<typeof setTimeout> | undefined
let ciTimer: ReturnType<typeof setTimeout> | undefined
onBeforeUnmount(() => {
  if (coTimer) clearTimeout(coTimer)
  if (ciTimer) clearTimeout(ciTimer)
})

function formatDate(d: string): string {
  if (!d) return ''
  const [y, m, day] = d.split('-')
  return `${Number(day)} ${MONTHS[Number(m) - 1] ?? m} ${y}`
}

const conditionItems = computed(() => CONDITION_KEYS.map(k => ({ value: k, label: t(`assignment.condition.${k}`) })))
const recipientItems = computed(() => recipientSeed.map(r => ({ value: r.name, label: r.name })))

const activeAssignments = computed(() => assignments.value.filter(a => a.status === 'active'))
const activeCount = computed(() => activeAssignments.value.length)
const activeItems = computed(() => activeAssignments.value.map(a => ({ value: a.id, label: `${a.tag} · ${a.nama} — ${a.pemegang}` })))

const ciSelected = computed(() => activeAssignments.value.find(a => a.id === ciId.value))
const ciInfo = computed(() => ciSelected.value ? t('assignment.checkin.info', { holder: ciSelected.value.pemegang, date: formatDate(ciSelected.value.pinjam) }) : '')

const coReady = computed(() => !!(coTag.value && coPegawai.value && coTgl.value))
const ciReady = computed(() => !!(ciId.value && ciTgl.value))

const tabs = computed(() => [
  { key: 'checkout' as const, label: t('assignment.tabs.checkout'), icon: 'i-lucide-circle-check-big' },
  { key: 'checkin' as const, label: t('assignment.tabs.checkin'), icon: 'i-lucide-square-check-big', badge: activeCount.value },
  { key: 'history' as const, label: t('assignment.tabs.history'), icon: 'i-lucide-history' }
])

const statusFilterItems = computed(() => [
  { value: ALL, label: t('assignment.history.allStatus') },
  { value: 'active', label: t('assignment.status.active') },
  { value: 'returned', label: t('assignment.status.returned') }
])

const histRows = computed(() => {
  const q = hq.value.trim().toLowerCase()
  return assignments.value.filter((a) => {
    if (hStatus.value !== ALL && a.status !== hStatus.value) return false
    if (q && !a.nama.toLowerCase().includes(q) && !a.tag.toLowerCase().includes(q) && !a.pemegang.toLowerCase().includes(q)) return false
    return true
  })
})

async function refresh() {
  assignments.value = await api.list()
  availableAssets.value = (await api.available()).map(a => ({ label: `${a.nama} · ${a.tag}`, value: a.tag, nama: a.nama }))
}

function resetCheckout() {
  coTag.value = ''
  coPegawai.value = ''
  coTgl.value = ''
  coKondisi.value = 'baik'
  coCatatan.value = ''
}

async function doCheckout() {
  if (!coReady.value) {
    coMsg.value = { text: t('assignment.checkout.errIncomplete'), type: 'error' }
    return
  }
  const asset = availableAssets.value.find(a => a.value === coTag.value)
  const recipient = recipientSeed.find(r => r.name === coPegawai.value)
  if (!asset || !recipient) return
  await api.checkout({ tag: asset.value, nama: asset.nama, pemegang: recipient.name, ini: recipient.ini, pinjam: coTgl.value, kondisi: coKondisi.value })
  coMsg.value = { text: t('assignment.checkout.ok', { name: asset.nama, holder: recipient.name }), type: 'ok' }
  resetCheckout()
  await refresh()
  if (coTimer) clearTimeout(coTimer)
  coTimer = setTimeout(() => {
    coMsg.value = null
  }, 4000)
}

async function doCheckin() {
  if (!ciReady.value) {
    ciMsg.value = { text: t('assignment.checkin.errIncomplete'), type: 'error' }
    return
  }
  const sel = ciSelected.value
  const name = sel?.nama ?? ''
  const kondisi: AssetCondition = ciMaint.value ? 'ringan' : ciKondisi.value
  await api.checkin(ciId.value, { kembali: ciTgl.value, kondisi })
  ciMsg.value = { text: ciMaint.value ? t('assignment.checkin.okMaint', { name }) : t('assignment.checkin.ok', { name }), type: 'ok' }
  ciId.value = ''
  ciTgl.value = ''
  ciKondisi.value = 'baik'
  ciMaint.value = false
  await refresh()
  if (ciTimer) clearTimeout(ciTimer)
  ciTimer = setTimeout(() => {
    ciMsg.value = null
  }, 4000)
}

function bannerClass(type: 'ok' | 'error'): string {
  return type === 'ok'
    ? 'bg-success/10 border-success/30 text-success'
    : 'bg-error/10 border-error/30 text-error'
}

onMounted(async () => {
  loading.value = true
  await refresh()
  loading.value = false
})
</script>

<template>
  <div class="max-w-[960px] mx-auto">
    <!-- Header -->
    <div class="mb-[18px]">
      <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
        {{ t('assignment.title') }}
      </h1>
      <p class="text-sm text-muted">
        {{ t('assignment.subtitle') }}
      </p>
    </div>

    <!-- Tabs -->
    <div class="flex gap-1 border-b border-default mb-[22px]">
      <button
        v-for="tb in tabs"
        :key="tb.key"
        class="inline-flex items-center gap-2 px-4 py-3 -mb-px text-sm border-b-2 transition-colors"
        :class="tab === tb.key ? 'font-semibold text-default border-primary' : 'font-medium text-muted border-transparent hover:text-default'"
        @click="tab = tb.key"
      >
        <UIcon
          :name="tb.icon"
          class="size-[15px]"
        />
        {{ tb.label }}
        <span
          v-if="tb.badge"
          class="min-w-[19px] h-[19px] px-1.5 inline-flex items-center justify-center text-[11px] font-bold rounded-full"
          :class="tab === tb.key ? 'bg-primary/15 text-primary' : 'bg-muted text-muted'"
        >{{ tb.badge }}</span>
      </button>
    </div>

    <!-- CHECK-OUT -->
    <div
      v-if="tab === 'checkout'"
      class="max-w-[600px]"
    >
      <div
        v-if="coMsg"
        class="flex gap-2.5 items-center px-3.5 py-3 mb-[18px] rounded-[11px] border text-[13px] font-medium"
        :class="bannerClass(coMsg.type)"
      >
        <UIcon
          :name="coMsg.type === 'ok' ? 'i-lucide-circle-check' : 'i-lucide-circle-alert'"
          class="size-[17px] flex-none"
        />
        {{ coMsg.text }}
      </div>

      <div class="bg-default border border-default rounded-[14px] shadow-sm p-[22px] flex flex-col gap-4">
        <UFormField
          :label="t('assignment.checkout.asset')"
          :hint="t('assignment.checkout.assetHint')"
          required
        >
          <USelectMenu
            v-model="coTag"
            value-key="value"
            :items="availableAssets"
            icon="i-lucide-search"
            :placeholder="t('assignment.checkout.assetPlaceholder')"
            :search-input="{ placeholder: t('assignment.checkout.assetPlaceholder') }"
            class="w-full"
          />
        </UFormField>

        <div class="grid grid-cols-2 gap-4">
          <UFormField
            :label="t('assignment.checkout.recipient')"
            required
          >
            <USelect
              v-model="coPegawai"
              value-key="value"
              :items="recipientItems"
              :placeholder="t('assignment.checkout.recipientPlaceholder')"
              class="w-full"
            />
          </UFormField>
          <UFormField
            :label="t('assignment.checkout.borrowDate')"
            required
          >
            <UInput
              v-model="coTgl"
              type="date"
              class="w-full"
            />
          </UFormField>
        </div>

        <UFormField :label="t('assignment.checkout.condOut')">
          <USelect
            v-model="coKondisi"
            value-key="value"
            :items="conditionItems"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('assignment.checkout.note')">
          <UTextarea
            v-model="coCatatan"
            :rows="3"
            :placeholder="t('assignment.checkout.notePlaceholder')"
            class="w-full"
          />
        </UFormField>

        <div class="flex justify-end gap-2.5 border-t border-default pt-4">
          <UButton
            color="neutral"
            variant="outline"
            :label="t('assignment.reset')"
            @click="resetCheckout"
          />
          <UButton
            icon="i-lucide-circle-check-big"
            :label="t('assignment.checkout.submit')"
            :disabled="!coReady"
            @click="doCheckout"
          />
        </div>
      </div>
    </div>

    <!-- CHECK-IN -->
    <div
      v-else-if="tab === 'checkin'"
      class="max-w-[600px]"
    >
      <div
        v-if="ciMsg"
        class="flex gap-2.5 items-center px-3.5 py-3 mb-[18px] rounded-[11px] border text-[13px] font-medium"
        :class="bannerClass(ciMsg.type)"
      >
        <UIcon
          :name="ciMsg.type === 'ok' ? 'i-lucide-circle-check' : 'i-lucide-circle-alert'"
          class="size-[17px] flex-none"
        />
        {{ ciMsg.text }}
      </div>

      <div
        v-if="activeCount > 0"
        class="bg-default border border-default rounded-[14px] shadow-sm p-[22px] flex flex-col gap-4"
      >
        <UFormField
          :label="t('assignment.checkin.active')"
          required
        >
          <USelect
            v-model="ciId"
            value-key="value"
            :items="activeItems"
            :placeholder="t('assignment.checkin.activePlaceholder')"
            class="w-full"
          />
        </UFormField>

        <div
          v-if="ciSelected"
          class="flex items-center gap-2.5 px-3.5 py-3 rounded-[11px] bg-muted"
        >
          <span class="size-[34px] rounded-[9px] bg-info/15 text-info flex items-center justify-center flex-none">
            <UIcon
              name="i-lucide-user"
              class="size-[17px]"
            />
          </span>
          <div class="text-[12.5px] text-muted">
            {{ ciInfo }}
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <UFormField
            :label="t('assignment.checkin.returnDate')"
            required
          >
            <UInput
              v-model="ciTgl"
              type="date"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('assignment.checkin.condIn')">
            <USelect
              v-model="ciKondisi"
              value-key="value"
              :items="conditionItems"
              class="w-full"
            />
          </UFormField>
        </div>

        <label
          class="flex items-start gap-2.5 px-3.5 py-3 rounded-[11px] border cursor-pointer transition-colors"
          :class="ciMaint ? 'bg-warning/10 border-warning/35' : 'bg-default border-default'"
        >
          <UCheckbox v-model="ciMaint" />
          <span>
            <span class="block text-[13.5px] font-semibold">{{ t('assignment.checkin.needsMaint') }}</span>
            <span class="block text-xs text-muted mt-0.5">{{ t('assignment.checkin.needsMaintNote') }}</span>
          </span>
        </label>

        <div class="flex justify-end gap-2.5 border-t border-default pt-4">
          <UButton
            icon="i-lucide-square-check-big"
            :label="t('assignment.checkin.submit')"
            :disabled="!ciReady"
            @click="doCheckin"
          />
        </div>
      </div>

      <div
        v-else
        class="bg-default border border-default rounded-[14px] shadow-sm py-[50px] px-6 text-center"
      >
        <div class="size-[54px] mx-auto mb-3.5 rounded-[14px] bg-muted text-dimmed flex items-center justify-center">
          <UIcon
            name="i-lucide-square-check-big"
            class="size-[26px]"
          />
        </div>
        <div class="text-base font-semibold mb-1.5">
          {{ t('assignment.checkin.emptyTitle') }}
        </div>
        <div class="text-sm text-muted">
          {{ t('assignment.checkin.emptySub') }}
        </div>
      </div>
    </div>

    <!-- HISTORY -->
    <div v-else>
      <div class="flex items-center gap-2.5 flex-wrap mb-3.5">
        <UInput
          v-model="hq"
          icon="i-lucide-search"
          :placeholder="t('assignment.history.searchPlaceholder')"
          class="flex-1 min-w-[220px]"
        />
        <USelect
          v-model="hStatus"
          value-key="value"
          :items="statusFilterItems"
          class="min-w-[160px]"
        />
      </div>

      <div
        v-if="loading"
        class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
      >
        <USkeleton class="h-[42px] w-full rounded-none" />
        <div
          v-for="n in 5"
          :key="n"
          class="flex items-center gap-4 px-4 py-3.5 border-t border-default"
        >
          <USkeleton class="h-3 w-[150px] rounded" />
          <USkeleton class="h-3 flex-1 rounded" />
          <USkeleton class="h-5 w-[90px] rounded-full" />
        </div>
      </div>

      <div
        v-else-if="histRows.length === 0"
        class="bg-default border border-default rounded-2xl shadow-sm py-[54px] px-6 text-center"
      >
        <div class="size-[54px] mx-auto mb-3.5 rounded-[14px] bg-muted text-dimmed flex items-center justify-center">
          <UIcon
            name="i-lucide-history"
            class="size-[26px]"
          />
        </div>
        <div class="text-base font-semibold mb-1.5">
          {{ t('assignment.history.emptyTitle') }}
        </div>
        <div class="text-sm text-muted">
          {{ t('assignment.history.emptySub') }}
        </div>
      </div>

      <div
        v-else
        class="bg-default border border-default rounded-[13px] shadow-sm overflow-hidden"
      >
        <div class="overflow-x-auto">
          <table class="w-full border-collapse text-[13.5px] whitespace-nowrap">
            <thead>
              <tr class="bg-muted text-muted">
                <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('assignment.history.colAsset') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('assignment.history.colHolder') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('assignment.history.colBorrow') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('assignment.history.colReturn') }}
                </th>
                <th class="text-left px-3.5 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('assignment.history.colStatus') }}
                </th>
                <th class="text-left px-4 py-[11px] text-xs font-semibold uppercase tracking-wide">
                  {{ t('assignment.history.colCondition') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="h in histRows"
                :key="h.id"
                class="border-t border-default hover:bg-muted transition-colors"
              >
                <td class="px-4 py-3">
                  <div class="font-medium">
                    {{ h.nama }}
                  </div>
                  <div class="font-mono text-[11.5px] text-dimmed">
                    {{ h.tag }}
                  </div>
                </td>
                <td class="px-3.5 py-3">
                  <div class="flex items-center gap-2">
                    <span class="size-[26px] rounded-full bg-muted text-muted flex items-center justify-center text-[10px] font-semibold flex-none">{{ h.ini }}</span>
                    <span>{{ h.pemegang }}</span>
                  </div>
                </td>
                <td class="px-3.5 py-3 text-muted">
                  {{ formatDate(h.pinjam) }}
                </td>
                <td
                  class="px-3.5 py-3"
                  :class="h.status === 'returned' ? 'text-muted' : 'text-dimmed'"
                >
                  {{ h.status === 'returned' ? formatDate(h.kembali) : '—' }}
                </td>
                <td class="px-3.5 py-3">
                  <UBadge
                    :color="h.status === 'returned' ? 'neutral' : 'info'"
                    variant="subtle"
                    class="rounded-full gap-1.5"
                  >
                    <span
                      class="size-1.5 rounded-full"
                      :class="h.status === 'returned' ? 'bg-[var(--ui-text-dimmed)]' : 'bg-info'"
                    />
                    {{ h.status === 'returned' ? t('assignment.status.returned') : t('assignment.status.active') }}
                  </UBadge>
                </td>
                <td class="px-4 py-3">
                  <span
                    class="text-[12.5px] font-semibold"
                    :class="CONDITION_TEXT[h.kondisi]"
                  >{{ t(`assignment.condition.${h.kondisi}`) }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="px-4 py-3 border-t border-default text-[13px] text-muted">
          {{ t('assignment.history.total', { n: histRows.length }) }}
        </div>
      </div>
    </div>
  </div>
</template>
