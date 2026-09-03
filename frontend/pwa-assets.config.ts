/**
 * Konfigurasi generate ikon PWA.
 *
 * Dijalankan MANUAL sekali, dan hasilnya di-commit ke public/ — generator sengaja
 * TIDAK dipasang sebagai dependency supaya `sharp` (native, puluhan MB) tidak ikut
 * terpasang di setiap install CI dan setiap build Docker untuk alat yang tidak pernah
 * dipakai saat build:
 *
 *   pnpm dlx @vite-pwa/assets-generator@1.0.2
 *
 * Karena itu berkas ini tidak mengimpor `defineConfig` dari paketnya; ia objek biasa.
 *
 * Preset bawaan `minimal-2023` memberi padding 0,3 dan latar putih untuk varian
 * maskable dan apple. Untuk mark ini keduanya salah: hasilnya kotak biru kecil di
 * tengah bidang putih. Mark-nya memang dirancang penuh-bidang, jadi padding dinolkan
 * dan latarnya diisi warna brand supaya sudut membulat sumber ikut terisi biru —
 * Android dan iOS yang menerapkan mask-nya sendiri.
 */
export default {
  preset: {
    transparent: {
      sizes: [64, 192, 512],
      favicons: [[48, 'favicon.ico']],
      padding: 0
    },
    maskable: {
      sizes: [512],
      padding: 0,
      resizeOptions: { background: '#005bfd' }
    },
    apple: {
      sizes: [180],
      padding: 0,
      resizeOptions: { background: '#005bfd' }
    }
  },
  images: ['public/logo-source.svg']
}
