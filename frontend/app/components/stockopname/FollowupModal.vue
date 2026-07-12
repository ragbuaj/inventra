<script setup lang="ts">
import type { Floor, Room } from '~/types'
import type { OpnameItem } from '~/composables/api/useStockOpname'

// Nuxt UI's <SelectItem> forbids an empty-string value (reserved to mean
// "clear selection"), so the "no room selected" option uses this sentinel
// instead and is translated back to null at the API boundary.
const NONE = '__none__'

const props = defineProps<{
  open: boolean
  item: OpnameItem | null
  submitting: boolean
}>()

const emit = defineEmits<{
  'update:open': [boolean]
  'confirm': [{ toOfficeId: string, toRoomId: string | null, reason: string }]
}>()

const floorsApi = useFloors()
const office = useOfficePicker()
const { t } = useI18n()

const officeId = ref('')
const roomId = ref(NONE)
const reason = ref('')

const destFloors = ref<Floor[]>([])
const destRoomsByFloor = ref<Record<string, Room[]>>({})

function flattenRoomOptions(floors: Floor[], roomsByFloor: Record<string, Room[]>): Array<{ value: string, label: string }> {
  const opts: Array<{ value: string, label: string }> = []
  for (const f of floors) {
    for (const r of (roomsByFloor[f.id] ?? [])) {
      opts.push({ value: r.id, label: `${f.name} · ${r.name}` })
    }
  }
  return opts
}

const roomItems = computed(() => [{ value: NONE, label: t('transfer.form.roomNone') }, ...flattenRoomOptions(destFloors.value, destRoomsByFloor.value)])

watch(() => props.open, (isOpen) => {
  if (isOpen) {
    officeId.value = ''
    roomId.value = NONE
    reason.value = ''
    destFloors.value = []
    destRoomsByFloor.value = {}
  }
})

watch(officeId, async (id) => {
  roomId.value = NONE
  destFloors.value = []
  destRoomsByFloor.value = {}
  if (!id) return
  try {
    const floors = await floorsApi.listByOffice(id)
    destFloors.value = floors
    const entries = await Promise.all(floors.map(async f => [f.id, await floorsApi.roomsByFloor(f.id)] as const))
    const map: Record<string, Room[]> = {}
    for (const [fid, rooms] of entries) map[fid] = rooms
    destRoomsByFloor.value = map
  } catch {
    // Best-effort — the destination room stays "not set" if this fails.
  }
})

const ready = computed(() => !!officeId.value)

function close() {
  emit('update:open', false)
}

function confirm() {
  if (!ready.value || props.submitting) return
  emit('confirm', {
    toOfficeId: officeId.value,
    toRoomId: roomId.value === NONE ? null : roomId.value,
    reason: reason.value.trim()
  })
}
</script>

<template>
  <UModal
    :open="open"
    :title="t('stockOpname.followup.title')"
    :description="item ? t('stockOpname.followup.sub', { name: item.asset_name ?? '—' }) : ''"
    @update:open="(v) => emit('update:open', v)"
  >
    <template #body>
      <div class="space-y-4">
        <UFormField
          :label="t('stockOpname.followup.office')"
          required
        >
          <AsyncSearchPicker
            :model-value="officeId || null"
            :search-fn="office.searchFn"
            :resolve-fn="office.resolveFn"
            :placeholder="t('common.searchOffice')"
            testid="office"
            @update:model-value="officeId = $event ?? ''"
          />
        </UFormField>
        <UFormField :label="t('stockOpname.followup.room')">
          <USelect
            v-model="roomId"
            data-testid="opname-followup-room"
            value-key="value"
            :items="roomItems"
            class="w-full"
          />
        </UFormField>
        <UFormField :label="t('stockOpname.followup.reason')">
          <UTextarea
            v-model="reason"
            data-testid="opname-followup-reason"
            :rows="3"
            :placeholder="t('stockOpname.followup.reasonPlaceholder')"
            class="w-full"
          />
        </UFormField>
      </div>
    </template>
    <template #footer>
      <div class="flex justify-end gap-2 w-full">
        <UButton
          color="neutral"
          variant="ghost"
          @click="close"
        >
          {{ t('stockOpname.create.cancel') }}
        </UButton>
        <UButton
          :loading="submitting"
          :disabled="!ready"
          data-testid="opname-followup-confirm"
          @click="confirm"
        >
          {{ t('stockOpname.followup.submit') }}
        </UButton>
      </div>
    </template>
  </UModal>
</template>
