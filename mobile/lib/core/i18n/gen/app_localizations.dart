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
