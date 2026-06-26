<script setup lang="ts">
import L from 'leaflet'
import type { MapOffice } from '~/types'
import { jenisMeta } from '~/mock/officeMap'

const props = defineProps<{ offices: MapOffice[], selectedId: string | null }>()
const emit = defineEmits<{ (e: 'select', id: string): void }>()

const el = ref<HTMLElement | null>(null)
let map: L.Map | null = null
let markers = new Map<string, L.Marker>()

function pinHtml(o: MapOffice, selected: boolean): string {
  const color = `var(${jenisMeta[o.jenis].pinVar})`
  const size = selected ? 34 : 27
  return `<div style="position:relative;display:flex;flex-direction:column;align-items:center;">
    <div style="display:flex;align-items:center;justify-content:center;width:${size}px;height:${size}px;border-radius:50% 50% 50% 0;background:${color};transform:rotate(-45deg);box-shadow:0 3px 8px rgba(0,0,0,.3);border:2px solid var(--ui-bg);"></div>
  </div>`
}

function icon(o: MapOffice, selected: boolean): L.DivIcon {
  const size = selected ? 34 : 27
  return L.divIcon({ html: pinHtml(o, selected), className: 'office-pin', iconSize: [size, size], iconAnchor: [size / 2, size] })
}

function render() {
  if (!map) return
  for (const m of markers.values()) {
    m.remove()
  }
  markers = new Map()
  for (const o of props.offices) {
    const selected = o.id === props.selectedId
    const m = L.marker([o.lat, o.lng], { icon: icon(o, selected), zIndexOffset: selected ? 1000 : 0 })
    m.on('click', () => {
      emit('select', o.id)
    })
    m.addTo(map)
    markers.set(o.id, m)
  }
}

function fitAll() {
  if (!map || props.offices.length === 0) return
  map.fitBounds(L.latLngBounds(props.offices.map(o => [o.lat, o.lng])), { padding: [48, 48] })
}

function resetView() {
  fitAll()
}
function zoomIn() {
  map?.zoomIn()
}
function zoomOut() {
  map?.zoomOut()
}
defineExpose({ resetView, zoomIn, zoomOut })

onMounted(() => {
  if (!el.value) return
  map = L.map(el.value, { zoomControl: false, attributionControl: true })
  L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', { maxZoom: 19, attribution: '© OpenStreetMap' }).addTo(map)
  render()
  fitAll()
})

onUnmounted(() => {
  map?.remove()
  map = null
})

watch(() => props.offices, () => {
  render()
  fitAll()
}, { deep: true })
watch(() => props.selectedId, (id) => {
  render()
  const o = props.offices.find(x => x.id === id)
  if (o && map) map.flyTo([o.lat, o.lng], Math.max(map.getZoom(), 12), { duration: 0.5 })
})
</script>

<template>
  <div
    ref="el"
    class="absolute inset-0"
  />
</template>
