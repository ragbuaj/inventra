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
