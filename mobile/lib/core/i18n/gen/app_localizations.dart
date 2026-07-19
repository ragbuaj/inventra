import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:intl/intl.dart' as intl;

import 'app_localizations_en.dart';
import 'app_localizations_id.dart';

// ignore_for_file: type=lint

/// Callers can lookup localized strings with an instance of AppLocalizations
/// returned by `AppLocalizations.of(context)`.
///
/// Applications need to include `AppLocalizations.delegate()` in their app's
/// `localizationDelegates` list, and the locales they support in the app's
/// `supportedLocales` list. For example:
///
/// ```dart
/// import 'gen/app_localizations.dart';
///
/// return MaterialApp(
///   localizationsDelegates: AppLocalizations.localizationsDelegates,
///   supportedLocales: AppLocalizations.supportedLocales,
///   home: MyApplicationHome(),
/// );
/// ```
///
/// ## Update pubspec.yaml
///
/// Please make sure to update your pubspec.yaml to include the following
/// packages:
///
/// ```yaml
/// dependencies:
///   # Internationalization support.
///   flutter_localizations:
///     sdk: flutter
///   intl: any # Use the pinned version from flutter_localizations
///
///   # Rest of dependencies
/// ```
///
/// ## iOS Applications
///
/// iOS applications define key application metadata, including supported
/// locales, in an Info.plist file that is built into the application bundle.
/// To configure the locales supported by your app, you’ll need to edit this
/// file.
///
/// First, open your project’s ios/Runner.xcworkspace Xcode workspace file.
/// Then, in the Project Navigator, open the Info.plist file under the Runner
/// project’s Runner folder.
///
/// Next, select the Information Property List item, select Add Item from the
/// Editor menu, then select Localizations from the pop-up menu.
///
/// Select and expand the newly-created Localizations item then, for each
/// locale your application supports, add a new item and select the locale
/// you wish to add from the pop-up menu in the Value field. This list should
/// be consistent with the languages listed in the AppLocalizations.supportedLocales
/// property.
abstract class AppLocalizations {
  AppLocalizations(String locale)
    : localeName = intl.Intl.canonicalizedLocale(locale.toString());

  final String localeName;

  static AppLocalizations of(BuildContext context) {
    return Localizations.of<AppLocalizations>(context, AppLocalizations)!;
  }

  static const LocalizationsDelegate<AppLocalizations> delegate =
      _AppLocalizationsDelegate();

  /// A list of this localizations delegate along with the default localizations
  /// delegates.
  ///
  /// Returns a list of localizations delegates containing this delegate along with
  /// GlobalMaterialLocalizations.delegate, GlobalCupertinoLocalizations.delegate,
  /// and GlobalWidgetsLocalizations.delegate.
  ///
  /// Additional delegates can be added by appending to this list in
  /// MaterialApp. This list does not have to be used at all if a custom list
  /// of delegates is preferred or required.
  static const List<LocalizationsDelegate<dynamic>> localizationsDelegates =
      <LocalizationsDelegate<dynamic>>[
        delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ];

  /// A list of this localizations delegate's supported locales.
  static const List<Locale> supportedLocales = <Locale>[
    Locale('en'),
    Locale('id'),
  ];

  /// Nama aplikasi, dipakai sebagai judul app
  ///
  /// In id, this message translates to:
  /// **'Inventra Mobile'**
  String get appTitle;

  /// Placeholder untuk fitur yang belum dibangun
  ///
  /// In id, this message translates to:
  /// **'Segera hadir'**
  String get commonComingSoon;

  /// Subjudul placeholder rute yang layarnya belum dibangun
  ///
  /// In id, this message translates to:
  /// **'Layar ini sedang dibangun dan akan tersedia pada pembaruan berikutnya.'**
  String get commonComingSoonBody;

  /// Label tombol coba ulang setelah error
  ///
  /// In id, this message translates to:
  /// **'Coba lagi'**
  String get commonRetry;

  /// Label tombol batal umum
  ///
  /// In id, this message translates to:
  /// **'Batal'**
  String get commonCancel;

  /// Teks default banner offline slim
  ///
  /// In id, this message translates to:
  /// **'Offline — scan tersimpan di perangkat'**
  String get commonOfflineBanner;

  /// Label SyncPill saat seluruh antrean tersinkron
  ///
  /// In id, this message translates to:
  /// **'Tersinkron'**
  String get commonSyncSynced;

  /// Label SyncPill saat masih ada antrean lokal
  ///
  /// In id, this message translates to:
  /// **'{count} belum tersinkron'**
  String commonSyncPending(int count);

  /// Label SyncPill saat sinkronisasi berjalan
  ///
  /// In id, this message translates to:
  /// **'Menyinkronkan…'**
  String get commonSyncSyncing;

  /// Label SyncPill saat sinkronisasi gagal
  ///
  /// In id, this message translates to:
  /// **'Gagal — coba lagi'**
  String get commonSyncFailed;

  /// Label SyncPill saat perangkat offline
  ///
  /// In id, this message translates to:
  /// **'Offline'**
  String get commonSyncOffline;

  /// Label tab bottom-nav beranda
  ///
  /// In id, this message translates to:
  /// **'Beranda'**
  String get shellTabHome;

  /// Label tab bottom-nav stock opname
  ///
  /// In id, this message translates to:
  /// **'Opname'**
  String get shellTabOpname;

  /// Label tombol pindai tengah bottom-nav
  ///
  /// In id, this message translates to:
  /// **'Pindai'**
  String get shellTabScan;

  /// Label tab bottom-nav approval inbox
  ///
  /// In id, this message translates to:
  /// **'Approval'**
  String get shellTabApproval;

  /// Label tab bottom-nav notifikasi
  ///
  /// In id, this message translates to:
  /// **'Notif'**
  String get shellTabNotifications;

  /// Judul layar feed notifikasi
  ///
  /// In id, this message translates to:
  /// **'Notifikasi'**
  String get notificationsTitle;

  /// Judul layar detail aset
  ///
  /// In id, this message translates to:
  /// **'Detail Aset'**
  String get assetDetailTitle;

  /// Judul overlay layar scan
  ///
  /// In id, this message translates to:
  /// **'Pindai Label Aset'**
  String get scanTitle;

  /// Pill petunjuk di bawah bingkai target scan
  ///
  /// In id, this message translates to:
  /// **'Arahkan ke barcode / QR pada label aset'**
  String get scanHint;

  /// Label tombol pembuka bottom sheet input tag manual
  ///
  /// In id, this message translates to:
  /// **'Ketik kode manual'**
  String get scanManualButton;

  /// Tooltip tombol tutup di layar scan
  ///
  /// In id, this message translates to:
  /// **'Tutup pemindai'**
  String get scanCloseTooltip;

  /// Tooltip toggle torch saat senter mati
  ///
  /// In id, this message translates to:
  /// **'Nyalakan senter'**
  String get scanTorchOnTooltip;

  /// Tooltip toggle torch saat senter menyala
  ///
  /// In id, this message translates to:
  /// **'Matikan senter'**
  String get scanTorchOffTooltip;

  /// Judul state kamera gagal (izin ditolak/emulator)
  ///
  /// In id, this message translates to:
  /// **'Kamera tidak tersedia'**
  String get scanCameraUnavailableTitle;

  /// Subjudul state kamera gagal, mengarahkan ke jalur manual
  ///
  /// In id, this message translates to:
  /// **'Izinkan akses kamera di pengaturan perangkat, atau gunakan input kode manual.'**
  String get scanCameraUnavailableBody;

  /// Judul bottom sheet input tag manual
  ///
  /// In id, this message translates to:
  /// **'Ketik kode manual'**
  String get scanManualSheetTitle;

  /// Label field kode aset pada sheet input manual
  ///
  /// In id, this message translates to:
  /// **'Kode aset'**
  String get scanManualFieldLabel;

  /// Placeholder field kode aset (contoh tag valid)
  ///
  /// In id, this message translates to:
  /// **'JKT01-ELK-2026-00001'**
  String get scanManualFieldHint;

  /// Teks bantuan format tag di bawah field kode aset
  ///
  /// In id, this message translates to:
  /// **'Format: KANTOR-KATEGORI-TAHUN-NOMOR'**
  String get scanManualFieldHelper;

  /// Label tombol submit pencarian tag manual
  ///
  /// In id, this message translates to:
  /// **'Cari'**
  String get scanManualSubmit;

  /// Keterangan placeholder saat aset tidak punya foto
  ///
  /// In id, this message translates to:
  /// **'Belum ada foto'**
  String get assetDetailPhotoPlaceholder;

  /// Judul seksi penempatan (kantor/ruangan/pemegang)
  ///
  /// In id, this message translates to:
  /// **'Penempatan'**
  String get assetDetailSectionPlacement;

  /// Judul seksi informasi umum aset
  ///
  /// In id, this message translates to:
  /// **'Informasi'**
  String get assetDetailSectionInfo;

  /// Judul seksi nilai finansial aset
  ///
  /// In id, this message translates to:
  /// **'Nilai'**
  String get assetDetailSectionValue;

  /// Label baris kantor pemilik aset
  ///
  /// In id, this message translates to:
  /// **'Kantor'**
  String get assetDetailFieldOffice;

  /// Label baris ruangan penempatan aset
  ///
  /// In id, this message translates to:
  /// **'Lantai / Ruangan'**
  String get assetDetailFieldRoom;

  /// Label baris pegawai pemegang aset
  ///
  /// In id, this message translates to:
  /// **'Pemegang saat ini'**
  String get assetDetailFieldHolder;

  /// Label baris kategori aset
  ///
  /// In id, this message translates to:
  /// **'Kategori'**
  String get assetDetailFieldCategory;

  /// Label baris brand dan model aset
  ///
  /// In id, this message translates to:
  /// **'Brand / Model'**
  String get assetDetailFieldBrandModel;

  /// Label baris nomor seri aset
  ///
  /// In id, this message translates to:
  /// **'No. seri'**
  String get assetDetailFieldSerial;

  /// Label baris tanggal pembelian aset
  ///
  /// In id, this message translates to:
  /// **'Tanggal beli'**
  String get assetDetailFieldPurchaseDate;

  /// Label baris vendor pengadaan aset
  ///
  /// In id, this message translates to:
  /// **'Vendor'**
  String get assetDetailFieldVendor;

  /// Label baris harga beli aset
  ///
  /// In id, this message translates to:
  /// **'Harga beli'**
  String get assetDetailFieldPurchaseCost;

  /// Label baris nilai buku aset
  ///
  /// In id, this message translates to:
  /// **'Nilai buku'**
  String get assetDetailFieldBookValue;

  /// Badge pada seksi yang sebagian fieldnya dimask field permission
  ///
  /// In id, this message translates to:
  /// **'Dibatasi untuk peran Anda'**
  String get assetDetailRestrictedBadge;

  /// Tooltip ikon gembok pada nilai yang dimask field permission
  ///
  /// In id, this message translates to:
  /// **'Field ini dibatasi untuk peran Anda'**
  String get assetDetailRestrictedTooltip;

  /// Label chip status aset available
  ///
  /// In id, this message translates to:
  /// **'Tersedia'**
  String get assetDetailStatusAvailable;

  /// Label chip status aset assigned
  ///
  /// In id, this message translates to:
  /// **'Dipinjam'**
  String get assetDetailStatusAssigned;

  /// Label chip status aset under_maintenance
  ///
  /// In id, this message translates to:
  /// **'Maintenance'**
  String get assetDetailStatusUnderMaintenance;

  /// Label chip status aset in_transfer
  ///
  /// In id, this message translates to:
  /// **'Dalam Mutasi'**
  String get assetDetailStatusInTransfer;

  /// Label chip status aset retired
  ///
  /// In id, this message translates to:
  /// **'Purna Pakai'**
  String get assetDetailStatusRetired;

  /// Label chip status aset disposed
  ///
  /// In id, this message translates to:
  /// **'Dilepas'**
  String get assetDetailStatusDisposed;

  /// Label chip status aset lost
  ///
  /// In id, this message translates to:
  /// **'Hilang'**
  String get assetDetailStatusLost;

  /// Judul empty state error umum detail aset
  ///
  /// In id, this message translates to:
  /// **'Gagal memuat detail aset'**
  String get assetDetailErrorTitle;

  /// Subjudul error detail aset saat offline/gangguan jaringan
  ///
  /// In id, this message translates to:
  /// **'Tidak ada koneksi. Periksa jaringan Anda lalu coba lagi.'**
  String get assetDetailErrorNetworkBody;

  /// Subjudul error detail aset untuk kegagalan lain
  ///
  /// In id, this message translates to:
  /// **'Terjadi kesalahan. Coba lagi.'**
  String get assetDetailErrorGenericBody;

  /// Judul empty state 403 detail aset
  ///
  /// In id, this message translates to:
  /// **'Akses dibatasi'**
  String get assetDetailForbiddenTitle;

  /// Subjudul empty state 403 detail aset
  ///
  /// In id, this message translates to:
  /// **'Peran Anda tidak memiliki izin melihat aset.'**
  String get assetDetailForbiddenBody;

  /// Judul empty state 404 detail aset
  ///
  /// In id, this message translates to:
  /// **'Kode tidak dikenal'**
  String get assetDetailNotFoundTitle;

  /// Subjudul empty state 404 detail aset dengan tag yang dicari
  ///
  /// In id, this message translates to:
  /// **'Kode {tag} tidak terdaftar, atau aset ini di luar wewenang Anda.'**
  String assetDetailNotFoundBody(String tag);

  /// Label aksi kembali memindai dari empty state 404
  ///
  /// In id, this message translates to:
  /// **'Pindai Lagi'**
  String get assetDetailScanAgain;

  /// Judul layar detail approval
  ///
  /// In id, this message translates to:
  /// **'Detail Approval'**
  String get approvalDetailTitle;

  /// Judul layar counting sesi opname
  ///
  /// In id, this message translates to:
  /// **'Detail Opname'**
  String get opnameDetailTitle;

  /// Judul layar variance sesi opname
  ///
  /// In id, this message translates to:
  /// **'Variance Opname'**
  String get opnameVarianceTitle;

  /// Judul layar profil dan sesi device
  ///
  /// In id, this message translates to:
  /// **'Profil'**
  String get accountTitle;

  /// Judul layar pengaturan
  ///
  /// In id, this message translates to:
  /// **'Pengaturan'**
  String get settingsTitle;

  /// Judul app bar tab beranda
  ///
  /// In id, this message translates to:
  /// **'Beranda'**
  String get homeTitle;

  /// Tooltip aksi logout sementara di app bar beranda
  ///
  /// In id, this message translates to:
  /// **'Keluar'**
  String get homeLogoutTooltip;

  /// Judul dialog konfirmasi logout
  ///
  /// In id, this message translates to:
  /// **'Keluar dari akun?'**
  String get homeLogoutConfirmTitle;

  /// Isi dialog konfirmasi logout
  ///
  /// In id, this message translates to:
  /// **'Sesi Anda di perangkat ini akan diakhiri.'**
  String get homeLogoutConfirmMessage;

  /// Label aksi utama dialog konfirmasi logout
  ///
  /// In id, this message translates to:
  /// **'Keluar'**
  String get homeLogoutConfirmAction;

  /// Wordmark produk pada layar login (tidak diterjemahkan)
  ///
  /// In id, this message translates to:
  /// **'Inventra'**
  String get loginBrandName;

  /// Badge pill di samping wordmark login
  ///
  /// In id, this message translates to:
  /// **'MOBILE'**
  String get loginBrandBadge;

  /// Tagline di bawah wordmark login
  ///
  /// In id, this message translates to:
  /// **'Pendamping lapangan manajemen aset'**
  String get loginTagline;

  /// Judul card form login
  ///
  /// In id, this message translates to:
  /// **'Masuk'**
  String get loginCardTitle;

  /// Subjudul card form login
  ///
  /// In id, this message translates to:
  /// **'Gunakan akun Inventra Anda'**
  String get loginCardSubtitle;

  /// Label field email login
  ///
  /// In id, this message translates to:
  /// **'Email'**
  String get loginEmailLabel;

  /// Placeholder field email login
  ///
  /// In id, this message translates to:
  /// **'nama@bank.co.id'**
  String get loginEmailHint;

  /// Label field kata sandi login
  ///
  /// In id, this message translates to:
  /// **'Kata sandi'**
  String get loginPasswordLabel;

  /// Placeholder field kata sandi login
  ///
  /// In id, this message translates to:
  /// **'Masukkan kata sandi'**
  String get loginPasswordHint;

  /// Tooltip toggle visibilitas kata sandi (sembunyi -> tampil)
  ///
  /// In id, this message translates to:
  /// **'Tampilkan kata sandi'**
  String get loginShowPassword;

  /// Tooltip toggle visibilitas kata sandi (tampil -> sembunyi)
  ///
  /// In id, this message translates to:
  /// **'Sembunyikan kata sandi'**
  String get loginHidePassword;

  /// Label tombol submit login
  ///
  /// In id, this message translates to:
  /// **'Masuk'**
  String get loginSubmitButton;

  /// Label tombol submit login saat memproses
  ///
  /// In id, this message translates to:
  /// **'Memproses…'**
  String get loginSubmitLoading;

  /// Pesan banner error login untuk kredensial salah
  ///
  /// In id, this message translates to:
  /// **'Email atau kata sandi salah. Coba lagi.'**
  String get loginErrorInvalidCredentials;

  /// Pesan banner error login saat offline/gangguan jaringan
  ///
  /// In id, this message translates to:
  /// **'Tidak ada koneksi. Periksa jaringan Anda lalu coba lagi.'**
  String get loginErrorNetwork;

  /// Pesan banner error login saat kena rate limit
  ///
  /// In id, this message translates to:
  /// **'Terlalu banyak percobaan. Coba lagi beberapa saat lagi.'**
  String get loginErrorRateLimited;

  /// Pesan banner error login untuk kegagalan lain
  ///
  /// In id, this message translates to:
  /// **'Terjadi kesalahan. Coba lagi.'**
  String get loginErrorGeneric;

  /// Label segmen bahasa Indonesia pada pill switch bahasa
  ///
  /// In id, this message translates to:
  /// **'ID'**
  String get loginLanguageIndonesian;

  /// Label segmen bahasa Inggris pada pill switch bahasa
  ///
  /// In id, this message translates to:
  /// **'EN'**
  String get loginLanguageEnglish;

  /// Teks versi aplikasi di footer login
  ///
  /// In id, this message translates to:
  /// **'Inventra Mobile v{version} · Build {build}'**
  String loginVersion(String version, String build);
}

class _AppLocalizationsDelegate
    extends LocalizationsDelegate<AppLocalizations> {
  const _AppLocalizationsDelegate();

  @override
  Future<AppLocalizations> load(Locale locale) {
    return SynchronousFuture<AppLocalizations>(lookupAppLocalizations(locale));
  }

  @override
  bool isSupported(Locale locale) =>
      <String>['en', 'id'].contains(locale.languageCode);

  @override
  bool shouldReload(_AppLocalizationsDelegate old) => false;
}

AppLocalizations lookupAppLocalizations(Locale locale) {
  // Lookup logic when only language code is specified.
  switch (locale.languageCode) {
    case 'en':
      return AppLocalizationsEn();
    case 'id':
      return AppLocalizationsId();
  }

  throw FlutterError(
    'AppLocalizations.delegate failed to load unsupported locale "$locale". This is likely '
    'an issue with the localizations generation tool. Please file an issue '
    'on GitHub with a reproducible sample app and the gen-l10n configuration '
    'that was used.',
  );
}
