<script setup lang="ts">
import type { Assignment } from '~/composables/api/useAssignment'
import type { AssetCondition } from '~/constants/assignmentMeta'
import { useAssignment } from '~/composables/api/useAssignment'
import { useEmployees } from '~/composables/api/useEmployees'
import { ASSIGNMENT_STATUS_TONE, CONDITION_KEYS, formatDateID } from '~/constants/assignmentMeta'

definePageMeta({ middleware: 'can', permission: 'assignment.manage' })

const ALL = '__all__'
const CONDITION_TEXT: Record<AssetCondition, string> = {
  baik: 'text-success',
  ringan: 'text-warning',
  berat: 'text-error'
}

const { t } = useI18n()
const api = useAssignment()
const employeesApi = useEmployees()

const tab = ref<'checkout' | 'checkin' | 'history'>('checkout')
const assignments = ref<Assignment[]>([])
const availableAssets = ref<{ label: string, value: string }[]>([])
const employees = ref<{ label: string, value: string }[]>([])
const loading = ref(true)
const loadError = ref(false)

// check-out form
const coAssetId = ref('')
const coEmployeeId = ref('')
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

const conditionItems = computed(() => CONDITION_KEYS.map(k => ({ value: k, label: t(`assignment.condition.${k}`) })))

const activeAssignments = computed(() => assignments.value.filter(a => a.status === 'active'))
const activeCount = computed(() => activeAssignments.value.length)
const activeItems = computed(() => activeAssignments.value.map(a => ({ value: a.id, label: `${a.asset_tag ?? '—'} · ${a.asset_name ?? '—'} — ${a.employee_name ?? '—'}` })))

const ciSelected = computed(() => activeAssignments.value.find(a => a.id === ciId.value))
const ciInfo = computed(() => ciSelected.value ? t('assignment.checkin.info', { holder: ciSelected.value.employee_name ?? '—', date: formatDateID(ciSelected.value.checkout_date) }) : '')

const coReady = computed(() => !!(coAssetId.value && coEmployeeId.value && coTgl.value))
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
    if (q) {
      const hay = `${a.asset_name ?? ''} ${a.asset_tag ?? ''} ${a.employee_name ?? ''}`.toLowerCase()
      if (!hay.includes(q)) return false
    }
    return true
  })
})

async function loadEmployees() {
  const page = await employeesApi.list({ limit: 100 })
  employees.value = page.data.map(e => ({ value: e.id, label: e.name }))
}

async function loadAssignments() {
  const page = await api.list()
  assignments.value = page.data
  const avail = await api.available()
  availableAssets.value = avail.data.map(a => ({ label: `${a.name} · ${a.asset_tag}`, value: a.id }))
}

async function refresh() {
  loading.value = true
  loadError.value = false
  try {
    await Promise.all([loadEmployees(), loadAssignments()])
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

function resetCheckout() {
  coAssetId.value = ''
  coEmployeeId.value = ''
  coTgl.value = ''
  coKondisi.value = 'baik'
  coCatatan.value = ''
}

async function doCheckout() {
  if (!coReady.value) {
    coMsg.value = { text: t('assignment.checkout.errIncomplete'), type: 'error' }
    return
  }
  const asset = availableAssets.value.find(a => a.value === coAssetId.value)
  const employee = employees.value.find(e => e.value === coEmployeeId.value)
  if (!asset || !employee) return
  await api.checkout({
    asset_id: coAssetId.value,
    employee_id: coEmployeeId.value,
    checkout_date: coTgl.value,
    condition_out: coKondisi.value,
    notes: coCatatan.value.trim() || null
  })
  coMsg.value = { text: t('assignment.checkout.ok', { name: asset.label, holder: employee.label }), type: 'ok' }
  resetCheckout()
  await loadAssignments()
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
  const name = sel?.asset_name ?? ''
  await api.checkin(ciId.value, {
    checkin_date: ciTgl.value,
    condition_in: ciKondisi.value,
    needs_maintenance: ciMaint.value
  })
  ciMsg.value = { text: ciMaint.value ? t('assignment.checkin.okMaint', { name }) : t('assignment.checkin.ok', { name }), type: 'ok' }
  ciId.value = ''
  ciTgl.value = ''
  ciKondisi.value = 'baik'
  ciMaint.value = false
  await loadAssignments()
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

onMounted(refresh)
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

    <div
      v-if="loadError"
      class="bg-default border border-default rounded-[14px] shadow-sm py-[50px] px-6 text-center"
    >
      <p class="text-sm text-muted mb-3">
        {{ t('common.loadError') }}
      </p>
      <UButton
        size="sm"
        color="neutral"
        variant="outline"
        icon="i-lucide-rotate-cw"
        @click="refresh"
      >
        {{ t('common.retry') }}
      </UButton>
    </div>

    <template v-else>
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
              v-model="coAssetId"
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
                v-model="coEmployeeId"
                value-key="value"
                :items="employees"
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
                      {{ h.asset_name ?? '—' }}
                    </div>
                    <div class="font-mono text-[11.5px] text-dimmed">
                      {{ h.asset_tag ?? '—' }}
                    </div>
                  </td>
                  <td class="px-3.5 py-3">
                    <div class="flex items-center gap-2">
                      <span class="size-[26px] rounded-full bg-muted text-muted flex items-center justify-center text-[10px] font-semibold flex-none">{{ (h.employee_name ?? '—').split(/\s+/).filter(Boolean).map(w => w[0]).slice(0, 2).join('').toUpperCase() }}</span>
                      <span>{{ h.employee_name ?? '—' }}</span>
                    </div>
                  </td>
                  <td class="px-3.5 py-3 text-muted">
                    {{ formatDateID(h.checkout_date) }}
                  </td>
                  <td
                    class="px-3.5 py-3"
                    :class="h.status === 'returned' ? 'text-muted' : 'text-dimmed'"
                  >
                    {{ h.status === 'returned' ? formatDateID(h.checkin_date) : '—' }}
                  </td>
                  <td class="px-3.5 py-3">
                    <UBadge
                      :color="ASSIGNMENT_STATUS_TONE[h.status]"
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
                      v-if="h.condition_out"
                      class="text-[12.5px] font-semibold"
                      :class="CONDITION_TEXT[h.condition_out as AssetCondition]"
                    >{{ t(`assignment.condition.${h.condition_out}`) }}</span>
                    <span
                      v-else
                      class="text-[12.5px] text-dimmed"
                    >—</span>
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
    </template>
  </div>
</template>
