// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Indonesian (`id`).
class AppLocalizationsId extends AppLocalizations {
  AppLocalizationsId([String locale = 'id']) : super(locale);

  @override
  String get appTitle => 'Inventra Mobile';

  @override
  String get commonRetry => 'Coba lagi';

  @override
  String get commonCancel => 'Batal';

  @override
  String get commonOfflineBanner => 'Offline — scan tersimpan di perangkat';

  @override
  String get commonSyncSynced => 'Tersinkron';

  @override
  String commonSyncPending(int count) {
    return '$count belum tersinkron';
  }

  @override
  String get commonSyncSyncing => 'Menyinkronkan…';

  @override
  String get commonSyncFailed => 'Gagal — coba lagi';

  @override
  String get commonSyncOffline => 'Offline';

  @override
  String get shellTabHome => 'Beranda';

  @override
  String get shellTabOpname => 'Opname';

  @override
  String get shellTabScan => 'Pindai';

  @override
  String get shellTabApproval => 'Approval';

  @override
  String get shellTabNotifications => 'Notif';

  @override
  String get notificationsTitle => 'Notifikasi';

  @override
  String get assetDetailTitle => 'Detail Aset';

  @override
  String get scanTitle => 'Pindai Label Aset';

  @override
  String get scanHint => 'Arahkan ke barcode / QR pada label aset';

  @override
  String get scanManualButton => 'Ketik kode manual';

  @override
  String get scanCloseTooltip => 'Tutup pemindai';

  @override
  String get scanTorchOnTooltip => 'Nyalakan senter';

  @override
  String get scanTorchOffTooltip => 'Matikan senter';

  @override
  String get scanCameraUnavailableTitle => 'Kamera tidak tersedia';

  @override
  String get scanCameraUnavailableBody =>
      'Izinkan akses kamera di pengaturan perangkat, atau gunakan input kode manual.';

  @override
  String get scanManualSheetTitle => 'Ketik kode manual';

  @override
  String get scanManualFieldLabel => 'Kode aset';

  @override
  String get scanManualFieldHint => 'JKT01-ELK-2026-00001';

  @override
  String get scanManualFieldHelper => 'Format: KANTOR-KATEGORI-TAHUN-NOMOR';

  @override
  String get scanManualSubmit => 'Cari';

  @override
  String get assetDetailPhotoPlaceholder => 'Belum ada foto';

  @override
  String get assetDetailSectionPlacement => 'Penempatan';

  @override
  String get assetDetailSectionInfo => 'Informasi';

  @override
  String get assetDetailSectionValue => 'Nilai';

  @override
  String get assetDetailFieldOffice => 'Kantor';

  @override
  String get assetDetailFieldRoom => 'Lantai / Ruangan';

  @override
  String get assetDetailFieldHolder => 'Pemegang saat ini';

  @override
  String get assetDetailFieldCategory => 'Kategori';

  @override
  String get assetDetailFieldBrandModel => 'Brand / Model';

  @override
  String get assetDetailFieldSerial => 'No. seri';

  @override
  String get assetDetailFieldPurchaseDate => 'Tanggal beli';

  @override
  String get assetDetailFieldVendor => 'Vendor';

  @override
  String get assetDetailFieldPurchaseCost => 'Harga beli';

  @override
  String get assetDetailFieldBookValue => 'Nilai buku';

  @override
  String get assetDetailRestrictedBadge => 'Dibatasi untuk peran Anda';

  @override
  String get assetDetailRestrictedTooltip =>
      'Field ini dibatasi untuk peran Anda';

  @override
  String get assetDetailStatusAvailable => 'Tersedia';

  @override
  String get assetDetailStatusAssigned => 'Dipinjam';

  @override
  String get assetDetailStatusUnderMaintenance => 'Maintenance';

  @override
  String get assetDetailStatusInTransfer => 'Dalam Mutasi';

  @override
  String get assetDetailStatusRetired => 'Purna Pakai';

  @override
  String get assetDetailStatusDisposed => 'Dilepas';

  @override
  String get assetDetailStatusLost => 'Hilang';

  @override
  String get assetDetailErrorTitle => 'Gagal memuat detail aset';

  @override
  String get assetDetailErrorNetworkBody =>
      'Tidak ada koneksi. Periksa jaringan Anda lalu coba lagi.';

  @override
  String get assetDetailErrorGenericBody => 'Terjadi kesalahan. Coba lagi.';

  @override
  String get assetDetailForbiddenTitle => 'Akses dibatasi';

  @override
  String get assetDetailForbiddenBody =>
      'Peran Anda tidak memiliki izin melihat aset.';

  @override
  String get assetDetailNotFoundTitle => 'Kode tidak dikenal';

  @override
  String assetDetailNotFoundBody(String tag) {
    return 'Kode $tag tidak terdaftar, atau aset ini di luar wewenang Anda.';
  }

  @override
  String get assetDetailScanAgain => 'Pindai Lagi';

  @override
  String get approvalDetailTitle => 'Detail Approval';

  @override
  String get opnameDetailTitle => 'Detail Opname';

  @override
  String get opnameVarianceTitle => 'Variance Opname';

  @override
  String get accountTitle => 'Profil';

  @override
  String get accountSettingsButton => 'Pengaturan';

  @override
  String get accountEditOnWeb =>
      'Penyuntingan profil dilakukan dari aplikasi web';

  @override
  String get accountSessionsTitle => 'Sesi Perangkat';

  @override
  String get accountSessionCurrentBadge => 'Perangkat ini';

  @override
  String get accountSessionActiveNow => 'aktif sekarang';

  @override
  String get accountSessionRevoke => 'Cabut';

  @override
  String get accountSessionRevokeConfirmTitle => 'Cabut sesi ini?';

  @override
  String accountSessionRevokeConfirmBody(String name) {
    return '$name akan keluar dan harus masuk kembali.';
  }

  @override
  String get accountSessionRevokeConfirmAction => 'Ya, Cabut';

  @override
  String accountSessionRevokedSnack(String name) {
    return 'Sesi $name dicabut';
  }

  @override
  String get accountSessionRevokeFailed => 'Gagal mencabut sesi. Coba lagi.';

  @override
  String get accountRevokeOthers => 'Keluar dari semua perangkat lain';

  @override
  String get accountRevokeOthersConfirmTitle =>
      'Keluar dari semua perangkat lain?';

  @override
  String accountRevokeOthersConfirmBody(int count) {
    return '$count sesi lain akan dicabut. Perangkat ini tetap masuk.';
  }

  @override
  String get accountRevokeOthersConfirmAction => 'Ya, Keluar';

  @override
  String get accountRevokeOthersFailed =>
      'Gagal mencabut sesi lain. Coba lagi.';

  @override
  String get accountSessionsEmpty => 'Belum ada sesi aktif yang tercatat.';

  @override
  String get accountSessionsErrorBody => 'Gagal memuat sesi perangkat.';

  @override
  String get accountLogout => 'Keluar';

  @override
  String get accountLogoutConfirmTitle => 'Keluar dari akun?';

  @override
  String get accountLogoutConfirmBody =>
      'Sesi Anda di perangkat ini akan diakhiri.';

  @override
  String get accountLogoutConfirmAction => 'Ya, Keluar';

  @override
  String get accountTimeJustNow => 'baru saja';

  @override
  String accountTimeMinutesAgo(int count) {
    return '$count mnt lalu';
  }

  @override
  String accountTimeHoursAgo(int count) {
    return '$count jam lalu';
  }

  @override
  String get accountTimeYesterday => 'kemarin';

  @override
  String accountTimeDaysAgo(int count) {
    return '$count hari lalu';
  }

  @override
  String get settingsTitle => 'Pengaturan';

  @override
  String get settingsSectionAppearance => 'Tampilan';

  @override
  String get settingsTheme => 'Tema';

  @override
  String get settingsThemeLight => 'Terang';

  @override
  String get settingsThemeDark => 'Gelap';

  @override
  String get settingsThemeSystem => 'Ikuti Sistem';

  @override
  String get settingsThemeSheetTitle => 'Pilih tema';

  @override
  String get settingsThemeApply => 'Terapkan';

  @override
  String get settingsLanguage => 'Bahasa';

  @override
  String get settingsLanguageSheetTitle => 'Pilih bahasa';

  @override
  String get settingsLanguageIndonesian => 'Indonesia';

  @override
  String get settingsLanguageEnglish => 'English';

  @override
  String get settingsSectionAbout => 'Tentang';

  @override
  String get settingsAppName => 'Inventra Mobile';

  @override
  String settingsVersion(String version, String build) {
    return 'Versi $version (build $build)';
  }

  @override
  String get homeTitle => 'Beranda';

  @override
  String get loginBrandName => 'Inventra';

  @override
  String get loginBrandBadge => 'MOBILE';

  @override
  String get loginTagline => 'Pendamping lapangan manajemen aset';

  @override
  String get loginCardTitle => 'Masuk';

  @override
  String get loginCardSubtitle => 'Gunakan akun Inventra Anda';

  @override
  String get loginEmailLabel => 'Email';

  @override
  String get loginEmailHint => 'nama@bank.co.id';

  @override
  String get loginPasswordLabel => 'Kata sandi';

  @override
  String get loginPasswordHint => 'Masukkan kata sandi';

  @override
  String get loginShowPassword => 'Tampilkan kata sandi';

  @override
  String get loginHidePassword => 'Sembunyikan kata sandi';

  @override
  String get loginSubmitButton => 'Masuk';

  @override
  String get loginSubmitLoading => 'Memproses…';

  @override
  String get loginErrorInvalidCredentials =>
      'Email atau kata sandi salah. Coba lagi.';

  @override
  String get loginErrorNetwork =>
      'Tidak ada koneksi. Periksa jaringan Anda lalu coba lagi.';

  @override
  String get loginErrorRateLimited =>
      'Terlalu banyak percobaan. Coba lagi beberapa saat lagi.';

  @override
  String get loginErrorGeneric => 'Terjadi kesalahan. Coba lagi.';

  @override
  String get loginLanguageIndonesian => 'ID';

  @override
  String get loginLanguageEnglish => 'EN';

  @override
  String loginVersion(String version, String build) {
    return 'Inventra Mobile v$version · Build $build';
  }

  @override
  String get approvalInboxTitle => 'Approval';

  @override
  String get approvalInboxFilterPending => 'Menunggu';

  @override
  String get approvalInboxFilterApproved => 'Disetujui';

  @override
  String get approvalInboxFilterRejected => 'Ditolak';

  @override
  String get approvalInboxFilterAll => 'Semua';

  @override
  String get approvalInboxPullToRefresh => 'Tarik untuk menyegarkan';

  @override
  String get approvalInboxEmptyPendingTitle => 'Tidak ada pengajuan menunggu';

  @override
  String get approvalInboxEmptyPendingBody =>
      'Semua pengajuan dalam lingkup Anda sudah diputus. Kerja bagus!';

  @override
  String get approvalInboxEmptyPendingAction => 'Lihat riwayat';

  @override
  String get approvalInboxEmptyFilteredTitle => 'Tidak ada pengajuan';

  @override
  String get approvalInboxEmptyFilteredBody =>
      'Belum ada pengajuan dengan status ini di lingkup Anda.';

  @override
  String get approvalInboxErrorTitle => 'Gagal memuat pengajuan';

  @override
  String get approvalInboxErrorNetworkBody =>
      'Tidak ada koneksi. Periksa jaringan Anda lalu coba lagi.';

  @override
  String get approvalInboxErrorGenericBody => 'Terjadi kesalahan. Coba lagi.';

  @override
  String get approvalInboxForbiddenTitle => 'Akses dibatasi';

  @override
  String get approvalInboxForbiddenBody =>
      'Peran Anda tidak memiliki izin melihat pengajuan.';

  @override
  String get approvalInboxLoadMoreFailed => 'Gagal memuat halaman berikutnya';

  @override
  String get approvalCardSensitive => 'sensitif';

  @override
  String get approvalTypeAssetCreate => 'Registrasi Aset';

  @override
  String get approvalTypeAssetDisposal => 'Penghapusan';

  @override
  String get approvalTypeAssetTransfer => 'Mutasi';

  @override
  String get approvalTypeAssignment => 'Peminjaman';

  @override
  String get approvalTypeMaintenance => 'Perbaikan';

  @override
  String get approvalTypeValuationExclusion => 'Pengecualian Valuasi';

  @override
  String get approvalStatusPending => 'Menunggu';

  @override
  String get approvalStatusApproved => 'Disetujui';

  @override
  String get approvalStatusRejected => 'Ditolak';

  @override
  String get approvalStatusCancelled => 'Dibatalkan';

  @override
  String get approvalTimeJustNow => 'baru saja';

  @override
  String approvalTimeMinutesAgo(int count) {
    return '$count mnt lalu';
  }

  @override
  String approvalTimeHoursAgo(int count) {
    return '$count jam lalu';
  }

  @override
  String get approvalTimeYesterday => 'kemarin';

  @override
  String approvalTimeDaysAgo(int count) {
    return '$count hari lalu';
  }

  @override
  String get approvalDetailSensitiveBanner =>
      'Tindakan sensitif — periksa saksama sebelum memutus';

  @override
  String get approvalDetailSectionData => 'Data yang diajukan';

  @override
  String get approvalDetailSectionSteps => 'Jenjang persetujuan';

  @override
  String get approvalDetailFieldAsset => 'Aset';

  @override
  String get approvalDetailFieldAmount => 'Nilai pengajuan';

  @override
  String get approvalDetailFieldReason => 'Alasan';

  @override
  String get approvalDetailFieldName => 'Nama aset';

  @override
  String get approvalDetailFieldCategory => 'Kategori';

  @override
  String get approvalDetailFieldOffice => 'Kantor';

  @override
  String get approvalDetailFieldRoom => 'Ruangan';

  @override
  String get approvalDetailFieldOfficeChange => 'Kantor penempatan';

  @override
  String get approvalDetailFieldAssetClass => 'Kelas aset';

  @override
  String get approvalDetailAssetClassTangible => 'Berwujud';

  @override
  String get approvalDetailAssetClassIntangible => 'Tak berwujud';

  @override
  String get approvalDetailFieldPurchaseCost => 'Harga beli';

  @override
  String get approvalDetailFieldPurchaseDate => 'Tanggal beli';

  @override
  String get approvalDetailFieldSerial => 'No. seri';

  @override
  String get approvalDetailFieldBrandModel => 'Brand / Model';

  @override
  String get approvalDetailFieldVendor => 'Vendor';

  @override
  String get approvalDetailFieldPoNumber => 'No. PO';

  @override
  String get approvalDetailFieldFundingSource => 'Sumber dana';

  @override
  String get approvalDetailFieldWarrantyExpiry => 'Akhir garansi';

  @override
  String get approvalDetailFieldNotes => 'Catatan';

  @override
  String get approvalDetailFieldMethod => 'Metode pelepasan';

  @override
  String get approvalDetailMethodSale => 'Penjualan';

  @override
  String get approvalDetailMethodAuction => 'Lelang';

  @override
  String get approvalDetailMethodDonation => 'Hibah';

  @override
  String get approvalDetailMethodWriteOff => 'Penghapusbukuan';

  @override
  String get approvalDetailFieldDisposalDate => 'Tanggal pelepasan';

  @override
  String get approvalDetailFieldProceeds => 'Nilai jual';

  @override
  String get approvalDetailFieldBookValue => 'Nilai buku';

  @override
  String get approvalDetailFieldBastNo => 'No. BAST';

  @override
  String get approvalDetailFieldConditionSent => 'Kondisi saat kirim';

  @override
  String get approvalDetailFieldTransferDate => 'Tanggal mutasi';

  @override
  String get approvalDetailRestrictedData => 'Dibatasi untuk peran Anda';

  @override
  String get approvalDetailStepMaker => 'Maker';

  @override
  String approvalDetailStepSubmitted(String date) {
    return 'Mengajukan · $date';
  }

  @override
  String approvalDetailStepApproved(String date) {
    return 'Disetujui · $date';
  }

  @override
  String approvalDetailStepRejected(String date) {
    return 'Ditolak · $date';
  }

  @override
  String get approvalDetailStepWaiting => 'Menunggu keputusan';

  @override
  String get approvalDetailStepUpcoming => 'Berikutnya';

  @override
  String get approvalDetailLevelOffice => 'Approver kantor';

  @override
  String get approvalDetailLevelOfficeSubtree => 'Approver kantor & jajaran';

  @override
  String get approvalDetailLevelWilayah => 'Approver kanwil';

  @override
  String get approvalDetailLevelPusat => 'Approver pusat';

  @override
  String get approvalDetailNoteHint => 'Tambahkan catatan (opsional)';

  @override
  String get approvalDetailApprove => 'Setujui';

  @override
  String get approvalDetailReject => 'Tolak';

  @override
  String get approvalDetailApproveConfirmTitle => 'Setujui pengajuan ini?';

  @override
  String approvalDetailApproveConfirmBody(String title, String maker) {
    return '$title dari $maker akan disetujui dan lanjut ke tahap berikutnya.';
  }

  @override
  String get approvalDetailApproveConfirmAction => 'Ya, Setujui';

  @override
  String get approvalDetailRejectConfirmTitle => 'Tolak pengajuan ini?';

  @override
  String approvalDetailRejectConfirmBody(String title, String maker) {
    return '$title dari $maker akan ditolak dan dikembalikan ke maker.';
  }

  @override
  String get approvalDetailRejectConfirmAction => 'Ya, Tolak';

  @override
  String get approvalDetailYourNote => 'Catatan Anda';

  @override
  String get approvalDetailApprovedSnack => 'Pengajuan disetujui';

  @override
  String get approvalDetailRejectedSnack => 'Pengajuan ditolak';

  @override
  String get approvalDetailDecidedApproved => 'Pengajuan telah disetujui';

  @override
  String get approvalDetailDecidedByYouApproved =>
      'Anda telah menyetujui pengajuan ini';

  @override
  String get approvalDetailDecidedRejected => 'Pengajuan telah ditolak';

  @override
  String get approvalDetailDecidedByYouRejected =>
      'Anda telah menolak pengajuan ini';

  @override
  String get approvalDetailDecidedCancelled =>
      'Pengajuan dibatalkan oleh maker';

  @override
  String get approvalDetailSodOwnRequest =>
      'Ini pengajuan Anda — keputusan menunggu approver lain (maker tidak boleh memutus pengajuannya sendiri).';

  @override
  String get approvalDetailErrorSod =>
      'Anda tidak berwenang memutus pengajuan ini — maker atau approver sebelumnya tidak boleh memutus pengajuannya sendiri.';

  @override
  String get approvalDetailErrorConflict =>
      'Pengajuan sudah berubah status di tempat lain. Memuat ulang…';

  @override
  String get approvalDetailErrorNetwork =>
      'Tidak ada koneksi. Periksa jaringan Anda lalu coba lagi.';

  @override
  String get approvalDetailErrorGeneric => 'Terjadi kesalahan. Coba lagi.';

  @override
  String get approvalDetailErrorTitle => 'Gagal memuat pengajuan';

  @override
  String get approvalDetailNotFoundTitle => 'Pengajuan tidak ditemukan';

  @override
  String get approvalDetailNotFoundBody =>
      'Pengajuan tidak ada atau di luar lingkup Anda.';

  @override
  String get approvalDetailForbiddenTitle => 'Akses dibatasi';

  @override
  String get approvalDetailForbiddenBody =>
      'Peran Anda tidak memiliki izin melihat pengajuan ini.';

  @override
  String get opnameSessionsTitle => 'Stock Opname';

  @override
  String get opnameSessionsFilterRunning => 'Berjalan';

  @override
  String get opnameSessionsFilterClosed => 'Selesai';

  @override
  String get opnameSessionsFilterAll => 'Semua';

  @override
  String opnameSessionsProgress(int counted, int total) {
    return '$counted dari $total tercocokkan';
  }

  @override
  String get opnameSessionsContinue => 'Lanjutkan Menghitung';

  @override
  String get opnameSessionsReportOnWeb => 'Berita Acara di web';

  @override
  String get opnameSessionsFootnote =>
      'Sesi dibuat dan diselesaikan dari aplikasi web';

  @override
  String get opnameSessionsEmptyTitle => 'Tidak ada sesi opname aktif';

  @override
  String get opnameSessionsEmptyBody =>
      'Sesi baru dibuat oleh admin dari aplikasi web. Anda akan diberi tahu bila ditugaskan.';

  @override
  String get opnameSessionsEmptyFilteredTitle => 'Tidak ada sesi';

  @override
  String get opnameSessionsEmptyFilteredBody =>
      'Belum ada sesi opname dengan status ini di lingkup Anda.';

  @override
  String get opnameSessionsErrorTitle => 'Gagal memuat sesi opname';

  @override
  String get opnameErrorNetworkBody =>
      'Tidak ada koneksi. Periksa jaringan Anda lalu coba lagi.';

  @override
  String get opnameErrorGenericBody => 'Terjadi kesalahan. Coba lagi.';

  @override
  String get opnameForbiddenTitle => 'Akses dibatasi';

  @override
  String get opnameForbiddenBody =>
      'Peran Anda tidak memiliki izin melihat stock opname.';

  @override
  String get opnameStatusOpen => 'Terjadwal';

  @override
  String get opnameStatusCounting => 'Berjalan';

  @override
  String get opnameStatusReconciling => 'Rekonsiliasi';

  @override
  String get opnameStatusClosed => 'Selesai';

  @override
  String get opnameOfflineBanner =>
      'Offline — pemindaian dinonaktifkan. Mode offline hadir di fase berikutnya.';

  @override
  String get opnameCountingScanButton => 'Pindai Aset Berikutnya';

  @override
  String get opnameCountingManualButton => 'Ketik kode';

  @override
  String get opnameCountingRecentHeader => 'Baru saja dipindai';

  @override
  String get opnameCountingRecentEmpty => 'Belum ada aset yang dipindai.';

  @override
  String opnameCountingRingTotal(int total) {
    return '/$total';
  }

  @override
  String get opnameCountingVarianceTooltip => 'Lihat variance';

  @override
  String get opnameDetailErrorTitle => 'Gagal memuat sesi opname';

  @override
  String get opnameDetailNotFoundTitle => 'Sesi tidak ditemukan';

  @override
  String get opnameDetailNotFoundBody =>
      'Sesi tidak ada atau di luar lingkup Anda.';

  @override
  String get opnameResultFound => 'Ditemukan';

  @override
  String get opnameResultNotFound => 'Tidak Ditemukan';

  @override
  String get opnameResultDamaged => 'Rusak';

  @override
  String get opnameResultMisplaced => 'Salah Lokasi';

  @override
  String get opnameResultPending => 'Belum dihitung';

  @override
  String get opnameOutOfSnapshot => 'Di Luar Catatan';

  @override
  String get opnameSheetResultLabel => 'Hasil:';

  @override
  String get opnameSheetNoteHint => 'Catatan (opsional)';

  @override
  String get opnameSheetSave => 'Simpan & Lanjut';

  @override
  String get opnameSheetOutOfSnapshotInfo =>
      'Aset ini di luar snapshot sesi — dicatat sebagai temuan di luar catatan.';

  @override
  String get opnameResultSavedSnack => 'Hasil tersimpan';

  @override
  String opnameScanErrorNotFound(String tag) {
    return 'Kode $tag tidak dikenal atau di luar lingkup sesi.';
  }

  @override
  String get opnameScanErrorNotCounting =>
      'Sesi tidak dalam tahap menghitung — pemindaian tidak diizinkan.';

  @override
  String get opnameVarianceTabItems => 'Item';

  @override
  String get opnameVarianceTabVariance => 'Variance';

  @override
  String opnameVarianceLastLocation(String location) {
    return 'terakhir: $location';
  }

  @override
  String opnameVarianceNote(String note) {
    return 'Catatan: \"$note\"';
  }

  @override
  String get opnameVarianceFollowupNone => 'Belum ditindaklanjuti';

  @override
  String get opnameVarianceFollowupRequested => 'Diajukan: menunggu approval';

  @override
  String get opnameVarianceFollowupRecord => 'Tiket maintenance dibuat';

  @override
  String get opnameVarianceEmptyTitle => 'Tidak ada selisih';

  @override
  String opnameVarianceEmptyBody(int total) {
    return 'Semua $total aset tercocokkan dengan catatan. Sesi siap diselesaikan dari aplikasi web.';
  }

  @override
  String get opnameVarianceFootnote =>
      'Penyelesaian sesi & Berita Acara dilakukan dari aplikasi web';

  @override
  String homeGreeting(String name) {
    return 'Halo, $name';
  }

  @override
  String get homeAccountTooltip => 'Profil';

  @override
  String get homeNotificationsTooltip => 'Notifikasi';

  @override
  String get homeOfflineBanner => 'Offline — data terakhir ditampilkan';

  @override
  String get homeOpnameCardTitle => 'Sesi Opname Aktif';

  @override
  String get homeOpnameEmptyBody =>
      'Tidak ada sesi opname yang sedang berjalan.';

  @override
  String get homeOpnameOpenList => 'Buka Opname';

  @override
  String get homeOpnameErrorBody => 'Gagal memuat sesi opname.';

  @override
  String homeOpnameProgress(int counted, int total) {
    return '$counted dari $total aset';
  }

  @override
  String get homeOpnameContinue => 'Lanjutkan';

  @override
  String get homeApprovalCardTitle => 'Approval Menunggu';

  @override
  String homeApprovalStale(int count) {
    return '$count di antaranya > 3 hari';
  }

  @override
  String get homeApprovalEmptyBody => 'Tidak ada pengajuan menunggu keputusan.';

  @override
  String get homeApprovalErrorBody => 'Gagal memuat pengajuan.';

  @override
  String get homeApprovalOpenInbox => 'Buka Inbox';

  @override
  String get homeQuickScan => 'Pindai Aset';

  @override
  String get homeQuickOpname => 'Sesi Opname';

  @override
  String get homeQuickApproval => 'Approval';

  @override
  String get homeQuickNotifications => 'Notifikasi';

  @override
  String get notificationsMarkAllRead => 'Tandai semua dibaca';

  @override
  String get notificationsMarkAllFailed =>
      'Gagal menandai semua dibaca. Coba lagi.';

  @override
  String get notificationsSectionToday => 'Hari ini';

  @override
  String get notificationsSectionYesterday => 'Kemarin';

  @override
  String get notificationsEmptyTitle => 'Belum ada notifikasi';

  @override
  String get notificationsEmptyBody =>
      'Pemberitahuan approval, maintenance, dan sinkronisasi akan muncul di sini.';

  @override
  String get notificationsErrorTitle => 'Gagal memuat notifikasi';

  @override
  String get notificationsErrorNetworkBody =>
      'Tidak ada koneksi. Periksa jaringan Anda lalu coba lagi.';

  @override
  String get notificationsErrorGenericBody => 'Terjadi kesalahan. Coba lagi.';

  @override
  String get notificationsLoadMoreFailed => 'Gagal memuat lebih banyak.';

  @override
  String get notificationsTimeJustNow => 'baru saja';

  @override
  String notificationsTimeMinutesAgo(int count) {
    return '$count menit lalu';
  }

  @override
  String notificationsTimeHoursAgo(int count) {
    return '$count jam lalu';
  }

  @override
  String notificationsTimeYesterdayAt(String time) {
    return 'Kemarin, $time';
  }

  @override
  String notificationsTimeAt(String date, String time) {
    return '$date, $time';
  }

  @override
  String get notificationsApprovalPendingTitle =>
      'Pengajuan menunggu persetujuan Anda';

  @override
  String notificationsApprovalPendingBody(String type, String step) {
    return '$type · Langkah $step';
  }

  @override
  String get notificationsApprovalApprovedTitle => 'Pengajuan Anda disetujui';

  @override
  String get notificationsApprovalRejectedTitle => 'Pengajuan Anda ditolak';

  @override
  String get notificationsApprovalDecidedTitle =>
      'Pengajuan Anda telah diputus';

  @override
  String get notificationsMaintenanceDueTitle => 'Maintenance jatuh tempo';

  @override
  String notificationsMaintenanceDueBody(String asset, String date) {
    return '$asset — jatuh tempo $date';
  }

  @override
  String notificationsMaintenanceDueDateOnly(String date) {
    return 'Jatuh tempo $date';
  }

  @override
  String get notificationsAssetReturnedTitle => 'Aset dikembalikan';

  @override
  String get catalogTitle => 'Katalog Aset';

  @override
  String get catalogSearchHint => 'Cari aset';

  @override
  String get catalogUnnamedAsset => 'Aset tanpa nama';

  @override
  String get catalogEmptyTitle => 'Belum ada aset';

  @override
  String get catalogEmptyBody => 'Aset dalam wilayah Anda akan tampil di sini.';

  @override
  String get catalogEmptySearchTitle => 'Tidak ada aset yang cocok';

  @override
  String get catalogEmptySearchBody =>
      'Coba kata kunci lain atau atur ulang pencarian.';

  @override
  String get catalogResetFilter => 'Atur ulang';

  @override
  String get catalogLoadMoreFailed => 'Gagal memuat lagi.';

  @override
  String get catalogErrorTitle => 'Gagal memuat katalog';

  @override
  String get catalogErrorNetworkBody => 'Periksa koneksi Anda lalu coba lagi.';

  @override
  String get catalogErrorGenericBody => 'Terjadi kesalahan. Coba lagi.';

  @override
  String get catalogForbiddenTitle => 'Tanpa akses';

  @override
  String get catalogForbiddenBody =>
      'Anda tidak memiliki izin melihat katalog aset.';

  @override
  String get catalogFilterCategory => 'Kategori';

  @override
  String get catalogFilterStatus => 'Status';

  @override
  String get catalogFilterOffice => 'Kantor';

  @override
  String get catalogFilterAll => 'Semua';

  @override
  String get catalogFilterNoOptions => 'Tidak ada data';

  @override
  String get catalogFilterOptionsError => 'Gagal memuat pilihan';

  @override
  String get catalogPickerStatusTitle => 'Pilih Status';

  @override
  String get catalogPickerCategoryTitle => 'Pilih Kategori';

  @override
  String get catalogPickerOfficeTitle => 'Pilih Kantor';

  @override
  String get myAssetsTitle => 'Aset Saya';

  @override
  String myAssetsCount(int count) {
    return '$count aset dipegang';
  }

  @override
  String myAssetsHeldSince(String date) {
    return 'Dipinjam sejak $date';
  }

  @override
  String myAssetsDue(String date) {
    return 'Jatuh tempo $date';
  }

  @override
  String get myAssetsOverdue => 'Terlambat';

  @override
  String get myAssetsEmptyTitle => 'Belum memegang aset';

  @override
  String get myAssetsEmptyBody =>
      'Aset yang ditugaskan ke Anda akan tampil di sini.';

  @override
  String get myAssetsErrorTitle => 'Gagal memuat aset Anda';

  @override
  String get myAssetsErrorNetworkBody => 'Periksa koneksi Anda lalu coba lagi.';

  @override
  String get myAssetsErrorGenericBody => 'Terjadi kesalahan. Coba lagi.';

  @override
  String get myAssetsForbiddenTitle => 'Tanpa akses';

  @override
  String get myAssetsForbiddenBody =>
      'Anda tidak memiliki izin melihat aset yang dipegang.';
}
