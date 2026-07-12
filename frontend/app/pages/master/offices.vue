<script setup lang="ts">
import type { Floor, Office, OfficeTier, ReferenceRow, Room, TreeNode } from '~/types'
import type { OfficeInput } from '~/composables/api/useOffices'
import { tierMeta } from '~/constants/officeMapMeta'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const toast = useToast()
const localePath = useLocalePath()
const { open: confirm } = useConfirm()
const api = useOffices()
const floorsApi = useFloors()
const refApi = useReference()

// Offices (flat, server-scoped) → tree built client-side.
const offices = ref<Office[]>([])
const selectedId = ref<string>()
const search = ref('')
const loading = ref(true)
const loadFailed = ref(false)

// Floors & rooms state (loaded per selected office / floor).
const floors = ref<Floor[]>([])
const floorRooms = ref<Record<string, Room[]>>({})
const floorOpen = ref<Record<string, boolean>>({})

// Inline rename state.
const editingFloorId = ref<string>()
const editingRoomId = ref<string>()
const editingRoomFloorId = ref<string>()
const editingFloorName = ref('')
const editingRoomName = ref('')

// FK reference data (office-types carry a tier; cities carry province_id).
const officeTypeRows = ref<ReferenceRow[]>([])
const provinceRows = ref<ReferenceRow[]>([])
const cityRows = ref<ReferenceRow[]>([])

const officeTypeOptions = computed(() => officeTypeRows.value.map(r => ({ value: r.id, label: r.name })))
const provinceOptions = computed(() => provinceRows.value.map(r => ({ value: r.id, label: r.name })))
const officeTypeMap = computed(() => new Map(officeTypeRows.value.map(r => [r.id, r.name])))
const provinceMap = computed(() => new Map(provinceRows.value.map(r => [r.id, r.name])))
const cityMap = computed(() => new Map(cityRows.value.map(r => [r.id, r.name])))
const cityById = computed(() => new Map(cityRows.value.map(r => [r.id, r])))

function toTier(raw: unknown): OfficeTier {
  return raw === 'pusat' || raw === 'wilayah' ? raw : 'office'
}
const officeTypeTier = computed(() => new Map(officeTypeRows.value.map(r => [r.id, toTier(r.tier)])))

function officeTypeName(id: string | null): string {
  return id ? (officeTypeMap.value.get(id) ?? id) : '—'
}
function provinceName(id: string | null): string {
  return id ? (provinceMap.value.get(id) ?? id) : '—'
}
function cityName(id: string | null): string {
  return id ? (cityMap.value.get(id) ?? id) : '—'
}

// Form state
const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<{
  parent_id: string | null
  office_type_id: string
  province_id: string | null
  city_id: string | null
  name: string
  code: string
  address: string
  is_active: boolean
  latitude: number | null
  longitude: number | null
}>({
  parent_id: null,
  office_type_id: '',
  province_id: null,
  city_id: null,
  name: '',
  code: '',
  address: '',
  is_active: true,
  latitude: null,
  longitude: null
})

const NONE = '__none__'

// USelect bridges: null ↔ '__none__' sentinel.
const formParentId = computed({
  get: () => form.parent_id ?? NONE,
  set: (val: string) => { form.parent_id = val === NONE ? null : val }
})
const formProvinceId = computed({
  get: () => form.province_id ?? NONE,
  set: (val: string) => {
    form.province_id = val === NONE ? null : val
    if (form.city_id && cityById.value.get(form.city_id)?.province_id !== form.province_id) form.city_id = null
  }
})
const formCityId = computed({
  get: () => form.city_id ?? NONE,
  set: (val: string) => { form.city_id = val === NONE ? null : val }
})

// Coordinate inputs: string ↔ number|null (empty/invalid → null).
function toCoord(v: string): number | null {
  const n = v.trim() === '' ? null : Number(v)
  return n == null || Number.isNaN(n) ? null : n
}
const formLat = computed({
  get: () => form.latitude == null ? '' : String(form.latitude),
  set: (v: string) => { form.latitude = toCoord(v) }
})
const formLng = computed({
  get: () => form.longitude == null ? '' : String(form.longitude),
  set: (v: string) => { form.longitude = toCoord(v) }
})

const cityOptions = computed(() => {
  if (!form.province_id) return []
  return cityRows.value
    .filter(r => r.province_id === form.province_id)
    .map(r => ({ value: r.id, label: r.name }))
})
const provinceItems = computed(() => [{ value: NONE, label: t('masterdata.offices.selectPlaceholder') }, ...provinceOptions.value])
const cityItems = computed(() => [{ value: NONE, label: t('masterdata.offices.selectPlaceholder') }, ...cityOptions.value])

// Tree
const nodes = computed<TreeNode[]>(() => buildTree(offices.value))

function buildTree(list: Office[]): TreeNode[] {
  const byParent = new Map<string | null, Office[]>()
  for (const o of list) {
    const arr = byParent.get(o.parent_id) ?? []
    arr.push(o)
    byParent.set(o.parent_id, arr)
  }
  function build(parentId: string | null): TreeNode[] {
    return (byParent.get(parentId) ?? []).map((o) => {
      const children = build(o.id)
      const meta = tierMeta[officeTypeTier.value.get(o.office_type_id) ?? 'office']
      return {
        id: o.id,
        label: o.name,
        icon: meta.icon,
        iconBg: meta.softBg,
        iconColor: meta.softText,
        inactive: !o.is_active,
        childCount: children.length || undefined,
        children: children.length ? children : undefined
      }
    })
  }
  return build(null)
}

const parentOptions = computed(() => {
  function flatten(list: TreeNode[], depth = 0): Array<{ value: string, label: string }> {
    const result: Array<{ value: string, label: string }> = []
    for (const n of list) {
      if (n.id !== editingId.value) {
        result.push({ value: n.id, label: '— '.repeat(depth) + n.label })
        if (n.children) result.push(...flatten(n.children, depth + 1))
      }
    }
    return result
  }
  return [
    { value: NONE, label: t('masterdata.offices.noParentLabel') },
    ...flatten(nodes.value)
  ]
})

const filteredNodes = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return nodes.value
  function filterTree(list: TreeNode[]): TreeNode[] {
    const result: TreeNode[] = []
    for (const n of list) {
      if (n.label.toLowerCase().includes(q)) {
        result.push({ ...n, children: n.children ? filterTree(n.children) : undefined })
      } else if (n.children) {
        const children = filterTree(n.children)
        if (children.length) result.push({ ...n, children })
      }
    }
    return result
  }
  return filterTree(nodes.value)
})

const selected = computed(() => offices.value.find(o => o.id === selectedId.value))
const parentName = computed(() => {
  const p = selected.value?.parent_id
  return p ? offices.value.find(o => o.id === p)?.name : undefined
})
const selectedTier = computed<OfficeTier>(() => selected.value ? (officeTypeTier.value.get(selected.value.office_type_id) ?? 'office') : 'office')
const selectedMeta = computed(() => tierMeta[selectedTier.value])
const tierColor: Record<OfficeTier, string> = { pusat: 'primary', wilayah: 'info', office: 'warning' }

async function refresh() {
  loading.value = true
  loadFailed.value = false
  try {
    const res = await api.list({ limit: 100 })
    offices.value = res.data
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

async function loadFkData() {
  const [types, provinces, cities] = await Promise.all([
    refApi.list('office-types', { limit: 100 }),
    refApi.list('provinces', { limit: 100 }),
    refApi.list('cities', { limit: 100 })
  ])
  officeTypeRows.value = types.data
  provinceRows.value = provinces.data
  cityRows.value = cities.data
}

async function loadFloors(officeId: string) {
  const fs = (await floorsApi.listByOffice(officeId)).sort((a, b) => (a.level ?? 0) - (b.level ?? 0))
  floors.value = fs
  const entries = await Promise.all(fs.map(async f => [f.id, await floorsApi.roomsByFloor(f.id)] as const))
  const roomMap: Record<string, Room[]> = {}
  for (const [fid, rooms] of entries) {
    roomMap[fid] = rooms
    if (!(fid in floorOpen.value)) floorOpen.value[fid] = true
  }
  floorRooms.value = roomMap
}

async function onSelect(id: string) {
  selectedId.value = id
  await loadFloors(id)
}

function openCreate() {
  editingId.value = undefined
  Object.assign(form, {
    parent_id: selectedId.value ?? null,
    office_type_id: officeTypeOptions.value[0]?.value ?? '',
    province_id: null,
    city_id: null,
    name: '',
    code: '',
    address: '',
    is_active: true,
    latitude: null,
    longitude: null
  })
  formOpen.value = true
}

function openEdit() {
  if (!selected.value) return
  editingId.value = selected.value.id
  Object.assign(form, {
    parent_id: selected.value.parent_id,
    office_type_id: selected.value.office_type_id,
    province_id: selected.value.province_id,
    city_id: selected.value.city_id,
    name: selected.value.name,
    code: selected.value.code,
    address: selected.value.address ?? '',
    is_active: selected.value.is_active,
    latitude: selected.value.latitude,
    longitude: selected.value.longitude
  })
  formOpen.value = true
}

async function onSubmit() {
  if (!form.name.trim() || !form.code.trim() || !form.office_type_id) {
    toast.add({ title: t('masterdata.offices.required'), color: 'error' })
    return
  }
  saving.value = true
  try {
    const input: OfficeInput = {
      parent_id: form.parent_id,
      office_type_id: form.office_type_id,
      province_id: form.province_id,
      city_id: form.city_id,
      name: form.name,
      code: form.code,
      address: form.address || null,
      is_active: form.is_active,
      latitude: form.latitude,
      longitude: form.longitude
    }
    const saved = editingId.value
      ? await api.update(editingId.value, input)
      : await api.create(input)
    formOpen.value = false
    await refresh()
    selectedId.value = saved.id
    await loadFloors(saved.id)
  } catch { /* useApiClient surfaces the error toast */ } finally {
    saving.value = false
  }
}

async function onDelete() {
  if (!selected.value) return
  const ok = await confirm({ title: t('masterdata.offices.deleteTitle'), description: t('masterdata.offices.deleteBody', { nama: selected.value.name }) })
  if (!ok) return
  try {
    await api.remove(selected.value.id)
    selectedId.value = undefined
    floors.value = []
    floorRooms.value = {}
    await refresh()
  } catch { /* useApiClient surfaces the error toast */ }
}

async function addFloor() {
  if (!selectedId.value) return
  const nextNum = floors.value.length + 1
  try {
    const floor = await floorsApi.createFloor({ office_id: selectedId.value, name: `Lantai ${nextNum}`, level: nextNum })
    floorOpen.value[floor.id] = true
    await loadFloors(selectedId.value)
  } catch { /* useApiClient surfaces the error toast */ }
}

async function deleteFloor(floorId: string) {
  const nama = floors.value.find(f => f.id === floorId)?.name ?? ''
  const ok = await confirm({ title: t('masterdata.offices.deleteFloorConfirm', { nama }) })
  if (!ok) return
  try {
    await floorsApi.removeFloor(floorId)
    if (selectedId.value) await loadFloors(selectedId.value)
  } catch { /* useApiClient surfaces the error toast */ }
}

async function addRoom(floorId: string) {
  try {
    await floorsApi.createRoom({ floor_id: floorId, name: 'Ruang Baru' })
    floorOpen.value[floorId] = true
    if (selectedId.value) await loadFloors(selectedId.value)
  } catch { /* useApiClient surfaces the error toast */ }
}

async function deleteRoom(roomId: string) {
  const nama = Object.values(floorRooms.value).flat().find(r => r.id === roomId)?.name ?? ''
  const ok = await confirm({ title: t('masterdata.offices.deleteRoomConfirm', { nama }) })
  if (!ok) return
  try {
    await floorsApi.removeRoom(roomId)
    if (selectedId.value) await loadFloors(selectedId.value)
  } catch { /* useApiClient surfaces the error toast */ }
}

function startEditFloor(floor: Floor) {
  editingFloorId.value = floor.id
  editingFloorName.value = floor.name
}

async function commitEditFloor() {
  const id = editingFloorId.value
  if (!id || !selectedId.value) return
  const name = editingFloorName.value.trim()
  if (!name) {
    toast.add({ title: t('masterdata.offices.nameRequired'), color: 'error' })
    return
  }
  editingFloorId.value = undefined
  try {
    await floorsApi.updateFloor(id, { office_id: selectedId.value, name })
    await loadFloors(selectedId.value)
  } catch { /* useApiClient surfaces the error toast */ }
}

function cancelEditFloor() {
  editingFloorId.value = undefined
}

function startEditRoom(room: Room) {
  editingRoomId.value = room.id
  editingRoomFloorId.value = room.floor_id
  editingRoomName.value = room.name
}

async function commitEditRoom() {
  const id = editingRoomId.value
  const floorId = editingRoomFloorId.value
  if (!id || !floorId) return
  const name = editingRoomName.value.trim()
  if (!name) {
    toast.add({ title: t('masterdata.offices.nameRequired'), color: 'error' })
    return
  }
  editingRoomId.value = undefined
  try {
    await floorsApi.updateRoom(id, { floor_id: floorId, name })
    if (selectedId.value) await loadFloors(selectedId.value)
  } catch { /* useApiClient surfaces the error toast */ }
}

function cancelEditRoom() {
  editingRoomId.value = undefined
}

function toggleFloor(floorId: string) {
  floorOpen.value[floorId] = !floorOpen.value[floorId]
}

onMounted(() => {
  refresh()
  loadFkData()
})
</script>

<template>
  <!-- Full-bleed split-panel: break out of layout's px-8 py-7 padding -->
  <div class="-mx-8 -my-7 flex h-[calc(100vh-61px)] overflow-hidden">
    <!-- LEFT: Tree panel (340px) -->
    <div class="w-[340px] flex-none flex flex-col overflow-hidden border-e border-default bg-default">
      <!-- Tree panel header -->
      <div class="flex-none px-4 pt-4 pb-3 border-b border-default">
        <div class="flex items-center justify-between mb-2.5">
          <span class="font-bold text-[15px]">{{ t('masterdata.offices.hierarki') }}</span>
          <div class="flex items-center gap-2">
            <Can permission="masterdata.office.manage">
              <UButton
                size="sm"
                icon="i-lucide-upload"
                color="neutral"
                variant="outline"
                :to="localePath('/master/import?target=office')"
              >
                {{ t('common.import') }}
              </UButton>
            </Can>
            <UButton
              size="sm"
              icon="i-lucide-plus"
              @click="openCreate"
            >
              {{ t('masterdata.offices.tambahKantor') }}
            </UButton>
          </div>
        </div>
        <UInput
          v-model="search"
          :placeholder="t('masterdata.offices.cariKantor')"
          icon="i-lucide-search"
          size="sm"
          class="w-full"
        />
      </div>
      <!-- Tree body -->
      <div class="flex-1 overflow-y-auto p-2.5">
        <div
          v-if="loading"
          class="p-4 text-center text-muted text-sm"
        >
          {{ t('common.loading') }}
        </div>
        <div
          v-else-if="loadFailed"
          class="px-4 py-10 text-center"
        >
          <p class="text-[13px] text-muted mb-3">
            {{ t('masterdata.offices.loadError') }}
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
        <div
          v-else-if="filteredNodes.length === 0"
          class="px-4 py-10 text-center text-[13px] text-dimmed"
        >
          {{ t('masterdata.offices.treeEmpty') }}
        </div>
        <TreeView
          v-else
          :nodes="filteredNodes"
          :selected-id="selectedId"
          @select="onSelect"
        />
      </div>
    </div>

    <!-- RIGHT: Detail panel -->
    <div class="flex-1 overflow-y-auto bg-muted/30">
      <!-- Placeholder when nothing selected -->
      <div
        v-if="!selected"
        class="h-full flex flex-col items-center justify-center gap-2.5 px-10 text-center"
      >
        <div class="size-[58px] rounded-[15px] bg-muted text-dimmed flex items-center justify-center">
          <UIcon
            name="i-lucide-building-2"
            class="size-7"
          />
        </div>
        <div class="font-semibold text-base">
          {{ t('masterdata.offices.pilihKantor') }}
        </div>
        <div class="text-sm text-muted max-w-[280px]">
          {{ t('masterdata.offices.pilihKantorSub') }}
        </div>
      </div>

      <!-- Detail view -->
      <div
        v-else
        class="px-7 py-6 max-w-[760px]"
      >
        <!-- Detail header -->
        <div class="flex items-start justify-between gap-4 flex-wrap mb-[18px]">
          <div class="min-w-0">
            <!-- Type + status chips -->
            <div class="flex items-center gap-2.5 flex-wrap mb-1.5">
              <UBadge
                :color="(tierColor[selectedTier] as any)"
                variant="subtle"
                size="md"
                class="rounded-full"
              >
                <UIcon
                  :name="selectedMeta.icon"
                  class="size-3.5 me-1.5"
                />
                {{ officeTypeName(selected.office_type_id) }}
              </UBadge>
              <UBadge
                :color="selected.is_active ? 'success' : 'neutral'"
                variant="subtle"
                size="md"
                class="rounded-full"
              >
                <span
                  class="size-1.5 rounded-full me-1.5 inline-block"
                  :class="selected.is_active ? 'bg-success' : 'bg-muted'"
                />
                {{ selected.is_active ? t('masterdata.offices.aktif') : t('masterdata.offices.nonaktif') }}
              </UBadge>
            </div>
            <!-- Office name & code -->
            <h1 class="m-0 font-bold text-[22px] tracking-tight leading-tight mb-[3px]">
              {{ selected.name }}
            </h1>
            <div class="font-mono text-[13px] text-muted">
              {{ selected.code }}
            </div>
          </div>
          <!-- Action buttons -->
          <Can permission="masterdata.office.manage">
            <div class="flex gap-2 flex-none">
              <UButton
                color="neutral"
                variant="outline"
                icon="i-lucide-pencil"
                @click="openEdit"
              >
                {{ t('common.edit') }}
              </UButton>
              <UButton
                color="error"
                variant="outline"
                icon="i-lucide-trash-2"
                @click="onDelete"
              >
                {{ t('common.delete') }}
              </UButton>
            </div>
          </Can>
        </div>

        <!-- Info card -->
        <div class="bg-default border border-default rounded-[13px] shadow-xs p-[18px_20px] mb-[22px]">
          <div class="grid grid-cols-2 gap-x-7 gap-y-3.5">
            <div>
              <div class="text-[12px] text-muted mb-[3px]">
                {{ t('masterdata.offices.fields.tipe') }}
              </div>
              <div
                class="text-[14px] font-medium"
                data-testid="office-detail-type"
              >
                {{ officeTypeName(selected.office_type_id) }}
              </div>
            </div>
            <div>
              <div class="text-[12px] text-muted mb-[3px]">
                {{ t('masterdata.offices.induk') }}
              </div>
              <div class="text-[14px] font-medium">
                {{ parentName ?? t('masterdata.offices.noParent') }}
              </div>
            </div>
            <div>
              <div class="text-[12px] text-muted mb-[3px]">
                {{ t('masterdata.offices.fields.provinsi') }}
              </div>
              <div class="text-[14px] font-medium">
                {{ provinceName(selected.province_id) }}
              </div>
            </div>
            <div>
              <div class="text-[12px] text-muted mb-[3px]">
                {{ t('masterdata.offices.fields.kota') }}
              </div>
              <div class="text-[14px] font-medium">
                {{ cityName(selected.city_id) }}
              </div>
            </div>
            <div class="col-span-2">
              <div class="text-[12px] text-muted mb-[3px]">
                {{ t('masterdata.offices.fields.alamat') }}
              </div>
              <div class="text-[14px] font-medium">
                {{ selected.address || '—' }}
              </div>
            </div>
          </div>
        </div>

        <!-- Lantai & Ruangan section -->
        <div class="flex items-center justify-between gap-3 mb-3">
          <div class="font-semibold text-[15px]">
            {{ t('masterdata.offices.lantaiRuangan') }}
          </div>
          <UButton
            color="neutral"
            variant="outline"
            size="sm"
            icon="i-lucide-plus"
            @click="addFloor"
          >
            {{ t('masterdata.offices.tambahLantai') }}
          </UButton>
        </div>

        <!-- Floor cards -->
        <div
          v-if="floors.length > 0"
          class="flex flex-col gap-2.5"
        >
          <div
            v-for="floor in floors"
            :key="floor.id"
            class="bg-default border border-default rounded-[12px] shadow-xs overflow-hidden"
          >
            <!-- Floor row header -->
            <div
              class="flex items-center gap-2.5 px-[15px] py-3 cursor-pointer hover:bg-muted/50"
              @click="toggleFloor(floor.id)"
            >
              <UIcon
                name="i-lucide-chevron-right"
                class="size-[15px] text-dimmed transition-transform duration-150 flex-none"
                :class="floorOpen[floor.id] ? 'rotate-90' : ''"
              />
              <div class="size-[30px] rounded-[8px] bg-primary/10 text-primary flex items-center justify-center flex-none">
                <UIcon
                  name="i-lucide-layers"
                  class="size-4"
                />
              </div>
              <!-- Inline-editable floor name -->
              <template v-if="editingFloorId === floor.id">
                <input
                  v-model="editingFloorName"
                  class="flex-1 font-semibold text-[14px] bg-default border border-primary rounded-[6px] px-2 py-0.5 outline-none focus:ring-2 focus:ring-primary/30"
                  :aria-label="t('masterdata.floors.editName')"
                  @click.stop
                  @blur="commitEditFloor"
                  @keydown.enter.prevent="commitEditFloor"
                  @keydown.esc.prevent="cancelEditFloor"
                >
                <UButton
                  color="neutral"
                  variant="ghost"
                  size="xs"
                  icon="i-lucide-x"
                  :title="t('common.cancel')"
                  @mousedown.prevent.stop="cancelEditFloor"
                />
              </template>
              <template v-else>
                <span class="flex-1 font-semibold text-[14px]">{{ floor.name }}</span>
                <UButton
                  color="neutral"
                  variant="ghost"
                  size="xs"
                  icon="i-lucide-pencil"
                  :title="t('masterdata.floors.editName')"
                  @click.stop="startEditFloor(floor)"
                />
              </template>
              <span class="text-[12px] text-muted font-medium">
                {{ t('masterdata.offices.roomCount', { n: (floorRooms[floor.id] ?? []).length }) }}
              </span>
              <UButton
                color="neutral"
                variant="ghost"
                size="xs"
                icon="i-lucide-plus"
                :title="t('masterdata.offices.tambahRuangan')"
                @click.stop="addRoom(floor.id)"
              />
              <UButton
                color="error"
                variant="ghost"
                size="xs"
                icon="i-lucide-trash-2"
                :title="t('masterdata.floors.deleteConfirm')"
                @click.stop="deleteFloor(floor.id)"
              />
            </div>
            <!-- Floor rooms -->
            <div
              v-if="floorOpen[floor.id]"
              class="border-t border-default px-[15px] pb-2.5 pt-1.5 ps-[50px]"
            >
              <div
                v-for="room in (floorRooms[floor.id] ?? [])"
                :key="room.id"
                class="flex items-center gap-2.5 py-[9px] border-b border-default last:border-b-0"
              >
                <UIcon
                  name="i-lucide-door-open"
                  class="size-[15px] text-dimmed flex-none"
                />
                <!-- Inline-editable room name -->
                <template v-if="editingRoomId === room.id">
                  <input
                    v-model="editingRoomName"
                    class="flex-1 text-[13.5px] font-medium bg-default border border-primary rounded-[6px] px-2 py-0.5 outline-none focus:ring-2 focus:ring-primary/30"
                    :aria-label="t('masterdata.rooms.editName')"
                    @blur="commitEditRoom"
                    @keydown.enter.prevent="commitEditRoom"
                    @keydown.esc.prevent="cancelEditRoom"
                  >
                  <UButton
                    color="neutral"
                    variant="ghost"
                    size="xs"
                    icon="i-lucide-x"
                    :title="t('common.cancel')"
                    @mousedown.prevent.stop="cancelEditRoom"
                  />
                </template>
                <template v-else>
                  <span class="flex-1 text-[13.5px] font-medium">{{ room.name }}</span>
                  <UButton
                    color="neutral"
                    variant="ghost"
                    size="xs"
                    icon="i-lucide-pencil"
                    :title="t('masterdata.rooms.editName')"
                    @click.stop="startEditRoom(room)"
                  />
                </template>
                <span class="font-mono text-[11.5px] text-dimmed">{{ room.code ?? '—' }}</span>
                <UButton
                  color="error"
                  variant="ghost"
                  size="xs"
                  icon="i-lucide-x"
                  :title="t('masterdata.rooms.deleteConfirm')"
                  @click="deleteRoom(room.id)"
                />
              </div>
              <div
                v-if="(floorRooms[floor.id] ?? []).length === 0"
                class="py-3 text-[12.5px] text-dimmed"
              >
                {{ t('masterdata.offices.noRoomMsg') }}
              </div>
            </div>
          </div>
        </div>

        <!-- Empty floors state -->
        <div
          v-else
          class="border-[1.5px] border-dashed border-strong rounded-[13px] p-10 text-center"
        >
          <div class="size-[50px] mx-auto mb-3 rounded-[13px] bg-muted text-dimmed flex items-center justify-center">
            <UIcon
              name="i-lucide-layers"
              class="size-6"
            />
          </div>
          <div class="font-semibold text-[15px] mb-1.5">
            {{ t('masterdata.offices.emptyFloors') }}
          </div>
          <div class="text-[13px] text-muted leading-relaxed max-w-[300px] mx-auto mb-4">
            {{ t('masterdata.offices.emptyFloorsSub') }}
          </div>
          <UButton
            icon="i-lucide-plus"
            @click="addFloor"
          >
            {{ t('masterdata.offices.tambahLantai') }}
          </UButton>
        </div>
      </div>
    </div>

    <!-- Office form slideover -->
    <FormSlideover
      v-model:open="formOpen"
      :title="editingId ? t('masterdata.offices.editTitle') : t('masterdata.offices.createTitle')"
      :subtitle="editingId ? t('masterdata.offices.editSub') : t('masterdata.offices.addSub')"
      :loading="saving"
      @submit="onSubmit"
    >
      <div class="space-y-4">
        <!-- Row 1: Induk + Jenis -->
        <div class="grid grid-cols-2 gap-3.5">
          <UFormField :label="t('masterdata.offices.induk')">
            <USelect
              v-model="formParentId"
              :items="parentOptions"
              class="w-full"
            />
          </UFormField>
          <UFormField
            :label="t('masterdata.offices.fields.tipe')"
            required
          >
            <USelect
              v-model="form.office_type_id"
              :items="officeTypeOptions"
              :placeholder="t('masterdata.offices.selectPlaceholder')"
              data-testid="office-type-select"
              class="w-full"
            />
          </UFormField>
        </div>
        <!-- Nama -->
        <UFormField
          :label="t('masterdata.offices.fields.nama')"
          required
        >
          <UInput
            v-model="form.name"
            :placeholder="t('masterdata.offices.fields.nama')"
            class="w-full"
          />
        </UFormField>
        <!-- Row 2: Kode + Provinsi -->
        <div class="grid grid-cols-2 gap-3.5">
          <UFormField
            :label="t('masterdata.offices.fields.kode')"
            required
          >
            <UInput
              v-model="form.code"
              placeholder="mis. JKT01"
              class="w-full font-mono"
            />
          </UFormField>
          <UFormField :label="t('masterdata.offices.fields.provinsi')">
            <USelect
              v-model="formProvinceId"
              :items="provinceItems"
              data-testid="office-province-select"
              class="w-full"
            />
          </UFormField>
        </div>
        <!-- Kota -->
        <UFormField :label="t('masterdata.offices.fields.kota')">
          <USelect
            v-model="formCityId"
            :items="cityItems"
            :disabled="!form.province_id"
            data-testid="office-city-select"
            class="w-full"
          />
        </UFormField>
        <!-- Alamat full-width -->
        <UFormField :label="t('masterdata.offices.fields.alamat')">
          <UTextarea
            v-model="form.address"
            :placeholder="t('masterdata.offices.fields.alamat')"
            class="w-full"
          />
        </UFormField>
        <!-- Coordinates -->
        <div>
          <div class="grid grid-cols-2 gap-3.5">
            <UFormField :label="t('masterdata.offices.fields.latitude')">
              <UInput
                v-model="formLat"
                type="number"
                step="any"
                placeholder="-6.2000"
                class="w-full font-mono"
              />
            </UFormField>
            <UFormField :label="t('masterdata.offices.fields.longitude')">
              <UInput
                v-model="formLng"
                type="number"
                step="any"
                placeholder="106.8166"
                class="w-full font-mono"
              />
            </UFormField>
          </div>
          <p class="text-[12px] text-muted mt-1.5">
            {{ t('masterdata.offices.coordHint') }}
          </p>
        </div>
        <!-- Aktif toggle -->
        <div class="flex items-center justify-between gap-2.5 px-3 py-[11px] rounded-[11px] bg-muted/50">
          <div>
            <div class="font-semibold text-[13.5px]">
              {{ t('masterdata.offices.aktif') }}
            </div>
            <div class="text-[12px] text-muted">
              {{ t('masterdata.offices.aktifHint') }}
            </div>
          </div>
          <USwitch v-model="form.is_active" />
        </div>
      </div>
    </FormSlideover>
  </div>
</template>
