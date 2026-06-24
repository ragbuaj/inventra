<script setup lang="ts">
import type { Office, Floor, Room, TreeNode } from '~/types'
import type { OfficeInput } from '~/composables/api/useOffices'
import { officeTipeMeta } from '~/mock/offices'

definePageMeta({ middleware: 'can', permission: 'masterdata.office.manage' })

const { t } = useI18n()
const toast = useToast()
const { open: confirm } = useConfirm()
const api = useOffices()
const floorsApi = useFloors()

// Tree state
const nodes = ref<TreeNode[]>([])
const selectedId = ref<string>()
const selected = ref<Office>()
const parentName = ref<string>()
const search = ref('')
const loading = ref(true)

// Floors & rooms state
const floors = ref<Floor[]>([])
const floorRooms = ref<Record<string, Room[]>>({})
const floorOpen = ref<Record<string, boolean>>({})

// Inline rename state: tracks which floor/room name is being edited
const editingFloorId = ref<string>()
const editingRoomId = ref<string>()
const editingFloorName = ref('')
const editingRoomName = ref('')

// Form state
const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<OfficeInput & { active: boolean }>({
  nama: '',
  kode: '',
  tipe: 'cabang',
  parent_id: null,
  provinsi: '',
  kota: '',
  alamat: '',
  active: true
})

const PROVINSI = [
  'DKI Jakarta', 'Jawa Barat', 'Jawa Tengah', 'Jawa Timur',
  'Banten', 'Sumatera Utara', 'Sumatera Barat', 'Sulawesi Selatan',
  'Kalimantan Timur', 'Bali'
]

const tipeOptions = (['pusat', 'kanwil', 'cabang', 'unit'] as const).map(v => ({
  value: v,
  label: t(`masterdata.offices.tipe.${v}`)
}))

const parentOptions = computed(() => {
  function flatten(nodes: TreeNode[], depth = 0): Array<{ value: string, label: string }> {
    const result: Array<{ value: string, label: string }> = []
    for (const n of nodes) {
      if (n.id !== editingId.value) {
        result.push({ value: n.id, label: '— '.repeat(depth) + n.label })
        if (n.children) {
          result.push(...flatten(n.children, depth + 1))
        }
      }
    }
    return result
  }
  return [
    { value: '__none__', label: t('masterdata.offices.noParentLabel') },
    ...flatten(nodes.value)
  ]
})

// USelect expects string | undefined, not string | null; bridge via computed
const formParentId = computed({
  get: () => form.parent_id ?? '__none__',
  set: (val: string) => { form.parent_id = val === '__none__' ? null : val }
})

const filteredNodes = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return nodes.value
  function filterTree(nodes: TreeNode[]): TreeNode[] {
    const result: TreeNode[] = []
    for (const n of nodes) {
      if (n.label.toLowerCase().includes(q)) {
        result.push({ ...n, children: n.children ? filterTree(n.children) : undefined })
      } else if (n.children) {
        const children = filterTree(n.children)
        if (children.length) {
          result.push({ ...n, children })
        }
      }
    }
    return result
  }
  return filterTree(nodes.value)
})

async function refresh() {
  loading.value = true
  nodes.value = await api.tree()
  loading.value = false
}

async function onSelect(id: string) {
  selectedId.value = id
  const office = await api.get(id)
  selected.value = office
  if (office && office.parent_id) {
    const parent = await api.get(office.parent_id)
    parentName.value = parent?.nama
  } else {
    parentName.value = undefined
  }
  loadFloors(id)
}

function loadFloors(officeId: string) {
  floors.value = floorsApi.listByOffice(officeId)
  const roomMap: Record<string, Room[]> = {}
  for (const f of floors.value) {
    roomMap[f.id] = floorsApi.roomsByFloor(f.id)
    if (!(f.id in floorOpen.value)) {
      floorOpen.value[f.id] = true
    }
  }
  floorRooms.value = roomMap
}

function openCreate() {
  editingId.value = undefined
  Object.assign(form, {
    nama: '',
    kode: '',
    tipe: 'cabang',
    parent_id: selectedId.value ?? null,
    provinsi: '',
    kota: '',
    alamat: '',
    active: true
  })
  formOpen.value = true
}

function openEdit() {
  if (!selected.value) return
  editingId.value = selected.value.id
  Object.assign(form, {
    nama: selected.value.nama,
    kode: selected.value.kode,
    tipe: selected.value.tipe,
    parent_id: selected.value.parent_id,
    provinsi: selected.value.provinsi,
    kota: selected.value.kota,
    alamat: selected.value.alamat,
    active: selected.value.active
  })
  formOpen.value = true
}

async function onSubmit() {
  saving.value = true
  try {
    const saved = editingId.value
      ? await api.update(editingId.value, { ...form })
      : await api.create({ ...form })
    formOpen.value = false
    await refresh()
    await onSelect(saved.id)
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  } finally {
    saving.value = false
  }
}

async function onDelete() {
  if (!selected.value) return
  const ok = await confirm({ title: t('masterdata.offices.deleteTitle'), description: t('masterdata.offices.deleteBody') })
  if (!ok) return
  try {
    await api.remove(selected.value.id)
    selected.value = undefined
    selectedId.value = undefined
    parentName.value = undefined
    floors.value = []
    floorRooms.value = {}
    await refresh()
  } catch (err) {
    toast.add({ title: t((err as Error).message), color: 'error' })
  }
}

function addFloor() {
  if (!selectedId.value) return
  const nextNum = floors.value.length + 1
  const name = `Lantai ${nextNum}`
  const floor = floorsApi.createFloor(selectedId.value, name, nextNum)
  floorOpen.value[floor.id] = true
  loadFloors(selectedId.value)
}

async function deleteFloor(floorId: string) {
  const ok = await confirm({ title: t('masterdata.offices.deleteFloorConfirm') })
  if (!ok) return
  floorsApi.removeFloor(floorId)
  if (selectedId.value) loadFloors(selectedId.value)
}

function addRoom(floorId: string) {
  if (!selectedId.value) return
  const roomsOnFloor = floorRooms.value[floorId] ?? []
  const kode = `R-${floorId.slice(-4)}-${roomsOnFloor.length + 1}`
  floorsApi.createRoom(floorId, selectedId.value, 'Ruang Baru', kode)
  floorOpen.value[floorId] = true
  loadFloors(selectedId.value)
}

async function deleteRoom(roomId: string) {
  const ok = await confirm({ title: t('masterdata.offices.deleteRoomConfirm') })
  if (!ok) return
  floorsApi.removeRoom(roomId)
  if (selectedId.value) loadFloors(selectedId.value)
}

function startEditFloor(floor: Floor) {
  editingFloorId.value = floor.id
  editingFloorName.value = floor.nama
}

function commitEditFloor() {
  const id = editingFloorId.value
  if (!id) return
  const name = editingFloorName.value.trim()
  if (!name) {
    toast.add({ title: t('masterdata.offices.nameRequired'), color: 'error' })
    return
  }
  floorsApi.updateFloor(id, { nama: name })
  if (selectedId.value) loadFloors(selectedId.value)
  editingFloorId.value = undefined
}

function cancelEditFloor() {
  editingFloorId.value = undefined
}

function startEditRoom(room: Room) {
  editingRoomId.value = room.id
  editingRoomName.value = room.nama
}

function commitEditRoom() {
  const id = editingRoomId.value
  if (!id) return
  const name = editingRoomName.value.trim()
  if (!name) {
    toast.add({ title: t('masterdata.offices.nameRequired'), color: 'error' })
    return
  }
  floorsApi.updateRoom(id, { nama: name })
  if (selectedId.value) loadFloors(selectedId.value)
  editingRoomId.value = undefined
}

function cancelEditRoom() {
  editingRoomId.value = undefined
}

function toggleFloor(floorId: string) {
  floorOpen.value[floorId] = !floorOpen.value[floorId]
}

const selectedMeta = computed(() => {
  if (!selected.value) return null
  return officeTipeMeta[selected.value.tipe]
})

onMounted(refresh)
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
          <UButton
            size="sm"
            icon="i-lucide-plus"
            @click="openCreate"
          >
            {{ t('masterdata.offices.tambahKantor') }}
          </UButton>
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
                v-if="selectedMeta"
                :color="selectedMeta.color as any"
                variant="subtle"
                size="md"
                class="rounded-full"
              >
                <UIcon
                  :name="selectedMeta.icon"
                  class="size-3.5 me-1.5"
                />
                {{ t(`masterdata.offices.tipe.${selected.tipe}`) }}
              </UBadge>
              <UBadge
                :color="selected.active ? 'success' : 'neutral'"
                variant="subtle"
                size="md"
                class="rounded-full"
              >
                <span
                  class="size-1.5 rounded-full me-1.5 inline-block"
                  :class="selected.active ? 'bg-success' : 'bg-muted'"
                />
                {{ selected.active ? t('masterdata.offices.aktif') : t('masterdata.offices.nonaktif') }}
              </UBadge>
            </div>
            <!-- Office name & code -->
            <h1 class="m-0 font-bold text-[22px] tracking-tight leading-tight mb-[3px]">
              {{ selected.nama }}
            </h1>
            <div class="font-mono text-[13px] text-muted">
              {{ selected.kode }}
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
              <div class="text-[14px] font-medium">
                {{ t(`masterdata.offices.tipe.${selected.tipe}`) }}
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
                {{ selected.provinsi || '—' }}
              </div>
            </div>
            <div>
              <div class="text-[12px] text-muted mb-[3px]">
                {{ t('masterdata.offices.fields.kota') }}
              </div>
              <div class="text-[14px] font-medium">
                {{ selected.kota || '—' }}
              </div>
            </div>
            <div class="col-span-2">
              <div class="text-[12px] text-muted mb-[3px]">
                {{ t('masterdata.offices.fields.alamat') }}
              </div>
              <div class="text-[14px] font-medium">
                {{ selected.alamat || '—' }}
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
                <span class="flex-1 font-semibold text-[14px]">{{ floor.nama }}</span>
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
                {{ (floorRooms[floor.id] ?? []).length }} {{ t('masterdata.rooms.title').toLowerCase() }}
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
                  <span class="flex-1 text-[13.5px] font-medium">{{ room.nama }}</span>
                  <UButton
                    color="neutral"
                    variant="ghost"
                    size="xs"
                    icon="i-lucide-pencil"
                    :title="t('masterdata.rooms.editName')"
                    @click.stop="startEditRoom(room)"
                  />
                </template>
                <span class="font-mono text-[11.5px] text-dimmed">{{ room.kode }}</span>
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
          <UFormField :label="t('masterdata.offices.fields.tipe')">
            <USelect
              v-model="form.tipe"
              :items="tipeOptions"
              class="w-full"
            />
          </UFormField>
        </div>
        <!-- Row 2: Kode + Provinsi -->
        <div class="grid grid-cols-2 gap-3.5">
          <UFormField :label="t('masterdata.offices.fields.kode')">
            <UInput
              v-model="form.kode"
              placeholder="mis. JKT01"
              class="w-full font-mono"
            />
          </UFormField>
          <UFormField :label="t('masterdata.offices.fields.provinsi')">
            <USelect
              v-model="form.provinsi"
              :items="PROVINSI.map(p => ({ value: p, label: p }))"
              class="w-full"
            />
          </UFormField>
        </div>
        <!-- Row 3: Nama + Kota -->
        <div class="grid grid-cols-2 gap-3.5">
          <UFormField :label="t('masterdata.offices.fields.nama')">
            <UInput
              v-model="form.nama"
              :placeholder="t('masterdata.offices.fields.nama')"
              class="w-full"
            />
          </UFormField>
          <UFormField :label="t('masterdata.offices.fields.kota')">
            <UInput
              v-model="form.kota"
              :placeholder="t('masterdata.offices.fields.kota')"
              class="w-full"
            />
          </UFormField>
        </div>
        <!-- Alamat full-width -->
        <UFormField :label="t('masterdata.offices.fields.alamat')">
          <UTextarea
            v-model="form.alamat"
            :placeholder="t('masterdata.offices.fields.alamat')"
            class="w-full"
          />
        </UFormField>
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
          <USwitch v-model="form.active" />
        </div>
      </div>
    </FormSlideover>
  </div>
</template>
