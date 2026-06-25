<script setup lang="ts">
const props = defineProps<{
  tag: string
  nama: string
  kantor: string
  size: { w: number, h: number, qr: number, bar: number }
  showQr: boolean
  showBarcode: boolean
  fields: { nama: boolean, kode: boolean, kantor: boolean }
}>()

// Deterministic Code128-style stripe widths from the tag (visual placeholder).
const bars = computed(() => {
  const out: { w: number, on: boolean }[] = [{ w: 5, on: false }]
  for (let i = 0; i < props.tag.length; i++) {
    const c = props.tag.charCodeAt(i)
    out.push({ w: (c % 3) + 1, on: true })
    out.push({ w: ((c >> 2) % 3) + 1, on: false })
    out.push({ w: ((c >> 4) % 2) + 1, on: true })
    out.push({ w: ((c >> 1) % 2) + 1, on: false })
  }
  out.push({ w: 2, on: true })
  out.push({ w: 5, on: false })
  return out
})

// Deterministic QR-like 21×21 grid (finder patterns + seeded fill).
const qr = computed(() => {
  const N = 21
  const g: number[][] = Array.from({ length: N }, () => Array<number>(N).fill(0))
  const finder = (or: number, oc: number) => {
    for (let r = 0; r < 7; r++) {
      for (let c = 0; c < 7; c++) {
        const edge = r === 0 || r === 6 || c === 0 || c === 6
        const inner = r >= 2 && r <= 4 && c >= 2 && c <= 4
        g[or + r]![oc + c] = (edge || inner) ? 1 : 0
      }
    }
  }
  finder(0, 0)
  finder(0, N - 7)
  finder(N - 7, 0)
  let seed = 7
  for (let i = 0; i < props.tag.length; i++) seed = (seed * 31 + props.tag.charCodeAt(i)) >>> 0
  const rnd = () => {
    seed = (seed * 1103515245 + 12345) >>> 0
    return (seed >>> 16) / 65535
  }
  for (let r = 0; r < N; r++) {
    for (let c = 0; c < N; c++) {
      const inF = (r < 8 && c < 8) || (r < 8 && c >= N - 8) || (r >= N - 8 && c < 8)
      if (!inF) g[r]![c] = rnd() > 0.52 ? 1 : 0
    }
  }
  return g.flat()
})
</script>

<template>
  <!-- A printable label is always light (paper), independent of theme. -->
  <div
    class="bg-white text-slate-900 border border-slate-300 rounded-md flex items-center gap-2 p-2 overflow-hidden"
    :style="{ width: `${size.w}px`, height: `${size.h}px` }"
  >
    <div
      v-if="showQr"
      class="grid flex-none"
      :style="{ width: `${size.qr}px`, height: `${size.qr}px`, gridTemplateColumns: 'repeat(21, 1fr)', gridTemplateRows: 'repeat(21, 1fr)' }"
    >
      <div
        v-for="(cell, i) in qr"
        :key="i"
        :style="{ background: cell ? '#0f172a' : '#fff' }"
      />
    </div>
    <div class="flex-1 min-w-0 flex flex-col justify-center gap-1">
      <div
        v-if="fields.nama"
        class="text-[11px] font-semibold leading-tight truncate"
      >
        {{ nama }}
      </div>
      <div
        v-if="showBarcode"
        class="flex items-stretch w-full"
        :style="{ height: `${size.bar}px` }"
      >
        <div
          v-for="(b, i) in bars"
          :key="i"
          :style="{ width: `${b.w * 1.3}px`, background: b.on ? '#0f172a' : '#fff' }"
        />
      </div>
      <div
        v-if="fields.kode"
        class="text-[10px] font-mono text-slate-700 truncate"
      >
        {{ tag }}
      </div>
      <div
        v-if="fields.kantor"
        class="text-[9px] text-slate-500 truncate"
      >
        {{ kantor }}
      </div>
    </div>
  </div>
</template>
