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
  String get commonComingSoon => 'Segera hadir';

  @override
  String get commonComingSoonBody =>
      'Layar ini sedang dibangun dan akan tersedia pada pembaruan berikutnya.';

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
  String get settingsTitle => 'Pengaturan';

  @override
  String get homeTitle => 'Beranda';

  @override
  String get homeLogoutTooltip => 'Keluar';

  @override
  String get homeLogoutConfirmTitle => 'Keluar dari akun?';

  @override
  String get homeLogoutConfirmMessage =>
      'Sesi Anda di perangkat ini akan diakhiri.';

  @override
  String get homeLogoutConfirmAction => 'Keluar';

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
}
