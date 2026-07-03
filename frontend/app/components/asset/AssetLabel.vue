<script setup lang="ts">
defineProps<{
  tag: string
  nama: string
  kantor: string
  size: { w: number, h: number, qr: number, bar: number }
  showQr: boolean
  showBarcode: boolean
  fields: { nama: boolean, kode: boolean, kantor: boolean }
  qrSrc?: string
  barcodeSrc?: string
}>()
</script>

<template>
  <!-- A printable label is always light (paper), independent of theme. -->
  <div
    class="bg-white text-slate-900 border border-slate-300 rounded-md flex items-center gap-2 p-2 overflow-hidden"
    :style="{ width: `${size.w}px`, height: `${size.h}px` }"
  >
    <div
      v-if="showQr"
      class="flex-none flex items-center justify-center"
      :style="{ width: `${size.qr}px`, height: `${size.qr}px` }"
    >
      <img
        v-if="qrSrc"
        :src="qrSrc"
        :alt="`QR ${tag}`"
        :style="{ width: `${size.qr}px`, height: `${size.qr}px` }"
        class="object-contain"
      >
      <USkeleton
        v-else
        :style="{ width: `${size.qr}px`, height: `${size.qr}px` }"
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
        class="flex items-center justify-center w-full"
        :style="{ height: `${size.bar}px` }"
      >
        <img
          v-if="barcodeSrc"
          :src="barcodeSrc"
          :alt="`Barcode ${tag}`"
          class="max-w-full max-h-full object-contain"
        >
        <USkeleton
          v-else
          class="w-full"
          :style="{ height: `${size.bar}px` }"
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
