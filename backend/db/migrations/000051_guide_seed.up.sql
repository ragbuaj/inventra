-- Seed konten Panduan Penggunaan: sembilan modul yang sampai rilis ini hidup
-- sebagai kunci i18n guidePage.sections di frontend.
--
-- Dipisah dari DDL 000050 supaya konten bisa di-seed ulang atau dibatalkan tanpa
-- menyentuh struktur. Idempoten lewat indeks unik parsial pada slug: menjalankan
-- migration ini dua kali tidak menggandakan modul.
--
-- Isi kolom *_id dan *_en disalin apa adanya dari id.json dan en.json supaya
-- halaman terlihat persis sama setelah beralih ke sumber data ini. Kunci i18n-nya
-- sendiri baru dihapus di rilis berikutnya (langkah 19 rencana), sehingga
-- membatalkan rilis ini cukup dengan mengembalikan kode.

INSERT INTO guide.guide_modules
  (slug, icon, sort_order, status, published_at, title_id, title_en, body_id, body_en, steps)
VALUES
  ($g$login-security$g$, $g$i-lucide-log-in$g$, 1, 'published', now(),
   $g$Masuk dan Keamanan Akun$g$,
   $g$Sign In and Account Security$g$,
   $g$Masuk dengan email kantor dan kata sandi Anda. Anda juga dapat masuk dengan akun Google organisasi bila fitur ini diaktifkan.$g$,
   $g$Sign in with your work email and password. You can also sign in with your organization Google account if the feature is enabled.$g$,
   $g$[{"text_id":"Buka halaman Masuk, isi email dan kata sandi, lalu tekan Masuk.","text_en":"Open the Sign In page, enter your email and password, then press Sign In."},{"text_id":"Jika lupa kata sandi, gunakan tautan Lupa Kata Sandi untuk menerima instruksi reset.","text_en":"If you forget your password, use the Forgot Password link to receive reset instructions."},{"text_id":"Ganti kata sandi secara berkala melalui menu Profil Akun.","text_en":"Change your password periodically via the My Account menu."}]$g$::jsonb),
  ($g$dashboard$g$, $g$i-lucide-layout-dashboard$g$, 2, 'published', now(),
   $g$Dashboard$g$,
   $g$Dashboard$g$,
   $g$Halaman awal menampilkan ringkasan aset, aktivitas terbaru, dan pintasan ke tugas yang membutuhkan perhatian Anda, seperti persetujuan yang tertunda.$g$,
   $g$The home page shows an asset summary, recent activity, and shortcuts to tasks that need your attention, such as pending approvals.$g$,
   $g$[]$g$::jsonb),
  ($g$asset-catalog$g$, $g$i-lucide-package$g$, 3, 'published', now(),
   $g$Katalog Aset$g$,
   $g$Asset Catalog$g$,
   $g$Telusuri dan kelola seluruh aset dalam lingkup kantor Anda.$g$,
   $g$Browse and manage all assets within your office scope.$g$,
   $g$[{"text_id":"Gunakan kolom pencarian dan filter (status, kategori) untuk menemukan aset.","text_en":"Use the search box and filters (status, category) to find assets."},{"text_id":"Klik sebuah aset untuk melihat detail, riwayat, dan dokumennya.","text_en":"Click an asset to view its details, history, and documents."},{"text_id":"Tekan Tambah Aset untuk registrasi aset baru, atau gunakan Import untuk unggah massal melalui berkas.","text_en":"Press Add Asset to register a new asset, or use Import for bulk upload via a file."},{"text_id":"Cetak label barcode atau QR dari menu Label & Barcode.","text_en":"Print barcode or QR labels from the Label & Barcode menu."}]$g$::jsonb),
  ($g$loan-assignment$g$, $g$i-lucide-hand$g$, 4, 'published', now(),
   $g$Peminjaman dan Penugasan$g$,
   $g$Borrowing and Assignment$g$,
   $g$Ajukan peminjaman aset atau serahkan aset kepada pegawai.$g$,
   $g$Request to borrow an asset or hand an asset to an employee.$g$,
   $g$[{"text_id":"Buka Peminjaman Aset, pilih aset yang tersedia, lalu ajukan permohonan.","text_en":"Open Asset Borrowing, pick an available asset, then submit a request."},{"text_id":"Permohonan masuk ke antrean persetujuan sesuai kewenangan.","text_en":"The request enters the approval queue according to authority."},{"text_id":"Aset yang telah disetujui tercatat pada menu Penugasan beserta pemegangnya.","text_en":"Approved assets are recorded under the Assignments menu along with the holder."}]$g$::jsonb),
  ($g$request-approval$g$, $g$i-lucide-check-square$g$, 5, 'published', now(),
   $g$Pengajuan dan Persetujuan$g$,
   $g$Requests and Approval$g$,
   $g$Inventra menerapkan mekanisme maker-checker berjenjang berdasarkan nilai aset.$g$,
   $g$Inventra applies a tiered maker-checker mechanism based on asset value.$g$,
   $g$[{"text_id":"Pembuat (maker) mengajukan tindakan; sistem menentukan penyetuju sesuai batas otorisasi.","text_en":"A maker submits an action; the system determines the approver by authorization limit."},{"text_id":"Penyetuju (checker) meninjau di menu Pengajuan & Approval, lalu menyetujui atau menolak.","text_en":"A checker reviews it in the Requests & Approval menu, then approves or rejects."},{"text_id":"Notifikasi memberi tahu Anda saat ada tugas yang menunggu.","text_en":"Notifications tell you when a task is waiting."}]$g$::jsonb),
  ($g$maintenance$g$, $g$i-lucide-wrench$g$, 6, 'published', now(),
   $g$Pemeliharaan (Maintenance)$g$,
   $g$Maintenance$g$,
   $g$Ajukan dan pantau perbaikan aset. Aset yang sedang diperbaiki berstatus Maintenance dan tidak dapat dipinjam hingga selesai.$g$,
   $g$Request and track asset repairs. An asset under repair has the Maintenance status and cannot be borrowed until it is done.$g$,
   $g$[]$g$::jsonb),
  ($g$transfer-opname-disposal$g$, $g$i-lucide-arrow-right-left$g$, 7, 'published', now(),
   $g$Mutasi, Stock Opname, dan Penghapusan$g$,
   $g$Transfer, Stock Opname, and Disposal$g$,
   $g$Kelola perpindahan antar kantor, pemeriksaan fisik, dan pelepasan aset.$g$,
   $g$Manage moves between offices, physical checks, and asset disposal.$g$,
   $g$[{"text_id":"Mutasi memindahkan aset antar kantor melalui persetujuan.","text_en":"Transfer moves an asset between offices through approval."},{"text_id":"Stock Opname mencatat hasil pemeriksaan fisik beserta selisihnya.","text_en":"Stock Opname records the results of a physical check and any discrepancy."},{"text_id":"Penghapusan melepas aset sambil mencatat keuntungan atau kerugian pelepasan.","text_en":"Disposal releases an asset while recording the gain or loss on disposal."}]$g$::jsonb),
  ($g$report-depreciation$g$, $g$i-lucide-bar-chart-2$g$, 8, 'published', now(),
   $g$Laporan dan Depresiasi$g$,
   $g$Reports and Depreciation$g$,
   $g$Lihat penyusutan komersial dan fiskal serta hasilkan laporan untuk kebutuhan pengawasan dan akuntansi melalui menu Depresiasi dan Laporan.$g$,
   $g$View commercial and fiscal depreciation and generate reports for oversight and accounting needs from the Depreciation and Reports menus.$g$,
   $g$[]$g$::jsonb),
  ($g$master-data-settings$g$, $g$i-lucide-database$g$, 9, 'published', now(),
   $g$Master Data dan Pengaturan$g$,
   $g$Master Data and Settings$g$,
   $g$Administrator mengelola data kantor, pegawai, dan kategori, serta pengaturan pengguna, peran (RBAC), lingkup data, dan izin field. Menu ini hanya tampil bagi pengguna dengan izin terkait.$g$,
   $g$Administrators manage offices, employees, and categories, plus user settings, roles (RBAC), data scope, and field permissions. These menus appear only for users with the relevant permissions.$g$,
   $g$[]$g$::jsonb)
ON CONFLICT DO NOTHING;
