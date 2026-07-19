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

  /// Label entri Pengaturan di app bar layar Profil
  ///
  /// In id, this message translates to:
  /// **'Pengaturan'**
  String get accountSettingsButton;

  /// Catatan kartu identitas: data diri diubah lewat web
  ///
  /// In id, this message translates to:
  /// **'Penyuntingan profil dilakukan dari aplikasi web'**
  String get accountEditOnWeb;

  /// Judul kartu daftar sesi device aktif
  ///
  /// In id, this message translates to:
  /// **'Sesi Perangkat'**
  String get accountSessionsTitle;

  /// Badge penanda sesi yang sedang dipakai
  ///
  /// In id, this message translates to:
  /// **'Perangkat ini'**
  String get accountSessionCurrentBadge;

  /// Label waktu sesi ini (menggantikan last_seen relatif)
  ///
  /// In id, this message translates to:
  /// **'aktif sekarang'**
  String get accountSessionActiveNow;

  /// Label tombol cabut satu sesi lain
  ///
  /// In id, this message translates to:
  /// **'Cabut'**
  String get accountSessionRevoke;

  /// Judul dialog konfirmasi cabut satu sesi
  ///
  /// In id, this message translates to:
  /// **'Cabut sesi ini?'**
  String get accountSessionRevokeConfirmTitle;

  /// Isi dialog konfirmasi cabut satu sesi
  ///
  /// In id, this message translates to:
  /// **'{name} akan keluar dan harus masuk kembali.'**
  String accountSessionRevokeConfirmBody(String name);

  /// Label aksi utama dialog cabut satu sesi
  ///
  /// In id, this message translates to:
  /// **'Ya, Cabut'**
  String get accountSessionRevokeConfirmAction;

  /// Snackbar setelah satu sesi berhasil dicabut
  ///
  /// In id, this message translates to:
  /// **'Sesi {name} dicabut'**
  String accountSessionRevokedSnack(String name);

  /// Snackbar saat server menolak pencabutan satu sesi
  ///
  /// In id, this message translates to:
  /// **'Gagal mencabut sesi. Coba lagi.'**
  String get accountSessionRevokeFailed;

  /// Label tombol cabut semua sesi lain sekaligus
  ///
  /// In id, this message translates to:
  /// **'Keluar dari semua perangkat lain'**
  String get accountRevokeOthers;

  /// Judul dialog konfirmasi cabut semua sesi lain
  ///
  /// In id, this message translates to:
  /// **'Keluar dari semua perangkat lain?'**
  String get accountRevokeOthersConfirmTitle;

  /// Isi dialog konfirmasi cabut semua sesi lain
  ///
  /// In id, this message translates to:
  /// **'{count} sesi lain akan dicabut. Perangkat ini tetap masuk.'**
  String accountRevokeOthersConfirmBody(int count);

  /// Label aksi utama dialog cabut semua sesi lain
  ///
  /// In id, this message translates to:
  /// **'Ya, Keluar'**
  String get accountRevokeOthersConfirmAction;

  /// Snackbar saat server menolak cabut semua sesi lain
  ///
  /// In id, this message translates to:
  /// **'Gagal mencabut sesi lain. Coba lagi.'**
  String get accountRevokeOthersFailed;

  /// Isi kartu sesi saat daftar dari server kosong
  ///
  /// In id, this message translates to:
  /// **'Belum ada sesi aktif yang tercatat.'**
  String get accountSessionsEmpty;

  /// Pesan error kartu sesi (dengan tombol coba lagi)
  ///
  /// In id, this message translates to:
  /// **'Gagal memuat sesi perangkat.'**
  String get accountSessionsErrorBody;

  /// Label tombol logout layar Profil
  ///
  /// In id, this message translates to:
  /// **'Keluar'**
  String get accountLogout;

  /// Judul dialog konfirmasi logout
  ///
  /// In id, this message translates to:
  /// **'Keluar dari akun?'**
  String get accountLogoutConfirmTitle;

  /// Isi dialog konfirmasi logout
  ///
  /// In id, this message translates to:
  /// **'Sesi Anda di perangkat ini akan diakhiri.'**
  String get accountLogoutConfirmBody;

  /// Label aksi utama dialog konfirmasi logout
  ///
  /// In id, this message translates to:
  /// **'Ya, Keluar'**
  String get accountLogoutConfirmAction;

  /// last_seen sesi < 1 menit lalu
  ///
  /// In id, this message translates to:
  /// **'baru saja'**
  String get accountTimeJustNow;

  /// last_seen sesi dalam menit
  ///
  /// In id, this message translates to:
  /// **'{count} mnt lalu'**
  String accountTimeMinutesAgo(int count);

  /// last_seen sesi dalam jam
  ///
  /// In id, this message translates to:
  /// **'{count} jam lalu'**
  String accountTimeHoursAgo(int count);

  /// last_seen sesi 24-48 jam lalu
  ///
  /// In id, this message translates to:
  /// **'kemarin'**
  String get accountTimeYesterday;

  /// last_seen sesi dalam hari (< 7 hari)
  ///
  /// In id, this message translates to:
  /// **'{count} hari lalu'**
  String accountTimeDaysAgo(int count);

  /// Judul layar pengaturan
  ///
  /// In id, this message translates to:
  /// **'Pengaturan'**
  String get settingsTitle;

  /// Judul kartu seksi tampilan (tema + bahasa)
  ///
  /// In id, this message translates to:
  /// **'Tampilan'**
  String get settingsSectionAppearance;

  /// Judul baris pengaturan tema
  ///
  /// In id, this message translates to:
  /// **'Tema'**
  String get settingsTheme;

  /// Opsi tema terang
  ///
  /// In id, this message translates to:
  /// **'Terang'**
  String get settingsThemeLight;

  /// Opsi tema gelap
  ///
  /// In id, this message translates to:
  /// **'Gelap'**
  String get settingsThemeDark;

  /// Opsi tema mengikuti sistem
  ///
  /// In id, this message translates to:
  /// **'Ikuti Sistem'**
  String get settingsThemeSystem;

  /// Judul bottom sheet pemilih tema
  ///
  /// In id, this message translates to:
  /// **'Pilih tema'**
  String get settingsThemeSheetTitle;

  /// Label tombol terapkan pada sheet pemilih tema
  ///
  /// In id, this message translates to:
  /// **'Terapkan'**
  String get settingsThemeApply;

  /// Judul baris pengaturan bahasa
  ///
  /// In id, this message translates to:
  /// **'Bahasa'**
  String get settingsLanguage;

  /// Judul bottom sheet pemilih bahasa
  ///
  /// In id, this message translates to:
  /// **'Pilih bahasa'**
  String get settingsLanguageSheetTitle;

  /// Nama bahasa Indonesia (ditulis dalam bahasanya sendiri)
  ///
  /// In id, this message translates to:
  /// **'Indonesia'**
  String get settingsLanguageIndonesian;

  /// Nama bahasa Inggris (ditulis dalam bahasanya sendiri)
  ///
  /// In id, this message translates to:
  /// **'English'**
  String get settingsLanguageEnglish;

  /// Judul kartu seksi tentang aplikasi
  ///
  /// In id, this message translates to:
  /// **'Tentang'**
  String get settingsSectionAbout;

  /// Nama aplikasi pada baris tentang (tidak diterjemahkan)
  ///
  /// In id, this message translates to:
  /// **'Inventra Mobile'**
  String get settingsAppName;

  /// Label versi aplikasi pada kartu tentang
  ///
  /// In id, this message translates to:
  /// **'Versi {version} (build {build})'**
  String settingsVersion(String version, String build);

  /// Judul app bar tab beranda
  ///
  /// In id, this message translates to:
  /// **'Beranda'**
  String get homeTitle;

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

  /// Judul layar inbox approval
  ///
  /// In id, this message translates to:
  /// **'Approval'**
  String get approvalInboxTitle;

  /// Label chip filter status pending
  ///
  /// In id, this message translates to:
  /// **'Menunggu'**
  String get approvalInboxFilterPending;

  /// Label chip filter status approved
  ///
  /// In id, this message translates to:
  /// **'Disetujui'**
  String get approvalInboxFilterApproved;

  /// Label chip filter status rejected
  ///
  /// In id, this message translates to:
  /// **'Ditolak'**
  String get approvalInboxFilterRejected;

  /// Label chip filter tanpa status (semua pengajuan)
  ///
  /// In id, this message translates to:
  /// **'Semua'**
  String get approvalInboxFilterAll;

  /// Petunjuk pull-to-refresh di atas daftar inbox
  ///
  /// In id, this message translates to:
  /// **'Tarik untuk menyegarkan'**
  String get approvalInboxPullToRefresh;

  /// Judul empty state filter Menunggu
  ///
  /// In id, this message translates to:
  /// **'Tidak ada pengajuan menunggu'**
  String get approvalInboxEmptyPendingTitle;

  /// Subjudul empty state filter Menunggu
  ///
  /// In id, this message translates to:
  /// **'Semua pengajuan dalam lingkup Anda sudah diputus. Kerja bagus!'**
  String get approvalInboxEmptyPendingBody;

  /// Aksi empty state Menunggu: pindah ke filter Semua
  ///
  /// In id, this message translates to:
  /// **'Lihat riwayat'**
  String get approvalInboxEmptyPendingAction;

  /// Judul empty state filter selain Menunggu
  ///
  /// In id, this message translates to:
  /// **'Tidak ada pengajuan'**
  String get approvalInboxEmptyFilteredTitle;

  /// Subjudul empty state filter selain Menunggu
  ///
  /// In id, this message translates to:
  /// **'Belum ada pengajuan dengan status ini di lingkup Anda.'**
  String get approvalInboxEmptyFilteredBody;

  /// Judul empty state error inbox approval
  ///
  /// In id, this message translates to:
  /// **'Gagal memuat pengajuan'**
  String get approvalInboxErrorTitle;

  /// Subjudul error inbox saat offline/gangguan jaringan
  ///
  /// In id, this message translates to:
  /// **'Tidak ada koneksi. Periksa jaringan Anda lalu coba lagi.'**
  String get approvalInboxErrorNetworkBody;

  /// Subjudul error inbox untuk kegagalan lain
  ///
  /// In id, this message translates to:
  /// **'Terjadi kesalahan. Coba lagi.'**
  String get approvalInboxErrorGenericBody;

  /// Judul empty state 403 inbox approval
  ///
  /// In id, this message translates to:
  /// **'Akses dibatasi'**
  String get approvalInboxForbiddenTitle;

  /// Subjudul empty state 403 inbox approval
  ///
  /// In id, this message translates to:
  /// **'Peran Anda tidak memiliki izin melihat pengajuan.'**
  String get approvalInboxForbiddenBody;

  /// Teks baris kaki daftar saat muat halaman berikutnya gagal
  ///
  /// In id, this message translates to:
  /// **'Gagal memuat halaman berikutnya'**
  String get approvalInboxLoadMoreFailed;

  /// Penanda jenis pengajuan sensitif pada kartu dan header detail
  ///
  /// In id, this message translates to:
  /// **'sensitif'**
  String get approvalCardSensitive;

  /// Label jenis pengajuan asset_create
  ///
  /// In id, this message translates to:
  /// **'Registrasi Aset'**
  String get approvalTypeAssetCreate;

  /// Label jenis pengajuan asset_disposal
  ///
  /// In id, this message translates to:
  /// **'Penghapusan'**
  String get approvalTypeAssetDisposal;

  /// Label jenis pengajuan asset_transfer
  ///
  /// In id, this message translates to:
  /// **'Mutasi'**
  String get approvalTypeAssetTransfer;

  /// Label jenis pengajuan assignment
  ///
  /// In id, this message translates to:
  /// **'Peminjaman'**
  String get approvalTypeAssignment;

  /// Label jenis pengajuan maintenance
  ///
  /// In id, this message translates to:
  /// **'Perbaikan'**
  String get approvalTypeMaintenance;

  /// Label jenis pengajuan valuation_exclusion
  ///
  /// In id, this message translates to:
  /// **'Pengecualian Valuasi'**
  String get approvalTypeValuationExclusion;

  /// Label chip status pengajuan pending
  ///
  /// In id, this message translates to:
  /// **'Menunggu'**
  String get approvalStatusPending;

  /// Label chip status pengajuan approved
  ///
  /// In id, this message translates to:
  /// **'Disetujui'**
  String get approvalStatusApproved;

  /// Label chip status pengajuan rejected
  ///
  /// In id, this message translates to:
  /// **'Ditolak'**
  String get approvalStatusRejected;

  /// Label chip status pengajuan cancelled
  ///
  /// In id, this message translates to:
  /// **'Dibatalkan'**
  String get approvalStatusCancelled;

  /// Waktu relatif di bawah satu menit
  ///
  /// In id, this message translates to:
  /// **'baru saja'**
  String get approvalTimeJustNow;

  /// Waktu relatif dalam menit
  ///
  /// In id, this message translates to:
  /// **'{count} mnt lalu'**
  String approvalTimeMinutesAgo(int count);

  /// Waktu relatif dalam jam
  ///
  /// In id, this message translates to:
  /// **'{count} jam lalu'**
  String approvalTimeHoursAgo(int count);

  /// Waktu relatif 24-48 jam lalu
  ///
  /// In id, this message translates to:
  /// **'kemarin'**
  String get approvalTimeYesterday;

  /// Waktu relatif dalam hari (di bawah seminggu)
  ///
  /// In id, this message translates to:
  /// **'{count} hari lalu'**
  String approvalTimeDaysAgo(int count);

  /// Banner peringatan jenis pengajuan sensitif di detail
  ///
  /// In id, this message translates to:
  /// **'Tindakan sensitif — periksa saksama sebelum memutus'**
  String get approvalDetailSensitiveBanner;

  /// Judul card data payload pengajuan
  ///
  /// In id, this message translates to:
  /// **'Data yang diajukan'**
  String get approvalDetailSectionData;

  /// Judul card timeline jenjang persetujuan
  ///
  /// In id, this message translates to:
  /// **'Jenjang persetujuan'**
  String get approvalDetailSectionSteps;

  /// Label baris aset target pengajuan
  ///
  /// In id, this message translates to:
  /// **'Aset'**
  String get approvalDetailFieldAsset;

  /// Label baris amount pengajuan
  ///
  /// In id, this message translates to:
  /// **'Nilai pengajuan'**
  String get approvalDetailFieldAmount;

  /// Label baris alasan pengajuan
  ///
  /// In id, this message translates to:
  /// **'Alasan'**
  String get approvalDetailFieldReason;

  /// Label baris nama aset payload registrasi
  ///
  /// In id, this message translates to:
  /// **'Nama aset'**
  String get approvalDetailFieldName;

  /// Label baris kategori payload registrasi
  ///
  /// In id, this message translates to:
  /// **'Kategori'**
  String get approvalDetailFieldCategory;

  /// Label baris kantor payload registrasi
  ///
  /// In id, this message translates to:
  /// **'Kantor'**
  String get approvalDetailFieldOffice;

  /// Label baris ruangan payload
  ///
  /// In id, this message translates to:
  /// **'Ruangan'**
  String get approvalDetailFieldRoom;

  /// Label baris perubahan kantor pada payload mutasi
  ///
  /// In id, this message translates to:
  /// **'Kantor penempatan'**
  String get approvalDetailFieldOfficeChange;

  /// Label baris kelas aset payload registrasi
  ///
  /// In id, this message translates to:
  /// **'Kelas aset'**
  String get approvalDetailFieldAssetClass;

  /// Nilai kelas aset tangible
  ///
  /// In id, this message translates to:
  /// **'Berwujud'**
  String get approvalDetailAssetClassTangible;

  /// Nilai kelas aset intangible
  ///
  /// In id, this message translates to:
  /// **'Tak berwujud'**
  String get approvalDetailAssetClassIntangible;

  /// Label baris harga beli payload registrasi
  ///
  /// In id, this message translates to:
  /// **'Harga beli'**
  String get approvalDetailFieldPurchaseCost;

  /// Label baris tanggal beli payload registrasi
  ///
  /// In id, this message translates to:
  /// **'Tanggal beli'**
  String get approvalDetailFieldPurchaseDate;

  /// Label baris nomor seri payload registrasi
  ///
  /// In id, this message translates to:
  /// **'No. seri'**
  String get approvalDetailFieldSerial;

  /// Label baris brand dan model payload registrasi
  ///
  /// In id, this message translates to:
  /// **'Brand / Model'**
  String get approvalDetailFieldBrandModel;

  /// Label baris vendor payload registrasi
  ///
  /// In id, this message translates to:
  /// **'Vendor'**
  String get approvalDetailFieldVendor;

  /// Label baris nomor PO payload registrasi
  ///
  /// In id, this message translates to:
  /// **'No. PO'**
  String get approvalDetailFieldPoNumber;

  /// Label baris sumber dana payload registrasi
  ///
  /// In id, this message translates to:
  /// **'Sumber dana'**
  String get approvalDetailFieldFundingSource;

  /// Label baris akhir garansi payload registrasi
  ///
  /// In id, this message translates to:
  /// **'Akhir garansi'**
  String get approvalDetailFieldWarrantyExpiry;

  /// Label baris catatan payload registrasi
  ///
  /// In id, this message translates to:
  /// **'Catatan'**
  String get approvalDetailFieldNotes;

  /// Label baris metode payload penghapusan
  ///
  /// In id, this message translates to:
  /// **'Metode pelepasan'**
  String get approvalDetailFieldMethod;

  /// Nilai metode pelepasan sale
  ///
  /// In id, this message translates to:
  /// **'Penjualan'**
  String get approvalDetailMethodSale;

  /// Nilai metode pelepasan auction
  ///
  /// In id, this message translates to:
  /// **'Lelang'**
  String get approvalDetailMethodAuction;

  /// Nilai metode pelepasan donation
  ///
  /// In id, this message translates to:
  /// **'Hibah'**
  String get approvalDetailMethodDonation;

  /// Nilai metode pelepasan write_off
  ///
  /// In id, this message translates to:
  /// **'Penghapusbukuan'**
  String get approvalDetailMethodWriteOff;

  /// Label baris tanggal pelepasan payload penghapusan
  ///
  /// In id, this message translates to:
  /// **'Tanggal pelepasan'**
  String get approvalDetailFieldDisposalDate;

  /// Label baris nilai jual payload penghapusan
  ///
  /// In id, this message translates to:
  /// **'Nilai jual'**
  String get approvalDetailFieldProceeds;

  /// Label baris nilai buku payload penghapusan
  ///
  /// In id, this message translates to:
  /// **'Nilai buku'**
  String get approvalDetailFieldBookValue;

  /// Label baris nomor BAST payload penghapusan
  ///
  /// In id, this message translates to:
  /// **'No. BAST'**
  String get approvalDetailFieldBastNo;

  /// Label baris kondisi kirim payload mutasi
  ///
  /// In id, this message translates to:
  /// **'Kondisi saat kirim'**
  String get approvalDetailFieldConditionSent;

  /// Label baris tanggal mutasi payload mutasi
  ///
  /// In id, this message translates to:
  /// **'Tanggal mutasi'**
  String get approvalDetailFieldTransferDate;

  /// Penanda payload/amount yang dimask field permission
  ///
  /// In id, this message translates to:
  /// **'Dibatasi untuk peran Anda'**
  String get approvalDetailRestrictedData;

  /// Label peran maker pada timeline jenjang
  ///
  /// In id, this message translates to:
  /// **'Maker'**
  String get approvalDetailStepMaker;

  /// Status baris maker pada timeline
  ///
  /// In id, this message translates to:
  /// **'Mengajukan · {date}'**
  String approvalDetailStepSubmitted(String date);

  /// Status tahap yang sudah disetujui
  ///
  /// In id, this message translates to:
  /// **'Disetujui · {date}'**
  String approvalDetailStepApproved(String date);

  /// Status tahap yang ditolak
  ///
  /// In id, this message translates to:
  /// **'Ditolak · {date}'**
  String approvalDetailStepRejected(String date);

  /// Status tahap aktif yang menunggu keputusan
  ///
  /// In id, this message translates to:
  /// **'Menunggu keputusan'**
  String get approvalDetailStepWaiting;

  /// Status tahap yang belum aktif
  ///
  /// In id, this message translates to:
  /// **'Berikutnya'**
  String get approvalDetailStepUpcoming;

  /// Label required_level office
  ///
  /// In id, this message translates to:
  /// **'Approver kantor'**
  String get approvalDetailLevelOffice;

  /// Label required_level office_subtree
  ///
  /// In id, this message translates to:
  /// **'Approver kantor & jajaran'**
  String get approvalDetailLevelOfficeSubtree;

  /// Label required_level wilayah
  ///
  /// In id, this message translates to:
  /// **'Approver kanwil'**
  String get approvalDetailLevelWilayah;

  /// Label required_level pusat
  ///
  /// In id, this message translates to:
  /// **'Approver pusat'**
  String get approvalDetailLevelPusat;

  /// Placeholder field catatan keputusan
  ///
  /// In id, this message translates to:
  /// **'Tambahkan catatan (opsional)'**
  String get approvalDetailNoteHint;

  /// Label tombol setujui
  ///
  /// In id, this message translates to:
  /// **'Setujui'**
  String get approvalDetailApprove;

  /// Label tombol tolak
  ///
  /// In id, this message translates to:
  /// **'Tolak'**
  String get approvalDetailReject;

  /// Judul dialog konfirmasi setujui
  ///
  /// In id, this message translates to:
  /// **'Setujui pengajuan ini?'**
  String get approvalDetailApproveConfirmTitle;

  /// Isi dialog konfirmasi setujui
  ///
  /// In id, this message translates to:
  /// **'{title} dari {maker} akan disetujui dan lanjut ke tahap berikutnya.'**
  String approvalDetailApproveConfirmBody(String title, String maker);

  /// Label aksi utama dialog konfirmasi setujui
  ///
  /// In id, this message translates to:
  /// **'Ya, Setujui'**
  String get approvalDetailApproveConfirmAction;

  /// Judul dialog konfirmasi tolak
  ///
  /// In id, this message translates to:
  /// **'Tolak pengajuan ini?'**
  String get approvalDetailRejectConfirmTitle;

  /// Isi dialog konfirmasi tolak
  ///
  /// In id, this message translates to:
  /// **'{title} dari {maker} akan ditolak dan dikembalikan ke maker.'**
  String approvalDetailRejectConfirmBody(String title, String maker);

  /// Label aksi utama dialog konfirmasi tolak
  ///
  /// In id, this message translates to:
  /// **'Ya, Tolak'**
  String get approvalDetailRejectConfirmAction;

  /// Label kutipan catatan pada dialog konfirmasi tolak
  ///
  /// In id, this message translates to:
  /// **'Catatan Anda'**
  String get approvalDetailYourNote;

  /// SnackBar sukses setelah menyetujui
  ///
  /// In id, this message translates to:
  /// **'Pengajuan disetujui'**
  String get approvalDetailApprovedSnack;

  /// SnackBar sukses setelah menolak
  ///
  /// In id, this message translates to:
  /// **'Pengajuan ditolak'**
  String get approvalDetailRejectedSnack;

  /// Banner kaki detail untuk pengajuan approved
  ///
  /// In id, this message translates to:
  /// **'Pengajuan telah disetujui'**
  String get approvalDetailDecidedApproved;

  /// Banner kaki detail bila pemutus akhirnya pengguna sendiri
  ///
  /// In id, this message translates to:
  /// **'Anda telah menyetujui pengajuan ini'**
  String get approvalDetailDecidedByYouApproved;

  /// Banner kaki detail untuk pengajuan rejected
  ///
  /// In id, this message translates to:
  /// **'Pengajuan telah ditolak'**
  String get approvalDetailDecidedRejected;

  /// Banner kaki detail bila penolaknya pengguna sendiri
  ///
  /// In id, this message translates to:
  /// **'Anda telah menolak pengajuan ini'**
  String get approvalDetailDecidedByYouRejected;

  /// Banner kaki detail untuk pengajuan cancelled
  ///
  /// In id, this message translates to:
  /// **'Pengajuan dibatalkan oleh maker'**
  String get approvalDetailDecidedCancelled;

  /// Banner SoD pengganti aksi saat pengguna adalah maker
  ///
  /// In id, this message translates to:
  /// **'Ini pengajuan Anda — keputusan menunggu approver lain (maker tidak boleh memutus pengajuannya sendiri).'**
  String get approvalDetailSodOwnRequest;

  /// Pesan 403 SoD saat approve/reject ditolak server
  ///
  /// In id, this message translates to:
  /// **'Anda tidak berwenang memutus pengajuan ini — maker atau approver sebelumnya tidak boleh memutus pengajuannya sendiri.'**
  String get approvalDetailErrorSod;

  /// Pesan 409 saat pengajuan sudah diputus/berubah
  ///
  /// In id, this message translates to:
  /// **'Pengajuan sudah berubah status di tempat lain. Memuat ulang…'**
  String get approvalDetailErrorConflict;

  /// Pesan gagal approve/reject saat offline
  ///
  /// In id, this message translates to:
  /// **'Tidak ada koneksi. Periksa jaringan Anda lalu coba lagi.'**
  String get approvalDetailErrorNetwork;

  /// Pesan gagal approve/reject untuk kegagalan lain
  ///
  /// In id, this message translates to:
  /// **'Terjadi kesalahan. Coba lagi.'**
  String get approvalDetailErrorGeneric;

  /// Judul empty state error detail approval
  ///
  /// In id, this message translates to:
  /// **'Gagal memuat pengajuan'**
  String get approvalDetailErrorTitle;

  /// Judul empty state 404 detail approval
  ///
  /// In id, this message translates to:
  /// **'Pengajuan tidak ditemukan'**
  String get approvalDetailNotFoundTitle;

  /// Subjudul empty state 404 detail approval
  ///
  /// In id, this message translates to:
  /// **'Pengajuan tidak ada atau di luar lingkup Anda.'**
  String get approvalDetailNotFoundBody;

  /// Judul empty state 403 detail approval
  ///
  /// In id, this message translates to:
  /// **'Akses dibatasi'**
  String get approvalDetailForbiddenTitle;

  /// Subjudul empty state 403 detail approval
  ///
  /// In id, this message translates to:
  /// **'Peran Anda tidak memiliki izin melihat pengajuan ini.'**
  String get approvalDetailForbiddenBody;

  /// Judul layar daftar sesi opname
  ///
  /// In id, this message translates to:
  /// **'Stock Opname'**
  String get opnameSessionsTitle;

  /// Label chip filter sesi yang belum selesai (open/counting/reconciling)
  ///
  /// In id, this message translates to:
  /// **'Berjalan'**
  String get opnameSessionsFilterRunning;

  /// Label chip filter sesi berstatus closed
  ///
  /// In id, this message translates to:
  /// **'Selesai'**
  String get opnameSessionsFilterClosed;

  /// Label chip filter tanpa saringan status
  ///
  /// In id, this message translates to:
  /// **'Semua'**
  String get opnameSessionsFilterAll;

  /// Baris progress kartu sesi: jumlah item terhitung dari total
  ///
  /// In id, this message translates to:
  /// **'{counted} dari {total} tercocokkan'**
  String opnameSessionsProgress(int counted, int total);

  /// Label tombol CTA kartu sesi yang belum selesai
  ///
  /// In id, this message translates to:
  /// **'Lanjutkan Menghitung'**
  String get opnameSessionsContinue;

  /// Chip info kartu sesi selesai: Berita Acara diakses dari web
  ///
  /// In id, this message translates to:
  /// **'Berita Acara di web'**
  String get opnameSessionsReportOnWeb;

  /// Catatan kaki daftar sesi opname
  ///
  /// In id, this message translates to:
  /// **'Sesi dibuat dan diselesaikan dari aplikasi web'**
  String get opnameSessionsFootnote;

  /// Judul empty state filter Berjalan
  ///
  /// In id, this message translates to:
  /// **'Tidak ada sesi opname aktif'**
  String get opnameSessionsEmptyTitle;

  /// Subjudul empty state filter Berjalan
  ///
  /// In id, this message translates to:
  /// **'Sesi baru dibuat oleh admin dari aplikasi web. Anda akan diberi tahu bila ditugaskan.'**
  String get opnameSessionsEmptyBody;

  /// Judul empty state filter Selesai/Semua
  ///
  /// In id, this message translates to:
  /// **'Tidak ada sesi'**
  String get opnameSessionsEmptyFilteredTitle;

  /// Subjudul empty state filter Selesai/Semua
  ///
  /// In id, this message translates to:
  /// **'Belum ada sesi opname dengan status ini di lingkup Anda.'**
  String get opnameSessionsEmptyFilteredBody;

  /// Judul empty state error daftar sesi opname
  ///
  /// In id, this message translates to:
  /// **'Gagal memuat sesi opname'**
  String get opnameSessionsErrorTitle;

  /// Subjudul error opname saat offline/gangguan jaringan
  ///
  /// In id, this message translates to:
  /// **'Tidak ada koneksi. Periksa jaringan Anda lalu coba lagi.'**
  String get opnameErrorNetworkBody;

  /// Subjudul error opname untuk kegagalan lain
  ///
  /// In id, this message translates to:
  /// **'Terjadi kesalahan. Coba lagi.'**
  String get opnameErrorGenericBody;

  /// Judul empty state 403 modul opname
  ///
  /// In id, this message translates to:
  /// **'Akses dibatasi'**
  String get opnameForbiddenTitle;

  /// Subjudul empty state 403 modul opname
  ///
  /// In id, this message translates to:
  /// **'Peran Anda tidak memiliki izin melihat stock opname.'**
  String get opnameForbiddenBody;

  /// Label chip status sesi open
  ///
  /// In id, this message translates to:
  /// **'Terjadwal'**
  String get opnameStatusOpen;

  /// Label chip status sesi counting
  ///
  /// In id, this message translates to:
  /// **'Berjalan'**
  String get opnameStatusCounting;

  /// Label chip status sesi reconciling
  ///
  /// In id, this message translates to:
  /// **'Rekonsiliasi'**
  String get opnameStatusReconciling;

  /// Label chip status sesi closed
  ///
  /// In id, this message translates to:
  /// **'Selesai'**
  String get opnameStatusClosed;

  /// Banner offline layar opname (fase M0 online-only; drift/antrean menyusul M5)
  ///
  /// In id, this message translates to:
  /// **'Offline — pemindaian dinonaktifkan. Mode offline hadir di fase berikutnya.'**
  String get opnameOfflineBanner;

  /// Label tombol utama scan pada layar counting
  ///
  /// In id, this message translates to:
  /// **'Pindai Aset Berikutnya'**
  String get opnameCountingScanButton;

  /// Label tombol pembuka sheet input tag manual pada counting
  ///
  /// In id, this message translates to:
  /// **'Ketik kode'**
  String get opnameCountingManualButton;

  /// Judul seksi daftar item yang baru dihitung
  ///
  /// In id, this message translates to:
  /// **'Baru saja dipindai'**
  String get opnameCountingRecentHeader;

  /// Teks saat belum ada item terhitung pada sesi
  ///
  /// In id, this message translates to:
  /// **'Belum ada aset yang dipindai.'**
  String get opnameCountingRecentEmpty;

  /// Pembagi total pada ring progress counting
  ///
  /// In id, this message translates to:
  /// **'/{total}'**
  String opnameCountingRingTotal(int total);

  /// Tooltip aksi app bar counting menuju layar variance
  ///
  /// In id, this message translates to:
  /// **'Lihat variance'**
  String get opnameCountingVarianceTooltip;

  /// Judul empty state error detail sesi (counting/variance)
  ///
  /// In id, this message translates to:
  /// **'Gagal memuat sesi opname'**
  String get opnameDetailErrorTitle;

  /// Judul empty state 404 detail sesi opname
  ///
  /// In id, this message translates to:
  /// **'Sesi tidak ditemukan'**
  String get opnameDetailNotFoundTitle;

  /// Subjudul empty state 404 detail sesi opname
  ///
  /// In id, this message translates to:
  /// **'Sesi tidak ada atau di luar lingkup Anda.'**
  String get opnameDetailNotFoundBody;

  /// Label hasil item found
  ///
  /// In id, this message translates to:
  /// **'Ditemukan'**
  String get opnameResultFound;

  /// Label hasil item not_found
  ///
  /// In id, this message translates to:
  /// **'Tidak Ditemukan'**
  String get opnameResultNotFound;

  /// Label hasil item damaged
  ///
  /// In id, this message translates to:
  /// **'Rusak'**
  String get opnameResultDamaged;

  /// Label hasil item misplaced
  ///
  /// In id, this message translates to:
  /// **'Salah Lokasi'**
  String get opnameResultMisplaced;

  /// Label hasil item pending (belum dihitung)
  ///
  /// In id, this message translates to:
  /// **'Belum dihitung'**
  String get opnameResultPending;

  /// Label temuan di luar snapshot sesi (expected=false)
  ///
  /// In id, this message translates to:
  /// **'Di Luar Catatan'**
  String get opnameOutOfSnapshot;

  /// Label pemilih hasil pada sheet hasil scan
  ///
  /// In id, this message translates to:
  /// **'Hasil:'**
  String get opnameSheetResultLabel;

  /// Placeholder field catatan pada sheet hasil scan
  ///
  /// In id, this message translates to:
  /// **'Catatan (opsional)'**
  String get opnameSheetNoteHint;

  /// Label tombol simpan hasil pada sheet hasil scan
  ///
  /// In id, this message translates to:
  /// **'Simpan & Lanjut'**
  String get opnameSheetSave;

  /// Info pada sheet hasil scan untuk temuan expected=false
  ///
  /// In id, this message translates to:
  /// **'Aset ini di luar snapshot sesi — dicatat sebagai temuan di luar catatan.'**
  String get opnameSheetOutOfSnapshotInfo;

  /// SnackBar sukses setelah hasil item disimpan
  ///
  /// In id, this message translates to:
  /// **'Hasil tersimpan'**
  String get opnameResultSavedSnack;

  /// Pesan 404 saat tag hasil scan tidak dikenal server
  ///
  /// In id, this message translates to:
  /// **'Kode {tag} tidak dikenal atau di luar lingkup sesi.'**
  String opnameScanErrorNotFound(String tag);

  /// Pesan 409 saat scan pada sesi yang bukan tahap counting
  ///
  /// In id, this message translates to:
  /// **'Sesi tidak dalam tahap menghitung — pemindaian tidak diizinkan.'**
  String get opnameScanErrorNotCounting;

  /// Segmen kiri toggle layar variance (kembali ke counting)
  ///
  /// In id, this message translates to:
  /// **'Item'**
  String get opnameVarianceTabItems;

  /// Segmen kanan toggle layar variance (aktif)
  ///
  /// In id, this message translates to:
  /// **'Variance'**
  String get opnameVarianceTabVariance;

  /// Lokasi terakhir tercatat pada kartu item variance
  ///
  /// In id, this message translates to:
  /// **'terakhir: {location}'**
  String opnameVarianceLastLocation(String location);

  /// Kutipan catatan petugas pada kartu item variance
  ///
  /// In id, this message translates to:
  /// **'Catatan: \"{note}\"'**
  String opnameVarianceNote(String note);

  /// Status tindak lanjut kosong pada kartu item variance
  ///
  /// In id, this message translates to:
  /// **'Belum ditindaklanjuti'**
  String get opnameVarianceFollowupNone;

  /// Status tindak lanjut berupa pengajuan approval
  ///
  /// In id, this message translates to:
  /// **'Diajukan: menunggu approval'**
  String get opnameVarianceFollowupRequested;

  /// Status tindak lanjut berupa record maintenance (damaged)
  ///
  /// In id, this message translates to:
  /// **'Tiket maintenance dibuat'**
  String get opnameVarianceFollowupRecord;

  /// Judul empty state variance saat semua tercocokkan
  ///
  /// In id, this message translates to:
  /// **'Tidak ada selisih'**
  String get opnameVarianceEmptyTitle;

  /// Subjudul empty state variance
  ///
  /// In id, this message translates to:
  /// **'Semua {total} aset tercocokkan dengan catatan. Sesi siap diselesaikan dari aplikasi web.'**
  String opnameVarianceEmptyBody(int total);

  /// Catatan kaki layar variance
  ///
  /// In id, this message translates to:
  /// **'Penyelesaian sesi & Berita Acara dilakukan dari aplikasi web'**
  String get opnameVarianceFootnote;

  /// Sapaan header Beranda dengan nama panggilan pengguna
  ///
  /// In id, this message translates to:
  /// **'Halo, {name}'**
  String homeGreeting(String name);

  /// Label semantik avatar header Beranda (menuju profil)
  ///
  /// In id, this message translates to:
  /// **'Profil'**
  String get homeAccountTooltip;

  /// Label semantik tombol lonceng header Beranda
  ///
  /// In id, this message translates to:
  /// **'Notifikasi'**
  String get homeNotificationsTooltip;

  /// Banner offline Beranda (mockup state offline)
  ///
  /// In id, this message translates to:
  /// **'Offline — data terakhir ditampilkan'**
  String get homeOfflineBanner;

  /// Judul kartu ringkasan sesi opname aktif
  ///
  /// In id, this message translates to:
  /// **'Sesi Opname Aktif'**
  String get homeOpnameCardTitle;

  /// Isi kartu opname saat tidak ada sesi berjalan
  ///
  /// In id, this message translates to:
  /// **'Tidak ada sesi opname yang sedang berjalan.'**
  String get homeOpnameEmptyBody;

  /// Label aksi kartu opname kosong menuju tab Opname
  ///
  /// In id, this message translates to:
  /// **'Buka Opname'**
  String get homeOpnameOpenList;

  /// Isi kartu opname saat sumber datanya gagal
  ///
  /// In id, this message translates to:
  /// **'Gagal memuat sesi opname.'**
  String get homeOpnameErrorBody;

  /// Baris progress kartu opname Beranda (mockup: aset)
  ///
  /// In id, this message translates to:
  /// **'{counted} dari {total} aset'**
  String homeOpnameProgress(int counted, int total);

  /// Label CTA kartu opname Beranda (mockup)
  ///
  /// In id, this message translates to:
  /// **'Lanjutkan'**
  String get homeOpnameContinue;

  /// Judul kartu ringkasan approval menunggu
  ///
  /// In id, this message translates to:
  /// **'Approval Menunggu'**
  String get homeApprovalCardTitle;

  /// Subjudul kartu approval: jumlah pengajuan menunggu lebih dari 3 hari
  ///
  /// In id, this message translates to:
  /// **'{count} di antaranya > 3 hari'**
  String homeApprovalStale(int count);

  /// Subjudul kartu approval saat tidak ada pengajuan pending
  ///
  /// In id, this message translates to:
  /// **'Tidak ada pengajuan menunggu keputusan.'**
  String get homeApprovalEmptyBody;

  /// Isi kartu approval saat sumber datanya gagal
  ///
  /// In id, this message translates to:
  /// **'Gagal memuat pengajuan.'**
  String get homeApprovalErrorBody;

  /// Label aksi kartu approval menuju tab inbox
  ///
  /// In id, this message translates to:
  /// **'Buka Inbox'**
  String get homeApprovalOpenInbox;

  /// Label quick action pindai aset
  ///
  /// In id, this message translates to:
  /// **'Pindai Aset'**
  String get homeQuickScan;

  /// Label quick action tab opname
  ///
  /// In id, this message translates to:
  /// **'Sesi Opname'**
  String get homeQuickOpname;

  /// Label quick action tab approval
  ///
  /// In id, this message translates to:
  /// **'Approval'**
  String get homeQuickApproval;

  /// Label quick action tab notifikasi
  ///
  /// In id, this message translates to:
  /// **'Notifikasi'**
  String get homeQuickNotifications;

  /// Aksi header feed: tandai seluruh notifikasi dibaca
  ///
  /// In id, this message translates to:
  /// **'Tandai semua dibaca'**
  String get notificationsMarkAllRead;

  /// Snackbar saat POST /notifications/read-all gagal
  ///
  /// In id, this message translates to:
  /// **'Gagal menandai semua dibaca. Coba lagi.'**
  String get notificationsMarkAllFailed;

  /// Header seksi feed untuk notifikasi hari ini
  ///
  /// In id, this message translates to:
  /// **'Hari ini'**
  String get notificationsSectionToday;

  /// Header seksi feed untuk notifikasi kemarin
  ///
  /// In id, this message translates to:
  /// **'Kemarin'**
  String get notificationsSectionYesterday;

  /// Judul empty state feed notifikasi
  ///
  /// In id, this message translates to:
  /// **'Belum ada notifikasi'**
  String get notificationsEmptyTitle;

  /// Subjudul empty state feed notifikasi
  ///
  /// In id, this message translates to:
  /// **'Pemberitahuan approval, maintenance, dan sinkronisasi akan muncul di sini.'**
  String get notificationsEmptyBody;

  /// Judul state error feed notifikasi
  ///
  /// In id, this message translates to:
  /// **'Gagal memuat notifikasi'**
  String get notificationsErrorTitle;

  /// Subjudul error feed saat offline/gangguan jaringan
  ///
  /// In id, this message translates to:
  /// **'Tidak ada koneksi. Periksa jaringan Anda lalu coba lagi.'**
  String get notificationsErrorNetworkBody;

  /// Subjudul error feed untuk kegagalan lain
  ///
  /// In id, this message translates to:
  /// **'Terjadi kesalahan. Coba lagi.'**
  String get notificationsErrorGenericBody;

  /// Baris kaki feed saat halaman berikutnya gagal dimuat
  ///
  /// In id, this message translates to:
  /// **'Gagal memuat lebih banyak.'**
  String get notificationsLoadMoreFailed;

  /// Label waktu kartu notifikasi di bawah satu menit
  ///
  /// In id, this message translates to:
  /// **'baru saja'**
  String get notificationsTimeJustNow;

  /// Label waktu kartu notifikasi dalam menit (hari ini)
  ///
  /// In id, this message translates to:
  /// **'{count} menit lalu'**
  String notificationsTimeMinutesAgo(int count);

  /// Label waktu kartu notifikasi dalam jam (hari ini)
  ///
  /// In id, this message translates to:
  /// **'{count} jam lalu'**
  String notificationsTimeHoursAgo(int count);

  /// Label waktu kartu notifikasi kemarin dengan jam
  ///
  /// In id, this message translates to:
  /// **'Kemarin, {time}'**
  String notificationsTimeYesterdayAt(String time);

  /// Label waktu kartu notifikasi lebih dari dua hari: tanggal pendek + jam
  ///
  /// In id, this message translates to:
  /// **'{date}, {time}'**
  String notificationsTimeAt(String date, String time);

  /// Judul notifikasi type approval_pending (dirender klien, ADR-0014)
  ///
  /// In id, this message translates to:
  /// **'Pengajuan menunggu persetujuan Anda'**
  String get notificationsApprovalPendingTitle;

  /// Isi notifikasi approval_pending dari params request_type + step
  ///
  /// In id, this message translates to:
  /// **'{type} · Langkah {step}'**
  String notificationsApprovalPendingBody(String type, String step);

  /// Judul notifikasi approval_decided berstatus approved
  ///
  /// In id, this message translates to:
  /// **'Pengajuan Anda disetujui'**
  String get notificationsApprovalApprovedTitle;

  /// Judul notifikasi approval_decided berstatus rejected
  ///
  /// In id, this message translates to:
  /// **'Pengajuan Anda ditolak'**
  String get notificationsApprovalRejectedTitle;

  /// Judul fallback approval_decided untuk status di luar approved/rejected
  ///
  /// In id, this message translates to:
  /// **'Pengajuan Anda telah diputus'**
  String get notificationsApprovalDecidedTitle;

  /// Judul notifikasi type maintenance_due
  ///
  /// In id, this message translates to:
  /// **'Maintenance jatuh tempo'**
  String get notificationsMaintenanceDueTitle;

  /// Isi notifikasi maintenance_due dari params aset + due_date
  ///
  /// In id, this message translates to:
  /// **'{asset} — jatuh tempo {date}'**
  String notificationsMaintenanceDueBody(String asset, String date);

  /// Isi maintenance_due saat params aset absen, hanya due_date
  ///
  /// In id, this message translates to:
  /// **'Jatuh tempo {date}'**
  String notificationsMaintenanceDueDateOnly(String date);

  /// Judul notifikasi type asset_returned
  ///
  /// In id, this message translates to:
  /// **'Aset dikembalikan'**
  String get notificationsAssetReturnedTitle;
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
