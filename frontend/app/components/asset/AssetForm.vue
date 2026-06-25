<script setup lang="ts">
import type { Asset } from '~/types'
import type { AssetInput } from '~/composables/api/useAssets'
import { useAssets } from '~/composables/api/useAssets'

const props = defineProps<{
  mode: 'new' | 'edit'
  initial?: Asset | null
}>()

const { t } = useI18n()
const toast = useToast()
const api = useAssets()
const localePath = useLocalePath()

const KATEGORI = ['Elektronik', 'Furnitur', 'Kendaraan', 'Perangkat IT']
const KAT_CODE: Record<string, string> = { 'Elektronik': 'ELK', 'Furnitur': 'FUR', 'Kendaraan': 'KEN', 'Perangkat IT': 'ITX' }
const KANTOR = ['Cabang Jakarta Selatan', 'Outlet Blok M', 'Outlet Kemang', 'Kantor Pusat']
const KANTOR_CODE: Record<string, string> = { 'Cabang Jakarta Selatan': 'JKT01', 'Outlet Blok M': 'BLM01', 'Outlet Kemang': 'KMG01', 'Kantor Pusat': 'PST00' }
const BRAND = ['Dell', 'HP', 'Lenovo', 'Apple', 'Epson', 'Toyota', 'Honda', 'IKEA', 'Daikin']
const MODEL = ['Latitude 5440', 'ProBook 450', 'ThinkPad E14', 'MacBook Air M3', 'EB-X51', 'Avanza 1.5 G', 'Vario 160', 'BEKANT', 'FTKC50']
const LANTAI = ['Lantai 1', 'Lantai 2', 'Lantai 3', 'Basement']
const RUANGAN = ['Ruang IT', 'Ruang Operasional', 'Ruang Server', 'Ruang Rapat A', 'Gudang Aset', 'Lobi']
const PEMEGANG = ['Rina Putri', 'Andi Saputra', 'Budi Hartono', 'Dewi Lestari']
const VENDOR = ['PT Sinar Komputindo', 'PT Mitra Furnitama', 'Auto2000', 'CV Teknologi Nusantara']

const opt = (arr: string[]) => arr.map(v => ({ value: v, label: v }))

function splitLokasi(lokasi: string): { lantai: string, ruangan: string } {
  const parts = lokasi.split('—').map(s => s.trim())
  if (parts.length === 2) return { lantai: parts[0] ?? '', ruangan: parts[1] ?? '' }
  return { lantai: '', ruangan: lokasi }
}
function splitBrand(brand: string): { brand: string, model: string } {
  const parts = brand.split(' ')
  return { brand: parts[0] ?? '', model: parts.slice(1).join(' ') }
}

const form = reactive({
  nama: '', kategori: '', brand: '', model: '', kantor: '', lantai: '', ruangan: '', pemegang: '',
  tglBeli: '', harga: '', vendor: '', metode: 'straight_line', masa: '4', residu: '0'
})
const errors = ref<Record<string, string>>({})

if (props.mode === 'edit' && props.initial) {
  const a = props.initial
  const bm = splitBrand(a.brand)
  const loc = splitLokasi(a.lokasi)
  Object.assign(form, {
    nama: a.nama, kategori: a.kategori, brand: bm.brand, model: bm.model,
    kantor: a.kantor, lantai: loc.lantai, ruangan: loc.ruangan, pemegang: a.holder === '—' ? '' : a.holder,
    tglBeli: a.tgl, harga: String(a.harga), vendor: '', metode: 'straight_line', masa: '4', residu: '0'
  })
}

const metodeOptions = computed(() => [
  { value: 'straight_line', label: t('assets.form.methodStraight') },
  { value: 'declining', label: t('assets.form.methodDeclining') }
])

const kodePreview = computed(() => {
  if (props.mode === 'edit' && props.initial) return props.initial.tag
  if (!form.kantor && !form.kategori) return '—'
  const kc = KANTOR_CODE[form.kantor] ?? 'XXX00'
  const cc = KAT_CODE[form.kategori] ?? 'XXX'
  const year = form.tglBeli ? form.tglBeli.slice(0, 4) : '2026'
  return `${kc}-${cc}-${year}-00001`
})

function setField(name: keyof typeof form, val: string) {
  form[name] = val
  if (errors.value[name]) {
    const { [name]: _omit, ...rest } = errors.value
    errors.value = rest
  }
}

const REQUIRED: (keyof typeof form)[] = ['nama', 'kategori', 'kantor', 'tglBeli', 'harga']
function validate(): boolean {
  const next: Record<string, string> = {}
  for (const k of REQUIRED) {
    if (!String(form[k]).trim()) next[k] = t(`assets.form.errors.${k}`)
  }
  errors.value = next
  return Object.keys(next).length === 0
}

const saving = ref(false)
async function save() {
  if (!validate()) {
    toast.add({ title: t('assets.form.fixErrors'), color: 'error', icon: 'i-lucide-circle-alert' })
    return
  }
  saving.value = true
  try {
    const brandModel = [form.brand, form.model].filter(Boolean).join(' ')
    const lokasi = [form.lantai, form.ruangan].filter(Boolean).join(' — ') || form.ruangan
    const harga = Number(form.harga) || 0
    if (props.mode === 'edit' && props.initial) {
      const input: Partial<AssetInput> = {
        nama: form.nama, kategori: form.kategori, brand: brandModel,
        kantor: form.kantor, lokasi, holder: form.pemegang || '—', tgl: form.tglBeli, harga
      }
      await api.update(props.initial.tag, input)
      toast.add({ title: t('assets.form.savedToast'), color: 'success', icon: 'i-lucide-save' })
      navigateTo(localePath(`/assets/${props.initial.tag}`))
    } else {
      const tag = kodePreview.value === '—' ? `NEW-${Date.now()}` : kodePreview.value
      await api.create({
        tag, nama: form.nama, kategori: form.kategori, brand: brandModel, status: 'tersedia',
        kantor: form.kantor, lokasi, holder: form.pemegang || '—', tgl: form.tglBeli, harga, buku: harga
      })
      toast.add({ title: t('assets.form.createdToast'), color: 'success', icon: 'i-lucide-plus' })
      navigateTo(localePath('/assets'))
    }
  } finally {
    saving.value = false
  }
}

function cancel() {
  navigateTo(localePath(props.mode === 'edit' && props.initial ? `/assets/${props.initial.tag}` : '/assets'))
}
</script>

<template>
  <div>
    <div class="flex items-start justify-between gap-4 flex-wrap mb-1.5">
      <div>
        <h1 class="text-[23px] font-bold tracking-tight mb-[5px]">
          {{ mode === 'edit' ? t('assets.form.titleEdit') : t('assets.form.titleNew') }}
        </h1>
        <p class="text-sm text-muted">
          {{ mode === 'edit' ? t('assets.form.subEdit') : t('assets.form.subNew') }}
        </p>
      </div>
    </div>

    <!-- maker-checker banner (create only) -->
    <div
      v-if="mode === 'new'"
      class="flex items-start gap-2.5 px-3.5 py-3 my-4 rounded-[11px] bg-info/10 border border-info/30"
    >
      <UIcon
        name="i-lucide-info"
        class="size-4 text-info mt-0.5 flex-none"
      />
      <span class="text-[13px] leading-snug text-info">{{ t('assets.form.banner') }}</span>
    </div>

    <div class="bg-default border border-default rounded-[14px] shadow-sm overflow-hidden mt-4">
      <div class="p-6 space-y-7">
        <!-- Identity -->
        <section>
          <div class="flex items-center gap-2 mb-4">
            <span class="text-xs font-semibold uppercase tracking-wide text-muted">{{ t('assets.form.sections.identity') }}</span>
            <div class="flex-1 h-px bg-default" />
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <UFormField
              :label="t('assets.form.fields.nama')"
              required
              :error="errors.nama"
              class="sm:col-span-2"
            >
              <UInput
                :model-value="form.nama"
                :placeholder="t('assets.form.placeholders.nama')"
                class="w-full"
                @update:model-value="setField('nama', String($event))"
              />
            </UFormField>
            <UFormField
              :label="t('assets.form.fields.kategori')"
              required
              :error="errors.kategori"
            >
              <USelect
                :model-value="form.kategori"
                :items="opt(KATEGORI)"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                @update:model-value="setField('kategori', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.kode')">
              <UInput
                :model-value="kodePreview"
                disabled
                class="w-full font-mono"
              />
              <template #hint>
                <span class="text-xs text-dimmed mt-1">{{ t('assets.form.kodeNote') }}</span>
              </template>
            </UFormField>
            <UFormField :label="t('assets.form.fields.brand')">
              <USelect
                :model-value="form.brand"
                :items="opt(BRAND)"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                @update:model-value="setField('brand', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.model')">
              <USelect
                :model-value="form.model"
                :items="opt(MODEL)"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                @update:model-value="setField('model', String($event))"
              />
            </UFormField>
          </div>
        </section>

        <!-- Placement -->
        <section>
          <div class="flex items-center gap-2 mb-4">
            <span class="text-xs font-semibold uppercase tracking-wide text-muted">{{ t('assets.form.sections.placement') }}</span>
            <div class="flex-1 h-px bg-default" />
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <UFormField
              :label="t('assets.form.fields.kantor')"
              required
              :error="errors.kantor"
            >
              <USelect
                :model-value="form.kantor"
                :items="opt(KANTOR)"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                @update:model-value="setField('kantor', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.pemegang')">
              <USelect
                :model-value="form.pemegang"
                :items="opt(PEMEGANG)"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                @update:model-value="setField('pemegang', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.lantai')">
              <USelect
                :model-value="form.lantai"
                :items="opt(LANTAI)"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                @update:model-value="setField('lantai', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.ruangan')">
              <USelect
                :model-value="form.ruangan"
                :items="opt(RUANGAN)"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                @update:model-value="setField('ruangan', String($event))"
              />
            </UFormField>
          </div>
        </section>

        <!-- Purchase -->
        <section>
          <div class="flex items-center gap-2 mb-4">
            <span class="text-xs font-semibold uppercase tracking-wide text-muted">{{ t('assets.form.sections.purchase') }}</span>
            <div class="flex-1 h-px bg-default" />
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <UFormField
              :label="t('assets.form.fields.tglBeli')"
              required
              :error="errors.tglBeli"
            >
              <UInput
                :model-value="form.tglBeli"
                type="date"
                class="w-full"
                @update:model-value="setField('tglBeli', String($event))"
              />
            </UFormField>
            <UFormField
              :label="t('assets.form.fields.harga')"
              required
              :error="errors.harga"
            >
              <UInput
                :model-value="form.harga"
                type="number"
                placeholder="0"
                class="w-full"
                @update:model-value="setField('harga', String($event))"
              />
            </UFormField>
            <UFormField
              :label="t('assets.form.fields.vendor')"
              class="sm:col-span-2"
            >
              <USelect
                :model-value="form.vendor"
                :items="opt(VENDOR)"
                :placeholder="t('assets.form.placeholders.select')"
                class="w-full"
                @update:model-value="setField('vendor', String($event))"
              />
            </UFormField>
          </div>
        </section>

        <!-- Depreciation -->
        <section>
          <div class="flex items-center gap-2 mb-4">
            <span class="text-xs font-semibold uppercase tracking-wide text-muted">{{ t('assets.form.sections.depreciation') }}</span>
            <div class="flex-1 h-px bg-default" />
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <UFormField :label="t('assets.form.fields.metode')">
              <USelect
                :model-value="form.metode"
                :items="metodeOptions"
                class="w-full"
                @update:model-value="setField('metode', String($event))"
              />
            </UFormField>
            <UFormField :label="t('assets.form.fields.masa')">
              <UInput
                :model-value="form.masa"
                type="number"
                class="w-full"
                :ui="{ trailing: 'pe-12' }"
                @update:model-value="setField('masa', String($event))"
              >
                <template #trailing>
                  <span class="text-xs text-muted">{{ t('assets.form.years') }}</span>
                </template>
              </UInput>
            </UFormField>
            <UFormField :label="t('assets.form.fields.residu')">
              <UInput
                :model-value="form.residu"
                type="number"
                class="w-full"
                @update:model-value="setField('residu', String($event))"
              />
            </UFormField>
          </div>
        </section>

        <!-- Attachments -->
        <section>
          <div class="flex items-center gap-2 mb-4">
            <span class="text-xs font-semibold uppercase tracking-wide text-muted">{{ t('assets.form.sections.attachments') }}</span>
            <div class="flex-1 h-px bg-default" />
          </div>
          <button
            type="button"
            class="w-full flex flex-col items-center justify-center gap-2 py-8 px-4 rounded-[12px] border-2 border-dashed border-default text-center cursor-pointer hover:border-primary transition-colors"
            @click="toast.add({ title: t('assets.comingSoon'), color: 'neutral', icon: 'i-lucide-info' })"
          >
            <UIcon
              name="i-lucide-upload-cloud"
              class="size-7 text-dimmed"
            />
            <span class="text-sm font-medium">{{ t('assets.form.dropTitle') }}</span>
            <span class="text-xs text-dimmed">{{ t('assets.form.dropSub') }}</span>
          </button>
        </section>
      </div>

      <!-- footer -->
      <div class="flex items-center justify-between gap-3 px-6 py-4 border-t border-default bg-elevated">
        <span class="text-[12.5px] text-dimmed">{{ t('assets.form.requiredNote') }}</span>
        <div class="flex gap-2.5">
          <UButton
            color="neutral"
            variant="outline"
            :label="t('common.cancel')"
            @click="cancel"
          />
          <UButton
            icon="i-lucide-save"
            :loading="saving"
            :label="t('common.save')"
            @click="save"
          />
        </div>
      </div>
    </div>
  </div>
</template>
