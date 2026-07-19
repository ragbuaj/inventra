<script setup lang="ts">
// On-screen replica of the backend's BTN label template
// (backend/internal/asset/barcode.go drawBTN): rounded outer border with a
// paper margin, QR (with centered logo) on the left, a vertical divider, and
// the fixed summary column on the right. Sizes come in px at 5 px/mm.
defineProps<{
  tag: string
  nama: string
  kategori: string
  kantor: string
  tahun: string
  company: string
  disclaimer: string
  size: { w: number, h: number }
  qrSrc?: string
}>()
</script>

<template>
  <!-- A printable label is always light (paper), independent of theme. -->
  <div
    class="bg-white text-black p-[5px] overflow-hidden"
    :style="{ width: `${size.w}px`, height: `${size.h}px` }"
  >
    <div class="w-full h-full border-[1.5px] border-black rounded-md flex items-stretch overflow-hidden">
      <!-- QR (left) -->
      <div class="flex-none relative flex items-center justify-center p-[4px]">
        <div class="relative h-full aspect-square">
          <img
            v-if="qrSrc"
            :src="qrSrc"
            :alt="`QR ${tag}`"
            class="w-full h-full object-contain"
          >
          <USkeleton
            v-else
            class="w-full h-full"
          />
          <img
            v-if="qrSrc"
            src="/logo-btn.png"
            alt=""
            class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[38%] bg-white p-[2px] rounded-[2px]"
          >
        </div>
      </div>
      <!-- Divider -->
      <div class="flex-none w-[1.5px] bg-black" />
      <!-- Summary column (right) -->
      <div class="flex-1 min-w-0 flex flex-col px-[5px] py-[4px]">
        <div class="flex items-center gap-[4px] min-w-0">
          <img
            src="/logo-btn.png"
            alt=""
            class="h-[11px] flex-none"
          >
          <span class="text-[9px] font-bold leading-tight truncate">{{ company }}</span>
        </div>
        <div class="text-[10px] leading-snug truncate">
          {{ tag }}
        </div>
        <div class="h-px bg-black my-[2px]" />
        <div class="flex items-baseline justify-between gap-2 min-w-0">
          <span class="text-[13px] font-bold leading-tight truncate">{{ kantor }}</span>
          <span class="text-[13px] font-bold leading-tight flex-none">TP: {{ tahun }}</span>
        </div>
        <div class="text-[10px] leading-snug truncate">
          {{ kategori }}
        </div>
        <div class="text-[10px] leading-snug truncate">
          {{ nama }}
        </div>
        <div class="mt-auto text-[7.5px] font-bold leading-[1.2] text-center text-red-700 line-clamp-2">
          {{ disclaimer }}
        </div>
      </div>
    </div>
  </div>
</template>
